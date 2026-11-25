package document

import (
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/resource"
	"context"
	"errors"
)

// 复用资源服务的真实文件排它锁、删除IO和配额结算，不通过删引用就宣布
// 物理成功。跨Doc引用或已确认保护仍保留；UNKNOWN不能冒充合法共享保留。
func (s *Resources) CleanRetainedResource(ctx context.Context, id, space, document uint64) error {
	u, err := repository.UploadRow(ctx, s.db, id, false)
	if err != nil {
		return err
	}
	release, err := s.files.Acquire(ctx, u.Key)
	if err != nil {
		return err
	}
	defer release()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	quotas, err := repository.LockResourceAllocationQuotas(ctx, tx, u.ID, u.Space, s.limits.SpaceBytes)
	if err != nil {
		return err
	}
	u, err = repository.UploadRow(ctx, tx, id, true)
	if err != nil {
		return err
	}
	if u.Space != space {
		known, err := repository.ResourceTransferEvidence(ctx, tx, id, document, space)
		if err != nil {
			return err
		}
		if !known {
			return domain.Violation(domain.StateConflict, "RETAINED_RESOURCE_BINDING_CHANGED")
		}
	}
	if u.State == "DELETED" {
		return nil
	}
	if err = repository.RecordCommittedAllocation(ctx, tx, u.ID); err != nil {
		return err
	}
	if err = repository.ValidateResourceAllocationQuotas(ctx, tx, u.ID, quotas); err != nil {
		return err
	}
	shared, err := repository.RetainedResourceProtection(ctx, tx, u, document)
	if err != nil {
		return err
	}
	if shared {
		if u.State != "READY" {
			return domain.Violation(domain.StateConflict, "RETAINED_SHARED_RESOURCE_NOT_READY")
		}
		return nil
	}
	committed, err := repository.ReadyCleanupCommitted(ctx, tx, u)
	if err != nil {
		return err
	}
	if !committed {
		reservation, err := repository.LockedReservationState(ctx, tx, u.Operation)
		if err != nil {
			return err
		}
		if reservation != "RESERVED" {
			return errors.New("RETAINED_RESOURCE_RESERVATION_PENDING")
		}
	}
	if committed && u.State == "READY" {
		changed, err := repository.MarkRetainedReadyDeleting(ctx, tx, u)
		if err != nil {
			return err
		}
		if !changed {
			return domain.Violation(domain.StateConflict, "RETAINED_RESOURCE_CLEANUP_PENDING")
		}
	} else if committed && u.State != "DELETING" {
		return errors.New("RETAINED_RESOURCE_STATE_CHANGED")
	} else if !committed && (u.Document != document || (u.State != "UPLOADING" && u.State != "DELETING")) {
		return errors.New("RETAINED_UPLOAD_BINDING_CHANGED")
	}
	if !committed && u.State == "UPLOADING" {
		if err = repository.MarkRetainedUploadDeleting(ctx, tx, u); err != nil {
			return err
		}
	}
	if err = tx.Commit(); err != nil {
		return errors.New("RETAINED_RESOURCE_COMMIT_UNKNOWN")
	}
	if committed {
		// 状态已持久DELETING；cleanupReady原号恢复实际删除及used扣减，
		// 不再走普通READY候选的一小时宽限，不重复释放temporary预算。
		return s.cleanupReady(ctx, u)
	}
	return s.cleanupUpload(ctx, u)
}
