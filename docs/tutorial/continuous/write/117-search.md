# 分步编写 `services/document/internal/client/search.go`

先按 [D14 · 搜索资格、快照重建与文档服务总装](../lessons/document/14-search-assembly.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/117-search.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

适配冻结搜索候选RPC并严格校验批锚点和排序响应。

候选完整不等于有Doc读取权限，客户端应拒绝损坏协议而非猜测补齐。

写之前先按这条执行路线理解各段的连接：

1. SearchCandidates核query/scope再调用peer
2. validSearchReply核数量、期限和绑定
3. validSearchSort核排序键
4. CloseCursor确认关闭
5. searchRPCError转安全码。

本文件需要守住的边界：

- 连接生命周期由bootstrap管理
- 不能解析或伪造Search opaque锚点内部MAC。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/client' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/client/search.go') -and (Get-Item -LiteralPath 'services/document/internal/client/search.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/client/search.go')) { New-Item -ItemType File -Path 'services/document/internal/client/search.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:425 -->
````go
package client

import (
	"bytes"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

````

## 第 2 步：searchRPCDeadline、searchRPCMaxBytes、searchRPCMaxFrame、searchRPCMaxSort、searchRPCBatch

插入位置：第 1 步代码之后，紧接追加；完成稿约第 26 行。

本步声明：`const (`。

<!-- write-step: 2 bytes=425:578 -->
````go
const (
	searchRPCDeadline = 5 * time.Second
	searchRPCMaxBytes = 64 << 10
	searchRPCMaxFrame = 768
	searchRPCMaxSort  = 128
	searchRPCBatch    = 100
)

````

## 第 3 步：searchPhysicalName

插入位置：第 2 步代码之后，紧接追加；完成稿约第 34 行。

本步声明：`var searchPhysicalName = regexp.MustCompile(\`^[a-z0-9][a-z0-9._-]{0,159}$\`)`。

<!-- write-step: 3 bytes=578:655 -->
````go
var searchPhysicalName = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,159}$`)

````

## 第 4 步：Search

插入位置：第 3 步代码之后，紧接追加；完成稿约第 36 行。

Search不拥有连接，svc用Doc的真实mTLS身份建立并关闭peer；此层仅适配冻结RPC。
候选完整也不代表Doc读取权限，正式版及当前来源资格由整页用例再核验。

本步声明：`type Search struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>peer</code> | <code>pb.SearchInternalClient</code> | 内部Search gRPC客户端，返回候选及搜索游标，最终读取授权仍由Doc完成。 |

<!-- write-step: 4 bytes=655:917 -->
````go
// Search不拥有连接，svc用Doc的真实mTLS身份建立并关闭peer；此层仅适配冻结RPC。
// 候选完整也不代表Doc读取权限，正式版及当前来源资格由整页用例再核验。
type Search struct {
	peer pb.SearchInternalClient
}

````

## 第 5 步：NewSearch

插入位置：第 4 步代码之后，紧接追加；完成稿约第 42 行。

本步声明：`func NewSearch(peer pb.SearchInternalClient) *Search`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>peer</code> | <code>pb.SearchInternalClient</code> | 在此声明中的实际使用：<code>peer: peer</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Search</code> | 使用 <code>Search</code> 类型；其定义处的职责：Search不拥有连接，svc用Doc的真实mTLS身份建立并关闭peer；此层仅适配冻结RPC。 候选完整也不代表Doc读取权限，正式版及当前来源资格由整页用例再核验。<br>实际返回表达式：<code>&amp;Search{peer: peer}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Search{peer: peer}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=917:1002 -->
````go
func NewSearch(peer pb.SearchInternalClient) *Search { return &Search{peer: peer} }

````

## 第 6 步：searchIncomplete

插入位置：第 5 步代码之后，紧接追加；完成稿约第 44 行。

本步声明：`func searchIncomplete() error`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.TemporarilyUnavailable, &quot;SEARCH_INCOMPLETE&quot;)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;SEARCH_INCOMPLETE&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1002:1114 -->
````go
func searchIncomplete() error {
	return domain.Violation(domain.TemporarilyUnavailable, "SEARCH_INCOMPLETE")
}

