package document

import (
	codec "collaboration/contracts/content"
	"collaboration/platform"
	"context"
	"os"
	"testing"
)

func TestActualSubmissionRecoveryBarrier(t *testing.T) {
	file := os.Getenv("COLLAB_TEST_DOCUMENT_CONFIG")
	if file == "" {
		t.Skip("需要显式指定本项目隔离文档数据库")
	}
	cfg, err := platform.LoadConfig(file)
	if err != nil {
		t.Fatal(err)
	}
	store, err := platform.OpenStore(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer store.DB.Close()
	submissions := NewSubmissions(store.DB, nil, nil, nil, nil, "readiness-test")
	reviews := NewReviews(store.DB, nil, nil, nil, nil)
	merges := NewMerges(store.DB, nil, nil, nil, nil, "readiness-test")
	controls := NewMainControls(store.DB, nil, nil, nil, nil, "readiness-test")
	pulls := NewPullMain(store.DB, nil, nil, nil, nil, "readiness-test")
	previews := NewStructurePreviews(store.DB, nil, nil, nil, nil, "readiness-test")
	drafts := NewDraftControls(store.DB, nil, nil, nil, nil, "readiness-test")
	validator, err := codec.NewValidator()
	if err != nil {
		t.Fatal(err)
	}
	reverse := NewReverseChanges(store.DB, nil, nil, nil, validator, "readiness-test")
	resolutions := NewResolutions(store.DB, nil, nil, nil, validator, "readiness-test")
	store.ManagedTasks = []string{"SUBMISSION_RECOVER", "CAPTURE_FINALIZE", "REVIEW_NOTIFICATION", "REVIEW_ACTION_NOTIFICATION", "MERGE_RECOVER", "MAIN_RECONCILE", "MERGE_NOTIFICATION", "MAIN_CONTROL_RECOVER", "MAIN_CONTROL_RECONCILE", "PULL_MAIN_RECOVER", "PULL_MAIN_RECONCILE", "STRUCTURE_PREVIEW_RECOVER", "DISCARD_PREVIEW_RECOVER", "DISCARD_DRAFT_RECOVER", "DISCARD_DRAFT_RECONCILE", "REVERSE_RECOVER", "REVERSE_RECONCILE", "RESOLUTION_RECOVER", "RESOLUTION_RECONCILE"}
	store.ReadyExtra = func(ctx context.Context) error {
		if err := submissions.Ready(ctx); err != nil {
			return err
		}
		if err := reviews.Ready(ctx); err != nil {
			return err
		}
		if err := merges.Ready(ctx); err != nil {
			return err
		}
		if err := controls.Ready(ctx); err != nil {
			return err
		}
		if err := pulls.Ready(ctx); err != nil {
			return err
		}
		if err := previews.Ready(ctx); err != nil {
			return err
		}
		if err := drafts.Ready(ctx); err != nil {
			return err
		}
		if err := reverse.Ready(ctx); err != nil {
			return err
		}
		return resolutions.Ready(ctx)
	}
	if err = submissions.Ready(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err = store.CheckReady(context.Background()); err != nil {
		t.Fatal(err)
	}
}
