// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package resource

import (
	"bytes"
	domain "collaboration/document/internal/domain/document"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/go-sql-driver/mysql"
	"sort"
	"strconv"
	"strings"
	"time"
)

// allocated_quota.go：保留原实现，按业务合并文件。

// READY 与真实 COMMITTED 上传结算同事务建立原分配，不增加第二笔 used。
// 已迁出的 RELEASED 分配不能由旧上传重试重新激活。
func RecordCommittedAllocation(ctx context.Context, tx *sql.Tx, id uint64) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO resource_space_allocation(resource_id,space_id,bytes,state,operation_id)
 SELECT r.id,r.space_id,r.byte_size,'ACTIVE',r.upload_operation_id FROM resource r
 JOIN quota_reservation q ON q.operation_id=r.upload_operation_id AND q.space_id=r.space_id AND q.bytes=r.byte_size AND q.state='COMMITTED'
 WHERE r.id=? AND r.state IN ('READY','DELETING')
 ON DUPLICATE KEY UPDATE resource_id=VALUES(resource_id)`, id)
	return err
}

// 当前资源状态转换之前按空间编号锁全部真实分配；物理原空间也需保留上传原收据锁序。
// 后续资源锁下再次核集合，新增跨空间分配不能绕过尚未持有的 quota 锁。
func LockResourceAllocationQuotas(ctx context.Context, tx *sql.Tx, id, physical, limit uint64) (map[uint64]Quota, error) {
	ids, err := ResourceAllocationSpaces(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	ids = append(ids, physical)
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	result := map[uint64]Quota{}
	for _, space := range ids {
		if _, exists := result[space]; exists {
			continue
		}
		q, err := LockQuota(ctx, tx, space, limit)
		if err != nil {
			return nil, err
		}
		result[space] = q
	}
	return result, nil
}

func ValidateResourceAllocationQuotas(ctx context.Context, tx *sql.Tx, id uint64, quotas map[uint64]Quota) error {
	rows, err := tx.QueryContext(ctx, `SELECT space_id FROM resource_space_allocation WHERE resource_id=? AND state='ACTIVE' ORDER BY space_id FOR UPDATE`, id)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var space uint64
		if err = rows.Scan(&space); err != nil {
			return err
		}
		if _, exists := quotas[space]; !exists {
			return errors.New("RESOURCE_ALLOCATION_SET_CHANGED")
		}
	}
	return rows.Err()
}

// 清理资格可沿已接受的固定转让追溯，包含最终物理 DELETED 后的同号重试；
// 普通读取/新增保护另用 ResourceAllowedForDocument，仍须 ACTIVE 真实计费分配。
func ResourceTransferEvidence(ctx context.Context, tx *sql.Tx, id, document, space uint64) (bool, error) {
	var valid bool
	err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM document_resource_allocation a
 JOIN personal_transfer t ON t.id=a.transfer_id AND t.document_id=a.document_id AND t.state='ACCEPTED'
 WHERE a.resource_id=? AND a.document_id=? AND a.space_id=?)`, id, document, space).Scan(&valid)
	return valid, err
}

// allocation_cleanup.go：保留原实现，按业务合并文件。

// 调用方在真实 CLEANED 后持清理 Doc、Space/quota 与排序资源锁，且使用 READ
// COMMITTED。迁入/迁出在改 Doc.space 之前均锁双方 quota；持此空间 quota 就
// 固定了消费者经过此空间的范围。资源锁又阻止新增 pin，无需在 quota 后锁其他
// Doc。引用、保护与 Doc.space 使用 READ COMMITTED 的新语句快照；不能在资源
// 之后加外文档 protection 锁，否则与其保护确认的 protection→resource 锁序相反。
func ReleaseUnusedDocumentAllocations(ctx context.Context, tx *sql.Tx, document, space, limit uint64, items []domain.TransferResource) error {
	for _, item := range items {
		q, err := LockQuota(ctx, tx, space, limit)
		if err != nil {
			return err
		}
		var state string
		if err = tx.QueryRowContext(ctx, `SELECT state FROM resource WHERE id=? FOR UPDATE`, item.ID).Scan(&state); err != nil {
			return err
		}
		consumers, err := TransferResourceConsumerDocuments(ctx, tx, []domain.TransferResource{item}, false)
		if err != nil {
			return err
		}
		var unknown string
		err = tx.QueryRowContext(ctx, `SELECT operation_id FROM resource_protection WHERE state='UNKNOWN' AND JSON_CONTAINS(resource_ids,CAST(? AS JSON)) LIMIT 1`, domain.TextID(item.ID)).Scan(&unknown)
		if err == nil {
			return domain.Violation(domain.StateConflict, "RESOURCE_ALLOCATION_PROTECTION_UNKNOWN")
		}
		if err != sql.ErrNoRows {
			return err
		}
		own, scoped := false, false
		for _, consumer := range consumers {
			var currentSpace uint64
			if err = tx.QueryRowContext(ctx, `SELECT space_id FROM document WHERE id=?`, consumer).Scan(&currentSpace); err != nil {
				return err
			}
			own = own || consumer == document
			scoped = scoped || currentSpace == space
		}
		if own {
			continue
		}
		if _, err = tx.ExecContext(ctx, `UPDATE document_resource_allocation SET state='RELEASED' WHERE resource_id=? AND document_id=? AND space_id=? AND state='ACTIVE'`, item.ID, document, space); err != nil {
			return err
		}
		// 无消费者的物理对象仍沿原 GC 真正 DELETED 后结算全部分配。
		// 本分支只释放其他空间真实共享保留下的空计费范围。
		if scoped || len(consumers) == 0 || state != "READY" {
			continue
		}
		var paid uint64
		err = tx.QueryRowContext(ctx, `SELECT bytes FROM resource_space_allocation WHERE resource_id=? AND space_id=? AND state='ACTIVE' FOR UPDATE`, item.ID, space).Scan(&paid)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return err
		}
		if paid > q.Used {
			return errors.New("RESOURCE_ALLOCATION_QUOTA_INCONSISTENT")
		}
		result, err := tx.ExecContext(ctx, `UPDATE resource_space_allocation SET state='RELEASED' WHERE resource_id=? AND space_id=? AND state='ACTIVE' AND bytes=?`, item.ID, space, paid)
		if err != nil {
			return err
		}
		changed, err := result.RowsAffected()
		if err != nil || changed != 1 {
			return errors.New("RESOURCE_ALLOCATION_CLEANUP_CHANGED")
		}
		if err = writeQuota(ctx, tx, q, q.Used-paid, q.Reserved); err != nil {
			return err
		}
	}
	return nil
}

// cleanup.go：保留原实现，按业务合并文件。

// CleanupCandidates 只发现未完成上传的过期回收任务，不承担 READY 共享资源 GC。
// 候选读取不是删除授权；业务层持上传互斥锁，再在事务内锁配额和资源重新核验。
func CleanupCandidates(ctx context.Context, db *sql.DB, limit int) ([]uint64, error) {
	if limit < 1 {
		limit = 1
	}
	if limit > 32 {
		limit = 32
	}
	rows, err := db.QueryContext(ctx, `SELECT r.id FROM resource r JOIN quota_reservation q ON q.operation_id=r.upload_operation_id
		WHERE q.state='RESERVED' AND ((r.state='UPLOADING' AND q.expires_at<=UTC_TIMESTAMP(6)) OR r.state='DELETING') ORDER BY r.id LIMIT ?`, limit)
	if err != nil {
		return nil, errors.New("RESOURCE_CLEANUP_READ_FAILED")
	}
	defer rows.Close()
	ids := make([]uint64, 0, limit)
	for rows.Next() {
		var id uint64
		if err := rows.Scan(&id); err != nil {
			return nil, errors.New("RESOURCE_CLEANUP_READ_FAILED")
		}
		ids = append(ids, id)
	}
	if rows.Err() != nil {
		return nil, errors.New("RESOURCE_CLEANUP_READ_FAILED")
	}
	return ids, nil
}

// MarkDeleting 的 u 必须来自同一事务加锁的 UploadRow，并已锁住空间配额。
// 只允许数据库时间已过期且仍 RESERVED 的 UPLOADING 转换；任何共享引用或
// 未释放保护均阻止删除。原 UPLOAD 引用保留供恢复核对，不视作共享引用。
func MarkDeleting(ctx context.Context, tx *sql.Tx, u Upload) error {
	if u.State != "UPLOADING" {
		return errors.New("RESOURCE_UPLOAD_CHANGED")
	}
	result, err := tx.ExecContext(ctx, `UPDATE resource r SET r.state='DELETING',r.revision=r.revision+1
		WHERE r.id=? AND r.state='UPLOADING' AND r.revision=? AND r.revision<18446744073709551615
		AND EXISTS (SELECT 1 FROM quota_reservation q WHERE q.operation_id=r.upload_operation_id AND q.state='RESERVED' AND q.expires_at<=UTC_TIMESTAMP(6))
		AND NOT EXISTS (SELECT 1 FROM resource_reference x WHERE x.resource_id=r.id AND NOT (x.owner_kind='OPERATION' AND x.owner_ref=r.upload_operation_id AND x.purpose='UPLOAD'))
		AND NOT EXISTS (SELECT 1 FROM resource_protection p WHERE p.state<>'RELEASED' AND JSON_CONTAINS(p.resource_ids,CAST(? AS JSON)))`, u.ID, u.Revision, strconv.FormatUint(u.ID, 10))
	if err != nil {
		return errors.New("RESOURCE_DELETING_WRITE_FAILED")
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return errors.New("RESOURCE_UPLOAD_CHANGED")
	}
	return nil
}

