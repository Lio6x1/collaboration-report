# 分步编写 `services/search/internal/svc/candidates.go`

先按 [S03 · 签名批锚点、PIT与当前权限二次过滤](../lessons/search/03-candidates-cursor.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/403-candidates.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

从Search配置装配游标签名、代次守卫、ES客户端并管理关闭。

密钥编码和TTL要在启动时明确失败，运行中不能静默换key使已签锚点失去绑定。

写之前先按这条执行路线理解各段的连接：

1. NewCandidates核search store/config，解码规范hex CursorKey
2. 创建ES和generation guard，再构造logic.Candidates及TTL
3. 构造失败关闭已创建ES连接
4. SearchCandidates/CloseCursor/Ready检查closed后委派，Close原子单次释放

本文件需要守住的边界：

- 不输出CursorKey到日志或教材
- Cleanup负责恢复SQL/PIT，关闭连接池不等于已经关闭全部持久链

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/search/internal/svc' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/search/internal/svc/candidates.go') -and (Get-Item -LiteralPath 'services/search/internal/svc/candidates.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/search/internal/svc/candidates.go')) { New-Item -ItemType File -Path 'services/search/internal/svc/candidates.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:306 -->
````go
package svc

import (
	pb "collaboration/contracts/internalv1"
	"collaboration/platform"
	"collaboration/search/internal/client/elastic"
	logic "collaboration/search/internal/logic/candidates"
	"collaboration/search/internal/logic/generation"
	"context"
	"encoding/hex"
	"errors"
	"sync/atomic"
	"time"
)

````

## 第 2 步：Candidates

插入位置：第 1 步代码之后，紧接追加；完成稿约第 16 行。

本步声明：`type Candidates struct {`。

**这一段为什么这样写：** 候选服务的装配与生命周期包装。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>usecase</code> | <code>*logic.Candidates</code> | 持久PIT链协议用例 |
| <code>elastic</code> | <code>*elastic.Client</code> | 此服务实例拥有的ES连接池 |
| <code>closed</code> | <code>atomic.Bool</code> | 原子停止标记，拒绝新查询 |

<!-- write-step: 2 bytes=306:407 -->
````go
type Candidates struct {
	usecase *logic.Candidates
	elastic *elastic.Client
	closed  atomic.Bool
}

````

## 第 3 步：NewCandidates

插入位置：第 2 步代码之后，紧接追加；完成稿约第 22 行。

本步声明：`func NewCandidates(store *platform.Store, config platform.Config) (*Candidates, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>*platform.Store</code> | 本服务私有依赖容器，包含数据库、生命周期上下文与就绪检查；不能用另一服务的Store跨库读写。<br>在此声明中的实际使用：<code>store == nil</code>；<code>store.DB</code>；<code>store.Role</code> |
| <code>config</code> | <code>platform.Config</code> | 经过平台加载的当前服务配置，装配层从这里选择角色、地址和本地文件路径；不从业务请求直接构造可信配置。<br>在此声明中的实际使用：<code>config.Role</code>；<code>config.Search</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Candidates</code> | 使用 <code>Candidates</code> 类型；其定义处的职责：候选服务的装配与生命周期包装。<br>实际返回表达式：<code>nil</code>；<code>&amp;Candidates{usecase: usecase, elastic: sink}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_CANDIDATE_CONFIG_INVALID&quot;)</code>；<code>errors.New(&quot;SEARCH_CURSOR_KEY_INVALID&quot;)</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>store == nil &#124;&#124; store.DB == nil &#124;&#124; store.Role != &quot;search&quot; &#124;&#124; config.Role != &quot;search&quot; &#124;&#124; config.Search == nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_CANDIDATE_CONFIG_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; hex.EncodeToString(key) != config.Search.CursorKey</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_CURSOR_KEY_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>func() { 		if !assembled { 			sink.Close() 		} 	}()</code>，当前函数退出时执行，提前 return 也会触发。
- 在本段的匿名函数内：检查 <code>!assembled</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return &amp;Candidates{usecase: usecase, elastic: sink}, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=407:1400 -->
````go
func NewCandidates(store *platform.Store, config platform.Config) (*Candidates, error) {
	if store == nil || store.DB == nil || store.Role != "search" || config.Role != "search" || config.Search == nil {
		return nil, errors.New("SEARCH_CANDIDATE_CONFIG_INVALID")
	}
	key, err := hex.DecodeString(config.Search.CursorKey)
	if err != nil || hex.EncodeToString(key) != config.Search.CursorKey {
		return nil, errors.New("SEARCH_CURSOR_KEY_INVALID")
	}
	sink, err := elastic.New(config.Search)
	if err != nil {
		return nil, err
	}
	assembled := false
	defer func() {
		if !assembled {
			sink.Close()
		}
	}()
	guard, err := generation.New(store.DB, sink, config.Search.Alias, config.Search.InitialIndex)
	if err != nil {
		return nil, err
	}
	usecase, err := logic.New(store.DB, guard, sink, config.Search.Alias, key, time.Duration(config.Search.CursorTTLSeconds)*time.Second)
	if err != nil {
		return nil, err
	}
	assembled = true
	return &Candidates{usecase: usecase, elastic: sink}, nil
}

````

## 第 4 步：Candidates.SearchCandidates

插入位置：第 3 步代码之后，紧接追加；完成稿约第 52 行。

本步声明：`func (c *Candidates) SearchCandidates(ctx context.Context, req *pb.SearchCandidatesRequest) (*pb.SearchCandidatesReply, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>c.usecase.SearchCandidates(ctx, req)</code> |
| <code>req</code> | <code>*pb.SearchCandidatesRequest</code> | 在此声明中的实际使用：<code>c.usecase.SearchCandidates(ctx, req)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.SearchCandidatesReply</code> | 由 <code>c.usecase.SearchCandidates(ctx, req)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>c.usecase.SearchCandidates(ctx, req)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_CANDIDATE_STOPPED&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>c.closed.Load()</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_CANDIDATE_STOPPED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return c.usecase.SearchCandidates(ctx, req)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=1400:1654 -->
````go
func (c *Candidates) SearchCandidates(ctx context.Context, req *pb.SearchCandidatesRequest) (*pb.SearchCandidatesReply, error) {
	if c.closed.Load() {
		return nil, errors.New("SEARCH_CANDIDATE_STOPPED")
	}
	return c.usecase.SearchCandidates(ctx, req)
}
````

## 第 5 步：Candidates.CloseCursor

插入位置：第 4 步代码之后，紧接追加；完成稿约第 58 行。

本步声明：`func (c *Candidates) CloseCursor(ctx context.Context, req *pb.CloseCursorRequest) (*pb.OperationResult, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>c.usecase.CloseCursor(ctx, req)</code> |
| <code>req</code> | <code>*pb.CloseCursorRequest</code> | 在此声明中的实际使用：<code>c.usecase.CloseCursor(ctx, req)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.OperationResult</code> | 由 <code>c.usecase.CloseCursor(ctx, req)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>c.usecase.CloseCursor(ctx, req)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_CANDIDATE_STOPPED&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>c.closed.Load()</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_CANDIDATE_STOPPED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return c.usecase.CloseCursor(ctx, req)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1654:1887 -->
````go
func (c *Candidates) CloseCursor(ctx context.Context, req *pb.CloseCursorRequest) (*pb.OperationResult, error) {
	if c.closed.Load() {
		return nil, errors.New("SEARCH_CANDIDATE_STOPPED")
	}
	return c.usecase.CloseCursor(ctx, req)
}
````

## 第 6 步：Candidates.Cleanup

插入位置：第 5 步代码之后，紧接追加；完成稿约第 64 行。

本步声明：`func (c *Candidates) Cleanup(ctx context.Context) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>c.usecase.Cleanup(ctx)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>c.usecase.Cleanup(ctx)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return c.usecase.Cleanup(ctx)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1887:1977 -->
````go
func (c *Candidates) Cleanup(ctx context.Context) error { return c.usecase.Cleanup(ctx) }
````

## 第 7 步：Candidates.Ready

插入位置：第 6 步代码之后，紧接追加；完成稿约第 65 行。

本步声明：`func (c *Candidates) Ready(ctx context.Context) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>c.usecase.Ready(ctx)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_CANDIDATE_STOPPED&quot;)</code>；<code>c.usecase.Ready(ctx)</code> |

**跟着本段代码走一遍：**

- 检查 <code>c.closed.Load()</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_CANDIDATE_STOPPED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return c.usecase.Ready(ctx)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=1977:2137 -->
````go
func (c *Candidates) Ready(ctx context.Context) error {
	if c.closed.Load() {
		return errors.New("SEARCH_CANDIDATE_STOPPED")
	}
	return c.usecase.Ready(ctx)
}
````

## 第 8 步：Candidates.Close

插入位置：第 7 步代码之后，紧接追加；完成稿约第 71 行。

本步声明：`func (c *Candidates) Close()`。

**跟着本段代码走一遍：**

- 检查 <code>c.closed.CompareAndSwap(false, true)</code>。

<!-- write-step: 8 bytes=2137:2236 -->
````go
func (c *Candidates) Close() {
	if c.closed.CompareAndSwap(false, true) {
		c.elastic.Close()
	}
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/search/internal/svc/candidates.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/search/03-candidates-cursor.md)。
