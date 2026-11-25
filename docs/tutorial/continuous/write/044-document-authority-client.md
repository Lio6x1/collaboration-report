# 分步编写 `services/account/internal/client/document_authority_client.go`

先按 [A07 · 会话退出、到期与账号注销可靠恢复](../lessons/account/07-closure-recovery.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/044-document-authority-client.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

把Account关闭、输出登记、成员离开及组织关闭请求发送给Document的受控接口。

HTTP 200只是运输成功；客户端还要确认完整JSON、固定binding、状态和摘要才可推进本地事务。

写之前先按这条执行路线理解各段的连接：

1. AccountClosureControl.send复用mTLS客户端并严格完整读取；Prepare/Plan/Relations/Complete分别验证对应协议事实。
2. Page与CleanDocument绑定原计划和文档ID，只有匹配的清理回执才推进游标。
3. Document.FenceAccess调用冻结RPC；Departure.Check未知时拒绝离开。

本文件需要守住的边界：

- 截断JSON、错误binding或未知结果不能当作完成。
- 网络调用不得擅自更换operation或使用浏览器凭据代替服务身份。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/account/internal/client' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/account/internal/client/document_authority_client.go') -and (Get-Item -LiteralPath 'services/account/internal/client/document_authority_client.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/account/internal/client/document_authority_client.go')) { New-Item -ItemType File -Path 'services/account/internal/client/document_authority_client.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:456 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package client

import (
	"bytes"
	domain "collaboration/account/internal/domain/account"
	pb "collaboration/contracts/internalv1"
	"collaboration/platform"
	"context"
	"encoding/json"
	"errors"
	"google.golang.org/grpc/status"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// account_closure.go：保留原实现，按业务合并文件。

````

## 第 2 步：AccountClosureControl

插入位置：第 1 步代码之后，紧接追加；完成稿约第 23 行。

本步声明：`type AccountClosureControl struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>http</code> | <code>*http.Client</code> | 注入的HTTP客户端，承载连接池、TLS和超时配置；请求沿自己的ctx取消，方法结束不应销毁共享连接池。 |
| <code>endpoint</code> | <code>string</code> | Doc内部HTTP接口基地址，由服务配置注入，不能来自浏览器参数。 |

<!-- write-step: 2 bytes=456:535 -->
````go
type AccountClosureControl struct {
	http     *http.Client
	endpoint string
}

````

## 第 3 步：NewAccountClosureControl

插入位置：第 2 步代码之后，紧接追加；完成稿约第 28 行。

本步声明：`func NewAccountClosureControl(httpClient *http.Client, endpoint string) *AccountClosureControl`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>httpClient</code> | <code>*http.Client</code> | 注入的HTTP客户端，承载连接池、TLS和超时配置；请求沿自己的ctx取消，方法结束不应销毁共享连接池。<br>在此声明中的实际使用：<code>http: httpClient</code> |
| <code>endpoint</code> | <code>string</code> | 在此声明中的实际使用：<code>endpoint: endpoint</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*AccountClosureControl</code> | 实际返回表达式：<code>&amp;AccountClosureControl{http: httpClient, endpoint: endpoint}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;AccountClosureControl{http: httpClient, endpoint: endpoint}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=535:704 -->
````go
func NewAccountClosureControl(httpClient *http.Client, endpoint string) *AccountClosureControl {
	return &AccountClosureControl{http: httpClient, endpoint: endpoint}
}

````

## 第 4 步：AccountClosureControl.call

插入位置：第 3 步代码之后，紧接追加；完成稿约第 32 行。

本步声明：`func (c *AccountClosureControl) call(ctx context.Context, path string, binding platform.AccountClosureBinding, result any) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>c.send(ctx, path, raw, result)</code> |
| <code>path</code> | <code>string</code> | 在此声明中的实际使用：<code>c.send(ctx, path, raw, result)</code> |
| <code>binding</code> | <code>platform.AccountClosureBinding</code> | 使用 <code>platform.AccountClosureBinding</code> 类型；其定义处的职责：AccountClosureBinding只在Account和Doc控制通道传输，不由公开注销请求提供。 固定关闭组织集合防止恢复时把新加入成员的组织换入旧的清理计划。<br>在此声明中的实际使用：<code>platform.ValidateAccountClosureBinding(binding)</code>；<code>json.Marshal(binding)</code> |
| <code>result</code> | <code>any</code> | 在此声明中的实际使用：<code>c.send(ctx, path, raw, result)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;ACCOUNT_CLOSURE_BINDING_INVALID&quot;)</code>；<code>err</code>；<code>c.send(ctx, path, raw, result)</code> |

**跟着本段代码走一遍：**

- 检查 <code>!platform.ValidateAccountClosureBinding(binding)</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_CLOSURE_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return c.send(ctx, path, raw, result)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=704:1058 -->
````go
func (c *AccountClosureControl) call(ctx context.Context, path string, binding platform.AccountClosureBinding, result any) error {
	if !platform.ValidateAccountClosureBinding(binding) {
		return errors.New("ACCOUNT_CLOSURE_BINDING_INVALID")
	}
	raw, err := json.Marshal(binding)
	if err != nil {
		return err
	}
	return c.send(ctx, path, raw, result)
}

````

## 第 5 步：AccountClosureControl.send

插入位置：第 4 步代码之后，紧接追加；完成稿约第 43 行。

内部绑定和有界分页共用mTLS/完整JSON核验，不接受成功HTTP中的截断回执。

本步声明：`func (c *AccountClosureControl) send(ctx context.Context, path string, raw []byte, result any) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>http.NewRequestWithContext(ctx, &quot;POST&quot;, c.endpoint+path, bytes.NewReader(raw))</code> |
| <code>path</code> | <code>string</code> | 在此声明中的实际使用：<code>c.endpoint+path</code> |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>bytes.NewReader(raw)</code> |
| <code>result</code> | <code>any</code> | 在此声明中的实际使用：<code>decoder.Decode(result)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;ACCOUNT_CLOSURE_CONTROL_UNAVAILABLE&quot;)</code>；<code>errors.New(&quot;ACCOUNT_CLOSURE_CONTROL_UNCONFIRMED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_CLOSURE_CONTROL_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 登记 <code>response.Body.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>response.StatusCode != 200 &#124;&#124; decoder.Decode(result) != nil &#124;&#124; decoder.Decode(new(any)) != io.EOF</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_CLOSURE_CONTROL_UNCONFIRMED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1058:1872 -->
````go
// 内部绑定和有界分页共用mTLS/完整JSON核验，不接受成功HTTP中的截断回执。
func (c *AccountClosureControl) send(ctx context.Context, path string, raw []byte, result any) error {
	request, err := http.NewRequestWithContext(ctx, "POST", c.endpoint+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(request)
	if err != nil {
		return errors.New("ACCOUNT_CLOSURE_CONTROL_UNAVAILABLE")
	}
	defer response.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(response.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if response.StatusCode != 200 || decoder.Decode(result) != nil || decoder.Decode(new(any)) != io.EOF {
		return errors.New("ACCOUNT_CLOSURE_CONTROL_UNCONFIRMED")
	}
	return nil
}

````

## 第 6 步：AccountClosureControl.Prepare

插入位置：第 5 步代码之后，紧接追加；完成稿约第 63 行。

本步声明：`func (c *AccountClosureControl) Prepare(ctx context.Context, binding platform.AccountClosureBinding) (platform.AccountClosurePreparation, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>c.call(ctx, &quot;/infra/lifecycle/account-close-prepare&quot;, binding, &amp;result)</code> |
| <code>binding</code> | <code>platform.AccountClosureBinding</code> | 使用 <code>platform.AccountClosureBinding</code> 类型；其定义处的职责：AccountClosureBinding只在Account和Doc控制通道传输，不由公开注销请求提供。 固定关闭组织集合防止恢复时把新加入成员的组织换入旧的清理计划。<br>在此声明中的实际使用：<code>c.call(ctx, &quot;/infra/lifecycle/account-close-prepare&quot;, binding, &amp;result)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>platform.AccountClosurePreparation</code> | 使用 <code>platform.AccountClosurePreparation</code> 类型；其定义处的职责：定义Account与Document之间的固定注销绑定和有界清理分页协议。<br>实际返回表达式：<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>err == nil &amp;&amp; (result.RequiredKnowledgeBases == nil &#124;&#124; result.ReservedKnowledgeBases == nil &#124;&#124; (result.State != &quot;FROZEN&quot; &amp;&amp; result.State != &quot;HANDOFF_REQUIRED&quot; &amp;&amp; result.State != &quot;REJECTED&quot;))</code>。
- 此处通过 <code>return result, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1872:2427 -->
````go
func (c *AccountClosureControl) Prepare(ctx context.Context, binding platform.AccountClosureBinding) (platform.AccountClosurePreparation, error) {
	var result platform.AccountClosurePreparation
	err := c.call(ctx, "/infra/lifecycle/account-close-prepare", binding, &result)
	if err == nil && (result.RequiredKnowledgeBases == nil || result.ReservedKnowledgeBases == nil || (result.State != "FROZEN" && result.State != "HANDOFF_REQUIRED" && result.State != "REJECTED")) {
		err = errors.New("ACCOUNT_CLOSURE_PREPARATION_INVALID")
	}
	return result, err
}

````

## 第 7 步：AccountClosureControl.Cancel

插入位置：第 6 步代码之后，紧接追加；完成稿约第 72 行。

本步声明：`func (c *AccountClosureControl) Cancel(ctx context.Context, binding platform.AccountClosureBinding) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>c.call(ctx, &quot;/infra/lifecycle/account-close-cancel&quot;, binding, &amp;result)</code> |
| <code>binding</code> | <code>platform.AccountClosureBinding</code> | 使用 <code>platform.AccountClosureBinding</code> 类型；其定义处的职责：AccountClosureBinding只在Account和Doc控制通道传输，不由公开注销请求提供。 固定关闭组织集合防止恢复时把新加入成员的组织换入旧的清理计划。<br>在此声明中的实际使用：<code>c.call(ctx, &quot;/infra/lifecycle/account-close-cancel&quot;, binding, &amp;result)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;ACCOUNT_CLOSURE_CANCEL_UNCONFIRMED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!result.Rejected</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_CLOSURE_CANCEL_UNCONFIRMED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=2427:2790 -->
````go
func (c *AccountClosureControl) Cancel(ctx context.Context, binding platform.AccountClosureBinding) error {
	var result platform.AccountClosureDecision
	if err := c.call(ctx, "/infra/lifecycle/account-close-cancel", binding, &result); err != nil {
		return err
	}
	if !result.Rejected {
		return errors.New("ACCOUNT_CLOSURE_CANCEL_UNCONFIRMED")
	}
	return nil
}

````

## 第 8 步：AccountClosureControl.Plan

插入位置：第 7 步代码之后，紧接追加；完成稿约第 83 行。

本步声明：`func (c *AccountClosureControl) Plan(ctx context.Context, binding platform.AccountClosureBinding) (platform.AccountClosureRetention, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>c.call(ctx, &quot;/infra/lifecycle/account-close-plan&quot;, binding, &amp;result)</code> |
| <code>binding</code> | <code>platform.AccountClosureBinding</code> | 使用 <code>platform.AccountClosureBinding</code> 类型；其定义处的职责：AccountClosureBinding只在Account和Doc控制通道传输，不由公开注销请求提供。 固定关闭组织集合防止恢复时把新加入成员的组织换入旧的清理计划。<br>在此声明中的实际使用：<code>c.call(ctx, &quot;/infra/lifecycle/account-close-plan&quot;, binding, &amp;result)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>platform.AccountClosureRetention</code> | 使用 <code>platform.AccountClosureRetention</code> 类型；其定义处的职责：PLANNED仅确认Doc固定了保留对象并登记输出屏障，不能解释为清理或注销完成。<br>实际返回表达式：<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>err == nil &amp;&amp; result.State != &quot;PLANNED&quot;</code>。
- 此处通过 <code>return result, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=2790:3186 -->
````go
func (c *AccountClosureControl) Plan(ctx context.Context, binding platform.AccountClosureBinding) (platform.AccountClosureRetention, error) {
	var result platform.AccountClosureRetention
	err := c.call(ctx, "/infra/lifecycle/account-close-plan", binding, &result)
	if err == nil && result.State != "PLANNED" {
		err = errors.New("ACCOUNT_CLOSURE_RETENTION_UNCONFIRMED")
	}
	return result, err
}

````

## 第 9 步：AccountClosureControl.Relations

插入位置：第 8 步代码之后，紧接追加；完成稿约第 92 行。

本步声明：`func (c *AccountClosureControl) Relations(ctx context.Context, binding platform.AccountClosureBinding) (platform.AccountClosureRetention, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>c.call(ctx, &quot;/infra/lifecycle/account-close-relations&quot;, binding, &amp;result)</code> |
| <code>binding</code> | <code>platform.AccountClosureBinding</code> | 使用 <code>platform.AccountClosureBinding</code> 类型；其定义处的职责：AccountClosureBinding只在Account和Doc控制通道传输，不由公开注销请求提供。 固定关闭组织集合防止恢复时把新加入成员的组织换入旧的清理计划。<br>在此声明中的实际使用：<code>c.call(ctx, &quot;/infra/lifecycle/account-close-relations&quot;, binding, &amp;result)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>platform.AccountClosureRetention</code> | 使用 <code>platform.AccountClosureRetention</code> 类型；其定义处的职责：PLANNED仅确认Doc固定了保留对象并登记输出屏障，不能解释为清理或注销完成。<br>实际返回表达式：<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>err == nil &amp;&amp; result.State != &quot;CLEANING&quot;</code>。
- 此处通过 <code>return result, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=3186:3593 -->
````go
func (c *AccountClosureControl) Relations(ctx context.Context, binding platform.AccountClosureBinding) (platform.AccountClosureRetention, error) {
	var result platform.AccountClosureRetention
	err := c.call(ctx, "/infra/lifecycle/account-close-relations", binding, &result)
	if err == nil && result.State != "CLEANING" {
		err = errors.New("ACCOUNT_CLOSURE_RELATIONS_UNCONFIRMED")
	}
	return result, err
}

````

## 第 10 步：AccountClosureControl.Complete

插入位置：第 9 步代码之后，紧接追加；完成稿约第 101 行。

本步声明：`func (c *AccountClosureControl) Complete(ctx context.Context, binding platform.AccountClosureBinding) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>c.call(ctx, &quot;/infra/lifecycle/account-close-complete&quot;, binding, &amp;result)</code> |
| <code>binding</code> | <code>platform.AccountClosureBinding</code> | 使用 <code>platform.AccountClosureBinding</code> 类型；其定义处的职责：AccountClosureBinding只在Account和Doc控制通道传输，不由公开注销请求提供。 固定关闭组织集合防止恢复时把新加入成员的组织换入旧的清理计划。<br>在此声明中的实际使用：<code>c.call(ctx, &quot;/infra/lifecycle/account-close-complete&quot;, binding, &amp;result)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;ACCOUNT_CLOSURE_COMPLETION_UNCONFIRMED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>result.State != &quot;SUCCEEDED&quot;</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_CLOSURE_COMPLETION_UNCONFIRMED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=3593:3976 -->
````go
func (c *AccountClosureControl) Complete(ctx context.Context, binding platform.AccountClosureBinding) error {
	var result platform.AccountClosureRetention
	if err := c.call(ctx, "/infra/lifecycle/account-close-complete", binding, &result); err != nil {
		return err
	}
	if result.State != "SUCCEEDED" {
		return errors.New("ACCOUNT_CLOSURE_COMPLETION_UNCONFIRMED")
	}
	return nil
}

````

## 第 11 步：AccountClosureControl.Page

插入位置：第 10 步代码之后，紧接追加；完成稿约第 112 行。

本步声明：`func (c *AccountClosureControl) Page(ctx context.Context, request platform.AccountClosurePageRequest) (platform.AccountClosurePage, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>c.send(ctx, &quot;/infra/lifecycle/account-close-progress&quot;, raw, &amp;result)</code> |
| <code>request</code> | <code>platform.AccountClosurePageRequest</code> | 使用 <code>platform.AccountClosurePageRequest</code> 类型；其定义处的职责：控制页只暴露固定依赖编号，不复制Doc正文。游标绑定不可变计划摘要， Account逐页/逐文档恢复，不能把一页已清完误当成整个账号完成。<br>在此声明中的实际使用：<code>request.AccountClosureBinding</code>；<code>json.Marshal(request)</code>；<code>result.Matches(request)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>platform.AccountClosurePage</code> | 使用 <code>platform.AccountClosurePage</code> 类型；其定义处的职责：定义Account与Document之间的固定注销绑定和有界清理分页协议。<br>实际返回表达式：<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;ACCOUNT_CLOSURE_BINDING_INVALID&quot;)</code>；<code>err</code>；<code>errors.New(&quot;ACCOUNT_CLOSURE_PAGE_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>!platform.ValidateAccountClosureBinding(request.AccountClosureBinding)</code>。条件成立时执行 <code>return result, errors.New(&quot;ACCOUNT_CLOSURE_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>!result.Matches(request)</code>。条件成立时执行 <code>return result, errors.New(&quot;ACCOUNT_CLOSURE_PAGE_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=3976:4610 -->
````go
func (c *AccountClosureControl) Page(ctx context.Context, request platform.AccountClosurePageRequest) (platform.AccountClosurePage, error) {
	var result platform.AccountClosurePage
	if !platform.ValidateAccountClosureBinding(request.AccountClosureBinding) {
		return result, errors.New("ACCOUNT_CLOSURE_BINDING_INVALID")
	}
	raw, err := json.Marshal(request)
	if err != nil {
		return result, err
	}
	if err = c.send(ctx, "/infra/lifecycle/account-close-progress", raw, &result); err != nil {
		return result, err
	}
	if !result.Matches(request) {
		return result, errors.New("ACCOUNT_CLOSURE_PAGE_INVALID")
	}
	return result, nil
}

````

## 第 12 步：AccountClosureControl.CleanDocument

插入位置：第 11 步代码之后，紧接追加；完成稿约第 130 行。

本步声明：`func (c *AccountClosureControl) CleanDocument(ctx context.Context, binding platform.AccountClosureBinding, document string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>c.send(ctx, &quot;/infra/lifecycle/document-clean-payloads&quot;, raw, &amp;result)</code> |
| <code>binding</code> | <code>platform.AccountClosureBinding</code> | 使用 <code>platform.AccountClosureBinding</code> 类型；其定义处的职责：AccountClosureBinding只在Account和Doc控制通道传输，不由公开注销请求提供。 固定关闭组织集合防止恢复时把新加入成员的组织换入旧的清理计划。<br>在此声明中的实际使用：<code>binding.Operation</code>；<code>platform.ValidateAccountClosureBinding(binding)</code> |
| <code>document</code> | <code>string</code> | 在此声明中的实际使用：<code>Document: document</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;ACCOUNT_CLOSURE_BINDING_INVALID&quot;)</code>；<code>err</code>；<code>errors.New(&quot;ACCOUNT_CLOSURE_DOCUMENT_UNCONFIRMED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>!platform.ValidateAccountClosureBinding(binding) &#124;&#124; !request.Valid()</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_CLOSURE_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>result.AccountDocumentCleanupRequest != request &#124;&#124; !result.Valid() &#124;&#124; result.State != &quot;PAYLOADS_REMOVED&quot;</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_CLOSURE_DOCUMENT_UNCONFIRMED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=4610:5461 -->
````go
func (c *AccountClosureControl) CleanDocument(ctx context.Context, binding platform.AccountClosureBinding, document string) error {
	request := platform.AccountDocumentCleanupRequest{Operation: binding.Operation, Document: document}
	if !platform.ValidateAccountClosureBinding(binding) || !request.Valid() {
		return errors.New("ACCOUNT_CLOSURE_BINDING_INVALID")
	}
	raw, err := json.Marshal(request)
	if err != nil {
		return err
	}
	var result platform.AccountDocumentCleanupReceipt
	if err = c.send(ctx, "/infra/lifecycle/document-clean-payloads", raw, &result); err != nil {
		return err
	}
	if result.AccountDocumentCleanupRequest != request || !result.Valid() || result.State != "PAYLOADS_REMOVED" {
		return errors.New("ACCOUNT_CLOSURE_DOCUMENT_UNCONFIRMED")
	}
	return nil
}

// account_output.go：保留原实现，按业务合并文件。

````

## 第 13 步：AccountOutput

插入位置：第 12 步代码之后，紧接追加；完成稿约第 151 行。

本步声明：`type AccountOutput struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>http</code> | <code>*http.Client</code> | 注入的HTTP客户端，承载连接池、TLS和超时配置；请求沿自己的ctx取消，方法结束不应销毁共享连接池。 |
| <code>endpoint</code> | <code>string</code> | Doc内部HTTP接口基地址，由服务配置注入，不能来自浏览器参数。 |

<!-- write-step: 13 bytes=5461:5532 -->
````go
type AccountOutput struct {
	http     *http.Client
	endpoint string
}

````

## 第 14 步：NewAccountOutput

插入位置：第 13 步代码之后，紧接追加；完成稿约第 156 行。

本步声明：`func NewAccountOutput(httpClient *http.Client, endpoint string) *AccountOutput`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>httpClient</code> | <code>*http.Client</code> | 注入的HTTP客户端，承载连接池、TLS和超时配置；请求沿自己的ctx取消，方法结束不应销毁共享连接池。<br>在此声明中的实际使用：<code>http: httpClient</code> |
| <code>endpoint</code> | <code>string</code> | 在此声明中的实际使用：<code>endpoint: endpoint</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*AccountOutput</code> | 实际返回表达式：<code>&amp;AccountOutput{http: httpClient, endpoint: endpoint}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;AccountOutput{http: httpClient, endpoint: endpoint}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=5532:5677 -->
````go
func NewAccountOutput(httpClient *http.Client, endpoint string) *AccountOutput {
	return &AccountOutput{http: httpClient, endpoint: endpoint}
}

````

## 第 15 步：AccountOutput.Register

插入位置：第 14 步代码之后，紧接追加；完成稿约第 160 行。

本步声明：`func (c *AccountOutput) Register(ctx context.Context, handler string, sources []domain.OutputSource) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>http.NewRequestWithContext(ctx, &quot;POST&quot;, c.endpoint+&quot;/infra/output/register-account&quot;, bytes.NewReader(raw))</code> |
| <code>handler</code> | <code>string</code> | 在此声明中的实际使用：<code>&quot;handler_id&quot;: handler</code> |
| <code>sources</code> | <code>&#91;&#93;domain.OutputSource</code> | 使用 <code>domain.OutputSource</code> 类型；其定义处的职责：收集一次响应实际依赖的来源与代次，并记录捕获冲突。<br>在此声明中的实际使用：<code>&quot;sources&quot;: sources</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;OUTPUT_REGISTER_UNKNOWN&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;OUTPUT_REGISTER_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 登记 <code>response.Body.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>json.NewDecoder(io.LimitReader(response.Body, 1024)).Decode(&amp;ack) != nil &#124;&#124; response.StatusCode != 200 &#124;&#124; ack.State != &quot;OPEN&quot;</code>。
- 此处通过 <code>return errors.New(&quot;OUTPUT_REGISTER_UNKNOWN&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=5677:6654 -->
````go
func (c *AccountOutput) Register(ctx context.Context, handler string, sources []domain.OutputSource) error {
	raw, _ := json.Marshal(map[string]any{"handler_id": handler, "sources": sources})
	request, err := http.NewRequestWithContext(ctx, "POST", c.endpoint+"/infra/output/register-account", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(request)
	if err != nil {
		return errors.New("OUTPUT_REGISTER_UNKNOWN")
	}
	defer response.Body.Close()
	var ack struct {
		State string `json:"state"`
		Code  string `json:"code"`
	}
	if json.NewDecoder(io.LimitReader(response.Body, 1024)).Decode(&ack) != nil || response.StatusCode != 200 || ack.State != "OPEN" {
		slog.Warn("account_output_registration_pending", "status", response.StatusCode, "code", ack.Code)
		return errors.New("OUTPUT_REGISTER_UNKNOWN")
	}
	return nil
}

// departure.go：保留原实现，按业务合并文件。

````

## 第 16 步：Departure

插入位置：第 15 步代码之后，紧接追加；完成稿约第 185 行。

本步声明：`type Departure struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>http</code> | <code>*http.Client</code> | 注入的HTTP客户端，承载连接池、TLS和超时配置；请求沿自己的ctx取消，方法结束不应销毁共享连接池。 |
| <code>endpoint</code> | <code>string</code> | Doc内部HTTP接口基地址，由服务配置注入，不能来自浏览器参数。 |

<!-- write-step: 16 bytes=6654:6721 -->
````go
type Departure struct {
	http     *http.Client
	endpoint string
}

````

## 第 17 步：NewDeparture

插入位置：第 16 步代码之后，紧接追加；完成稿约第 190 行。

本步声明：`func NewDeparture(httpClient *http.Client, endpoint string) *Departure`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>httpClient</code> | <code>*http.Client</code> | 注入的HTTP客户端，承载连接池、TLS和超时配置；请求沿自己的ctx取消，方法结束不应销毁共享连接池。<br>在此声明中的实际使用：<code>http: httpClient</code> |
| <code>endpoint</code> | <code>string</code> | 在此声明中的实际使用：<code>endpoint: endpoint</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Departure</code> | 实际返回表达式：<code>&amp;Departure{http: httpClient, endpoint: endpoint}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Departure{http: httpClient, endpoint: endpoint}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 17 bytes=6721:6854 -->
````go
func NewDeparture(httpClient *http.Client, endpoint string) *Departure {
	return &Departure{http: httpClient, endpoint: endpoint}
}

````

## 第 18 步：Departure.Check

插入位置：第 17 步代码之后，紧接追加；完成稿约第 194 行。

本步声明：`func (c *Departure) Check(ctx context.Context, organization, user, operation string, epoch uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx, cancel := context.WithTimeout(ctx, 1200*time.Millisecond)</code>；<code>context.WithTimeout(ctx, 1200*time.Millisecond)</code>；<code>http.NewRequestWithContext(ctx, &quot;GET&quot;, endpoint, nil)</code> |
| <code>organization</code> | <code>string</code> | 在此声明中的实际使用：<code>url.QueryEscape(organization)</code> |
| <code>user</code> | <code>string</code> | 在此声明中的实际使用：<code>url.QueryEscape(user)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>url.QueryEscape(operation)</code> |
| <code>epoch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>strconv.FormatUint(epoch, 10)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;DEPARTURE_CHECK_UNAVAILABLE&quot;)</code>；<code>domain.RuleViolation{Kind: domain.StateConflict, Code: &quot;ADMIN_RETIREMENT_REQUIRED&quot;}</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>context.WithTimeout(ctx, 1200*time.Millisecond)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;DEPARTURE_CHECK_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 登记 <code>response.Body.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>response.StatusCode != 200</code>。条件成立时执行 <code>return errors.New(&quot;DEPARTURE_CHECK_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>json.NewDecoder(io.LimitReader(response.Body, 1024)).Decode(&amp;value) != nil</code>。条件成立时执行 <code>return errors.New(&quot;DEPARTURE_CHECK_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>value.RequiresHandoff</code>。条件成立时执行 <code>return domain.RuleViolation{Kind: domain.StateConflict, Code: &quot;ADMIN_RETIREMENT_REQUIRED&quot;}</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=6854:8177 -->
````go
func (c *Departure) Check(ctx context.Context, organization, user, operation string, epoch uint64) error {
	// 调用方仍持有组织锁；Doc须锁住同一member来源，不能用两次普通读取代替互斥。
	ctx, cancel := context.WithTimeout(ctx, 1200*time.Millisecond)
	defer cancel()
	endpoint := c.endpoint + "/infra/lifecycle/member-departure-eligibility?organization_id=" + url.QueryEscape(organization) + "&user_id=" + url.QueryEscape(user) + "&operation_id=" + url.QueryEscape(operation) + "&epoch=" + strconv.FormatUint(epoch, 10)
	request, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return errors.New("DEPARTURE_CHECK_UNAVAILABLE")
	}
	response, err := c.http.Do(request)
	if err != nil {
		return errors.New("DEPARTURE_CHECK_UNAVAILABLE")
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return errors.New("DEPARTURE_CHECK_UNAVAILABLE")
	}
	var value struct {
		RequiresHandoff bool `json:"requires_handoff"`
	}
	if json.NewDecoder(io.LimitReader(response.Body, 1024)).Decode(&value) != nil {
		return errors.New("DEPARTURE_CHECK_UNAVAILABLE")
	}
	if value.RequiresHandoff {
		return domain.RuleViolation{Kind: domain.StateConflict, Code: "ADMIN_RETIREMENT_REQUIRED"}
	}
	return nil
}

// document.go：保留原实现，按业务合并文件。

````

## 第 19 步：Document

插入位置：第 18 步代码之后，紧接追加；完成稿约第 225 行。

本步声明：`type Document struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>rpc</code> | <code>pb.DocumentInternalClient</code> | 已建立内部TLS连接的Document gRPC客户端，执行跨服务关闭/核验。 |
| <code>http</code> | <code>*http.Client</code> | 注入的HTTP客户端，承载连接池、TLS和超时配置；请求沿自己的ctx取消，方法结束不应销毁共享连接池。 |
| <code>endpoint</code> | <code>string</code> | Doc内部HTTP接口基地址，由服务配置注入，不能来自浏览器参数。 |

<!-- write-step: 19 bytes=8177:8279 -->
````go
type Document struct {
	rpc      pb.DocumentInternalClient
	http     *http.Client
	endpoint string
}

````

## 第 20 步：NewDocument

插入位置：第 19 步代码之后，紧接追加；完成稿约第 231 行。

本步声明：`func NewDocument(rpc pb.DocumentInternalClient, httpClient *http.Client, endpoint string) *Document`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>rpc</code> | <code>pb.DocumentInternalClient</code> | 在此声明中的实际使用：<code>rpc: rpc</code> |
| <code>httpClient</code> | <code>*http.Client</code> | 注入的HTTP客户端，承载连接池、TLS和超时配置；请求沿自己的ctx取消，方法结束不应销毁共享连接池。<br>在此声明中的实际使用：<code>http: httpClient</code> |
| <code>endpoint</code> | <code>string</code> | 在此声明中的实际使用：<code>endpoint: endpoint</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Document</code> | 实际返回表达式：<code>&amp;Document{rpc: rpc, http: httpClient, endpoint: endpoint}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Document{rpc: rpc, http: httpClient, endpoint: endpoint}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 20 bytes=8279:8450 -->
````go
func NewDocument(rpc pb.DocumentInternalClient, httpClient *http.Client, endpoint string) *Document {
	return &Document{rpc: rpc, http: httpClient, endpoint: endpoint}
}

````

## 第 21 步：Document.FenceAccess

插入位置：第 20 步代码之后，紧接追加；完成稿约第 235 行。

本步声明：`func (c *Document) FenceAccess(ctx context.Context, request *pb.FenceRequest) (*pb.FenceResult, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>c.rpc.FenceAccess(ctx, request)</code> |
| <code>request</code> | <code>*pb.FenceRequest</code> | 在此声明中的实际使用：<code>c.rpc.FenceAccess(ctx, request)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.FenceResult</code> | 由 <code>c.rpc.FenceAccess(ctx, request)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>c.rpc.FenceAccess(ctx, request)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return c.rpc.FenceAccess(ctx, request)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 21 bytes=8450:8598 -->
````go
func (c *Document) FenceAccess(ctx context.Context, request *pb.FenceRequest) (*pb.FenceResult, error) {
	return c.rpc.FenceAccess(ctx, request)
}

````

## 第 22 步：Document.FailureCode

插入位置：第 21 步代码之后，紧接追加；完成稿约第 239 行。

本步声明：`func (c *Document) FailureCode(err error) string`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>err</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>在此声明中的实际使用：<code>status.Code(err)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>status.Code(err).String()</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return status.Code(err).String()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 22 bytes=8598:8685 -->
````go
func (c *Document) FailureCode(err error) string { return status.Code(err).String() }

````

## 第 23 步：Document.CloseDepartedGrants

插入位置：第 22 步代码之后，紧接追加；完成稿约第 241 行。

本步声明：`func (c *Document) CloseDepartedGrants(ctx context.Context, binding domain.SessionClosure, operation string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>http.NewRequestWithContext(ctx, &quot;POST&quot;, endpoint, nil)</code> |
| <code>binding</code> | <code>domain.SessionClosure</code> | 使用 <code>domain.SessionClosure</code> 类型；其定义处的职责：SessionClosure是持久任务的固定绑定。后续重试不能改写原scope或关闭代次。<br>在此声明中的实际使用：<code>binding.OrganizationID</code>；<code>binding.UserID</code>；<code>binding.Epoch</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>url.QueryEscape(operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;DEPARTURE_GRANTS_UNKNOWN&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>response.Body.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>response.StatusCode != 200</code>。条件成立时执行 <code>return errors.New(&quot;DEPARTURE_GRANTS_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 23 bytes=8685:9529 -->
````go
func (c *Document) CloseDepartedGrants(ctx context.Context, binding domain.SessionClosure, operation string) error {
	// 客户端继续使用平台装配的mTLS连接；浏览器声明不能代替账号服务证书。
	endpoint := c.endpoint + "/infra/lifecycle/member-departure-close?organization_id=" + url.QueryEscape(binding.OrganizationID) + "&user_id=" + url.QueryEscape(binding.UserID) + "&operation_id=" + url.QueryEscape(operation) + "&epoch=" + binding.Epoch
	request, err := http.NewRequestWithContext(ctx, "POST", endpoint, nil)
	if err != nil {
		return err
	}
	response, err := c.http.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return errors.New("DEPARTURE_GRANTS_UNKNOWN")
	}
	return nil
}

// organization_close.go：保留原实现，按业务合并文件。

````

## 第 24 步：OrganizationCloseControl

插入位置：第 23 步代码之后，紧接追加；完成稿约第 261 行。

传输复用同一mTLS与严格完整JSON读取器；绑定协议和路由属于独立组织关闭。

本步声明：`type OrganizationCloseControl struct{ transport *AccountClosureControl }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>transport</code> | <code>*AccountClosureControl</code> | 复用账号关闭客户端的受控HTTP传输；业务请求仍使用组织关闭自己的绑定结构和路径。 |

<!-- write-step: 24 bytes=9529:9708 -->
````go
// 传输复用同一mTLS与严格完整JSON读取器；绑定协议和路由属于独立组织关闭。
type OrganizationCloseControl struct{ transport *AccountClosureControl }

````

## 第 25 步：NewOrganizationCloseControl

插入位置：第 24 步代码之后，紧接追加；完成稿约第 264 行。

本步声明：`func NewOrganizationCloseControl(httpClient *http.Client, endpoint string) *OrganizationCloseControl`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>httpClient</code> | <code>*http.Client</code> | 注入的HTTP客户端，承载连接池、TLS和超时配置；请求沿自己的ctx取消，方法结束不应销毁共享连接池。<br>在此声明中的实际使用：<code>NewAccountClosureControl(httpClient, endpoint)</code> |
| <code>endpoint</code> | <code>string</code> | 在此声明中的实际使用：<code>NewAccountClosureControl(httpClient, endpoint)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*OrganizationCloseControl</code> | 使用 <code>OrganizationCloseControl</code> 类型；其定义处的职责：传输复用同一mTLS与严格完整JSON读取器；绑定协议和路由属于独立组织关闭。<br>实际返回表达式：<code>&amp;OrganizationCloseControl{transport: NewAccountClosureControl(httpClient, endpoint)}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;OrganizationCloseControl{transport: NewAccountClosureControl(httpClient, endpoint)}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 25 bytes=9708:9906 -->
````go
func NewOrganizationCloseControl(httpClient *http.Client, endpoint string) *OrganizationCloseControl {
	return &OrganizationCloseControl{transport: NewAccountClosureControl(httpClient, endpoint)}
}
````

## 第 26 步：OrganizationCloseControl.call

插入位置：第 25 步代码之后，紧接追加；完成稿约第 267 行。

本步声明：`func (c *OrganizationCloseControl) call(ctx context.Context, path string, b platform.OrganizationClosureBinding, result any) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>c.transport.send(ctx, path, raw, result)</code> |
| <code>path</code> | <code>string</code> | 在此声明中的实际使用：<code>c.transport.send(ctx, path, raw, result)</code> |
| <code>b</code> | <code>platform.OrganizationClosureBinding</code> | 使用 <code>platform.OrganizationClosureBinding</code> 类型；其定义处的职责：OrganizationClosureBinding 来自 Account 锁内固定的 owner/成员事实；用户不提交范围。 Epoch 是原组织成员来源的关闭代次，绝不代表账号身份被关闭。<br>在此声明中的实际使用：<code>b.Valid</code>；<code>json.Marshal(b)</code> |
| <code>result</code> | <code>any</code> | 在此声明中的实际使用：<code>c.transport.send(ctx, path, raw, result)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;ORGANIZATION_CLOSE_BINDING_INVALID&quot;)</code>；<code>err</code>；<code>c.transport.send(ctx, path, raw, result)</code> |

**跟着本段代码走一遍：**

- 检查 <code>!b.Valid()</code>。条件成立时执行 <code>return errors.New(&quot;ORGANIZATION_CLOSE_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return c.transport.send(ctx, path, raw, result)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 26 bytes=9906:10230 -->
````go
func (c *OrganizationCloseControl) call(ctx context.Context, path string, b platform.OrganizationClosureBinding, result any) error {
	if !b.Valid() {
		return errors.New("ORGANIZATION_CLOSE_BINDING_INVALID")
	}
	raw, err := json.Marshal(b)
	if err != nil {
		return err
	}
	return c.transport.send(ctx, path, raw, result)
}
````

## 第 27 步：OrganizationCloseControl.Plan

插入位置：第 26 步代码之后，紧接追加；完成稿约第 277 行。

本步声明：`func (c *OrganizationCloseControl) Plan(ctx context.Context, b platform.OrganizationClosureBinding) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>c.call(ctx, &quot;/infra/lifecycle/organization-close-plan&quot;, b, &amp;result)</code> |
| <code>b</code> | <code>platform.OrganizationClosureBinding</code> | 使用 <code>platform.OrganizationClosureBinding</code> 类型；其定义处的职责：OrganizationClosureBinding 来自 Account 锁内固定的 owner/成员事实；用户不提交范围。 Epoch 是原组织成员来源的关闭代次，绝不代表账号身份被关闭。<br>在此声明中的实际使用：<code>c.call(ctx, &quot;/infra/lifecycle/organization-close-plan&quot;, b, &amp;result)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;ORGANIZATION_CLOSE_PLAN_UNCONFIRMED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>result.State != &quot;PLANNED&quot; &amp;&amp; result.State != &quot;CLEANING&quot; &amp;&amp; result.State != &quot;SUCCEEDED&quot;</code>。条件成立时执行 <code>return errors.New(&quot;ORGANIZATION_CLOSE_PLAN_UNCONFIRMED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 27 bytes=10230:10662 -->
````go
func (c *OrganizationCloseControl) Plan(ctx context.Context, b platform.OrganizationClosureBinding) error {
	var result platform.OrganizationClosureState
	if err := c.call(ctx, "/infra/lifecycle/organization-close-plan", b, &result); err != nil {
		return err
	}
	if result.State != "PLANNED" && result.State != "CLEANING" && result.State != "SUCCEEDED" {
		return errors.New("ORGANIZATION_CLOSE_PLAN_UNCONFIRMED")
	}
	return nil
}
````

## 第 28 步：OrganizationCloseControl.Complete

插入位置：第 27 步代码之后，紧接追加；完成稿约第 287 行。

本步声明：`func (c *OrganizationCloseControl) Complete(ctx context.Context, b platform.OrganizationClosureBinding) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>c.call(ctx, &quot;/infra/lifecycle/organization-close-complete&quot;, b, &amp;result)</code> |
| <code>b</code> | <code>platform.OrganizationClosureBinding</code> | 使用 <code>platform.OrganizationClosureBinding</code> 类型；其定义处的职责：OrganizationClosureBinding 来自 Account 锁内固定的 owner/成员事实；用户不提交范围。 Epoch 是原组织成员来源的关闭代次，绝不代表账号身份被关闭。<br>在此声明中的实际使用：<code>c.call(ctx, &quot;/infra/lifecycle/organization-close-complete&quot;, b, &amp;result)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;ORGANIZATION_CLOSE_COMPLETE_UNCONFIRMED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>result.State != &quot;SUCCEEDED&quot;</code>。条件成立时执行 <code>return errors.New(&quot;ORGANIZATION_CLOSE_COMPLETE_UNCONFIRMED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 28 bytes=10662:11047 -->
````go
func (c *OrganizationCloseControl) Complete(ctx context.Context, b platform.OrganizationClosureBinding) error {
	var result platform.OrganizationClosureState
	if err := c.call(ctx, "/infra/lifecycle/organization-close-complete", b, &result); err != nil {
		return err
	}
	if result.State != "SUCCEEDED" {
		return errors.New("ORGANIZATION_CLOSE_COMPLETE_UNCONFIRMED")
	}
	return nil
}
````

## 第 29 步：OrganizationCloseControl.Page

插入位置：第 28 步代码之后，紧接追加；完成稿约第 297 行。

本步声明：`func (c *OrganizationCloseControl) Page(ctx context.Context, request platform.OrganizationClosurePageRequest) (platform.OrganizationClosurePage, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>c.transport.send(ctx, &quot;/infra/lifecycle/organization-close-progress&quot;, raw, &amp;result)</code> |
| <code>request</code> | <code>platform.OrganizationClosurePageRequest</code> | 使用 <code>platform.OrganizationClosurePageRequest</code> 类型；其定义处的职责：集中定义跨服务清理、通知资格、草稿计量、组织关闭和转让存储测量协议。<br>在此声明中的实际使用：<code>request.Binding</code>；<code>json.Marshal(request)</code>；<code>result.Matches(request)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>platform.OrganizationClosurePage</code> | 使用 <code>platform.OrganizationClosurePage</code> 类型；其定义处的职责：集中定义跨服务清理、通知资格、草稿计量、组织关闭和转让存储测量协议。<br>实际返回表达式：<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;ORGANIZATION_CLOSE_BINDING_INVALID&quot;)</code>；<code>err</code>；<code>errors.New(&quot;ORGANIZATION_CLOSE_PAGE_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>!request.Binding.Valid()</code>。条件成立时执行 <code>return result, errors.New(&quot;ORGANIZATION_CLOSE_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>!result.Matches(request)</code>。条件成立时执行 <code>return result, errors.New(&quot;ORGANIZATION_CLOSE_PAGE_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 29 bytes=11047:11673 -->
````go
func (c *OrganizationCloseControl) Page(ctx context.Context, request platform.OrganizationClosurePageRequest) (platform.OrganizationClosurePage, error) {
	var result platform.OrganizationClosurePage
	if !request.Binding.Valid() {
		return result, errors.New("ORGANIZATION_CLOSE_BINDING_INVALID")
	}
	raw, err := json.Marshal(request)
	if err != nil {
		return result, err
	}
	if err = c.transport.send(ctx, "/infra/lifecycle/organization-close-progress", raw, &result); err != nil {
		return result, err
	}
	if !result.Matches(request) {
		return result, errors.New("ORGANIZATION_CLOSE_PAGE_INVALID")
	}
	return result, nil
}
````

## 第 30 步：OrganizationCloseControl.CleanDocument

插入位置：第 29 步代码之后，紧接追加；完成稿约第 314 行。

本步声明：`func (c *OrganizationCloseControl) CleanDocument(ctx context.Context, b platform.OrganizationClosureBinding, document string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>c.transport.send(ctx, &quot;/infra/lifecycle/organization-close-clean-payloads&quot;, raw, &amp;result)</code> |
| <code>b</code> | <code>platform.OrganizationClosureBinding</code> | 使用 <code>platform.OrganizationClosureBinding</code> 类型；其定义处的职责：OrganizationClosureBinding 来自 Account 锁内固定的 owner/成员事实；用户不提交范围。 Epoch 是原组织成员来源的关闭代次，绝不代表账号身份被关闭。<br>在此声明中的实际使用：<code>b.Operation</code>；<code>b.Valid</code> |
| <code>document</code> | <code>string</code> | 在此声明中的实际使用：<code>Document: document</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;ORGANIZATION_CLOSE_BINDING_INVALID&quot;)</code>；<code>err</code>；<code>errors.New(&quot;ORGANIZATION_CLOSE_CLEANUP_UNCONFIRMED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>!b.Valid() &#124;&#124; !request.Valid()</code>。条件成立时执行 <code>return errors.New(&quot;ORGANIZATION_CLOSE_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>result.AccountDocumentCleanupRequest != request &#124;&#124; !result.Valid() &#124;&#124; result.State != &quot;PAYLOADS_REMOVED&quot;</code>。条件成立时执行 <code>return errors.New(&quot;ORGANIZATION_CLOSE_CLEANUP_UNCONFIRMED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 30 bytes=11673:12439 -->
````go
func (c *OrganizationCloseControl) CleanDocument(ctx context.Context, b platform.OrganizationClosureBinding, document string) error {
	request := platform.AccountDocumentCleanupRequest{Operation: b.Operation, Document: document}
	if !b.Valid() || !request.Valid() {
		return errors.New("ORGANIZATION_CLOSE_BINDING_INVALID")
	}
	raw, err := json.Marshal(request)
	if err != nil {
		return err
	}
	var result platform.AccountDocumentCleanupReceipt
	if err = c.transport.send(ctx, "/infra/lifecycle/organization-close-clean-payloads", raw, &result); err != nil {
		return err
	}
	if result.AccountDocumentCleanupRequest != request || !result.Valid() || result.State != "PAYLOADS_REMOVED" {
		return errors.New("ORGANIZATION_CLOSE_CLEANUP_UNCONFIRMED")
	}
	return nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/account/internal/client/document_authority_client.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/account/07-closure-recovery.md)。
