# services/document/internal/repository/comment/comment_queries.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/139-comment-queries.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：集中SQL和行锁；使用用例传入的同一事务，不隐式提交。

主要声明：`Entry`、`scanComment`、`CommentRow`、`InsertComment`、`EditComment`、`DeleteComment`、`ListComments`、`DTO`、`ContextVersion`、`BranchScope`、`QueueMentions`、`Queryer`、`Thread`、`scanner`、`scanThread`、`ThreadRow`、`InsertThread`、`UpdateThreadState`、`ThreadFilter`、`ListThreads`、`readError`、`insertedID`、`revisionChanged`、`changed`、`pageLimit`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package comment

import (
	"bytes"
	dto "collaboration/contracts/httpdto"
	domain "collaboration/document/internal/domain/document"
	notices "collaboration/document/internal/repository/notification"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
)

// comments.go：保留原实现，按业务合并文件。

type Entry struct {
	ID, Thread, Author, Revision uint64
	Body                         string
	Deleted                      bool
}

const commentColumns = `id,thread_id,author_id,revision,IF(deleted_at IS NULL,body,''),deleted_at IS NOT NULL`

func scanComment(row scanner) (Entry, error) {
	var entry Entry
	if err := row.Scan(&entry.ID, &entry.Thread, &entry.Author, &entry.Revision, &entry.Body, &entry.Deleted); err != nil {
		return entry, readError(err)
	}
	if entry.ID == 0 || entry.Thread == 0 || entry.Author == 0 || entry.Revision == 0 {
		return entry, errors.New("COMMENT_ENTRY_INVALID")
	}
	return entry, nil
}

func CommentRow(ctx context.Context, q Queryer, id uint64, lock bool) (Entry, error) {
	query := "SELECT " + commentColumns + " FROM comment WHERE id=?"
	if lock {
		query += " FOR UPDATE"
	}
	return scanComment(q.QueryRowContext(ctx, query, id))
}

