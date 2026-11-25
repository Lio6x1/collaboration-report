// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	"bytes"
	codec "collaboration/contracts/content"
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"google.golang.org/protobuf/proto"
	"reflect"
	"sort"
	"time"
)

// conflict_contexts.go：保留原实现，按业务合并文件。

func (s *Reviews) LookupConflict(ctx context.Context, operation string) (domain.MergeFact, error) {
	if !domain.ValidOperationID(operation) {
		return domain.MergeFact{}, domain.Violation(domain.InvalidRequest, "CONFLICT_REF_INVALID")
	}
	fact, err := repository.MergeRow(ctx, s.db, operation)
	if err == sql.ErrNoRows {
		return fact, domain.Violation(domain.EntityNotFound, "CONFLICT_CONTEXT_NOT_FOUND")
	}
	return fact, err
}

func (s *Reviews) Conflict(ctx context.Context, actor Actor, fact domain.MergeFact) ([]*pb.SourceEpoch, ReadBody, error) {
	pointer := domain.ReviewPointer{ID: fact.Request, Document: fact.Document, Unit: fact.Unit, Branch: fact.Branch}
	_, access, err := s.authorized(ctx, actor, pointer, "READ_REVIEW_HISTORY")
	if err != nil {
		return nil, nil, err
	}
	// 旧冲突尚无HTTP版本引用时，只根据原固定事实建立幂等快照。互斥先于来源锁，与写用例锁序一致。
	// 这一步不输出正文；后面的独立读取事务仍须再次核验当前来源并持有真实输出租约。
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback()
	if err = repository.LockOperationGate(ctx, tx, fact.Operation); err != nil {
		return nil, nil, err
	}
	if err = LockSources(ctx, tx, access.Sources); err != nil {
		return nil, nil, err
	}
	if err = s.validateReviewReadAccess(ctx, tx, access, pointer); err != nil {
		return nil, nil, err
	}
	attempt, fixed, _, err := s.conflictOriginal(ctx, tx, fact)
	if err != nil {
		return nil, nil, err
	}
	if _, err = ensureConflictVersions(ctx, tx, fact, fixed); err != nil {
		return nil, nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, nil, err
	}
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		if err := s.validateReviewReadAccess(ctx, tx, access, pointer); err != nil {
			return nil, err
		}
		original, fixed, expires, err := s.conflictOriginal(ctx, tx, fact)
		if err != nil {
			return nil, err
		}
		if original.ID != attempt.ID {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "CONFLICT_ATTEMPT_CHANGED")
		}
		return readConflictView(ctx, tx, fact, original, fixed, expires)
	}
	return access.Sources, run, nil
}

func (s *Reviews) conflictOriginal(ctx context.Context, tx *sql.Tx, fact domain.MergeFact) (repository.ConflictAttempt, mergeFixedInputs, time.Time, error) {
	var empty mergeFixedInputs
	receipt, err := repository.MergeResult(ctx, tx, fact.Operation)
	if err != nil {
		return repository.ConflictAttempt{}, empty, time.Time{}, err
	}
	if receipt.State != "REJECTED" || !receipt.Expires.After(time.Now()) {
		return repository.ConflictAttempt{}, empty, time.Time{}, domain.Violation(domain.EntityNotFound, "CONFLICT_CONTEXT_NOT_FOUND")
	}
	attempt, err := repository.ConflictAttemptRow(ctx, tx, fact.Operation)
	if err != nil {
		return attempt, empty, time.Time{}, err
	}
	if attempt.State != "CONFLICT" || len(attempt.Conflicts) == 0 {
		return attempt, empty, time.Time{}, domain.Violation(domain.EntityNotFound, "CONFLICT_CONTEXT_NOT_FOUND")
	}
	fixed, err := readConflictFixed(ctx, tx, s.validator, fact, attempt)
	if err == nil && !bytes.Equal(receipt.Fingerprint, fixed.Input.Context.RequestFingerprint) {
		err = domain.Violation(domain.TemporarilyUnavailable, "CONFLICT_INPUTS_UNKNOWN")
	}
	return attempt, fixed, receipt.Expires, err
}

