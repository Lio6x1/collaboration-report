// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	codec "collaboration/contracts/content"
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"collaboration/platform"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"google.golang.org/protobuf/proto"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// search_audience.go：保留原实现，按业务合并文件。

type searchAudience struct {
	identity *Identity
	access   *Access
	actor    *Actor
	spaces   map[uint64]*pb.IdentityAdmission
	proofs   map[string]*pb.IdentityAdmission
}

func newSearchAudience(identity *Identity, access *Access, actor *Actor) *searchAudience {
	return &searchAudience{identity: identity, access: access, actor: actor, spaces: make(map[uint64]*pb.IdentityAdmission), proofs: make(map[string]*pb.IdentityAdmission)}
}

func (a *searchAudience) authorize(ctx context.Context, facts domain.PublicationFacts, policy string) (*DocumentAccess, []*pb.SourceEpoch, bool, error) {
	if a.actor == nil && policy != "PUBLIC" {
		return nil, nil, false, nil
	}
	proofs := []*pb.IdentityAdmission{}
	var access *DocumentAccess
	if a.actor != nil {
		proofs = append(proofs, a.actor.Proof)
		switch policy {
		case "SPACE_MEMBERS":
			space, err := domain.CanonicalID(facts.Space.Id)
			if err != nil {
				return nil, nil, false, err
			}
			proof := a.spaces[space]
			if proof == nil {
				_, proof, err = a.access.SpaceIdentity(ctx, *a.actor, space, pb.IdentityCapability_ORGANIZATION_MEMBER, "READ_PUBLIC_DOCUMENT", 0)
				if err != nil {
					return a.denied(ctx, err)
				}
				a.spaces[space] = proof
			}
			proofs = append(proofs, proof)
		case "COLLABORATORS":
			id, err := domain.CanonicalID(facts.Document.Id)
			if err != nil {
				return nil, nil, false, err
			}
			value, err := a.access.Authorize(ctx, *a.actor, id, "READ_PUBLIC_DOCUMENT")
			if err != nil {
				return a.denied(ctx, err)
			}
			// 内部授权读的是当前主线资格；若与候选一致快照已有冲突，中止整页而非输出旧摘要。
			if value.Document.AclRevision != facts.Document.AclRevision || value.Document.SpaceId != facts.Document.SpaceId || optionalText(value.Document.KnowledgeBaseId) != optionalText(facts.Document.KnowledgeBaseId) {
				return nil, nil, false, domain.Violation(domain.StateConflict, "PAGE_CHANGED_RETRY")
			}
			access = &value
			proofs = append(proofs, value.Proofs...)
		}
	}
	identitySources, err := MergeSources(proofs...)
	if err != nil {
		return nil, nil, false, err
	}
	for _, proof := range proofs {
		if err = a.identity.Revalidate(ctx, proof); err != nil {
			return nil, nil, false, err
		}
	}
	publication, err := publicationSources(facts)
	if err != nil {
		return nil, nil, false, err
	}
	groups := [][]*pb.SourceEpoch{identitySources, publication}
	if access != nil {
		groups = append(groups, access.Sources)
	}
	sources, err := searchSourceUnion(groups...)
	if err == nil {
		for _, proof := range proofs {
			a.proofs[proof.AdmissionId] = proof
		}
	}
	return access, sources, err == nil, err
}

func (a *searchAudience) finalProofs() []*pb.IdentityAdmission {
	result := make([]*pb.IdentityAdmission, 0, len(a.proofs)+1)
	if a.actor != nil {
		result = append(result, a.actor.Proof)
	}
	for _, proof := range a.proofs {
		if a.actor == nil || proof.AdmissionId != a.actor.Proof.AdmissionId {
			result = append(result, proof)
		}
	}
	return result
}

