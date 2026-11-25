# services/document/internal/logic/document/resource_finalize.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/226-resource-finalize.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：编排用例、权限、事务及未知结果恢复；跨服务调用不能替代持久事实。

主要声明：`Finalize`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
package document

import (
	"bytes"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	resources "collaboration/document/internal/repository/resource"
	"context"
	"encoding/hex"
	"sort"
)

func (s *DraftResources) Finalize(ctx context.Context, in *pb.DraftResourceFinalize, release bool) (*pb.DraftResourceReceipt, error) {
	if in == nil {
		return nil, domain.RejectProtection(domain.ProtectionInvalid, "PROTECTION_FINALIZE_INVALID")
	}
	req, err := s.queryProtection(ctx, in.Query)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(req.UpdateHash, in.UpdateHash) {
		return nil, domain.RejectProtection(domain.ProtectionConflict, "UPDATE_HASH_CONFLICT")
	}
	proof, err := s.outcomes.UpdateOutcome(ctx, req)
	if err != nil {
		return nil, err
	}
	// 自报COMMITTED或seq不构成保存证据，必须逐项匹配实际Node日志的原保护、grant和hash。
	committed := proof.State == "COMMITTED" && proof.Protection == req.Context.OperationId && proof.Grant == req.GrantId && proof.Hash == hex.EncodeToString(req.UpdateHash) && proof.Seq == domain.TextID(in.DurableSeq) && in.NodeReceiptRef == domain.TextID(req.RoomId)+":"+domain.TextID(req.WorkspaceEpoch)+":"+proof.Seq
	if !committed || in.ProvenNodeOutcome != "COMMITTED" {
		return nil, domain.RejectProtection(domain.ProtectionPrecondition, "NODE_OUTCOME_NOT_PROVEN")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = validateProtectionGrant(ctx, tx, req); err != nil {
		return nil, err
	}
	space, lifecycle, err := repository.LockedResourceDocument(ctx, tx, req.Context.Target.DocumentId)
	if err != nil || lifecycle != "ACTIVE" {
		return nil, domain.RejectProtection(domain.ProtectionForbidden, "DOCUMENT_NOT_CURRENT")
	}
	if err = repository.LockDocumentCapacity(ctx, tx, req.Context.Target.DocumentId); err != nil {
		return nil, err
	}
	state, err := repository.LockedResourceProtectionState(ctx, tx, req.Context.OperationId)
	if err != nil {
		return nil, domain.RejectProtection(domain.ProtectionUnknown, "PROTECTION_UNKNOWN")
	}
	if state == "RELEASED" {
		if release {
			return protectionReceipt(req, state), nil
		}
		return nil, domain.RejectProtection(domain.ProtectionPrecondition, "PROTECTION_RELEASED")
	}
	if release && state != "CONFIRMED" {
		return nil, domain.RejectProtection(domain.ProtectionPrecondition, "SAFE_TRANSFER_REQUIRED")
	}
	limit, err := resources.QuotaLimit(ctx, tx, space)
	if err != nil {
		return nil, err
	}
	q, err := resources.LockQuota(ctx, tx, space, limit)
	if err != nil {
		return nil, err
	}
	if err = resources.SettleDraftBody(ctx, tx, q, req.Context.OperationId, proof.BodyBytes, proof.BodyHash); err != nil {
		return nil, err
	}
	ids := append([]uint64{}, req.ResourceIds...)
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		current, err := repository.LockedResourceState(ctx, tx, id)
		if err != nil || current != "READY" {
			return nil, domain.RejectProtection(domain.ProtectionPrecondition, "RESOURCE_NOT_READY")
		}
		owner := domain.TextID(req.RoomId) + ":" + domain.TextID(req.WorkspaceEpoch)
		if err = repository.InsertDraftResourceRef(ctx, tx, id, req.Context.Target.DocumentId, owner, req.Generation); err != nil {
			return nil, err
		}
	}
	// 同一事务先让DRAFT引用接住，再删临时pin；未知结果只多保留，不造成资源悬空。
	if err = repository.DeleteOperationResourceRefs(ctx, tx, req.Context.OperationId); err != nil {
		return nil, err
	}
	next := "CONFIRMED"
	if release {
		next = "RELEASED"
	}
	if err = repository.FinalizeResourceProtection(ctx, tx, req.Context.OperationId, next, in.NodeReceiptRef, in.DurableSeq); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, domain.RejectProtection(domain.ProtectionUnknown, "PROTECTION_COMMIT_UNKNOWN")
	}
	return protectionReceipt(req, next), nil
}
````

UTF-8/LF全文SHA-256：`7fbaaf6809016cdf8f360215237209ff648fb53d418c18a898cb8328c3301496`。[参考实现](../../../../services/document/internal/logic/document/resource_finalize.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