// 回复关系只能指向同线程已有评论，包括墓碑；删除作者正文不应破坏讨论结构。
// 调用方先锁线程，避免插入回复与线程删除各自只看见一半的状态。
func InsertComment(ctx context.Context, tx *sql.Tx, thread, actor uint64, body string, reply *uint64) (uint64, error) {
	if thread == 0 || actor == 0 {
		return 0, errors.New("COMMENT_ENTRY_INVALID")
	}
	if reply != nil {
		var id uint64
		err := tx.QueryRowContext(ctx, `SELECT id FROM comment WHERE id=? AND thread_id=? FOR SHARE`, *reply, thread).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) {
			return 0, domain.Violation(domain.EntityNotFound, "COMMENT_REPLY_NOT_FOUND")
		}
		if err != nil {
			return 0, readError(err)
		}
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO comment(thread_id,author_id,reply_to_id,body) VALUES(?,?,?,?)`, thread, actor, reply, body)
	if err != nil {
		return 0, errors.New("COMMENT_WRITE_FAILED")
	}
	return insertedID(ctx, tx)
}

func EditComment(ctx context.Context, tx *sql.Tx, entry Entry, body string) error {
	if entry.Deleted || entry.Revision == ^uint64(0) {
		return revisionChanged()
	}
	result, err := tx.ExecContext(ctx, `UPDATE comment SET body=?,revision=revision+1
		WHERE id=? AND thread_id=? AND revision=? AND deleted_at IS NULL AND revision<18446744073709551615`, body, entry.ID, entry.Thread, entry.Revision)
	return changed(result, err)
}

// 墓碑保留主键与 reply_to_id，正文清空；后续分页仍返回此位置，避免游标与回复断裂。
func DeleteComment(ctx context.Context, tx *sql.Tx, entry Entry) error {
	if entry.Deleted || entry.Revision == ^uint64(0) {
		return revisionChanged()
	}
	result, err := tx.ExecContext(ctx, `UPDATE comment SET body='',deleted_at=UTC_TIMESTAMP(6),revision=revision+1
		WHERE id=? AND thread_id=? AND revision=? AND deleted_at IS NULL AND revision<18446744073709551615`, entry.ID, entry.Thread, entry.Revision)
	return changed(result, err)
}

func ListComments(ctx context.Context, tx *sql.Tx, thread, after uint64, limit int) ([]Entry, error) {
	rows, err := tx.QueryContext(ctx, "SELECT "+commentColumns+" FROM comment WHERE thread_id=? AND id>? ORDER BY id LIMIT ?", thread, after, pageLimit(limit))
	if err != nil {
		return nil, readError(err)
	}
	defer rows.Close()
	items := []Entry{}
	for rows.Next() {
		entry, err := scanComment(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, entry)
	}
	return items, readError(rows.Err())
}

func (entry Entry) DTO() dto.Comment {
	body := entry.Body
	if entry.Deleted {
		body = ""
	}
	return dto.Comment{Id: domain.TextID(entry.ID), ThreadId: domain.TextID(entry.Thread), AuthorId: domain.TextID(entry.Author), Body: body, Revision: domain.TextID(entry.Revision)}
}

// context.go：保留原实现，按业务合并文件。

// ContextVersion 仅证明固定版本属于指定上下文，不代替当前或历史访问权限。
// BRANCH 只接受该分支真实谱系或送审记录，不能因版本属于同一单元就扩大可读范围。
func ContextVersion(ctx context.Context, tx *sql.Tx, document uint64, kind string, contextID, version uint64) (bool, error) {
	var query string
	switch kind {
	case "MAIN":
		query = `SELECT EXISTS (SELECT 1 FROM manifest_entry e
			JOIN content_version v ON v.id=e.content_version_id AND v.document_id=e.document_id AND v.unit_id=e.unit_id
			WHERE e.document_id=? AND e.manifest_id=? AND v.id=?)`
	case "REVIEW":
		query = `SELECT EXISTS (SELECT 1 FROM review_round r
			JOIN content_version v ON v.document_id=r.document_id AND v.unit_id=r.unit_id
				AND v.id IN (r.content_version_id,r.base_version_id)
			WHERE r.document_id=? AND r.id=? AND v.id=?)`
	case "BRANCH":
		// review_round 没有 branch_id，必须经 review_request 绑定真实分支，
		// 同时逐层核对文档与单元，避免历史分支间借用另一个送审版本。
		query = `SELECT EXISTS (SELECT 1 FROM branch b
			JOIN content_version v ON v.document_id=b.document_id AND v.unit_id=b.unit_id
			WHERE b.document_id=? AND b.id=? AND v.id=? AND (
				EXISTS (SELECT 1 FROM branch_lineage l WHERE l.branch_id=b.id
					AND l.document_id=b.document_id AND l.unit_id=b.unit_id AND l.base_version_id=v.id)
				OR EXISTS (SELECT 1 FROM review_request r JOIN review_round rr
					ON rr.request_id=r.id AND rr.document_id=r.document_id AND rr.unit_id=r.unit_id
					WHERE r.branch_id=b.id AND r.document_id=b.document_id AND r.unit_id=b.unit_id
					AND v.id IN (rr.content_version_id,rr.base_version_id))))`
	default:
		return false, errors.New("COMMENT_CONTEXT_INVALID")
	}
	var found bool
	err := tx.QueryRowContext(ctx, query, document, contextID, version).Scan(&found)
	return found, readError(err)
}

// BranchScope 保留历史分支身份，但单独返回是否仍为该单元当前活动分支。
// 创建新线程由业务层要求 active；历史线程读取不能误用此标志抹去已有事实。
func BranchScope(ctx context.Context, tx *sql.Tx, document, branch uint64) (unit uint64, active bool, err error) {
	err = tx.QueryRowContext(ctx, `SELECT b.unit_id,
		(b.lifecycle='ACTIVE' AND u.retired_structure_id IS NULL AND COALESCE(u.active_branch_id,0)=b.id)
		FROM branch b JOIN collaboration_unit u ON u.id=b.unit_id AND u.document_id=b.document_id
		WHERE b.document_id=? AND b.id=?`, document, branch).Scan(&unit, &active)
	return unit, active, readError(err)
}

// events.go：保留原实现，按业务合并文件。

// QueueMentions 与评论和通用操作回执同事务持久化通知意图，避免评论提交后
// 进程退出丢失提醒。这里只记录原固定目标；实际收件资格由 worker 重新核验。
// 原操作只在首次业务执行时插入，重复键不得覆盖已经绑定的正文目标或收件人。
func QueueMentions(ctx context.Context, tx *sql.Tx, operation string, thread Thread, comment, actor uint64, mentions []uint64) error {
	participants, err := notices.CommentRecipients(ctx, tx, thread.ID, actor)
	if err != nil {
		return err
	}
	// 无@的回复也需要提醒原参与者；是否有合格收件人由投递事务判断。
	payload := domain.CommentNotification{
		Document: thread.Document, Thread: thread.ID, Comment: comment,
		ContextKind: thread.Kind, ContextID: thread.Context, Version: thread.Version,
		Actor: actor, Recipients: mentions, ReplyRecipients: participants,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return errors.New("COMMENT_NOTIFICATION_PAYLOAD_INVALID")
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO durable_task(kind,business_key,operation_id,payload,state,retry_at)
		VALUES('COMMENT_NOTIFICATION',?,?,?,'QUEUED',UTC_TIMESTAMP(6))`, operation, operation, string(raw))
	if err != nil {
		return errors.New("COMMENT_NOTIFICATION_WRITE_FAILED")
	}
	return nil
}

