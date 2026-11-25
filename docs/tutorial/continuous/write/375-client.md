# 分步编写 `services/search/internal/client/elastic/client.go`

先按 [S01 · Canal事件与Doc当前正式投影](../lessons/search/01-event-authority.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/375-client.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

验证ES端点/owner并统一有界HTTP请求与健康检查。

ES连接和响应也可能超时/截断；不能把HTTP完成或部分JSON误认为投影已应用。

写之前先按这条执行路线理解各段的连接：

1. New限制端点scheme/host，无用户信息、路径查询片段，owner满足固定格式
2. request JSON编码限制64MiB，以context和5秒客户端期限发送
3. LimitReader读maximum+1区分真实超限/截断
4. Check要求health明确yellow/green且未timed_out

本文件需要守住的边界：

- health只说明ES集群可用，不等于消费恢复或索引代次正确
- 不能打印原ES响应作为公开错误

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/search/internal/client/elastic' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/search/internal/client/elastic/client.go') -and (Get-Item -LiteralPath 'services/search/internal/client/elastic/client.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/search/internal/client/elastic/client.go')) { New-Item -ItemType File -Path 'services/search/internal/client/elastic/client.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:160 -->
````go
package elastic

import (
	"bytes"
	"collaboration/platform"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

````

## 第 2 步：Client

插入位置：第 1 步代码之后，紧接追加；完成稿约第 17 行。

本步声明：`type Client struct {`。

**这一段为什么这样写：** 共享ES HTTP连接和索引归属身份。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>endpoint</code> | <code>string</code> | 经过限制的ES根地址 |
| <code>owner</code> | <code>string</code> | 本项目实例owner，用于mapping归属校验 |
| <code>http</code> | <code>*http.Client</code> | 有5秒期限与连接池的HTTP客户端 |

<!-- write-step: 2 bytes=160:241 -->
````go
type Client struct {
	endpoint string
	owner    string
	http     *http.Client
}

````

## 第 3 步：physicalName

插入位置：第 2 步代码之后，紧接追加；完成稿约第 23 行。

本步声明：`var physicalName = regexp.MustCompile(\`^[a-z0-9][a-z0-9._-]{0,159}$\`)`。

<!-- write-step: 3 bytes=241:311 -->
````go
var physicalName = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,159}$`)
````

## 第 4 步：ownerName

插入位置：第 3 步代码之后，紧接追加；完成稿约第 24 行。

本步声明：`var ownerName = regexp.MustCompile(\`^[A-Za-z0-9_.-]{1,64}$\`)`。

<!-- write-step: 4 bytes=311:373 -->
````go
var ownerName = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,64}$`)

````

## 第 5 步：New

插入位置：第 4 步代码之后，紧接追加；完成稿约第 26 行。

本步声明：`func New(configuration *platform.SearchConfig) (*Client, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>configuration</code> | <code>*platform.SearchConfig</code> | 使用 <code>platform.SearchConfig</code> 类型；其定义处的职责：加载服务自身配置，创建有界数据库池，并以迁移和恢复事实判定就绪。<br>在此声明中的实际使用：<code>configuration == nil</code>；<code>configuration.Owner</code>；<code>configuration.ElasticEndpoint</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Client</code> | 使用 <code>Client</code> 类型；其定义处的职责：共享ES HTTP连接和索引归属身份。<br>实际返回表达式：<code>nil</code>；<code>&amp;Client{endpoint: strings.TrimSuffix(configuration.ElasticEndpoint, &quot;/&quot;), owner: configuration.Owner, 		http: &amp;http.Client{Timeout: 5 * time.Second, Transport: transport}}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_ES_CONFIG_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>configuration == nil &#124;&#124; !ownerName.MatchString(configuration.Owner)</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_ES_CONFIG_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; (endpoint.Scheme != &quot;http&quot; &amp;&amp; endpoint.Scheme != &quot;https&quot;) &#124;&#124; endpoint.Host == &quot;&quot; &#124;&#124; endpoint.User != nil &#124;&#124; 		(endpoint.Path != &quot;&quot; &amp;&amp; endpoint.Path != &quot;/&quot;) &#124;&#124; endpoin …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_ES_CONFIG_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return &amp;Client{endpoint: strings.TrimSuffix(configuration.ElasticEndpoint, &quot;/&quot;), owner: configuration.Owner, 		http: &amp;http.Client{Timeout: 5 * time.Second, Transport: transport}}, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=373:1323 -->
````go
func New(configuration *platform.SearchConfig) (*Client, error) {
	if configuration == nil || !ownerName.MatchString(configuration.Owner) {
		return nil, errors.New("SEARCH_ES_CONFIG_INVALID")
	}
	endpoint, err := url.Parse(configuration.ElasticEndpoint)
	if err != nil || (endpoint.Scheme != "http" && endpoint.Scheme != "https") || endpoint.Host == "" || endpoint.User != nil ||
		(endpoint.Path != "" && endpoint.Path != "/") || endpoint.RawQuery != "" || endpoint.Fragment != "" {
		return nil, errors.New("SEARCH_ES_CONFIG_INVALID")
	}
	// 新后台索引请求有独立有限预算，用户HTTP/RPC原期限不变；Ready调用仍受一秒父context限制。
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConnsPerHost = 4
	return &Client{endpoint: strings.TrimSuffix(configuration.ElasticEndpoint, "/"), owner: configuration.Owner,
		http: &http.Client{Timeout: 5 * time.Second, Transport: transport}}, nil
}

````

## 第 6 步：Client.Close

插入位置：第 5 步代码之后，紧接追加；完成稿约第 42 行。

本步声明：`func (c *Client) Close()`。

<!-- write-step: 6 bytes=1323:1383 -->
````go
func (c *Client) Close() { c.http.CloseIdleConnections() }

````

## 第 7 步：Client.request

插入位置：第 6 步代码之后，紧接追加；完成稿约第 44 行。

本步声明：`func (c *Client) request(ctx context.Context, method, path string, value any, maximum int64) (int, []byte, error)`。

**这一段为什么这样写：** 执行一个有限预算ES JSON请求。

1. 编码并限请求体
2. 使用父context发送
3. 限制响应字节并返回状态码/完整体


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 请求取消与上层期限<br>在此声明中的实际使用：<code>http.NewRequestWithContext(ctx, method, c.endpoint+path, &amp;body)</code> |
| <code>method</code> | <code>string</code> | HTTP方法<br>在此声明中的实际使用：<code>http.NewRequestWithContext(ctx, method, c.endpoint+path, &amp;body)</code> |
| <code>path</code> | <code>string</code> | 经调用方验证的ES路径<br>在此声明中的实际使用：<code>c.endpoint+path</code> |
| <code>value</code> | <code>any</code> | 待JSON编码对象，nil表示无请求对象<br>在此声明中的实际使用：<code>value != nil</code>；<code>encoder.Encode(value)</code> |
| <code>maximum</code> | <code>int64</code> | 最大允许响应字节数<br>在此声明中的实际使用：<code>maximum+1</code>；<code>int64(len(data)) &gt; maximum</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>int</code> | ES HTTP状态码<br>实际返回表达式：<code>0</code>；<code>response.StatusCode</code> |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 完整有界响应字节<br>实际返回表达式：<code>nil</code>；<code>data</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 传输/编码/截断稳定错误<br>实际返回表达式：<code>errors.New(&quot;SEARCH_ES_REQUEST_LIMIT&quot;)</code>；<code>errors.New(&quot;SEARCH_ES_REQUEST_INVALID&quot;)</code>；<code>errors.New(&quot;SEARCH_ES_UNAVAILABLE&quot;)</code>；<code>errors.New(&quot;SEARCH_ES_RESPONSE_INCOMPLETE&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>value != nil</code>。
- 检查 <code>err != nil &#124;&#124; body.Len() &gt; 64&lt;&lt;20</code>。条件成立时执行 <code>return 0, nil, errors.New(&quot;SEARCH_ES_REQUEST_LIMIT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return 0, nil, errors.New(&quot;SEARCH_ES_REQUEST_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return 0, nil, errors.New(&quot;SEARCH_ES_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 登记 <code>response.Body.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil &#124;&#124; int64(len(data)) &gt; maximum</code>。条件成立时执行 <code>return response.StatusCode, nil, errors.New(&quot;SEARCH_ES_RESPONSE_INCOMPLETE&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return response.StatusCode, data, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=1383:2333 -->
````go
func (c *Client) request(ctx context.Context, method, path string, value any, maximum int64) (int, []byte, error) {
	var body bytes.Buffer
	if value != nil {
		encoder := json.NewEncoder(&body)
		encoder.SetEscapeHTML(false)
		if err := encoder.Encode(value); err != nil || body.Len() > 64<<20 {
			return 0, nil, errors.New("SEARCH_ES_REQUEST_LIMIT")
		}
	}
	request, err := http.NewRequestWithContext(ctx, method, c.endpoint+path, &body)
	if err != nil {
		return 0, nil, errors.New("SEARCH_ES_REQUEST_INVALID")
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(request)
	if err != nil {
		return 0, nil, errors.New("SEARCH_ES_UNAVAILABLE")
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, maximum+1))
	if err != nil || int64(len(data)) > maximum {
		return response.StatusCode, nil, errors.New("SEARCH_ES_RESPONSE_INCOMPLETE")
	}
	return response.StatusCode, data, nil
}

````

## 第 8 步：Client.Check

插入位置：第 7 步代码之后，紧接追加；完成稿约第 70 行。

本步声明：`func (c *Client) Check(ctx context.Context) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>c.request(ctx, &quot;GET&quot;, &quot;/_cluster/health?wait_for_status=yellow&amp;timeout=500ms&quot;, nil, 1&lt;&lt;20)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_ES_NOT_READY&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil &#124;&#124; status != 200 &#124;&#124; json.Unmarshal(data, &amp;health) != nil &#124;&#124; health.TimedOut &#124;&#124; 		(health.Status != &quot;yellow&quot; &amp;&amp; health.Status != &quot;green&quot;)</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_ES_NOT_READY&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=2333:2807 -->
````go
func (c *Client) Check(ctx context.Context) error {
	status, data, err := c.request(ctx, "GET", "/_cluster/health?wait_for_status=yellow&timeout=500ms", nil, 1<<20)
	var health struct {
		Status   string `json:"status"`
		TimedOut bool   `json:"timed_out"`
	}
	if err != nil || status != 200 || json.Unmarshal(data, &health) != nil || health.TimedOut ||
		(health.Status != "yellow" && health.Status != "green") {
		return errors.New("SEARCH_ES_NOT_READY")
	}
	return nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/search/internal/client/elastic/client.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/search/01-event-authority.md)。
