// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	codec "collaboration/contracts/content"
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
)

// review_actions.go：保留原实现，按业务合并文件。

type reviewCommand struct {
	operation, kind                        string
	round, version, change, changeRevision uint64
	reason                                 *string
}

func (s *Reviews) RequestChanges(ctx context.Context, actor Actor, pointer domain.ReviewPointer, input dto.RequestChanges) (Actor, MutationBody, MutationPresentation, error) {
	round, err := domain.CanonicalID(input.RoundId)
	if err != nil {
		return actor, nil, nil, err
	}
	version, err := domain.CanonicalID(input.ContentVersionId)
	if err != nil {
		return actor, nil, nil, err
	}
	if input.Type != "REQUEST_CHANGES" || !domain.ValidName(input.Reason, 2000) {
		return actor, nil, nil, domain.Violation(domain.InvalidRequest, "REVIEW_CHANGES_INPUT_INVALID")
	}
	return s.action(ctx, actor, pointer, reviewCommand{operation: input.OperationId, kind: "REQUEST_CHANGES", round: round, version: version, reason: &input.Reason})
}

func (s *Reviews) Withdraw(ctx context.Context, actor Actor, pointer domain.ReviewPointer, input dto.Withdrawal) (Actor, MutationBody, MutationPresentation, error) {
	round, err := domain.CanonicalID(input.ExpectedRoundId)
	if err != nil {
		return actor, nil, nil, err
	}
	if input.Reason != nil && !domain.ValidName(*input.Reason, 2000) {
		return actor, nil, nil, domain.Violation(domain.InvalidRequest, "REVIEW_REASON_INVALID")
	}
	return s.action(ctx, actor, pointer, reviewCommand{operation: input.OperationId, kind: "WITHDRAW", round: round, reason: input.Reason})
}

func (s *Reviews) Resolve(ctx context.Context, actor Actor, pointer domain.ReviewPointer, change uint64, input dto.ResolveChanges) (Actor, MutationBody, MutationPresentation, error) {
	round, err := domain.CanonicalID(input.ExpectedRoundId)
	if err != nil {
		return actor, nil, nil, err
	}
	revision, err := domain.CanonicalID(input.ExpectedChangeRevision)
	if err != nil {
		return actor, nil, nil, err
	}
	if !domain.ValidName(input.Reason, 2000) {
		return actor, nil, nil, domain.Violation(domain.InvalidRequest, "REVIEW_REASON_INVALID")
	}
	return s.action(ctx, actor, pointer, reviewCommand{operation: input.OperationId, kind: "RESOLVE_CHANGES", round: round, change: change, changeRevision: revision, reason: &input.Reason})
}

