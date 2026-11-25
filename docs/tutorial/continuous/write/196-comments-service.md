# 分步编写 `services/document/internal/logic/document/comments_service.go`

先按 [D09 · 固定批注、提及与可靠通知](../lessons/document/09-comments-notifications.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/196-comments-service.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

执行固定上下文批注的创建、回复、编辑、删除、状态与分页读取。

持久讨论不能随短期read view过期消失，但每次读取仍要依据原版本和当前权限。 同一组上下文/版本筛选既限制SQL又绑定签名cursor，因此翻页不会把另一正文范围的扫描位置带进来。

写之前先按这条执行路线理解各段的连接：

1. Create验证anchor并写线程首评
2. Change按来源→文档→线程→评论锁序执行
3. validateContext区分首次创建与后续历史读取
4. ReadThread只定位并受统一输出约束。
5. Comments.List先取可选ThreadFilter首项；Kind只接受MAIN/BRANCH/REVIEW，Context非空必须有Kind，Context与Version均核规范ID。
6. 把整个filter JSON放入commentPageBinding的target，继续绑定actor、SessionRef、当前structure和完整来源摘要；旧过滤条件游标不可跨条件续读。
7. ReadBody内ValidateAccess，按过滤条件有界扫描，逐线程validateContext后组可见页；最多扫描1000条，预算耗尽可返回空页和next。

本文件需要守住的边界：

- 普通评论不修改审核阻塞意见
- 只有当前活动BRANCH可新建线程，旧讨论读取另走历史规则。
- 服务端筛选先于可见limit，但不能跳过原版本、历史上下文和当前权限；非法筛选不能退回全量讨论。
- 空页加next只说明本次授权扫描还可继续；不泄露隐藏线程总数，不把无可见项冒充扫描已结束。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/logic/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/logic/document/comments_service.go') -and (Get-Item -LiteralPath 'services/document/internal/logic/document/comments_service.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/logic/document/comments_service.go')) { New-Item -ItemType File -Path 'services/document/internal/logic/document/comments_service.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:664 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	codec "collaboration/contracts/content"
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	comments "collaboration/document/internal/repository/comment"
	documents "collaboration/document/internal/repository/document"
	resources "collaboration/document/internal/repository/resource"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
)

// comment_mutations.go：保留原实现，按业务合并文件。

````

## 第 2 步：commentMentions

插入位置：第 1 步代码之后，紧接追加；完成稿约第 23 行。

