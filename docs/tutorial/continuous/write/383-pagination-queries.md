# 分步编写 `services/search/internal/repository/cursor/pagination_queries.go`

先按 [S03 · 签名批锚点、PIT与当前权限二次过滤](../lessons/search/03-candidates-cursor.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/383-pagination-queries.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

调度到期链关闭、保守保存进度并删除已过期CLOSED行。

关闭未知不等于资源已释放；迟到查询不能重新打开CLOSING链，也不能更换原scope/session/期限。

写之前先按这条执行路线理解各段的连接：

1. Due按expires/retry_at/deadline选最多16条并SKIP LOCKED，提交后才访问ES
2. Save重读并逐项比不可变绑定，只允许更新PIT/Control/State
3. 状态底线禁止CLOSING→OPEN及CLOSED复活
4. DeleteClosed先跳锁取精确hash，仅删除原期限已过的CLOSED

本文件需要守住的边界：

- Due只是调度观察，领取时必须再Lock核nonce/期限
- 不能DELETE CLOSING来释放配额
- 不能把失败关闭链续到新Expires

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/search/internal/repository/cursor' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/search/internal/repository/cursor/pagination_queries.go') -and (Get-Item -LiteralPath 'services/search/internal/repository/cursor/pagination_queries.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/search/internal/repository/cursor/pagination_queries.go')) { New-Item -ItemType File -Path 'services/search/internal/repository/cursor/pagination_queries.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:239 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package cursor

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"time"
)

// cleanup.go：保留原实现，按业务合并文件。

````

## 第 2 步：validCleanup

插入位置：第 1 步代码之后，紧接追加；完成稿约第 14 行。