func (s *Reviews) action(ctx context.Context, actor Actor, pointer domain.ReviewPointer, command reviewCommand) (Actor, MutationBody, MutationPresentation, error) {
	actor, access, err := s.authorized(ctx, actor, pointer, "REVIEW_ACTION")
	if err != nil {
		return actor, nil, nil, err
	}
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		if _, err := ValidateAccess(ctx, tx, access); err != nil {
			return nil, err
		}
		// 与送审、合并遵守同一顺序：来源→文档→单元 guard→请求；不持锁调用远端。
		guard, err := repository.LockedReviewGuard(ctx, tx, pointer.Unit)
		if err != nil {
			return nil, err
		}
		current, err := repository.LockedReviewPointer(ctx, tx, pointer.ID)
		if err != nil {
			return nil, err
		}
		if err = domain.ValidateReviewRound(current, command.round); err != nil {
			return nil, err
		}
		if command.kind == "WITHDRAW" {
			if !access.Permissions.Edit[current.Unit] {
				return nil, domain.Violation(domain.PermissionDenied, "REVIEW_WITHDRAW_EDIT_REQUIRED")
			}
			if guard.Ongoing != current.ID || guard.Submission != "" {
				return nil, domain.Violation(domain.StateConflict, "UNIT_REVIEW_POSITION_CHANGED")
			}
		} else if command.kind == "REQUEST_CHANGES" {
			if !access.Permissions.Review {
				return nil, domain.Violation(domain.PermissionDenied, "REVIEW_REQUIRED")
			}
			version, err := repository.ReviewRoundVersion(ctx, tx, current.ID, current.Round)
			if err != nil {
				return nil, err
			}
			if version != command.version {
				return nil, domain.Violation(domain.StateConflict, "REVIEW_CONTENT_CHANGED")
			}
		} else {
			change, err := repository.LockedChange(ctx, tx, current.ID, command.change)
			if err == sql.ErrNoRows {
				return nil, domain.Violation(domain.EntityNotFound, "CHANGE_REQUEST_NOT_FOUND")
			}
			if err != nil {
				return nil, err
			}
			if change.Status != "OPEN" || change.Revision != command.changeRevision {
				return nil, domain.Violation(domain.StateConflict, "CHANGE_REQUEST_CHANGED")
			}
			ever, err := repository.EverSubmitted(ctx, tx, current.ID, actor.ID)
			if err != nil {
				return nil, err
			}
			if !domain.CanResolveChange(access.Permissions, actor.ID, change, ever) {
				return nil, domain.Violation(domain.PermissionDenied, "CHANGE_RESOLUTION_NOT_ALLOWED")
			}
		}
		action, err := repository.InsertReviewAction(ctx, tx, command.operation, command.kind, current.ID, current.Round, actor.ID, command.version, command.change, command.reason)
		if err != nil {
			return nil, err
		}
		entity := action
		switch command.kind {
		case "REQUEST_CHANGES":
			entity, err = repository.InsertChange(ctx, tx, current.ID, current.Round, action, actor.ID)
		case "RESOLVE_CHANGES":
			entity = command.change
			err = repository.ResolveChange(ctx, tx, command.change, action)
		case "WITHDRAW":
			entity = current.ID
			err = repository.WithdrawReview(ctx, tx, current)
		}
		if err != nil {
			return nil, err
		}
		if command.kind != "WITHDRAW" {
			if err = repository.AdvanceReviewRevision(ctx, tx, current.ID); err != nil {
				return nil, err
			}
		}
		if err = repository.QueueReviewActionNotification(ctx, tx, command.operation, command.kind, current, action, actor.ID); err != nil {
			return nil, err
		}
		return map[string]any{"operation_id": command.operation, "state": "SUCCEEDED", "result": map[string]string{"entity_id": domain.TextID(entity), "request_id": domain.TextID(current.ID), "round_id": domain.TextID(current.Round)}}, nil
	}
	present := func(ctx context.Context, tx *sql.Tx, value map[string]any) (any, []*pb.SourceEpoch, error) {
		if _, err := ValidateAccess(ctx, tx, access); err != nil {
			return nil, nil, err
		}
		current, err := repository.ReviewPointerRow(ctx, tx, pointer.ID)
		if err != nil {
			return nil, nil, err
		}
		// 固定结果和当前状态分别展示；同号重试不重新执行动作，不返回永久正文访问凭证。
		response := map[string]any{}
		for key, item := range value {
			response[key] = item
		}
		response["current_state"] = map[string]string{"review_state": current.Status, "current_round_id": domain.TextID(current.Round), "revision": domain.TextID(current.Revision)}
		return response, access.Sources, nil
	}
	return actor, run, present, nil
}

// review_historical_access.go：保留原实现，按业务合并文件。

func historicalReviewPurpose(purpose string) bool {
	return purpose == "READ_REVIEW" || purpose == "READ_REVIEW_ROUND" || purpose == "READ_REVIEW_HISTORY" || purpose == "LIST_REVIEWS"
}

