# services/document/internal/worker/content_workflows_workers.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/271-content-workflows-workers.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：领取持久任务并恢复原操作；控制并发、进程取消与关闭。

主要声明：`RunResolutions`、`RunDraftControls`、`RunMainControls`、`RunMerges`、`RunPullMain`、`RunReverseChanges`、`RunStructurePreviews`、`RunSubmissions`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
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
````

UTF-8/LF全文SHA-256：`e7e4693bd4a1562b3b20e9c7f44732e85b5143c51704be8da2df02c3538a2d9b`。[参考实现](../../../../services/document/internal/worker/content_workflows_workers.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