// MarkDeleted 仅在业务层已删除完整文件及临时文件后调用；与配额和暂存预算释放
// 同事务提交。保留原 UPLOAD 引用与预留记录，使提交结果未知时仍可恢复、审计。
func MarkDeleted(ctx context.Context, tx *sql.Tx, u Upload) error {
	if u.State != "DELETING" {
		return errors.New("RESOURCE_UPLOAD_CHANGED")
	}
	result, err := tx.ExecContext(ctx, `UPDATE resource SET state='DELETED',revision=revision+1 WHERE id=? AND state='DELETING' AND revision=? AND revision<18446744073709551615`, u.ID, u.Revision)
	if err != nil {
		return errors.New("RESOURCE_DELETED_WRITE_FAILED")
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return errors.New("RESOURCE_UPLOAD_CHANGED")
	}
	return nil
}

// comment_context.go：保留原实现，按业务合并文件。

// BranchCommentResourceVersions 只列出未删除讨论原版本中的图片引用。
// 分支谱系归属仍由批注仓库的 ContextVersion 统一核验，不能将同单元任意版本放行。
func BranchCommentResourceVersions(ctx context.Context, tx *sql.Tx, document, branch, resource uint64) ([]uint64, error) {
	rows, err := tx.QueryContext(ctx, `SELECT DISTINCT v.id FROM comment_thread h
		JOIN content_version v ON v.id=h.content_version_id AND v.document_id=h.document_id
		JOIN resource_reference r ON r.document_id=v.document_id AND r.owner_kind='VERSION'
		AND r.owner_ref=CAST(v.id AS CHAR) AND r.purpose='CONTENT' AND r.resource_id=?
		WHERE h.document_id=? AND h.context_kind='BRANCH' AND h.context_ref=CAST(? AS CHAR) AND h.status<>'DELETED'`, resource, document, branch)
	if err != nil {
		return nil, contextReadError(err)
	}
	defer rows.Close()
	versions := []uint64{}
	for rows.Next() {
		var version uint64
		if err := rows.Scan(&version); err != nil {
			return nil, contextReadError(err)
		}
		versions = append(versions, version)
	}
	return versions, contextReadError(rows.Err())
}

// 历史MAIN图片必须仍被一个未删除讨论的原锚点版本实际引用；同清单中的
// 其他单元和后来版本不能借这个讨论获得下载资格。
func MainCommentResourceUnits(ctx context.Context, tx *sql.Tx, document, manifest, resource uint64) ([]uint64, error) {
	rows, err := tx.QueryContext(ctx, `SELECT DISTINCT v.unit_id FROM comment_thread h
		JOIN manifest_entry e ON e.document_id=h.document_id AND CAST(e.manifest_id AS CHAR)=h.context_ref AND e.content_version_id=h.content_version_id
		JOIN content_version v ON v.id=e.content_version_id AND v.document_id=e.document_id AND v.unit_id=e.unit_id
		JOIN resource_reference r ON r.document_id=v.document_id AND r.owner_kind='VERSION'
		AND r.owner_ref=CAST(v.id AS CHAR) AND r.purpose='CONTENT' AND r.resource_id=?
		WHERE h.document_id=? AND h.context_kind='MAIN' AND h.context_ref=CAST(? AS CHAR) AND h.status<>'DELETED'`, resource, document, manifest)
	if err != nil {
		return nil, contextReadError(err)
	}
	defer rows.Close()
	units := []uint64{}
	for rows.Next() {
		var unit uint64
		if err := rows.Scan(&unit); err != nil {
			return nil, contextReadError(err)
		}
		units = append(units, unit)
	}
	return units, contextReadError(rows.Err())
}

// conflict_context.go：保留原实现，按业务合并文件。

// ConflictResourceOperation 只定位同轮次已物化的冲突版本候选，不以历史正文扫描
// 或任意旧合并记录授权下载。业务层还须验证原 attempt、固定输入和版本摘要。
// JSON 数字以十进制字符串比较，避免大 uint64 被浮点或隐式数值比较截断。
func ConflictResourceOperation(ctx context.Context, tx *sql.Tx, document, round, resource uint64) (string, error) {
	var operation string
	err := tx.QueryRowContext(ctx, `SELECT m.operation_id FROM merge_operation m
		JOIN review_request r ON r.id=m.request_id AND r.document_id=?
		JOIN review_round v ON v.id=m.round_id AND v.request_id=r.id AND v.document_id=r.document_id
		JOIN operation_receipt o ON o.operation_id=m.operation_id AND o.kind='MERGE'
			AND o.state='REJECTED' AND o.expires_at>UTC_TIMESTAMP(6)
		JOIN durable_task t ON t.operation_id=m.operation_id AND t.kind='CONFLICT_CONTEXT_VERSIONS' AND t.state='SUCCEEDED'
		JOIN resource_reference x ON x.document_id=r.document_id AND x.resource_id=?
			AND x.owner_kind='VERSION' AND x.purpose='CONTENT'
			AND x.owner_ref IN (JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.Base')),
				JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.Main')),JSON_UNQUOTE(JSON_EXTRACT(t.payload,'$.Submitted')))
		WHERE m.round_id=? AND m.state='REJECTED'
		ORDER BY m.created_at DESC,m.operation_id LIMIT 1`, document, resource, round).Scan(&operation)
	return operation, contextReadError(err)
}

// context.go：保留原实现，按业务合并文件。

type Object struct {
	ID, Space, Size, Revision uint64
	Key, MediaType, State     string
	Checksum                  [32]byte
}

// ObjectRow 只读取对象事实；上传来源不能替代下载上下文的引用与权限证明。
// 下载持共享锁，防止检查 READY 后资源状态在同一事务内被回收方改变。
func ObjectRow(ctx context.Context, q Queryer, id uint64, lock bool) (Object, error) {
	var object Object
	var checksum []byte
	query := `SELECT id,space_id,byte_size,revision,storage_key,media_type,state,checksum FROM resource WHERE id=?`
	if lock {
		query += " FOR SHARE"
	}
	err := q.QueryRowContext(ctx, query, id).Scan(&object.ID, &object.Space, &object.Size, &object.Revision, &object.Key, &object.MediaType, &object.State, &checksum)
	if err != nil {
		return object, contextReadError(err)
	}
	if len(checksum) != len(object.Checksum) || object.Revision == 0 {
		return object, errors.New("RESOURCE_OBJECT_INVALID")
	}
	copy(object.Checksum[:], checksum)
	return object, nil
}

// ManifestContains 只承认清单固定版本的 CONTENT 引用，避免把同文档的草稿或
// 其他历史版本附件泄露到当前阅读上下文。字符串比较避免 owner_ref 数字隐式转换。
func ManifestContains(ctx context.Context, tx *sql.Tx, document, manifest, resource uint64) (bool, error) {
	var found bool
	err := tx.QueryRowContext(ctx, `SELECT EXISTS (
		SELECT 1 FROM manifest_entry e
		JOIN content_version v ON v.id=e.content_version_id AND v.document_id=e.document_id AND v.unit_id=e.unit_id
		JOIN resource_reference r ON r.document_id=e.document_id AND r.owner_kind='VERSION'
			AND r.owner_ref=CAST(v.id AS CHAR) AND r.purpose='CONTENT' AND r.resource_id=?
		WHERE e.document_id=? AND e.manifest_id=?)`, resource, document, manifest).Scan(&found)
	return found, contextReadError(err)
}

func VersionContains(ctx context.Context, tx *sql.Tx, document, version, resource uint64) (bool, error) {
	var found bool
	err := tx.QueryRowContext(ctx, `SELECT EXISTS (
		SELECT 1 FROM content_version v JOIN resource_reference r
		ON r.document_id=v.document_id AND r.owner_kind='VERSION' AND r.owner_ref=CAST(v.id AS CHAR)
		AND r.purpose='CONTENT' AND r.resource_id=? WHERE v.document_id=? AND v.id=?)`, resource, document, version).Scan(&found)
	return found, contextReadError(err)
}

// MainManifestAccessible 只证明调用者仍持有该固定清单的有效阅读视图。
// 当前 MAIN 指针与访问权限由业务层在文档锁下判断；这里不能放宽为任意历史清单。
func MainManifestAccessible(ctx context.Context, tx *sql.Tx, document, manifest, actor uint64, session []byte) (bool, error) {
	var found int
	err := tx.QueryRowContext(ctx, `SELECT 1 FROM read_view v
		JOIN document_admission a ON a.admission_id=v.admission_id AND a.document_id=v.document_id
		WHERE v.document_id=? AND v.manifest_id=? AND v.state='OPEN' AND v.expires_at>UTC_TIMESTAMP(6)
		AND a.state='OPEN' AND a.expires_at>UTC_TIMESTAMP(6) AND a.actor_id=? AND a.session_ref=?
		LIMIT 1 FOR SHARE`, document, manifest, actor, session).Scan(&found)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, contextReadError(err)
}

