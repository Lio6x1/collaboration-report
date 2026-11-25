# 分步编写 `services/search/internal/svc/consumer_wiring.go`

先按 [S02 · 持久消费、日志身份与代次锁](../lessons/search/02-consumer-ledger.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/404-consumer-wiring.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

装配消费用例，并用UUID直接Fetch核对名称消费组取出的每条原字节。

成熟组协议按名称读取/提交，额外TopicID核验防同名主题重建后继续解释旧offset。

写之前先按这条执行路线理解各段的连接：

1. boundBroker.capture首次把identity持久绑定，后续Capture要求相同
2. Window前后核UUID，再读组窗口
3. Fetch缓存组record，用logs.Read按UUID取同offset逐字节比较才交付
4. Commit前核身份，组提交成功后再次核；后检查失败保存committed供原记录重试确认
5. NewConsumer组合Snapshots/ES/Kafka/LogSource/Guard/Processor并清理构造失败资源

本文件需要守住的边界：

- pending/committed分别代表UUID未核完和组提交后身份未核完，不能混为可丢弃缓存
- Ready独立identity锁不等待2秒Fetch大锁

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/search/internal/svc' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/search/internal/svc/consumer_wiring.go') -and (Get-Item -LiteralPath 'services/search/internal/svc/consumer_wiring.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/search/internal/svc/consumer_wiring.go')) { New-Item -ItemType File -Path 'services/search/internal/svc/consumer_wiring.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:674 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package svc

import (
	"bytes"
	"collaboration/platform"
	snapshots "collaboration/search/internal/client/document"
	"collaboration/search/internal/client/elastic"
	"collaboration/search/internal/client/kafka"
	"collaboration/search/internal/client/logsource"
	domain "collaboration/search/internal/domain/event"
	logic "collaboration/search/internal/logic/consumer"
	"collaboration/search/internal/logic/generation"
	"collaboration/search/internal/logic/indexing"
	"context"
	"errors"
	"sync"
	"sync/atomic"
)

// bound_broker.go：保留原实现，按业务合并文件。

````

## 第 2 步：boundBroker

插入位置：第 1 步代码之后，紧接追加；完成稿约第 23 行。

boundBroker保留成熟消费组实现，但所有交付给索引器的原字节必须由UUID Fetch再次确认。
持久绑定由服务自己的SQL用例提供；失败时缓存原在途记录，绝不越过它继续Fetch。

本步声明：`type boundBroker struct {`。

**这一段为什么这样写：** 给按主题名的组协议补上不可变日志身份验证。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>group</code> | <code>*kafka.Client</code> | 普通Kafka组读取/同步提交客户端 |
| <code>logs</code> | <code>*logsource.Client</code> | 按TopicID直接只读的日志客户端 |
| <code>bind</code> | <code>func(context.Context, logsource.Identity) error</code> | 首次身份落SQL函数 |
| <code>mu</code> | <code>sync.Mutex</code> | 串行Fetch/Commit/Close |
| <code>identityMu</code> | <code>sync.Mutex</code> | 仅串行首次/持续身份捕获，不挡Ready于长Fetch |
| <code>identity</code> | <code>*logsource.Identity</code> | 已持久固定的Cluster/Topic/UUID |
| <code>pending</code> | <code>*domain.Record</code> | 组已取但UUID原字节尚未核验记录 |
| <code>committed</code> | <code>*domain.Record</code> | 组已确认提交但后置身份核验尚未知的原记录 |

<!-- write-step: 2 bytes=674:1183 -->
````go
// boundBroker保留成熟消费组实现，但所有交付给索引器的原字节必须由UUID Fetch再次确认。
// 持久绑定由服务自己的SQL用例提供；失败时缓存原在途记录，绝不越过它继续Fetch。
type boundBroker struct {
	group      *kafka.Client
	logs       *logsource.Client
	bind       func(context.Context, logsource.Identity) error
	mu         sync.Mutex
	identityMu sync.Mutex
	identity   *logsource.Identity
	pending    *domain.Record
	committed  *domain.Record
}

````

## 第 3 步：boundBroker.capture

插入位置：第 2 步代码之后，紧接追加；完成稿约第 36 行。

本步声明：`func (b *boundBroker) capture(ctx context.Context) (logsource.Window, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>b.logs.Capture(ctx, b.identity)</code>；<code>b.bind(ctx, w.Identity)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>logsource.Window</code> | 使用 <code>logsource.Window</code> 类型；其定义处的职责：同一次身份核验期间观察的完整三分区窗口。<br>实际返回表达式：<code>w</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>b.identityMu.Lock()</code>。
- 登记 <code>b.identityMu.Unlock()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>b.identityMu.Unlock()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return w, err</code>。当前函数在这里结束。
- 检查 <code>b.identity == nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return w, err</code>。当前函数在这里结束。
- 此处通过 <code>return w, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=1183:1557 -->
````go
func (b *boundBroker) capture(ctx context.Context) (logsource.Window, error) {
	b.identityMu.Lock()
	defer b.identityMu.Unlock()
	w, err := b.logs.Capture(ctx, b.identity)
	if err != nil {
		return w, err
	}
	if b.identity == nil {
		if err := b.bind(ctx, w.Identity); err != nil {
			return w, err
		}
		identity := w.Identity
		b.identity = &identity
	}
	return w, nil
}

````

## 第 4 步：boundBroker.Window

插入位置：第 3 步代码之后，紧接追加；完成稿约第 53 行。

本步声明：`func (b *boundBroker) Window(ctx context.Context) (map[int]domain.PartitionWindow, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>b.capture(ctx)</code>；<code>b.group.Window(ctx)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>map&#91;int&#93;domain.PartitionWindow</code> | 实际返回表达式：<code>nil</code>；<code>w</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return w, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=1557:2150 -->
````go
func (b *boundBroker) Window(ctx context.Context) (map[int]domain.PartitionWindow, error) {
	// Ready的窗口检查不等待最多两秒的组Fetch；只有首次身份绑定需要独立串行。
	// kafka.Client.Window使用元数据连接，不读取reader/pending状态。
	if _, err := b.capture(ctx); err != nil {
		return nil, err
	}
	w, err := b.group.Window(ctx)
	if err != nil {
		return nil, err
	}
	// 组API按名称读取，前后身份检查防止将同名新主题的窗口误认为原日志。
	if _, err := b.capture(ctx); err != nil {
		return nil, err
	}
	return w, nil
}

````

## 第 5 步：boundBroker.Fetch

插入位置：第 4 步代码之后，紧接追加；完成稿约第 70 行。

本步声明：`func (b *boundBroker) Fetch(ctx context.Context) (domain.Record, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>b.capture(ctx)</code>；<code>b.group.Fetch(ctx)</code>；<code>b.logs.Read(ctx, w, record.Partition, record.Offset, record.Offset+1)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.Record</code> | 使用 <code>domain.Record</code> 类型；其定义处的职责：把Kafka库对象转换为业务能校验的记录。<br>实际返回表达式：<code>domain.Record{}</code>；<code>record</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;SEARCH_LOG_RANGE_INVALID&quot;)</code>；<code>errors.New(&quot;SEARCH_KAFKA_RECORD_IDENTITY_CHANGED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>b.mu.Lock()</code>。
- 登记 <code>b.mu.Unlock()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>b.mu.Unlock()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return domain.Record{}, err</code>。当前函数在这里结束。
- 检查 <code>b.pending == nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return domain.Record{}, err</code>。当前函数在这里结束。
- 检查 <code>record.Offset == int64(^uint64(0)&gt;&gt;1)</code>。条件成立时执行 <code>return domain.Record{}, errors.New(&quot;SEARCH_LOG_RANGE_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return domain.Record{}, err</code>。当前函数在这里结束。
- 检查 <code>len(rows) != 1 &#124;&#124; rows&#91;0&#93;.Position != record.Position &#124;&#124; !bytes.Equal(rows&#91;0&#93;.Value, record.Value)</code>。条件成立时执行 <code>return domain.Record{}, errors.New(&quot;SEARCH_KAFKA_RECORD_IDENTITY_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return record, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=2150:3162 -->
````go
func (b *boundBroker) Fetch(ctx context.Context) (domain.Record, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	w, err := b.capture(ctx)
	if err != nil {
		return domain.Record{}, err
	}
	if b.pending == nil {
		record, err := b.group.Fetch(ctx)
		if err != nil {
			return domain.Record{}, err
		}
		b.pending = &record
	}
	record := *b.pending
	if record.Offset == int64(^uint64(0)>>1) {
		return domain.Record{}, errors.New("SEARCH_LOG_RANGE_INVALID")
	}
	// 原窗口的L取当前保留起点，仅用于此条逐字节核对；持久消费next仍独立由SQL校验。
	for p, bounds := range w.Partitions {
		w.Partitions[p].Last = bounds.First
	}
	rows, _, err := b.logs.Read(ctx, w, record.Partition, record.Offset, record.Offset+1)
	if err != nil {
		return domain.Record{}, err
	}
	if len(rows) != 1 || rows[0].Position != record.Position || !bytes.Equal(rows[0].Value, record.Value) {
		return domain.Record{}, errors.New("SEARCH_KAFKA_RECORD_IDENTITY_CHANGED")
	}
	b.pending = nil
	return record, nil
}

````

## 第 6 步：boundBroker.Commit

插入位置：第 5 步代码之后，紧接追加；完成稿约第 103 行。

本步声明：`func (b *boundBroker) Commit(ctx context.Context, record domain.Record) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>b.capture(ctx)</code>；<code>b.group.Commit(ctx, record)</code> |
| <code>record</code> | <code>domain.Record</code> | 使用 <code>domain.Record</code> 类型；其定义处的职责：把Kafka库对象转换为业务能校验的记录。<br>在此声明中的实际使用：<code>record.Position</code>；<code>record.Value</code>；<code>b.group.Commit(ctx, record)</code>；<code>&amp;record</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;SEARCH_KAFKA_COMMIT_UNBOUND&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>b.mu.Lock()</code>。
- 登记 <code>b.mu.Unlock()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>b.mu.Unlock()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>b.committed != nil</code>。
- 检查 <code>b.committed.Position != record.Position &#124;&#124; !bytes.Equal(b.committed.Value, record.Value)</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_KAFKA_COMMIT_UNBOUND&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>b.group.Commit(ctx, record)</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 检查 <code>err == nil</code>。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=3162:3825 -->
````go
func (b *boundBroker) Commit(ctx context.Context, record domain.Record) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, err := b.capture(ctx); err != nil {
		return err
	}
	if b.committed != nil {
		if b.committed.Position != record.Position || !bytes.Equal(b.committed.Value, record.Value) {
			return errors.New("SEARCH_KAFKA_COMMIT_UNBOUND")
		}
		b.committed = nil
		return nil
	}
	if err := b.group.Commit(ctx, record); err != nil {
		return err
	}
	// 提交仍是名称协议，不声称与元数据原子；变化后拒绝继续使用原SQL进度。
	b.committed = &record
	_, err := b.capture(ctx)
	if err == nil {
		b.committed = nil
	}
	return err
}

````

## 第 7 步：boundBroker.Close

插入位置：第 6 步代码之后，紧接追加；完成稿约第 128 行。

本步声明：`func (b *boundBroker) Close() error`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>b.group.Close()</code> |

**跟着本段代码走一遍：**

- 执行 <code>b.mu.Lock()</code>。
- 登记 <code>b.mu.Unlock()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>b.mu.Unlock()</code>。
- 此处通过 <code>return b.group.Close()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=3825:4001 -->
````go
func (b *boundBroker) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.logs.Close()
	return b.group.Close()
}

