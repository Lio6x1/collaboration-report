# services/document/internal/svc/content_workflows_wiring.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/276-content-workflows-wiring.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：创建依赖、注入构造函数；同一私有Store贯穿本服务。

主要声明：`NewResolutions`、`NewDraftControls`、`NewDraftResources`、`NewMainControls`、`NewMainReadViews`、`NewMerges`、`NewPullMain`、`NewReverseChanges`、`NewReviews`、`NewStructurePreviews`、`NewSubmissions`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package svc

import (
	codec "collaboration/contracts/content"
	"collaboration/document/internal/client"
	logic "collaboration/document/internal/logic/document"
	"collaboration/platform"
	"net/http"
)

// conflict_resolutions.go：保留原实现，按业务合并文件。

func NewResolutions(store *platform.Store, cfg platform.Config, identity *logic.Identity, access *logic.Access, outputs *logic.Outputs) (*logic.Resolutions, error) {
	validator, err := codec.NewValidator()
	if err != nil {
		return nil, err
	}
	peer, err := platform.HTTPPeer(cfg, "collaboration")
	if err != nil {
		return nil, err
	}
	return logic.NewResolutions(store.DB, identity, access, client.NewDraftTransitions(cfg, peer), validator, outputs.Self().ID), nil
}

// draft_controls.go：保留原实现，按业务合并文件。

func NewDraftControls(store *platform.Store, cfg platform.Config, identity *logic.Identity, access *logic.Access, outputs *logic.Outputs) (*logic.DraftControls, error) {
	validator, err := codec.NewValidator()
	if err != nil {
		return nil, err
	}
	peer, err := platform.HTTPPeer(cfg, "collaboration")
	if err != nil {
		return nil, err
	}
	return logic.NewDraftControls(store.DB, identity, access, client.NewDraftTransitions(cfg, peer), validator, outputs.Self().ID), nil
}

// draft_resources.go：保留原实现，按业务合并文件。

func NewDraftResources(store *platform.Store, cfg platform.Config, peer *http.Client) *logic.DraftResources {
	return logic.NewDraftResources(store.DB, client.NewNodePersistence(peer, cfg.Peers["collaborationHttp"]))
}

// main_controls.go：保留原实现，按业务合并文件。

func NewMainControls(store *platform.Store, cfg platform.Config, identity *logic.Identity, access *logic.Access, outputs *logic.Outputs) (*logic.MainControls, error) {
	validator, err := codec.NewValidator()
	if err != nil {
		return nil, err
	}
	peer, err := platform.HTTPPeer(cfg, "collaboration")
	if err != nil {
		return nil, err
	}
	return logic.NewMainControls(store.DB, identity, access, client.NewMerges(cfg, peer), validator, outputs.Self().ID), nil
}

// main_read.go：保留原实现，按业务合并文件。

func NewMainReadViews(store *platform.Store, cfg platform.Config, identity *logic.Identity) (*logic.MainReadViews, error) {
	validator, err := codec.NewValidator()
	if err != nil {
		return nil, err
	}
	peer, err := platform.HTTPPeer(cfg, "collaboration")
	if err != nil {
		return nil, err
	}
	return logic.NewMainReadViews(store.DB, identity, client.NewMerges(cfg, peer), validator), nil
}

// merges.go：保留原实现，按业务合并文件。

func NewMerges(store *platform.Store, cfg platform.Config, identity *logic.Identity, access *logic.Access, outputs *logic.Outputs) (*logic.Merges, error) {
	validator, err := codec.NewValidator()
	if err != nil {
		return nil, err
	}
	peer, err := platform.HTTPPeer(cfg, "collaboration")
	if err != nil {
		return nil, err
	}
	return logic.NewMerges(store.DB, identity, access, client.NewMerges(cfg, peer), validator, outputs.Self().ID), nil
}

// pull_main.go：保留原实现，按业务合并文件。

func NewPullMain(store *platform.Store, cfg platform.Config, identity *logic.Identity, access *logic.Access, outputs *logic.Outputs) (*logic.PullMain, error) {
	validator, err := codec.NewValidator()
	if err != nil {
		return nil, err
	}
	peer, err := platform.HTTPPeer(cfg, "collaboration")
	if err != nil {
		return nil, err
	}
	return logic.NewPullMain(store.DB, identity, access, client.NewDraftTransitions(cfg, peer), validator, outputs.Self().ID), nil
}

// reverse_changes.go：保留原实现，按业务合并文件。

func NewReverseChanges(store *platform.Store, cfg platform.Config, identity *logic.Identity, access *logic.Access, outputs *logic.Outputs) (*logic.ReverseChanges, error) {
	validator, err := codec.NewValidator()
	if err != nil {
		return nil, err
	}
	peer, err := platform.HTTPPeer(cfg, "collaboration")
	if err != nil {
		return nil, err
	}
	return logic.NewReverseChanges(store.DB, identity, access, client.NewDraftTransitions(cfg, peer), validator, outputs.Self().ID), nil
}

// reviews.go：保留原实现，按业务合并文件。

func NewReviews(store *platform.Store, access *logic.Access, pagination *logic.Pagination, directory logic.DirectoryReader) (*logic.Reviews, error) {
	validator, err := codec.NewValidator()
	if err != nil {
		return nil, err
	}
	return logic.NewReviews(store.DB, access, pagination, directory, validator), nil
}

// structure_previews.go：保留原实现，按业务合并文件。

func NewStructurePreviews(store *platform.Store, cfg platform.Config, identity *logic.Identity, access *logic.Access, outputs *logic.Outputs) (*logic.StructurePreviews, error) {
	validator, err := codec.NewValidator()
	if err != nil {
		return nil, err
	}
	peer, err := platform.HTTPPeer(cfg, "collaboration")
	if err != nil {
		return nil, err
	}
	return logic.NewStructurePreviews(store.DB, identity, access, client.NewDraftTransitions(cfg, peer), validator, outputs.Self().ID), nil
}

// submissions.go：保留原实现，按业务合并文件。

func NewSubmissions(store *platform.Store, cfg platform.Config, identity *logic.Identity, access *logic.Access, outputs *logic.Outputs) (*logic.Submissions, error) {
	validator, err := codec.NewValidator()
	if err != nil {
		return nil, err
	}
	return logic.NewSubmissions(store.DB, identity, access, client.NewSubmissionCaptures(cfg), validator, outputs.Self().ID), nil
}
````

UTF-8/LF全文SHA-256：`e30f98c5c2859cab377816826022eabc420f591837b34f2ef84d9efed5be99ee`。[参考实现](../../../../services/document/internal/svc/content_workflows_wiring.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
