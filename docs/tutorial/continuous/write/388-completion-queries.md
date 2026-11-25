# 分步编写 `services/search/internal/repository/rebuild/completion_queries.go`

先按 [S05 · 固定窗口重放、完整核验与别名恢复切换](../lessons/search/05-cutover-wiring.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/388-completion-queries.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

读取专用重放检查点和终态证明，并在原事务内落定索引代次及任务成功。

重建必须有自己从原L创建的检查点；借用普通消费进度会把未重放到目标的消息误判为完成。

写之前先按这条执行路线理解各段的连接：

1. PendingReplay 仅选 ACCEPTED 的原任务；缺失检查点不是从0开始。
2. ReplayCheckpoints 验证每个next处在原L与当前上界之间，读取时不持锁跨Processor事务。
3. LoadTerminalFacts 与 CheckTerminalFrontier 在原事务内锁住任务和三分区检查点，要求next恰好等于封存H。
4. CompleteSwitch 先CAS index_generation，再以原号/指纹/验证摘要将job置SUCCEEDED，提交由外层统一负责。

本文件需要守住的边界：

- 任一影响行数不为1都必须返回冲突，不能部分提交。
- next超过H也不算本次有限区间已验证；提交未知只能原号重读。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/search/internal/repository/rebuild' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/search/internal/repository/rebuild/completion_queries.go') -and (Get-Item -LiteralPath 'services/search/internal/repository/rebuild/completion_queries.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/search/internal/repository/rebuild/completion_queries.go')) { New-Item -ItemType File -Path 'services/search/internal/repository/rebuild/completion_queries.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:283 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package rebuild

import (
	events "collaboration/search/internal/domain/event"
	"context"
	"database/sql"
	"errors"
	"math"
)

// replay.go：保留原实现，按业务合并文件。

````

## 第 2 步：PendingReplay

插入位置：第 1 步代码之后，紧接追加；完成稿约第 14 行。

本步声明：`func PendingReplay(ctx context.Context, db *sql.DB) (string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>db.QueryRowContext(ctx, `SELECT b.operation_id FROM rebuild_binding b JOIN rebuild_job j ON j.id=b.job_id 		JOIN rebuild_progress p ON p.job_id=j.id 		WHERE b.state=&#x27;ACCEPTED&#x27; AND  …</code>（长表达式，按本步完整代码继续） |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db.QueryRowContext</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;&quot;</code>；<code>operation</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>errors.New(&quot;SEARCH_REBUILD_JOB_READ_FAILED&quot;)</code> |

**跟着本段代码走一遍：**

- 执行 <code>db.QueryRowContext(ctx, `SELECT b.operation_id FROM rebuild_binding b JOIN rebuild_job j ON j.id=b.job_id 		JOIN rebuild_progress p ON p.job_id=j.id 		WHERE b.state=&#x27;ACCEPTED&#x27; AND  …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>db.QueryRowContext(ctx, `SELECT b.operation_id FROM rebuild_binding b JOIN rebuild_job j ON j.id=b.job_id 		JOIN rebuild_progress p ON p.job_id=j.id 		WHERE b.state=&#x27;ACCEPTED&#x27; AND  …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return &quot;&quot;, nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;&quot;, errors.New(&quot;SEARCH_REBUILD_JOB_READ_FAILED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return operation, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=283:843 -->
````go
func PendingReplay(ctx context.Context, db *sql.DB) (string, error) {
	var operation string
	err := db.QueryRowContext(ctx, `SELECT b.operation_id FROM rebuild_binding b JOIN rebuild_job j ON j.id=b.job_id
		JOIN rebuild_progress p ON p.job_id=j.id
		WHERE b.state='ACCEPTED' AND (j.state='REPLAYING' OR (j.state='SWITCHING' AND p.validation_digest IS NULL)) ORDER BY j.id LIMIT 1`).Scan(&operation)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", errors.New("SEARCH_REBUILD_JOB_READ_FAILED")
	}
	return operation, nil
}

````

## 第 3 步：ReplayCheckpoints

插入位置：第 2 步代码之后，紧接追加；完成稿约第 28 行。

ReplayCheckpoints要求Start显式建立的三个检查点全部存在；缺行不能沿用普通消费的从0语义。
此处只读进度，不持有checkpoint锁跨越Processor自己的事件/整记录提交事务。

本步声明：`func ReplayCheckpoints(ctx context.Context, db *sql.DB, id uint64, topic string, starts, ends [events.PartitionCount]int64) ([events.PartitionCount]int64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>db.QueryRowContext(ctx, `SELECT next_offset,revision FROM consumer_checkpoint 			WHERE consumer_name=? AND kafka_topic=? AND partition_id=?`, ConsumerName(id), topic, partition)</code> |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db.QueryRowContext</code> |
| <code>id</code> | <code>uint64</code> | 原重建任务数据库主键。<br>在此声明中的实际使用：<code>ConsumerName(id)</code> |
| <code>topic</code> | <code>string</code> | 原准备绑定的Kafka主题名。<br>在此声明中的实际使用：<code>db.QueryRowContext(ctx, `SELECT next_offset,revision FROM consumer_checkpoint 			WHERE consumer_name=? AND kafka_topic=? AND partition_id=?`, ConsumerName(id), topic, partition)</code> |
| <code>starts</code> | <code>&#91;events.PartitionCount&#93;int64</code> | 准备阶段固定的三个分区起点L。<br>在此声明中的实际使用：<code>starts&#91;partition&#93; &lt; 0</code>；<code>int64(next) &lt; starts&#91;partition&#93;</code> |
| <code>ends</code> | <code>&#91;events.PartitionCount&#93;int64</code> | 本次读取允许的三个分区终点；终态确认时为已封存H。<br>在此声明中的实际使用：<code>int64(next) &gt; ends&#91;partition&#93;</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;events.PartitionCount&#93;int64</code> | 实际返回表达式：<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_REBUILD_CHECKPOINT_MISSING&quot;)</code>；<code>errors.New(&quot;SEARCH_CHECKPOINT_READ_FAILED&quot;)</code>；<code>errors.New(&quot;SEARCH_REBUILD_CHECKPOINT_UNBOUND&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>db.QueryRowContext(ctx, `SELECT next_offset,revision FROM consumer_checkpoint 			WHERE consumer_name=? AND kafka_topic=? AND partition_id=?`, ConsumerName(id), topic, partition).Scan(&amp;next, &amp;revision)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>db.QueryRowContext(ctx, `SELECT next_offset,revision FROM consumer_checkpoint 			WHERE consumer_name=? AND kafka_topic=? AND partition_id=?`, ConsumerName(id), topic, partition)</code>。
- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return result, errors.New(&quot;SEARCH_REBUILD_CHECKPOINT_MISSING&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, errors.New(&quot;SEARCH_CHECKPOINT_READ_FAILED&quot;)</code>。当前函数在这里结束。
- 检查 <code>next &gt; math.MaxInt64 &#124;&#124; revision == 0 &#124;&#124; starts&#91;partition&#93; &lt; 0 &#124;&#124; 			int64(next) &lt; starts&#91;partition&#93; &#124;&#124; int64(next) &gt; ends&#91;partition&#93;</code>。条件成立时执行 <code>return result, errors.New(&quot;SEARCH_REBUILD_CHECKPOINT_UNBOUND&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=843:2067 -->
````go
// ReplayCheckpoints要求Start显式建立的三个检查点全部存在；缺行不能沿用普通消费的从0语义。
// 此处只读进度，不持有checkpoint锁跨越Processor自己的事件/整记录提交事务。
func ReplayCheckpoints(ctx context.Context, db *sql.DB, id uint64, topic string, starts, ends [events.PartitionCount]int64) ([events.PartitionCount]int64, error) {
	var result [events.PartitionCount]int64
	for partition := range result {
		var next, revision uint64
		err := db.QueryRowContext(ctx, `SELECT next_offset,revision FROM consumer_checkpoint
			WHERE consumer_name=? AND kafka_topic=? AND partition_id=?`, ConsumerName(id), topic, partition).Scan(&next, &revision)
		if errors.Is(err, sql.ErrNoRows) {
			return result, errors.New("SEARCH_REBUILD_CHECKPOINT_MISSING")
		}
		if err != nil {
			return result, errors.New("SEARCH_CHECKPOINT_READ_FAILED")
		}
		if next > math.MaxInt64 || revision == 0 || starts[partition] < 0 ||
			int64(next) < starts[partition] || int64(next) > ends[partition] {
			return result, errors.New("SEARCH_REBUILD_CHECKPOINT_UNBOUND")
		}
		result[partition] = int64(next)
	}
	return result, nil
}

// terminal.go：保留原实现，按业务合并文件。

````

## 第 4 步：TerminalFacts

插入位置：第 3 步代码之后，紧接追加；完成稿约第 53 行。

本步声明：`type TerminalFacts struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Cursor</code> | <code>&#91;&#93;byte</code> | 任务cursor_json中的固定终态封装。 |
| <code>Frontier</code> | <code>&#91;&#93;byte</code> | rebuild_progress中不可变的三分区H载荷。 |
| <code>Validation</code> | <code>&#91;&#93;byte</code> | 已完成验证的摘要，缺失时不能确认终态。 |

<!-- write-step: 4 bytes=2067:2135 -->
````go
type TerminalFacts struct {
	Cursor, Frontier, Validation []byte
}

````

## 第 5 步：LoadTerminalFacts

插入位置：第 4 步代码之后，紧接追加；完成稿约第 57 行。

本步声明：`func LoadTerminalFacts(ctx context.Context, tx *sql.Tx, id uint64) (TerminalFacts, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT j.cursor_json,p.frontier_payload,p.validation_digest 		FROM rebuild_job j JOIN rebuild_progress p ON p.job_id=j.id WHERE j.id=? FOR UPDATE`, id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>id</code> | <code>uint64</code> | 原重建任务数据库主键。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT j.cursor_json,p.frontier_payload,p.validation_digest 		FROM rebuild_job j JOIN rebuild_progress p ON p.job_id=j.id WHERE j.id=? FOR UPDATE`, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>TerminalFacts</code> | 实际返回表达式：<code>facts</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_REBUILD_TERMINAL_READ_FAILED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT j.cursor_json,p.frontier_payload,p.validation_digest 		FROM rebuild_job j JOIN rebuild_progress p ON p.job_id=j.id WHERE j.id=? FOR UPDATE`, id)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return facts, errors.New(&quot;SEARCH_REBUILD_TERMINAL_READ_FAILED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return facts, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=2135:2604 -->
````go
func LoadTerminalFacts(ctx context.Context, tx *sql.Tx, id uint64) (TerminalFacts, error) {
	var facts TerminalFacts
	err := tx.QueryRowContext(ctx, `SELECT j.cursor_json,p.frontier_payload,p.validation_digest
		FROM rebuild_job j JOIN rebuild_progress p ON p.job_id=j.id WHERE j.id=? FOR UPDATE`, id).
		Scan(&facts.Cursor, &facts.Frontier, &facts.Validation)
	if err != nil {
		return facts, errors.New("SEARCH_REBUILD_TERMINAL_READ_FAILED")
	}
	return facts, nil
}

````

## 第 6 步：CheckTerminalFrontier

插入位置：第 5 步代码之后，紧接追加；完成稿约第 68 行。

CheckTerminalFrontier在同一排他事务内读取原专用检查点；只接受完整到达H，
普通消费进度或超过H的不同区间都不能替代本次封存证明。

本步声明：`func CheckTerminalFrontier(ctx context.Context, tx *sql.Tx, id uint64, topic string, ends [events.PartitionCount]int64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT next_offset,revision FROM consumer_checkpoint 			WHERE consumer_name=? AND kafka_topic=? AND partition_id=? FOR UPDATE`, ConsumerName(id), topic, partition)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>id</code> | <code>uint64</code> | 原重建任务数据库主键。<br>在此声明中的实际使用：<code>ConsumerName(id)</code> |
| <code>topic</code> | <code>string</code> | 原准备绑定的Kafka主题名。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT next_offset,revision FROM consumer_checkpoint 			WHERE consumer_name=? AND kafka_topic=? AND partition_id=? FOR UPDATE`, ConsumerName(id), topic, partition)</code> |
| <code>ends</code> | <code>&#91;events.PartitionCount&#93;int64</code> | 本次读取允许的三个分区终点；终态确认时为已封存H。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_REBUILD_CHECKPOINT_READ_FAILED&quot;)</code>；<code>errors.New(&quot;SEARCH_REBUILD_FRONTIER_INCOMPLETE&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_CHECKPOINT_READ_FAILED&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT next_offset,revision FROM consumer_checkpoint 			WHERE consumer_name=? AND kafka_topic=? AND partition_id=? FOR UPDATE`, ConsumerName(id), topic, partition).Scan(&amp;next, &amp;revision)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT next_offset,revision FROM consumer_checkpoint 			WHERE consumer_name=? AND kafka_topic=? AND partition_id=? FOR UPDATE`, ConsumerName(id), topic, partition)</code>。
- 检查 <code>end &lt; 0 &#124;&#124; next != uint64(end) &#124;&#124; revision == 0</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_FRONTIER_INCOMPLETE&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=2604:3420 -->
````go
// CheckTerminalFrontier在同一排他事务内读取原专用检查点；只接受完整到达H，
// 普通消费进度或超过H的不同区间都不能替代本次封存证明。
func CheckTerminalFrontier(ctx context.Context, tx *sql.Tx, id uint64, topic string, ends [events.PartitionCount]int64) error {
	for partition, end := range ends {
		var next, revision uint64
		if err := tx.QueryRowContext(ctx, `SELECT next_offset,revision FROM consumer_checkpoint
			WHERE consumer_name=? AND kafka_topic=? AND partition_id=? FOR UPDATE`, ConsumerName(id), topic, partition).Scan(&next, &revision); err != nil {
			return errors.New("SEARCH_REBUILD_CHECKPOINT_READ_FAILED")
		}
		if end < 0 || next != uint64(end) || revision == 0 {
			return errors.New("SEARCH_REBUILD_FRONTIER_INCOMPLETE")
		}
	}
	return nil
}

````

## 第 7 步：CompleteSwitch

插入位置：第 6 步代码之后，紧接追加；完成稿约第 84 行。

CompleteSwitch不提交事务。调用者持有generation排他锁及原binding/job锁，
且已经确认真实alias唯一指向目标；CAS、终态和固定证明必须一起提交。
任一语句失败都必须回滚，未知提交结果只能重新读取原任务。

本步声明：`func CompleteSwitch(ctx context.Context, tx *sql.Tx, operation, alias string, job Job, sourceGeneration, sourceRevision uint64, fingerprint, validation, cursor []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE index_generation SET physical_index=?,generation=?,revision=revision+1 		WHERE alias_name=? AND physical_index=? AND generation=? AND revision=?`, 		job …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, `UPDATE rebuild_job j JOIN rebuild_progress p ON p.job_id=j.id 		JOIN rebuild_binding b ON b.job_id=j.id SET j.state=&#x27;SUCCEEDED&#x27;,j.cursor_json=?,j.last_error=NU …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>operation</code> | <code>string</code> | 固定的重建操作号。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE rebuild_job j JOIN rebuild_progress p ON p.job_id=j.id 		JOIN rebuild_binding b ON b.job_id=j.id SET j.state=&#x27;SUCCEEDED&#x27;,j.cursor_json=?,j.last_error=NU …</code>（长表达式，按本步完整代码继续） |
| <code>alias</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE index_generation SET physical_index=?,generation=?,revision=revision+1 		WHERE alias_name=? AND physical_index=? AND generation=? AND revision=?`, 		job …</code>（长表达式，按本步完整代码继续） |
| <code>job</code> | <code>Job</code> | 已在原锁序下读取的任务事实，包含来源、目标和目标代次。<br>在此声明中的实际使用：<code>job.Generation</code>；<code>job.Target</code>；<code>job.Source</code>；<code>job.ID</code> |
| <code>sourceGeneration</code> | <code>uint64</code> | 原准备固定的来源代次，用作CAS条件。<br>在此声明中的实际使用：<code>sourceGeneration == 0</code>；<code>sourceGeneration == ^uint64(0)</code>；<code>sourceGeneration+1</code>；<code>tx.ExecContext(ctx, `UPDATE index_generation SET physical_index=?,generation=?,revision=revision+1 		WHERE alias_name=? AND physical_index=? AND generation=? AND revision=?`, 		job …</code>（长表达式，按本步完整代码继续） |
| <code>sourceRevision</code> | <code>uint64</code> | 原准备固定的来源修订号，用作CAS条件。<br>在此声明中的实际使用：<code>sourceRevision == 0</code>；<code>sourceRevision == ^uint64(0)</code>；<code>tx.ExecContext(ctx, `UPDATE index_generation SET physical_index=?,generation=?,revision=revision+1 		WHERE alias_name=? AND physical_index=? AND generation=? AND revision=?`, 		job …</code>（长表达式，按本步完整代码继续） |
| <code>fingerprint</code> | <code>&#91;&#93;byte</code> | 绑定原重建请求的32字节摘要。<br>在此声明中的实际使用：<code>len(fingerprint)</code>；<code>tx.ExecContext(ctx, `UPDATE rebuild_job j JOIN rebuild_progress p ON p.job_id=j.id 		JOIN rebuild_binding b ON b.job_id=j.id SET j.state=&#x27;SUCCEEDED&#x27;,j.cursor_json=?,j.last_error=NU …</code>（长表达式，按本步完整代码继续） |
| <code>validation</code> | <code>&#91;&#93;byte</code> | 已经封存的32字节验证摘要。<br>在此声明中的实际使用：<code>len(validation)</code>；<code>tx.ExecContext(ctx, `UPDATE rebuild_job j JOIN rebuild_progress p ON p.job_id=j.id 		JOIN rebuild_binding b ON b.job_id=j.id SET j.state=&#x27;SUCCEEDED&#x27;,j.cursor_json=?,j.last_error=NU …</code>（长表达式，按本步完整代码继续） |
| <code>cursor</code> | <code>&#91;&#93;byte</code> | 包含固定成功结果的序列化终态封装。<br>在此声明中的实际使用：<code>len(cursor)</code>；<code>tx.ExecContext(ctx, `UPDATE rebuild_job j JOIN rebuild_progress p ON p.job_id=j.id 		JOIN rebuild_binding b ON b.job_id=j.id SET j.state=&#x27;SUCCEEDED&#x27;,j.cursor_json=?,j.last_error=NU …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_REBUILD_TERMINAL_INVALID&quot;)</code>；<code>errors.New(&quot;SEARCH_REBUILD_GENERATION_WRITE_FAILED&quot;)</code>；<code>errors.New(&quot;SEARCH_REBUILD_GENERATION_CHANGED&quot;)</code>；<code>errors.New(&quot;SEARCH_REBUILD_TERMINAL_WRITE_FAILED&quot;)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>sourceGeneration == 0 &#124;&#124; sourceGeneration == ^uint64(0) &#124;&#124; sourceRevision == 0 &#124;&#124; sourceRevision == ^uint64(0) &#124;&#124; job.Generation != sourceGeneration+1 &#124;&#124; len(fingerprint) != 32 &#124;&#124;  …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_TERMINAL_INVALID&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `UPDATE index_generation SET physical_index=?,generation=?,revision=revision+1 		WHERE alias_name=? AND physical_index=? AND generation=? AND revision=?`, 		job …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_GENERATION_WRITE_FAILED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; n != 1</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_GENERATION_CHANGED&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `UPDATE rebuild_job j JOIN rebuild_progress p ON p.job_id=j.id 		JOIN rebuild_binding b ON b.job_id=j.id SET j.state=&#x27;SUCCEEDED&#x27;,j.cursor_json=?,j.last_error=NU …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_TERMINAL_WRITE_FAILED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; n != 1</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_PROGRESS_CONFLICT&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=3420:5382 -->
````go
// CompleteSwitch不提交事务。调用者持有generation排他锁及原binding/job锁，
// 且已经确认真实alias唯一指向目标；CAS、终态和固定证明必须一起提交。
// 任一语句失败都必须回滚，未知提交结果只能重新读取原任务。
func CompleteSwitch(ctx context.Context, tx *sql.Tx, operation, alias string, job Job, sourceGeneration, sourceRevision uint64, fingerprint, validation, cursor []byte) error {
	if sourceGeneration == 0 || sourceGeneration == ^uint64(0) || sourceRevision == 0 || sourceRevision == ^uint64(0) || job.Generation != sourceGeneration+1 || len(fingerprint) != 32 || len(validation) != 32 || len(cursor) == 0 {
		return errors.New("SEARCH_REBUILD_TERMINAL_INVALID")
	}
	result, err := tx.ExecContext(ctx, `UPDATE index_generation SET physical_index=?,generation=?,revision=revision+1
		WHERE alias_name=? AND physical_index=? AND generation=? AND revision=?`,
		job.Target, job.Generation, alias, job.Source, sourceGeneration, sourceRevision)
	if err != nil {
		return errors.New("SEARCH_REBUILD_GENERATION_WRITE_FAILED")
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return errors.New("SEARCH_REBUILD_GENERATION_CHANGED")
	}
	result, err = tx.ExecContext(ctx, `UPDATE rebuild_job j JOIN rebuild_progress p ON p.job_id=j.id
		JOIN rebuild_binding b ON b.job_id=j.id SET j.state='SUCCEEDED',j.cursor_json=?,j.last_error=NULL
		WHERE j.id=? AND j.state='SWITCHING' AND j.source_index=? AND j.target_index=? AND j.generation=?
		AND b.operation_id=? AND b.state='ACCEPTED' AND b.request_fingerprint=?
		AND p.frontier_payload IS NOT NULL AND p.validation_digest=?`,
		cursor, job.ID, job.Source, job.Target, job.Generation, operation, fingerprint, validation)
	if err != nil {
		return errors.New("SEARCH_REBUILD_TERMINAL_WRITE_FAILED")
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return errors.New("SEARCH_REBUILD_PROGRESS_CONFLICT")
	}
	return nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/search/internal/repository/rebuild/completion_queries.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/search/05-cutover-wiring.md)。