````

## 第 7 步：searchBindingInvalid

插入位置：第 6 步代码之后，紧接追加；完成稿约第 48 行。

本步声明：`func searchBindingInvalid() error`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.InvalidRequest, &quot;SEARCH_BINDING_INVALID&quot;)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return domain.Violation(domain.InvalidRequest, &quot;SEARCH_BINDING_INVALID&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=1114:1227 -->
````go
func searchBindingInvalid() error {
	return domain.Violation(domain.InvalidRequest, "SEARCH_BINDING_INVALID")
}

````

## 第 8 步：Search.SearchCandidates

插入位置：第 7 步代码之后，紧接追加；完成稿约第 52 行。

本步声明：`func (s *Search) SearchCandidates(ctx context.Context, input *pb.SearchCandidatesRequest) (*pb.SearchCandidatesReply, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx, cancel := context.WithTimeout(ctx, searchRPCDeadline)</code>；<code>context.WithTimeout(ctx, searchRPCDeadline)</code>；<code>ctx.Err</code>；<code>s.peer.SearchCandidates(ctx, request, grpc.MaxCallRecvMsgSize(searchRPCMaxBytes))</code> |
| <code>input</code> | <code>*pb.SearchCandidatesRequest</code> | 在此声明中的实际使用：<code>validSearchRequest(input)</code>；<code>proto.Clone(input)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.SearchCandidatesReply</code> | 实际返回表达式：<code>nil</code>；<code>proto.Clone(reply).(*pb.SearchCandidatesReply)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>searchIncomplete()</code>；<code>searchBindingInvalid()</code>；<code>searchRPCError(err)</code>；<code>domain.Violation(domain.StateConflict, &quot;CURSOR_EXPIRED&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>s == nil &#124;&#124; s.peer == nil</code>。条件成立时执行 <code>return nil, searchIncomplete()</code>。当前函数在这里结束。
- 检查 <code>!validSearchRequest(input)</code>。条件成立时执行 <code>return nil, searchBindingInvalid()</code>。当前函数在这里结束。
- 执行 <code>context.WithTimeout(ctx, searchRPCDeadline)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>ctx.Err() != nil</code>。条件成立时执行 <code>return nil, searchIncomplete()</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, searchRPCError(err)</code>。当前函数在这里结束。
- 检查 <code>ctx.Err() != nil &#124;&#124; !validSearchReply(reply, received)</code>。条件成立时执行 <code>return nil, searchIncomplete()</code>。当前函数在这里结束。
- 检查 <code>reply.ExpiresUnixMs &lt;= received.UnixMilli()</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;CURSOR_EXPIRED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return proto.Clone(reply).(*pb.SearchCandidatesReply), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=1227:2357 -->
````go
func (s *Search) SearchCandidates(ctx context.Context, input *pb.SearchCandidatesRequest) (*pb.SearchCandidatesReply, error) {
	if s == nil || s.peer == nil {
		return nil, searchIncomplete()
	}
	if !validSearchRequest(input) {
		return nil, searchBindingInvalid()
	}
	// WithTimeout保留原请求取消/值及更早的HTTP期限，不创建脱离原请求的重试。
	ctx, cancel := context.WithTimeout(ctx, searchRPCDeadline)
	defer cancel()
	if ctx.Err() != nil {
		return nil, searchIncomplete()
	}
	request := proto.Clone(input).(*pb.SearchCandidatesRequest)
	reply, err := s.peer.SearchCandidates(ctx, request, grpc.MaxCallRecvMsgSize(searchRPCMaxBytes))
	if err != nil {
		return nil, searchRPCError(err)
	}
	received := time.Now()
	if ctx.Err() != nil || !validSearchReply(reply, received) {
		return nil, searchIncomplete()
	}
	if reply.ExpiresUnixMs <= received.UnixMilli() {
		return nil, domain.Violation(domain.StateConflict, "CURSOR_EXPIRED")
	}
	// 不向用例暴露peer持有的切片，防止验证后变化或分页前缀改写影响原响应。
	return proto.Clone(reply).(*pb.SearchCandidatesReply), nil
}

````

## 第 9 步：Search.CloseCursor

插入位置：第 8 步代码之后，紧接追加；完成稿约第 81 行。

本步声明：`func (s *Search) CloseCursor(ctx context.Context, input *pb.CloseCursorRequest) (*pb.OperationResult, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx, cancel := context.WithTimeout(ctx, searchRPCDeadline)</code>；<code>context.WithTimeout(ctx, searchRPCDeadline)</code>；<code>ctx.Err</code>；<code>s.peer.CloseCursor(ctx, proto.Clone(input).(*pb.CloseCursorRequest), grpc.MaxCallRecvMsgSize(searchRPCMaxBytes))</code> |
| <code>input</code> | <code>*pb.CloseCursorRequest</code> | 在此声明中的实际使用：<code>input == nil</code>；<code>input.Cursor</code>；<code>input.Scope</code>；<code>proto.Size(input)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.OperationResult</code> | 实际返回表达式：<code>nil</code>；<code>proto.Clone(reply).(*pb.OperationResult)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>searchIncomplete()</code>；<code>searchBindingInvalid()</code>；<code>searchRPCError(err)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>s == nil &#124;&#124; s.peer == nil</code>。条件成立时执行 <code>return nil, searchIncomplete()</code>。当前函数在这里结束。
- 检查 <code>input == nil &#124;&#124; len(input.Cursor) == 0 &#124;&#124; len(input.Cursor) &gt; searchRPCMaxFrame &#124;&#124; !validSearchScope(input.Scope) &#124;&#124; proto.Size(input) &gt; searchRPCMaxBytes</code>。条件成立时执行 <code>return nil, searchBindingInvalid()</code>。当前函数在这里结束。
- 执行 <code>context.WithTimeout(ctx, searchRPCDeadline)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>ctx.Err() != nil</code>。条件成立时执行 <code>return nil, searchIncomplete()</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, searchRPCError(err)</code>。当前函数在这里结束。
- 检查 <code>ctx.Err() != nil &#124;&#124; !validSearchClose(reply)</code>。条件成立时执行 <code>return nil, searchIncomplete()</code>。当前函数在这里结束。
- 此处通过 <code>return proto.Clone(reply).(*pb.OperationResult), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=2357:3306 -->
````go
func (s *Search) CloseCursor(ctx context.Context, input *pb.CloseCursorRequest) (*pb.OperationResult, error) {
	if s == nil || s.peer == nil {
		return nil, searchIncomplete()
	}
	// Close使用原SCB锚点，不套SCE；Search权威负责验签、scope及关闭资源的持久状态。
	if input == nil || len(input.Cursor) == 0 || len(input.Cursor) > searchRPCMaxFrame || !validSearchScope(input.Scope) || proto.Size(input) > searchRPCMaxBytes {
		return nil, searchBindingInvalid()
	}
	ctx, cancel := context.WithTimeout(ctx, searchRPCDeadline)
	defer cancel()
	if ctx.Err() != nil {
		return nil, searchIncomplete()
	}
	reply, err := s.peer.CloseCursor(ctx, proto.Clone(input).(*pb.CloseCursorRequest), grpc.MaxCallRecvMsgSize(searchRPCMaxBytes))
	if err != nil {
		return nil, searchRPCError(err)
	}
	if ctx.Err() != nil || !validSearchClose(reply) {
		return nil, searchIncomplete()
	}
	return proto.Clone(reply).(*pb.OperationResult), nil
}

````

## 第 10 步：validSearchRequest

插入位置：第 9 步代码之后，紧接追加；完成稿约第 104 行。

本步声明：`func validSearchRequest(input *pb.SearchCandidatesRequest) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>input</code> | <code>*pb.SearchCandidatesRequest</code> | 在此声明中的实际使用：<code>input == nil</code>；<code>input.Query</code>；<code>input.PageSize</code>；<code>input.Scope</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code>；<code>len(frame) &gt; 7 &amp;&amp; len(frame) &lt;= searchRPCMaxFrame &amp;&amp; bytes.Equal(frame&#91;:4&#93;, &#91;&#93;byte{&#x27;S&#x27;, &#x27;C&#x27;, &#x27;E&#x27;, 1}) &amp;&amp; 		(frame&#91;4&#93; == 1 &#124;&#124; frame&#91;4&#93; == 2) &amp;&amp; int(binary.BigEndian.Uint16(frame&#91;5:7&#93;)) == len(frame)-7</code> |

**跟着本段代码走一遍：**

- 检查 <code>input == nil &#124;&#124; len(input.Query) &gt; 500*utf8.UTFMax &#124;&#124; !utf8.ValidString(input.Query) &#124;&#124; utf8.RuneCountInString(input.Query) &gt; 500 &#124;&#124; strings.TrimSpace(input.Query) == &quot;&quot; &#124;&#124; input.P …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>input.QueryDigest != hex.EncodeToString(digest&#91;:&#93;) &#124;&#124; proto.Size(input) &gt; searchRPCMaxBytes</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>len(input.Cursor) == 0</code>。条件成立时执行 <code>return true</code>。当前函数在这里结束。
- 此处通过 <code>return len(frame) &gt; 7 &amp;&amp; len(frame) &lt;= searchRPCMaxFrame &amp;&amp; bytes.Equal(frame&#91;:4&#93;, &#91;&#93;byte{&#x27;S&#x27;, &#x27;C&#x27;, &#x27;E&#x27;, 1}) &amp;&amp; 		(frame&#91;4&#93; == 1 &#124;&#124; frame&#91;4&#93; == 2) &amp;&amp; int(binary.BigEndian.Uint16(frame&#91;5:7&#93;)) == len(frame)-7</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=3306:4190 -->
````go
func validSearchRequest(input *pb.SearchCandidatesRequest) bool {
	if input == nil || len(input.Query) > 500*utf8.UTFMax || !utf8.ValidString(input.Query) || utf8.RuneCountInString(input.Query) > 500 || strings.TrimSpace(input.Query) == "" || input.PageSize != searchRPCBatch || !validSearchScope(input.Scope) {
		return false
	}
	// exact UTF8摘要沿冻结RPC核对，不trim/NFC改写查询，也不生成或替换调用方真实scope。
	digest := sha256.Sum256([]byte(input.Query))
	if input.QueryDigest != hex.EncodeToString(digest[:]) || proto.Size(input) > searchRPCMaxBytes {
		return false
	}
	if len(input.Cursor) == 0 {
		return true
	}
	frame := input.Cursor
	return len(frame) > 7 && len(frame) <= searchRPCMaxFrame && bytes.Equal(frame[:4], []byte{'S', 'C', 'E', 1}) &&
		(frame[4] == 1 || frame[4] == 2) && int(binary.BigEndian.Uint16(frame[5:7])) == len(frame)-7
}

````

## 第 11 步：validSearchScope

插入位置：第 10 步代码之后，紧接追加；完成稿约第 121 行。

本步声明：`func validSearchScope(scope *pb.SearchScope) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>scope</code> | <code>*pb.SearchScope</code> | 在此声明中的实际使用：<code>scope == nil</code>；<code>scope.ScopeHash</code>；<code>scope.SpaceIds</code>；<code>scope.Anonymous</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |

**跟着本段代码走一遍：**

- 检查 <code>scope == nil &#124;&#124; !searchNonzeroDigest(scope.ScopeHash) &#124;&#124; len(scope.SpaceIds) &gt; 1024</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>scope.Anonymous</code>。
- 检查 <code>len(scope.SessionRef) != 0</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>len(scope.SessionRef) != sha256.Size &#124;&#124; subtle.ConstantTimeCompare(scope.SessionRef, zero&#91;:&#93;) == 1</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>space == 0 &#124;&#124; (i &gt; 0 &amp;&amp; space &lt;= scope.SpaceIds&#91;i-1&#93;)</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 此处通过 <code>return true</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=4190:4734 -->
````go
func validSearchScope(scope *pb.SearchScope) bool {
	if scope == nil || !searchNonzeroDigest(scope.ScopeHash) || len(scope.SpaceIds) > 1024 {
		return false
	}
	if scope.Anonymous {
		if len(scope.SessionRef) != 0 {
			return false
		}
	} else {
		var zero [sha256.Size]byte
		if len(scope.SessionRef) != sha256.Size || subtle.ConstantTimeCompare(scope.SessionRef, zero[:]) == 1 {
			return false
		}
	}
	for i, space := range scope.SpaceIds {
		if space == 0 || (i > 0 && space <= scope.SpaceIds[i-1]) {
			return false
		}
	}
	return true
}

````

## 第 12 步：searchNonzeroDigest

插入位置：第 11 步代码之后，紧接追加；完成稿约第 143 行。

本步声明：`func searchNonzeroDigest(value string) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>string</code> | 在此声明中的实际使用：<code>len(value)</code>；<code>hex.DecodeString(value)</code>；<code>hex.EncodeToString(raw) == value</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>err == nil &amp;&amp; hex.EncodeToString(raw) == value &amp;&amp; subtle.ConstantTimeCompare(raw, zero&#91;:&#93;) != 1</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(value) != sha256.Size*2</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 此处通过 <code>return err == nil &amp;&amp; hex.EncodeToString(raw) == value &amp;&amp; subtle.ConstantTimeCompare(raw, zero&#91;:&#93;) != 1</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=4734:5004 -->
````go
func searchNonzeroDigest(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	raw, err := hex.DecodeString(value)
	var zero [sha256.Size]byte
	return err == nil && hex.EncodeToString(raw) == value && subtle.ConstantTimeCompare(raw, zero[:]) != 1
}

````

## 第 13 步：validSearchReply

插入位置：第 12 步代码之后，紧接追加；完成稿约第 152 行。

本步声明：`func validSearchReply(reply *pb.SearchCandidatesReply, received time.Time) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>reply</code> | <code>*pb.SearchCandidatesReply</code> | 在此声明中的实际使用：<code>reply == nil</code>；<code>reply.Complete</code>；<code>reply.Candidates</code>；<code>reply.HasMore</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |
| <code>received</code> | <code>time.Time</code> | 一个时间点。判断期限时要与当前时间比较，不能把它当成持续时长再任意续期。<br>在此声明中的实际使用：<code>received.Add</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>proto.Size(reply) &lt;= searchRPCMaxBytes</code> |

**跟着本段代码走一遍：**

- 检查 <code>reply == nil &#124;&#124; !reply.Complete &#124;&#124; len(reply.Candidates) &gt; searchRPCBatch &#124;&#124; (reply.HasMore &amp;&amp; len(reply.Candidates) != searchRPCBatch) &#124;&#124; 		reply.IndexGeneration == 0 &#124;&#124; !searchPh …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>candidate == nil &#124;&#124; candidate.DocumentId == 0 &#124;&#124; candidate.ReleaseId == 0 &#124;&#124; candidate.AggregateSeq == 0 &#124;&#124; seen&#91;candidate.DocumentId&#93; &#124;&#124; 			candidate.PhysicalIndex != reply.Physic …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 此处通过 <code>return proto.Size(reply) &lt;= searchRPCMaxBytes</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=5004:6294 -->
````go
func validSearchReply(reply *pb.SearchCandidatesReply, received time.Time) bool {
	// 建链预留发生在RPC发出之后；仅核收到时的最大TTL，不改写或续期原expires。
	if reply == nil || !reply.Complete || len(reply.Candidates) > searchRPCBatch || (reply.HasMore && len(reply.Candidates) != searchRPCBatch) ||
		reply.IndexGeneration == 0 || !searchPhysicalName.MatchString(reply.PhysicalIndex) || reply.ExpiresUnixMs <= 0 || reply.ExpiresUnixMs > received.Add(2*time.Minute).UnixMilli() ||
		len(reply.Continuation) == 0 || len(reply.Continuation) > searchRPCMaxFrame {
		return false
	}
	// EOF仍需保留终批锚点给批内lookahead重放；拒绝后续无法构造有界RPC的opaque字节。
	if _, err := domain.EnvelopeSearchAnchor(reply.Continuation, false); err != nil {
		return false
	}
	seen := make(map[uint64]bool, len(reply.Candidates))
	for _, candidate := range reply.Candidates {
		if candidate == nil || candidate.DocumentId == 0 || candidate.ReleaseId == 0 || candidate.AggregateSeq == 0 || seen[candidate.DocumentId] ||
			candidate.PhysicalIndex != reply.PhysicalIndex || !validSearchSort(candidate.SortJsonUtf8, candidate.DocumentId) {
			return false
		}
		seen[candidate.DocumentId] = true
	}
	return proto.Size(reply) <= searchRPCMaxBytes
}

````

## 第 14 步：validSearchSort

插入位置：第 13 步代码之后，紧接追加；完成稿约第 174 行。

本步声明：`func validSearchSort(raw []byte, document uint64) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>len(raw)</code>；<code>utf8.Valid(raw)</code>；<code>json.Unmarshal(raw, &amp;tuple)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>strconv.FormatUint(document, 10)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>err == nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(raw) == 0 &#124;&#124; len(raw) &gt; searchRPCMaxSort &#124;&#124; !utf8.Valid(raw)</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(raw, &amp;tuple) != nil &#124;&#124; len(tuple) != 3</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>decoder.Decode(&amp;value) != nil</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>!ok</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; math.IsNaN(finite) &#124;&#124; math.IsInf(finite, 0)</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>!bytes.Equal(doc, &#91;&#93;byte(`&quot;`+strconv.FormatUint(document, 10)+`&quot;`))</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>len(shard) == 0 &#124;&#124; len(shard) &gt; 20 &#124;&#124; (shard&#91;0&#93; == &#x27;0&#x27; &amp;&amp; len(shard) &gt; 1)</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>digit &lt; &#x27;0&#x27; &#124;&#124; digit &gt; &#x27;9&#x27;</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 此处通过 <code>return err == nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=6294:7399 -->
````go
func validSearchSort(raw []byte, document uint64) bool {
	if len(raw) == 0 || len(raw) > searchRPCMaxSort || !utf8.Valid(raw) {
		return false
	}
	var tuple []json.RawMessage
	if json.Unmarshal(raw, &tuple) != nil || len(tuple) != 3 {
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(tuple[0]))
	decoder.UseNumber()
	var value any
	if decoder.Decode(&value) != nil {
		return false
	}
	score, ok := value.(json.Number)
	if !ok {
		return false
	}
	finite, err := score.Float64()
	if err != nil || math.IsNaN(finite) || math.IsInf(finite, 0) {
		return false
	}
	// 只把score解析成数值检查有限性；Doc/shard直接核对原十进制，返回sort绝不重写。
	doc := bytes.TrimSpace(tuple[1])
	if !bytes.Equal(doc, []byte(`"`+strconv.FormatUint(document, 10)+`"`)) {
		return false
	}
	shard := bytes.TrimSpace(tuple[2])
	if len(shard) == 0 || len(shard) > 20 || (shard[0] == '0' && len(shard) > 1) {
		return false
	}
	for _, digit := range shard {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	_, err = strconv.ParseUint(string(shard), 10, 64)
	return err == nil
}

````

## 第 15 步：validSearchClose

插入位置：第 14 步代码之后，紧接追加；完成稿约第 214 行。

本步声明：`func validSearchClose(reply *pb.OperationResult) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>reply</code> | <code>*pb.OperationResult</code> | 在此声明中的实际使用：<code>reply == nil</code>；<code>reply.Generation</code>；<code>reply.OperationId</code>；<code>reply.TerminalRevision</code><br>另有 7 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>(reply.State == pb.OperationResult_PROCESSING &amp;&amp; reply.BusinessCode == &quot;CURSOR_CLOSING&quot;) &#124;&#124; 		(reply.State == pb.OperationResult_SUCCEEDED &amp;&amp; reply.BusinessCode == &quot;CURSOR_CLOSED&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>reply == nil &#124;&#124; reply.Generation == 0 &#124;&#124; !searchNonzeroDigest(reply.OperationId) &#124;&#124; reply.TerminalRevision != 0 &#124;&#124; len(reply.ResultIds) != 0 &#124;&#124; 		len(reply.ResultDigest) != 0 &#124;&#124; le …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 此处通过 <code>return (reply.State == pb.OperationResult_PROCESSING &amp;&amp; reply.BusinessCode == &quot;CURSOR_CLOSING&quot;) &#124;&#124; 		(reply.State == pb.OperationResult_SUCCEEDED &amp;&amp; reply.BusinessCode == &quot;CURSOR_CLOSED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=7399:8078 -->
````go
func validSearchClose(reply *pb.OperationResult) bool {
	if reply == nil || reply.Generation == 0 || !searchNonzeroDigest(reply.OperationId) || reply.TerminalRevision != 0 || len(reply.ResultIds) != 0 ||
		len(reply.ResultDigest) != 0 || len(reply.FixedResultJsonUtf8) != 0 || len(reply.CurrentStateJsonUtf8) != 0 || proto.Size(reply) > searchRPCMaxBytes {
		return false
	}
	// PROCESSING是持久关闭尚未完成，不能当作PIT已释放；仅精确状态/固定码组合通过。
	return (reply.State == pb.OperationResult_PROCESSING && reply.BusinessCode == "CURSOR_CLOSING") ||
		(reply.State == pb.OperationResult_SUCCEEDED && reply.BusinessCode == "CURSOR_CLOSED")
}

````

## 第 16 步：searchRPCError

插入位置：第 15 步代码之后，紧接追加；完成稿约第 224 行。

本步声明：`func searchRPCError(err error) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>err</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>在此声明中的实际使用：<code>status.FromError(err)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>searchIncomplete()</code>；<code>domain.Violation(domain.StateConflict, &quot;CURSOR_EXPIRED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;CURSOR_STALE&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;SEARCH_BINDING_INVALID&quot;)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!ok</code>。条件成立时执行 <code>return searchIncomplete()</code>。当前函数在这里结束。
- 此处通过 <code>return domain.Violation(domain.StateConflict, &quot;CURSOR_EXPIRED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return domain.Violation(domain.StateConflict, &quot;CURSOR_STALE&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return domain.Violation(domain.StateConflict, &quot;SEARCH_BINDING_INVALID&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return domain.Violation(domain.InvalidRequest, &quot;CURSOR_INVALID&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return searchBindingInvalid()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=8078:9142 -->
````go
func searchRPCError(err error) error {
	// 同时匹配gRPC类别和封闭业务码，禁止把peer的SQL/ES正文带进HTTP或误判成过期。
	wire, ok := status.FromError(err)
	if !ok {
		return searchIncomplete()
	}
	switch wire.Code() {
	case codes.FailedPrecondition:
		switch wire.Message() {
		case "SEARCH_CURSOR_EXPIRED":
			return domain.Violation(domain.StateConflict, "CURSOR_EXPIRED")
		case "SEARCH_CURSOR_STALE", "SEARCH_CURSOR_CLOSED", "SEARCH_CURSOR_REPLAY_CHANGED":
			return domain.Violation(domain.StateConflict, "CURSOR_STALE")
		case "SEARCH_CURSOR_BINDING_MISMATCH":
			return domain.Violation(domain.StateConflict, "SEARCH_BINDING_INVALID")
		}
	case codes.InvalidArgument:
		switch wire.Message() {
		case "SEARCH_CURSOR_INVALID", "SEARCH_CURSOR_ANCHOR_INVALID", "SEARCH_CURSOR_ENVELOPE_INVALID":
			return domain.Violation(domain.InvalidRequest, "CURSOR_INVALID")
		case "SEARCH_QUERY_BINDING_INVALID", "SEARCH_SCOPE_INVALID", "SEARCH_CANDIDATE_BATCH_SIZE_INVALID":
			return searchBindingInvalid()
		}
	}
	return searchIncomplete()
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/client/search.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/14-search-assembly.md)。
