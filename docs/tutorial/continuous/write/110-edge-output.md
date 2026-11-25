# 分步编写 `services/document/internal/client/edge_output.go`

先按 [D11 · 输出登记、撤权与真实排空](../lessons/document/11-output-fence.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/110-edge-output.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

通过内部HTTP协议调用Edge输出登记和关闭确认。

Edge是实际向用户写字节的参与者，其响应必须按固定协议确认，不能只凭请求已发送。

写之前先按这条执行路线理解各段的连接：

1. EdgeOutput.Call序列化输入，使用注入的受信客户端请求并严格解析结果，失败映射稳定输出错误。

本文件需要守住的边界：

- 网络超时属于未确认，不能当OUTPUT_STOPPED
- 不要把上游原始详情泄漏到公共响应。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/client' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/client/edge_output.go') -and (Get-Item -LiteralPath 'services/document/internal/client/edge_output.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/client/edge_output.go')) { New-Item -ItemType File -Path 'services/document/internal/client/edge_output.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:104 -->
````go
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

````

## 第 2 步：EdgeOutput

插入位置：第 1 步代码之后，紧接追加；完成稿约第 13 行。

本步声明：`type EdgeOutput struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>base</code> | <code>string</code> | Edge内部HTTP服务地址，由可信配置注入；路径在客户端方法中限定，不接受用户提供的任意目标地址。 |
| <code>http</code> | <code>*http.Client</code> | 注入的HTTP客户端，承载连接池、TLS和超时配置；请求沿自己的ctx取消，方法结束不应销毁共享连接池。 |

<!-- write-step: 2 bytes=104:164 -->
````go
type EdgeOutput struct {
	base string
	http *http.Client
}

````

## 第 3 步：NewEdgeOutput

插入位置：第 2 步代码之后，紧接追加；完成稿约第 18 行。

本步声明：`func NewEdgeOutput(base string, httpClient *http.Client) *EdgeOutput`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>base</code> | <code>string</code> | 在此声明中的实际使用：<code>base: base</code> |
| <code>httpClient</code> | <code>*http.Client</code> | 注入的HTTP客户端，承载连接池、TLS和超时配置；请求沿自己的ctx取消，方法结束不应销毁共享连接池。<br>在此声明中的实际使用：<code>http: httpClient</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*EdgeOutput</code> | 实际返回表达式：<code>&amp;EdgeOutput{base: base, http: httpClient}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;EdgeOutput{base: base, http: httpClient}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=164:287 -->
````go
func NewEdgeOutput(base string, httpClient *http.Client) *EdgeOutput {
	return &EdgeOutput{base: base, http: httpClient}
}
````

## 第 4 步：EdgeOutput.Call

插入位置：第 3 步代码之后，紧接追加；完成稿约第 21 行。

本步声明：`func (c *EdgeOutput) Call(ctx context.Context, path string, input any, result any) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>http.NewRequestWithContext(ctx, method, url, body)</code> |
| <code>path</code> | <code>string</code> | 在此声明中的实际使用：<code>strings.Replace(c.base, &quot;localhost&quot;, &quot;127.0.0.1&quot;, 1) + path</code> |
| <code>input</code> | <code>any</code> | 在此声明中的实际使用：<code>input != nil</code>；<code>json.Marshal(input)</code>；<code>input == nil</code> |
| <code>result</code> | <code>any</code> | 在此声明中的实际使用：<code>json.NewDecoder(io.LimitReader(res.Body, 16384)).Decode(result)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;EDGE_OUTPUT_UNAVAILABLE&quot;)</code>；<code>errors.New(&quot;EDGE_OUTPUT_NOT_CONFIRMED&quot;)</code>；<code>json.NewDecoder(io.LimitReader(res.Body, 16384)).Decode(result)</code> |

**跟着本段代码走一遍：**

- 检查 <code>input != nil</code>。
- 检查 <code>input == nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;EDGE_OUTPUT_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 登记 <code>res.Body.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>res.StatusCode != 200</code>。条件成立时执行 <code>return errors.New(&quot;EDGE_OUTPUT_NOT_CONFIRMED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return json.NewDecoder(io.LimitReader(res.Body, 16384)).Decode(result)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=287:1023 -->
````go
func (c *EdgeOutput) Call(ctx context.Context, path string, input any, result any) error {
	var body io.Reader
	if input != nil {
		raw, _ := json.Marshal(input)
		body = bytes.NewReader(raw)
	}
	method := "POST"
	if input == nil {
		method = "GET"
	}
	url := strings.Replace(c.base, "localhost", "127.0.0.1", 1) + path
	r, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return err
	}
	r.Header.Set("Content-Type", "application/json")
	res, err := c.http.Do(r)
	if err != nil {
		return errors.New("EDGE_OUTPUT_UNAVAILABLE")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return errors.New("EDGE_OUTPUT_NOT_CONFIRMED")
	}
	return json.NewDecoder(io.LimitReader(res.Body, 16384)).Decode(result)
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/client/edge_output.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/11-output-fence.md)。
