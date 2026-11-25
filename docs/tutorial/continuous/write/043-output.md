# 分步编写 `services/account/internal/domain/account/output.go`

先按 [A08 · 输出保护与HTTP、RPC、worker完整装配](../lessons/account/08-output-and-wiring.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/043-output.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

收集一次响应实际依赖的来源与代次，并记录捕获冲突。

响应构造期间可能读到不同来源；只有完整依赖集合才能在发送前注册可撤销输出。

写之前先按这条执行路线理解各段的连接：

1. OutputSource序列化scope_key与十进制epoch；OutputBinding.Sources用scope作键。
2. 同来源捕获不一致时置Invalid，后续保护器拒绝输出。

本文件需要守住的边界：

- 不能用最新epoch覆盖旧查询结果对应epoch，覆盖会掩盖撤权竞争。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/account/internal/domain/account' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/account/internal/domain/account/output.go') -and (Get-Item -LiteralPath 'services/account/internal/domain/account/output.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/account/internal/domain/account/output.go')) { New-Item -ItemType File -Path 'services/account/internal/domain/account/output.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:17 -->
````go
package account

````

## 第 2 步：OutputSource

插入位置：第 1 步代码之后，紧接追加；完成稿约第 3 行。

本步声明：`type OutputSource struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Key</code> | <code>string</code> | 输出依赖的账号来源范围键。<br>序列化/校验标记：<code>`json:&quot;scope_key&quot;`</code> |
| <code>Epoch</code> | <code>string</code> | 准入时该来源的固定十进制代次。<br>序列化/校验标记：<code>`json:&quot;epoch&quot;`</code> |

<!-- write-step: 2 bytes=17:109 -->
````go
type OutputSource struct {
	Key   string `json:"scope_key"`
	Epoch string `json:"epoch"`
}

````

## 第 3 步：OutputBinding

插入位置：第 2 步代码之后，紧接追加；完成稿约第 8 行。

本步声明：`type OutputBinding struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Sources</code> | <code>map&#91;string&#93;string</code> | 规范范围键到代次的映射，所有来源都必须仍有效。 |
| <code>Invalid</code> | <code>bool</code> | 解码或组合发现不合法来源时置true，不能带无效绑定继续输出。 |

<!-- write-step: 3 bytes=109:180 -->
````go
type OutputBinding struct {
	Sources map[string]string
	Invalid bool
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/account/internal/domain/account/output.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/account/08-output-and-wiring.md)。
