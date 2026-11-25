# 分步编写 `services/search/internal/repository/event/events_queries.go`

先按 [S02 · 持久消费、日志身份与代次锁](../lessons/search/02-consumer-ledger.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/385-events-queries.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

实现原事件账本、物理索引应用水位、整记录checkpoint和日志身份绑定。

ES/SQL/Kafka没有共同事务，必须明确哪些持久证据已成立，并按原记录重试未知结果。

写之前先按这条执行路线理解各段的连接：

1. Receive同事务保存consumed_event和event_origin；重复时核原行/摘要/topic/partition，保留首次成功绑定
2. LockApplication按physical+document锁水位，零摘要占位只能活在未提交事务
3. ES返回实际状态后SaveApplication检查单调序号与revision，Complete才写authority证明和SUCCEEDED
4. AdvanceCheckpoint核整条所有行SUCCEEDED，禁止offset缺口，SQL next=offset+1提交后才能提交broker
5. BindLogIdentity只插入不可变cluster/topic UUID，重复键后FOR SHARE核原值；CheckRecovery拒绝缺原行/假成功

本文件需要守住的边界：

- RECEIVED/APPLYING/RETRY不是消费成功
- ES写成而SQL未知可重试，但不能先写成功回执
- 已成功原事件的物理代次/权威摘要不能在重建时改写
- INSERT IGNORE会吞截断等异常，日志身份绑定只能接受1062主键冲突

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/search/internal/repository/event' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/search/internal/repository/event/events_queries.go') -and (Get-Item -LiteralPath 'services/search/internal/repository/event/events_queries.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/search/internal/repository/event/events_queries.go')) { New-Item -ItemType File -Path 'services/search/internal/repository/event/events_queries.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:392 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package event

import (
	"bytes"
	domain "collaboration/search/internal/domain/event"
	index "collaboration/search/internal/domain/index"
	"context"
	"database/sql"
	"errors"
	"github.com/go-sql-driver/mysql"
	"math"
	"strconv"
)

// application.go：保留原实现，按业务合并文件。

````

## 第 2 步：readApplication

插入位置：第 1 步代码之后，紧接追加；完成稿约第 18 行。

本步声明：`func readApplication(ctx context.Context, tx *sql.Tx, physical string, document uint64) (index.State, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT applied_seq,authority_digest,CAST(release_id AS CHAR),visibility,visibility_policy,revision 		FROM index_application WHERE physical_index=? AND docu …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>physical</code> | <code>string</code> | ES物理索引名，应用水位按它隔离<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT applied_seq,authority_digest,CAST(release_id AS CHAR),visibility,visibility_policy,revision 		FROM index_application WHERE physical_index=? AND docu …</code>（长表达式，按本步完整代码继续） |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT applied_seq,authority_digest,CAST(release_id AS CHAR),visibility,visibility_policy,revision 		FROM index_application WHERE physical_index=? AND docu …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>index.State</code> | 使用 <code>index.State</code> 类型；其定义处的职责：某物理索引对一个文档实际应用的水位。<br>实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;SEARCH_INDEX_STATE_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, `SELECT applied_seq,authority_digest,CAST(release_id AS CHAR),visibility,visibility_policy,revision 		FROM index_application WHERE physical_index=? AND docu …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return value, err</code>。当前函数在这里结束。
- 检查 <code>len(digest) != 32 &#124;&#124; value.Revision == 0</code>。条件成立时执行 <code>return value, errors.New(&quot;SEARCH_INDEX_STATE_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>release.Valid</code>。
- 检查 <code>err != nil &#124;&#124; value.Release == 0</code>。条件成立时执行 <code>return value, errors.New(&quot;SEARCH_INDEX_STATE_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>(visibility != &quot;PUBLIC&quot; &amp;&amp; visibility != &quot;HIDDEN&quot;) &#124;&#124; (value.Visible != (value.Release != 0)) &#124;&#124; 		(value.Policy != &quot;PUBLIC&quot; &amp;&amp; value.Policy != &quot;SPACE_MEMBERS&quot; &amp;&amp; value.Policy != &quot;COLLABORATORS&quot;)</code>。条件成立时执行 <code>return value, errors.New(&quot;SEARCH_INDEX_STATE_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return value, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=392:1608 -->
````go
func readApplication(ctx context.Context, tx *sql.Tx, physical string, document uint64) (index.State, error) {
	var value index.State
	var digest []byte
	var release sql.NullString
	var visibility string
	err := tx.QueryRowContext(ctx, `SELECT applied_seq,authority_digest,CAST(release_id AS CHAR),visibility,visibility_policy,revision
		FROM index_application WHERE physical_index=? AND document_id=? FOR UPDATE`, physical, document).
		Scan(&value.Sequence, &digest, &release, &visibility, &value.Policy, &value.Revision)
	if err != nil {
		return value, err
	}
	if len(digest) != 32 || value.Revision == 0 {
		return value, errors.New("SEARCH_INDEX_STATE_INVALID")
	}
	copy(value.Digest[:], digest)
	value.Visible = visibility == "PUBLIC"
	if release.Valid {
		value.Release, err = strconv.ParseUint(release.String, 10, 64)
		if err != nil || value.Release == 0 {
			return value, errors.New("SEARCH_INDEX_STATE_INVALID")
		}
	}
	if (visibility != "PUBLIC" && visibility != "HIDDEN") || (value.Visible != (value.Release != 0)) ||
		(value.Policy != "PUBLIC" && value.Policy != "SPACE_MEMBERS" && value.Policy != "COLLABORATORS") {
		return value, errors.New("SEARCH_INDEX_STATE_INVALID")
	}
	return value, nil
}

````

## 第 3 步：LockApplication

插入位置：第 2 步代码之后，紧接追加；完成稿约第 47 行。

LockApplication按物理索引隔离同文档。新行的零摘要仅存在于未提交事务，
调用方必须ES成功并SaveApplication后才可提交；失败回滚不会留下虚假水位。

本步声明：`func LockApplication(ctx context.Context, tx *sql.Tx, physical string, document uint64) (index.State, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>readApplication(ctx, tx, physical, document)</code>；<code>tx.ExecContext(ctx, `INSERT INTO index_application(physical_index,document_id,applied_seq,authority_digest,release_id,visibility,visibility_policy) 		VALUES(?,?,0,?,NULL,&#x27;HIDDEN&#x27;,&#x27; …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>readApplication(ctx, tx, physical, document)</code>；<code>tx.ExecContext</code> |
| <code>physical</code> | <code>string</code> | ES物理索引名，应用水位按它隔离<br>在此声明中的实际使用：<code>physicalName.MatchString(physical)</code>；<code>readApplication(ctx, tx, physical, document)</code>；<code>tx.ExecContext(ctx, `INSERT INTO index_application(physical_index,document_id,applied_seq,authority_digest,release_id,visibility,visibility_policy) 		VALUES(?,?,0,?,NULL,&#x27;HIDDEN&#x27;,&#x27; …</code>（长表达式，按本步完整代码继续） |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>document == 0</code>；<code>readApplication(ctx, tx, physical, document)</code>；<code>tx.ExecContext(ctx, `INSERT INTO index_application(physical_index,document_id,applied_seq,authority_digest,release_id,visibility,visibility_policy) 		VALUES(?,?,0,?,NULL,&#x27;HIDDEN&#x27;,&#x27; …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>index.State</code> | 使用 <code>index.State</code> 类型；其定义处的职责：某物理索引对一个文档实际应用的水位。<br>实际返回表达式：<code>index.State{}</code>；<code>value</code>；<code>index.State{Revision: 1}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_INDEX_BINDING_INVALID&quot;)</code>；<code>errors.New(&quot;SEARCH_INDEX_STATE_UNBOUND&quot;)</code>；<code>nil</code>；<code>errors.New(&quot;SEARCH_APPLICATION_READ_FAILED&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!physicalName.MatchString(physical) &#124;&#124; document == 0</code>。条件成立时执行 <code>return index.State{}, errors.New(&quot;SEARCH_INDEX_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err == nil</code>。
- 检查 <code>value.Digest == (&#91;32&#93;byte{})</code>。条件成立时执行 <code>return value, errors.New(&quot;SEARCH_INDEX_STATE_UNBOUND&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return value, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != sql.ErrNoRows</code>。条件成立时执行 <code>return value, errors.New(&quot;SEARCH_APPLICATION_READ_FAILED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return value, errors.New(&quot;SEARCH_APPLICATION_LOCK_FAILED&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `INSERT INTO index_application(physical_index,document_id,applied_seq,authority_digest,release_id,visibility,visibility_policy) 		VALUES(?,?,0,?,NULL,&#x27;HIDDEN&#x27;,&#x27; …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return index.State{Revision: 1}, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=1608:2700 -->
````go
// LockApplication按物理索引隔离同文档。新行的零摘要仅存在于未提交事务，
// 调用方必须ES成功并SaveApplication后才可提交；失败回滚不会留下虚假水位。
func LockApplication(ctx context.Context, tx *sql.Tx, physical string, document uint64) (index.State, error) {
	if !physicalName.MatchString(physical) || document == 0 {
		return index.State{}, errors.New("SEARCH_INDEX_BINDING_INVALID")
	}
	value, err := readApplication(ctx, tx, physical, document)
	if err == nil {
		if value.Digest == ([32]byte{}) {
			return value, errors.New("SEARCH_INDEX_STATE_UNBOUND")
		}
		return value, nil
	}
	if err != sql.ErrNoRows {
		return value, errors.New("SEARCH_APPLICATION_READ_FAILED")
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO index_application(physical_index,document_id,applied_seq,authority_digest,release_id,visibility,visibility_policy)
		VALUES(?,?,0,?,NULL,'HIDDEN','PUBLIC')`, physical, document, make([]byte, 32)); err != nil {
		return value, errors.New("SEARCH_APPLICATION_LOCK_FAILED")
	}
	return index.State{Revision: 1}, nil
}

````

## 第 4 步：SaveApplication

插入位置：第 3 步代码之后，紧接追加；完成稿约第 70 行。

本步声明：`func SaveApplication(ctx context.Context, tx *sql.Tx, physical string, document uint64, next index.State) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>readApplication(ctx, tx, physical, document)</code>；<code>tx.ExecContext(ctx, `UPDATE index_application SET applied_seq=?,authority_digest=?,release_id=?,visibility=?,visibility_policy=?, 		revision=?,updated_at=UTC_TIMESTAMP(6) WHERE phy …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>readApplication(ctx, tx, physical, document)</code>；<code>tx.ExecContext</code> |
| <code>physical</code> | <code>string</code> | ES物理索引名，应用水位按它隔离<br>在此声明中的实际使用：<code>physicalName.MatchString(physical)</code>；<code>readApplication(ctx, tx, physical, document)</code>；<code>tx.ExecContext(ctx, `UPDATE index_application SET applied_seq=?,authority_digest=?,release_id=?,visibility=?,visibility_policy=?, 		revision=?,updated_at=UTC_TIMESTAMP(6) WHERE phy …</code>（长表达式，按本步完整代码继续） |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>document == 0</code>；<code>readApplication(ctx, tx, physical, document)</code>；<code>tx.ExecContext(ctx, `UPDATE index_application SET applied_seq=?,authority_digest=?,release_id=?,visibility=?,visibility_policy=?, 		revision=?,updated_at=UTC_TIMESTAMP(6) WHERE phy …</code>（长表达式，按本步完整代码继续） |
| <code>next</code> | <code>index.State</code> | SQL已完成整条记录后的下一offset<br>在此声明中的实际使用：<code>next.Sequence</code>；<code>next.Release</code>；<code>next.Visible</code>；<code>next.Policy</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_INDEX_BINDING_INVALID&quot;)</code>；<code>errors.New(&quot;SEARCH_APPLICATION_READ_FAILED&quot;)</code>；<code>err</code>；<code>errors.New(&quot;SEARCH_APPLICATION_REGRESSED&quot;)</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!physicalName.MatchString(physical) &#124;&#124; document == 0</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_INDEX_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_APPLICATION_READ_FAILED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>next.Sequence &lt; current.Sequence</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_APPLICATION_REGRESSED&quot;)</code>。当前函数在这里结束。
- 检查 <code>!apply</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>current.Revision == math.MaxUint64</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_INDEX_REVISION_EXHAUSTED&quot;)</code>。当前函数在这里结束。
- 检查 <code>next.Visible</code>。
- 执行 <code>tx.ExecContext(ctx, `UPDATE index_application SET applied_seq=?,authority_digest=?,release_id=?,visibility=?,visibility_policy=?, 		revision=?,updated_at=UTC_TIMESTAMP(6) WHERE phy …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_APPLICATION_WRITE_FAILED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; count != 1</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_APPLICATION_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=2700:4182 -->
````go
func SaveApplication(ctx context.Context, tx *sql.Tx, physical string, document uint64, next index.State) error {
	if !physicalName.MatchString(physical) || document == 0 {
		return errors.New("SEARCH_INDEX_BINDING_INVALID")
	}
	current, err := readApplication(ctx, tx, physical, document)
	if err != nil {
		return errors.New("SEARCH_APPLICATION_READ_FAILED")
	}
	apply, err := index.ShouldApply(current, index.Authority{Sequence: next.Sequence, Release: next.Release, Visible: next.Visible, Policy: next.Policy, Digest: next.Digest})
	if err != nil {
		return err
	}
	if next.Sequence < current.Sequence {
		return errors.New("SEARCH_APPLICATION_REGRESSED")
	}
	if !apply {
		return nil
	}
	if current.Revision == math.MaxUint64 {
		return errors.New("SEARCH_INDEX_REVISION_EXHAUSTED")
	}
	visibility := "HIDDEN"
	var release any
	if next.Visible {
		visibility, release = "PUBLIC", next.Release
	}
	result, err := tx.ExecContext(ctx, `UPDATE index_application SET applied_seq=?,authority_digest=?,release_id=?,visibility=?,visibility_policy=?,
		revision=?,updated_at=UTC_TIMESTAMP(6) WHERE physical_index=? AND document_id=? AND revision=?`, next.Sequence, next.Digest[:], release, visibility, next.Policy,
		current.Revision+1, physical, document, current.Revision)
	if err != nil {
		return errors.New("SEARCH_APPLICATION_WRITE_FAILED")
	}
	count, err := result.RowsAffected()
	if err != nil || count != 1 {
		return errors.New("SEARCH_APPLICATION_CHANGED")
	}
	return nil
}

````

## 第 5 步：Complete

插入位置：第 4 步代码之后，紧接追加；完成稿约第 109 行。

Complete只认可已经在同事务保存的实际应用状态；RPC/ES未知不能先写成功回执。

本步声明：`func Complete(ctx context.Context, tx *sql.Tx, consumer, id, physical string, applied index.State) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT document_id,aggregate_seq,state FROM consumed_event WHERE consumer_name=? AND event_id=? FOR UPDATE&quot;, consumer, id)</code>；<code>readApplication(ctx, tx, physical, document)</code>；<code>tx.ExecContext(ctx, `UPDATE event_origin SET applied_index=?,applied_seq=?,authority_digest=? 		WHERE consumer_name=? AND event_id=? AND applied_index IS NULL AND applied_seq IS NU …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, &quot;UPDATE consumed_event SET state=&#x27;SUCCEEDED&#x27;,last_error=NULL,retry_at=NULL WHERE consumer_name=? AND event_id=?&quot;, consumer, id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code>；<code>readApplication(ctx, tx, physical, document)</code>；<code>tx.ExecContext</code> |
| <code>consumer</code> | <code>string</code> | 事件账本和checkpoint所属消费组<br>在此声明中的实际使用：<code>consumerName.MatchString(consumer)</code>；<code>tx.QueryRowContext(ctx, &quot;SELECT document_id,aggregate_seq,state FROM consumed_event WHERE consumer_name=? AND event_id=? FOR UPDATE&quot;, consumer, id)</code>；<code>tx.ExecContext(ctx, `UPDATE event_origin SET applied_index=?,applied_seq=?,authority_digest=? 		WHERE consumer_name=? AND event_id=? AND applied_index IS NULL AND applied_seq IS NU …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, &quot;UPDATE consumed_event SET state=&#x27;SUCCEEDED&#x27;,last_error=NULL,retry_at=NULL WHERE consumer_name=? AND event_id=?&quot;, consumer, id)</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>len(id)</code>；<code>tx.QueryRowContext(ctx, &quot;SELECT document_id,aggregate_seq,state FROM consumed_event WHERE consumer_name=? AND event_id=? FOR UPDATE&quot;, consumer, id)</code>；<code>tx.ExecContext(ctx, `UPDATE event_origin SET applied_index=?,applied_seq=?,authority_digest=? 		WHERE consumer_name=? AND event_id=? AND applied_index IS NULL AND applied_seq IS NU …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, &quot;UPDATE consumed_event SET state=&#x27;SUCCEEDED&#x27;,last_error=NULL,retry_at=NULL WHERE consumer_name=? AND event_id=?&quot;, consumer, id)</code> |
| <code>physical</code> | <code>string</code> | ES物理索引名，应用水位按它隔离<br>在此声明中的实际使用：<code>physicalName.MatchString(physical)</code>；<code>readApplication(ctx, tx, physical, document)</code>；<code>tx.ExecContext(ctx, `UPDATE event_origin SET applied_index=?,applied_seq=?,authority_digest=? 		WHERE consumer_name=? AND event_id=? AND applied_index IS NULL AND applied_seq IS NU …</code>（长表达式，按本步完整代码继续） |
| <code>applied</code> | <code>index.State</code> | ES重读得到并已写SQL的实际高水位<br>在此声明中的实际使用：<code>applied.Sequence</code>；<code>applied.Digest</code>；<code>applied.Release</code>；<code>applied.Visible</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_APPLICATION_BINDING_INVALID&quot;)</code>；<code>errors.New(&quot;SEARCH_EVENT_RECEIPT_MISSING&quot;)</code>；<code>errors.New(&quot;SEARCH_EVENT_STATE_CHANGED&quot;)</code>；<code>errors.New(&quot;SEARCH_APPLICATION_NOT_DURABLE&quot;)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!consumerName.MatchString(consumer) &#124;&#124; !physicalName.MatchString(physical) &#124;&#124; len(id) == 0 &#124;&#124; len(id) &gt; 96</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_APPLICATION_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>tx.QueryRowContext(ctx, &quot;SELECT document_id,aggregate_seq,state FROM consumed_event WHERE consumer_name=? AND event_id=? FOR UPDATE&quot;, consumer, id). 		Scan(&amp;document, &amp;sequence, &amp;state) != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_EVENT_RECEIPT_MISSING&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT document_id,aggregate_seq,state FROM consumed_event WHERE consumer_name=? AND event_id=? FOR UPDATE&quot;, consumer, id)</code>。
- 检查 <code>state != &quot;RECEIVED&quot; &amp;&amp; state != &quot;APPLYING&quot; &amp;&amp; state != &quot;RETRY&quot;</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_EVENT_STATE_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; actual.Sequence &lt; sequence &#124;&#124; actual.Sequence != applied.Sequence &#124;&#124; actual.Digest != applied.Digest &#124;&#124; 		actual.Release != applied.Release &#124;&#124; actual.Visible != appli …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return errors.New(&quot;SEARCH_APPLICATION_NOT_DURABLE&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `UPDATE event_origin SET applied_index=?,applied_seq=?,authority_digest=? 		WHERE consumer_name=? AND event_id=? AND applied_index IS NULL AND applied_seq IS NU …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_ORIGIN_COMPLETION_FAILED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; count != 1</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_ORIGIN_COMPLETION_FAILED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_EVENT_COMPLETION_FAILED&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE consumed_event SET state=&#x27;SUCCEEDED&#x27;,last_error=NULL,retry_at=NULL WHERE consumer_name=? AND event_id=?&quot;, consumer, id)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=4182:6181 -->
````go
// Complete只认可已经在同事务保存的实际应用状态；RPC/ES未知不能先写成功回执。
func Complete(ctx context.Context, tx *sql.Tx, consumer, id, physical string, applied index.State) error {
	if !consumerName.MatchString(consumer) || !physicalName.MatchString(physical) || len(id) == 0 || len(id) > 96 {
		return errors.New("SEARCH_APPLICATION_BINDING_INVALID")
	}
	var document, sequence uint64
	var state string
	if tx.QueryRowContext(ctx, "SELECT document_id,aggregate_seq,state FROM consumed_event WHERE consumer_name=? AND event_id=? FOR UPDATE", consumer, id).
		Scan(&document, &sequence, &state) != nil {
		return errors.New("SEARCH_EVENT_RECEIPT_MISSING")
	}
	if state != "RECEIVED" && state != "APPLYING" && state != "RETRY" {
		return errors.New("SEARCH_EVENT_STATE_CHANGED")
	}
	actual, err := readApplication(ctx, tx, physical, document)
	if err != nil || actual.Sequence < sequence || actual.Sequence != applied.Sequence || actual.Digest != applied.Digest ||
		actual.Release != applied.Release || actual.Visible != applied.Visible || actual.Policy != applied.Policy || actual.Digest == ([32]byte{}) {
		return errors.New("SEARCH_APPLICATION_NOT_DURABLE")
	}
	result, err := tx.ExecContext(ctx, `UPDATE event_origin SET applied_index=?,applied_seq=?,authority_digest=?
		WHERE consumer_name=? AND event_id=? AND applied_index IS NULL AND applied_seq IS NULL AND authority_digest IS NULL`,
		physical, actual.Sequence, actual.Digest[:], consumer, id)
	if err != nil {
		return errors.New("SEARCH_ORIGIN_COMPLETION_FAILED")
	}
	count, err := result.RowsAffected()
	if err != nil || count != 1 {
		return errors.New("SEARCH_ORIGIN_COMPLETION_FAILED")
	}
	if _, err = tx.ExecContext(ctx, "UPDATE consumed_event SET state='SUCCEEDED',last_error=NULL,retry_at=NULL WHERE consumer_name=? AND event_id=?", consumer, id); err != nil {
		return errors.New("SEARCH_EVENT_COMPLETION_FAILED")
	}
	return nil
}

// checkpoint.go：保留原实现，按业务合并文件。

````

## 第 6 步：AdvanceCheckpoint

插入位置：第 5 步代码之后，紧接追加；完成稿约第 146 行。

AdvanceCheckpoint核对整条记录所有行成功，才写SQL下一offset；Kafka实际提交仍在tx.Commit之后。
原SQL已成功但Kafka未知时允许较早记录重读，不允许越过同分区未完成的中间记录。

本步声明：`func AdvanceCheckpoint(ctx context.Context, tx *sql.Tx, consumer string, position Position, events []domain.Event) error`。

**这一段为什么这样写：** 以整条Kafka记录全部行成功作为推进SQL next的条件。

1. 核每行原摘要及SUCCEEDED
2. 读取该分区next/revision
3. 旧记录重读不回退，未来缺口拒绝
4. 等于next时写offset+1


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 事务调用期限<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT c.state,o.raw_digest,o.kafka_topic,o.kafka_partition,o.kafka_offset 			FROM consumed_event c JOIN event_origin o ON o.consumer_name=c.consumer_name  …</code>（长表达式，按本步完整代码继续）；<code>tx.QueryRowContext(ctx, `SELECT next_offset,revision FROM consumer_checkpoint 		WHERE consumer_name=? AND kafka_topic=? AND partition_id=? FOR UPDATE`, consumer, position.Topic, position.Partition)</code>；<code>tx.ExecContext(ctx, `INSERT INTO consumer_checkpoint(consumer_name,kafka_topic,partition_id,next_offset) 			VALUES(?,?,?,1)`, consumer, position.Topic, position.Partition)</code>；<code>tx.ExecContext(ctx, `UPDATE consumer_checkpoint SET next_offset=?,revision=?,updated_at=UTC_TIMESTAMP(6) 		WHERE consumer_name=? AND kafka_topic=? AND partition_id=? AND revision=? …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 外层checkpoint事务<br>在此声明中的实际使用：<code>tx.QueryRowContext</code>；<code>tx.ExecContext</code> |
| <code>consumer</code> | <code>string</code> | 消费组账本命名空间<br>在此声明中的实际使用：<code>validatePosition(consumer, position)</code>；<code>tx.QueryRowContext(ctx, `SELECT c.state,o.raw_digest,o.kafka_topic,o.kafka_partition,o.kafka_offset 			FROM consumed_event c JOIN event_origin o ON o.consumer_name=c.consumer_name  …</code>（长表达式，按本步完整代码继续）；<code>tx.QueryRowContext(ctx, `SELECT next_offset,revision FROM consumer_checkpoint 		WHERE consumer_name=? AND kafka_topic=? AND partition_id=? FOR UPDATE`, consumer, position.Topic, position.Partition)</code>；<code>tx.ExecContext(ctx, `INSERT INTO consumer_checkpoint(consumer_name,kafka_topic,partition_id,next_offset) 			VALUES(?,?,?,1)`, consumer, position.Topic, position.Partition)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>position</code> | <code>Position</code> | 该完整记录位置<br>在此声明中的实际使用：<code>validatePosition(consumer, position)</code>；<code>position.Offset</code>；<code>position.Topic</code>；<code>position.Partition</code> |
| <code>events</code> | <code>&#91;&#93;domain.Event</code> | ParseCanal完整解析出的所有原行<br>在此声明中的实际使用：<code>len(events)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | nil仅表示事务内更新成立，外层仍需COMMIT<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;SEARCH_CHECKPOINT_INPUT_INVALID&quot;)</code>；<code>errors.New(&quot;SEARCH_RECORD_ROWS_NOT_COMPLETE&quot;)</code>；<code>errors.New(&quot;SEARCH_OFFSET_GAP&quot;)</code><br>另有 6 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>position.Offset == math.MaxInt64 &#124;&#124; len(events) == 0 &#124;&#124; len(events) &gt; 1024</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_CHECKPOINT_INPUT_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>tx.QueryRowContext(ctx, `SELECT c.state,o.raw_digest,o.kafka_topic,o.kafka_partition,o.kafka_offset 			FROM consumed_event c JOIN event_origin o ON o.consumer_name=c.consumer_name  …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return errors.New(&quot;SEARCH_RECORD_ROWS_NOT_COMPLETE&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT c.state,o.raw_digest,o.kafka_topic,o.kafka_partition,o.kafka_offset 			FROM consumed_event c JOIN event_origin o ON o.consumer_name=c.consumer_name  …</code>（长表达式，按本步完整代码继续）。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT next_offset,revision FROM consumer_checkpoint 		WHERE consumer_name=? AND kafka_topic=? AND partition_id=? FOR UPDATE`, consumer, position.Topic, position.Partition).Scan(&amp;next, &amp;revision)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT next_offset,revision FROM consumer_checkpoint 		WHERE consumer_name=? AND kafka_topic=? AND partition_id=? FOR UPDATE`, consumer, position.Topic, position.Partition)</code>。
- 检查 <code>err == sql.ErrNoRows</code>。
- 检查 <code>position.Offset != 0</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_OFFSET_GAP&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_CHECKPOINT_WRITE_FAILED&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `INSERT INTO consumer_checkpoint(consumer_name,kafka_topic,partition_id,next_offset) 			VALUES(?,?,?,1)`, consumer, position.Topic, position.Partition)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_CHECKPOINT_READ_FAILED&quot;)</code>。当前函数在这里结束。
- 检查 <code>next &gt; uint64(math.MaxInt64) &#124;&#124; revision == 0</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_CHECKPOINT_UNBOUND&quot;)</code>。当前函数在这里结束。
- 检查 <code>uint64(position.Offset) &gt; next</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_OFFSET_GAP&quot;)</code>。当前函数在这里结束。

<!-- write-step: 6 bytes=6181:8361 -->
````go
// AdvanceCheckpoint核对整条记录所有行成功，才写SQL下一offset；Kafka实际提交仍在tx.Commit之后。
// 原SQL已成功但Kafka未知时允许较早记录重读，不允许越过同分区未完成的中间记录。
func AdvanceCheckpoint(ctx context.Context, tx *sql.Tx, consumer string, position Position, events []domain.Event) error {
	if err := validatePosition(consumer, position); err != nil {
		return err
	}
	if position.Offset == math.MaxInt64 || len(events) == 0 || len(events) > 1024 {
		return errors.New("SEARCH_CHECKPOINT_INPUT_INVALID")
	}
	for _, value := range events {
		if err := validateEvent(value); err != nil {
			return err
		}
		var state, topic string
		var digest []byte
		var partition int
		var offset int64
		if tx.QueryRowContext(ctx, `SELECT c.state,o.raw_digest,o.kafka_topic,o.kafka_partition,o.kafka_offset
			FROM consumed_event c JOIN event_origin o ON o.consumer_name=c.consumer_name AND o.event_id=c.event_id
			WHERE c.consumer_name=? AND c.event_id=? FOR UPDATE`, consumer, value.ID).
			Scan(&state, &digest, &topic, &partition, &offset) != nil || state != "SUCCEEDED" ||
			!bytes.Equal(digest, value.RawDigest[:]) || topic != position.Topic || partition != position.Partition || offset > position.Offset {
			return errors.New("SEARCH_RECORD_ROWS_NOT_COMPLETE")
		}
	}
	var next, revision uint64
	err := tx.QueryRowContext(ctx, `SELECT next_offset,revision FROM consumer_checkpoint
		WHERE consumer_name=? AND kafka_topic=? AND partition_id=? FOR UPDATE`, consumer, position.Topic, position.Partition).Scan(&next, &revision)
	if err == sql.ErrNoRows {
		if position.Offset != 0 {
			return errors.New("SEARCH_OFFSET_GAP")
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO consumer_checkpoint(consumer_name,kafka_topic,partition_id,next_offset)
			VALUES(?,?,?,1)`, consumer, position.Topic, position.Partition); err != nil {
			return errors.New("SEARCH_CHECKPOINT_WRITE_FAILED")
		}
		return nil
	}
	if err != nil {
		return errors.New("SEARCH_CHECKPOINT_READ_FAILED")
	}
	if next > uint64(math.MaxInt64) || revision == 0 {
		return errors.New("SEARCH_CHECKPOINT_UNBOUND")
	}
	if uint64(position.Offset) > next {
````

## 第 7 步：AdvanceCheckpoint（接着实现）

插入位置：第 6 步代码之后，紧接追加；完成稿约第 191 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return errors.New(&quot;SEARCH_OFFSET_GAP&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>uint64(position.Offset) &lt; next</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>revision == math.MaxUint64</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_CHECKPOINT_REVISION_EXHAUSTED&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `UPDATE consumer_checkpoint SET next_offset=?,revision=?,updated_at=UTC_TIMESTAMP(6) 		WHERE consumer_name=? AND kafka_topic=? AND partition_id=? AND revision=? …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_CHECKPOINT_WRITE_FAILED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; count != 1</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_CHECKPOINT_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=8361:9116 -->
````go
		return errors.New("SEARCH_OFFSET_GAP")
	}
	if uint64(position.Offset) < next {
		return nil
	}
	if revision == math.MaxUint64 {
		return errors.New("SEARCH_CHECKPOINT_REVISION_EXHAUSTED")
	}
	result, err := tx.ExecContext(ctx, `UPDATE consumer_checkpoint SET next_offset=?,revision=?,updated_at=UTC_TIMESTAMP(6)
		WHERE consumer_name=? AND kafka_topic=? AND partition_id=? AND revision=?`, position.Offset+1, revision+1,
		consumer, position.Topic, position.Partition, revision)
	if err != nil {
		return errors.New("SEARCH_CHECKPOINT_WRITE_FAILED")
	}
	count, err := result.RowsAffected()
	if err != nil || count != 1 {
		return errors.New("SEARCH_CHECKPOINT_CHANGED")
	}
	return nil
}

// log_identity.go：保留原实现，按业务合并文件。

````

## 第 8 步：BindLogIdentity

插入位置：第 7 步代码之后，紧接追加；完成稿约第 214 行。

BindLogIdentity在调用方事务内固定消费者的物理日志，绝不覆盖旧身份。
旧checkpoint的首次升级需要调用方先验证历史来源；当前broker身份不能证明旧offset所属日志。

本步声明：`func BindLogIdentity(ctx context.Context, tx *sql.Tx, consumer, topic, cluster string, id [16]byte) error`。

**这一段为什么这样写：** 一次性将历史offset解释绑定到物理日志。

1. 尝试INSERT
2. 只容忍主键已存在
3. FOR SHARE读取并严格比较topic/cluster/UUID/revision1


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 事务期限<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `INSERT INTO consumer_log_identity(consumer_name,kafka_topic,cluster_id,topic_id) 		VALUES(?,?,?,?)`, consumer, topic, cluster, id&#91;:&#93;)</code>；<code>tx.QueryRowContext(ctx, `SELECT kafka_topic,cluster_id,topic_id,revision FROM consumer_log_identity 		WHERE consumer_name=? FOR SHARE`, consumer)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 当前绑定事务<br>在此声明中的实际使用：<code>tx == nil</code>；<code>tx.ExecContext</code>；<code>tx.QueryRowContext</code> |
| <code>consumer</code> | <code>string</code> | 消费组身份<br>在此声明中的实际使用：<code>consumerName.MatchString(consumer)</code>；<code>tx.ExecContext(ctx, `INSERT INTO consumer_log_identity(consumer_name,kafka_topic,cluster_id,topic_id) 		VALUES(?,?,?,?)`, consumer, topic, cluster, id&#91;:&#93;)</code>；<code>tx.QueryRowContext(ctx, `SELECT kafka_topic,cluster_id,topic_id,revision FROM consumer_log_identity 		WHERE consumer_name=? FOR SHARE`, consumer)</code> |
| <code>topic</code> | <code>string</code> | 主题名称<br>在此声明中的实际使用：<code>topicName.MatchString(topic)</code>；<code>tx.ExecContext(ctx, `INSERT INTO consumer_log_identity(consumer_name,kafka_topic,cluster_id,topic_id) 		VALUES(?,?,?,?)`, consumer, topic, cluster, id&#91;:&#93;)</code>；<code>storedTopic != topic</code> |
| <code>cluster</code> | <code>string</code> | Kafka ClusterID<br>在此声明中的实际使用：<code>validClusterID(cluster)</code>；<code>tx.ExecContext(ctx, `INSERT INTO consumer_log_identity(consumer_name,kafka_topic,cluster_id,topic_id) 		VALUES(?,?,?,?)`, consumer, topic, cluster, id&#91;:&#93;)</code>；<code>storedCluster != cluster</code> |
| <code>id</code> | <code>&#91;16&#93;byte</code> | Kafka TopicID<br>在此声明中的实际使用：<code>id == (&#91;16&#93;byte{})</code>；<code>tx.ExecContext(ctx, `INSERT INTO consumer_log_identity(consumer_name,kafka_topic,cluster_id,topic_id) 		VALUES(?,?,?,?)`, consumer, topic, cluster, id&#91;:&#93;)</code>；<code>bytes.Equal(storedID, id&#91;:&#93;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 身份已相同绑定或稳定错误<br>实际返回表达式：<code>errors.New(&quot;SEARCH_LOG_IDENTITY_INVALID&quot;)</code>；<code>errors.New(&quot;SEARCH_LOG_IDENTITY_WRITE_FAILED&quot;)</code>；<code>errors.New(&quot;SEARCH_LOG_IDENTITY_READ_FAILED&quot;)</code>；<code>errors.New(&quot;SEARCH_LOG_IDENTITY_CHANGED&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>tx == nil &#124;&#124; !consumerName.MatchString(consumer) &#124;&#124; !topicName.MatchString(topic) &#124;&#124; 		!validClusterID(cluster) &#124;&#124; id == (&#91;16&#93;byte{})</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_LOG_IDENTITY_INVALID&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `INSERT INTO consumer_log_identity(consumer_name,kafka_topic,cluster_id,topic_id) 		VALUES(?,?,?,?)`, consumer, topic, cluster, id&#91;:&#93;)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。
- 检查 <code>!errors.As(err, &amp;duplicate) &#124;&#124; duplicate.Number != 1062</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_LOG_IDENTITY_WRITE_FAILED&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT kafka_topic,cluster_id,topic_id,revision FROM consumer_log_identity 		WHERE consumer_name=? FOR SHARE`, consumer).Scan(&amp;storedTopic, &amp;storedCluster, &amp;storedID, &amp;revision)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT kafka_topic,cluster_id,topic_id,revision FROM consumer_log_identity 		WHERE consumer_name=? FOR SHARE`, consumer)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_LOG_IDENTITY_READ_FAILED&quot;)</code>。当前函数在这里结束。
- 检查 <code>storedTopic != topic &#124;&#124; storedCluster != cluster &#124;&#124; !bytes.Equal(storedID, id&#91;:&#93;) &#124;&#124; revision != 1</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_LOG_IDENTITY_CHANGED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=9116:10809 -->
````go
// BindLogIdentity在调用方事务内固定消费者的物理日志，绝不覆盖旧身份。
// 旧checkpoint的首次升级需要调用方先验证历史来源；当前broker身份不能证明旧offset所属日志。
func BindLogIdentity(ctx context.Context, tx *sql.Tx, consumer, topic, cluster string, id [16]byte) error {
	if tx == nil || !consumerName.MatchString(consumer) || !topicName.MatchString(topic) ||
		!validClusterID(cluster) || id == ([16]byte{}) {
		return errors.New("SEARCH_LOG_IDENTITY_INVALID")
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO consumer_log_identity(consumer_name,kafka_topic,cluster_id,topic_id)
		VALUES(?,?,?,?)`, consumer, topic, cluster, id[:])
	if err != nil {
		// 只接受主键冲突；INSERT IGNORE会同时吞掉截断等错误，不能用于身份凭据。
		var duplicate *mysql.MySQLError
		if !errors.As(err, &duplicate) || duplicate.Number != 1062 {
			return errors.New("SEARCH_LOG_IDENTITY_WRITE_FAILED")
		}
	}
	var storedTopic, storedCluster string
	var storedID []byte
	var revision uint64
	// 当前读避免REPEATABLE READ旧快照漏掉并发首次绑定；共享锁只需SELECT权限。
	// 本表没有UPDATE/DELETE授权，无需通过更新原事实来取得锁。
	err = tx.QueryRowContext(ctx, `SELECT kafka_topic,cluster_id,topic_id,revision FROM consumer_log_identity
		WHERE consumer_name=? FOR SHARE`, consumer).Scan(&storedTopic, &storedCluster, &storedID, &revision)
	if err != nil {
		return errors.New("SEARCH_LOG_IDENTITY_READ_FAILED")
	}
	if storedTopic != topic || storedCluster != cluster || !bytes.Equal(storedID, id[:]) || revision != 1 {
		return errors.New("SEARCH_LOG_IDENTITY_CHANGED")
	}
	return nil
}

````

## 第 9 步：validClusterID

插入位置：第 8 步代码之后，紧接追加；完成稿约第 246 行。

本步声明：`func validClusterID(value string) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>string</code> | 不可变原outbox事件<br>在此声明中的实际使用：<code>len(value)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(value) == 0 &#124;&#124; len(value) &gt; 160</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>character &lt; 33 &#124;&#124; character &gt; 126</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 此处通过 <code>return true</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=10809:11084 -->
````go
func validClusterID(value string) bool {
	if len(value) == 0 || len(value) > 160 {
		return false
	}
	for _, character := range value {
		if character < 33 || character > 126 {
			return false
		}
	}
	return true
}

// origin.go：保留原实现，按业务合并文件。

````

## 第 10 步：Receipt

插入位置：第 9 步代码之后，紧接追加；完成稿约第 260 行。

本步声明：`type Receipt struct {`。

**这一段为什么这样写：** 原事件第一次应用事实的最小重放信息。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>State</code> | <code>string</code> | consumed_event阶段：RECEIVED/APPLYING/RETRY/SUCCEEDED等 |
| <code>Physical</code> | <code>string</code> | 首次成功应用的物理索引，未成功为空 |
| <code>Applied</code> | <code>index.State</code> | 首次成功权威水位和摘要，不是当前最新状态 |

<!-- write-step: 10 bytes=11084:11165 -->
````go
type Receipt struct {
	State    string
	Physical string
	Applied  index.State
}

````

## 第 11 步：Receive

插入位置：第 10 步代码之后，紧接追加；完成稿约第 266 行。

Receive接受调用方同一事务：原回执和原行一起落定，不把重复投递的后来内容覆盖原事实。

本步声明：`func Receive(ctx context.Context, tx *sql.Tx, consumer string, value domain.Event, position Position) (Receipt, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT c.document_id,c.aggregate_seq,c.state, 		o.raw_digest,o.original_row,o.kafka_topic,o.kafka_partition,CAST(o.kafka_offset AS CHAR), 		o.applied_index …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, &quot;INSERT INTO consumed_event(consumer_name,event_id,document_id,aggregate_seq,state) VALUES(?,?,?,?,&#x27;RECEIVED&#x27;)&quot;, consumer, value.ID, value.Document, value.Sequence)</code>；<code>tx.ExecContext(ctx, `INSERT INTO event_origin(consumer_name,event_id,raw_digest,original_row,kafka_topic,kafka_partition,kafka_offset) 			VALUES(?,?,?,?,?,?,?)`, consumer, value.ID …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code>；<code>tx.ExecContext</code> |
| <code>consumer</code> | <code>string</code> | 事件账本和checkpoint所属消费组<br>在此声明中的实际使用：<code>validatePosition(consumer, position)</code>；<code>tx.QueryRowContext(ctx, `SELECT c.document_id,c.aggregate_seq,c.state, 		o.raw_digest,o.original_row,o.kafka_topic,o.kafka_partition,CAST(o.kafka_offset AS CHAR), 		o.applied_index …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, &quot;INSERT INTO consumed_event(consumer_name,event_id,document_id,aggregate_seq,state) VALUES(?,?,?,?,&#x27;RECEIVED&#x27;)&quot;, consumer, value.ID, value.Document, value.Sequence)</code>；<code>tx.ExecContext(ctx, `INSERT INTO event_origin(consumer_name,event_id,raw_digest,original_row,kafka_topic,kafka_partition,kafka_offset) 			VALUES(?,?,?,?,?,?,?)`, consumer, value.ID …</code>（长表达式，按本步完整代码继续） |
| <code>value</code> | <code>domain.Event</code> | 不可变原outbox事件<br>在此声明中的实际使用：<code>validateEvent(value)</code>；<code>value.ID</code>；<code>value.Document</code>；<code>value.Sequence</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>position</code> | <code>Position</code> | 原Kafka topic/partition/offset<br>在此声明中的实际使用：<code>validatePosition(consumer, position)</code>；<code>position.Topic</code>；<code>position.Partition</code>；<code>position.Offset</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Receipt</code> | 使用 <code>Receipt</code> 类型；其定义处的职责：原事件第一次应用事实的最小重放信息。<br>实际返回表达式：<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;SEARCH_LEDGER_WRITE_FAILED&quot;)</code>；<code>errors.New(&quot;SEARCH_ORIGIN_WRITE_FAILED&quot;)</code>；<code>nil</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT c.document_id,c.aggregate_seq,c.state, 		o.raw_digest,o.original_row,o.kafka_topic,o.kafka_partition,CAST(o.kafka_offset AS CHAR), 		o.applied_index …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err == sql.ErrNoRows</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, errors.New(&quot;SEARCH_LEDGER_WRITE_FAILED&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO consumed_event(consumer_name,event_id,document_id,aggregate_seq,state) VALUES(?,?,?,?,&#x27;RECEIVED&#x27;)&quot;, consumer, value.ID, value.Document, value.Sequence)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, errors.New(&quot;SEARCH_ORIGIN_WRITE_FAILED&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `INSERT INTO event_origin(consumer_name,event_id,raw_digest,original_row,kafka_topic,kafka_partition,kafka_offset) 			VALUES(?,?,?,?,?,?,?)`, consumer, value.ID …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, errors.New(&quot;SEARCH_LEDGER_READ_FAILED&quot;)</code>。当前函数在这里结束。
- 检查 <code>document != value.Document &#124;&#124; sequence != value.Sequence &#124;&#124; len(rawDigest) != 32 &#124;&#124; 		!bytes.Equal(rawDigest, value.RawDigest&#91;:&#93;) &#124;&#124; !bytes.Equal(original, value.Original) &#124;&#124; 		!to …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return result, errors.New(&quot;SEARCH_ORIGINAL_EVENT_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; first &lt; 0</code>。条件成立时执行 <code>return result, errors.New(&quot;SEARCH_ORIGIN_POSITION_UNBOUND&quot;)</code>。当前函数在这里结束。
- 检查 <code>physical.Valid &#124;&#124; applied.Valid &#124;&#124; len(authority) != 0</code>。条件成立时执行 <code>return result, errors.New(&quot;SEARCH_APPLICATION_RECEIPT_UNBOUND&quot;)</code>。当前函数在这里结束。

<!-- write-step: 11 bytes=11165:13755 -->
````go
// Receive接受调用方同一事务：原回执和原行一起落定，不把重复投递的后来内容覆盖原事实。
func Receive(ctx context.Context, tx *sql.Tx, consumer string, value domain.Event, position Position) (Receipt, error) {
	var result Receipt
	if err := validatePosition(consumer, position); err != nil {
		return result, err
	}
	if err := validateEvent(value); err != nil {
		return result, err
	}
	var document, sequence uint64
	var rawDigest, original, authority []byte
	var topic, offset, physical, applied sql.NullString
	var partition sql.NullInt64
	err := tx.QueryRowContext(ctx, `SELECT c.document_id,c.aggregate_seq,c.state,
		o.raw_digest,o.original_row,o.kafka_topic,o.kafka_partition,CAST(o.kafka_offset AS CHAR),
		o.applied_index,CAST(o.applied_seq AS CHAR),o.authority_digest
		FROM consumed_event c LEFT JOIN event_origin o ON o.consumer_name=c.consumer_name AND o.event_id=c.event_id
		WHERE c.consumer_name=? AND c.event_id=? FOR UPDATE`, consumer, value.ID).
		Scan(&document, &sequence, &result.State, &rawDigest, &original, &topic, &partition, &offset, &physical, &applied, &authority)
	if err == sql.ErrNoRows {
		if _, err = tx.ExecContext(ctx, "INSERT INTO consumed_event(consumer_name,event_id,document_id,aggregate_seq,state) VALUES(?,?,?,?,'RECEIVED')", consumer, value.ID, value.Document, value.Sequence); err != nil {
			return result, errors.New("SEARCH_LEDGER_WRITE_FAILED")
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO event_origin(consumer_name,event_id,raw_digest,original_row,kafka_topic,kafka_partition,kafka_offset)
			VALUES(?,?,?,?,?,?,?)`, consumer, value.ID, value.RawDigest[:], value.Original, position.Topic, position.Partition, position.Offset); err != nil {
			return result, errors.New("SEARCH_ORIGIN_WRITE_FAILED")
		}
		result.State = "RECEIVED"
		return result, nil
	}
	if err != nil {
		return result, errors.New("SEARCH_LEDGER_READ_FAILED")
	}
	if document != value.Document || sequence != value.Sequence || len(rawDigest) != 32 ||
		!bytes.Equal(rawDigest, value.RawDigest[:]) || !bytes.Equal(original, value.Original) ||
		!topic.Valid || topic.String != position.Topic || !partition.Valid || partition.Int64 != int64(position.Partition) || !offset.Valid {
		return result, errors.New("SEARCH_ORIGINAL_EVENT_CONFLICT")
	}
	first, err := strconv.ParseInt(offset.String, 10, 64)
	if err != nil || first < 0 {
		return result, errors.New("SEARCH_ORIGIN_POSITION_UNBOUND")
	}
	switch result.State {
	case "RECEIVED", "APPLYING", "RETRY", "REJECTED":
		if physical.Valid || applied.Valid || len(authority) != 0 {
````

## 第 12 步：Receive（接着实现）

插入位置：第 11 步代码之后，紧接追加；完成稿约第 311 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return result, errors.New(&quot;SEARCH_APPLICATION_RECEIPT_UNBOUND&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!physical.Valid &#124;&#124; !physicalName.MatchString(physical.String) &#124;&#124; !applied.Valid &#124;&#124; len(authority) != 32</code>。条件成立时执行 <code>return result, errors.New(&quot;SEARCH_APPLICATION_RECEIPT_UNBOUND&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; result.Applied.Sequence &lt; sequence</code>。条件成立时执行 <code>return result, errors.New(&quot;SEARCH_APPLICATION_RECEIPT_UNBOUND&quot;)</code>。当前函数在这里结束。
- 检查 <code>result.Applied.Digest == (&#91;32&#93;byte{})</code>。条件成立时执行 <code>return result, errors.New(&quot;SEARCH_APPLICATION_RECEIPT_UNBOUND&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return result, errors.New(&quot;SEARCH_EVENT_STATE_UNKNOWN&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=13755:14519 -->
````go
			return result, errors.New("SEARCH_APPLICATION_RECEIPT_UNBOUND")
		}
	case "SUCCEEDED":
		if !physical.Valid || !physicalName.MatchString(physical.String) || !applied.Valid || len(authority) != 32 {
			return result, errors.New("SEARCH_APPLICATION_RECEIPT_UNBOUND")
		}
		result.Physical = physical.String
		result.Applied.Sequence, err = strconv.ParseUint(applied.String, 10, 64)
		if err != nil || result.Applied.Sequence < sequence {
			return result, errors.New("SEARCH_APPLICATION_RECEIPT_UNBOUND")
		}
		copy(result.Applied.Digest[:], authority)
		if result.Applied.Digest == ([32]byte{}) {
			return result, errors.New("SEARCH_APPLICATION_RECEIPT_UNBOUND")
		}
	default:
		return result, errors.New("SEARCH_EVENT_STATE_UNKNOWN")
	}
	return result, nil
}

````

## 第 13 步：SetApplying

插入位置：第 12 步代码之后，紧接追加；完成稿约第 332 行。

本步声明：`func SetApplying(ctx context.Context, tx *sql.Tx, consumer, id string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>transition(ctx, tx, consumer, id, &quot;APPLYING&quot;, &quot;&quot;)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>transition(ctx, tx, consumer, id, &quot;APPLYING&quot;, &quot;&quot;)</code> |
| <code>consumer</code> | <code>string</code> | 事件账本和checkpoint所属消费组<br>在此声明中的实际使用：<code>transition(ctx, tx, consumer, id, &quot;APPLYING&quot;, &quot;&quot;)</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>transition(ctx, tx, consumer, id, &quot;APPLYING&quot;, &quot;&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>transition(ctx, tx, consumer, id, &quot;APPLYING&quot;, &quot;&quot;)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return transition(ctx, tx, consumer, id, &quot;APPLYING&quot;, &quot;&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=14519:14658 -->
````go
func SetApplying(ctx context.Context, tx *sql.Tx, consumer, id string) error {
	return transition(ctx, tx, consumer, id, "APPLYING", "")
}
````

## 第 14 步：SetRetry

插入位置：第 13 步代码之后，紧接追加；完成稿约第 335 行。

本步声明：`func SetRetry(ctx context.Context, tx *sql.Tx, consumer, id, code string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>transition(ctx, tx, consumer, id, &quot;RETRY&quot;, code)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>transition(ctx, tx, consumer, id, &quot;RETRY&quot;, code)</code> |
| <code>consumer</code> | <code>string</code> | 事件账本和checkpoint所属消费组<br>在此声明中的实际使用：<code>transition(ctx, tx, consumer, id, &quot;RETRY&quot;, code)</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>transition(ctx, tx, consumer, id, &quot;RETRY&quot;, code)</code> |
| <code>code</code> | <code>string</code> | 在此声明中的实际使用：<code>errorCode.MatchString(code)</code>；<code>transition(ctx, tx, consumer, id, &quot;RETRY&quot;, code)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_RETRY_CODE_INVALID&quot;)</code>；<code>transition(ctx, tx, consumer, id, &quot;RETRY&quot;, code)</code> |

**跟着本段代码走一遍：**

- 检查 <code>!errorCode.MatchString(code)</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_RETRY_CODE_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return transition(ctx, tx, consumer, id, &quot;RETRY&quot;, code)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=14658:14886 -->
````go
func SetRetry(ctx context.Context, tx *sql.Tx, consumer, id, code string) error {
	if !errorCode.MatchString(code) {
		return errors.New("SEARCH_RETRY_CODE_INVALID")
	}
	return transition(ctx, tx, consumer, id, "RETRY", code)
}
````

## 第 15 步：transition

插入位置：第 14 步代码之后，紧接追加；完成稿约第 341 行。

本步声明：`func transition(ctx context.Context, tx *sql.Tx, consumer, id, state, code string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE consumed_event SET state=?,last_error=NULLIF(?,&#x27;&#x27;), 		retry_at=CASE WHEN ?=&#x27;RETRY&#x27; THEN DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND) ELSE NULL END 		WHE …</code>（长表达式，按本步完整代码继续）；<code>tx.QueryRowContext(ctx, &quot;SELECT state FROM consumed_event WHERE consumer_name=? AND event_id=? FOR UPDATE&quot;, consumer, id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code>；<code>tx.QueryRowContext</code> |
| <code>consumer</code> | <code>string</code> | 事件账本和checkpoint所属消费组<br>在此声明中的实际使用：<code>consumerName.MatchString(consumer)</code>；<code>tx.ExecContext(ctx, `UPDATE consumed_event SET state=?,last_error=NULLIF(?,&#x27;&#x27;), 		retry_at=CASE WHEN ?=&#x27;RETRY&#x27; THEN DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND) ELSE NULL END 		WHE …</code>（长表达式，按本步完整代码继续）；<code>tx.QueryRowContext(ctx, &quot;SELECT state FROM consumed_event WHERE consumer_name=? AND event_id=? FOR UPDATE&quot;, consumer, id)</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>len(id)</code>；<code>tx.ExecContext(ctx, `UPDATE consumed_event SET state=?,last_error=NULLIF(?,&#x27;&#x27;), 		retry_at=CASE WHEN ?=&#x27;RETRY&#x27; THEN DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND) ELSE NULL END 		WHE …</code>（长表达式，按本步完整代码继续）；<code>tx.QueryRowContext(ctx, &quot;SELECT state FROM consumed_event WHERE consumer_name=? AND event_id=? FOR UPDATE&quot;, consumer, id)</code> |
| <code>state</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE consumed_event SET state=?,last_error=NULLIF(?,&#x27;&#x27;), 		retry_at=CASE WHEN ?=&#x27;RETRY&#x27; THEN DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND) ELSE NULL END 		WHE …</code>（长表达式，按本步完整代码继续）；<code>actual != state</code> |
| <code>code</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, `UPDATE consumed_event SET state=?,last_error=NULLIF(?,&#x27;&#x27;), 		retry_at=CASE WHEN ?=&#x27;RETRY&#x27; THEN DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND) ELSE NULL END 		WHE …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_EVENT_BINDING_INVALID&quot;)</code>；<code>errors.New(&quot;SEARCH_EVENT_TRANSITION_FAILED&quot;)</code>；<code>errors.New(&quot;SEARCH_EVENT_STATE_CHANGED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>!consumerName.MatchString(consumer) &#124;&#124; len(id) == 0 &#124;&#124; len(id) &gt; 96</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_EVENT_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `UPDATE consumed_event SET state=?,last_error=NULLIF(?,&#x27;&#x27;), 		retry_at=CASE WHEN ?=&#x27;RETRY&#x27; THEN DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND) ELSE NULL END 		WHE …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_EVENT_TRANSITION_FAILED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; count &gt; 1</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_EVENT_TRANSITION_FAILED&quot;)</code>。当前函数在这里结束。
- 检查 <code>tx.QueryRowContext(ctx, &quot;SELECT state FROM consumed_event WHERE consumer_name=? AND event_id=? FOR UPDATE&quot;, consumer, id).Scan(&amp;actual) != nil &#124;&#124; actual != state</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_EVENT_STATE_CHANGED&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state FROM consumed_event WHERE consumer_name=? AND event_id=? FOR UPDATE&quot;, consumer, id).Scan(&amp;actual)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state FROM consumed_event WHERE consumer_name=? AND event_id=? FOR UPDATE&quot;, consumer, id)</code>。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=14886:16050 -->
````go
func transition(ctx context.Context, tx *sql.Tx, consumer, id, state, code string) error {
	if !consumerName.MatchString(consumer) || len(id) == 0 || len(id) > 96 {
		return errors.New("SEARCH_EVENT_BINDING_INVALID")
	}
	result, err := tx.ExecContext(ctx, `UPDATE consumed_event SET state=?,last_error=NULLIF(?,''),
		retry_at=CASE WHEN ?='RETRY' THEN DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 1 SECOND) ELSE NULL END
		WHERE consumer_name=? AND event_id=? AND state IN ('RECEIVED','APPLYING','RETRY')`, state, code, state, consumer, id)
	if err != nil {
		return errors.New("SEARCH_EVENT_TRANSITION_FAILED")
	}
	count, err := result.RowsAffected()
	if err != nil || count > 1 {
		return errors.New("SEARCH_EVENT_TRANSITION_FAILED")
	}
	// 同一APPLYING重试可返回0 affected rows；必须再读实际状态，不能把缺行解释为成功。
	var actual string
	if tx.QueryRowContext(ctx, "SELECT state FROM consumed_event WHERE consumer_name=? AND event_id=? FOR UPDATE", consumer, id).Scan(&actual) != nil || actual != state {
		return errors.New("SEARCH_EVENT_STATE_CHANGED")
	}
	return nil
}

// read_checkpoint.go：保留原实现，按业务合并文件。

````

## 第 16 步：Checkpoint

插入位置：第 15 步代码之后，紧接追加；完成稿约第 365 行。

Checkpoint读的是本消费者已落定的下一offset。没有行只代表从0开始，
不能把当前Kafka记录位置或broker最新位置当成已消费过的历史。

本步声明：`func Checkpoint(ctx context.Context, tx *sql.Tx, consumer, topic string, partition int) (int64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT next_offset,revision FROM consumer_checkpoint 		WHERE consumer_name=? AND kafka_topic=? AND partition_id=?`, consumer, topic, partition)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>consumer</code> | <code>string</code> | 事件账本和checkpoint所属消费组<br>在此声明中的实际使用：<code>validatePosition(consumer, Position{Topic: topic, Partition: partition})</code>；<code>tx.QueryRowContext(ctx, `SELECT next_offset,revision FROM consumer_checkpoint 		WHERE consumer_name=? AND kafka_topic=? AND partition_id=?`, consumer, topic, partition)</code> |
| <code>topic</code> | <code>string</code> | 在此声明中的实际使用：<code>Topic: topic</code>；<code>tx.QueryRowContext(ctx, `SELECT next_offset,revision FROM consumer_checkpoint 		WHERE consumer_name=? AND kafka_topic=? AND partition_id=?`, consumer, topic, partition)</code> |
| <code>partition</code> | <code>int</code> | 在此声明中的实际使用：<code>Partition: partition</code>；<code>tx.QueryRowContext(ctx, `SELECT next_offset,revision FROM consumer_checkpoint 		WHERE consumer_name=? AND kafka_topic=? AND partition_id=?`, consumer, topic, partition)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>int64</code> | 实际返回表达式：<code>0</code>；<code>int64(next)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code>；<code>errors.New(&quot;SEARCH_CHECKPOINT_READ_FAILED&quot;)</code>；<code>errors.New(&quot;SEARCH_CHECKPOINT_UNBOUND&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return 0, err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT next_offset,revision FROM consumer_checkpoint 		WHERE consumer_name=? AND kafka_topic=? AND partition_id=?`, consumer, topic, partition).Scan(&amp;next, &amp;revision)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT next_offset,revision FROM consumer_checkpoint 		WHERE consumer_name=? AND kafka_topic=? AND partition_id=?`, consumer, topic, partition)</code>。
- 检查 <code>err == sql.ErrNoRows</code>。条件成立时执行 <code>return 0, nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return 0, errors.New(&quot;SEARCH_CHECKPOINT_READ_FAILED&quot;)</code>。当前函数在这里结束。
- 检查 <code>next &gt; math.MaxInt64 &#124;&#124; revision == 0</code>。条件成立时执行 <code>return 0, errors.New(&quot;SEARCH_CHECKPOINT_UNBOUND&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return int64(next), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=16050:16989 -->
````go
// Checkpoint读的是本消费者已落定的下一offset。没有行只代表从0开始，
// 不能把当前Kafka记录位置或broker最新位置当成已消费过的历史。
func Checkpoint(ctx context.Context, tx *sql.Tx, consumer, topic string, partition int) (int64, error) {
	if err := validatePosition(consumer, Position{Topic: topic, Partition: partition}); err != nil {
		return 0, err
	}
	var next, revision uint64
	err := tx.QueryRowContext(ctx, `SELECT next_offset,revision FROM consumer_checkpoint
		WHERE consumer_name=? AND kafka_topic=? AND partition_id=?`, consumer, topic, partition).Scan(&next, &revision)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, errors.New("SEARCH_CHECKPOINT_READ_FAILED")
	}
	if next > math.MaxInt64 || revision == 0 {
		return 0, errors.New("SEARCH_CHECKPOINT_UNBOUND")
	}
	return int64(next), nil
}

// recovery.go：保留原实现，按业务合并文件。

````

## 第 17 步：CheckRecovery

插入位置：第 16 步代码之后，紧接追加；完成稿约第 388 行。

启动前核验本消费者已有账本，缺原行、改摘要、错误topic和假成功都拒绝。
未完成但原事实完整的行允许从原Kafka位置恢复；这不是“待办为空”的就绪判断。
MEDIUMBLOB的摘要按原字节核验；JSON_VALID需显式解释为UTF-8文本，不能检查binary字符集。

本步声明：`func CheckRecovery(ctx context.Context, tx *sql.Tx, consumer, topic string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, `SELECT partition_id FROM consumer_checkpoint WHERE consumer_name=? 		AND (kafka_topic&lt;&gt;? OR partition_id&gt;=? OR next_offset&gt;9223372036854775807 OR revision= …</code>（长表达式，按本步完整代码继续）；<code>tx.QueryRowContext(ctx, `SELECT c.event_id FROM consumed_event c 		LEFT JOIN event_origin o ON o.consumer_name=c.consumer_name AND o.event_id=c.event_id 		LEFT JOIN index_applicati …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>consumer</code> | <code>string</code> | 事件账本和checkpoint所属消费组<br>在此声明中的实际使用：<code>validatePosition(consumer, Position{Topic: topic})</code>；<code>tx.QueryRowContext(ctx, `SELECT partition_id FROM consumer_checkpoint WHERE consumer_name=? 		AND (kafka_topic&lt;&gt;? OR partition_id&gt;=? OR next_offset&gt;9223372036854775807 OR revision= …</code>（长表达式，按本步完整代码继续）；<code>tx.QueryRowContext(ctx, `SELECT c.event_id FROM consumed_event c 		LEFT JOIN event_origin o ON o.consumer_name=c.consumer_name AND o.event_id=c.event_id 		LEFT JOIN index_applicati …</code>（长表达式，按本步完整代码继续） |
| <code>topic</code> | <code>string</code> | 在此声明中的实际使用：<code>Topic: topic</code>；<code>tx.QueryRowContext(ctx, `SELECT partition_id FROM consumer_checkpoint WHERE consumer_name=? 		AND (kafka_topic&lt;&gt;? OR partition_id&gt;=? OR next_offset&gt;9223372036854775807 OR revision= …</code>（长表达式，按本步完整代码继续）；<code>tx.QueryRowContext(ctx, `SELECT c.event_id FROM consumed_event c 		LEFT JOIN event_origin o ON o.consumer_name=c.consumer_name AND o.event_id=c.event_id 		LEFT JOIN index_applicati …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;SEARCH_CHECKPOINT_UNBOUND&quot;)</code>；<code>errors.New(&quot;SEARCH_RECOVERY_LEDGER_READ_FAILED&quot;)</code>；<code>nil</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT partition_id FROM consumer_checkpoint WHERE consumer_name=? 		AND (kafka_topic&lt;&gt;? OR partition_id&gt;=? OR next_offset&gt;9223372036854775807 OR revision= …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT partition_id FROM consumer_checkpoint WHERE consumer_name=? 		AND (kafka_topic&lt;&gt;? OR partition_id&gt;=? OR next_offset&gt;9223372036854775807 OR revision= …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>checkpointErr == nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_CHECKPOINT_UNBOUND&quot;)</code>。当前函数在这里结束。
- 检查 <code>checkpointErr != sql.ErrNoRows</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_RECOVERY_LEDGER_READ_FAILED&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT c.event_id FROM consumed_event c 		LEFT JOIN event_origin o ON o.consumer_name=c.consumer_name AND o.event_id=c.event_id 		LEFT JOIN index_applicati …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT c.event_id FROM consumed_event c 		LEFT JOIN event_origin o ON o.consumer_name=c.consumer_name AND o.event_id=c.event_id 		LEFT JOIN index_applicati …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err == sql.ErrNoRows</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_RECOVERY_LEDGER_READ_FAILED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return errors.New(&quot;SEARCH_ORIGIN_RECOVERY_REQUIRED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 17 bytes=16989:19213 -->
````go
// 启动前核验本消费者已有账本，缺原行、改摘要、错误topic和假成功都拒绝。
// 未完成但原事实完整的行允许从原Kafka位置恢复；这不是“待办为空”的就绪判断。
// MEDIUMBLOB的摘要按原字节核验；JSON_VALID需显式解释为UTF-8文本，不能检查binary字符集。
func CheckRecovery(ctx context.Context, tx *sql.Tx, consumer, topic string) error {
	if err := validatePosition(consumer, Position{Topic: topic}); err != nil {
		return err
	}
	var id string
	var partition uint64
	checkpointErr := tx.QueryRowContext(ctx, `SELECT partition_id FROM consumer_checkpoint WHERE consumer_name=?
		AND (kafka_topic<>? OR partition_id>=? OR next_offset>9223372036854775807 OR revision=0) LIMIT 1`, consumer, topic, domain.PartitionCount).Scan(&partition)
	if checkpointErr == nil {
		return errors.New("SEARCH_CHECKPOINT_UNBOUND")
	}
	if checkpointErr != sql.ErrNoRows {
		return errors.New("SEARCH_RECOVERY_LEDGER_READ_FAILED")
	}
	err := tx.QueryRowContext(ctx, `SELECT c.event_id FROM consumed_event c
		LEFT JOIN event_origin o ON o.consumer_name=c.consumer_name AND o.event_id=c.event_id
		LEFT JOIN index_application a ON a.physical_index=o.applied_index AND a.document_id=c.document_id
		WHERE c.consumer_name=? AND (o.event_id IS NULL OR o.kafka_topic<>? OR o.kafka_partition>=?
		OR o.kafka_offset>9223372036854775806 OR o.raw_digest<>UNHEX(SHA2(o.original_row,256))
		OR NOT JSON_VALID(CAST(o.original_row AS CHAR CHARACTER SET utf8mb4)) OR c.state='REJECTED'
		OR (c.state='SUCCEEDED' AND (o.applied_index IS NULL OR o.applied_seq IS NULL OR o.authority_digest IS NULL
			OR o.applied_seq<c.aggregate_seq OR o.authority_digest=UNHEX(REPEAT('00',32)) OR a.document_id IS NULL
			OR a.applied_seq<o.applied_seq OR (a.applied_seq=o.applied_seq AND a.authority_digest<>o.authority_digest)))
		OR (c.state<>'SUCCEEDED' AND (o.applied_index IS NOT NULL OR o.applied_seq IS NOT NULL OR o.authority_digest IS NOT NULL)))
		LIMIT 1`, consumer, topic, domain.PartitionCount).Scan(&id)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return errors.New("SEARCH_RECOVERY_LEDGER_READ_FAILED")
	}
	return errors.New("SEARCH_ORIGIN_RECOVERY_REQUIRED")
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/search/internal/repository/event/events_queries.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/search/02-consumer-ledger.md)。
