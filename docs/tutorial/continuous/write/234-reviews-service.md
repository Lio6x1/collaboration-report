# 分步编写 `services/document/internal/logic/document/reviews_service.go`

先按 [D05 · 可靠送审与固定轮次](../lessons/document/05-submission-review.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/234-reviews-service.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

执行请求修改、撤回、解决意见和审核列表/历史的当前资格检查。

历史范围证明只允许阅读旧轮次，不能授予已退役单元新的编辑或再提交能力。

写之前先按这条执行路线理解各段的连接：

1. action锁当前request/round/revision并写审计
2. historicalContentRangeCovered核完整后继范围
3. List逐候选鉴权并推进已检查ID
4. requestView基于完整历史计算操作提示。

本文件需要守住的边界：

- 不能根据浏览器加载了多少页判断是否允许合并
- 隐藏项不泄漏总数
- 组织目录仅优化候选范围。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/logic/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/logic/document/reviews_service.go') -and (Get-Item -LiteralPath 'services/document/internal/logic/document/reviews_service.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/logic/document/reviews_service.go')) { New-Item -ItemType File -Path 'services/document/internal/logic/document/reviews_service.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:534 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	codec "collaboration/contracts/content"
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
)

// review_actions.go：保留原实现，按业务合并文件。

````

## 第 2 步：reviewCommand

插入位置：第 1 步代码之后，紧接追加；完成稿约第 21 行。

