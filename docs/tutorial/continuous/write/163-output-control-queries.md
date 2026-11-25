# 分步编写 `services/document/internal/repository/document/output_control_queries.go`

先按 [D11 · 输出登记、撤权与真实排空](../lessons/document/11-output-fence.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/163-output-control-queries.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

保存关闭门闩、准入关闭、两参与者ACK及进程/输出结算事实。

真实输出与业务读取必须在同一事务登记，才能防撤权与输出打开之间漏掉一条流。

写之前先按这条执行路线理解各段的连接：

1. InsertFence固定scope/epoch
2. CloseSourceAdmissions关闭旧准入
3. RecordEdgeAck/RecordNodeAck保存证据
4. InsertOutputLease登记流
5. ConfirmExit仅接受已验证退出
6. Settled只认持久SUCCEEDED。

本文件需要守住的边界：

- 记录缺失、查询失败、租约过期都不是停止证明
- settle必须绑定原output及instance。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/repository/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/repository/document/output_control_queries.go') -and (Get-Item -LiteralPath 'services/document/internal/repository/document/output_control_queries.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/repository/document/output_control_queries.go')) { New-Item -ItemType File -Path 'services/document/internal/repository/document/output_control_queries.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:322 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	"bytes"
	domain "collaboration/document/internal/domain/document"
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
)

// fence.go：保留原实现，按业务合并文件。

````

## 第 2 步：Fences

插入位置：第 1 步代码之后，紧接追加；完成稿约第 16 行。

