# services/document/internal/logic/document/comments_service.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/196-comments-service.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：编排用例、权限、事务及未知结果恢复；跨服务调用不能替代持久事实。

主要声明：`commentMentions`、`Create`、`CommentCommand`、`Change`、`commentPageBinding`、`ReadThread`、`List`、`ReadComments`、`Comments`、`NewComments`、`LookupThread`、`LookupComment`、`authorize`、`validateContext`、`commentReceipt`、`commentContextInput`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	codec "collaboration/contracts/content"
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	comments "collaboration/document/internal/repository/comment"
	documents "collaboration/document/internal/repository/document"
	resources "collaboration/document/internal/repository/resource"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
)

// comment_mutations.go：保留原实现，按业务合并文件。

func commentMentions(values []string) ([]uint64, error) {
	if len(values) > 100 {
		return nil, domain.Violation(domain.InvalidRequest, "COMMENT_MENTIONS_INVALID")
	}
	seen := map[uint64]bool{}
	ids := make([]uint64, 0, len(values))
	for _, value := range values {
		id, err := domain.CanonicalID(value)
		if err != nil {
			return nil, err
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids, nil
}

func (s *Comments) Create(ctx context.Context, actor Actor, document uint64, input dto.CommentThreadCreate) (Actor, MutationBody, MutationPresentation, error) {
	t, err := commentContextInput(document, input)
	if err != nil {
		return actor, nil, nil, err
	}
	if err = domain.ValidateCommentBody(input.Body); err != nil {
		return actor, nil, nil, err
	}
	mentions, err := commentMentions(input.Mentions)
	if err != nil {
		return actor, nil, nil, err
	}
	actor, a, err := s.authorize(ctx, actor, document, true)
	if err != nil {
		return actor, nil, nil, err
	}
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		if err := s.validateContext(ctx, tx, actor, a, t, true); err != nil {
			return nil, err
		}
		id, err := comments.InsertThread(ctx, tx, document, actor.ID, t.Kind, t.Context, t.Anchor)
		if err != nil {
			return nil, err
		}
		comment, err := comments.InsertComment(ctx, tx, id, actor.ID, input.Body, nil)
		if err != nil {
			return nil, err
		}
		target := t
		target.ID = id
		if err := comments.QueueMentions(ctx, tx, input.OperationId, target, comment, actor.ID, mentions); err != nil {
			return nil, err
		}
		return OperationResult(input.OperationId, "SUCCEEDED", id), nil
	}
	present := func(ctx context.Context, tx *sql.Tx, value map[string]any) (any, []*pb.SourceEpoch, error) {
		id, err := commentReceipt(value)
		if err != nil {
			return nil, nil, err
		}
		current, err := comments.ThreadRow(ctx, tx, id, false)
		if err != nil {
			return nil, nil, err
		}
		if current.Document != document || current.Author != actor.ID {
			return nil, nil, domain.Violation(domain.StateConflict, "COMMENT_RECEIPT_MISMATCH")
		}
		// 已删除的线程仍保留原创建成功事实；当前资格须复核，但不恢复线程。
		current.Status = "OPEN"
		if err := s.validateContext(ctx, tx, actor, a, current, false); err != nil {
			return nil, nil, err
		}
		return value, a.Sources, nil
	}
	return actor, run, present, nil
}

type CommentCommand struct {
	Operation, Kind, Body, State string
	Comment, Revision            uint64
	Reply                        *uint64
	Mentions                     []string
}

// 所有写路径都先锁来源和文档，再锁线程与评论；普通讨论不触碰审核阻止意见表。
func (s *Comments) Change(ctx context.Context, actor Actor, original comments.Thread, command CommentCommand) (Actor, MutationBody, MutationPresentation, error) {
	if command.Kind != "REPLY" && command.Kind != "EDIT" && command.Kind != "DELETE" && command.Kind != "STATE" {
		return actor, nil, nil, domain.Violation(domain.InvalidRequest, "COMMENT_ACTION_INVALID")
	}
	if command.Kind == "REPLY" || command.Kind == "EDIT" {
		if err := domain.ValidateCommentBody(command.Body); err != nil {
			return actor, nil, nil, err
		}
	}
	if command.Kind == "STATE" && command.State != "OPEN" && command.State != "RESOLVED" && command.State != "DELETED" {
		return actor, nil, nil, domain.Violation(domain.InvalidRequest, "COMMENT_STATE_INVALID")
	}
	mentions, err := commentMentions(command.Mentions)
	if err != nil {
		return actor, nil, nil, err
	}
	// 新回复需要全文 COMMENT；已有内容的维护沿冻结契约核验作者/管理者及当前上下文可读资格。
	actor, a, err := s.authorize(ctx, actor, original.Document, command.Kind == "REPLY")
	if err != nil {
		return actor, nil, nil, err
	}
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		if _, err := ValidateAccess(ctx, tx, a); err != nil {
			return nil, err
		}
		t, err := comments.ThreadRow(ctx, tx, original.ID, true)
		if err != nil {
			return nil, err
		}
		if t.Document != original.Document {
			return nil, domain.Violation(domain.StateConflict, "COMMENT_CONTEXT_CHANGED")
		}
		if err := s.validateContext(ctx, tx, actor, a, t, false); err != nil {
			return nil, err
		}
		id := t.ID
		switch command.Kind {
		case "REPLY":
			id, err = comments.InsertComment(ctx, tx, t.ID, actor.ID, command.Body, command.Reply)
			if err == nil {
				err = comments.QueueMentions(ctx, tx, command.Operation, t, id, actor.ID, mentions)
			}
		case "STATE":
			if actor.ID != t.Author && !a.Permissions.Manage {
				return nil, domain.Violation(domain.PermissionDenied, "COMMENT_THREAD_AUTHOR_REQUIRED")
			}
			if t.Revision != command.Revision {
				return nil, domain.Violation(domain.StateConflict, "COMMENT_REVISION_CHANGED")
			}
			err = comments.UpdateThreadState(ctx, tx, t, command.State)
		default:
			e, readErr := comments.CommentRow(ctx, tx, command.Comment, true)
			if readErr != nil {
				return nil, readErr
			}
			if e.Thread != t.ID {
				return nil, domain.Violation(domain.PermissionDenied, "COMMENT_THREAD_MISMATCH")
			}
			if e.Author != actor.ID && !(command.Kind == "DELETE" && a.Permissions.Manage) {
				return nil, domain.Violation(domain.PermissionDenied, "COMMENT_AUTHOR_REQUIRED")
			}
			if e.Revision != command.Revision {
				return nil, domain.Violation(domain.StateConflict, "COMMENT_REVISION_CHANGED")
			}
			id = e.ID
			if command.Kind == "EDIT" {
				err = comments.EditComment(ctx, tx, e, command.Body)
			} else {
				err = comments.DeleteComment(ctx, tx, e)
			}
		}
		if err != nil {
			return nil, err
		}
		return OperationResult(command.Operation, "SUCCEEDED", id), nil
	}
	present := func(ctx context.Context, tx *sql.Tx, value map[string]any) (any, []*pb.SourceEpoch, error) {
		t, err := comments.ThreadRow(ctx, tx, original.ID, false)
		if err != nil {
			return nil, nil, err
		}
		t.Status = "OPEN"
		if err := s.validateContext(ctx, tx, actor, a, t, false); err != nil {
			return nil, nil, err
		}
		return value, a.Sources, nil
	}
	return actor, run, present, nil
}

