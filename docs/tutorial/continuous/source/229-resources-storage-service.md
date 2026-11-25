# services/document/internal/logic/document/resources_storage_service.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/229-resources-storage-service.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：编排用例、权限、事务及未知结果恢复；跨服务调用不能替代持久事实。

主要声明：`HoldUpload`、`resourceDTO`、`WriteUpload`、`validateUpload`、`validateUploadInTransaction`、`RecoverUpload`、`cleanupUpload`、`ownedUpload`、`PrepareComplete`、`RecoverReady`、`cleanupReady`、`ResourceLimits`、`Resources`、`ResourceFiles`、`NewResources`、`SpaceQuotaLimit`、`uploadAccess`、`PrepareUpload`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/resource"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"io"
	"mime"
	"strings"
	"time"
)

// resource_bytes.go：保留原实现，按业务合并文件。

// HoldUpload在读取文件前取得进程间互斥，直到完成确认的事务和响应结束才释放。
// 清理也必须持有同一把锁；取消请求不等于写入结束，不能仅凭TTL回收配额。
func (s *Resources) HoldUpload(ctx context.Context, actor Actor, id uint64) (func(), error) {
	u, _, err := s.ownedUpload(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	if s.files == nil {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "RESOURCE_STORAGE_UNAVAILABLE")
	}
	return s.files.Acquire(ctx, u.Key)
}

func resourceDTO(u repository.Upload) dto.Resource {
	digest := hex.EncodeToString(u.Checksum[:])
	return dto.Resource{Id: domain.TextID(u.ID), State: u.State, ByteSize: domain.TextID(u.Size), Checksum: &digest, MediaType: u.MediaType, Revision: domain.TextID(u.Revision)}
}

func (s *Resources) WriteUpload(ctx context.Context, actor Actor, id uint64, body io.Reader) (Actor, ReadBody, error) {
	release, err := s.HoldUpload(ctx, actor, id)
	if err != nil {
		return actor, nil, err
	}
	defer release()
	// 等锁之前取得的状态不能用于继续写入；清理可能已经将原号关闭。
	u, access, err := s.ownedUpload(ctx, actor, id)
	if err != nil {
		return actor, nil, err
	}
	if u.State != "UPLOADING" || !u.Expires.After(time.Now()) {
		return actor, nil, domain.Violation(domain.StateConflict, "RESOURCE_UPLOAD_CLOSED")
	}
	if err := s.validateUpload(ctx, actor, access, u); err != nil {
		return actor, nil, err
	}
	// 持锁后可以清除前一进程遗留的半文件；预算在创建上传时已持久预留。
	if err := s.files.CleanupUpload(ctx, u.Key); err != nil {
		return actor, nil, err
	}
	if err := s.files.Put(ctx, u.Key, body, u.Size, u.Checksum); err != nil {
		return actor, nil, err
	}
	// 文件落盘不授予READY，且写入期间的撤权必须在确认响应前重新核验。
	_, access, err = s.ownedUpload(ctx, actor, id)
	if err != nil {
		return actor, nil, err
	}
	actor.Sources = access.Sources
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		if err := validateUploadInTransaction(ctx, tx, access, u); err != nil {
			return nil, err
		}
		return resourceDTO(u), nil
	}
	return actor, run, nil
}

func (s *Resources) validateUpload(ctx context.Context, actor Actor, access DocumentAccess, u repository.Upload) error {
	if err := s.access.identity.Revalidate(ctx, actor.Proof); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := LockSources(ctx, tx, access.Sources); err != nil {
		return err
	}
	if err := validateUploadInTransaction(ctx, tx, access, u); err != nil {
		return err
	}
	return tx.Commit()
}

func validateUploadInTransaction(ctx context.Context, tx *sql.Tx, access DocumentAccess, u repository.Upload) error {
	doc, err := ValidateAccess(ctx, tx, access)
	if err != nil {
		return err
	}
	if doc.Lifecycle != "ACTIVE" {
		return domain.Violation(domain.StateConflict, "DOCUMENT_NOT_ACTIVE")
	}
	current, err := repository.UploadRow(ctx, tx, u.ID, false)
	if err != nil {
		return err
	}
	if current.State != "UPLOADING" || current.Revision != u.Revision || current.Key != u.Key || !current.Expires.After(time.Now()) {
		return domain.Violation(domain.StateConflict, "RESOURCE_UPLOAD_CLOSED")
	}
	return nil
}

