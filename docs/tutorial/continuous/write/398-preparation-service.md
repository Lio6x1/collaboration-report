# 分步编写 `services/search/internal/logic/rebuild/preparation_service.go`

先按 [S04 · 先封存L，再导入签名快照并种子回填](../lessons/search/04-rebuild-snapshot.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/398-preparation-service.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

组织重建准备、任务接受、封存切换屏障、两阶段完整验证及未知切换恢复。

要先固定Kafka原L，再允许Doc建立快照S；追赶接近时才封H暂停普通访问。原H固定后验证的是有限区间，避免新发布让验证永远追不上。

写之前先按这条执行路线理解各段的连接：

1. Preparer.Prepare按generation→binding锁序恢复原号；新准备核当前alias、捕获三分区L、签发并保存PREPARED。
2. JobService.StartRebuild/GetRebuild绑定同一请求指纹；新接受时创建任务及原L检查点，已有任务按原证明恢复。
3. Barrier.Seal等每分区距离末端不超过32条才封存H与SWITCHING；先取排他锁，不从共享锁升级。
4. Validator先确认next恰为H，再检查快照最低覆盖、每条物理应用与ES实际内容和总数，最后封摘要。
5. Coordinator.finish在原排他事务中核真实alias；SOURCE执行精确切换，TARGET仅落定原SQL结果。

本文件需要守住的边界：

- 缺少验证摘要不能只凭job.state宣布SUCCEEDED；空索引也必须有非零封存证明。
- 原号恢复不要求当前代次仍是旧源代次，也不因当前证书过期重新签发原Prepare。
- 切换未知时保留SWITCHING，不解除屏障让旧查询越过未完成的代次转换。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/search/internal/logic/rebuild' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/search/internal/logic/rebuild/preparation_service.go') -and (Get-Item -LiteralPath 'services/search/internal/logic/rebuild/preparation_service.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/search/internal/logic/rebuild/preparation_service.go')) { New-Item -ItemType File -Path 'services/search/internal/logic/rebuild/preparation_service.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:867 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package rebuild

import (
	"bytes"
	pb "collaboration/contracts/internalv1"
	"collaboration/platform/rebuildartifact"
	"collaboration/platform/rebuildmeta"
	"collaboration/search/internal/client/elastic"
	"collaboration/search/internal/client/logsource"
	events "collaboration/search/internal/domain/event"
	index "collaboration/search/internal/domain/index"
	event "collaboration/search/internal/repository/event"
	generation "collaboration/search/internal/repository/generation"
	repository "collaboration/search/internal/repository/rebuild"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// barrier.go：保留原实现，按业务合并文件。

````

## 第 2 步：SwitchIndexes

插入位置：第 1 步代码之后，紧接追加；完成稿约第 33 行。

本步声明：`type SwitchIndexes interface {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>InspectAliasSwitch</code> | <code>(context.Context, string, string, string) (elastic.AliasSwitchState, error)</code> | ctx、alias、source、target；只核验真实ES状态并返回SOURCE/TARGET，不修改绑定。 |
| <code>SwitchAlias</code> | <code>(context.Context, string, string, string) error</code> | ctx和原alias/source/target；精确执行remove+add，未知回执返回错误交原任务恢复。 |

<!-- write-step: 2 bytes=867:1056 -->
````go
type SwitchIndexes interface {
	InspectAliasSwitch(context.Context, string, string, string) (elastic.AliasSwitchState, error)
	SwitchAlias(context.Context, string, string, string) error
}

````

## 第 3 步：Barrier

插入位置：第 2 步代码之后，紧接追加；完成稿约第 38 行。

Barrier只封存切换屏障，不执行alias变更。调用方必须先装配完整恢复器，
因为提交后普通消费及查询会由既有CheckSwitching明确暂停。

本步声明：`type Barrier struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Seed</code> | <code>*Seeder</code> | 提供原任务DB、锁序、Kafka身份和固定准备。 |
| <code>Indexes</code> | <code>SwitchIndexes</code> | 核验及切换原别名的能力，封屏障前必须已装配完整恢复器。 |

<!-- write-step: 3 bytes=1056:1302 -->
````go
// Barrier只封存切换屏障，不执行alias变更。调用方必须先装配完整恢复器，
// 因为提交后普通消费及查询会由既有CheckSwitching明确暂停。
type Barrier struct {
	Seed    *Seeder
	Indexes SwitchIndexes
}

````

## 第 4 步：Barrier.Seal

插入位置：第 3 步代码之后，紧接追加；完成稿约第 45 行。

本步声明：`func (b *Barrier) Seal(ctx context.Context, operation string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx, cancel := context.WithTimeout(ctx, 5*time.Second)</code>；<code>context.WithTimeout(ctx, 5*time.Second)</code>；<code>b.Seed.DB.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>；<code>generation.Find(ctx, tx, b.Seed.Alias, true)</code><br>另有 6 处使用，继续按下方完整代码的顺序处理。 |
| <code>operation</code> | <code>string</code> | 贯穿准备、导入、RPC接受和恢复的原操作号。<br>在此声明中的实际使用：<code>b.Seed.lock(ctx, tx, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_REBUILD_DEPENDENCIES_UNAVAILABLE&quot;)</code>；<code>errors.New(&quot;SEARCH_REBUILD_TRANSACTION_UNAVAILABLE&quot;)</code>；<code>err</code>；<code>errors.New(&quot;SEARCH_GENERATION_MISSING&quot;)</code><br>另有 9 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>b == nil &#124;&#124; b.Seed == nil &#124;&#124; b.Seed.DB == nil &#124;&#124; b.Seed.Logs == nil &#124;&#124; b.Indexes == nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_DEPENDENCIES_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 执行 <code>context.WithTimeout(ctx, 5*time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>b.Seed.DB.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_TRANSACTION_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!found</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_GENERATION_MISSING&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>input.job.State == &quot;SWITCHING&quot;</code>。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>input.job.State != &quot;REPLAYING&quot;</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_PROGRESS_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>state != elastic.AliasAtSource</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_ALIAS_SWITCH_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; len(uuid) != len(identity.TopicID)</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_LOG_IDENTITY_INVALID&quot;)</code>。当前函数在这里结束。

<!-- write-step: 4 bytes=1302:2988 -->
````go
func (b *Barrier) Seal(ctx context.Context, operation string) error {
	if b == nil || b.Seed == nil || b.Seed.DB == nil || b.Seed.Logs == nil || b.Indexes == nil {
		return errors.New("SEARCH_REBUILD_DEPENDENCIES_UNAVAILABLE")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := b.Seed.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return errors.New("SEARCH_REBUILD_TRANSACTION_UNAVAILABLE")
	}
	defer tx.Rollback()
	// 排他锁先等待普通消费和重建的完整记录退出，再读取原任务；
	// 不在持有共享锁后升级，避免两个协调器同时升级产生死锁。
	_, found, err := generation.Find(ctx, tx, b.Seed.Alias, true)
	if err != nil {
		return err
	}
	if !found {
		return errors.New("SEARCH_GENERATION_MISSING")
	}
	input, err := b.Seed.lock(ctx, tx, operation)
	if err != nil {
		return err
	}
	if input.job.State == "SWITCHING" {
		_, err = repository.LoadFrontier(ctx, tx, input.job.ID)
		return err
	}
	if input.job.State != "REPLAYING" {
		return errors.New("SEARCH_REBUILD_PROGRESS_CONFLICT")
	}
	state, err := b.Indexes.InspectAliasSwitch(ctx, b.Seed.Alias, input.job.Source, input.job.Target)
	if err != nil {
		return err
	}
	if state != elastic.AliasAtSource {
		return errors.New("SEARCH_ALIAS_SWITCH_CONFLICT")
	}
	identity := logsource.Identity{Cluster: input.prepare.Cluster, Topic: input.prepare.Topic}
	uuid, err := hex.DecodeString(input.prepare.TopicID)
	if err != nil || len(uuid) != len(identity.TopicID) {
		return errors.New("SEARCH_REBUILD_LOG_IDENTITY_INVALID")
	}
	copy(identity.TopicID[:], uuid)
	window, err := b.Seed.Logs.Capture(ctx, &identity)
````

## 第 5 步：Barrier.Seal（接着实现）

插入位置：第 4 步代码之后，紧接追加；完成稿约第 90 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>window.Identity != identity</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_LOG_IDENTITY_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; bounds.First &gt; start &#124;&#124; bounds.Last &lt; start</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_LOG_WINDOW_LOST&quot;)</code>。当前函数在这里结束。
- 检查 <code>tx.QueryRowContext(ctx, `SELECT next_offset FROM consumer_checkpoint WHERE consumer_name=? AND kafka_topic=? AND partition_id=?`, repository.ConsumerName(input.job.ID), input.prepare.Topic, p).Scan(&amp;next) != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_CHECKPOINT_MISSING&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT next_offset FROM consumer_checkpoint WHERE consumer_name=? AND kafka_topic=? AND partition_id=?`, repository.ConsumerName(input.job.ID), input.prepare.Topic, p).Scan(&amp;next)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT next_offset FROM consumer_checkpoint WHERE consumer_name=? AND kafka_topic=? AND partition_id=?`, repository.ConsumerName(input.job.ID), input.prepare.Topic, p)</code>。
- 检查 <code>next &lt; start &#124;&#124; next &gt; bounds.Last</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_CHECKPOINT_UNBOUND&quot;)</code>。当前函数在这里结束。
- 检查 <code>bounds.Last-next &gt; 32</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>tx.Commit() != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_COMMIT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=2988:4184 -->
````go
	if err != nil {
		return err
	}
	if window.Identity != identity {
		return errors.New("SEARCH_LOG_IDENTITY_CHANGED")
	}
	var ends [events.PartitionCount]int64
	for p, bounds := range window.Partitions {
		start, err := strconv.ParseInt(input.prepare.Starts[p], 10, 64)
		if err != nil || bounds.First > start || bounds.Last < start {
			return errors.New("SEARCH_REBUILD_LOG_WINDOW_LOST")
		}
		ends[p] = bounds.Last
		var next int64
		if tx.QueryRowContext(ctx, `SELECT next_offset FROM consumer_checkpoint WHERE consumer_name=? AND kafka_topic=? AND partition_id=?`, repository.ConsumerName(input.job.ID), input.prepare.Topic, p).Scan(&next) != nil {
			return errors.New("SEARCH_REBUILD_CHECKPOINT_MISSING")
		}
		if next < start || next > bounds.Last {
			return errors.New("SEARCH_REBUILD_CHECKPOINT_UNBOUND")
		}
		// 尚未追近时继续后台重放，不提前暂停当前搜索。
		if bounds.Last-next > 32 {
			return nil
		}
	}
	if err := repository.SealFrontier(ctx, tx, input.job.ID, ends); err != nil {
		return err
	}
	if tx.Commit() != nil {
		return errors.New("SEARCH_REBUILD_COMMIT_UNKNOWN")
	}
	return nil
}

// coordinator.go：保留原实现，按业务合并文件。

````

## 第 6 步：Coordinator

插入位置：第 5 步代码之后，紧接追加；完成稿约第 126 行。

本步声明：`type Coordinator struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Barrier</code> | <code>*Barrier</code> | 负责将原H与SWITCHING一起封存。 |
| <code>Validator</code> | <code>*Validator</code> | 负责有限区间追平后的完整内容验证。 |

<!-- write-step: 6 bytes=4184:4255 -->
````go
type Coordinator struct {
	Barrier   *Barrier
	Validator *Validator
}

````

## 第 7 步：Coordinator.Step

插入位置：第 6 步代码之后，紧接追加；完成稿约第 131 行。

本步声明：`func (c *Coordinator) Step(ctx context.Context) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>c.Barrier.Seed.DB.QueryRowContext(ctx, `SELECT b.operation_id,j.state FROM rebuild_binding b JOIN rebuild_job j ON j.id=b.job_id WHERE b.state=&#x27;ACCEPTED&#x27; AND j.state IN (&#x27;REPLAYING&#x27;,&#x27;SWITCHING&#x27;) ORDER BY j.id LIMIT 1`)</code>；<code>c.Barrier.Seal(ctx, operation)</code>；<code>c.Validator.Step(ctx, operation)</code>；<code>c.finish(ctx, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_REBUILD_DEPENDENCIES_UNAVAILABLE&quot;)</code>；<code>nil</code>；<code>errors.New(&quot;SEARCH_REBUILD_JOB_READ_FAILED&quot;)</code>；<code>c.Barrier.Seal(ctx, operation)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>c == nil &#124;&#124; c.Barrier == nil &#124;&#124; c.Barrier.Seed == nil &#124;&#124; c.Validator == nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_DEPENDENCIES_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 执行 <code>c.Barrier.Seed.DB.QueryRowContext(ctx, `SELECT b.operation_id,j.state FROM rebuild_binding b JOIN rebuild_job j ON j.id=b.job_id WHERE b.state=&#x27;ACCEPTED&#x27; AND j.state IN (&#x27;REPLAYING …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>c.Barrier.Seed.DB.QueryRowContext(ctx, `SELECT b.operation_id,j.state FROM rebuild_binding b JOIN rebuild_job j ON j.id=b.job_id WHERE b.state=&#x27;ACCEPTED&#x27; AND j.state IN (&#x27;REPLAYING&#x27;,&#x27;SWITCHING&#x27;) ORDER BY j.id LIMIT 1`)</code>。
- 检查 <code>err == sql.ErrNoRows</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_JOB_READ_FAILED&quot;)</code>。当前函数在这里结束。
- 检查 <code>state == &quot;REPLAYING&quot;</code>。条件成立时执行 <code>return c.Barrier.Seal(ctx, operation)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return c.finish(ctx, operation)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=4255:5037 -->
````go
func (c *Coordinator) Step(ctx context.Context) error {
	if c == nil || c.Barrier == nil || c.Barrier.Seed == nil || c.Validator == nil {
		return errors.New("SEARCH_REBUILD_DEPENDENCIES_UNAVAILABLE")
	}
	var operation, state string
	err := c.Barrier.Seed.DB.QueryRowContext(ctx, `SELECT b.operation_id,j.state FROM rebuild_binding b JOIN rebuild_job j ON j.id=b.job_id WHERE b.state='ACCEPTED' AND j.state IN ('REPLAYING','SWITCHING') ORDER BY j.id LIMIT 1`).Scan(&operation, &state)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return errors.New("SEARCH_REBUILD_JOB_READ_FAILED")
	}
	if state == "REPLAYING" {
		return c.Barrier.Seal(ctx, operation)
	}
	if err := c.Validator.Step(ctx, operation); err != nil {
		return err
	}
	return c.finish(ctx, operation)
}

