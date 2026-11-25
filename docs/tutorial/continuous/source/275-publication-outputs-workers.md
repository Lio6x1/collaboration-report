# services/document/internal/worker/publication_outputs_workers.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/275-publication-outputs-workers.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：领取持久任务并恢复原操作；控制并发、进程取消与关闭。

主要声明：`exportQueue`、`RunExports`、`runExportWorker`、`RunPublications`、`RunResources`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package worker

import (
	logic "collaboration/document/internal/logic/document"
	"context"
	"log/slog"
	"regexp"
	"sync"
	"time"
)

// exports.go：保留原实现，按业务合并文件。

type exportQueue interface {
	WorkerCount() int
	Recover(context.Context) error
}

// 固定worker自行拉取持久任务，不为每次HTTP导出创建无界goroutine。
// 停止时等待取消传播到转换器，不能只退出调度器而留下后台转换。
func RunExports(ctx context.Context, exports exportQueue) {
	slog.Info("export_pool_started", "workers", exports.WorkerCount())
	var workers sync.WaitGroup
	for range exports.WorkerCount() {
		workers.Go(func() { runExportWorker(ctx, exports) })
	}
	workers.Wait()
}

func runExportWorker(ctx context.Context, exports exportQueue) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		// 格式转换有独立有限预算，不能占用普通HTTP原三秒期限。
		attempt, cancel := context.WithTimeout(ctx, 75*time.Second)
		err := exports.Recover(attempt)
		cancel()
		if err != nil && ctx.Err() == nil {
			// 仅记录固定错误码，SQL原文、私密路径和转换器stderr不能进入运行日志。
			code := "EXPORT_RECOVERY_FAILED"
			if exportErrorCode.MatchString(err.Error()) {
				code = err.Error()
			}
			slog.Warn("EXPORT_RECOVERY_RETRY", "code", code)
		}
	}
}

var exportErrorCode = regexp.MustCompile(`^[A-Z][A-Z0-9_]{1,63}$`)

// publications.go：保留原实现，按业务合并文件。

func RunPublications(ctx context.Context, publications *logic.Publications) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			publications.RecoverOne(ctx)
		}
	}
}

// resources.go：保留原实现，按业务合并文件。

func RunResources(ctx context.Context, resources *logic.Resources) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			attempt, cancel := context.WithTimeout(ctx, 3*time.Second)
			err := resources.RecoverUpload(attempt)
			cancel()
			if err != nil && ctx.Err() == nil {
				// 错误保留持久DELETING/预留，不把文件删除未知当作已释放。
				slog.Warn("RESOURCE_UPLOAD_RECOVERY_RETRY")
			}
			// 两条持久恢复链各有自己的预算；坏上传不能阻止无引用READY对象回收。
			attempt, cancel = context.WithTimeout(ctx, 3*time.Second)
			err = resources.RecoverReady(attempt)
			cancel()
			if err != nil && ctx.Err() == nil {
				slog.Warn("RESOURCE_READY_RECOVERY_RETRY")
			}
		}
	}
}
````

UTF-8/LF全文SHA-256：`226895641c18d49e1b9fa4585a54cc638fcfb436cf0f75067ff6c84a0f8889cc`。[参考实现](../../../../services/document/internal/worker/publication_outputs_workers.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
