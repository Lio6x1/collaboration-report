# 分步编写 `services/account/internal/logic/account/governance_service.go`

先按 [A04 · 原操作回执、授权证明与平台治理任免](../lessons/account/04-receipts-governance.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/060-governance-service.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

编排本机运维平台管理员任免，并把治理资格纳入身份准入来源。

RETAINED_GOVERNANCE 需要独立platform-admin来源，普通ACTIVE_ACCOUNT不足以管理保留数据。

写之前先按这条执行路线理解各段的连接：

1. GovernanceOperator.Change 锁目标user，核对原号/指纹与expected revision，再授予或登记持久撤销。
2. Identity.admissionSources 在指定用途附加platform-admin:user的epoch。
3. finishGovernance 在真实Doc ACK后重锁角色与任务ownership，关闭角色并结束原回执。

本文件需要守住的边界：

- operation_id重试必须携带原参数；换expected revision不能覆盖原请求。
- 没有公开HTTP任免接口，部署凭据入口不能变成浏览器权限。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/account/internal/logic/account' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/account/internal/logic/account/governance_service.go') -and (Get-Item -LiteralPath 'services/account/internal/logic/account/governance_service.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/account/internal/logic/account/governance_service.go')) { New-Item -ItemType File -Path 'services/account/internal/logic/account/governance_service.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:489 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package account

import (
	"bytes"
	domain "collaboration/account/internal/domain/account"
	repository "collaboration/account/internal/repository/account"
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
)

// governance_identity.go：保留原实现，按业务合并文件。

````

## 第 2 步：Identity.admissionSources

插入位置：第 1 步代码之后，紧接追加；完成稿约第 20 行。

冻结RPC已经绑定purpose/Target/原号；基础ACTIVE_ACCOUNT不能代替平台资格。
专用purpose在Acquire和Revalidate都核真实角色，并把可撤权代次写入同一准入。

本步声明：`func (s *Identity) admissionSources(ctx context.Context, tx *sql.Tx, actor uint64, session []byte, capability pb.IdentityCapability, target *pb.IdentityTarget, purpose string) ([]*pb.SourceEpoch, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.currentSources(ctx, tx, actor, session, capability, target)</code>；<code>s.repository.LockGovernanceGrant(ctx, tx, actor)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>s.currentSources(ctx, tx, actor, session, capability, target)</code>；<code>s.repository.LockGovernanceGrant(ctx, tx, actor)</code> |
| <code>actor</code> | <code>uint64</code> | 在此声明中的实际使用：<code>target.UserId != actor</code>；<code>s.currentSources(ctx, tx, actor, session, capability, target)</code>；<code>s.repository.LockGovernanceGrant(ctx, tx, actor)</code>；<code>strconv.FormatUint(actor, 10)</code> |
| <code>session</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>s.currentSources(ctx, tx, actor, session, capability, target)</code> |
| <code>capability</code> | <code>pb.IdentityCapability</code> | 在此声明中的实际使用：<code>capability != pb.IdentityCapability_ACTIVE_ACCOUNT</code>；<code>s.currentSources(ctx, tx, actor, session, capability, target)</code> |
| <code>target</code> | <code>*pb.IdentityTarget</code> | 在此声明中的实际使用：<code>target == nil</code>；<code>target.Kind</code>；<code>target.UserId</code>；<code>target.OrganizationId</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>purpose</code> | <code>string</code> | 在此声明中的实际使用：<code>purpose == &quot;RETAINED_GOVERNANCE&quot;</code>；<code>purpose != &quot;RETAINED_GOVERNANCE&quot;</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 实际返回表达式：<code>nil</code>；<code>sources</code>；<code>append(sources, &amp;pb.SourceEpoch{ScopeKey: &quot;platform-admin:&quot; + strconv.FormatUint(actor, 10), Epoch: epoch})</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>identityDenied()</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>purpose == &quot;RETAINED_GOVERNANCE&quot; &amp;&amp; (capability != pb.IdentityCapability_ACTIVE_ACCOUNT &#124;&#124; target == nil &#124;&#124; target.Kind != pb.IdentityTarget_USER &#124;&#124; target.UserId != actor &#124;&#124; targe …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return nil, identityDenied()</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; purpose != &quot;RETAINED_GOVERNANCE&quot;</code>。条件成立时执行 <code>return sources, err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; state != &quot;ACTIVE&quot; &#124;&#124; epoch == 0</code>。条件成立时执行 <code>return nil, identityDenied()</code>。当前函数在这里结束。
- 此处通过 <code>return append(sources, &amp;pb.SourceEpoch{ScopeKey: &quot;platform-admin:&quot; + strconv.FormatUint(actor, 10), Epoch: epoch}), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=489:1669 -->
````go
// 冻结RPC已经绑定purpose/Target/原号；基础ACTIVE_ACCOUNT不能代替平台资格。
// 专用purpose在Acquire和Revalidate都核真实角色，并把可撤权代次写入同一准入。
func (s *Identity) admissionSources(ctx context.Context, tx *sql.Tx, actor uint64, session []byte, capability pb.IdentityCapability, target *pb.IdentityTarget, purpose string) ([]*pb.SourceEpoch, error) {
	if purpose == "RETAINED_GOVERNANCE" && (capability != pb.IdentityCapability_ACTIVE_ACCOUNT || target == nil || target.Kind != pb.IdentityTarget_USER || target.UserId != actor || target.OrganizationId != 0 || target.SpaceId != 0) {
		return nil, identityDenied()
	}
	sources, err := s.currentSources(ctx, tx, actor, session, capability, target)
	if err != nil || purpose != "RETAINED_GOVERNANCE" {
		return sources, err
	}
	state, epoch, err := s.repository.LockGovernanceGrant(ctx, tx, actor)
	if err != nil || state != "ACTIVE" || epoch == 0 {
		return nil, identityDenied()
	}
	return append(sources, &pb.SourceEpoch{ScopeKey: "platform-admin:" + strconv.FormatUint(actor, 10), Epoch: epoch}), nil
}

// governance_operator.go：保留原实现，按业务合并文件。

````

## 第 3 步：GovernanceOperator

插入位置：第 2 步代码之后，紧接追加；完成稿约第 39 行。

仅由持有Account部署凭据的本机运维入口装配，不注册公共HTTP任免接口。
普通注册账号没有隐含管理员；运维原号/目标/预期修订和实际资格都有持久审计。

本步声明：`type GovernanceOperator struct{ db *sql.DB }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |

<!-- write-step: 3 bytes=1669:1928 -->
````go
// 仅由持有Account部署凭据的本机运维入口装配，不注册公共HTTP任免接口。
// 普通注册账号没有隐含管理员；运维原号/目标/预期修订和实际资格都有持久审计。
type GovernanceOperator struct{ db *sql.DB }

````

## 第 4 步：NewGovernanceOperator

插入位置：第 3 步代码之后，紧接追加；完成稿约第 43 行。

本步声明：`func NewGovernanceOperator(db *sql.DB) *GovernanceOperator`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*GovernanceOperator</code> | 使用 <code>GovernanceOperator</code> 类型；其定义处的职责：仅由持有Account部署凭据的本机运维入口装配，不注册公共HTTP任免接口。 普通注册账号没有隐含管理员；运维原号/目标/预期修订和实际资格都有持久审计。<br>实际返回表达式：<code>&amp;GovernanceOperator{db: db}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;GovernanceOperator{db: db}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=1928:2027 -->
````go
func NewGovernanceOperator(db *sql.DB) *GovernanceOperator { return &GovernanceOperator{db: db} }

````

## 第 5 步：GovernanceOperatorInput

插入位置：第 4 步代码之后，紧接追加；完成稿约第 45 行。

本步声明：`type GovernanceOperatorInput struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Operation</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;operation_id&quot;`</code> |
| <code>User</code> | <code>uint64</code> | 序列化/校验标记：<code>`json:&quot;user_id&quot;`</code> |
| <code>Action</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;action&quot;`</code> |
| <code>Expected</code> | <code>uint64</code> | 序列化/校验标记：<code>`json:&quot;expected_revision&quot;`</code> |

<!-- write-step: 5 bytes=2027:2222 -->
````go
type GovernanceOperatorInput struct {
	Operation string `json:"operation_id"`
	User      uint64 `json:"user_id"`
	Action    string `json:"action"`
	Expected  uint64 `json:"expected_revision"`
}

````

## 第 6 步：GovernanceOperator.Change

插入位置：第 5 步代码之后，紧接追加；完成稿约第 52 行。

本步声明：`func (s *GovernanceOperator) Change(ctx context.Context, input GovernanceOperatorInput) (json.RawMessage, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>；<code>repository.GovernanceUserActive(ctx, tx, input.User)</code>；<code>repository.NewMutation().Receipt(ctx, tx, input.Operation)</code>；<code>repository.GovernanceGrantRow(ctx, tx, input.User, true)</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |
| <code>input</code> | <code>GovernanceOperatorInput</code> | 在此声明中的实际使用：<code>input.User</code>；<code>input.Operation</code>；<code>input.Action</code>；<code>json.Marshal(input)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>json.RawMessage</code> | 尚未按业务结构解码的JSON原始字节；继续使用前需按当前协议核验字段和类型。<br>实际返回表达式：<code>nil</code>；<code>receipt.Result</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.RuleViolation{Kind: domain.InvalidRequest, Code: &quot;PLATFORM_GOVERNANCE_INPUT_INVALID&quot;}</code>；<code>err</code>；<code>errors.New(&quot;PLATFORM_GOVERNANCE_USER_NOT_ACTIVE&quot;)</code>；<code>errors.New(&quot;PLATFORM_GOVERNANCE_OPERATION_CONFLICT&quot;)</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>input.User == 0 &#124;&#124; !domain.ValidOperationID(input.Operation) &#124;&#124; (input.Action != &quot;GRANT&quot; &amp;&amp; input.Action != &quot;REVOKE&quot;)</code>。条件成立时执行 <code>return nil, domain.RuleViolation{Kind: domain.InvalidRequest, Code: &quot;PLATFORM_GOVERNANCE_INPUT_INVALID&quot;}</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, &amp;sql.TxOptions{Isolation: sql.LevelReadCommitted})</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil &#124;&#124; !active</code>。条件成立时执行 <code>return nil, errors.New(&quot;PLATFORM_GOVERNANCE_USER_NOT_ACTIVE&quot;)</code>。当前函数在这里结束。
- 检查 <code>err == nil</code>。
- 检查 <code>receipt.ActorID != &quot;0&quot; &#124;&#124; receipt.Kind != &quot;PLATFORM_GOVERNANCE_OPERATOR&quot; &#124;&#124; !bytes.Equal(receipt.Fingerprint, fingerprint&#91;:&#93;)</code>。条件成立时执行 <code>return nil, errors.New(&quot;PLATFORM_GOVERNANCE_OPERATION_CONFLICT&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return receipt.Result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>grant.Revision != input.Expected &#124;&#124; grant.State == &quot;REVOKING&quot;</code>。条件成立时执行 <code>return nil, errors.New(&quot;PLATFORM_GOVERNANCE_REVISION_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>grant.Epoch == ^uint64(0) &#124;&#124; grant.Revision == ^uint64(0)</code>。条件成立时执行 <code>return nil, errors.New(&quot;PLATFORM_GOVERNANCE_EPOCH_EXHAUSTED&quot;)</code>。当前函数在这里结束。
- 检查 <code>grant.State != &quot;ACTIVE&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。

<!-- write-step: 6 bytes=2222:4108 -->
````go
func (s *GovernanceOperator) Change(ctx context.Context, input GovernanceOperatorInput) (json.RawMessage, error) {
	if input.User == 0 || !domain.ValidOperationID(input.Operation) || (input.Action != "GRANT" && input.Action != "REVOKE") {
		return nil, domain.RuleViolation{Kind: domain.InvalidRequest, Code: "PLATFORM_GOVERNANCE_INPUT_INVALID"}
	}
	raw, _ := json.Marshal(input)
	fingerprint := sha256.Sum256(raw)
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	active, err := repository.GovernanceUserActive(ctx, tx, input.User)
	if err != nil || !active {
		return nil, errors.New("PLATFORM_GOVERNANCE_USER_NOT_ACTIVE")
	}
	// 持目标user锁序列化同一角色变更；原号重取不能用后来修订改写原请求。
	receipt, err := repository.NewMutation().Receipt(ctx, tx, input.Operation)
	if err == nil {
		if receipt.ActorID != "0" || receipt.Kind != "PLATFORM_GOVERNANCE_OPERATOR" || !bytes.Equal(receipt.Fingerprint, fingerprint[:]) {
			return nil, errors.New("PLATFORM_GOVERNANCE_OPERATION_CONFLICT")
		}
		return receipt.Result, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	grant, err := repository.GovernanceGrantRow(ctx, tx, input.User, true)
	if errors.Is(err, sql.ErrNoRows) {
		grant = repository.GovernanceGrant{}
	} else if err != nil {
		return nil, err
	}
	if grant.Revision != input.Expected || grant.State == "REVOKING" {
		return nil, errors.New("PLATFORM_GOVERNANCE_REVISION_CHANGED")
	}
	if grant.Epoch == ^uint64(0) || grant.Revision == ^uint64(0) {
		return nil, errors.New("PLATFORM_GOVERNANCE_EPOCH_EXHAUSTED")
	}
	state := "SUCCEEDED"
	switch input.Action {
	case "GRANT":
		if grant.State != "ACTIVE" {
			if err = repository.GrantGovernance(ctx, tx, input.User, input.Operation, grant); err != nil {
				return nil, err
			}
		}
````

## 第 7 步：GovernanceOperator.Change（接着实现）

插入位置：第 6 步代码之后，紧接追加；完成稿约第 98 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>grant.State == &quot;ACTIVE&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, errors.New(&quot;PLATFORM_GOVERNANCE_COMMIT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=4108:5004 -->
````go
	case "REVOKE":
		if grant.State == "ACTIVE" {
			if err = repository.RevokeGovernance(ctx, tx, input.User, input.Operation); err != nil {
				return nil, err
			}
			closure := domain.SessionClosure{Kind: "PLATFORM_GOVERNANCE_REVOKE", UserID: strconv.FormatUint(input.User, 10), Scope: "platform-admin:" + strconv.FormatUint(input.User, 10), Epoch: strconv.FormatUint(grant.Epoch, 10)}
			if err = EnqueueSourceClosure(ctx, tx, input.Operation, "0", closure); err != nil {
				return nil, err
			}
			state = "PREPARING"
		}
	}
	result, _ := json.Marshal(map[string]string{"operation_id": input.Operation, "state": state})
	if err = repository.InsertGovernanceOperatorReceipt(ctx, tx, input.Operation, fingerprint, raw, result, state); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, errors.New("PLATFORM_GOVERNANCE_COMMIT_UNKNOWN")
	}
	return result, nil
}

````

## 第 8 步：GovernanceOperator.Current

插入位置：第 7 步代码之后，紧接追加；完成稿约第 120 行。

本步声明：`func (s *GovernanceOperator) Current(ctx context.Context, user uint64) (map[string]string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.GovernanceGrantRow(ctx, s.db, user, false)</code> |
| <code>user</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.GovernanceGrantRow(ctx, s.db, user, false)</code>；<code>strconv.FormatUint(user, 10)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>map&#91;string&#93;string</code> | 实际返回表达式：<code>map&#91;string&#93;string{&quot;user_id&quot;: strconv.FormatUint(user, 10), &quot;state&quot;: &quot;CLOSED&quot;, &quot;revision&quot;: &quot;0&quot;}</code>；<code>nil</code>；<code>map&#91;string&#93;string{&quot;user_id&quot;: strconv.FormatUint(user, 10), &quot;state&quot;: grant.State, &quot;revision&quot;: strconv.FormatUint(grant.Revision, 10), &quot;epoch&quot;: strconv.FormatUint(grant.Epoch, 10)}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return map&#91;string&#93;string{&quot;user_id&quot;: strconv.FormatUint(user, 10), &quot;state&quot;: &quot;CLOSED&quot;, &quot;revision&quot;: &quot;0&quot;}, nil</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return map&#91;string&#93;string{&quot;user_id&quot;: strconv.FormatUint(user, 10), &quot;state&quot;: grant.State, &quot;revision&quot;: strconv.FormatUint(grant.Revision, 10), &quot;epoch&quot;: strconv.FormatUint(grant.Epoch, 10)}, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=5004:5554 -->
````go
func (s *GovernanceOperator) Current(ctx context.Context, user uint64) (map[string]string, error) {
	grant, err := repository.GovernanceGrantRow(ctx, s.db, user, false)
	if errors.Is(err, sql.ErrNoRows) {
		return map[string]string{"user_id": strconv.FormatUint(user, 10), "state": "CLOSED", "revision": "0"}, nil
	}
	if err != nil {
		return nil, err
	}
	return map[string]string{"user_id": strconv.FormatUint(user, 10), "state": grant.State, "revision": strconv.FormatUint(grant.Revision, 10), "epoch": strconv.FormatUint(grant.Epoch, 10)}, nil
}

````

## 第 9 步：GovernanceOperator.Ready

插入位置：第 8 步代码之后，紧接追加；完成稿约第 131 行。

本步声明：`func (s *GovernanceOperator) Ready(ctx context.Context) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.GovernanceRecoveryReady(ctx, s.db)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>repository.GovernanceRecoveryReady(ctx, s.db)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return repository.GovernanceRecoveryReady(ctx, s.db)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=5554:5749 -->
````go
func (s *GovernanceOperator) Ready(ctx context.Context) error {
	return repository.GovernanceRecoveryReady(ctx, s.db)
}

// governance_revocation.go：保留原实现，按业务合并文件。

````

## 第 10 步：Closure.finishGovernance

插入位置：第 9 步代码之后，紧接追加；完成稿约第 137 行。

本步声明：`func (s *Closure) finishGovernance(ctx context.Context, task domain.ClosureTask, instance string, binding domain.SessionClosure, digest []byte)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, nil)</code>；<code>repository.GovernanceGrantRow(ctx, tx, user, true)</code>；<code>s.repository.LockOwnership(ctx, tx, task.ID)</code>；<code>repository.CompleteGovernanceRevocation(ctx, tx, user, epoch+1, task.OperationID)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>domain.ClosureTask</code> | 使用 <code>domain.ClosureTask</code> 类型；其定义处的职责：定义退出、凭据/组织来源关闭的持久任务载荷与会话关闭准备值。<br>在此声明中的实际使用：<code>task.OperationID</code>；<code>task.ID</code>；<code>s.repository.FinishBase(ctx, tx, task, binding, nil, digest, terminal)</code> |
| <code>instance</code> | <code>string</code> | 在此声明中的实际使用：<code>ownership.Instance != instance</code> |
| <code>binding</code> | <code>domain.SessionClosure</code> | 使用 <code>domain.SessionClosure</code> 类型；其定义处的职责：SessionClosure是持久任务的固定绑定。后续重试不能改写原scope或关闭代次。<br>在此声明中的实际使用：<code>binding.UserID</code>；<code>binding.Scope</code>；<code>binding.Epoch</code>；<code>s.repository.FinishBase(ctx, tx, task, binding, nil, digest, terminal)</code> |
| <code>digest</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>s.repository.FinishBase(ctx, tx, task, binding, nil, digest, terminal)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil &#124;&#124; user == 0 &#124;&#124; strconv.FormatUint(user, 10) != binding.UserID &#124;&#124; binding.Scope != &quot;platform-admin:&quot;+binding.UserID</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; epoch == 0 &#124;&#124; epoch == ^uint64(0) &#124;&#124; strconv.FormatUint(epoch, 10) != binding.Epoch</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil &#124;&#124; grant.State != &quot;REVOKING&quot; &#124;&#124; grant.Epoch != epoch+1 &#124;&#124; grant.Operation != task.OperationID</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; ownership.Instance != instance &#124;&#124; ownership.State != &quot;RUNNING&quot;</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 10 bytes=5749:7226 -->
````go
func (s *Closure) finishGovernance(ctx context.Context, task domain.ClosureTask, instance string, binding domain.SessionClosure, digest []byte) {
	user, err := strconv.ParseUint(binding.UserID, 10, 64)
	if err != nil || user == 0 || strconv.FormatUint(user, 10) != binding.UserID || binding.Scope != "platform-admin:"+binding.UserID {
		return
	}
	epoch, err := strconv.ParseUint(binding.Epoch, 10, 64)
	if err != nil || epoch == 0 || epoch == ^uint64(0) || strconv.FormatUint(epoch, 10) != binding.Epoch {
		return
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer tx.Rollback()
	// 先角色，再任务ownership，与任免入口保持同一锁顺序；真实Doc ACK已经核验。
	grant, err := repository.GovernanceGrantRow(ctx, tx, user, true)
	if err != nil || grant.State != "REVOKING" || grant.Epoch != epoch+1 || grant.Operation != task.OperationID {
		return
	}
	ownership, err := s.repository.LockOwnership(ctx, tx, task.ID)
	if err != nil || ownership.Instance != instance || ownership.State != "RUNNING" {
		return
	}
	if err = repository.CompleteGovernanceRevocation(ctx, tx, user, epoch+1, task.OperationID); err != nil {
		return
	}
	terminal, _ := json.Marshal(dto.Operation{OperationId: task.OperationID, State: "SUCCEEDED"})
	if err = s.repository.FinishBase(ctx, tx, task, binding, nil, digest, terminal); err != nil {
		return
	}
	// 未知提交仍重取原角色/回执；绝不按租约结束补写成功。
	_ = tx.Commit()
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/account/internal/logic/account/governance_service.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/account/04-receipts-governance.md)。
