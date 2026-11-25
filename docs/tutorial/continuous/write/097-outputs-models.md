# 分步编写 `services/document/internal/domain/document/outputs_models.go`

先按 [D11 · 输出登记、撤权与真实排空](../lessons/document/11-output-fence.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/097-outputs-models.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

定义编辑准入、票据、关闭状态、输出实例与租约的固定事实。

准入到期只限制新使用，不能证明旧HTTP/WebSocket或旧进程已真实停止。

写之前先按这条执行路线理解各段的连接：

1. EditingTicket绑定client/grant
2. FenceState绑定scope/epoch
3. OutputInstance记录PID归属
4. OutputLease记录来源和真实输出
5. CollaborationOutput引用已兑换准入。

本文件需要守住的边界：

- 缺失支线与过期票据不能默认为有效
- 调用者不可自报更少来源逃避撤权。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/domain/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/domain/document/outputs_models.go') -and (Get-Item -LiteralPath 'services/document/internal/domain/document/outputs_models.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/domain/document/outputs_models.go')) { New-Item -ItemType File -Path 'services/document/internal/domain/document/outputs_models.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:200 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	"time"
)

// edit_session.go：保留原实现，按业务合并文件。

````

## 第 2 步：EditWorkspace

插入位置：第 1 步代码之后，紧接追加；完成稿约第 10 行。

支线、准入和票据各自保留原事实；仓储扫描不把缺失支线或过期票据默认成有效。

本步声明：`type EditWorkspace struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Workspace</code> | <code>uint64</code> | 支线工作区ID |
| <code>Epoch</code> | <code>uint64</code> | 物理工作区代次 |
| <code>Lineage</code> | <code>uint64</code> | 当前来源谱系ID |
| <code>BaseManifest</code> | <code>uint64</code> | 支线所依据MAIN清单 |
| <code>BaseVersion</code> | <code>uint64</code> | 该单元所依据MAIN版本 |

<!-- write-step: 2 bytes=200:406 -->
````go
// 支线、准入和票据各自保留原事实；仓储扫描不把缺失支线或过期票据默认成有效。
type EditWorkspace struct {
	Workspace, Epoch, Lineage, BaseManifest, BaseVersion uint64
}
````

## 第 3 步：EditingAdmission

插入位置：第 2 步代码之后，紧接追加；完成稿约第 14 行。

本步声明：`type EditingAdmission struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Actor</code> | <code>uint64</code> | 准入真实用户 |
| <code>SessionRef</code> | <code>&#91;&#93;byte</code> | 原会话引用 |
| <code>TargetJSON</code> | <code>&#91;&#93;byte</code> | 固定workspace/epoch/目标绑定 |
| <code>State</code> | <code>string</code> | 准入OPEN或关闭状态 |
| <code>Expires</code> | <code>time.Time</code> | 短期准入绝对期限 |

<!-- write-step: 3 bytes=406:566 -->
````go
type EditingAdmission struct {
	Actor                  uint64
	SessionRef, TargetJSON []byte
	State                  string
	Expires                time.Time
}
````

## 第 4 步：EditingTicket

插入位置：第 3 步代码之后，紧接追加；完成稿约第 20 行。

本步声明：`type EditingTicket struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Admission</code> | <code>EditingAdmission</code> | 原EDIT准入完整事实 |
| <code>Purpose</code> | <code>string</code> | 票据对应业务用途 |
| <code>Client</code> | <code>string</code> | 领取票据的客户端会话ID |
| <code>State</code> | <code>string</code> | 未用或已兑换状态 |
| <code>Expires</code> | <code>time.Time</code> | 一次票据期限 |

<!-- write-step: 4 bytes=566:761 -->
````go
type EditingTicket struct {
	Admission              EditingAdmission
	Purpose, Client, State string
	Expires                time.Time
}

// fence.go：保留原实现，按业务合并文件。

````

## 第 5 步：FenceState

插入位置：第 4 步代码之后，紧接追加；完成稿约第 28 行。

FenceState保存本服务已经持久登记的关闭绑定，固定来源和代次不能被同号新请求替换。

本步声明：`type FenceState struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Scope</code> | <code>string</code> | 原关闭来源 |
| <code>Epoch</code> | <code>uint64</code> | 原关闭代次 |
| <code>State</code> | <code>string</code> | 关闭进度 |
| <code>TargetPermission</code> | <code>&#91;&#93;byte</code> | 原完整目标权限绑定字节，用于同号冲突检查 |

<!-- write-step: 5 bytes=761:1007 -->
````go
// FenceState保存本服务已经持久登记的关闭绑定，固定来源和代次不能被同号新请求替换。
type FenceState struct {
	Scope            string
	Epoch            uint64
	State            string
	TargetPermission []byte
}
````

## 第 6 步：PersistedOutput

插入位置：第 5 步代码之后，紧接追加；完成稿约第 35 行。