// threads.go：保留原实现，按业务合并文件。

type Queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type Thread struct {
	ID, Document, Author, Revision, Version uint64
	Kind, Context, Status                   string
	Anchor                                  dto.Anchor
}

type scanner interface{ Scan(...any) error }

const threadColumns = `id,document_id,author_id,revision,COALESCE(content_version_id,0),context_kind,context_ref,status,anchor_json,anchor_schema`

// 锚点正文与关系列必须共同指向同一固定版本，损坏元数据不能降级成当前正文锚点。
func scanThread(row scanner) (Thread, error) {
	var t Thread
	var raw []byte
	var schema string
	if err := row.Scan(&t.ID, &t.Document, &t.Author, &t.Revision, &t.Version, &t.Kind, &t.Context, &t.Status, &raw, &schema); err != nil {
		return t, readError(err)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&t.Anchor); err != nil {
		return t, errors.New("COMMENT_ANCHOR_INVALID")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return t, errors.New("COMMENT_ANCHOR_INVALID")
	}
	version, err := domain.CanonicalID(t.Anchor.ContentVersionId)
	_, contextErr := domain.CanonicalID(t.Context)
	if err != nil || contextErr != nil || version != t.Version || schema == "" || schema != t.Anchor.SchemaVersion || t.ID == 0 || t.Document == 0 || t.Author == 0 || t.Revision == 0 {
		return t, errors.New("COMMENT_THREAD_INVALID")
	}
	if (t.Kind != "MAIN" && t.Kind != "BRANCH" && t.Kind != "REVIEW") || (t.Status != "OPEN" && t.Status != "RESOLVED" && t.Status != "DELETED") {
		return t, errors.New("COMMENT_THREAD_INVALID")
	}
	return t, nil
}

func ThreadRow(ctx context.Context, q Queryer, id uint64, lock bool) (Thread, error) {
	query := "SELECT " + threadColumns + " FROM comment_thread WHERE id=?"
	if lock {
		query += " FOR UPDATE"
	}
	return scanThread(q.QueryRowContext(ctx, query, id))
}