func ReviewContext(ctx context.Context, tx *sql.Tx, document, round uint64) (request, version, baseVersion uint64, err error) {
	err = tx.QueryRowContext(ctx, `SELECT request_id,content_version_id,base_version_id FROM review_round WHERE document_id=? AND id=?`, document, round).Scan(&request, &version, &baseVersion)
	return request, version, baseVersion, contextReadError(err)
}

type Branch struct {
	Unit, Epoch, BaseVersion uint64
	Room                     uint64
}

// BranchContext 在业务层文档锁之后锁当前分支事实，严格绑定现行谱系与单元。
// 房间尚未创建不影响基线附件读取；存在但损坏的登记必须报错，不能当作没有草稿。
func BranchContext(ctx context.Context, tx *sql.Tx, document, branch uint64) (Branch, error) {
	var current Branch
	err := tx.QueryRowContext(ctx, `SELECT b.unit_id,b.workspace_epoch,l.base_version_id
		FROM branch b JOIN collaboration_unit u ON u.id=b.unit_id AND u.document_id=b.document_id AND u.active_branch_id=b.id
		JOIN branch_lineage l ON l.id=b.current_lineage_id AND l.branch_id=b.id AND l.document_id=b.document_id AND l.unit_id=b.unit_id
		WHERE b.id=? AND b.document_id=? AND b.lifecycle='ACTIVE' AND u.retired_structure_id IS NULL FOR SHARE`, branch, document).Scan(&current.Unit, &current.Epoch, &current.BaseVersion)
	if err != nil {
		return current, contextReadError(err)
	}
	key := "BRANCH:" + strconv.FormatUint(branch, 10) + ":" + strconv.FormatUint(current.Unit, 10) + ":" + strconv.FormatUint(current.Epoch, 10)
	var raw []byte
	err = tx.QueryRowContext(ctx, `SELECT payload FROM durable_task WHERE kind='COLLABORATION_ROOM' AND business_key=? AND state='SUCCEEDED' FOR SHARE`, key).Scan(&raw)
	if err == sql.ErrNoRows {
		return current, nil
	}
	if err != nil {
		return current, contextReadError(err)
	}
	var payload struct {
		Room string `json:"room_id"`
	}
	if json.Unmarshal(raw, &payload) != nil {
		return current, errors.New("RESOURCE_ROOM_BINDING_INVALID")
	}
	room, err := strconv.ParseUint(payload.Room, 10, 64)
	if err != nil || room == 0 || strconv.FormatUint(room, 10) != payload.Room {
		return current, errors.New("RESOURCE_ROOM_BINDING_INVALID")
	}
	current.Room = room
	return current, nil
}

