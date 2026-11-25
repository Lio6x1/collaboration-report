# 分步编写 `services/document/internal/domain/document/authorization_models.go`

先按 [D01 · 身份、权限与共享准入](../lessons/document/01-identity-access.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/090-authorization-models.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

把账号会话事实、文档授权来源和撤权关闭计划表达成领域类型。

编辑权限按单元计算，管理角色与直接授权取并集，撤销一个来源不能删除其他来源赋予的资格。

写之前先按这条执行路线理解各段的连接：

1. ResolvePermissions先只接受当前Actor的GrantFact，再按DOCUMENT/CHAPTER/UNIT扩展能力
2. CoversHistoricalRange要求所有后继单元均可编辑
3. TokenHash验证32字节规范编码再散列。

本文件需要守住的边界：

- 历史范围proven=false或后继为空必须拒绝普通编辑者
- 邀请不能直接以明文令牌作为数据库身份。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/domain/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/domain/document/authorization_models.go') -and (Get-Item -LiteralPath 'services/document/internal/domain/document/authorization_models.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/domain/document/authorization_models.go')) { New-Item -ItemType File -Path 'services/document/internal/domain/document/authorization_models.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:237 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	"crypto/sha256"
	"encoding/base64"
	"sort"
)

// authorization.go：保留原实现，按业务合并文件。

````

## 第 2 步：Closure

插入位置：第 1 步代码之后，紧接追加；完成稿约第 12 行。

Closure固定已提交的关闭计划；恢复必须继续原编号、原范围及原代次。

本步声明：`type Closure struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Scope</code> | <code>string</code> | 要关闭的权限来源键，例如文档ACL<br>序列化/校验标记：<code>`json:&quot;scope&quot;`</code> |
| <code>Epoch</code> | <code>uint64</code> | 这次关闭的旧来源代次<br>序列化/校验标记：<code>`json:&quot;epoch&quot;`</code> |
| <code>Entity</code> | <code>uint64</code> | 关闭动作关联的文档或知识库编号<br>序列化/校验标记：<code>`json:&quot;entity&quot;`</code> |
| <code>Action</code> | <code>string</code> | 关闭后的业务收尾动作，如TRASH_DOCUMENT或CLOSE_KB<br>序列化/校验标记：<code>`json:&quot;action&quot;`</code> |

<!-- write-step: 2 bytes=237:481 -->
````go
// Closure固定已提交的关闭计划；恢复必须继续原编号、原范围及原代次。
type Closure struct {
	Scope  string `json:"scope"`
	Epoch  uint64 `json:"epoch"`
	Entity uint64 `json:"entity"`
	Action string `json:"action"`
}
````

## 第 3 步：AuthorizationTask

插入位置：第 2 步代码之后，紧接追加；完成稿约第 19 行。

本步声明：`type AuthorizationTask struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ID</code> | <code>uint64</code> | durable_task行号，用于领取和所有权CAS |
| <code>Operation</code> | <code>string</code> | 原撤权操作号，所有恢复和ACK继续使用 |
| <code>Payload</code> | <code>&#91;&#93;byte</code> | 序列化固定Closure，不在重试时重新选择范围 |

<!-- write-step: 3 bytes=481:569 -->
````go
type AuthorizationTask struct {
	ID        uint64
	Operation string
	Payload   []byte
}
````

## 第 4 步：AuthorizationStatus

插入位置：第 3 步代码之后，紧接追加；完成稿约第 24 行。

本步声明：`type AuthorizationStatus struct{ State, Scope, Epoch, ReceiptState string }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>State</code> | <code>string</code> | 授权变化当前阶段 |
| <code>Scope</code> | <code>string</code> | 原关闭来源键 |
| <code>Epoch</code> | <code>string</code> | 原来源代次的十进制文本 |
| <code>ReceiptState</code> | <code>string</code> | 通用操作回执状态，用于交叉核验成功 |

<!-- write-step: 4 bytes=569:645 -->
````go
type AuthorizationStatus struct{ State, Scope, Epoch, ReceiptState string }
````

## 第 5 步：TaskOwnership

插入位置：第 4 步代码之后，紧接追加；完成稿约第 25 行。

本步声明：`type TaskOwnership struct{ Instance, State string }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Instance</code> | <code>string</code> | 当前领取任务的服务实例ID |
| <code>State</code> | <code>string</code> | 任务持久状态，只有仍归此实例的RUNNING可完成 |