func (a *searchAudience) denied(ctx context.Context, err error) (*DocumentAccess, []*pb.SourceEpoch, bool, error) {
	var failure domain.RuleViolation
	if errors.As(err, &failure) && failure.Kind == domain.PermissionDenied {
		// 同一个PermissionDenied可能来自账户注销或目标资格不足。再次确认真实账户，
		// 只有账户仍有效的确定目标拒绝才能略过候选，不能匿名降级失效会话。
		if err = a.identity.Revalidate(ctx, a.actor.Proof); err != nil {
			return nil, nil, false, err
		}
		return nil, nil, false, nil
	}
	if errors.As(err, &failure) && failure.Kind == domain.EntityNotFound {
		return nil, nil, false, domain.Violation(domain.StateConflict, "PAGE_CHANGED_RETRY")
	}
	return nil, nil, false, err
}

// search_candidates.go：保留原实现，按业务合并文件。

type SearchCandidatesPeer interface {
	SearchCandidates(context.Context, *pb.SearchCandidatesRequest) (*pb.SearchCandidatesReply, error)
	CloseCursor(context.Context, *pb.CloseCursorRequest) (*pb.OperationResult, error)
}

// 扫描器只依赖候选查询，权限/固定内容通过逐批一致读取的评估器接入。
// 它不能授予阅读权限，也不能把组件测试当真实SQL、RPC或输出验收。
type searchCandidatePeer interface {
	SearchCandidates(context.Context, *pb.SearchCandidatesRequest) (*pb.SearchCandidatesReply, error)
}

type searchPageEntry struct {
	Summary dto.SearchResult
	Facts   domain.PublicationFacts
	Access  *DocumentAccess
	Sources []*pb.SourceEpoch
}

type searchCollection struct {
	// 至多limit+1条；最后一条lookahead也参加最终权限和联合输出复核。
	Entries []searchPageEntry
	Next    *domain.SearchPageCursor
	Until   int64
}

type searchCandidateEvaluator func(context.Context, *pb.SearchCandidate) (*searchPageEntry, error)
type searchBatchEvaluator func(context.Context, []*pb.SearchCandidate) (searchCandidateEvaluator, error)

func searchIncomplete(err error) error {
	var failure domain.RuleViolation
	if errors.As(err, &failure) {
		return failure
	}
	return domain.Violation(domain.TemporarilyUnavailable, "SEARCH_INCOMPLETE")
}