// comment_readings.go：保留原实现，按业务合并文件。

func commentPageBinding(actor Actor, a DocumentAccess, target string) string {
	// 游标绑定实际来源代次及当前结构。撤权/重授予不能续用上次扫描位置。
	sources, _ := json.Marshal(SourcesWire(a.Sources))
	// 权限来源数量不能让生成的游标超出解析器上限；摘要仍绑定完整来源集合。
	digest := sha256.Sum256(sources)
	return "comments:" + domain.TextID(actor.ID) + ":" + hex.EncodeToString(actor.Proof.SessionRef) + ":" + target + ":" + a.Document.CurrentStructureId + ":" + hex.EncodeToString(digest[:])
}

// ReadThread 供通知按真实线程身份定位原文档；不返回评论正文，定位结果仍须
// 在统一输出租约事务内检查当前身份、原固定上下文与删除状态，不能成为权限旁路。
func (s *Comments) ReadThread(ctx context.Context, actor Actor, original comments.Thread) ([]*pb.SourceEpoch, ReadBody, error) {
	_, a, err := s.authorize(ctx, actor, original.Document, false)
	if err != nil {
		return nil, nil, err
	}
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		if _, err := ValidateAccess(ctx, tx, a); err != nil {
			return nil, err
		}
		thread, err := comments.ThreadRow(ctx, tx, original.ID, false)
		if err != nil {
			return nil, err
		}
		if thread.Document != original.Document {
			return nil, domain.Violation(domain.StateConflict, "COMMENT_CONTEXT_CHANGED")
		}
		if err := s.validateContext(ctx, tx, actor, a, thread, false); err != nil {
			return nil, err
		}
		return thread.DTO(), nil
	}
	return a.Sources, run, nil
}