func (s *Reviews) reviewReadAllowed(ctx context.Context, access DocumentAccess, pointer domain.ReviewPointer) (bool, error) {
	if reviewContextAllowed(access, pointer) {
		return true, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	if err = LockSources(ctx, tx, access.Sources); err != nil {
		return false, err
	}
	if _, err = ValidateAccess(ctx, tx, access); err != nil {
		return false, err
	}
	allowed, err := s.historicalReviewCovered(ctx, tx, access, pointer)
	if err != nil {
		return false, err
	}
	return allowed, tx.Commit()
}

func (s *Reviews) validateReviewReadAccess(ctx context.Context, tx *sql.Tx, access DocumentAccess, pointer domain.ReviewPointer) error {
	if _, err := ValidateAccess(ctx, tx, access); err != nil {
		return err
	}
	if reviewContextAllowed(access, pointer) {
		return nil
	}
	allowed, err := s.historicalReviewCovered(ctx, tx, access, pointer)
	if err != nil {
		return err
	}
	if !allowed {
		return domain.Violation(domain.PermissionDenied, "REVIEW_CONTEXT_NOT_ALLOWED")
	}
	return nil
}

// 历史授权只服务只读用例，绝不把旧unit加入EDIT集合；旧请求的撤回/再提交资格仍由当前可编辑单元决定。
func (s *Reviews) historicalReviewCovered(ctx context.Context, tx *sql.Tx, access DocumentAccess, pointer domain.ReviewPointer) (bool, error) {
	return historicalContentRangeCovered(ctx, tx, access, pointer.Document, pointer.Unit, s.validator)
}

// 固定版本与审核历史共用同一原范围证明；抽取后不改变映射证据、预算或失败规则。
func historicalContentRangeCovered(ctx context.Context, tx *sql.Tx, access DocumentAccess, document, originalUnit uint64, validator *codec.Validator) (bool, error) {
	if access.Document.Id != domain.TextID(document) || len(access.Permissions.Edit) == 0 {
		return false, nil
	}
	if validator == nil {
		return false, domain.Violation(domain.TemporarilyUnavailable, "HISTORICAL_RANGE_VALIDATOR_UNAVAILABLE")
	}
	structure, err := domain.CanonicalID(access.Document.CurrentStructureId)
	if err != nil {
		return false, err
	}
	units, err := repository.CurrentStructurePreviewUnits(ctx, tx, document, structure)
	if err != nil {
		return false, err
	}
	current := map[uint64]bool{}
	for _, unit := range units {
		current[unit.Unit] = true
	}
	witnesses := map[string]historicalStructureWitness{}
	versions := map[uint64]*pb.FixedContent{}
	seen := map[uint64]bool{}
	queue := []uint64{originalUnit}
	edges := []domain.HistoricalRangeEdge{}
	bytesRead := 0
	limit := func() error { return domain.Violation(domain.TemporarilyUnavailable, "HISTORICAL_RANGE_LIMIT") }
	version := func(id uint64) (*pb.FixedContent, error) {
		if content := versions[id]; content != nil {
			return content, nil
		}
		content, err := repository.VersionContent(ctx, tx, id, document)
		if err != nil {
			return nil, err
		}
		bytesRead += len(content.BodyJsonUtf8)
		if bytesRead > 128<<20 {
			return nil, limit()
		}
		versions[id] = content
		return content, nil
	}
	for index := 0; index < len(queue); index++ {
		unit := queue[index]
		if seen[unit] || current[unit] {
			continue
		}
		seen[unit] = true
		if len(seen) > 100000 {
			return false, limit()
		}
		rows, err := repository.HistoricalOutgoingMappings(ctx, tx, document, unit)
		if err != nil {
			return false, err
		}
		if len(rows) == 0 {
			return false, nil
		}
		if len(rows) > 10000 || len(edges)+len(rows) > 100000 {
			return false, limit()
		}
		for _, row := range rows {
			witness, exists := witnesses[row.Operation]
			if !exists {
				witness, err = readHistoricalStructureWitness(ctx, tx, row.Operation, document, validator)
				if err != nil {
					var rule domain.RuleViolation
					if errors.As(err, &rule) && rule.Kind == domain.PermissionDenied {
						return false, nil
					}
					return false, err
				}
				bytesRead += witness.Bytes
				if bytesRead > 128<<20 {
					return false, limit()
				}
				witnesses[row.Operation] = witness
			}
			source, err := version(row.SourceVersion)
			if err != nil {
				if err == sql.ErrNoRows {
					return false, nil
				}
				return false, err
			}
			target, err := version(row.TargetVersion)
			if err != nil {
				if err == sql.ErrNoRows {
					return false, nil
				}
				return false, err
			}
			edge, ok := proveHistoricalMapping(row, source, target, witness)
			if !ok {
				return false, nil
			}
			edges = append(edges, edge)
			queue = append(queue, edge.Target)
		}
	}
	return domain.HistoricalRangeCovered(access.Permissions.Edit, originalUnit, current, edges), nil
}

// review_lists.go：保留原实现，按业务合并文件。

// 列表候选范围、分页推进与正文授权分开；游标只推进已实际检查的ID，不返回隐藏项总数。
func (s *Reviews) List(ctx context.Context, actor Actor, document uint64, limitText, cursorText string) ([]*pb.SourceEpoch, ReadBody, error) {
	binding := "review-list:" + domain.TextID(actor.ID) + ":" + domain.TextID(document)
	limit, last, err := s.pagination.Parse(limitText, cursorText, binding)
	if err != nil {
		return nil, nil, err
	}
	if last == "" {
		last = "0"
	}
	sources := append([]*pb.SourceEpoch{}, actor.Proof.Sources...)
	var documentAccess *DocumentAccess
	if document != 0 {
		access, err := s.access.Authorize(ctx, actor, document, "LIST_REVIEWS")
		if err != nil {
			return nil, nil, err
		}
		documentAccess = &access
		sources = access.Sources
	}
	var ids []uint64
	if document == 0 {
		organizations, scopeErr := s.organizationReviewScope(ctx, actor)
		if scopeErr != nil {
			return nil, nil, scopeErr
		}
		ids, err = repository.ScopedReviewRequestCandidates(ctx, s.db, actor.ID, organizations, last, limit*10+1)
	} else {
		ids, err = repository.ReviewRequestCandidates(ctx, s.db, document, last, limit*10+1)
	}
	if err != nil {
		return nil, nil, err
	}
	var selected []domain.ReviewPointer
	var accesses []DocumentAccess
	more := false
	scanned := last
	for index, id := range ids {
		if index >= limit*10 {
			more = true
			break
		}
		pointer, err := repository.ReviewPointerRow(ctx, s.db, id)
		if err != nil {
			return nil, nil, err
		}
		var access DocumentAccess
		if documentAccess != nil {
			access = *documentAccess
		} else {
			access, err = s.access.Authorize(ctx, actor, pointer.Document, "LIST_REVIEWS")
			if err != nil {
				var rule domain.RuleViolation
				if errors.As(err, &rule) && (rule.Kind == domain.PermissionDenied || rule.Kind == domain.EntityNotFound) {
					scanned = domain.TextID(id)
					continue
				}
				return nil, nil, err
			}
		}
		allowed := reviewContextAllowed(access, pointer)
		if !allowed && document != 0 {
			allowed, err = s.reviewReadAllowed(ctx, access, pointer)
			if err != nil {
				return nil, nil, err
			}
		}
		if !allowed || (document == 0 && !access.Permissions.Review) {
			scanned = domain.TextID(id)
			continue
		}
		if len(selected) == limit {
			more = true
			break
		}
		selected = append(selected, pointer)
		accesses = append(accesses, access)
		scanned = domain.TextID(id)
		for _, source := range access.Sources {
			found := false
			for _, known := range sources {
				if known.ScopeKey == source.ScopeKey {
					if known.Epoch != source.Epoch {
						return nil, nil, domain.Violation(domain.StateConflict, "REVIEW_LIST_SOURCE_CHANGED")
					}
					found = true
					break
				}
			}
			if !found {
				sources = append(sources, source)
			}
		}
	}
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		if documentAccess != nil {
			if _, err := ValidateAccess(ctx, tx, *documentAccess); err != nil {
				return nil, err
			}
		}
		page := dto.ReviewRequestPage{Items: []dto.ReviewRequest{}, HasMore: more}
		for i, pointer := range selected {
			if err := s.validateReviewReadAccess(ctx, tx, accesses[i], pointer); err != nil {
				return nil, err
			}
			value, err := s.requestView(ctx, tx, pointer, actor.ID, accesses[i], false)
			if err != nil {
				return nil, err
			}
			page.Items = append(page.Items, value)
		}
		if more {
			page.NextCursor = s.pagination.Next(binding, scanned)
		}
		return page, nil
	}
	return sources, run, nil
}