func collectSearchPage(ctx context.Context, peer searchCandidatePeer, query domain.SearchQuery, scope *pb.SearchScope, binding [32]byte, start *domain.SearchPageCursor, evaluate searchBatchEvaluator) (searchCollection, error) {
	fail := func(err error) (searchCollection, error) { return searchCollection{}, searchIncomplete(err) }
	if query.Limit < 1 || query.Limit > 100 || peer == nil || evaluate == nil || scope == nil || binding == [32]byte{} {
		return fail(nil)
	}
	digest := sha256.Sum256([]byte(query.Text))
	request := &pb.SearchCandidatesRequest{Query: query.Text, QueryDigest: hex.EncodeToString(digest[:]), Scope: scope, PageSize: 100}
	var until int64
	prefix := 0
	if start != nil {
		if start.Binding != binding {
			return fail(domain.Violation(domain.StateConflict, "CURSOR_STALE"))
		}
		if start.Until <= time.Now().UnixMilli() {
			return fail(domain.Violation(domain.StateConflict, "CURSOR_EXPIRED"))
		}
		var err error
		request.Cursor, err = domain.EnvelopeSearchAnchor(start.Anchor, true)
		if err != nil {
			return fail(err)
		}
		until, prefix = start.Until, int(start.Prefix)
	}
	page := searchCollection{Entries: make([]searchPageEntry, 0, query.Limit+1)}
	var physical string
	var generation uint64
	checkDeadline := func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if until != 0 && until <= time.Now().UnixMilli() {
			return domain.Violation(domain.StateConflict, "CURSOR_EXPIRED")
		}
		return nil
	}
	for {
		if err := checkDeadline(); err != nil {
			return fail(err)
		}
		batch, err := peer.SearchCandidates(ctx, request)
		if err != nil {
			return fail(err)
		}
		if batch == nil || !batch.Complete || len(batch.Candidates) > 100 || (batch.HasMore && len(batch.Candidates) != 100) || batch.ExpiresUnixMs <= time.Now().UnixMilli() {
			return fail(nil)
		}
		if until == 0 {
			until = batch.ExpiresUnixMs
			physical, generation = batch.PhysicalIndex, batch.IndexGeneration
		}
		if physical == "" {
			physical, generation = batch.PhysicalIndex, batch.IndexGeneration
		}
		page.Until = until
		// 同链的绝对期限、物理索引和代次不可随批次变化；原HTTP期限也不被延长。
		if until != batch.ExpiresUnixMs || physical != batch.PhysicalIndex || generation != batch.IndexGeneration {
			return fail(nil)
		}
		if prefix > len(batch.Candidates) || (start != nil && prefix == len(batch.Candidates)) {
			return fail(domain.Violation(domain.InvalidRequest, "CURSOR_INVALID"))
		}
		assess, err := evaluate(ctx, batch.Candidates)
		if err != nil || assess == nil {
			return fail(err)
		}
		for index := prefix; index < len(batch.Candidates); index++ {
			if err = ctx.Err(); err != nil {
				return fail(err)
			}
			entry, err := assess(ctx, batch.Candidates[index])
			if err != nil {
				return fail(err)
			}
			// 正文与身份检查可能耗完原期限；最后一项也须复核，不能靠下一轮循环兜底。
			if err = checkDeadline(); err != nil {
				return fail(err)
			}
			if entry == nil {
				continue
			}
			page.Entries = append(page.Entries, *entry)
			if len(page.Entries) == query.Limit+1 {
				// Prefix停在lookahead之前，而不是批尾。终批HasMore=false仍须保留锚点。
				// 下一页REPLAY并重新核验这个候选，不能复用本页旧权限或跳过它。
				page.Next = &domain.SearchPageCursor{Anchor: append([]byte(nil), batch.Continuation...), Prefix: uint8(index), Binding: binding, Until: until}
				return page, nil
			}
		}
		if !batch.HasMore {
			if err = checkDeadline(); err != nil {
				return fail(err)
			}
			return page, nil
		}
		request.Cursor, err = domain.EnvelopeSearchAnchor(batch.Continuation, false)
		if err != nil {
			return fail(err)
		}
		prefix = 0
		start = nil
		// 不设1000候选的静默截断。只有找到lookahead或真实EOF才能成功，
		// 其余情况由原请求期限失败；返回的空结构保证未知结果不泄露半页。
	}
}

// search_commit.go：保留原实现，按业务合并文件。