本步声明：`func validCleanup(tx *sql.Tx, now time.Time, limit int) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx != nil</code> |
| <code>now</code> | <code>time.Time</code> | 一个时间点。判断期限时要与当前时间比较，不能把它当成持续时长再任意续期。<br>在此声明中的实际使用：<code>now.IsZero</code>；<code>now.UnixMilli</code> |
| <code>limit</code> | <code>int</code> | 在此声明中的实际使用：<code>limit &gt; 0</code>；<code>limit &lt;= 16</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>tx != nil &amp;&amp; !now.IsZero() &amp;&amp; now.UnixMilli() &gt; 0 &amp;&amp; limit &gt; 0 &amp;&amp; limit &lt;= 16</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return tx != nil &amp;&amp; !now.IsZero() &amp;&amp; now.UnixMilli() &gt; 0 &amp;&amp; limit &gt; 0 &amp;&amp; limit &lt;= 16</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=239:391 -->
````go
func validCleanup(tx *sql.Tx, now time.Time, limit int) bool {
	return tx != nil && !now.IsZero() && now.UnixMilli() > 0 && limit > 0 && limit <= 16
}

````

## 第 3 步：Due

插入位置：第 2 步代码之后，紧接追加；完成稿约第 18 行。

Due在原事务内跳过被锁行并选择至多16个hash，调用方提交后再访问ES。
领取者仍须再Lock复核状态/nonce/期限，不能把调度观察当成关闭授权；
retry_at让前面的失败链暂时让位，而原绝对期限到期必须结束等待，不续期。

本步声明：`func Due(ctx context.Context, tx *sql.Tx, now time.Time, limit int) ([][32]byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryContext(ctx, `SELECT cursor_hash FROM cursor_registry WHERE 		(state=&#x27;CLOSING&#x27; AND (expires_at&lt;=? OR ( 			COALESCE(CAST(JSON_UNQUOTE(JSON_EXTRACT(last_sort,&#x27;$.retry_at&#x27;)) A …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>validCleanup(tx, now, limit)</code>；<code>tx.QueryContext</code> |
| <code>now</code> | <code>time.Time</code> | 一个时间点。判断期限时要与当前时间比较，不能把它当成持续时长再任意续期。<br>在此声明中的实际使用：<code>validCleanup(tx, now, limit)</code>；<code>now = now.UTC().Truncate(time.Millisecond)</code>；<code>now.UTC</code>；<code>tx.QueryContext(ctx, `SELECT cursor_hash FROM cursor_registry WHERE 		(state=&#x27;CLOSING&#x27; AND (expires_at&lt;=? OR ( 			COALESCE(CAST(JSON_UNQUOTE(JSON_EXTRACT(last_sort,&#x27;$.retry_at&#x27;)) A …</code>（长表达式，按本步完整代码继续）<br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>limit</code> | <code>int</code> | 在此声明中的实际使用：<code>validCleanup(tx, now, limit)</code>；<code>tx.QueryContext(ctx, `SELECT cursor_hash FROM cursor_registry WHERE 		(state=&#x27;CLOSING&#x27; AND (expires_at&lt;=? OR ( 			COALESCE(CAST(JSON_UNQUOTE(JSON_EXTRACT(last_sort,&#x27;$.retry_at&#x27;)) A …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;&#91;32&#93;byte</code> | 实际返回表达式：<code>nil</code>；<code>hashes</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_CURSOR_REGISTRY_INVALID&quot;)</code>；<code>errors.New(&quot;SEARCH_CURSOR_REGISTRY_READ_FAILED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>!validCleanup(tx, now, limit)</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_CURSOR_REGISTRY_INVALID&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.QueryContext(ctx, `SELECT cursor_hash FROM cursor_registry WHERE 		(state=&#x27;CLOSING&#x27; AND (expires_at&lt;=? OR ( 			COALESCE(CAST(JSON_UNQUOTE(JSON_EXTRACT(last_sort,&#x27;$.retry_at&#x27;)) A …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_CURSOR_REGISTRY_READ_FAILED&quot;)</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_CURSOR_REGISTRY_READ_FAILED&quot;)</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;encoded)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 检查 <code>len(encoded) != len(hash)</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_CURSOR_REGISTRY_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>hash == &#91;32&#93;byte{}</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_CURSOR_REGISTRY_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>rows.Err() != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;SEARCH_CURSOR_REGISTRY_READ_FAILED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return hashes, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=391:2492 -->
````go
// Due在原事务内跳过被锁行并选择至多16个hash，调用方提交后再访问ES。
// 领取者仍须再Lock复核状态/nonce/期限，不能把调度观察当成关闭授权；
// retry_at让前面的失败链暂时让位，而原绝对期限到期必须结束等待，不续期。
func Due(ctx context.Context, tx *sql.Tx, now time.Time, limit int) ([][32]byte, error) {
	if !validCleanup(tx, now, limit) {
		return nil, errors.New("SEARCH_CURSOR_REGISTRY_INVALID")
	}
	now = now.UTC().Truncate(time.Millisecond)
	// CLOSING也可能仍有原query/close调用在途，健康deadline前不能再次领取。
	// 控制对象的严格读取留给Lock；这里仅做有界调度，不把JSON字段当业务授权。
	rows, err := tx.QueryContext(ctx, `SELECT cursor_hash FROM cursor_registry WHERE
		(state='CLOSING' AND (expires_at<=? OR (
			COALESCE(CAST(JSON_UNQUOTE(JSON_EXTRACT(last_sort,'$.retry_at')) AS SIGNED),0)<=?
			AND NOT (JSON_UNQUOTE(JSON_EXTRACT(last_sort,'$.phase')) IN ('OPENING','INFLIGHT')
				AND CAST(JSON_UNQUOTE(JSON_EXTRACT(last_sort,'$.deadline')) AS SIGNED)>?))))
		OR (state='OPEN' AND (expires_at<=? OR (
			JSON_UNQUOTE(JSON_EXTRACT(last_sort,'$.phase')) IN ('OPENING','INFLIGHT')
			AND CAST(JSON_UNQUOTE(JSON_EXTRACT(last_sort,'$.deadline')) AS SIGNED)<=?)))
		ORDER BY expires_at,cursor_hash LIMIT ? FOR UPDATE SKIP LOCKED`, now, now.UnixMilli(), now.UnixMilli(), now, now.UnixMilli(), limit)
	if err != nil {
		return nil, errors.New("SEARCH_CURSOR_REGISTRY_READ_FAILED")
	}
	defer rows.Close()
	var hashes [][32]byte
	for rows.Next() {
		var encoded []byte
		if err := rows.Scan(&encoded); err != nil {
			return nil, errors.New("SEARCH_CURSOR_REGISTRY_READ_FAILED")
		}
		var hash [32]byte
		if len(encoded) != len(hash) {
			return nil, errors.New("SEARCH_CURSOR_REGISTRY_INVALID")
		}
		copy(hash[:], encoded)
		if hash == [32]byte{} {
			return nil, errors.New("SEARCH_CURSOR_REGISTRY_INVALID")
		}
		hashes = append(hashes, hash)
	}
	if rows.Err() != nil {
		return nil, errors.New("SEARCH_CURSOR_REGISTRY_READ_FAILED")
	}
	return hashes, nil
}

````

## 第 4 步：DeleteClosed

插入位置：第 3 步代码之后，紧接追加；完成稿约第 63 行。

DeleteClosed只在原期限已过后有界删除已释放行；仍未知的CLOSING不得靠DELETE释放配额。

本步声明：`func DeleteClosed(ctx context.Context, tx *sql.Tx, now time.Time, limit int) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryContext(ctx, `SELECT cursor_hash FROM cursor_registry WHERE state=&#x27;CLOSED&#x27; AND expires_at&lt;=? 		ORDER BY expires_at,cursor_hash LIMIT ? FOR UPDATE SKIP LOCKED`, now.UTC().Truncate(time.Millisecond), limit)</code>；<code>tx.ExecContext(ctx, &quot;DELETE FROM cursor_registry WHERE cursor_hash=? AND state=&#x27;CLOSED&#x27; AND expires_at&lt;=?&quot;, hash, now.UTC().Truncate(time.Millisecond))</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>validCleanup(tx, now, limit)</code>；<code>tx.QueryContext</code>；<code>tx.ExecContext</code> |
| <code>now</code> | <code>time.Time</code> | 一个时间点。判断期限时要与当前时间比较，不能把它当成持续时长再任意续期。<br>在此声明中的实际使用：<code>validCleanup(tx, now, limit)</code>；<code>now.UTC</code> |
| <code>limit</code> | <code>int</code> | 在此声明中的实际使用：<code>validCleanup(tx, now, limit)</code>；<code>tx.QueryContext(ctx, `SELECT cursor_hash FROM cursor_registry WHERE state=&#x27;CLOSED&#x27; AND expires_at&lt;=? 		ORDER BY expires_at,cursor_hash LIMIT ? FOR UPDATE SKIP LOCKED`, now.UTC().Truncate(time.Millisecond), limit)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;SEARCH_CURSOR_REGISTRY_INVALID&quot;)</code>；<code>errors.New(&quot;SEARCH_CURSOR_REGISTRY_READ_FAILED&quot;)</code>；<code>errors.New(&quot;SEARCH_CURSOR_REGISTRY_WRITE_FAILED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>!validCleanup(tx, now, limit)</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_CURSOR_REGISTRY_INVALID&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.QueryContext(ctx, `SELECT cursor_hash FROM cursor_registry WHERE state=&#x27;CLOSED&#x27; AND expires_at&lt;=? 		ORDER BY expires_at,cursor_hash LIMIT ? FOR UPDATE SKIP LOCKED`, now.UTC().Truncate(time.Millisecond), limit)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_CURSOR_REGISTRY_READ_FAILED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; len(hash) != 32</code>。
- 执行 <code>rows.Scan(&amp;hash)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return errors.New(&quot;SEARCH_CURSOR_REGISTRY_INVALID&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>readErr != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_CURSOR_REGISTRY_READ_FAILED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_CURSOR_REGISTRY_WRITE_FAILED&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;DELETE FROM cursor_registry WHERE cursor_hash=? AND state=&#x27;CLOSED&#x27; AND expires_at&lt;=?&quot;, hash, now.UTC().Truncate(time.Millisecond))</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=2492:4042 -->
````go
// DeleteClosed只在原期限已过后有界删除已释放行；仍未知的CLOSING不得靠DELETE释放配额。
func DeleteClosed(ctx context.Context, tx *sql.Tx, now time.Time, limit int) error {
	if !validCleanup(tx, now, limit) {
		return errors.New("SEARCH_CURSOR_REGISTRY_INVALID")
	}
	// DELETE的排序扫描会等待更早的被锁行，包括尚未释放的OPEN；先跳锁选择，再按精确hash删除。
	// 两步使用同一事务，所选CLOSED行的锁保持到caller提交/回滚，避免资格改变或隐式提交。
	rows, err := tx.QueryContext(ctx, `SELECT cursor_hash FROM cursor_registry WHERE state='CLOSED' AND expires_at<=?
		ORDER BY expires_at,cursor_hash LIMIT ? FOR UPDATE SKIP LOCKED`, now.UTC().Truncate(time.Millisecond), limit)
	if err != nil {
		return errors.New("SEARCH_CURSOR_REGISTRY_READ_FAILED")
	}
	var hashes [][]byte
	for rows.Next() {
		var hash []byte
		if err := rows.Scan(&hash); err != nil || len(hash) != 32 {
			rows.Close()
			return errors.New("SEARCH_CURSOR_REGISTRY_INVALID")
		}
		hashes = append(hashes, hash)
	}
	readErr := rows.Err()
	rows.Close()
	if readErr != nil {
		return errors.New("SEARCH_CURSOR_REGISTRY_READ_FAILED")
	}
	for _, hash := range hashes {
		if _, err := tx.ExecContext(ctx, "DELETE FROM cursor_registry WHERE cursor_hash=? AND state='CLOSED' AND expires_at<=?", hash, now.UTC().Truncate(time.Millisecond)); err != nil {
			return errors.New("SEARCH_CURSOR_REGISTRY_WRITE_FAILED")
		}
	}
	return nil
}

// progress.go：保留原实现，按业务合并文件。

````

## 第 5 步：Save

插入位置：第 4 步代码之后，紧接追加；完成稿约第 99 行。

Save只能推进同一链的最新确认PIT/控制对象/释放状态。再次读取原行，防止调用方
把错误query/session/期限的完整Record悄悄当成合法进度写入；任何错误仍留给原事务回滚。

本步声明：`func Save(ctx context.Context, tx *sql.Tx, record Record) error`。

**这一段为什么这样写：** 在SQL边界防止进度更新顺便篡改链身份。

1. 校验Record
2. 锁当前原行
3. 比较physical/generation/query/scope/session/page/expiry
4. 核关闭不可逆
5. 更新句柄与控制状态


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | SQL调用期限<br>在此声明中的实际使用：<code>Lock(ctx, tx, record.Hash)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE cursor_registry SET pit_handle=?,last_sort=?,state=? WHERE cursor_hash=?&quot;, 		record.PIT, control, record.State, record.Hash&#91;:&#93;)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 已开始的事务<br>在此声明中的实际使用：<code>Lock(ctx, tx, record.Hash)</code>；<code>tx.ExecContext</code> |
| <code>record</code> | <code>Record</code> | 保留原绑定、仅修改进度的目标记录<br>在此声明中的实际使用：<code>validateRecord(record)</code>；<code>record.Hash</code>；<code>record.Physical</code>；<code>record.Generation</code><br>另有 7 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | nil尚待外层COMMIT<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;SEARCH_CURSOR_REGISTRY_MISSING&quot;)</code>；<code>errors.New(&quot;SEARCH_CURSOR_BINDING_CHANGED&quot;)</code>；<code>errors.New(&quot;SEARCH_CURSOR_STATE_CHANGED&quot;)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!found</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_CURSOR_REGISTRY_MISSING&quot;)</code>。当前函数在这里结束。
- 检查 <code>current.Physical != record.Physical &#124;&#124; current.Generation != record.Generation &#124;&#124; 		current.ScopeHash != record.ScopeHash &#124;&#124; current.QueryHash != record.QueryHash &#124;&#124; 		!bytes.Equal …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return errors.New(&quot;SEARCH_CURSOR_BINDING_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>(current.State == Closing &amp;&amp; record.State == Open) &#124;&#124; (current.State == Closed &amp;&amp; record.State != Closed)</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_CURSOR_STATE_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;SEARCH_CURSOR_REGISTRY_WRITE_FAILED&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE cursor_registry SET pit_handle=?,last_sort=?,state=? WHERE cursor_hash=?&quot;, 		record.PIT, control, record.State, record.Hash&#91;:&#93;)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=4042:5541 -->
````go
// Save只能推进同一链的最新确认PIT/控制对象/释放状态。再次读取原行，防止调用方
// 把错误query/session/期限的完整Record悄悄当成合法进度写入；任何错误仍留给原事务回滚。
func Save(ctx context.Context, tx *sql.Tx, record Record) error {
	control, err := validateRecord(record)
	if err != nil {
		return err
	}
	current, found, err := Lock(ctx, tx, record.Hash)
	if err != nil {
		return err
	}
	if !found {
		return errors.New("SEARCH_CURSOR_REGISTRY_MISSING")
	}
	if current.Physical != record.Physical || current.Generation != record.Generation ||
		current.ScopeHash != record.ScopeHash || current.QueryHash != record.QueryHash ||
		!bytes.Equal(current.Session, record.Session) || current.PageSize != record.PageSize || !current.Expires.Equal(record.Expires) {
		return errors.New("SEARCH_CURSOR_BINDING_CHANGED")
	}
	// 显式Close已登记未知结果后，迟到的查询确认不得重新开放；nonce是否匹配由用例
	// 在同一行锁下复核，本层仍保留不可逆的关闭状态底线。
	if (current.State == Closing && record.State == Open) || (current.State == Closed && record.State != Closed) {
		return errors.New("SEARCH_CURSOR_STATE_CHANGED")
	}
	if _, err := tx.ExecContext(ctx, "UPDATE cursor_registry SET pit_handle=?,last_sort=?,state=? WHERE cursor_hash=?",
		record.PIT, control, record.State, record.Hash[:]); err != nil {
		return errors.New("SEARCH_CURSOR_REGISTRY_WRITE_FAILED")
	}
	return nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/search/internal/repository/cursor/pagination_queries.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/search/03-candidates-cursor.md)。