// review_readings.go：保留原实现，按业务合并文件。

func (s *Reviews) requestView(ctx context.Context, tx *sql.Tx, pointer domain.ReviewPointer, actor uint64, access DocumentAccess, detail bool) (dto.ReviewRequest, error) {
	value, err := repository.ReviewRequestRow(ctx, tx, pointer.ID)
	if err != nil {
		return value, err
	}
	round, err := domain.CanonicalID(value.CurrentRoundId)
	if err != nil {
		return value, err
	}
	summary, body, err := repository.ReviewRequestSummaryRow(ctx, tx, pointer.ID, round, pointer.Document)
	if err != nil {
		return value, err
	}
	summary.Excerpt, err = s.reviewSummaryExcerpt(body)
	if err != nil {
		return value, err
	}
	value.Summary = &summary
	if value.ArchivedAt == nil {
		if value.Status == "REVIEWING" {
			if access.Permissions.Edit[pointer.Unit] {
				value.Actions = append(value.Actions, "WITHDRAW")
			}
			if access.Permissions.Review {
				value.Actions = append(value.Actions, "REQUEST_CHANGES")
			}
			if access.Permissions.Review || access.Permissions.Manage {
				facts, err := repository.MergeApprovalFacts(ctx, tx, pointer.ID, actor)
				if err != nil {
					return value, err
				}
				if domain.CanOfferMerge(access.Permissions, facts) {
					value.Actions = append(value.Actions, "APPROVE_AND_MERGE")
				}
			}
		} else if value.Status == "WITHDRAWN" && access.Permissions.Edit[pointer.Unit] {
			value.Actions = append(value.Actions, "RESUBMIT")
		}
	}
	if detail {
		fixed, err := repository.ReviewRoundRow(ctx, tx, pointer.ID, round, access.Document.Id)
		if err != nil {
			return value, err
		}
		value.CurrentRound = &fixed
	}
	return value, nil
}

