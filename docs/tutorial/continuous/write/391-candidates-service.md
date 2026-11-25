# 分步编写 `services/search/internal/logic/candidates/candidates_service.go`

先按 [S03 · 签名批锚点、PIT与当前权限二次过滤](../lessons/search/03-candidates-cursor.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/391-candidates-service.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

组织单链PIT预留、查询确认、签名NEXT/REPLAY及未知资源关闭恢复。

SQL与ES不可原子提交，必须先持久预留再分配PIT，用nonce拒绝迟到响应，并在不确定轮换时保留配额到原到期。

写之前先按这条执行路线理解各段的连接：

1. SearchCandidates固定批100，先限并发2；新链reserve在排他代次锁下登记OPENING后才申请PIT
2. 按chain hash互斥，在generation.Do中beginQuery核原query/scope/session/代次/期限并登记nonce
3. Open成功先retainOpened落SQL，再Candidates；REPLAY比较完整批digest/count/more/end，confirmQuery持原nonce写IDLE才回复
4. fail仅做有界SQL补偿转CLOSING，迟到补偿不能覆盖新nonce或已确认IDLE
5. closeHash独立关闭槽：锁内领取→锁外关ES→锁内原claim确认；unknown轮换保留占额至原Expires
6. Cleanup每轮16项且跳锁，Ready要求已恢复并重新Check数据库

本文件需要守住的边界：

- Search候选不是当前读权限，Doc需对每个候选二次核权再读固定正式正文
- 没有批次历史缓存；REPLAY在同PIT重新取批并核摘要
- 绝对到期只证明应用链不能再用，不保证ES reaper立即释放所有context
- 不能跨ES调用持SQL行锁或靠删除未知链释放配额

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/search/internal/logic/candidates' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/search/internal/logic/candidates/candidates_service.go') -and (Get-Item -LiteralPath 'services/search/internal/logic/candidates/candidates_service.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/search/internal/logic/candidates/candidates_service.go')) { New-Item -ItemType File -Path 'services/search/internal/logic/candidates/candidates_service.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:605 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package candidates

import (
	"bytes"
	pb "collaboration/contracts/internalv1"
	"collaboration/search/internal/client/elastic"
	domain "collaboration/search/internal/domain/cursor"
	repository "collaboration/search/internal/repository/cursor"
	generation "collaboration/search/internal/repository/generation"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// candidates.go：保留原实现，按业务合并文件。

````

## 第 2 步：Generations

插入位置：第 1 步代码之后，紧接追加；完成稿约第 25 行。

本步声明：`type Generations interface {`。

**这一段为什么这样写：** 候选查询期间固定物理索引代次。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Ready</code> | <code>(context.Context) error</code> | ctx→核当前ES/SQL别名一致和无待恢复切换 |
| <code>Do</code> | <code>(context.Context, func(context.Context, generation.Target) error) error</code> | ctx、func(ctx,Target)→错误；完整查询期间持代次共享锁 |

<!-- write-step: 2 bytes=605:742 -->
````go
type Generations interface {
	Ready(context.Context) error
	Do(context.Context, func(context.Context, generation.Target) error) error
}

````

## 第 3 步：Indexes

插入位置：第 2 步代码之后，紧接追加；完成稿约第 30 行。

本步声明：`type Indexes interface {`。

**这一段为什么这样写：** PIT客户端协议，所有时长都来自原绝对期限剩余量。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>OpenPIT</code> | <code>(context.Context, string, time.Duration) (string, error)</code> | ctx、physical、remaining→PIT句柄及错误；错误可能仍带关闭线索 |
| <code>Candidates</code> | <code>(context.Context, string, string, json.RawMessage, int, time.Duration) (elastic.CandidateBatch, error)</code> | ctx、pit、query、原after JSON、批size、remaining→CandidateBatch和错误 |
| <code>ClosePIT</code> | <code>(context.Context, string) error</code> | ctx、pit→明确关闭成功或未知错误 |

<!-- write-step: 3 bytes=742:990 -->
````go
type Indexes interface {
	OpenPIT(context.Context, string, time.Duration) (string, error)
	Candidates(context.Context, string, string, json.RawMessage, int, time.Duration) (elastic.CandidateBatch, error)
	ClosePIT(context.Context, string) error
}

````

## 第 4 步：Candidates

插入位置：第 3 步代码之后，紧接追加；完成稿约第 36 行。

Candidates拥有单链资源和跨系统提交编排；SQL不假装与ES原子，索引候选也不等于读取权限。

本步声明：`type Candidates struct {`。

**这一段为什么这样写：** 游标链业务状态和本进程资源限流。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>database</code> | <code>*sql.DB</code> | Search私有SQL注册表 |
| <code>generation</code> | <code>Generations</code> | ES/SQL代次共享锁接口 |
| <code>indexes</code> | <code>Indexes</code> | 真实PIT客户端 |
| <code>alias</code> | <code>string</code> | 逻辑索引别名 |
| <code>codec</code> | <code>*domain.Codec</code> | 签名批锚点HMAC编码器 |
| <code>ttl</code> | <code>time.Duration</code> | 建链绝对寿命，最多2分钟 |
| <code>admission</code> | <code>chan struct{}</code> | 最多2个并行候选查询槽 |
| <code>queries</code> | <code>keyLocks</code> | 同chain查询串行锁 |
| <code>closingAdmission</code> | <code>chan struct{}</code> | 显式Close与worker共享的1个关闭槽 |
| <code>sweepMu</code> | <code>sync.Mutex</code> | 防Cleanup轮次重叠 |
| <code>recovered</code> | <code>atomic.Bool</code> | 至少一次完整Cleanup成功的进程标志 |

<!-- write-step: 4 bytes=990:1463 -->
````go
// Candidates拥有单链资源和跨系统提交编排；SQL不假装与ES原子，索引候选也不等于读取权限。
type Candidates struct {
	database         *sql.DB
	generation       Generations
	indexes          Indexes
	alias            string
	codec            *domain.Codec
	ttl              time.Duration
	admission        chan struct{}
	queries          keyLocks
	closingAdmission chan struct{}
	sweepMu          sync.Mutex
	recovered        atomic.Bool
}

````

## 第 5 步：New

插入位置：第 4 步代码之后，紧接追加；完成稿约第 51 行。

本步声明：`func New(database *sql.DB, generations Generations, indexes Indexes, alias string, key []byte, ttl time.Duration) (*Candidates, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>database</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>database == nil</code>；<code>database: database</code> |
| <code>generations</code> | <code>Generations</code> | 使用 <code>Generations</code> 类型；其定义处的职责：候选查询期间固定物理索引代次。<br>在此声明中的实际使用：<code>generations == nil</code>；<code>generation: generations</code> |
| <code>indexes</code> | <code>Indexes</code> | 使用 <code>Indexes</code> 类型；其定义处的职责：PIT客户端协议，所有时长都来自原绝对期限剩余量。<br>在此声明中的实际使用：<code>indexes == nil</code>；<code>indexes: indexes</code> |
| <code>alias</code> | <code>string</code> | 在此声明中的实际使用：<code>alias == &quot;&quot;</code>；<code>alias: alias</code> |
| <code>key</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>domain.NewCodec(key)</code> |
| <code>ttl</code> | <code>time.Duration</code> | 一段持续时间，按 time.Second 等单位运算；与表示某个瞬间的 time.Time 区别开。<br>在此声明中的实际使用：<code>ttl &lt; time.Millisecond</code>；<code>ttl &gt; 2*time.Minute</code>；<code>ttl: ttl</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Candidates</code> | 使用 <code>Candidates</code> 类型；其定义处的职责：游标链业务状态和本进程资源限流。<br>实际返回表达式：<code>nil</code>；<code>&amp;Candidates{database: database, generation: generations, indexes: indexes, alias: alias, codec: codec, ttl: ttl, 		admission: make(chan struct{}, 2), closingAdmission: make(chan struct{}, 1)}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_CANDIDATE_CONFIG_INVALID&quot;)</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>database == nil &#124;&#124; generations == nil &#124;&#124; indexes == nil &#124;&#124; alias == &quot;&quot; &#124;&#124; ttl &lt; time.Millisecond &#124;&#124; ttl &gt; 2*time.Minute</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_CANDIDATE_CONFIG_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return &amp;Candidates{database: database, generation: generations, indexes: indexes, alias: alias, codec: codec, ttl: ttl, 		admission: make(chan struct{}, 2), closingAdmission: make(chan struct{}, 1)}, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1463:2071 -->
````go
func New(database *sql.DB, generations Generations, indexes Indexes, alias string, key []byte, ttl time.Duration) (*Candidates, error) {
	if database == nil || generations == nil || indexes == nil || alias == "" || ttl < time.Millisecond || ttl > 2*time.Minute {
		return nil, errors.New("SEARCH_CANDIDATE_CONFIG_INVALID")
	}
	codec, err := domain.NewCodec(key)
	if err != nil {
		return nil, err
	}
	return &Candidates{database: database, generation: generations, indexes: indexes, alias: alias, codec: codec, ttl: ttl,
		admission: make(chan struct{}, 2), closingAdmission: make(chan struct{}, 1)}, nil
}

````

## 第 6 步：Candidates.SearchCandidates

插入位置：第 5 步代码之后，紧接追加；完成稿约第 63 行。

本步声明：`func (c *Candidates) SearchCandidates(ctx context.Context, request *pb.SearchCandidatesRequest) (*pb.SearchCandidatesReply, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx, cancel := context.WithTimeout(ctx, 5*time.Second)</code>；<code>context.WithTimeout(ctx, 5*time.Second)</code>；<code>c.enter(ctx)</code>；<code>c.reserve(ctx, query)</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |
| <code>request</code> | <code>*pb.SearchCandidatesRequest</code> | 在此声明中的实际使用：<code>domain.ValidateQuery(request)</code>；<code>request.Cursor</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.SearchCandidatesReply</code> | 实际返回表达式：<code>nil</code>；<code>reply</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;SEARCH_CANDIDATE_BATCH_SIZE_INVALID&quot;)</code>；<code>errors.New(&quot;SEARCH_CANDIDATE_REPLY_UNKNOWN&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>query.PageSize != 100</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_CANDIDATE_BATCH_SIZE_INVALID&quot;)</code>。当前函数在这里结束。
- 执行 <code>context.WithTimeout(ctx, 5*time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>c.leave()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>newChain</code>。
- 检查 <code>err == nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>release()</code>，当前函数退出时执行，提前 return 也会触发。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。

<!-- write-step: 6 bytes=2071:3427 -->
````go
func (c *Candidates) SearchCandidates(ctx context.Context, request *pb.SearchCandidatesRequest) (*pb.SearchCandidatesReply, error) {
	query, err := domain.ValidateQuery(request)
	if err != nil {
		return nil, err
	}
	// HTTP page大小属于Doc；Search固定批100，避免小批More与已冻结批锚点出现两套语义。
	if query.PageSize != 100 {
		return nil, errors.New("SEARCH_CANDIDATE_BATCH_SIZE_INVALID")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := c.enter(ctx); err != nil {
		return nil, err
	}
	defer c.leave()
	var anchor domain.Batch
	var action domain.Action
	var record repository.Record
	newChain := len(request.Cursor) == 0
	if newChain {
		record, anchor.Chain, err = c.reserve(ctx, query)
	} else {
		var signed []byte
		action, signed, err = domain.ParseEnvelope(request.Cursor)
		if err == nil {
			anchor, err = c.codec.Open(signed)
		}
	}
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(anchor.Chain[:])
	release, err := c.queries.acquire(ctx, hash)
	if err != nil {
		return nil, err
	}
	defer release()
	var reply *pb.SearchCandidatesReply
	err = c.generation.Do(ctx, func(ctx context.Context, target generation.Target) error {
		var err error
		record, err = c.beginQuery(ctx, hash, query, target, newChain)
		if err != nil {
			return err
		}
		before := anchor.After
````

## 第 7 步：Candidates.SearchCandidates（接着实现）

插入位置：第 6 步代码之后，紧接追加；完成稿约第 108 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：检查 <code>newChain &#124;&#124; action == domain.ActionReplay</code>。
- 在本段的匿名函数内：检查 <code>newChain</code>。
- 在本段的匿名函数内：检查 <code>openErr != nil</code>。
- 在本段的匿名函数内：此处通过 <code>return openErr</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>err != nil</code>。
- 在本段的匿名函数内：此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>batch.PIT != &quot;&quot;</code>。
- 在本段的匿名函数内：检查 <code>queryErr != nil</code>。
- 在本段的匿名函数内：此处通过 <code>return queryErr</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>candidate.PhysicalIndex != record.Physical</code>。
- 在本段的匿名函数内：此处通过 <code>return errors.New(&quot;SEARCH_CURSOR_PHYSICAL_MISMATCH&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>err != nil</code>。
- 在本段的匿名函数内：此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>!newChain &amp;&amp; action == domain.ActionReplay &amp;&amp; !sameBatch(anchor, batch, digest)</code>。
- 在本段的匿名函数内：此处通过 <code>return errors.New(&quot;SEARCH_CURSOR_REPLAY_CHANGED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>err != nil</code>。
- 在本段的匿名函数内：此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>err != nil</code>。

<!-- write-step: 7 bytes=3427:5061 -->
````go
		if newChain || action == domain.ActionReplay {
			before = anchor.Before
		}
		if newChain {
			pit, openErr := c.indexes.OpenPIT(ctx, record.Physical, time.Until(record.Expires))
			if openErr != nil {
				c.fail(ctx, record, pit, true)
				return openErr
			}
			record.PIT = pit
			// Open成功的句柄先落SQL再查候选；中间退出仍能沿原行关闭，不重复分配资源。
			if err := c.retainOpened(ctx, record); err != nil {
				c.fail(ctx, record, pit, false)
				return err
			}
		}
		batch, queryErr := c.indexes.Candidates(ctx, record.PIT, query.Query, before, 100, time.Until(record.Expires))
		if batch.PIT != "" {
			record.PIT = batch.PIT
		}
		if queryErr != nil {
			c.fail(ctx, record, record.PIT, true)
			return queryErr
		}
		for _, candidate := range batch.Candidates {
			if candidate.PhysicalIndex != record.Physical {
				c.fail(ctx, record, record.PIT, true)
				return errors.New("SEARCH_CURSOR_PHYSICAL_MISMATCH")
			}
		}
		digest, err := domain.DigestCandidates(before, batch.End, batch.Candidates, batch.More)
		if err != nil {
			c.fail(ctx, record, record.PIT, true)
			return err
		}
		if !newChain && action == domain.ActionReplay && !sameBatch(anchor, batch, digest) {
			c.fail(ctx, record, record.PIT, false)
			return errors.New("SEARCH_CURSOR_REPLAY_CHANGED")
		}
		signed, err := c.codec.Seal(domain.Batch{Chain: anchor.Chain, Before: before, After: batch.End, Count: uint8(len(batch.Candidates)), More: batch.More, Digest: digest})
		if err != nil {
			c.fail(ctx, record, record.PIT, false)
			return err
		}
		if err := c.confirmQuery(ctx, record, batch.End); err != nil {
````

## 第 8 步：Candidates.SearchCandidates（接着实现）

插入位置：第 7 步代码之后，紧接追加；完成稿约第 153 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。
- 检查 <code>newChain</code>。
- 此处通过 <code>return nil, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>ctx.Err() != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_CANDIDATE_REPLY_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return reply, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=5061:5689 -->
````go
			c.fail(ctx, record, record.PIT, true)
			return err
		}
		reply = &pb.SearchCandidatesReply{Candidates: batch.Candidates, Continuation: signed, IndexGeneration: record.Generation,
			PhysicalIndex: record.Physical, ExpiresUnixMs: record.Expires.UnixMilli(), HasMore: batch.More, Complete: true}
		return nil
	})
	if err != nil {
		// 代次拒绝发生在ES调用之前时，同样收尾原预留；没有新查询可继续这条链。
		if newChain {
			c.fail(ctx, record, record.PIT, true)
		}
		return nil, err
	}
	if ctx.Err() != nil {
		return nil, errors.New("SEARCH_CANDIDATE_REPLY_UNKNOWN")
	}
	return reply, nil
}

````

## 第 9 步：Candidates.enter

插入位置：第 8 步代码之后，紧接追加；完成稿约第 173 行。

本步声明：`func (c *Candidates) enter(ctx context.Context) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx.Done</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>errors.New(&quot;SEARCH_CANDIDATE_ADMISSION_UNAVAILABLE&quot;)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return errors.New(&quot;SEARCH_CANDIDATE_ADMISSION_UNAVAILABLE&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=5689:5995 -->
````go
func (c *Candidates) enter(ctx context.Context) error {
	// 先限候选并发，再申请任何SQL锁；池8连接要给消费、Ready和关闭器保留资源。
	select {
	case c.admission <- struct{}{}:
		return nil
	case <-ctx.Done():
		return errors.New("SEARCH_CANDIDATE_ADMISSION_UNAVAILABLE")
	}
}
````

## 第 10 步：Candidates.leave

插入位置：第 9 步代码之后，紧接追加；完成稿约第 182 行。

本步声明：`func (c *Candidates) leave()`。

<!-- write-step: 10 bytes=5995:6043 -->
````go
func (c *Candidates) leave() { <-c.admission }

````

## 第 11 步：randomToken

插入位置：第 10 步代码之后，紧接追加；完成稿约第 184 行。

本步声明：`func randomToken() (string, error)`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;&quot;</code>；<code>hex.EncodeToString(raw&#91;:&#93;)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_CURSOR_ENTROPY_UNAVAILABLE&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;&quot;, errors.New(&quot;SEARCH_CURSOR_ENTROPY_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return hex.EncodeToString(raw&#91;:&#93;), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=6043:6251 -->
````go
func randomToken() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", errors.New("SEARCH_CURSOR_ENTROPY_UNAVAILABLE")
	}
	return hex.EncodeToString(raw[:]), nil
}

````

## 第 12 步：operationDeadline

插入位置：第 11 步代码之后，紧接追加；完成稿约第 192 行。

本步声明：`func operationDeadline(ctx context.Context, expires time.Time) int64`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx.Deadline</code> |
| <code>expires</code> | <code>time.Time</code> | 一个时间点。判断期限时要与当前时间比较，不能把它当成持续时长再任意续期。<br>在此声明中的实际使用：<code>expires.Before</code>；<code>deadline = expires</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>int64</code> | 实际返回表达式：<code>deadline.UnixMilli()</code> |

**跟着本段代码走一遍：**

- 检查 <code>!ok &#124;&#124; expires.Before(deadline)</code>。
- 此处通过 <code>return deadline.UnixMilli()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=6251:6448 -->
````go
func operationDeadline(ctx context.Context, expires time.Time) int64 {
	deadline, ok := ctx.Deadline()
	if !ok || expires.Before(deadline) {
		deadline = expires
	}
	return deadline.UnixMilli()
}

````

## 第 13 步：Candidates.transact

插入位置：第 12 步代码之后，紧接追加；完成稿约第 200 行。

本步声明：`func (c *Candidates) transact(ctx context.Context, work func(*sql.Tx) error) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>c.database.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code> |
| <code>work</code> | <code>func(*sql.Tx) error</code> | 在此声明中的实际使用：<code>work(tx)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_CURSOR_TRANSACTION_UNAVAILABLE&quot;)</code>；<code>err</code>；<code>errors.New(&quot;SEARCH_CURSOR_COMMIT_UNKNOWN&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>c.database.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_CURSOR_TRANSACTION_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>tx.Commit() != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_CURSOR_COMMIT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=6448:6927 -->
````go
func (c *Candidates) transact(ctx context.Context, work func(*sql.Tx) error) error {
	tx, err := c.database.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return errors.New("SEARCH_CURSOR_TRANSACTION_UNAVAILABLE")
	}
	defer tx.Rollback()
	if err := work(tx); err != nil {
		return err
	}
	if tx.Commit() != nil {
		return errors.New("SEARCH_CURSOR_COMMIT_UNKNOWN")
	}
	return nil
}

// closing.go：保留原实现，按业务合并文件。

````

## 第 14 步：Candidates.CloseCursor

插入位置：第 13 步代码之后，紧接追加；完成稿约第 217 行。

本步声明：`func (c *Candidates) CloseCursor(ctx context.Context, request *pb.CloseCursorRequest) (*pb.OperationResult, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx, cancel := context.WithTimeout(ctx, 5*time.Second)</code>；<code>context.WithTimeout(ctx, 5*time.Second)</code>；<code>c.closeHash(ctx, hash, request.Scope)</code> |
| <code>request</code> | <code>*pb.CloseCursorRequest</code> | 在此声明中的实际使用：<code>request == nil</code>；<code>request.Scope</code>；<code>request.Cursor</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.OperationResult</code> | 实际返回表达式：<code>nil</code>；<code>&amp;pb.OperationResult{State: state, OperationId: hex.EncodeToString(hash&#91;:&#93;), Generation: record.Generation, BusinessCode: code}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_SCOPE_INVALID&quot;)</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>request == nil &#124;&#124; domain.ValidateScope(request.Scope) != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_SCOPE_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>context.WithTimeout(ctx, 5*time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>record.State == repository.Closed</code>。
- 此处通过 <code>return &amp;pb.OperationResult{State: state, OperationId: hex.EncodeToString(hash&#91;:&#93;), Generation: record.Generation, BusinessCode: code}, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=6927:7762 -->
````go
func (c *Candidates) CloseCursor(ctx context.Context, request *pb.CloseCursorRequest) (*pb.OperationResult, error) {
	if request == nil || domain.ValidateScope(request.Scope) != nil {
		return nil, errors.New("SEARCH_SCOPE_INVALID")
	}
	anchor, err := c.codec.Open(request.Cursor)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	hash := sha256.Sum256(anchor.Chain[:])
	record, err := c.closeHash(ctx, hash, request.Scope)
	if err != nil {
		return nil, err
	}
	state, code := pb.OperationResult_PROCESSING, "CURSOR_CLOSING"
	if record.State == repository.Closed {
		state, code = pb.OperationResult_SUCCEEDED, "CURSOR_CLOSED"
	}
	return &pb.OperationResult{State: state, OperationId: hex.EncodeToString(hash[:]), Generation: record.Generation, BusinessCode: code}, nil
}

````

## 第 15 步：Candidates.closeHash

插入位置：第 14 步代码之后，紧接追加；完成稿约第 239 行。

本步声明：`func (c *Candidates) closeHash(ctx context.Context, hash [32]byte, scope *pb.SearchScope) (repository.Record, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx.Done</code>；<code>c.transact(ctx, func(tx *sql.Tx) error { 		var exists bool 		var err error 		if scope == nil { 			record, exists, err = repository.TryLock(ctx, tx, hash) 		} else { 			record, exis …</code>（长表达式，按本步完整代码继续）；<code>repository.TryLock(ctx, tx, hash)</code>；<code>repository.Lock(ctx, tx, hash)</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |
| <code>hash</code> | <code>&#91;32&#93;byte</code> | 在此声明中的实际使用：<code>repository.TryLock(ctx, tx, hash)</code>；<code>repository.Lock(ctx, tx, hash)</code>；<code>repository.Lock(finish, tx, hash)</code> |
| <code>scope</code> | <code>*pb.SearchScope</code> | 在此声明中的实际使用：<code>scope == nil</code>；<code>scope != nil</code>；<code>scope.ScopeHash</code>；<code>scope.SessionRef</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>repository.Record</code> | 使用 <code>repository.Record</code> 类型；其定义处的职责：单链不可变身份与可推进资源状态。<br>实际返回表达式：<code>repository.Record{}</code>；<code>record</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_CURSOR_CLOSE_BUSY&quot;)</code>；<code>err</code> |

**跟着本段代码走一遍：**

- 登记 <code>func() { &lt;-c.closingAdmission }()</code>，当前函数退出时执行，提前 return 也会触发。
- 此处通过 <code>return repository.Record{}, errors.New(&quot;SEARCH_CURSOR_CLOSE_BUSY&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return record, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>scope == nil</code>。
- 在本段的匿名函数内：执行 <code>repository.Lock(ctx, tx, hash)</code>。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>!exists</code>。
- 在本段的匿名函数内：检查 <code>scope == nil</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return errors.New(&quot;SEARCH_CURSOR_INVALID&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>scope != nil &amp;&amp; (hex.EncodeToString(record.ScopeHash&#91;:&#93;) != scope.ScopeHash &#124;&#124; !bytes.Equal(record.Session, scope.SessionRef))</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_CURSOR_BINDING_MISMATCH&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>record.State == repository.Closed</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>scope == nil &amp;&amp; record.Expires.After(time.Now())</code>。
- 在本段的匿名函数内：检查 <code>active &#124;&#124; (record.State == repository.Open &amp;&amp; record.Control.Phase == domain.PhaseIdle) &#124;&#124; 				(record.State == repository.Closing &amp;&amp; record.Control.RetryAt &gt; time.Now().UnixMilli())</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。

<!-- write-step: 15 bytes=7762:9465 -->
````go
func (c *Candidates) closeHash(ctx context.Context, hash [32]byte, scope *pb.SearchScope) (repository.Record, error) {
	// 关闭器与显式Close共享一个槽，先准入再SQL；不与查询mutex共用，才能在ES在途时关闭。
	select {
	case c.closingAdmission <- struct{}{}:
		defer func() { <-c.closingAdmission }()
	case <-ctx.Done():
		return repository.Record{}, errors.New("SEARCH_CURSOR_CLOSE_BUSY")
	}
	var record repository.Record
	claimed := false
	claim, err := randomToken()
	if err != nil {
		return record, err
	}
	err = c.transact(ctx, func(tx *sql.Tx) error {
		var exists bool
		var err error
		if scope == nil {
			record, exists, err = repository.TryLock(ctx, tx, hash)
		} else {
			record, exists, err = repository.Lock(ctx, tx, hash)
		}
		if err != nil {
			return err
		}
		if !exists {
			if scope == nil {
				return nil
			}
			return errors.New("SEARCH_CURSOR_INVALID")
		}
		if scope != nil && (hex.EncodeToString(record.ScopeHash[:]) != scope.ScopeHash || !bytes.Equal(record.Session, scope.SessionRef)) {
			return errors.New("SEARCH_CURSOR_BINDING_MISMATCH")
		}
		if record.State == repository.Closed {
			return nil
		}
		if scope == nil && record.Expires.After(time.Now()) {
			// Due只是旧观察，领取前必须锁内重核；原query刚确认或新close仍在途就让位。
			active := (record.Control.Phase == domain.PhaseOpening || record.Control.Phase == domain.PhaseInflight) && record.Control.Deadline > time.Now().UnixMilli()
			if active || (record.State == repository.Open && record.Control.Phase == domain.PhaseIdle) ||
				(record.State == repository.Closing && record.Control.RetryAt > time.Now().UnixMilli()) {
				return nil
			}
		}
````

## 第 16 步：Candidates.closeHash（接着实现）

插入位置：第 15 步代码之后，紧接追加；完成稿约第 284 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil &#124;&#124; !claimed</code>。条件成立时执行 <code>return record, err</code>。当前函数在这里结束。
- 检查 <code>record.PIT != &quot;&quot;</code>。
- 执行 <code>context.WithTimeout(ctx, 2*time.Second)</code>。
- 执行 <code>context.WithTimeout(context.WithoutCancel(ctx), time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 在本段的匿名函数内：执行 <code>repository.Lock(finish, tx, hash)</code>。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>!exists &#124;&#124; current.State != repository.Closing &#124;&#124; current.Control.Nonce != claim</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_CURSOR_CLOSE_CONFIRMATION_STALE&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>closeErr == nil</code>。
- 在本段的匿名函数内：检查 <code>expired &#124;&#124; (closeErr == nil &amp;&amp; !current.Control.Uncertain)</code>。
- 在本段的匿名函数内：检查 <code>closeErr == nil</code>。

<!-- write-step: 16 bytes=9465:11299 -->
````go
		uncertain := record.Control.Uncertain || record.Control.Phase != domain.PhaseIdle
		record.State = repository.Closing
		record.Control = domain.Control{Version: 1, Phase: domain.PhaseInflight, Tail: record.Control.Tail,
			Nonce: claim, Deadline: operationDeadline(ctx, record.Expires), Uncertain: uncertain}
		if err := repository.Save(ctx, tx, record); err != nil {
			return err
		}
		claimed = true
		return nil
	})
	if err != nil || !claimed {
		return record, err
	}
	var closeErr error
	if record.PIT != "" {
		closeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		closeErr = c.indexes.ClosePIT(closeCtx, record.PIT)
		cancel()
	}
	// ES无论成功还是未知都先回SQL确认原claim；中途query迟到改了最新句柄必须留给下一轮。
	finish, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
	defer cancel()
	err = c.transact(finish, func(tx *sql.Tx) error {
		current, exists, err := repository.Lock(finish, tx, hash)
		if err != nil {
			return err
		}
		if !exists || current.State != repository.Closing || current.Control.Nonce != claim {
			return errors.New("SEARCH_CURSOR_CLOSE_CONFIRMATION_STALE")
		}
		expired := !current.Expires.After(time.Now())
		current.Control.Phase = domain.PhaseIdle
		current.Control.Nonce = ""
		current.Control.Deadline = 0
		if closeErr == nil {
			current.PIT = ""
		}
		if expired || (closeErr == nil && !current.Control.Uncertain) {
			// 绝对期限兜底未知句柄，只证明应用链不可再用；不承诺ES reaper立即排空所有context。
			current.State = repository.Closed
			current.PIT = ""
			current.Control.RetryAt = 0
		} else if closeErr == nil {
			// 关闭已知句柄不能证明未知轮换句柄不存在，占额保留到原绝对期限。
			current.Control.RetryAt = current.Expires.UnixMilli()
		} else {
````

## 第 17 步：Candidates.closeHash（接着实现）

插入位置：第 16 步代码之后，紧接追加；完成稿约第 330 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return record, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 17 bytes=11299:11546 -->
````go
			current.Control.RetryAt = min(time.Now().Add(time.Second).UnixMilli(), current.Expires.UnixMilli())
		}
		if err := repository.Save(finish, tx, current); err != nil {
			return err
		}
		record = current
		return nil
	})
	return record, err
}

````

## 第 18 步：Candidates.Cleanup

插入位置：第 17 步代码之后，紧接追加；完成稿约第 341 行。

本步声明：`func (c *Candidates) Cleanup(ctx context.Context) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>c.transact(ctx, func(tx *sql.Tx) error { 		if err := repository.Check(ctx, tx); err != nil { 			return err 		} 		var err error 		due, err = repository.Due(ctx, tx, time.Now().UTC(), 16) 		return err 	})</code>；<code>repository.Check(ctx, tx)</code>；<code>repository.Due(ctx, tx, time.Now().UTC(), 16)</code>；<code>ctx.Err</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>firstError</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>c.sweepMu.Lock()</code>。
- 登记 <code>c.sweepMu.Unlock()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>c.sweepMu.Unlock()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>ctx.Err() != nil</code>。
- 检查 <code>firstError == nil</code>。
- 执行 <code>context.WithTimeout(ctx, 2*time.Second)</code>。
- 检查 <code>err != nil &amp;&amp; firstError == nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return repository.DeleteClosed(ctx, tx, time.Now().UTC(), 16)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>firstError != nil</code>。条件成立时执行 <code>return firstError</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=11546:12621 -->
````go
func (c *Candidates) Cleanup(ctx context.Context) error {
	c.sweepMu.Lock()
	defer c.sweepMu.Unlock()
	var due [][32]byte
	if err := c.transact(ctx, func(tx *sql.Tx) error {
		if err := repository.Check(ctx, tx); err != nil {
			return err
		}
		var err error
		due, err = repository.Due(ctx, tx, time.Now().UTC(), 16)
		return err
	}); err != nil {
		return err
	}
	var firstError error
	for _, hash := range due {
		if ctx.Err() != nil {
			if firstError == nil {
				firstError = errors.New("SEARCH_CURSOR_RECOVERY_UNAVAILABLE")
			}
			break
		}
		// 单项锁/确认故障有独立预算，保留首错但仍处理后续项；不能用坏首行垄断整轮。
		item, cancel := context.WithTimeout(ctx, 2*time.Second)
		_, err := c.closeHash(item, hash, nil)
		cancel()
		if err != nil && firstError == nil {
			firstError = err
		}
	}
	if err := c.transact(ctx, func(tx *sql.Tx) error { return repository.DeleteClosed(ctx, tx, time.Now().UTC(), 16) }); err != nil {
		return err
	}
	if firstError != nil {
		return firstError
	}
	c.recovered.Store(true)
	return nil
}

````

## 第 19 步：Candidates.Ready

插入位置：第 18 步代码之后，紧接追加；完成稿约第 381 行。

本步声明：`func (c *Candidates) Ready(ctx context.Context) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>c.generation.Ready(ctx)</code>；<code>c.transact(ctx, func(tx *sql.Tx) error { return repository.Check(ctx, tx) })</code>；<code>repository.Check(ctx, tx)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_CURSOR_RECOVERY_REQUIRED&quot;)</code>；<code>err</code>；<code>c.transact(ctx, func(tx *sql.Tx) error { return repository.Check(ctx, tx) })</code> |

**跟着本段代码走一遍：**

- 检查 <code>!c.recovered.Load()</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_CURSOR_RECOVERY_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return c.transact(ctx, func(tx *sql.Tx) error { return repository.Check(ctx, tx) })</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：此处通过 <code>return repository.Check(ctx, tx)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 19 bytes=12621:12979 -->
````go
func (c *Candidates) Ready(ctx context.Context) error {
	if !c.recovered.Load() {
		return errors.New("SEARCH_CURSOR_RECOVERY_REQUIRED")
	}
	if err := c.generation.Ready(ctx); err != nil {
		return err
	}
	return c.transact(ctx, func(tx *sql.Tx) error { return repository.Check(ctx, tx) })
}

// continuation.go：保留原实现，按业务合并文件。

````

## 第 20 步：sameBatch

插入位置：第 19 步代码之后，紧接追加；完成稿约第 393 行。

本步声明：`func sameBatch(anchor domain.Batch, batch elastic.CandidateBatch, digest [32]byte) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>anchor</code> | <code>domain.Batch</code> | 使用 <code>domain.Batch</code> 类型；其定义处的职责：不可随意改写的一个完整候选批证据。<br>在此声明中的实际使用：<code>anchor.Count</code>；<code>anchor.More</code>；<code>anchor.Digest</code>；<code>anchor.After</code> |
| <code>batch</code> | <code>elastic.CandidateBatch</code> | 使用 <code>elastic.CandidateBatch</code> 类型；其定义处的职责：一次完整ES查询结果及最新资源句柄。<br>在此声明中的实际使用：<code>batch.Candidates</code>；<code>batch.More</code>；<code>batch.End</code> |
| <code>digest</code> | <code>&#91;32&#93;byte</code> | 在此声明中的实际使用：<code>anchor.Digest == digest</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>anchor.Count == uint8(len(batch.Candidates)) &amp;&amp; anchor.More == batch.More &amp;&amp; anchor.Digest == digest &amp;&amp; bytes.Equal(anchor.After, batch.End)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return anchor.Count == uint8(len(batch.Candidates)) &amp;&amp; anchor.More == batch.More &amp;&amp; anchor.Digest == digest &amp;&amp; bytes.Equal(anchor.After, batch.End)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 20 bytes=12979:13221 -->
````go
func sameBatch(anchor domain.Batch, batch elastic.CandidateBatch, digest [32]byte) bool {
	return anchor.Count == uint8(len(batch.Candidates)) && anchor.More == batch.More && anchor.Digest == digest && bytes.Equal(anchor.After, batch.End)
}

````

## 第 21 步：Candidates.retainOpened

插入位置：第 20 步代码之后，紧接追加；完成稿约第 397 行。

本步声明：`func (c *Candidates) retainOpened(ctx context.Context, opened repository.Record) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>c.transact(ctx, func(tx *sql.Tx) error { 		current, exists, err := repository.Lock(ctx, tx, opened.Hash) 		if err != nil { 			return err 		} 		if !exists &#124;&#124; current.State != reposi …</code>（长表达式，按本步完整代码继续）；<code>repository.Lock(ctx, tx, opened.Hash)</code>；<code>repository.Save(ctx, tx, current)</code> |
| <code>opened</code> | <code>repository.Record</code> | 使用 <code>repository.Record</code> 类型；其定义处的职责：单链不可变身份与可推进资源状态。<br>在此声明中的实际使用：<code>opened.Hash</code>；<code>opened.Control</code>；<code>opened.PIT</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>c.transact(ctx, func(tx *sql.Tx) error { 		current, exists, err := repository.Lock(ctx, tx, opened.Hash) 		if err != nil { 			return err 		} 		if !exists &#124;&#124; current.State != reposi …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 此处通过 <code>return c.transact(ctx, func(tx *sql.Tx) error { 		current, exists, err := repository.Lock(ctx, tx, opened.Hash) 		if err != nil { 			return err 		} 		if !exists &#124;&#124; current.State != …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：执行 <code>repository.Lock(ctx, tx, opened.Hash)</code>。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>!exists &#124;&#124; current.State != repository.Open &#124;&#124; current.Control.Nonce != opened.Control.Nonce &#124;&#124; current.Control.Phase != domain.PhaseOpening</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_CURSOR_OPENING_CHANGED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return repository.Save(ctx, tx, current)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 21 bytes=13221:13789 -->
````go
func (c *Candidates) retainOpened(ctx context.Context, opened repository.Record) error {
	return c.transact(ctx, func(tx *sql.Tx) error {
		current, exists, err := repository.Lock(ctx, tx, opened.Hash)
		if err != nil {
			return err
		}
		if !exists || current.State != repository.Open || current.Control.Nonce != opened.Control.Nonce || current.Control.Phase != domain.PhaseOpening {
			return errors.New("SEARCH_CURSOR_OPENING_CHANGED")
		}
		current.PIT = opened.PIT
		current.Control.Phase = domain.PhaseInflight
		return repository.Save(ctx, tx, current)
	})
}

````

## 第 22 步：Candidates.confirmQuery

插入位置：第 21 步代码之后，紧接追加；完成稿约第 412 行。

本步声明：`func (c *Candidates) confirmQuery(ctx context.Context, queried repository.Record, end json.RawMessage) error`。

**这一段为什么这样写：** 只接受原在途调用在原期限内返回的完整批次。

1. 锁原记录
2. 核OPEN、nonce、INFLIGHT及deadline/expiry
3. 保存最新PIT和end并转IDLE清nonce


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 确认事务期限<br>在此声明中的实际使用：<code>c.transact(ctx, func(tx *sql.Tx) error { 		current, exists, err := repository.Lock(ctx, tx, queried.Hash) 		if err != nil { 			return err 		} 		// Close/worker只能使状态收紧；迟到的ES回复必须匹配原在 …</code>（长表达式，按本步完整代码继续）；<code>repository.Lock(ctx, tx, queried.Hash)</code>；<code>repository.Save(ctx, tx, current)</code> |
| <code>queried</code> | <code>repository.Record</code> | 本次领取时记录及最新返回PIT<br>在此声明中的实际使用：<code>queried.Hash</code>；<code>queried.Control</code>；<code>queried.PIT</code> |
| <code>end</code> | <code>json.RawMessage</code> | 完整批次最后sort原字节<br>在此声明中的实际使用：<code>append(json.RawMessage(nil), end...)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 确认失败不得发送Complete=true<br>实际返回表达式：<code>c.transact(ctx, func(tx *sql.Tx) error { 		current, exists, err := repository.Lock(ctx, tx, queried.Hash) 		if err != nil { 			return err 		} 		// Close/worker只能使状态收紧；迟到的ES回复必须匹配原在 …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 此处通过 <code>return c.transact(ctx, func(tx *sql.Tx) error { 		current, exists, err := repository.Lock(ctx, tx, queried.Hash) 		if err != nil { 			return err 		} 		// Close/worker只能使状态收紧；迟到的ES回 …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：执行 <code>repository.Lock(ctx, tx, queried.Hash)</code>。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>!exists &#124;&#124; current.State != repository.Open &#124;&#124; current.Control.Nonce != queried.Control.Nonce &#124;&#124; 			current.Control.Phase != domain.PhaseInflight &#124;&#124; current.Control.Deadline &lt;= tim …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return errors.New(&quot;SEARCH_CURSOR_CONFIRMATION_STALE&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return repository.Save(ctx, tx, current)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 22 bytes=13789:14657 -->
````go
func (c *Candidates) confirmQuery(ctx context.Context, queried repository.Record, end json.RawMessage) error {
	return c.transact(ctx, func(tx *sql.Tx) error {
		current, exists, err := repository.Lock(ctx, tx, queried.Hash)
		if err != nil {
			return err
		}
		// Close/worker只能使状态收紧；迟到的ES回复必须匹配原在途nonce，绝不重新OPEN。
		if !exists || current.State != repository.Open || current.Control.Nonce != queried.Control.Nonce ||
			current.Control.Phase != domain.PhaseInflight || current.Control.Deadline <= time.Now().UnixMilli() || !current.Expires.After(time.Now()) {
			return errors.New("SEARCH_CURSOR_CONFIRMATION_STALE")
		}
		current.PIT = queried.PIT
		current.Control = domain.Control{Version: 1, Phase: domain.PhaseIdle, Tail: append(json.RawMessage(nil), end...)}
		return repository.Save(ctx, tx, current)
	})
}

````

## 第 23 步：Candidates.fail

插入位置：第 22 步代码之后，紧接追加；完成稿约第 429 行。

本步声明：`func (c *Candidates) fail(ctx context.Context, record repository.Record, pit string, uncertain bool)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>context.WithoutCancel(ctx)</code> |
| <code>record</code> | <code>repository.Record</code> | 使用 <code>repository.Record</code> 类型；其定义处的职责：单链不可变身份与可推进资源状态。<br>在此声明中的实际使用：<code>record.Hash</code>；<code>record.Control</code> |
| <code>pit</code> | <code>string</code> | 在此声明中的实际使用：<code>pit != &quot;&quot;</code>；<code>current.PIT = pit</code> |
| <code>uncertain</code> | <code>bool</code> | 在此声明中的实际使用：<code>current.Control.Uncertain &#124;&#124; uncertain</code> |

**跟着本段代码走一遍：**

- 检查 <code>record.Hash == (&#91;32&#93;byte{})</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 执行 <code>context.WithTimeout(context.WithoutCancel(ctx), time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 在本段的匿名函数内：执行 <code>repository.Lock(cleanup, tx, record.Hash)</code>。
- 在本段的匿名函数内：检查 <code>err != nil &#124;&#124; !exists</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>current.State == repository.Closed</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>current.State == repository.Open &amp;&amp; current.Control.Nonce != record.Control.Nonce</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>pit != &quot;&quot;</code>。
- 在本段的匿名函数内：此处通过 <code>return repository.Save(cleanup, tx, current)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 23 bytes=14657:15920 -->
````go
func (c *Candidates) fail(ctx context.Context, record repository.Record, pit string, uncertain bool) {
	if record.Hash == ([32]byte{}) {
		return
	}
	// 仅做有界SQL补偿；原请求取消后不继续ES或重新打开资源。SQL再未知由持久inflight期限恢复。
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
	defer cancel()
	_ = c.transact(cleanup, func(tx *sql.Tx) error {
		current, exists, err := repository.Lock(cleanup, tx, record.Hash)
		if err != nil || !exists {
			return err
		}
		if current.State == repository.Closed {
			return nil
		}
		if current.State == repository.Open && current.Control.Nonce != record.Control.Nonce {
			// COMMIT已成功但回执丢失时，重读可确认IDLE且nonce已清空，保留可重试批。
			// 较新的在途nonce同样不能被旧补偿收紧；只有原nonce仍在途才标记未知关闭。
			return nil
		}
		if pit != "" {
			current.PIT = pit
		}
		current.State = repository.Closing
		current.Control = domain.Control{Version: 1, Phase: domain.PhaseIdle, Tail: current.Control.Tail, Uncertain: current.Control.Uncertain || uncertain}
		return repository.Save(cleanup, tx, current)
	})
}

// coordination.go：保留原实现，按业务合并文件。

````

## 第 24 步：keyLock

插入位置：第 23 步代码之后，紧接追加；完成稿约第 460 行。

本步声明：`type keyLock struct {`。

**这一段为什么这样写：** 单链查询的进程内可取消互斥。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>token</code> | <code>chan struct{}</code> | 容量1通道，持有即执行权 |
| <code>references</code> | <code>int</code> | 持有者和等待者总数，归零才删Map |

<!-- write-step: 24 bytes=15920:15986 -->
````go
type keyLock struct {
	token      chan struct{}
	references int
}
````

## 第 25 步：keyLocks

插入位置：第 24 步代码之后，紧接追加；完成稿约第 464 行。

本步声明：`type keyLocks struct {`。

**这一段为什么这样写：** 按游标chain hash隔离的查询锁集合。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>mu</code> | <code>sync.Mutex</code> | 保护entries及引用计数 |
| <code>entries</code> | <code>map&#91;&#91;32&#93;byte&#93;*keyLock</code> | chain hash→keyLock映射 |

<!-- write-step: 25 bytes=15986:16063 -->
````go
type keyLocks struct {
	mu      sync.Mutex
	entries map[[32]byte]*keyLock
}

````

## 第 26 步：keyLocks.acquire

插入位置：第 25 步代码之后，紧接追加；完成稿约第 469 行。

本步声明：`func (locks *keyLocks) acquire(ctx context.Context, key [32]byte) (func(), error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ctx.Done</code> |
| <code>key</code> | <code>&#91;32&#93;byte</code> | 在此声明中的实际使用：<code>entry := locks.entries&#91;key&#93;</code>；<code>locks.entries&#91;key&#93; = entry</code>；<code>delete(locks.entries, key)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>func()</code> | 实际返回表达式：<code>func() { &lt;-entry.token; drop() }</code>；<code>nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>errors.New(&quot;SEARCH_CURSOR_BUSY&quot;)</code> |

**跟着本段代码走一遍：**

- 执行 <code>locks.mu.Lock()</code>。
- 检查 <code>locks.entries == nil</code>。
- 检查 <code>entry == nil</code>。
- 执行 <code>locks.mu.Unlock()</code>。
- 在本段的匿名函数内：执行 <code>locks.mu.Lock()</code>。
- 在本段的匿名函数内：登记 <code>locks.mu.Unlock()</code>，当前函数退出时执行，提前 return 也会触发。
- 在本段的匿名函数内：执行 <code>locks.mu.Unlock()</code>。
- 在本段的匿名函数内：检查 <code>entry.references == 0</code>。
- 此处通过 <code>return func() { &lt;-entry.token; drop() }, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return nil, errors.New(&quot;SEARCH_CURSOR_BUSY&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 26 bytes=16063:16795 -->
````go
func (locks *keyLocks) acquire(ctx context.Context, key [32]byte) (func(), error) {
	locks.mu.Lock()
	if locks.entries == nil {
		locks.entries = make(map[[32]byte]*keyLock)
	}
	entry := locks.entries[key]
	if entry == nil {
		entry = &keyLock{token: make(chan struct{}, 1)}
		locks.entries[key] = entry
	}
	entry.references++
	locks.mu.Unlock()
	drop := func() {
		locks.mu.Lock()
		defer locks.mu.Unlock()
		entry.references--
		if entry.references == 0 {
			delete(locks.entries, key)
		}
	}
	select {
	case entry.token <- struct{}{}:
		return func() { <-entry.token; drop() }, nil
	case <-ctx.Done():
		drop()
		return nil, errors.New("SEARCH_CURSOR_BUSY")
	}
}

// registration.go：保留原实现，按业务合并文件。

````

## 第 27 步：Candidates.reserve

插入位置：第 26 步代码之后，紧接追加；完成稿约第 500 行。

本步声明：`func (c *Candidates) reserve(ctx context.Context, query domain.Query) (repository.Record, [32]byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>operationDeadline(ctx, record.Expires)</code>；<code>c.transact(ctx, func(tx *sql.Tx) error { 		// 排他代次锁串行化所有新链配额；提交后再进入共享Guard，避免反向嵌套锁自锁。 		target, exists, err := generation.Find(ctx, tx, c.alias, true) 		if err != nil { 			return e …</code>（长表达式，按本步完整代码继续）；<code>generation.Find(ctx, tx, c.alias, true)</code>；<code>generation.CheckSwitching(ctx, tx)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>query</code> | <code>domain.Query</code> | 使用 <code>domain.Query</code> 类型；其定义处的职责：已校验且自有副本的Doc候选请求。<br>在此声明中的实际使用：<code>query.QueryHash</code>；<code>query.Scope</code>；<code>query.PageSize</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>repository.Record</code> | 使用 <code>repository.Record</code> 类型；其定义处的职责：单链不可变身份与可推进资源状态。<br>实际返回表达式：<code>repository.Record{}</code>；<code>record</code> |
| <code>(匿名/未命名)</code> | <code>&#91;32&#93;byte</code> | 实际返回表达式：<code>chain</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_CURSOR_ENTROPY_UNAVAILABLE&quot;)</code>；<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return repository.Record{}, chain, errors.New(&quot;SEARCH_CURSOR_ENTROPY_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return repository.Record{}, chain, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>!exists</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_GENERATION_MISSING&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return repository.Create(ctx, tx, &amp;record, 128, 1024)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return record, chain, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 27 bytes=16795:18255 -->
````go
func (c *Candidates) reserve(ctx context.Context, query domain.Query) (repository.Record, [32]byte, error) {
	var chain [32]byte
	if _, err := rand.Read(chain[:]); err != nil {
		return repository.Record{}, chain, errors.New("SEARCH_CURSOR_ENTROPY_UNAVAILABLE")
	}
	nonce, err := randomToken()
	if err != nil {
		return repository.Record{}, chain, err
	}
	queryHash, _ := hex.DecodeString(query.QueryHash)
	scopeHash, _ := hex.DecodeString(query.Scope.ScopeHash)
	record := repository.Record{Hash: sha256.Sum256(chain[:]), PageSize: query.PageSize, Session: query.Scope.SessionRef,
		Expires: time.Now().Add(c.ttl).UTC().Truncate(time.Millisecond), State: repository.Open}
	copy(record.ScopeHash[:], scopeHash)
	copy(record.QueryHash[:], queryHash)
	record.Control = domain.Control{Version: 1, Phase: domain.PhaseOpening, Nonce: nonce, Deadline: operationDeadline(ctx, record.Expires)}
	err = c.transact(ctx, func(tx *sql.Tx) error {
		// 排他代次锁串行化所有新链配额；提交后再进入共享Guard，避免反向嵌套锁自锁。
		target, exists, err := generation.Find(ctx, tx, c.alias, true)
		if err != nil {
			return err
		}
		if !exists {
			return errors.New("SEARCH_GENERATION_MISSING")
		}
		if err := generation.CheckSwitching(ctx, tx); err != nil {
			return err
		}
		record.Physical, record.Generation = target.Physical, target.Generation
		return repository.Create(ctx, tx, &record, 128, 1024)
	})
	return record, chain, err
}

````

## 第 28 步：matchesQuery

插入位置：第 27 步代码之后，紧接追加；完成稿约第 534 行。

本步声明：`func matchesQuery(record repository.Record, query domain.Query) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>record</code> | <code>repository.Record</code> | 使用 <code>repository.Record</code> 类型；其定义处的职责：单链不可变身份与可推进资源状态。<br>在此声明中的实际使用：<code>record.QueryHash</code>；<code>record.ScopeHash</code>；<code>record.Session</code>；<code>record.PageSize</code> |
| <code>query</code> | <code>domain.Query</code> | 使用 <code>domain.Query</code> 类型；其定义处的职责：已校验且自有副本的Doc候选请求。<br>在此声明中的实际使用：<code>query.QueryHash</code>；<code>query.Scope</code>；<code>query.PageSize</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>hex.EncodeToString(record.QueryHash&#91;:&#93;) == query.QueryHash &amp;&amp; hex.EncodeToString(record.ScopeHash&#91;:&#93;) == query.Scope.ScopeHash &amp;&amp; 		bytes.Equal(record.Session, query.Scope.SessionRef) &amp;&amp; record.PageSize == query.PageSize</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return hex.EncodeToString(record.QueryHash&#91;:&#93;) == query.QueryHash &amp;&amp; hex.EncodeToString(record.ScopeHash&#91;:&#93;) == query.Scope.ScopeHash &amp;&amp; 		bytes.Equal(record.Session, query.Scope.S …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 28 bytes=18255:18558 -->
````go
func matchesQuery(record repository.Record, query domain.Query) bool {
	return hex.EncodeToString(record.QueryHash[:]) == query.QueryHash && hex.EncodeToString(record.ScopeHash[:]) == query.Scope.ScopeHash &&
		bytes.Equal(record.Session, query.Scope.SessionRef) && record.PageSize == query.PageSize
}

````

## 第 29 步：Candidates.beginQuery

插入位置：第 28 步代码之后，紧接追加；完成稿约第 539 行。

本步声明：`func (c *Candidates) beginQuery(ctx context.Context, hash [32]byte, query domain.Query, target generation.Target, newChain bool) (repository.Record, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>c.transact(ctx, func(tx *sql.Tx) error { 		var exists bool 		var err error 		record, exists, err = repository.Lock(ctx, tx, hash) 		if err != nil { 			return err 		} 		if !exists { …</code>（长表达式，按本步完整代码继续）；<code>repository.Lock(ctx, tx, hash)</code>；<code>operationDeadline(ctx, record.Expires)</code>；<code>repository.Save(ctx, tx, record)</code> |
| <code>hash</code> | <code>&#91;32&#93;byte</code> | 在此声明中的实际使用：<code>repository.Lock(ctx, tx, hash)</code> |
| <code>query</code> | <code>domain.Query</code> | 使用 <code>domain.Query</code> 类型；其定义处的职责：已校验且自有副本的Doc候选请求。<br>在此声明中的实际使用：<code>matchesQuery(record, query)</code> |
| <code>target</code> | <code>generation.Target</code> | 使用 <code>generation.Target</code> 类型；其定义处的职责：一次记录处理期间固定的写入目标。<br>在此声明中的实际使用：<code>target.Physical</code>；<code>target.Generation</code> |
| <code>newChain</code> | <code>bool</code> | 在此声明中的实际使用：<code>c.transact(ctx, func(tx *sql.Tx) error { 		var exists bool 		var err error 		record, exists, err = repository.Lock(ctx, tx, hash) 		if err != nil { 			return err 		} 		if !exists { …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>repository.Record</code> | 使用 <code>repository.Record</code> 类型；其定义处的职责：单链不可变身份与可推进资源状态。<br>实际返回表达式：<code>record</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 在本段的匿名函数内：执行 <code>repository.Lock(ctx, tx, hash)</code>。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>!exists</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_CURSOR_INVALID&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>!matchesQuery(record, query)</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_CURSOR_BINDING_MISMATCH&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>record.State != repository.Open</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_CURSOR_CLOSED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>!record.Expires.After(time.Now())</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_CURSOR_EXPIRED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>record.Physical != target.Physical &#124;&#124; record.Generation != target.Generation</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_CURSOR_STALE&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>newChain</code>。
- 在本段的匿名函数内：检查 <code>record.Control.Phase != domain.PhaseOpening</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_CURSOR_OPENING_CHANGED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>record.Control.Phase != domain.PhaseIdle</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_CURSOR_BUSY&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return repository.Save(ctx, tx, record)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return record, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 29 bytes=18558:19910 -->
````go
func (c *Candidates) beginQuery(ctx context.Context, hash [32]byte, query domain.Query, target generation.Target, newChain bool) (repository.Record, error) {
	var record repository.Record
	err := c.transact(ctx, func(tx *sql.Tx) error {
		var exists bool
		var err error
		record, exists, err = repository.Lock(ctx, tx, hash)
		if err != nil {
			return err
		}
		if !exists {
			return errors.New("SEARCH_CURSOR_INVALID")
		}
		if !matchesQuery(record, query) {
			return errors.New("SEARCH_CURSOR_BINDING_MISMATCH")
		}
		if record.State != repository.Open {
			return errors.New("SEARCH_CURSOR_CLOSED")
		}
		if !record.Expires.After(time.Now()) {
			return errors.New("SEARCH_CURSOR_EXPIRED")
		}
		if record.Physical != target.Physical || record.Generation != target.Generation {
			return errors.New("SEARCH_CURSOR_STALE")
		}
		if newChain {
			if record.Control.Phase != domain.PhaseOpening {
				return errors.New("SEARCH_CURSOR_OPENING_CHANGED")
			}
			return nil
		}
		if record.Control.Phase != domain.PhaseIdle {
			return errors.New("SEARCH_CURSOR_BUSY")
		}
		nonce, err := randomToken()
		if err != nil {
			return err
		}
		record.Control.Phase = domain.PhaseInflight
		record.Control.Nonce = nonce
		record.Control.Deadline = operationDeadline(ctx, record.Expires)
		return repository.Save(ctx, tx, record)
	})
	return record, err
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/search/internal/logic/candidates/candidates_service.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/search/03-candidates-cursor.md)。