func (s *SearchPages) commitSearchPage(ctx context.Context, actor *Actor, entries []searchPageEntry, proofs []*pb.IdentityAdmission, until int64, handler string, bind BindOutput) error {
	if handler == "" || bind == nil {
		return domain.Violation(domain.TemporarilyUnavailable, "EDGE_OUTPUT_REQUIRED")
	}
	groups := make([][]*pb.SourceEpoch, 0, len(entries)+1)
	var proofUntil int64
	for _, proof := range proofs {
		if proof == nil || proof.ExpiresUnixMs <= 0 {
			return searchIncomplete(nil)
		}
		if err := s.identity.Revalidate(ctx, proof); err != nil {
			return err
		}
		if proofUntil == 0 || proof.ExpiresUnixMs < proofUntil {
			proofUntil = proof.ExpiresUnixMs
		}
	}
	checkDeadline := func() error {
		if err := ctx.Err(); err != nil {
			return searchIncomplete(err)
		}
		now := time.Now().UnixMilli()
		if until <= now {
			return domain.Violation(domain.StateConflict, "CURSOR_EXPIRED")
		}
		if actor != nil && actor.SessionExpiresUnixMs <= now {
			return domain.Violation(domain.SessionRequired, "SESSION_EXPIRED")
		}
		if proofUntil != 0 && proofUntil <= now {
			return domain.Violation(domain.StateConflict, "PAGE_CHANGED_RETRY")
		}
		return nil
	}
	if err := checkDeadline(); err != nil {
		return err
	}
	if actor != nil {
		sources, err := MergeSources(actor.Proof)
		if err != nil {
			return err
		}
		groups = append(groups, sources)
	}
	for _, entry := range entries {
		groups = append(groups, entry.Sources)
	}
	sources, err := searchSourceUnion(groups...)
	if err != nil {
		return err
	}
	if len(sources) == 0 {
		return domain.Violation(domain.TemporarilyUnavailable, "SEARCH_OUTPUT_SOURCES_REQUIRED")
	}
	// 所有跨进程RPC/Edge检查在SQL锁外。两个进程登记各自真实handler/instance，
	// 使用相同完整来源并集，而非将多文档页伪装成某个Doc的准入。
	edge, err := s.outputs.EdgeInstance(ctx, handler)
	if err != nil || edge == nil {
		return domain.Violation(domain.TemporarilyUnavailable, "EDGE_OUTPUT_UNAVAILABLE")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	locked, err := lockSearchOutputSources(ctx, tx, sources)
	if err != nil {
		return searchFinalChange(err)
	}
	ordered := append([]searchPageEntry(nil), entries...)
	sort.Slice(ordered, func(i, j int) bool {
		left, _ := domain.CanonicalID(ordered[i].Facts.Document.Id)
		right, _ := domain.CanonicalID(ordered[j].Facts.Document.Id)
		return left < right
	})
	for _, entry := range ordered {
		id, err := domain.CanonicalID(entry.Facts.Document.Id)
		if err != nil {
			return searchFinalChange(err)
		}
		current, err := repository.PublicationFactsRow(ctx, tx, id, true)
		if err != nil {
			return searchFinalChange(err)
		}
		if publicFactsBinding(current) != publicFactsBinding(entry.Facts) {
			return domain.Violation(domain.StateConflict, "PAGE_CHANGED_RETRY")
		}
		if entry.Access != nil {
			if _, err = ValidateAccess(ctx, tx, *entry.Access); err != nil {
				return searchFinalChange(err)
			}
			// responsible字段不是只有nil/non-nil之分；保留确切资格绑定，不靠旧快照推断。
			if optionalText(current.Document.ResponsibleUserId) != optionalText(entry.Access.Document.ResponsibleUserId) {
				return domain.Violation(domain.StateConflict, "PAGE_CHANGED_RETRY")
			}
		}
	}
	registration, err := s.outputs.registerLockedSearch(ctx, locked, handler, edge)
	if err != nil {
		return err
	}
	output := bind(registration, locked.copySources())
	if output == nil {
		return domain.Violation(domain.TemporarilyUnavailable, "SEARCH_OUTPUT_BINDING_INVALID")
	}
	// 锁等待、Edge及输出绑定也在原期限内；SQL内不发RPC，只核验已验证证明的绝对期限。
	if err = checkDeadline(); err != nil {
		output.Abort()
		return err
	}
	if err = tx.Commit(); err != nil {
		output.Abort()
		return domain.Violation(domain.TemporarilyUnavailable, "OUTPUT_COMMIT_UNKNOWN")
	}
	if err = checkDeadline(); err != nil {
		output.Abort()
		return err
	}
	return nil
}

func searchFinalChange(err error) error {
	var failure domain.RuleViolation
	if errors.As(err, &failure) && (failure.Kind == domain.StateConflict || failure.Kind == domain.EntityNotFound || failure.Kind == domain.PermissionDenied) {
		return domain.Violation(domain.StateConflict, "PAGE_CHANGED_RETRY")
	}
	return searchIncomplete(err)
}

// search_fixed.go：保留原实现，按业务合并文件。

func (s *SearchPages) fixedSearchSummary(ctx context.Context, facts domain.PublicationFacts, release uint64, query string, used uint64) (dto.SearchResult, uint64, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return dto.SearchResult{}, 0, err
	}
	defer tx.Rollback()
	value, next, err := s.fixedSearchSummaryRow(ctx, tx, facts, release, query, used)
	if err != nil {
		return dto.SearchResult{}, 0, err
	}
	if err = tx.Commit(); err != nil {
		return dto.SearchResult{}, 0, err
	}
	return value, next, nil
}