<!-- write-step: 5 bytes=645:759 -->
````go
type TaskOwnership struct{ Instance, State string }

// identity.go：保留原实现，按业务合并文件。

````

## 第 6 步：SessionVerification

插入位置：第 5 步代码之后，紧接追加；完成稿约第 29 行。

SessionVerification只携带账号权威验证所需元数据，业务层不保存HTTP请求。

本步声明：`type SessionVerification struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Cookie</code> | <code>string</code> | 原请求Cookie头，由Account核验会话 |
| <code>Origin</code> | <code>string</code> | 浏览器请求来源，用于变更入口校验 |
| <code>CSRF</code> | <code>string</code> | 原X-CSRF-Token，绑定会话变更 |
| <code>Mutation</code> | <code>bool</code> | 是否执行变更，决定是否必须通过Origin和CSRF验证 |

<!-- write-step: 6 bytes=759:951 -->
````go
// SessionVerification只携带账号权威验证所需元数据，业务层不保存HTTP请求。
type SessionVerification struct {
	Cookie, Origin, CSRF string
	Mutation             bool
}

````

## 第 7 步：VerifiedSession

插入位置：第 6 步代码之后，紧接追加；完成稿约第 35 行。

本步声明：`type VerifiedSession struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ActorID</code> | <code>uint64</code> | Account确认的当前用户ID |
| <code>ExpiresUnixMs</code> | <code>int64</code> | 真实会话绝对过期时间，单位毫秒，短期证明续期不能越过它 |

<!-- write-step: 7 bytes=951:1091 -->
````go
type VerifiedSession struct {
	ActorID       uint64
	ExpiresUnixMs int64
}

// permissions.go：保留原实现，按业务合并文件。

````

## 第 8 步：GrantFact

插入位置：第 7 步代码之后，紧接追加；完成稿约第 42 行。

权限来源取并集；撤销直接授权不能抹掉仍然有效的上级来源。

本步声明：`type GrantFact struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>User</code> | <code>uint64</code> | 被授权用户ID，只处理等于Actor的记录 |
| <code>Kind</code> | <code>string</code> | 授权范围种类DOCUMENT、CHAPTER或UNIT |
| <code>Scope</code> | <code>uint64</code> | 该范围实体ID，不能跨文档使用 |
| <code>Capability</code> | <code>string</code> | MANAGE、REVIEW、COMMENT或EDIT能力 |

<!-- write-step: 8 bytes=1091:1281 -->
````go
// 权限来源取并集；撤销直接授权不能抹掉仍然有效的上级来源。
type GrantFact struct {
	User       uint64
	Kind       string
	Scope      uint64
	Capability string
}
````

## 第 9 步：UnitFact

插入位置：第 8 步代码之后，紧接追加；完成稿约第 49 行。

本步声明：`type UnitFact struct{ ID, Chapter uint64 }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ID</code> | <code>uint64</code> | 当前结构中未退役协作单元ID |
| <code>Chapter</code> | <code>uint64</code> | 单元所属章节，用于扩展章节EDIT授权 |

<!-- write-step: 9 bytes=1281:1324 -->
````go
type UnitFact struct{ ID, Chapter uint64 }
````

## 第 10 步：PermissionFacts

插入位置：第 9 步代码之后，紧接追加；完成稿约第 50 行。