本步声明：`type reviewCommand struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>operation</code> | <code>string</code> | 本次审核动作原操作号。 |
| <code>kind</code> | <code>string</code> | 审核动作种类，决定修改要求或审核状态路径。 |
| <code>round</code> | <code>uint64</code> | 动作绑定的审核轮次。 |
| <code>version</code> | <code>uint64</code> | 动作针对的固定正文版本。 |
| <code>change</code> | <code>uint64</code> | 修改要求编号。 |
| <code>changeRevision</code> | <code>uint64</code> | 用户看到的修改要求修订号。 |
| <code>reason</code> | <code>*string</code> | 可选操作理由；指针区分未给出与空字符串。 |

<!-- write-step: 2 bytes=534:707 -->
````go
type reviewCommand struct {
	operation, kind                        string
	round, version, change, changeRevision uint64
	reason                                 *string
}

````

## 第 3 步：Reviews.RequestChanges

插入位置：第 2 步代码之后，紧接追加；完成稿约第 27 行。

本步声明：`func (s *Reviews) RequestChanges(ctx context.Context, actor Actor, pointer domain.ReviewPointer, input dto.RequestChanges) (Actor, MutationBody, MutationPresentation, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.action(ctx, actor, pointer, reviewCommand{operation: input.OperationId, kind: &quot;REQUEST_CHANGES&quot;, round: round, version: version, reason: &amp;input.Reason})</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>return actor, nil, nil, err</code>；<code>return actor, nil, nil, domain.Violation(domain.InvalidRequest, &quot;REVIEW_CHANGES_INPUT_INVALID&quot;)</code>；<code>s.action(ctx, actor, pointer, reviewCommand{operation: input.OperationId, kind: &quot;REQUEST_CHANGES&quot;, round: round, version: version, reason: &amp;input.Reason})</code> |
| <code>pointer</code> | <code>domain.ReviewPointer</code> | 使用 <code>domain.ReviewPointer</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>s.action(ctx, actor, pointer, reviewCommand{operation: input.OperationId, kind: &quot;REQUEST_CHANGES&quot;, round: round, version: version, reason: &amp;input.Reason})</code> |
| <code>input</code> | <code>dto.RequestChanges</code> | 在此声明中的实际使用：<code>input.RoundId</code>；<code>input.ContentVersionId</code>；<code>input.Type</code>；<code>input.Reason</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>由 <code>s.action(ctx, actor, pointer, reviewCommand{operation: input.OperationId, kind: &quot;REQUEST_CHANGES&quot;, round: round, version: version, reason: &amp;input.Reason})</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>actor</code> |
| <code>(匿名/未命名)</code> | <code>MutationBody</code> | 使用 <code>MutationBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>由 <code>s.action(ctx, actor, pointer, reviewCommand{operation: input.OperationId, kind: &quot;REQUEST_CHANGES&quot;, round: round, version: version, reason: &amp;input.Reason})</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>nil</code> |
| <code>(匿名/未命名)</code> | <code>MutationPresentation</code> | 使用 <code>MutationPresentation</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>由 <code>s.action(ctx, actor, pointer, reviewCommand{operation: input.OperationId, kind: &quot;REQUEST_CHANGES&quot;, round: round, version: version, reason: &amp;input.Reason})</code> 的第 3 个返回项原样转交。<br>实际返回表达式：<code>nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>s.action(ctx, actor, pointer, reviewCommand{operation: input.OperationId, kind: &quot;REQUEST_CHANGES&quot;, round: round, version: version, reason: &amp;input.Reason})</code> 的第 4 个返回项原样转交。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.InvalidRequest, &quot;REVIEW_CHANGES_INPUT_INVALID&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>input.Type != &quot;REQUEST_CHANGES&quot; &#124;&#124; !domain.ValidName(input.Reason, 2000)</code>。条件成立时执行 <code>return actor, nil, nil, domain.Violation(domain.InvalidRequest, &quot;REVIEW_CHANGES_INPUT_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return s.action(ctx, actor, pointer, reviewCommand{operation: input.OperationId, kind: &quot;REQUEST_CHANGES&quot;, round: round, version: version, reason: &amp;input.Reason})</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=707:1438 -->
````go
func (s *Reviews) RequestChanges(ctx context.Context, actor Actor, pointer domain.ReviewPointer, input dto.RequestChanges) (Actor, MutationBody, MutationPresentation, error) {
	round, err := domain.CanonicalID(input.RoundId)
	if err != nil {
		return actor, nil, nil, err
	}
	version, err := domain.CanonicalID(input.ContentVersionId)
	if err != nil {
		return actor, nil, nil, err
	}
	if input.Type != "REQUEST_CHANGES" || !domain.ValidName(input.Reason, 2000) {
		return actor, nil, nil, domain.Violation(domain.InvalidRequest, "REVIEW_CHANGES_INPUT_INVALID")
	}
	return s.action(ctx, actor, pointer, reviewCommand{operation: input.OperationId, kind: "REQUEST_CHANGES", round: round, version: version, reason: &input.Reason})
}

````

## 第 4 步：Reviews.Withdraw

插入位置：第 3 步代码之后，紧接追加；完成稿约第 42 行。

本步声明：`func (s *Reviews) Withdraw(ctx context.Context, actor Actor, pointer domain.ReviewPointer, input dto.Withdrawal) (Actor, MutationBody, MutationPresentation, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.action(ctx, actor, pointer, reviewCommand{operation: input.OperationId, kind: &quot;WITHDRAW&quot;, round: round, reason: input.Reason})</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>return actor, nil, nil, err</code>；<code>return actor, nil, nil, domain.Violation(domain.InvalidRequest, &quot;REVIEW_REASON_INVALID&quot;)</code>；<code>s.action(ctx, actor, pointer, reviewCommand{operation: input.OperationId, kind: &quot;WITHDRAW&quot;, round: round, reason: input.Reason})</code> |
| <code>pointer</code> | <code>domain.ReviewPointer</code> | 使用 <code>domain.ReviewPointer</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>s.action(ctx, actor, pointer, reviewCommand{operation: input.OperationId, kind: &quot;WITHDRAW&quot;, round: round, reason: input.Reason})</code> |
| <code>input</code> | <code>dto.Withdrawal</code> | 在此声明中的实际使用：<code>input.ExpectedRoundId</code>；<code>input.Reason</code>；<code>input.OperationId</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>由 <code>s.action(ctx, actor, pointer, reviewCommand{operation: input.OperationId, kind: &quot;WITHDRAW&quot;, round: round, reason: input.Reason})</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>actor</code> |
| <code>(匿名/未命名)</code> | <code>MutationBody</code> | 使用 <code>MutationBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>由 <code>s.action(ctx, actor, pointer, reviewCommand{operation: input.OperationId, kind: &quot;WITHDRAW&quot;, round: round, reason: input.Reason})</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>nil</code> |
| <code>(匿名/未命名)</code> | <code>MutationPresentation</code> | 使用 <code>MutationPresentation</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>由 <code>s.action(ctx, actor, pointer, reviewCommand{operation: input.OperationId, kind: &quot;WITHDRAW&quot;, round: round, reason: input.Reason})</code> 的第 3 个返回项原样转交。<br>实际返回表达式：<code>nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>s.action(ctx, actor, pointer, reviewCommand{operation: input.OperationId, kind: &quot;WITHDRAW&quot;, round: round, reason: input.Reason})</code> 的第 4 个返回项原样转交。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.InvalidRequest, &quot;REVIEW_REASON_INVALID&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>input.Reason != nil &amp;&amp; !domain.ValidName(*input.Reason, 2000)</code>。条件成立时执行 <code>return actor, nil, nil, domain.Violation(domain.InvalidRequest, &quot;REVIEW_REASON_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return s.action(ctx, actor, pointer, reviewCommand{operation: input.OperationId, kind: &quot;WITHDRAW&quot;, round: round, reason: input.Reason})</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=1438:2013 -->
````go
func (s *Reviews) Withdraw(ctx context.Context, actor Actor, pointer domain.ReviewPointer, input dto.Withdrawal) (Actor, MutationBody, MutationPresentation, error) {
	round, err := domain.CanonicalID(input.ExpectedRoundId)
	if err != nil {
		return actor, nil, nil, err
	}
	if input.Reason != nil && !domain.ValidName(*input.Reason, 2000) {
		return actor, nil, nil, domain.Violation(domain.InvalidRequest, "REVIEW_REASON_INVALID")
	}
	return s.action(ctx, actor, pointer, reviewCommand{operation: input.OperationId, kind: "WITHDRAW", round: round, reason: input.Reason})
}

````

## 第 5 步：Reviews.Resolve

插入位置：第 4 步代码之后，紧接追加；完成稿约第 53 行。

本步声明：`func (s *Reviews) Resolve(ctx context.Context, actor Actor, pointer domain.ReviewPointer, change uint64, input dto.ResolveChanges) (Actor, MutationBody, MutationPresentation, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.action(ctx, actor, pointer, reviewCommand{operation: input.OperationId, kind: &quot;RESOLVE_CHANGES&quot;, round: round, change: change, changeRevision: revision, reason: &amp;input.Reason})</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>return actor, nil, nil, err</code>；<code>return actor, nil, nil, domain.Violation(domain.InvalidRequest, &quot;REVIEW_REASON_INVALID&quot;)</code>；<code>s.action(ctx, actor, pointer, reviewCommand{operation: input.OperationId, kind: &quot;RESOLVE_CHANGES&quot;, round: round, change: change, changeRevision: revision, reason: &amp;input.Reason})</code> |
| <code>pointer</code> | <code>domain.ReviewPointer</code> | 使用 <code>domain.ReviewPointer</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>s.action(ctx, actor, pointer, reviewCommand{operation: input.OperationId, kind: &quot;RESOLVE_CHANGES&quot;, round: round, change: change, changeRevision: revision, reason: &amp;input.Reason})</code> |
| <code>change</code> | <code>uint64</code> | 在此声明中的实际使用：<code>change: change</code> |
| <code>input</code> | <code>dto.ResolveChanges</code> | 在此声明中的实际使用：<code>input.ExpectedRoundId</code>；<code>input.ExpectedChangeRevision</code>；<code>input.Reason</code>；<code>input.OperationId</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>由 <code>s.action(ctx, actor, pointer, reviewCommand{operation: input.OperationId, kind: &quot;RESOLVE_CHANGES&quot;, round: round, change: change, changeRevision: revision, reason: &amp;input.Reason})</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>actor</code> |
| <code>(匿名/未命名)</code> | <code>MutationBody</code> | 使用 <code>MutationBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>由 <code>s.action(ctx, actor, pointer, reviewCommand{operation: input.OperationId, kind: &quot;RESOLVE_CHANGES&quot;, round: round, change: change, changeRevision: revision, reason: &amp;input.Reason})</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>nil</code> |
| <code>(匿名/未命名)</code> | <code>MutationPresentation</code> | 使用 <code>MutationPresentation</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>由 <code>s.action(ctx, actor, pointer, reviewCommand{operation: input.OperationId, kind: &quot;RESOLVE_CHANGES&quot;, round: round, change: change, changeRevision: revision, reason: &amp;input.Reason})</code> 的第 3 个返回项原样转交。<br>实际返回表达式：<code>nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>s.action(ctx, actor, pointer, reviewCommand{operation: input.OperationId, kind: &quot;RESOLVE_CHANGES&quot;, round: round, change: change, changeRevision: revision, reason: &amp;input.Reason})</code> 的第 4 个返回项原样转交。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.InvalidRequest, &quot;REVIEW_REASON_INVALID&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>!domain.ValidName(input.Reason, 2000)</code>。条件成立时执行 <code>return actor, nil, nil, domain.Violation(domain.InvalidRequest, &quot;REVIEW_REASON_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return s.action(ctx, actor, pointer, reviewCommand{operation: input.OperationId, kind: &quot;RESOLVE_CHANGES&quot;, round: round, change: change, changeRevision: revision, reason: &amp;input.Reason})</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=2013:2749 -->
````go
func (s *Reviews) Resolve(ctx context.Context, actor Actor, pointer domain.ReviewPointer, change uint64, input dto.ResolveChanges) (Actor, MutationBody, MutationPresentation, error) {
	round, err := domain.CanonicalID(input.ExpectedRoundId)
	if err != nil {
		return actor, nil, nil, err
	}
	revision, err := domain.CanonicalID(input.ExpectedChangeRevision)
	if err != nil {
		return actor, nil, nil, err
	}
	if !domain.ValidName(input.Reason, 2000) {
		return actor, nil, nil, domain.Violation(domain.InvalidRequest, "REVIEW_REASON_INVALID")
	}
	return s.action(ctx, actor, pointer, reviewCommand{operation: input.OperationId, kind: "RESOLVE_CHANGES", round: round, change: change, changeRevision: revision, reason: &input.Reason})
}

````

## 第 6 步：Reviews.action

插入位置：第 5 步代码之后，紧接追加；完成稿约第 68 行。

本步声明：`func (s *Reviews) action(ctx context.Context, actor Actor, pointer domain.ReviewPointer, command reviewCommand) (Actor, MutationBody, MutationPresentation, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.authorized(ctx, actor, pointer, &quot;REVIEW_ACTION&quot;)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor, access, err := s.authorized(ctx, actor, pointer, &quot;REVIEW_ACTION&quot;)</code>；<code>s.authorized(ctx, actor, pointer, &quot;REVIEW_ACTION&quot;)</code>；<code>return actor, nil, nil, err</code>；<code>actor.ID</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>pointer</code> | <code>domain.ReviewPointer</code> | 使用 <code>domain.ReviewPointer</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>s.authorized(ctx, actor, pointer, &quot;REVIEW_ACTION&quot;)</code>；<code>pointer.Unit</code>；<code>pointer.ID</code> |
| <code>command</code> | <code>reviewCommand</code> | 在此声明中的实际使用：<code>command.round</code>；<code>command.kind</code>；<code>command.version</code>；<code>command.change</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |


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
- 在本段的匿名函数内：检查 <code>command.kind == &quot;WITHDRAW&quot;</code>。
- 在本段的匿名函数内：检查 <code>!access.Permissions.Edit&#91;current.Unit&#93;</code>。条件成立时执行 <code>return nil, domain.Violation(domain.PermissionDenied, &quot;REVIEW_WITHDRAW_EDIT_REQUIRED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>guard.Ongoing != current.ID &#124;&#124; guard.Submission != &quot;&quot;</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;UNIT_REVIEW_POSITION_CHANGED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>command.kind == &quot;REQUEST_CHANGES&quot;</code>。
- 在本段的匿名函数内：检查 <code>!access.Permissions.Review</code>。条件成立时执行 <code>return nil, domain.Violation(domain.PermissionDenied, &quot;REVIEW_REQUIRED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>version != command.version</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;REVIEW_CONTENT_CHANGED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err == sql.ErrNoRows</code>。条件成立时执行 <code>return nil, domain.Violation(domain.EntityNotFound, &quot;CHANGE_REQUEST_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。

<!-- write-step: 6 bytes=2749:4597 -->
````go
func (s *Reviews) action(ctx context.Context, actor Actor, pointer domain.ReviewPointer, command reviewCommand) (Actor, MutationBody, MutationPresentation, error) {
	actor, access, err := s.authorized(ctx, actor, pointer, "REVIEW_ACTION")
	if err != nil {
		return actor, nil, nil, err
	}
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		if _, err := ValidateAccess(ctx, tx, access); err != nil {
			return nil, err
		}
		// 与送审、合并遵守同一顺序：来源→文档→单元 guard→请求；不持锁调用远端。
		guard, err := repository.LockedReviewGuard(ctx, tx, pointer.Unit)
		if err != nil {
			return nil, err
		}
		current, err := repository.LockedReviewPointer(ctx, tx, pointer.ID)
		if err != nil {
			return nil, err
		}
		if err = domain.ValidateReviewRound(current, command.round); err != nil {
			return nil, err
		}
		if command.kind == "WITHDRAW" {
			if !access.Permissions.Edit[current.Unit] {
				return nil, domain.Violation(domain.PermissionDenied, "REVIEW_WITHDRAW_EDIT_REQUIRED")
			}
			if guard.Ongoing != current.ID || guard.Submission != "" {
				return nil, domain.Violation(domain.StateConflict, "UNIT_REVIEW_POSITION_CHANGED")
			}
		} else if command.kind == "REQUEST_CHANGES" {
			if !access.Permissions.Review {
				return nil, domain.Violation(domain.PermissionDenied, "REVIEW_REQUIRED")
			}
			version, err := repository.ReviewRoundVersion(ctx, tx, current.ID, current.Round)
			if err != nil {
				return nil, err
			}
			if version != command.version {
				return nil, domain.Violation(domain.StateConflict, "REVIEW_CONTENT_CHANGED")
			}
		} else {
			change, err := repository.LockedChange(ctx, tx, current.ID, command.change)
			if err == sql.ErrNoRows {
				return nil, domain.Violation(domain.EntityNotFound, "CHANGE_REQUEST_NOT_FOUND")
			}
			if err != nil {
````

## 第 7 步：Reviews.action（接着实现）

插入位置：第 6 步代码之后，紧接追加；完成稿约第 113 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：此处通过 <code>return nil, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>change.Status != &quot;OPEN&quot; &#124;&#124; change.Revision != command.changeRevision</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;CHANGE_REQUEST_CHANGED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>!domain.CanResolveChange(access.Permissions, actor.ID, change, ever)</code>。条件成立时执行 <code>return nil, domain.Violation(domain.PermissionDenied, &quot;CHANGE_RESOLUTION_NOT_ALLOWED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>command.kind != &quot;WITHDRAW&quot;</code>。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return map&#91;string&#93;any{&quot;operation_id&quot;: command.operation, &quot;state&quot;: &quot;SUCCEEDED&quot;, &quot;result&quot;: map&#91;string&#93;string{&quot;entity_id&quot;: domain.TextID(entity), &quot;request_id&quot;: domain.TextID(current.I …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。

<!-- write-step: 7 bytes=4597:6451 -->
````go
				return nil, err
			}
			if change.Status != "OPEN" || change.Revision != command.changeRevision {
				return nil, domain.Violation(domain.StateConflict, "CHANGE_REQUEST_CHANGED")
			}
			ever, err := repository.EverSubmitted(ctx, tx, current.ID, actor.ID)
			if err != nil {
				return nil, err
			}
			if !domain.CanResolveChange(access.Permissions, actor.ID, change, ever) {
				return nil, domain.Violation(domain.PermissionDenied, "CHANGE_RESOLUTION_NOT_ALLOWED")
			}
		}
		action, err := repository.InsertReviewAction(ctx, tx, command.operation, command.kind, current.ID, current.Round, actor.ID, command.version, command.change, command.reason)
		if err != nil {
			return nil, err
		}
		entity := action
		switch command.kind {
		case "REQUEST_CHANGES":
			entity, err = repository.InsertChange(ctx, tx, current.ID, current.Round, action, actor.ID)
		case "RESOLVE_CHANGES":
			entity = command.change
			err = repository.ResolveChange(ctx, tx, command.change, action)
		case "WITHDRAW":
			entity = current.ID
			err = repository.WithdrawReview(ctx, tx, current)
		}
		if err != nil {
			return nil, err
		}
		if command.kind != "WITHDRAW" {
			if err = repository.AdvanceReviewRevision(ctx, tx, current.ID); err != nil {
				return nil, err
			}
		}
		if err = repository.QueueReviewActionNotification(ctx, tx, command.operation, command.kind, current, action, actor.ID); err != nil {
			return nil, err
		}
		return map[string]any{"operation_id": command.operation, "state": "SUCCEEDED", "result": map[string]string{"entity_id": domain.TextID(entity), "request_id": domain.TextID(current.ID), "round_id": domain.TextID(current.Round)}}, nil
	}
	present := func(ctx context.Context, tx *sql.Tx, value map[string]any) (any, []*pb.SourceEpoch, error) {
		if _, err := ValidateAccess(ctx, tx, access); err != nil {
			return nil, nil, err
		}
````

## 第 8 步：Reviews.action（接着实现）

插入位置：第 7 步代码之后，紧接追加；完成稿约第 158 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return response, access.Sources, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return actor, run, present, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=6451:7106 -->
````go
		current, err := repository.ReviewPointerRow(ctx, tx, pointer.ID)
		if err != nil {
			return nil, nil, err
		}
		// 固定结果和当前状态分别展示；同号重试不重新执行动作，不返回永久正文访问凭证。
		response := map[string]any{}
		for key, item := range value {
			response[key] = item
		}
		response["current_state"] = map[string]string{"review_state": current.Status, "current_round_id": domain.TextID(current.Round), "revision": domain.TextID(current.Revision)}
		return response, access.Sources, nil
	}
	return actor, run, present, nil
}

// review_historical_access.go：保留原实现，按业务合并文件。

````

## 第 9 步：historicalReviewPurpose

插入位置：第 8 步代码之后，紧接追加；完成稿约第 175 行。

本步声明：`func historicalReviewPurpose(purpose string) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>purpose</code> | <code>string</code> | 在此声明中的实际使用：<code>purpose == &quot;READ_REVIEW&quot;</code>；<code>purpose == &quot;READ_REVIEW_ROUND&quot;</code>；<code>purpose == &quot;READ_REVIEW_HISTORY&quot;</code>；<code>purpose == &quot;LIST_REVIEWS&quot;</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>purpose == &quot;READ_REVIEW&quot; &#124;&#124; purpose == &quot;READ_REVIEW_ROUND&quot; &#124;&#124; purpose == &quot;READ_REVIEW_HISTORY&quot; &#124;&#124; purpose == &quot;LIST_REVIEWS&quot;</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return purpose == &quot;READ_REVIEW&quot; &#124;&#124; purpose == &quot;READ_REVIEW_ROUND&quot; &#124;&#124; purpose == &quot;READ_REVIEW_HISTORY&quot; &#124;&#124; purpose == &quot;LIST_REVIEWS&quot;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=7106:7293 -->
````go
func historicalReviewPurpose(purpose string) bool {
	return purpose == "READ_REVIEW" || purpose == "READ_REVIEW_ROUND" || purpose == "READ_REVIEW_HISTORY" || purpose == "LIST_REVIEWS"
}

````

## 第 10 步：Reviews.reviewReadAllowed

插入位置：第 9 步代码之后，紧接追加；完成稿约第 179 行。

本步声明：`func (s *Reviews) reviewReadAllowed(ctx context.Context, access DocumentAccess, pointer domain.ReviewPointer) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, nil)</code>；<code>LockSources(ctx, tx, access.Sources)</code>；<code>ValidateAccess(ctx, tx, access)</code>；<code>s.historicalReviewCovered(ctx, tx, access, pointer)</code> |
| <code>access</code> | <code>DocumentAccess</code> | 使用 <code>DocumentAccess</code> 类型；其定义处的职责：把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。<br>在此声明中的实际使用：<code>reviewContextAllowed(access, pointer)</code>；<code>access.Sources</code>；<code>ValidateAccess(ctx, tx, access)</code>；<code>s.historicalReviewCovered(ctx, tx, access, pointer)</code> |
| <code>pointer</code> | <code>domain.ReviewPointer</code> | 使用 <code>domain.ReviewPointer</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>reviewContextAllowed(access, pointer)</code>；<code>s.historicalReviewCovered(ctx, tx, access, pointer)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>true</code>；<code>false</code>；<code>allowed</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>err</code>；<code>tx.Commit()</code> |

**跟着本段代码走一遍：**

- 检查 <code>reviewContextAllowed(access, pointer)</code>。条件成立时执行 <code>return true, nil</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 此处通过 <code>return allowed, tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 10 bytes=7293:7888 -->
````go
func (s *Reviews) reviewReadAllowed(ctx context.Context, access DocumentAccess, pointer domain.ReviewPointer) (bool, error) {
	if reviewContextAllowed(access, pointer) {
		return true, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	if err = LockSources(ctx, tx, access.Sources); err != nil {
		return false, err
	}
	if _, err = ValidateAccess(ctx, tx, access); err != nil {
		return false, err
	}
	allowed, err := s.historicalReviewCovered(ctx, tx, access, pointer)
	if err != nil {
		return false, err
	}
	return allowed, tx.Commit()
}

````

## 第 11 步：Reviews.validateReviewReadAccess

插入位置：第 10 步代码之后，紧接追加；完成稿约第 201 行。

本步声明：`func (s *Reviews) validateReviewReadAccess(ctx context.Context, tx *sql.Tx, access DocumentAccess, pointer domain.ReviewPointer) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ValidateAccess(ctx, tx, access)</code>；<code>s.historicalReviewCovered(ctx, tx, access, pointer)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>ValidateAccess(ctx, tx, access)</code>；<code>s.historicalReviewCovered(ctx, tx, access, pointer)</code> |
| <code>access</code> | <code>DocumentAccess</code> | 使用 <code>DocumentAccess</code> 类型；其定义处的职责：把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。<br>在此声明中的实际使用：<code>ValidateAccess(ctx, tx, access)</code>；<code>reviewContextAllowed(access, pointer)</code>；<code>s.historicalReviewCovered(ctx, tx, access, pointer)</code> |
| <code>pointer</code> | <code>domain.ReviewPointer</code> | 使用 <code>domain.ReviewPointer</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>reviewContextAllowed(access, pointer)</code>；<code>s.historicalReviewCovered(ctx, tx, access, pointer)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code>；<code>domain.Violation(domain.PermissionDenied, &quot;REVIEW_CONTEXT_NOT_ALLOWED&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>reviewContextAllowed(access, pointer)</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!allowed</code>。条件成立时执行 <code>return domain.Violation(domain.PermissionDenied, &quot;REVIEW_CONTEXT_NOT_ALLOWED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=7888:8377 -->
````go
func (s *Reviews) validateReviewReadAccess(ctx context.Context, tx *sql.Tx, access DocumentAccess, pointer domain.ReviewPointer) error {
	if _, err := ValidateAccess(ctx, tx, access); err != nil {
		return err
	}
	if reviewContextAllowed(access, pointer) {
		return nil
	}
	allowed, err := s.historicalReviewCovered(ctx, tx, access, pointer)
	if err != nil {
		return err
	}
	if !allowed {
		return domain.Violation(domain.PermissionDenied, "REVIEW_CONTEXT_NOT_ALLOWED")
	}
	return nil
}

````

## 第 12 步：Reviews.historicalReviewCovered

插入位置：第 11 步代码之后，紧接追加；完成稿约第 218 行。

历史授权只服务只读用例，绝不把旧unit加入EDIT集合；旧请求的撤回/再提交资格仍由当前可编辑单元决定。

本步声明：`func (s *Reviews) historicalReviewCovered(ctx context.Context, tx *sql.Tx, access DocumentAccess, pointer domain.ReviewPointer) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>historicalContentRangeCovered(ctx, tx, access, pointer.Document, pointer.Unit, s.validator)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>historicalContentRangeCovered(ctx, tx, access, pointer.Document, pointer.Unit, s.validator)</code> |
| <code>access</code> | <code>DocumentAccess</code> | 使用 <code>DocumentAccess</code> 类型；其定义处的职责：把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。<br>在此声明中的实际使用：<code>historicalContentRangeCovered(ctx, tx, access, pointer.Document, pointer.Unit, s.validator)</code> |
| <code>pointer</code> | <code>domain.ReviewPointer</code> | 使用 <code>domain.ReviewPointer</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>pointer.Document</code>；<code>pointer.Unit</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 由 <code>historicalContentRangeCovered(ctx, tx, access, pointer.Document, pointer.Unit, s.validator)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>historicalContentRangeCovered(ctx, tx, access, pointer.Document, pointer.Unit, s.validator)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return historicalContentRangeCovered(ctx, tx, access, pointer.Document, pointer.Unit, s.validator)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=8377:8769 -->
````go
// 历史授权只服务只读用例，绝不把旧unit加入EDIT集合；旧请求的撤回/再提交资格仍由当前可编辑单元决定。
func (s *Reviews) historicalReviewCovered(ctx context.Context, tx *sql.Tx, access DocumentAccess, pointer domain.ReviewPointer) (bool, error) {
	return historicalContentRangeCovered(ctx, tx, access, pointer.Document, pointer.Unit, s.validator)
}

````

## 第 13 步：historicalContentRangeCovered

插入位置：第 12 步代码之后，紧接追加；完成稿约第 223 行。

固定版本与审核历史共用同一原范围证明；抽取后不改变映射证据、预算或失败规则。

本步声明：`func historicalContentRangeCovered(ctx context.Context, tx *sql.Tx, access DocumentAccess, document, originalUnit uint64, validator *codec.Validator) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.CurrentStructurePreviewUnits(ctx, tx, document, structure)</code>；<code>repository.VersionContent(ctx, tx, id, document)</code>；<code>repository.HistoricalOutgoingMappings(ctx, tx, document, unit)</code>；<code>readHistoricalStructureWitness(ctx, tx, row.Operation, document, validator)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.CurrentStructurePreviewUnits(ctx, tx, document, structure)</code>；<code>repository.VersionContent(ctx, tx, id, document)</code>；<code>repository.HistoricalOutgoingMappings(ctx, tx, document, unit)</code>；<code>readHistoricalStructureWitness(ctx, tx, row.Operation, document, validator)</code> |
| <code>access</code> | <code>DocumentAccess</code> | 使用 <code>DocumentAccess</code> 类型；其定义处的职责：把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。<br>在此声明中的实际使用：<code>access.Document</code>；<code>access.Permissions</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>domain.TextID(document)</code>；<code>repository.CurrentStructurePreviewUnits(ctx, tx, document, structure)</code>；<code>repository.VersionContent(ctx, tx, id, document)</code>；<code>repository.HistoricalOutgoingMappings(ctx, tx, document, unit)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>originalUnit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>queue := &#91;&#93;uint64{originalUnit}</code>；<code>domain.HistoricalRangeCovered(access.Permissions.Edit, originalUnit, current, edges)</code> |
| <code>validator</code> | <code>*codec.Validator</code> | 冻结协议的结构校验器，先限制请求形状，再由业务核验身份、权限、版本和事务条件。<br>在此声明中的实际使用：<code>validator == nil</code>；<code>readHistoricalStructureWitness(ctx, tx, row.Operation, document, validator)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>domain.HistoricalRangeCovered(access.Permissions.Edit, originalUnit, current, edges)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;HISTORICAL_RANGE_VALIDATOR_UNAVAILABLE&quot;)</code>；<code>err</code>；<code>limit()</code> |

**跟着本段代码走一遍：**

- 检查 <code>access.Document.Id != domain.TextID(document) &#124;&#124; len(access.Permissions.Edit) == 0</code>。条件成立时执行 <code>return false, nil</code>。当前函数在这里结束。
- 检查 <code>validator == nil</code>。条件成立时执行 <code>return false, domain.Violation(domain.TemporarilyUnavailable, &quot;HISTORICAL_RANGE_VALIDATOR_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;HISTORICAL_RANGE_LIMIT&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>content != nil</code>。条件成立时执行 <code>return content, nil</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>bytesRead &gt; 128&lt;&lt;20</code>。条件成立时执行 <code>return nil, limit()</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return content, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>seen&#91;unit&#93; &#124;&#124; current&#91;unit&#93;</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。

<!-- write-step: 13 bytes=8769:10437 -->
````go
// 固定版本与审核历史共用同一原范围证明；抽取后不改变映射证据、预算或失败规则。
func historicalContentRangeCovered(ctx context.Context, tx *sql.Tx, access DocumentAccess, document, originalUnit uint64, validator *codec.Validator) (bool, error) {
	if access.Document.Id != domain.TextID(document) || len(access.Permissions.Edit) == 0 {
		return false, nil
	}
	if validator == nil {
		return false, domain.Violation(domain.TemporarilyUnavailable, "HISTORICAL_RANGE_VALIDATOR_UNAVAILABLE")
	}
	structure, err := domain.CanonicalID(access.Document.CurrentStructureId)
	if err != nil {
		return false, err
	}
	units, err := repository.CurrentStructurePreviewUnits(ctx, tx, document, structure)
	if err != nil {
		return false, err
	}
	current := map[uint64]bool{}
	for _, unit := range units {
		current[unit.Unit] = true
	}
	witnesses := map[string]historicalStructureWitness{}
	versions := map[uint64]*pb.FixedContent{}
	seen := map[uint64]bool{}
	queue := []uint64{originalUnit}
	edges := []domain.HistoricalRangeEdge{}
	bytesRead := 0
	limit := func() error { return domain.Violation(domain.TemporarilyUnavailable, "HISTORICAL_RANGE_LIMIT") }
	version := func(id uint64) (*pb.FixedContent, error) {
		if content := versions[id]; content != nil {
			return content, nil
		}
		content, err := repository.VersionContent(ctx, tx, id, document)
		if err != nil {
			return nil, err
		}
		bytesRead += len(content.BodyJsonUtf8)
		if bytesRead > 128<<20 {
			return nil, limit()
		}
		versions[id] = content
		return content, nil
	}
	for index := 0; index < len(queue); index++ {
		unit := queue[index]
		if seen[unit] || current[unit] {
````

## 第 14 步：historicalContentRangeCovered（接着实现）

插入位置：第 13 步代码之后，紧接追加；完成稿约第 268 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>len(seen) &gt; 100000</code>。条件成立时执行 <code>return false, limit()</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false, err</code>。当前函数在这里结束。
- 检查 <code>len(rows) == 0</code>。条件成立时执行 <code>return false, nil</code>。当前函数在这里结束。
- 检查 <code>len(rows) &gt; 10000 &#124;&#124; len(edges)+len(rows) &gt; 100000</code>。条件成立时执行 <code>return false, limit()</code>。当前函数在这里结束。
- 检查 <code>!exists</code>。
- 检查 <code>err != nil</code>。
- 检查 <code>errors.As(err, &amp;rule) &amp;&amp; rule.Kind == domain.PermissionDenied</code>。条件成立时执行 <code>return false, nil</code>。当前函数在这里结束。
- 此处通过 <code>return false, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>bytesRead &gt; 128&lt;&lt;20</code>。条件成立时执行 <code>return false, limit()</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 检查 <code>err == sql.ErrNoRows</code>。条件成立时执行 <code>return false, nil</code>。当前函数在这里结束。
- 此处通过 <code>return false, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 检查 <code>err == sql.ErrNoRows</code>。条件成立时执行 <code>return false, nil</code>。当前函数在这里结束。

<!-- write-step: 14 bytes=10437:11567 -->
````go
			continue
		}
		seen[unit] = true
		if len(seen) > 100000 {
			return false, limit()
		}
		rows, err := repository.HistoricalOutgoingMappings(ctx, tx, document, unit)
		if err != nil {
			return false, err
		}
		if len(rows) == 0 {
			return false, nil
		}
		if len(rows) > 10000 || len(edges)+len(rows) > 100000 {
			return false, limit()
		}
		for _, row := range rows {
			witness, exists := witnesses[row.Operation]
			if !exists {
				witness, err = readHistoricalStructureWitness(ctx, tx, row.Operation, document, validator)
				if err != nil {
					var rule domain.RuleViolation
					if errors.As(err, &rule) && rule.Kind == domain.PermissionDenied {
						return false, nil
					}
					return false, err
				}
				bytesRead += witness.Bytes
				if bytesRead > 128<<20 {
					return false, limit()
				}
				witnesses[row.Operation] = witness
			}
			source, err := version(row.SourceVersion)
			if err != nil {
				if err == sql.ErrNoRows {
					return false, nil
				}
				return false, err
			}
			target, err := version(row.TargetVersion)
			if err != nil {
				if err == sql.ErrNoRows {
					return false, nil
				}
````

## 第 15 步：historicalContentRangeCovered（接着实现）

插入位置：第 14 步代码之后，紧接追加；完成稿约第 313 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return false, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!ok</code>。条件成立时执行 <code>return false, nil</code>。当前函数在这里结束。
- 此处通过 <code>return domain.HistoricalRangeCovered(access.Permissions.Edit, originalUnit, current, edges), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=11567:11943 -->
````go
				return false, err
			}
			edge, ok := proveHistoricalMapping(row, source, target, witness)
			if !ok {
				return false, nil
			}
			edges = append(edges, edge)
			queue = append(queue, edge.Target)
		}
	}
	return domain.HistoricalRangeCovered(access.Permissions.Edit, originalUnit, current, edges), nil
}

// review_lists.go：保留原实现，按业务合并文件。

````

## 第 16 步：Reviews.List

插入位置：第 15 步代码之后，紧接追加；完成稿约第 328 行。

列表候选范围、分页推进与正文授权分开；游标只推进已实际检查的ID，不返回隐藏项总数。

本步声明：`func (s *Reviews) List(ctx context.Context, actor Actor, document uint64, limitText, cursorText string) ([]*pb.SourceEpoch, ReadBody, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, document, &quot;LIST_REVIEWS&quot;)</code>；<code>s.organizationReviewScope(ctx, actor)</code>；<code>repository.ScopedReviewRequestCandidates(ctx, s.db, actor.ID, organizations, last, limit*10+1)</code>；<code>repository.ReviewRequestCandidates(ctx, s.db, document, last, limit*10+1)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor.ID</code>；<code>actor.Proof</code>；<code>s.access.Authorize(ctx, actor, document, &quot;LIST_REVIEWS&quot;)</code>；<code>s.organizationReviewScope(ctx, actor)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>domain.TextID(document)</code>；<code>document != 0</code>；<code>s.access.Authorize(ctx, actor, document, &quot;LIST_REVIEWS&quot;)</code>；<code>document == 0</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>limitText</code> | <code>string</code> | 在此声明中的实际使用：<code>s.pagination.Parse(limitText, cursorText, binding)</code> |
| <code>cursorText</code> | <code>string</code> | 在此声明中的实际使用：<code>s.pagination.Parse(limitText, cursorText, binding)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 实际返回表达式：<code>nil</code>；<code>sources</code> |
| <code>(匿名/未命名)</code> | <code>ReadBody</code> | 使用 <code>ReadBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>scopeErr</code>；<code>domain.Violation(domain.StateConflict, &quot;REVIEW_LIST_SOURCE_CHANGED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>last == &quot;&quot;</code>。
- 检查 <code>document != 0</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>document == 0</code>。
- 检查 <code>scopeErr != nil</code>。条件成立时执行 <code>return nil, nil, scopeErr</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>index &gt;= limit*10</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。

<!-- write-step: 16 bytes=11943:13444 -->
````go
// 列表候选范围、分页推进与正文授权分开；游标只推进已实际检查的ID，不返回隐藏项总数。
func (s *Reviews) List(ctx context.Context, actor Actor, document uint64, limitText, cursorText string) ([]*pb.SourceEpoch, ReadBody, error) {
	binding := "review-list:" + domain.TextID(actor.ID) + ":" + domain.TextID(document)
	limit, last, err := s.pagination.Parse(limitText, cursorText, binding)
	if err != nil {
		return nil, nil, err
	}
	if last == "" {
		last = "0"
	}
	sources := append([]*pb.SourceEpoch{}, actor.Proof.Sources...)
	var documentAccess *DocumentAccess
	if document != 0 {
		access, err := s.access.Authorize(ctx, actor, document, "LIST_REVIEWS")
		if err != nil {
			return nil, nil, err
		}
		documentAccess = &access
		sources = access.Sources
	}
	var ids []uint64
	if document == 0 {
		organizations, scopeErr := s.organizationReviewScope(ctx, actor)
		if scopeErr != nil {
			return nil, nil, scopeErr
		}
		ids, err = repository.ScopedReviewRequestCandidates(ctx, s.db, actor.ID, organizations, last, limit*10+1)
	} else {
		ids, err = repository.ReviewRequestCandidates(ctx, s.db, document, last, limit*10+1)
	}
	if err != nil {
		return nil, nil, err
	}
	var selected []domain.ReviewPointer
	var accesses []DocumentAccess
	more := false
	scanned := last
	for index, id := range ids {
		if index >= limit*10 {
			more = true
			break
		}
		pointer, err := repository.ReviewPointerRow(ctx, s.db, id)
		if err != nil {
			return nil, nil, err
		}
````

## 第 17 步：Reviews.List（接着实现）

插入位置：第 16 步代码之后，紧接追加；完成稿约第 374 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>documentAccess != nil</code>。
- 检查 <code>err != nil</code>。
- 检查 <code>errors.As(err, &amp;rule) &amp;&amp; (rule.Kind == domain.PermissionDenied &#124;&#124; rule.Kind == domain.EntityNotFound)</code>。
- 此处通过 <code>return nil, nil, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!allowed &amp;&amp; document != 0</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>!allowed &#124;&#124; (document == 0 &amp;&amp; !access.Permissions.Review)</code>。
- 检查 <code>len(selected) == limit</code>。
- 检查 <code>known.ScopeKey == source.ScopeKey</code>。
- 检查 <code>known.Epoch != source.Epoch</code>。条件成立时执行 <code>return nil, nil, domain.Violation(domain.StateConflict, &quot;REVIEW_LIST_SOURCE_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>!found</code>。

<!-- write-step: 17 bytes=13444:14727 -->
````go
		var access DocumentAccess
		if documentAccess != nil {
			access = *documentAccess
		} else {
			access, err = s.access.Authorize(ctx, actor, pointer.Document, "LIST_REVIEWS")
			if err != nil {
				var rule domain.RuleViolation
				if errors.As(err, &rule) && (rule.Kind == domain.PermissionDenied || rule.Kind == domain.EntityNotFound) {
					scanned = domain.TextID(id)
					continue
				}
				return nil, nil, err
			}
		}
		allowed := reviewContextAllowed(access, pointer)
		if !allowed && document != 0 {
			allowed, err = s.reviewReadAllowed(ctx, access, pointer)
			if err != nil {
				return nil, nil, err
			}
		}
		if !allowed || (document == 0 && !access.Permissions.Review) {
			scanned = domain.TextID(id)
			continue
		}
		if len(selected) == limit {
			more = true
			break
		}
		selected = append(selected, pointer)
		accesses = append(accesses, access)
		scanned = domain.TextID(id)
		for _, source := range access.Sources {
			found := false
			for _, known := range sources {
				if known.ScopeKey == source.ScopeKey {
					if known.Epoch != source.Epoch {
						return nil, nil, domain.Violation(domain.StateConflict, "REVIEW_LIST_SOURCE_CHANGED")
					}
					found = true
					break
				}
			}
			if !found {
				sources = append(sources, source)
			}
		}
	}
````

## 第 18 步：Reviews.List（接着实现）

插入位置：第 17 步代码之后，紧接追加；完成稿约第 422 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：检查 <code>documentAccess != nil</code>。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>more</code>。
- 在本段的匿名函数内：此处通过 <code>return page, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return sources, run, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=14727:15512 -->
````go
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		if documentAccess != nil {
			if _, err := ValidateAccess(ctx, tx, *documentAccess); err != nil {
				return nil, err
			}
		}
		page := dto.ReviewRequestPage{Items: []dto.ReviewRequest{}, HasMore: more}
		for i, pointer := range selected {
			if err := s.validateReviewReadAccess(ctx, tx, accesses[i], pointer); err != nil {
				return nil, err
			}
			value, err := s.requestView(ctx, tx, pointer, actor.ID, accesses[i], false)
			if err != nil {
				return nil, err
			}
			page.Items = append(page.Items, value)
		}
		if more {
			page.NextCursor = s.pagination.Next(binding, scanned)
		}
		return page, nil
	}
	return sources, run, nil
}

// review_readings.go：保留原实现，按业务合并文件。

````

## 第 19 步：Reviews.requestView

插入位置：第 18 步代码之后，紧接追加；完成稿约第 449 行。

本步声明：`func (s *Reviews) requestView(ctx context.Context, tx *sql.Tx, pointer domain.ReviewPointer, actor uint64, access DocumentAccess, detail bool) (dto.ReviewRequest, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.ReviewRequestRow(ctx, tx, pointer.ID)</code>；<code>repository.ReviewRequestSummaryRow(ctx, tx, pointer.ID, round, pointer.Document)</code>；<code>repository.MergeApprovalFacts(ctx, tx, pointer.ID, actor)</code>；<code>repository.ReviewRoundRow(ctx, tx, pointer.ID, round, access.Document.Id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.ReviewRequestRow(ctx, tx, pointer.ID)</code>；<code>repository.ReviewRequestSummaryRow(ctx, tx, pointer.ID, round, pointer.Document)</code>；<code>repository.MergeApprovalFacts(ctx, tx, pointer.ID, actor)</code>；<code>repository.ReviewRoundRow(ctx, tx, pointer.ID, round, access.Document.Id)</code> |
| <code>pointer</code> | <code>domain.ReviewPointer</code> | 使用 <code>domain.ReviewPointer</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>pointer.ID</code>；<code>pointer.Document</code>；<code>pointer.Unit</code> |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.MergeApprovalFacts(ctx, tx, pointer.ID, actor)</code> |
| <code>access</code> | <code>DocumentAccess</code> | 使用 <code>DocumentAccess</code> 类型；其定义处的职责：把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。<br>在此声明中的实际使用：<code>access.Permissions</code>；<code>access.Document</code> |
| <code>detail</code> | <code>bool</code> | true读取审核请求完整详情，false返回列表摘要；列表不默认加载整份送审正文。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>dto.ReviewRequest</code> | 实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return value, err</code>。当前函数在这里结束。
- 检查 <code>value.ArchivedAt == nil</code>。
- 检查 <code>value.Status == &quot;REVIEWING&quot;</code>。
- 检查 <code>access.Permissions.Edit&#91;pointer.Unit&#93;</code>。
- 检查 <code>access.Permissions.Review</code>。
- 检查 <code>access.Permissions.Review &#124;&#124; access.Permissions.Manage</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return value, err</code>。当前函数在这里结束。
- 检查 <code>domain.CanOfferMerge(access.Permissions, facts)</code>。
- 检查 <code>value.Status == &quot;WITHDRAWN&quot; &amp;&amp; access.Permissions.Edit&#91;pointer.Unit&#93;</code>。
- 检查 <code>detail</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return value, err</code>。当前函数在这里结束。

<!-- write-step: 19 bytes=15512:17059 -->
````go
func (s *Reviews) requestView(ctx context.Context, tx *sql.Tx, pointer domain.ReviewPointer, actor uint64, access DocumentAccess, detail bool) (dto.ReviewRequest, error) {
	value, err := repository.ReviewRequestRow(ctx, tx, pointer.ID)
	if err != nil {
		return value, err
	}
	round, err := domain.CanonicalID(value.CurrentRoundId)
	if err != nil {
		return value, err
	}
	summary, body, err := repository.ReviewRequestSummaryRow(ctx, tx, pointer.ID, round, pointer.Document)
	if err != nil {
		return value, err
	}
	summary.Excerpt, err = s.reviewSummaryExcerpt(body)
	if err != nil {
		return value, err
	}
	value.Summary = &summary
	if value.ArchivedAt == nil {
		if value.Status == "REVIEWING" {
			if access.Permissions.Edit[pointer.Unit] {
				value.Actions = append(value.Actions, "WITHDRAW")
			}
			if access.Permissions.Review {
				value.Actions = append(value.Actions, "REQUEST_CHANGES")
			}
			if access.Permissions.Review || access.Permissions.Manage {
				facts, err := repository.MergeApprovalFacts(ctx, tx, pointer.ID, actor)
				if err != nil {
					return value, err
				}
				if domain.CanOfferMerge(access.Permissions, facts) {
					value.Actions = append(value.Actions, "APPROVE_AND_MERGE")
				}
			}
		} else if value.Status == "WITHDRAWN" && access.Permissions.Edit[pointer.Unit] {
			value.Actions = append(value.Actions, "RESUBMIT")
		}
	}
	if detail {
		fixed, err := repository.ReviewRoundRow(ctx, tx, pointer.ID, round, access.Document.Id)
		if err != nil {
			return value, err
		}
		value.CurrentRound = &fixed
	}
````

## 第 20 步：Reviews.requestView（接着实现）

插入位置：第 19 步代码之后，紧接追加；完成稿约第 495 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return value, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 20 bytes=17059:17081 -->
````go
	return value, nil
}

````

## 第 21 步：Reviews.reviewSummaryExcerpt

插入位置：第 20 步代码之后，紧接追加；完成稿约第 498 行。

本步声明：`func (s *Reviews) reviewSummaryExcerpt(raw []byte) (string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>codec.Decode(raw)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;&quot;</code>；<code>string(characters&#91;:140&#93;) + &quot;…&quot;</code>；<code>string(characters)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;&quot;, err</code>。当前函数在这里结束。
- 检查 <code>len(characters) &gt; 140</code>。条件成立时执行 <code>return string(characters&#91;:140&#93;) + &quot;…&quot;, nil</code>。当前函数在这里结束。
- 此处通过 <code>return string(characters), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 21 bytes=17081:17643 -->
````go
func (s *Reviews) reviewSummaryExcerpt(raw []byte) (string, error) {
	body, err := codec.Decode(raw)
	if err != nil {
		return "", err
	}
	if err = s.validator.Body(body); err != nil {
		return "", err
	}
	text, err := searchSnippet(body)
	if err != nil {
		return "", err
	}
	// 复用正文语义投影，再压平段落空白；按Unicode码点截断，避免切坏中文和emoji。
	characters := []rune(strings.Join(strings.Fields(text), " "))
	if len(characters) > 140 {
		return string(characters[:140]) + "…", nil
	}
	return string(characters), nil
}

````

## 第 22 步：Reviews.Request

插入位置：第 21 步代码之后，紧接追加；完成稿约第 518 行。

本步声明：`func (s *Reviews) Request(ctx context.Context, actor Actor, pointer domain.ReviewPointer) ([]*pb.SourceEpoch, ReadBody, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.authorized(ctx, actor, pointer, &quot;READ_REVIEW&quot;)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.authorized(ctx, actor, pointer, &quot;READ_REVIEW&quot;)</code>；<code>actor.ID</code> |
| <code>pointer</code> | <code>domain.ReviewPointer</code> | 使用 <code>domain.ReviewPointer</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>s.authorized(ctx, actor, pointer, &quot;READ_REVIEW&quot;)</code>；<code>s.validateReviewReadAccess(ctx, tx, access, pointer)</code>；<code>s.requestView(ctx, tx, pointer, actor.ID, access, true)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 实际返回表达式：<code>nil</code>；<code>access.Sources</code> |
| <code>(匿名/未命名)</code> | <code>ReadBody</code> | 使用 <code>ReadBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return s.requestView(ctx, tx, pointer, actor.ID, access, true)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return access.Sources, run, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 22 bytes=17643:18160 -->
````go
func (s *Reviews) Request(ctx context.Context, actor Actor, pointer domain.ReviewPointer) ([]*pb.SourceEpoch, ReadBody, error) {
	_, access, err := s.authorized(ctx, actor, pointer, "READ_REVIEW")
	if err != nil {
		return nil, nil, err
	}
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		if err := s.validateReviewReadAccess(ctx, tx, access, pointer); err != nil {
			return nil, err
		}
		return s.requestView(ctx, tx, pointer, actor.ID, access, true)
	}
	return access.Sources, run, nil
}

````

## 第 23 步：Reviews.Round

插入位置：第 22 步代码之后，紧接追加；完成稿约第 532 行。

本步声明：`func (s *Reviews) Round(ctx context.Context, actor Actor, pointer domain.ReviewPointer, round uint64) ([]*pb.SourceEpoch, ReadBody, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.authorized(ctx, actor, pointer, &quot;READ_REVIEW_ROUND&quot;)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.authorized(ctx, actor, pointer, &quot;READ_REVIEW_ROUND&quot;)</code> |
| <code>pointer</code> | <code>domain.ReviewPointer</code> | 使用 <code>domain.ReviewPointer</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>s.authorized(ctx, actor, pointer, &quot;READ_REVIEW_ROUND&quot;)</code>；<code>s.validateReviewReadAccess(ctx, tx, access, pointer)</code>；<code>pointer.ID</code> |
| <code>round</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.ReviewRoundRow(ctx, tx, pointer.ID, round, access.Document.Id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 实际返回表达式：<code>nil</code>；<code>access.Sources</code> |
| <code>(匿名/未命名)</code> | <code>ReadBody</code> | 使用 <code>ReadBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err == sql.ErrNoRows</code>。条件成立时执行 <code>return nil, domain.Violation(domain.EntityNotFound, &quot;REVIEW_ROUND_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return value, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return access.Sources, run, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 23 bytes=18160:18853 -->
````go
func (s *Reviews) Round(ctx context.Context, actor Actor, pointer domain.ReviewPointer, round uint64) ([]*pb.SourceEpoch, ReadBody, error) {
	_, access, err := s.authorized(ctx, actor, pointer, "READ_REVIEW_ROUND")
	if err != nil {
		return nil, nil, err
	}
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		if err := s.validateReviewReadAccess(ctx, tx, access, pointer); err != nil {
			return nil, err
		}
		value, err := repository.ReviewRoundRow(ctx, tx, pointer.ID, round, access.Document.Id)
		if err == sql.ErrNoRows {
			return nil, domain.Violation(domain.EntityNotFound, "REVIEW_ROUND_NOT_FOUND")
		}
		return value, err
	}
	return access.Sources, run, nil
}

````

## 第 24 步：reviewPageBinding

插入位置：第 23 步代码之后，紧接追加；完成稿约第 550 行。

本步声明：`func reviewPageBinding(actor Actor, access DocumentAccess, pointer domain.ReviewPointer, kind string) string`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor.ID</code> |
| <code>access</code> | <code>DocumentAccess</code> | 使用 <code>DocumentAccess</code> 类型；其定义处的职责：把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。<br>在此声明中的实际使用：<code>access.Sources</code> |
| <code>pointer</code> | <code>domain.ReviewPointer</code> | 使用 <code>domain.ReviewPointer</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>pointer.ID</code>；<code>pointer.Revision</code> |
| <code>kind</code> | <code>string</code> | 在此声明中的实际使用：<code>&quot;review:&quot; + kind</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;review:&quot; + kind + &quot;:&quot; + domain.TextID(actor.ID) + &quot;:&quot; + domain.TextID(pointer.ID) + &quot;:&quot; + domain.TextID(pointer.Revision) + &quot;:&quot; + string(data)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &quot;review:&quot; + kind + &quot;:&quot; + domain.TextID(actor.ID) + &quot;:&quot; + domain.TextID(pointer.ID) + &quot;:&quot; + domain.TextID(pointer.Revision) + &quot;:&quot; + string(data)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 24 bytes=18853:19276 -->
````go
func reviewPageBinding(actor Actor, access DocumentAccess, pointer domain.ReviewPointer, kind string) string {
	// 游标固定当前权限来源及控制修订；跨用户、跨请求或中途新动作都会使旧页失效。
	data, _ := json.Marshal(access.Sources)
	return "review:" + kind + ":" + domain.TextID(actor.ID) + ":" + domain.TextID(pointer.ID) + ":" + domain.TextID(pointer.Revision) + ":" + string(data)
}

````

## 第 25 步：Reviews.History

插入位置：第 24 步代码之后，紧接追加；完成稿约第 556 行。

本步声明：`func (s *Reviews) History(ctx context.Context, actor Actor, pointer domain.ReviewPointer, kind, limitText, cursorText string) ([]*pb.SourceEpoch, ReadBody, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.authorized(ctx, actor, pointer, &quot;READ_REVIEW_HISTORY&quot;)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.authorized(ctx, actor, pointer, &quot;READ_REVIEW_HISTORY&quot;)</code>；<code>reviewPageBinding(actor, access, pointer, kind)</code> |
| <code>pointer</code> | <code>domain.ReviewPointer</code> | 使用 <code>domain.ReviewPointer</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>s.authorized(ctx, actor, pointer, &quot;READ_REVIEW_HISTORY&quot;)</code>；<code>reviewPageBinding(actor, access, pointer, kind)</code>；<code>s.validateReviewReadAccess(ctx, tx, access, pointer)</code>；<code>pointer.ID</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>kind</code> | <code>string</code> | 在此声明中的实际使用：<code>reviewPageBinding(actor, access, pointer, kind)</code>；<code>run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) { 		if err := s.validateReviewReadAccess(ctx, tx, access, pointer); err != nil { 			return nil, err 		} 		curren …</code>（长表达式，按本步完整代码继续） |
| <code>limitText</code> | <code>string</code> | 在此声明中的实际使用：<code>s.pagination.Parse(limitText, cursorText, binding)</code> |
| <code>cursorText</code> | <code>string</code> | 在此声明中的实际使用：<code>s.pagination.Parse(limitText, cursorText, binding)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 实际返回表达式：<code>nil</code>；<code>access.Sources</code> |
| <code>(匿名/未命名)</code> | <code>ReadBody</code> | 使用 <code>ReadBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>last == &quot;&quot;</code>。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>current.Revision != pointer.Revision</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;REVIEW_PAGE_CHANGED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>len(page.Items) == limit</code>。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。

<!-- write-step: 25 bytes=19276:20853 -->
````go
func (s *Reviews) History(ctx context.Context, actor Actor, pointer domain.ReviewPointer, kind, limitText, cursorText string) ([]*pb.SourceEpoch, ReadBody, error) {
	_, access, err := s.authorized(ctx, actor, pointer, "READ_REVIEW_HISTORY")
	if err != nil {
		return nil, nil, err
	}
	binding := reviewPageBinding(actor, access, pointer, kind)
	limit, last, err := s.pagination.Parse(limitText, cursorText, binding)
	if err != nil {
		return nil, nil, err
	}
	if last == "" {
		last = "0"
	}
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		if err := s.validateReviewReadAccess(ctx, tx, access, pointer); err != nil {
			return nil, err
		}
		current, err := repository.LockedReviewPointer(ctx, tx, pointer.ID)
		if err != nil {
			return nil, err
		}
		if current.Revision != pointer.Revision {
			return nil, domain.Violation(domain.StateConflict, "REVIEW_PAGE_CHANGED")
		}
		switch kind {
		case "rounds":
			ids, err := repository.ReviewRoundIDs(ctx, tx, pointer.ID, last, limit+1)
			if err != nil {
				return nil, err
			}
			page := dto.ReviewRoundPage{Items: []dto.ReviewRound{}}
			bytes := 0
			for _, id := range ids {
				if len(page.Items) == limit {
					page.HasMore = true
					break
				}
				fixed, err := repository.ReviewRoundRow(ctx, tx, pointer.ID, id, access.Document.Id)
				if err != nil {
					return nil, err
				}
				encoded, err := json.Marshal(fixed)
				if err != nil {
					return nil, err
				}
				// 正文上限按单轮规定；历史页还需总字节预算，避免合法的多份大轮次撑爆输出内存。
````

## 第 26 步：Reviews.History（接着实现）

插入位置：第 25 步代码之后，紧接追加；完成稿约第 602 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：检查 <code>len(page.Items) &gt; 0 &amp;&amp; bytes+len(encoded) &gt; 24&lt;&lt;20</code>。
- 在本段的匿名函数内：检查 <code>page.HasMore</code>。
- 在本段的匿名函数内：此处通过 <code>return page, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>page.HasMore</code>。
- 在本段的匿名函数内：此处通过 <code>return page, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>page.HasMore</code>。
- 在本段的匿名函数内：此处通过 <code>return page, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：此处通过 <code>return nil, domain.Violation(domain.InvalidRequest, &quot;REVIEW_HISTORY_KIND_INVALID&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return access.Sources, run, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 26 bytes=20853:22094 -->
````go
				if len(page.Items) > 0 && bytes+len(encoded) > 24<<20 {
					page.HasMore = true
					break
				}
				page.Items = append(page.Items, fixed)
				bytes += len(encoded)
			}
			if page.HasMore {
				page.NextCursor = s.pagination.Next(binding, page.Items[len(page.Items)-1].Id)
			}
			return page, nil
		case "changes":
			items, err := repository.ReviewChanges(ctx, tx, pointer.ID, last, limit+1)
			if err != nil {
				return nil, err
			}
			page := dto.ChangeRequestPage{Items: items, HasMore: len(items) > limit}
			if page.HasMore {
				page.Items = items[:limit]
				page.NextCursor = s.pagination.Next(binding, page.Items[limit-1].Id)
			}
			return page, nil
		case "actions":
			items, err := repository.ReviewActions(ctx, tx, pointer.ID, last, limit+1)
			if err != nil {
				return nil, err
			}
			page := dto.ReviewActionPage{Items: items, HasMore: len(items) > limit}
			if page.HasMore {
				page.Items = items[:limit]
				page.NextCursor = s.pagination.Next(binding, page.Items[limit-1].Id)
			}
			return page, nil
		default:
			return nil, domain.Violation(domain.InvalidRequest, "REVIEW_HISTORY_KIND_INVALID")
		}
	}
	return access.Sources, run, nil
}

// review_scope.go：保留原实现，按业务合并文件。

````

## 第 27 步：Reviews.organizationReviewScope

插入位置：第 26 步代码之后，紧接追加；完成稿约第 644 行。

组织范围只从账号服务当前目录取得；不会把其他用户的组织当作逐篇身份RPC探测目标。
这是SQL候选优化，不授予正文访问；每个候选仍执行Authorize与输出事务内ValidateAccess。

本步声明：`func (s *Reviews) organizationReviewScope(ctx context.Context, actor Actor) ([]uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.directory.Read(ctx, cookie, &quot;/v1/organizations?limit=100&amp;cursor=&quot;+url.QueryEscape(cursor), &amp;page)</code>；<code>s.access.identity.Acquire(ctx, actor, pb.IdentityCapability_ORGANIZATION_ADMIN, &amp;pb.IdentityTarget{Kind: pb.IdentityTarget_ORGANIZATION, OrganizationId: id, UserId: actor.ID}, &quot;LIST_REVIEWS_SCOPE&quot;, 0, 0)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor.Token</code>；<code>actor.ID</code>；<code>s.access.identity.Acquire(ctx, actor, pb.IdentityCapability_ORGANIZATION_ADMIN, &amp;pb.IdentityTarget{Kind: pb.IdentityTarget_ORGANIZATION, OrganizationId: id, UserId: actor.ID}, &quot;LIST_REVIEWS_SCOPE&quot;, 0, 0)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;uint64</code> | 实际返回表达式：<code>nil</code>；<code>organizations</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;REVIEW_DIRECTORY_UNAVAILABLE&quot;)</code>；<code>err</code>；<code>nil</code>；<code>errors.New(&quot;REVIEW_DIRECTORY_CURSOR_INVALID&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>s.directory == nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;REVIEW_DIRECTORY_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>organization.OwnerId != domain.TextID(actor.ID)</code>。
- 检查 <code>err != nil</code>。
- 检查 <code>errors.As(err, &amp;rule) &amp;&amp; rule.Kind == domain.PermissionDenied</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 此处通过 <code>return nil, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!page.HasMore</code>。条件成立时执行 <code>return organizations, nil</code>。当前函数在这里结束。
- 检查 <code>page.NextCursor == nil &#124;&#124; *page.NextCursor == &quot;&quot; &#124;&#124; *page.NextCursor == cursor</code>。条件成立时执行 <code>return nil, errors.New(&quot;REVIEW_DIRECTORY_CURSOR_INVALID&quot;)</code>。当前函数在这里结束。

<!-- write-step: 27 bytes=22094:23770 -->
````go
// 组织范围只从账号服务当前目录取得；不会把其他用户的组织当作逐篇身份RPC探测目标。
// 这是SQL候选优化，不授予正文访问；每个候选仍执行Authorize与输出事务内ValidateAccess。
func (s *Reviews) organizationReviewScope(ctx context.Context, actor Actor) ([]uint64, error) {
	if s.directory == nil {
		return nil, errors.New("REVIEW_DIRECTORY_UNAVAILABLE")
	}
	cookie := "__Host-collabSession=" + base64.RawURLEncoding.EncodeToString(actor.Token)
	var organizations []uint64
	cursor := ""
	for {
		var page dto.OrganizationPage
		if err := s.directory.Read(ctx, cookie, "/v1/organizations?limit=100&cursor="+url.QueryEscape(cursor), &page); err != nil {
			return nil, err
		}
		for _, organization := range page.Items {
			id, err := domain.CanonicalID(organization.Id)
			if err != nil {
				return nil, err
			}
			if organization.OwnerId != domain.TextID(actor.ID) {
				_, err = s.access.identity.Acquire(ctx, actor, pb.IdentityCapability_ORGANIZATION_ADMIN, &pb.IdentityTarget{Kind: pb.IdentityTarget_ORGANIZATION, OrganizationId: id, UserId: actor.ID}, "LIST_REVIEWS_SCOPE", 0, 0)
				if err != nil {
					var rule domain.RuleViolation
					if errors.As(err, &rule) && rule.Kind == domain.PermissionDenied {
						continue
					}
					return nil, err
				}
			}
			organizations = append(organizations, id)
		}
		if !page.HasMore {
			return organizations, nil
		}
		if page.NextCursor == nil || *page.NextCursor == "" || *page.NextCursor == cursor {
			return nil, errors.New("REVIEW_DIRECTORY_CURSOR_INVALID")
		}
		cursor = *page.NextCursor
	}
}

// reviews.go：保留原实现，按业务合并文件。

````

## 第 28 步：Reviews

插入位置：第 27 步代码之后，紧接追加；完成稿约第 687 行。

本步声明：`type Reviews struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>access</code> | <code>*Access</code> | 使用 <code>Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。 |
| <code>pagination</code> | <code>*Pagination</code> | 使用 <code>Pagination</code> 类型；其定义处的职责：生成并校验绑定查询上下文的签名翻页游标。 |
| <code>directory</code> | <code>DirectoryReader</code> | 使用 <code>DirectoryReader</code> 类型；其定义处的职责：空间目录查询通过Account接口完成，Doc不跨库直接枚举组织成员。 |
| <code>validator</code> | <code>*codec.Validator</code> | 冻结协议的结构校验器，先限制请求形状，再由业务核验身份、权限、版本和事务条件。 |

<!-- write-step: 28 bytes=23770:23916 -->
````go
type Reviews struct {
	db         *sql.DB
	access     *Access
	pagination *Pagination
	directory  DirectoryReader
	validator  *codec.Validator
}

````

## 第 29 步：NewReviews

插入位置：第 28 步代码之后，紧接追加；完成稿约第 695 行。

本步声明：`func NewReviews(db *sql.DB, access *Access, pagination *Pagination, directory DirectoryReader, validator *codec.Validator) *Reviews`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |
| <code>access</code> | <code>*Access</code> | 使用 <code>Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。<br>在此声明中的实际使用：<code>access: access</code> |
| <code>pagination</code> | <code>*Pagination</code> | 使用 <code>Pagination</code> 类型；其定义处的职责：生成并校验绑定查询上下文的签名翻页游标。<br>在此声明中的实际使用：<code>pagination: pagination</code> |
| <code>directory</code> | <code>DirectoryReader</code> | 使用 <code>DirectoryReader</code> 类型；其定义处的职责：空间目录查询通过Account接口完成，Doc不跨库直接枚举组织成员。<br>在此声明中的实际使用：<code>directory: directory</code> |
| <code>validator</code> | <code>*codec.Validator</code> | 冻结协议的结构校验器，先限制请求形状，再由业务核验身份、权限、版本和事务条件。<br>在此声明中的实际使用：<code>validator: validator</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Reviews</code> | 实际返回表达式：<code>&amp;Reviews{db: db, access: access, pagination: pagination, directory: directory, validator: validator}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Reviews{db: db, access: access, pagination: pagination, directory: directory, validator: validator}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 29 bytes=23916:24162 -->
````go
func NewReviews(db *sql.DB, access *Access, pagination *Pagination, directory DirectoryReader, validator *codec.Validator) *Reviews {
	return &Reviews{db: db, access: access, pagination: pagination, directory: directory, validator: validator}
}

````

## 第 30 步：Reviews.Lookup

插入位置：第 29 步代码之后，紧接追加；完成稿约第 699 行。

本步声明：`func (s *Reviews) Lookup(ctx context.Context, id uint64) (domain.ReviewPointer, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.ReviewPointerRow(ctx, s.db, id)</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.ReviewPointerRow(ctx, s.db, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.ReviewPointer</code> | 使用 <code>domain.ReviewPointer</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>实际返回表达式：<code>pointer</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.EntityNotFound, &quot;REVIEW_REQUEST_NOT_FOUND&quot;)</code>；<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>err == sql.ErrNoRows</code>。条件成立时执行 <code>return pointer, domain.Violation(domain.EntityNotFound, &quot;REVIEW_REQUEST_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return pointer, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 30 bytes=24162:24451 -->
````go
func (s *Reviews) Lookup(ctx context.Context, id uint64) (domain.ReviewPointer, error) {
	pointer, err := repository.ReviewPointerRow(ctx, s.db, id)
	if err == sql.ErrNoRows {
		return pointer, domain.Violation(domain.EntityNotFound, "REVIEW_REQUEST_NOT_FOUND")
	}
	return pointer, err
}

````

## 第 31 步：Reviews.Ready

插入位置：第 30 步代码之后，紧接追加；完成稿约第 707 行。

本步声明：`func (s *Reviews) Ready(ctx context.Context) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.UnboundReviewActionNotifications(ctx, s.db)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;REVIEW_NOTIFICATION_RECOVERY_UNAVAILABLE&quot;)</code>；<code>errors.New(&quot;REVIEW_NOTIFICATION_BINDING_UNKNOWN&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;REVIEW_NOTIFICATION_RECOVERY_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>invalid</code>。条件成立时执行 <code>return errors.New(&quot;REVIEW_NOTIFICATION_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 31 bytes=24451:24751 -->
````go
func (s *Reviews) Ready(ctx context.Context) error {
	invalid, err := repository.UnboundReviewActionNotifications(ctx, s.db)
	if err != nil {
		return errors.New("REVIEW_NOTIFICATION_RECOVERY_UNAVAILABLE")
	}
	if invalid {
		return errors.New("REVIEW_NOTIFICATION_BINDING_UNKNOWN")
	}
	return nil
}

````

## 第 32 步：reviewContextAllowed

插入位置：第 31 步代码之后，紧接追加；完成稿约第 718 行。

本步声明：`func reviewContextAllowed(access DocumentAccess, pointer domain.ReviewPointer) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>access</code> | <code>DocumentAccess</code> | 使用 <code>DocumentAccess</code> 类型；其定义处的职责：把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。<br>在此声明中的实际使用：<code>access.Permissions</code> |
| <code>pointer</code> | <code>domain.ReviewPointer</code> | 使用 <code>domain.ReviewPointer</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>pointer.Unit</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>access.Permissions.Comment &#124;&#124; access.Permissions.Review &#124;&#124; access.Permissions.Manage &#124;&#124; access.Permissions.Edit&#91;pointer.Unit&#93;</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return access.Permissions.Comment &#124;&#124; access.Permissions.Review &#124;&#124; access.Permissions.Manage &#124;&#124; access.Permissions.Edit&#91;pointer.Unit&#93;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 32 bytes=24751:24974 -->
````go
func reviewContextAllowed(access DocumentAccess, pointer domain.ReviewPointer) bool {
	return access.Permissions.Comment || access.Permissions.Review || access.Permissions.Manage || access.Permissions.Edit[pointer.Unit]
}

````

## 第 33 步：Reviews.authorized

插入位置：第 32 步代码之后，紧接追加；完成稿约第 722 行。

本步声明：`func (s *Reviews) authorized(ctx context.Context, actor Actor, pointer domain.ReviewPointer, purpose string) (Actor, DocumentAccess, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, pointer.Document, purpose)</code>；<code>s.reviewReadAllowed(ctx, access, pointer)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, pointer.Document, purpose)</code>；<code>return actor, access, err</code>；<code>return actor, access, domain.Violation(domain.PermissionDenied, &quot;REVIEW_CONTEXT_NOT_ALLOWED&quot;)</code>；<code>actor.Sources</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>pointer</code> | <code>domain.ReviewPointer</code> | 使用 <code>domain.ReviewPointer</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>pointer.Document</code>；<code>reviewContextAllowed(access, pointer)</code>；<code>s.reviewReadAllowed(ctx, access, pointer)</code> |
| <code>purpose</code> | <code>string</code> | 在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, pointer.Document, purpose)</code>；<code>historicalReviewPurpose(purpose)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>实际返回表达式：<code>actor</code> |
| <code>(匿名/未命名)</code> | <code>DocumentAccess</code> | 使用 <code>DocumentAccess</code> 类型；其定义处的职责：把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。<br>实际返回表达式：<code>access</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.PermissionDenied, &quot;REVIEW_CONTEXT_NOT_ALLOWED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, access, err</code>。当前函数在这里结束。
- 检查 <code>!allowed &amp;&amp; historicalReviewPurpose(purpose)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, access, err</code>。当前函数在这里结束。
- 检查 <code>!allowed</code>。条件成立时执行 <code>return actor, access, domain.Violation(domain.PermissionDenied, &quot;REVIEW_CONTEXT_NOT_ALLOWED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return actor, access, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 33 bytes=24974:25627 -->
````go
func (s *Reviews) authorized(ctx context.Context, actor Actor, pointer domain.ReviewPointer, purpose string) (Actor, DocumentAccess, error) {
	access, err := s.access.Authorize(ctx, actor, pointer.Document, purpose)
	if err != nil {
		return actor, access, err
	}
	allowed := reviewContextAllowed(access, pointer)
	if !allowed && historicalReviewPurpose(purpose) {
		allowed, err = s.reviewReadAllowed(ctx, access, pointer)
		if err != nil {
			return actor, access, err
		}
	}
	if !allowed {
		return actor, access, domain.Violation(domain.PermissionDenied, "REVIEW_CONTEXT_NOT_ALLOWED")
	}
	actor.Sources = access.Sources
	return actor, access, nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/logic/document/reviews_service.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/05-submission-review.md)。
