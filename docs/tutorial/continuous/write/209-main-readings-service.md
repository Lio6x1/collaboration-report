# 分步编写 `services/document/internal/logic/document/main_readings_service.go`

先按 [D02 · 创建工作区与可靠正文阅读](../lessons/document/02-workspace-content.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/209-main-readings-service.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

从真实MAIN可靠尾部捕获整篇内容，物化成可安全输出的固定阅读快照。

数据库旧版本未必包含协作服务已可靠保存的尾部；直接读旧manifest会丢掉用户刚保存的内容。

写之前先按这条执行路线理解各段的连接：

1. Capture登记原准入后在事务外调用CaptureMain
2. validateCapture核验固定来源
3. Publish在原事务追加版本与VERSION资源引用
4. validateCurrent再核权限、门闩与原准入。

本文件需要守住的边界：

- 只读捕获不得安装MAIN或推进control
- 捕获后撤权或结构变化必须阻止旧正文输出。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/logic/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/logic/document/main_readings_service.go') -and (Get-Item -LiteralPath 'services/document/internal/logic/document/main_readings_service.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/logic/document/main_readings_service.go')) { New-Item -ItemType File -Path 'services/document/internal/logic/document/main_readings_service.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:565 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	"bytes"
	codec "collaboration/contracts/content"
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"collaboration/platform"
	"context"
	"database/sql"
	"encoding/json"
	"google.golang.org/protobuf/proto"
	"sort"
	"time"
)

// main_read.go：保留原实现，按业务合并文件。

````

## 第 2 步：MainReadViews

插入位置：第 1 步代码之后，紧接追加；完成稿约第 22 行。

