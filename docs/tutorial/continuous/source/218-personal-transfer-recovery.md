# services/document/internal/logic/document/personal_transfer_recovery.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/218-personal-transfer-recovery.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：编排用例、权限、事务及未知结果恢复；跨服务调用不能替代持久事实。

主要声明：`Recover`、`claimRecovery`、`measureRecovery`、`commitRecovery`、`lockRecoveryInputs`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
package document

import (
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	resources "collaboration/document/internal/repository/resource"
	"collaboration/platform"
	"context"
	"database/sql"
	"errors"
	"slices"
	"sort"
	"time"
)

// Recover 串起认领、远端关闭/计量、事务落定三个阶段。未知结果始终重试原任务。
func (s *PersonalTransfers) Recover(parent context.Context) error {
	if !s.Ready() {
		return errors.New("PERSONAL_TRANSFER_NOT_READY")
	}
	ctx, cancel := context.WithTimeout(parent, 4*time.Second)
	defer cancel()
	task, offer, err := s.claimRecovery(ctx)
	if err != nil {
		return err
	}
	complete := false
	defer func() {
		if !complete {
			retryCtx, retryCancel := context.WithTimeout(parent, time.Second)
			defer retryCancel()
			_ = repository.RetryPersonalTransfer(retryCtx, s.db, task, s.instance)
		}
	}()
	measured, err := s.measureRecovery(ctx, offer.Plan)
	if err != nil {
		return err
	}
	if err = s.commitRecovery(ctx, task, offer.Plan, measured); err != nil {
		return err
	}
	complete = true
	return nil
}

// 认领单独提交；远端调用不持有这笔短事务的锁，最终落定时再验证原任务代次。
func (s *PersonalTransfers) claimRecovery(ctx context.Context) (repository.PurgeTask, domain.PersonalTransfer, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return repository.PurgeTask{}, domain.PersonalTransfer{}, err
	}
	defer tx.Rollback()
	task, err := repository.ClaimPersonalTransfer(ctx, tx, s.instance)
	if errors.Is(err, sql.ErrNoRows) {
		return repository.PurgeTask{}, domain.PersonalTransfer{}, ErrPersonalTransferIdle
	}
	if err != nil {
		return repository.PurgeTask{}, domain.PersonalTransfer{}, err
	}
	offer, err := repository.LockPersonalTransferTask(ctx, tx, task, s.instance)
	if err != nil {
		return repository.PurgeTask{}, domain.PersonalTransfer{}, err
	}
	if err = tx.Commit(); err != nil {
		return repository.PurgeTask{}, domain.PersonalTransfer{}, err
	}
	return task, offer, nil
}

func (s *PersonalTransfers) measureRecovery(ctx context.Context, plan domain.PersonalTransferPlan) (PersonalTransferStorage, error) {
	for _, scope := range plan.Scopes {
		request := &pb.FenceRequest{RevocationId: scope.Operation, ClosedScopes: []*pb.SourceEpoch{{ScopeKey: scope.Key, Epoch: scope.Epoch}}, Generation: 1, TargetDigest: platform.SourceClosureDigest(scope.Key, scope.Epoch)}
		ack, err := s.fences.Close(ctx, request)
		if err != nil {
			return PersonalTransferStorage{}, err
		}
		if !platform.MatchesBootstrapAck(request, ack, "document") {
			return PersonalTransferStorage{}, errors.New("PERSONAL_TRANSFER_ACK_UNKNOWN")
		}
	}
	// 真实 ACK 后固定 Node 的 checkpoint/update_log 字节。未知计量只能重试原号，
	// 不猜零字节；确定容量不足才执行下面的业务拒绝补偿。
	measured, err := s.storage.MeasurePersonalTransfer(ctx, plan)
	if err != nil {
		return PersonalTransferStorage{}, err
	}
	if measured.Operation != plan.Operation || measured.Document != plan.Document || len(measured.Digest) != 32 {
		return PersonalTransferStorage{}, errors.New("PERSONAL_TRANSFER_STORAGE_INVALID")
	}
	return measured, nil
}

