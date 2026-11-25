# 分步编写 `services/account/internal/domain/account/notification_recipients.go`

先按 [A05 · 通知收件资格、事件投递与确认](../lessons/account/05-notification-delivery.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/040-notification-recipients.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

表达通知收件人的账号/组织/成员权威快照，并规范批量锁顺序。

通知投递资格不需要伪造用户Session；直接收集当前身份和成员来源即可。

写之前先按这条执行路线理解各段的连接：

1. NotificationRecipientIDs 复制并升序排序1至100个用户ID，拒绝0和重复值。
2. 快照分别携带用户identity/credential epoch、组织owner/state和成员role/state/epoch。

本文件需要守住的边界：

- 不能默默去重，调用方需要逐用户一一对应结果；不同批次必须同序加锁。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/account/internal/domain/account' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/account/internal/domain/account/notification_recipients.go') -and (Get-Item -LiteralPath 'services/account/internal/domain/account/notification_recipients.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/account/internal/domain/account/notification_recipients.go')) { New-Item -ItemType File -Path 'services/account/internal/domain/account/notification_recipients.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:47 -->
````go
package account

import (
	"errors"
	"sort"
)

````

## 第 2 步：ErrNotificationRecipientsInput

插入位置：第 1 步代码之后，紧接追加；完成稿约第 8 行。

本步声明：`var ErrNotificationRecipientsInput = errors.New("NOTIFICATION_RECIPIENTS_INVALID")`。

<!-- write-step: 2 bytes=47:131 -->
````go
var ErrNotificationRecipientsInput = errors.New("NOTIFICATION_RECIPIENTS_INVALID")

````

## 第 3 步：NotificationRecipientIdentity

插入位置：第 2 步代码之后，紧接追加；完成稿约第 10 行。

本步声明：`type NotificationRecipientIdentity struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ID</code> | <code>uint64</code> | 准备核验的收件账号编号。 |
| <code>IdentityEpoch</code> | <code>uint64</code> | 身份状态代次；准入与恢复时比较这份身份事实是否仍对应当前账号状态。 |
| <code>CredentialEpoch</code> | <code>uint64</code> | 凭据代次；改密码等凭据变化会推进它，旧会话与当前代次不一致时不能继续使用。 |
| <code>State</code> | <code>string</code> | 账号当前状态，只有符合规则的存续账号可收件。 |

<!-- write-step: 3 bytes=131:264 -->
````go
type NotificationRecipientIdentity struct {
	ID, IdentityEpoch, CredentialEpoch uint64
	State                              string
}

````

## 第 4 步：NotificationRecipientOrganization

插入位置：第 3 步代码之后，紧接追加；完成稿约第 15 行。

本步声明：`type NotificationRecipientOrganization struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Owner</code> | <code>uint64</code> | 组织所有者账号，决定某些组织通知的管理收件资格。 |
| <code>State</code> | <code>string</code> | 组织存续状态，关闭后不能继续按旧成员资格投递。 |

<!-- write-step: 4 bytes=264:343 -->
````go
type NotificationRecipientOrganization struct {
	Owner uint64
	State string
}

````

## 第 5 步：NotificationRecipientMembership

插入位置：第 4 步代码之后，紧接追加；完成稿约第 20 行。

本步声明：`type NotificationRecipientMembership struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Epoch</code> | <code>uint64</code> | 收件人的组织成员来源代次。 |
| <code>Role</code> | <code>string</code> | 收件人的成员角色，管理事件按相应角色过滤。 |
| <code>State</code> | <code>string</code> | 成员关系是否仍ACTIVE，撤权中不能继续依赖旧资格。 |

<!-- write-step: 5 bytes=343:432 -->
````go
type NotificationRecipientMembership struct {
	Epoch       uint64
	Role, State string
}

````

## 第 6 步：NotificationRecipientIDs

插入位置：第 5 步代码之后，紧接追加；完成稿约第 25 行。

固定升序用户锁避免同一批次反向遍历；不默默去重，调用者必须核对一一对应结果。

本步声明：`func NotificationRecipientIDs(ids []uint64) ([]uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ids</code> | <code>&#91;&#93;uint64</code> | 在此声明中的实际使用：<code>len(ids)</code>；<code>append(&#91;&#93;uint64(nil), ids...)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;uint64</code> | 实际返回表达式：<code>nil</code>；<code>ordered</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>ErrNotificationRecipientsInput</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(ids) == 0 &#124;&#124; len(ids) &gt; 100</code>。条件成立时执行 <code>return nil, ErrNotificationRecipientsInput</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return ordered&#91;i&#93; &lt; ordered&#91;j&#93;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>id == 0 &#124;&#124; (i &gt; 0 &amp;&amp; id == ordered&#91;i-1&#93;)</code>。条件成立时执行 <code>return nil, ErrNotificationRecipientsInput</code>。当前函数在这里结束。
- 此处通过 <code>return ordered, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=432:973 -->
````go
// 固定升序用户锁避免同一批次反向遍历；不默默去重，调用者必须核对一一对应结果。
func NotificationRecipientIDs(ids []uint64) ([]uint64, error) {
	if len(ids) == 0 || len(ids) > 100 {
		return nil, ErrNotificationRecipientsInput
	}
	ordered := append([]uint64(nil), ids...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	for i, id := range ordered {
		if id == 0 || (i > 0 && id == ordered[i-1]) {
			return nil, ErrNotificationRecipientsInput
		}
	}
	return ordered, nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/account/internal/domain/account/notification_recipients.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/account/05-notification-delivery.md)。