func (s *Comments) List(ctx context.Context, actor Actor, document uint64, limitText, cursor string, filters ...comments.ThreadFilter) ([]*pb.SourceEpoch, ReadBody, error) {
	filter := comments.ThreadFilter{}
	if len(filters) > 0 {
		filter = filters[0]
	}
	if filter.Kind != "" && filter.Kind != "MAIN" && filter.Kind != "BRANCH" && filter.Kind != "REVIEW" {
		return nil, nil, domain.Violation(domain.InvalidRequest, "COMMENT_FILTER_INVALID")
	}
	if filter.Context != "" && filter.Kind == "" {
		return nil, nil, domain.Violation(domain.InvalidRequest, "COMMENT_FILTER_INVALID")
	}
	for _, id := range []string{filter.Context, filter.Version} {
		if id != "" {
			if _, err := domain.CanonicalID(id); err != nil {
				return nil, nil, err
			}
		}
	}
	_, a, err := s.authorize(ctx, actor, document, false)
	if err != nil {
		return nil, nil, err
	}
	encodedFilter, _ := json.Marshal(filter)
	binding := commentPageBinding(actor, a, "document:"+domain.TextID(document)+":"+string(encodedFilter))
	limit, last, err := s.pagination.Parse(limitText, cursor, binding)
	if err != nil {
		return nil, nil, err
	}
	var after uint64
	if last != "" {
		after, err = domain.CanonicalID(last)
		if err != nil {
			return nil, nil, err
		}
	}
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		if _, err := ValidateAccess(ctx, tx, a); err != nil {
			return nil, err
		}
		page := dto.CommentThreadPage{Items: []dto.CommentThread{}}
		scanned := after
		// 每页最多扫描1000条，所有返回项逐项检查资格。扫描预算用尽可以返回空页和续页游标，不能把隐藏项计数泄露给用户。
		for count := 0; count < 1000; {
			rows, err := comments.ListThreads(ctx, tx, document, scanned, 100, filter)
			if err != nil {
				return nil, err
			}
			for _, t := range rows {
				count++
				err := s.validateContext(ctx, tx, actor, a, t, false)
				if err != nil {
					var denied domain.RuleViolation
					if errors.As(err, &denied) && (denied.Kind == domain.PermissionDenied || denied.Kind == domain.EntityNotFound) {
						scanned = t.ID
						continue
					}
					return nil, err
				}
				if len(page.Items) == limit {
					page.HasMore = true
					page.NextCursor = s.pagination.Next(binding, domain.TextID(scanned))
					return page, nil
				}
				page.Items = append(page.Items, t.DTO())
				scanned = t.ID
			}
			if len(rows) < 100 {
				return page, nil
			}
		}
		page.HasMore = true
		page.NextCursor = s.pagination.Next(binding, domain.TextID(scanned))
		return page, nil
	}
	return a.Sources, run, nil
}