// consumer.go：保留原实现，按业务合并文件。

````

## 第 8 步：Consumer

插入位置：第 7 步代码之后，紧接追加；完成稿约第 137 行。

svc组合本服务的私有库和客户端生命周期，业务提交顺序留给logic。

本步声明：`type Consumer struct {`。

**这一段为什么这样写：** 组合服务资源生命周期与业务消费循环。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>logic</code> | <code>*logic.Consumer</code> | 持在途记录/恢复状态的Consumer用例 |
| <code>kafka</code> | <code>*boundBroker</code> | 带UUID验证的boundBroker |
| <code>elastic</code> | <code>*elastic.Client</code> | ES连接池客户端 |
| <code>closed</code> | <code>atomic.Bool</code> | 原子关闭标志，避免重复释放 |

<!-- write-step: 8 bytes=4001:4213 -->
````go
// svc组合本服务的私有库和客户端生命周期，业务提交顺序留给logic。
type Consumer struct {
	logic   *logic.Consumer
	kafka   *boundBroker
	elastic *elastic.Client
	closed  atomic.Bool
}

````

## 第 9 步：NewConsumer

插入位置：第 8 步代码之后，紧接追加；完成稿约第 145 行。

本步声明：`func NewConsumer(store *platform.Store, config platform.Config) (*Consumer, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>store</code> | <code>*platform.Store</code> | 本服务私有依赖容器，包含数据库、生命周期上下文与就绪检查；不能用另一服务的Store跨库读写。<br>在此声明中的实际使用：<code>store == nil</code>；<code>store.DB</code>；<code>store.Role</code> |
| <code>config</code> | <code>platform.Config</code> | 经过平台加载的当前服务配置，装配层从这里选择角色、地址和本地文件路径；不从业务请求直接构造可信配置。<br>在此声明中的实际使用：<code>config.Role</code>；<code>config.Search</code>；<code>snapshots.NewSnapshots(config)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Consumer</code> | 使用 <code>Consumer</code> 类型；其定义处的职责：组合服务资源生命周期与业务消费循环。<br>实际返回表达式：<code>nil</code>；<code>&amp;Consumer{logic: consumer, kafka: bound, elastic: sink}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_CONSUMER_CONFIG_INVALID&quot;)</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>store == nil &#124;&#124; store.DB == nil &#124;&#124; store.Role != &quot;search&quot; &#124;&#124; config.Role != &quot;search&quot; &#124;&#124; config.Search == nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_CONSUMER_CONFIG_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>func() { 		if !assembled { 			sink.Close() 		} 	}()</code>，当前函数退出时执行，提前 return 也会触发。
- 在本段的匿名函数内：检查 <code>!assembled</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>func() { 		if !assembled { 			broker.Close() 		} 	}()</code>，当前函数退出时执行，提前 return 也会触发。
- 在本段的匿名函数内：检查 <code>!assembled</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return bindConsumerLog(ctx, store.DB, config.Search, identity)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return &amp;Consumer{logic: consumer, kafka: bound, elastic: sink}, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=4213:5551 -->
````go
func NewConsumer(store *platform.Store, config platform.Config) (*Consumer, error) {
	if store == nil || store.DB == nil || store.Role != "search" || config.Role != "search" || config.Search == nil {
		return nil, errors.New("SEARCH_CONSUMER_CONFIG_INVALID")
	}
	source, err := snapshots.NewSnapshots(config)
	if err != nil {
		return nil, err
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
	broker, err := kafka.New(config.Search)
	if err != nil {
		return nil, err
	}
	defer func() {
		if !assembled {
			broker.Close()
		}
	}()
	guard, err := generation.New(store.DB, sink, config.Search.Alias, config.Search.InitialIndex)
	if err != nil {
		return nil, err
	}
	processor := indexing.New(store.DB, source, sink, config.Search.Group)
	logs, err := logsource.New(config.Search.Brokers, config.Search.Topic)
	if err != nil {
		return nil, err
	}
	bound := &boundBroker{group: broker, logs: logs, bind: func(ctx context.Context, identity logsource.Identity) error {
		return bindConsumerLog(ctx, store.DB, config.Search, identity)
	}}
	consumer := logic.New(store.DB, bound, guard, processor, config.Search.Group, config.Search.Topic)
	assembled = true
	return &Consumer{logic: consumer, kafka: bound, elastic: sink}, nil
}

````

## 第 10 步：Consumer.Step

插入位置：第 9 步代码之后，紧接追加；完成稿约第 189 行。

本步声明：`func (c *Consumer) Step(ctx context.Context) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>c.logic.Step(ctx)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>c.logic.Step(ctx)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return c.logic.Step(ctx)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=5551:5631 -->
````go
func (c *Consumer) Step(ctx context.Context) error { return c.logic.Step(ctx) }
````