func (s *Reviews) reviewSummaryExcerpt(raw []byte) (string, error) {
	body, err := codec.Decode(raw)
	if err != nil {
		return "", err
	}
	if err = s.validator.Body(body); err != nil {
		return "", err
	}
	text, err := searchSnippet(body)
	if err != nil {
		return "", err
	}
	// 复用正文语义投影，再压平段落空白；按Unicode码点截断，避免切坏中文和emoji。
	characters := []rune(strings.Join(strings.Fields(text), " "))
	if len(characters) > 140 {
		return string(characters[:140]) + "…", nil
	}
	return string(characters), nil
}

func (s *Reviews) Request(ctx context.Context, actor Actor, pointer domain.ReviewPointer) ([]*pb.SourceEpoch, ReadBody, error) {
	_, access, err := s.authorized(ctx, actor, pointer, "READ_REVIEW")
	if err != nil {
		return nil, nil, err
	}
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		if err := s.validateReviewReadAccess(ctx, tx, access, pointer); err != nil {
			return nil, err
		}
		return s.requestView(ctx, tx, pointer, actor.ID, access, true)
	}
	return access.Sources, run, nil
}

func (s *Reviews) Round(ctx context.Context, actor Actor, pointer domain.ReviewPointer, round uint64) ([]*pb.SourceEpoch, ReadBody, error) {
	_, access, err := s.authorized(ctx, actor, pointer, "READ_REVIEW_ROUND")
	if err != nil {
		return nil, nil, err
	}
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		if err := s.validateReviewReadAccess(ctx, tx, access, pointer); err != nil {
			return nil, err
		}
		value, err := repository.ReviewRoundRow(ctx, tx, pointer.ID, round, access.Document.Id)
		if err == sql.ErrNoRows {
			return nil, domain.Violation(domain.EntityNotFound, "REVIEW_ROUND_NOT_FOUND")
		}
		return value, err
	}
	return access.Sources, run, nil
}