// 固定清单与正文来自同一只读事务；它不持有权限锁。最终整页事务会重新核验
// 当前正式指针与联合来源，期间任何切版/撤下都不能输出这个乐观摘要。
func (s *SearchPages) fixedSearchSummaryRow(ctx context.Context, tx *sql.Tx, facts domain.PublicationFacts, release uint64, query string, used uint64) (dto.SearchResult, uint64, error) {
	invalid := domain.Violation(domain.TemporarilyUnavailable, "SEARCH_FIXED_CONTENT_INVALID")
	fail := func(err error) (dto.SearchResult, uint64, error) {
		if err == nil {
			err = invalid
		}
		return dto.SearchResult{}, 0, err
	}
	id, err := domain.CanonicalID(facts.Document.Id)
	if err != nil {
		return fail(err)
	}
	fixed, err := repository.FixedReleaseRow(ctx, tx, id, release)
	if err != nil {
		return fail(err)
	}
	if fixed.Release.DocumentId != facts.Document.Id || fixed.Release.Id != domain.TextID(release) || !utf8.ValidString(fixed.Title) || utf8.RuneCountInString(fixed.Title) > 240 {
		return fail(nil)
	}
	structure, err := repository.StructureRow(ctx, tx, fixed.Structure, facts.Document.Id)
	if err != nil {
		return fail(err)
	}
	style, err := repository.StyleRow(ctx, tx, fixed.Style, facts.Document.Id)
	if err != nil {
		return fail(err)
	}
	if structure.SchemaVersion != "structure-v1" || style.SchemaVersion != "style-v1" || len(structure.Entries) > domain.SearchManifestMaxUnits {
		return fail(nil)
	}
	units, err := repository.ManifestUnits(ctx, tx, fixed.Release.ManifestId, id)
	if err != nil {
		return fail(err)
	}
	if err = validateReadingUnits(structure, units); err != nil {
		return fail(nil)
	}
	manifest, err := domain.CanonicalID(fixed.Release.ManifestId)
	if err != nil {
		return fail(err)
	}
	metadata, err := repository.SearchManifestMetadata(ctx, tx, manifest, id, len(units))
	if err != nil {
		return fail(err)
	}
	sizes := make([]uint64, 0, len(metadata))
	byUnit := make(map[string]domain.SearchManifestUnit, len(metadata))
	for _, unit := range metadata {
		sizes = append(sizes, unit.ByteSize)
		byUnit[domain.TextID(unit.Unit)] = unit
	}
	for _, unit := range units {
		meta, present := byUnit[unit["unit_id"]]
		if !present || domain.TextID(meta.Version) != unit["content_version_id"] {
			return fail(nil)
		}
	}
	next := used
	if len(metadata) > 0 {
		// 整篇元数据占额发生在任何VersionContent正文读取前；lookahead共用此累计预算。
		// 空正式结构经上面的完整归属检查，没有正文可占额，不伪造零字节版本。
		next, err = domain.ReserveSearchBody(used, sizes)
		if err != nil {
			return fail(err)
		}
	} else if used > domain.SearchPageBodyLimit {
		return fail(domain.Violation(domain.TemporarilyUnavailable, "SEARCH_INCOMPLETE"))
	}
	structureID, err := domain.CanonicalID(fixed.Structure)
	if err != nil {
		return fail(err)
	}
	value := dto.SearchResult{DocumentId: facts.Document.Id, ReleaseId: fixed.Release.Id, Title: fixed.Title}
	matched := false
	seen := make(map[string]bool, len(metadata))
	for _, entry := range structure.Entries {
		if entry.UnitId == nil {
			continue
		}
		if seen[*entry.UnitId] {
			return fail(nil)
		}
		seen[*entry.UnitId] = true
		if _, err = domain.CanonicalID(entry.ChapterId); err != nil {
			return fail(nil)
		}
		unit, present := byUnit[*entry.UnitId]
		if !present {
			return fail(nil)
		}
		text, err := s.searchUnitText(ctx, tx, id, structureID, unit)
		if err != nil {
			return fail(err)
		}
		match := strings.Contains(strings.ToLower(text), strings.ToLower(query)) || strings.Contains(strings.ToLower(entry.Title), strings.ToLower(query))
		if (!matched && match) || (value.Snippet == "" && !matched && text != "") {
			value.Snippet = text
			chapter := entry.ChapterId
			value.ChapterId = &chapter
			matched = match
		}
		// 达到摘要2000字符后仍验证全部剩余单位；损坏后缀不能成为部分成功。
	}
	if len(seen) != len(metadata) {
		return fail(nil)
	}
	return value, next, nil
}

