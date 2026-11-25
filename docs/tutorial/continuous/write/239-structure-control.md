# 分步编写 `services/document/internal/logic/document/structure_control.go`

先按 [D04 · 结构预览、拆章、合章与历史范围](../lessons/document/04-structure.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/239-structure-control.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

实现无需正文范围迁移的稳定目录控制。

改标题或移动完整章组只应追加新结构，不能影响原单元身份与未审核草稿。

写之前先按这条执行路线理解各段的连接：

1. stableControlledEntries先核expected structure再执行纯变换
2. prepareStableControlledLayout追加结构修订并返回新布局。

本文件需要守住的边界：

- 输入结构与当前不一致必须报STRUCTURE_CHANGED
- 不可写穿旧历史条目。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/logic/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/logic/document/structure_control.go') -and (Get-Item -LiteralPath 'services/document/internal/logic/document/structure_control.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/logic/document/structure_control.go')) { New-Item -ItemType File -Path 'services/document/internal/logic/document/structure_control.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:220 -->
````go
package document

import (
	dto "collaboration/contracts/httpdto"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"context"
	"database/sql"
)

````

## 第 2 步：stableControlledEntries

插入位置：第 1 步代码之后，紧接追加；完成稿约第 11 行。

本步声明：`func stableControlledEntries(ctx context.Context, tx *sql.Tx, current dto.Document, input dto.StructureChange) ([]dto.StructureEntry, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.StructureRow(ctx, tx, current.CurrentStructureId, current.Id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.StructureRow(ctx, tx, current.CurrentStructureId, current.Id)</code> |
| <code>current</code> | <code>dto.Document</code> | 在此声明中的实际使用：<code>current.CurrentStructureId</code>；<code>current.Id</code> |
| <code>input</code> | <code>dto.StructureChange</code> | 在此声明中的实际使用：<code>input.ExpectedStructureId</code>；<code>domain.ApplyStableStructureChanges(old.Entries, input)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;dto.StructureEntry</code> | 由 <code>domain.ApplyStableStructureChanges(old.Entries, input)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>domain.ApplyStableStructureChanges(old.Entries, input)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>domain.Violation(domain.StateConflict, &quot;STRUCTURE_CHANGED&quot;)</code>；<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>current.CurrentStructureId != input.ExpectedStructureId</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;STRUCTURE_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return domain.ApplyStableStructureChanges(old.Entries, input)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=220:692 -->
````go
func stableControlledEntries(ctx context.Context, tx *sql.Tx, current dto.Document, input dto.StructureChange) ([]dto.StructureEntry, error) {
	if current.CurrentStructureId != input.ExpectedStructureId {
		return nil, domain.Violation(domain.StateConflict, "STRUCTURE_CHANGED")
	}
	old, err := repository.StructureRow(ctx, tx, current.CurrentStructureId, current.Id)
	if err != nil {
		return nil, err
	}
	return domain.ApplyStableStructureChanges(old.Entries, input)
}

````

## 第 3 步：prepareStableControlledLayout

插入位置：第 2 步代码之后，紧接追加；完成稿约第 22 行。

本步声明：`func prepareStableControlledLayout(ctx context.Context, tx *sql.Tx, current dto.Document, fixed mainControlContext) (mainControlLayout, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>stableControlledEntries(ctx, tx, current, *fixed.Intent.Structure)</code>；<code>repository.InsertControlledStructure(ctx, tx, fixed.Document, fixed.Structure, fixed.Intent.Operation)</code>；<code>repository.InsertStructureEntries(ctx, tx, fixed.Document, layout.Structure, entries)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>stableControlledEntries(ctx, tx, current, *fixed.Intent.Structure)</code>；<code>repository.InsertControlledStructure(ctx, tx, fixed.Document, fixed.Structure, fixed.Intent.Operation)</code>；<code>repository.InsertStructureEntries(ctx, tx, fixed.Document, layout.Structure, entries)</code> |
| <code>current</code> | <code>dto.Document</code> | 在此声明中的实际使用：<code>stableControlledEntries(ctx, tx, current, *fixed.Intent.Structure)</code> |
| <code>fixed</code> | <code>mainControlContext</code> | 在此声明中的实际使用：<code>fixed.Structure</code>；<code>fixed.Intent</code>；<code>fixed.Document</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>mainControlLayout</code> | 实际返回表达式：<code>layout</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.TemporarilyUnavailable, &quot;STRUCTURE_INTENT_UNKNOWN&quot;)</code>；<code>err</code>；<code>repository.InsertStructureEntries(ctx, tx, fixed.Document, layout.Structure, entries)</code> |

**跟着本段代码走一遍：**

- 检查 <code>fixed.Intent.Structure == nil</code>。条件成立时执行 <code>return layout, domain.Violation(domain.TemporarilyUnavailable, &quot;STRUCTURE_INTENT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return layout, err</code>。当前函数在这里结束。
- 此处通过 <code>return layout, repository.InsertStructureEntries(ctx, tx, fixed.Document, layout.Structure, entries)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=692:1636 -->
````go
func prepareStableControlledLayout(ctx context.Context, tx *sql.Tx, current dto.Document, fixed mainControlContext) (mainControlLayout, error) {
	layout := mainControlLayout{Structure: fixed.Structure}
	if fixed.Intent.Structure == nil {
		return layout, domain.Violation(domain.TemporarilyUnavailable, "STRUCTURE_INTENT_UNKNOWN")
	}
	entries, err := stableControlledEntries(ctx, tx, current, *fixed.Intent.Structure)
	if err != nil {
		return layout, err
	}
	// 原可靠MAIN已经准备；目录与各单元版本/manifest在调用方同一最终事务追加。
	// rename/move保持unit与branch身份，不改写历史，也不重新采集已固定的审核正文。
	layout.Structure, err = repository.InsertControlledStructure(ctx, tx, fixed.Document, fixed.Structure, fixed.Intent.Operation)
	if err != nil {
		return layout, err
	}
	return layout, repository.InsertStructureEntries(ctx, tx, fixed.Document, layout.Structure, entries)
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/logic/document/structure_control.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/04-structure.md)。