````

## 第 8 步：Coordinator.finish

插入位置：第 7 步代码之后，紧接追加；完成稿约第 152 行。

finish专用恢复路径不调用拒绝SWITCHING的普通Guard。ES已在目标时只完成
原SQL事务，绝不再次增加代次；未知回执留在同一任务下由下一步重读。

本步声明：`func (c *Coordinator) finish(parent context.Context, operation string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>parent</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>context.WithTimeout(parent, 10*time.Second)</code> |
| <code>operation</code> | <code>string</code> | 贯穿准备、导入、RPC接受和恢复的原操作号。<br>在此声明中的实际使用：<code>s.lock(ctx, tx, operation)</code>；<code>finalizeRebuild(ctx, tx, operation, input)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_REBUILD_TRANSACTION_UNAVAILABLE&quot;)</code>；<code>err</code>；<code>errors.New(&quot;SEARCH_GENERATION_MISSING&quot;)</code>；<code>nil</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 执行 <code>context.WithTimeout(parent, 10*time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>s.DB.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_TRANSACTION_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!found</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_GENERATION_MISSING&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>input.job.State == &quot;SUCCEEDED&quot;</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>input.job.State != &quot;SWITCHING&quot;</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_PROGRESS_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!progress.Complete</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>state == elastic.AliasAtSource</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>state != elastic.AliasAtTarget</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_ALIAS_SWITCH_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。

<!-- write-step: 8 bytes=5037:6583 -->
````go
// finish专用恢复路径不调用拒绝SWITCHING的普通Guard。ES已在目标时只完成
// 原SQL事务，绝不再次增加代次；未知回执留在同一任务下由下一步重读。
func (c *Coordinator) finish(parent context.Context, operation string) error {
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	s := c.Barrier.Seed
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return errors.New("SEARCH_REBUILD_TRANSACTION_UNAVAILABLE")
	}
	defer tx.Rollback()
	if _, found, err := generation.Find(ctx, tx, s.Alias, true); err != nil {
		return err
	} else if !found {
		return errors.New("SEARCH_GENERATION_MISSING")
	}
	input, err := s.lock(ctx, tx, operation)
	if err != nil {
		return err
	}
	if input.job.State == "SUCCEEDED" {
		return nil
	}
	if input.job.State != "SWITCHING" {
		return errors.New("SEARCH_REBUILD_PROGRESS_CONFLICT")
	}
	progress, err := repository.LoadValidation(ctx, tx, input.job.ID)
	if err != nil {
		return err
	}
	if !progress.Complete {
		return nil
	}
	state, err := c.Barrier.Indexes.InspectAliasSwitch(ctx, s.Alias, input.job.Source, input.job.Target)
	if err != nil {
		return err
	}
	if state == elastic.AliasAtSource {
		if err := c.Barrier.Indexes.SwitchAlias(ctx, s.Alias, input.job.Source, input.job.Target); err != nil {
			return err
		}
	} else if state != elastic.AliasAtTarget {
		return errors.New("SEARCH_ALIAS_SWITCH_CONFLICT")
	}
	if _, err := finalizeRebuild(ctx, tx, operation, input); err != nil {
````

## 第 9 步：Coordinator.finish（接着实现）

插入位置：第 8 步代码之后，紧接追加；完成稿约第 197 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>tx.Commit() != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_COMMIT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=6583:6752 -->
````go
		return err
	}
	if tx.Commit() != nil {
		return errors.New("SEARCH_REBUILD_COMMIT_UNKNOWN")
	}
	return nil
}

// jobs.go：保留原实现，按业务合并文件。

````

## 第 10 步：JobService

插入位置：第 9 步代码之后，紧接追加；完成稿约第 207 行。

本步声明：`type JobService struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>DB</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>Owner</code> | <code>string</code> | 必须匹配Prepare的本实例owner。 |
| <code>Alias</code> | <code>string</code> | 必须匹配Prepare的本服务稳定别名。 |
| <code>AcceptedDir</code> | <code>string</code> | RPC接受前检查完整快照副本可用的绝对目录。 |