本步声明：`type MainReadViews struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。 |
| <code>client</code> | <code>MainReadClient</code> | 使用 <code>MainReadClient</code> 类型；其定义处的职责：阅读页从Node可靠主线采样，保持所有单元来自同一次受约束的读取。 |
| <code>validator</code> | <code>*codec.Validator</code> | 冻结协议的结构校验器，先限制请求形状，再由业务核验身份、权限、版本和事务条件。 |

<!-- write-step: 2 bytes=565:690 -->
````go
type MainReadViews struct {
	db        *sql.DB
	identity  *Identity
	client    MainReadClient
	validator *codec.Validator
}

````

## 第 3 步：MainReadClient

插入位置：第 2 步代码之后，紧接追加；完成稿约第 29 行。

本步声明：`type MainReadClient interface {`。

**这一段为什么这样写：** 阅读页从Node可靠主线采样，保持所有单元来自同一次受约束的读取。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>CaptureMain</code> | <code>(context.Context, *pb.CaptureMainRequest) (*pb.MainCaptureReply, error)</code> | ctx与主线采样请求返回固定主线内容及cut；不是直接从正在变化的内存拼阅读页。 |
| <code>MainRevision</code> | <code>(context.Context, uint64) (string, error)</code> | ctx与文档编号返回Node持久主线修订标记，用于检测是否需要更新阅读快照。 |

<!-- write-step: 3 bytes=690:864 -->
````go
type MainReadClient interface {
	CaptureMain(context.Context, *pb.CaptureMainRequest) (*pb.MainCaptureReply, error)
	MainRevision(context.Context, uint64) (string, error)
}

````

## 第 4 步：mainReadContext

插入位置：第 3 步代码之后，紧接追加；完成稿约第 34 行。

固定内容来自真实只读准入与完整MAIN来源，和管理控制上下文分开。
私有捕获可以追加阅读快照，却不能据此锁定主线、推进control或批准任何轮次。

本步声明：`type mainReadContext struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Actor</code> | <code>uint64</code> | 序列化/校验标记：<code>`json:&quot;actor&quot;`</code> |
| <code>SessionExpires</code> | <code>int64</code> | 账号会话的到期时间，以协议约定的时间值传递；下游准入不能延长原会话有效期。<br>序列化/校验标记：<code>`json:&quot;session_expires&quot;`</code> |
| <code>Access</code> | <code>DocumentAccess</code> | 使用 <code>DocumentAccess</code> 类型；其定义处的职责：把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。<br>序列化/校验标记：<code>`json:&quot;access&quot;`</code> |
| <code>Context</code> | <code>*pb.OperationContext</code> | 内部协议的原操作上下文，绑定操作号、目标、身份与准入来源；恢复必须匹配原上下文，不能只认操作号。<br>序列化/校验标记：<code>`json:&quot;context&quot;`</code> |
| <code>Manifest</code> | <code>uint64</code> | 序列化/校验标记：<code>`json:&quot;manifest&quot;`</code> |
| <code>Structure</code> | <code>uint64</code> | 序列化/校验标记：<code>`json:&quot;structure&quot;`</code> |
| <code>Style</code> | <code>uint64</code> | 序列化/校验标记：<code>`json:&quot;style&quot;`</code> |
| <code>Control</code> | <code>uint64</code> | 序列化/校验标记：<code>`json:&quot;control&quot;`</code> |
| <code>MainEpoch</code> | <code>uint64</code> | 序列化/校验标记：<code>`json:&quot;main_epoch,omitempty&quot;`</code> |
| <code>ExpiresUnixMs</code> | <code>int64</code> | 序列化/校验标记：<code>`json:&quot;expires_unix_ms&quot;`</code> |
| <code>Units</code> | <code>&#91;&#93;uint64</code> | 序列化/校验标记：<code>`json:&quot;units&quot;`</code> |
| <code>Versions</code> | <code>map&#91;uint64&#93;uint64</code> | 序列化/校验标记：<code>`json:&quot;versions&quot;`</code> |

<!-- write-step: 4 bytes=864:1776 -->
````go
// 固定内容来自真实只读准入与完整MAIN来源，和管理控制上下文分开。
// 私有捕获可以追加阅读快照，却不能据此锁定主线、推进control或批准任何轮次。
type mainReadContext struct {
	Actor          uint64               `json:"actor"`
	SessionExpires int64                `json:"session_expires"`
	Access         DocumentAccess       `json:"access"`
	Context        *pb.OperationContext `json:"context"`
	Manifest       uint64               `json:"manifest"`
	Structure      uint64               `json:"structure"`
	Style          uint64               `json:"style"`
	Control        uint64               `json:"control"`
	MainEpoch      uint64               `json:"main_epoch,omitempty"`
	ExpiresUnixMs  int64                `json:"expires_unix_ms"`
	Units          []uint64             `json:"units"`
	Versions       map[uint64]uint64    `json:"versions"`
}

````

## 第 5 步：mainReadCapture

插入位置：第 4 步代码之后，紧接追加；完成稿约第 51 行。

本步声明：`type mainReadCapture struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Fixed</code> | <code>mainReadContext</code> | 这次阅读捕获固定的权限、文档和版本上下文。 |
| <code>Main</code> | <code>*pb.MainCaptureReply</code> | Node返回的可靠主线采样及cut，Doc继续核验后创建阅读结果。 |

<!-- write-step: 5 bytes=1776:1860 -->
````go
type mainReadCapture struct {
	Fixed mainReadContext
	Main  *pb.MainCaptureReply
}

````

## 第 6 步：NewMainReadViews

插入位置：第 5 步代码之后，紧接追加；完成稿约第 56 行。

本步声明：`func NewMainReadViews(db *sql.DB, identity *Identity, client MainReadClient, validator *codec.Validator) *MainReadViews`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。<br>在此声明中的实际使用：<code>identity: identity</code> |
| <code>client</code> | <code>MainReadClient</code> | 使用 <code>MainReadClient</code> 类型；其定义处的职责：阅读页从Node可靠主线采样，保持所有单元来自同一次受约束的读取。<br>在此声明中的实际使用：<code>client: client</code> |
| <code>validator</code> | <code>*codec.Validator</code> | 冻结协议的结构校验器，先限制请求形状，再由业务核验身份、权限、版本和事务条件。<br>在此声明中的实际使用：<code>validator: validator</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*MainReadViews</code> | 实际返回表达式：<code>&amp;MainReadViews{db: db, identity: identity, client: client, validator: validator}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;MainReadViews{db: db, identity: identity, client: client, validator: validator}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1860:2074 -->
````go
func NewMainReadViews(db *sql.DB, identity *Identity, client MainReadClient, validator *codec.Validator) *MainReadViews {
	return &MainReadViews{db: db, identity: identity, client: client, validator: validator}
}

````

## 第 7 步：MainReadViews.fixedContext

插入位置：第 6 步代码之后，紧接追加；完成稿约第 60 行。

本步声明：`func (s *MainReadViews) fixedContext(ctx context.Context, operation string) (mainReadContext, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.MainReadContext(ctx, s.db, operation)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>repository.MainReadContext(ctx, s.db, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>mainReadContext</code> | 实际返回表达式：<code>fixed</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_READ_BINDING_UNKNOWN&quot;)</code>；<code>validateMainReadContext(fixed)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return fixed, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return fixed, domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_READ_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return fixed, validateMainReadContext(fixed)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=2074:2573 -->
````go
func (s *MainReadViews) fixedContext(ctx context.Context, operation string) (mainReadContext, error) {
	var fixed mainReadContext
	raw, err := repository.MainReadContext(ctx, s.db, operation)
	if err != nil {
		return fixed, err
	}
	err = json.Unmarshal(raw, &fixed)
	if err != nil {
		return fixed, domain.Violation(domain.TemporarilyUnavailable, "MAIN_READ_BINDING_UNKNOWN")
	}
	return fixed, validateMainReadContext(fixed)
}

// main_read_capture.go：保留原实现，按业务合并文件。

````

## 第 8 步：mainReadRequest

插入位置：第 7 步代码之后，紧接追加；完成稿约第 75 行。

本步声明：`func mainReadRequest(fixed mainReadContext) *pb.CaptureMainRequest`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>fixed</code> | <code>mainReadContext</code> | 在此声明中的实际使用：<code>fixed.Context</code>；<code>fixed.Manifest</code>；<code>fixed.Structure</code>；<code>fixed.Style</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.CaptureMainRequest</code> | 实际返回表达式：<code>&amp;pb.CaptureMainRequest{ 		Context: fixed.Context, Usage: &quot;MAIN_READ_VIEW:1&quot;, 		ExpectedManifestId: fixed.Manifest, ExpectedStructureId: fixed.Structure, 		ExpectedStyleId: fixed.St …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;pb.CaptureMainRequest{ 		Context: fixed.Context, Usage: &quot;MAIN_READ_VIEW:1&quot;, 		ExpectedManifestId: fixed.Manifest, ExpectedStructureId: fixed.Structure, 		ExpectedStyleId: f …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=2573:2881 -->
````go
func mainReadRequest(fixed mainReadContext) *pb.CaptureMainRequest {
	return &pb.CaptureMainRequest{
		Context: fixed.Context, Usage: "MAIN_READ_VIEW:1",
		ExpectedManifestId: fixed.Manifest, ExpectedStructureId: fixed.Structure,
		ExpectedStyleId: fixed.Style, ExpectedControlRevision: fixed.Control,
	}
}

````

## 第 9 步：MainReadViews.Capture

插入位置：第 8 步代码之后，紧接追加；完成稿约第 83 行。

登记、跨服务固定和最终发布是三个边界；任何RPC都不能占用Doc的来源/文档SQL锁。
只保存actor编号和原证明，不保存可再次登录的会话Token，也不为普通读取续期身份。

本步声明：`func (s *MainReadViews) Capture(ctx context.Context, actor Actor, access DocumentAccess, document uint64) (*mainReadCapture, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.revalidate(ctx, fixed)</code>；<code>s.db.BeginTx(ctx, nil)</code>；<code>LockSources(ctx, tx, access.Sources)</code>；<code>ValidateAccess(ctx, tx, access)</code><br>另有 8 处使用，继续按下方完整代码的顺序处理。 |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor.ID</code>；<code>actor.SessionExpiresUnixMs</code>；<code>actor.Proof</code>；<code>InsertAdmission(ctx, tx, actor, access.Sources, document, &quot;READ_MAIN&quot;, admission)</code> |
| <code>access</code> | <code>DocumentAccess</code> | 使用 <code>DocumentAccess</code> 类型；其定义处的职责：把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。<br>在此声明中的实际使用：<code>Access: access</code>；<code>access.Proofs</code>；<code>access.Sources</code>；<code>ValidateAccess(ctx, tx, access)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.LockedMergeBarrier(ctx, tx, document)</code>；<code>repository.MainWorkspaceEpoch(ctx, tx, document)</code>；<code>repository.ManifestUnits(ctx, tx, current.CurrentManifestId, document)</code>；<code>InsertAdmission(ctx, tx, actor, access.Sources, document, &quot;READ_MAIN&quot;, admission)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*mainReadCapture</code> | 实际返回表达式：<code>nil</code>；<code>&amp;mainReadCapture{Fixed: fixed, Main: main}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_READ_IDENTITY_UNKNOWN&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;MAIN_SOURCE_CHANGED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;MAIN_COMMIT_PENDING&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>proof == nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_READ_IDENTITY_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>proof.ExpiresUnixMs &lt; fixed.ExpiresUnixMs</code>。
- 检查 <code>fixed.SessionExpires &lt; fixed.ExpiresUnixMs</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>current.MainLocked &#124;&#124; current.Lifecycle != &quot;ACTIVE&quot;</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;MAIN_SOURCE_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>barrier.Phase != &quot;OPEN&quot;</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;MAIN_COMMIT_PENDING&quot;)</code>。当前函数在这里结束。

<!-- write-step: 9 bytes=2881:4727 -->
````go
// 登记、跨服务固定和最终发布是三个边界；任何RPC都不能占用Doc的来源/文档SQL锁。
// 只保存actor编号和原证明，不保存可再次登录的会话Token，也不为普通读取续期身份。
func (s *MainReadViews) Capture(ctx context.Context, actor Actor, access DocumentAccess, document uint64) (*mainReadCapture, error) {
	fixed := mainReadContext{Actor: actor.ID, SessionExpires: actor.SessionExpiresUnixMs, Access: access, ExpiresUnixMs: actor.Proof.ExpiresUnixMs}
	for _, proof := range access.Proofs {
		if proof == nil {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "MAIN_READ_IDENTITY_UNKNOWN")
		}
		if proof.ExpiresUnixMs < fixed.ExpiresUnixMs {
			fixed.ExpiresUnixMs = proof.ExpiresUnixMs
		}
	}
	if fixed.SessionExpires < fixed.ExpiresUnixMs {
		fixed.ExpiresUnixMs = fixed.SessionExpires
	}
	if err := s.revalidate(ctx, fixed); err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = LockSources(ctx, tx, access.Sources); err != nil {
		return nil, err
	}
	current, err := ValidateAccess(ctx, tx, access)
	if err != nil {
		return nil, err
	}
	if current.MainLocked || current.Lifecycle != "ACTIVE" {
		return nil, domain.Violation(domain.StateConflict, "MAIN_SOURCE_CHANGED")
	}
	barrier, err := repository.LockedMergeBarrier(ctx, tx, document)
	if err != nil {
		return nil, err
	}
	if barrier.Phase != "OPEN" {
		return nil, domain.Violation(domain.StateConflict, "MAIN_COMMIT_PENDING")
	}
	fixed.Access.Document = current
	fixed.Manifest, _ = domain.CanonicalID(current.CurrentManifestId)
	fixed.Structure, _ = domain.CanonicalID(current.CurrentStructureId)
	fixed.Style, _ = domain.CanonicalID(current.CurrentStyleId)
	fixed.Control, _ = domain.CanonicalRevision(current.ControlRevision)
````

## 第 10 步：MainReadViews.Capture（接着实现）

插入位置：第 9 步代码之后，紧接追加；完成稿约第 128 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。

<!-- write-step: 10 bytes=4727:6593 -->
````go
	fixed.MainEpoch, err = repository.MainWorkspaceEpoch(ctx, tx, document)
	if err != nil {
		return nil, err
	}
	units, err := repository.ManifestUnits(ctx, tx, current.CurrentManifestId, document)
	if err != nil {
		return nil, err
	}
	structure, err := repository.StructureRow(ctx, tx, current.CurrentStructureId, current.Id)
	if err != nil {
		return nil, err
	}
	if err = validateReadingUnits(structure, units); err != nil {
		return nil, err
	}
	fixed.Units = make([]uint64, 0, len(units))
	fixed.Versions = make(map[uint64]uint64, len(units))
	for _, unit := range units {
		id, _ := domain.CanonicalID(unit["unit_id"])
		version, _ := domain.CanonicalID(unit["content_version_id"])
		fixed.Units = append(fixed.Units, id)
		fixed.Versions[id] = version
	}
	admission := platform.NewTraceID()
	if err = InsertAdmission(ctx, tx, actor, access.Sources, document, "READ_MAIN", admission); err != nil {
		return nil, err
	}
	target := &pb.Target{DocumentId: document, StructureId: fixed.Structure, WorkspaceEpoch: fixed.Control, Purpose: "READ_MAIN"}
	rawTarget, err := json.Marshal(target)
	if err != nil {
		return nil, err
	}
	if err = repository.BindMainReadAdmission(ctx, tx, admission, actor.Proof.OperationId, rawTarget); err != nil {
		return nil, err
	}
	fingerprint, err := domain.OperationFingerprint("/v1/documents/"+current.Id+"/main", map[string]any{"manifest_id": current.CurrentManifestId, "structure_id": current.CurrentStructureId, "style_id": current.CurrentStyleId, "control_revision": current.ControlRevision})
	if err != nil {
		return nil, err
	}
	fixed.Context = &pb.OperationContext{
		OperationId: actor.Proof.OperationId, ActorId: actor.ID,
		IdentityAdmissionRef: actor.Proof.AdmissionId, DocumentAdmissionRef: admission,
		Target: target, Sources: access.Sources, RequestFingerprint: fingerprint[:],
	}
	raw, err := json.Marshal(fixed)
````

## 第 11 步：MainReadViews.Capture（接着实现）

插入位置：第 10 步代码之后，紧接追加；完成稿约第 173 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return &amp;mainReadCapture{Fixed: fixed, Main: main}, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=6593:7114 -->
````go
	if err != nil {
		return nil, err
	}
	if err = repository.InsertMainReadContext(ctx, tx, fixed.Context.OperationId, raw); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	main, err := s.client.CaptureMain(ctx, mainReadRequest(fixed))
	if err != nil {
		return nil, err
	}
	if err = s.validateCapture(fixed, main); err != nil {
		return nil, err
	}
	if err = s.revalidate(ctx, fixed); err != nil {
		return nil, err
	}
	return &mainReadCapture{Fixed: fixed, Main: main}, nil
}

````

## 第 12 步：MainReadViews.revalidate

插入位置：第 11 步代码之后，紧接追加；完成稿约第 195 行。

本步声明：`func (s *MainReadViews) revalidate(ctx context.Context, fixed mainReadContext) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.identity.Revalidate(ctx, proof)</code> |
| <code>fixed</code> | <code>mainReadContext</code> | 在此声明中的实际使用：<code>fixed.Access</code>；<code>fixed.ExpiresUnixMs</code>；<code>fixed.SessionExpires</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.PermissionDenied, &quot;MAIN_READ_EXPIRED&quot;)</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(fixed.Access.Proofs) == 0 &#124;&#124; fixed.ExpiresUnixMs &lt;= time.Now().UnixMilli() &#124;&#124; fixed.SessionExpires &lt;= time.Now().UnixMilli()</code>。条件成立时执行 <code>return domain.Violation(domain.PermissionDenied, &quot;MAIN_READ_EXPIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>proof == nil &#124;&#124; proof.ExpiresUnixMs &lt;= time.Now().UnixMilli()</code>。条件成立时执行 <code>return domain.Violation(domain.PermissionDenied, &quot;MAIN_READ_EXPIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=7114:7698 -->
````go
func (s *MainReadViews) revalidate(ctx context.Context, fixed mainReadContext) error {
	if len(fixed.Access.Proofs) == 0 || fixed.ExpiresUnixMs <= time.Now().UnixMilli() || fixed.SessionExpires <= time.Now().UnixMilli() {
		return domain.Violation(domain.PermissionDenied, "MAIN_READ_EXPIRED")
	}
	for _, proof := range fixed.Access.Proofs {
		if proof == nil || proof.ExpiresUnixMs <= time.Now().UnixMilli() {
			return domain.Violation(domain.PermissionDenied, "MAIN_READ_EXPIRED")
		}
		if err := s.identity.Revalidate(ctx, proof); err != nil {
			return err
		}
	}
	return nil
}

````

## 第 13 步：MainReadViews.validateCapture

插入位置：第 12 步代码之后，紧接追加；完成稿约第 210 行。

本步声明：`func (s *MainReadViews) validateCapture(fixed mainReadContext, main *pb.MainCaptureReply) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>fixed</code> | <code>mainReadContext</code> | 在此声明中的实际使用：<code>validateMainReadContext(fixed)</code>；<code>mainReadBinding(fixed)</code> |
| <code>main</code> | <code>*pb.MainCaptureReply</code> | 在此声明中的实际使用：<code>validateReliableMainCapture(s.validator, mainReadBinding(fixed), main, &quot;MAIN_READ_VIEW:1&quot;, &quot;MAIN_READ&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>validateReliableMainCapture(s.validator, mainReadBinding(fixed), main, &quot;MAIN_READ_VIEW:1&quot;, &quot;MAIN_READ&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return validateReliableMainCapture(s.validator, mainReadBinding(fixed), main, &quot;MAIN_READ_VIEW:1&quot;, &quot;MAIN_READ&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=7698:8052 -->
````go
func (s *MainReadViews) validateCapture(fixed mainReadContext, main *pb.MainCaptureReply) error {
	if err := validateMainReadContext(fixed); err != nil {
		return err
	}
	return validateReliableMainCapture(s.validator, mainReadBinding(fixed), main, "MAIN_READ_VIEW:1", "MAIN_READ")
}

// main_read_commit.go：保留原实现，按业务合并文件。

````

## 第 14 步：MainReadViews.Publish

插入位置：第 13 步代码之后，紧接追加；完成稿约第 219 行。

快照物化与输出登记由调用方同一事务提交；此处不安装MAIN、不改变任何业务指针。
每次阅读版本保留实际捕获来源和VERSION资源引用，不能给可靠尾部套用旧版本编号。

本步声明：`func (s *MainReadViews) Publish(ctx context.Context, tx *sql.Tx, actor Actor, capture *mainReadCapture, admission string) (dto.MainView, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.validateCurrent(ctx, tx, fixed)</code>；<code>validateReliableMainVersion(ctx, tx, mainReadBinding(fixed), content, &quot;MAIN_READ&quot;)</code>；<code>repository.LockedDocumentResource(ctx, tx, resource, document, space)</code>；<code>repository.InsertControlledManifest(ctx, tx, document, fixed.Manifest, fixed.Structure, fixed.Style, fixed.Context.OperationId, &quot;READ_VIEW&quot;, fixed.Context.OperationId+&quot;:MAIN_READ_VIEW:1&quot;)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>s.validateCurrent(ctx, tx, fixed)</code>；<code>validateReliableMainVersion(ctx, tx, mainReadBinding(fixed), content, &quot;MAIN_READ&quot;)</code>；<code>repository.LockedDocumentResource(ctx, tx, resource, document, space)</code>；<code>repository.InsertControlledManifest(ctx, tx, document, fixed.Manifest, fixed.Structure, fixed.Style, fixed.Context.OperationId, &quot;READ_VIEW&quot;, fixed.Context.OperationId+&quot;:MAIN_READ_VIEW:1&quot;)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor.ID</code>；<code>actor.Proof</code> |
| <code>capture</code> | <code>*mainReadCapture</code> | 在此声明中的实际使用：<code>capture.Fixed</code>；<code>capture.Main</code> |
| <code>admission</code> | <code>string</code> | 在此声明中的实际使用：<code>readingMainView(ctx, tx, current, document, domain.TextID(manifest), admission)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>dto.MainView</code> | 由 <code>readingMainView(ctx, tx, current, document, domain.TextID(manifest), admission)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>view</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>readingMainView(ctx, tx, current, document, domain.TextID(manifest), admission)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>domain.Violation(domain.PermissionDenied, &quot;MAIN_READ_ACTOR_CHANGED&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;RESOURCE_NOT_READY&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>actor.ID != fixed.Actor &#124;&#124; actor.Proof.AdmissionId != fixed.Context.IdentityAdmissionRef &#124;&#124; actor.Proof.OperationId != fixed.Context.OperationId</code>。条件成立时执行 <code>return view, domain.Violation(domain.PermissionDenied, &quot;MAIN_READ_ACTOR_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return view, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return ordered&#91;i&#93; &lt; ordered&#91;j&#93;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return view, err</code>。当前函数在这里结束。
- 检查 <code>state != &quot;READY&quot;</code>。条件成立时执行 <code>return view, domain.Violation(domain.StateConflict, &quot;RESOURCE_NOT_READY&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return view, err</code>。当前函数在这里结束。

<!-- write-step: 14 bytes=8052:10130 -->
````go
// 快照物化与输出登记由调用方同一事务提交；此处不安装MAIN、不改变任何业务指针。
// 每次阅读版本保留实际捕获来源和VERSION资源引用，不能给可靠尾部套用旧版本编号。
func (s *MainReadViews) Publish(ctx context.Context, tx *sql.Tx, actor Actor, capture *mainReadCapture, admission string) (dto.MainView, error) {
	var view dto.MainView
	fixed := capture.Fixed
	if actor.ID != fixed.Actor || actor.Proof.AdmissionId != fixed.Context.IdentityAdmissionRef || actor.Proof.OperationId != fixed.Context.OperationId {
		return view, domain.Violation(domain.PermissionDenied, "MAIN_READ_ACTOR_CHANGED")
	}
	current, err := s.validateCurrent(ctx, tx, fixed)
	if err != nil {
		return view, err
	}
	document := fixed.Context.Target.DocumentId
	space, _ := domain.CanonicalID(current.SpaceId)
	resources := make(map[uint64]bool)
	for _, content := range capture.Main.Units {
		if err = validateReliableMainVersion(ctx, tx, mainReadBinding(fixed), content, "MAIN_READ"); err != nil {
			return view, err
		}
		for _, resource := range content.ResourceIds {
			resources[resource] = true
		}
	}
	// 按数值顺序锁资源，避免多单元重复引用造成锁顺序翻转。
	ordered := make([]uint64, 0, len(resources))
	for resource := range resources {
		ordered = append(ordered, resource)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	for _, resource := range ordered {
		state, err := repository.LockedDocumentResource(ctx, tx, resource, document, space)
		if err != nil {
			return view, err
		}
		if state != "READY" {
			return view, domain.Violation(domain.StateConflict, "RESOURCE_NOT_READY")
		}
	}
	manifest, err := repository.InsertControlledManifest(ctx, tx, document, fixed.Manifest, fixed.Structure, fixed.Style, fixed.Context.OperationId, "READ_VIEW", fixed.Context.OperationId+":MAIN_READ_VIEW:1")
	if err != nil {
		return view, err
	}
	for _, content := range capture.Main.Units {
		version, err := repository.InsertReadingVersion(ctx, tx, content)
		if err != nil {
````

## 第 15 步：MainReadViews.Publish（接着实现）

插入位置：第 14 步代码之后，紧接追加；完成稿约第 264 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return view, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return view, err</code>。当前函数在这里结束。
- 此处通过 <code>return readingMainView(ctx, tx, current, document, domain.TextID(manifest), admission)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=10130:10458 -->
````go
			return view, err
		}
		if err = repository.InsertInitialManifestEntry(ctx, tx, document, manifest, content.UnitId, version); err != nil {
			return view, err
		}
	}
	return readingMainView(ctx, tx, current, document, domain.TextID(manifest), admission)
}

// main_read_source.go：保留原实现，按业务合并文件。

````

## 第 16 步：MainReadViews.MainSource

插入位置：第 15 步代码之后，紧接追加；完成稿约第 275 行。

本步声明：`func (s *MainReadViews) MainSource(ctx context.Context, input *pb.CaptureMainRequest) (map[string]any, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.fixedContext(ctx, input.Context.OperationId)</code>；<code>s.revalidate(ctx, fixed)</code>；<code>s.db.BeginTx(ctx, nil)</code>；<code>s.validateCurrent(ctx, tx, fixed)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>input</code> | <code>*pb.CaptureMainRequest</code> | 在此声明中的实际使用：<code>input == nil</code>；<code>input.Context</code>；<code>proto.Equal(mainReadRequest(fixed), input)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>map&#91;string&#93;any</code> | 实际返回表达式：<code>nil</code>；<code>map&#91;string&#93;any{&quot;manifest_id&quot;: current.CurrentManifestId, &quot;structure_id&quot;: current.CurrentStructureId, &quot;style_id&quot;: current.CurrentStyleId, &quot;control_revision&quot;: current.ControlRevision …</code>（长表达式，按本步完整代码继续） |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.InvalidRequest, &quot;MAIN_SOURCE_QUERY_INVALID&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>input == nil &#124;&#124; input.Context == nil &#124;&#124; input.Context.Target == nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.InvalidRequest, &quot;MAIN_SOURCE_QUERY_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>!proto.Equal(mainReadRequest(fixed), input)</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return map&#91;string&#93;any{&quot;manifest_id&quot;: current.CurrentManifestId, &quot;structure_id&quot;: current.CurrentStructureId, &quot;style_id&quot;: current.CurrentStyleId, &quot;control_revision&quot;: current.ControlR …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=10458:11686 -->
````go
func (s *MainReadViews) MainSource(ctx context.Context, input *pb.CaptureMainRequest) (map[string]any, error) {
	if input == nil || input.Context == nil || input.Context.Target == nil {
		return nil, domain.Violation(domain.InvalidRequest, "MAIN_SOURCE_QUERY_INVALID")
	}
	fixed, err := s.fixedContext(ctx, input.Context.OperationId)
	if err != nil {
		return nil, err
	}
	if !proto.Equal(mainReadRequest(fixed), input) {
		return nil, domain.Violation(domain.StateConflict, "OPERATION_ID_CONFLICT")
	}
	if err = s.revalidate(ctx, fixed); err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	current, err := s.validateCurrent(ctx, tx, fixed)
	if err != nil {
		return nil, err
	}
	units, epoch, err := mainSourceUnits(ctx, tx, fixed.Context.Target.DocumentId, current.CurrentManifestId, fixed.Control)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return map[string]any{"manifest_id": current.CurrentManifestId, "structure_id": current.CurrentStructureId, "style_id": current.CurrentStyleId, "control_revision": current.ControlRevision, "workspace_epoch": domain.TextID(epoch), "units": units}, nil
}

````

## 第 17 步：MainReadViews.validateCurrent

插入位置：第 16 步代码之后，紧接追加；完成稿约第 308 行。

原来源锁、MAIN门闩和原准入一起复核；私有mTLS资格不能代替用户的READ资格。
发布再次调用本函数，所以捕获后发生撤权/结构切换时不会释放过时的敏感正文。

本步声明：`func (s *MainReadViews) validateCurrent(ctx context.Context, tx *sql.Tx, fixed mainReadContext) (dto.Document, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>LockSources(ctx, tx, fixed.Access.Sources)</code>；<code>ValidateAccess(ctx, tx, fixed.Access)</code>；<code>repository.LockedMergeBarrier(ctx, tx, fixed.Context.Target.DocumentId)</code>；<code>repository.MainWorkspaceEpoch(ctx, tx, fixed.Context.Target.DocumentId)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>LockSources(ctx, tx, fixed.Access.Sources)</code>；<code>ValidateAccess(ctx, tx, fixed.Access)</code>；<code>repository.LockedMergeBarrier(ctx, tx, fixed.Context.Target.DocumentId)</code>；<code>repository.MainWorkspaceEpoch(ctx, tx, fixed.Context.Target.DocumentId)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>fixed</code> | <code>mainReadContext</code> | 在此声明中的实际使用：<code>fixed.Access</code>；<code>fixed.Manifest</code>；<code>fixed.Structure</code>；<code>fixed.Style</code><br>另有 6 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>dto.Document</code> | 实际返回表达式：<code>current</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;MAIN_SOURCE_CHANGED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;MAIN_COMMIT_PENDING&quot;)</code>；<code>domain.Violation(domain.PermissionDenied, &quot;MAIN_READ_NOT_CURRENT&quot;)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return current, err</code>。当前函数在这里结束。
- 检查 <code>!fixed.Access.Permissions.Read &#124;&#124; current.MainLocked &#124;&#124; current.Lifecycle != &quot;ACTIVE&quot; &#124;&#124; 		current.CurrentManifestId != domain.TextID(fixed.Manifest) &#124;&#124; 		current.CurrentStructureI …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return current, domain.Violation(domain.StateConflict, &quot;MAIN_SOURCE_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return current, err</code>。当前函数在这里结束。
- 检查 <code>barrier.Phase != &quot;OPEN&quot;</code>。条件成立时执行 <code>return current, domain.Violation(domain.StateConflict, &quot;MAIN_COMMIT_PENDING&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return current, err</code>。当前函数在这里结束。
- 检查 <code>epoch != reliableMainEpoch(mainReadBinding(fixed))</code>。条件成立时执行 <code>return current, domain.Violation(domain.StateConflict, &quot;MAIN_SOURCE_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return current, err</code>。当前函数在这里结束。
- 检查 <code>state.State != &quot;OPEN&quot; &#124;&#124; !state.Expires.After(time.Now()) &#124;&#124; fixed.ExpiresUnixMs &lt;= time.Now().UnixMilli() &#124;&#124; fixed.SessionExpires &lt;= time.Now().UnixMilli()</code>。条件成立时执行 <code>return current, domain.Violation(domain.PermissionDenied, &quot;MAIN_READ_NOT_CURRENT&quot;)</code>。当前函数在这里结束。

<!-- write-step: 17 bytes=11686:13784 -->
````go
// 原来源锁、MAIN门闩和原准入一起复核；私有mTLS资格不能代替用户的READ资格。
// 发布再次调用本函数，所以捕获后发生撤权/结构切换时不会释放过时的敏感正文。
func (s *MainReadViews) validateCurrent(ctx context.Context, tx *sql.Tx, fixed mainReadContext) (dto.Document, error) {
	var current dto.Document
	if err := LockSources(ctx, tx, fixed.Access.Sources); err != nil {
		return current, err
	}
	current, err := ValidateAccess(ctx, tx, fixed.Access)
	if err != nil {
		return current, err
	}
	if !fixed.Access.Permissions.Read || current.MainLocked || current.Lifecycle != "ACTIVE" ||
		current.CurrentManifestId != domain.TextID(fixed.Manifest) ||
		current.CurrentStructureId != domain.TextID(fixed.Structure) ||
		current.CurrentStyleId != domain.TextID(fixed.Style) || current.ControlRevision != domain.TextID(fixed.Control) {
		return current, domain.Violation(domain.StateConflict, "MAIN_SOURCE_CHANGED")
	}
	barrier, err := repository.LockedMergeBarrier(ctx, tx, fixed.Context.Target.DocumentId)
	if err != nil {
		return current, err
	}
	if barrier.Phase != "OPEN" {
		return current, domain.Violation(domain.StateConflict, "MAIN_COMMIT_PENDING")
	}
	epoch, err := repository.MainWorkspaceEpoch(ctx, tx, fixed.Context.Target.DocumentId)
	if err != nil {
		return current, err
	}
	if epoch != reliableMainEpoch(mainReadBinding(fixed)) {
		return current, domain.Violation(domain.StateConflict, "MAIN_SOURCE_CHANGED")
	}
	actual, err := repository.OriginalAdmission(ctx, tx, fixed.Context.DocumentAdmissionRef, true)
	if err != nil {
		return current, err
	}
	if err = validateMainReadAdmission(fixed, actual); err != nil {
		return current, err
	}
	state, err := repository.LockedAdmission(ctx, tx, fixed.Context.DocumentAdmissionRef)
	if err != nil {
		return current, err
	}
	if state.State != "OPEN" || !state.Expires.After(time.Now()) || fixed.ExpiresUnixMs <= time.Now().UnixMilli() || fixed.SessionExpires <= time.Now().UnixMilli() {
		return current, domain.Violation(domain.PermissionDenied, "MAIN_READ_NOT_CURRENT")
	}
````

## 第 18 步：MainReadViews.validateCurrent（接着实现）

插入位置：第 17 步代码之后，紧接追加；完成稿约第 353 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>proof == nil &#124;&#124; proof.ExpiresUnixMs &lt;= time.Now().UnixMilli()</code>。条件成立时执行 <code>return current, domain.Violation(domain.PermissionDenied, &quot;MAIN_READ_EXPIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !equalPullSources(sources, fixed.Access.Sources)</code>。条件成立时执行 <code>return current, domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_READ_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return current, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=13784:14268 -->
````go
	for _, proof := range fixed.Access.Proofs {
		if proof == nil || proof.ExpiresUnixMs <= time.Now().UnixMilli() {
			return current, domain.Violation(domain.PermissionDenied, "MAIN_READ_EXPIRED")
		}
	}
	sources, err := repository.AdmissionSources(ctx, tx, fixed.Context.DocumentAdmissionRef)
	if err != nil || !equalPullSources(sources, fixed.Access.Sources) {
		return current, domain.Violation(domain.TemporarilyUnavailable, "MAIN_READ_BINDING_UNKNOWN")
	}
	return current, nil
}

````

## 第 19 步：validateMainReadAdmission

插入位置：第 18 步代码之后，紧接追加；完成稿约第 365 行。

本步声明：`func validateMainReadAdmission(fixed mainReadContext, actual repository.OriginalAdmissionBinding) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>fixed</code> | <code>mainReadContext</code> | 在此声明中的实际使用：<code>validateMainReadContext(fixed)</code>；<code>fixed.Context</code>；<code>fixed.Actor</code>；<code>fixed.Access</code> |
| <code>actual</code> | <code>repository.OriginalAdmissionBinding</code> | 使用 <code>repository.OriginalAdmissionBinding</code> 类型；其定义处的职责：读权限事实并持久化准入、撤权任务、成员离开门闩和来源代次。<br>在此声明中的实际使用：<code>actual.Actor</code>；<code>actual.Document</code>；<code>actual.Unit</code>；<code>actual.Identity</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_READ_BINDING_UNKNOWN&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>actual.Actor != fixed.Actor &#124;&#124; actual.Document != ctx.Target.DocumentId &#124;&#124; actual.Unit != 0 &#124;&#124; 		actual.Identity != ctx.IdentityAdmissionRef &#124;&#124; actual.Operation != ctx.OperationId  …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_READ_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(actual.Target, target) != nil &#124;&#124; !proto.Equal(target, ctx.Target)</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;MAIN_READ_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 19 bytes=14268:15127 -->
````go
func validateMainReadAdmission(fixed mainReadContext, actual repository.OriginalAdmissionBinding) error {
	if err := validateMainReadContext(fixed); err != nil {
		return err
	}
	ctx := fixed.Context
	if actual.Actor != fixed.Actor || actual.Document != ctx.Target.DocumentId || actual.Unit != 0 ||
		actual.Identity != ctx.IdentityAdmissionRef || actual.Operation != ctx.OperationId ||
		actual.Purpose != "READ_MAIN" || actual.SessionExpires != fixed.Access.Proofs[0].ExpiresUnixMs ||
		!bytes.Equal(actual.Session, fixed.Access.Proofs[0].SessionRef) {
		return domain.Violation(domain.TemporarilyUnavailable, "MAIN_READ_BINDING_UNKNOWN")
	}
	target := &pb.Target{}
	if json.Unmarshal(actual.Target, target) != nil || !proto.Equal(target, ctx.Target) {
		return domain.Violation(domain.TemporarilyUnavailable, "MAIN_READ_BINDING_UNKNOWN")
	}
	return nil
}

````

## 第 20 步：validateMainReadContext

插入位置：第 19 步代码之后，紧接追加；完成稿约第 383 行。

本步声明：`func validateMainReadContext(fixed mainReadContext) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>fixed</code> | <code>mainReadContext</code> | 在此声明中的实际使用：<code>mainReadBinding(fixed)</code>；<code>fixed.Control</code>；<code>fixed.Context</code>；<code>fixed.Access</code><br>另有 8 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>invalid</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>epoch == 0 &#124;&#124; epoch &gt; fixed.Control</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>ctx == nil &#124;&#124; ctx.Target == nil &#124;&#124; len(fixed.Access.Proofs) == 0 &#124;&#124; fixed.Access.Proofs&#91;0&#93; == nil</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>fixed.Actor == 0 &#124;&#124; ctx.ActorId != fixed.Actor &#124;&#124; ctx.DocumentAdmissionRef == &quot;&quot; &#124;&#124; 		!fixed.Access.Permissions.Read &#124;&#124; fixed.Access.Document.MainLocked &#124;&#124; 		fixed.Access.Document. …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>target.DocumentId == 0 &#124;&#124; fixed.Manifest == 0 &#124;&#124; fixed.Structure == 0 &#124;&#124; fixed.Style == 0 &#124;&#124; fixed.Control == 0 &#124;&#124; !proto.Equal(ctx.Target, target) &#124;&#124; !equalPullSources(ctx.Sources, fixed.Access.Sources)</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>document.Id != domain.TextID(target.DocumentId) &#124;&#124; 		document.CurrentManifestId != domain.TextID(fixed.Manifest) &#124;&#124; 		document.CurrentStructureId != domain.TextID(fixed.Structure)  …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>first.AdmissionId != ctx.IdentityAdmissionRef &#124;&#124; first.OperationId != ctx.OperationId</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>proof == nil &#124;&#124; proof.Target == nil &#124;&#124; proof.ActorId != fixed.Actor &#124;&#124; proof.Purpose != &quot;READ_MAIN&quot; &#124;&#124; 			proof.Target.DocumentId != target.DocumentId &#124;&#124; proof.Target.Purpose != &quot;R …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !bytes.Equal(fingerprint&#91;:&#93;, ctx.RequestFingerprint)</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>len(fixed.Versions) != len(fixed.Units)</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。

<!-- write-step: 20 bytes=15127:17549 -->
````go
func validateMainReadContext(fixed mainReadContext) error {
	invalid := domain.Violation(domain.TemporarilyUnavailable, "MAIN_READ_BINDING_UNKNOWN")
	if epoch := reliableMainEpoch(mainReadBinding(fixed)); epoch == 0 || epoch > fixed.Control {
		return invalid
	}
	ctx := fixed.Context
	if ctx == nil || ctx.Target == nil || len(fixed.Access.Proofs) == 0 || fixed.Access.Proofs[0] == nil {
		return invalid
	}
	if fixed.Actor == 0 || ctx.ActorId != fixed.Actor || ctx.DocumentAdmissionRef == "" ||
		!fixed.Access.Permissions.Read || fixed.Access.Document.MainLocked ||
		fixed.Access.Document.Lifecycle != "ACTIVE" || fixed.ExpiresUnixMs <= 0 ||
		fixed.ExpiresUnixMs > fixed.SessionExpires {
		return invalid
	}
	target := &pb.Target{DocumentId: ctx.Target.DocumentId, StructureId: fixed.Structure, WorkspaceEpoch: fixed.Control, Purpose: "READ_MAIN"}
	if target.DocumentId == 0 || fixed.Manifest == 0 || fixed.Structure == 0 || fixed.Style == 0 || fixed.Control == 0 || !proto.Equal(ctx.Target, target) || !equalPullSources(ctx.Sources, fixed.Access.Sources) {
		return invalid
	}
	document := fixed.Access.Document
	if document.Id != domain.TextID(target.DocumentId) ||
		document.CurrentManifestId != domain.TextID(fixed.Manifest) ||
		document.CurrentStructureId != domain.TextID(fixed.Structure) ||
		document.CurrentStyleId != domain.TextID(fixed.Style) || document.ControlRevision != domain.TextID(fixed.Control) {
		return invalid
	}
	first := fixed.Access.Proofs[0]
	if first.AdmissionId != ctx.IdentityAdmissionRef || first.OperationId != ctx.OperationId {
		return invalid
	}
	for _, proof := range fixed.Access.Proofs {
		if proof == nil || proof.Target == nil || proof.ActorId != fixed.Actor || proof.Purpose != "READ_MAIN" ||
			proof.Target.DocumentId != target.DocumentId || proof.Target.Purpose != "READ_MAIN" ||
			len(proof.SessionRef) != 32 || !bytes.Equal(proof.SessionRef, first.SessionRef) ||
			fixed.ExpiresUnixMs > proof.ExpiresUnixMs {
			return invalid
		}
	}
	fingerprint, err := domain.OperationFingerprint("/v1/documents/"+document.Id+"/main", map[string]any{"manifest_id": document.CurrentManifestId, "structure_id": document.CurrentStructureId, "style_id": document.CurrentStyleId, "control_revision": document.ControlRevision})
	if err != nil || !bytes.Equal(fingerprint[:], ctx.RequestFingerprint) {
		return invalid
	}
	if len(fixed.Versions) != len(fixed.Units) {
		return invalid
	}
````

## 第 21 步：validateMainReadContext（接着实现）

插入位置：第 20 步代码之后，紧接追加；完成稿约第 428 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>unit == 0 &#124;&#124; seen&#91;unit&#93; &#124;&#124; fixed.Versions&#91;unit&#93; == 0</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 21 bytes=17549:17824 -->
````go
	seen := make(map[uint64]bool, len(fixed.Units))
	for _, unit := range fixed.Units {
		if unit == 0 || seen[unit] || fixed.Versions[unit] == 0 {
			return invalid
		}
		seen[unit] = true
	}
	return nil
}

// main_source_units.go：保留原实现，按业务合并文件。

````

## 第 22 步：mainSourceUnits

插入位置：第 21 步代码之后，紧接追加；完成稿约第 440 行。

MAIN阅读、合并与管理控制共享全篇版本读取；所有字节属于同一Doc事务，不向Node开放任意版本查询。

本步声明：`func mainSourceUnits(ctx context.Context, tx *sql.Tx, document uint64, manifest string, control uint64) ([]any, uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.MainWorkspaceEpoch(ctx, tx, document)</code>；<code>repository.ManifestUnits(ctx, tx, manifest, document)</code>；<code>repository.VersionContent(ctx, tx, version, document)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.MainWorkspaceEpoch(ctx, tx, document)</code>；<code>repository.ManifestUnits(ctx, tx, manifest, document)</code>；<code>repository.VersionContent(ctx, tx, version, document)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.MainWorkspaceEpoch(ctx, tx, document)</code>；<code>repository.ManifestUnits(ctx, tx, manifest, document)</code>；<code>repository.VersionContent(ctx, tx, version, document)</code> |
| <code>manifest</code> | <code>string</code> | 在此声明中的实际使用：<code>repository.ManifestUnits(ctx, tx, manifest, document)</code> |
| <code>control</code> | <code>uint64</code> | 在此声明中的实际使用：<code>content.WorkspaceEpoch = control</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;any</code> | 实际返回表达式：<code>nil</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>0</code>；<code>epoch</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, 0, err</code>。当前函数在这里结束。
- 此处通过 <code>return result, epoch, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 22 bytes=17824:18996 -->
````go
// MAIN阅读、合并与管理控制共享全篇版本读取；所有字节属于同一Doc事务，不向Node开放任意版本查询。
func mainSourceUnits(ctx context.Context, tx *sql.Tx, document uint64, manifest string, control uint64) ([]any, uint64, error) {
	epoch, err := repository.MainWorkspaceEpoch(ctx, tx, document)
	if err != nil {
		return nil, 0, err
	}
	units, err := repository.ManifestUnits(ctx, tx, manifest, document)
	if err != nil {
		return nil, 0, err
	}
	result := make([]any, 0, len(units))
	for _, unit := range units {
		version, _ := domain.CanonicalID(unit["content_version_id"])
		content, err := repository.VersionContent(ctx, tx, version, document)
		if err != nil {
			return nil, 0, err
		}
		if err = canonicalVersion(content, version); err != nil {
			return nil, 0, err
		}
		content.WorkspaceEpoch = control
		wire, err := fixedWire(content)
		if err != nil {
			return nil, 0, err
		}
		result = append(result, map[string]any{"unit_id": unit["unit_id"], "content_version_id": unit["content_version_id"], "content": wire})
	}
	return result, epoch, nil
}

// reliable_main_capture.go：保留原实现，按业务合并文件。

````

## 第 23 步：reliableMainBinding

插入位置：第 22 步代码之后，紧接追加；完成稿约第 472 行。

reliableMainBinding只表达可靠整篇内容的固定来源。阅读准入的短期限与发布的
持久管理证明分别由各自用例验证，不能因为复用内容验证而共享授权用途。

本步声明：`type reliableMainBinding struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Context</code> | <code>*pb.OperationContext</code> | 内部协议的原操作上下文，绑定操作号、目标、身份与准入来源；恢复必须匹配原上下文，不能只认操作号。<br>序列化/校验标记：<code>`json:&quot;context&quot;`</code> |
| <code>Manifest</code> | <code>uint64</code> | 序列化/校验标记：<code>`json:&quot;manifest&quot;`</code> |
| <code>Structure</code> | <code>uint64</code> | 序列化/校验标记：<code>`json:&quot;structure&quot;`</code> |
| <code>Style</code> | <code>uint64</code> | 序列化/校验标记：<code>`json:&quot;style&quot;`</code> |
| <code>Control</code> | <code>uint64</code> | 序列化/校验标记：<code>`json:&quot;control&quot;`</code> |
| <code>Epoch</code> | <code>uint64</code> | 序列化/校验标记：<code>`json:&quot;main_epoch,omitempty&quot;`</code> |
| <code>Units</code> | <code>&#91;&#93;uint64</code> | 序列化/校验标记：<code>`json:&quot;units&quot;`</code> |
| <code>Versions</code> | <code>map&#91;uint64&#93;uint64</code> | 序列化/校验标记：<code>`json:&quot;versions&quot;`</code> |

<!-- write-step: 23 bytes=18996:19648 -->
````go
// reliableMainBinding只表达可靠整篇内容的固定来源。阅读准入的短期限与发布的
// 持久管理证明分别由各自用例验证，不能因为复用内容验证而共享授权用途。
type reliableMainBinding struct {
	Context   *pb.OperationContext `json:"context"`
	Manifest  uint64               `json:"manifest"`
	Structure uint64               `json:"structure"`
	Style     uint64               `json:"style"`
	Control   uint64               `json:"control"`
	Epoch     uint64               `json:"main_epoch,omitempty"`
	Units     []uint64             `json:"units"`
	Versions  map[uint64]uint64    `json:"versions"`
}

````

## 第 24 步：mainReadBinding

插入位置：第 23 步代码之后，紧接追加；完成稿约第 485 行。

本步声明：`func mainReadBinding(fixed mainReadContext) reliableMainBinding`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>fixed</code> | <code>mainReadContext</code> | 在此声明中的实际使用：<code>fixed.Context</code>；<code>fixed.Manifest</code>；<code>fixed.Structure</code>；<code>fixed.Style</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>reliableMainBinding</code> | 实际返回表达式：<code>reliableMainBinding{Context: fixed.Context, Manifest: fixed.Manifest, Structure: fixed.Structure, 		Style: fixed.Style, Control: fixed.Control, Epoch: fixed.MainEpoch, Units: fixed.Units, Versions: fixed.Versions}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return reliableMainBinding{Context: fixed.Context, Manifest: fixed.Manifest, Structure: fixed.Structure, 		Style: fixed.Style, Control: fixed.Control, Epoch: fixed.MainEpoch, Units: fixed.Units, Versions: fixed.Versions}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 24 bytes=19648:19939 -->
````go
func mainReadBinding(fixed mainReadContext) reliableMainBinding {
	return reliableMainBinding{Context: fixed.Context, Manifest: fixed.Manifest, Structure: fixed.Structure,
		Style: fixed.Style, Control: fixed.Control, Epoch: fixed.MainEpoch, Units: fixed.Units, Versions: fixed.Versions}
}

````

## 第 25 步：reliableMainEpoch

插入位置：第 24 步代码之后，紧接追加；完成稿约第 490 行。

旧固定上下文缺少此字段时，原实现的物理epoch就是原control。新上下文
必须从当前manifest的实际完成安装取得epoch，不能用当前元数据修订猜测。

本步声明：`func reliableMainEpoch(fixed reliableMainBinding) uint64`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>fixed</code> | <code>reliableMainBinding</code> | 在此声明中的实际使用：<code>fixed.Epoch</code>；<code>fixed.Control</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>fixed.Epoch</code>；<code>fixed.Control</code> |

**跟着本段代码走一遍：**

- 检查 <code>fixed.Epoch != 0</code>。条件成立时执行 <code>return fixed.Epoch</code>。当前函数在这里结束。
- 此处通过 <code>return fixed.Control</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 25 bytes=19939:20268 -->
````go
// 旧固定上下文缺少此字段时，原实现的物理epoch就是原control。新上下文
// 必须从当前manifest的实际完成安装取得epoch，不能用当前元数据修订猜测。
func reliableMainEpoch(fixed reliableMainBinding) uint64 {
	if fixed.Epoch != 0 {
		return fixed.Epoch
	}
	return fixed.Control
}

````

## 第 26 步：validateReliableMainCapture

插入位置：第 25 步代码之后，紧接追加；完成稿约第 499 行。

本步声明：`func validateReliableMainCapture(validator *codec.Validator, fixed reliableMainBinding, main *pb.MainCaptureReply, usage, prefix string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>validator</code> | <code>*codec.Validator</code> | 冻结协议的结构校验器，先限制请求形状，再由业务核验身份、权限、版本和事务条件。<br>在此声明中的实际使用：<code>validateFixedMergeContent(validator, content, fixed.Context.Target.DocumentId, content.UnitId, fixed.Structure)</code> |
| <code>fixed</code> | <code>reliableMainBinding</code> | 在此声明中的实际使用：<code>fixed.Context</code>；<code>fixed.Structure</code>；<code>fixed.Style</code>；<code>fixed.Control</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |
| <code>main</code> | <code>*pb.MainCaptureReply</code> | 在此声明中的实际使用：<code>main == nil</code>；<code>main.State</code>；<code>main.Cut</code>；<code>main.Units</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>usage</code> | <code>string</code> | 在此声明中的实际使用：<code>fixed.Context.OperationId+&quot;:&quot;+usage</code> |
| <code>prefix</code> | <code>string</code> | 在此声明中的实际使用：<code>prefix+&quot;_CAPTURE_INVALID&quot;</code>；<code>validateReliableMainContentBinding(fixed, content, rooms, prefix)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>invalid</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>fixed.Context == nil &#124;&#124; fixed.Context.Target == nil &#124;&#124; main == nil &#124;&#124; main.State != pb.CaptureReply_FIXED &#124;&#124; main.Cut == nil</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>main.Cut.DocumentId != fixed.Context.Target.DocumentId &#124;&#124; 		main.Cut.StructureId != fixed.Structure &#124;&#124; main.Cut.StyleId != fixed.Style &#124;&#124; 		main.Cut.ControlRevision != fixed.Control &#124;&#124; len(main.Units) != len(fixed.Units)</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>room == nil &#124;&#124; room.RoomId == 0 &#124;&#124; room.WorkspaceEpoch != reliableMainEpoch(fixed) &#124;&#124; rooms&#91;room.RoomId&#93; != nil</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>unit == 0 &#124;&#124; expected&#91;unit&#93; &#124;&#124; fixed.Versions&#91;unit&#93; == 0</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>content == nil &#124;&#124; !expected&#91;content.UnitId&#93; &#124;&#124; content.BranchId != 0 &#124;&#124; 			content.WorkspaceEpoch != fixed.Control &#124;&#124; content.OperationId != fixed.Context.OperationId &#124;&#124; 			content …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>len(rooms) != 0</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 26 bytes=20268:22210 -->
````go
func validateReliableMainCapture(validator *codec.Validator, fixed reliableMainBinding, main *pb.MainCaptureReply, usage, prefix string) error {
	invalid := domain.Violation(domain.TemporarilyUnavailable, prefix+"_CAPTURE_INVALID")
	if fixed.Context == nil || fixed.Context.Target == nil || main == nil || main.State != pb.CaptureReply_FIXED || main.Cut == nil {
		return invalid
	}
	if main.Cut.DocumentId != fixed.Context.Target.DocumentId ||
		main.Cut.StructureId != fixed.Structure || main.Cut.StyleId != fixed.Style ||
		main.Cut.ControlRevision != fixed.Control || len(main.Units) != len(fixed.Units) {
		return invalid
	}
	rooms := make(map[uint64]*pb.RoomCut, len(main.Cut.Rooms))
	for _, room := range main.Cut.Rooms {
		if room == nil || room.RoomId == 0 || room.WorkspaceEpoch != reliableMainEpoch(fixed) || rooms[room.RoomId] != nil {
			return invalid
		}
		rooms[room.RoomId] = room
	}
	if err := validateMainCaptureDigest(main); err != nil {
		return err
	}
	expected := make(map[uint64]bool, len(fixed.Units))
	for _, unit := range fixed.Units {
		if unit == 0 || expected[unit] || fixed.Versions[unit] == 0 {
			return invalid
		}
		expected[unit] = true
	}
	for _, content := range main.Units {
		if content == nil || !expected[content.UnitId] || content.BranchId != 0 ||
			content.WorkspaceEpoch != fixed.Control || content.OperationId != fixed.Context.OperationId ||
			content.CaptureId != fixed.Context.OperationId+":"+usage+":"+domain.TextID(content.UnitId) {
			return invalid
		}
		delete(expected, content.UnitId)
		if err := validateFixedMergeContent(validator, content, fixed.Context.Target.DocumentId, content.UnitId, fixed.Structure); err != nil {
			return err
		}
		if err := validateReliableMainContentBinding(fixed, content, rooms, prefix); err != nil {
			return err
		}
	}
	if len(rooms) != 0 {
		return invalid
	}
	return nil
}

// reliable_main_content.go：保留原实现，按业务合并文件。

````

## 第 27 步：validateReliableMainContentBinding

插入位置：第 26 步代码之后，紧接追加；完成稿约第 548 行。

全篇摘要自洽仍不等于归属正确：每个单元必须消费唯一真实room cut，或明确引用原固定版本。
provenance的顶层来源同时绑定原业务清单、单元基线和本次操作号。

本步声明：`func validateReliableMainContentBinding(fixed reliableMainBinding, content *pb.FixedContent, rooms map[uint64]*pb.RoomCut, prefix string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>fixed</code> | <code>reliableMainBinding</code> | 在此声明中的实际使用：<code>fixed.Versions</code>；<code>fixed.Manifest</code>；<code>fixed.Context</code> |
| <code>content</code> | <code>*pb.FixedContent</code> | 在此声明中的实际使用：<code>content.CutJsonUtf8</code>；<code>content.UnitId</code>；<code>content.ProvenanceJsonUtf8</code>；<code>content.LineageRef</code> |
| <code>rooms</code> | <code>map&#91;uint64&#93;*pb.RoomCut</code> | 在此声明中的实际使用：<code>room := rooms&#91;roomID&#93;</code>；<code>delete(rooms, roomID)</code> |
| <code>prefix</code> | <code>string</code> | 在此声明中的实际使用：<code>prefix+&quot;_CONTENT_BINDING_INVALID&quot;</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>invalid</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !bytes.Equal(canonical, content.CutJsonUtf8)</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>!ok</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>ok</code>。
- 检查 <code>err != nil &#124;&#124; room == nil &#124;&#124; len(cut) != 3 &#124;&#124; cut&#91;&quot;workspace_epoch&quot;&#93; != domain.TextID(room.WorkspaceEpoch) &#124;&#124; cut&#91;&quot;durable_seq&quot;&#93; != domain.TextID(room.DurableSeq)</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>len(cut) != 2 &#124;&#124; cut&#91;&quot;content_version_id&quot;&#93; != domain.TextID(fixed.Versions&#91;content.UnitId&#93;) &#124;&#124; !ok &#124;&#124; ref == &quot;&quot;</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>!ok &#124;&#124; provenance&#91;&quot;base_manifest_id&quot;&#93; != domain.TextID(fixed.Manifest) &#124;&#124; 		provenance&#91;&quot;base_content_version_id&quot;&#93; != domain.TextID(fixed.Versions&#91;content.UnitId&#93;) &#124;&#124; 		provenance&#91;&quot; …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 27 bytes=22210:23946 -->
````go
// 全篇摘要自洽仍不等于归属正确：每个单元必须消费唯一真实room cut，或明确引用原固定版本。
// provenance的顶层来源同时绑定原业务清单、单元基线和本次操作号。
func validateReliableMainContentBinding(fixed reliableMainBinding, content *pb.FixedContent, rooms map[uint64]*pb.RoomCut, prefix string) error {
	invalid := domain.Violation(domain.TemporarilyUnavailable, prefix+"_CONTENT_BINDING_INVALID")
	cutValue, err := codec.Decode(content.CutJsonUtf8)
	if err != nil {
		return invalid
	}
	canonical, err := codec.Canonical(cutValue)
	if err != nil || !bytes.Equal(canonical, content.CutJsonUtf8) {
		return invalid
	}
	cut, ok := cutValue.(map[string]any)
	if !ok {
		return invalid
	}
	if roomText, ok := cut["room_id"].(string); ok {
		roomID, err := domain.CanonicalID(roomText)
		room := rooms[roomID]
		if err != nil || room == nil || len(cut) != 3 || cut["workspace_epoch"] != domain.TextID(room.WorkspaceEpoch) || cut["durable_seq"] != domain.TextID(room.DurableSeq) {
			return invalid
		}
		delete(rooms, roomID)
	} else {
		ref, ok := cut["source_ref"].(string)
		if len(cut) != 2 || cut["content_version_id"] != domain.TextID(fixed.Versions[content.UnitId]) || !ok || ref == "" {
			return invalid
		}
	}
	value, err := codec.Decode(content.ProvenanceJsonUtf8)
	if err != nil {
		return invalid
	}
	provenance, ok := value.(map[string]any)
	if !ok || provenance["base_manifest_id"] != domain.TextID(fixed.Manifest) ||
		provenance["base_content_version_id"] != domain.TextID(fixed.Versions[content.UnitId]) ||
		provenance["source_operation_id"] != fixed.Context.OperationId || provenance["lineage_id"] != content.LineageRef {
		return invalid
	}
	return nil
}

````

## 第 28 步：validateReliableMainVersion

插入位置：第 27 步代码之后，紧接追加；完成稿约第 590 行。

本步声明：`func validateReliableMainVersion(ctx context.Context, tx *sql.Tx, fixed reliableMainBinding, content *pb.FixedContent, prefix string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.VersionContent(ctx, tx, version, fixed.Context.Target.DocumentId)</code>；<code>repository.LockedCollaborationRoom(ctx, tx, key)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.VersionContent(ctx, tx, version, fixed.Context.Target.DocumentId)</code>；<code>repository.LockedCollaborationRoom(ctx, tx, key)</code> |
| <code>fixed</code> | <code>reliableMainBinding</code> | 在此声明中的实际使用：<code>fixed.Versions</code>；<code>fixed.Context</code>；<code>fixed.Structure</code>；<code>reliableMainEpoch(fixed)</code> |
| <code>content</code> | <code>*pb.FixedContent</code> | 在此声明中的实际使用：<code>content.UnitId</code>；<code>content.LineageRef</code>；<code>content.ProvenanceJsonUtf8</code>；<code>content.CutJsonUtf8</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>prefix</code> | <code>string</code> | 在此声明中的实际使用：<code>prefix+&quot;_VERSION_BINDING_INVALID&quot;</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>invalid</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>original.UnitId != content.UnitId &#124;&#124; original.StructureId != fixed.Structure &#124;&#124; original.LineageRef != content.LineageRef</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>!oldOK &#124;&#124; !nextOK</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !bytes.Equal(accepted, current)</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(content.CutJsonUtf8, &amp;cut) != nil</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>cut&#91;&quot;room_id&quot;&#93; == &quot;&quot;</code>。
- 检查 <code>cut&#91;&quot;source_ref&quot;&#93; != original.CaptureId &#124;&#124; !bytes.Equal(original.BodyHash, content.BodyHash)</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 28 bytes=23946:25597 -->
````go
func validateReliableMainVersion(ctx context.Context, tx *sql.Tx, fixed reliableMainBinding, content *pb.FixedContent, prefix string) error {
	invalid := domain.Violation(domain.TemporarilyUnavailable, prefix+"_VERSION_BINDING_INVALID")
	version := fixed.Versions[content.UnitId]
	original, err := repository.VersionContent(ctx, tx, version, fixed.Context.Target.DocumentId)
	if err != nil {
		return err
	}
	if err = canonicalVersion(original, version); err != nil {
		return err
	}
	if original.UnitId != content.UnitId || original.StructureId != fixed.Structure || original.LineageRef != content.LineageRef {
		return invalid
	}
	oldValue, err := codec.Decode(original.ProvenanceJsonUtf8)
	if err != nil {
		return invalid
	}
	newValue, err := codec.Decode(content.ProvenanceJsonUtf8)
	if err != nil {
		return invalid
	}
	old, oldOK := oldValue.(map[string]any)
	next, nextOK := newValue.(map[string]any)
	if !oldOK || !nextOK {
		return invalid
	}
	accepted, err := codec.Canonical(old["accepted_captures"])
	if err != nil {
		return invalid
	}
	current, err := codec.Canonical(next["accepted_captures"])
	if err != nil || !bytes.Equal(accepted, current) {
		return invalid
	}
	var cut map[string]string
	if json.Unmarshal(content.CutJsonUtf8, &cut) != nil {
		return invalid
	}
	if cut["room_id"] == "" {
		if cut["source_ref"] != original.CaptureId || !bytes.Equal(original.BodyHash, content.BodyHash) {
			return invalid
		}
		return nil
	}
	// 只读取Doc已经分配的真实房间，不在读取时创建或重置一个MAIN房间。
	// 管理元数据可以增加control而不安装新MAIN；房间编号必须仍绑定原物理代次。
````

## 第 29 步：validateReliableMainVersion（接着实现）

插入位置：第 28 步代码之后，紧接追加；完成稿约第 636 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(raw, &amp;room) != nil &#124;&#124; room&#91;&quot;room_id&quot;&#93; != cut&#91;&quot;room_id&quot;&#93;</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 29 bytes=25597:25965 -->
````go
	key := "MAIN:" + domain.TextID(content.UnitId) + ":" + domain.TextID(content.UnitId) + ":" + domain.TextID(reliableMainEpoch(fixed))
	raw, err := repository.LockedCollaborationRoom(ctx, tx, key)
	if err != nil {
		return err
	}
	var room map[string]string
	if json.Unmarshal(raw, &room) != nil || room["room_id"] != cut["room_id"] {
		return invalid
	}
	return nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/logic/document/main_readings_service.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/02-workspace-content.md)。
