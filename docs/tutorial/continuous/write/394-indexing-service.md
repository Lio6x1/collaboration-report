# 分步编写 `services/search/internal/logic/indexing/indexing_service.go`

先按 [S02 · 持久消费、日志身份与代次锁](../lessons/search/02-consumer-ledger.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/394-indexing-service.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

以整条Canal记录推进checkpoint，并提供重建快照seed的同事务应用。

一条消息可有多行，前几行成功不允许提交整条offset；快照seed没有event_id，不能伪造消费账本。

写之前先按这条执行路线理解各段的连接：

1. Processor.Record先ParseCanal全部行，再检查position<=SQL next
2. 逐事件Event成功后completeRecord核所有回执并提交SQL checkpoint
3. Seed独立事务调用SeedInTransaction
4. SeedInTransaction用快照项minimum/release查Doc当前权威，在调用方事务保存ES实际水位，方便与next_entry原子推进

本文件需要守住的边界：

- Kafka同步Commit由外层consumer完成
- seed不能写event_origin/consumed_event假事件
- ES成功而SQL未知时原快照项必须重试，不能跳进度

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/search/internal/logic/indexing' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/search/internal/logic/indexing/indexing_service.go') -and (Get-Item -LiteralPath 'services/search/internal/logic/indexing/indexing_service.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/search/internal/logic/indexing/indexing_service.go')) { New-Item -ItemType File -Path 'services/search/internal/logic/indexing/indexing_service.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:435 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package indexing

import (
	"collaboration/platform/rebuildmeta"
	domain "collaboration/search/internal/domain/event"
	index "collaboration/search/internal/domain/index"
	repository "collaboration/search/internal/repository/event"
	"context"
	"database/sql"
	"errors"
	"time"
)

// record.go：保留原实现，按业务合并文件。

````

## 第 2 步：Processor.Record

插入位置：第 1 步代码之后，紧接追加；完成稿约第 17 行。

Record把Kafka的一条记录作为提交单位。全部原行解析完成后才应用，
全部应用成功之后才提交SQL下一offset；实际Kafka同步提交属于外层消费者。

本步声明：`func (p *Processor) Record(ctx context.Context, physical string, position repository.Position, raw []byte) error`。

**这一段为什么这样写：** 将Kafka记录定义为提交offset的最小单位。

1. 先完整解析
2. 核无offset缺口
3. 逐行完成索引
4. 提交SQL下一offset


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 记录期限<br>在此声明中的实际使用：<code>ctx, cancel := context.WithTimeout(ctx, 30*time.Second)</code>；<code>context.WithTimeout(ctx, 30*time.Second)</code>；<code>p.database.BeginTx(ctx, nil)</code>；<code>repository.Checkpoint(ctx, tx, p.consumer, position.Topic, position.Partition)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>physical</code> | <code>string</code> | 已固定的物理索引<br>在此声明中的实际使用：<code>p.Event(ctx, physical, event, position)</code> |
| <code>position</code> | <code>repository.Position</code> | 整条消息位置<br>在此声明中的实际使用：<code>position.Topic</code>；<code>position.Partition</code>；<code>position.Offset</code>；<code>p.Event(ctx, physical, event, position)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 未经改写Canal原字节<br>在此声明中的实际使用：<code>domain.ParseCanal(raw)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | nil说明SQL整条完成，仍待外层broker确认<br>实际返回表达式：<code>errors.New(&quot;SEARCH_INDEXING_DEPENDENCIES_UNAVAILABLE&quot;)</code>；<code>err</code>；<code>errors.New(&quot;SEARCH_CHECKPOINT_TRANSACTION_UNAVAILABLE&quot;)</code>；<code>errors.New(&quot;SEARCH_OFFSET_GAP&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>p.database == nil &#124;&#124; p.source == nil &#124;&#124; p.sink == nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_INDEXING_DEPENDENCIES_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 执行 <code>context.WithTimeout(ctx, 30*time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>p.database.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_CHECKPOINT_TRANSACTION_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>position.Offset &lt; 0 &#124;&#124; position.Offset &gt; next</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_OFFSET_GAP&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return p.completeRecord(ctx, position, events)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=435:1550 -->
````go
// Record把Kafka的一条记录作为提交单位。全部原行解析完成后才应用，
// 全部应用成功之后才提交SQL下一offset；实际Kafka同步提交属于外层消费者。
func (p *Processor) Record(ctx context.Context, physical string, position repository.Position, raw []byte) error {
	if p.database == nil || p.source == nil || p.sink == nil {
		return errors.New("SEARCH_INDEXING_DEPENDENCIES_UNAVAILABLE")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	events, err := domain.ParseCanal(raw)
	if err != nil {
		return err
	}
	tx, err := p.database.BeginTx(ctx, nil)
	if err != nil {
		return errors.New("SEARCH_CHECKPOINT_TRANSACTION_UNAVAILABLE")
	}
	next, err := repository.Checkpoint(ctx, tx, p.consumer, position.Topic, position.Partition)
	tx.Rollback()
	if err != nil {
		return err
	}
	if position.Offset < 0 || position.Offset > next {
		return errors.New("SEARCH_OFFSET_GAP")
	}
	for _, event := range events {
		if err := p.Event(ctx, physical, event, position); err != nil {
			return err
		}
	}
	return p.completeRecord(ctx, position, events)
}

````

## 第 3 步：Processor.completeRecord

插入位置：第 2 步代码之后，紧接追加；完成稿约第 49 行。

本步声明：`func (p *Processor) completeRecord(ctx context.Context, position repository.Position, events []domain.Event) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>p.database.BeginTx(ctx, nil)</code>；<code>repository.AdvanceCheckpoint(ctx, tx, p.consumer, position, events)</code> |
| <code>position</code> | <code>repository.Position</code> | 使用 <code>repository.Position</code> 类型；其定义处的职责：重新解析并核验准备写账本的Event原行及传输位置。<br>在此声明中的实际使用：<code>repository.AdvanceCheckpoint(ctx, tx, p.consumer, position, events)</code> |
| <code>events</code> | <code>&#91;&#93;domain.Event</code> | 使用 <code>domain.Event</code> 类型；其定义处的职责：保存能够重新核验的原业务事件，而不是ES当前投影。<br>在此声明中的实际使用：<code>repository.AdvanceCheckpoint(ctx, tx, p.consumer, position, events)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_CHECKPOINT_TRANSACTION_UNAVAILABLE&quot;)</code>；<code>err</code>；<code>errors.New(&quot;SEARCH_CHECKPOINT_COMMIT_UNKNOWN&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>p.database.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_CHECKPOINT_TRANSACTION_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>tx.Commit() != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_CHECKPOINT_COMMIT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=1550:2079 -->
````go
func (p *Processor) completeRecord(ctx context.Context, position repository.Position, events []domain.Event) error {
	tx, err := p.database.BeginTx(ctx, nil)
	if err != nil {
		return errors.New("SEARCH_CHECKPOINT_TRANSACTION_UNAVAILABLE")
	}
	defer tx.Rollback()
	if err := repository.AdvanceCheckpoint(ctx, tx, p.consumer, position, events); err != nil {
		return err
	}
	if tx.Commit() != nil {
		return errors.New("SEARCH_CHECKPOINT_COMMIT_UNKNOWN")
	}
	return nil
}

// seed.go：保留原实现，按业务合并文件。

````

## 第 4 步：Processor.Seed

插入位置：第 3 步代码之后，紧接追加；完成稿约第 66 行。

Seed用于独立应用一个快照条目；重建任务需要原子推进进度时应使用SeedInTransaction。
快照条目没有原事件身份，不能伪造event_origin或consumed_event成功回执。

本步声明：`func (p *Processor) Seed(ctx context.Context, physical string, entry rebuildmeta.Entry) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx, cancel := context.WithTimeout(ctx, 30*time.Second)</code>；<code>context.WithTimeout(ctx, 30*time.Second)</code>；<code>p.database.BeginTx(ctx, nil)</code>；<code>p.SeedInTransaction(ctx, tx, physical, entry)</code> |
| <code>physical</code> | <code>string</code> | 在此声明中的实际使用：<code>p.SeedInTransaction(ctx, tx, physical, entry)</code> |
| <code>entry</code> | <code>rebuildmeta.Entry</code> | 使用 <code>rebuildmeta.Entry</code> 类型；其定义处的职责：流式编码和校验Doc交给Search的重建元数据行。<br>在此声明中的实际使用：<code>p.SeedInTransaction(ctx, tx, physical, entry)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_INDEXING_DEPENDENCIES_UNAVAILABLE&quot;)</code>；<code>errors.New(&quot;SEARCH_APPLICATION_TRANSACTION_UNAVAILABLE&quot;)</code>；<code>err</code>；<code>errors.New(&quot;SEARCH_APPLICATION_COMMIT_UNKNOWN&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>p.database == nil &#124;&#124; p.source == nil &#124;&#124; p.sink == nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_INDEXING_DEPENDENCIES_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 执行 <code>context.WithTimeout(ctx, 30*time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>p.database.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_APPLICATION_TRANSACTION_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>tx.Commit() != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_APPLICATION_COMMIT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=2079:2920 -->
````go
// Seed用于独立应用一个快照条目；重建任务需要原子推进进度时应使用SeedInTransaction。
// 快照条目没有原事件身份，不能伪造event_origin或consumed_event成功回执。
func (p *Processor) Seed(ctx context.Context, physical string, entry rebuildmeta.Entry) error {
	if p.database == nil || p.source == nil || p.sink == nil {
		return errors.New("SEARCH_INDEXING_DEPENDENCIES_UNAVAILABLE")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	tx, err := p.database.BeginTx(ctx, nil)
	if err != nil {
		return errors.New("SEARCH_APPLICATION_TRANSACTION_UNAVAILABLE")
	}
	defer tx.Rollback()
	if err = p.SeedInTransaction(ctx, tx, physical, entry); err != nil {
		return err
	}
	if tx.Commit() != nil {
		return errors.New("SEARCH_APPLICATION_COMMIT_UNKNOWN")
	}
	return nil
}

````

## 第 5 步：Processor.SeedInTransaction

插入位置：第 4 步代码之后，紧接追加；完成稿约第 88 行。

SeedInTransaction沿用调用方的代次、任务锁和事务，使应用水位与快照进度一起提交。
此处不提交事务；ES成功但SQL提交未知时，调用方必须保留原条目重试，不能跳过进度。

本步声明：`func (p *Processor) SeedInTransaction(ctx context.Context, tx *sql.Tx, physical string, entry rebuildmeta.Entry) error`。

**这一段为什么这样写：** 在重建任务事务内应用快照项，以便水位与快照进度同提交。

1. 从Doc读当前权威
2. 锁目标应用水位
3. ES单调更新后保存实际State
4. 不COMMIT，交给任务方


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 重建步骤有界期限<br>在此声明中的实际使用：<code>ctx, cancel := context.WithTimeout(ctx, 30*time.Second)</code>；<code>context.WithTimeout(ctx, 30*time.Second)</code>；<code>p.source.Current(ctx, domain.Event{Document: entry.Document, Sequence: entry.Minimum, Release: entry.Release})</code>；<code>repository.LockApplication(ctx, tx, physical, entry.Document)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 持代次/任务锁的外层事务<br>在此声明中的实际使用：<code>tx == nil</code>；<code>repository.LockApplication(ctx, tx, physical, entry.Document)</code>；<code>repository.SaveApplication(ctx, tx, physical, entry.Document, actual)</code> |
| <code>physical</code> | <code>string</code> | 新建目标物理索引<br>在此声明中的实际使用：<code>repository.LockApplication(ctx, tx, physical, entry.Document)</code>；<code>p.sink.Apply(ctx, physical, authority)</code>；<code>repository.SaveApplication(ctx, tx, physical, entry.Document, actual)</code> |
| <code>entry</code> | <code>rebuildmeta.Entry</code> | 快照目录document/minimum/release引用<br>在此声明中的实际使用：<code>entry.Document</code>；<code>entry.Minimum</code>；<code>entry.Release</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 本项应用错误，外层不能推进next_entry<br>实际返回表达式：<code>errors.New(&quot;SEARCH_INDEXING_DEPENDENCIES_UNAVAILABLE&quot;)</code>；<code>publicError(err, &quot;SEARCH_DOC_SNAPSHOT_UNKNOWN&quot;)</code>；<code>errors.New(&quot;SEARCH_CURRENT_AUTHORITY_UNBOUND&quot;)</code>；<code>err</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>tx == nil &#124;&#124; p.source == nil &#124;&#124; p.sink == nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_INDEXING_DEPENDENCIES_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 执行 <code>context.WithTimeout(ctx, 30*time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return publicError(err, &quot;SEARCH_DOC_SNAPSHOT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>entry.Document == 0 &#124;&#124; authority.Document != entry.Document &#124;&#124; authority.Sequence &lt; entry.Minimum</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_CURRENT_AUTHORITY_UNBOUND&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !apply</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return publicError(err, &quot;SEARCH_ES_APPLICATION_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>actual.Sequence &lt; authority.Sequence &#124;&#124; 		(actual.Sequence == authority.Sequence &amp;&amp; (actual.Digest != authority.Digest &#124;&#124; 			actual.Visible != authority.Visible &#124;&#124; actual.Release ! …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return errors.New(&quot;SEARCH_ES_APPLICATION_UNBOUND&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return repository.SaveApplication(ctx, tx, physical, entry.Document, actual)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=2920:4788 -->
````go
// SeedInTransaction沿用调用方的代次、任务锁和事务，使应用水位与快照进度一起提交。
// 此处不提交事务；ES成功但SQL提交未知时，调用方必须保留原条目重试，不能跳过进度。
func (p *Processor) SeedInTransaction(ctx context.Context, tx *sql.Tx, physical string, entry rebuildmeta.Entry) error {
	if tx == nil || p.source == nil || p.sink == nil {
		return errors.New("SEARCH_INDEXING_DEPENDENCIES_UNAVAILABLE")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	// Event仅复用快照查询参数；Minimum可以为零，未发布文档的隐藏权威同样要落盘。
	authority, err := p.source.Current(ctx, domain.Event{Document: entry.Document, Sequence: entry.Minimum, Release: entry.Release})
	if err != nil {
		return publicError(err, "SEARCH_DOC_SNAPSHOT_UNKNOWN")
	}
	if entry.Document == 0 || authority.Document != entry.Document || authority.Sequence < entry.Minimum {
		return errors.New("SEARCH_CURRENT_AUTHORITY_UNBOUND")
	}
	actual, err := repository.LockApplication(ctx, tx, physical, entry.Document)
	if err != nil {
		return err
	}
	apply, err := index.ShouldApply(actual, authority)
	if err != nil || !apply {
		return err
	}
	actual, err = p.sink.Apply(ctx, physical, authority)
	if err != nil {
		return publicError(err, "SEARCH_ES_APPLICATION_UNKNOWN")
	}
	if actual.Sequence < authority.Sequence ||
		(actual.Sequence == authority.Sequence && (actual.Digest != authority.Digest ||
			actual.Visible != authority.Visible || actual.Release != authority.Release || actual.Policy != authority.Policy)) {
		return errors.New("SEARCH_ES_APPLICATION_UNBOUND")
	}
	// SaveApplication再次校验ES返回的实际高水位，允许并发后继权威，禁止回退或同序换正文。
	return repository.SaveApplication(ctx, tx, physical, entry.Document, actual)
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/search/internal/logic/indexing/indexing_service.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/search/02-consumer-ledger.md)。
