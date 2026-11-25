# 分步编写 `packages/platform/schema_readiness.go`

先按 [F03 · 配置、连接池、就绪与安全出口](../lessons/foundation/03-runtime-and-security.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/028-schema-readiness.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

校验已安装迁移集合与发布配置中的连续版本和摘要完全相等。

数据库不能通过自报一个新摘要让服务接受未知结构，可信期望必须来自发布配置。

写之前先按这条执行路线理解各段的连接：

1. matchesSchemaMigrations先比较总数，再逐版本1到N取期望SHA；要求32字节hex且与actual完全相同。

本文件需要守住的边界：

- 缺迁移、跳号、未知额外版本或摘要不符都false；不能只检查最新version。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'packages/platform' -Force | Out-Null
if ((Test-Path -LiteralPath 'packages/platform/schema_readiness.go') -and (Get-Item -LiteralPath 'packages/platform/schema_readiness.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'packages/platform/schema_readiness.go')) { New-Item -ItemType File -Path 'packages/platform/schema_readiness.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:41 -->
````go
package platform

import "encoding/hex"

````

## 第 2 步：matchesSchemaMigrations

插入位置：第 1 步代码之后，紧接追加；完成稿约第 5 行。

期望集合来自受校验的发布配置；数据库不能自行声明一个新摘要就让服务认可。
001仍独立比较，附加迁移既不能缺失，也不能跳过版本或忽略未知记录。

本步声明：`func matchesSchemaMigrations(baseline string, additional, actual map[int]string) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>baseline</code> | <code>string</code> | 在此声明中的实际使用：<code>expected := baseline</code> |
| <code>additional</code> | <code>map&#91;int&#93;string</code> | 在此声明中的实际使用：<code>len(additional)</code>；<code>expected = additional&#91;version&#93;</code> |
| <code>actual</code> | <code>map&#91;int&#93;string</code> | 在此声明中的实际使用：<code>len(actual)</code>；<code>actual&#91;version&#93; != expected</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(actual) != 1+len(additional)</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>version &gt; 1</code>。
- 检查 <code>err != nil &#124;&#124; len(decoded) != 32 &#124;&#124; actual&#91;version&#93; != expected</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 此处通过 <code>return true</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=41:693 -->
````go
// 期望集合来自受校验的发布配置；数据库不能自行声明一个新摘要就让服务认可。
// 001仍独立比较，附加迁移既不能缺失，也不能跳过版本或忽略未知记录。
func matchesSchemaMigrations(baseline string, additional, actual map[int]string) bool {
	if len(actual) != 1+len(additional) {
		return false
	}
	for version := 1; version <= 1+len(additional); version++ {
		expected := baseline
		if version > 1 {
			expected = additional[version]
		}
		decoded, err := hex.DecodeString(expected)
		if err != nil || len(decoded) != 32 || actual[version] != expected {
			return false
		}
	}
	return true
}
````

## 文件完成后的检查

```powershell
gofmt -w 'packages/platform/schema_readiness.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/foundation/03-runtime-and-security.md)。