// resource_cleanup.go：保留原实现，按业务合并文件。

// RecoverUpload只处理未完成上传。READY的共享引用回收另走引用/保护核验，
// 不能把上传过期条件套用于已被草稿、版本或发布引用的资源。
func (s *Resources) RecoverUpload(ctx context.Context) error {
	ids, err := repository.CleanupCandidates(ctx, s.db, 32)
	if err != nil {
		return err
	}
	for _, id := range ids {
		u, err := repository.UploadRow(ctx, s.db, id, false)
		if err != nil {
			return err
		}
		release, err := s.files.Acquire(ctx, u.Key)
		if err != nil {
			var violation domain.RuleViolation
			if errors.As(err, &violation) && violation.Code == "RESOURCE_UPLOAD_BUSY" {
				continue
			}
			return err
		}
		err = s.cleanupUpload(ctx, u)
		release()
		return err
	}
	return nil
}

func (s *Resources) cleanupUpload(ctx context.Context, expected repository.Upload) error {
	// 先持久关闭上传，再删文件；此提交未知时停止，不凭客户端错误推断回滚。
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := repository.LockQuota(ctx, tx, expected.Space, s.limits.SpaceBytes); err != nil {
		return err
	}
	u, err := repository.UploadRow(ctx, tx, expected.ID, true)
	if err != nil {
		return err
	}
	if u.State == "READY" || u.State == "DELETED" {
		return nil
	}
	if u.State == "UPLOADING" {
		if err := repository.MarkDeleting(ctx, tx, u); err != nil {
			return err
		}
	} else if u.State != "DELETING" {
		return nil
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	// guard覆盖整个IO与结算，进程退出由OS释放；原号恢复从DELETING继续。
	if err := s.files.CleanupUpload(ctx, u.Key); err != nil {
		return err
	}
	if err := s.files.Delete(ctx, u.Key); err != nil {
		return err
	}
	tx, err = s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	q, err := repository.LockQuota(ctx, tx, u.Space, s.limits.SpaceBytes)
	if err != nil {
		return err
	}
	u, err = repository.UploadRow(ctx, tx, u.ID, true)
	if err != nil {
		return err
	}
	if u.State == "DELETED" {
		return nil
	}
	if err := repository.MarkDeleted(ctx, tx, u); err != nil {
		return err
	}
	if err := repository.SettleQuota(ctx, tx, q, u.Operation, false); err != nil {
		return err
	}
	if err := repository.ReleaseTemporary(ctx, tx, u.Size); err != nil {
		return err
	}
	return tx.Commit()
}

// resource_complete.go：保留原实现，按业务合并文件。

// ownedUpload要求当前操作者既是上传发起人，也仍拥有文档准入来源。
// 仅知道资源id不能替他人完成上传；返回的来源还需在最终事务中重检。
func (s *Resources) ownedUpload(ctx context.Context, actor Actor, id uint64) (repository.Upload, DocumentAccess, error) {
	u, err := repository.UploadRow(ctx, s.db, id, false)
	if err == sql.ErrNoRows {
		err = domain.Violation(domain.EntityNotFound, "RESOURCE_NOT_FOUND")
	}
	if err != nil {
		return u, DocumentAccess{}, err
	}
	if u.Uploader != actor.ID {
		return u, DocumentAccess{}, domain.Violation(domain.PermissionDenied, "RESOURCE_UPLOAD_NOT_OWNED")
	}
	a, err := s.uploadAccess(ctx, actor, u.Document)
	return u, a, err
}

// PrepareComplete先在锁外验证真实文件，再返回交给统一Mutation执行的事务体与结果呈现函数。
// 调用方不能把“准备成功”当作READY：容量结算、状态变更和原operation回执必须一起落定。
func (s *Resources) PrepareComplete(ctx context.Context, actor Actor, id uint64, input dto.UploadComplete) (Actor, MutationBody, MutationPresentation, error) {
	u, access, err := s.ownedUpload(ctx, actor, id)
	if err != nil {
		return actor, nil, nil, err
	}
	if input.ByteSize != domain.TextID(u.Size) || input.Checksum != hex.EncodeToString(u.Checksum[:]) {
		return actor, nil, nil, domain.Violation(domain.StateConflict, "RESOURCE_CHECKSUM_MISMATCH")
	}
	if (u.State != "UPLOADING" && u.State != "READY") || (u.State == "UPLOADING" && !u.Expires.After(time.Now())) {
		return actor, nil, nil, domain.Violation(domain.StateConflict, "RESOURCE_UPLOAD_CLOSED")
	}
	if s.files == nil {
		return actor, nil, nil, domain.Violation(domain.TemporarilyUnavailable, "RESOURCE_STORAGE_UNAVAILABLE")
	}
	// 完整文件读取在SQL锁外。存储key不可覆盖，最终事务复核同一key/摘要/代次；
	// 真实文件不存在、只写半份或摘要不符时，绝不结算READY和used容量。
	file, err := s.files.Open(ctx, u.Key, u.Size, u.Checksum)
	if err != nil {
		return actor, nil, nil, err
	}
	if err := file.Close(); err != nil {
		return actor, nil, nil, domain.Violation(domain.TemporarilyUnavailable, "RESOURCE_STORAGE_UNAVAILABLE")
	}
	if err := s.files.CleanupUpload(ctx, u.Key); err != nil {
		return actor, nil, nil, err
	}
	actor.Sources = access.Sources
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		document, err := ValidateAccess(ctx, tx, access)
		if err != nil {
			return nil, err
		}
		if document.Lifecycle != "ACTIVE" {
			return nil, domain.Violation(domain.StateConflict, "DOCUMENT_NOT_ACTIVE")
		}
		quota, err := repository.LockQuota(ctx, tx, u.Space, s.limits.SpaceBytes)
		if err != nil {
			return nil, err
		}
		current, err := repository.UploadRow(ctx, tx, id, true)
		if err != nil {
			return nil, err
		}
		if current.Key != u.Key || current.Checksum != u.Checksum || current.Size != u.Size || current.Document != u.Document || current.Uploader != actor.ID {
			return nil, domain.Violation(domain.StateConflict, "RESOURCE_UPLOAD_CHANGED")
		}
		if current.State == "READY" {
			// 完成请求可以重放，但已结算的容量不能再次扣除；只补记同一资源的提交事实。
			if err = repository.RecordCommittedAllocation(ctx, tx, id); err != nil {
				return nil, err
			}
			return OperationResult(input.OperationId, "SUCCEEDED", id), nil
		}
		if current.State != "UPLOADING" || current.Revision != u.Revision {
			return nil, domain.Violation(domain.StateConflict, "RESOURCE_UPLOAD_CHANGED")
		}
		if err := repository.SettleQuota(ctx, tx, quota, u.Operation, true); err != nil {
			return nil, err
		}
		if err := repository.ReleaseTemporary(ctx, tx, u.Size); err != nil {
			return nil, err
		}
		if err := repository.MarkReady(ctx, tx, current); err != nil {
			return nil, err
		}
		if err = repository.RecordCommittedAllocation(ctx, tx, id); err != nil {
			return nil, err
		}
		return OperationResult(input.OperationId, "SUCCEEDED", id), nil
	}
	present := func(ctx context.Context, tx *sql.Tx, _ map[string]any) (any, []*pb.SourceEpoch, error) {
		// 原operation已成功仍不代表当前请求有权看结果；呈现前重新核验文档与来源。
		document, err := ValidateAccess(ctx, tx, access)
		if err != nil {
			return nil, nil, err
		}
		if document.Lifecycle != "ACTIVE" {
			return nil, nil, domain.Violation(domain.StateConflict, "DOCUMENT_NOT_ACTIVE")
		}
		current, err := repository.UploadRow(ctx, tx, id, false)
		if err != nil {
			return nil, nil, err
		}
		if current.State != "READY" {
			return nil, nil, domain.Violation(domain.StateConflict, "RESOURCE_NOT_READY")
		}
		digest := hex.EncodeToString(current.Checksum[:])
		return dto.Resource{Id: domain.TextID(id), State: current.State, ByteSize: domain.TextID(current.Size), Checksum: &digest, MediaType: current.MediaType, Revision: domain.TextID(current.Revision)}, access.Sources, nil
	}
	return actor, run, present, nil
}

