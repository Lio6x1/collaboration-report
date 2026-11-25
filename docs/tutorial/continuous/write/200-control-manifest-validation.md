# 分步编写 `services/document/internal/logic/document/control_manifest_validation.go`

先按 [D03 · 固定主线操作与支线更新](../lessons/document/03-main-draft.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/200-control-manifest-validation.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

复核主线控制安装的结构单元集合与清单是否完整匹配。

摘要相同不足以证明没有漏掉单元；合法空清单必须来自真实空目录与同号manifest。

写之前先按这条执行路线理解各段的连接：

1. structureManifestUnitSetMatches比较结构和行集合
2. controlManifestUnitSetMatches再比较安装单元
3. validateControlInstallation在私库同一只读事务核固定依赖。

本文件需要守住的边界：

- 系统收尾只能验证已提交安装事实，不能重新获取已撤销用户权限来重做业务。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/logic/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/logic/document/control_manifest_validation.go') -and (Get-Item -LiteralPath 'services/document/internal/logic/document/control_manifest_validation.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/logic/document/control_manifest_validation.go')) { New-Item -ItemType File -Path 'services/document/internal/logic/document/control_manifest_validation.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:270 -->
````go
package document

import (
	"bytes"
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"context"
	"database/sql"
)

````

## 第 2 步：validEmptyControlledManifest

插入位置：第 1 步代码之后，紧接追加；完成稿约第 13 行。

本步声明：`func validEmptyControlledManifest(fixed mainInstallContext) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>fixed</code> | <code>mainInstallContext</code> | 在此声明中的实际使用：<code>fixed.Units</code>；<code>fixed.Context</code>；<code>fixed.Manifest</code>；<code>fixed.Structure</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>err == nil &amp;&amp; bytes.Equal(digest&#91;:&#93;, fixed.Digest)</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(fixed.Units) != 0 &#124;&#124; fixed.Context == nil &#124;&#124; fixed.Context.Target == nil &#124;&#124; fixed.Context.Target.DocumentId == 0 &#124;&#124; 		fixed.Context.Target.Purpose != &quot;MAIN_CONTROL&quot; &#124;&#124; fixed.Ma …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 此处通过 <code>return err == nil &amp;&amp; bytes.Equal(digest&#91;:&#93;, fixed.Digest)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=270:761 -->
````go
func validEmptyControlledManifest(fixed mainInstallContext) bool {
	if len(fixed.Units) != 0 || fixed.Context == nil || fixed.Context.Target == nil || fixed.Context.Target.DocumentId == 0 ||
		fixed.Context.Target.Purpose != "MAIN_CONTROL" || fixed.Manifest == 0 || fixed.Structure == 0 || fixed.Style == 0 || fixed.Control == 0 {
		return false
	}
	digest, err := domain.MainManifestDigest(fixed.Manifest, []*pb.ManifestUnit{})
	return err == nil && bytes.Equal(digest[:], fixed.Digest)
}

````

## 第 3 步：structureManifestUnitSetMatches

插入位置：第 2 步代码之后，紧接追加；完成稿约第 22 行。

本步声明：`func structureManifestUnitSetMatches(structure dto.StructureRevision, rows []map[string]string) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>structure</code> | <code>dto.StructureRevision</code> | 在此声明中的实际使用：<code>structure.Entries</code> |
| <code>rows</code> | <code>&#91;&#93;map&#91;string&#93;string</code> | 在此声明中的实际使用：<code>len(rows)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>len(expected) == 0</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(rows) == 0 &amp;&amp; len(structure.Entries) != 0</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>entry.UnitId == nil</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>err != nil &#124;&#124; expected&#91;*entry.UnitId&#93;</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>len(expected) != len(rows)</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>!expected&#91;row&#91;&quot;unit_id&quot;&#93;&#93;</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 此处通过 <code>return len(expected) == 0</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=761:1492 -->
````go
func structureManifestUnitSetMatches(structure dto.StructureRevision, rows []map[string]string) bool {
	if len(rows) == 0 && len(structure.Entries) != 0 {
		return false
	}
	expected := map[string]bool{}
	for _, entry := range structure.Entries {
		if entry.UnitId == nil {
			continue
		}
		if _, err := domain.CanonicalID(*entry.UnitId); err != nil || expected[*entry.UnitId] {
			return false
		}
		expected[*entry.UnitId] = true
	}
	if len(expected) != len(rows) {
		return false
	}
	for _, row := range rows {
		if !expected[row["unit_id"]] {
			return false
		}
		if _, err := domain.CanonicalID(row["content_version_id"]); err != nil {
			return false
		}
		delete(expected, row["unit_id"])
	}
	return len(expected) == 0
}

````

## 第 4 步：controlManifestUnitSetMatches

插入位置：第 3 步代码之后，紧接追加；完成稿约第 51 行。

本步声明：`func controlManifestUnitSetMatches(structure dto.StructureRevision, rows []map[string]string, units []*pb.ManifestUnit) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>structure</code> | <code>dto.StructureRevision</code> | 在此声明中的实际使用：<code>structureManifestUnitSetMatches(structure, rows)</code> |
| <code>rows</code> | <code>&#91;&#93;map&#91;string&#93;string</code> | 在此声明中的实际使用：<code>structureManifestUnitSetMatches(structure, rows)</code>；<code>len(rows)</code> |
| <code>units</code> | <code>&#91;&#93;*pb.ManifestUnit</code> | 在此声明中的实际使用：<code>len(units)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |

**跟着本段代码走一遍：**

- 检查 <code>!structureManifestUnitSetMatches(structure, rows) &#124;&#124; len(rows) != len(units)</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>unit == nil &#124;&#124; unit.UnitId == 0 &#124;&#124; unit.ContentVersionId == 0 &#124;&#124; expected&#91;domain.TextID(unit.UnitId)&#93; != &quot;&quot;</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>expected&#91;row&#91;&quot;unit_id&quot;&#93;&#93; != row&#91;&quot;content_version_id&quot;&#93;</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 此处通过 <code>return true</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=1492:2127 -->
````go
func controlManifestUnitSetMatches(structure dto.StructureRevision, rows []map[string]string, units []*pb.ManifestUnit) bool {
	if !structureManifestUnitSetMatches(structure, rows) || len(rows) != len(units) {
		return false
	}
	expected := map[string]string{}
	for _, unit := range units {
		if unit == nil || unit.UnitId == 0 || unit.ContentVersionId == 0 || expected[domain.TextID(unit.UnitId)] != "" {
			return false
		}
		expected[domain.TextID(unit.UnitId)] = domain.TextID(unit.ContentVersionId)
	}
	for _, row := range rows {
		if expected[row["unit_id"]] != row["content_version_id"] {
			return false
		}
	}
	return true
}

````

## 第 5 步：MainControls.validateControlInstallation

插入位置：第 4 步代码之后，紧接追加；完成稿约第 70 行。

空清单只能来自真实空目录与同号实际manifest；摘要一致仍不足以证明没有漏掉正文。
这里是系统收尾读取，沿私有库同一只读TX核验，不重新取得已撤销的原用户准入。

本步声明：`func (s *MainControls) validateControlInstallation(ctx context.Context, fixed mainInstallContext) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, &amp;sql.TxOptions{ReadOnly: true})</code>；<code>repository.MainControlManifestBinding(ctx, tx, document, fixed.Manifest, fixed.Structure, fixed.Style, fixed.Context.OperationId)</code>；<code>repository.StructureRow(ctx, tx, domain.TextID(fixed.Structure), domain.TextID(document))</code>；<code>repository.ManifestUnits(ctx, tx, domain.TextID(fixed.Manifest), document)</code> |
| <code>fixed</code> | <code>mainInstallContext</code> | 在此声明中的实际使用：<code>fixed.Context</code>；<code>fixed.Manifest</code>；<code>fixed.Structure</code>；<code>fixed.Style</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>invalid</code>；<code>err</code>；<code>tx.Commit()</code> |