func (s *Comments) ReadComments(ctx context.Context, actor Actor, original comments.Thread, limitText, cursor string) ([]*pb.SourceEpoch, ReadBody, error) {
	_, a, err := s.authorize(ctx, actor, original.Document, false)
	if err != nil {
		return nil, nil, err
	}
	binding := commentPageBinding(actor, a, "thread:"+domain.TextID(original.ID))
	limit, last, err := s.pagination.Parse(limitText, cursor, binding)
	if err != nil {
		return nil, nil, err
	}
	var after uint64
	if last != "" {
		after, err = domain.CanonicalID(last)
		if err != nil {
			return nil, nil, err
		}
	}
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		// 与删除路径保持文档在前的锁顺序，避免先建立旧快照后等待删除提交。
		if _, err := ValidateAccess(ctx, tx, a); err != nil {
			return nil, err
		}
		t, err := comments.ThreadRow(ctx, tx, original.ID, false)
		if err != nil {
			return nil, err
		}
		if t.Document != original.Document {
			return nil, domain.Violation(domain.StateConflict, "COMMENT_CONTEXT_CHANGED")
		}
		if err := s.validateContext(ctx, tx, actor, a, t, false); err != nil {
			return nil, err
		}
		rows, err := comments.ListComments(ctx, tx, t.ID, after, limit+1)
		if err != nil {
			return nil, err
		}
		page := dto.CommentPage{Items: []dto.Comment{}, HasMore: len(rows) > limit}
		if page.HasMore {
			rows = rows[:limit]
			page.NextCursor = s.pagination.Next(binding, domain.TextID(rows[len(rows)-1].ID))
		}
		for _, row := range rows {
			page.Items = append(page.Items, row.DTO())
		}
		return page, nil
	}
	return a.Sources, run, nil
}

// comments.go：保留原实现，按业务合并文件。

type Comments struct {
	db         *sql.DB
	access     *Access
	reviews    *Reviews
	pagination *Pagination
	validator  *codec.Validator
}

func NewComments(db *sql.DB, access *Access, reviews *Reviews, pagination *Pagination) *Comments {
	return &Comments{db: db, access: access, reviews: reviews, pagination: pagination, validator: reviews.validator}
}

func (s *Comments) LookupThread(ctx context.Context, id uint64) (comments.Thread, error) {
	t, err := comments.ThreadRow(ctx, s.db, id, false)
	if err == sql.ErrNoRows {
		err = domain.Violation(domain.EntityNotFound, "COMMENT_THREAD_NOT_FOUND")
	}
	return t, err
}

func (s *Comments) LookupComment(ctx context.Context, id uint64) (comments.Entry, comments.Thread, error) {
	e, err := comments.CommentRow(ctx, s.db, id, false)
	if err == sql.ErrNoRows {
		err = domain.Violation(domain.EntityNotFound, "COMMENT_NOT_FOUND")
	}
	if err != nil {
		return e, comments.Thread{}, err
	}
	t, err := s.LookupThread(ctx, e.Thread)
	return e, t, err
}

func (s *Comments) authorize(ctx context.Context, actor Actor, document uint64, write bool) (Actor, DocumentAccess, error) {
	a, err := s.access.Authorize(ctx, actor, document, "COMMENT_CONTEXT")
	if err == nil && write && !a.Permissions.Comment {
		err = domain.Violation(domain.PermissionDenied, "COMMENT_REQUIRED")
	}
	actor.Sources = a.Sources
	return actor, a, err
}

