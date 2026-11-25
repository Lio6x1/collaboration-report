# services/document/internal/logic/document/resources_reading_service.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/228-resources-reading-service.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：编排用例、权限、事务及未知结果恢复；跨服务调用不能替代持久事实。

主要声明：`InitializeQuotas`、`branchCommentContains`、`mainCommentContains`、`resourceContextCheck`、`context`、`publicContext`、`PrepareQuota`、`ResourceTarget`、`ResourceReading`、`NewResourceReading`、`ResourceDownload`、`Close`、`Open`、`resourceReadFacts`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	comments "collaboration/document/internal/repository/comment"
	documents "collaboration/document/internal/repository/document"
	repository "collaboration/document/internal/repository/resource"
	"collaboration/platform"
	"context"
	"database/sql"
	"errors"
	"io"
	"sync"
)

// resource_capacity.go：保留原实现，按业务合并文件。

func (s *Resources) InitializeQuotas(ctx context.Context) error {
	return repository.InitializeQuotaAccounts(ctx, s.db, s.limits.SpaceBytes)
}

// resource_comment_context.go：保留原实现，按业务合并文件。

func (s *ResourceReading) branchCommentContains(ctx context.Context, tx *sql.Tx, access DocumentAccess, target ResourceTarget) (bool, error) {
	versions, err := repository.BranchCommentResourceVersions(ctx, tx, target.Document, target.Context, target.Resource)
	if err != nil || len(versions) == 0 {
		return false, err
	}
	// 与持久批注读取保持相同的真实分支归属证明，不依赖可变草稿或当前谱系。
	bound := false
	for _, version := range versions {
		bound, err = comments.ContextVersion(ctx, tx, target.Document, "BRANCH", target.Context, version)
		if err != nil {
			return false, err
		}
		if bound {
			break
		}
	}
	if !bound {
		return false, nil
	}
	unit, _, err := comments.BranchScope(ctx, tx, target.Document, target.Context)
	if err != nil {
		return false, err
	}
	// COMMENT 可读取已保存讨论的固定原版本，但不能据此访问任何当前房间草稿。
	if access.Permissions.Comment || access.Permissions.Manage || access.Permissions.Edit[unit] {
		return true, nil
	}
	return historicalContentRangeCovered(ctx, tx, access, target.Document, unit, s.reviews.validator)
}

func (s *ResourceReading) mainCommentContains(ctx context.Context, tx *sql.Tx, access DocumentAccess, target ResourceTarget) (bool, error) {
	units, err := repository.MainCommentResourceUnits(ctx, tx, target.Document, target.Context, target.Resource)
	if err != nil {
		return false, err
	}
	for _, unit := range units {
		if access.Permissions.Comment || access.Permissions.Manage || access.Permissions.Edit[unit] {
			return true, nil
		}
		covered, err := historicalContentRangeCovered(ctx, tx, access, target.Document, unit, s.reviews.validator)
		if err != nil {
			return false, err
		}
		if covered {
			return true, nil
		}
	}
	return false, nil
}

// resource_context.go：保留原实现，按业务合并文件。

type resourceContextCheck func(context.Context, *sql.Tx) (resourceReadFacts, error)