// 下面的复核、配额补偿与最终迁移只由本函数拥有的同一事务落定。
func (s *PersonalTransfers) commitRecovery(ctx context.Context, task repository.PurgeTask, plan domain.PersonalTransferPlan, measured PersonalTransferStorage) error {
	consumers, err := resources.TransferResourceConsumerDocuments(ctx, s.db, plan.Resources, false)
	if err != nil {
		return err
	}
	// gate 必须先于 KB 行锁。先取候选当前代次，再在下面的 KB 排他锁下核对，
	// 这样已完成的 KB 变更无需依赖某次外部读取才开放新的安全代次。
	targetACL, err := repository.ActiveTransferTargetACL(ctx, s.db, plan.TargetKB, plan.TargetSpace)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	plan, err = s.lockRecoveryInputs(ctx, tx, task, plan, consumers, targetACL)
	if err != nil {
		return err
	}
	source, target, err := resources.LockTransferQuotas(ctx, tx, plan.SourceSpace, plan.TargetSpace, s.quotaLimit)
	if err != nil {
		return err
	}
	if len(plan.NodeMeasurement) > 0 {
		if plan.NodeBytes != measured.Bytes || string(plan.NodeMeasurement) != string(measured.Digest) {
			return errors.New("PERSONAL_TRANSFER_STORAGE_CHANGED")
		}
	}
	plan, resourceBytes, err := resources.RefreshTransferResourceCharges(ctx, tx, plan)
	if err != nil {
		return err
	}
	currentConsumers, err := resources.TransferResourceConsumerDocuments(ctx, tx, plan.Resources, true)
	if err != nil {
		return err
	}
	if !slices.Equal(consumers, currentConsumers) {
		return errors.New("PERSONAL_TRANSFER_CONSUMERS_CHANGED")
	}
	required, err := domain.TransferCapacity(plan.BodyBytes, resourceBytes)
	if err != nil {
		return err
	}
	required, err = domain.TransferCapacity(required, measured.Bytes)
	if err != nil {
		return err
	}
	if required > plan.ReservedBytes {
		err = resources.ReserveTransferCapacity(ctx, tx, target, required-plan.ReservedBytes, target.Revision)
		if err != nil {
			var violation domain.RuleViolation
			if errors.As(err, &violation) && violation.Code == "QUOTA_EXCEEDED" {
				if err = resources.ReleaseTransferCapacity(ctx, tx, target, plan.ReservedBytes); err != nil {
					return err
				}
				if err = repository.RejectPersonalTransfer(ctx, tx, plan, "QUOTA_EXCEEDED"); err != nil {
					return err
				}
				if err = tx.Commit(); err != nil {
					return err
				}
				return nil
			}
			return err
		}
	} else if required < plan.ReservedBytes {
		if err = resources.ReleaseTransferCapacity(ctx, tx, target, plan.ReservedBytes-required); err != nil {
			return err
		}
	}
	plan.NodeBytes = measured.Bytes
	plan.NodeMeasurement = measured.Digest
	plan.ReservedBytes = required
	// 计量/账目与最终变更使用同一事务；未提交的调整不会泄漏到任务恢复。
	target, err = resources.LockQuota(ctx, tx, plan.TargetSpace, s.quotaLimit)
	if err != nil {
		return err
	}
	released, redundant, err := resources.ApplyTransferResources(ctx, tx, plan)
	if err != nil {
		return err
	}
	released, err = domain.TransferCapacity(released, plan.SourceBodyBytes)
	if err != nil {
		return err
	}
	if err = resources.CommitTransferCapacity(ctx, tx, source, target, plan.ReservedBytes, released, redundant); err != nil {
		return err
	}
	if err = repository.CompletePersonalTransfer(ctx, tx, plan); err != nil {
		return err
	}
	if err = repository.SavePersonalTransferMeasurement(ctx, tx, plan); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	return nil
}

// 锁顺序仍为来源 gate → 所有消费者文档 → 原任务 → KB/父空间；此函数不提交。
func (s *PersonalTransfers) lockRecoveryInputs(ctx context.Context, tx *sql.Tx, task repository.PurgeTask, plan domain.PersonalTransferPlan, consumers []uint64, targetACL uint64) (domain.PersonalTransferPlan, error) {
	// 来源 gate 在 Doc/任务之前排序锁定。后台不重验原 owner 的会话，避免其注销
	// 和已登记迁移互相等待；Account 固定父空间范围必须先等待本任务终态。
	var err error
	scopes := append([]domain.PurgeScope{}, plan.Scopes...)
	targetKey := "kb-acl:" + domain.TextID(plan.TargetKB)
	lockScopes := append(append([]domain.PurgeScope{}, scopes...), domain.PurgeScope{Key: targetKey})
	sort.Slice(lockScopes, func(i, j int) bool { return lockScopes[i].Key < lockScopes[j].Key })
	for _, scope := range lockScopes {
		gate, err := repository.LockedSourceGate(ctx, tx, scope.Key)
		if err != nil {
			return plan, err
		}
		epoch, state := gate.Epoch, gate.State
		if scope.Key == targetKey {
			pending, err := repository.PendingSourceTransition(ctx, tx, targetKey)
			if err != nil {
				return plan, err
			}
			if pending || epoch > targetACL || (epoch == targetACL && state != "OPEN") {
				return plan, errors.New("PERSONAL_TRANSFER_TARGET_SOURCE_PENDING")
			}
			if epoch < targetACL {
				if err = repository.ReopenSourceGate(ctx, tx, targetKey, targetACL); err != nil {
					return plan, err
				}
			}
			continue
		}
		if state != "BLOCKED" || epoch < scope.Epoch {
			return plan, errors.New("PERSONAL_TRANSFER_SOURCE_CHANGED")
		}
	}
	if err := repository.LockTransferConsumerDocuments(ctx, tx, append(append([]uint64{}, consumers...), plan.Document)); err != nil {
		return plan, err
	}
	facts, err := repository.TrashDocumentRow(ctx, tx, plan.Document, true)
	if err != nil {
		return plan, err
	}
	doc := facts.Document
	if doc.Lifecycle != "CLOSING" || doc.ControlRevision != domain.TextID(plan.ClosingControl) || doc.SpaceId != domain.TextID(plan.SourceSpace) {
		return plan, errors.New("PERSONAL_TRANSFER_DOCUMENT_CHANGED")
	}
	offer, err := repository.LockPersonalTransferTask(ctx, tx, task, s.instance)
	if err != nil {
		return plan, err
	}
	plan = offer.Plan
	for _, scope := range scopes {
		if err = repository.TransferFenceConfirmed(ctx, tx, scope); err != nil {
			return plan, err
		}
	}
	if err = lockTransferParents(ctx, tx, plan.SourceSpace, plan.TargetSpace, plan.TargetKB, plan.TargetUser); err != nil {
		return plan, err
	}
	targetKB, err := repository.LockedCreationKnowledgeBase(ctx, tx, plan.TargetKB)
	if err != nil {
		return plan, err
	}
	if targetKB.Revision != targetACL {
		return plan, errors.New("PERSONAL_TRANSFER_TARGET_SOURCE_CHANGED")
	}
	return plan, nil
}
````

UTF-8/LF全文SHA-256：`88f653cb36536407c8e405c28f2a19b79242017ca5c33c71a295dd085345b160`。[参考实现](../../../../services/document/internal/logic/document/personal_transfer_recovery.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
