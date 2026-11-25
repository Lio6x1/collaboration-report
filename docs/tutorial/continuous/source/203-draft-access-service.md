# services/document/internal/logic/document/draft_access_service.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/203-draft-access-service.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：编排用例、权限、事务及未知结果恢复；跨服务调用不能替代持久事实。

主要声明：`DraftControlClient`、`DraftControls`、`discardPreviewContext`、`NewDraftControls`、`discardPreviewRequest`、`draftSourceScope`、`draftMainEpoch`、`readDraftSource`、`draftPrepared`、`draftInstall`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	codec "collaboration/contracts/content"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"context"
	"database/sql"
)

// draft_controls.go：保留原实现，按业务合并文件。

type DraftControlClient interface {
	Preview(context.Context, *pb.DraftPreviewRequest) (*pb.DraftPreviewReply, error)
	FixedDraft(context.Context, *pb.DraftPreviewRequest) ([]byte, error)
	Prepare(context.Context, any) ([]byte, error)
	Apply(context.Context, *pb.DraftResolutionRequest) (*pb.OperationResult, error)
}

type DraftControls struct {
	db        *sql.DB
	identity  *Identity
	access    *Access
	client    DraftControlClient
	validator *codec.Validator
	instance  string
}

// 原上下文与恢复进度分开保存；预览不取得支线控制占位，也不伪造送审操作/FLUSH。
type discardPreviewContext struct {
	Actor          uint64
	SessionExpires int64
	Intent         domain.DiscardPreviewIntent
	Access         DocumentAccess
	Context        *pb.OperationContext
	Scope          draftSourceScope
}

func NewDraftControls(db *sql.DB, identity *Identity, access *Access, client DraftControlClient, validator *codec.Validator, instance string) *DraftControls {
	return &DraftControls{db: db, identity: identity, access: access, client: client, validator: validator, instance: instance}
}

func discardPreviewRequest(fixed discardPreviewContext) *pb.DraftPreviewRequest {
	return &pb.DraftPreviewRequest{Context: fixed.Context, ExpectedBranchId: fixed.Scope.Workspace.Workspace, ExpectedLineageId: fixed.Scope.Workspace.Lineage, ExpectedStructureId: fixed.Scope.Structure}
}

// draft_source.go：保留原实现，按业务合并文件。

// 每个用例先核验自己的授权与占位规则。正文装配只使用该调用方TX，不能隐式跨池重新取快照。
type draftSourceScope struct {
	Document, Unit, Room, Manifest, Structure, Style, Control uint64
	MainEpoch                                                 uint64 `json:"main_epoch,omitempty"`
	Workspace                                                 domain.EditWorkspace
}

func draftMainEpoch(fixed draftSourceScope) uint64 {
	if fixed.MainEpoch != 0 {
		return fixed.MainEpoch
	}
	// 已持久旧上下文的缺省字段对应当时原control，不按当前Doc控制修订重新解释。
	return fixed.Control
}

func readDraftSource(ctx context.Context, tx *sql.Tx, fixed draftSourceScope) (map[string]any, error) {
	epoch, err := repository.MainWorkspaceEpoch(ctx, tx, fixed.Document)
	if err != nil {
		return nil, err
	}
	if epoch != draftMainEpoch(fixed) || epoch > fixed.Control {
		return nil, domain.Violation(domain.StateConflict, "MAIN_SOURCE_CHANGED")
	}
	baseline, err := repository.VersionContent(ctx, tx, fixed.Workspace.BaseVersion, fixed.Document)
	if err != nil {
		return nil, err
	}
	if err = canonicalVersion(baseline, fixed.Workspace.BaseVersion); err != nil {
		return nil, err
	}
	_, _, provenance, err := repository.BranchBootstrapLineage(ctx, tx, fixed.Workspace.Workspace)
	if err != nil {
		return nil, err
	}
	branchBaseline, err := domain.BranchBaselineBody(baseline.BodyJsonUtf8)
	if err != nil {
		return nil, err
	}
	body, err := codec.Decode(branchBaseline)
	if err != nil {
		return nil, err
	}
	sources, err := codec.Decode(provenance)
	if err != nil {
		return nil, err
	}
	version, err := repository.MainUnitVersion(ctx, tx, fixed.Document, fixed.Unit, fixed.Manifest)
	if err != nil {
		return nil, err
	}
	main, err := repository.VersionContent(ctx, tx, version, fixed.Document)
	if err != nil {
		return nil, err
	}
	if err = canonicalVersion(main, version); err != nil {
		return nil, err
	}
	main.WorkspaceEpoch = fixed.Control
	wire, err := fixedWire(main)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"room_id":          domain.TextID(fixed.Room),
		"main_manifest_id": domain.TextID(fixed.Manifest), "main_version_id": domain.TextID(version),
		"structure_id": domain.TextID(fixed.Structure), "style_id": domain.TextID(fixed.Style),
		"control_revision": domain.TextID(fixed.Control), "workspace_epoch": domain.TextID(epoch),
		"main": wire,
		"lineage": map[string]any{
			"lineage_id":       domain.TextID(fixed.Workspace.Lineage),
			"base_manifest_id": domain.TextID(fixed.Workspace.BaseManifest), "base_version_id": domain.TextID(fixed.Workspace.BaseVersion),
			"body": body, "provenance": sources,
		},
	}, nil
}

// draft_transition_protocol.go：保留原实现，按业务合并文件。

// 拉取与丢弃共用“固定候选→准备→安装”的私有事实形状；业务资格和最终数据库动作各自编排。
// 保留已有JSON字段，类型整理不会改历史PULL或DISCARD原固定字节。
type draftPrepared struct {
	State           string            `json:"state"`
	Code            string            `json:"code"`
	Preview         string            `json:"preview_cut_ref"`
	Digest          []byte            `json:"input_digest"`
	Cut             map[string]string `json:"draft_cut"`
	Candidate, Main *pb.FixedContent
}

type draftInstall struct {
	Context                                  *pb.OperationContext `json:"context"`
	Branch, OldRoom, NewRoom, Epoch, Lineage uint64
	Preview                                  string           `json:"preview"`
	Digest                                   []byte           `json:"digest"`
	Content                                  *pb.FixedContent `json:"content"`
	LineageContext                           map[string]any   `json:"lineage_context"`
}
````

UTF-8/LF全文SHA-256：`cd90496f9bc024e023068feea2e58e5ab35463d5f9101d4a3138d17890607026`。[参考实现](../../../../services/document/internal/logic/document/draft_access_service.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