// DraftContains 必须使用持久房间身份和当前 epoch，旧房间引用不能授权新工作区。
func DraftContains(ctx context.Context, tx *sql.Tx, document, room, epoch, resource uint64) (bool, error) {
	owner := strconv.FormatUint(room, 10) + ":" + strconv.FormatUint(epoch, 10)
	var found bool
	err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM resource_reference
		WHERE document_id=? AND resource_id=? AND owner_kind='DRAFT' AND owner_ref=? AND purpose='ROOM_DRAFT')`, document, resource, owner).Scan(&found)
	return found, contextReadError(err)
}

// 不把数据库故障误报为不存在；仅真实缺行保留 ErrNoRows 供业务层区分。
func contextReadError(err error) error {
	if err == nil || err == sql.ErrNoRows {
		return err
	}
	return errors.New("RESOURCE_CONTEXT_READ_FAILED")
}

// document_capacity.go：保留原实现，按业务合并文件。

// 新 canonical 历史正文必须和 INSERT content_version 同事务记账。旧库第一次
// 遇到写入时先物化已有历史，因此不能用零字节文档代表完整容量已经接通。
// 调用方已锁 Doc，随后按现有权限→Doc→KB→Space→Quota 顺序调用。
func ChargeDocumentCapacity(ctx context.Context, tx *sql.Tx, document, space, added, limit uint64, operation string) error {
	q, err := LockQuota(ctx, tx, space, limit)
	if err != nil {
		return err
	}
	var paid, owner uint64
	err = tx.QueryRowContext(ctx, `SELECT space_id,bytes FROM document_capacity WHERE document_id=? FOR UPDATE`, document).Scan(&owner, &paid)
	charge := added
	if err == sql.ErrNoRows {
		// helper 在正文 INSERT 后执行，所以历史总量已经包含本次 added。
		if err = tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(v.byte_size),0) FROM content_version v LEFT JOIN reading_body_capacity r ON r.version_id=v.id WHERE v.document_id=? AND r.version_id IS NULL`, document).Scan(&paid); err != nil {
			return err
		}
		charge = paid
		paid = 0
		owner = space
	} else if err != nil {
		return err
	}
	if owner != space {
		return errors.New("DOCUMENT_CAPACITY_SPACE_CHANGED")
	}
	next, err := domain.TransferCapacity(paid, charge)
	if err != nil {
		return err
	}
	if q.Used > q.Limit || q.Reserved > q.Limit-q.Used || charge > q.Limit-q.Used-q.Reserved {
		return domain.Violation(domain.StateConflict, "QUOTA_EXCEEDED")
	}
	if err = writeQuota(ctx, tx, q, q.Used+charge, q.Reserved); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO document_capacity(document_id,space_id,bytes,operation_id) VALUES(?,?,?,?)
 ON DUPLICATE KEY UPDATE bytes=VALUES(bytes),operation_id=VALUES(operation_id)`, document, space, next, operation)
	return err
}

// 仅在真实永久删除/Account 清理已确认后的原清理事务释放；TRASHED 不释放历史。
func ReleaseDocumentCapacity(ctx context.Context, tx *sql.Tx, document, limit uint64) error {
	var space uint64
	err := tx.QueryRowContext(ctx, `SELECT space_id FROM document_capacity WHERE document_id=?`, document).Scan(&space)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	q, err := LockQuota(ctx, tx, space, limit)
	if err != nil {
		return err
	}
	var bytes uint64
	if err = tx.QueryRowContext(ctx, `SELECT bytes FROM document_capacity WHERE document_id=? AND space_id=? FOR UPDATE`, document, space).Scan(&bytes); err != nil {
		return err
	}
	if bytes > q.Used {
		return errors.New("DOCUMENT_CAPACITY_INCONSISTENT")
	}
	if err = writeQuota(ctx, tx, q, q.Used-bytes, q.Reserved); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `DELETE FROM document_capacity WHERE document_id=?`, document)
	return err
}

// draft_body_capacity.go：保留原实现，按业务合并文件。

// 调用方先锁来源、准入与 Doc/Space。每个 room/epoch 只增长一次正文高水位；
// 尚未取得 Node 结算证明的差额也保留在高水位中，多人或迟到确认不会重复预留。
func ReserveDraftBody(ctx context.Context, tx *sql.Tx, q Quota, document, room, epoch uint64, operation string, body domain.DraftBodyMeasure, prior bool) error {
	var savedDoc, savedSpace, savedRoom, savedEpoch, savedBytes uint64
	var savedHash []byte
	var state string
	err := tx.QueryRowContext(ctx, `SELECT document_id,space_id,room_id,workspace_epoch,body_bytes,body_hash,state
 FROM draft_body_reservation WHERE operation_id=? FOR UPDATE`, operation).Scan(&savedDoc, &savedSpace, &savedRoom, &savedEpoch, &savedBytes, &savedHash, &state)
	if err == nil {
		if savedDoc != document || savedSpace != q.Space || savedRoom != room || savedEpoch != epoch || savedBytes != body.Bytes || !bytes.Equal(savedHash, body.Hash) {
			return domain.Violation(domain.StateConflict, "DRAFT_BODY_BINDING_CHANGED")
		}
		if state == "RELEASED" {
			return domain.Violation(domain.StateConflict, "DRAFT_BODY_RESERVATION_CLOSED")
		}
		return nil
	}
	if err != sql.ErrNoRows {
		return err
	}
	if prior {
		return domain.Violation(domain.StateConflict, "DRAFT_BODY_BINDING_CHANGED")
	}
	var owner, scope, high uint64
	err = tx.QueryRowContext(ctx, `SELECT document_id,space_id,bytes FROM draft_body_capacity WHERE room_id=? AND workspace_epoch=? FOR UPDATE`, room, epoch).Scan(&owner, &scope, &high)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if err == nil && (owner != document || scope != q.Space) {
		return errors.New("DRAFT_BODY_OWNER_CHANGED")
	}
	added := uint64(0)
	if body.Bytes > high {
		added = body.Bytes - high
	}
	// 满额/旧库超额仍允许不增加占用的删除编辑；旧日志尚在，不以缩短正文假退款。
	if added > 0 && (q.Used > q.Limit || q.Reserved > q.Limit-q.Used || added > q.Limit-q.Used-q.Reserved) {
		return domain.Violation(domain.StateConflict, "QUOTA_EXCEEDED")
	}
	if err = writeQuota(ctx, tx, q, q.Used, q.Reserved+added); err != nil {
		return err
	}
	if body.Bytes > high {
		high = body.Bytes
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO draft_body_capacity(room_id,workspace_epoch,document_id,space_id,bytes) VALUES(?,?,?,?,?)
 ON DUPLICATE KEY UPDATE bytes=VALUES(bytes)`, room, epoch, document, q.Space, high); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO draft_body_reservation(operation_id,document_id,space_id,room_id,workspace_epoch,body_bytes,body_hash,bytes,state)
 VALUES(?,?,?,?,?,?,?,?,'RESERVED')`, operation, document, q.Space, room, epoch, body.Bytes, body.Hash, added)
	return err
}

// 只在上层逐项核对真实 Node COMMITTED 后调用。每笔原差额独立落账，
// 既不依赖确认次序，也不把过期准入或一个自报 seq 当成释放/成功证明。
func SettleDraftBody(ctx context.Context, tx *sql.Tx, q Quota, operation, size, hash string) error {
	var document, space, bodyBytes, amount uint64
	var digest []byte
	var state string
	err := tx.QueryRowContext(ctx, `SELECT document_id,space_id,body_bytes,body_hash,bytes,state FROM draft_body_reservation WHERE operation_id=? FOR UPDATE`, operation).Scan(&document, &space, &bodyBytes, &digest, &amount, &state)
	if err == sql.ErrNoRows {
		return nil
	} // 旧冻结资源保护没有正文侧账，保留原兼容路径。
	if err != nil {
		return err
	}
	body, err := domain.ParseDraftBodyMeasure(size, hash)
	if err != nil || body.Bytes != bodyBytes || !bytes.Equal(body.Hash, digest) || space != q.Space {
		return errors.New("DRAFT_BODY_PROOF_CHANGED")
	}
	if state == "COMMITTED" {
		return nil
	}
	if state != "RESERVED" || amount > q.Reserved || amount > ^uint64(0)-q.Used {
		return errors.New("DRAFT_BODY_SETTLEMENT_INCONSISTENT")
	}
	// 接受之前已检查容量；管理员后来降低上限也不能推翻实际已持久的原承诺。
	if err = writeQuota(ctx, tx, q, q.Used+amount, q.Reserved-amount); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO document_capacity(document_id,space_id,bytes,operation_id) VALUES(?,?,?,?)
 ON DUPLICATE KEY UPDATE bytes=bytes+VALUES(bytes),operation_id=VALUES(operation_id)`, document, space, amount, operation); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE draft_body_reservation SET state='COMMITTED' WHERE operation_id=?`, operation)
	return err
}

// 只有已核验 Node CLEANED 的生命周期事务调用：未落定操作也已不可能迟到写入。
// 已结算占用由 document_capacity 的实际保留版本统一退款，不能再减一次 used。
func ReleaseCleanedDraftBodies(ctx context.Context, tx *sql.Tx, q Quota, document uint64) error {
	var reserved uint64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(bytes),0) FROM draft_body_reservation WHERE document_id=? AND space_id=? AND state='RESERVED'`, document, q.Space).Scan(&reserved); err != nil {
		return err
	}
	if reserved > q.Reserved {
		return errors.New("DRAFT_BODY_RELEASE_INCONSISTENT")
	}
	if err := writeQuota(ctx, tx, q, q.Used, q.Reserved-reserved); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE draft_body_reservation SET state='RELEASED' WHERE document_id=? AND state<>'RELEASED'`, document); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `DELETE FROM draft_body_capacity WHERE document_id=?`, document)
	return err
}

// quota.go：保留原实现，按业务合并文件。

type Quota struct{ Space, Limit, Used, Reserved, Revision uint64 }

// LockQuota在权限/文档/操作锁之后调用。所有扣减均持有同一配额行，
// 不以事务外余额决定上传是否可接受，也不使用会回绕的无符号加法。
func LockQuota(ctx context.Context, tx *sql.Tx, space, initialLimit uint64) (Quota, error) {
	var q Quota
	if space == 0 || initialLimit == 0 {
		return q, domain.Violation(domain.InvalidRequest, "QUOTA_CONFIG_INVALID")
	}
	// 迁移后的首次空间访问从已落定的分配计费，不能为已有历史生成零余额。
	_, err := tx.ExecContext(ctx, `INSERT INTO quota_account(space_id,limit_bytes,used_bytes,reserved_bytes)
 SELECT ?,?,COALESCE((SELECT SUM(bytes) FROM document_capacity WHERE space_id=?),0)
 +COALESCE((SELECT SUM(bytes) FROM resource_space_allocation WHERE space_id=? AND state='ACTIVE'),0),0`, space, initialLimit, space, space)
	if err != nil {
		var duplicate *mysql.MySQLError
		if !errors.As(err, &duplicate) || duplicate.Number != 1062 {
			return q, errors.New("QUOTA_ACCOUNT_UNAVAILABLE")
		}
	}
	err = tx.QueryRowContext(ctx, `SELECT space_id,limit_bytes,used_bytes,reserved_bytes,revision FROM quota_account WHERE space_id=? FOR UPDATE`, space).Scan(&q.Space, &q.Limit, &q.Used, &q.Reserved, &q.Revision)
	if err != nil {
		return q, errors.New("QUOTA_ACCOUNT_UNAVAILABLE")
	}
	// 历史回填可能如实呈现超限；仍允许查询与确定的删除退款。
	// 每个新增预留/正文写入在自己的边界检查可用额度，绝不提高用户上限。
	if q.Revision == 0 {
		return q, errors.New("QUOTA_ACCOUNT_INCONSISTENT")
	}
	return q, nil
}

// ReserveQuota的q必须来自本事务LockQuota。原操作重试只能核对原金额，
// 不能延长expires_at或把已释放预留恢复为可上传。
func ReserveQuota(ctx context.Context, tx *sql.Tx, q Quota, operation string, size, expected uint64, expires time.Time) error {
	var space, amount uint64
	var state string
	var alive bool
	err := tx.QueryRowContext(ctx, `SELECT space_id,bytes,state,expires_at>UTC_TIMESTAMP(6) FROM quota_reservation WHERE operation_id=? FOR UPDATE`, operation).Scan(&space, &amount, &state, &alive)
	if err == nil {
		if space != q.Space || amount != size {
			return domain.Violation(domain.StateConflict, "OPERATION_INPUT_CONFLICT")
		}
		if state == "COMMITTED" || (state == "RESERVED" && alive) {
			return nil
		}
		return domain.Violation(domain.StateConflict, "QUOTA_RESERVATION_CLOSED")
	}
	if err != sql.ErrNoRows {
		return errors.New("QUOTA_RESERVATION_UNAVAILABLE")
	}
	if expected != q.Revision {
		return domain.Violation(domain.StateConflict, "QUOTA_REVISION_CHANGED")
	}
	if q.Revision == ^uint64(0) {
		return domain.Violation(domain.StateConflict, "QUOTA_REVISION_EXHAUSTED")
	}
	if q.Used > q.Limit || q.Reserved > q.Limit-q.Used || size > q.Limit-q.Used-q.Reserved {
		return domain.Violation(domain.StateConflict, "QUOTA_EXCEEDED")
	}
	if expires.IsZero() {
		return domain.Violation(domain.InvalidRequest, "QUOTA_EXPIRY_REQUIRED")
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO quota_reservation(operation_id,space_id,bytes,state,expires_at) VALUES(?,?,?,'RESERVED',?)`, operation, q.Space, size, expires.UTC()); err != nil {
		return errors.New("QUOTA_RESERVATION_WRITE_FAILED")
	}
	return writeQuota(ctx, tx, q, q.Used, q.Reserved+size)
}

