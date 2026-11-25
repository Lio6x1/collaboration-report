# 分步编写 `services/document/internal/handler/httpx/body.go`

先按 [D01 · 身份、权限与共享准入](../lessons/document/01-identity-access.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/266-body.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

为小控制请求和大正文请求提供共用严格JSON解码。

契约中的必填/可空规则与未知字段拒绝不能因某个入口自行解码而失效。

写之前先按这条执行路线理解各段的连接：

1. DecodeBody默认16KiB
2. DecodeBodyLimit先限长读取、检查RequiredFields，再DisallowUnknownFields解码
3. ContentBodyLimit为16MiB正文预留传输空间。

本文件需要守住的边界：

- value必须是生成DTO的指针
- 必填字段缺失与不允许null分别检查
- 不要只用结构体零值判断是否填写。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/handler/httpx' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/handler/httpx/body.go') -and (Get-Item -LiteralPath 'services/document/internal/handler/httpx/body.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/handler/httpx/body.go')) { New-Item -ItemType File -Path 'services/document/internal/handler/httpx/body.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:121 -->
````go
package httpx

import (
	"bytes"
	dto "collaboration/contracts/httpdto"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
)

````

## 第 2 步：ContentBodyLimit

插入位置：第 1 步代码之后，紧接追加；完成稿约第 12 行。

16MiB规范正文另留协议和JSON转义空间；路由框架与本地解码共用同一个传输上限。

本步声明：`const ContentBodyLimit int64 = 18 << 20`。

<!-- write-step: 2 bytes=121:274 -->
````go
// 16MiB规范正文另留协议和JSON转义空间；路由框架与本地解码共用同一个传输上限。
const ContentBodyLimit int64 = 18 << 20

````

## 第 3 步：DecodeBody

插入位置：第 2 步代码之后，紧接追加；完成稿约第 15 行。

解码保留生成契约的必填/可空规则、请求上限及未知字段拒绝，不能由各用例复制较弱版本。

本步声明：`func DecodeBody(w http.ResponseWriter, r *http.Request, value any) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>DecodeBodyLimit(w, r, value, 16384)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>DecodeBodyLimit(w, r, value, 16384)</code> |
| <code>value</code> | <code>any</code> | 在此声明中的实际使用：<code>DecodeBodyLimit(w, r, value, 16384)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>DecodeBodyLimit(w, r, value, 16384)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return DecodeBodyLimit(w, r, value, 16384)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=274:523 -->
````go
// 解码保留生成契约的必填/可空规则、请求上限及未知字段拒绝，不能由各用例复制较弱版本。
func DecodeBody(w http.ResponseWriter, r *http.Request, value any) bool {
	return DecodeBodyLimit(w, r, value, 16384)
}

````

## 第 4 步：DecodeBodyLimit

插入位置：第 3 步代码之后，紧接追加；完成稿约第 20 行。

正文修复按冻结的16MiB正文契约接收；其余小型控制请求保留原上限和相同字段校验。

本步声明：`func DecodeBodyLimit(w http.ResponseWriter, r *http.Request, value any, limit int64) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>w</code> | <code>http.ResponseWriter</code> | 当前 HTTP 响应的写入器，状态码和正文写到这次请求；不要存入服务对象跨请求复用。<br>在此声明中的实际使用：<code>Fail(w, 400, &quot;JSON_REQUIRED&quot;)</code>；<code>Fail(w, 400, &quot;REQUEST_INVALID&quot;)</code>；<code>Fail(w, 400, &quot;REQUIRED_FIELD_INVALID&quot;)</code> |
| <code>r</code> | <code>*http.Request</code> | 当前 HTTP 请求，承载路径、请求头、请求体与取消上下文；请求中的账号或权限声明仍需服务核验。<br>在此声明中的实际使用：<code>r.Header</code>；<code>r.Body</code> |
| <code>value</code> | <code>any</code> | 在此声明中的实际使用：<code>reflect.TypeOf(value)</code>；<code>d.Decode(value)</code> |
| <code>limit</code> | <code>int64</code> | 在此声明中的实际使用：<code>limit+1</code>；<code>int64(len(raw)) &gt; limit</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |

**跟着本段代码走一遍：**

- 检查 <code>r.Header.Get(&quot;Content-Type&quot;) != &quot;application/json&quot;</code>。
- 此处通过 <code>return false</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil &#124;&#124; int64(len(raw)) &gt; limit</code>。
- 此处通过 <code>return false</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>json.Unmarshal(raw, &amp;fields) != nil</code>。
- 此处通过 <code>return false</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!ok &#124;&#124; (!nullable &amp;&amp; string(v) == &quot;null&quot;)</code>。
- 此处通过 <code>return false</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>d.Decode(value) != nil</code>。
- 此处通过 <code>return false</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return true</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=523:1533 -->
````go
// 正文修复按冻结的16MiB正文契约接收；其余小型控制请求保留原上限和相同字段校验。
func DecodeBodyLimit(w http.ResponseWriter, r *http.Request, value any, limit int64) bool {
	if r.Header.Get("Content-Type") != "application/json" {
		Fail(w, 400, "JSON_REQUIRED")
		return false
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, limit+1))
	if err != nil || int64(len(raw)) > limit {
		Fail(w, 400, "REQUEST_INVALID")
		return false
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		Fail(w, 400, "REQUEST_INVALID")
		return false
	}
	name := reflect.TypeOf(value).Elem().Name()
	for field, nullable := range dto.RequiredFields[name] {
		v, ok := fields[field]
		if !ok || (!nullable && string(v) == "null") {
			Fail(w, 400, "REQUIRED_FIELD_INVALID")
			return false
		}
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(value) != nil {
		Fail(w, 400, "REQUEST_INVALID")
		return false
	}
	return true
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/handler/httpx/body.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/01-identity-access.md)。
