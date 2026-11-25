# 分步编写 `services/document/internal/repository/document/admin_retirement.go`

先按 [D13 · 账号注销、组织关闭与公开留存](../lessons/document/13-account-retirement.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/144-admin-retirement.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

锁定知识库管理员退休候选并预留原注销操作。

两个管理员并发注销时只锁各自成员会都误判还有另一个管理员，必须先锁同一KB。

写之前先按这条执行路线理解各段的连接：

1. LockAdministratorRetirementCandidates按KB编号取锁并核其他预留
2. ReserveAdministratorRetirement同号幂等保存。

本文件需要守住的边界：

- 仓储只存预留，最后管理员规则由服务决定
- 不能覆盖另一未完成原号。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/repository/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/repository/document/admin_retirement.go') -and (Get-Item -LiteralPath 'services/document/internal/repository/document/admin_retirement.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/repository/document/admin_retirement.go')) { New-Item -ItemType File -Path 'services/document/internal/repository/document/admin_retirement.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:125 -->
````go
package document

import (
	domain "collaboration/document/internal/domain/document"
	"context"
	"database/sql"
	"errors"
)

````

## 第 2 步：AdministratorRetirementCandidate

插入位置：第 1 步代码之后，紧接追加；完成稿约第 10 行。

本步声明：`type AdministratorRetirementCandidate struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>KnowledgeBase</code> | <code>uint64</code> | 需要检查管理员交接的知识库。 |
| <code>Organization</code> | <code>uint64</code> | 知识库所属组织；个人空间不按组织管理员路径处理。 |
| <code>PersonalOwner</code> | <code>uint64</code> | 个人空间的所有者账号。 |
| <code>Lifecycle</code> | <code>string</code> | 知识库当前生命周期状态。 |
| <code>MemberState</code> | <code>string</code> | 当前账号在知识库中的成员状态。 |
| <code>ReservationOperation</code> | <code>string</code> | 已占用管理员退役预约的操作号。 |
| <code>ReservationState</code> | <code>string</code> | 该预约状态，决定能否复用或拒绝。 |
| <code>Available</code> | <code>int</code> | 排除已预约退出等情况后仍可承担管理职责的管理员数量。 |

<!-- write-step: 2 bytes=125:376 -->
````go
type AdministratorRetirementCandidate struct {
	KnowledgeBase, Organization, PersonalOwner uint64
	Lifecycle, MemberState                     string
	ReservationOperation, ReservationState     string
	Available                                  int
}

````

## 第 3 步：LockAdministratorRetirementCandidates

插入位置：第 2 步代码之后，紧接追加；完成稿约第 17 行。

LockAdministratorRetirementCandidates 使用调用方原事务，并先按知识库编号持锁。
其他管理员的预留同样受知识库锁串行化，不能只锁当前成员就分别误判最后一名管理员。

本步声明：`func LockAdministratorRetirementCandidates(ctx context.Context, tx *sql.Tx, user uint64) ([]AdministratorRetirementCandidate, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryContext(ctx, `SELECT k.id 		FROM knowledge_base k JOIN knowledge_base_member m ON m.knowledge_base_id=k.id 		WHERE m.user_id=? AND m.role=&#x27;ADMIN&#x27; AND m.state IN (&#x27;ACTIVE&#x27;,&#x27; …</code>（长表达式，按本步完整代码继续）；<code>tx.QueryRowContext(ctx, `SELECT space_id,lifecycle FROM knowledge_base WHERE id=? FOR UPDATE`, id)</code>；<code>tx.QueryRowContext(ctx, `SELECT role,state FROM knowledge_base_member WHERE knowledge_base_id=? AND user_id=? FOR UPDATE`, id, user)</code>；<code>tx.QueryRowContext(ctx, `SELECT COALESCE(organization_id,0),COALESCE(account_owner_id,0) FROM space WHERE id=? FOR SHARE`, space)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryContext</code>；<code>tx.QueryRowContext</code>；<code>lockAdministratorRetirementFacts(ctx, tx, user, &amp;candidate)</code> |
| <code>user</code> | <code>uint64</code> | 在此声明中的实际使用：<code>user == 0</code>；<code>tx.QueryContext(ctx, `SELECT k.id 		FROM knowledge_base k JOIN knowledge_base_member m ON m.knowledge_base_id=k.id 		WHERE m.user_id=? AND m.role=&#x27;ADMIN&#x27; AND m.state IN (&#x27;ACTIVE&#x27;,&#x27; …</code>（长表达式，按本步完整代码继续）；<code>tx.QueryRowContext(ctx, `SELECT role,state FROM knowledge_base_member WHERE knowledge_base_id=? AND user_id=? FOR UPDATE`, id, user)</code>；<code>lockAdministratorRetirementFacts(ctx, tx, user, &amp;candidate)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;AdministratorRetirementCandidate</code> | 实际返回表达式：<code>nil</code>；<code>candidates</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;ADMIN_RETIREMENT_USER_INVALID&quot;)</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>user == 0</code>。条件成立时执行 <code>return nil, errors.New(&quot;ADMIN_RETIREMENT_USER_INVALID&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.QueryContext(ctx, `SELECT k.id 		FROM knowledge_base k JOIN knowledge_base_member m ON m.knowledge_base_id=k.id 		WHERE m.user_id=? AND m.role=&#x27;ADMIN&#x27; AND m.state IN (&#x27;ACTIVE&#x27;,&#x27; …</code>（长表达式，按本步完整代码继续）。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 执行 <code>rows.Scan(&amp;id)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return nil, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT space_id,lifecycle FROM knowledge_base WHERE id=? FOR UPDATE`, id).Scan(&amp;space, &amp;candidate.Lifecycle)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT space_id,lifecycle FROM knowledge_base WHERE id=? FOR UPDATE`, id)</code>。
- 检查 <code>candidate.Lifecycle != &quot;ACTIVE&quot; &amp;&amp; candidate.Lifecycle != &quot;CLOSING&quot;</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT role,state FROM knowledge_base_member WHERE knowledge_base_id=? AND user_id=? FOR UPDATE`, id, user).Scan(&amp;role, &amp;candidate.MemberState)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT role,state FROM knowledge_base_member WHERE knowledge_base_id=? AND user_id=? FOR UPDATE`, id, user)</code>。
- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。

<!-- write-step: 3 bytes=376:2335 -->
````go
// LockAdministratorRetirementCandidates 使用调用方原事务，并先按知识库编号持锁。
// 其他管理员的预留同样受知识库锁串行化，不能只锁当前成员就分别误判最后一名管理员。
func LockAdministratorRetirementCandidates(ctx context.Context, tx *sql.Tx, user uint64) ([]AdministratorRetirementCandidate, error) {
	if user == 0 {
		return nil, errors.New("ADMIN_RETIREMENT_USER_INVALID")
	}
	// 调用方已锁定目标身份来源，任命路径也使用该来源锁；发现编号本身不取成员锁，
	// 避免两个退出者先分别持有自己的成员行，再竞争同一知识库时形成反向死锁。
	rows, err := tx.QueryContext(ctx, `SELECT k.id
		FROM knowledge_base k JOIN knowledge_base_member m ON m.knowledge_base_id=k.id
		WHERE m.user_id=? AND m.role='ADMIN' AND m.state IN ('ACTIVE','REVOKING')
		AND k.lifecycle IN ('ACTIVE','CLOSING') ORDER BY k.id`, user)
	if err != nil {
		return nil, err
	}
	ids := []uint64{}
	for rows.Next() {
		var id uint64
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	candidates := []AdministratorRetirementCandidate{}
	for _, id := range ids {
		candidate := AdministratorRetirementCandidate{KnowledgeBase: id}
		var space uint64
		// 显式先锁知识库，不依赖JOIN优化器决定加锁顺序。
		if err = tx.QueryRowContext(ctx, `SELECT space_id,lifecycle FROM knowledge_base WHERE id=? FOR UPDATE`, id).Scan(&space, &candidate.Lifecycle); err != nil {
			return nil, err
		}
		if candidate.Lifecycle != "ACTIVE" && candidate.Lifecycle != "CLOSING" {
			continue
		}
		var role string
		err = tx.QueryRowContext(ctx, `SELECT role,state FROM knowledge_base_member WHERE knowledge_base_id=? AND user_id=? FOR UPDATE`, id, user).Scan(&role, &candidate.MemberState)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
````

## 第 4 步：LockAdministratorRetirementCandidates（接着实现）

插入位置：第 3 步代码之后，紧接追加；完成稿约第 62 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>role != &quot;ADMIN&quot; &#124;&#124; (candidate.MemberState != &quot;ACTIVE&quot; &amp;&amp; candidate.MemberState != &quot;REVOKING&quot;)</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT COALESCE(organization_id,0),COALESCE(account_owner_id,0) FROM space WHERE id=? FOR SHARE`, space).Scan(&amp;candidate.Organization, &amp;candidate.PersonalOwner)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT COALESCE(organization_id,0),COALESCE(account_owner_id,0) FROM space WHERE id=? FOR SHARE`, space)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return candidates, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=2335:2910 -->
````go
		if err != nil {
			return nil, err
		}
		if role != "ADMIN" || (candidate.MemberState != "ACTIVE" && candidate.MemberState != "REVOKING") {
			continue
		}
		if err = tx.QueryRowContext(ctx, `SELECT COALESCE(organization_id,0),COALESCE(account_owner_id,0) FROM space WHERE id=? FOR SHARE`, space).Scan(&candidate.Organization, &candidate.PersonalOwner); err != nil {
			return nil, err
		}
		if err = lockAdministratorRetirementFacts(ctx, tx, user, &candidate); err != nil {
			return nil, err
		}
		candidates = append(candidates, candidate)
	}
	return candidates, nil
}

````

## 第 5 步：lockAdministratorRetirementFacts

插入位置：第 4 步代码之后，紧接追加；完成稿约第 79 行。

本步声明：`func lockAdministratorRetirementFacts(ctx context.Context, tx *sql.Tx, user uint64, candidate *AdministratorRetirementCandidate) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryContext(ctx, `SELECT user_id FROM knowledge_base_member 		WHERE knowledge_base_id=? AND role=&#x27;ADMIN&#x27; AND state=&#x27;ACTIVE&#x27; ORDER BY user_id FOR UPDATE`, candidate.KnowledgeBase)</code>；<code>tx.QueryContext(ctx, `SELECT user_id,operation_id,state FROM admin_retirement 		WHERE knowledge_base_id=? ORDER BY user_id FOR UPDATE`, candidate.KnowledgeBase)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryContext</code> |
| <code>user</code> | <code>uint64</code> | 在此声明中的实际使用：<code>administrator == user</code> |
| <code>candidate</code> | <code>*AdministratorRetirementCandidate</code> | 在此声明中的实际使用：<code>candidate.KnowledgeBase</code>；<code>candidate.ReservationOperation</code>；<code>candidate.ReservationState</code>；<code>candidate.Available</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 执行 <code>tx.QueryContext(ctx, `SELECT user_id FROM knowledge_base_member 		WHERE knowledge_base_id=? AND role=&#x27;ADMIN&#x27; AND state=&#x27;ACTIVE&#x27; ORDER BY user_id FOR UPDATE`, candidate.KnowledgeBase)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 执行 <code>rows.Scan(&amp;administrator)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryContext(ctx, `SELECT user_id,operation_id,state FROM admin_retirement 		WHERE knowledge_base_id=? ORDER BY user_id FOR UPDATE`, candidate.KnowledgeBase)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 执行 <code>rows.Scan(&amp;administrator, &amp;operation, &amp;state)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>state == &quot;RESERVED&quot;</code>。
- 检查 <code>administrator == user</code>。

<!-- write-step: 5 bytes=2910:4409 -->
````go
func lockAdministratorRetirementFacts(ctx context.Context, tx *sql.Tx, user uint64, candidate *AdministratorRetirementCandidate) error {
	// 全部采用当前锁定读：即使外层事务此前已建立RR快照，等待知识库锁后也必须
	// 看见前一事务刚提交的预留。普通COUNT快照读在这里会错误放行并发退休。
	rows, err := tx.QueryContext(ctx, `SELECT user_id FROM knowledge_base_member
		WHERE knowledge_base_id=? AND role='ADMIN' AND state='ACTIVE' ORDER BY user_id FOR UPDATE`, candidate.KnowledgeBase)
	if err != nil {
		return err
	}
	available := map[uint64]struct{}{}
	for rows.Next() {
		var administrator uint64
		if err = rows.Scan(&administrator); err != nil {
			rows.Close()
			return err
		}
		available[administrator] = struct{}{}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	rows, err = tx.QueryContext(ctx, `SELECT user_id,operation_id,state FROM admin_retirement
		WHERE knowledge_base_id=? ORDER BY user_id FOR UPDATE`, candidate.KnowledgeBase)
	if err != nil {
		return err
	}
	for rows.Next() {
		var administrator uint64
		var operation, state string
		if err = rows.Scan(&administrator, &operation, &state); err != nil {
			rows.Close()
			return err
		}
		if state == "RESERVED" {
			delete(available, administrator)
		}
		if administrator == user {
			candidate.ReservationOperation = operation
			candidate.ReservationState = state
		}
	}
	err = rows.Err()
	rows.Close()
	candidate.Available = len(available)
````

## 第 6 步：lockAdministratorRetirementFacts（接着实现）

插入位置：第 5 步代码之后，紧接追加；完成稿约第 124 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=4409:4424 -->
````go
	return err
}

````

## 第 7 步：ReserveAdministratorRetirement

插入位置：第 6 步代码之后，紧接追加；完成稿约第 127 行。

ReserveAdministratorRetirement 只负责预留事实；是否允许退休及最后管理员规则由用例层判断。
知识库锁是成员/预留行的前置锁，重复同号只返回成功，不能覆盖另一未完成操作。

本步声明：`func ReserveAdministratorRetirement(ctx context.Context, tx *sql.Tx, kb, user uint64, operation string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>tx.QueryRowContext(ctx, &quot;SELECT id FROM knowledge_base WHERE id=? FOR UPDATE&quot;, kb)</code>；<code>tx.QueryRowContext(ctx, `SELECT operation_id,state FROM admin_retirement 		WHERE knowledge_base_id=? AND user_id=? FOR UPDATE`, kb, user)</code>；<code>tx.ExecContext(ctx, `INSERT INTO admin_retirement(knowledge_base_id,user_id,operation_id,state) 			VALUES(?,?,?,&#x27;RESERVED&#x27;)`, kb, user, operation)</code>；<code>tx.ExecContext(ctx, `UPDATE admin_retirement SET operation_id=?,state=&#x27;RESERVED&#x27;,created_at=UTC_TIMESTAMP(6) 		WHERE knowledge_base_id=? AND user_id=? AND state IN (&#x27;CANCELLED&#x27;,&#x27;APPLIED&#x27;)`, operation, kb, user)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>tx.QueryRowContext</code>；<code>tx.ExecContext</code> |
| <code>kb</code> | <code>uint64</code> | 在此声明中的实际使用：<code>kb == 0</code>；<code>tx.QueryRowContext(ctx, &quot;SELECT id FROM knowledge_base WHERE id=? FOR UPDATE&quot;, kb)</code>；<code>tx.QueryRowContext(ctx, `SELECT operation_id,state FROM admin_retirement 		WHERE knowledge_base_id=? AND user_id=? FOR UPDATE`, kb, user)</code>；<code>tx.ExecContext(ctx, `INSERT INTO admin_retirement(knowledge_base_id,user_id,operation_id,state) 			VALUES(?,?,?,&#x27;RESERVED&#x27;)`, kb, user, operation)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>user</code> | <code>uint64</code> | 在此声明中的实际使用：<code>user == 0</code>；<code>tx.QueryRowContext(ctx, `SELECT operation_id,state FROM admin_retirement 		WHERE knowledge_base_id=? AND user_id=? FOR UPDATE`, kb, user)</code>；<code>tx.ExecContext(ctx, `INSERT INTO admin_retirement(knowledge_base_id,user_id,operation_id,state) 			VALUES(?,?,?,&#x27;RESERVED&#x27;)`, kb, user, operation)</code>；<code>tx.ExecContext(ctx, `UPDATE admin_retirement SET operation_id=?,state=&#x27;RESERVED&#x27;,created_at=UTC_TIMESTAMP(6) 		WHERE knowledge_base_id=? AND user_id=? AND state IN (&#x27;CANCELLED&#x27;,&#x27;APPLIED&#x27;)`, operation, kb, user)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>domain.ValidOperationID(operation)</code>；<code>tx.ExecContext(ctx, `INSERT INTO admin_retirement(knowledge_base_id,user_id,operation_id,state) 			VALUES(?,?,?,&#x27;RESERVED&#x27;)`, kb, user, operation)</code>；<code>existing == operation</code>；<code>tx.ExecContext(ctx, `UPDATE admin_retirement SET operation_id=?,state=&#x27;RESERVED&#x27;,created_at=UTC_TIMESTAMP(6) 		WHERE knowledge_base_id=? AND user_id=? AND state IN (&#x27;CANCELLED&#x27;,&#x27;APPLIED&#x27;)`, operation, kb, user)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;ADMIN_RETIREMENT_BINDING_INVALID&quot;)</code>；<code>err</code>；<code>nil</code>；<code>domain.Violation(domain.StateConflict, &quot;KB_ADMIN_RETIREMENT_RESERVED&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>kb == 0 &#124;&#124; user == 0 &#124;&#124; !domain.ValidOperationID(operation)</code>。条件成立时执行 <code>return errors.New(&quot;ADMIN_RETIREMENT_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT id FROM knowledge_base WHERE id=? FOR UPDATE&quot;, kb).Scan(&amp;locked)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, &quot;SELECT id FROM knowledge_base WHERE id=? FOR UPDATE&quot;, kb)</code>。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT operation_id,state FROM admin_retirement 		WHERE knowledge_base_id=? AND user_id=? FOR UPDATE`, kb, user).Scan(&amp;existing, &amp;state)</code>。查询列与接收变量按位置对应，调换顺序会读错数据。
- 执行 <code>tx.QueryRowContext(ctx, `SELECT operation_id,state FROM admin_retirement 		WHERE knowledge_base_id=? AND user_id=? FOR UPDATE`, kb, user)</code>。
- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。
- 执行 <code>tx.ExecContext(ctx, `INSERT INTO admin_retirement(knowledge_base_id,user_id,operation_id,state) 			VALUES(?,?,?,&#x27;RESERVED&#x27;)`, kb, user, operation)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>state == &quot;RESERVED&quot;</code>。
- 检查 <code>existing == operation</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 此处通过 <code>return domain.Violation(domain.StateConflict, &quot;KB_ADMIN_RETIREMENT_RESERVED&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>state != &quot;CANCELLED&quot; &amp;&amp; state != &quot;APPLIED&quot;</code>。条件成立时执行 <code>return errors.New(&quot;ADMIN_RETIREMENT_STATE_INVALID&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.ExecContext(ctx, `UPDATE admin_retirement SET operation_id=?,state=&#x27;RESERVED&#x27;,created_at=UTC_TIMESTAMP(6) 		WHERE knowledge_base_id=? AND user_id=? AND state IN (&#x27;CANCELLED&#x27;,&#x27;APPLIED&#x27;)`, operation, kb, user)</code>。执行写入后仍需检查错误；条件更新的影响行数与数据库错误是两个不同的结果。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=4424:5994 -->
````go
// ReserveAdministratorRetirement 只负责预留事实；是否允许退休及最后管理员规则由用例层判断。
// 知识库锁是成员/预留行的前置锁，重复同号只返回成功，不能覆盖另一未完成操作。
func ReserveAdministratorRetirement(ctx context.Context, tx *sql.Tx, kb, user uint64, operation string) error {
	if kb == 0 || user == 0 || !domain.ValidOperationID(operation) {
		return errors.New("ADMIN_RETIREMENT_BINDING_INVALID")
	}
	var locked uint64
	if err := tx.QueryRowContext(ctx, "SELECT id FROM knowledge_base WHERE id=? FOR UPDATE", kb).Scan(&locked); err != nil {
		return err
	}
	var existing, state string
	err := tx.QueryRowContext(ctx, `SELECT operation_id,state FROM admin_retirement
		WHERE knowledge_base_id=? AND user_id=? FOR UPDATE`, kb, user).Scan(&existing, &state)
	if errors.Is(err, sql.ErrNoRows) {
		_, err = tx.ExecContext(ctx, `INSERT INTO admin_retirement(knowledge_base_id,user_id,operation_id,state)
			VALUES(?,?,?,'RESERVED')`, kb, user, operation)
		return err
	}
	if err != nil {
		return err
	}
	if state == "RESERVED" {
		if existing == operation {
			return nil
		}
		return domain.Violation(domain.StateConflict, "KB_ADMIN_RETIREMENT_RESERVED")
	}
	if state != "CANCELLED" && state != "APPLIED" {
		return errors.New("ADMIN_RETIREMENT_STATE_INVALID")
	}
	_, err = tx.ExecContext(ctx, `UPDATE admin_retirement SET operation_id=?,state='RESERVED',created_at=UTC_TIMESTAMP(6)
		WHERE knowledge_base_id=? AND user_id=? AND state IN ('CANCELLED','APPLIED')`, operation, kb, user)
	return err
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/repository/document/admin_retirement.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/13-account-retirement.md)。
