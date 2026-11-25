# 分步编写 `services/search/internal/server/rpc_server.go`

先按 [S05 · 固定窗口重放、完整核验与别名恢复切换](../lessons/search/05-cutover-wiring.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/399-rpc-server.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

把冻结的Search内部RPC适配到候选查询、游标关闭和重建用例，并限定外发错误码。

传输层只转换请求与错误，不承担索引事务；依赖故障保持Unavailable，不能伪装成确定业务拒绝或泄露SQL/ES响应原文。

写之前先按这条执行路线理解各段的连接：

1. 构造器只注入所需能力，缺失能力时对应RPC返回Unavailable。
2. SearchCandidates/CloseCursor委派候选用例，StartRebuild/GetRebuild委派原号重建用例。
3. candidateError按固定业务码区分输入、过期/冲突、配额与故障；rebuildError只允许受限字符长度的SEARCH_REBUILD码。

本文件需要守住的边界：

- 提交未知仍是可按原号查询的Unavailable，不能换新号重试副作用。
- NewSearch的兼容可变参数只在恰好一个用例时装配，不应误传多个能力对象。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/search/internal/server' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/search/internal/server/rpc_server.go') -and (Get-Item -LiteralPath 'services/search/internal/server/rpc_server.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/search/internal/server/rpc_server.go')) { New-Item -ItemType File -Path 'services/search/internal/server/rpc_server.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:313 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package server

import (
	pb "collaboration/contracts/internalv1"
	"context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"strings"
)

// rebuild.go：保留原实现，按业务合并文件。

````

## 第 2 步：RebuildOperations

插入位置：第 1 步代码之后，紧接追加；完成稿约第 14 行。

本步声明：`type RebuildOperations interface {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>StartRebuild</code> | <code>(context.Context, *pb.RebuildRequest) (*pb.OperationResult, error)</code> | ctx及完整RebuildRequest；返回原操作处理状态/固定结果或错误。 |
| <code>GetRebuild</code> | <code>(context.Context, *pb.OperationQuery) (*pb.OperationResult, error)</code> | ctx及带原号和请求指纹的OperationQuery；只查询原请求事实。 |

<!-- write-step: 2 bytes=313:509 -->
````go
type RebuildOperations interface {
	StartRebuild(context.Context, *pb.RebuildRequest) (*pb.OperationResult, error)
	GetRebuild(context.Context, *pb.OperationQuery) (*pb.OperationResult, error)
}

````

## 第 3 步：NewSearchWithRebuild

插入位置：第 2 步代码之后，紧接追加；完成稿约第 19 行。

本步声明：`func NewSearchWithRebuild(candidates CandidateOperations, rebuild RebuildOperations) *Search`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>candidates</code> | <code>CandidateOperations</code> | 候选查询和关闭游标的业务能力，不是ES裸客户端。<br>在此声明中的实际使用：<code>candidates: candidates</code> |
| <code>rebuild</code> | <code>RebuildOperations</code> | 原号开始/查询重建的业务能力。<br>在此声明中的实际使用：<code>rebuild: rebuild</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Search</code> | 使用 <code>Search</code> 类型；其定义处的职责：Search只做冻结RPC的传输适配，候选、关闭和重建分别委派业务用例。<br>实际返回表达式：<code>&amp;Search{candidates: candidates, rebuild: rebuild}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Search{candidates: candidates, rebuild: rebuild}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=509:665 -->
````go
func NewSearchWithRebuild(candidates CandidateOperations, rebuild RebuildOperations) *Search {
	return &Search{candidates: candidates, rebuild: rebuild}
}

````

## 第 4 步：Search.StartRebuild

插入位置：第 3 步代码之后，紧接追加；完成稿约第 23 行。

本步声明：`func (s *Search) StartRebuild(ctx context.Context, request *pb.RebuildRequest) (*pb.OperationResult, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.rebuild.StartRebuild(ctx, request)</code> |
| <code>request</code> | <code>*pb.RebuildRequest</code> | 当前冻结RPC请求，具体参数规则由所委派用例验证。<br>在此声明中的实际使用：<code>s.rebuild.StartRebuild(ctx, request)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.OperationResult</code> | 实际返回表达式：<code>nil</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>status.Error(codes.Unavailable, &quot;SEARCH_REBUILD_UNAVAILABLE&quot;)</code>；<code>rebuildError(err)</code> |

**跟着本段代码走一遍：**

- 检查 <code>s.rebuild == nil</code>。条件成立时执行 <code>return nil, status.Error(codes.Unavailable, &quot;SEARCH_REBUILD_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return result, rebuildError(err)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=665:967 -->
````go
func (s *Search) StartRebuild(ctx context.Context, request *pb.RebuildRequest) (*pb.OperationResult, error) {
	if s.rebuild == nil {
		return nil, status.Error(codes.Unavailable, "SEARCH_REBUILD_UNAVAILABLE")
	}
	result, err := s.rebuild.StartRebuild(ctx, request)
	return result, rebuildError(err)
}

````

## 第 5 步：Search.GetRebuild

插入位置：第 4 步代码之后，紧接追加；完成稿约第 31 行。

本步声明：`func (s *Search) GetRebuild(ctx context.Context, request *pb.OperationQuery) (*pb.OperationResult, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.rebuild.GetRebuild(ctx, request)</code> |
| <code>request</code> | <code>*pb.OperationQuery</code> | 当前冻结RPC请求，具体参数规则由所委派用例验证。<br>在此声明中的实际使用：<code>s.rebuild.GetRebuild(ctx, request)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.OperationResult</code> | 实际返回表达式：<code>nil</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>status.Error(codes.Unavailable, &quot;SEARCH_REBUILD_UNAVAILABLE&quot;)</code>；<code>rebuildError(err)</code> |

**跟着本段代码走一遍：**

- 检查 <code>s.rebuild == nil</code>。条件成立时执行 <code>return nil, status.Error(codes.Unavailable, &quot;SEARCH_REBUILD_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return result, rebuildError(err)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=967:1265 -->
````go
func (s *Search) GetRebuild(ctx context.Context, request *pb.OperationQuery) (*pb.OperationResult, error) {
	if s.rebuild == nil {
		return nil, status.Error(codes.Unavailable, "SEARCH_REBUILD_UNAVAILABLE")
	}
	result, err := s.rebuild.GetRebuild(ctx, request)
	return result, rebuildError(err)
}

````

## 第 6 步：rebuildError

插入位置：第 5 步代码之后，紧接追加；完成稿约第 39 行。

本步声明：`func rebuildError(err error) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>err</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>在此声明中的实际使用：<code>err == nil</code>；<code>err.Error</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>status.Error(codes.Unavailable, &quot;SEARCH_REBUILD_UNAVAILABLE&quot;)</code>；<code>status.Error(codes.InvalidArgument, code)</code>；<code>status.Error(codes.FailedPrecondition, code)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err == nil</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>!strings.HasPrefix(code, &quot;SEARCH_REBUILD_&quot;) &#124;&#124; len(code) &gt; 64</code>。条件成立时执行 <code>return status.Error(codes.Unavailable, &quot;SEARCH_REBUILD_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>!(r &gt;= &#x27;A&#x27; &amp;&amp; r &lt;= &#x27;Z&#x27; &#124;&#124; r &gt;= &#x27;0&#x27; &amp;&amp; r &lt;= &#x27;9&#x27; &#124;&#124; r == &#x27;_&#x27;)</code>。条件成立时执行 <code>return status.Error(codes.Unavailable, &quot;SEARCH_REBUILD_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>strings.HasSuffix(code, &quot;_INVALID&quot;)</code>。条件成立时执行 <code>return status.Error(codes.InvalidArgument, code)</code>。当前函数在这里结束。
- 检查 <code>strings.HasSuffix(code, &quot;_CONFLICT&quot;) &#124;&#124; strings.HasSuffix(code, &quot;_MISMATCH&quot;) &#124;&#124; strings.HasSuffix(code, &quot;_NOT_IMPORTED&quot;)</code>。条件成立时执行 <code>return status.Error(codes.FailedPrecondition, code)</code>。当前函数在这里结束。
- 检查 <code>strings.HasSuffix(code, &quot;_NOT_FOUND&quot;)</code>。条件成立时执行 <code>return status.Error(codes.NotFound, code)</code>。当前函数在这里结束。
- 此处通过 <code>return status.Error(codes.Unavailable, code)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1265:2248 -->
````go
func rebuildError(err error) error {
	if err == nil {
		return nil
	}
	code := err.Error()
	if !strings.HasPrefix(code, "SEARCH_REBUILD_") || len(code) > 64 {
		return status.Error(codes.Unavailable, "SEARCH_REBUILD_UNAVAILABLE")
	}
	for _, r := range code {
		if !(r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_') {
			return status.Error(codes.Unavailable, "SEARCH_REBUILD_UNAVAILABLE")
		}
	}
	// 只有输入错误/确定冲突按拒绝处理；提交或依赖未知仍须查询原操作。
	if strings.HasSuffix(code, "_INVALID") {
		return status.Error(codes.InvalidArgument, code)
	}
	if strings.HasSuffix(code, "_CONFLICT") || strings.HasSuffix(code, "_MISMATCH") || strings.HasSuffix(code, "_NOT_IMPORTED") {
		return status.Error(codes.FailedPrecondition, code)
	}
	if strings.HasSuffix(code, "_NOT_FOUND") {
		return status.Error(codes.NotFound, code)
	}
	return status.Error(codes.Unavailable, code)
}

// search.go：保留原实现，按业务合并文件。

````

## 第 7 步：Search

插入位置：第 6 步代码之后，紧接追加；完成稿约第 67 行。

Search只做冻结RPC的传输适配，候选、关闭和重建分别委派业务用例。

本步声明：`type Search struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>pb.UnimplementedSearchInternalServer</code> | <code>pb.UnimplementedSearchInternalServer</code> | 嵌入的类型/接口，组合其字段或方法集合。  |
| <code>candidates</code> | <code>CandidateOperations</code> | 候选业务依赖，为nil时不接受候选相关RPC。 |
| <code>rebuild</code> | <code>RebuildOperations</code> | 重建业务依赖，为nil时不接受重建相关RPC。 |

<!-- write-step: 7 bytes=2248:2466 -->
````go
// Search只做冻结RPC的传输适配，候选、关闭和重建分别委派业务用例。
type Search struct {
	pb.UnimplementedSearchInternalServer
	candidates CandidateOperations
	rebuild    RebuildOperations
}

````

## 第 8 步：CandidateOperations

插入位置：第 7 步代码之后，紧接追加；完成稿约第 74 行。

本步声明：`type CandidateOperations interface {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>SearchCandidates</code> | <code>(context.Context, *pb.SearchCandidatesRequest) (*pb.SearchCandidatesReply, error)</code> | ctx与规范查询绑定；返回有界候选批次及续页信息，最终用户可见性仍由文档侧核验。 |
| <code>CloseCursor</code> | <code>(context.Context, *pb.CloseCursorRequest) (*pb.OperationResult, error)</code> | ctx与原CloseCursorRequest；幂等关闭对应游标并返回操作状态。 |

<!-- write-step: 8 bytes=2466:2688 -->
````go
type CandidateOperations interface {
	SearchCandidates(context.Context, *pb.SearchCandidatesRequest) (*pb.SearchCandidatesReply, error)
	CloseCursor(context.Context, *pb.CloseCursorRequest) (*pb.OperationResult, error)
}

````

## 第 9 步：NewSearch

插入位置：第 8 步代码之后，紧接追加；完成稿约第 79 行。

本步声明：`func NewSearch(usecases ...CandidateOperations) *Search`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>usecases</code> | <code>...CandidateOperations</code> | 兼容构造入口的候选用例列表；仅恰好一项才装配。<br>在此声明中的实际使用：<code>len(usecases)</code>；<code>server.candidates = usecases&#91;0&#93;</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Search</code> | 使用 <code>Search</code> 类型；其定义处的职责：Search只做冻结RPC的传输适配，候选、关闭和重建分别委派业务用例。<br>实际返回表达式：<code>server</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(usecases) == 1</code>。
- 此处通过 <code>return server</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=2688:2847 -->
````go
func NewSearch(usecases ...CandidateOperations) *Search {
	server := &Search{}
	if len(usecases) == 1 {
		server.candidates = usecases[0]
	}
	return server
}

````

## 第 10 步：Search.SearchCandidates

插入位置：第 9 步代码之后，紧接追加；完成稿约第 87 行。

本步声明：`func (s *Search) SearchCandidates(ctx context.Context, request *pb.SearchCandidatesRequest) (*pb.SearchCandidatesReply, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.candidates.SearchCandidates(ctx, request)</code> |
| <code>request</code> | <code>*pb.SearchCandidatesRequest</code> | 当前冻结RPC请求，具体参数规则由所委派用例验证。<br>在此声明中的实际使用：<code>s.candidates.SearchCandidates(ctx, request)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.SearchCandidatesReply</code> | 实际返回表达式：<code>nil</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>status.Error(codes.Unavailable, &quot;SEARCH_CANDIDATE_UNAVAILABLE&quot;)</code>；<code>candidateError(err)</code> |

**跟着本段代码走一遍：**

- 检查 <code>s.candidates == nil</code>。条件成立时执行 <code>return nil, status.Error(codes.Unavailable, &quot;SEARCH_CANDIDATE_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return result, candidateError(err)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=2847:3182 -->
````go
func (s *Search) SearchCandidates(ctx context.Context, request *pb.SearchCandidatesRequest) (*pb.SearchCandidatesReply, error) {
	if s.candidates == nil {
		return nil, status.Error(codes.Unavailable, "SEARCH_CANDIDATE_UNAVAILABLE")
	}
	result, err := s.candidates.SearchCandidates(ctx, request)
	return result, candidateError(err)
}

````

## 第 11 步：Search.CloseCursor

插入位置：第 10 步代码之后，紧接追加；完成稿约第 95 行。

本步声明：`func (s *Search) CloseCursor(ctx context.Context, request *pb.CloseCursorRequest) (*pb.OperationResult, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.candidates.CloseCursor(ctx, request)</code> |
| <code>request</code> | <code>*pb.CloseCursorRequest</code> | 当前冻结RPC请求，具体参数规则由所委派用例验证。<br>在此声明中的实际使用：<code>s.candidates.CloseCursor(ctx, request)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.OperationResult</code> | 实际返回表达式：<code>nil</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>status.Error(codes.Unavailable, &quot;SEARCH_CANDIDATE_UNAVAILABLE&quot;)</code>；<code>candidateError(err)</code> |

**跟着本段代码走一遍：**

- 检查 <code>s.candidates == nil</code>。条件成立时执行 <code>return nil, status.Error(codes.Unavailable, &quot;SEARCH_CANDIDATE_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return result, candidateError(err)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=3182:3496 -->
````go
func (s *Search) CloseCursor(ctx context.Context, request *pb.CloseCursorRequest) (*pb.OperationResult, error) {
	if s.candidates == nil {
		return nil, status.Error(codes.Unavailable, "SEARCH_CANDIDATE_UNAVAILABLE")
	}
	result, err := s.candidates.CloseCursor(ctx, request)
	return result, candidateError(err)
}

````

## 第 12 步：candidateError

插入位置：第 11 步代码之后，紧接追加；完成稿约第 103 行。

本步声明：`func candidateError(err error) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>err</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>在此声明中的实际使用：<code>err == nil</code>；<code>err.Error</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>status.Error(codes.Unavailable, &quot;SEARCH_CANDIDATE_UNAVAILABLE&quot;)</code>；<code>status.Error(codes.InvalidArgument, code)</code>；<code>status.Error(codes.FailedPrecondition, code)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err == nil</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>!strings.HasPrefix(code, &quot;SEARCH_&quot;) &#124;&#124; len(code) &gt; 64</code>。条件成立时执行 <code>return status.Error(codes.Unavailable, &quot;SEARCH_CANDIDATE_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>!(r &gt;= &#x27;A&#x27; &amp;&amp; r &lt;= &#x27;Z&#x27; &#124;&#124; r &gt;= &#x27;0&#x27; &amp;&amp; r &lt;= &#x27;9&#x27; &#124;&#124; r == &#x27;_&#x27;)</code>。条件成立时执行 <code>return status.Error(codes.Unavailable, &quot;SEARCH_CANDIDATE_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return status.Error(codes.InvalidArgument, code)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return status.Error(codes.FailedPrecondition, code)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return status.Error(codes.ResourceExhausted, code)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return status.Error(codes.Unavailable, code)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=3496:4668 -->
````go
func candidateError(err error) error {
	if err == nil {
		return nil
	}
	code := err.Error()
	// 本层只交付固定业务码，不暴露SQL/ES原文；当前依赖均返回这些明确的封闭码。
	if !strings.HasPrefix(code, "SEARCH_") || len(code) > 64 {
		return status.Error(codes.Unavailable, "SEARCH_CANDIDATE_UNAVAILABLE")
	}
	for _, r := range code {
		if !(r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_') {
			return status.Error(codes.Unavailable, "SEARCH_CANDIDATE_UNAVAILABLE")
		}
	}
	switch code {
	case "SEARCH_QUERY_BINDING_INVALID", "SEARCH_SCOPE_INVALID", "SEARCH_CURSOR_INVALID", "SEARCH_CURSOR_ANCHOR_INVALID", "SEARCH_CURSOR_ENVELOPE_INVALID", "SEARCH_CANDIDATE_BATCH_SIZE_INVALID":
		return status.Error(codes.InvalidArgument, code)
	case "SEARCH_CURSOR_EXPIRED", "SEARCH_CURSOR_STALE", "SEARCH_CURSOR_CLOSED", "SEARCH_CURSOR_BINDING_MISMATCH", "SEARCH_CURSOR_REPLAY_CHANGED":
		return status.Error(codes.FailedPrecondition, code)
	case "SEARCH_CURSOR_QUOTA_EXCEEDED", "SEARCH_CURSOR_BUSY", "SEARCH_CURSOR_CLOSE_BUSY":
		return status.Error(codes.ResourceExhausted, code)
	default:
		return status.Error(codes.Unavailable, code)
	}
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/search/internal/server/rpc_server.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/search/05-cutover-wiring.md)。
