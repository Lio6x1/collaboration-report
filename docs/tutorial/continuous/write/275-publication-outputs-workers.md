# 分步编写 `services/document/internal/worker/publication_outputs_workers.go`

先按 [D14 · 搜索资格、快照重建与文档服务总装](../lessons/document/14-search-assembly.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/275-publication-outputs-workers.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

以固定worker数量处理导出，并调度发布和资源回收。

每次HTTP起goroutine会造成无界转换进程，退出调度器也不能留下转换器继续写。

写之前先按这条执行路线理解各段的连接：

1. RunExports根据WorkerCount启动有限worker并等待退出
2. runExportWorker调用Recover
3. RunPublications推进发布
4. RunResources分别恢复上传与READY回收。

本文件需要守住的边界：

- 取消必须传播到实际进程树并等待完成
- 资源候选仍需业务持锁重验。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/worker' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/worker/publication_outputs_workers.go') -and (Get-Item -LiteralPath 'services/document/internal/worker/publication_outputs_workers.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/worker/publication_outputs_workers.go')) { New-Item -ItemType File -Path 'services/document/internal/worker/publication_outputs_workers.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:290 -->
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

````

## 第 2 步：exportQueue

插入位置：第 1 步代码之后，紧接追加；完成稿约第 15 行。

本步声明：`type exportQueue interface {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>WorkerCount</code> | <code>() int</code> | 返回配置的有限导出worker数量，不根据用户点击次数无界启动转换。 |
| <code>Recover</code> | <code>(context.Context) error</code> | 以服务ctx领取并推进一个原导出任务，失败留给持久队列重试；进程退出取消由ctx传递。 |

<!-- write-step: 2 bytes=290:373 -->
````go
type exportQueue interface {
	WorkerCount() int
	Recover(context.Context) error
}

````

## 第 3 步：RunExports

插入位置：第 2 步代码之后，紧接追加；完成稿约第 20 行。

固定worker自行拉取持久任务，不为每次HTTP导出创建无界goroutine。
停止时等待取消传播到转换器，不能只退出调度器而留下后台转换。

本步声明：`func RunExports(ctx context.Context, exports exportQueue)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>runExportWorker(ctx, exports)</code> |
| <code>exports</code> | <code>exportQueue</code> | 在此声明中的实际使用：<code>exports.WorkerCount</code>；<code>runExportWorker(ctx, exports)</code> |

<!-- write-step: 3 bytes=373:824 -->
````go
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

````

## 第 4 步：runExportWorker

插入位置：第 3 步代码之后，紧接追加；完成稿约第 31 行。

本步声明：`func runExportWorker(ctx context.Context, exports exportQueue)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx.Done</code>；<code>context.WithTimeout(ctx, 75*time.Second)</code>；<code>ctx.Err</code> |
| <code>exports</code> | <code>exportQueue</code> | 在此声明中的实际使用：<code>exports.Recover</code> |

**跟着本段代码走一遍：**

- 登记 <code>ticker.Stop()</code>，当前函数退出时执行，提前 return 也会触发。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>context.WithTimeout(ctx, 75*time.Second)</code>。
- 检查 <code>err != nil &amp;&amp; ctx.Err() == nil</code>。
- 检查 <code>exportErrorCode.MatchString(err.Error())</code>。

<!-- write-step: 4 bytes=824:1526 -->
````go
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

````

## 第 5 步：exportErrorCode

插入位置：第 4 步代码之后，紧接追加；完成稿约第 55 行。

本步声明：`var exportErrorCode = regexp.MustCompile(\`^[A-Z][A-Z0-9_]{1,63}$\`)`。

<!-- write-step: 5 bytes=1526:1659 -->
````go
var exportErrorCode = regexp.MustCompile(`^[A-Z][A-Z0-9_]{1,63}$`)

// publications.go：保留原实现，按业务合并文件。

````

## 第 6 步：RunPublications

插入位置：第 5 步代码之后，紧接追加；完成稿约第 59 行。

本步声明：`func RunPublications(ctx context.Context, publications *logic.Publications)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx.Done</code>；<code>publications.RecoverOne(ctx)</code> |
| <code>publications</code> | <code>*logic.Publications</code> | 使用 <code>logic.Publications</code> 类型；其定义处的职责：创建正式版本、切历史正式版或撤销发布，并恢复可靠MAIN捕获。<br>在此声明中的实际使用：<code>publications.RecoverOne</code> |

**跟着本段代码走一遍：**

- 登记 <code>ticker.Stop()</code>，当前函数退出时执行，提前 return 也会触发。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1659:1980 -->
````go
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

````

## 第 7 步：RunResources

插入位置：第 6 步代码之后，紧接追加；完成稿约第 74 行。

本步声明：`func RunResources(ctx context.Context, resources *logic.Resources)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx.Done</code>；<code>context.WithTimeout(ctx, 3*time.Second)</code>；<code>ctx.Err</code> |
| <code>resources</code> | <code>*logic.Resources</code> | 使用 <code>logic.Resources</code> 类型；其定义处的职责：编排上传预留、流式写入、完成结算和未引用对象回收。<br>在此声明中的实际使用：<code>resources.RecoverUpload</code>；<code>resources.RecoverReady</code> |

**跟着本段代码走一遍：**

- 登记 <code>ticker.Stop()</code>，当前函数退出时执行，提前 return 也会触发。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>context.WithTimeout(ctx, 3*time.Second)</code>。
- 检查 <code>err != nil &amp;&amp; ctx.Err() == nil</code>。
- 执行 <code>context.WithTimeout(ctx, 3*time.Second)</code>。
- 检查 <code>err != nil &amp;&amp; ctx.Err() == nil</code>。

<!-- write-step: 7 bytes=1980:2792 -->
````go
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

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/worker/publication_outputs_workers.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/14-search-assembly.md)。