func (s *ResourceReading) context(ctx context.Context, actor *Actor, target ResourceTarget) ([]*pb.SourceEpoch, resourceContextCheck, error) {
	if target.Resource == 0 || target.Document == 0 || target.Context == 0 {
		return nil, nil, domain.Violation(domain.InvalidRequest, "RESOURCE_CONTEXT_INVALID")
	}
	if target.Kind == "PUBLIC" {
		return s.publicContext(ctx, actor, target)
	}
	if target.Kind != "MAIN" && target.Kind != "BRANCH" && target.Kind != "REVIEW" {
		return nil, nil, domain.Violation(domain.InvalidRequest, "RESOURCE_CONTEXT_INVALID")
	}
	if actor == nil {
		return nil, nil, domain.Violation(domain.SessionRequired, "SESSION_REQUIRED")
	}
	access, err := s.resources.access.Authorize(ctx, *actor, target.Document, "READ_RESOURCE")
	if err != nil {
		return nil, nil, err
	}
	check := func(ctx context.Context, tx *sql.Tx) (resourceReadFacts, error) {
		var facts resourceReadFacts
		doc, err := ValidateAccess(ctx, tx, access)
		if err != nil {
			return facts, err
		}
		if doc.Lifecycle != "ACTIVE" {
			return facts, domain.Violation(domain.StateConflict, "DOCUMENT_NOT_ACTIVE")
		}
		facts.space, _ = domain.CanonicalID(doc.SpaceId)
		allowed := false
		switch target.Kind {
		case "MAIN":
			// 当前锁定清单或本人会话的有效阅读快照，不能用任意历史manifest取得文件。
			allowed = doc.MainLocked && doc.CurrentManifestId == domain.TextID(target.Context)
			if !allowed {
				allowed, err = repository.MainManifestAccessible(ctx, tx, target.Document, target.Context, actor.ID, actor.Proof.SessionRef)
			}
			if err == nil && allowed {
				allowed, err = repository.ManifestContains(ctx, tx, target.Document, target.Context, target.Resource)
			}
			if err == nil && !allowed {
				// 阅读快照到期不删除持久讨论；只放行当前仍可读讨论原版本内的图片。
				allowed, err = s.mainCommentContains(ctx, tx, access, target)
			}
		case "BRANCH":
			branch, e := repository.BranchContext(ctx, tx, target.Document, target.Context)
			err = e
			if err != nil && err != sql.ErrNoRows {
				break
			}
			if err == nil && access.Permissions.Edit[branch.Unit] {
				allowed, err = repository.VersionContains(ctx, tx, target.Document, branch.BaseVersion, target.Resource)
				if err == nil && !allowed && branch.Room != 0 {
					allowed, err = repository.DraftContains(ctx, tx, target.Document, branch.Room, branch.Epoch, target.Resource)
				}
			}
			if !allowed && (err == nil || err == sql.ErrNoRows) {
				// 历史分支或草稿已移除图片时，只以可读的未删除讨论原版本补足引用。
				// 补足失败仍保留原归属缺失，不能把跨文档的不存在上下文改成普通无权。
				original := err
				allowed, err = s.branchCommentContains(ctx, tx, access, target)
				if err == nil && !allowed {
					err = original
				}
			}
		case "REVIEW":
			request, version, base, e := repository.ReviewContext(ctx, tx, target.Document, target.Context)
			err = e
			if err != nil {
				break
			}
			pointer, e := documents.ReviewPointerRow(ctx, tx, request)
			err = e
			if err != nil {
				break
			}
			if err = s.reviews.validateReviewReadAccess(ctx, tx, access, pointer); err != nil {
				break
			}
			allowed, err = repository.VersionContains(ctx, tx, target.Document, version, target.Resource)
			if err == nil && !allowed {
				allowed, err = repository.VersionContains(ctx, tx, target.Document, base, target.Resource)
			}
			if err == nil && !allowed {
				// 同一审核轮次可读取的原冲突输入也属于其固定上下文，
				// 但必须仍有有效原冲突收据，不能借现在的主线引用放行。
				allowed, err = s.conflictContains(ctx, tx, target)
			}
		}
		if err == sql.ErrNoRows {
			return facts, domain.Violation(domain.EntityNotFound, "RESOURCE_CONTEXT_NOT_FOUND")
		}
		if err != nil {
			return facts, err
		}
		if !allowed {
			return facts, domain.Violation(domain.PermissionDenied, "RESOURCE_REFERENCE_REQUIRED")
		}
		return facts, nil
	}
	return access.Sources, check, nil
}