func readConflictView(ctx context.Context, tx *sql.Tx, fact domain.MergeFact, attempt repository.ConflictAttempt, fixed mergeFixedInputs, expires time.Time) (dto.ConflictContext, error) {
	// 公开契约使用不透明操作引用；命名空间加完整uint64定宽编码与真实attempt一一对应。
	// 私有固定输入仍使用原数据库主键，不生成新尝试，也不把公开引用写回旧固定事实。
	attemptRef := domain.ConflictAttemptRef(attempt.ID)
	value := dto.ConflictContext{
		ConflictRef: fact.Operation,
		OperationId: fact.Operation,
		AttemptId:   attemptRef,
		DocumentId:  domain.TextID(fact.Document),
		UnitId:      domain.TextID(fact.Unit),
		RequestId:   domain.TextID(fact.Request),
		RoundId:     domain.TextID(fact.Round),
		InputDigest: hex.EncodeToString(attempt.Digest),
		ExpiresAt:   expires.UTC().Format(time.RFC3339Nano),
		Conflicts:   []dto.Conflict{},
	}
	raw, err := repository.MergeArtifact(ctx, tx, "CONFLICT_CONTEXT_VERSIONS", attemptKey(fact.Operation, attempt.ID))
	if err != nil {
		return value, err
	}
	versions, err := validateConflictVersions(ctx, tx, raw, fixed)
	if err != nil {
		return value, err
	}
	if value.Base, err = repository.ContentRow(ctx, tx, domain.TextID(versions.Base), value.DocumentId); err != nil {
		return value, err
	}
	if value.Submitted, err = repository.ContentRow(ctx, tx, domain.TextID(versions.Submitted), value.DocumentId); err != nil {
		return value, err
	}
	if value.Main, err = repository.ContentRow(ctx, tx, domain.TextID(versions.Main), value.DocumentId); err != nil {
		return value, err
	}
	if value.Structure, err = repository.StructureRow(ctx, tx, domain.TextID(fixed.Attempt.Main.ExpectedStructureId), value.DocumentId); err != nil {
		return value, err
	}
	if value.Style, err = repository.StyleRow(ctx, tx, domain.TextID(fixed.Attempt.Main.ExpectedStyleId), value.DocumentId); err != nil {
		return value, err
	}
	if err = json.Unmarshal(attempt.Conflicts, &value.Conflicts); err != nil || len(value.Conflicts) == 0 {
		return value, domain.Violation(domain.TemporarilyUnavailable, "CONFLICT_LIST_UNKNOWN")
	}
	// 私有计算结果按固定数组保存，尚无公共冲突引用。用真实attempt与原数组序号建立稳定引用；
	// 后续修复必须在同一attempt/digest下核对整组引用，不能跨尝试沿用序号或改变旧计算事实。
	for index := range value.Conflicts {
		value.Conflicts[index].Id = domain.ConflictItemRef(attempt.ID, index+1)
	}
	return value, nil
}

// conflict_fixed.go：保留原实现，按业务合并文件。

func readConflictFixed(ctx context.Context, tx *sql.Tx, validator *codec.Validator, fact domain.MergeFact, attempt repository.ConflictAttempt) (mergeFixedInputs, error) {
	var fixed mergeFixedInputs
	unknown := domain.Violation(domain.TemporarilyUnavailable, "CONFLICT_INPUTS_UNKNOWN")
	key := attemptKey(fact.Operation, attempt.ID)
	raw, err := repository.MergeArtifact(ctx, tx, "MERGE_ATTEMPT_RANGE_FIXED", key)
	if err == sql.ErrNoRows {
		raw, err = repository.MergeArtifact(ctx, tx, "MERGE_ATTEMPT_FIXED", key)
	}
	if err != nil {
		return fixed, err
	}
	if json.Unmarshal(raw, &fixed) != nil || fixed.Input == nil || fixed.Input.Context == nil || fixed.Input.Context.Target == nil || fixed.Attempt.Main == nil || fixed.Main == nil || fixed.Main.Cut == nil {
		return fixed, unknown
	}
	input, target := fixed.Input, fixed.Input.Context.Target
	if fixed.Attempt.ID != attempt.ID || input.AttemptId != domain.TextID(attempt.ID) || input.Context.OperationId != fact.Operation || input.Context.ActorId != fact.Actor || target.DocumentId != fact.Document || target.UnitId != fact.Unit || target.BranchId != fact.Branch || target.Purpose != "MERGE" || !proto.Equal(input.MainCut, fixed.Main.Cut) {
		return fixed, unknown
	}
	digest, err := domain.MergeInputDigest(input)
	if err != nil || !bytes.Equal(digest[:], input.InputDigest) || !bytes.Equal(digest[:], attempt.Digest) {
		return fixed, unknown
	}
	round, err := repository.MergeRoundInputs(ctx, tx, fact.Request, fact.Round)
	if err != nil || !reflect.DeepEqual(round, fixed.Attempt.Round) || round.SubmittedVersion != fact.Version {
		return fixed, unknown
	}
	if err = validateMainCaptureDigest(fixed.Main); err != nil {
		return fixed, err
	}
	matched := false
	for _, main := range fixed.Main.Units {
		if main != nil && main.UnitId == fact.Unit && proto.Equal(main, input.Main) {
			matched = true
		}
	}
	if !matched {
		return fixed, unknown
	}
	for _, content := range []*pb.FixedContent{input.Base, input.Submitted, input.Main} {
		if err = validateFixedMergeContent(validator, content, fact.Document, fact.Unit, target.StructureId); err != nil {
			return fixed, err
		}
	}
	return fixed, nil
}

// conflict_version_binding.go：保留原实现，按业务合并文件。