func (s *SearchPages) searchUnitText(ctx context.Context, tx *sql.Tx, document, structure uint64, unit domain.SearchManifestUnit) (string, error) {
	invalid := domain.Violation(domain.TemporarilyUnavailable, "SEARCH_FIXED_CONTENT_INVALID")
	content, err := repository.VersionContent(ctx, tx, unit.Version, document)
	if err != nil {
		return "", err
	}
	// 单个固定单元只在此函数持有，完成后清除原字节并释放树，不装配PublicDocument.units。
	defer func() { clear(content.BodyJsonUtf8); clear(content.ProvenanceJsonUtf8) }()
	if content.ByteSize != unit.ByteSize || uint64(len(content.BodyJsonUtf8)) != unit.ByteSize {
		return "", invalid
	}
	// canonical_body是BLOB，必须保留并验证原规范字节/hash；provenance_json是MySQL
	// JSON列，只保存语义，会重排键并添加空白。先沿既有读取规则规范化它，再核完整
	// schema/资源/绑定，不能把MySQL正常的JSON词法改写误判成正文损坏。
	if canonicalVersion(content, unit.Version) != nil || validateFixedMergeContent(s.validator, content, document, unit.Unit, structure) != nil {
		return "", invalid
	}
	body, err := codec.Decode(content.BodyJsonUtf8)
	if err != nil {
		return "", invalid
	}
	text, err := searchSnippet(body)
	if err != nil {
		return "", invalid
	}
	return text, nil
}

// search_output_sources.go：保留原实现，按业务合并文件。

// 仅搜索最终事务内部拥有此证明；不能以bool声明已锁定，不能换tx或来源。
// 持证明后的事实复核只读，不修改gate/transition；原tx结束即失效。
type lockedOutputSources struct {
	tx       *sql.Tx
	sources  []*pb.SourceEpoch
	consumed bool
}

func (p *lockedOutputSources) copySources() []*pb.SourceEpoch {
	values := make([]*pb.SourceEpoch, len(p.sources))
	for i, source := range p.sources {
		values[i] = proto.Clone(source).(*pb.SourceEpoch)
	}
	return values
}

func lockSearchOutputSources(ctx context.Context, tx *sql.Tx, sources []*pb.SourceEpoch) (*lockedOutputSources, error) {
	if tx == nil || len(sources) == 0 {
		return nil, errors.New("SOURCE_BINDING_INVALID")
	}
	proof := &lockedOutputSources{tx: tx, sources: make([]*pb.SourceEpoch, len(sources))}
	for i, source := range sources {
		if source == nil || source.Epoch == 0 {
			return nil, errors.New("SOURCE_BINDING_INVALID")
		}
		proof.sources[i] = proto.Clone(source).(*pb.SourceEpoch)
	}
	sort.Slice(proof.sources, func(i, j int) bool { return proof.sources[i].ScopeKey < proof.sources[j].ScopeKey })
	gates, err := repository.LockSourceGatesBatch(ctx, tx, SourcesWire(proof.sources))
	if err != nil {
		return nil, err
	}
	documentKeys := make([]string, 0, len(proof.sources))
	for _, source := range proof.sources {
		if platform.ValidDocumentScope(source.ScopeKey) {
			documentKeys = append(documentKeys, source.ScopeKey)
		}
	}
	pending, err := repository.PendingSourceTransitionsBatch(ctx, tx, documentKeys)
	if err != nil {
		return nil, err
	}
	if pending {
		return nil, domain.Violation(domain.StateConflict, "SCOPE_TRANSITION_PENDING")
	}
	for _, source := range proof.sources {
		gate := gates[source.ScopeKey]
		if gate.Epoch > source.Epoch || (gate.Epoch == source.Epoch && gate.State == "BLOCKED") {
			return nil, domain.Violation(domain.PermissionDenied, "SOURCE_CLOSED")
		}
		if gate.Epoch < source.Epoch {
			if err := repository.ReopenSourceGate(ctx, tx, source.ScopeKey, source.Epoch); err != nil {
				return nil, err
			}
		}
	}
	return proof, nil
}