本步声明：`type PermissionFacts struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Actor</code> | <code>uint64</code> | 正在求权限的用户 |
| <code>Responsible</code> | <code>bool</code> | 用户是否为当前负责人 |
| <code>PersonalOwner</code> | <code>bool</code> | 用户是否为当前个人空间所有者 |
| <code>OrganizationAdmin</code> | <code>bool</code> | Account证明的当前组织管理员 |
| <code>KnowledgeBaseAdmin</code> | <code>bool</code> | 当前活动知识库管理员 |
| <code>Grants</code> | <code>&#91;&#93;GrantFact</code> | 该用户活动直接授权事实 |
| <code>Units</code> | <code>&#91;&#93;UnitFact</code> | 当前结构中的单元及所属章 |

<!-- write-step: 10 bytes=1324:1659 -->
````go
type PermissionFacts struct {
	Actor                                                             uint64
	Responsible, PersonalOwner, OrganizationAdmin, KnowledgeBaseAdmin bool
	Grants                                                            []GrantFact
	Units                                                             []UnitFact
}
````

## 第 11 步：Permissions

插入位置：第 10 步代码之后，紧接追加；完成稿约第 56 行。

本步声明：`type Permissions struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Read</code> | <code>bool</code> | 可读取文档内部正文 |
| <code>Comment</code> | <code>bool</code> | 可创建普通批注 |
| <code>Review</code> | <code>bool</code> | 可执行审核动作，但仍受提交者与轮次规则限制 |
| <code>Manage</code> | <code>bool</code> | 可管理文档并继承读评审及全部单元编辑 |
| <code>Edit</code> | <code>map&#91;uint64&#93;bool</code> | 单元ID到当前编辑资格的集合，不包含自动推导的历史写权限 |

<!-- write-step: 11 bytes=1659:1771 -->
````go
type Permissions struct {
	Read, Comment, Review, Manage bool
	Edit                          map[uint64]bool
}

````

## 第 12 步：ResolvePermissions

插入位置：第 11 步代码之后，紧接追加；完成稿约第 61 行。

本步声明：`func ResolvePermissions(f PermissionFacts) Permissions`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>f</code> | <code>PermissionFacts</code> | 在此声明中的实际使用：<code>f.Responsible</code>；<code>f.PersonalOwner</code>；<code>f.OrganizationAdmin</code>；<code>f.KnowledgeBaseAdmin</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Permissions</code> | 实际返回表达式：<code>p</code> |

**跟着本段代码走一遍：**

- 检查 <code>g.User != f.Actor</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>g.Kind == &quot;DOCUMENT&quot;</code>。
- 检查 <code>(g.Kind == &quot;UNIT&quot; &amp;&amp; g.Scope == u.ID) &#124;&#124; (g.Kind == &quot;CHAPTER&quot; &amp;&amp; g.Scope == u.Chapter)</code>。
- 检查 <code>p.Manage</code>。
- 此处通过 <code>return p</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=1771:2672 -->
````go
func ResolvePermissions(f PermissionFacts) Permissions {
	p := Permissions{Edit: map[uint64]bool{}, Manage: f.Responsible || f.PersonalOwner || f.OrganizationAdmin || f.KnowledgeBaseAdmin}
	for _, g := range f.Grants {
		if g.User != f.Actor {
			continue
		}
		switch g.Capability {
		case "MANAGE":
			if g.Kind == "DOCUMENT" {
				p.Manage = true
			}
		case "REVIEW":
			if g.Kind == "DOCUMENT" {
				p.Review = true
				p.Comment = true
				p.Read = true
			}
		case "COMMENT":
			if g.Kind == "DOCUMENT" {
				p.Comment = true
				p.Read = true
			}
		case "EDIT":
			for _, u := range f.Units {
				if (g.Kind == "UNIT" && g.Scope == u.ID) || (g.Kind == "CHAPTER" && g.Scope == u.Chapter) {
					p.Edit[u.ID] = true
					p.Read = true
				}
			}
		}
	}
	if p.Manage {
		p.Read = true
		p.Comment = true
		p.Review = true
		for _, u := range f.Units {
			p.Edit[u.ID] = true
		}
	}
	return p
}

````

## 第 13 步：CoversHistoricalRange

插入位置：第 12 步代码之后，紧接追加；完成稿约第 103 行。

草稿历史必须从稳定lineage证明原范围被当前权限完整覆盖；空映射不是空范围成功。