// resource_ready_cleanup.go：保留原实现，按业务合并文件。

// RecoverReady回收已完成但不再被引用的对象。候选查询不提供删除许可；
// 真正关闭新增引用的边界是资源行锁下提交READY→DELETING。
func (s *Resources) RecoverReady(ctx context.Context) error {
	ids, err := repository.ReadyCleanupCandidates(ctx, s.db, 32)
	if err != nil {
		return err
	}
	for _, id := range ids {
		u, err := repository.UploadRow(ctx, s.db, id, false)
		if err != nil {
			return err
		}
		release, err := s.files.Acquire(ctx, u.Key)
		if err != nil {
			var violation domain.RuleViolation
			if errors.As(err, &violation) && violation.Code == "RESOURCE_UPLOAD_BUSY" {
				continue
			}
			return err
		}
		err = s.cleanupReady(ctx, u)
		release()
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Resources) cleanupReady(ctx context.Context, expected repository.Upload) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	quotas, err := repository.LockResourceAllocationQuotas(ctx, tx, expected.ID, expected.Space, s.limits.SpaceBytes)
	if err != nil {
		return err
	}
	u, err := repository.UploadRow(ctx, tx, expected.ID, true)
	if err != nil {
		return err
	}
	if err = repository.RecordCommittedAllocation(ctx, tx, u.ID); err != nil {
		return err
	}
	if err = repository.ValidateResourceAllocationQuotas(ctx, tx, u.ID, quotas); err != nil {
		return err
	}
	committed, err := repository.ReadyCleanupCommitted(ctx, tx, u)
	if err != nil {
		return err
	}
	if !committed {
		return nil
	}
	if u.State == "READY" {
		changed, err := repository.MarkReadyDeleting(ctx, tx, u)
		if err != nil {
			return err
		}
		if !changed {
			return nil
		}
	} else if u.State != "DELETING" {
		return nil
	}
	// 提交结果未知时不执行外部删除，后续根据持久DELETING继续同一对象。
	if err := tx.Commit(); err != nil {
		return err
	}
	if err := s.files.CleanupUpload(ctx, u.Key); err != nil {
		return err
	}
	if err := s.files.Delete(ctx, u.Key); err != nil {
		return err
	}
	tx, err = s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	quotas, err = repository.LockResourceAllocationQuotas(ctx, tx, u.ID, u.Space, s.limits.SpaceBytes)
	if err != nil {
		return err
	}
	u, err = repository.UploadRow(ctx, tx, u.ID, true)
	if err != nil {
		return err
	}
	if u.State == "DELETED" {
		return nil
	}
	if err = repository.ValidateResourceAllocationQuotas(ctx, tx, u.ID, quotas); err != nil {
		return err
	}
	// DELETED和used扣减同事务。文件删除幂等，提交丢回复也不会再次释放容量。
	if err := repository.MarkDeleted(ctx, tx, u); err != nil {
		return err
	}
	if err := repository.ReleaseResourceAllocations(ctx, tx, u.ID, quotas); err != nil {
		return err
	}
	return tx.Commit()
}