本步声明：`type Fences struct{ db *sql.DB }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |

<!-- write-step: 2 bytes=322:356 -->
````go
type Fences struct{ db *sql.DB }

````

## 第 3 步：NewFences

插入位置：第 2 步代码之后，紧接追加；完成稿约第 18 行。

本步声明：`func NewFences(db *sql.DB) *Fences`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Fences</code> | 实际返回表达式：<code>&amp;Fences{db: db}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Fences{db: db}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=356:418 -->
````go
func NewFences(db *sql.DB) *Fences { return &Fences{db: db} }
````

## 第 4 步：BlockFenceScope

插入位置：第 3 步代码之后，紧接追加；完成稿约第 19 行。

本步声明：`func BlockFenceScope(ctx context.Context, tx *sql.Tx, key string, epoch uint64, id string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO access_gate(scope_key,max_epoch,state,last_revocation_id) VALUES(?,?,&#x27;BLOCKED&#x27;,?) ON DUPLICATE KEY UPDATE max_epoch=GREATEST(max_epoch,VALUES(max_e …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>key</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO access_gate(scope_key,max_epoch,state,last_revocation_id) VALUES(?,?,&#x27;BLOCKED&#x27;,?) ON DUPLICATE KEY UPDATE max_epoch=GREATEST(max_epoch,VALUES(max_e …</code>（长表达式，按本步完整代码继续） |
| <code>epoch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO access_gate(scope_key,max_epoch,state,last_revocation_id) VALUES(?,?,&#x27;BLOCKED&#x27;,?) ON DUPLICATE KEY UPDATE max_epoch=GREATEST(max_epoch,VALUES(max_e …</code>（长表达式，按本步完整代码继续） |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO access_gate(scope_key,max_epoch,state,last_revocation_id) VALUES(?,?,&#x27;BLOCKED&#x27;,?) ON DUPLICATE KEY UPDATE max_epoch=GREATEST(max_epoch,VALUES(max_e …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO access_gate(scope_key,max_epoch,state,last_revocation_id) VALUES(?,?,&#x27;BLOCKED&#x27;,?) ON DUPLICATE KEY UPDATE max_epoch=GREATEST(max_epoch,VALUES(max_e …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=418:920 -->
````go
func BlockFenceScope(ctx context.Context, tx *sql.Tx, key string, epoch uint64, id string) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO access_gate(scope_key,max_epoch,state,last_revocation_id) VALUES(?,?,'BLOCKED',?) ON DUPLICATE KEY UPDATE max_epoch=GREATEST(max_epoch,VALUES(max_epoch)),state=IF(VALUES(max_epoch)>=max_epoch,'BLOCKED',state),last_revocation_id=IF(VALUES(max_epoch)>=max_epoch,VALUES(last_revocation_id),last_revocation_id),revision=revision+1", key, epoch, id)
	return err
}
````

## 第 5 步：LockedFence

插入位置：第 4 步代码之后，紧接追加；完成稿约第 23 行。

本步声明：`func LockedFence(ctx context.Context, tx *sql.Tx, id string) (domain.FenceState, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT scope_key,epoch,state,target_permission FROM authorization_change WHERE revocation_id=? FOR UPDATE&quot;, id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT scope_key,epoch,state,target_permission FROM authorization_change WHERE revocation_id=? FOR UPDATE&quot;, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.FenceState</code> | 使用 <code>domain.FenceState</code> 类型；其定义处的职责：FenceState保存本服务已经持久登记的关闭绑定，固定来源和代次不能被同号新请求替换。<br>实际返回表达式：<code>current</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT scope_key,epoch,state,target_permission FROM authorization_change WHERE revocation_id=? FOR UPDATE&quot;, id).Scan(&amp;current.Scope, &amp;current.Epoch, &amp;current.State, &amp;current.TargetPermission)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT scope_key,epoch,state,target_permission FROM authorization_change WHERE revocation_id=? FOR UPDATE&quot;, id)</code>。
- 此处通过 <code>return current, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=920:1289 -->
````go
func LockedFence(ctx context.Context, tx *sql.Tx, id string) (domain.FenceState, error) {
	var current domain.FenceState
	err := tx.QueryRowContext(ctx, "SELECT scope_key,epoch,state,target_permission FROM authorization_change WHERE revocation_id=? FOR UPDATE", id).Scan(&current.Scope, &current.Epoch, &current.State, &current.TargetPermission)
	return current, err
}
````

## 第 6 步：InsertFence

插入位置：第 5 步代码之后，紧接追加；完成稿约第 28 行。

本步声明：`func InsertFence(ctx context.Context, tx *sql.Tx, id, key string, epoch uint64, binding []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, `INSERT INTO authorization_change(revocation_id,scope_key,epoch,target_permission,state,participant_plan) VALUES(?,?,?,?,&#x27;FENCING&#x27;,&#x27;&#91;&quot;edge&quot;,&quot;collaboration&quot;&#93;&#x27;)`, id, key, epoch, string(binding))</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, `INSERT INTO authorization_change(revocation_id,scope_key,epoch,target_permission,state,participant_plan) VALUES(?,?,?,?,&#x27;FENCING&#x27;,&#x27;&#91;&quot;edge&quot;,&quot;collaboration&quot;&#93;&#x27;)`, id, key, epoch, string(binding))</code> |
| <code>key</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, `INSERT INTO authorization_change(revocation_id,scope_key,epoch,target_permission,state,participant_plan) VALUES(?,?,?,?,&#x27;FENCING&#x27;,&#x27;&#91;&quot;edge&quot;,&quot;collaboration&quot;&#93;&#x27;)`, id, key, epoch, string(binding))</code> |
| <code>epoch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, `INSERT INTO authorization_change(revocation_id,scope_key,epoch,target_permission,state,participant_plan) VALUES(?,?,?,?,&#x27;FENCING&#x27;,&#x27;&#91;&quot;edge&quot;,&quot;collaboration&quot;&#93;&#x27;)`, id, key, epoch, string(binding))</code> |
| <code>binding</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>string(binding)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, `INSERT INTO authorization_change(revocation_id,scope_key,epoch,target_permission,state,participant_plan) VALUES(?,?,?,?,&#x27;FENCING&#x27;,&#x27;&#91;&quot;edge&quot;,&quot;collaboration&quot;&#93;&#x27;)`, id, key, epoch, string(binding))</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1289:1632 -->
````go
func InsertFence(ctx context.Context, tx *sql.Tx, id, key string, epoch uint64, binding []byte) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO authorization_change(revocation_id,scope_key,epoch,target_permission,state,participant_plan) VALUES(?,?,?,?,'FENCING','["edge","collaboration"]')`, id, key, epoch, string(binding))
	return err
}
````

## 第 7 步：CloseSourceAdmissions

插入位置：第 6 步代码之后，紧接追加；完成稿约第 32 行。

本步声明：`func CloseSourceAdmissions(ctx context.Context, tx *sql.Tx, key string, epoch uint64) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document_admission a JOIN admission_source x ON x.admission_id=a.admission_id SET a.state=&#x27;CLOSED&#x27; WHERE x.scope_key=? AND x.epoch&lt;=? AND a.state&lt;&gt;&#x27;CLOSED&#x27;&quot;, key, epoch)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE read_view v JOIN document_admission a ON a.admission_id=v.admission_id JOIN admission_source x ON x.admission_id=a.admission_id SET v.state=&#x27;CLOSED&#x27; WHE …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, &quot;UPDATE collaboration_ticket t JOIN document_admission a ON a.admission_id=t.admission_id JOIN admission_source x ON x.admission_id=a.admission_id SET t.state=&#x27; …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>key</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document_admission a JOIN admission_source x ON x.admission_id=a.admission_id SET a.state=&#x27;CLOSED&#x27; WHERE x.scope_key=? AND x.epoch&lt;=? AND a.state&lt;&gt;&#x27;CLOSED&#x27;&quot;, key, epoch)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE read_view v JOIN document_admission a ON a.admission_id=v.admission_id JOIN admission_source x ON x.admission_id=a.admission_id SET v.state=&#x27;CLOSED&#x27; WHE …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, &quot;UPDATE collaboration_ticket t JOIN document_admission a ON a.admission_id=t.admission_id JOIN admission_source x ON x.admission_id=a.admission_id SET t.state=&#x27; …</code>（长表达式，按本步完整代码继续） |
| <code>epoch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE document_admission a JOIN admission_source x ON x.admission_id=a.admission_id SET a.state=&#x27;CLOSED&#x27; WHERE x.scope_key=? AND x.epoch&lt;=? AND a.state&lt;&gt;&#x27;CLOSED&#x27;&quot;, key, epoch)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE read_view v JOIN document_admission a ON a.admission_id=v.admission_id JOIN admission_source x ON x.admission_id=a.admission_id SET v.state=&#x27;CLOSED&#x27; WHE …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, &quot;UPDATE collaboration_ticket t JOIN document_admission a ON a.admission_id=t.admission_id JOIN admission_source x ON x.admission_id=a.admission_id SET t.state=&#x27; …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE document_admission a JOIN admission_source x ON x.admission_id=a.admission_id SET a.state=&#x27;CLOSED&#x27; WHERE x.scope_key=? AND x.epoch&lt;=? AND a.state&lt;&gt;&#x27;CLOSED&#x27;&quot;, key, epoch)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE read_view v JOIN document_admission a ON a.admission_id=v.admission_id JOIN admission_source x ON x.admission_id=a.admission_id SET v.state=&#x27;CLOSED&#x27; WHE …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE collaboration_ticket t JOIN document_admission a ON a.admission_id=t.admission_id JOIN admission_source x ON x.admission_id=a.admission_id SET t.state=&#x27; …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。