func reviewPageBinding(actor Actor, access DocumentAccess, pointer domain.ReviewPointer, kind string) string {
	// 游标固定当前权限来源及控制修订；跨用户、跨请求或中途新动作都会使旧页失效。
	data, _ := json.Marshal(access.Sources)
	return "review:" + kind + ":" + domain.TextID(actor.ID) + ":" + domain.TextID(pointer.ID) + ":" + domain.TextID(pointer.Revision) + ":" + string(data)
}

func (s *Reviews) History(ctx context.Context, actor Actor, pointer domain.ReviewPointer, kind, limitText, cursorText string) ([]*pb.SourceEpoch, ReadBody, error) {
	_, access, err := s.authorized(ctx, actor, pointer, "READ_REVIEW_HISTORY")
	if err != nil {
		return nil, nil, err
	}
	binding := reviewPageBinding(actor, access, pointer, kind)
	limit, last, err := s.pagination.Parse(limitText, cursorText, binding)
	if err != nil {
		return nil, nil, err
	}
	if last == "" {
		last = "0"
	}
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		if err := s.validateReviewReadAccess(ctx, tx, access, pointer); err != nil {
			return nil, err
		}
		current, err := repository.LockedReviewPointer(ctx, tx, pointer.ID)
		if err != nil {
			return nil, err
		}
		if current.Revision != pointer.Revision {
			return nil, domain.Violation(domain.StateConflict, "REVIEW_PAGE_CHANGED")
		}
		switch kind {
		case "rounds":
			ids, err := repository.ReviewRoundIDs(ctx, tx, pointer.ID, last, limit+1)
			if err != nil {
				return nil, err
			}
			page := dto.ReviewRoundPage{Items: []dto.ReviewRound{}}
			bytes := 0
			for _, id := range ids {
				if len(page.Items) == limit {
					page.HasMore = true
					break
				}
				fixed, err := repository.ReviewRoundRow(ctx, tx, pointer.ID, id, access.Document.Id)
				if err != nil {
					return nil, err
				}
				encoded, err := json.Marshal(fixed)
				if err != nil {
					return nil, err
				}
				// 正文上限按单轮规定；历史页还需总字节预算，避免合法的多份大轮次撑爆输出内存。
				if len(page.Items) > 0 && bytes+len(encoded) > 24<<20 {
					page.HasMore = true
					break
				}
				page.Items = append(page.Items, fixed)
				bytes += len(encoded)
			}
			if page.HasMore {
				page.NextCursor = s.pagination.Next(binding, page.Items[len(page.Items)-1].Id)
			}
			return page, nil
		case "changes":
			items, err := repository.ReviewChanges(ctx, tx, pointer.ID, last, limit+1)
			if err != nil {
				return nil, err
			}
			page := dto.ChangeRequestPage{Items: items, HasMore: len(items) > limit}
			if page.HasMore {
				page.Items = items[:limit]
				page.NextCursor = s.pagination.Next(binding, page.Items[limit-1].Id)
			}
			return page, nil
		case "actions":
			items, err := repository.ReviewActions(ctx, tx, pointer.ID, last, limit+1)
			if err != nil {
				return nil, err
			}
			page := dto.ReviewActionPage{Items: items, HasMore: len(items) > limit}
			if page.HasMore {
				page.Items = items[:limit]
				page.NextCursor = s.pagination.Next(binding, page.Items[limit-1].Id)
			}
			return page, nil
		default:
			return nil, domain.Violation(domain.InvalidRequest, "REVIEW_HISTORY_KIND_INVALID")
		}
	}
	return access.Sources, run, nil
}

// review_scope.go：保留原实现，按业务合并文件。

