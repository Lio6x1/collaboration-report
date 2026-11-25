# 分步编写 `services/document/internal/logic/document/pagination.go`

先按 [D01 · 身份、权限与共享准入](../lessons/document/01-identity-access.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/216-pagination.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

生成并校验绑定查询上下文的签名翻页游标。

客户端只能携带服务签出的最后一项位置，不能改游标后跨用户或跨查询读取。

写之前先按这条执行路线理解各段的连接：

1. Parse处理limit并验证编码、签名及binding
2. Next把同一binding与last封装成下一页令牌。

本文件需要守住的边界：

- 签名正确但binding已变化仍应报CURSOR_STALE
- 不能把游标当未校验SQL片段。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/logic/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/logic/document/pagination.go') -and (Get-Item -LiteralPath 'services/document/internal/logic/document/pagination.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/logic/document/pagination.go')) { New-Item -ItemType File -Path 'services/document/internal/logic/document/pagination.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:158 -->
````go
package document

import (
	domain "collaboration/document/internal/domain/document"
	"crypto/hmac"
	"encoding/base64"
	"encoding/json"
	"strconv"
	"time"
)

````

## 第 2 步：CursorSigner

插入位置：第 1 步代码之后，紧接追加；完成稿约第 12 行。

本步声明：`type CursorSigner interface{ Sign([]byte) []byte }`。

**这一段为什么这样写：** 游标签名能力与分页用例隔离；分页层负责绑定范围，签名器保管密钥。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Sign</code> | <code>(&#91;&#93;byte) &#91;&#93;byte</code> | 输入规范游标字节，返回签名/MAC字节；查询范围和位置编码后一起签名，客户端不能任意改边界。 |

<!-- write-step: 2 bytes=158:209 -->
````go
type CursorSigner interface{ Sign([]byte) []byte }
````

## 第 3 步：Pagination

插入位置：第 2 步代码之后，紧接追加；完成稿约第 13 行。

本步声明：`type Pagination struct{ signer CursorSigner }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>signer</code> | <code>CursorSigner</code> | 使用 <code>CursorSigner</code> 类型；其定义处的职责：游标签名能力与分页用例隔离；分页层负责绑定范围，签名器保管密钥。 |

<!-- write-step: 3 bytes=209:256 -->
````go
type Pagination struct{ signer CursorSigner }

````

## 第 4 步：NewPagination

插入位置：第 3 步代码之后，紧接追加；完成稿约第 15 行。

本步声明：`func NewPagination(signer CursorSigner) *Pagination`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>signer</code> | <code>CursorSigner</code> | 使用 <code>CursorSigner</code> 类型；其定义处的职责：游标签名能力与分页用例隔离；分页层负责绑定范围，签名器保管密钥。<br>在此声明中的实际使用：<code>signer: signer</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Pagination</code> | 实际返回表达式：<code>&amp;Pagination{signer: signer}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Pagination{signer: signer}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=256:347 -->
````go
func NewPagination(signer CursorSigner) *Pagination { return &Pagination{signer: signer} }
````

## 第 5 步：Pagination.Parse

插入位置：第 4 步代码之后，紧接追加；完成稿约第 16 行。

本步声明：`func (s *Pagination) Parse(limitText, encoded, binding string) (int, string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>limitText</code> | <code>string</code> | 在此声明中的实际使用：<code>limitText != &quot;&quot;</code>；<code>strconv.Atoi(limitText)</code> |
| <code>encoded</code> | <code>string</code> | 在此声明中的实际使用：<code>encoded == &quot;&quot;</code>；<code>len(encoded)</code>；<code>base64.RawURLEncoding.DecodeString(encoded)</code> |
| <code>binding</code> | <code>string</code> | 在此声明中的实际使用：<code>cursor.Binding != binding</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>int</code> | 实际返回表达式：<code>0</code>；<code>limit</code> |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;&quot;</code>；<code>cursor.Last</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.InvalidRequest, &quot;LIMIT_INVALID&quot;)</code>；<code>nil</code>；<code>domain.Violation(domain.InvalidRequest, &quot;CURSOR_INVALID&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;CURSOR_STALE&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>limitText != &quot;&quot;</code>。
- 检查 <code>err != nil &#124;&#124; value &lt; 1 &#124;&#124; value &gt; 100</code>。条件成立时执行 <code>return 0, &quot;&quot;, domain.Violation(domain.InvalidRequest, &quot;LIMIT_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>encoded == &quot;&quot;</code>。条件成立时执行 <code>return limit, &quot;&quot;, nil</code>。当前函数在这里结束。
- 检查 <code>len(encoded) &gt; 2048</code>。条件成立时执行 <code>return 0, &quot;&quot;, domain.Violation(domain.InvalidRequest, &quot;CURSOR_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; len(raw) &lt; 33</code>。条件成立时执行 <code>return 0, &quot;&quot;, domain.Violation(domain.InvalidRequest, &quot;CURSOR_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>!hmac.Equal(signature, s.signer.Sign(data))</code>。条件成立时执行 <code>return 0, &quot;&quot;, domain.Violation(domain.InvalidRequest, &quot;CURSOR_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(data, &amp;cursor) != nil &#124;&#124; cursor.Binding != binding &#124;&#124; cursor.Until &lt;= time.Now().Unix()</code>。条件成立时执行 <code>return 0, &quot;&quot;, domain.Violation(domain.StateConflict, &quot;CURSOR_STALE&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return limit, cursor.Last, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=347:1396 -->
````go
func (s *Pagination) Parse(limitText, encoded, binding string) (int, string, error) {
	limit := 20
	if limitText != "" {
		value, err := strconv.Atoi(limitText)
		if err != nil || value < 1 || value > 100 {
			return 0, "", domain.Violation(domain.InvalidRequest, "LIMIT_INVALID")
		}
		limit = value
	}
	if encoded == "" {
		return limit, "", nil
	}
	if len(encoded) > 2048 {
		return 0, "", domain.Violation(domain.InvalidRequest, "CURSOR_INVALID")
	}
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || len(raw) < 33 {
		return 0, "", domain.Violation(domain.InvalidRequest, "CURSOR_INVALID")
	}
	data, signature := raw[:len(raw)-32], raw[len(raw)-32:]
	if !hmac.Equal(signature, s.signer.Sign(data)) {
		return 0, "", domain.Violation(domain.InvalidRequest, "CURSOR_INVALID")
	}
	var cursor domain.Cursor
	if json.Unmarshal(data, &cursor) != nil || cursor.Binding != binding || cursor.Until <= time.Now().Unix() {
		return 0, "", domain.Violation(domain.StateConflict, "CURSOR_STALE")
	}
	return limit, cursor.Last, nil
}
````

## 第 6 步：Pagination.Next

插入位置：第 5 步代码之后，紧接追加；完成稿约第 45 行。

本步声明：`func (s *Pagination) Next(binding, last string) *string`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>binding</code> | <code>string</code> | 在此声明中的实际使用：<code>Binding: binding</code> |
| <code>last</code> | <code>string</code> | 在此声明中的实际使用：<code>Last: last</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*string</code> | 实际返回表达式：<code>&amp;encoded</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;encoded</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1396:1674 -->
````go
func (s *Pagination) Next(binding, last string) *string {
	raw, _ := json.Marshal(domain.Cursor{Binding: binding, Last: last, Until: time.Now().Add(5 * time.Minute).Unix()})
	encoded := base64.RawURLEncoding.EncodeToString(append(raw, s.signer.Sign(raw)...))
	return &encoded
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/logic/document/pagination.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/01-identity-access.md)。