// InsertThread 与首条评论、操作回执共享业务事务；此处不提交，避免空线程可见。
func InsertThread(ctx context.Context, tx *sql.Tx, document, actor uint64, kind, contextRef string, anchor dto.Anchor) (uint64, error) {
	version, err := domain.CanonicalID(anchor.ContentVersionId)
	_, contextErr := domain.CanonicalID(contextRef)
	if err != nil || contextErr != nil || document == 0 || actor == 0 || anchor.SchemaVersion == "" || (kind != "MAIN" && kind != "BRANCH" && kind != "REVIEW") {
		return 0, errors.New("COMMENT_THREAD_INVALID")
	}
	raw, err := json.Marshal(anchor)
	if err != nil {
		return 0, errors.New("COMMENT_ANCHOR_INVALID")
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO comment_thread(document_id,context_kind,context_ref,content_version_id,anchor_json,anchor_schema,author_id,status)
		VALUES(?,?,?,?,?,?,?,'OPEN')`, document, kind, contextRef, version, string(raw), anchor.SchemaVersion, actor)
	if err != nil {
		return 0, errors.New("COMMENT_THREAD_WRITE_FAILED")
	}
	return insertedID(ctx, tx)
}

// CAS 与文档/线程锁配合，旧修订号不能覆盖另一条已完成的解决或重开动作。
func UpdateThreadState(ctx context.Context, tx *sql.Tx, t Thread, state string) error {
	if t.Revision == ^uint64(0) || t.Status == "DELETED" {
		return revisionChanged()
	}
	if state != "OPEN" && state != "RESOLVED" && state != "DELETED" {
		return errors.New("COMMENT_THREAD_STATE_INVALID")
	}
	result, err := tx.ExecContext(ctx, `UPDATE comment_thread SET status=?,revision=revision+1
		WHERE id=? AND revision=? AND status<>'DELETED' AND revision<18446744073709551615`, state, t.ID, t.Revision)
	return changed(result, err)
}

type ThreadFilter struct{ Kind, Context, Version string }

func ListThreads(ctx context.Context, tx *sql.Tx, document, after uint64, limit int, filters ...ThreadFilter) ([]Thread, error) {
	statement := "SELECT " + threadColumns + " FROM comment_thread WHERE document_id=? AND id>? AND status<>'DELETED'"
	args := []any{document, after}
	if len(filters) > 0 {
		f := filters[0]
		for _, c := range []struct{ column, value string }{{"context_kind", f.Kind}, {"context_ref", f.Context}, {"content_version_id", f.Version}} {
			if c.value != "" {
				statement += " AND " + c.column + "=?"
				args = append(args, c.value)
			}
		}
	}
	statement += " ORDER BY id LIMIT ?"
	args = append(args, pageLimit(limit))
	rows, err := tx.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, readError(err)
	}
	defer rows.Close()
	items := []Thread{}
	for rows.Next() {
		item, err := scanThread(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, readError(rows.Err())
}

func (t Thread) DTO() dto.CommentThread {
	return dto.CommentThread{Id: domain.TextID(t.ID), DocumentId: domain.TextID(t.Document), ContextKind: t.Kind, ContextId: t.Context, Anchor: t.Anchor, AuthorId: domain.TextID(t.Author), Status: t.Status, Revision: domain.TextID(t.Revision)}
}

func readError(err error) error {
	if err == nil || err == sql.ErrNoRows {
		return err
	}
	return errors.New("COMMENT_READ_FAILED")
}

func insertedID(ctx context.Context, tx *sql.Tx) (uint64, error) {
	var id uint64
	if tx.QueryRowContext(ctx, "SELECT LAST_INSERT_ID()").Scan(&id) != nil || id == 0 {
		return 0, errors.New("COMMENT_WRITE_FAILED")
	}
	return id, nil
}

func revisionChanged() error {
	return domain.Violation(domain.StateConflict, "COMMENT_REVISION_CHANGED")
}

func changed(result sql.Result, err error) error {
	if err != nil {
		return errors.New("COMMENT_WRITE_FAILED")
	}
	n, err := result.RowsAffected()
	if err != nil {
		return errors.New("COMMENT_WRITE_FAILED")
	}
	if n != 1 {
		return revisionChanged()
	}
	return nil
}

func pageLimit(limit int) int {
	if limit < 1 {
		return 1
	}
	// 允许业务层多取一项判定是否存在下一页。
	if limit > 101 {
		return 101
	}
	return limit
}
````

UTF-8/LF全文SHA-256：`55516ba16fb972eb50b0d4773e4abab629a57bd97d9b4caaf106fb89819de44c`。[参考实现](../../../../services/document/internal/repository/comment/comment_queries.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
