# 分步编写 `services/search/internal/logic/rebuild/completion_service.go`

先按 [S05 · 固定窗口重放、完整核验与别名恢复切换](../lessons/search/05-cutover-wiring.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/396-completion-service.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

按原日志窗口逐条重放，并把已验证切换事实封装为可重复读取的固定成功结果。

跨分区没有总顺序；任务成功也不能只依赖state字符串，需要原指纹、H、验证摘要与代次共同构成不可变证明。

写之前先按这条执行路线理解各段的连接：

1. Replayer.Step 在锁外获取原Kafka身份的窗口，使用任务专用SQL next，单步最多32条记录；SWITCHING后只补到原H。
2. apply 重获原job锁并重验next；旧批次已经完成则返回，跳号则失败，Processor独立完成应用/事件/整记录事务。
3. fixedRebuildResult生成含原号、job、目标代次、终态修订和验证摘要的固定JSON，并计算结果摘要。
4. finalizeRebuild核原指纹及三分区next=H，CompleteSwitch与终态封装共同提交；SUCCEEDED查询从原封装恢复。

本文件需要守住的边界：

- 锁住job直到Record返回，不代表ES与SQL变成原子事务；未知结果仍按原记录账本恢复。
- 不能用当前alias或后来代次重新计算旧操作的成功答复。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/search/internal/logic/rebuild' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/search/internal/logic/rebuild/completion_service.go') -and (Get-Item -LiteralPath 'services/search/internal/logic/rebuild/completion_service.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/search/internal/logic/rebuild/completion_service.go')) { New-Item -ItemType File -Path 'services/search/internal/logic/rebuild/completion_service.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:601 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package rebuild

import (
	"bytes"
	pb "collaboration/contracts/internalv1"
	"collaboration/platform/rebuildmeta"
	"collaboration/search/internal/client/logsource"
	domain "collaboration/search/internal/domain/event"
	event "collaboration/search/internal/repository/event"
	repository "collaboration/search/internal/repository/rebuild"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"time"
)

// replay.go：保留原实现，按业务合并文件。

````

## 第 2 步：ReplayLogs

插入位置：第 1 步代码之后，紧接追加；完成稿约第 24 行。

本步声明：`type ReplayLogs interface {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Logs</code> | <code>Logs</code> | 复用准备阶段获取原Kafka身份和窗口的能力。 |
| <code>Read</code> | <code>(context.Context, logsource.Window, int, int64, int64) (&#91;&#93;domain.Record, int64, error)</code> | ctx控制取消；Window固定原集群/主题身份与L；int为分区，两个int64为next与有限终点；返回连续原记录及读取位置，错误不允许推进SQL。 |

<!-- write-step: 2 bytes=601:730 -->
````go
type ReplayLogs interface {
	Logs
	Read(context.Context, logsource.Window, int, int64, int64) ([]domain.Record, int64, error)
}

````

## 第 3 步：ReplayProcessor

插入位置：第 2 步代码之后，紧接追加；完成稿约第 29 行。

本步声明：`type ReplayProcessor interface {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Record</code> | <code>(context.Context, string, event.Position, &#91;&#93;byte) error</code> | ctx、目标物理索引、原主题分区offset和原消息字节；完整记录确认后才能推进专用SQL检查点。 |

<!-- write-step: 3 bytes=730:829 -->
````go
type ReplayProcessor interface {
	Record(context.Context, string, event.Position, []byte) error
}

````

## 第 4 步：Replayer

插入位置：第 3 步代码之后，紧接追加；完成稿约第 33 行。

本步声明：`type Replayer struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Seed</code> | <code>*Seeder</code> | 复用原任务锁序和导入绑定核验。 |
| <code>Logs</code> | <code>ReplayLogs</code> | 能捕获窗口并按原身份读取有限日志的客户端。 |
| <code>Processor</code> | <code>func(consumer string) ReplayProcessor</code> | 以专用consumer名创建处理器，隔离普通消费进度。 |

<!-- write-step: 4 bytes=829:945 -->
````go
type Replayer struct {
	Seed      *Seeder
	Logs      ReplayLogs
	Processor func(consumer string) ReplayProcessor
}

````

## 第 5 步：Replayer.Step

插入位置：第 4 步代码之后，紧接追加；完成稿约第 39 行。

Step只推进独立任务的SQL next；当前观察到的末端不是切换H，也不改变普通消费组。
每步最多32条完整记录，跨分区没有全局顺序，分区内保留broker原始次序和原字节。

本步声明：`func (r *Replayer) Step(ctx context.Context) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx, cancel := context.WithTimeout(ctx, 30*time.Second)</code>；<code>context.WithTimeout(ctx, 30*time.Second)</code>；<code>repository.PendingReplay(ctx, r.Seed.DB)</code>；<code>r.Seed.DB.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code><br>另有 6 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_REBUILD_DEPENDENCIES_UNAVAILABLE&quot;)</code>；<code>err</code>；<code>errors.New(&quot;SEARCH_REBUILD_TRANSACTION_UNAVAILABLE&quot;)</code>；<code>nil</code><br>另有 6 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>r == nil &#124;&#124; r.Seed == nil &#124;&#124; r.Seed.DB == nil &#124;&#124; r.Logs == nil &#124;&#124; r.Processor == nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_DEPENDENCIES_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 执行 <code>context.WithTimeout(ctx, 30*time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil &#124;&#124; operation == &quot;&quot;</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>r.Seed.DB.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_TRANSACTION_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>err == nil &amp;&amp; input.job.State == &quot;SWITCHING&quot;</code>。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>input.job.State != &quot;REPLAYING&quot; &amp;&amp; input.job.State != &quot;SWITCHING&quot;</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; len(uuid) != len(original.Identity.TopicID)</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_LOG_IDENTITY_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; starts&#91;partition&#93; &lt; 0</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_LOG_WINDOW_INVALID&quot;)</code>。当前函数在这里结束。

<!-- write-step: 5 bytes=945:2995 -->
````go
// Step只推进独立任务的SQL next；当前观察到的末端不是切换H，也不改变普通消费组。
// 每步最多32条完整记录，跨分区没有全局顺序，分区内保留broker原始次序和原字节。
func (r *Replayer) Step(ctx context.Context) error {
	if r == nil || r.Seed == nil || r.Seed.DB == nil || r.Logs == nil || r.Processor == nil {
		return errors.New("SEARCH_REBUILD_DEPENDENCIES_UNAVAILABLE")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	operation, err := repository.PendingReplay(ctx, r.Seed.DB)
	if err != nil || operation == "" {
		return err
	}
	tx, err := r.Seed.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return errors.New("SEARCH_REBUILD_TRANSACTION_UNAVAILABLE")
	}
	input, err := r.Seed.lock(ctx, tx, operation)
	var frontier [domain.PartitionCount]int64
	if err == nil && input.job.State == "SWITCHING" {
		frontier, err = repository.LoadFrontier(ctx, tx, input.job.ID)
	}
	tx.Rollback()
	if err != nil {
		return err
	}
	if input.job.State != "REPLAYING" && input.job.State != "SWITCHING" {
		return nil
	}
	original := logsource.Window{Identity: logsource.Identity{Cluster: input.prepare.Cluster, Topic: input.prepare.Topic}}
	uuid, err := hex.DecodeString(input.prepare.TopicID)
	if err != nil || len(uuid) != len(original.Identity.TopicID) {
		return errors.New("SEARCH_REBUILD_LOG_IDENTITY_INVALID")
	}
	copy(original.Identity.TopicID[:], uuid)
	var starts, ends [domain.PartitionCount]int64
	for partition, raw := range input.prepare.Starts {
		starts[partition], err = strconv.ParseInt(raw, 10, 64)
		if err != nil || starts[partition] < 0 {
			return errors.New("SEARCH_REBUILD_LOG_WINDOW_INVALID")
		}
		// Read的原窗口Last始终是持久L，绝不能用恢复next或本轮末端覆盖。
		original.Partitions[partition].Last = starts[partition]
	}
	// 元数据与Fetch都在任务锁之外；Capture/Read核验原Cluster、UUID和严格三个分区。
	window, err := r.Logs.Capture(ctx, &original.Identity)
````

## 第 6 步：Replayer.Step（接着实现）

插入位置：第 5 步代码之后，紧接追加；完成稿约第 84 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>window.Identity != original.Identity</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_LOG_IDENTITY_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>bounds.First &lt; 0 &#124;&#124; bounds.First &gt; starts&#91;partition&#93; &#124;&#124; bounds.Last &lt; starts&#91;partition&#93;</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_LOG_WINDOW_LOST&quot;)</code>。当前函数在这里结束。
- 检查 <code>input.job.State == &quot;SWITCHING&quot;</code>。
- 检查 <code>frontier&#91;partition&#93; &lt; starts&#91;partition&#93; &#124;&#124; frontier&#91;partition&#93; &gt; bounds.Last</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_FRONTIER_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>processor == nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_DEPENDENCIES_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>next&#91;partition&#93; == ends&#91;partition&#93;</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>record.Topic != input.prepare.Topic &#124;&#124; record.Partition != partition &#124;&#124; 				record.Offset != next&#91;partition&#93; &#124;&#124; record.Offset &gt;= ends&#91;partition&#93;</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_LOG_RECORD_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。

<!-- write-step: 6 bytes=2995:4643 -->
````go
	if err != nil {
		return err
	}
	if window.Identity != original.Identity {
		return errors.New("SEARCH_LOG_IDENTITY_CHANGED")
	}
	for partition, bounds := range window.Partitions {
		if bounds.First < 0 || bounds.First > starts[partition] || bounds.Last < starts[partition] {
			return errors.New("SEARCH_REBUILD_LOG_WINDOW_LOST")
		}
		ends[partition] = bounds.Last
		if input.job.State == "SWITCHING" {
			// 持久屏障之后只补到原H。新发布留给切换后的普通消费，
			// 不能让不断增长的broker末端改写本次完整验证的有限边界。
			if frontier[partition] < starts[partition] || frontier[partition] > bounds.Last {
				return errors.New("SEARCH_REBUILD_FRONTIER_INVALID")
			}
			ends[partition] = frontier[partition]
		}
	}
	next, err := repository.ReplayCheckpoints(ctx, r.Seed.DB, input.job.ID, input.prepare.Topic, starts, ends)
	if err != nil {
		return err
	}
	processor := r.Processor(repository.ConsumerName(input.job.ID))
	if processor == nil {
		return errors.New("SEARCH_REBUILD_DEPENDENCIES_UNAVAILABLE")
	}
	remaining := 32
	for partition := range next {
		if next[partition] == ends[partition] {
			continue
		}
		records, _, err := r.Logs.Read(ctx, original, partition, next[partition], ends[partition])
		if err != nil {
			return err
		}
		for _, record := range records {
			if record.Topic != input.prepare.Topic || record.Partition != partition ||
				record.Offset != next[partition] || record.Offset >= ends[partition] {
				return errors.New("SEARCH_REBUILD_LOG_RECORD_INVALID")
			}
			if err := r.apply(ctx, operation, input, processor, record); err != nil {
				return err
			}
````

## 第 7 步：Replayer.Step（接着实现）

插入位置：第 6 步代码之后，紧接追加；完成稿约第 129 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>remaining == 0</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。

<!-- write-step: 7 bytes=4643:4744 -->
````go
			next[partition]++
			remaining--
			if remaining == 0 {
				return nil
			}
		}
	}
	return nil
}

````

## 第 8 步：Replayer.apply

插入位置：第 7 步代码之后，紧接追加；完成稿约第 139 行。

本步声明：`func (r *Replayer) apply(parent context.Context, operation string, input seedJob, processor ReplayProcessor, record domain.Record) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>parent</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>context.WithTimeout(parent, 30*time.Second)</code> |
| <code>operation</code> | <code>string</code> | 固定重建操作号。<br>在此声明中的实际使用：<code>r.Seed.lock(ctx, tx, operation)</code> |
| <code>input</code> | <code>seedJob</code> | 此前锁内读出的原准备、导入与job绑定，写入前仍须重读核验。<br>在此声明中的实际使用：<code>input.job</code> |
| <code>processor</code> | <code>ReplayProcessor</code> | 绑定该job专用consumer名称的整记录处理器。<br>在此声明中的实际使用：<code>processor.Record</code> |
| <code>record</code> | <code>domain.Record</code> | broker原始记录，保留主题、分区、连续offset与原字节。<br>在此声明中的实际使用：<code>record.Topic</code>；<code>record.Partition</code>；<code>record.Offset</code>；<code>record.Position</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_REBUILD_TRANSACTION_UNAVAILABLE&quot;)</code>；<code>err</code>；<code>errors.New(&quot;SEARCH_REBUILD_PROGRESS_CONFLICT&quot;)</code>；<code>errors.New(&quot;SEARCH_REBUILD_CHECKPOINT_MISSING&quot;)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 执行 <code>context.WithTimeout(parent, 30*time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>r.Seed.DB.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_TRANSACTION_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>current.job.ID != input.job.ID &#124;&#124; current.job.State != input.job.State &#124;&#124; current.job.Target != input.job.Target</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_PROGRESS_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>tx.QueryRowContext(ctx, `SELECT next_offset FROM consumer_checkpoint WHERE consumer_name=? AND kafka_topic=? AND partition_id=?`, 		repository.ConsumerName(current.job.ID), record. …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_CHECKPOINT_MISSING&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT next_offset FROM consumer_checkpoint WHERE consumer_name=? AND kafka_topic=? AND partition_id=?`, 		repository.ConsumerName(current.job.ID), record.Topic, record.Partition).Scan(&amp;next)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT next_offset FROM consumer_checkpoint WHERE consumer_name=? AND kafka_topic=? AND partition_id=?`, 		repository.ConsumerName(current.job.ID), record.Topic, record.Partition)</code>。
- 检查 <code>record.Offset &lt; next</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>record.Offset != next</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_OFFSET_GAP&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return processor.Record(ctx, current.job.Target, record.Position, record.Value)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=4744:6587 -->
````go
func (r *Replayer) apply(parent context.Context, operation string, input seedJob, processor ReplayProcessor, record domain.Record) error {
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	tx, err := r.Seed.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return errors.New("SEARCH_REBUILD_TRANSACTION_UNAVAILABLE")
	}
	defer tx.Rollback()
	current, err := r.Seed.lock(ctx, tx, operation)
	if err != nil {
		return err
	}
	if current.job.ID != input.job.ID || current.job.State != input.job.State || current.job.Target != input.job.Target {
		return errors.New("SEARCH_REBUILD_PROGRESS_CONFLICT")
	}
	// 另一个实例可能已推进next并开始验证；旧批次不能再次向Doc取更新正文，
	// 否则即使offset已完成也会在验证之后改写目标。必须在原job锁内重新判定。
	var next int64
	if tx.QueryRowContext(ctx, `SELECT next_offset FROM consumer_checkpoint WHERE consumer_name=? AND kafka_topic=? AND partition_id=?`,
		repository.ConsumerName(current.job.ID), record.Topic, record.Partition).Scan(&next) != nil {
		return errors.New("SEARCH_REBUILD_CHECKPOINT_MISSING")
	}
	if record.Offset < next {
		return nil
	}
	if record.Offset != next {
		return errors.New("SEARCH_OFFSET_GAP")
	}
	// 此事务仅持有generation SHARE→binding/job锁，直到整条Record返回才释放。
	// Processor仍明确拥有各应用/事件/整记录checkpoint事务，不能反向取得job锁；
	// 与普通Guard相同，这不是ES/SQL原子事务。ES或SQL提交未知时保留原任务，
	// 下一步从SQL next重读，由原行摘要、物理应用账本及Record幂等恢复，绝不猜进度。
	return processor.Record(ctx, current.job.Target, record.Position, record.Value)
}

// terminal.go：保留原实现，按业务合并文件。

````

## 第 9 步：terminalProof

插入位置：第 8 步代码之后，紧接追加；完成稿约第 176 行。

terminalProof只含已封存事实；JSON中的uint64用字符串保留完整精度。
原请求摘要和job绑定使后来代次/当前alias变化不会改写这次操作的答复。

本步声明：`type terminalProof struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Version</code> | <code>int</code> | 序列化/校验标记：<code>`json:&quot;version&quot;`</code> |
| <code>State</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;state&quot;`</code> |
| <code>Operation</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;operation_id&quot;`</code> |
| <code>Fingerprint</code> | <code>string</code> | 规范化请求内容的摘要，用来拒绝相同操作号携带不同内容；不作为身份或授权凭据。<br>序列化/校验标记：<code>`json:&quot;request_fingerprint&quot;`</code> |
| <code>JobID</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;job_id&quot;`</code> |
| <code>Generation</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;generation&quot;`</code> |
| <code>Revision</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;terminal_revision&quot;`</code> |
| <code>Alias</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;alias&quot;`</code> |
| <code>Source</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;source_index&quot;`</code> |
| <code>Target</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;target_index&quot;`</code> |
| <code>Snapshot</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;snapshot_ref&quot;`</code> |
| <code>Ends</code> | <code>&#91;domain.PartitionCount&#93;string</code> | 序列化/校验标记：<code>`json:&quot;frontier&quot;`</code> |
| <code>Validation</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;validation_digest&quot;`</code> |

<!-- write-step: 9 bytes=6587:7639 -->
````go
// terminalProof只含已封存事实；JSON中的uint64用字符串保留完整精度。
// 原请求摘要和job绑定使后来代次/当前alias变化不会改写这次操作的答复。
type terminalProof struct {
	Version     int                           `json:"version"`
	State       string                        `json:"state"`
	Operation   string                        `json:"operation_id"`
	Fingerprint string                        `json:"request_fingerprint"`
	JobID       string                        `json:"job_id"`
	Generation  string                        `json:"generation"`
	Revision    string                        `json:"terminal_revision"`
	Alias       string                        `json:"alias"`
	Source      string                        `json:"source_index"`
	Target      string                        `json:"target_index"`
	Snapshot    string                        `json:"snapshot_ref"`
	Ends        [domain.PartitionCount]string `json:"frontier"`
	Validation  string                        `json:"validation_digest"`
}

````

## 第 10 步：terminalEnvelope

插入位置：第 9 步代码之后，紧接追加；完成稿约第 194 行。

本步声明：`type terminalEnvelope struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Version</code> | <code>int</code> | 序列化/校验标记：<code>`json:&quot;version&quot;`</code> |
| <code>Fixed</code> | <code>&#91;&#93;byte</code> | []byte序列化为base64，MySQL JSON键序/空白规范化不能改变原固定结果字节。<br>序列化/校验标记：<code>`json:&quot;fixed_result&quot;`</code> |

<!-- write-step: 10 bytes=7639:7839 -->
````go
type terminalEnvelope struct {
	Version int `json:"version"`
	// []byte序列化为base64，MySQL JSON键序/空白规范化不能改变原固定结果字节。
	Fixed []byte `json:"fixed_result"`
}

````

## 第 11 步：fixedRebuildResult

插入位置：第 10 步代码之后，紧接追加；完成稿约第 200 行。

本步声明：`func fixedRebuildResult(operation string, p rebuildmeta.Prepare, job repository.Job, fingerprint []byte, facts repository.TerminalFacts) (*pb.OperationResult, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>operation</code> | <code>string</code> | 固定重建操作号。<br>在此声明中的实际使用：<code>p.Operation != operation</code>；<code>json.Marshal(terminalProof{1, &quot;SUCCEEDED&quot;, operation, hex.EncodeToString(fingerprint), strconv.FormatUint(job.ID, 10), strconv.FormatUint(job.Generation, 10), strconv.FormatUint(re …</code>（长表达式，按本步完整代码继续）；<code>OperationId: operation</code> |
| <code>p</code> | <code>rebuildmeta.Prepare</code> | 使用 <code>rebuildmeta.Prepare</code> 类型；其定义处的职责：Prepare的整数采用字符串。原L与源/目标代次一起签名，不能靠两台机器的时间戳证明先后。<br>在此声明中的实际使用：<code>p.SourceGeneration</code>；<code>p.TargetGeneration</code>；<code>p.SourceRevision</code>；<code>p.Operation</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |
| <code>job</code> | <code>repository.Job</code> | 使用 <code>repository.Job</code> 类型；其定义处的职责：持久化重建准备绑定、三分区封存前沿、任务创建及分页验证进度。<br>在此声明中的实际使用：<code>job.Generation</code>；<code>job.ID</code>；<code>job.Source</code>；<code>job.Target</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>fingerprint</code> | <code>&#91;&#93;byte</code> | 32字节原StartRebuild请求摘要。<br>在此声明中的实际使用：<code>len(fingerprint)</code>；<code>hex.EncodeToString(fingerprint)</code> |
| <code>facts</code> | <code>repository.TerminalFacts</code> | 原任务已封存的终态游标、前沿和验证摘要。<br>在此声明中的实际使用：<code>facts.Validation</code>；<code>facts.Frontier</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.OperationResult</code> | 实际返回表达式：<code>nil</code>；<code>&amp;pb.OperationResult{State: pb.OperationResult_SUCCEEDED, OperationId: operation, Generation: job.Generation, TerminalRevision: revision + 1, ResultIds: &#91;&#93;uint64{job.ID}, ResultDige …</code>（长表达式，按本步完整代码继续） |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_REBUILD_TERMINAL_INVALID&quot;)</code>；<code>err</code>；<code>errors.New(&quot;SEARCH_REBUILD_FRONTIER_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil &#124;&#124; sourceGeneration == ^uint64(0) &#124;&#124; job.Generation != sourceGeneration+1 &#124;&#124; strconv.FormatUint(job.Generation, 10) != p.TargetGeneration</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_REBUILD_TERMINAL_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; revision == ^uint64(0) &#124;&#124; len(fingerprint) != 32 &#124;&#124; len(facts.Validation) != 32 &#124;&#124; bytes.Equal(facts.Validation, make(&#91;&#93;byte, 32)) &#124;&#124; job.ID == 0 &#124;&#124; p.Operation != op …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_REBUILD_TERMINAL_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; end &lt; start</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_REBUILD_FRONTIER_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_REBUILD_TERMINAL_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return &amp;pb.OperationResult{State: pb.OperationResult_SUCCEEDED, OperationId: operation, Generation: job.Generation, TerminalRevision: revision + 1, ResultIds: &#91;&#93;uint64{job.ID}, Res …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=7839:9710 -->
````go
func fixedRebuildResult(operation string, p rebuildmeta.Prepare, job repository.Job, fingerprint []byte, facts repository.TerminalFacts) (*pb.OperationResult, error) {
	sourceGeneration, err := rebuildmeta.Decimal(p.SourceGeneration, true)
	if err != nil || sourceGeneration == ^uint64(0) || job.Generation != sourceGeneration+1 || strconv.FormatUint(job.Generation, 10) != p.TargetGeneration {
		return nil, errors.New("SEARCH_REBUILD_TERMINAL_INVALID")
	}
	revision, err := rebuildmeta.Decimal(p.SourceRevision, true)
	if err != nil || revision == ^uint64(0) || len(fingerprint) != 32 || len(facts.Validation) != 32 || bytes.Equal(facts.Validation, make([]byte, 32)) || job.ID == 0 || p.Operation != operation || job.Source != p.Source || job.Target != p.Target {
		return nil, errors.New("SEARCH_REBUILD_TERMINAL_INVALID")
	}
	ends, err := repository.DecodeFrontier(facts.Frontier)
	if err != nil {
		return nil, err
	}
	var values [domain.PartitionCount]string
	for partition, end := range ends {
		start, err := strconv.ParseInt(p.Starts[partition], 10, 64)
		if err != nil || end < start {
			return nil, errors.New("SEARCH_REBUILD_FRONTIER_INVALID")
		}
		values[partition] = strconv.FormatInt(end, 10)
	}
	fixed, err := json.Marshal(terminalProof{1, "SUCCEEDED", operation, hex.EncodeToString(fingerprint), strconv.FormatUint(job.ID, 10), strconv.FormatUint(job.Generation, 10), strconv.FormatUint(revision+1, 10), p.Alias, job.Source, job.Target, job.SnapshotRef, values, hex.EncodeToString(facts.Validation)})
	if err != nil {
		return nil, errors.New("SEARCH_REBUILD_TERMINAL_INVALID")
	}
	digest := sha256.Sum256(fixed)
	return &pb.OperationResult{State: pb.OperationResult_SUCCEEDED, OperationId: operation, Generation: job.Generation, TerminalRevision: revision + 1, ResultIds: []uint64{job.ID}, ResultDigest: digest[:], FixedResultJsonUtf8: fixed}, nil
}

````

## 第 12 步：loadTerminalResult

插入位置：第 11 步代码之后，紧接追加；完成稿约第 229 行。

本步声明：`func loadTerminalResult(ctx context.Context, tx *sql.Tx, operation string, p rebuildmeta.Prepare, job repository.Job, fingerprint []byte) (*pb.OperationResult, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.LoadTerminalFacts(ctx, tx, job.ID)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.LoadTerminalFacts(ctx, tx, job.ID)</code> |
| <code>operation</code> | <code>string</code> | 固定重建操作号。<br>在此声明中的实际使用：<code>fixedRebuildResult(operation, p, job, fingerprint, facts)</code> |
| <code>p</code> | <code>rebuildmeta.Prepare</code> | 使用 <code>rebuildmeta.Prepare</code> 类型；其定义处的职责：Prepare的整数采用字符串。原L与源/目标代次一起签名，不能靠两台机器的时间戳证明先后。<br>在此声明中的实际使用：<code>fixedRebuildResult(operation, p, job, fingerprint, facts)</code> |
| <code>job</code> | <code>repository.Job</code> | 使用 <code>repository.Job</code> 类型；其定义处的职责：持久化重建准备绑定、三分区封存前沿、任务创建及分页验证进度。<br>在此声明中的实际使用：<code>job.State</code>；<code>job.ID</code>；<code>fixedRebuildResult(operation, p, job, fingerprint, facts)</code> |
| <code>fingerprint</code> | <code>&#91;&#93;byte</code> | 32字节原StartRebuild请求摘要。<br>在此声明中的实际使用：<code>fixedRebuildResult(operation, p, job, fingerprint, facts)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.OperationResult</code> | 实际返回表达式：<code>nil</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_REBUILD_TERMINAL_PROOF_UNAVAILABLE&quot;)</code>；<code>err</code>；<code>errors.New(&quot;SEARCH_REBUILD_TERMINAL_PROOF_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>job.State != &quot;SUCCEEDED&quot;</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_REBUILD_TERMINAL_PROOF_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(facts.Cursor, &amp;envelope) != nil &#124;&#124; envelope.Version != 1 &#124;&#124; !bytes.Equal(envelope.Fixed, result.FixedResultJsonUtf8)</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_REBUILD_TERMINAL_PROOF_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=9710:10455 -->
````go
func loadTerminalResult(ctx context.Context, tx *sql.Tx, operation string, p rebuildmeta.Prepare, job repository.Job, fingerprint []byte) (*pb.OperationResult, error) {
	if job.State != "SUCCEEDED" {
		return nil, errors.New("SEARCH_REBUILD_TERMINAL_PROOF_UNAVAILABLE")
	}
	facts, err := repository.LoadTerminalFacts(ctx, tx, job.ID)
	if err != nil {
		return nil, err
	}
	result, err := fixedRebuildResult(operation, p, job, fingerprint, facts)
	if err != nil {
		return nil, err
	}
	var envelope terminalEnvelope
	if json.Unmarshal(facts.Cursor, &envelope) != nil || envelope.Version != 1 || !bytes.Equal(envelope.Fixed, result.FixedResultJsonUtf8) {
		return nil, errors.New("SEARCH_REBUILD_TERMINAL_PROOF_INVALID")
	}
	return result, nil
}

````

## 第 13 步：finalizeRebuild

插入位置：第 12 步代码之后，紧接追加；完成稿约第 248 行。

finalizeRebuild只落定SQL；调用者在同一排他事务内先验证原alias已经到目标。
返回值只有事务提交成功后才能对外发布，提交未知由原号读取固定证明恢复。

本步声明：`func finalizeRebuild(ctx context.Context, tx *sql.Tx, operation string, input seedJob) (*pb.OperationResult, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.LoadJobBinding(ctx, tx, operation)</code>；<code>loadTerminalResult(ctx, tx, operation, input.prepare, input.job, bound.Fingerprint)</code>；<code>repository.LoadTerminalFacts(ctx, tx, input.job.ID)</code>；<code>repository.CheckTerminalFrontier(ctx, tx, input.job.ID, input.prepare.Topic, ends)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.LoadJobBinding(ctx, tx, operation)</code>；<code>loadTerminalResult(ctx, tx, operation, input.prepare, input.job, bound.Fingerprint)</code>；<code>repository.LoadTerminalFacts(ctx, tx, input.job.ID)</code>；<code>repository.CheckTerminalFrontier(ctx, tx, input.job.ID, input.prepare.Topic, ends)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>operation</code> | <code>string</code> | 固定重建操作号。<br>在此声明中的实际使用：<code>repository.LoadJobBinding(ctx, tx, operation)</code>；<code>loadTerminalResult(ctx, tx, operation, input.prepare, input.job, bound.Fingerprint)</code>；<code>rebuildmeta.RequestFingerprint(operation, input.job.Generation, input.job.SnapshotRef, watermark)</code>；<code>fixedRebuildResult(operation, input.prepare, input.job, bound.Fingerprint, facts)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>input</code> | <code>seedJob</code> | 此前锁内读出的原准备、导入与job绑定，写入前仍须重读核验。<br>在此声明中的实际使用：<code>input.job</code>；<code>input.prepare</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.OperationResult</code> | 由 <code>loadTerminalResult(ctx, tx, operation, input.prepare, input.job, bound.Fingerprint)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>nil</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>loadTerminalResult(ctx, tx, operation, input.prepare, input.job, bound.Fingerprint)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;SEARCH_REBUILD_OPERATION_CONFLICT&quot;)</code>；<code>errors.New(&quot;SEARCH_REBUILD_PROGRESS_CONFLICT&quot;)</code>；<code>errors.New(&quot;SEARCH_REBUILD_JOB_INVALID&quot;)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>!bound.JobID.Valid &#124;&#124; bound.JobID.String != strconv.FormatUint(input.job.ID, 10)</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_REBUILD_OPERATION_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>input.job.State == &quot;SUCCEEDED&quot;</code>。条件成立时执行 <code>return loadTerminalResult(ctx, tx, operation, input.prepare, input.job, bound.Fingerprint)</code>。当前函数在这里结束。
- 检查 <code>input.job.State != &quot;SWITCHING&quot;</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_REBUILD_PROGRESS_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(input.job.Watermark, &amp;descriptor) != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_REBUILD_JOB_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !bytes.Equal(expected&#91;:&#93;, bound.Fingerprint)</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_REBUILD_OPERATION_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。

<!-- write-step: 13 bytes=10455:12373 -->
````go
// finalizeRebuild只落定SQL；调用者在同一排他事务内先验证原alias已经到目标。
// 返回值只有事务提交成功后才能对外发布，提交未知由原号读取固定证明恢复。
func finalizeRebuild(ctx context.Context, tx *sql.Tx, operation string, input seedJob) (*pb.OperationResult, error) {
	bound, err := repository.LoadJobBinding(ctx, tx, operation)
	if err != nil {
		return nil, err
	}
	if !bound.JobID.Valid || bound.JobID.String != strconv.FormatUint(input.job.ID, 10) {
		return nil, errors.New("SEARCH_REBUILD_OPERATION_CONFLICT")
	}
	if input.job.State == "SUCCEEDED" {
		return loadTerminalResult(ctx, tx, operation, input.prepare, input.job, bound.Fingerprint)
	}
	if input.job.State != "SWITCHING" {
		return nil, errors.New("SEARCH_REBUILD_PROGRESS_CONFLICT")
	}
	facts, err := repository.LoadTerminalFacts(ctx, tx, input.job.ID)
	if err != nil {
		return nil, err
	}
	ends, err := repository.DecodeFrontier(facts.Frontier)
	if err != nil {
		return nil, err
	}
	if err := repository.CheckTerminalFrontier(ctx, tx, input.job.ID, input.prepare.Topic, ends); err != nil {
		return nil, err
	}
	// 原watermark是MySQL JSON，先还原规范字节再核对原请求摘要。
	var descriptor rebuildmeta.Descriptor
	if json.Unmarshal(input.job.Watermark, &descriptor) != nil {
		return nil, errors.New("SEARCH_REBUILD_JOB_INVALID")
	}
	watermark, err := descriptor.Bytes()
	if err != nil {
		return nil, err
	}
	expected, err := rebuildmeta.RequestFingerprint(operation, input.job.Generation, input.job.SnapshotRef, watermark)
	if err != nil || !bytes.Equal(expected[:], bound.Fingerprint) {
		return nil, errors.New("SEARCH_REBUILD_OPERATION_CONFLICT")
	}
	result, err := fixedRebuildResult(operation, input.prepare, input.job, bound.Fingerprint, facts)
	if err != nil {
		return nil, err
	}
	cursor, err := json.Marshal(terminalEnvelope{1, result.FixedResultJsonUtf8})
````

## 第 14 步：finalizeRebuild（接着实现）

插入位置：第 13 步代码之后，紧接追加；完成稿约第 293 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_REBUILD_TERMINAL_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=12373:12838 -->
````go
	if err != nil {
		return nil, errors.New("SEARCH_REBUILD_TERMINAL_INVALID")
	}
	sourceGeneration, _ := rebuildmeta.Decimal(input.prepare.SourceGeneration, true)
	sourceRevision, _ := rebuildmeta.Decimal(input.prepare.SourceRevision, true)
	if err := repository.CompleteSwitch(ctx, tx, operation, input.prepare.Alias, input.job, sourceGeneration, sourceRevision, bound.Fingerprint, facts.Validation, cursor); err != nil {
		return nil, err
	}
	return result, nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/search/internal/logic/rebuild/completion_service.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/search/05-cutover-wiring.md)。