// resource_upload.go：保留原实现，按业务合并文件。

type ResourceLimits struct {
	SpaceBytes, UploadBytes, TemporaryBytes uint64
	UploadTTL                               time.Duration
}

type Resources struct {
	db     *sql.DB
	access *Access
	limits ResourceLimits
	origin string
	files  ResourceFiles
}

type ResourceFiles interface {
	Acquire(context.Context, string) (func(), error)
	AcquireRead(context.Context, string) (func(), error)
	CleanupUpload(context.Context, string) error
	Put(context.Context, string, io.Reader, uint64, [32]byte) error
	Open(context.Context, string, uint64, [32]byte) (io.ReadCloser, error)
	Delete(context.Context, string) error
}

func NewResources(db *sql.DB, access *Access, limits ResourceLimits, origin string, files ResourceFiles) *Resources {
	return &Resources{db: db, access: access, limits: limits, origin: strings.TrimRight(origin, "/"), files: files}
}

func (s *Resources) SpaceQuotaLimit() uint64 { return s.limits.SpaceBytes }

func (s *Resources) uploadAccess(ctx context.Context, actor Actor, document uint64) (DocumentAccess, error) {
	a, err := s.access.Authorize(ctx, actor, document, "RESOURCE_UPLOAD")
	if err != nil {
		return a, err
	}
	if !a.Permissions.Manage && len(a.Permissions.Edit) == 0 {
		return a, domain.Violation(domain.PermissionDenied, "RESOURCE_EDIT_REQUIRED")
	}
	if a.Document.Lifecycle != "ACTIVE" {
		return a, domain.Violation(domain.StateConflict, "DOCUMENT_NOT_ACTIVE")
	}
	return a, nil
}