本步声明：`func CoversHistoricalRange(p Permissions, descendants []uint64, proven bool) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>p</code> | <code>Permissions</code> | 在此声明中的实际使用：<code>p.Manage</code>；<code>p.Edit</code> |
| <code>descendants</code> | <code>&#91;&#93;uint64</code> | 在此声明中的实际使用：<code>len(descendants)</code> |
| <code>proven</code> | <code>bool</code> | 在此声明中的实际使用：<code>!proven</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>true</code>；<code>false</code> |

**跟着本段代码走一遍：**

- 检查 <code>p.Manage</code>。条件成立时执行 <code>return true</code>。当前函数在这里结束。
- 检查 <code>!proven &#124;&#124; len(descendants) == 0</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>!p.Edit&#91;unit&#93;</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 此处通过 <code>return true</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=2672:3057 -->
````go
// 草稿历史必须从稳定lineage证明原范围被当前权限完整覆盖；空映射不是空范围成功。
func CoversHistoricalRange(p Permissions, descendants []uint64, proven bool) bool {
	if p.Manage {
		return true
	}
	if !proven || len(descendants) == 0 {
		return false
	}
	for _, unit := range descendants {
		if !p.Edit[unit] {
			return false
		}
	}
	return true
}

````

## 第 14 步：KBRoleFact

插入位置：第 13 步代码之后，紧接追加；完成稿约第 119 行。

本步声明：`type KBRoleFact struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>User</code> | <code>uint64</code> | 知识库成员ID |
| <code>Role</code> | <code>string</code> | 知识库角色，ADMIN才是管理员通知候选 |
| <code>State</code> | <code>string</code> | 角色状态，只有ACTIVE参与候选 |

<!-- write-step: 14 bytes=3057:3125 -->
````go
type KBRoleFact struct {
	User        uint64
	Role, State string
}

````

## 第 15 步：KnowledgeBaseNotificationTargets

插入位置：第 14 步代码之后，紧接追加；完成稿约第 124 行。

本步声明：`func KnowledgeBaseNotificationTargets(roles []KBRoleFact, _ []uint64) []uint64`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>roles</code> | <code>&#91;&#93;KBRoleFact</code> | 知识库角色事实列表，仅取ACTIVE的ADMIN账号，去重后按编号排序；组织管理员不能因此自动成为库管理员接收人。 |
| <code>_</code> | <code>&#91;&#93;uint64</code> | 这个位置的输入按接口约定接收，但本实现明确不使用它；下划线避免产生未使用变量。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;uint64</code> | 实际返回表达式：<code>list</code> |

**跟着本段代码走一遍：**

- 检查 <code>r.Role == &quot;ADMIN&quot; &amp;&amp; r.State == &quot;ACTIVE&quot; &amp;&amp; !seen&#91;r.User&#93;</code>。
- 在本段的匿名函数内：此处通过 <code>return list&#91;i&#93; &lt; list&#91;j&#93;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return list</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=3125:3549 -->
````go
func KnowledgeBaseNotificationTargets(roles []KBRoleFact, _ []uint64) []uint64 {
	seen := map[uint64]bool{}
	var list []uint64
	for _, r := range roles {
		if r.Role == "ADMIN" && r.State == "ACTIVE" && !seen[r.User] {
			seen[r.User] = true
			list = append(list, r.User)
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i] < list[j] })
	return list
}

// sharing.go：保留原实现，按业务合并文件。

````

## 第 16 步：TokenHash

插入位置：第 15 步代码之后，紧接追加；完成稿约第 139 行。

TokenHash只接受规范的32字节随机令牌；不同编码不能绕过同一邀请的持久身份。

本步声明：`func TokenHash(token string) ([]byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>token</code> | <code>string</code> | 在此声明中的实际使用：<code>base64.RawURLEncoding.DecodeString(token)</code>；<code>base64.RawURLEncoding.EncodeToString(raw) != token</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>nil</code>；<code>hash&#91;:&#93;</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>Violation(InvalidRequest, &quot;INVITATION_TOKEN_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil &#124;&#124; len(raw) != 32 &#124;&#124; base64.RawURLEncoding.EncodeToString(raw) != token</code>。条件成立时执行 <code>return nil, Violation(InvalidRequest, &quot;INVITATION_TOKEN_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return hash&#91;:&#93;, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=3549:3970 -->
````go
// TokenHash只接受规范的32字节随机令牌；不同编码不能绕过同一邀请的持久身份。
func TokenHash(token string) ([]byte, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) != 32 || base64.RawURLEncoding.EncodeToString(raw) != token {
		return nil, Violation(InvalidRequest, "INVITATION_TOKEN_INVALID")
	}
	hash := sha256.Sum256(raw)
	return hash[:], nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/domain/document/authorization_models.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/01-identity-access.md)。
