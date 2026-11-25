# 分步编写 `services/search/internal/client/document/snapshot.go`

先按 [S01 · Canal事件与Doc当前正式投影](../lessons/search/01-event-authority.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/372-snapshot.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

以Search服务身份调用Doc GetPublicSnapshot并验证当前正式投影。

Doc不可达与文档当前隐藏是两件事；只有明确合法的隐藏快照能清除索引正文。

写之前先按这条执行路线理解各段的连接：

1. NewSnapshots要求role=search并加载共享内容校验器
2. Current取Event的document/sequence/release查询提示
3. Snapshot每次mTLS DialPeer，设2.5秒期限及32MiB响应上限
4. 成功调用FromSnapshot，任何RPC错误统一SEARCH_DOC_SNAPSHOT_UNKNOWN

本文件需要守住的边界：

- NotFound也不能当删除
- expected_release只是请求绑定提示，当前权威可以比原事件更新

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/search/internal/client/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/search/internal/client/document/snapshot.go') -and (Get-Item -LiteralPath 'services/search/internal/client/document/snapshot.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/search/internal/client/document/snapshot.go')) { New-Item -ItemType File -Path 'services/search/internal/client/document/snapshot.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:298 -->
````go
package document

import (
	codec "collaboration/contracts/content"
	pb "collaboration/contracts/internalv1"
	"collaboration/platform"
	domain "collaboration/search/internal/domain/event"
	index "collaboration/search/internal/domain/index"
	"context"
	"errors"
	"google.golang.org/grpc"
	"time"
)

````

## 第 2 步：Snapshots

插入位置：第 1 步代码之后，紧接追加；完成稿约第 15 行。

本步声明：`type Snapshots struct {`。

**这一段为什么这样写：** 封装Doc身份传输和共享正文校验依赖。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>configuration</code> | <code>platform.Config</code> | Search服务公开配置，供DialPeer加载运行凭据 |
| <code>validator</code> | <code>*codec.Validator</code> | 共享正文/provenance schema校验器 |

<!-- write-step: 2 bytes=298:388 -->
````go
type Snapshots struct {
	configuration platform.Config
	validator     *codec.Validator
}

````

## 第 3 步：NewSnapshots

插入位置：第 2 步代码之后，紧接追加；完成稿约第 20 行。

本步声明：`func NewSnapshots(configuration platform.Config) (*Snapshots, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>configuration</code> | <code>platform.Config</code> | 经过平台加载的当前服务配置，装配层从这里选择角色、地址和本地文件路径；不从业务请求直接构造可信配置。<br>在此声明中的实际使用：<code>configuration.Role</code>；<code>configuration: configuration</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Snapshots</code> | 使用 <code>Snapshots</code> 类型；其定义处的职责：封装Doc身份传输和共享正文校验依赖。<br>实际返回表达式：<code>nil</code>；<code>&amp;Snapshots{configuration: configuration, validator: validator}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_RPC_IDENTITY_INVALID&quot;)</code>；<code>errors.New(&quot;SEARCH_CONTENT_SCHEMA_UNAVAILABLE&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>configuration.Role != &quot;search&quot;</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_RPC_IDENTITY_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_CONTENT_SCHEMA_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return &amp;Snapshots{configuration: configuration, validator: validator}, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=388:756 -->
````go
func NewSnapshots(configuration platform.Config) (*Snapshots, error) {
	if configuration.Role != "search" {
		return nil, errors.New("SEARCH_RPC_IDENTITY_INVALID")
	}
	validator, err := codec.NewValidator()
	if err != nil {
		return nil, errors.New("SEARCH_CONTENT_SCHEMA_UNAVAILABLE")
	}
	return &Snapshots{configuration: configuration, validator: validator}, nil
}

````

## 第 4 步：Snapshots.Current

插入位置：第 3 步代码之后，紧接追加；完成稿约第 31 行。

本步声明：`func (s *Snapshots) Current(ctx context.Context, event domain.Event) (index.Authority, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.Snapshot(ctx, event.Document, event.Sequence, event.Release)</code> |
| <code>event</code> | <code>domain.Event</code> | 使用 <code>domain.Event</code> 类型；其定义处的职责：保存能够重新核验的原业务事件，而不是ES当前投影。<br>在此声明中的实际使用：<code>event.Document</code>；<code>event.Sequence</code>；<code>event.Release</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>index.Authority</code> | 使用 <code>index.Authority</code> 类型；其定义处的职责：Doc当前完整固定快照生成的索引权威。<br>由 <code>s.Snapshot(ctx, event.Document, event.Sequence, event.Release)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>s.Snapshot(ctx, event.Document, event.Sequence, event.Release)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return s.Snapshot(ctx, event.Document, event.Sequence, event.Release)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=756:926 -->
````go
func (s *Snapshots) Current(ctx context.Context, event domain.Event) (index.Authority, error) {
	return s.Snapshot(ctx, event.Document, event.Sequence, event.Release)
}

````

## 第 5 步：Snapshots.Snapshot

插入位置：第 4 步代码之后，紧接追加；完成稿约第 35 行。

本步声明：`func (s *Snapshots) Snapshot(ctx context.Context, document, minimum, release uint64) (index.Authority, error)`。

**这一段为什么这样写：** 请求并认证当前事实，保持原事件只提供最低水位。

1. 建立Doc客户端
2. 有界调用GetPublicSnapshot
3. 验证快照并返回Authority


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 父调用生命周期<br>在此声明中的实际使用：<code>ctx, cancel := context.WithTimeout(ctx, 2500*time.Millisecond)</code>；<code>context.WithTimeout(ctx, 2500*time.Millisecond)</code>；<code>pb.NewDocumentInternalClient(peer).GetPublicSnapshot(ctx, &amp;pb.PublicSnapshotRequest{ 		DocumentId: document, AggregateSeq: minimum, ExpectedReleaseId: release, 	}, grpc.MaxCallRecvMsgSize(32&lt;&lt;20))</code> |
| <code>document</code> | <code>uint64</code> | 目标文档ID<br>在此声明中的实际使用：<code>DocumentId: document</code>；<code>index.FromSnapshot(s.validator, reply, document, minimum)</code> |
| <code>minimum</code> | <code>uint64</code> | 可接受的最小聚合序号<br>在此声明中的实际使用：<code>AggregateSeq: minimum</code>；<code>index.FromSnapshot(s.validator, reply, document, minimum)</code> |
| <code>release</code> | <code>uint64</code> | 原事件/快照项的发布引用提示<br>在此声明中的实际使用：<code>ExpectedReleaseId: release</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>index.Authority</code> | 当前可信Authority<br>由 <code>index.FromSnapshot(s.validator, reply, document, minimum)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 未知或无效快照错误，禁止当隐藏<br>由 <code>index.FromSnapshot(s.validator, reply, document, minimum)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_DOC_RPC_UNAVAILABLE&quot;)</code>；<code>errors.New(&quot;SEARCH_DOC_SNAPSHOT_UNKNOWN&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, errors.New(&quot;SEARCH_DOC_RPC_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 登记 <code>peer.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>context.WithTimeout(ctx, 2500*time.Millisecond)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, errors.New(&quot;SEARCH_DOC_SNAPSHOT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return index.FromSnapshot(s.validator, reply, document, minimum)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=926:1782 -->
````go
func (s *Snapshots) Snapshot(ctx context.Context, document, minimum, release uint64) (index.Authority, error) {
	var result index.Authority
	peer, err := platform.DialPeer(s.configuration, "document")
	if err != nil {
		return result, errors.New("SEARCH_DOC_RPC_UNAVAILABLE")
	}
	defer peer.Close()
	ctx, cancel := context.WithTimeout(ctx, 2500*time.Millisecond)
	defer cancel()
	reply, err := pb.NewDocumentInternalClient(peer).GetPublicSnapshot(ctx, &pb.PublicSnapshotRequest{
		DocumentId: document, AggregateSeq: minimum, ExpectedReleaseId: release,
	}, grpc.MaxCallRecvMsgSize(32<<20))
	// 包括NotFound在内的未知返回都不能解释成删除。隐藏必须来自Doc的完整当前权威回复。
	if err != nil {
		return result, errors.New("SEARCH_DOC_SNAPSHOT_UNKNOWN")
	}
	return index.FromSnapshot(s.validator, reply, document, minimum)
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/search/internal/client/document/snapshot.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/search/01-event-authority.md)。
