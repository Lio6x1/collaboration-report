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
