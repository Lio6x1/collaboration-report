# 分步编写 `services/search/internal/logic/indexing/processor.go`

先按 [S02 · 持久消费、日志身份与代次锁](../lessons/search/02-consumer-ledger.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/395-processor.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

把一个原事件转成Doc当前权威并按ES→SQL成功证据顺序应用。

事件到达时Doc可能已更高序号；保存旧event digest不妨碍索引当前事实，但两种摘要必须独立。

写之前先按这条执行路线理解各段的连接：

1. Event先receive原行/APPLYING并提交，网络查Doc不占原事件事务
2. 同物理索引已SUCCEEDED走verifyReplay只读核验
3. apply锁原事件与index_application，ShouldApply后调用ES并记录实际返回水位
4. 同事务SaveApplication+Complete后COMMIT
5. 失败用独立2秒预算记RETRY，不能覆盖已有SUCCEEDED

本文件需要守住的边界：

- Doc未知不能合成为隐藏墓碑
- ES成功但SQL未知保留原记录重试
- verifyReplay只读回滚，不能提交缺行零摘要占位

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/search/internal/logic/indexing' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/search/internal/logic/indexing/processor.go') -and (Get-Item -LiteralPath 'services/search/internal/logic/indexing/processor.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/search/internal/logic/indexing/processor.go')) { New-Item -ItemType File -Path 'services/search/internal/logic/indexing/processor.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:251 -->
````go
package indexing

import (
	domain "collaboration/search/internal/domain/event"
	index "collaboration/search/internal/domain/index"
	repository "collaboration/search/internal/repository/event"
	"context"
	"database/sql"
	"errors"
	"regexp"
	"time"
)

````

## 第 2 步：SnapshotSource

插入位置：第 1 步代码之后，紧接追加；完成稿约第 14 行。

接口由索引用例定义：客户端负责真实协议，用例负责三个系统之间的提交顺序。

本步声明：`type SnapshotSource interface {`。

**这一段为什么这样写：** 索引用例的权威读取边界。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Current</code> | <code>(context.Context, domain.Event) (index.Authority, error)</code> | 输入ctx与原Event（document/最低sequence/release提示），返回Doc当前Authority及错误；失败不得推断隐藏 |

<!-- write-step: 2 bytes=251:463 -->
````go
// 接口由索引用例定义：客户端负责真实协议，用例负责三个系统之间的提交顺序。
type SnapshotSource interface {
	Current(context.Context, domain.Event) (index.Authority, error)
}

````

## 第 3 步：IndexSink

插入位置：第 2 步代码之后，紧接追加；完成稿约第 19 行。

本步声明：`type IndexSink interface {`。

**这一段为什么这样写：** 索引用例的实际ES写入边界。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Apply</code> | <code>(context.Context, string, index.Authority) (index.State, error)</code> | 输入ctx、物理索引名、Authority；返回ES实际State和错误，状态可能高于输入水位 |

<!-- write-step: 3 bytes=463:563 -->
````go
type IndexSink interface {
	Apply(context.Context, string, index.Authority) (index.State, error)
}

````

## 第 4 步：Processor

插入位置：第 3 步代码之后，紧接追加；完成稿约第 23 行。

本步声明：`type Processor struct {`。

**这一段为什么这样写：** 组织原事件账本与当前投影恢复顺序。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>database</code> | <code>*sql.DB</code> | Search私有库，保存原事件与应用水位 |
| <code>source</code> | <code>SnapshotSource</code> | Doc当前快照读取器 |
| <code>sink</code> | <code>IndexSink</code> | ES单调写入器 |
| <code>consumer</code> | <code>string</code> | 原事件账本所属消费组 |

<!-- write-step: 4 bytes=563:670 -->
````go
type Processor struct {
	database *sql.DB
	source   SnapshotSource
	sink     IndexSink
	consumer string
}

````

## 第 5 步：New

插入位置：第 4 步代码之后，紧接追加；完成稿约第 30 行。

本步声明：`func New(database *sql.DB, source SnapshotSource, sink IndexSink, consumer string) *Processor`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>database</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>database: database</code> |
| <code>source</code> | <code>SnapshotSource</code> | 使用 <code>SnapshotSource</code> 类型；其定义处的职责：索引用例的权威读取边界。<br>在此声明中的实际使用：<code>source: source</code> |
| <code>sink</code> | <code>IndexSink</code> | 使用 <code>IndexSink</code> 类型；其定义处的职责：索引用例的实际ES写入边界。<br>在此声明中的实际使用：<code>sink: sink</code> |
| <code>consumer</code> | <code>string</code> | 在此声明中的实际使用：<code>consumer: consumer</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Processor</code> | 使用 <code>Processor</code> 类型；其定义处的职责：组织原事件账本与当前投影恢复顺序。<br>实际返回表达式：<code>&amp;Processor{database: database, source: source, sink: sink, consumer: consumer}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Processor{database: database, source: source, sink: sink, consumer: consumer}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=670:856 -->
````go
func New(database *sql.DB, source SnapshotSource, sink IndexSink, consumer string) *Processor {
	return &Processor{database: database, source: source, sink: sink, consumer: consumer}
}

````

## 第 6 步：Processor.Event

插入位置：第 5 步代码之后，紧接追加；完成稿约第 34 行。

Event不提交Kafka。先持久保存原行，再取Doc当前权威，最后在ES成功之后
同事务保存实际应用水位与成功回执；中间任何未知结果都允许沿原消息重读。

本步声明：`func (p *Processor) Event(ctx context.Context, physical string, event domain.Event, position repository.Position) (outcome error)`。

**这一段为什么这样写：** 处理事件但绝不提交Kafka。

1. 登记不可变原行
2. 查询当前权威
3. 幂等应用ES
4. 同事务保存实际水位/成功回执


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 记录级有界context<br>在此声明中的实际使用：<code>ctx, cancel := context.WithTimeout(ctx, 30*time.Second)</code>；<code>context.WithTimeout(ctx, 30*time.Second)</code>；<code>p.receive(ctx, event, position)</code>；<code>p.verifyReplay(ctx, physical, event, position)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>physical</code> | <code>string</code> | 代次锁固定的目标索引<br>在此声明中的实际使用：<code>receipt.Physical == physical</code>；<code>p.verifyReplay(ctx, physical, event, position)</code>；<code>p.apply(ctx, physical, event, position, authority)</code> |
| <code>event</code> | <code>domain.Event</code> | 已严格解析原outbox事件<br>在此声明中的实际使用：<code>p.receive(ctx, event, position)</code>；<code>p.retry(event, position, outcome)</code>；<code>p.verifyReplay(ctx, physical, event, position)</code>；<code>p.source.Current(ctx, event)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>position</code> | <code>repository.Position</code> | 该事件来自的Kafka记录位置<br>在此声明中的实际使用：<code>p.receive(ctx, event, position)</code>；<code>p.retry(event, position, outcome)</code>；<code>p.verifyReplay(ctx, physical, event, position)</code>；<code>p.apply(ctx, physical, event, position, authority)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>outcome</code> | <code>error</code> | 错误会触发原号RETRY记账，调用方保留整条记录<br>实际返回表达式：<code>errors.New(&quot;SEARCH_INDEXING_DEPENDENCIES_UNAVAILABLE&quot;)</code>；<code>err</code>；<code>errors.New(&quot;SEARCH_ORIGINAL_EVENT_REJECTED&quot;)</code>；<code>p.verifyReplay(ctx, physical, event, position)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>p.database == nil &#124;&#124; p.source == nil &#124;&#124; p.sink == nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_INDEXING_DEPENDENCIES_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 执行 <code>context.WithTimeout(ctx, 30*time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>receipt.State == &quot;REJECTED&quot;</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_ORIGINAL_EVENT_REJECTED&quot;)</code>。当前函数在这里结束。
- 登记 <code>func() { 		if outcome != nil { 			p.retry(event, position, outcome) 		} 	}()</code>，当前函数退出时执行，提前 return 也会触发。
- 在本段的匿名函数内：检查 <code>outcome != nil</code>。
- 检查 <code>receipt.State == &quot;SUCCEEDED&quot; &amp;&amp; receipt.Physical == physical</code>。条件成立时执行 <code>return p.verifyReplay(ctx, physical, event, position)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return publicError(err, &quot;SEARCH_DOC_SNAPSHOT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>authority.Document != event.Document &#124;&#124; authority.Sequence &lt; event.Sequence</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_CURRENT_AUTHORITY_UNBOUND&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return p.apply(ctx, physical, event, position, authority)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=856:2203 -->
````go
// Event不提交Kafka。先持久保存原行，再取Doc当前权威，最后在ES成功之后
// 同事务保存实际应用水位与成功回执；中间任何未知结果都允许沿原消息重读。
func (p *Processor) Event(ctx context.Context, physical string, event domain.Event, position repository.Position) (outcome error) {
	if p.database == nil || p.source == nil || p.sink == nil {
		return errors.New("SEARCH_INDEXING_DEPENDENCIES_UNAVAILABLE")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	receipt, err := p.receive(ctx, event, position)
	if err != nil {
		return err
	}
	if receipt.State == "REJECTED" {
		return errors.New("SEARCH_ORIGINAL_EVENT_REJECTED")
	}
	defer func() {
		if outcome != nil {
			p.retry(event, position, outcome)
		}
	}()
	if receipt.State == "SUCCEEDED" && receipt.Physical == physical {
		return p.verifyReplay(ctx, physical, event, position)
	}
	// 网络请求不占用原事件事务；Doc不可达或未知不能被合成为删除墓碑。
	authority, err := p.source.Current(ctx, event)
	if err != nil {
		return publicError(err, "SEARCH_DOC_SNAPSHOT_UNKNOWN")
	}
	if authority.Document != event.Document || authority.Sequence < event.Sequence {
		return errors.New("SEARCH_CURRENT_AUTHORITY_UNBOUND")
	}
	return p.apply(ctx, physical, event, position, authority)
}

````

## 第 7 步：Processor.receive

插入位置：第 6 步代码之后，紧接追加；完成稿约第 68 行。

本步声明：`func (p *Processor) receive(ctx context.Context, event domain.Event, position repository.Position) (repository.Receipt, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>p.database.BeginTx(ctx, nil)</code>；<code>repository.Receive(ctx, tx, p.consumer, event, position)</code>；<code>repository.SetApplying(ctx, tx, p.consumer, event.ID)</code> |
| <code>event</code> | <code>domain.Event</code> | 使用 <code>domain.Event</code> 类型；其定义处的职责：保存能够重新核验的原业务事件，而不是ES当前投影。<br>在此声明中的实际使用：<code>repository.Receive(ctx, tx, p.consumer, event, position)</code>；<code>event.ID</code> |
| <code>position</code> | <code>repository.Position</code> | 使用 <code>repository.Position</code> 类型；其定义处的职责：重新解析并核验准备写账本的Event原行及传输位置。<br>在此声明中的实际使用：<code>repository.Receive(ctx, tx, p.consumer, event, position)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>repository.Receipt</code> | 使用 <code>repository.Receipt</code> 类型；其定义处的职责：原事件第一次应用事实的最小重放信息。<br>实际返回表达式：<code>repository.Receipt{}</code>；<code>receipt</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_LEDGER_TRANSACTION_UNAVAILABLE&quot;)</code>；<code>err</code>；<code>errors.New(&quot;SEARCH_ORIGIN_COMMIT_UNKNOWN&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>p.database.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return repository.Receipt{}, errors.New(&quot;SEARCH_LEDGER_TRANSACTION_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return receipt, err</code>。当前函数在这里结束。
- 检查 <code>receipt.State != &quot;SUCCEEDED&quot; &amp;&amp; receipt.State != &quot;REJECTED&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return receipt, err</code>。当前函数在这里结束。
- 检查 <code>tx.Commit() != nil</code>。条件成立时执行 <code>return receipt, errors.New(&quot;SEARCH_ORIGIN_COMMIT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return receipt, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=2203:2901 -->
````go
func (p *Processor) receive(ctx context.Context, event domain.Event, position repository.Position) (repository.Receipt, error) {
	tx, err := p.database.BeginTx(ctx, nil)
	if err != nil {
		return repository.Receipt{}, errors.New("SEARCH_LEDGER_TRANSACTION_UNAVAILABLE")
	}
	defer tx.Rollback()
	receipt, err := repository.Receive(ctx, tx, p.consumer, event, position)
	if err != nil {
		return receipt, err
	}
	if receipt.State != "SUCCEEDED" && receipt.State != "REJECTED" {
		if err = repository.SetApplying(ctx, tx, p.consumer, event.ID); err != nil {
			return receipt, err
		}
	}
	if tx.Commit() != nil {
		return receipt, errors.New("SEARCH_ORIGIN_COMMIT_UNKNOWN")
	}
	return receipt, nil
}

````

## 第 8 步：Processor.verifyReplay

插入位置：第 7 步代码之后，紧接追加；完成稿约第 89 行。

本步声明：`func (p *Processor) verifyReplay(ctx context.Context, physical string, event domain.Event, position repository.Position) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>p.database.BeginTx(ctx, nil)</code>；<code>repository.Receive(ctx, tx, p.consumer, event, position)</code>；<code>repository.LockApplication(ctx, tx, physical, event.Document)</code> |
| <code>physical</code> | <code>string</code> | 在此声明中的实际使用：<code>repository.LockApplication(ctx, tx, physical, event.Document)</code>；<code>receipt.Physical != physical</code> |
| <code>event</code> | <code>domain.Event</code> | 使用 <code>domain.Event</code> 类型；其定义处的职责：保存能够重新核验的原业务事件，而不是ES当前投影。<br>在此声明中的实际使用：<code>repository.Receive(ctx, tx, p.consumer, event, position)</code>；<code>event.Document</code> |
| <code>position</code> | <code>repository.Position</code> | 使用 <code>repository.Position</code> 类型；其定义处的职责：重新解析并核验准备写账本的Event原行及传输位置。<br>在此声明中的实际使用：<code>repository.Receive(ctx, tx, p.consumer, event, position)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_REPLAY_TRANSACTION_UNAVAILABLE&quot;)</code>；<code>err</code>；<code>errors.New(&quot;SEARCH_REPLAY_APPLICATION_UNBOUND&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>p.database.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REPLAY_TRANSACTION_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; receipt.State != &quot;SUCCEEDED&quot; &#124;&#124; receipt.Physical != physical &#124;&#124; 		actual.Digest == (&#91;32&#93;byte{}) &#124;&#124; actual.Sequence &lt; receipt.Applied.Sequence &#124;&#124; 		(actual.Sequence == …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return errors.New(&quot;SEARCH_REPLAY_APPLICATION_UNBOUND&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=2901:3795 -->
````go
func (p *Processor) verifyReplay(ctx context.Context, physical string, event domain.Event, position repository.Position) error {
	tx, err := p.database.BeginTx(ctx, nil)
	if err != nil {
		return errors.New("SEARCH_REPLAY_TRANSACTION_UNAVAILABLE")
	}
	defer tx.Rollback()
	receipt, err := repository.Receive(ctx, tx, p.consumer, event, position)
	if err != nil {
		return err
	}
	actual, err := repository.LockApplication(ctx, tx, physical, event.Document)
	if err != nil || receipt.State != "SUCCEEDED" || receipt.Physical != physical ||
		actual.Digest == ([32]byte{}) || actual.Sequence < receipt.Applied.Sequence ||
		(actual.Sequence == receipt.Applied.Sequence && actual.Digest != receipt.Applied.Digest) {
		return errors.New("SEARCH_REPLAY_APPLICATION_UNBOUND")
	}
	// 只读核验后回滚，不能因重读增加应用revision或提交缺失状态的零摘要占位。
	return nil
}

````

## 第 9 步：Processor.apply

插入位置：第 8 步代码之后，紧接追加；完成稿约第 109 行。

本步声明：`func (p *Processor) apply(ctx context.Context, physical string, event domain.Event, position repository.Position, authority index.Authority) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>p.database.BeginTx(ctx, nil)</code>；<code>repository.Receive(ctx, tx, p.consumer, event, position)</code>；<code>repository.LockApplication(ctx, tx, physical, event.Document)</code>；<code>p.sink.Apply(ctx, physical, authority)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>physical</code> | <code>string</code> | 在此声明中的实际使用：<code>repository.LockApplication(ctx, tx, physical, event.Document)</code>；<code>p.sink.Apply(ctx, physical, authority)</code>；<code>repository.SaveApplication(ctx, tx, physical, event.Document, actual)</code>；<code>repository.Complete(ctx, tx, p.consumer, event.ID, physical, actual)</code> |
| <code>event</code> | <code>domain.Event</code> | 使用 <code>domain.Event</code> 类型；其定义处的职责：保存能够重新核验的原业务事件，而不是ES当前投影。<br>在此声明中的实际使用：<code>repository.Receive(ctx, tx, p.consumer, event, position)</code>；<code>event.Document</code>；<code>event.ID</code> |
| <code>position</code> | <code>repository.Position</code> | 使用 <code>repository.Position</code> 类型；其定义处的职责：重新解析并核验准备写账本的Event原行及传输位置。<br>在此声明中的实际使用：<code>repository.Receive(ctx, tx, p.consumer, event, position)</code> |
| <code>authority</code> | <code>index.Authority</code> | 使用 <code>index.Authority</code> 类型；其定义处的职责：Doc当前完整固定快照生成的索引权威。<br>在此声明中的实际使用：<code>index.ShouldApply(actual, authority)</code>；<code>p.sink.Apply(ctx, physical, authority)</code>；<code>authority.Sequence</code>；<code>authority.Digest</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_APPLICATION_TRANSACTION_UNAVAILABLE&quot;)</code>；<code>err</code>；<code>errors.New(&quot;SEARCH_ORIGINAL_EVENT_REJECTED&quot;)</code>；<code>publicError(err, &quot;SEARCH_ES_APPLICATION_UNKNOWN&quot;)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 执行 <code>p.database.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_APPLICATION_TRANSACTION_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>receipt.State == &quot;REJECTED&quot;</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_ORIGINAL_EVENT_REJECTED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>apply</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return publicError(err, &quot;SEARCH_ES_APPLICATION_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>actual.Sequence &lt; authority.Sequence &#124;&#124; 			(actual.Sequence == authority.Sequence &amp;&amp; (actual.Digest != authority.Digest &#124;&#124; 				actual.Visible != authority.Visible &#124;&#124; actual.Release …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return errors.New(&quot;SEARCH_ES_APPLICATION_UNBOUND&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>receipt.State != &quot;SUCCEEDED&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>tx.Commit() != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_APPLICATION_COMMIT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 9 bytes=3795:5458 -->
````go
func (p *Processor) apply(ctx context.Context, physical string, event domain.Event, position repository.Position, authority index.Authority) error {
	tx, err := p.database.BeginTx(ctx, nil)
	if err != nil {
		return errors.New("SEARCH_APPLICATION_TRANSACTION_UNAVAILABLE")
	}
	defer tx.Rollback()
	receipt, err := repository.Receive(ctx, tx, p.consumer, event, position)
	if err != nil {
		return err
	}
	if receipt.State == "REJECTED" {
		return errors.New("SEARCH_ORIGINAL_EVENT_REJECTED")
	}
	actual, err := repository.LockApplication(ctx, tx, physical, event.Document)
	if err != nil {
		return err
	}
	apply, err := index.ShouldApply(actual, authority)
	if err != nil {
		return err
	}
	if apply {
		actual, err = p.sink.Apply(ctx, physical, authority)
		if err != nil {
			return publicError(err, "SEARCH_ES_APPLICATION_UNKNOWN")
		}
		if actual.Sequence < authority.Sequence ||
			(actual.Sequence == authority.Sequence && (actual.Digest != authority.Digest ||
				actual.Visible != authority.Visible || actual.Release != authority.Release || actual.Policy != authority.Policy)) {
			return errors.New("SEARCH_ES_APPLICATION_UNBOUND")
		}
		if err = repository.SaveApplication(ctx, tx, physical, event.Document, actual); err != nil {
			return err
		}
	}
	// 后来事件已持久应用时使用实际高水位；重建到另一物理索引也不改写
	// 原事件第一次成功绑定的物理代次和权威摘要。
	if receipt.State != "SUCCEEDED" {
		if err = repository.Complete(ctx, tx, p.consumer, event.ID, physical, actual); err != nil {
			return err
		}
	}
	if tx.Commit() != nil {
		return errors.New("SEARCH_APPLICATION_COMMIT_UNKNOWN")
	}
````

## 第 10 步：Processor.apply（接着实现）

插入位置：第 9 步代码之后，紧接追加；完成稿约第 154 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=5458:5473 -->
````go
	return nil
}

````

## 第 11 步：safeCode

插入位置：第 10 步代码之后，紧接追加；完成稿约第 157 行。

本步声明：`var safeCode = regexp.MustCompile(\`^SEARCH_[A-Z0-9_]{1,56}$\`)`。

<!-- write-step: 11 bytes=5473:5536 -->
````go
var safeCode = regexp.MustCompile(`^SEARCH_[A-Z0-9_]{1,56}$`)

````

## 第 12 步：publicError

插入位置：第 11 步代码之后，紧接追加；完成稿约第 159 行。

本步声明：`func publicError(err error, fallback string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>err</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>在此声明中的实际使用：<code>err.Error</code>；<code>return err</code> |
| <code>fallback</code> | <code>string</code> | 在此声明中的实际使用：<code>errors.New(fallback)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(fallback)</code> |

**跟着本段代码走一遍：**

- 检查 <code>safeCode.MatchString(err.Error())</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return errors.New(fallback)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=5536:5677 -->
````go
func publicError(err error, fallback string) error {
	if safeCode.MatchString(err.Error()) {
		return err
	}
	return errors.New(fallback)
}

````

## 第 13 步：Processor.retry

插入位置：第 12 步代码之后，紧接追加；完成稿约第 166 行。

本步声明：`func (p *Processor) retry(event domain.Event, position repository.Position, cause error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>event</code> | <code>domain.Event</code> | 使用 <code>domain.Event</code> 类型；其定义处的职责：保存能够重新核验的原业务事件，而不是ES当前投影。<br>在此声明中的实际使用：<code>repository.Receive(ctx, tx, p.consumer, event, position)</code>；<code>event.ID</code> |
| <code>position</code> | <code>repository.Position</code> | 使用 <code>repository.Position</code> 类型；其定义处的职责：重新解析并核验准备写账本的Event原行及传输位置。<br>在此声明中的实际使用：<code>repository.Receive(ctx, tx, p.consumer, event, position)</code> |
| <code>cause</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>在此声明中的实际使用：<code>publicError(cause, &quot;SEARCH_INDEXING_RETRY_REQUIRED&quot;)</code> |

**跟着本段代码走一遍：**

- 执行 <code>context.WithTimeout(context.Background(), 2*time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>p.database.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil &#124;&#124; receipt.State == &quot;SUCCEEDED&quot; &#124;&#124; receipt.State == &quot;REJECTED&quot;</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>repository.SetRetry(ctx, tx, p.consumer, event.ID, code) == nil</code>。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 13 bytes=5677:6428 -->
````go
func (p *Processor) retry(event domain.Event, position repository.Position, cause error) {
	// 主任务取消后只做有界的失败记账。记账本身未知时仍保留APPLYING；
	// 既有SUCCEEDED不能被迟到的错误回写为RETRY。
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	tx, err := p.database.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer tx.Rollback()
	receipt, err := repository.Receive(ctx, tx, p.consumer, event, position)
	if err != nil || receipt.State == "SUCCEEDED" || receipt.State == "REJECTED" {
		return
	}
	code := publicError(cause, "SEARCH_INDEXING_RETRY_REQUIRED").Error()
	if repository.SetRetry(ctx, tx, p.consumer, event.ID, code) == nil {
		tx.Commit()
	}
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/search/internal/logic/indexing/processor.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/search/02-consumer-ledger.md)。
