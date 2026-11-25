# 分步编写 `services/document/internal/logic/document/draft_access_service.go`

先按 [D02 · 创建工作区与可靠正文阅读](../lessons/document/02-workspace-content.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/203-draft-access-service.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

为拉取与丢弃草稿提供固定来源装配及准备/安装结果形状。

两个动作共享数据协议，但资格和最终写入不同，不能把预览误当作支线控制或送审。

写之前先按这条执行路线理解各段的连接：

1. draftSourceScope带入原工作区与MAIN依据
2. readDraftSource只使用调用方事务读取正文
3. discardPreviewRequest把原上下文投给协作服务
4. draftPrepared与draftInstall分开计算候选和已提交安装。

本文件需要守住的边界：

- 预览不占支线控制位置，不生成送审FLUSH
- 不能换连接重取可能已变化的正文快照。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/logic/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/logic/document/draft_access_service.go') -and (Get-Item -LiteralPath 'services/document/internal/logic/document/draft_access_service.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/logic/document/draft_access_service.go')) { New-Item -ItemType File -Path 'services/document/internal/logic/document/draft_access_service.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:427 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	codec "collaboration/contracts/content"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"context"
	"database/sql"
)

// draft_controls.go：保留原实现，按业务合并文件。

````

## 第 2 步：DraftControlClient

插入位置：第 1 步代码之后，紧接追加；完成稿约第 15 行。

本步声明：`type DraftControlClient interface {`。

