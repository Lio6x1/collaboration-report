# 分步编写 `services/document/internal/domain/document/discussion_models.go`

先按 [D09 · 固定批注、提及与可靠通知](../lessons/document/09-comments-notifications.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/091-discussion-models.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

定义固定评论通知、批注正文规则和原版本锚点校验。

批注定位的是当时不可变正文，普通讨论不应拥有正式审核change_request的阻塞或解除能力。

写之前先按这条执行路线理解各段的连接：

1. ValidateCommentBody校验用户文本
2. ValidateCommentAnchor在固定正文树查块/范围
3. CommentNotification固定thread/comment和显式mentions。

本文件需要守住的边界：

- 后续编辑评论不能重新猜原通知收件人
- 锚点损坏不能回退为当前正文。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/domain/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/domain/document/discussion_models.go') -and (Get-Item -LiteralPath 'services/document/internal/domain/document/discussion_models.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/domain/document/discussion_models.go')) { New-Item -ItemType File -Path 'services/document/internal/domain/document/discussion_models.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:300 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	dto "collaboration/contracts/httpdto"
	"encoding/json"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// comment_notification.go：保留原实现，按业务合并文件。

````

## 第 2 步：CommentNotification

插入位置：第 1 步代码之后，紧接追加；完成稿约第 14 行。

CommentNotification固定创建/回复时的原讨论与显式@集合；恢复不能读取后来编辑的正文重新猜收件人。

本步声明：`type CommentNotification struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Document</code> | <code>uint64</code> | 评论原文档<br>序列化/校验标记：<code>`json:&quot;document_id&quot;`</code> |
| <code>Thread</code> | <code>uint64</code> | 原讨论线程<br>序列化/校验标记：<code>`json:&quot;thread_id&quot;`</code> |
| <code>Comment</code> | <code>uint64</code> | 产生事件的评论<br>序列化/校验标记：<code>`json:&quot;comment_id&quot;`</code> |
| <code>ContextKind</code> | <code>string</code> | MAIN、BRANCH或REVIEW<br>序列化/校验标记：<code>`json:&quot;context_kind&quot;`</code> |
| <code>ContextID</code> | <code>string</code> | 该类原上下文引用<br>序列化/校验标记：<code>`json:&quot;context_id&quot;`</code> |
| <code>Version</code> | <code>uint64</code> | 锚点固定版本<br>序列化/校验标记：<code>`json:&quot;content_version_id&quot;`</code> |
| <code>Actor</code> | <code>uint64</code> | 触发评论事件的用户<br>序列化/校验标记：<code>`json:&quot;actor_id&quot;`</code> |
| <code>Recipients</code> | <code>&#91;&#93;uint64</code> | 显式提及等原固定收件集合<br>序列化/校验标记：<code>`json:&quot;recipients&quot;`</code> |
| <code>ReplyRecipients</code> | <code>&#91;&#93;uint64</code> | 回复参与者固定集合<br>序列化/校验标记：<code>`json:&quot;reply_recipients,omitempty&quot;`</code> |
| <code>DeliveryAfter</code> | <code>uint64</code> | 已持久投递到的用户ID游标<br>序列化/校验标记：<code>`json:&quot;delivery_after,omitempty&quot;`</code> |

<!-- write-step: 2 bytes=300:1135 -->
````go
// CommentNotification固定创建/回复时的原讨论与显式@集合；恢复不能读取后来编辑的正文重新猜收件人。
type CommentNotification struct {
	Document        uint64   `json:"document_id"`
	Thread          uint64   `json:"thread_id"`
	Comment         uint64   `json:"comment_id"`
	ContextKind     string   `json:"context_kind"`
	ContextID       string   `json:"context_id"`
	Version         uint64   `json:"content_version_id"`
	Actor           uint64   `json:"actor_id"`
	Recipients      []uint64 `json:"recipients"`
	ReplyRecipients []uint64 `json:"reply_recipients,omitempty"`
	// 超过一批的收件人逐批提交，恢复只推进已持久化游标，不依赖进程内列表。
	DeliveryAfter uint64 `json:"delivery_after,omitempty"`
}

// comments.go：保留原实现，按业务合并文件。

````

## 第 3 步：ValidateCommentBody

插入位置：第 2 步代码之后，紧接追加；完成稿约第 31 行。

普通批注只描述讨论，不携带正式change_request的状态或解除权限。

本步声明：`func ValidateCommentBody(body string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>body</code> | <code>string</code> | 在此声明中的实际使用：<code>utf8.ValidString(body)</code>；<code>strings.TrimSpace(body)</code>；<code>utf8.RuneCountInString(body)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>Violation(InvalidRequest, &quot;COMMENT_BODY_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>!utf8.ValidString(body) &#124;&#124; strings.TrimSpace(body) == &quot;&quot; &#124;&#124; utf8.RuneCountInString(body) &gt; 8000</code>。条件成立时执行 <code>return Violation(InvalidRequest, &quot;COMMENT_BODY_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=1135:1450 -->
````go
// 普通批注只描述讨论，不携带正式change_request的状态或解除权限。
func ValidateCommentBody(body string) error {
	if !utf8.ValidString(body) || strings.TrimSpace(body) == "" || utf8.RuneCountInString(body) > 8000 {
		return Violation(InvalidRequest, "COMMENT_BODY_INVALID")
	}
	return nil
}

````

## 第 4 步：ValidateCommentAnchor

插入位置：第 3 步代码之后，紧接追加；完成稿约第 39 行。

本步声明：`func ValidateCommentAnchor(anchor dto.Anchor, body []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>anchor</code> | <code>dto.Anchor</code> | 在此声明中的实际使用：<code>anchor.SchemaVersion</code>；<code>anchor.ContentVersionId</code>；<code>anchor.Quote</code>；<code>anchor.BlockId</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>body</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>json.Unmarshal(body, &amp;root)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>invalid</code>；<code>nil</code>；<code>Violation(TemporarilyUnavailable, &quot;COMMENT_CONTENT_INVALID&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>anchor.SchemaVersion != &quot;anchor-v1&quot;</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>anchor.Quote != nil &amp;&amp; (!utf8.ValidString(*anchor.Quote) &#124;&#124; utf8.RuneCountInString(*anchor.Quote) &gt; 500)</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>anchor.BlockId == nil</code>。
- 检查 <code>anchor.StartOffset != nil &#124;&#124; anchor.EndOffset != nil &#124;&#124; anchor.Quote != nil</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!ValidName(*anchor.BlockId, 96) &#124;&#124; (anchor.StartOffset == nil) != (anchor.EndOffset == nil)</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(body, &amp;root) != nil</code>。条件成立时执行 <code>return Violation(TemporarilyUnavailable, &quot;COMMENT_CONTENT_INVALID&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>ok &amp;&amp; id == *anchor.BlockId</code>。
- 检查 <code>found == nil</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>anchor.StartOffset == nil</code>。
- 检查 <code>anchor.Quote != nil</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=1450:2780 -->
````go
func ValidateCommentAnchor(anchor dto.Anchor, body []byte) error {
	invalid := Violation(InvalidRequest, "COMMENT_ANCHOR_INVALID")
	if anchor.SchemaVersion != "anchor-v1" {
		return invalid
	}
	if _, err := CanonicalID(anchor.ContentVersionId); err != nil {
		return invalid
	}
	if anchor.Quote != nil && (!utf8.ValidString(*anchor.Quote) || utf8.RuneCountInString(*anchor.Quote) > 500) {
		return invalid
	}
	if anchor.BlockId == nil {
		if anchor.StartOffset != nil || anchor.EndOffset != nil || anchor.Quote != nil {
			return invalid
		}
		return nil
	}
	if !ValidName(*anchor.BlockId, 96) || (anchor.StartOffset == nil) != (anchor.EndOffset == nil) {
		return invalid
	}
	var root structNode
	if json.Unmarshal(body, &root) != nil {
		return Violation(TemporarilyUnavailable, "COMMENT_CONTENT_INVALID")
	}
	var found *structNode
	var visit func(*structNode)
	visit = func(node *structNode) {
		if id, ok := node.Attrs["id"].(string); ok && id == *anchor.BlockId {
			found = node
		}
		for i := range node.Content {
			visit(&node.Content[i])
		}
	}
	visit(&root)
	if found == nil {
		return invalid
	}
	if anchor.StartOffset == nil {
		if anchor.Quote != nil {
			return invalid
		}
		return nil
	}
	// 浏览器选区偏移按UTF-16单位核验；原子图片/公式占一个占位符，不能按UTF-8字节截断中文。
````

## 第 5 步：ValidateCommentAnchor（接着实现）

插入位置：第 4 步代码之后，紧接追加；完成稿约第 84 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：检查 <code>node.Type == &quot;text&quot;</code>。条件成立时执行 <code>return node.Text</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>node.Type == &quot;image&quot; &#124;&#124; node.Type == &quot;formula&quot;</code>。条件成立时执行 <code>return &quot;\ufffc&quot;</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>node.Type == &quot;hard_break&quot;</code>。条件成立时执行 <code>return &quot;\n&quot;</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return out.String()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>start &lt; 0 &#124;&#124; end &lt; start &#124;&#124; end &gt; int64(len(units))</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return index == 0 &#124;&#124; index == int64(len(units)) &#124;&#124; !(units&#91;index-1&#93; &gt;= 0xd800 &amp;&amp; units&#91;index-1&#93; &lt;= 0xdbff &amp;&amp; units&#91;index&#93; &gt;= 0xdc00 &amp;&amp; units&#91;index&#93; &lt;= 0xdfff)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!boundary(start) &#124;&#124; !boundary(end)</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>anchor.Quote != nil &amp;&amp; string(utf16.Decode(units&#91;start:end&#93;)) != *anchor.Quote</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=2780:3719 -->
````go
	var text func(structNode) string
	text = func(node structNode) string {
		if node.Type == "text" {
			return node.Text
		}
		if node.Type == "image" || node.Type == "formula" {
			return "\ufffc"
		}
		if node.Type == "hard_break" {
			return "\n"
		}
		var out strings.Builder
		for _, child := range node.Content {
			out.WriteString(text(child))
		}
		return out.String()
	}
	units := utf16.Encode([]rune(text(*found)))
	start, end := *anchor.StartOffset, *anchor.EndOffset
	if start < 0 || end < start || end > int64(len(units)) {
		return invalid
	}
	boundary := func(index int64) bool {
		return index == 0 || index == int64(len(units)) || !(units[index-1] >= 0xd800 && units[index-1] <= 0xdbff && units[index] >= 0xdc00 && units[index] <= 0xdfff)
	}
	if !boundary(start) || !boundary(end) {
		return invalid
	}
	if anchor.Quote != nil && string(utf16.Decode(units[start:end])) != *anchor.Quote {
		return invalid
	}
	return nil
}

````

## 第 6 步：structNode

插入位置：第 5 步代码之后，紧接追加；完成稿约第 118 行。

本步声明：`type structNode struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Type</code> | <code>string</code> | content-v1正文节点种类<br>序列化/校验标记：<code>`json:&quot;type&quot;`</code> |
| <code>Text</code> | <code>string</code> | 文本节点内容<br>序列化/校验标记：<code>`json:&quot;text&quot;`</code> |
| <code>Attrs</code> | <code>map&#91;string&#93;any</code> | 块ID及锚点校验所需属性<br>序列化/校验标记：<code>`json:&quot;attrs&quot;`</code> |
| <code>Content</code> | <code>&#91;&#93;structNode</code> | 子节点，用于遍历原固定正文<br>序列化/校验标记：<code>`json:&quot;content&quot;`</code> |

<!-- write-step: 6 bytes=3719:3902 -->
````go
type structNode struct {
	Type    string         `json:"type"`
	Text    string         `json:"text"`
	Attrs   map[string]any `json:"attrs"`
	Content []structNode   `json:"content"`
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/domain/document/discussion_models.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/09-comments-notifications.md)。
