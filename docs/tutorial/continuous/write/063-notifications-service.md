# 分步编写 `services/account/internal/logic/account/notifications_service.go`

先按 [A05 · 通知收件资格、事件投递与确认](../lessons/account/05-notification-delivery.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/063-notifications-service.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

判断通知收件资格并返回5秒有效的来源快照，解释离开原事务的实际状态。

锁内事实释放后会改变，因此返回全部撤权来源与短期限，由Doc在提交投递前锁定它们。

写之前先按这条执行路线理解各段的连接：

1. Check 按升序锁user再锁organization/member，ACTIVE且代次合法才Eligible。
2. 组装identity、credential以及成员/admin/owner来源，提交前确定ExpiresUnixMS。
3. DepartureStatus 找到原receipt为COMMITTED；无回执且原成员仍ACTIVE同epoch为ROLLED_BACK，否则SUPERSEDED。

本文件需要守住的边界：

- 数据库故障必须报未知，不能凭网络超时返回ROLLED_BACK。
- 传输耗时不能刷新快照期限。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/account/internal/logic/account' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/account/internal/logic/account/notifications_service.go') -and (Get-Item -LiteralPath 'services/account/internal/logic/account/notifications_service.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/account/internal/logic/account/notifications_service.go')) { New-Item -ItemType File -Path 'services/account/internal/logic/account/notifications_service.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:397 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package account

import (
	domain "collaboration/account/internal/domain/account"
	repository "collaboration/account/internal/repository/account"
	"collaboration/platform"
	"context"
	"database/sql"
	"errors"
	"strconv"
	"time"
)

// departure_status.go：保留原实现，按业务合并文件。

````

## 第 2 步：Organizations.DepartureStatus

插入位置：第 1 步代码之后，紧接追加；完成稿约第 17 行。

DepartureStatus不根据超时猜测准备结果；同一组织/member锁保证读取原事务的持久事实。

本步声明：`func (s *Organizations) DepartureStatus(ctx context.Context, organization, user, operation, expectedEpoch string) (string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, nil)</code>；<code>s.repository.LockDepartureOrganization(ctx, tx, organization)</code>；<code>s.repository.LockDepartureMember(ctx, tx, organization, user)</code>；<code>s.repository.LockDepartureReceipt(ctx, tx, operation)</code> |
| <code>organization</code> | <code>string</code> | 在此声明中的实际使用：<code>s.repository.LockDepartureOrganization(ctx, tx, organization)</code>；<code>s.repository.LockDepartureMember(ctx, tx, organization, user)</code> |
| <code>user</code> | <code>string</code> | 在此声明中的实际使用：<code>s.repository.LockDepartureMember(ctx, tx, organization, user)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>s.repository.LockDepartureReceipt(ctx, tx, operation)</code> |
| <code>expectedEpoch</code> | <code>string</code> | 在此声明中的实际使用：<code>epoch == expectedEpoch</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;&quot;</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>unknown</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;&quot;, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>s.repository.LockDepartureOrganization(ctx, tx, organization) != nil</code>。条件成立时执行 <code>return &quot;&quot;, unknown</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;&quot;, unknown</code>。当前函数在这里结束。
- 检查 <code>err == sql.ErrNoRows</code>。
- 检查 <code>state == &quot;ACTIVE&quot; &amp;&amp; epoch == expectedEpoch</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;&quot;, unknown</code>。当前函数在这里结束。
- 检查 <code>tx.Commit() != nil</code>。条件成立时执行 <code>return &quot;&quot;, unknown</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=397:1494 -->
````go
// DepartureStatus不根据超时猜测准备结果；同一组织/member锁保证读取原事务的持久事实。
func (s *Organizations) DepartureStatus(ctx context.Context, organization, user, operation, expectedEpoch string) (string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	unknown := domain.RuleViolation{Kind: domain.TemporarilyUnavailable, Code: "DEPARTURE_STATUS_UNKNOWN"}
	if s.repository.LockDepartureOrganization(ctx, tx, organization) != nil {
		return "", unknown
	}
	state, epoch, err := s.repository.LockDepartureMember(ctx, tx, organization, user)
	if err != nil {
		return "", unknown
	}
	err = s.repository.LockDepartureReceipt(ctx, tx, operation)
	result := "COMMITTED"
	if err == sql.ErrNoRows {
		if state == "ACTIVE" && epoch == expectedEpoch {
			result = "ROLLED_BACK"
		} else {
			result = "SUPERSEDED"
		}
	} else if err != nil {
		return "", unknown
	}
	if tx.Commit() != nil {
		return "", unknown
	}
	return result, nil
}

// notification_administrators.go：保留原实现，按业务合并文件。

````

## 第 3 步：NotificationRecipients.Administrators

插入位置：第 2 步代码之后，紧接追加；完成稿约第 51 行。

本步声明：`func (s *NotificationRecipients) Administrators(ctx context.Context, input platform.NotificationAdministratorsRequest) (platform.NotificationAdministratorsResponse, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.NotificationAdministrators(ctx, s.db, input.OrganizationID, input.After)</code> |
| <code>input</code> | <code>platform.NotificationAdministratorsRequest</code> | 使用 <code>platform.NotificationAdministratorsRequest</code> 类型；其定义处的职责：管理员目录仅产生候选人；投递前必须重新调用接收资格核验，不能把目录当授权凭据。<br>在此声明中的实际使用：<code>input.OrganizationID</code>；<code>input.After</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>platform.NotificationAdministratorsResponse</code> | 使用 <code>platform.NotificationAdministratorsResponse</code> 类型；其定义处的职责：集中定义跨服务清理、通知资格、草稿计量、组织关闭和转让存储测量协议。<br>实际返回表达式：<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.ErrNotificationAdministratorsInput</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>input.OrganizationID == 0</code>。条件成立时执行 <code>return result, domain.ErrNotificationAdministratorsInput</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>len(ids) &gt; domain.NotificationAdministratorPageSize</code>。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=1494:2390 -->
````go
func (s *NotificationRecipients) Administrators(ctx context.Context, input platform.NotificationAdministratorsRequest) (platform.NotificationAdministratorsResponse, error) {
	result := platform.NotificationAdministratorsResponse{OrganizationID: input.OrganizationID, UserIDs: []uint64{}}
	if input.OrganizationID == 0 {
		return result, domain.ErrNotificationAdministratorsInput
	}
	ids, err := repository.NotificationAdministrators(ctx, s.db, input.OrganizationID, input.After)
	if err != nil {
		// 不存在或已关闭组织可以返回空目录，数据库未知必须向上报错并重试。
		return result, err
	}
	if len(ids) > domain.NotificationAdministratorPageSize {
		ids = ids[:domain.NotificationAdministratorPageSize]
		result.NextAfter = ids[len(ids)-1]
	}
	result.UserIDs = ids
	return result, nil
}

// notification_recipients.go：保留原实现，按业务合并文件。

````

## 第 4 步：NotificationRecipients

插入位置：第 3 步代码之后，紧接追加；完成稿约第 71 行。

本步声明：`type NotificationRecipients struct{ db *sql.DB }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |

<!-- write-step: 4 bytes=2390:2440 -->
````go
type NotificationRecipients struct{ db *sql.DB }

````

## 第 5 步：NewNotificationRecipients

插入位置：第 4 步代码之后，紧接追加；完成稿约第 73 行。

本步声明：`func NewNotificationRecipients(db *sql.DB) *NotificationRecipients`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*NotificationRecipients</code> | 实际返回表达式：<code>&amp;NotificationRecipients{db: db}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;NotificationRecipients{db: db}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=2440:2552 -->
````go
func NewNotificationRecipients(db *sql.DB) *NotificationRecipients {
	return &NotificationRecipients{db: db}
}

````

## 第 6 步：NotificationRecipients.Check

插入位置：第 5 步代码之后，紧接追加；完成稿约第 77 行。

Check 不创建用户准入或伪造会话。Doc仅可在短期限内锁全部返回来源后提交投递。
当前角色变更关闭org-member，所有者转移关闭org-owner，密码变更关闭credential；
将来开放账号注销/组织关闭前，必须同样无条件广播identity/成员来源关闭高水位。

本步声明：`func (s *NotificationRecipients) Check(ctx context.Context, input platform.NotificationRecipientsRequest) (platform.NotificationRecipientsResponse, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, nil)</code>；<code>repository.NotificationRecipientUser(ctx, tx, id)</code>；<code>repository.NotificationRecipientOrg(ctx, tx, input.OrganizationID)</code>；<code>repository.NotificationRecipientMember(ctx, tx, input.OrganizationID, user.ID)</code> |
| <code>input</code> | <code>platform.NotificationRecipientsRequest</code> | 使用 <code>platform.NotificationRecipientsRequest</code> 类型；其定义处的职责：通知收件资格只在服务间传递，不是用户会话，也不能用于发起用户修改。<br>在此声明中的实际使用：<code>input.OrganizationID</code>；<code>input.UserIDs</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>platform.NotificationRecipientsResponse</code> | 使用 <code>platform.NotificationRecipientsResponse</code> 类型；其定义处的职责：集中定义跨服务清理、通知资格、草稿计量、组织关闭和转让存储测量协议。<br>实际返回表达式：<code>result</code>；<code>platform.NotificationRecipientsResponse{}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>errors.New(&quot;NOTIFICATION_IDENTITY_INCONSISTENT&quot;)</code>；<code>errors.New(&quot;NOTIFICATION_MEMBERSHIP_INCONSISTENT&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil &amp;&amp; err != sql.ErrNoRows</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>input.OrganizationID != 0</code>。
- 检查 <code>err != nil &amp;&amp; err != sql.ErrNoRows</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>item.Eligible &amp;&amp; (user.IdentityEpoch == 0 &#124;&#124; user.CredentialEpoch == 0)</code>。条件成立时执行 <code>return result, errors.New(&quot;NOTIFICATION_IDENTITY_INCONSISTENT&quot;)</code>。当前函数在这里结束。
- 检查 <code>input.OrganizationID != 0</code>。
- 检查 <code>err != nil &amp;&amp; err != sql.ErrNoRows</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>item.Eligible &amp;&amp; (member.Epoch == 0 &#124;&#124; (member.Role != &quot;ADMIN&quot; &amp;&amp; member.Role != &quot;MEMBER&quot;))</code>。条件成立时执行 <code>return result, errors.New(&quot;NOTIFICATION_MEMBERSHIP_INCONSISTENT&quot;)</code>。当前函数在这里结束。

<!-- write-step: 6 bytes=2552:4842 -->
````go
// Check 不创建用户准入或伪造会话。Doc仅可在短期限内锁全部返回来源后提交投递。
// 当前角色变更关闭org-member，所有者转移关闭org-owner，密码变更关闭credential；
// 将来开放账号注销/组织关闭前，必须同样无条件广播identity/成员来源关闭高水位。
func (s *NotificationRecipients) Check(ctx context.Context, input platform.NotificationRecipientsRequest) (platform.NotificationRecipientsResponse, error) {
	result := platform.NotificationRecipientsResponse{OrganizationID: input.OrganizationID, Recipients: []platform.NotificationRecipient{}}
	ids, err := domain.NotificationRecipientIDs(input.UserIDs)
	if err != nil {
		return result, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	users := make([]domain.NotificationRecipientIdentity, len(ids))
	for i, id := range ids {
		users[i], err = repository.NotificationRecipientUser(ctx, tx, id)
		if err != nil && err != sql.ErrNoRows {
			return result, err
		}
	}
	var organization domain.NotificationRecipientOrganization
	if input.OrganizationID != 0 {
		organization, err = repository.NotificationRecipientOrg(ctx, tx, input.OrganizationID)
		if err != nil && err != sql.ErrNoRows {
			return result, err
		}
	}
	for _, user := range users {
		item := platform.NotificationRecipient{UserID: user.ID, IdentityEpoch: user.IdentityEpoch, CredentialEpoch: user.CredentialEpoch, Sources: []platform.NotificationRecipientSource{}}
		item.Eligible = user.State == "ACTIVE"
		if item.Eligible && (user.IdentityEpoch == 0 || user.CredentialEpoch == 0) {
			return result, errors.New("NOTIFICATION_IDENTITY_INCONSISTENT")
		}
		if input.OrganizationID != 0 {
			member, err := repository.NotificationRecipientMember(ctx, tx, input.OrganizationID, user.ID)
			if err != nil && err != sql.ErrNoRows {
				return result, err
			}
			item.Role, item.MemberEpoch = member.Role, member.Epoch
			item.OrgOwner = organization.Owner == user.ID
			item.Eligible = item.Eligible && organization.State == "ACTIVE" && member.State == "ACTIVE"
			if item.Eligible && (member.Epoch == 0 || (member.Role != "ADMIN" && member.Role != "MEMBER")) {
				return result, errors.New("NOTIFICATION_MEMBERSHIP_INCONSISTENT")
			}
		}
````

## 第 7 步：NotificationRecipients.Check（接着实现）

插入位置：第 6 步代码之后，紧接追加；完成稿约第 123 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>item.Eligible</code>。
- 检查 <code>input.OrganizationID != 0</code>。
- 检查 <code>item.Role == &quot;ADMIN&quot;</code>。
- 检查 <code>item.OrgOwner</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return platform.NotificationRecipientsResponse{}, err</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=4842:6087 -->
````go
		if item.Eligible {
			userID := strconv.FormatUint(user.ID, 10)
			item.Sources = append(item.Sources,
				platform.NotificationRecipientSource{ScopeKey: "identity:" + userID, Epoch: item.IdentityEpoch},
				platform.NotificationRecipientSource{ScopeKey: "credential:" + userID, Epoch: item.CredentialEpoch})
			if input.OrganizationID != 0 {
				suffix := strconv.FormatUint(input.OrganizationID, 10) + ":" + userID
				item.Sources = append(item.Sources, platform.NotificationRecipientSource{ScopeKey: "org-member:" + suffix, Epoch: item.MemberEpoch})
				if item.Role == "ADMIN" {
					item.Sources = append(item.Sources, platform.NotificationRecipientSource{ScopeKey: "org-admin:" + suffix, Epoch: item.MemberEpoch})
				}
				if item.OrgOwner {
					item.Sources = append(item.Sources, platform.NotificationRecipientSource{ScopeKey: "org-owner:" + suffix, Epoch: item.MemberEpoch})
				}
			}
		}
		result.Recipients = append(result.Recipients, item)
	}
	// 在释放事实锁前确定期限，提交或传输耗时不能重新延长快照有效期。
	result.ExpiresUnixMS = time.Now().Add(5 * time.Second).UnixMilli()
	if err := tx.Commit(); err != nil {
		return platform.NotificationRecipientsResponse{}, err
	}
	return result, nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/account/internal/logic/account/notifications_service.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/account/05-notification-delivery.md)。