<!-- write-step: 7 bytes=1632:2780 -->
````go
func CloseSourceAdmissions(ctx context.Context, tx *sql.Tx, key string, epoch uint64) error {
	_, err := tx.ExecContext(ctx, "UPDATE document_admission a JOIN admission_source x ON x.admission_id=a.admission_id SET a.state='CLOSED' WHERE x.scope_key=? AND x.epoch<=? AND a.state<>'CLOSED'", key, epoch)
	if err != nil {
		return err
	}
	// 关闭只处理这次原范围及旧代次的派生能力。重扫全部已关闭历史不仅增加成本，
	// 还会等在另一个范围的行锁上，令独立关闭任务彼此阻塞。
	_, err = tx.ExecContext(ctx, "UPDATE read_view v JOIN document_admission a ON a.admission_id=v.admission_id JOIN admission_source x ON x.admission_id=a.admission_id SET v.state='CLOSED' WHERE x.scope_key=? AND x.epoch<=? AND a.state='CLOSED' AND v.state<>'CLOSED'", key, epoch)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "UPDATE collaboration_ticket t JOIN document_admission a ON a.admission_id=t.admission_id JOIN admission_source x ON x.admission_id=a.admission_id SET t.state='CLOSED' WHERE x.scope_key=? AND x.epoch<=? AND a.state='CLOSED' AND t.state<>'CLOSED'", key, epoch)
	return err
}
````

## 第 8 步：Fences.Unsettled

插入位置：第 7 步代码之后，紧接追加；完成稿约第 46 行。

