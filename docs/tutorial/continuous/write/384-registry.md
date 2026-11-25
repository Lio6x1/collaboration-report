# 分步编写 `services/search/internal/repository/cursor/registry.go`

先按 [S03 · 签名批锚点、PIT与当前权限二次过滤](../lessons/search/03-candidates-cursor.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/384-registry.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

持久登记单条PIT链的身份、绝对期限、阶段和配额。

进程退出不能丢掉未知PIT资源事实；OPENING和CLOSING仍占配额，不能通过失败建链无限分配ES资源。

写之前先按这条执行路线理解各段的连接：

1. validateRecord核chain/query/scope哈希、session、physical/generation、毫秒期限和Control
2. Create要求调用方已持排他generation锁，锁定读计OPEN+CLOSING<=128、总行<=1024
3. Lock明确读取存在性，TryLock供worker SKIP LOCKED但false不证明不存在
4. Check读取最多1025行验证所有持久事实可恢复

本文件需要守住的边界：

- 未知OPEN或轮换句柄可为空但只允许特定状态
- PIT最长64KiB且无空白，不能截断后继续
- 配额必须当前锁定读，不能沿旧一致快照漏掉其他代次

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/search/internal/repository/cursor' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/search/internal/repository/cursor/registry.go') -and (Get-Item -LiteralPath 'services/search/internal/repository/cursor/registry.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/search/internal/repository/cursor/registry.go')) { New-Item -ItemType File -Path 'services/search/internal/repository/cursor/registry.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:164 -->
````go
package cursor

import (
	domain "collaboration/search/internal/domain/cursor"
	"context"
	"database/sql"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

````

## 第 2 步：Open、Closing、Closed、maxOpenChains、maxRows、rowColumns

插入位置：第 1 步代码之后，紧接追加；完成稿约第 14 行。

本步声明：`const (`。

<!-- write-step: 2 bytes=164:423 -->
````go
const (
	Open    = "OPEN"
	Closing = "CLOSING"
	Closed  = "CLOSED"

	maxOpenChains = 128
	maxRows       = 1024
	rowColumns    = "cursor_hash,pit_handle,physical_index,index_generation,scope_hash,query_hash,session_ref,page_size,expires_at,state,last_sort"
)

````

## 第 3 步：Record

插入位置：第 2 步代码之后，紧接追加；完成稿约第 24 行。

Record是单链的持久事实；绑定字段不随NEXT/REPLAY或重试更新，Expires也是原绝对期限。

本步声明：`type Record struct {`。

**这一段为什么这样写：** 单链不可变身份与可推进资源状态。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Hash</code> | <code>&#91;32&#93;byte</code> | 随机Chain的SHA-256，SQL主键 |
| <code>PIT</code> | <code>string</code> | 最新已观测ES PIT句柄，不进入用户token |
| <code>Physical</code> | <code>string</code> | 建链时物理索引 |
| <code>Generation</code> | <code>uint64</code> | 建链时SQL索引代次 |
| <code>ScopeHash</code> | <code>&#91;32&#93;byte</code> | Doc授权范围绑定摘要 |
| <code>QueryHash</code> | <code>&#91;32&#93;byte</code> | 原查询文本摘要 |
| <code>Session</code> | <code>&#91;&#93;byte</code> | 原登录session引用；匿名为空 |
| <code>PageSize</code> | <code>uint32</code> | 固定候选批大小100 |
| <code>Expires</code> | <code>time.Time</code> | 创建时绝对到期时间，重试不续期 |
| <code>State</code> | <code>string</code> | OPEN/CLOSING/CLOSED，只能向关闭方向收紧 |
| <code>Control</code> | <code>domain.Control</code> | 最新尾部、在途nonce/deadline和未知标记 |

<!-- write-step: 3 bytes=423:788 -->
````go
// Record是单链的持久事实；绑定字段不随NEXT/REPLAY或重试更新，Expires也是原绝对期限。
type Record struct {
	Hash       [32]byte
	PIT        string
	Physical   string
	Generation uint64
	ScopeHash  [32]byte
	QueryHash  [32]byte
	Session    []byte
	PageSize   uint32
	Expires    time.Time
	State      string
	Control    domain.Control
}

````

## 第 4 步：physicalName

插入位置：第 3 步代码之后，紧接追加；完成稿约第 39 行。

本步声明：`var physicalName = regexp.MustCompile(\`^[a-z0-9][a-z0-9._-]{0,159}$\`)`。

<!-- write-step: 4 bytes=788:859 -->
````go
var physicalName = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,159}$`)

````

## 第 5 步：validateRecord

插入位置：第 4 步代码之后，紧接追加；完成稿约第 41 行。

本步声明：`func validateRecord(record Record) ([]byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>record</code> | <code>Record</code> | 使用 <code>Record</code> 类型；其定义处的职责：单链不可变身份与可推进资源状态。<br>在此声明中的实际使用：<code>record.Hash</code>；<code>record.ScopeHash</code>；<code>record.QueryHash</code>；<code>record.Physical</code><br>另有 7 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>nil</code>；<code>control</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>invalid</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>record.Hash == zero &#124;&#124; record.ScopeHash == zero &#124;&#124; record.QueryHash == zero &#124;&#124; 		!physicalName.MatchString(record.Physical) &#124;&#124; record.Generation == 0 &#124;&#124; 		record.PageSize == 0 &#124;&#124; r …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return nil, invalid</code>。当前函数在这里结束。
- 检查 <code>len(record.Session) != 0</code>。
- 检查 <code>len(record.Session) != 32</code>。条件成立时执行 <code>return nil, invalid</code>。当前函数在这里结束。
- 检查 <code>session == zero</code>。条件成立时执行 <code>return nil, invalid</code>。当前函数在这里结束。
- 检查 <code>record.PIT == &quot;&quot;</code>。
- 检查 <code>record.State != Closed &amp;&amp; !(record.State == Open &amp;&amp; record.Control.Phase == domain.PhaseOpening) &amp;&amp; 			!(record.State == Closing &amp;&amp; record.Control.Uncertain)</code>。条件成立时执行 <code>return nil, invalid</code>。当前函数在这里结束。
- 检查 <code>len(record.PIT) &gt; 64*1024 &#124;&#124; strings.IndexFunc(record.PIT, func(r rune) bool { return r &lt;= &#x27; &#x27; &#124;&#124; r == 127 }) &gt;= 0</code>。条件成立时执行 <code>return nil, invalid</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return r &lt;= &#x27; &#x27; &#124;&#124; r == 127</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil &#124;&#124; len(control) &gt; domain.MaxControlBytes &#124;&#124; 		record.Control.Deadline &gt; record.Expires.UnixMilli() &#124;&#124; record.Control.RetryAt &gt; record.Expires.UnixMilli()</code>。条件成立时执行 <code>return nil, invalid</code>。当前函数在这里结束。
- 此处通过 <code>return control, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=859:2443 -->
````go
func validateRecord(record Record) ([]byte, error) {
	invalid := errors.New("SEARCH_CURSOR_REGISTRY_INVALID")
	zero := [32]byte{}
	if record.Hash == zero || record.ScopeHash == zero || record.QueryHash == zero ||
		!physicalName.MatchString(record.Physical) || record.Generation == 0 ||
		record.PageSize == 0 || record.PageSize > 100 || record.Expires.IsZero() ||
		record.Expires.UnixMilli() <= 0 || record.Expires.Year() > 9999 ||
		record.Expires.Nanosecond()%int(time.Millisecond) != 0 || !utf8.ValidString(record.PIT) {
		return nil, invalid
	}
	if len(record.Session) != 0 {
		if len(record.Session) != 32 {
			return nil, invalid
		}
		var session [32]byte
		copy(session[:], record.Session)
		if session == zero {
			return nil, invalid
		}
	}
	switch record.State {
	case Open, Closing, Closed:
	default:
		return nil, invalid
	}
	if record.PIT == "" {
		// 未知初次open或轮换句柄仍占配额，关闭领取时可转INFLIGHT但不能捏造PIT。
		if record.State != Closed && !(record.State == Open && record.Control.Phase == domain.PhaseOpening) &&
			!(record.State == Closing && record.Control.Uncertain) {
			return nil, invalid
		}
	} else if len(record.PIT) > 64*1024 || strings.IndexFunc(record.PIT, func(r rune) bool { return r <= ' ' || r == 127 }) >= 0 {
		return nil, invalid
	}
	control, err := domain.EncodeControl(record.Control)
	if err != nil || len(control) > domain.MaxControlBytes ||
		record.Control.Deadline > record.Expires.UnixMilli() || record.Control.RetryAt > record.Expires.UnixMilli() {
		return nil, invalid
	}
	return control, nil
}

````

## 第 6 步：scanner

插入位置：第 5 步代码之后，紧接追加；完成稿约第 83 行。

本步声明：`type scanner interface {`。

**这一段为什么这样写：** 兼容sql.Row/sql.Rows的行读取接口。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Scan</code> | <code>(...any) error</code> | 按rowColumns固定列顺序填充指针，返回驱动读取错误 |

<!-- write-step: 6 bytes=2443:2491 -->
````go
type scanner interface {
	Scan(...any) error
}

````

## 第 7 步：scanRecord

插入位置：第 6 步代码之后，紧接追加；完成稿约第 87 行。

本步声明：`func scanRecord(source scanner) (Record, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>source</code> | <code>scanner</code> | 在此声明中的实际使用：<code>source.Scan</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Record</code> | 使用 <code>Record</code> 类型；其定义处的职责：单链不可变身份与可推进资源状态。<br>实际返回表达式：<code>Record{}</code>；<code>record</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;SEARCH_CURSOR_REGISTRY_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>source.Scan(&amp;hash, &amp;record.PIT, &amp;record.Physical, &amp;record.Generation, &amp;scope, &amp;query, 		&amp;session, &amp;record.PageSize, &amp;record.Expires, &amp;record.State, &amp;control)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return Record{}, err</code>。当前函数在这里结束。
- 检查 <code>len(hash) != 32 &#124;&#124; len(scope) != 32 &#124;&#124; len(query) != 32 &#124;&#124; (session != nil &amp;&amp; len(session) != 32)</code>。条件成立时执行 <code>return Record{}, errors.New(&quot;SEARCH_CURSOR_REGISTRY_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return Record{}, errors.New(&quot;SEARCH_CURSOR_REGISTRY_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return Record{}, err</code>。当前函数在这里结束。
- 此处通过 <code>return record, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=2491:3411 -->
````go
func scanRecord(source scanner) (Record, error) {
	var record Record
	var hash, scope, query, session, control []byte
	err := source.Scan(&hash, &record.PIT, &record.Physical, &record.Generation, &scope, &query,
		&session, &record.PageSize, &record.Expires, &record.State, &control)
	if err != nil {
		return Record{}, err
	}
	if len(hash) != 32 || len(scope) != 32 || len(query) != 32 || (session != nil && len(session) != 32) {
		return Record{}, errors.New("SEARCH_CURSOR_REGISTRY_INVALID")
	}
	copy(record.Hash[:], hash)
	copy(record.ScopeHash[:], scope)
	copy(record.QueryHash[:], query)
	record.Session = append([]byte(nil), session...)
	record.Expires = record.Expires.UTC()
	record.Control, err = domain.DecodeControl(control)
	if err != nil {
		return Record{}, errors.New("SEARCH_CURSOR_REGISTRY_INVALID")
	}
	if _, err := validateRecord(record); err != nil {
		return Record{}, err
	}
	return record, nil
}

````

## 第 8 步：Create

插入位置：第 7 步代码之后，紧接追加；完成稿约第 113 行。

Create要求调用方已持有当前代次排他锁。配额使用当前锁定读，不能沿旧一致快照
漏掉其它代次已登记的链；OPENING仍是SQL OPEN，CLOSING也尚未释放配额。

本步声明：`func Create(ctx context.Context, tx *sql.Tx, record *Record, openLimit, rowLimit int) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT COALESCE(SUM(state IN (&#x27;OPEN&#x27;,&#x27;CLOSING&#x27;)),0),COUNT(*) FROM cursor_registry FOR UPDATE&quot;)</code>；<code>tx.ExecContext(ctx, `INSERT INTO cursor_registry(cursor_hash,pit_handle,physical_index,index_generation, 		scope_hash,query_hash,session_ref,last_sort,page_size,expires_at,state) V …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx == nil</code>；<code>tx.QueryRowContext</code>；<code>tx.ExecContext</code> |
| <code>record</code> | <code>*Record</code> | 使用 <code>Record</code> 类型；其定义处的职责：单链不可变身份与可推进资源状态。<br>在此声明中的实际使用：<code>record == nil</code>；<code>validateRecord(*record)</code>；<code>record.Session</code>；<code>record.Hash</code><br>另有 8 处使用，继续按下方完整代码的顺序处理。 |
| <code>openLimit</code> | <code>int</code> | 在此声明中的实际使用：<code>openLimit &lt;= 0</code>；<code>openLimit &gt; maxOpenChains</code>；<code>open &gt;= openLimit</code> |
| <code>rowLimit</code> | <code>int</code> | 在此声明中的实际使用：<code>rowLimit &lt;= 0</code>；<code>rowLimit &gt; maxRows</code>；<code>total &gt;= rowLimit</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_CURSOR_REGISTRY_INVALID&quot;)</code>；<code>err</code>；<code>errors.New(&quot;SEARCH_CURSOR_REGISTRY_READ_FAILED&quot;)</code>；<code>errors.New(&quot;SEARCH_CURSOR_QUOTA_EXCEEDED&quot;)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>tx == nil &#124;&#124; record == nil &#124;&#124; openLimit &lt;= 0 &#124;&#124; openLimit &gt; maxOpenChains &#124;&#124; rowLimit &lt;= 0 &#124;&#124; rowLimit &gt; maxRows</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_CURSOR_REGISTRY_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_CURSOR_REGISTRY_READ_FAILED&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT COALESCE(SUM(state IN (&#x27;OPEN&#x27;,&#x27;CLOSING&#x27;)),0),COUNT(*) FROM cursor_registry FOR UPDATE&quot;).Scan(&amp;open, &amp;total)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT COALESCE(SUM(state IN (&#x27;OPEN&#x27;,&#x27;CLOSING&#x27;)),0),COUNT(*) FROM cursor_registry FOR UPDATE&quot;)</code>。
- 检查 <code>open &gt;= openLimit &#124;&#124; total &gt;= rowLimit</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_CURSOR_QUOTA_EXCEEDED&quot;)</code>。当前函数在这里结束。
- 检查 <code>len(record.Session) != 0</code>。
- 执行 <code>tx.ExecContext(ctx, `INSERT INTO cursor_registry(cursor_hash,pit_handle,physical_index,index_generation, 		scope_hash,query_hash,session_ref,last_sort,page_size,expires_at,state) V …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_CURSOR_REGISTRY_WRITE_FAILED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=3411:4922 -->
````go
// Create要求调用方已持有当前代次排他锁。配额使用当前锁定读，不能沿旧一致快照
// 漏掉其它代次已登记的链；OPENING仍是SQL OPEN，CLOSING也尚未释放配额。
func Create(ctx context.Context, tx *sql.Tx, record *Record, openLimit, rowLimit int) error {
	if tx == nil || record == nil || openLimit <= 0 || openLimit > maxOpenChains || rowLimit <= 0 || rowLimit > maxRows {
		return errors.New("SEARCH_CURSOR_REGISTRY_INVALID")
	}
	control, err := validateRecord(*record)
	if err != nil {
		return err
	}
	var open, total int
	if err := tx.QueryRowContext(ctx, "SELECT COALESCE(SUM(state IN ('OPEN','CLOSING')),0),COUNT(*) FROM cursor_registry FOR UPDATE").Scan(&open, &total); err != nil {
		return errors.New("SEARCH_CURSOR_REGISTRY_READ_FAILED")
	}
	if open >= openLimit || total >= rowLimit {
		return errors.New("SEARCH_CURSOR_QUOTA_EXCEEDED")
	}
	var session any
	if len(record.Session) != 0 {
		session = record.Session
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO cursor_registry(cursor_hash,pit_handle,physical_index,index_generation,
		scope_hash,query_hash,session_ref,last_sort,page_size,expires_at,state) VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
		record.Hash[:], record.PIT, record.Physical, record.Generation, record.ScopeHash[:], record.QueryHash[:],
		session, control, record.PageSize, record.Expires.UTC(), record.State)
	if err != nil {
		return errors.New("SEARCH_CURSOR_REGISTRY_WRITE_FAILED")
	}
	record.Expires = record.Expires.UTC()
	return nil
}

````

## 第 9 步：Lock

插入位置：第 8 步代码之后，紧接追加；完成稿约第 145 行。

Lock只锁登记行，不跨ES调用持有事务；用例在锁内核对nonce和当前状态后再保存确认。

本步声明：`func Lock(ctx context.Context, tx *sql.Tx, hash [32]byte) (Record, bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT &quot;+rowColumns+&quot; FROM cursor_registry WHERE cursor_hash=? FOR UPDATE&quot;, hash&#91;:&#93;)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx == nil</code>；<code>tx.QueryRowContext</code> |
| <code>hash</code> | <code>&#91;32&#93;byte</code> | 在此声明中的实际使用：<code>hash == &#91;32&#93;byte{}</code>；<code>tx.QueryRowContext(ctx, &quot;SELECT &quot;+rowColumns+&quot; FROM cursor_registry WHERE cursor_hash=? FOR UPDATE&quot;, hash&#91;:&#93;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Record</code> | 使用 <code>Record</code> 类型；其定义处的职责：单链不可变身份与可推进资源状态。<br>实际返回表达式：<code>Record{}</code>；<code>record</code> |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_CURSOR_REGISTRY_INVALID&quot;)</code>；<code>nil</code>；<code>err</code>；<code>errors.New(&quot;SEARCH_CURSOR_REGISTRY_READ_FAILED&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>tx == nil &#124;&#124; hash == &#91;32&#93;byte{}</code>。条件成立时执行 <code>return Record{}, false, errors.New(&quot;SEARCH_CURSOR_REGISTRY_INVALID&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT &quot;+rowColumns+&quot; FROM cursor_registry WHERE cursor_hash=? FOR UPDATE&quot;, hash&#91;:&#93;)</code>。
- 检查 <code>err == sql.ErrNoRows</code>。条件成立时执行 <code>return Record{}, false, nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 检查 <code>err.Error() == &quot;SEARCH_CURSOR_REGISTRY_INVALID&quot;</code>。条件成立时执行 <code>return Record{}, false, err</code>。当前函数在这里结束。
- 此处通过 <code>return Record{}, false, errors.New(&quot;SEARCH_CURSOR_REGISTRY_READ_FAILED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return record, true, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=4922:5645 -->
````go
// Lock只锁登记行，不跨ES调用持有事务；用例在锁内核对nonce和当前状态后再保存确认。
func Lock(ctx context.Context, tx *sql.Tx, hash [32]byte) (Record, bool, error) {
	if tx == nil || hash == [32]byte{} {
		return Record{}, false, errors.New("SEARCH_CURSOR_REGISTRY_INVALID")
	}
	record, err := scanRecord(tx.QueryRowContext(ctx, "SELECT "+rowColumns+" FROM cursor_registry WHERE cursor_hash=? FOR UPDATE", hash[:]))
	if err == sql.ErrNoRows {
		return Record{}, false, nil
	}
	if err != nil {
		if err.Error() == "SEARCH_CURSOR_REGISTRY_INVALID" {
			return Record{}, false, err
		}
		return Record{}, false, errors.New("SEARCH_CURSOR_REGISTRY_READ_FAILED")
	}
	return record, true, nil
}

````

## 第 10 步：TryLock

插入位置：第 9 步代码之后，紧接追加；完成稿约第 163 行。

TryLock供关闭worker跳过正在使用的链，避免首个被锁候选耗尽整批恢复预算。
false仅表示此次未领取（可能不存在，也可能被其它事务锁住），不能用来判定链
不存在或准许关闭完成；普通业务核验仍使用Lock及其明确的存在语义。

本步声明：`func TryLock(ctx context.Context, tx *sql.Tx, hash [32]byte) (Record, bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT &quot;+rowColumns+&quot; FROM cursor_registry WHERE cursor_hash=? FOR UPDATE SKIP LOCKED&quot;, hash&#91;:&#93;)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx == nil</code>；<code>tx.QueryRowContext</code> |
| <code>hash</code> | <code>&#91;32&#93;byte</code> | 在此声明中的实际使用：<code>hash == &#91;32&#93;byte{}</code>；<code>tx.QueryRowContext(ctx, &quot;SELECT &quot;+rowColumns+&quot; FROM cursor_registry WHERE cursor_hash=? FOR UPDATE SKIP LOCKED&quot;, hash&#91;:&#93;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Record</code> | 使用 <code>Record</code> 类型；其定义处的职责：单链不可变身份与可推进资源状态。<br>实际返回表达式：<code>Record{}</code>；<code>record</code> |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_CURSOR_REGISTRY_INVALID&quot;)</code>；<code>nil</code>；<code>err</code>；<code>errors.New(&quot;SEARCH_CURSOR_REGISTRY_READ_FAILED&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>tx == nil &#124;&#124; hash == &#91;32&#93;byte{}</code>。条件成立时执行 <code>return Record{}, false, errors.New(&quot;SEARCH_CURSOR_REGISTRY_INVALID&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT &quot;+rowColumns+&quot; FROM cursor_registry WHERE cursor_hash=? FOR UPDATE SKIP LOCKED&quot;, hash&#91;:&#93;)</code>。
- 检查 <code>err == sql.ErrNoRows</code>。条件成立时执行 <code>return Record{}, false, nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 检查 <code>err.Error() == &quot;SEARCH_CURSOR_REGISTRY_INVALID&quot;</code>。条件成立时执行 <code>return Record{}, false, err</code>。当前函数在这里结束。
- 此处通过 <code>return Record{}, false, errors.New(&quot;SEARCH_CURSOR_REGISTRY_READ_FAILED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return record, true, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=5645:6579 -->
````go
// TryLock供关闭worker跳过正在使用的链，避免首个被锁候选耗尽整批恢复预算。
// false仅表示此次未领取（可能不存在，也可能被其它事务锁住），不能用来判定链
// 不存在或准许关闭完成；普通业务核验仍使用Lock及其明确的存在语义。
func TryLock(ctx context.Context, tx *sql.Tx, hash [32]byte) (Record, bool, error) {
	if tx == nil || hash == [32]byte{} {
		return Record{}, false, errors.New("SEARCH_CURSOR_REGISTRY_INVALID")
	}
	record, err := scanRecord(tx.QueryRowContext(ctx, "SELECT "+rowColumns+" FROM cursor_registry WHERE cursor_hash=? FOR UPDATE SKIP LOCKED", hash[:]))
	if err == sql.ErrNoRows {
		return Record{}, false, nil
	}
	if err != nil {
		if err.Error() == "SEARCH_CURSOR_REGISTRY_INVALID" {
			return Record{}, false, err
		}
		return Record{}, false, errors.New("SEARCH_CURSOR_REGISTRY_READ_FAILED")
	}
	return record, true, nil
}

````

## 第 11 步：Check

插入位置：第 10 步代码之后，紧接追加；完成稿约第 183 行。

Check供Ready逐项证明持久事实可恢复，不用进程初始化标志代替数据库关口。
多读一行仅用于发现总量违反1024上限，检查过程不领取链或修改任何事实。

本步声明：`func Check(ctx context.Context, tx *sql.Tx) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryContext(ctx, &quot;SELECT &quot;+rowColumns+&quot; FROM cursor_registry ORDER BY cursor_hash LIMIT 1025&quot;)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx == nil</code>；<code>tx.QueryContext</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_CURSOR_REGISTRY_INVALID&quot;)</code>；<code>errors.New(&quot;SEARCH_CURSOR_REGISTRY_READ_FAILED&quot;)</code>；<code>errors.New(&quot;SEARCH_CURSOR_QUOTA_EXCEEDED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>tx == nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_CURSOR_REGISTRY_INVALID&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.QueryContext(ctx, &quot;SELECT &quot;+rowColumns+&quot; FROM cursor_registry ORDER BY cursor_hash LIMIT 1025&quot;)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_CURSOR_REGISTRY_READ_FAILED&quot;)</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>count &gt; maxRows</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_CURSOR_QUOTA_EXCEEDED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_CURSOR_REGISTRY_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>record.State == Open &#124;&#124; record.State == Closing</code>。
- 检查 <code>unreleased &gt; maxOpenChains</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_CURSOR_QUOTA_EXCEEDED&quot;)</code>。当前函数在这里结束。
- 检查 <code>rows.Err() != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_CURSOR_REGISTRY_READ_FAILED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=6579:7643 -->
````go
// Check供Ready逐项证明持久事实可恢复，不用进程初始化标志代替数据库关口。
// 多读一行仅用于发现总量违反1024上限，检查过程不领取链或修改任何事实。
func Check(ctx context.Context, tx *sql.Tx) error {
	if tx == nil {
		return errors.New("SEARCH_CURSOR_REGISTRY_INVALID")
	}
	rows, err := tx.QueryContext(ctx, "SELECT "+rowColumns+" FROM cursor_registry ORDER BY cursor_hash LIMIT 1025")
	if err != nil {
		return errors.New("SEARCH_CURSOR_REGISTRY_READ_FAILED")
	}
	defer rows.Close()
	count := 0
	unreleased := 0
	for rows.Next() {
		count++
		if count > maxRows {
			return errors.New("SEARCH_CURSOR_QUOTA_EXCEEDED")
		}
		record, err := scanRecord(rows)
		if err != nil {
			return errors.New("SEARCH_CURSOR_REGISTRY_INVALID")
		}
		if record.State == Open || record.State == Closing {
			unreleased++
			if unreleased > maxOpenChains {
				return errors.New("SEARCH_CURSOR_QUOTA_EXCEEDED")
			}
		}
	}
	if rows.Err() != nil {
		return errors.New("SEARCH_CURSOR_REGISTRY_READ_FAILED")
	}
	return nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/search/internal/repository/cursor/registry.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/search/03-candidates-cursor.md)。
