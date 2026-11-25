# 分步编写 `services/search/internal/logic/consumer/consumer.go`

先按 [S02 · 持久消费、日志身份与代次锁](../lessons/search/02-consumer-ledger.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/392-consumer.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

维护唯一在途Kafka记录，按投影/SQL完成后再提交broker并判定恢复就绪。

失败后换下一条记录会丢事件，启动就绪还必须赶上启动时冻结的日志前沿。

写之前先按这条执行路线理解各段的连接：

1. Step互斥，初始化核代次、恢复账本与窗口并记录startup frontier
2. 只在current为空时Fetch，之后复核broker窗口与SQL next
3. generation.Do内Processor.Record完成SQL，再broker.Commit
4. 两步都确认才current=nil；错误保留原记录
5. Ready核最近失败、真实窗口、alias代次及SQL已追到启动frontier

本文件需要守住的边界：

- broker Committed不得领先SQL next；SQL next不能落在保留窗口外
- 空闲Fetch取消不代表业务失败，可转inspect核就绪
- frontier是启动时末尾，不是每次追随不断增长末尾

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/search/internal/logic/consumer' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/search/internal/logic/consumer/consumer.go') -and (Get-Item -LiteralPath 'services/search/internal/logic/consumer/consumer.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/search/internal/logic/consumer/consumer.go')) { New-Item -ItemType File -Path 'services/search/internal/logic/consumer/consumer.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:263 -->
````go
package consumer

import (
	domain "collaboration/search/internal/domain/event"
	repository "collaboration/search/internal/repository/event"
	generation "collaboration/search/internal/repository/generation"
	"context"
	"database/sql"
	"errors"
	"sync"
	"time"
)

````

## 第 2 步：Broker

插入位置：第 1 步代码之后，紧接追加；完成稿约第 14 行。

Broker仅暴露消费窗口、单条读取与确认；业务不依赖具体Kafka客户端。

本步声明：`type Broker interface {`。

**这一段为什么这样写：** 消费组传输需要的最小能力。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Window</code> | <code>(context.Context) (map&#91;int&#93;domain.PartitionWindow, error)</code> | ctx→完整三分区First/Last/Committed窗口或错误 |
| <code>Fetch</code> | <code>(context.Context) (domain.Record, error)</code> | ctx→唯一在途原Record，失败不得越过原位置 |
| <code>Commit</code> | <code>(context.Context, domain.Record) error</code> | ctx、原Record→同步提交结果；仅在SQL整条完成后调用 |

<!-- write-step: 2 bytes=263:544 -->
````go
// Broker仅暴露消费窗口、单条读取与确认；业务不依赖具体Kafka客户端。
type Broker interface {
	Window(context.Context) (map[int]domain.PartitionWindow, error)
	Fetch(context.Context) (domain.Record, error)
	Commit(context.Context, domain.Record) error
}

````

## 第 3 步：Generations

插入位置：第 2 步代码之后，紧接追加；完成稿约第 21 行。

Generations把一次投影及其确认绑定到同一物理索引，避免重建切换时写错代次。

本步声明：`type Generations interface {`。

**这一段为什么这样写：** 整条记录目标代次的锁边界。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Initialize</code> | <code>(context.Context) error</code> | ctx→核/建ES及SQL初代并恢复一致性 |
| <code>Ready</code> | <code>(context.Context) error</code> | ctx→核当前代次和别名可用 |
| <code>Do</code> | <code>(context.Context, func(context.Context, generation.Target) error) error</code> | ctx、接收有界ctx与Target的回调→错误；回调期间持共享代次锁 |

<!-- write-step: 3 bytes=544:824 -->
````go
// Generations把一次投影及其确认绑定到同一物理索引，避免重建切换时写错代次。
type Generations interface {
	Initialize(context.Context) error
	Ready(context.Context) error
	Do(context.Context, func(context.Context, generation.Target) error) error
}

````

## 第 4 步：Indexer

插入位置：第 3 步代码之后，紧接追加；完成稿约第 28 行。

Indexer按消息位置及内容记录投影，内部维护索引写入与SQL回执的可恢复顺序。

本步声明：`type Indexer interface {`。

**这一段为什么这样写：** Consumer只委托完整记录，不处理单行完成状态。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Record</code> | <code>(context.Context, string, repository.Position, &#91;&#93;byte) error</code> | ctx、physical、Position、原Value字节→错误；nil证明所有行及SQL checkpoint已完成 |

<!-- write-step: 4 bytes=824:1027 -->
````go
// Indexer按消息位置及内容记录投影，内部维护索引写入与SQL回执的可恢复顺序。
type Indexer interface {
	Record(context.Context, string, repository.Position, []byte) error
}

````

## 第 5 步：Consumer

插入位置：第 4 步代码之后，紧接追加；完成稿约第 33 行。

Consumer持有当前未确认消息及恢复状态；Step由互斥锁串行执行，Ready可并发读取。

本步声明：`type Consumer struct {`。

**这一段为什么这样写：** 一个消费循环的恢复状态。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>database</code> | <code>*sql.DB</code> | Search私有库 |
| <code>broker</code> | <code>Broker</code> | 日志身份已绑定的组传输 |
| <code>generation</code> | <code>Generations</code> | 代次共享锁执行器 |
| <code>indexer</code> | <code>Indexer</code> | 完整记录索引用例 |
| <code>group</code> | <code>string</code> | 消费者账本命名空间 |
| <code>topic</code> | <code>string</code> | 固定日志名称 |
| <code>stepMu</code> | <code>sync.Mutex</code> | 串行Step，保障仅一条current |
| <code>current</code> | <code>*domain.Record</code> | 失败时保留的原记录 |
| <code>stateMu</code> | <code>sync.RWMutex</code> | 保护Ready并发读取恢复状态 |
| <code>initialized</code> | <code>bool</code> | 启动账本/窗口核验是否完成 |
| <code>frontier</code> | <code>map&#91;int&#93;int64</code> | 初始化观察的各分区排他末尾 |
| <code>failure</code> | <code>error</code> | 最近一次Step错误，使Ready保守失败 |

<!-- write-step: 5 bytes=1027:1424 -->
````go
// Consumer持有当前未确认消息及恢复状态；Step由互斥锁串行执行，Ready可并发读取。
type Consumer struct {
	database     *sql.DB
	broker       Broker
	generation   Generations
	indexer      Indexer
	group, topic string
	stepMu       sync.Mutex
	current      *domain.Record
	stateMu      sync.RWMutex
	initialized  bool
	frontier     map[int]int64
	failure      error
}

````

## 第 6 步：New

插入位置：第 5 步代码之后，紧接追加；完成稿约第 48 行。

本步声明：`func New(database *sql.DB, broker Broker, generations Generations, indexer Indexer, group, topic string) *Consumer`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>database</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>database: database</code> |
| <code>broker</code> | <code>Broker</code> | 使用 <code>Broker</code> 类型；其定义处的职责：消费组传输需要的最小能力。<br>在此声明中的实际使用：<code>broker: broker</code> |
| <code>generations</code> | <code>Generations</code> | 使用 <code>Generations</code> 类型；其定义处的职责：整条记录目标代次的锁边界。<br>在此声明中的实际使用：<code>generation: generations</code> |
| <code>indexer</code> | <code>Indexer</code> | 使用 <code>Indexer</code> 类型；其定义处的职责：Consumer只委托完整记录，不处理单行完成状态。<br>在此声明中的实际使用：<code>indexer: indexer</code> |
| <code>group</code> | <code>string</code> | 在此声明中的实际使用：<code>group: group</code> |
| <code>topic</code> | <code>string</code> | 在此声明中的实际使用：<code>topic: topic</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Consumer</code> | 使用 <code>Consumer</code> 类型；其定义处的职责：一个消费循环的恢复状态。<br>实际返回表达式：<code>&amp;Consumer{database: database, broker: broker, generation: generations, indexer: indexer, group: group, topic: topic}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Consumer{database: database, broker: broker, generation: generations, indexer: indexer, group: group, topic: topic}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1424:1669 -->
````go
func New(database *sql.DB, broker Broker, generations Generations, indexer Indexer, group, topic string) *Consumer {
	return &Consumer{database: database, broker: broker, generation: generations, indexer: indexer, group: group, topic: topic}
}

````

## 第 7 步：Consumer.Step

插入位置：第 6 步代码之后，紧接追加；完成稿约第 52 行。

Step串行拥有一条在途记录。任何失败都留下原current，下一次Step重做原记录，
不Fetch后面的记录，也不根据Reader预取的最高offset批量提交。

本步声明：`func (c *Consumer) Step(ctx context.Context) (outcome error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>c.initialize(ctx)</code>；<code>context.WithTimeout(ctx, 2*time.Second)</code>；<code>ctx.Err</code>；<code>c.inspect(ctx)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>outcome</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>c.inspect(ctx)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>c.stepMu.Lock()</code>。
- 登记 <code>c.stepMu.Unlock()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>c.stepMu.Unlock()</code>。
- 登记 <code>func() { c.stateMu.Lock(); c.failure = outcome; c.stateMu.Unlock() }()</code>，当前函数退出时执行，提前 return 也会触发。
- 在本段的匿名函数内：执行 <code>c.stateMu.Lock()</code>。
- 在本段的匿名函数内：执行 <code>c.stateMu.Unlock()</code>。
- 检查 <code>!c.initialized</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>c.current == nil</code>。
- 执行 <code>context.WithTimeout(ctx, 2*time.Second)</code>。
- 检查 <code>err != nil</code>。
- 检查 <code>ctx.Err() == nil &amp;&amp; err.Error() == &quot;SEARCH_KAFKA_FETCH_CANCELED&quot;</code>。条件成立时执行 <code>return c.inspect(ctx)</code>。当前函数在这里结束。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return c.broker.Commit(ctx, *c.current)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：执行 <code>c.broker.Commit(ctx, *c.current)</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。

<!-- write-step: 7 bytes=1669:3356 -->
````go
// Step串行拥有一条在途记录。任何失败都留下原current，下一次Step重做原记录，
// 不Fetch后面的记录，也不根据Reader预取的最高offset批量提交。
func (c *Consumer) Step(ctx context.Context) (outcome error) {
	c.stepMu.Lock()
	defer c.stepMu.Unlock()
	defer func() { c.stateMu.Lock(); c.failure = outcome; c.stateMu.Unlock() }()
	if !c.initialized {
		if err := c.initialize(ctx); err != nil {
			return err
		}
	}
	if c.current == nil {
		poll, done := context.WithTimeout(ctx, 2*time.Second)
		record, err := c.broker.Fetch(poll)
		done()
		if err != nil {
			if ctx.Err() == nil && err.Error() == "SEARCH_KAFKA_FETCH_CANCELED" {
				return c.inspect(ctx)
			}
			return err
		}
		c.current = &record
	}
	window, err := c.broker.Window(ctx)
	if err != nil {
		return err
	}
	positions, err := c.checkpoints(ctx, false)
	if err != nil {
		return err
	}
	if err := validateWindow(window, positions); err != nil {
		return err
	}
	// 本条消息绑定当前物理索引代次：索引投影、SQL回执和Kafka提交是独立步骤。
	// Record内部按原序号/摘要允许已成功部分重放，不能把Kafka提交当跨系统事务。
	err = c.generation.Do(ctx, func(ctx context.Context, target generation.Target) error {
		if err := c.indexer.Record(ctx, target.Physical, c.current.Position, c.current.Value); err != nil {
			return err
		}
		// SQL整条记录已成功后才能同步Kafka；这个独立动作超时仍保留原记录重试。
		return c.broker.Commit(ctx, *c.current)
	})
	if err != nil {
		return err
	}
	// 两个步骤都确认成功才丢掉原记录；否则下一轮继续原消息，不越过失败位置。
````

## 第 8 步：Consumer.Step（接着实现）

插入位置：第 7 步代码之后，紧接追加；完成稿约第 99 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=3356:3388 -->
````go
	c.current = nil
	return nil
}

````

## 第 9 步：Consumer.initialize

插入位置：第 8 步代码之后，紧接追加；完成稿约第 103 行。

本步声明：`func (c *Consumer) initialize(ctx context.Context) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx, done := context.WithTimeout(ctx, 10*time.Second)</code>；<code>context.WithTimeout(ctx, 10*time.Second)</code>；<code>c.generation.Initialize(ctx)</code>；<code>c.broker.Window(ctx)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>context.WithTimeout(ctx, 10*time.Second)</code>。
- 登记 <code>done()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>c.stateMu.Lock()</code>。
- 执行 <code>c.stateMu.Unlock()</code>。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=3388:4020 -->
````go
func (c *Consumer) initialize(ctx context.Context) error {
	ctx, done := context.WithTimeout(ctx, 10*time.Second)
	defer done()
	if err := c.generation.Initialize(ctx); err != nil {
		return err
	}
	window, err := c.broker.Window(ctx)
	if err != nil {
		return err
	}
	positions, err := c.checkpoints(ctx, true)
	if err != nil {
		return err
	}
	if err := validateWindow(window, positions); err != nil {
		return err
	}
	frontier := make(map[int]int64, 3)
	for partition, value := range window {
		frontier[partition] = value.Last
	}
	c.stateMu.Lock()
	c.frontier = frontier
	c.initialized = true
	c.stateMu.Unlock()
	return nil
}

````

## 第 10 步：Consumer.Ready

插入位置：第 9 步代码之后，紧接追加；完成稿约第 131 行。

Ready同时检查初始化、最近失败、消息保留窗口与启动时的消费水位。
进程存活或Kafka连接成功不足以证明索引已完成恢复，失败时不能报告可查询。

本步声明：`func (c *Consumer) Ready(ctx context.Context) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>c.inspect(ctx)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_CONSUMER_RECOVERY_REQUIRED&quot;)</code>；<code>c.inspect(ctx)</code> |

**跟着本段代码走一遍：**

- 检查 <code>!initialized</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_CONSUMER_RECOVERY_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>failure != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_CONSUMER_RECOVERY_REQUIRED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return c.inspect(ctx)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=4020:4554 -->
````go
// Ready同时检查初始化、最近失败、消息保留窗口与启动时的消费水位。
// 进程存活或Kafka连接成功不足以证明索引已完成恢复，失败时不能报告可查询。
func (c *Consumer) Ready(ctx context.Context) error {
	c.stateMu.RLock()
	initialized, failure := c.initialized, c.failure
	c.stateMu.RUnlock()
	if !initialized {
		return errors.New("SEARCH_CONSUMER_RECOVERY_REQUIRED")
	}
	if failure != nil {
		return errors.New("SEARCH_CONSUMER_RECOVERY_REQUIRED")
	}
	return c.inspect(ctx)
}

````

## 第 11 步：Consumer.inspect

插入位置：第 10 步代码之后，紧接追加；完成稿约第 146 行。

本步声明：`func (c *Consumer) inspect(ctx context.Context) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>c.generation.Ready(ctx)</code>；<code>c.broker.Window(ctx)</code>；<code>c.checkpoints(ctx, false)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;SEARCH_CONSUMER_CATCHUP_REQUIRED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>c.stateMu.RUnlock()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>positions&#91;partition&#93; &lt; offset</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_CONSUMER_CATCHUP_REQUIRED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=4554:5105 -->
````go
func (c *Consumer) inspect(ctx context.Context) error {
	if err := c.generation.Ready(ctx); err != nil {
		return err
	}
	window, err := c.broker.Window(ctx)
	if err != nil {
		return err
	}
	positions, err := c.checkpoints(ctx, false)
	if err != nil {
		return err
	}
	if err := validateWindow(window, positions); err != nil {
		return err
	}
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	for partition, offset := range c.frontier {
		if positions[partition] < offset {
			return errors.New("SEARCH_CONSUMER_CATCHUP_REQUIRED")
		}
	}
	return nil
}

````

## 第 12 步：Consumer.checkpoints

插入位置：第 11 步代码之后，紧接追加；完成稿约第 171 行。

本步声明：`func (c *Consumer) checkpoints(ctx context.Context, audit bool) (map[int]int64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>c.database.BeginTx(ctx, nil)</code>；<code>repository.CheckRecovery(ctx, tx, c.group, c.topic)</code>；<code>repository.Checkpoint(ctx, tx, c.group, c.topic, partition)</code> |
| <code>audit</code> | <code>bool</code> | true先在同一事务检查消费恢复记录与检查点一致性，再读取全部分区下一位置；正常逐条推进时可传false，不能据此省略启动恢复审计。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>map&#91;int&#93;int64</code> | 实际返回表达式：<code>nil</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_CHECKPOINT_TRANSACTION_UNAVAILABLE&quot;)</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>c.database.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_CHECKPOINT_TRANSACTION_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>audit</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=5105:5735 -->
````go
func (c *Consumer) checkpoints(ctx context.Context, audit bool) (map[int]int64, error) {
	tx, err := c.database.BeginTx(ctx, nil)
	if err != nil {
		return nil, errors.New("SEARCH_CHECKPOINT_TRANSACTION_UNAVAILABLE")
	}
	defer tx.Rollback()
	if audit {
		if err := repository.CheckRecovery(ctx, tx, c.group, c.topic); err != nil {
			return nil, err
		}
	}
	result := make(map[int]int64, 3)
	for partition := 0; partition < domain.PartitionCount; partition++ {
		next, err := repository.Checkpoint(ctx, tx, c.group, c.topic, partition)
		if err != nil {
			return nil, err
		}
		result[partition] = next
	}
	return result, nil
}

````

## 第 13 步：validateWindow

插入位置：第 12 步代码之后，紧接追加；完成稿约第 193 行。

本步声明：`func validateWindow(window map[int]domain.PartitionWindow, positions map[int]int64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>window</code> | <code>map&#91;int&#93;domain.PartitionWindow</code> | 在此声明中的实际使用：<code>len(window)</code>；<code>value, exists := window&#91;partition&#93;</code> |
| <code>positions</code> | <code>map&#91;int&#93;int64</code> | 在此声明中的实际使用：<code>len(positions)</code>；<code>next, present := positions&#91;partition&#93;</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_KAFKA_PARTITIONS_UNBOUND&quot;)</code>；<code>errors.New(&quot;SEARCH_KAFKA_WINDOW_LOST&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(window) != domain.PartitionCount &#124;&#124; len(positions) != domain.PartitionCount</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_KAFKA_PARTITIONS_UNBOUND&quot;)</code>。当前函数在这里结束。
- 检查 <code>!exists &#124;&#124; !present &#124;&#124; next &lt; 0 &#124;&#124; value.First &lt; 0 &#124;&#124; value.Last &lt; value.First &#124;&#124; next &lt; value.First &#124;&#124; 			next &gt; value.Last &#124;&#124; value.Committed &lt; -1 &#124;&#124; value.Committed &gt; next &#124;&#124; 		 …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return errors.New(&quot;SEARCH_KAFKA_WINDOW_LOST&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=5735:6432 -->
````go
func validateWindow(window map[int]domain.PartitionWindow, positions map[int]int64) error {
	if len(window) != domain.PartitionCount || len(positions) != domain.PartitionCount {
		return errors.New("SEARCH_KAFKA_PARTITIONS_UNBOUND")
	}
	for partition := 0; partition < domain.PartitionCount; partition++ {
		value, exists := window[partition]
		next, present := positions[partition]
		if !exists || !present || next < 0 || value.First < 0 || value.Last < value.First || next < value.First ||
			next > value.Last || value.Committed < -1 || value.Committed > next ||
			(value.Committed >= 0 && value.Committed < value.First) {
			return errors.New("SEARCH_KAFKA_WINDOW_LOST")
		}
	}
	return nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/search/internal/logic/consumer/consumer.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/search/02-consumer-ledger.md)。