本步声明：`func (r *Fences) Unsettled(ctx context.Context) ([]domain.PersistedOutput, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>r.db.QueryContext(ctx, &quot;SELECT business_key,payload FROM durable_task WHERE kind=&#x27;OUTPUT_LEASE&#x27; AND state&lt;&gt;&#x27;SUCCEEDED&#x27;&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;domain.PersistedOutput</code> | 使用 <code>domain.PersistedOutput</code> 类型；其定义处的职责：定义编辑准入、票据、关闭状态、输出实例与租约的固定事实。<br>实际返回表达式：<code>nil</code>；<code>list</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;OUTPUT_RECORD_INVALID&quot;)</code>；<code>rows.Err()</code> |

**跟着本段代码走一遍：**

- 执行 <code>r.db.QueryContext(ctx, &quot;SELECT business_key,payload FROM durable_task WHERE kind=&#x27;OUTPUT_LEASE&#x27; AND state&lt;&gt;&#x27;SUCCEEDED&#x27;&quot;)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>rows.Scan(&amp;output.ID, &amp;output.Payload) != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;OUTPUT_RECORD_INVALID&quot;)</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;output.ID, &amp;output.Payload)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return list, rows.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=2780:3315 -->
````go
func (r *Fences) Unsettled(ctx context.Context) ([]domain.PersistedOutput, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT business_key,payload FROM durable_task WHERE kind='OUTPUT_LEASE' AND state<>'SUCCEEDED'")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []domain.PersistedOutput
	for rows.Next() {
		var output domain.PersistedOutput
		if rows.Scan(&output.ID, &output.Payload) != nil {
			return nil, errors.New("OUTPUT_RECORD_INVALID")
		}
		list = append(list, output)
	}
	return list, rows.Err()
}
````

## 第 9 步：Fences.RecordEdgeAck

插入位置：第 8 步代码之后，紧接追加；完成稿约第 62 行。

本步声明：`func (r *Fences) RecordEdgeAck(ctx context.Context, id, key string, epoch uint64, digest []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>r.db.ExecContext(ctx, &quot;INSERT INTO authorization_ack(revocation_id,participant,scope_key,epoch,phase,result_digest,completed_at) VALUES(?,&#x27;edge&#x27;,?,?,&#x27;OUTPUT_STOPPED&#x27;,?,UTC_TIMESTAM …</code>（长表达式，按本步完整代码继续） |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>r.db.ExecContext(ctx, &quot;INSERT INTO authorization_ack(revocation_id,participant,scope_key,epoch,phase,result_digest,completed_at) VALUES(?,&#x27;edge&#x27;,?,?,&#x27;OUTPUT_STOPPED&#x27;,?,UTC_TIMESTAM …</code>（长表达式，按本步完整代码继续） |
| <code>key</code> | <code>string</code> | 在此声明中的实际使用：<code>r.db.ExecContext(ctx, &quot;INSERT INTO authorization_ack(revocation_id,participant,scope_key,epoch,phase,result_digest,completed_at) VALUES(?,&#x27;edge&#x27;,?,?,&#x27;OUTPUT_STOPPED&#x27;,?,UTC_TIMESTAM …</code>（长表达式，按本步完整代码继续） |
| <code>epoch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>r.db.ExecContext(ctx, &quot;INSERT INTO authorization_ack(revocation_id,participant,scope_key,epoch,phase,result_digest,completed_at) VALUES(?,&#x27;edge&#x27;,?,?,&#x27;OUTPUT_STOPPED&#x27;,?,UTC_TIMESTAM …</code>（长表达式，按本步完整代码继续） |
| <code>digest</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>r.db.ExecContext(ctx, &quot;INSERT INTO authorization_ack(revocation_id,participant,scope_key,epoch,phase,result_digest,completed_at) VALUES(?,&#x27;edge&#x27;,?,?,&#x27;OUTPUT_STOPPED&#x27;,?,UTC_TIMESTAM …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>r.db.ExecContext(ctx, &quot;INSERT INTO authorization_ack(revocation_id,participant,scope_key,epoch,phase,result_digest,completed_at) VALUES(?,&#x27;edge&#x27;,?,?,&#x27;OUTPUT_STOPPED&#x27;,?,UTC_TIMESTAM …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=3315:3717 -->
````go
func (r *Fences) RecordEdgeAck(ctx context.Context, id, key string, epoch uint64, digest []byte) error {
	_, err := r.db.ExecContext(ctx, "INSERT INTO authorization_ack(revocation_id,participant,scope_key,epoch,phase,result_digest,completed_at) VALUES(?,'edge',?,?,'OUTPUT_STOPPED',?,UTC_TIMESTAMP(6)) ON DUPLICATE KEY UPDATE result_digest=VALUES(result_digest)", id, key, epoch, digest)
	return err
}
````

## 第 10 步：RecordNodeAck

插入位置：第 9 步代码之后，紧接追加；完成稿约第 66 行。

本步声明：`func RecordNodeAck(ctx context.Context, tx *sql.Tx, id, key string, epoch uint64, digest []byte) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO authorization_ack(revocation_id,participant,scope_key,epoch,phase,result_digest,completed_at) VALUES(?,&#x27;collaboration&#x27;,?,?,&#x27;OUTPUT_STOPPED&#x27;,?,UTC_T …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO authorization_ack(revocation_id,participant,scope_key,epoch,phase,result_digest,completed_at) VALUES(?,&#x27;collaboration&#x27;,?,?,&#x27;OUTPUT_STOPPED&#x27;,?,UTC_T …</code>（长表达式，按本步完整代码继续） |
| <code>key</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO authorization_ack(revocation_id,participant,scope_key,epoch,phase,result_digest,completed_at) VALUES(?,&#x27;collaboration&#x27;,?,?,&#x27;OUTPUT_STOPPED&#x27;,?,UTC_T …</code>（长表达式，按本步完整代码继续） |
| <code>epoch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO authorization_ack(revocation_id,participant,scope_key,epoch,phase,result_digest,completed_at) VALUES(?,&#x27;collaboration&#x27;,?,?,&#x27;OUTPUT_STOPPED&#x27;,?,UTC_T …</code>（长表达式，按本步完整代码继续） |
| <code>digest</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO authorization_ack(revocation_id,participant,scope_key,epoch,phase,result_digest,completed_at) VALUES(?,&#x27;collaboration&#x27;,?,?,&#x27;OUTPUT_STOPPED&#x27;,?,UTC_T …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO authorization_ack(revocation_id,participant,scope_key,epoch,phase,result_digest,completed_at) VALUES(?,&#x27;collaboration&#x27;,?,?,&#x27;OUTPUT_STOPPED&#x27;,?,UTC_T …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=3717:4126 -->
````go
func RecordNodeAck(ctx context.Context, tx *sql.Tx, id, key string, epoch uint64, digest []byte) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO authorization_ack(revocation_id,participant,scope_key,epoch,phase,result_digest,completed_at) VALUES(?,'collaboration',?,?,'OUTPUT_STOPPED',?,UTC_TIMESTAMP(6)) ON DUPLICATE KEY UPDATE result_digest=VALUES(result_digest)", id, key, epoch, digest)
	return err
}
````

## 第 11 步：CompleteFence

插入位置：第 10 步代码之后，紧接追加；完成稿约第 70 行。

本步声明：`func CompleteFence(ctx context.Context, tx *sql.Tx, id string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE authorization_change SET state=&#x27;SUCCEEDED&#x27; WHERE revocation_id=?&quot;, id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE authorization_change SET state=&#x27;SUCCEEDED&#x27; WHERE revocation_id=?&quot;, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE authorization_change SET state=&#x27;SUCCEEDED&#x27; WHERE revocation_id=?&quot;, id)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=4126:4381 -->
````go
func CompleteFence(ctx context.Context, tx *sql.Tx, id string) error {
	_, err := tx.ExecContext(ctx, "UPDATE authorization_change SET state='SUCCEEDED' WHERE revocation_id=?", id)
	return err
}

// output.go：保留原实现，按业务合并文件。

````

## 第 12 步：Outputs

插入位置：第 11 步代码之后，紧接追加；完成稿约第 77 行。

本步声明：`type Outputs struct{ db *sql.DB }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |

<!-- write-step: 12 bytes=4381:4416 -->
````go
type Outputs struct{ db *sql.DB }

````

## 第 13 步：NewOutputs

插入位置：第 12 步代码之后，紧接追加；完成稿约第 79 行。

本步声明：`func NewOutputs(db *sql.DB) *Outputs`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Outputs</code> | 实际返回表达式：<code>&amp;Outputs{db: db}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Outputs{db: db}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=4416:4482 -->
````go
func NewOutputs(db *sql.DB) *Outputs { return &Outputs{db: db} }

````

## 第 14 步：EnsureInstance

插入位置：第 13 步代码之后，紧接追加；完成稿约第 81 行。

所有写方法只使用用例传入的tx，来源锁和输出记录不能分散到另一连接。

本步声明：`func EnsureInstance(ctx context.Context, tx *sql.Tx, i domain.OutputInstance) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO output_instance(instance_id,boot_digest,pid,state,last_heartbeat_at) VALUES(?,?,?,&#x27;RUNNING&#x27;,UTC_TIMESTAMP(6)) ON DUPLICATE KEY UPDATE last_heartbea …</code>（长表达式，按本步完整代码继续）；<code>tx.QueryRowContext(ctx, &quot;SELECT pid,boot_digest,state FROM output_instance WHERE instance_id=? FOR UPDATE&quot;, i.ID)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code>；<code>tx.QueryRowContext</code> |
| <code>i</code> | <code>domain.OutputInstance</code> | 使用 <code>domain.OutputInstance</code> 类型；其定义处的职责：输出租约是持久恢复协议的事实；期限本身不构成旧进程退出或handler排空的证明。<br>在此声明中的实际使用：<code>i.Boot</code>；<code>i.ID</code>；<code>i.PID</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;OUTPUT_INSTANCE_INVALID&quot;)</code>；<code>err</code>；<code>errors.New(&quot;OUTPUT_INSTANCE_CONFLICT&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil &#124;&#124; len(digest) != 32 &#124;&#124; i.ID == &quot;&quot; &#124;&#124; i.PID == 0</code>。条件成立时执行 <code>return errors.New(&quot;OUTPUT_INSTANCE_INVALID&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO output_instance(instance_id,boot_digest,pid,state,last_heartbeat_at) VALUES(?,?,?,&#x27;RUNNING&#x27;,UTC_TIMESTAMP(6)) ON DUPLICATE KEY UPDATE last_heartbea …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>tx.QueryRowContext(ctx, &quot;SELECT pid,boot_digest,state FROM output_instance WHERE instance_id=? FOR UPDATE&quot;, i.ID).Scan(&amp;pid, &amp;prior, &amp;state) != nil &#124;&#124; pid != i.PID &#124;&#124; !bytes.Equal(prior, digest) &#124;&#124; state != &quot;RUNNING&quot;</code>。条件成立时执行 <code>return errors.New(&quot;OUTPUT_INSTANCE_CONFLICT&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT pid,boot_digest,state FROM output_instance WHERE instance_id=? FOR UPDATE&quot;, i.ID).Scan(&amp;pid, &amp;prior, &amp;state)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT pid,boot_digest,state FROM output_instance WHERE instance_id=? FOR UPDATE&quot;, i.ID)</code>。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=4482:5436 -->
````go
// 所有写方法只使用用例传入的tx，来源锁和输出记录不能分散到另一连接。
func EnsureInstance(ctx context.Context, tx *sql.Tx, i domain.OutputInstance) error {
	digest, err := hex.DecodeString(i.Boot)
	if err != nil || len(digest) != 32 || i.ID == "" || i.PID == 0 {
		return errors.New("OUTPUT_INSTANCE_INVALID")
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO output_instance(instance_id,boot_digest,pid,state,last_heartbeat_at) VALUES(?,?,?,'RUNNING',UTC_TIMESTAMP(6)) ON DUPLICATE KEY UPDATE last_heartbeat_at=UTC_TIMESTAMP(6)", i.ID, digest, i.PID)
	if err != nil {
		return err
	}
	var pid uint64
	var prior []byte
	var state string
	if tx.QueryRowContext(ctx, "SELECT pid,boot_digest,state FROM output_instance WHERE instance_id=? FOR UPDATE", i.ID).Scan(&pid, &prior, &state) != nil || pid != i.PID || !bytes.Equal(prior, digest) || state != "RUNNING" {
		return errors.New("OUTPUT_INSTANCE_CONFLICT")
	}
	return nil
}

````

## 第 15 步：InsertOutputLease

插入位置：第 14 步代码之后，紧接追加；完成稿约第 100 行。

本步声明：`func InsertOutputLease(ctx context.Context, tx *sql.Tx, l domain.OutputLease) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;INSERT INTO durable_task(kind,business_key,payload,state,instance_id,lease_until,retry_at) VALUES(&#x27;OUTPUT_LEASE&#x27;,?,?,&#x27;RUNNING&#x27;,?,DATE_ADD(UTC_TIMESTAMP(6),INTE …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, &quot;INSERT INTO delivery_session(delivery_id,admission_id,document_id,publication_epoch,release_id,target_kind,target_ref,instance_id,handler_id,state,expires_at)  …</code>（长表达式，按本步完整代码继续）；<code>tx.ExecContext(ctx, &quot;INSERT INTO delivery_session(delivery_id,admission_id,document_id,target_kind,target_ref,instance_id,handler_id,state,expires_at) VALUES(?,?,?,&#x27;HTTP&#x27;,?,?,?,&#x27;OP …</code>（长表达式，按本步完整代码继续） |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>l</code> | <code>domain.OutputLease</code> | 使用 <code>domain.OutputLease</code> 类型；其定义处的职责：定义编辑准入、票据、关闭状态、输出实例与租约的固定事实。<br>在此声明中的实际使用：<code>l.PublicationEpoch</code>；<code>l.Release</code>；<code>l.Document</code>；<code>l.Admission</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;OUTPUT_PUBLICATION_INVALID&quot;)</code>；<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>public</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;OUTPUT_PUBLICATION_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; l.Document == 0 &#124;&#124; l.Admission != &quot;&quot;</code>。条件成立时执行 <code>return errors.New(&quot;OUTPUT_PUBLICATION_INVALID&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO durable_task(kind,business_key,payload,state,instance_id,lease_until,retry_at) VALUES(&#x27;OUTPUT_LEASE&#x27;,?,?,&#x27;RUNNING&#x27;,?,DATE_ADD(UTC_TIMESTAMP(6),INTE …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>l.Document != 0</code>。
- 检查 <code>public</code>。
- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO delivery_session(delivery_id,admission_id,document_id,publication_epoch,release_id,target_kind,target_ref,instance_id,handler_id,state,expires_at)  …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 执行 <code>tx.ExecContext(ctx, &quot;INSERT INTO delivery_session(delivery_id,admission_id,document_id,target_kind,target_ref,instance_id,handler_id,state,expires_at) VALUES(?,?,?,&#x27;HTTP&#x27;,?,?,?,&#x27;OP …</code>（长表达式，按本步完整代码继续）。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=5436:7074 -->
````go
func InsertOutputLease(ctx context.Context, tx *sql.Tx, l domain.OutputLease) error {
	var epoch, release uint64
	public := l.PublicationEpoch != "" || l.Release != ""
	if public {
		var err error
		epoch, err = domain.CanonicalID(l.PublicationEpoch)
		if err != nil {
			return errors.New("OUTPUT_PUBLICATION_INVALID")
		}
		release, err = domain.CanonicalID(l.Release)
		if err != nil || l.Document == 0 || l.Admission != "" {
			return errors.New("OUTPUT_PUBLICATION_INVALID")
		}
	}
	raw, _ := json.Marshal(l)
	_, err := tx.ExecContext(ctx, "INSERT INTO durable_task(kind,business_key,payload,state,instance_id,lease_until,retry_at) VALUES('OUTPUT_LEASE',?,?,'RUNNING',?,DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 30 SECOND),UTC_TIMESTAMP(6))", l.ID, string(raw), l.Instance)
	if err != nil {
		return err
	}
	if l.Document != 0 {
		if public {
			// 匿名和已登录正式读者都绑定当前正式指针；身份依赖另存sources，不伪造Doc准入。
			_, err = tx.ExecContext(ctx, "INSERT INTO delivery_session(delivery_id,admission_id,document_id,publication_epoch,release_id,target_kind,target_ref,instance_id,handler_id,state,expires_at) VALUES(?,NULL,?,?,?,'HTTP',?,?,?,'OPEN',DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 30 SECOND))", l.ID, l.Document, epoch, release, l.ID, l.Instance, l.Handler)
		} else {
			_, err = tx.ExecContext(ctx, "INSERT INTO delivery_session(delivery_id,admission_id,document_id,target_kind,target_ref,instance_id,handler_id,state,expires_at) VALUES(?,?,?,'HTTP',?,?,?,'OPEN',DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 30 SECOND))", l.ID, l.Admission, l.Document, l.ID, l.Instance, l.Handler)
		}
	}
	return err
}
````

## 第 16 步：Outputs.Settle

插入位置：第 15 步代码之后，紧接追加；完成稿约第 129 行。

本步声明：`func (m *Outputs) Settle(ctx context.Context, tx *sql.Tx, id, instance string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE durable_task SET state=&#x27;SUCCEEDED&#x27; WHERE kind=&#x27;OUTPUT_LEASE&#x27; AND business_key=? AND instance_id=?&quot;, id, instance)</code>；<code>tx.QueryRowContext(ctx, &quot;SELECT state FROM durable_task WHERE kind=&#x27;OUTPUT_LEASE&#x27; AND business_key=? AND instance_id=? FOR UPDATE&quot;, id, instance)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE delivery_session SET state=&#x27;SETTLED&#x27; WHERE delivery_id=? AND instance_id=?&quot;, id, instance)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code>；<code>tx.QueryRowContext</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE durable_task SET state=&#x27;SUCCEEDED&#x27; WHERE kind=&#x27;OUTPUT_LEASE&#x27; AND business_key=? AND instance_id=?&quot;, id, instance)</code>；<code>tx.QueryRowContext(ctx, &quot;SELECT state FROM durable_task WHERE kind=&#x27;OUTPUT_LEASE&#x27; AND business_key=? AND instance_id=? FOR UPDATE&quot;, id, instance)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE delivery_session SET state=&#x27;SETTLED&#x27; WHERE delivery_id=? AND instance_id=?&quot;, id, instance)</code> |
| <code>instance</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE durable_task SET state=&#x27;SUCCEEDED&#x27; WHERE kind=&#x27;OUTPUT_LEASE&#x27; AND business_key=? AND instance_id=?&quot;, id, instance)</code>；<code>tx.QueryRowContext(ctx, &quot;SELECT state FROM durable_task WHERE kind=&#x27;OUTPUT_LEASE&#x27; AND business_key=? AND instance_id=? FOR UPDATE&quot;, id, instance)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE delivery_session SET state=&#x27;SETTLED&#x27; WHERE delivery_id=? AND instance_id=?&quot;, id, instance)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;OUTPUT_LEASE_UNKNOWN&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE durable_task SET state=&#x27;SUCCEEDED&#x27; WHERE kind=&#x27;OUTPUT_LEASE&#x27; AND business_key=? AND instance_id=?&quot;, id, instance)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>count == 0</code>。
- 检查 <code>tx.QueryRowContext(ctx, &quot;SELECT state FROM durable_task WHERE kind=&#x27;OUTPUT_LEASE&#x27; AND business_key=? AND instance_id=? FOR UPDATE&quot;, id, instance).Scan(&amp;existing) != nil &#124;&#124; existing != &quot;SUCCEEDED&quot;</code>。条件成立时执行 <code>return errors.New(&quot;OUTPUT_LEASE_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state FROM durable_task WHERE kind=&#x27;OUTPUT_LEASE&#x27; AND business_key=? AND instance_id=? FOR UPDATE&quot;, id, instance).Scan(&amp;existing)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT state FROM durable_task WHERE kind=&#x27;OUTPUT_LEASE&#x27; AND business_key=? AND instance_id=? FOR UPDATE&quot;, id, instance)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE delivery_session SET state=&#x27;SETTLED&#x27; WHERE delivery_id=? AND instance_id=?&quot;, id, instance)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=7074:7858 -->
````go
func (m *Outputs) Settle(ctx context.Context, tx *sql.Tx, id, instance string) error {
	result, err := tx.ExecContext(ctx, "UPDATE durable_task SET state='SUCCEEDED' WHERE kind='OUTPUT_LEASE' AND business_key=? AND instance_id=?", id, instance)
	if err != nil {
		return err
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		var existing string
		if tx.QueryRowContext(ctx, "SELECT state FROM durable_task WHERE kind='OUTPUT_LEASE' AND business_key=? AND instance_id=? FOR UPDATE", id, instance).Scan(&existing) != nil || existing != "SUCCEEDED" {
			return errors.New("OUTPUT_LEASE_UNKNOWN")
		}
	}
	if _, err = tx.ExecContext(ctx, "UPDATE delivery_session SET state='SETTLED' WHERE delivery_id=? AND instance_id=?", id, instance); err != nil {
		return err
	}
	return nil
}

````

## 第 17 步：OutputProcess

插入位置：第 16 步代码之后，紧接追加；完成稿约第 147 行。

本步声明：`type OutputProcess struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ID</code> | <code>string</code> | 输出进程的实例标识，配合PID核对归属。 |
| <code>PID</code> | <code>uint64</code> | 操作系统进程号；PID会复用，不能单独凭它终止进程。 |

<!-- write-step: 17 bytes=7858:7913 -->
````go
type OutputProcess struct {
	ID  string
	PID uint64
}

````

## 第 18 步：Outputs.ActiveInstances

插入位置：第 17 步代码之后，紧接追加；完成稿约第 152 行。

本步声明：`func (r *Outputs) ActiveInstances(ctx context.Context) ([]OutputProcess, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>r.db.QueryContext(ctx, &quot;SELECT instance_id,pid FROM output_instance WHERE state&lt;&gt;&#x27;EXIT_CONFIRMED&#x27;&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;OutputProcess</code> | 实际返回表达式：<code>nil</code>；<code>list</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;OUTPUT_RECOVERY_INVALID&quot;)</code>；<code>rows.Err()</code> |

**跟着本段代码走一遍：**

- 执行 <code>r.db.QueryContext(ctx, &quot;SELECT instance_id,pid FROM output_instance WHERE state&lt;&gt;&#x27;EXIT_CONFIRMED&#x27;&quot;)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>rows.Scan(&amp;instance.ID, &amp;instance.PID) != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;OUTPUT_RECOVERY_INVALID&quot;)</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;instance.ID, &amp;instance.PID)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return list, rows.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=7913:8413 -->
````go
func (r *Outputs) ActiveInstances(ctx context.Context) ([]OutputProcess, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT instance_id,pid FROM output_instance WHERE state<>'EXIT_CONFIRMED'")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []OutputProcess
	for rows.Next() {
		var instance OutputProcess
		if rows.Scan(&instance.ID, &instance.PID) != nil {
			return nil, errors.New("OUTPUT_RECOVERY_INVALID")
		}
		list = append(list, instance)
	}
	return list, rows.Err()
}
````

## 第 19 步：Outputs.ConfirmExit

插入位置：第 18 步代码之后，紧接追加；完成稿约第 168 行。

本步声明：`func (r *Outputs) ConfirmExit(ctx context.Context, tx *sql.Tx, id, state string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE output_instance SET state=&#x27;EXIT_CONFIRMED&#x27;,exit_proof_ref=? WHERE instance_id=?&quot;, &quot;PID_CHECK:&quot;+state, id)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE delivery_session SET state=&#x27;SETTLED&#x27;,stop_reason=&#x27;EXIT_CONFIRMED&#x27; WHERE instance_id=? AND state&lt;&gt;&#x27;SETTLED&#x27;&quot;, id)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE durable_task SET state=&#x27;SUCCEEDED&#x27;,last_error=&#x27;OUTPUT_EXIT_CONFIRMED&#x27; WHERE kind=&#x27;OUTPUT_LEASE&#x27; AND instance_id=? AND state&lt;&gt;&#x27;SUCCEEDED&#x27;&quot;, id)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.ExecContext</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>tx.ExecContext(ctx, &quot;UPDATE output_instance SET state=&#x27;EXIT_CONFIRMED&#x27;,exit_proof_ref=? WHERE instance_id=?&quot;, &quot;PID_CHECK:&quot;+state, id)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE delivery_session SET state=&#x27;SETTLED&#x27;,stop_reason=&#x27;EXIT_CONFIRMED&#x27; WHERE instance_id=? AND state&lt;&gt;&#x27;SETTLED&#x27;&quot;, id)</code>；<code>tx.ExecContext(ctx, &quot;UPDATE durable_task SET state=&#x27;SUCCEEDED&#x27;,last_error=&#x27;OUTPUT_EXIT_CONFIRMED&#x27; WHERE kind=&#x27;OUTPUT_LEASE&#x27; AND instance_id=? AND state&lt;&gt;&#x27;SUCCEEDED&#x27;&quot;, id)</code> |
| <code>state</code> | <code>string</code> | 在此声明中的实际使用：<code>&quot;PID_CHECK:&quot;+state</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE output_instance SET state=&#x27;EXIT_CONFIRMED&#x27;,exit_proof_ref=? WHERE instance_id=?&quot;, &quot;PID_CHECK:&quot;+state, id)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err == nil</code>。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE delivery_session SET state=&#x27;SETTLED&#x27;,stop_reason=&#x27;EXIT_CONFIRMED&#x27; WHERE instance_id=? AND state&lt;&gt;&#x27;SETTLED&#x27;&quot;, id)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 检查 <code>err == nil</code>。
- 执行 <code>tx.ExecContext(ctx, &quot;UPDATE durable_task SET state=&#x27;SUCCEEDED&#x27;,last_error=&#x27;OUTPUT_EXIT_CONFIRMED&#x27; WHERE kind=&#x27;OUTPUT_LEASE&#x27; AND instance_id=? AND state&lt;&gt;&#x27;SUCCEEDED&#x27;&quot;, id)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 19 bytes=8413:9035 -->
````go
func (r *Outputs) ConfirmExit(ctx context.Context, tx *sql.Tx, id, state string) error {
	_, err := tx.ExecContext(ctx, "UPDATE output_instance SET state='EXIT_CONFIRMED',exit_proof_ref=? WHERE instance_id=?", "PID_CHECK:"+state, id)
	if err == nil {
		_, err = tx.ExecContext(ctx, "UPDATE delivery_session SET state='SETTLED',stop_reason='EXIT_CONFIRMED' WHERE instance_id=? AND state<>'SETTLED'", id)
	}
	if err == nil {
		_, err = tx.ExecContext(ctx, "UPDATE durable_task SET state='SUCCEEDED',last_error='OUTPUT_EXIT_CONFIRMED' WHERE kind='OUTPUT_LEASE' AND instance_id=? AND state<>'SUCCEEDED'", id)
	}
	return err
}
````

## 第 20 步：Outputs.UnrecoveredCount

插入位置：第 19 步代码之后，紧接追加；完成稿约第 178 行。

本步声明：`func (r *Outputs) UnrecoveredCount(ctx context.Context) (int, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>r.db.QueryRowContext(ctx, &quot;SELECT COUNT(*) FROM durable_task t LEFT JOIN output_instance i ON i.instance_id=t.instance_id WHERE t.kind=&#x27;OUTPUT_LEASE&#x27; AND t.state&lt;&gt;&#x27;SUCCEEDED&#x27; AND (i.state IS NULL OR i.state&lt;&gt;&#x27;RUNNING&#x27;)&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>int</code> | 实际返回表达式：<code>count</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>r.db.QueryRowContext(ctx, &quot;SELECT COUNT(*) FROM durable_task t LEFT JOIN output_instance i ON i.instance_id=t.instance_id WHERE t.kind=&#x27;OUTPUT_LEASE&#x27; AND t.state&lt;&gt;&#x27;SUCCEEDED&#x27; AND ( …</code>（长表达式，按本步完整代码继续）。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>r.db.QueryRowContext(ctx, &quot;SELECT COUNT(*) FROM durable_task t LEFT JOIN output_instance i ON i.instance_id=t.instance_id WHERE t.kind=&#x27;OUTPUT_LEASE&#x27; AND t.state&lt;&gt;&#x27;SUCCEEDED&#x27; AND (i.state IS NULL OR i.state&lt;&gt;&#x27;RUNNING&#x27;)&quot;)</code>。
- 此处通过 <code>return count, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 20 bytes=9035:9385 -->
````go
func (r *Outputs) UnrecoveredCount(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM durable_task t LEFT JOIN output_instance i ON i.instance_id=t.instance_id WHERE t.kind='OUTPUT_LEASE' AND t.state<>'SUCCEEDED' AND (i.state IS NULL OR i.state<>'RUNNING')").Scan(&count)
	return count, err
}

````

## 第 21 步：PendingOutput

插入位置：第 20 步代码之后，紧接追加；完成稿约第 184 行。

本步声明：`type PendingOutput struct{ ID, Instance string }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ID</code> | <code>string</code> | 尚待停止或确认的输出编号。 |
| <code>Instance</code> | <code>string</code> | 输出所属实例，恢复时定位对应输出参与者。 |

<!-- write-step: 21 bytes=9385:9435 -->
````go
type PendingOutput struct{ ID, Instance string }

````

## 第 22 步：Outputs.Settled

插入位置：第 21 步代码之后，紧接追加；完成稿约第 186 行。

Pending和本地handler不是同一时点；重新查询时仍须绑定原输出及原实例。
只接受持久SUCCEEDED，缺记录、读取失败和过期租约都不能证明输出已结束。

本步声明：`func (r *Outputs) Settled(ctx context.Context, id, instance string) (bool, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>r.db.QueryRowContext(ctx, &quot;SELECT state FROM durable_task WHERE kind=&#x27;OUTPUT_LEASE&#x27; AND business_key=? AND instance_id=?&quot;, id, instance)</code> |
| <code>id</code> | <code>string</code> | 在此声明中的实际使用：<code>r.db.QueryRowContext(ctx, &quot;SELECT state FROM durable_task WHERE kind=&#x27;OUTPUT_LEASE&#x27; AND business_key=? AND instance_id=?&quot;, id, instance)</code> |
| <code>instance</code> | <code>string</code> | 在此声明中的实际使用：<code>r.db.QueryRowContext(ctx, &quot;SELECT state FROM durable_task WHERE kind=&#x27;OUTPUT_LEASE&#x27; AND business_key=? AND instance_id=?&quot;, id, instance)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>state == &quot;SUCCEEDED&quot;</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>r.db.QueryRowContext(ctx, &quot;SELECT state FROM durable_task WHERE kind=&#x27;OUTPUT_LEASE&#x27; AND business_key=? AND instance_id=?&quot;, id, instance).Scan(&amp;state)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>r.db.QueryRowContext(ctx, &quot;SELECT state FROM durable_task WHERE kind=&#x27;OUTPUT_LEASE&#x27; AND business_key=? AND instance_id=?&quot;, id, instance)</code>。
- 此处通过 <code>return state == &quot;SUCCEEDED&quot;, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 22 bytes=9435:9934 -->
````go
// Pending和本地handler不是同一时点；重新查询时仍须绑定原输出及原实例。
// 只接受持久SUCCEEDED，缺记录、读取失败和过期租约都不能证明输出已结束。
func (r *Outputs) Settled(ctx context.Context, id, instance string) (bool, error) {
	var state string
	err := r.db.QueryRowContext(ctx, "SELECT state FROM durable_task WHERE kind='OUTPUT_LEASE' AND business_key=? AND instance_id=?", id, instance).Scan(&state)
	return state == "SUCCEEDED", err
}

````

## 第 23 步：Outputs.Pending

插入位置：第 22 步代码之后，紧接追加；完成稿约第 194 行。

本步声明：`func (r *Outputs) Pending(ctx context.Context) ([]PendingOutput, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>r.db.QueryContext(ctx, &quot;SELECT business_key,instance_id FROM durable_task WHERE kind=&#x27;OUTPUT_LEASE&#x27; AND state&lt;&gt;&#x27;SUCCEEDED&#x27;&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;PendingOutput</code> | 实际返回表达式：<code>nil</code>；<code>list</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;OUTPUT_RECORD_INVALID&quot;)</code>；<code>rows.Err()</code> |

**跟着本段代码走一遍：**

- 执行 <code>r.db.QueryContext(ctx, &quot;SELECT business_key,instance_id FROM durable_task WHERE kind=&#x27;OUTPUT_LEASE&#x27; AND state&lt;&gt;&#x27;SUCCEEDED&#x27;&quot;)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>rows.Close()</code>，当前函数退出时执行，提前 return 也会触发。
- 检查 <code>rows.Scan(&amp;output.ID, &amp;output.Instance) != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;OUTPUT_RECORD_INVALID&quot;)</code>。当前函数在这里结束。
- 执行 <code>rows.Scan(&amp;output.ID, &amp;output.Instance)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return list, rows.Err()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 23 bytes=9934:10446 -->
````go
func (r *Outputs) Pending(ctx context.Context) ([]PendingOutput, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT business_key,instance_id FROM durable_task WHERE kind='OUTPUT_LEASE' AND state<>'SUCCEEDED'")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []PendingOutput
	for rows.Next() {
		var output PendingOutput
		if rows.Scan(&output.ID, &output.Instance) != nil {
			return nil, errors.New("OUTPUT_RECORD_INVALID")
		}
		list = append(list, output)
	}
	return list, rows.Err()
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/repository/document/output_control_queries.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/11-output-fence.md)。
