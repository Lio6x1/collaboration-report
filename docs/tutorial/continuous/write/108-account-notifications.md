# 分步编写 `services/document/internal/client/account_notifications.go`

先按 [D09 · 固定批注、提及与可靠通知](../lessons/document/09-comments-notifications.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/108-account-notifications.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

通过mTLS读取Account未确认事件并确认已导入行。

按未确认队列读取可避免较小ID晚提交被单调高水位永久跳过。

写之前先按这条执行路线理解各段的连接：

1. Events调用私有事件接口并校验批次
2. AckEvents发送原ID集合
3. accountNotificationPost统一有界严格解码。

本文件需要守住的边界：

- 不能以lastID游标替换未确认语义
- 事件客户端不构造浏览器会话。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/client' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/client/account_notifications.go') -and (Get-Item -LiteralPath 'services/document/internal/client/account_notifications.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/client/account_notifications.go')) { New-Item -ItemType File -Path 'services/document/internal/client/account_notifications.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:129 -->
````go
package client

import (
	"bytes"
	"collaboration/platform"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
)

````

## 第 2 步：accountNotificationEventPattern

插入位置：第 1 步代码之后，紧接追加；完成稿约第 14 行。

本步声明：`var accountNotificationEventPattern = regexp.MustCompile(\`^account-owner:[a-f0-9]{64}$\`)`。

<!-- write-step: 2 bytes=129:219 -->
````go
var accountNotificationEventPattern = regexp.MustCompile(`^account-owner:[a-f0-9]{64}$`)

````

## 第 3 步：NotificationRecipients.Events

插入位置：第 2 步代码之后，紧接追加；完成稿约第 16 行。

Events只通过已有document服务mTLS客户端读取Account原始事实，不构造用户会话。
没有高水位：迟提交的小ID事件仍会留在Account未确认队列中，供下一轮获取。

本步声明：`func (c *NotificationRecipients) Events(ctx context.Context) ([]platform.AccountNotificationEvent, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>c.accountNotificationPost(ctx, &quot;/infra/notification/events&quot;, struct{}{}, &amp;value)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;platform.AccountNotificationEvent</code> | 使用 <code>platform.AccountNotificationEvent</code> 类型；其定义处的职责：Account原始事件只携带稳定标识；正文和组织名称不跨服务复制。<br>实际返回表达式：<code>nil</code>；<code>value.Events</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;ACCOUNT_NOTIFICATION_RESPONSE_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>value.Events == nil &#124;&#124; len(value.Events) &gt; 100</code>。条件成立时执行 <code>return nil, errors.New(&quot;ACCOUNT_NOTIFICATION_RESPONSE_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>event.ID == 0 &#124;&#124; event.RecipientID == 0 &#124;&#124; event.OrganizationID == 0 &#124;&#124; seen&#91;event.ID&#93; &#124;&#124; event.Kind != &quot;ORG_OWNER_TRANSFER&quot; &#124;&#124; !accountNotificationEventPattern.MatchString(event.EventID)</code>。条件成立时执行 <code>return nil, errors.New(&quot;ACCOUNT_NOTIFICATION_RESPONSE_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return value.Events, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=219:1364 -->
````go
// Events只通过已有document服务mTLS客户端读取Account原始事实，不构造用户会话。
// 没有高水位：迟提交的小ID事件仍会留在Account未确认队列中，供下一轮获取。
func (c *NotificationRecipients) Events(ctx context.Context) ([]platform.AccountNotificationEvent, error) {
	var value platform.AccountNotificationEventsResponse
	if err := c.accountNotificationPost(ctx, "/infra/notification/events", struct{}{}, &value); err != nil {
		return nil, err
	}
	if value.Events == nil || len(value.Events) > 100 {
		return nil, errors.New("ACCOUNT_NOTIFICATION_RESPONSE_INVALID")
	}
	seen := make(map[uint64]bool, len(value.Events))
	for _, event := range value.Events {
		// uint64解码已拒绝符号、前导零、小数和溢出；零值及重复原始行号仍须显式拒绝。
		if event.ID == 0 || event.RecipientID == 0 || event.OrganizationID == 0 || seen[event.ID] || event.Kind != "ORG_OWNER_TRANSFER" || !accountNotificationEventPattern.MatchString(event.EventID) {
			return nil, errors.New("ACCOUNT_NOTIFICATION_RESPONSE_INVALID")
		}
		seen[event.ID] = true
	}
	return value.Events, nil
}

````

## 第 4 步：NotificationRecipients.AckEvents

插入位置：第 3 步代码之后，紧接追加；完成稿约第 37 行。

本步声明：`func (c *NotificationRecipients) AckEvents(ctx context.Context, ids []uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>c.accountNotificationPost(ctx, &quot;/infra/notification/events/ack&quot;, platform.AccountNotificationAckRequest{IDs: ids}, &amp;value)</code> |
| <code>ids</code> | <code>&#91;&#93;uint64</code> | 在此声明中的实际使用：<code>len(ids)</code>；<code>IDs: ids</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;ACCOUNT_NOTIFICATION_ACK_INVALID&quot;)</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(ids) == 0 &#124;&#124; len(ids) &gt; 100</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_NOTIFICATION_ACK_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>id == 0 &#124;&#124; seen&#91;id&#93;</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_NOTIFICATION_ACK_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!value.Acknowledged</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_NOTIFICATION_ACK_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=1364:2038 -->
````go
func (c *NotificationRecipients) AckEvents(ctx context.Context, ids []uint64) error {
	if len(ids) == 0 || len(ids) > 100 {
		return errors.New("ACCOUNT_NOTIFICATION_ACK_INVALID")
	}
	seen := make(map[uint64]bool, len(ids))
	for _, id := range ids {
		if id == 0 || seen[id] {
			return errors.New("ACCOUNT_NOTIFICATION_ACK_INVALID")
		}
		seen[id] = true
	}
	var value platform.AccountNotificationAckResponse
	if err := c.accountNotificationPost(ctx, "/infra/notification/events/ack", platform.AccountNotificationAckRequest{IDs: ids}, &value); err != nil {
		return err
	}
	if !value.Acknowledged {
		return errors.New("ACCOUNT_NOTIFICATION_ACK_INVALID")
	}
	return nil
}

````

## 第 5 步：NotificationRecipients.accountNotificationPost

插入位置：第 4 步代码之后，紧接追加；完成稿约第 58 行。

本步声明：`func (c *NotificationRecipients) accountNotificationPost(ctx context.Context, path string, input, output any) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(raw))</code> |
| <code>path</code> | <code>string</code> | 在此声明中的实际使用：<code>c.baseURL+path</code> |
| <code>input</code> | <code>any</code> | 在此声明中的实际使用：<code>json.Marshal(input)</code> |
| <code>output</code> | <code>any</code> | 在此声明中的实际使用：<code>decoder.Decode(output)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;ACCOUNT_NOTIFICATION_UNAVAILABLE&quot;)</code>；<code>errors.New(&quot;ACCOUNT_NOTIFICATION_RESPONSE_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>response.Body.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>response.StatusCode != http.StatusOK</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_NOTIFICATION_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>len(body) &gt; limit</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_NOTIFICATION_RESPONSE_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>decoder.Decode(output) != nil &#124;&#124; decoder.Decode(new(any)) != io.EOF</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_NOTIFICATION_RESPONSE_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=2038:3142 -->
````go
func (c *NotificationRecipients) accountNotificationPost(ctx context.Context, path string, input, output any) error {
	raw, err := json.Marshal(input)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return errors.New("ACCOUNT_NOTIFICATION_UNAVAILABLE")
	}
	// 多读一个字节检测超限，不能让LimitReader的人工EOF掩盖响应尾部。
	const limit = 128 << 10
	body, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return err
	}
	if len(body) > limit {
		return errors.New("ACCOUNT_NOTIFICATION_RESPONSE_INVALID")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if decoder.Decode(output) != nil || decoder.Decode(new(any)) != io.EOF {
		return errors.New("ACCOUNT_NOTIFICATION_RESPONSE_INVALID")
	}
	return nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/client/account_notifications.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/09-comments-notifications.md)。