**跟着本段代码走一遍：**

- 检查 <code>fixed.Context == nil &#124;&#124; fixed.Context.Target == nil</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, &amp;sql.TxOptions{ReadOnly: true})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!valid</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!controlManifestUnitSetMatches(structure, rows, fixed.Units)</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 此处通过 <code>return tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 5 bytes=2127:3380 -->
````go
// 空清单只能来自真实空目录与同号实际manifest；摘要一致仍不足以证明没有漏掉正文。
// 这里是系统收尾读取，沿私有库同一只读TX核验，不重新取得已撤销的原用户准入。
func (s *MainControls) validateControlInstallation(ctx context.Context, fixed mainInstallContext) error {
	invalid := domain.Violation(domain.TemporarilyUnavailable, "MAIN_CONTROL_MANIFEST_UNKNOWN")
	if fixed.Context == nil || fixed.Context.Target == nil {
		return invalid
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	document := fixed.Context.Target.DocumentId
	valid, err := repository.MainControlManifestBinding(ctx, tx, document, fixed.Manifest, fixed.Structure, fixed.Style, fixed.Context.OperationId)
	if err != nil {
		return err
	}
	if !valid {
		return invalid
	}
	structure, err := repository.StructureRow(ctx, tx, domain.TextID(fixed.Structure), domain.TextID(document))
	if err != nil {
		return err
	}
	rows, err := repository.ManifestUnits(ctx, tx, domain.TextID(fixed.Manifest), document)
	if err != nil {
		return err
	}
	if !controlManifestUnitSetMatches(structure, rows, fixed.Units) {
		return invalid
	}
	return tx.Commit()
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/logic/document/control_manifest_validation.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/03-main-draft.md)。