// SettleQuota与resource READY/DELETED转换由同一业务事务提交。
// UNKNOWN不自动回收，只有调用方证明文件写入结束/删除完成后才允许结算。
func SettleQuota(ctx context.Context, tx *sql.Tx, q Quota, operation string, commit bool) error {
	var space, size uint64
	var state string
	var alive bool
	if tx.QueryRowContext(ctx, `SELECT space_id,bytes,state,expires_at>UTC_TIMESTAMP(6) FROM quota_reservation WHERE operation_id=? FOR UPDATE`, operation).Scan(&space, &size, &state, &alive) != nil {
		return errors.New("QUOTA_RESERVATION_UNAVAILABLE")
	}
	target := "RELEASED"
	if commit {
		target = "COMMITTED"
	}
	if space != q.Space {
		return domain.Violation(domain.StateConflict, "QUOTA_TARGET_CHANGED")
	}
	if state == target {
		return nil
	}
	if state != "RESERVED" || (commit && !alive) {
		return domain.Violation(domain.StateConflict, "QUOTA_RESERVATION_CLOSED")
	}
	if size > q.Reserved || q.Revision == ^uint64(0) {
		return errors.New("QUOTA_ACCOUNT_INCONSISTENT")
	}
	used := q.Used
	if commit {
		if used > q.Limit || size > q.Limit-used {
			return errors.New("QUOTA_ACCOUNT_INCONSISTENT")
		}
		used += size
	}
	if err := writeQuota(ctx, tx, q, used, q.Reserved-size); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE quota_reservation SET state=? WHERE operation_id=? AND state='RESERVED'`, target, operation); err != nil {
		return errors.New("QUOTA_RESERVATION_WRITE_FAILED")
	}
	return nil
}

func writeQuota(ctx context.Context, tx *sql.Tx, q Quota, used, reserved uint64) error {
	result, err := tx.ExecContext(ctx, `UPDATE quota_account SET used_bytes=?,reserved_bytes=?,revision=revision+1 WHERE space_id=? AND revision=? AND revision<18446744073709551615`, used, reserved, q.Space, q.Revision)
	if err != nil {
		return errors.New("QUOTA_ACCOUNT_WRITE_FAILED")
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return domain.Violation(domain.StateConflict, "QUOTA_REVISION_CHANGED")
	}
	return nil
}

// quota_initialization.go：保留原实现，按业务合并文件。

// 启动时为迁移前尚未上传过图片的空间物化真实上限。已存在的用户上限和费用
// 完全沿用；历史 backfill 由不可变迁移完成，不能在每次启动重复追加。
func InitializeQuotaAccounts(ctx context.Context, db *sql.DB, limit uint64) error {
	if limit == 0 {
		return errors.New("QUOTA_CONFIG_INVALID")
	}
	_, err := db.ExecContext(ctx, `INSERT INTO quota_account(space_id,limit_bytes,used_bytes,reserved_bytes)
 SELECT s.id,?,COALESCE(d.bytes,0)+COALESCE(r.bytes,0),0 FROM space s
 LEFT JOIN (SELECT space_id,SUM(bytes) bytes FROM document_capacity GROUP BY space_id) d ON d.space_id=s.id
 LEFT JOIN (SELECT space_id,SUM(bytes) bytes FROM resource_space_allocation WHERE state='ACTIVE' GROUP BY space_id) r ON r.space_id=s.id
 WHERE NOT EXISTS(SELECT 1 FROM quota_account q WHERE q.space_id=s.id)
 ON DUPLICATE KEY UPDATE space_id=VALUES(space_id)`, limit)
	return err
}

// ready_cleanup.go：保留原实现，按业务合并文件。

// ReadyCleanupCandidates 只发现候选，不能授权物理删除。原上传期限后额外保留
// 一小时插入正文的宽限，保证临近上传期限完成的对象仍有时间建立草稿引用。
// 该宽限仅用于 GC 判断，不修改原上传 expires 收据，也不为上传续期。
// 保留所有历史、草稿及未释放保护；原 UPLOAD 引用仅为恢复凭据，不阻止 GC。
func ReadyCleanupCandidates(ctx context.Context, db *sql.DB, limit int) ([]uint64, error) {
	if limit < 1 {
		limit = 1
	}
	if limit > 32 {
		limit = 32
	}
	rows, err := db.QueryContext(ctx, `SELECT r.id FROM resource r
		JOIN quota_reservation q ON q.operation_id=r.upload_operation_id
		WHERE q.state='COMMITTED' AND q.space_id=r.space_id AND q.bytes=r.byte_size
		AND ((r.state='READY' AND DATE_ADD(q.expires_at,INTERVAL 1 HOUR)<=UTC_TIMESTAMP(6)) OR r.state='DELETING')
		AND NOT EXISTS (SELECT 1 FROM resource_reference x WHERE x.resource_id=r.id
			AND NOT (x.owner_kind='OPERATION' AND x.owner_ref=r.upload_operation_id AND x.purpose='UPLOAD'))
		AND NOT EXISTS (SELECT 1 FROM resource_protection p WHERE p.state<>'RELEASED'
			AND JSON_CONTAINS(p.resource_ids,CAST(CAST(r.id AS CHAR) AS JSON)))
		ORDER BY r.id LIMIT ?`, limit)
	if err != nil {
		return nil, errors.New("RESOURCE_READY_CLEANUP_READ_FAILED")
	}
	defer rows.Close()
	ids := make([]uint64, 0, limit)
	for rows.Next() {
		var id uint64
		if err := rows.Scan(&id); err != nil {
			return nil, errors.New("RESOURCE_READY_CLEANUP_READ_FAILED")
		}
		ids = append(ids, id)
	}
	if rows.Err() != nil {
		return nil, errors.New("RESOURCE_READY_CLEANUP_READ_FAILED")
	}
	return ids, nil
}

// MarkReadyDeleting 由业务层先持文件排他锁，再依次锁配额、资源后调用。
// 候选发现与加锁之间可能新增引用，因此转换时重查全部约束；竞争返回 false，
// 只有成功进入 DELETING 后业务层才可删除文件，后续恢复沿用原上传操作身份。
func MarkReadyDeleting(ctx context.Context, tx *sql.Tx, u Upload) (bool, error) {
	if u.State != "READY" {
		return false, nil
	}
	if u.Revision == ^uint64(0) {
		return false, errors.New("RESOURCE_REVISION_EXHAUSTED")
	}
	result, err := tx.ExecContext(ctx, `UPDATE resource r SET r.state='DELETING',r.revision=r.revision+1
		WHERE r.id=? AND r.state='READY' AND r.revision=? AND r.revision<18446744073709551615
		AND EXISTS (SELECT 1 FROM quota_reservation q WHERE q.operation_id=r.upload_operation_id
			AND q.state='COMMITTED' AND q.space_id=r.space_id AND q.bytes=r.byte_size
			AND DATE_ADD(q.expires_at,INTERVAL 1 HOUR)<=UTC_TIMESTAMP(6))
		AND NOT EXISTS (SELECT 1 FROM resource_reference x WHERE x.resource_id=r.id
			AND NOT (x.owner_kind='OPERATION' AND x.owner_ref=r.upload_operation_id AND x.purpose='UPLOAD'))
		AND NOT EXISTS (SELECT 1 FROM resource_protection p WHERE p.state<>'RELEASED'
			AND JSON_CONTAINS(p.resource_ids,CAST(? AS JSON)))`, u.ID, u.Revision, strconv.FormatUint(u.ID, 10))
	if err != nil {
		return false, errors.New("RESOURCE_READY_DELETING_WRITE_FAILED")
	}
	n, err := result.RowsAffected()
	if err != nil || n > 1 {
		return false, errors.New("RESOURCE_READY_DELETING_WRITE_FAILED")
	}
	return n == 1, nil
}

// ReadyCleanupCommitted 在删除文件前区分两类 DELETING 恢复路径，不能只靠资源
// 状态判断它来自 READY GC。调用方仍须先按配额、资源顺序加锁。
func ReadyCleanupCommitted(ctx context.Context, tx *sql.Tx, u Upload) (bool, error) {
	var space, size uint64
	var state string
	err := tx.QueryRowContext(ctx, `SELECT space_id,bytes,state FROM quota_reservation WHERE operation_id=? FOR UPDATE`, u.Operation).Scan(&space, &size, &state)
	if err != nil {
		return false, errors.New("QUOTA_RESERVATION_UNAVAILABLE")
	}
	if space != u.Space || size != u.Size {
		return false, errors.New("RESOURCE_USED_QUOTA_INCONSISTENT")
	}
	return state == "COMMITTED", nil
}

// ReleaseUsedQuota 必须紧随同事务成功的 MarkDeleted；调用方传入转换前加锁的 u。
// MarkDeleted 的 CAS 保证每个资源只结算一次，不能单独重试本函数。COMMITTED
// 收据保留原样供上传幂等核对，READY GC 只减少 used，不再次释放暂存/预留预算。
func ReleaseUsedQuota(ctx context.Context, tx *sql.Tx, q Quota, u Upload) error {
	if u.State != "DELETING" || u.Space != q.Space || u.Size > q.Used || q.Revision == ^uint64(0) {
		return errors.New("RESOURCE_USED_QUOTA_INCONSISTENT")
	}
	committed, err := ReadyCleanupCommitted(ctx, tx, u)
	if err != nil {
		return err
	}
	if !committed {
		return errors.New("RESOURCE_USED_QUOTA_INCONSISTENT")
	}
	return writeQuota(ctx, tx, q, q.Used-u.Size, q.Reserved)
}

// retained_cleanup.go：保留原实现，按业务合并文件。

// 只供已取得Node墓碑并清除正文的孤立留存治理使用。与普通GC相同，最终
// 许可在资源锁下重查引用/保护；区别仅是此明确删除不等上传一小时宽限。
func MarkRetainedReadyDeleting(ctx context.Context, tx *sql.Tx, u Upload) (bool, error) {
	if u.State != "READY" || u.Revision == ^uint64(0) {
		return false, errors.New("RETAINED_RESOURCE_STATE_CHANGED")
	}
	result, err := tx.ExecContext(ctx, `UPDATE resource r SET r.state='DELETING',r.revision=r.revision+1
	 WHERE r.id=? AND r.state='READY' AND r.revision=? AND r.revision<18446744073709551615
	 AND EXISTS(SELECT 1 FROM quota_reservation q WHERE q.operation_id=r.upload_operation_id AND q.state='COMMITTED' AND q.space_id=r.space_id AND q.bytes=r.byte_size)
	 AND NOT EXISTS(SELECT 1 FROM resource_reference x WHERE x.resource_id=r.id
	  AND NOT(x.owner_kind='OPERATION' AND x.owner_ref=r.upload_operation_id AND x.purpose='UPLOAD'))
	 AND NOT EXISTS(SELECT 1 FROM resource_protection p WHERE p.state<>'RELEASED' AND JSON_CONTAINS(p.resource_ids,CAST(? AS JSON)))`,
		u.ID, u.Revision, strconv.FormatUint(u.ID, 10))
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	if err != nil || n > 1 {
		return false, errors.New("RETAINED_RESOURCE_DELETING_FAILED")
	}
	return n == 1, nil
}

func RetainedResourceProtection(ctx context.Context, tx *sql.Tx, u Upload, document uint64) (bool, error) {
	var own, shared, unknown bool
	err := tx.QueryRowContext(ctx, `SELECT
	 EXISTS(SELECT 1 FROM resource_reference WHERE resource_id=? AND document_id=? AND NOT(owner_kind='OPERATION' AND owner_ref=? AND purpose='UPLOAD')),
	 EXISTS(SELECT 1 FROM resource_reference WHERE resource_id=? AND document_id<>? AND NOT(owner_kind='OPERATION' AND owner_ref=? AND purpose='UPLOAD'))
	 OR EXISTS(SELECT 1 FROM resource_protection WHERE document_id<>? AND state IN ('PROTECTED','CONFIRMED') AND JSON_CONTAINS(resource_ids,CAST(? AS JSON))),
	 EXISTS(SELECT 1 FROM resource_protection WHERE state='UNKNOWN' AND JSON_CONTAINS(resource_ids,CAST(? AS JSON)))`,
		u.ID, document, u.Operation, u.ID, document, u.Operation, document, strconv.FormatUint(u.ID, 10), strconv.FormatUint(u.ID, 10)).Scan(&own, &shared, &unknown)
	if err != nil {
		return false, err
	}
	if own || unknown {
		return false, errors.New("RETAINED_RESOURCE_PROTECTION_PENDING")
	}
	return shared, nil
}

// 孤立文档已永久停止输入且持文件排它锁，未完成上传也可立即回收；
// RESERVED才可转换，UNKNOWN预留不能在删除IO后才发现无法结算。
func MarkRetainedUploadDeleting(ctx context.Context, tx *sql.Tx, u Upload) error {
	result, err := tx.ExecContext(ctx, `UPDATE resource r SET r.state='DELETING',r.revision=r.revision+1
	 WHERE r.id=? AND r.state='UPLOADING' AND r.revision=? AND r.revision<18446744073709551615
	 AND EXISTS(SELECT 1 FROM quota_reservation q WHERE q.operation_id=r.upload_operation_id AND q.state='RESERVED' AND q.space_id=r.space_id AND q.bytes=r.byte_size)
	 AND NOT EXISTS(SELECT 1 FROM resource_reference x WHERE x.resource_id=r.id
	  AND NOT(x.owner_kind='OPERATION' AND x.owner_ref=r.upload_operation_id AND x.purpose='UPLOAD'))
	 AND NOT EXISTS(SELECT 1 FROM resource_protection p WHERE p.state<>'RELEASED' AND JSON_CONTAINS(p.resource_ids,CAST(? AS JSON)))`,
		u.ID, u.Revision, strconv.FormatUint(u.ID, 10))
	if err != nil {
		return err
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return errors.New("RETAINED_UPLOAD_DELETING_FAILED")
	}
	return nil
}

// temporary.go：保留原实现，按业务合并文件。

type temporaryBudget struct {
	limit, reserved, revision uint64
}

// ReserveTemporary 在空间配额与资源锁之后占用全局上传暂存预算。
// 仅新建 UPLOADING 的事务调用一次；操作重试沿用资源状态，不能再次占用。
// limit 只用于首次初始化，后续配置变更不能隐式覆盖已持久化的预算。
func ReserveTemporary(ctx context.Context, tx *sql.Tx, limit, size uint64) error {
	if limit == 0 {
		return domain.Violation(domain.InvalidRequest, "TEMPORARY_BUDGET_CONFIG_INVALID")
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO temporary_budget(kind,limit_bytes,reserved_bytes) VALUES('RESOURCE_UPLOAD',?,0)`, limit)
	if err != nil {
		var duplicate *mysql.MySQLError
		if !errors.As(err, &duplicate) || duplicate.Number != 1062 {
			return errors.New("TEMPORARY_BUDGET_UNAVAILABLE")
		}
	}
	b, err := lockTemporary(ctx, tx)
	if err != nil {
		return err
	}
	if size > b.limit-b.reserved {
		return domain.Violation(domain.StateConflict, "TEMPORARY_BUDGET_EXCEEDED")
	}
	return writeTemporary(ctx, tx, b, b.reserved+size)
}

