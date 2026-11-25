# 分步编写 `services/document/internal/logic/document/resource_conflict_context.go`

先按 [D08 · 资源上传、配额、保护与真实对象存储](../lessons/document/08-resources-storage.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/225-resource-conflict-context.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

让审核上下文资源下载覆盖原冲突中已物化的真实版本。

冲突可能需要base/submitted/MAIN图片，但不能因此开放同轮次任意历史附件。

写之前先按这条执行路线理解各段的连接：

1. conflictContains在原REVIEW资格通过后定位冲突operation，核原attempt与固定版本，再检查资源实际引用。

本文件需要守住的边界：

- 这里只读取已物化版本，不在下载中隐式创建冲突快照
- 候选索引不是授权证据。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/logic/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/logic/document/resource_conflict_context.go') -and (Get-Item -LiteralPath 'services/document/internal/logic/document/resource_conflict_context.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/logic/document/resource_conflict_context.go')) { New-Item -ItemType File -Path 'services/document/internal/logic/document/resource_conflict_context.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:245 -->
````go
package document

import (
	domain "collaboration/document/internal/domain/document"
	documents "collaboration/document/internal/repository/document"
	resources "collaboration/document/internal/repository/resource"
	"context"
	"database/sql"
)

````

## 第 2 步：ResourceReading.conflictContains

插入位置：第 1 步代码之后，紧接追加；完成稿约第 11 行。

conflictContains 在 REVIEW 原轮次权限已核验后扩展原冲突输入的资源集合。
只读取冲突页面已物化的版本，不隐式创建快照；候选索引不能代替原固定事实证明。

本步声明：`func (s *ResourceReading) conflictContains(ctx context.Context, tx *sql.Tx, target ResourceTarget) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>resources.ConflictResourceOperation(ctx, tx, target.Document, target.Context, target.Resource)</code>；<code>documents.MergeRow(ctx, tx, operation)</code>；<code>s.reviews.conflictOriginal(ctx, tx, fact)</code>；<code>documents.MergeArtifact(ctx, tx, &quot;CONFLICT_CONTEXT_VERSIONS&quot;, attemptKey(fact.Operation, attempt.ID))</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>resources.ConflictResourceOperation(ctx, tx, target.Document, target.Context, target.Resource)</code>；<code>documents.MergeRow(ctx, tx, operation)</code>；<code>s.reviews.conflictOriginal(ctx, tx, fact)</code>；<code>documents.MergeArtifact(ctx, tx, &quot;CONFLICT_CONTEXT_VERSIONS&quot;, attemptKey(fact.Operation, attempt.ID))</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>target</code> | <code>ResourceTarget</code> | 使用 <code>ResourceTarget</code> 类型；其定义处的职责：按PUBLIC、MAIN、REVIEW或BRANCH实际引用打开资源，并提供空间配额读取。<br>在此声明中的实际使用：<code>target.Document</code>；<code>target.Context</code>；<code>target.Resource</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;CONFLICT_INPUTS_UNKNOWN&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err == sql.ErrNoRows</code>。条件成立时执行 <code>return false, nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 检查 <code>fact.Document != target.Document &#124;&#124; fact.Round != target.Context</code>。条件成立时执行 <code>return false, domain.Violation(domain.TemporarilyUnavailable, &quot;CONFLICT_INPUTS_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 检查 <code>found</code>。条件成立时执行 <code>return true, nil</code>。当前函数在这里结束。
- 此处通过 <code>return false, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=245:1788 -->
````go
// conflictContains 在 REVIEW 原轮次权限已核验后扩展原冲突输入的资源集合。
// 只读取冲突页面已物化的版本，不隐式创建快照；候选索引不能代替原固定事实证明。
func (s *ResourceReading) conflictContains(ctx context.Context, tx *sql.Tx, target ResourceTarget) (bool, error) {
	operation, err := resources.ConflictResourceOperation(ctx, tx, target.Document, target.Context, target.Resource)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	fact, err := documents.MergeRow(ctx, tx, operation)
	if err != nil {
		return false, err
	}
	if fact.Document != target.Document || fact.Round != target.Context {
		return false, domain.Violation(domain.TemporarilyUnavailable, "CONFLICT_INPUTS_UNKNOWN")
	}
	attempt, fixed, _, err := s.reviews.conflictOriginal(ctx, tx, fact)
	if err != nil {
		return false, err
	}
	raw, err := documents.MergeArtifact(ctx, tx, "CONFLICT_CONTEXT_VERSIONS", attemptKey(fact.Operation, attempt.ID))
	if err != nil {
		return false, err
	}
	versions, err := validateConflictVersions(ctx, tx, raw, fixed)
	if err != nil {
		return false, err
	}
	// 三份版本已逐项比对原固定输入，仍以真实 CONTENT 引用确认目标资源。
	for _, version := range []uint64{versions.Base, versions.Main, versions.Submitted} {
		found, err := resources.VersionContains(ctx, tx, target.Document, version, target.Resource)
		if err != nil {
			return false, err
		}
		if found {
			return true, nil
		}
	}
	return false, nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/logic/document/resource_conflict_context.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/08-resources-storage.md)。
