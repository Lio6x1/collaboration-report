// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package worker

import (
	logic "collaboration/document/internal/logic/document"
	"context"
	"time"
)

// conflict_resolutions.go：保留原实现，按业务合并文件。

func RunResolutions(ctx context.Context, resolutions *logic.Resolutions) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			resolutions.RecoverOne(ctx)
		}
	}
}

// draft_controls.go：保留原实现，按业务合并文件。

func RunDraftControls(ctx context.Context, controls *logic.DraftControls) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			controls.RecoverOne(ctx)
			controls.RecoverDiscardOne(ctx)
		}
	}
}

// main_controls.go：保留原实现，按业务合并文件。

func RunMainControls(ctx context.Context, controls *logic.MainControls) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			controls.RecoverOne(ctx)
		}
	}
}

// merges.go：保留原实现，按业务合并文件。

func RunMerges(ctx context.Context, merges *logic.Merges) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			merges.RecoverOne(ctx)
		}
	}
}

// pull_main.go：保留原实现，按业务合并文件。

func RunPullMain(ctx context.Context, pulls *logic.PullMain) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pulls.RecoverOne(ctx)
		}
	}
}

// reverse_changes.go：保留原实现，按业务合并文件。

func RunReverseChanges(ctx context.Context, changes *logic.ReverseChanges) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			changes.RecoverOne(ctx)
		}
	}
}

// structure_previews.go：保留原实现，按业务合并文件。

func RunStructurePreviews(ctx context.Context, previews *logic.StructurePreviews) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			previews.RecoverOne(ctx)
		}
	}
}

// submissions.go：保留原实现，按业务合并文件。

func RunSubmissions(ctx context.Context, submissions *logic.Submissions) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			submissions.RecoverOne(ctx)
		}
	}
}