func (s *Resources) PrepareUpload(ctx context.Context, actor Actor, document uint64, input dto.ResourceUpload) (Actor, MutationBody, MutationPresentation, error) {
	invalid := domain.Violation(domain.InvalidRequest, "RESOURCE_UPLOAD_INVALID")
	size, err := domain.CanonicalRevision(input.ByteSize)
	if err != nil || size > s.limits.UploadBytes || s.limits.SpaceBytes == 0 || s.limits.UploadTTL <= 0 {
		return actor, nil, nil, invalid
	}
	revision, err := domain.CanonicalRevision(input.ExpectedQuotaRevision)
	if err != nil {
		return actor, nil, nil, invalid
	}
	checksum, err := hex.DecodeString(input.Checksum)
	if err != nil || len(checksum) != 32 || hex.EncodeToString(checksum) != input.Checksum {
		return actor, nil, nil, invalid
	}
	media, params, err := mime.ParseMediaType(input.MediaType)
	if err != nil || media != input.MediaType || len(params) != 0 || len(media) > 100 {
		return actor, nil, nil, invalid
	}
	access, err := s.uploadAccess(ctx, actor, document)
	if err != nil {
		return actor, nil, nil, err
	}
	actor.Sources = access.Sources
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		current, err := ValidateAccess(ctx, tx, access)
		if err != nil {
			return nil, err
		}
		if current.Lifecycle != "ACTIVE" {
			return nil, domain.Violation(domain.StateConflict, "DOCUMENT_NOT_ACTIVE")
		}
		space, _ := domain.CanonicalID(current.SpaceId)
		quota, err := repository.LockQuota(ctx, tx, space, s.limits.SpaceBytes)
		if err != nil {
			return nil, err
		}
		expires := time.Now().UTC().Add(s.limits.UploadTTL)
		if err := repository.ReserveQuota(ctx, tx, quota, input.OperationId, size, revision, expires); err != nil {
			return nil, err
		}
		if err := repository.ReserveTemporary(ctx, tx, s.limits.TemporaryBytes, size); err != nil {
			return nil, err
		}
		var key [32]byte
		if _, err := rand.Read(key[:]); err != nil {
			return nil, err
		}
		u := repository.Upload{Space: space, Document: document, Uploader: actor.ID, Operation: input.OperationId, Key: hex.EncodeToString(key[:]), Size: size, MediaType: media}
		copy(u.Checksum[:], checksum)
		id, err := repository.InsertUpload(ctx, tx, u)
		if err != nil {
			return nil, err
		}
		return OperationResult(input.OperationId, "SUCCEEDED", id), nil
	}
	present := func(ctx context.Context, tx *sql.Tx, value map[string]any) (any, []*pb.SourceEpoch, error) {
		current, err := ValidateAccess(ctx, tx, access)
		if err != nil {
			return nil, nil, err
		}
		if current.Lifecycle != "ACTIVE" {
			return nil, nil, domain.Violation(domain.StateConflict, "DOCUMENT_NOT_ACTIVE")
		}
		result, ok := value["result"].(map[string]any)
		var rawID string
		if ok {
			rawID, _ = result["entity_id"].(string)
		} else if initial, ok := value["result"].(map[string]string); ok {
			rawID = initial["entity_id"]
		}
		id, err := domain.CanonicalID(rawID)
		if err != nil {
			return nil, nil, domain.Violation(domain.TemporarilyUnavailable, "RESOURCE_RECEIPT_INVALID")
		}
		u, err := repository.UploadRow(ctx, tx, id, false)
		if err != nil {
			return nil, nil, err
		}
		if u.Document != document || u.Uploader != actor.ID || u.Operation != input.OperationId {
			return nil, nil, domain.Violation(domain.StateConflict, "RESOURCE_UPLOAD_BINDING_CHANGED")
		}
		return dto.UploadHandle{ResourceId: rawID, UploadUrl: s.origin + "/v1/resources/" + rawID + "/upload", ExpiresAt: u.Expires.UTC().Format(time.RFC3339Nano)}, access.Sources, nil
	}
	return actor, run, present, nil
}
````

UTF-8/LF全文SHA-256：`f80945f9dfccb065109a27bb13ce3f3e77155d1564a0326efe1ebdacacd4c2fa`。[参考实现](../../../../services/document/internal/logic/document/resources_storage_service.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