// 组织范围只从账号服务当前目录取得；不会把其他用户的组织当作逐篇身份RPC探测目标。
// 这是SQL候选优化，不授予正文访问；每个候选仍执行Authorize与输出事务内ValidateAccess。
func (s *Reviews) organizationReviewScope(ctx context.Context, actor Actor) ([]uint64, error) {
	if s.directory == nil {
		return nil, errors.New("REVIEW_DIRECTORY_UNAVAILABLE")
	}
	cookie := "__Host-collabSession=" + base64.RawURLEncoding.EncodeToString(actor.Token)
	var organizations []uint64
	cursor := ""
	for {
		var page dto.OrganizationPage
		if err := s.directory.Read(ctx, cookie, "/v1/organizations?limit=100&cursor="+url.QueryEscape(cursor), &page); err != nil {
			return nil, err
		}
		for _, organization := range page.Items {
			id, err := domain.CanonicalID(organization.Id)
			if err != nil {
				return nil, err
			}
			if organization.OwnerId != domain.TextID(actor.ID) {
				_, err = s.access.identity.Acquire(ctx, actor, pb.IdentityCapability_ORGANIZATION_ADMIN, &pb.IdentityTarget{Kind: pb.IdentityTarget_ORGANIZATION, OrganizationId: id, UserId: actor.ID}, "LIST_REVIEWS_SCOPE", 0, 0)
				if err != nil {
					var rule domain.RuleViolation
					if errors.As(err, &rule) && rule.Kind == domain.PermissionDenied {
						continue
					}
					return nil, err
				}
			}
			organizations = append(organizations, id)
		}
		if !page.HasMore {
			return organizations, nil
		}
		if page.NextCursor == nil || *page.NextCursor == "" || *page.NextCursor == cursor {
			return nil, errors.New("REVIEW_DIRECTORY_CURSOR_INVALID")
		}
		cursor = *page.NextCursor
	}
}

// reviews.go：保留原实现，按业务合并文件。

type Reviews struct {
	db         *sql.DB
	access     *Access
	pagination *Pagination
	directory  DirectoryReader
	validator  *codec.Validator
}

func NewReviews(db *sql.DB, access *Access, pagination *Pagination, directory DirectoryReader, validator *codec.Validator) *Reviews {
	return &Reviews{db: db, access: access, pagination: pagination, directory: directory, validator: validator}
}

func (s *Reviews) Lookup(ctx context.Context, id uint64) (domain.ReviewPointer, error) {
	pointer, err := repository.ReviewPointerRow(ctx, s.db, id)
	if err == sql.ErrNoRows {
		return pointer, domain.Violation(domain.EntityNotFound, "REVIEW_REQUEST_NOT_FOUND")
	}
	return pointer, err
}

func (s *Reviews) Ready(ctx context.Context) error {
	invalid, err := repository.UnboundReviewActionNotifications(ctx, s.db)
	if err != nil {
		return errors.New("REVIEW_NOTIFICATION_RECOVERY_UNAVAILABLE")
	}
	if invalid {
		return errors.New("REVIEW_NOTIFICATION_BINDING_UNKNOWN")
	}
	return nil
}

func reviewContextAllowed(access DocumentAccess, pointer domain.ReviewPointer) bool {
	return access.Permissions.Comment || access.Permissions.Review || access.Permissions.Manage || access.Permissions.Edit[pointer.Unit]
}

func (s *Reviews) authorized(ctx context.Context, actor Actor, pointer domain.ReviewPointer, purpose string) (Actor, DocumentAccess, error) {
	access, err := s.access.Authorize(ctx, actor, pointer.Document, purpose)
	if err != nil {
		return actor, access, err
	}
	allowed := reviewContextAllowed(access, pointer)
	if !allowed && historicalReviewPurpose(purpose) {
		allowed, err = s.reviewReadAllowed(ctx, access, pointer)
		if err != nil {
			return actor, access, err
		}
	}
	if !allowed {
		return actor, access, domain.Violation(domain.PermissionDenied, "REVIEW_CONTEXT_NOT_ALLOWED")
	}
	actor.Sources = access.Sources
	return actor, access, nil
}