// 恢复记录只是索引，真实不可变版本才是正文来源。不能仅凭两个可写记录中的摘要相同放行。
// 调用方传入当前事务；三份原型来自已经验证真实attempt/digest/轮次的固定计算输入。
func validateConflictVersions(ctx context.Context, q repository.Queryer, raw []byte, fixed mergeFixedInputs) (conflictVersions, error) {
	var value conflictVersions
	unknown := domain.Violation(domain.TemporarilyUnavailable, "CONFLICT_VERSIONS_UNKNOWN")
	if json.Unmarshal(raw, &value) != nil || value.Attempt != fixed.Attempt.ID || !bytes.Equal(value.Digest, fixed.Input.InputDigest) || value.Base == 0 || value.Submitted == 0 || value.Main == 0 {
		return value, unknown
	}
	for _, part := range []struct {
		version uint64
		fixed   *pb.FixedContent
	}{
		{value.Base, fixed.Input.Base},
		{value.Submitted, fixed.Input.Submitted},
		{value.Main, fixed.Input.Main},
	} {
		stored, err := repository.VersionContent(ctx, q, part.version, part.fixed.DocumentId)
		if err == sql.ErrNoRows {
			return value, unknown
		}
		if err != nil {
			return value, err
		}
		matched, err := conflictVersionMatches(stored, part.fixed, part.version)
		if err != nil || !matched {
			return value, unknown
		}
	}
	return value, nil
}

func conflictVersionMatches(stored, fixed *pb.FixedContent, version uint64) (bool, error) {
	if err := canonicalVersion(stored, version); err != nil {
		return false, err
	}
	// VERSION的引用切点与房间采样切点属于不同传输表示。只规范这三个字段，
	// 原正文/hash/字节数/schema/谱系/来源操作与引用/结构/完整资源集仍逐项精确比较。
	expected := proto.Clone(fixed).(*pb.FixedContent)
	expected.BranchId, expected.WorkspaceEpoch = 0, 0
	cut, err := codec.Canonical(map[string]any{"content_version_id": domain.TextID(version), "source_ref": fixed.CaptureId})
	if err != nil {
		return false, err
	}
	expected.CutJsonUtf8 = cut
	return proto.Equal(stored, expected), nil
}

// conflict_versions.go：保留原实现，按业务合并文件。

type conflictVersions struct {
	Attempt, Base, Submitted, Main uint64
	Digest                         []byte
}

// 把旧冲突的原固定事实幂等物化；不读取当前房间，不改主线、轮次或共同草稿。
// 调用方先持有原操作互斥及当前来源/文档锁，同一事务保存VERSION引用和版本绑定。
func ensureConflictVersions(ctx context.Context, tx *sql.Tx, fact domain.MergeFact, fixed mergeFixedInputs) (conflictVersions, error) {
	value := conflictVersions{Attempt: fixed.Attempt.ID, Digest: fixed.Input.InputDigest}
	key := attemptKey(fact.Operation, value.Attempt)
	unknown := domain.Violation(domain.TemporarilyUnavailable, "CONFLICT_VERSIONS_UNKNOWN")
	if raw, err := repository.MergeArtifact(ctx, tx, "CONFLICT_CONTEXT_VERSIONS", key); err == nil {
		return validateConflictVersions(ctx, tx, raw, fixed)
	} else if err != sql.ErrNoRows {
		return value, err
	}
	document, err := repository.DocumentRow(ctx, tx, fact.Document, true)
	if err != nil {
		return value, err
	}
	space, err := domain.CanonicalID(document.SpaceId)
	if err != nil {
		return value, err
	}
	set := map[uint64]bool{}
	for _, part := range []*pb.FixedContent{fixed.Input.Base, fixed.Input.Submitted, fixed.Input.Main} {
		for _, resource := range part.ResourceIds {
			set[resource] = true
		}
	}
	ids := make([]uint64, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		state, err := repository.LockedDocumentResource(ctx, tx, id, fact.Document, space)
		if err != nil {
			return value, err
		}
		if state != "READY" {
			return value, unknown
		}
	}
	value.Base, err = conflictSnapshotVersion(ctx, tx, fixed.Input.Base, fixed.Attempt.Round.BaseVersion, "RESTORE")
	if err != nil {
		return value, err
	}
	value.Submitted, err = conflictSnapshotVersion(ctx, tx, fixed.Input.Submitted, fixed.Attempt.Round.SubmittedVersion, "RESTORE")
	if err != nil {
		return value, err
	}
	value.Main, err = conflictSnapshotVersion(ctx, tx, fixed.Input.Main, 0, "MAIN_CAPTURE")
	if err != nil {
		return value, err
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return value, err
	}
	return value, repository.InsertSubmissionTask(ctx, tx, "CONFLICT_CONTEXT_VERSIONS", key, fact.Operation, raw, "SUCCEEDED")
}

func conflictSnapshotVersion(ctx context.Context, tx *sql.Tx, fixed *pb.FixedContent, original uint64, kind string) (uint64, error) {
	if original != 0 {
		stored, err := repository.VersionContent(ctx, tx, original, fixed.DocumentId)
		if err != nil {
			return 0, err
		}
		matched, err := conflictVersionMatches(stored, fixed, original)
		if err != nil {
			return 0, err
		}
		if matched {
			return original, nil
		}
	}
	// 稳定范围的结构投影或可靠MAIN尾部必须保存独立真实快照，不能把旧正式版本号贴到新正文上。
	return repository.InsertMergeVersion(ctx, tx, fixed, kind, fixed.CaptureId)
}