本步声明：`func commentMentions(values []string) ([]uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>values</code> | <code>&#91;&#93;string</code> | 在此声明中的实际使用：<code>len(values)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;uint64</code> | 实际返回表达式：<code>nil</code>；<code>ids</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.InvalidRequest, &quot;COMMENT_MENTIONS_INVALID&quot;)</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(values) &gt; 100</code>。条件成立时执行 <code>return nil, domain.Violation(domain.InvalidRequest, &quot;COMMENT_MENTIONS_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>!seen&#91;id&#93;</code>。
- 在本段的匿名函数内：此处通过 <code>return ids&#91;i&#93; &lt; ids&#91;j&#93;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return ids, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=664:1162 -->
````go
func commentMentions(values []string) ([]uint64, error) {
	if len(values) > 100 {
		return nil, domain.Violation(domain.InvalidRequest, "COMMENT_MENTIONS_INVALID")
	}
	seen := map[uint64]bool{}
	ids := make([]uint64, 0, len(values))
	for _, value := range values {
		id, err := domain.CanonicalID(value)
		if err != nil {
			return nil, err
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids, nil
}

````

## 第 3 步：Comments.Create

插入位置：第 2 步代码之后，紧接追加；完成稿约第 43 行。

本步声明：`func (s *Comments) Create(ctx context.Context, actor Actor, document uint64, input dto.CommentThreadCreate) (Actor, MutationBody, MutationPresentation, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.authorize(ctx, actor, document, true)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>return actor, nil, nil, err</code>；<code>actor, a, err := s.authorize(ctx, actor, document, true)</code>；<code>s.authorize(ctx, actor, document, true)</code>；<code>s.validateContext(ctx, tx, actor, a, t, true)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>commentContextInput(document, input)</code>；<code>s.authorize(ctx, actor, document, true)</code>；<code>comments.InsertThread(ctx, tx, document, actor.ID, t.Kind, t.Context, t.Anchor)</code>；<code>current.Document != document</code> |
| <code>input</code> | <code>dto.CommentThreadCreate</code> | 在此声明中的实际使用：<code>commentContextInput(document, input)</code>；<code>input.Body</code>；<code>input.Mentions</code>；<code>input.OperationId</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>实际返回表达式：<code>actor</code> |
| <code>(匿名/未命名)</code> | <code>MutationBody</code> | 使用 <code>MutationBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>MutationPresentation</code> | 使用 <code>MutationPresentation</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>present</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return OperationResult(input.OperationId, &quot;SUCCEEDED&quot;, id), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>current.Document != document &#124;&#124; current.Author != actor.ID</code>。条件成立时执行 <code>return nil, nil, domain.Violation(domain.StateConflict, &quot;COMMENT_RECEIPT_MISMATCH&quot;)</code>。当前函数在这里结束。

<!-- write-step: 3 bytes=1162:2741 -->
````go
func (s *Comments) Create(ctx context.Context, actor Actor, document uint64, input dto.CommentThreadCreate) (Actor, MutationBody, MutationPresentation, error) {
	t, err := commentContextInput(document, input)
	if err != nil {
		return actor, nil, nil, err
	}
	if err = domain.ValidateCommentBody(input.Body); err != nil {
		return actor, nil, nil, err
	}
	mentions, err := commentMentions(input.Mentions)
	if err != nil {
		return actor, nil, nil, err
	}
	actor, a, err := s.authorize(ctx, actor, document, true)
	if err != nil {
		return actor, nil, nil, err
	}
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		if err := s.validateContext(ctx, tx, actor, a, t, true); err != nil {
			return nil, err
		}
		id, err := comments.InsertThread(ctx, tx, document, actor.ID, t.Kind, t.Context, t.Anchor)
		if err != nil {
			return nil, err
		}
		comment, err := comments.InsertComment(ctx, tx, id, actor.ID, input.Body, nil)
		if err != nil {
			return nil, err
		}
		target := t
		target.ID = id
		if err := comments.QueueMentions(ctx, tx, input.OperationId, target, comment, actor.ID, mentions); err != nil {
			return nil, err
		}
		return OperationResult(input.OperationId, "SUCCEEDED", id), nil
	}
	present := func(ctx context.Context, tx *sql.Tx, value map[string]any) (any, []*pb.SourceEpoch, error) {
		id, err := commentReceipt(value)
		if err != nil {
			return nil, nil, err
		}
		current, err := comments.ThreadRow(ctx, tx, id, false)
		if err != nil {
			return nil, nil, err
		}
		if current.Document != document || current.Author != actor.ID {
````

## 第 4 步：Comments.Create（接着实现）

插入位置：第 3 步代码之后，紧接追加；完成稿约第 88 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：此处通过 <code>return nil, nil, domain.Violation(domain.StateConflict, &quot;COMMENT_RECEIPT_MISMATCH&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return value, a.Sources, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return actor, run, present, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=2741:3137 -->
````go
			return nil, nil, domain.Violation(domain.StateConflict, "COMMENT_RECEIPT_MISMATCH")
		}
		// 已删除的线程仍保留原创建成功事实；当前资格须复核，但不恢复线程。
		current.Status = "OPEN"
		if err := s.validateContext(ctx, tx, actor, a, current, false); err != nil {
			return nil, nil, err
		}
		return value, a.Sources, nil
	}
	return actor, run, present, nil
}

````

## 第 5 步：CommentCommand

插入位置：第 4 步代码之后，紧接追加；完成稿约第 100 行。

本步声明：`type CommentCommand struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Operation</code> | <code>string</code> | 此次评论修改的原操作号，用于幂等。 |
| <code>Kind</code> | <code>string</code> | 新建、回复、修改、删除或状态修改等动作种类，决定执行分支。 |
| <code>Body</code> | <code>string</code> | 待保存的评论正文。 |
| <code>State</code> | <code>string</code> | 请求设置的讨论状态，仅适用于对应动作。 |
| <code>Comment</code> | <code>uint64</code> | 要修改/回复所关联的评论编号。 |
| <code>Revision</code> | <code>uint64</code> | 客户端看到的评论/讨论修订，写入前核对防止覆盖。 |
| <code>Reply</code> | <code>*uint64</code> | 回复指向的评论编号，可为空；有值时必须属于同一讨论。 |
| <code>Mentions</code> | <code>&#91;&#93;string</code> | 被提及账号编号列表，不能把昵称直接当账号授权。 |

<!-- write-step: 5 bytes=3137:3320 -->
````go
type CommentCommand struct {
	Operation, Kind, Body, State string
	Comment, Revision            uint64
	Reply                        *uint64
	Mentions                     []string
}

````

## 第 6 步：Comments.Change

插入位置：第 5 步代码之后，紧接追加；完成稿约第 107 行。

所有写路径都先锁来源和文档，再锁线程与评论；普通讨论不触碰审核阻止意见表。

本步声明：`func (s *Comments) Change(ctx context.Context, actor Actor, original comments.Thread, command CommentCommand) (Actor, MutationBody, MutationPresentation, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.authorize(ctx, actor, original.Document, command.Kind == &quot;REPLY&quot;)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>return actor, nil, nil, domain.Violation(domain.InvalidRequest, &quot;COMMENT_ACTION_INVALID&quot;)</code>；<code>return actor, nil, nil, err</code>；<code>return actor, nil, nil, domain.Violation(domain.InvalidRequest, &quot;COMMENT_STATE_INVALID&quot;)</code>；<code>actor, a, err := s.authorize(ctx, actor, original.Document, command.Kind == &quot;REPLY&quot;)</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |
| <code>original</code> | <code>comments.Thread</code> | 使用 <code>comments.Thread</code> 类型；其定义处的职责：保存线程、评论、回复墓碑、版本上下文和通知意图。<br>在此声明中的实际使用：<code>original.Document</code>；<code>original.ID</code> |
| <code>command</code> | <code>CommentCommand</code> | 在此声明中的实际使用：<code>command.Kind</code>；<code>command.Body</code>；<code>command.State</code>；<code>command.Mentions</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>实际返回表达式：<code>actor</code> |
| <code>(匿名/未命名)</code> | <code>MutationBody</code> | 使用 <code>MutationBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>MutationPresentation</code> | 使用 <code>MutationPresentation</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>present</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.InvalidRequest, &quot;COMMENT_ACTION_INVALID&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.InvalidRequest, &quot;COMMENT_STATE_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>command.Kind != &quot;REPLY&quot; &amp;&amp; command.Kind != &quot;EDIT&quot; &amp;&amp; command.Kind != &quot;DELETE&quot; &amp;&amp; command.Kind != &quot;STATE&quot;</code>。条件成立时执行 <code>return actor, nil, nil, domain.Violation(domain.InvalidRequest, &quot;COMMENT_ACTION_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>command.Kind == &quot;REPLY&quot; &#124;&#124; command.Kind == &quot;EDIT&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>command.Kind == &quot;STATE&quot; &amp;&amp; command.State != &quot;OPEN&quot; &amp;&amp; command.State != &quot;RESOLVED&quot; &amp;&amp; command.State != &quot;DELETED&quot;</code>。条件成立时执行 <code>return actor, nil, nil, domain.Violation(domain.InvalidRequest, &quot;COMMENT_STATE_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>t.Document != original.Document</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;COMMENT_CONTEXT_CHANGED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err == nil</code>。
- 在本段的匿名函数内：检查 <code>actor.ID != t.Author &amp;&amp; !a.Permissions.Manage</code>。条件成立时执行 <code>return nil, domain.Violation(domain.PermissionDenied, &quot;COMMENT_THREAD_AUTHOR_REQUIRED&quot;)</code>。当前函数在这里结束。

<!-- write-step: 6 bytes=3320:5339 -->
````go
// 所有写路径都先锁来源和文档，再锁线程与评论；普通讨论不触碰审核阻止意见表。
func (s *Comments) Change(ctx context.Context, actor Actor, original comments.Thread, command CommentCommand) (Actor, MutationBody, MutationPresentation, error) {
	if command.Kind != "REPLY" && command.Kind != "EDIT" && command.Kind != "DELETE" && command.Kind != "STATE" {
		return actor, nil, nil, domain.Violation(domain.InvalidRequest, "COMMENT_ACTION_INVALID")
	}
	if command.Kind == "REPLY" || command.Kind == "EDIT" {
		if err := domain.ValidateCommentBody(command.Body); err != nil {
			return actor, nil, nil, err
		}
	}
	if command.Kind == "STATE" && command.State != "OPEN" && command.State != "RESOLVED" && command.State != "DELETED" {
		return actor, nil, nil, domain.Violation(domain.InvalidRequest, "COMMENT_STATE_INVALID")
	}
	mentions, err := commentMentions(command.Mentions)
	if err != nil {
		return actor, nil, nil, err
	}
	// 新回复需要全文 COMMENT；已有内容的维护沿冻结契约核验作者/管理者及当前上下文可读资格。
	actor, a, err := s.authorize(ctx, actor, original.Document, command.Kind == "REPLY")
	if err != nil {
		return actor, nil, nil, err
	}
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		if _, err := ValidateAccess(ctx, tx, a); err != nil {
			return nil, err
		}
		t, err := comments.ThreadRow(ctx, tx, original.ID, true)
		if err != nil {
			return nil, err
		}
		if t.Document != original.Document {
			return nil, domain.Violation(domain.StateConflict, "COMMENT_CONTEXT_CHANGED")
		}
		if err := s.validateContext(ctx, tx, actor, a, t, false); err != nil {
			return nil, err
		}
		id := t.ID
		switch command.Kind {
		case "REPLY":
			id, err = comments.InsertComment(ctx, tx, t.ID, actor.ID, command.Body, command.Reply)
			if err == nil {
				err = comments.QueueMentions(ctx, tx, command.Operation, t, id, actor.ID, mentions)
			}
		case "STATE":
			if actor.ID != t.Author && !a.Permissions.Manage {
````

## 第 7 步：Comments.Change（接着实现）

插入位置：第 6 步代码之后，紧接追加；完成稿约第 152 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：此处通过 <code>return nil, domain.Violation(domain.PermissionDenied, &quot;COMMENT_THREAD_AUTHOR_REQUIRED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>t.Revision != command.Revision</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;COMMENT_REVISION_CHANGED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>readErr != nil</code>。条件成立时执行 <code>return nil, readErr</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>e.Thread != t.ID</code>。条件成立时执行 <code>return nil, domain.Violation(domain.PermissionDenied, &quot;COMMENT_THREAD_MISMATCH&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>e.Author != actor.ID &amp;&amp; !(command.Kind == &quot;DELETE&quot; &amp;&amp; a.Permissions.Manage)</code>。条件成立时执行 <code>return nil, domain.Violation(domain.PermissionDenied, &quot;COMMENT_AUTHOR_REQUIRED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>e.Revision != command.Revision</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;COMMENT_REVISION_CHANGED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>command.Kind == &quot;EDIT&quot;</code>。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return OperationResult(command.Operation, &quot;SUCCEEDED&quot;, id), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return value, a.Sources, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return actor, run, present, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=5339:6923 -->
````go
				return nil, domain.Violation(domain.PermissionDenied, "COMMENT_THREAD_AUTHOR_REQUIRED")
			}
			if t.Revision != command.Revision {
				return nil, domain.Violation(domain.StateConflict, "COMMENT_REVISION_CHANGED")
			}
			err = comments.UpdateThreadState(ctx, tx, t, command.State)
		default:
			e, readErr := comments.CommentRow(ctx, tx, command.Comment, true)
			if readErr != nil {
				return nil, readErr
			}
			if e.Thread != t.ID {
				return nil, domain.Violation(domain.PermissionDenied, "COMMENT_THREAD_MISMATCH")
			}
			if e.Author != actor.ID && !(command.Kind == "DELETE" && a.Permissions.Manage) {
				return nil, domain.Violation(domain.PermissionDenied, "COMMENT_AUTHOR_REQUIRED")
			}
			if e.Revision != command.Revision {
				return nil, domain.Violation(domain.StateConflict, "COMMENT_REVISION_CHANGED")
			}
			id = e.ID
			if command.Kind == "EDIT" {
				err = comments.EditComment(ctx, tx, e, command.Body)
			} else {
				err = comments.DeleteComment(ctx, tx, e)
			}
		}
		if err != nil {
			return nil, err
		}
		return OperationResult(command.Operation, "SUCCEEDED", id), nil
	}
	present := func(ctx context.Context, tx *sql.Tx, value map[string]any) (any, []*pb.SourceEpoch, error) {
		t, err := comments.ThreadRow(ctx, tx, original.ID, false)
		if err != nil {
			return nil, nil, err
		}
		t.Status = "OPEN"
		if err := s.validateContext(ctx, tx, actor, a, t, false); err != nil {
			return nil, nil, err
		}
		return value, a.Sources, nil
	}
	return actor, run, present, nil
}

// comment_readings.go：保留原实现，按业务合并文件。

````

## 第 8 步：commentPageBinding

插入位置：第 7 步代码之后，紧接追加；完成稿约第 200 行。

本步声明：`func commentPageBinding(actor Actor, a DocumentAccess, target string) string`。

**这一段为什么这样写：** 用有限长度摘要绑定完整来源集合，连同真实会话、结构及包含filter的目标固定分页范围。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>actor</code> | <code>Actor</code> | 当前用户与SessionRef，不仅是可复用的用户ID。<br>在此声明中的实际使用：<code>actor.ID</code>；<code>actor.Proof</code> |
| <code>a</code> | <code>DocumentAccess</code> | 本次DocumentAccess，带当前结构和实际来源代次集合。<br>在此声明中的实际使用：<code>a.Sources</code>；<code>a.Document</code> |
| <code>target</code> | <code>string</code> | document:ID:filterJSON或thread:ID；筛选变更会得到不同绑定。<br>在此声明中的实际使用：<code>&quot;comments:&quot; + domain.TextID(actor.ID) + &quot;:&quot; + hex.EncodeToString(actor.Proof.SessionRef) + &quot;:&quot; + target</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 交Pagination.Parse/Next使用的固定绑定字符串；不是授权凭据本身。<br>实际返回表达式：<code>&quot;comments:&quot; + domain.TextID(actor.ID) + &quot;:&quot; + hex.EncodeToString(actor.Proof.SessionRef) + &quot;:&quot; + target + &quot;:&quot; + a.Document.CurrentStructureId + &quot;:&quot; + hex.EncodeToString(digest&#91;:&#93;)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &quot;comments:&quot; + domain.TextID(actor.ID) + &quot;:&quot; + hex.EncodeToString(actor.Proof.SessionRef) + &quot;:&quot; + target + &quot;:&quot; + a.Document.CurrentStructureId + &quot;:&quot; + hex.EncodeToString(digest&#91;:&#93;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=6923:7488 -->
````go
func commentPageBinding(actor Actor, a DocumentAccess, target string) string {
	// 游标绑定实际来源代次及当前结构。撤权/重授予不能续用上次扫描位置。
	sources, _ := json.Marshal(SourcesWire(a.Sources))
	// 权限来源数量不能让生成的游标超出解析器上限；摘要仍绑定完整来源集合。
	digest := sha256.Sum256(sources)
	return "comments:" + domain.TextID(actor.ID) + ":" + hex.EncodeToString(actor.Proof.SessionRef) + ":" + target + ":" + a.Document.CurrentStructureId + ":" + hex.EncodeToString(digest[:])
}

````

## 第 9 步：Comments.ReadThread

插入位置：第 8 步代码之后，紧接追加；完成稿约第 208 行。

ReadThread 供通知按真实线程身份定位原文档；不返回评论正文，定位结果仍须
在统一输出租约事务内检查当前身份、原固定上下文与删除状态，不能成为权限旁路。

本步声明：`func (s *Comments) ReadThread(ctx context.Context, actor Actor, original comments.Thread) ([]*pb.SourceEpoch, ReadBody, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.authorize(ctx, actor, original.Document, false)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.authorize(ctx, actor, original.Document, false)</code>；<code>s.validateContext(ctx, tx, actor, a, thread, false)</code> |
| <code>original</code> | <code>comments.Thread</code> | 使用 <code>comments.Thread</code> 类型；其定义处的职责：保存线程、评论、回复墓碑、版本上下文和通知意图。<br>在此声明中的实际使用：<code>original.Document</code>；<code>original.ID</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 实际返回表达式：<code>nil</code>；<code>a.Sources</code> |
| <code>(匿名/未命名)</code> | <code>ReadBody</code> | 使用 <code>ReadBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>thread.Document != original.Document</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;COMMENT_CONTEXT_CHANGED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return thread.DTO(), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return a.Sources, run, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=7488:8494 -->
````go
// ReadThread 供通知按真实线程身份定位原文档；不返回评论正文，定位结果仍须
// 在统一输出租约事务内检查当前身份、原固定上下文与删除状态，不能成为权限旁路。
func (s *Comments) ReadThread(ctx context.Context, actor Actor, original comments.Thread) ([]*pb.SourceEpoch, ReadBody, error) {
	_, a, err := s.authorize(ctx, actor, original.Document, false)
	if err != nil {
		return nil, nil, err
	}
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		if _, err := ValidateAccess(ctx, tx, a); err != nil {
			return nil, err
		}
		thread, err := comments.ThreadRow(ctx, tx, original.ID, false)
		if err != nil {
			return nil, err
		}
		if thread.Document != original.Document {
			return nil, domain.Violation(domain.StateConflict, "COMMENT_CONTEXT_CHANGED")
		}
		if err := s.validateContext(ctx, tx, actor, a, thread, false); err != nil {
			return nil, err
		}
		return thread.DTO(), nil
	}
	return a.Sources, run, nil
}

````

## 第 10 步：Comments.List

插入位置：第 9 步代码之后，紧接追加；完成稿约第 234 行。

本步声明：`func (s *Comments) List(ctx context.Context, actor Actor, document uint64, limitText, cursor string, filters ...comments.ThreadFilter) ([]*pb.SourceEpoch, ReadBody, error)`。

**这一段为什么这样写：** 将正文上下文筛选、签名游标、逐项历史资格和输出事务集中在同一读取流程。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用上下文。<br>在此声明中的实际使用：<code>s.authorize(ctx, actor, document, false)</code> |
| <code>actor</code> | <code>Actor</code> | 当前会话与Proof，包含SessionRef；不能从URL自报身份。<br>在此声明中的实际使用：<code>s.authorize(ctx, actor, document, false)</code>；<code>commentPageBinding(actor, a, &quot;document:&quot;+domain.TextID(document)+&quot;:&quot;+string(encodedFilter))</code>；<code>s.validateContext(ctx, tx, actor, a, t, false)</code> |
| <code>document</code> | <code>uint64</code> | 真实文档ID。<br>在此声明中的实际使用：<code>s.authorize(ctx, actor, document, false)</code>；<code>domain.TextID(document)</code>；<code>comments.ListThreads(ctx, tx, document, scanned, 100, filter)</code> |
| <code>limitText</code> | <code>string</code> | 线程展示页大小文本，当前界面传10。<br>在此声明中的实际使用：<code>s.pagination.Parse(limitText, cursor, binding)</code> |
| <code>cursor</code> | <code>string</code> | 原opaque签名cursor，首页为空；必须与当前filter/会话/结构/来源匹配。<br>在此声明中的实际使用：<code>s.pagination.Parse(limitText, cursor, binding)</code> |
| <code>filters</code> | <code>...comments.ThreadFilter</code> | 可选comments.ThreadFilter切片，只取首项；零参数兼容原调用，Context要求Kind，Context/Version非空须规范ID。<br>在此声明中的实际使用：<code>len(filters)</code>；<code>filter = filters&#91;0&#93;</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 当前实际文档权限来源，供输出闸门使用。<br>实际返回表达式：<code>nil</code>；<code>a.Sources</code> |
| <code>(匿名/未命名)</code> | <code>ReadBody</code> | 原事务内再次ValidateAccess、按固定filter扫描并核上下文后输出CommentThreadPage的闭包。<br>实际返回表达式：<code>nil</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 非法过滤、失效cursor、权限、上下文或依赖错误。<br>实际返回表达式：<code>domain.Violation(domain.InvalidRequest, &quot;COMMENT_FILTER_INVALID&quot;)</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(filters) &gt; 0</code>。
- 检查 <code>filter.Kind != &quot;&quot; &amp;&amp; filter.Kind != &quot;MAIN&quot; &amp;&amp; filter.Kind != &quot;BRANCH&quot; &amp;&amp; filter.Kind != &quot;REVIEW&quot;</code>。条件成立时执行 <code>return nil, nil, domain.Violation(domain.InvalidRequest, &quot;COMMENT_FILTER_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>filter.Context != &quot;&quot; &amp;&amp; filter.Kind == &quot;&quot;</code>。条件成立时执行 <code>return nil, nil, domain.Violation(domain.InvalidRequest, &quot;COMMENT_FILTER_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>id != &quot;&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>last != &quot;&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。

<!-- write-step: 10 bytes=8494:10254 -->
````go
func (s *Comments) List(ctx context.Context, actor Actor, document uint64, limitText, cursor string, filters ...comments.ThreadFilter) ([]*pb.SourceEpoch, ReadBody, error) {
	filter := comments.ThreadFilter{}
	if len(filters) > 0 {
		filter = filters[0]
	}
	if filter.Kind != "" && filter.Kind != "MAIN" && filter.Kind != "BRANCH" && filter.Kind != "REVIEW" {
		return nil, nil, domain.Violation(domain.InvalidRequest, "COMMENT_FILTER_INVALID")
	}
	if filter.Context != "" && filter.Kind == "" {
		return nil, nil, domain.Violation(domain.InvalidRequest, "COMMENT_FILTER_INVALID")
	}
	for _, id := range []string{filter.Context, filter.Version} {
		if id != "" {
			if _, err := domain.CanonicalID(id); err != nil {
				return nil, nil, err
			}
		}
	}
	_, a, err := s.authorize(ctx, actor, document, false)
	if err != nil {
		return nil, nil, err
	}
	encodedFilter, _ := json.Marshal(filter)
	binding := commentPageBinding(actor, a, "document:"+domain.TextID(document)+":"+string(encodedFilter))
	limit, last, err := s.pagination.Parse(limitText, cursor, binding)
	if err != nil {
		return nil, nil, err
	}
	var after uint64
	if last != "" {
		after, err = domain.CanonicalID(last)
		if err != nil {
			return nil, nil, err
		}
	}
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		if _, err := ValidateAccess(ctx, tx, a); err != nil {
			return nil, err
		}
		page := dto.CommentThreadPage{Items: []dto.CommentThread{}}
		scanned := after
		// 每页最多扫描1000条，所有返回项逐项检查资格。扫描预算用尽可以返回空页和续页游标，不能把隐藏项计数泄露给用户。
		for count := 0; count < 1000; {
			rows, err := comments.ListThreads(ctx, tx, document, scanned, 100, filter)
			if err != nil {
````

## 第 11 步：Comments.List（接着实现）

插入位置：第 10 步代码之后，紧接追加；完成稿约第 279 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：此处通过 <code>return nil, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>err != nil</code>。
- 在本段的匿名函数内：检查 <code>errors.As(err, &amp;denied) &amp;&amp; (denied.Kind == domain.PermissionDenied &#124;&#124; denied.Kind == domain.EntityNotFound)</code>。
- 在本段的匿名函数内：此处通过 <code>return nil, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>len(page.Items) == limit</code>。
- 在本段的匿名函数内：此处通过 <code>return page, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>len(rows) &lt; 100</code>。条件成立时执行 <code>return page, nil</code>。当前函数在这里结束。
- 此处通过 <code>return a.Sources, run, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=10254:11052 -->
````go
				return nil, err
			}
			for _, t := range rows {
				count++
				err := s.validateContext(ctx, tx, actor, a, t, false)
				if err != nil {
					var denied domain.RuleViolation
					if errors.As(err, &denied) && (denied.Kind == domain.PermissionDenied || denied.Kind == domain.EntityNotFound) {
						scanned = t.ID
						continue
					}
					return nil, err
				}
				if len(page.Items) == limit {
					page.HasMore = true
					page.NextCursor = s.pagination.Next(binding, domain.TextID(scanned))
					return page, nil
				}
				page.Items = append(page.Items, t.DTO())
				scanned = t.ID
			}
			if len(rows) < 100 {
				return page, nil
			}
		}
		page.HasMore = true
		page.NextCursor = s.pagination.Next(binding, domain.TextID(scanned))
		return page, nil
	}
	return a.Sources, run, nil
}

````

## 第 12 步：Comments.ReadComments

插入位置：第 11 步代码之后，紧接追加；完成稿约第 311 行。

本步声明：`func (s *Comments) ReadComments(ctx context.Context, actor Actor, original comments.Thread, limitText, cursor string) ([]*pb.SourceEpoch, ReadBody, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.authorize(ctx, actor, original.Document, false)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.authorize(ctx, actor, original.Document, false)</code>；<code>commentPageBinding(actor, a, &quot;thread:&quot;+domain.TextID(original.ID))</code>；<code>s.validateContext(ctx, tx, actor, a, t, false)</code> |
| <code>original</code> | <code>comments.Thread</code> | 使用 <code>comments.Thread</code> 类型；其定义处的职责：保存线程、评论、回复墓碑、版本上下文和通知意图。<br>在此声明中的实际使用：<code>original.Document</code>；<code>original.ID</code> |
| <code>limitText</code> | <code>string</code> | 在此声明中的实际使用：<code>s.pagination.Parse(limitText, cursor, binding)</code> |
| <code>cursor</code> | <code>string</code> | 在此声明中的实际使用：<code>s.pagination.Parse(limitText, cursor, binding)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 实际返回表达式：<code>nil</code>；<code>a.Sources</code> |
| <code>(匿名/未命名)</code> | <code>ReadBody</code> | 使用 <code>ReadBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>last != &quot;&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>t.Document != original.Document</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;COMMENT_CONTEXT_CHANGED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>page.HasMore</code>。
- 在本段的匿名函数内：此处通过 <code>return page, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=11052:12623 -->
````go
func (s *Comments) ReadComments(ctx context.Context, actor Actor, original comments.Thread, limitText, cursor string) ([]*pb.SourceEpoch, ReadBody, error) {
	_, a, err := s.authorize(ctx, actor, original.Document, false)
	if err != nil {
		return nil, nil, err
	}
	binding := commentPageBinding(actor, a, "thread:"+domain.TextID(original.ID))
	limit, last, err := s.pagination.Parse(limitText, cursor, binding)
	if err != nil {
		return nil, nil, err
	}
	var after uint64
	if last != "" {
		after, err = domain.CanonicalID(last)
		if err != nil {
			return nil, nil, err
		}
	}
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		// 与删除路径保持文档在前的锁顺序，避免先建立旧快照后等待删除提交。
		if _, err := ValidateAccess(ctx, tx, a); err != nil {
			return nil, err
		}
		t, err := comments.ThreadRow(ctx, tx, original.ID, false)
		if err != nil {
			return nil, err
		}
		if t.Document != original.Document {
			return nil, domain.Violation(domain.StateConflict, "COMMENT_CONTEXT_CHANGED")
		}
		if err := s.validateContext(ctx, tx, actor, a, t, false); err != nil {
			return nil, err
		}
		rows, err := comments.ListComments(ctx, tx, t.ID, after, limit+1)
		if err != nil {
			return nil, err
		}
		page := dto.CommentPage{Items: []dto.Comment{}, HasMore: len(rows) > limit}
		if page.HasMore {
			rows = rows[:limit]
			page.NextCursor = s.pagination.Next(binding, domain.TextID(rows[len(rows)-1].ID))
		}
		for _, row := range rows {
			page.Items = append(page.Items, row.DTO())
		}
		return page, nil
	}
````

## 第 13 步：Comments.ReadComments（接着实现）

插入位置：第 12 步代码之后，紧接追加；完成稿约第 357 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return a.Sources, run, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=12623:12715 -->
````go
	return a.Sources, run, nil
}

// comments.go：保留原实现，按业务合并文件。

````

## 第 14 步：Comments

插入位置：第 13 步代码之后，紧接追加；完成稿约第 362 行。

本步声明：`type Comments struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>access</code> | <code>*Access</code> | 使用 <code>Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。 |
| <code>reviews</code> | <code>*Reviews</code> | 使用 <code>Reviews</code> 类型；其定义处的职责：执行请求修改、撤回、解决意见和审核列表/历史的当前资格检查。 |
| <code>pagination</code> | <code>*Pagination</code> | 使用 <code>Pagination</code> 类型；其定义处的职责：生成并校验绑定查询上下文的签名翻页游标。 |
| <code>validator</code> | <code>*codec.Validator</code> | 冻结协议的结构校验器，先限制请求形状，再由业务核验身份、权限、版本和事务条件。 |

<!-- write-step: 14 bytes=12715:12855 -->
````go
type Comments struct {
	db         *sql.DB
	access     *Access
	reviews    *Reviews
	pagination *Pagination
	validator  *codec.Validator
}

````

## 第 15 步：NewComments

插入位置：第 14 步代码之后，紧接追加；完成稿约第 370 行。

本步声明：`func NewComments(db *sql.DB, access *Access, reviews *Reviews, pagination *Pagination) *Comments`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |
| <code>access</code> | <code>*Access</code> | 使用 <code>Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。<br>在此声明中的实际使用：<code>access: access</code> |
| <code>reviews</code> | <code>*Reviews</code> | 使用 <code>Reviews</code> 类型；其定义处的职责：执行请求修改、撤回、解决意见和审核列表/历史的当前资格检查。<br>在此声明中的实际使用：<code>reviews: reviews</code>；<code>reviews.validator</code> |
| <code>pagination</code> | <code>*Pagination</code> | 使用 <code>Pagination</code> 类型；其定义处的职责：生成并校验绑定查询上下文的签名翻页游标。<br>在此声明中的实际使用：<code>pagination: pagination</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Comments</code> | 实际返回表达式：<code>&amp;Comments{db: db, access: access, reviews: reviews, pagination: pagination, validator: reviews.validator}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Comments{db: db, access: access, reviews: reviews, pagination: pagination, validator: reviews.validator}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=12855:13071 -->
````go
func NewComments(db *sql.DB, access *Access, reviews *Reviews, pagination *Pagination) *Comments {
	return &Comments{db: db, access: access, reviews: reviews, pagination: pagination, validator: reviews.validator}
}

````

## 第 16 步：Comments.LookupThread

插入位置：第 15 步代码之后，紧接追加；完成稿约第 374 行。

本步声明：`func (s *Comments) LookupThread(ctx context.Context, id uint64) (comments.Thread, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>comments.ThreadRow(ctx, s.db, id, false)</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>comments.ThreadRow(ctx, s.db, id, false)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>comments.Thread</code> | 使用 <code>comments.Thread</code> 类型；其定义处的职责：保存线程、评论、回复墓碑、版本上下文和通知意图。<br>实际返回表达式：<code>t</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>err == sql.ErrNoRows</code>。
- 此处通过 <code>return t, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=13071:13338 -->
````go
func (s *Comments) LookupThread(ctx context.Context, id uint64) (comments.Thread, error) {
	t, err := comments.ThreadRow(ctx, s.db, id, false)
	if err == sql.ErrNoRows {
		err = domain.Violation(domain.EntityNotFound, "COMMENT_THREAD_NOT_FOUND")
	}
	return t, err
}

````

## 第 17 步：Comments.LookupComment

插入位置：第 16 步代码之后，紧接追加；完成稿约第 382 行。

本步声明：`func (s *Comments) LookupComment(ctx context.Context, id uint64) (comments.Entry, comments.Thread, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>comments.CommentRow(ctx, s.db, id, false)</code>；<code>s.LookupThread(ctx, e.Thread)</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>comments.CommentRow(ctx, s.db, id, false)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>comments.Entry</code> | 使用 <code>comments.Entry</code> 类型；其定义处的职责：保存线程、评论、回复墓碑、版本上下文和通知意图。<br>实际返回表达式：<code>e</code> |
| <code>(匿名/未命名)</code> | <code>comments.Thread</code> | 使用 <code>comments.Thread</code> 类型；其定义处的职责：保存线程、评论、回复墓碑、版本上下文和通知意图。<br>实际返回表达式：<code>comments.Thread{}</code>；<code>t</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>err == sql.ErrNoRows</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return e, comments.Thread{}, err</code>。当前函数在这里结束。
- 此处通过 <code>return e, t, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 17 bytes=13338:13715 -->
````go
func (s *Comments) LookupComment(ctx context.Context, id uint64) (comments.Entry, comments.Thread, error) {
	e, err := comments.CommentRow(ctx, s.db, id, false)
	if err == sql.ErrNoRows {
		err = domain.Violation(domain.EntityNotFound, "COMMENT_NOT_FOUND")
	}
	if err != nil {
		return e, comments.Thread{}, err
	}
	t, err := s.LookupThread(ctx, e.Thread)
	return e, t, err
}

````

## 第 18 步：Comments.authorize

插入位置：第 17 步代码之后，紧接追加；完成稿约第 394 行。

本步声明：`func (s *Comments) authorize(ctx context.Context, actor Actor, document uint64, write bool) (Actor, DocumentAccess, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, document, &quot;COMMENT_CONTEXT&quot;)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, document, &quot;COMMENT_CONTEXT&quot;)</code>；<code>actor.Sources</code>；<code>return actor, a, err</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, document, &quot;COMMENT_CONTEXT&quot;)</code> |
| <code>write</code> | <code>bool</code> | 在此声明中的实际使用：<code>err == nil &amp;&amp; write</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>实际返回表达式：<code>actor</code> |
| <code>(匿名/未命名)</code> | <code>DocumentAccess</code> | 使用 <code>DocumentAccess</code> 类型；其定义处的职责：把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。<br>实际返回表达式：<code>a</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>err == nil &amp;&amp; write &amp;&amp; !a.Permissions.Comment</code>。
- 此处通过 <code>return actor, a, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=13715:14088 -->
````go
func (s *Comments) authorize(ctx context.Context, actor Actor, document uint64, write bool) (Actor, DocumentAccess, error) {
	a, err := s.access.Authorize(ctx, actor, document, "COMMENT_CONTEXT")
	if err == nil && write && !a.Permissions.Comment {
		err = domain.Violation(domain.PermissionDenied, "COMMENT_REQUIRED")
	}
	actor.Sources = a.Sources
	return actor, a, err
}

````

## 第 19 步：Comments.validateContext

插入位置：第 18 步代码之后，紧接追加；完成稿约第 403 行。

每次读取/修改都重新核验固定上下文。创建时才检查短期阅读凭据；持久讨论
后续依据原版本与当前权限读取，不能随着阅读视图到期丢掉已保存的讨论。

本步声明：`func (s *Comments) validateContext(ctx context.Context, tx *sql.Tx, actor Actor, a DocumentAccess, t comments.Thread, creating bool) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ValidateAccess(ctx, tx, a)</code>；<code>comments.ContextVersion(ctx, tx, t.Document, t.Kind, id, t.Version)</code>；<code>documents.ContentVersionScope(ctx, tx, t.Version)</code>；<code>historicalContentRangeCovered(ctx, tx, a, t.Document, scope.Unit, s.validator)</code><br>另有 7 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>ValidateAccess(ctx, tx, a)</code>；<code>comments.ContextVersion(ctx, tx, t.Document, t.Kind, id, t.Version)</code>；<code>documents.ContentVersionScope(ctx, tx, t.Version)</code>；<code>historicalContentRangeCovered(ctx, tx, a, t.Document, scope.Unit, s.validator)</code><br>另有 7 处使用，继续按下方完整代码的顺序处理。 |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor.ID</code>；<code>actor.Proof</code> |
| <code>a</code> | <code>DocumentAccess</code> | 使用 <code>DocumentAccess</code> 类型；其定义处的职责：把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。<br>在此声明中的实际使用：<code>ValidateAccess(ctx, tx, a)</code>；<code>a.Permissions</code>；<code>historicalContentRangeCovered(ctx, tx, a, t.Document, scope.Unit, s.validator)</code>；<code>historicalContentRangeCovered(ctx, tx, a, t.Document, unit, s.validator)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>t</code> | <code>comments.Thread</code> | 使用 <code>comments.Thread</code> 类型；其定义处的职责：保存线程、评论、回复墓碑、版本上下文和通知意图。<br>在此声明中的实际使用：<code>t.Status</code>；<code>t.Context</code>；<code>t.Document</code>；<code>t.Kind</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>creating</code> | <code>bool</code> | 在此声明中的实际使用：<code>creating &amp;&amp; !(doc.MainLocked &amp;&amp; doc.CurrentManifestId == t.Context)</code>；<code>creating &amp;&amp; !active</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;DOCUMENT_NOT_ACTIVE&quot;)</code>；<code>domain.Violation(domain.EntityNotFound, &quot;COMMENT_THREAD_NOT_FOUND&quot;)</code>；<code>domain.Violation(domain.PermissionDenied, &quot;COMMENT_CONTEXT_MISMATCH&quot;)</code><br>另有 6 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>doc.Lifecycle != &quot;ACTIVE&quot;</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;DOCUMENT_NOT_ACTIVE&quot;)</code>。当前函数在这里结束。
- 检查 <code>t.Status == &quot;DELETED&quot;</code>。条件成立时执行 <code>return domain.Violation(domain.EntityNotFound, &quot;COMMENT_THREAD_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!bound</code>。条件成立时执行 <code>return domain.Violation(domain.PermissionDenied, &quot;COMMENT_CONTEXT_MISMATCH&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!a.Permissions.Comment &amp;&amp; !a.Permissions.Manage &amp;&amp; !a.Permissions.Edit&#91;scope.Unit&#93;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!covered</code>。条件成立时执行 <code>return domain.Violation(domain.PermissionDenied, &quot;COMMENT_CONTEXT_NOT_ALLOWED&quot;)</code>。当前函数在这里结束。
- 检查 <code>creating &amp;&amp; !(doc.MainLocked &amp;&amp; doc.CurrentManifestId == t.Context)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!valid</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;READ_VIEW_NOT_CURRENT&quot;)</code>。当前函数在这里结束。

<!-- write-step: 19 bytes=14088:15719 -->
````go
// 每次读取/修改都重新核验固定上下文。创建时才检查短期阅读凭据；持久讨论
// 后续依据原版本与当前权限读取，不能随着阅读视图到期丢掉已保存的讨论。
func (s *Comments) validateContext(ctx context.Context, tx *sql.Tx, actor Actor, a DocumentAccess, t comments.Thread, creating bool) error {
	doc, err := ValidateAccess(ctx, tx, a)
	if err != nil {
		return err
	}
	if doc.Lifecycle != "ACTIVE" {
		return domain.Violation(domain.StateConflict, "DOCUMENT_NOT_ACTIVE")
	}
	if t.Status == "DELETED" {
		return domain.Violation(domain.EntityNotFound, "COMMENT_THREAD_NOT_FOUND")
	}
	id, err := domain.CanonicalID(t.Context)
	if err != nil {
		return err
	}
	bound, err := comments.ContextVersion(ctx, tx, t.Document, t.Kind, id, t.Version)
	if err != nil {
		return err
	}
	if !bound {
		return domain.Violation(domain.PermissionDenied, "COMMENT_CONTEXT_MISMATCH")
	}
	scope, err := documents.ContentVersionScope(ctx, tx, t.Version)
	if err != nil {
		return err
	}
	switch t.Kind {
	case "MAIN":
		if !a.Permissions.Comment && !a.Permissions.Manage && !a.Permissions.Edit[scope.Unit] {
			covered, err := historicalContentRangeCovered(ctx, tx, a, t.Document, scope.Unit, s.validator)
			if err != nil {
				return err
			}
			if !covered {
				return domain.Violation(domain.PermissionDenied, "COMMENT_CONTEXT_NOT_ALLOWED")
			}
		}
		if creating && !(doc.MainLocked && doc.CurrentManifestId == t.Context) {
			valid, err := resources.MainManifestAccessible(ctx, tx, t.Document, id, actor.ID, actor.Proof.SessionRef)
			if err != nil {
				return err
			}
			if !valid {
````

## 第 20 步：Comments.validateContext（接着实现）

插入位置：第 19 步代码之后，紧接追加；完成稿约第 448 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return domain.Violation(domain.StateConflict, &quot;READ_VIEW_NOT_CURRENT&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>creating &amp;&amp; !active</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;COMMENT_BRANCH_NOT_CURRENT&quot;)</code>。当前函数在这里结束。
- 检查 <code>!allowed</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!allowed</code>。条件成立时执行 <code>return domain.Violation(domain.PermissionDenied, &quot;COMMENT_CONTEXT_NOT_ALLOWED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return domain.Violation(domain.InvalidRequest, &quot;COMMENT_CONTEXT_INVALID&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>scope.Document != t.Document</code>。条件成立时执行 <code>return domain.Violation(domain.PermissionDenied, &quot;COMMENT_CONTEXT_MISMATCH&quot;)</code>。当前函数在这里结束。
- 检查 <code>creating</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return domain.ValidateCommentAnchor(t.Anchor, fixed.BodyJsonUtf8)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 20 bytes=15719:17099 -->
````go
				return domain.Violation(domain.StateConflict, "READ_VIEW_NOT_CURRENT")
			}
		}
	case "BRANCH":
		unit, active, err := comments.BranchScope(ctx, tx, t.Document, id)
		if err != nil {
			return err
		}
		if creating && !active {
			return domain.Violation(domain.StateConflict, "COMMENT_BRANCH_NOT_CURRENT")
		}
		allowed := a.Permissions.Comment || a.Permissions.Manage || a.Permissions.Edit[unit]
		if !allowed {
			allowed, err = historicalContentRangeCovered(ctx, tx, a, t.Document, unit, s.validator)
			if err != nil {
				return err
			}
		}
		if !allowed {
			return domain.Violation(domain.PermissionDenied, "COMMENT_CONTEXT_NOT_ALLOWED")
		}
	case "REVIEW":
		request, _, _, err := resources.ReviewContext(ctx, tx, t.Document, id)
		if err != nil {
			return err
		}
		pointer, err := documents.ReviewPointerRow(ctx, tx, request)
		if err != nil {
			return err
		}
		if err := s.reviews.validateReviewReadAccess(ctx, tx, a, pointer); err != nil {
			return err
		}
	default:
		return domain.Violation(domain.InvalidRequest, "COMMENT_CONTEXT_INVALID")
	}
	if scope.Document != t.Document {
		return domain.Violation(domain.PermissionDenied, "COMMENT_CONTEXT_MISMATCH")
	}
	if creating {
		fixed, err := documents.VersionContent(ctx, tx, t.Version, t.Document)
		if err != nil {
			return err
		}
		return domain.ValidateCommentAnchor(t.Anchor, fixed.BodyJsonUtf8)
	}
````

## 第 21 步：Comments.validateContext（接着实现）

插入位置：第 20 步代码之后，紧接追加；完成稿约第 494 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 21 bytes=17099:17114 -->
````go
	return nil
}

````

## 第 22 步：commentReceipt

插入位置：第 21 步代码之后，紧接追加；完成稿约第 497 行。

本步声明：`func commentReceipt(value map[string]any) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>map&#91;string&#93;any</code> | 在此声明中的实际使用：<code>result := value&#91;&quot;result&quot;&#93;.(type)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 由 <code>domain.CanonicalID(raw)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>domain.CanonicalID(raw)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return domain.CanonicalID(raw)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 22 bytes=17114:17492 -->
````go
func commentReceipt(value map[string]any) (uint64, error) {
	// 首次结果和持久回执反序列化后的 map 值类型不同，但都只读取原实体编号。
	var raw string
	switch result := value["result"].(type) {
	case map[string]string:
		raw = result["entity_id"]
	case map[string]any:
		raw, _ = result["entity_id"].(string)
	}
	return domain.CanonicalID(raw)
}

````

## 第 23 步：commentContextInput

插入位置：第 22 步代码之后，紧接追加；完成稿约第 509 行。

本步声明：`func commentContextInput(document uint64, input dto.CommentThreadCreate) (comments.Thread, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>Document: document</code> |
| <code>input</code> | <code>dto.CommentThreadCreate</code> | 在此声明中的实际使用：<code>input.Anchor</code>；<code>input.ContextId</code>；<code>input.ContextKind</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>comments.Thread</code> | 使用 <code>comments.Thread</code> 类型；其定义处的职责：保存线程、评论、回复墓碑、版本上下文和通知意图。<br>实际返回表达式：<code>comments.Thread{}</code>；<code>comments.Thread{Document: document, Kind: input.ContextKind, Context: input.ContextId, Version: version, Anchor: input.Anchor, Status: &quot;OPEN&quot;}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.InvalidRequest, &quot;COMMENT_CONTEXT_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return comments.Thread{}, err</code>。当前函数在这里结束。
- 检查 <code>input.ContextKind != &quot;MAIN&quot; &amp;&amp; input.ContextKind != &quot;BRANCH&quot; &amp;&amp; input.ContextKind != &quot;REVIEW&quot;</code>。条件成立时执行 <code>return comments.Thread{}, domain.Violation(domain.InvalidRequest, &quot;COMMENT_CONTEXT_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return comments.Thread{Document: document, Kind: input.ContextKind, Context: input.ContextId, Version: version, Anchor: input.Anchor, Status: &quot;OPEN&quot;}, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 23 bytes=17492:18166 -->
````go
func commentContextInput(document uint64, input dto.CommentThreadCreate) (comments.Thread, error) {
	version, err := domain.CanonicalID(input.Anchor.ContentVersionId)
	if err != nil {
		return comments.Thread{}, err
	}
	if _, err := domain.CanonicalID(input.ContextId); err != nil {
		return comments.Thread{}, err
	}
	if input.ContextKind != "MAIN" && input.ContextKind != "BRANCH" && input.ContextKind != "REVIEW" {
		return comments.Thread{}, domain.Violation(domain.InvalidRequest, "COMMENT_CONTEXT_INVALID")
	}
	return comments.Thread{Document: document, Kind: input.ContextKind, Context: input.ContextId, Version: version, Anchor: input.Anchor, Status: "OPEN"}, nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/logic/document/comments_service.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/09-comments-notifications.md)。