// ReleaseTemporary 与 READY 或 DELETED 状态转换处于同一事务。
// 调用方先锁定资源并确认尚未完成转换，因此重试不会重复释放其他上传的预算。
// 删除路径必须先完成文件清理，再释放预算，避免磁盘空间尚未回收就接纳新上传。
func ReleaseTemporary(ctx context.Context, tx *sql.Tx, size uint64) error {
	b, err := lockTemporary(ctx, tx)
	if err != nil {
		return err
	}
	if size > b.reserved {
		return errors.New("TEMPORARY_BUDGET_INCONSISTENT")
	}
	return writeTemporary(ctx, tx, b, b.reserved-size)
}

func lockTemporary(ctx context.Context, tx *sql.Tx) (temporaryBudget, error) {
	var b temporaryBudget
	if err := tx.QueryRowContext(ctx, `SELECT limit_bytes,reserved_bytes,revision FROM temporary_budget WHERE kind='RESOURCE_UPLOAD' FOR UPDATE`).Scan(&b.limit, &b.reserved, &b.revision); err != nil {
		return b, errors.New("TEMPORARY_BUDGET_UNAVAILABLE")
	}
	if b.limit == 0 || b.revision == 0 || b.reserved > b.limit {
		return b, errors.New("TEMPORARY_BUDGET_INCONSISTENT")
	}
	return b, nil
}

func writeTemporary(ctx context.Context, tx *sql.Tx, b temporaryBudget, reserved uint64) error {
	if b.revision == ^uint64(0) {
		return domain.Violation(domain.StateConflict, "TEMPORARY_BUDGET_REVISION_EXHAUSTED")
	}
	result, err := tx.ExecContext(ctx, `UPDATE temporary_budget SET reserved_bytes=?,revision=revision+1 WHERE kind='RESOURCE_UPLOAD' AND revision=? AND revision<18446744073709551615`, reserved, b.revision)
	if err != nil {
		return errors.New("TEMPORARY_BUDGET_WRITE_FAILED")
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return domain.Violation(domain.StateConflict, "TEMPORARY_BUDGET_REVISION_CHANGED")
	}
	return nil
}

// transfer_allocation.go：保留原实现，按业务合并文件。

