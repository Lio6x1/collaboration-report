# 分步编写 `services/document/internal/worker/content_workflows_workers.go`

先按 [D14 · 搜索资格、快照重建与文档服务总装](../lessons/document/14-search-assembly.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/271-content-workflows-workers.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

定期调度送审、合并、主线控制和草稿/结构恢复。

HTTP成功登记后必须由固定恢复循环继续完成，不能依赖用户重试触发。

写之前先按这条执行路线理解各段的连接：

1. 各Run函数响应context并周期调用对应RecoverOne
2. 草稿预览与确认各自领取原任务。

本文件需要守住的边界：

- 调度层不写SQL或推断ACK
- 不能让一种worker误领另一业务kind。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/worker' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/worker/content_workflows_workers.go') -and (Get-Item -LiteralPath 'services/document/internal/worker/content_workflows_workers.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/worker/content_workflows_workers.go')) { New-Item -ItemType File -Path 'services/document/internal/worker/content_workflows_workers.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:273 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package worker

import (
	logic "collaboration/document/internal/logic/document"
	"context"
	"time"
)

// conflict_resolutions.go：保留原实现，按业务合并文件。

````

## 第 2 步：RunResolutions

插入位置：第 1 步代码之后，紧接追加；完成稿约第 12 行。

本步声明：`func RunResolutions(ctx context.Context, resolutions *logic.Resolutions)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx.Done</code>；<code>resolutions.RecoverOne(ctx)</code> |
| <code>resolutions</code> | <code>*logic.Resolutions</code> | 使用 <code>logic.Resolutions</code> 类型；其定义处的职责：固定修复候选、发起协作准备并验证恢复上下文和来源。<br>在此声明中的实际使用：<code>resolutions.RecoverOne</code> |

**跟着本段代码走一遍：**

- 登记 <code>ticker.Stop()</code>，当前函数退出时执行，提前 return 也会触发。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=273:595 -->
````go
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

````

## 第 3 步：RunDraftControls

插入位置：第 2 步代码之后，紧接追加；完成稿约第 27 行。

本步声明：`func RunDraftControls(ctx context.Context, controls *logic.DraftControls)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx.Done</code>；<code>controls.RecoverOne(ctx)</code>；<code>controls.RecoverDiscardOne(ctx)</code> |
| <code>controls</code> | <code>*logic.DraftControls</code> | 使用 <code>logic.DraftControls</code> 类型；其定义处的职责：为拉取与丢弃草稿提供固定来源装配及准备/安装结果形状。<br>在此声明中的实际使用：<code>controls.RecoverOne</code>；<code>controls.RecoverDiscardOne</code> |

**跟着本段代码走一遍：**

- 登记 <code>ticker.Stop()</code>，当前函数退出时执行，提前 return 也会触发。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=595:949 -->
````go
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

````

## 第 4 步：RunMainControls

插入位置：第 3 步代码之后，紧接追加；完成稿约第 43 行。

本步声明：`func RunMainControls(ctx context.Context, controls *logic.MainControls)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx.Done</code>；<code>controls.RecoverOne(ctx)</code> |
| <code>controls</code> | <code>*logic.MainControls</code> | 使用 <code>logic.MainControls</code> 类型；其定义处的职责：实现锁主线、改样式、新章节等控制动作的捕获、准备、提交与安装恢复。<br>在此声明中的实际使用：<code>controls.RecoverOne</code> |

**跟着本段代码走一遍：**

- 登记 <code>ticker.Stop()</code>，当前函数退出时执行，提前 return 也会触发。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=949:1259 -->
````go
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

````

## 第 5 步：RunMerges

插入位置：第 4 步代码之后，紧接追加；完成稿约第 58 行。

本步声明：`func RunMerges(ctx context.Context, merges *logic.Merges)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx.Done</code>；<code>merges.RecoverOne(ctx)</code> |
| <code>merges</code> | <code>*logic.Merges</code> | 使用 <code>logic.Merges</code> 类型；其定义处的职责：固定合并的审核轮次、base、送审版本与可靠MAIN切面，并校验稳定结构投影。<br>在此声明中的实际使用：<code>merges.RecoverOne</code> |

**跟着本段代码走一遍：**

- 登记 <code>ticker.Stop()</code>，当前函数退出时执行，提前 return 也会触发。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1259:1556 -->
````go
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

````

## 第 6 步：RunPullMain

插入位置：第 5 步代码之后，紧接追加；完成稿约第 73 行。

本步声明：`func RunPullMain(ctx context.Context, pulls *logic.PullMain)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx.Done</code>；<code>pulls.RecoverOne(ctx)</code> |
| <code>pulls</code> | <code>*logic.PullMain</code> | 使用 <code>logic.PullMain</code> 类型；其定义处的职责：将固定MAIN变化拉入现有支线，保留已接纳来源，并可恢复新工作区安装。<br>在此声明中的实际使用：<code>pulls.RecoverOne</code> |

**跟着本段代码走一遍：**

- 登记 <code>ticker.Stop()</code>，当前函数退出时执行，提前 return 也会触发。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1556:1861 -->
````go
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

````

## 第 7 步：RunReverseChanges

插入位置：第 6 步代码之后，紧接追加；完成稿约第 88 行。

本步声明：`func RunReverseChanges(ctx context.Context, changes *logic.ReverseChanges)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx.Done</code>；<code>changes.RecoverOne(ctx)</code> |
| <code>changes</code> | <code>*logic.ReverseChanges</code> | 使用 <code>logic.ReverseChanges</code> 类型；其定义处的职责：把指定合并贡献的反向差异应用到当前支线，形成新审核轮次并恢复安装。<br>在此声明中的实际使用：<code>changes.RecoverOne</code> |

**跟着本段代码走一遍：**

- 登记 <code>ticker.Stop()</code>，当前函数退出时执行，提前 return 也会触发。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=1861:2185 -->
````go
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

````

## 第 8 步：RunStructurePreviews

插入位置：第 7 步代码之后，紧接追加；完成稿约第 103 行。

本步声明：`func RunStructurePreviews(ctx context.Context, previews *logic.StructurePreviews)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx.Done</code>；<code>previews.RecoverOne(ctx)</code> |
| <code>previews</code> | <code>*logic.StructurePreviews</code> | 使用 <code>logic.StructurePreviews</code> 类型；其定义处的职责：捕获拆章合章整份MAIN与草稿候选，记录阻塞原因并支持原号恢复。<br>在此声明中的实际使用：<code>previews.RecoverOne</code> |

**跟着本段代码走一遍：**

- 登记 <code>ticker.Stop()</code>，当前函数退出时执行，提前 return 也会触发。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=2185:2510 -->
````go
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

````

## 第 9 步：RunSubmissions

插入位置：第 8 步代码之后，紧接追加；完成稿约第 118 行。

本步声明：`func RunSubmissions(ctx context.Context, submissions *logic.Submissions)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx.Done</code>；<code>submissions.RecoverOne(ctx)</code> |
| <code>submissions</code> | <code>*logic.Submissions</code> | 使用 <code>logic.Submissions</code> 类型；其定义处的职责：将可靠支线保存切面固定成审核轮次，并恢复捕获最终确认。<br>在此声明中的实际使用：<code>submissions.RecoverOne</code> |

**跟着本段代码走一遍：**

- 登记 <code>ticker.Stop()</code>，当前函数退出时执行，提前 return 也会触发。
- 此处通过 <code>return</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=2510:2764 -->
````go
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

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/worker/content_workflows_workers.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/14-search-assembly.md)。