本步声明：`type PersistedOutput struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ID</code> | <code>string</code> | 持久输出租约ID |
| <code>Payload</code> | <code>&#91;&#93;byte</code> | 原租约完整固定JSON |

<!-- write-step: 6 bytes=1007:1131 -->
````go
type PersistedOutput struct {
	ID      string
	Payload []byte
}

// output.go：保留原实现，按业务合并文件。

````

## 第 7 步：OutputInstance

插入位置：第 6 步代码之后，紧接追加；完成稿约第 42 行。

输出租约是持久恢复协议的事实；期限本身不构成旧进程退出或handler排空的证明。

本步声明：`type OutputInstance struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ID</code> | <code>string</code> | 服务本次启动实例ID<br>序列化/校验标记：<code>`json:&quot;instance_id&quot;`</code> |
| <code>PID</code> | <code>uint64</code> | 操作系统进程ID<br>序列化/校验标记：<code>`json:&quot;pid&quot;`</code> |
| <code>Boot</code> | <code>string</code> | 启动归属摘要，用于防PID复用误判<br>序列化/校验标记：<code>`json:&quot;boot_digest&quot;`</code> |

<!-- write-step: 7 bytes=1131:1369 -->
````go
// 输出租约是持久恢复协议的事实；期限本身不构成旧进程退出或handler排空的证明。
type OutputInstance struct {
	ID   string `json:"instance_id"`
	PID  uint64 `json:"pid"`
	Boot string `json:"boot_digest"`
}
````

## 第 8 步：LeaseSource

插入位置：第 7 步代码之后，紧接追加；完成稿约第 48 行。

本步声明：`type LeaseSource struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Key</code> | <code>string</code> | 权限来源scope key<br>序列化/校验标记：<code>`json:&quot;scope_key&quot;`</code> |
| <code>Epoch</code> | <code>string</code> | 代次十进制文本，避免JavaScript uint64精度丢失<br>序列化/校验标记：<code>`json:&quot;epoch&quot;`</code> |

<!-- write-step: 8 bytes=1369:1459 -->
````go
type LeaseSource struct {
	Key   string `json:"scope_key"`
	Epoch string `json:"epoch"`
}
````

## 第 9 步：OutputLease

插入位置：第 8 步代码之后，紧接追加；完成稿约第 52 行。

本步声明：`type OutputLease struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ID</code> | <code>string</code> | 该实际输出租约ID<br>序列化/校验标记：<code>`json:&quot;lease_id&quot;`</code> |
| <code>Instance</code> | <code>string</code> | 承担此流的原服务实例<br>序列化/校验标记：<code>`json:&quot;instance_id&quot;`</code> |
| <code>Handler</code> | <code>string</code> | 真实HTTP/WebSocket处理器标识<br>序列化/校验标记：<code>`json:&quot;handler_id&quot;`</code> |
| <code>Sources</code> | <code>&#91;&#93;LeaseSource</code> | 撤权要匹配的完整来源集合<br>序列化/校验标记：<code>`json:&quot;sources&quot;`</code> |
| <code>Document</code> | <code>uint64</code> | 输出文档<br>序列化/校验标记：<code>`json:&quot;document_id,omitempty&quot;`</code> |
| <code>Admission</code> | <code>string</code> | 内部正文真实准入，正式输出不伪造此值<br>序列化/校验标记：<code>`json:&quot;admission_id,omitempty&quot;`</code> |
| <code>Authority</code> | <code>string</code> | 输出权威类型<br>序列化/校验标记：<code>`json:&quot;authority,omitempty&quot;`</code> |
| <code>PublicationEpoch</code> | <code>string</code> | 当前正式发布代次文本<br>序列化/校验标记：<code>`json:&quot;publication_epoch,omitempty&quot;`</code> |
| <code>Release</code> | <code>string</code> | 固定正式版ID文本<br>序列化/校验标记：<code>`json:&quot;release_id,omitempty&quot;`</code> |

<!-- write-step: 9 bytes=1459:2051 -->
````go
type OutputLease struct {
	ID        string        `json:"lease_id"`
	Instance  string        `json:"instance_id"`
	Handler   string        `json:"handler_id"`
	Sources   []LeaseSource `json:"sources"`
	Document  uint64        `json:"document_id,omitempty"`
	Admission string        `json:"admission_id,omitempty"`
	Authority string        `json:"authority,omitempty"`
	// 正式输出没有虚构admission；十进制字符串避免Edge的JSON数字丢失完整uint64。
	PublicationEpoch string `json:"publication_epoch,omitempty"`
	Release          string `json:"release_id,omitempty"`
}

````

## 第 10 步：CollaborationOutput

插入位置：第 9 步代码之后，紧接追加；完成稿约第 65 行。

CollaborationOutput引用已兑换准入，调用方只提交固定绑定，不能自定权限来源。

本步声明：`type CollaborationOutput struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Admission</code> | <code>string</code> | 已经兑换过的Doc准入 |
| <code>Grant</code> | <code>string</code> | 原兑换grant |
| <code>Handler</code> | <code>string</code> | 实际WSS输出处理器ID |

<!-- write-step: 10 bytes=2051:2228 -->
````go
// CollaborationOutput引用已兑换准入，调用方只提交固定绑定，不能自定权限来源。
type CollaborationOutput struct {
	Admission, Grant, Handler string
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/domain/document/outputs_models.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/11-output-fence.md)。