func (m *Outputs) registerLockedSearch(ctx context.Context, proof *lockedOutputSources, handler string, edge *domain.OutputInstance) (*ResponseRegistration, error) {
	if proof == nil || proof.tx == nil || proof.consumed || len(proof.sources) == 0 || handler == "" || edge == nil {
		return nil, errors.New("SEARCH_OUTPUT_PROOF_INVALID")
	}
	proof.consumed = true
	// SQL仍在原tx执行；已回滚/提交的证明将被database/sql拒绝。
	return m.writeResponseRegistration(ctx, proof.tx, proof.copySources(), domain.OutputLease{}, handler, edge)
}

// search_page.go：保留原实现，按业务合并文件。

type SearchPages struct {
	db        *sql.DB
	identity  *Identity
	access    *Access
	outputs   *Outputs
	peer      SearchCandidatesPeer
	cursors   *domain.SearchCursorCodec
	validator *codec.Validator
	admission chan struct{}
}

func NewSearchPages(db *sql.DB, identity *Identity, access *Access, outputs *Outputs, peer SearchCandidatesPeer, cursors *domain.SearchCursorCodec, validator *codec.Validator) *SearchPages {
	return &SearchPages{db: db, identity: identity, access: access, outputs: outputs, peer: peer, cursors: cursors, validator: validator, admission: make(chan struct{}, 2)}
}

func (s *SearchPages) Page(ctx context.Context, actor *Actor, query domain.SearchQuery, handler string, bind BindOutput) (dto.SearchResultPage, error) {
	fail := func(err error) (dto.SearchResultPage, error) { return dto.SearchResultPage{}, searchIncomplete(err) }
	if s == nil || s.db == nil || s.peer == nil || s.cursors == nil || s.validator == nil || s.outputs == nil || s.identity == nil || s.access == nil {
		return fail(nil)
	}
	// 在任何SQL读取前准入，保留原8连接池给消费、关闭与其他业务；不等持锁后再排队。
	select {
	case s.admission <- struct{}{}:
		defer func() { <-s.admission }()
	case <-ctx.Done():
		return fail(ctx.Err())
	}
	var session []byte
	if actor != nil {
		if actor.Proof == nil || actor.ID == 0 || actor.Proof.ActorId != actor.ID {
			return fail(nil)
		}
		session = actor.Proof.SessionRef
		if err := s.identity.Revalidate(ctx, actor.Proof); err != nil {
			return fail(err)
		}
	}
	binding, err := query.Binding(actor == nil, session)
	if err != nil {
		return fail(err)
	}
	scope := &pb.SearchScope{ScopeHash: hex.EncodeToString(binding[:]), Anonymous: actor == nil, SessionRef: append([]byte(nil), session...)}
	if query.SpaceID != 0 {
		scope.SpaceIds = []uint64{query.SpaceID}
	}
	var start *domain.SearchPageCursor
	if query.Cursor != "" {
		cursor, err := s.cursors.Open(query.Cursor, binding, time.Now())
		if err != nil {
			return fail(err)
		}
		start = &cursor
	}
	var bodyBytes uint64
	audience := newSearchAudience(s.identity, s.access, actor)
	collection, err := collectSearchPage(ctx, s.peer, query, scope, binding, start, func(ctx context.Context, candidates []*pb.SearchCandidate) (searchCandidateEvaluator, error) {
		facts, err := s.batchFacts(ctx, candidates)
		if err != nil {
			return nil, err
		}
		if actor != nil {
			if err = s.identity.Revalidate(ctx, actor.Proof); err != nil {
				return nil, err
			}
		}
		return func(ctx context.Context, candidate *pb.SearchCandidate) (*searchPageEntry, error) {
			current, present := facts[candidate.DocumentId]
			if !present {
				return nil, nil
			}
			policy, visible, err := qualifySearchCandidate(candidate, current, query.SpaceID)
			if err != nil || !visible {
				return nil, err
			}
			access, sources, allowed, err := audience.authorize(ctx, current, policy)
			if err != nil || !allowed {
				return nil, err
			}
			summary, nextBytes, err := s.fixedSearchSummary(ctx, current, candidate.ReleaseId, query.Text, bodyBytes)
			if err != nil {
				return nil, err
			}
			bodyBytes = nextBytes
			return &searchPageEntry{Summary: summary, Facts: current, Access: access, Sources: sources}, nil
		}, nil
	})
	if err != nil {
		return fail(err)
	}
	page := dto.SearchResultPage{Items: make([]dto.SearchResult, 0, query.Limit), HasMore: collection.Next != nil}
	for i, entry := range collection.Entries {
		if i < query.Limit {
			page.Items = append(page.Items, entry.Summary)
		}
	}
	if collection.Next != nil {
		cursor, err := s.cursors.Seal(*collection.Next)
		if err != nil {
			return fail(err)
		}
		page.NextCursor = &cursor
	}
	// 完全耗尽的匿名空页没有标题、正文或游标，不捏造账户/单Doc输出准入。
	if actor == nil && len(collection.Entries) == 0 && collection.Next == nil {
		return page, nil
	}
	if err = s.commitSearchPage(ctx, actor, collection.Entries, audience.finalProofs(), collection.Until, handler, bind); err != nil {
		return fail(err)
	}
	// 正常EOF保留Search原链至原期限，支持HTTP回复丢失后的同锚点重试。
	// 不在最后一批已输出时提前Close，从而破坏终批后缀或前页重试。
	return page, nil
}