func (s *ResourceReading) publicContext(ctx context.Context, actor *Actor, target ResourceTarget) ([]*pb.SourceEpoch, resourceContextCheck, error) {
	facts, err := s.public.facts(ctx, target.Document)
	if err != nil {
		return nil, nil, err
	}
	policy, err := domain.PublicPolicy(facts)
	if err != nil {
		return nil, nil, err
	}
	if !domain.HasCurrentPublication(facts, policy) || facts.Document.CurrentReleaseId == nil || *facts.Document.CurrentReleaseId != domain.TextID(target.Context) {
		return nil, nil, domain.Violation(domain.EntityNotFound, "PUBLIC_RESOURCE_NOT_CURRENT")
	}
	_, sources, err := s.public.audience(ctx, actor, facts, policy)
	if err != nil {
		return nil, nil, err
	}
	check := func(ctx context.Context, tx *sql.Tx) (resourceReadFacts, error) {
		var result resourceReadFacts
		current, err := documents.PublicationFactsRow(ctx, tx, target.Document, true)
		if err != nil {
			return result, err
		}
		if publicFactsBinding(current) != publicFactsBinding(facts) || !domain.HasCurrentPublication(current, policy) {
			return result, domain.Violation(domain.StateConflict, "PUBLIC_DOCUMENT_CHANGED")
		}
		fixed, err := documents.FixedReleaseRow(ctx, tx, target.Document, target.Context)
		if err != nil {
			return result, err
		}
		manifest, err := domain.CanonicalID(fixed.Release.ManifestId)
		if err != nil {
			return result, err
		}
		allowed, err := repository.ManifestContains(ctx, tx, target.Document, manifest, target.Resource)
		if err != nil {
			return result, err
		}
		if !allowed {
			return result, domain.Violation(domain.PermissionDenied, "RESOURCE_REFERENCE_REQUIRED")
		}
		result.space, _ = domain.CanonicalID(current.Space.Id)
		result.epoch, _ = domain.CanonicalID(current.Document.PublicationEpoch)
		result.release = target.Context
		return result, nil
	}
	return sources, check, nil
}

// resource_quota.go：保留原实现，按业务合并文件。

func (s *Resources) PrepareQuota(ctx context.Context, actor Actor, space uint64) ([]*pb.SourceEpoch, ReadBody, error) {
	_, proof, err := s.access.SpaceIdentity(ctx, actor, space, pb.IdentityCapability_ORGANIZATION_MEMBER, "READ_RESOURCE_QUOTA", 0)
	if err != nil {
		return nil, nil, err
	}
	if err := s.access.identity.Revalidate(ctx, proof); err != nil {
		return nil, nil, err
	}
	sources, err := MergeSources(actor.Proof, proof)
	if err != nil {
		return nil, nil, err
	}
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		// 首次读取才物化配额账户，身份来源锁由统一Read事务持有。
		// 返回账户本身的revision，上传预留必须比较同一份余额版本。
		if _, err := documents.SpaceRow(ctx, tx, space, true); err != nil {
			return nil, err
		}
		q, err := repository.LockQuota(ctx, tx, space, s.limits.SpaceBytes)
		if err != nil {
			return nil, err
		}
		return dto.Quota{SpaceId: domain.TextID(q.Space), LimitBytes: domain.TextID(q.Limit), UsedBytes: domain.TextID(q.Used), ReservedBytes: domain.TextID(q.Reserved), Revision: domain.TextID(q.Revision)}, nil
	}
	return sources, run, nil
}

// resource_reading.go：保留原实现，按业务合并文件。

type ResourceTarget struct {
	Resource, Document, Context uint64
	Kind                        string
}

type ResourceReading struct {
	resources *Resources
	public    *PublicReading
	reviews   *Reviews
	outputs   *Outputs
}

func NewResourceReading(resources *Resources, public *PublicReading, reviews *Reviews, outputs *Outputs) *ResourceReading {
	return &ResourceReading{resources: resources, public: public, reviews: reviews, outputs: outputs}
}

type ResourceDownload struct {
	ID, Size  uint64
	MediaType string
	Reader    io.ReadCloser
	release   func()
	once      sync.Once
}