**这一段为什么这样写：** Go负责业务和事务，Node负责真实协作文档；此接口隔离预览、准备和安装三个阶段。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Preview</code> | <code>(context.Context, *pb.DraftPreviewRequest) (*pb.DraftPreviewReply, error)</code> | ctx与固定预览请求送往Node，返回预览引用/摘要；首次确定的输入后续原号复用。 |
| <code>FixedDraft</code> | <code>(context.Context, *pb.DraftPreviewRequest) (&#91;&#93;byte, error)</code> | 按原预览请求读取固定草稿字节，Doc继续校验内容及摘要。 |
| <code>Prepare</code> | <code>(context.Context, any) (&#91;&#93;byte, error)</code> | ctx与对应准备请求交给Node，返回待解析的准备事实字节；不是正文安装完成。 |
| <code>Apply</code> | <code>(context.Context, *pb.DraftResolutionRequest) (*pb.OperationResult, error)</code> | ctx与固定安装请求提交给Node，返回原操作结果；未知结果继续查询原号。 |

<!-- write-step: 2 bytes=427:746 -->
````go
type DraftControlClient interface {
	Preview(context.Context, *pb.DraftPreviewRequest) (*pb.DraftPreviewReply, error)
	FixedDraft(context.Context, *pb.DraftPreviewRequest) ([]byte, error)
	Prepare(context.Context, any) ([]byte, error)
	Apply(context.Context, *pb.DraftResolutionRequest) (*pb.OperationResult, error)
}

````

## 第 3 步：DraftControls

插入位置：第 2 步代码之后，紧接追加；完成稿约第 22 行。

本步声明：`type DraftControls struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。 |
| <code>access</code> | <code>*Access</code> | 使用 <code>Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。 |
| <code>client</code> | <code>DraftControlClient</code> | 使用 <code>DraftControlClient</code> 类型；其定义处的职责：Go负责业务和事务，Node负责真实协作文档；此接口隔离预览、准备和安装三个阶段。 |
| <code>validator</code> | <code>*codec.Validator</code> | 冻结协议的结构校验器，先限制请求形状，再由业务核验身份、权限、版本和事务条件。 |
| <code>instance</code> | <code>string</code> | 当前服务执行实例标识，用于领取/写回持久任务并核对所有者；不是用户操作号，重启后由新实例接管原操作。 |

<!-- write-step: 3 bytes=746:912 -->
````go
type DraftControls struct {
	db        *sql.DB
	identity  *Identity
	access    *Access
	client    DraftControlClient
	validator *codec.Validator
	instance  string
}

````

## 第 4 步：discardPreviewContext

插入位置：第 3 步代码之后，紧接追加；完成稿约第 31 行。

原上下文与恢复进度分开保存；预览不取得支线控制占位，也不伪造送审操作/FLUSH。

本步声明：`type discardPreviewContext struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Actor</code> | <code>uint64</code> | 请求预览的账号。 |
| <code>SessionExpires</code> | <code>int64</code> | 账号会话的到期时间，以协议约定的时间值传递；下游准入不能延长原会话有效期。 |
| <code>Intent</code> | <code>domain.DiscardPreviewIntent</code> | 使用 <code>domain.DiscardPreviewIntent</code> 类型；其定义处的职责：预览原意图只包含调用方声明的支线和结构；epoch/lineage必须由真实数据库读取，不能由浏览器补齐。 |
| <code>Access</code> | <code>DocumentAccess</code> | 使用 <code>DocumentAccess</code> 类型；其定义处的职责：把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。 |
| <code>Context</code> | <code>*pb.OperationContext</code> | 内部协议的原操作上下文，绑定操作号、目标、身份与准入来源；恢复必须匹配原上下文，不能只认操作号。 |
| <code>Scope</code> | <code>draftSourceScope</code> | 预览绑定的文档、单元、房间和版本上下文。 |

<!-- write-step: 4 bytes=912:1256 -->
````go
// 原上下文与恢复进度分开保存；预览不取得支线控制占位，也不伪造送审操作/FLUSH。
type discardPreviewContext struct {
	Actor          uint64
	SessionExpires int64
	Intent         domain.DiscardPreviewIntent
	Access         DocumentAccess
	Context        *pb.OperationContext
	Scope          draftSourceScope
}

````

## 第 5 步：NewDraftControls

插入位置：第 4 步代码之后，紧接追加；完成稿约第 41 行。

本步声明：`func NewDraftControls(db *sql.DB, identity *Identity, access *Access, client DraftControlClient, validator *codec.Validator, instance string) *DraftControls`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。<br>在此声明中的实际使用：<code>identity: identity</code> |
| <code>access</code> | <code>*Access</code> | 使用 <code>Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。<br>在此声明中的实际使用：<code>access: access</code> |
| <code>client</code> | <code>DraftControlClient</code> | 使用 <code>DraftControlClient</code> 类型；其定义处的职责：Go负责业务和事务，Node负责真实协作文档；此接口隔离预览、准备和安装三个阶段。<br>在此声明中的实际使用：<code>client: client</code> |
| <code>validator</code> | <code>*codec.Validator</code> | 冻结协议的结构校验器，先限制请求形状，再由业务核验身份、权限、版本和事务条件。<br>在此声明中的实际使用：<code>validator: validator</code> |
| <code>instance</code> | <code>string</code> | 在此声明中的实际使用：<code>instance: instance</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*DraftControls</code> | 实际返回表达式：<code>&amp;DraftControls{db: db, identity: identity, access: access, client: client, validator: validator, instance: instance}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;DraftControls{db: db, identity: identity, access: access, client: client, validator: validator, instance: instance}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1256:1543 -->
````go
func NewDraftControls(db *sql.DB, identity *Identity, access *Access, client DraftControlClient, validator *codec.Validator, instance string) *DraftControls {
	return &DraftControls{db: db, identity: identity, access: access, client: client, validator: validator, instance: instance}
}

````

## 第 6 步：discardPreviewRequest

插入位置：第 5 步代码之后，紧接追加；完成稿约第 45 行。

本步声明：`func discardPreviewRequest(fixed discardPreviewContext) *pb.DraftPreviewRequest`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>fixed</code> | <code>discardPreviewContext</code> | 在此声明中的实际使用：<code>fixed.Context</code>；<code>fixed.Scope</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.DraftPreviewRequest</code> | 实际返回表达式：<code>&amp;pb.DraftPreviewRequest{Context: fixed.Context, ExpectedBranchId: fixed.Scope.Workspace.Workspace, ExpectedLineageId: fixed.Scope.Workspace.Lineage, ExpectedStructureId: fixed.Scope.Structure}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;pb.DraftPreviewRequest{Context: fixed.Context, ExpectedBranchId: fixed.Scope.Workspace.Workspace, ExpectedLineageId: fixed.Scope.Workspace.Lineage, ExpectedStructureId: fixed.Scope.Structure}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1543:1894 -->
````go
func discardPreviewRequest(fixed discardPreviewContext) *pb.DraftPreviewRequest {
	return &pb.DraftPreviewRequest{Context: fixed.Context, ExpectedBranchId: fixed.Scope.Workspace.Workspace, ExpectedLineageId: fixed.Scope.Workspace.Lineage, ExpectedStructureId: fixed.Scope.Structure}
}

// draft_source.go：保留原实现，按业务合并文件。

````

## 第 7 步：draftSourceScope

插入位置：第 6 步代码之后，紧接追加；完成稿约第 51 行。

每个用例先核验自己的授权与占位规则。正文装配只使用该调用方TX，不能隐式跨池重新取快照。

本步声明：`type draftSourceScope struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Document</code> | <code>uint64</code> | 目标文档编号。 |
| <code>Unit</code> | <code>uint64</code> | 目标协作单元编号。 |
| <code>Room</code> | <code>uint64</code> | 目标支线当前物理房间。 |
| <code>Manifest</code> | <code>uint64</code> | 预览固定的主线清单。 |
| <code>Structure</code> | <code>uint64</code> | 预览固定的结构版本。 |
| <code>Style</code> | <code>uint64</code> | 预览固定的样式版本。 |
| <code>Control</code> | <code>uint64</code> | 预览固定的主线控制上下文值，安装时必须和原请求一致。 |
| <code>MainEpoch</code> | <code>uint64</code> | 序列化/校验标记：<code>`json:&quot;main_epoch,omitempty&quot;`</code> |
| <code>Workspace</code> | <code>domain.EditWorkspace</code> | 使用 <code>domain.EditWorkspace</code> 类型；其定义处的职责：支线、准入和票据各自保留原事实；仓储扫描不把缺失支线或过期票据默认成有效。 |

<!-- write-step: 7 bytes=1894:2302 -->
````go
// 每个用例先核验自己的授权与占位规则。正文装配只使用该调用方TX，不能隐式跨池重新取快照。
type draftSourceScope struct {
	Document, Unit, Room, Manifest, Structure, Style, Control uint64
	MainEpoch                                                 uint64 `json:"main_epoch,omitempty"`
	Workspace                                                 domain.EditWorkspace
}

````

## 第 8 步：draftMainEpoch

插入位置：第 7 步代码之后，紧接追加；完成稿约第 58 行。

本步声明：`func draftMainEpoch(fixed draftSourceScope) uint64`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>fixed</code> | <code>draftSourceScope</code> | 在此声明中的实际使用：<code>fixed.MainEpoch</code>；<code>fixed.Control</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>fixed.MainEpoch</code>；<code>fixed.Control</code> |

**跟着本段代码走一遍：**

- 检查 <code>fixed.MainEpoch != 0</code>。条件成立时执行 <code>return fixed.MainEpoch</code>。当前函数在这里结束。
- 此处通过 <code>return fixed.Control</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=2302:2543 -->
````go
func draftMainEpoch(fixed draftSourceScope) uint64 {
	if fixed.MainEpoch != 0 {
		return fixed.MainEpoch
	}
	// 已持久旧上下文的缺省字段对应当时原control，不按当前Doc控制修订重新解释。
	return fixed.Control
}

````

## 第 9 步：readDraftSource

插入位置：第 8 步代码之后，紧接追加；完成稿约第 66 行。

本步声明：`func readDraftSource(ctx context.Context, tx *sql.Tx, fixed draftSourceScope) (map[string]any, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.MainWorkspaceEpoch(ctx, tx, fixed.Document)</code>；<code>repository.VersionContent(ctx, tx, fixed.Workspace.BaseVersion, fixed.Document)</code>；<code>repository.BranchBootstrapLineage(ctx, tx, fixed.Workspace.Workspace)</code>；<code>repository.MainUnitVersion(ctx, tx, fixed.Document, fixed.Unit, fixed.Manifest)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.MainWorkspaceEpoch(ctx, tx, fixed.Document)</code>；<code>repository.VersionContent(ctx, tx, fixed.Workspace.BaseVersion, fixed.Document)</code>；<code>repository.BranchBootstrapLineage(ctx, tx, fixed.Workspace.Workspace)</code>；<code>repository.MainUnitVersion(ctx, tx, fixed.Document, fixed.Unit, fixed.Manifest)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>fixed</code> | <code>draftSourceScope</code> | 在此声明中的实际使用：<code>fixed.Document</code>；<code>draftMainEpoch(fixed)</code>；<code>fixed.Control</code>；<code>fixed.Workspace</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>map&#91;string&#93;any</code> | 实际返回表达式：<code>nil</code>；<code>map&#91;string&#93;any{ 		&quot;room_id&quot;:          domain.TextID(fixed.Room), 		&quot;main_manifest_id&quot;: domain.TextID(fixed.Manifest), &quot;main_version_id&quot;: domain.TextID(version), 		&quot;structure_id&quot;: d …</code>（长表达式，按本步完整代码继续） |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;MAIN_SOURCE_CHANGED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>epoch != draftMainEpoch(fixed) &#124;&#124; epoch &gt; fixed.Control</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;MAIN_SOURCE_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。

<!-- write-step: 9 bytes=2543:3947 -->
````go
func readDraftSource(ctx context.Context, tx *sql.Tx, fixed draftSourceScope) (map[string]any, error) {
	epoch, err := repository.MainWorkspaceEpoch(ctx, tx, fixed.Document)
	if err != nil {
		return nil, err
	}
	if epoch != draftMainEpoch(fixed) || epoch > fixed.Control {
		return nil, domain.Violation(domain.StateConflict, "MAIN_SOURCE_CHANGED")
	}
	baseline, err := repository.VersionContent(ctx, tx, fixed.Workspace.BaseVersion, fixed.Document)
	if err != nil {
		return nil, err
	}
	if err = canonicalVersion(baseline, fixed.Workspace.BaseVersion); err != nil {
		return nil, err
	}
	_, _, provenance, err := repository.BranchBootstrapLineage(ctx, tx, fixed.Workspace.Workspace)
	if err != nil {
		return nil, err
	}
	branchBaseline, err := domain.BranchBaselineBody(baseline.BodyJsonUtf8)
	if err != nil {
		return nil, err
	}
	body, err := codec.Decode(branchBaseline)
	if err != nil {
		return nil, err
	}
	sources, err := codec.Decode(provenance)
	if err != nil {
		return nil, err
	}
	version, err := repository.MainUnitVersion(ctx, tx, fixed.Document, fixed.Unit, fixed.Manifest)
	if err != nil {
		return nil, err
	}
	main, err := repository.VersionContent(ctx, tx, version, fixed.Document)
	if err != nil {
		return nil, err
	}
	if err = canonicalVersion(main, version); err != nil {
		return nil, err
	}
	main.WorkspaceEpoch = fixed.Control
	wire, err := fixedWire(main)
	if err != nil {
````

## 第 10 步：readDraftSource（接着实现）

插入位置：第 9 步代码之后，紧接追加；完成稿约第 111 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return nil, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return map&#91;string&#93;any{ 		&quot;room_id&quot;:          domain.TextID(fixed.Room), 		&quot;main_manifest_id&quot;: domain.TextID(fixed.Manifest), &quot;main_version_id&quot;: domain.TextID(version), 		&quot;structure …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=3947:4693 -->
````go
		return nil, err
	}
	return map[string]any{
		"room_id":          domain.TextID(fixed.Room),
		"main_manifest_id": domain.TextID(fixed.Manifest), "main_version_id": domain.TextID(version),
		"structure_id": domain.TextID(fixed.Structure), "style_id": domain.TextID(fixed.Style),
		"control_revision": domain.TextID(fixed.Control), "workspace_epoch": domain.TextID(epoch),
		"main": wire,
		"lineage": map[string]any{
			"lineage_id":       domain.TextID(fixed.Workspace.Lineage),
			"base_manifest_id": domain.TextID(fixed.Workspace.BaseManifest), "base_version_id": domain.TextID(fixed.Workspace.BaseVersion),
			"body": body, "provenance": sources,
		},
	}, nil
}

// draft_transition_protocol.go：保留原实现，按业务合并文件。

````

## 第 11 步：draftPrepared

插入位置：第 10 步代码之后，紧接追加；完成稿约第 129 行。

拉取与丢弃共用“固定候选→准备→安装”的私有事实形状；业务资格和最终数据库动作各自编排。
保留已有JSON字段，类型整理不会改历史PULL或DISCARD原固定字节。

本步声明：`type draftPrepared struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>State</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;state&quot;`</code> |
| <code>Code</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;code&quot;`</code> |
| <code>Preview</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;preview_cut_ref&quot;`</code> |
| <code>Digest</code> | <code>&#91;&#93;byte</code> | 序列化/校验标记：<code>`json:&quot;input_digest&quot;`</code> |
| <code>Cut</code> | <code>map&#91;string&#93;string</code> | 序列化/校验标记：<code>`json:&quot;draft_cut&quot;`</code> |
| <code>Candidate</code> | <code>*pb.FixedContent</code> | Node准备完成后固定的待安装草稿候选。 |
| <code>Main</code> | <code>*pb.FixedContent</code> | 候选所依据的固定主线输入。 |

<!-- write-step: 11 bytes=4693:5252 -->
````go
// 拉取与丢弃共用“固定候选→准备→安装”的私有事实形状；业务资格和最终数据库动作各自编排。
// 保留已有JSON字段，类型整理不会改历史PULL或DISCARD原固定字节。
type draftPrepared struct {
	State           string            `json:"state"`
	Code            string            `json:"code"`
	Preview         string            `json:"preview_cut_ref"`
	Digest          []byte            `json:"input_digest"`
	Cut             map[string]string `json:"draft_cut"`
	Candidate, Main *pb.FixedContent
}

````

## 第 12 步：draftInstall

插入位置：第 11 步代码之后，紧接追加；完成稿约第 140 行。

本步声明：`type draftInstall struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Context</code> | <code>*pb.OperationContext</code> | 内部协议的原操作上下文，绑定操作号、目标、身份与准入来源；恢复必须匹配原上下文，不能只认操作号。<br>序列化/校验标记：<code>`json:&quot;context&quot;`</code> |
| <code>Branch</code> | <code>uint64</code> | 要推进的支线编号。 |
| <code>OldRoom</code> | <code>uint64</code> | 被替代的原物理房间。 |
| <code>NewRoom</code> | <code>uint64</code> | 安装候选的新物理房间。 |
| <code>Epoch</code> | <code>uint64</code> | 新工作区代次；旧连接不能继续写新工作区。 |
| <code>Lineage</code> | <code>uint64</code> | 安装对应的新谱系编号，用于追溯基线和来源。 |
| <code>Preview</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;preview&quot;`</code> |
| <code>Digest</code> | <code>&#91;&#93;byte</code> | 序列化/校验标记：<code>`json:&quot;digest&quot;`</code> |
| <code>Content</code> | <code>*pb.FixedContent</code> | 序列化/校验标记：<code>`json:&quot;content&quot;`</code> |
| <code>LineageContext</code> | <code>map&#91;string&#93;any</code> | 序列化/校验标记：<code>`json:&quot;lineage_context&quot;`</code> |

<!-- write-step: 12 bytes=5252:5721 -->
````go
type draftInstall struct {
	Context                                  *pb.OperationContext `json:"context"`
	Branch, OldRoom, NewRoom, Epoch, Lineage uint64
	Preview                                  string           `json:"preview"`
	Digest                                   []byte           `json:"digest"`
	Content                                  *pb.FixedContent `json:"content"`
	LineageContext                           map[string]any   `json:"lineage_context"`
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/logic/document/draft_access_service.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/02-workspace-content.md)。