func (s *SearchPages) batchFacts(ctx context.Context, candidates []*pb.SearchCandidate) (map[uint64]domain.PublicationFacts, error) {
	ids := make([]uint64, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate == nil {
			return nil, searchIncomplete(nil)
		}
		ids = append(ids, candidate.DocumentId)
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	facts, err := repository.SearchPublicationFacts(ctx, tx, ids)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return facts, nil
}

// search_qualification.go：保留原实现，按业务合并文件。

func qualifySearchCandidate(candidate *pb.SearchCandidate, facts domain.PublicationFacts, space uint64) (string, bool, error) {
	if candidate == nil || facts.Document.Id != domain.TextID(candidate.DocumentId) {
		return "", false, domain.Violation(domain.TemporarilyUnavailable, "SEARCH_FACTS_BINDING_INVALID")
	}
	currentSpace, err := domain.CanonicalID(facts.Document.SpaceId)
	if err != nil {
		return "", false, err
	}
	// ES可以落后，但不拥有正式指针。明确旧版本/水位/空间的候选先排除，
	// 不读旧正文，也不拿管理标题或MAIN回退填充页面。
	if (space != 0 && currentSpace != space) || facts.Sequence != candidate.AggregateSeq || facts.Document.CurrentReleaseId == nil || *facts.Document.CurrentReleaseId != domain.TextID(candidate.ReleaseId) {
		return "", false, nil
	}
	policy, err := domain.PublicPolicy(facts)
	if err != nil {
		return "", false, err
	}
	if !domain.HasCurrentPublication(facts, policy) {
		return "", false, nil
	}
	return policy, true, nil
}

func searchSourceUnion(groups ...[]*pb.SourceEpoch) ([]*pb.SourceEpoch, error) {
	values := map[string]uint64{}
	for _, group := range groups {
		for _, source := range group {
			if source == nil || source.ScopeKey == "" || source.Epoch == 0 {
				return nil, domain.Violation(domain.StateConflict, "PAGE_CHANGED_RETRY")
			}
			if epoch, present := values[source.ScopeKey]; present && epoch != source.Epoch {
				return nil, domain.Violation(domain.StateConflict, "PAGE_CHANGED_RETRY")
			}
			values[source.ScopeKey] = source.Epoch
		}
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]*pb.SourceEpoch, 0, len(keys))
	for _, key := range keys {
		result = append(result, &pb.SourceEpoch{ScopeKey: key, Epoch: values[key]})
	}
	return result, nil
}