func (d *ResourceDownload) Close() {
	d.once.Do(func() { d.Reader.Close(); d.release() })
}

func (s *ResourceReading) Open(ctx context.Context, actor *Actor, target ResourceTarget, handler string, bind BindOutput) (*ResourceDownload, error) {
	sources, check, err := s.context(ctx, actor, target)
	if err != nil {
		return nil, err
	}
	edge, err := s.outputs.EdgeInstance(ctx, handler)
	if err != nil || edge == nil {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "EDGE_OUTPUT_UNAVAILABLE")
	}
	// 先确认真实上下文引用再读取文件，避免无权请求触发大文件校验。
	// 该短事务不跨文件IO持锁；下面登记输出前仍须重新核对，关闭撤权窗口。
	preflight, err := s.resources.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	if err = LockSources(ctx, preflight, sources); err == nil {
		_, err = check(ctx, preflight)
	}
	preflight.Rollback()
	if err != nil {
		return nil, err
	}
	object, err := repository.ObjectRow(ctx, s.resources.db, target.Resource, false)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.Violation(domain.EntityNotFound, "RESOURCE_NOT_FOUND")
	}
	if err != nil {
		return nil, err
	}
	if object.State != "READY" || object.Size > 32<<20 {
		return nil, domain.Violation(domain.StateConflict, "RESOURCE_NOT_READY")
	}
	release, err := s.resources.files.AcquireRead(ctx, object.Key)
	if err != nil {
		return nil, err
	}
	// 不可变文件完整校验放在数据库事务外，共享文件锁阻止并发删除，但允许多个读者。
	file, err := s.resources.files.Open(ctx, object.Key, object.Size, object.Checksum)
	if err != nil {
		release()
		return nil, err
	}
	download := &ResourceDownload{ID: object.ID, Size: object.Size, MediaType: object.MediaType, Reader: file, release: release}
	accepted := false
	defer func() {
		if !accepted {
			download.Close()
		}
	}()
	if actor != nil {
		if err := s.resources.access.identity.Revalidate(ctx, actor.Proof); err != nil {
			return nil, err
		}
	}
	tx, err := s.resources.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err := LockSources(ctx, tx, sources); err != nil {
		return nil, err
	}
	facts, err := check(ctx, tx)
	if err != nil {
		return nil, err
	}
	current, err := repository.ObjectRow(ctx, tx, target.Resource, true)
	if err != nil {
		return nil, err
	}
	allowed, err := repository.ResourceAllowedForDocument(ctx, tx, target.Resource, target.Document, facts.space)
	if err != nil {
		return nil, err
	}
	if current != object || !allowed || current.State != "READY" {
		return nil, domain.Violation(domain.StateConflict, "RESOURCE_CONTEXT_CHANGED")
	}
	var registration *ResponseRegistration
	if facts.release != 0 {
		registration, err = s.outputs.RegisterPublication(ctx, tx, sources, target.Document, facts.epoch, facts.release, handler, edge)
	} else {
		// 私有文档输出必须有真实准入记录；资源上传者或裸文档ID不能代替准入。
		admission := platform.NewTraceID()
		if err = InsertAdmission(ctx, tx, *actor, sources, target.Document, "READ_RESOURCE", admission); err != nil {
			return nil, err
		}
		registration, err = s.outputs.Register(ctx, tx, sources, target.Document, admission, handler, edge)
	}
	if err != nil {
		return nil, err
	}
	output := bind(registration, sources)
	if err := tx.Commit(); err != nil {
		output.Abort()
		return nil, err
	}
	accepted = true
	return download, nil
}

type resourceReadFacts struct{ space, epoch, release uint64 }
````

UTF-8/LF全文SHA-256：`1f016934539d5354859dbae9795b3843bfd1eed1c96f6f2605ea3dcf99223e0d`。[参考实现](../../../../services/document/internal/logic/document/resources_reading_service.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