// 每次读取/修改都重新核验固定上下文。创建时才检查短期阅读凭据；持久讨论
// 后续依据原版本与当前权限读取，不能随着阅读视图到期丢掉已保存的讨论。
func (s *Comments) validateContext(ctx context.Context, tx *sql.Tx, actor Actor, a DocumentAccess, t comments.Thread, creating bool) error {
	doc, err := ValidateAccess(ctx, tx, a)
	if err != nil {
		return err
	}
	if doc.Lifecycle != "ACTIVE" {
		return domain.Violation(domain.StateConflict, "DOCUMENT_NOT_ACTIVE")
	}
	if t.Status == "DELETED" {
		return domain.Violation(domain.EntityNotFound, "COMMENT_THREAD_NOT_FOUND")
	}
	id, err := domain.CanonicalID(t.Context)
	if err != nil {
		return err
	}
	bound, err := comments.ContextVersion(ctx, tx, t.Document, t.Kind, id, t.Version)
	if err != nil {
		return err
	}
	if !bound {
		return domain.Violation(domain.PermissionDenied, "COMMENT_CONTEXT_MISMATCH")
	}
	scope, err := documents.ContentVersionScope(ctx, tx, t.Version)
	if err != nil {
		return err
	}
	switch t.Kind {
	case "MAIN":
		if !a.Permissions.Comment && !a.Permissions.Manage && !a.Permissions.Edit[scope.Unit] {
			covered, err := historicalContentRangeCovered(ctx, tx, a, t.Document, scope.Unit, s.validator)
			if err != nil {
				return err
			}
			if !covered {
				return domain.Violation(domain.PermissionDenied, "COMMENT_CONTEXT_NOT_ALLOWED")
			}
		}
		if creating && !(doc.MainLocked && doc.CurrentManifestId == t.Context) {
			valid, err := resources.MainManifestAccessible(ctx, tx, t.Document, id, actor.ID, actor.Proof.SessionRef)
			if err != nil {
				return err
			}
			if !valid {
				return domain.Violation(domain.StateConflict, "READ_VIEW_NOT_CURRENT")
			}
		}
	case "BRANCH":
		unit, active, err := comments.BranchScope(ctx, tx, t.Document, id)
		if err != nil {
			return err
		}
		if creating && !active {
			return domain.Violation(domain.StateConflict, "COMMENT_BRANCH_NOT_CURRENT")
		}
		allowed := a.Permissions.Comment || a.Permissions.Manage || a.Permissions.Edit[unit]
		if !allowed {
			allowed, err = historicalContentRangeCovered(ctx, tx, a, t.Document, unit, s.validator)
			if err != nil {
				return err
			}
		}
		if !allowed {
			return domain.Violation(domain.PermissionDenied, "COMMENT_CONTEXT_NOT_ALLOWED")
		}
	case "REVIEW":
		request, _, _, err := resources.ReviewContext(ctx, tx, t.Document, id)
		if err != nil {
			return err
		}
		pointer, err := documents.ReviewPointerRow(ctx, tx, request)
		if err != nil {
			return err
		}
		if err := s.reviews.validateReviewReadAccess(ctx, tx, a, pointer); err != nil {
			return err
		}
	default:
		return domain.Violation(domain.InvalidRequest, "COMMENT_CONTEXT_INVALID")
	}
	if scope.Document != t.Document {
		return domain.Violation(domain.PermissionDenied, "COMMENT_CONTEXT_MISMATCH")
	}
	if creating {
		fixed, err := documents.VersionContent(ctx, tx, t.Version, t.Document)
		if err != nil {
			return err
		}
		return domain.ValidateCommentAnchor(t.Anchor, fixed.BodyJsonUtf8)
	}
	return nil
}

func commentReceipt(value map[string]any) (uint64, error) {
	// 首次结果和持久回执反序列化后的 map 值类型不同，但都只读取原实体编号。
	var raw string
	switch result := value["result"].(type) {
	case map[string]string:
		raw = result["entity_id"]
	case map[string]any:
		raw, _ = result["entity_id"].(string)
	}
	return domain.CanonicalID(raw)
}

func commentContextInput(document uint64, input dto.CommentThreadCreate) (comments.Thread, error) {
	version, err := domain.CanonicalID(input.Anchor.ContentVersionId)
	if err != nil {
		return comments.Thread{}, err
	}
	if _, err := domain.CanonicalID(input.ContextId); err != nil {
		return comments.Thread{}, err
	}
	if input.ContextKind != "MAIN" && input.ContextKind != "BRANCH" && input.ContextKind != "REVIEW" {
		return comments.Thread{}, domain.Violation(domain.InvalidRequest, "COMMENT_CONTEXT_INVALID")
	}
	return comments.Thread{Document: document, Kind: input.ContextKind, Context: input.ContextId, Version: version, Anchor: input.Anchor, Status: "OPEN"}, nil
}
````

UTF-8/LF全文SHA-256：`c0293f68af10fe2db77b581d9a353be9ef7cc132ce0b769a87e0b293be45f39f`。[参考实现](../../../../services/document/internal/logic/document/comments_service.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