## 第 11 步：Consumer.Ready

插入位置：第 10 步代码之后，紧接追加；完成稿约第 190 行。

本步声明：`func (c *Consumer) Ready(ctx context.Context) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>c.logic.Ready(ctx)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_CONSUMER_STOPPED&quot;)</code>；<code>c.logic.Ready(ctx)</code> |

**跟着本段代码走一遍：**

- 检查 <code>c.closed.Load()</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_CONSUMER_STOPPED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return c.logic.Ready(ctx)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=5631:5786 -->
````go
func (c *Consumer) Ready(ctx context.Context) error {
	if c.closed.Load() {
		return errors.New("SEARCH_CONSUMER_STOPPED")
	}
	return c.logic.Ready(ctx)
}
````

## 第 12 步：Consumer.Close

插入位置：第 11 步代码之后，紧接追加；完成稿约第 196 行。

本步声明：`func (c *Consumer) Close() error`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>c.kafka.Close()</code> |

**跟着本段代码走一遍：**

- 检查 <code>!c.closed.CompareAndSwap(false, true)</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 登记 <code>c.elastic.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 此处通过 <code>return c.kafka.Close()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=5786:5932 -->
````go
func (c *Consumer) Close() error {
	if !c.closed.CompareAndSwap(false, true) {
		return nil
	}
	defer c.elastic.Close()
	return c.kafka.Close()
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/search/internal/svc/consumer_wiring.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/search/02-consumer-ledger.md)。