<!-- write-step: 10 bytes=6752:6849 -->
````go
type JobService struct {
	DB                        *sql.DB
	Owner, Alias, AcceptedDir string
}

````

## 第 11 步：JobService.StartRebuild

插入位置：第 10 步代码之后，紧接追加；完成稿约第 212 行。

本步声明：`func (s *JobService) StartRebuild(ctx context.Context, request *pb.RebuildRequest) (*pb.OperationResult, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.run(ctx, request.OperationId, fingerprint&#91;:&#93;, request)</code> |
| <code>request</code> | <code>*pb.RebuildRequest</code> | 固定RebuildRequest，包含原号、目标代次、snapshot引用和规范watermark。<br>在此声明中的实际使用：<code>request == nil</code>；<code>request.ProtoReflect</code>；<code>request.OperationId</code>；<code>request.Generation</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.OperationResult</code> | 由 <code>s.run(ctx, request.OperationId, fingerprint&#91;:&#93;, request)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>s.run(ctx, request.OperationId, fingerprint&#91;:&#93;, request)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_REBUILD_REQUEST_INVALID&quot;)</code>；<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>request == nil &#124;&#124; len(request.ProtoReflect().GetUnknown()) != 0</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_REBUILD_REQUEST_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return s.run(ctx, request.OperationId, fingerprint&#91;:&#93;, request)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=6849:7342 -->
````go
func (s *JobService) StartRebuild(ctx context.Context, request *pb.RebuildRequest) (*pb.OperationResult, error) {
	if request == nil || len(request.ProtoReflect().GetUnknown()) != 0 {
		return nil, errors.New("SEARCH_REBUILD_REQUEST_INVALID")
	}
	fingerprint, err := rebuildmeta.RequestFingerprint(request.OperationId, request.Generation, request.SnapshotRef, request.WatermarkJsonUtf8)
	if err != nil {
		return nil, err
	}
	return s.run(ctx, request.OperationId, fingerprint[:], request)
}

````

## 第 12 步：JobService.GetRebuild

插入位置：第 11 步代码之后，紧接追加；完成稿约第 223 行。

本步声明：`func (s *JobService) GetRebuild(ctx context.Context, query *pb.OperationQuery) (*pb.OperationResult, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.run(ctx, query.OperationId, query.RequestFingerprint, nil)</code> |
| <code>query</code> | <code>*pb.OperationQuery</code> | 原号查询，必须携带32字节原请求指纹，不能带文档级Target。<br>在此声明中的实际使用：<code>query == nil</code>；<code>query.OperationId</code>；<code>query.RequestFingerprint</code>；<code>query.ProtoReflect</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.OperationResult</code> | 由 <code>s.run(ctx, query.OperationId, query.RequestFingerprint, nil)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>s.run(ctx, query.OperationId, query.RequestFingerprint, nil)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_REBUILD_QUERY_INVALID&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>query == nil &#124;&#124; !rebuildmeta.OperationValid(query.OperationId) &#124;&#124; len(query.RequestFingerprint) != 32 &#124;&#124; len(query.ProtoReflect().GetUnknown()) != 0</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_REBUILD_QUERY_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>t != nil &amp;&amp; (t.DocumentId != 0 &#124;&#124; t.UnitId != 0 &#124;&#124; t.BranchId != 0 &#124;&#124; t.StructureId != 0 &#124;&#124; t.WorkspaceEpoch != 0 &#124;&#124; t.Purpose != &quot;&quot; &#124;&#124; len(t.ProtoReflect().GetUnknown()) != 0)</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_REBUILD_QUERY_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return s.run(ctx, query.OperationId, query.RequestFingerprint, nil)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=7342:8111 -->
````go
func (s *JobService) GetRebuild(ctx context.Context, query *pb.OperationQuery) (*pb.OperationResult, error) {
	if query == nil || !rebuildmeta.OperationValid(query.OperationId) || len(query.RequestFingerprint) != 32 || len(query.ProtoReflect().GetUnknown()) != 0 {
		return nil, errors.New("SEARCH_REBUILD_QUERY_INVALID")
	}
	// 重建没有文档级Target；即使已知字段全零，未知字段也不能悄悄改变查询语义。
	t := query.Target
	if t != nil && (t.DocumentId != 0 || t.UnitId != 0 || t.BranchId != 0 || t.StructureId != 0 || t.WorkspaceEpoch != 0 || t.Purpose != "" || len(t.ProtoReflect().GetUnknown()) != 0) {
		return nil, errors.New("SEARCH_REBUILD_QUERY_INVALID")
	}
	return s.run(ctx, query.OperationId, query.RequestFingerprint, nil)
}

````

## 第 13 步：JobService.run

插入位置：第 12 步代码之后，紧接追加；完成稿约第 235 行。

本步声明：`func (s *JobService) run(ctx context.Context, operation string, fingerprint []byte, request *pb.RebuildRequest) (*pb.OperationResult, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.DB.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>；<code>generation.Find(ctx, tx, s.Alias, true)</code>；<code>repository.Find(ctx, tx, operation)</code>；<code>repository.FindImport(ctx, tx, operation)</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |
| <code>operation</code> | <code>string</code> | 贯穿准备、导入、RPC接受和恢复的原操作号。<br>在此声明中的实际使用：<code>repository.Find(ctx, tx, operation)</code>；<code>repository.FindImport(ctx, tx, operation)</code>；<code>rebuildmeta.RequestFingerprint(operation, targetGeneration, snapshotRef, watermark)</code>；<code>repository.LoadJobBinding(ctx, tx, operation)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>fingerprint</code> | <code>&#91;&#93;byte</code> | 原重建请求的规范摘要。<br>在此声明中的实际使用：<code>bytes.Equal(expected&#91;:&#93;, fingerprint)</code>；<code>bytes.Equal(row.Fingerprint, fingerprint)</code>；<code>repository.AcceptJob(ctx, tx, operation, fingerprint, job, p.Topic, p.Starts)</code>；<code>loadTerminalResult(ctx, tx, operation, p, job, fingerprint)</code> |
| <code>request</code> | <code>*pb.RebuildRequest</code> | 固定RebuildRequest，包含原号、目标代次、snapshot引用和规范watermark。<br>在此声明中的实际使用：<code>request == nil</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.OperationResult</code> | 实际返回表达式：<code>nil</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_REBUILD_UNAVAILABLE&quot;)</code>；<code>errors.New(&quot;SEARCH_REBUILD_TRANSACTION_UNAVAILABLE&quot;)</code>；<code>err</code>；<code>errors.New(&quot;SEARCH_GENERATION_MISSING&quot;)</code><br>另有 9 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>s == nil &#124;&#124; s.DB == nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_REBUILD_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 执行 <code>s.DB.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_REBUILD_TRANSACTION_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>!exists</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_GENERATION_MISSING&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>!found</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_REBUILD_OPERATION_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 检查 <code>p.Owner != s.Owner &#124;&#124; p.Alias != s.Alias</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_REBUILD_OPERATION_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>binding.State != &quot;IMPORTED&quot; &amp;&amp; binding.State != &quot;ACCEPTED&quot;</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_REBUILD_NOT_IMPORTED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>!found</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_REBUILD_NOT_IMPORTED&quot;)</code>。当前函数在这里结束。
- 检查 <code>imported.Seal.PrepareRef != prepareRef</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_REBUILD_OPERATION_CONFLICT&quot;)</code>。当前函数在这里结束。

<!-- write-step: 13 bytes=8111:9807 -->
````go
func (s *JobService) run(ctx context.Context, operation string, fingerprint []byte, request *pb.RebuildRequest) (*pb.OperationResult, error) {
	if s == nil || s.DB == nil {
		return nil, errors.New("SEARCH_REBUILD_UNAVAILABLE")
	}
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, errors.New("SEARCH_REBUILD_TRANSACTION_UNAVAILABLE")
	}
	defer tx.Rollback()
	// 统一generation→binding→job锁序；原号恢复不要求当前代次仍是旧源代次。
	current, exists, err := generation.Find(ctx, tx, s.Alias, true)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.New("SEARCH_GENERATION_MISSING")
	}
	binding, found, err := repository.Find(ctx, tx, operation)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errors.New("SEARCH_REBUILD_OPERATION_NOT_FOUND")
	}
	p := binding.Prepare
	if p.Owner != s.Owner || p.Alias != s.Alias {
		return nil, errors.New("SEARCH_REBUILD_OPERATION_CONFLICT")
	}
	if binding.State != "IMPORTED" && binding.State != "ACCEPTED" {
		return nil, errors.New("SEARCH_REBUILD_NOT_IMPORTED")
	}
	imported, found, err := repository.FindImport(ctx, tx, operation)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errors.New("SEARCH_REBUILD_NOT_IMPORTED")
	}
	prepareDigest := sha256.Sum256(binding.Envelope)
	prepareRef := "rebuild-prepare-v1:" + hex.EncodeToString(prepareDigest[:])
	snapshotRef := "rebuild-snapshot-v1:" + hex.EncodeToString(imported.Digest[:])
	if imported.Seal.PrepareRef != prepareRef {
		return nil, errors.New("SEARCH_REBUILD_OPERATION_CONFLICT")
	}
	descriptor, err := rebuildmeta.DescriptorFor(p, prepareRef, snapshotRef)
````

## 第 14 步：JobService.run（接着实现）

插入位置：第 13 步代码之后，紧接追加；完成稿约第 280 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !bytes.Equal(expected&#91;:&#93;, fingerprint)</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_REBUILD_OPERATION_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>binding.State == &quot;ACCEPTED&quot;</code>。
- 检查 <code>!bytes.Equal(row.Fingerprint, fingerprint) &#124;&#124; !row.JobID.Valid &#124;&#124; parseErr != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_REBUILD_OPERATION_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(job.Watermark, &amp;persisted) != nil &#124;&#124; persisted != descriptor &#124;&#124; job.Generation != targetGeneration &#124;&#124; job.Source != p.Source &#124;&#124; job.Target != p.Target &#124;&#124; job.SnapshotRef != snapshotRef</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_REBUILD_JOB_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>request == nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_REBUILD_NOT_ACCEPTED&quot;)</code>。当前函数在这里结束。
- 检查 <code>current.Physical != p.Source &#124;&#124; strconv.FormatUint(current.Generation, 10) != p.SourceGeneration &#124;&#124; strconv.FormatUint(current.Revision, 10) != p.SourceRevision</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_REBUILD_GENERATION_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>!filepath.IsAbs(s.AcceptedDir)</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_REBUILD_SNAPSHOT_UNAVAILABLE&quot;)</code>。当前函数在这里结束。

<!-- write-step: 14 bytes=9807:11710 -->
````go
	if err != nil {
		return nil, err
	}
	watermark, err := descriptor.Bytes()
	if err != nil {
		return nil, err
	}
	targetGeneration, _ := rebuildmeta.Decimal(p.TargetGeneration, true)
	expected, err := rebuildmeta.RequestFingerprint(operation, targetGeneration, snapshotRef, watermark)
	if err != nil || !bytes.Equal(expected[:], fingerprint) {
		return nil, errors.New("SEARCH_REBUILD_OPERATION_CONFLICT")
	}
	row, err := repository.LoadJobBinding(ctx, tx, operation)
	if err != nil {
		return nil, err
	}
	var job repository.Job
	if binding.State == "ACCEPTED" {
		id, parseErr := rebuildmeta.Decimal(row.JobID.String, true)
		if !bytes.Equal(row.Fingerprint, fingerprint) || !row.JobID.Valid || parseErr != nil {
			return nil, errors.New("SEARCH_REBUILD_OPERATION_CONFLICT")
		}
		job, err = repository.LoadJob(ctx, tx, id)
		if err != nil {
			return nil, err
		}
		// MySQL JSON会重排键；重新规范化后比较原descriptor，不对数据库文本验签。
		var persisted rebuildmeta.Descriptor
		if json.Unmarshal(job.Watermark, &persisted) != nil || persisted != descriptor || job.Generation != targetGeneration || job.Source != p.Source || job.Target != p.Target || job.SnapshotRef != snapshotRef {
			return nil, errors.New("SEARCH_REBUILD_JOB_INVALID")
		}
	} else {
		if request == nil {
			return nil, errors.New("SEARCH_REBUILD_NOT_ACCEPTED")
		}
		if current.Physical != p.Source || strconv.FormatUint(current.Generation, 10) != p.SourceGeneration || strconv.FormatUint(current.Revision, 10) != p.SourceRevision {
			return nil, errors.New("SEARCH_REBUILD_GENERATION_CHANGED")
		}
		if err := generation.CheckSwitching(ctx, tx); err != nil {
			return nil, err
		}
		// RPC仅核对已导入的受控副本可用；整库摘要由import检查，worker仍须流式核验。
		if !filepath.IsAbs(s.AcceptedDir) {
			return nil, errors.New("SEARCH_REBUILD_SNAPSHOT_UNAVAILABLE")
		}
````

## 第 15 步：JobService.run（接着实现）

插入位置：第 14 步代码之后，紧接追加；完成稿约第 325 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil &#124;&#124; !info.Mode().IsRegular()</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_REBUILD_SNAPSHOT_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>job.State == &quot;SUCCEEDED&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>tx.Commit() != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_REBUILD_COMMIT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=11710:12604 -->
````go
		path := filepath.Join(s.AcceptedDir, strings.TrimPrefix(snapshotRef, "rebuild-snapshot-v1:")+".entries.ndjson")
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() {
			return nil, errors.New("SEARCH_REBUILD_SNAPSHOT_UNAVAILABLE")
		}
		job = repository.Job{State: "SNAPSHOT", SnapshotRef: snapshotRef, Watermark: watermark, Source: p.Source, Target: p.Target, Generation: targetGeneration}
		job.ID, err = repository.AcceptJob(ctx, tx, operation, fingerprint, job, p.Topic, p.Starts)
		if err != nil {
			return nil, err
		}
	}
	var result *pb.OperationResult
	if job.State == "SUCCEEDED" {
		result, err = loadTerminalResult(ctx, tx, operation, p, job, fingerprint)
	} else {
		result, err = jobResult(operation, job)
	}
	if err != nil {
		return nil, err
	}
	if tx.Commit() != nil {
		return nil, errors.New("SEARCH_REBUILD_COMMIT_UNKNOWN")
	}
	return result, nil
}

````

## 第 16 步：jobResult

插入位置：第 15 步代码之后，紧接追加；完成稿约第 351 行。

本步声明：`func jobResult(operation string, job repository.Job) (*pb.OperationResult, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>operation</code> | <code>string</code> | 贯穿准备、导入、RPC接受和恢复的原操作号。<br>在此声明中的实际使用：<code>OperationId: operation</code> |
| <code>job</code> | <code>repository.Job</code> | 原SQL重建任务，状态与固定来源均须核验。<br>在此声明中的实际使用：<code>job.State</code>；<code>job.ID</code>；<code>job.SnapshotAfter</code>；<code>job.Generation</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.OperationResult</code> | 实际返回表达式：<code>nil</code>；<code>&amp;pb.OperationResult{OperationId: operation, Generation: job.Generation, State: pb.OperationResult_PROCESSING, CurrentStateJsonUtf8: current}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_REBUILD_TERMINAL_PROOF_UNAVAILABLE&quot;)</code>；<code>errors.New(&quot;SEARCH_REBUILD_JOB_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return nil, errors.New(&quot;SEARCH_REBUILD_TERMINAL_PROOF_UNAVAILABLE&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return nil, errors.New(&quot;SEARCH_REBUILD_JOB_INVALID&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return &amp;pb.OperationResult{OperationId: operation, Generation: job.Generation, State: pb.OperationResult_PROCESSING, CurrentStateJsonUtf8: current}, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=12604:13557 -->
````go
func jobResult(operation string, job repository.Job) (*pb.OperationResult, error) {
	switch job.State {
	case "SNAPSHOT", "REPLAYING", "VALIDATING", "SWITCHING":
	case "SUCCEEDED", "REJECTED":
		// 终态必须由worker持久证明封存后实现；仅状态字符串不足以制造固定成功结果。
		return nil, errors.New("SEARCH_REBUILD_TERMINAL_PROOF_UNAVAILABLE")
	default:
		return nil, errors.New("SEARCH_REBUILD_JOB_INVALID")
	}
	current, _ := json.Marshal(struct {
		Version       int    `json:"version"`
		JobID         string `json:"job_id"`
		State         string `json:"state"`
		SnapshotAfter string `json:"snapshot_after"`
	}{1, strconv.FormatUint(job.ID, 10), job.State, strconv.FormatUint(job.SnapshotAfter, 10)})
	return &pb.OperationResult{OperationId: operation, Generation: job.Generation, State: pb.OperationResult_PROCESSING, CurrentStateJsonUtf8: current}, nil
}

// prepare.go：保留原实现，按业务合并文件。

````

## 第 17 步：Logs

插入位置：第 16 步代码之后，紧接追加；完成稿约第 371 行。

本步声明：`type Logs interface {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Capture</code> | <code>(context.Context, *logsource.Identity) (logsource.Window, error)</code> | ctx控制取消；可选Identity指定原集群/主题UUID，nil用于新准备；返回三个分区的原始可读窗口。 |

<!-- write-step: 17 bytes=13557:13654 -->
````go
type Logs interface {
	Capture(context.Context, *logsource.Identity) (logsource.Window, error)
}
````

## 第 18 步：Aliases

插入位置：第 17 步代码之后，紧接追加；完成稿约第 374 行。

本步声明：`type Aliases interface {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ResolveAlias</code> | <code>(context.Context, string) (string, bool, error)</code> | ctx和稳定别名；返回唯一合法物理索引、是否存在及错误，不把404当依赖故障。 |

<!-- write-step: 18 bytes=13654:13742 -->
````go
type Aliases interface {
	ResolveAlias(context.Context, string) (string, bool, error)
}
````

## 第 19 步：Preparer

插入位置：第 18 步代码之后，紧接追加；完成稿约第 377 行。

本步声明：`type Preparer struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>DB</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>Logs</code> | <code>Logs</code> | 捕获原L及Kafka身份的能力。 |
| <code>Aliases</code> | <code>Aliases</code> | 确认真实ES别名仍与SQL来源匹配。 |
| <code>Signer</code> | <code>*rebuildartifact.Signer</code> | 新准备的签名器，可由LoadSigner延迟提供。 |
| <code>LoadSigner</code> | <code>func() (*rebuildartifact.Signer, error)</code> | 仅在新操作确需签发时读取签名器，原号恢复不依赖新证书仍有效。 |
| <code>Verifier</code> | <code>*rebuildartifact.Verifier</code> | 立即验证新签名封装，确认本服务信任链与身份。 |
| <code>Owner</code> | <code>string</code> | 当前部署owner，用于绑定及目标名派生。 |
| <code>Alias</code> | <code>string</code> | 本服务稳定搜索别名。 |

<!-- write-step: 19 bytes=13742:13984 -->
````go
type Preparer struct {
	DB           *sql.DB
	Logs         Logs
	Aliases      Aliases
	Signer       *rebuildartifact.Signer
	LoadSigner   func() (*rebuildartifact.Signer, error)
	Verifier     *rebuildartifact.Verifier
	Owner, Alias string
}

````

## 第 20 步：Preparer.Prepare

插入位置：第 19 步代码之后，紧接追加；完成稿约第 387 行。

Prepare先封存Kafka原L再允许Doc建立S。代次锁只覆盖有界准备，不跨Doc的整库扫描。

本步声明：`func (p *Preparer) Prepare(ctx context.Context, operation string, target uint64) ([]byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx, cancel := context.WithTimeout(ctx, 10*time.Second)</code>；<code>context.WithTimeout(ctx, 10*time.Second)</code>；<code>p.DB.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>；<code>generation.Find(ctx, tx, p.Alias, true)</code><br>另有 6 处使用，继续按下方完整代码的顺序处理。 |
| <code>operation</code> | <code>string</code> | 贯穿准备、导入、RPC接受和恢复的原操作号。<br>在此声明中的实际使用：<code>rebuildmeta.OperationValid(operation)</code>；<code>repository.Find(ctx, tx, operation)</code>；<code>p.Owner + &quot;\x00&quot; + p.Alias + &quot;\x00&quot; + operation</code>；<code>Operation: operation</code> |
| <code>target</code> | <code>uint64</code> | 请求的目标代次，必须是当前源代次加1。<br>在此声明中的实际使用：<code>target == 0</code>；<code>strconv.FormatUint(target, 10)</code>；<code>target != source.Generation+1</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>nil</code>；<code>original.Envelope</code>；<code>envelope</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_REBUILD_PREPARE_INVALID&quot;)</code>；<code>errors.New(&quot;SEARCH_REBUILD_TRANSACTION_UNAVAILABLE&quot;)</code>；<code>err</code>；<code>errors.New(&quot;SEARCH_GENERATION_MISSING&quot;)</code><br>另有 8 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>p.DB == nil &#124;&#124; p.Logs == nil &#124;&#124; p.Aliases == nil &#124;&#124; !rebuildmeta.OperationValid(operation) &#124;&#124; target == 0</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_REBUILD_PREPARE_INVALID&quot;)</code>。当前函数在这里结束。
- 执行 <code>context.WithTimeout(ctx, 10*time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>p.DB.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_REBUILD_TRANSACTION_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>!exists</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_GENERATION_MISSING&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>found</code>。
- 检查 <code>original.Prepare.Owner != p.Owner &#124;&#124; original.Prepare.Alias != p.Alias &#124;&#124; original.Prepare.TargetGeneration != strconv.FormatUint(target, 10)</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_REBUILD_OPERATION_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>original.State == &quot;REJECTED&quot;</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_REBUILD_PREPARE_REJECTED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return original.Envelope, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>source.Generation == math.MaxUint64 &#124;&#124; source.Revision == math.MaxUint64 &#124;&#124; target != source.Generation+1</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_REBUILD_GENERATION_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。

<!-- write-step: 20 bytes=13984:15714 -->
````go
// Prepare先封存Kafka原L再允许Doc建立S。代次锁只覆盖有界准备，不跨Doc的整库扫描。
func (p *Preparer) Prepare(ctx context.Context, operation string, target uint64) ([]byte, error) {
	if p.DB == nil || p.Logs == nil || p.Aliases == nil || !rebuildmeta.OperationValid(operation) || target == 0 {
		return nil, errors.New("SEARCH_REBUILD_PREPARE_INVALID")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	tx, err := p.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, errors.New("SEARCH_REBUILD_TRANSACTION_UNAVAILABLE")
	}
	defer tx.Rollback()
	source, exists, err := generation.Find(ctx, tx, p.Alias, true)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.New("SEARCH_GENERATION_MISSING")
	}
	original, found, err := repository.Find(ctx, tx, operation)
	if err != nil {
		return nil, err
	}
	if found {
		if original.Prepare.Owner != p.Owner || original.Prepare.Alias != p.Alias || original.Prepare.TargetGeneration != strconv.FormatUint(target, 10) {
			return nil, errors.New("SEARCH_REBUILD_OPERATION_CONFLICT")
		}
		if original.State == "REJECTED" {
			return nil, errors.New("SEARCH_REBUILD_PREPARE_REJECTED")
		}
		return original.Envelope, nil
	}
	if source.Generation == math.MaxUint64 || source.Revision == math.MaxUint64 || target != source.Generation+1 {
		return nil, errors.New("SEARCH_REBUILD_GENERATION_INVALID")
	}
	if err := generation.CheckSwitching(ctx, tx); err != nil {
		return nil, err
	}
	if err := repository.CheckAvailable(ctx, tx, p.Alias); err != nil {
		return nil, err
	}
	actual, present, err := p.Aliases.ResolveAlias(ctx, p.Alias)
	if err != nil {
		return nil, err
	}
````

## 第 21 步：Preparer.Prepare（接着实现）

插入位置：第 20 步代码之后，紧接追加；完成稿约第 432 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>!present &#124;&#124; actual != source.Physical</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_ALIAS_GENERATION_MISMATCH&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_REBUILD_NONCE_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>signer == nil &amp;&amp; p.LoadSigner != nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>signer == nil &#124;&#124; p.Verifier == nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_REBUILD_SIGNER_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>tx.Commit() != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_REBUILD_PREPARE_COMMIT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 21 bytes=15714:17450 -->
````go
	if !present || actual != source.Physical {
		return nil, errors.New("SEARCH_ALIAS_GENERATION_MISMATCH")
	}
	window, err := p.Logs.Capture(ctx, nil)
	if err != nil {
		return nil, err
	}
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, errors.New("SEARCH_REBUILD_NONCE_UNAVAILABLE")
	}
	name := sha256.Sum256([]byte(p.Owner + "\x00" + p.Alias + "\x00" + operation))
	input := rebuildmeta.Prepare{Version: 1, Operation: operation, Owner: p.Owner, Alias: p.Alias, Source: source.Physical, SourceGeneration: strconv.FormatUint(source.Generation, 10), SourceRevision: strconv.FormatUint(source.Revision, 10),
		Target: "rebuild-" + hex.EncodeToString(name[:]), TargetGeneration: strconv.FormatUint(target, 10), Cluster: window.Identity.Cluster, Topic: window.Identity.Topic, TopicID: hex.EncodeToString(window.Identity.TopicID[:]), Nonce: hex.EncodeToString(nonce[:])}
	for i, part := range window.Partitions {
		input.Starts[i] = strconv.FormatInt(part.Last, 10)
	}
	payload, err := input.Bytes()
	if err != nil {
		return nil, err
	}
	signer := p.Signer
	if signer == nil && p.LoadSigner != nil {
		signer, err = p.LoadSigner()
		if err != nil {
			return nil, err
		}
	}
	if signer == nil || p.Verifier == nil {
		return nil, errors.New("SEARCH_REBUILD_SIGNER_UNAVAILABLE")
	}
	envelope, err := signer.Sign(rebuildartifact.PrepareV1, payload)
	if err != nil {
		return nil, err
	}
	verified, err := p.Verifier.Verify(envelope, rebuildartifact.PrepareV1)
	if err != nil {
		return nil, err
	}
	if err := repository.Insert(ctx, tx, input, envelope, verified.SignerFingerprint); err != nil {
		return nil, err
	}
	if tx.Commit() != nil {
		return nil, errors.New("SEARCH_REBUILD_PREPARE_COMMIT_UNKNOWN")
	}
````

## 第 22 步：Preparer.Prepare（接着实现）

插入位置：第 21 步代码之后，紧接追加；完成稿约第 477 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return envelope, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 22 bytes=17450:17536 -->
````go
	return envelope, nil
}

// validate.go：保留原实现，按业务合并文件。

````

## 第 23 步：ValidationIndexes

插入位置：第 22 步代码之后，紧接追加；完成稿约第 482 行。

本步声明：`type ValidationIndexes interface {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Get</code> | <code>(context.Context, string, uint64) (index.Authority, error)</code> | ctx、目标物理索引、文档ID；返回严格校验后的实际权威内容，隐藏墓碑不得带正文。 |
| <code>RefreshCount</code> | <code>(context.Context, string) (uint64, error)</code> | ctx和目标物理索引；刷新后读取实际文档总数，核对无漏项或多余项。 |

<!-- write-step: 23 bytes=17536:17692 -->
````go
type ValidationIndexes interface {
	Get(context.Context, string, uint64) (index.Authority, error)
	RefreshCount(context.Context, string) (uint64, error)
}

````

## 第 24 步：Validator

插入位置：第 23 步代码之后，紧接追加；完成稿约第 487 行。

本步声明：`type Validator struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Seed</code> | <code>*Seeder</code> | 提供原准备、导入快照及job锁定能力。 |
| <code>Indexes</code> | <code>ValidationIndexes</code> | 读取并严格校验目标ES记录及计数的能力。 |

<!-- write-step: 24 bytes=17692:17763 -->
````go
type Validator struct {
	Seed    *Seeder
	Indexes ValidationIndexes
}

````

## 第 25 步：Validator.Step

插入位置：第 24 步代码之后，紧接追加；完成稿约第 492 行。

Step先证明原三个分区已经到H，再按原快照覆盖和全部物理应用两阶段核验。
每页与原job锁共同提交；正文逐篇读取，不把整个索引加载进内存。

本步声明：`func (v *Validator) Step(ctx context.Context, operation string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx, cancel := context.WithTimeout(ctx, 30*time.Second)</code>；<code>context.WithTimeout(ctx, 30*time.Second)</code>；<code>v.Seed.DB.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>；<code>v.Seed.lock(ctx, tx, operation)</code><br>另有 12 处使用，继续按下方完整代码的顺序处理。 |
| <code>operation</code> | <code>string</code> | 贯穿准备、导入、RPC接受和恢复的原操作号。<br>在此声明中的实际使用：<code>v.Seed.lock(ctx, tx, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_REBUILD_DEPENDENCIES_UNAVAILABLE&quot;)</code>；<code>errors.New(&quot;SEARCH_REBUILD_TRANSACTION_UNAVAILABLE&quot;)</code>；<code>err</code>；<code>errors.New(&quot;SEARCH_REBUILD_PROGRESS_CONFLICT&quot;)</code><br>另有 9 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>v == nil &#124;&#124; v.Seed == nil &#124;&#124; v.Seed.DB == nil &#124;&#124; v.Indexes == nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_DEPENDENCIES_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 执行 <code>context.WithTimeout(ctx, 30*time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>v.Seed.DB.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_TRANSACTION_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>input.job.State != &quot;SWITCHING&quot;</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_PROGRESS_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>tx.QueryRowContext(ctx, `SELECT next_offset FROM consumer_checkpoint WHERE consumer_name=? AND kafka_topic=? AND partition_id=?`, consumer, input.prepare.Topic, p).Scan(&amp;next) != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_CHECKPOINT_MISSING&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT next_offset FROM consumer_checkpoint WHERE consumer_name=? AND kafka_topic=? AND partition_id=?`, consumer, input.prepare.Topic, p).Scan(&amp;next)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT next_offset FROM consumer_checkpoint WHERE consumer_name=? AND kafka_topic=? AND partition_id=?`, consumer, input.prepare.Topic, p)</code>。
- 检查 <code>next &lt; end</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>next != end</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_FRONTIER_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; progress.Complete</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>progress.Phase == &quot;SNAPSHOT&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。

<!-- write-step: 25 bytes=17763:19483 -->
````go
// Step先证明原三个分区已经到H，再按原快照覆盖和全部物理应用两阶段核验。
// 每页与原job锁共同提交；正文逐篇读取，不把整个索引加载进内存。
func (v *Validator) Step(ctx context.Context, operation string) error {
	if v == nil || v.Seed == nil || v.Seed.DB == nil || v.Indexes == nil {
		return errors.New("SEARCH_REBUILD_DEPENDENCIES_UNAVAILABLE")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	tx, err := v.Seed.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return errors.New("SEARCH_REBUILD_TRANSACTION_UNAVAILABLE")
	}
	defer tx.Rollback()
	input, err := v.Seed.lock(ctx, tx, operation)
	if err != nil {
		return err
	}
	if input.job.State != "SWITCHING" {
		return errors.New("SEARCH_REBUILD_PROGRESS_CONFLICT")
	}
	frontier, err := repository.LoadFrontier(ctx, tx, input.job.ID)
	if err != nil {
		return err
	}
	consumer := repository.ConsumerName(input.job.ID)
	for p, end := range frontier {
		var next int64
		if tx.QueryRowContext(ctx, `SELECT next_offset FROM consumer_checkpoint WHERE consumer_name=? AND kafka_topic=? AND partition_id=?`, consumer, input.prepare.Topic, p).Scan(&next) != nil {
			return errors.New("SEARCH_REBUILD_CHECKPOINT_MISSING")
		}
		if next < end {
			return nil
		}
		if next != end {
			return errors.New("SEARCH_REBUILD_FRONTIER_INVALID")
		}
	}
	progress, err := repository.LoadValidation(ctx, tx, input.job.ID)
	if err != nil || progress.Complete {
		return err
	}
	if progress.Phase == "SNAPSHOT" {
		page, err := snapshotPage(ctx, v.Seed.AcceptedDir, input.job.SnapshotRef, input.imported.Seal, progress.After)
		if err != nil {
			return err
		}
````

## 第 26 步：Validator.Step（接着实现）

插入位置：第 25 步代码之后，紧接追加；完成稿约第 538 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>tx.QueryRowContext(ctx, `SELECT applied_seq FROM index_application WHERE physical_index=? AND document_id=?`, input.job.Target, entry.Document).Scan(&amp;applied) != nil &#124;&#124; applied &lt; entry.Minimum</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_SNAPSHOT_COVERAGE_FAILED&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT applied_seq FROM index_application WHERE physical_index=? AND document_id=?`, input.job.Target, entry.Document).Scan(&amp;applied)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT applied_seq FROM index_application WHERE physical_index=? AND document_id=?`, input.job.Target, entry.Document)</code>。
- 检查 <code>len(page) == 0</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>actual.Sequence != s.Sequence &#124;&#124; actual.Digest != s.Digest &#124;&#124; actual.Release != s.Release &#124;&#124; actual.Visible != s.Visible &#124;&#124; actual.Policy != s.Policy &#124;&#124; s.Digest == (&#91;32&#93;byte{})</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_TARGET_MISMATCH&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_VALIDATION_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>count == ^uint64(0)</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_VALIDATION_INVALID&quot;)</code>。当前函数在这里结束。

<!-- write-step: 26 bytes=19483:21193 -->
````go
		for _, entry := range page {
			var applied uint64
			if tx.QueryRowContext(ctx, `SELECT applied_seq FROM index_application WHERE physical_index=? AND document_id=?`, input.job.Target, entry.Document).Scan(&applied) != nil || applied < entry.Minimum {
				return errors.New("SEARCH_REBUILD_SNAPSHOT_COVERAGE_FAILED")
			}
			progress.After = entry.Document
		}
		if len(page) == 0 {
			progress.Phase = "APPLICATION"
			progress.After = 0
		}
		if err := repository.SaveValidation(ctx, tx, input.job.ID, progress, false); err != nil {
			return err
		}
	} else {
		page, err := repository.ValidationApplications(ctx, tx, input.job.Target, progress.After)
		if err != nil {
			return err
		}
		count, _ := strconv.ParseUint(progress.Count, 10, 64)
		for _, application := range page {
			actual, err := v.Indexes.Get(ctx, input.job.Target, application.Document)
			if err != nil {
				return err
			}
			s := application.State
			if actual.Sequence != s.Sequence || actual.Digest != s.Digest || actual.Release != s.Release || actual.Visible != s.Visible || actual.Policy != s.Policy || s.Digest == ([32]byte{}) {
				return errors.New("SEARCH_REBUILD_TARGET_MISMATCH")
			}
			// Get已验证隐藏墓碑无正文；摘要链同时覆盖实际完整正文及逐文档SQL绑定。
			raw, err := json.Marshal(actual)
			if err != nil {
				return errors.New("SEARCH_REBUILD_VALIDATION_INVALID")
			}
			previous, _ := hex.DecodeString(progress.Digest)
			h := sha256.New()
			h.Write(previous)
			h.Write(raw)
			progress.Digest = hex.EncodeToString(h.Sum(nil))
			if count == ^uint64(0) {
				return errors.New("SEARCH_REBUILD_VALIDATION_INVALID")
			}
			count++
			progress.After = application.Document
		}
````

## 第 27 步：Validator.Step（接着实现）

插入位置：第 26 步代码之后，紧接追加；完成稿约第 583 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>complete</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM consumed_event e LEFT JOIN event_origin o ON o.consumer_name=e.consumer_name AND o.event_id=e.event_id WHERE e.consumer_name=? AND (e. …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_LOG_COVERAGE_FAILED&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM consumed_event e LEFT JOIN event_origin o ON o.consumer_name=e.consumer_name AND o.event_id=e.event_id WHERE e.consumer_name=? AND (e. …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM consumed_event e LEFT JOIN event_origin o ON o.consumer_name=e.consumer_name AND o.event_id=e.event_id WHERE e.consumer_name=? AND (e. …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>actualCount != count</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_TARGET_COUNT_MISMATCH&quot;)</code>。当前函数在这里结束。
- 检查 <code>count == 0</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>tx.Commit() != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_REBUILD_COMMIT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 27 bytes=21193:22476 -->
````go
		progress.Count = strconv.FormatUint(count, 10)
		complete := len(page) == 0
		if complete {
			if err := event.CheckRecovery(ctx, tx, consumer, input.prepare.Topic); err != nil {
				return err
			}
			var pending uint64
			if tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM consumed_event e LEFT JOIN event_origin o ON o.consumer_name=e.consumer_name AND o.event_id=e.event_id WHERE e.consumer_name=? AND (e.state<>'SUCCEEDED' OR o.applied_index IS NULL OR o.applied_index<>?)`, consumer, input.job.Target).Scan(&pending) != nil || pending != 0 {
				return errors.New("SEARCH_REBUILD_LOG_COVERAGE_FAILED")
			}
			actualCount, err := v.Indexes.RefreshCount(ctx, input.job.Target)
			if err != nil {
				return err
			}
			if actualCount != count {
				return errors.New("SEARCH_REBUILD_TARGET_COUNT_MISMATCH")
			}
			// 空快照/空日志同样有非零封存证明，不能让全零摘要冒充未验证状态。
			if count == 0 {
				digest := sha256.Sum256([]byte("search-rebuild-empty-validation-v1"))
				progress.Digest = hex.EncodeToString(digest[:])
			}
		}
		if err := repository.SaveValidation(ctx, tx, input.job.ID, progress, complete); err != nil {
			return err
		}
	}
	if tx.Commit() != nil {
		return errors.New("SEARCH_REBUILD_COMMIT_UNKNOWN")
	}
	return nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/search/internal/logic/rebuild/preparation_service.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/search/04-rebuild-snapshot.md)。