// 精确资格绑定已接受的 Doc、目标空间与原转让，不允许只凭资源编号跨空间复用。
func ResourceAllowedForDocument(ctx context.Context, tx *sql.Tx, resource, document, space uint64) (bool, error) {
	var n int
	err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM resource r WHERE r.id=? AND
 ((r.space_id=? AND (EXISTS(SELECT 1 FROM resource_space_allocation z WHERE z.resource_id=r.id AND z.space_id=r.space_id AND z.state='ACTIVE')
 OR (NOT EXISTS(SELECT 1 FROM resource_space_allocation z WHERE z.resource_id=r.id AND z.space_id=r.space_id)
 AND EXISTS(SELECT 1 FROM quota_reservation q WHERE q.operation_id=r.upload_operation_id AND q.state='COMMITTED' AND q.space_id=r.space_id AND q.bytes=r.byte_size))))
 OR EXISTS(SELECT 1 FROM document_resource_allocation a
 JOIN personal_transfer t ON t.id=a.transfer_id AND t.state='ACCEPTED'
 JOIN resource_space_allocation s ON s.resource_id=a.resource_id AND s.space_id=a.space_id AND s.state='ACTIVE'
 WHERE a.resource_id=r.id AND a.document_id=? AND a.space_id=? AND a.state='ACTIVE'
 AND t.document_id=a.document_id))`, resource, space, document, space).Scan(&n)
	return n == 1, err
}

// 所有转让按照空间编号持锁，然后才锁资源。计划预留无 TTL：UNKNOWN 不归还预算。
func LockTransferQuotas(ctx context.Context, tx *sql.Tx, source, target, limit uint64) (Quota, Quota, error) {
	ids := []uint64{source, target}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	values := map[uint64]Quota{}
	for _, id := range ids {
		if _, ok := values[id]; ok {
			continue
		}
		q, err := LockQuota(ctx, tx, id, limit)
		if err != nil {
			return Quota{}, Quota{}, err
		}
		values[id] = q
	}
	return values[source], values[target], nil
}

func ReserveTransferCapacity(ctx context.Context, tx *sql.Tx, q Quota, bytes, expected uint64) error {
	if q.Revision != expected {
		return domain.Violation(domain.StateConflict, "QUOTA_REVISION_CHANGED")
	}
	if q.Used > q.Limit || q.Reserved > q.Limit-q.Used || bytes > q.Limit-q.Used-q.Reserved {
		return domain.Violation(domain.StateConflict, "QUOTA_EXCEEDED")
	}
	if q.Revision == ^uint64(0) {
		return errors.New("QUOTA_REVISION_EXHAUSTED")
	}
	return writeQuota(ctx, tx, q, q.Used, q.Reserved+bytes)
}

func CommitTransferCapacity(ctx context.Context, tx *sql.Tx, source, target Quota, reserved, released, redundant uint64) error {
	if redundant > reserved || reserved > target.Reserved || reserved-redundant > target.Limit-target.Used || released > source.Used {
		return errors.New("TRANSFER_QUOTA_INCONSISTENT")
	}
	if err := writeQuota(ctx, tx, source, source.Used-released, source.Reserved); err != nil {
		return err
	}
	return writeQuota(ctx, tx, target, target.Used+reserved-redundant, target.Reserved-reserved)
}

func ReleaseTransferCapacity(ctx context.Context, tx *sql.Tx, q Quota, reserved uint64) error {
	if reserved > q.Reserved {
		return errors.New("TRANSFER_QUOTA_INCONSISTENT")
	}
	return writeQuota(ctx, tx, q, q.Used, q.Reserved-reserved)
}

func PrepareTransferResources(ctx context.Context, tx *sql.Tx, document, source, target uint64) ([]domain.TransferResource, uint64, error) {
	// 保护先于 Node ACK 持久化，覆盖尚未转为 DRAFT 引用的已保存更新。
	rows, err := tx.QueryContext(ctx, `SELECT DISTINCT id FROM (
 SELECT resource_id id FROM resource_reference WHERE document_id=? AND NOT(owner_kind='OPERATION' AND purpose='UPLOAD')
 UNION SELECT j.id FROM resource_protection p JOIN JSON_TABLE(p.resource_ids,'$[*]' COLUMNS(id BIGINT UNSIGNED PATH '$')) j
 WHERE p.document_id=? AND p.state<>'RELEASED') x ORDER BY id`, document, document)
	if err != nil {
		return nil, 0, err
	}
	ids := []uint64{}
	for rows.Next() {
		var id uint64
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return nil, 0, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, 0, err
	}
	result := make([]domain.TransferResource, 0, len(ids))
	var total uint64
	for _, id := range ids {
		var physical, size uint64
		var state, operation string
		if err = tx.QueryRowContext(ctx, `SELECT space_id,byte_size,state,upload_operation_id FROM resource WHERE id=? FOR UPDATE`, id).Scan(&physical, &size, &state, &operation); err != nil {
			return nil, 0, err
		}
		if state != "READY" {
			return nil, 0, domain.Violation(domain.StateConflict, "TRANSFER_RESOURCE_NOT_READY")
		}
		allowed, err := ResourceAllowedForDocument(ctx, tx, id, document, source)
		if err != nil {
			return nil, 0, err
		}
		if !allowed {
			return nil, 0, domain.Violation(domain.PermissionDenied, "TRANSFER_RESOURCE_SCOPE_CHANGED")
		}
		// 兼容迁移后上传：只有真实 COMMITTED 原上传才能物化原计费事实。
		if _, err = tx.ExecContext(ctx, `INSERT INTO resource_space_allocation(resource_id,space_id,bytes,state,operation_id)
   SELECT id,space_id,byte_size,'ACTIVE',upload_operation_id FROM resource r WHERE id=?
   AND EXISTS(SELECT 1 FROM quota_reservation q WHERE q.operation_id=r.upload_operation_id AND q.state='COMMITTED' AND q.bytes=r.byte_size AND q.space_id=r.space_id)
   ON DUPLICATE KEY UPDATE resource_id=VALUES(resource_id)`, id); err != nil {
			return nil, 0, err
		}
		charged, err := lockedAllocationActive(ctx, tx, id, target)
		if err != nil {
			return nil, 0, err
		}
		r := domain.TransferResource{ID: id, Size: size, TargetCharge: !charged}
		result = append(result, r)
		if r.TargetCharge {
			total, err = domain.TransferCapacity(total, size)
			if err != nil {
				return nil, 0, err
			}
		}
	}
	return result, total, nil
}

func ApplyTransferResources(ctx context.Context, tx *sql.Tx, p domain.PersonalTransferPlan) (uint64, uint64, error) {
	var released, redundant uint64
	for _, r := range p.Resources {
		var size uint64
		var state string
		if err := tx.QueryRowContext(ctx, `SELECT byte_size,state FROM resource WHERE id=? FOR UPDATE`, r.ID).Scan(&size, &state); err != nil {
			return 0, 0, err
		}
		if size != r.Size || state != "READY" {
			return 0, 0, domain.Violation(domain.StateConflict, "TRANSFER_RESOURCE_CHANGED")
		}
		if r.TargetCharge {
			// 目标的另一份计划可能先提交同资源 allocation。容量预留仍真实存在，
			// 多收的部分在本事务归还，不因并发共享增加第二笔 used。
			active, err := lockedAllocationActive(ctx, tx, r.ID, p.TargetSpace)
			if err != nil {
				return 0, 0, err
			}
			if active {
				var err error
				redundant, err = domain.TransferCapacity(redundant, r.Size)
				if err != nil {
					return 0, 0, err
				}
			} else {
				if _, err := tx.ExecContext(ctx, `INSERT INTO resource_space_allocation(resource_id,space_id,bytes,state,operation_id) VALUES(?,?,?,'ACTIVE',?)
     ON DUPLICATE KEY UPDATE bytes=VALUES(bytes),state='ACTIVE',operation_id=VALUES(operation_id)`, r.ID, p.TargetSpace, r.Size, p.Operation); err != nil {
					return 0, 0, err
				}
			}
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO document_resource_allocation(resource_id,document_id,space_id,transfer_id,state) VALUES(?,?,?,?,'ACTIVE')
   ON DUPLICATE KEY UPDATE transfer_id=VALUES(transfer_id),state='ACTIVE'`, r.ID, p.Document, p.TargetSpace, p.Transfer); err != nil {
			return 0, 0, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE document_resource_allocation SET state='RELEASED' WHERE resource_id=? AND document_id=? AND space_id=?`, r.ID, p.Document, p.SourceSpace); err != nil {
			return 0, 0, err
		}
		consumers, err := TransferResourceConsumerDocuments(ctx, tx, []domain.TransferResource{r}, true)
		if err != nil {
			return 0, 0, err
		}
		others := false
		for _, id := range consumers {
			if id == p.Document {
				continue
			}
			var space uint64
			// 调用方已在配额/资源之前锁定全部消费者 Doc，这里只是当前读确认空间。
			if err = tx.QueryRowContext(ctx, `SELECT space_id FROM document WHERE id=? FOR UPDATE`, id).Scan(&space); err != nil {
				return 0, 0, err
			}
			if space == p.SourceSpace {
				others = true
			}
		}
		if !others {
			var bytes uint64
			err := tx.QueryRowContext(ctx, `SELECT bytes FROM resource_space_allocation WHERE resource_id=? AND space_id=? AND state='ACTIVE' FOR UPDATE`, r.ID, p.SourceSpace).Scan(&bytes)
			if err == sql.ErrNoRows {
				continue
			}
			if err != nil {
				return 0, 0, err
			}
			if bytes != r.Size {
				return 0, 0, errors.New("TRANSFER_ALLOCATION_INCONSISTENT")
			}
			released, err = domain.TransferCapacity(released, bytes)
			if err != nil {
				return 0, 0, err
			}
			if _, err = tx.ExecContext(ctx, `UPDATE resource_space_allocation SET state='RELEASED' WHERE resource_id=? AND space_id=?`, r.ID, p.SourceSpace); err != nil {
				return 0, 0, err
			}
		}
	}
	return released, redundant, nil
}

// 最终行锁下重计目标空间已经存在的分配。接受固定范围不变；共享资源可能由另一个
// 原迁移先迁入或迁出，所以旧 TargetCharge 不能决定现在的真实预算。
func RefreshTransferResourceCharges(ctx context.Context, tx *sql.Tx, p domain.PersonalTransferPlan) (domain.PersonalTransferPlan, uint64, error) {
	var total uint64
	for i, r := range p.Resources {
		var size uint64
		var state string
		if err := tx.QueryRowContext(ctx, `SELECT byte_size,state FROM resource WHERE id=? FOR UPDATE`, r.ID).Scan(&size, &state); err != nil {
			return p, 0, err
		}
		if size != r.Size || state != "READY" {
			return p, 0, domain.Violation(domain.StateConflict, "TRANSFER_RESOURCE_CHANGED")
		}
		charged, err := lockedAllocationActive(ctx, tx, r.ID, p.TargetSpace)
		if err != nil {
			return p, 0, err
		}
		p.Resources[i].TargetCharge = !charged
		if !charged {
			var err error
			total, err = domain.TransferCapacity(total, size)
			if err != nil {
				return p, 0, err
			}
		}
	}
	return p, total, nil
}

func lockedAllocationActive(ctx context.Context, tx *sql.Tx, resource, space uint64) (bool, error) {
	var state string
	err := tx.QueryRowContext(ctx, `SELECT state FROM resource_space_allocation WHERE resource_id=? AND space_id=? FOR UPDATE`, resource, space).Scan(&state)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return state == "ACTIVE", nil
}

// GC 调用方须先锁所有真实 allocation 的配额，再锁资源并证明无引用/UNKNOWN保护。
// MarkDeleted 的 CAS 与本函数同事务；重复删除不能二次返还 used。
func ResourceAllocationSpaces(ctx context.Context, q interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, id uint64) ([]uint64, error) {
	rows, err := q.QueryContext(ctx, `SELECT space_id FROM resource_space_allocation WHERE resource_id=? AND state='ACTIVE' ORDER BY space_id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []uint64{}
	for rows.Next() {
		var id uint64
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func ReleaseResourceAllocations(ctx context.Context, tx *sql.Tx, id uint64, quotas map[uint64]Quota) error {
	rows, err := tx.QueryContext(ctx, `SELECT space_id,bytes FROM resource_space_allocation WHERE resource_id=? AND state='ACTIVE' ORDER BY space_id FOR UPDATE`, id)
	if err != nil {
		return err
	}
	type allocation struct{ space, bytes uint64 }
	items := []allocation{}
	for rows.Next() {
		var a allocation
		if err = rows.Scan(&a.space, &a.bytes); err != nil {
			rows.Close()
			return err
		}
		items = append(items, a)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, a := range items {
		q, ok := quotas[a.space]
		if !ok || a.bytes > q.Used {
			return errors.New("RESOURCE_ALLOCATION_QUOTA_INCONSISTENT")
		}
		if err = writeQuota(ctx, tx, q, q.Used-a.bytes, q.Reserved); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, `UPDATE resource_space_allocation SET state='RELEASED' WHERE resource_id=? AND state='ACTIVE'`, id)
	return err
}

// transfer_consumers.go：保留原实现，按业务合并文件。

// 先发现共享 Doc 集合，调用方按编号在 quota/resource 之前锁这些 Doc。
// 资源锁后再次当前读核对集合，若发现新消费者则重试，避免 REPEATABLE READ 的
// 旧空间快照错误释放另一篇文档仍在使用的源计费分配。
func TransferResourceConsumerDocuments(ctx context.Context, q interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, items []domain.TransferResource, lock bool) ([]uint64, error) {
	if len(items) == 0 {
		return []uint64{}, nil
	}
	marks := strings.TrimSuffix(strings.Repeat("?,", len(items)), ",")
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	// 直接锁真实引用行，在下面已有的集合中去重，保留当前读而非旧快照。
	rows, err := q.QueryContext(ctx, `SELECT document_id FROM resource_reference WHERE resource_id IN (`+marks+`) AND document_id IS NOT NULL
 AND NOT(owner_kind='OPERATION' AND purpose='UPLOAD') ORDER BY document_id`+suffix, resourceIDs(items)...)
	if err != nil {
		return nil, err
	}
	ids := map[uint64]bool{}
	for rows.Next() {
		var id uint64
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids[id] = true
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	// 已确认保护即使尚未成为普通引用，也属于真实消费者；UNKNOWN 同样保留。
	protectionLock := ""
	if lock {
		protectionLock = " FOR UPDATE OF p"
	}
	// JSON_TABLE 是同一保护行的展开视图；只锁 p，否则 MySQL 把函数表加入
	// 锁定子句时触发 ER_DUPLICATE_TABLE_LOCK，原迁移无法落定。
	rows, err = q.QueryContext(ctx, `SELECT p.document_id FROM resource_protection p
 JOIN JSON_TABLE(p.resource_ids,'$[*]' COLUMNS(resource_id BIGINT UNSIGNED PATH '$')) j
 WHERE p.state<>'RELEASED' AND j.resource_id IN (`+marks+`) ORDER BY p.document_id`+protectionLock, resourceIDs(items)...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id uint64
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids[id] = true
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	result := make([]uint64, 0, len(ids))
	for id := range ids {
		result = append(result, id)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result, nil
}

func resourceIDs(items []domain.TransferResource) []any {
	args := make([]any, len(items))
	for i, item := range items {
		args[i] = item.ID
	}
	return args
}

// upload.go：保留原实现，按业务合并文件。

type Upload struct {
	ID, Space, Document, Uploader, Size, Revision uint64
	Operation, Key, MediaType, State              string
	Checksum                                      [32]byte
	Expires                                       time.Time
}

type Queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

// UploadRow从持久UPLOAD引用取得原文档，不能用调用者提供的另一个document_id替换。
func UploadRow(ctx context.Context, q Queryer, id uint64, lock bool) (Upload, error) {
	var u Upload
	var checksum []byte
	query := `SELECT r.id,r.space_id,x.document_id,r.uploader_id,r.byte_size,r.revision,r.upload_operation_id,r.storage_key,r.media_type,r.state,r.checksum,q.expires_at
		FROM resource r JOIN resource_reference x ON x.resource_id=r.id AND x.owner_kind='OPERATION' AND x.owner_ref=r.upload_operation_id AND x.purpose='UPLOAD'
		JOIN quota_reservation q ON q.operation_id=r.upload_operation_id WHERE r.id=?`
	if lock {
		query += " FOR UPDATE"
	}
	err := q.QueryRowContext(ctx, query, id).Scan(&u.ID, &u.Space, &u.Document, &u.Uploader, &u.Size, &u.Revision, &u.Operation, &u.Key, &u.MediaType, &u.State, &checksum, &u.Expires)
	if err == sql.ErrNoRows {
		return u, err
	}
	if err != nil {
		return u, errors.New("RESOURCE_UPLOAD_READ_FAILED")
	}
	if len(checksum) != 32 || u.Document == 0 || u.Revision == 0 {
		return u, errors.New("RESOURCE_UPLOAD_BINDING_INVALID")
	}
	copy(u.Checksum[:], checksum)
	return u, nil
}

// InsertUpload必须与quota reservation和通用操作回执使用同一事务。
func InsertUpload(ctx context.Context, tx *sql.Tx, u Upload) (uint64, error) {
	if _, err := tx.ExecContext(ctx, `INSERT INTO resource(space_id,uploader_id,storage_key,checksum,byte_size,media_type,state,upload_operation_id) VALUES(?,?,?,?,?,?,'UPLOADING',?)`, u.Space, u.Uploader, u.Key, u.Checksum[:], u.Size, u.MediaType, u.Operation); err != nil {
		return 0, errors.New("RESOURCE_UPLOAD_WRITE_FAILED")
	}
	var id uint64
	if tx.QueryRowContext(ctx, "SELECT LAST_INSERT_ID()").Scan(&id) != nil || id == 0 {
		return 0, errors.New("RESOURCE_UPLOAD_WRITE_FAILED")
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO resource_reference(resource_id,document_id,owner_kind,owner_ref,purpose) VALUES(?,?,'OPERATION',?,'UPLOAD')`, id, u.Document, u.Operation); err != nil {
		return 0, errors.New("RESOURCE_UPLOAD_BINDING_FAILED")
	}
	return id, nil
}

func MarkReady(ctx context.Context, tx *sql.Tx, u Upload) error {
	result, err := tx.ExecContext(ctx, `UPDATE resource SET state='READY',revision=revision+1 WHERE id=? AND state='UPLOADING' AND revision=? AND revision<18446744073709551615`, u.ID, u.Revision)
	if err != nil {
		return errors.New("RESOURCE_READY_WRITE_FAILED")
	}
	if n, e := result.RowsAffected(); e != nil || n != 1 {
		return errors.New("RESOURCE_UPLOAD_CHANGED")
	}
	return nil
}

func QuotaLimit(ctx context.Context, q Queryer, space uint64) (uint64, error) {
	var limit uint64
	err := q.QueryRowContext(ctx, "SELECT limit_bytes FROM quota_account WHERE space_id=?", space).Scan(&limit)
	return limit, err
}

func LockedReservationState(ctx context.Context, tx *sql.Tx, operation string) (string, error) {
	var state string
	err := tx.QueryRowContext(ctx, "SELECT state FROM quota_reservation WHERE operation_id=? FOR UPDATE", operation).Scan(&state)
	return state, err
}
