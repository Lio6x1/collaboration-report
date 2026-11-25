# 分步编写 `services/document/internal/logic/document/publication_service.go`

先按 [D07 · 正式发布与当前受众](../lessons/document/07-publication.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/220-publication-service.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

创建正式版本、切历史正式版或撤销发布，并恢复可靠MAIN捕获。

发布必须固定可靠MAIN切点，不能从编辑草稿或过时数据库尾部随意取内容。

写之前先按这条执行路线理解各段的连接：

1. Prepare固定意图与原证明
2. recoverPublication在SQL锁外捕获
3. commit复核任务、权限与切点后追加版本
4. applyPublication原子写指针和outbox
5. OperationResult供内部收尾。

本文件需要守住的边界：

- 业务成功后的系统关闭无需用户在线，浏览器Progress仍需当前管理资格
- 未知捕获不能生成第二个发布原号。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/logic/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/logic/document/publication_service.go') -and (Get-Item -LiteralPath 'services/document/internal/logic/document/publication_service.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/logic/document/publication_service.go')) { New-Item -ItemType File -Path 'services/document/internal/logic/document/publication_service.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:700 -->
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
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"sort"
	"time"
)

// publication.go：保留原实现，按业务合并文件。

````

## 第 2 步：Publications

插入位置：第 1 步代码之后，紧接追加；完成稿约第 28 行。

本步声明：`type Publications struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。 |
| <code>access</code> | <code>*Access</code> | 使用 <code>Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。 |
| <code>client</code> | <code>MergeClient</code> | 使用 <code>MergeClient</code> 类型；其定义处的职责：合并的Node参与者接口：计算、屏障准备和安装分别有明确结果，SQL提交未知时不得换新输入重试。 |
| <code>validator</code> | <code>*codec.Validator</code> | 冻结协议的结构校验器，先限制请求形状，再由业务核验身份、权限、版本和事务条件。 |
| <code>instance</code> | <code>string</code> | 当前服务执行实例标识，用于领取/写回持久任务并核对所有者；不是用户操作号，重启后由新实例接管原操作。 |

<!-- write-step: 2 bytes=700:858 -->
````go
type Publications struct {
	db        *sql.DB
	identity  *Identity
	access    *Access
	client    MergeClient
	validator *codec.Validator
	instance  string
}

````

## 第 3 步：publicationContext

插入位置：第 2 步代码之后，紧接追加；完成稿约第 37 行。

本步声明：`type publicationContext struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Actor</code> | <code>uint64</code> | 序列化/校验标记：<code>`json:&quot;actor&quot;`</code> |
| <code>Document</code> | <code>uint64</code> | 序列化/校验标记：<code>`json:&quot;document&quot;`</code> |
| <code>SessionExpires</code> | <code>int64</code> | 账号会话的到期时间，以协议约定的时间值传递；下游准入不能延长原会话有效期。<br>序列化/校验标记：<code>`json:&quot;session_expires&quot;`</code> |
| <code>Intent</code> | <code>domain.PublicationIntent</code> | 使用 <code>domain.PublicationIntent</code> 类型；其定义处的职责：PublicationIntent固定发布动作与页面看到的水位。它不包含正文或数据库依赖， 发布正文必须来自可靠MAIN切点，历史版归属必须由最终业务事务核验。<br>序列化/校验标记：<code>`json:&quot;intent&quot;`</code> |
| <code>Access</code> | <code>DocumentAccess</code> | 使用 <code>DocumentAccess</code> 类型；其定义处的职责：把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。<br>序列化/校验标记：<code>`json:&quot;access&quot;`</code> |
| <code>Main</code> | <code>reliableMainBinding</code> | 序列化/校验标记：<code>`json:&quot;main&quot;`</code> |
| <code>Path</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;path&quot;`</code> |

<!-- write-step: 3 bytes=858:1299 -->
````go
type publicationContext struct {
	Actor          uint64                   `json:"actor"`
	Document       uint64                   `json:"document"`
	SessionExpires int64                    `json:"session_expires"`
	Intent         domain.PublicationIntent `json:"intent"`
	Access         DocumentAccess           `json:"access"`
	Main           reliableMainBinding      `json:"main"`
	Path           string                   `json:"path"`
}

````

## 第 4 步：NewPublications

插入位置：第 3 步代码之后，紧接追加；完成稿约第 47 行。

本步声明：`func NewPublications(db *sql.DB, identity *Identity, access *Access, client MergeClient, validator *codec.Validator, instance string) *Publications`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |
| <code>identity</code> | <code>*Identity</code> | 使用 <code>Identity</code> 类型；其定义处的职责：封装账号服务的会话核验与身份准入证明获取、续期和复核；它确认调用者身份与组织能力，文档ACL仍由Access结合文档事实判断。<br>在此声明中的实际使用：<code>identity: identity</code> |
| <code>access</code> | <code>*Access</code> | 使用 <code>Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。<br>在此声明中的实际使用：<code>access: access</code> |
| <code>client</code> | <code>MergeClient</code> | 使用 <code>MergeClient</code> 类型；其定义处的职责：合并的Node参与者接口：计算、屏障准备和安装分别有明确结果，SQL提交未知时不得换新输入重试。<br>在此声明中的实际使用：<code>client: client</code> |
| <code>validator</code> | <code>*codec.Validator</code> | 冻结协议的结构校验器，先限制请求形状，再由业务核验身份、权限、版本和事务条件。<br>在此声明中的实际使用：<code>validator: validator</code> |
| <code>instance</code> | <code>string</code> | 在此声明中的实际使用：<code>instance: instance</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Publications</code> | 实际返回表达式：<code>&amp;Publications{db: db, identity: identity, access: access, client: client, validator: validator, instance: instance}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Publications{db: db, identity: identity, access: access, client: client, validator: validator, instance: instance}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=1299:1576 -->
````go
func NewPublications(db *sql.DB, identity *Identity, access *Access, client MergeClient, validator *codec.Validator, instance string) *Publications {
	return &Publications{db: db, identity: identity, access: access, client: client, validator: validator, instance: instance}
}

````

## 第 5 步：publicationRequest

插入位置：第 4 步代码之后，紧接追加；完成稿约第 51 行。

本步声明：`func publicationRequest(fixed publicationContext) *pb.CaptureMainRequest`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>fixed</code> | <code>publicationContext</code> | 在此声明中的实际使用：<code>fixed.Main</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.CaptureMainRequest</code> | 实际返回表达式：<code>&amp;pb.CaptureMainRequest{Context: main.Context, Usage: &quot;RELEASE_CREATE:1&quot;, ExpectedManifestId: main.Manifest, 		ExpectedStructureId: main.Structure, ExpectedStyleId: main.Style, ExpectedControlRevision: main.Control}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;pb.CaptureMainRequest{Context: main.Context, Usage: &quot;RELEASE_CREATE:1&quot;, ExpectedManifestId: main.Manifest, 		ExpectedStructureId: main.Structure, ExpectedStyleId: main.Styl …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1576:1897 -->
````go
func publicationRequest(fixed publicationContext) *pb.CaptureMainRequest {
	main := fixed.Main
	return &pb.CaptureMainRequest{Context: main.Context, Usage: "RELEASE_CREATE:1", ExpectedManifestId: main.Manifest,
		ExpectedStructureId: main.Structure, ExpectedStyleId: main.Style, ExpectedControlRevision: main.Control}
}

````

## 第 6 步：publicationInput

插入位置：第 5 步代码之后，紧接追加；完成稿约第 57 行。

本步声明：`func publicationInput(intent domain.PublicationIntent) any`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>intent</code> | <code>domain.PublicationIntent</code> | 使用 <code>domain.PublicationIntent</code> 类型；其定义处的职责：PublicationIntent固定发布动作与页面看到的水位。它不包含正文或数据库依赖， 发布正文必须来自可靠MAIN切点，历史版归属必须由最终业务事务核验。<br>在此声明中的实际使用：<code>intent.Kind</code>；<code>intent.Operation</code>；<code>intent.Publication</code>；<code>intent.Control</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>any</code> | 实际返回表达式：<code>dto.ReleaseCreate{OperationId: intent.Operation, ExpectedPublicationRevision: domain.TextID(intent.Publication), ExpectedControlRevision: domain.TextID(intent.Control)}</code>；<code>input</code> |

**跟着本段代码走一遍：**

- 检查 <code>intent.Kind == &quot;RELEASE_CREATE&quot;</code>。条件成立时执行 <code>return dto.ReleaseCreate{OperationId: intent.Operation, ExpectedPublicationRevision: domain.TextID(intent.Publication), ExpectedControlRevision: domain.TextID(intent.Control)}</code>。当前函数在这里结束。
- 检查 <code>intent.Kind == &quot;SWITCH&quot;</code>。
- 此处通过 <code>return input</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=1897:2441 -->
````go
func publicationInput(intent domain.PublicationIntent) any {
	if intent.Kind == "RELEASE_CREATE" {
		return dto.ReleaseCreate{OperationId: intent.Operation, ExpectedPublicationRevision: domain.TextID(intent.Publication), ExpectedControlRevision: domain.TextID(intent.Control)}
	}
	input := dto.Publication{OperationId: intent.Operation, ExpectedPublicationRevision: domain.TextID(intent.Publication), Action: intent.Kind}
	if intent.Kind == "SWITCH" {
		release := domain.TextID(intent.Release)
		input.ReleaseId = &release
	}
	return input
}

````

## 第 7 步：Publications.Lookup

插入位置：第 6 步代码之后，紧接追加；完成稿约第 69 行。

本步声明：`func (s *Publications) Lookup(ctx context.Context, operation string) (publicationContext, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.MergeArtifact(ctx, s.db, &quot;PUBLICATION_CONTEXT&quot;, operation)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>repository.MergeArtifact(ctx, s.db, &quot;PUBLICATION_CONTEXT&quot;, operation)</code>；<code>fixed.Intent.Operation != operation</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>publicationContext</code> | 实际返回表达式：<code>fixed</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;PUBLICATION_BINDING_UNKNOWN&quot;)</code>；<code>validatePublicationContext(fixed)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return fixed, err</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(raw, &amp;fixed) != nil &#124;&#124; fixed.Intent.Operation != operation</code>。条件成立时执行 <code>return fixed, domain.Violation(domain.TemporarilyUnavailable, &quot;PUBLICATION_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return fixed, validatePublicationContext(fixed)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=2441:2923 -->
````go
func (s *Publications) Lookup(ctx context.Context, operation string) (publicationContext, error) {
	var fixed publicationContext
	raw, err := repository.MergeArtifact(ctx, s.db, "PUBLICATION_CONTEXT", operation)
	if err != nil {
		return fixed, err
	}
	if json.Unmarshal(raw, &fixed) != nil || fixed.Intent.Operation != operation {
		return fixed, domain.Violation(domain.TemporarilyUnavailable, "PUBLICATION_BINDING_UNKNOWN")
	}
	return fixed, validatePublicationContext(fixed)
}

````

## 第 8 步：publicationLockSources

插入位置：第 7 步代码之后，紧接追加；完成稿约第 81 行。

本步声明：`func publicationLockSources(access DocumentAccess) []*pb.SourceEpoch`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>access</code> | <code>DocumentAccess</code> | 使用 <code>DocumentAccess</code> 类型；其定义处的职责：把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。<br>在此声明中的实际使用：<code>access.Sources</code>；<code>access.Document</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 实际返回表达式：<code>sources</code> |

**跟着本段代码走一遍：**

- 检查 <code>revision != 0</code>。
- 此处通过 <code>return sources</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=2923:3354 -->
````go
func publicationLockSources(access DocumentAccess) []*pb.SourceEpoch {
	sources := append([]*pb.SourceEpoch(nil), access.Sources...)
	revision, _ := domain.CanonicalRevision(access.Document.PublicationEpoch)
	if revision != 0 {
		sources = append(sources, &pb.SourceEpoch{ScopeKey: "publication:" + access.Document.Id, Epoch: revision})
	}
	return sources
}

// publication_binding.go：保留原实现，按业务合并文件。

````

## 第 9 步：validatePublicationContext

插入位置：第 8 步代码之后，紧接追加；完成稿约第 92 行。

本步声明：`func validatePublicationContext(fixed publicationContext) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>fixed</code> | <code>publicationContext</code> | 在此声明中的实际使用：<code>fixed.Main</code>；<code>fixed.Actor</code>；<code>fixed.Document</code>；<code>fixed.SessionExpires</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>invalid</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>epoch == 0 &#124;&#124; epoch &gt; main.Control</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>ctx == nil &#124;&#124; ctx.Target == nil &#124;&#124; fixed.Actor == 0 &#124;&#124; fixed.Document == 0 &#124;&#124; 		fixed.SessionExpires &lt;= 0 &#124;&#124; len(fixed.Access.Proofs) == 0 &#124;&#124; fixed.Access.Proofs&#91;0&#93; == nil &#124;&#124; 		!do …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>domain.ValidatePublication(fixed.Access.Permissions, fixed.Access.Document, fixed.Intent) != nil</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>!proto.Equal(ctx.Target, target) &#124;&#124; document.Id != domain.TextID(fixed.Document) &#124;&#124; 		main.Manifest == 0 &#124;&#124; main.Structure == 0 &#124;&#124; main.Style == 0 &#124;&#124; 		document.CurrentManifestId ! …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>first.AdmissionId != ctx.IdentityAdmissionRef &#124;&#124; len(first.SessionRef) != 32</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>proof == nil &#124;&#124; proof.IdentityTarget == nil &#124;&#124; proof.ActorId != fixed.Actor &#124;&#124; proof.OperationId != ctx.OperationId &#124;&#124; 			proof.Purpose != &quot;PUBLICATION&quot; &#124;&#124; !proto.Equal(proof.Targe …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>document.KnowledgeBaseId != nil</code>。
- 检查 <code>err != nil &#124;&#124; fixed.Access.KBRevision == 0</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。

<!-- write-step: 9 bytes=3354:5711 -->
````go
func validatePublicationContext(fixed publicationContext) error {
	invalid := domain.Violation(domain.TemporarilyUnavailable, "PUBLICATION_BINDING_UNKNOWN")
	main, ctx := fixed.Main, fixed.Main.Context
	if epoch := reliableMainEpoch(main); epoch == 0 || epoch > main.Control {
		return invalid
	}
	if ctx == nil || ctx.Target == nil || fixed.Actor == 0 || fixed.Document == 0 ||
		fixed.SessionExpires <= 0 || len(fixed.Access.Proofs) == 0 || fixed.Access.Proofs[0] == nil ||
		!domain.ValidOperationID(fixed.Intent.Operation) || !fixed.Access.Permissions.Manage ||
		ctx.OperationId != fixed.Intent.Operation || ctx.ActorId != fixed.Actor || ctx.DocumentAdmissionRef == "" {
		return invalid
	}
	if domain.ValidatePublication(fixed.Access.Permissions, fixed.Access.Document, fixed.Intent) != nil {
		return invalid
	}
	document := fixed.Access.Document
	target := &pb.Target{DocumentId: fixed.Document, StructureId: main.Structure, WorkspaceEpoch: main.Control, Purpose: "PUBLICATION"}
	if !proto.Equal(ctx.Target, target) || document.Id != domain.TextID(fixed.Document) ||
		main.Manifest == 0 || main.Structure == 0 || main.Style == 0 ||
		document.CurrentManifestId != domain.TextID(main.Manifest) || document.CurrentStructureId != domain.TextID(main.Structure) ||
		document.CurrentStyleId != domain.TextID(main.Style) || document.ControlRevision != domain.TextID(main.Control) {
		return invalid
	}
	first := fixed.Access.Proofs[0]
	if first.AdmissionId != ctx.IdentityAdmissionRef || len(first.SessionRef) != 32 {
		return invalid
	}
	for _, proof := range fixed.Access.Proofs {
		if proof == nil || proof.IdentityTarget == nil || proof.ActorId != fixed.Actor || proof.OperationId != ctx.OperationId ||
			proof.Purpose != "PUBLICATION" || !proto.Equal(proof.Target, target) || !bytes.Equal(proof.SessionRef, first.SessionRef) ||
			proof.ExpiresUnixMs <= 0 || proof.ExpiresUnixMs > fixed.SessionExpires {
			return invalid
		}
	}
	sources, err := MergeSources(fixed.Access.Proofs...)
	if err != nil {
		return invalid
	}
	acl, err := domain.CanonicalID(document.AclRevision)
	if err != nil {
		return invalid
	}
	sources = append(sources, &pb.SourceEpoch{ScopeKey: "doc-acl:" + document.Id, Epoch: acl})
	if document.KnowledgeBaseId != nil {
		if _, err := domain.CanonicalID(*document.KnowledgeBaseId); err != nil || fixed.Access.KBRevision == 0 {
````

## 第 10 步：validatePublicationContext（接着实现）

插入位置：第 9 步代码之后，紧接追加；完成稿约第 137 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return invalid</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!equalPullSources(sources, fixed.Access.Sources) &#124;&#124; !equalPullSources(ctx.Sources, sources)</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>fixed.Intent.Kind == &quot;RELEASE_CREATE&quot;</code>。
- 检查 <code>err != nil &#124;&#124; fixed.Path != path &#124;&#124; !bytes.Equal(fingerprint&#91;:&#93;, ctx.RequestFingerprint)</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>len(main.Units) != len(main.Versions)</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>unit == 0 &#124;&#124; seen&#91;unit&#93; &#124;&#124; main.Versions&#91;unit&#93; == 0</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 检查 <code>fixed.Intent.Kind == &quot;RELEASE_CREATE&quot; &amp;&amp; len(main.Units) == 0</code>。条件成立时执行 <code>return invalid</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=5711:6694 -->
````go
			return invalid
		}
		sources = append(sources, &pb.SourceEpoch{ScopeKey: "kb-acl:" + *document.KnowledgeBaseId, Epoch: fixed.Access.KBRevision})
	}
	if !equalPullSources(sources, fixed.Access.Sources) || !equalPullSources(ctx.Sources, sources) {
		return invalid
	}
	path := "/v1/documents/" + document.Id + "/publication"
	if fixed.Intent.Kind == "RELEASE_CREATE" {
		path = "/v1/documents/" + document.Id + "/releases"
	}
	fingerprint, err := domain.OperationFingerprint(path, publicationInput(fixed.Intent))
	if err != nil || fixed.Path != path || !bytes.Equal(fingerprint[:], ctx.RequestFingerprint) {
		return invalid
	}
	if len(main.Units) != len(main.Versions) {
		return invalid
	}
	seen := make(map[uint64]bool, len(main.Units))
	for _, unit := range main.Units {
		if unit == 0 || seen[unit] || main.Versions[unit] == 0 {
			return invalid
		}
		seen[unit] = true
	}
	if fixed.Intent.Kind == "RELEASE_CREATE" && len(main.Units) == 0 {
		return invalid
	}
	return nil
}

````

## 第 11 步：Publications.resume

插入位置：第 10 步代码之后，紧接追加；完成稿约第 168 行。

本步声明：`func (s *Publications) resume(ctx context.Context, fixed publicationContext) ([]*pb.IdentityAdmission, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.identity.Resume(ctx, original)</code> |
| <code>fixed</code> | <code>publicationContext</code> | 在此声明中的实际使用：<code>validatePublicationContext(fixed)</code>；<code>fixed.SessionExpires</code>；<code>fixed.Access</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.IdentityAdmission</code> | 实际返回表达式：<code>nil</code>；<code>proofs</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.PermissionDenied, &quot;SESSION_EXPIRED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>fixed.SessionExpires &lt;= time.Now().UnixMilli()</code>。条件成立时执行 <code>return nil, domain.Violation(domain.PermissionDenied, &quot;SESSION_EXPIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return proofs, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=6694:7284 -->
````go
func (s *Publications) resume(ctx context.Context, fixed publicationContext) ([]*pb.IdentityAdmission, error) {
	if err := validatePublicationContext(fixed); err != nil {
		return nil, err
	}
	if fixed.SessionExpires <= time.Now().UnixMilli() {
		return nil, domain.Violation(domain.PermissionDenied, "SESSION_EXPIRED")
	}
	proofs := make([]*pb.IdentityAdmission, 0, len(fixed.Access.Proofs))
	for _, original := range fixed.Access.Proofs {
		proof, err := s.identity.Resume(ctx, original)
		if err != nil {
			return nil, err
		}
		proofs = append(proofs, proof)
	}
	return proofs, nil
}

````

## 第 12 步：validatePublicationProofs

插入位置：第 11 步代码之后，紧接追加；完成稿约第 186 行。

本步声明：`func validatePublicationProofs(fixed publicationContext, proofs []*pb.IdentityAdmission) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>fixed</code> | <code>publicationContext</code> | 在此声明中的实际使用：<code>fixed.SessionExpires</code>；<code>fixed.Access</code> |
| <code>proofs</code> | <code>&#91;&#93;*pb.IdentityAdmission</code> | 在此声明中的实际使用：<code>len(proofs)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.PermissionDenied, &quot;SESSION_EXPIRED&quot;)</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;IDENTITY_PROOF_REFRESH_REQUIRED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>fixed.SessionExpires &lt;= time.Now().UnixMilli()</code>。条件成立时执行 <code>return domain.Violation(domain.PermissionDenied, &quot;SESSION_EXPIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>len(proofs) != len(fixed.Access.Proofs)</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;IDENTITY_PROOF_REFRESH_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>proof == nil &#124;&#124; proof.ExpiresUnixMs &lt;= time.Now().UnixMilli() &#124;&#124; proof.ActorId != old.ActorId &#124;&#124; 			proof.Purpose != old.Purpose &#124;&#124; proof.OperationId != old.OperationId &#124;&#124; !proto.E …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;IDENTITY_PROOF_REFRESH_REQUIRED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=7284:8208 -->
````go
func validatePublicationProofs(fixed publicationContext, proofs []*pb.IdentityAdmission) error {
	if fixed.SessionExpires <= time.Now().UnixMilli() {
		return domain.Violation(domain.PermissionDenied, "SESSION_EXPIRED")
	}
	if len(proofs) != len(fixed.Access.Proofs) {
		return domain.Violation(domain.TemporarilyUnavailable, "IDENTITY_PROOF_REFRESH_REQUIRED")
	}
	for i, proof := range proofs {
		old := fixed.Access.Proofs[i]
		if proof == nil || proof.ExpiresUnixMs <= time.Now().UnixMilli() || proof.ActorId != old.ActorId ||
			proof.Purpose != old.Purpose || proof.OperationId != old.OperationId || !proto.Equal(proof.Target, old.Target) ||
			!proto.Equal(proof.IdentityTarget, old.IdentityTarget) || !bytes.Equal(proof.SessionRef, old.SessionRef) ||
			!equalPullSources(proof.Sources, old.Sources) {
			return domain.Violation(domain.TemporarilyUnavailable, "IDENTITY_PROOF_REFRESH_REQUIRED")
		}
	}
	return nil
}

````

## 第 13 步：validatePublicationAdmission

插入位置：第 12 步代码之后，紧接追加；完成稿约第 205 行。

本步声明：`func validatePublicationAdmission(ctx context.Context, tx *sql.Tx, fixed publicationContext) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.OriginalAdmission(ctx, tx, fixed.Main.Context.DocumentAdmissionRef, true)</code>；<code>repository.LockedAdmission(ctx, tx, fixed.Main.Context.DocumentAdmissionRef)</code>；<code>repository.AdmissionSources(ctx, tx, fixed.Main.Context.DocumentAdmissionRef)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.OriginalAdmission(ctx, tx, fixed.Main.Context.DocumentAdmissionRef, true)</code>；<code>repository.LockedAdmission(ctx, tx, fixed.Main.Context.DocumentAdmissionRef)</code>；<code>repository.AdmissionSources(ctx, tx, fixed.Main.Context.DocumentAdmissionRef)</code> |
| <code>fixed</code> | <code>publicationContext</code> | 在此声明中的实际使用：<code>fixed.Main</code>；<code>fixed.Actor</code>；<code>fixed.Document</code>；<code>fixed.Intent</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;PUBLICATION_BINDING_UNKNOWN&quot;)</code>；<code>domain.Violation(domain.PermissionDenied, &quot;PUBLICATION_ADMISSION_CLOSED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>actual.Actor != fixed.Actor &#124;&#124; actual.Document != fixed.Document &#124;&#124; actual.Unit != 0 &#124;&#124; 		actual.Identity != fixed.Main.Context.IdentityAdmissionRef &#124;&#124; actual.Operation != fixed.In …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;PUBLICATION_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>state.State != &quot;OPEN&quot; &#124;&#124; !state.Expires.After(time.Now())</code>。条件成立时执行 <code>return domain.Violation(domain.PermissionDenied, &quot;PUBLICATION_ADMISSION_CLOSED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!equalPullSources(sources, fixed.Access.Sources)</code>。条件成立时执行 <code>return domain.Violation(domain.TemporarilyUnavailable, &quot;PUBLICATION_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=8208:9658 -->
````go
func validatePublicationAdmission(ctx context.Context, tx *sql.Tx, fixed publicationContext) error {
	actual, err := repository.OriginalAdmission(ctx, tx, fixed.Main.Context.DocumentAdmissionRef, true)
	if err != nil {
		return err
	}
	target := &pb.Target{}
	if actual.Actor != fixed.Actor || actual.Document != fixed.Document || actual.Unit != 0 ||
		actual.Identity != fixed.Main.Context.IdentityAdmissionRef || actual.Operation != fixed.Intent.Operation ||
		actual.Purpose != "PUBLICATION" || actual.SessionExpires != fixed.SessionExpires ||
		!bytes.Equal(actual.Session, fixed.Access.Proofs[0].SessionRef) || json.Unmarshal(actual.Target, target) != nil ||
		!proto.Equal(target, fixed.Main.Context.Target) {
		return domain.Violation(domain.TemporarilyUnavailable, "PUBLICATION_BINDING_UNKNOWN")
	}
	state, err := repository.LockedAdmission(ctx, tx, fixed.Main.Context.DocumentAdmissionRef)
	if err != nil {
		return err
	}
	if state.State != "OPEN" || !state.Expires.After(time.Now()) {
		return domain.Violation(domain.PermissionDenied, "PUBLICATION_ADMISSION_CLOSED")
	}
	sources, err := repository.AdmissionSources(ctx, tx, fixed.Main.Context.DocumentAdmissionRef)
	if err != nil {
		return err
	}
	if !equalPullSources(sources, fixed.Access.Sources) {
		return domain.Violation(domain.TemporarilyUnavailable, "PUBLICATION_BINDING_UNKNOWN")
	}
	return nil
}

// publication_capture_errors.go：保留原实现，按业务合并文件。

````

## 第 14 步：publicationCaptureError

插入位置：第 13 步代码之后，紧接追加；完成稿约第 237 行。

本步声明：`func publicationCaptureError(err error) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>err</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>在此声明中的实际使用：<code>err == nil</code>；<code>status.FromError(err)</code>；<code>return err</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>nil</code>；<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;PUBLICATION_CONTENT_TOO_LARGE&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;PUBLICATION_TRANSFER_LIMIT&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err == nil</code>。条件成立时执行 <code>return nil</code>。当前函数在这里结束。
- 检查 <code>!encoded &#124;&#124; rpc.Code() != codes.ResourceExhausted</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return domain.Violation(domain.StateConflict, &quot;PUBLICATION_CONTENT_TOO_LARGE&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return domain.Violation(domain.StateConflict, &quot;PUBLICATION_TRANSFER_LIMIT&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=9658:10399 -->
````go
func publicationCaptureError(err error) error {
	if err == nil {
		return nil
	}
	rpc, encoded := status.FromError(err)
	if !encoded || rpc.Code() != codes.ResourceExhausted {
		return err
	}
	// 仅信任Node已执行的固定容量规则；通用传输大小、配额或依赖耗尽没有
	// 足够事实确认业务失败，仍沿原号恢复。未知远端文本不进入公开错误。
	switch rpc.Message() {
	case "MAIN_CONTENT_TOO_LARGE":
		return domain.Violation(domain.StateConflict, "PUBLICATION_CONTENT_TOO_LARGE")
	case "MAIN_VIEW_TRANSFER_LIMIT":
		return domain.Violation(domain.StateConflict, "PUBLICATION_TRANSFER_LIMIT")
	default:
		return err
	}
}

// publication_commit.go：保留原实现，按业务合并文件。

````

## 第 15 步：Publications.commit

插入位置：第 14 步代码之后，紧接追加；完成稿约第 259 行。

commit把已捕获的固定主线发布为正式版本；它不读取正在变化的编辑草稿。
RPC捕获已完成后才开启最终事务，事务内重检权威资格、切点和任务归属。

本步声明：`func (s *Publications) commit(ctx context.Context, task domain.SubmissionTask, fixed publicationContext, main *pb.MainCaptureReply, proofs []*pb.IdentityAdmission) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, nil)</code>；<code>repository.LockOperationGate(ctx, tx, task.Operation)</code>；<code>s.validateCurrent(ctx, tx, fixed, proofs)</code>；<code>repository.ValidateMergeTask(ctx, tx, task)</code><br>另有 9 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>task.Operation</code>；<code>repository.ValidateMergeTask(ctx, tx, task)</code>；<code>repository.FinishMergeTask(ctx, tx, task, &quot;SUCCEEDED&quot;)</code> |
| <code>fixed</code> | <code>publicationContext</code> | 在此声明中的实际使用：<code>fixed.Main</code>；<code>s.validateCurrent(ctx, tx, fixed, proofs)</code>；<code>fixed.Document</code>；<code>fixed.Actor</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>main</code> | <code>*pb.MainCaptureReply</code> | 在此声明中的实际使用：<code>validateReliableMainCapture(s.validator, fixed.Main, main, &quot;RELEASE_CREATE:1&quot;, &quot;PUBLICATION&quot;)</code>；<code>main.Units</code> |
| <code>proofs</code> | <code>&#91;&#93;*pb.IdentityAdmission</code> | 在此声明中的实际使用：<code>s.validateCurrent(ctx, tx, fixed, proofs)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;RESOURCE_NOT_READY&quot;)</code>；<code>tx.Commit()</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return ordered&#91;i&#93; &lt; ordered&#91;j&#93;</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>state != &quot;READY&quot;</code>。条件成立时执行 <code>return domain.Violation(domain.StateConflict, &quot;RESOURCE_NOT_READY&quot;)</code>。当前函数在这里结束。

<!-- write-step: 15 bytes=10399:12311 -->
````go
// commit把已捕获的固定主线发布为正式版本；它不读取正在变化的编辑草稿。
// RPC捕获已完成后才开启最终事务，事务内重检权威资格、切点和任务归属。
func (s *Publications) commit(ctx context.Context, task domain.SubmissionTask, fixed publicationContext, main *pb.MainCaptureReply, proofs []*pb.IdentityAdmission) error {
	if err := validateReliableMainCapture(s.validator, fixed.Main, main, "RELEASE_CREATE:1", "PUBLICATION"); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// 固定结果的RPC已完成；最终锁序为operation→完整来源→Doc/KB→MAIN屏障→原任务。
	// 只有当前原管理资格和同一切点全部成立，才能一次落定版本、指针与事件。
	if err = repository.LockOperationGate(ctx, tx, task.Operation); err != nil {
		return err
	}
	current, err := s.validateCurrent(ctx, tx, fixed, proofs)
	if err != nil {
		return err
	}
	if err = repository.ValidateMergeTask(ctx, tx, task); err != nil {
		return err
	}
	space, _ := domain.CanonicalID(current.SpaceId)
	resources := make(map[uint64]bool)
	for _, content := range main.Units {
		if err = validateReliableMainVersion(ctx, tx, fixed.Main, content, "PUBLICATION"); err != nil {
			return err
		}
		for _, resource := range content.ResourceIds {
			resources[resource] = true
		}
	}
	// 合并各单元引用并按数值升序锁资源，避免两篇文档以相反顺序持锁形成死锁。
	ordered := make([]uint64, 0, len(resources))
	for resource := range resources {
		ordered = append(ordered, resource)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	for _, resource := range ordered {
		state, err := repository.LockedDocumentResource(ctx, tx, resource, fixed.Document, space)
		if err != nil {
			return err
		}
		if state != "READY" {
````

## 第 16 步：Publications.commit（接着实现）

插入位置：第 15 步代码之后，紧接追加；完成稿约第 304 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return domain.Violation(domain.StateConflict, &quot;RESOURCE_NOT_READY&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 16 bytes=12311:13725 -->
````go
			return domain.Violation(domain.StateConflict, "RESOURCE_NOT_READY")
		}
	}
	manifest, err := repository.InsertControlledManifest(ctx, tx, fixed.Document, fixed.Main.Manifest, fixed.Main.Structure, fixed.Main.Style, task.Operation, "RELEASE", task.Operation+":RELEASE_CREATE:1")
	if err != nil {
		return err
	}
	for _, content := range main.Units {
		version, err := repository.InsertMergeVersion(ctx, tx, content, "MAIN_CAPTURE", content.CaptureId)
		if err != nil {
			return err
		}
		if err = repository.InsertInitialManifestEntry(ctx, tx, fixed.Document, manifest, content.UnitId, version); err != nil {
			return err
		}
	}
	release, err := repository.InsertRelease(ctx, tx, fixed.Document, manifest, fixed.Actor, current.Title, task.Operation)
	if err != nil {
		return err
	}
	result, err := s.applyPublication(ctx, tx, fixed, release)
	if err != nil {
		return err
	}
	// 正式版/发布指针/outbox事件/原号结果与任务终态一起提交，不能拆成独立DAO提交。
	// tx.Commit若返回未知，恢复器仍读取这个operation，不新建一份正式版冒充重试。
	raw, err := json.Marshal(result)
	if err != nil {
		return err
	}
	if err = repository.UpdateOperationResult(ctx, tx, task.Operation, result["state"].(string), raw); err != nil {
		return err
	}
	if err = repository.FinishMergeTask(ctx, tx, task, "SUCCEEDED"); err != nil {
		return err
	}
	return tx.Commit()
}

````

## 第 17 步：Publications.applyPublication

插入位置：第 16 步代码之后，紧接追加；完成稿约第 343 行。

本步声明：`func (s *Publications) applyPublication(ctx context.Context, tx *sql.Tx, fixed publicationContext, release uint64) (map[string]any, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.PublicationEventSequence(ctx, tx, fixed.Document)</code>；<code>repository.SetPublication(ctx, tx, fixed.Document, release, epoch, sequence, fixed.Intent.Operation, eventType, payload)</code>；<code>QueueClosure(ctx, tx, fixed.Intent.Operation, fixed.Actor, closure)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.PublicationEventSequence(ctx, tx, fixed.Document)</code>；<code>repository.SetPublication(ctx, tx, fixed.Document, release, epoch, sequence, fixed.Intent.Operation, eventType, payload)</code>；<code>QueueClosure(ctx, tx, fixed.Intent.Operation, fixed.Actor, closure)</code> |
| <code>fixed</code> | <code>publicationContext</code> | 在此声明中的实际使用：<code>fixed.Document</code>；<code>fixed.Intent</code>；<code>fixed.Actor</code> |
| <code>release</code> | <code>uint64</code> | 在此声明中的实际使用：<code>release != 0</code>；<code>Release: release</code>；<code>repository.SetPublication(ctx, tx, fixed.Document, release, epoch, sequence, fixed.Intent.Operation, eventType, payload)</code>；<code>domain.TextID(release)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>map&#91;string&#93;any</code> | 实际返回表达式：<code>nil</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>release != 0</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>fixed.Intent.Publication != 0</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>release != 0</code>。
- 检查 <code>state == &quot;PREPARING&quot;</code>。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 17 bytes=13725:15569 -->
````go
func (s *Publications) applyPublication(ctx context.Context, tx *sql.Tx, fixed publicationContext, release uint64) (map[string]any, error) {
	sequence, err := repository.PublicationEventSequence(ctx, tx, fixed.Document)
	if err != nil {
		return nil, err
	}
	epoch, sequence, err := domain.NextPublicationSequences(fixed.Intent.Publication, sequence)
	if err != nil {
		return nil, err
	}
	eventType := "DOCUMENT_UNPUBLISHED"
	if release != 0 {
		eventType = "DOCUMENT_PUBLISHED"
	}
	// 事件仅承载固定业务水位和正式引用；正文、身份票据及内部草稿不进入消息总线。
	payload, err := domain.EncodeIndexEvent(domain.IndexEvent{
		ID: fixed.Intent.Operation, Type: eventType, Document: fixed.Document,
		Sequence: sequence, Release: release, PublicationEpoch: epoch, OccurredAt: time.Now().UTC(),
	})
	if err != nil {
		return nil, err
	}
	if err = repository.SetPublication(ctx, tx, fixed.Document, release, epoch, sequence, fixed.Intent.Operation, eventType, payload); err != nil {
		return nil, err
	}
	state := "SUCCEEDED"
	if fixed.Intent.Publication != 0 {
		// 已提交业务不等于旧公开输出已结束；关闭由既有系统worker沿原号完成，用户无需在线。
		closure := domain.Closure{Scope: "publication:" + domain.TextID(fixed.Document), Epoch: fixed.Intent.Publication}
		if err = QueueClosure(ctx, tx, fixed.Intent.Operation, fixed.Actor, closure); err != nil {
			return nil, err
		}
		state = "PREPARING"
	}
	result := map[string]any{"operation_id": fixed.Intent.Operation, "state": state}
	if release != 0 {
		result["result"] = map[string]string{"entity_id": domain.TextID(release)}
	}
	if state == "PREPARING" {
		result["poll_url"] = "/v1/operations/" + fixed.Intent.Operation
	}
	return result, nil
}

// publication_prepare.go：保留原实现，按业务合并文件。

````

## 第 18 步：Publications.Prepare

插入位置：第 17 步代码之后，紧接追加；完成稿约第 388 行。

本步声明：`func (s *Publications) Prepare(ctx context.Context, actor Actor, document uint64, intent domain.PublicationIntent, input any, path string) (Actor, MutationBody, MutationPresentation, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, document, &quot;PUBLICATION&quot;)</code>；<code>repository.PublicationResult(ctx, s.db, intent.Operation)</code>；<code>s.identity.BindProofs(ctx, actor, proofs, intent.Operation, target)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, document, &quot;PUBLICATION&quot;)</code>；<code>return actor, nil, nil, err</code>；<code>return actor, nil, nil, domain.Violation(domain.PermissionDenied, &quot;PUBLICATION_NOT_ALLOWED&quot;)</code>；<code>actor.Sources</code><br>另有 8 处使用，继续按下方完整代码的顺序处理。 |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, document, &quot;PUBLICATION&quot;)</code>；<code>DocumentId: document</code>；<code>repository.LockedMergeBarrier(ctx, tx, document)</code>；<code>InsertAdmission(ctx, tx, actor, access.Sources, document, &quot;PUBLICATION&quot;, admission)</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |
| <code>intent</code> | <code>domain.PublicationIntent</code> | 使用 <code>domain.PublicationIntent</code> 类型；其定义处的职责：PublicationIntent固定发布动作与页面看到的水位。它不包含正文或数据库依赖， 发布正文必须来自可靠MAIN切点，历史版归属必须由最终业务事务核验。<br>在此声明中的实际使用：<code>intent.Operation</code>；<code>domain.ValidatePublication(access.Permissions, current, intent)</code>；<code>intent.Kind</code>；<code>Intent: intent</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>input</code> | <code>any</code> | 在此声明中的实际使用：<code>domain.OperationFingerprint(path, input)</code> |
| <code>path</code> | <code>string</code> | 在此声明中的实际使用：<code>domain.OperationFingerprint(path, input)</code>；<code>Path: path</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>实际返回表达式：<code>actor</code> |
| <code>(匿名/未命名)</code> | <code>MutationBody</code> | 使用 <code>MutationBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>func(context.Context, *sql.Tx) (map&#91;string&#93;any, error) { 			return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;ORIGINAL_OPERATION_UNAVAILABLE&quot;) 		}</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>MutationPresentation</code> | 使用 <code>MutationPresentation</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>present</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.PermissionDenied, &quot;PUBLICATION_NOT_ALLOWED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>!access.Permissions.Manage</code>。条件成立时执行 <code>return actor, nil, nil, domain.Violation(domain.PermissionDenied, &quot;PUBLICATION_NOT_ALLOWED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err == nil</code>。
- 此处通过 <code>return actor, func(context.Context, *sql.Tx) (map&#91;string&#93;any, error) { 			return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;ORIGINAL_OPERATION_UNAVAILABLE&quot;) 		}, present, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：此处通过 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;ORIGINAL_OPERATION_UNAVAILABLE&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != sql.ErrNoRows</code>。条件成立时执行 <code>return actor, nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>intent.Kind == &quot;RELEASE_CREATE&quot; &amp;&amp; barrier.Phase != &quot;OPEN&quot;</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;MAIN_COMMIT_PENDING&quot;)</code>。当前函数在这里结束。

<!-- write-step: 18 bytes=15569:17638 -->
````go
func (s *Publications) Prepare(ctx context.Context, actor Actor, document uint64, intent domain.PublicationIntent, input any, path string) (Actor, MutationBody, MutationPresentation, error) {
	access, err := s.access.Authorize(ctx, actor, document, "PUBLICATION")
	if err != nil {
		return actor, nil, nil, err
	}
	if !access.Permissions.Manage {
		return actor, nil, nil, domain.Violation(domain.PermissionDenied, "PUBLICATION_NOT_ALLOWED")
	}
	actor.Sources = publicationLockSources(access)
	present := s.presentation(actor, access, intent.Operation)
	if _, err = repository.PublicationResult(ctx, s.db, intent.Operation); err == nil {
		// 先由通用receipt核验原actor/kind/输入指纹，再投影原结果，不重新要求旧期望匹配新水位。
		actor.Sources = access.Sources
		return actor, func(context.Context, *sql.Tx) (map[string]any, error) {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "ORIGINAL_OPERATION_UNAVAILABLE")
		}, present, nil
	} else if err != sql.ErrNoRows {
		return actor, nil, nil, err
	}
	structure, _ := domain.CanonicalID(access.Document.CurrentStructureId)
	control, _ := domain.CanonicalRevision(access.Document.ControlRevision)
	target := &pb.Target{DocumentId: document, StructureId: structure, WorkspaceEpoch: control, Purpose: "PUBLICATION"}
	proofs := access.Proofs
	access.Proofs, err = s.identity.BindProofs(ctx, actor, proofs, intent.Operation, target)
	if err != nil {
		return actor, nil, nil, err
	}
	actor.Proof = access.Proofs[0]
	fingerprint, err := domain.OperationFingerprint(path, input)
	if err != nil {
		return actor, nil, nil, err
	}
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		current, err := ValidateAccess(ctx, tx, access)
		if err != nil {
			return nil, err
		}
		if err = domain.ValidatePublication(access.Permissions, current, intent); err != nil {
			return nil, err
		}
		barrier, err := repository.LockedMergeBarrier(ctx, tx, document)
		if err != nil {
			return nil, err
		}
		if intent.Kind == "RELEASE_CREATE" && barrier.Phase != "OPEN" {
````

## 第 19 步：Publications.Prepare（接着实现）

插入位置：第 18 步代码之后，紧接追加；完成稿约第 433 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：此处通过 <code>return nil, domain.Violation(domain.StateConflict, &quot;MAIN_COMMIT_PENDING&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>intent.Kind == &quot;RELEASE_CREATE&quot;</code>。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。

<!-- write-step: 19 bytes=17638:19699 -->
````go
			return nil, domain.Violation(domain.StateConflict, "MAIN_COMMIT_PENDING")
		}
		admission := platform.NewTraceID()
		if err = InsertAdmission(ctx, tx, actor, access.Sources, document, "PUBLICATION", admission); err != nil {
			return nil, err
		}
		rawTarget, err := json.Marshal(target)
		if err != nil {
			return nil, err
		}
		if err = repository.ConfigureMainOperationAdmission(ctx, tx, admission, intent.Operation, rawTarget, actor.SessionExpiresUnixMs); err != nil {
			return nil, err
		}
		access.Document = current
		manifest, _ := domain.CanonicalID(current.CurrentManifestId)
		style, _ := domain.CanonicalID(current.CurrentStyleId)
		fixed := publicationContext{Actor: actor.ID, Document: document, SessionExpires: actor.SessionExpiresUnixMs, Intent: intent, Access: access, Path: path,
			Main: reliableMainBinding{Manifest: manifest, Structure: structure, Style: style, Control: control,
				Context: &pb.OperationContext{OperationId: intent.Operation, ActorId: actor.ID, IdentityAdmissionRef: actor.Proof.AdmissionId,
					DocumentAdmissionRef: admission, Target: target, Sources: access.Sources, RequestFingerprint: fingerprint[:]}}}
		if intent.Kind == "RELEASE_CREATE" {
			fixed.Main.Epoch, err = repository.MainWorkspaceEpoch(ctx, tx, document)
			if err != nil {
				return nil, err
			}
			entries, err := repository.ManifestUnits(ctx, tx, current.CurrentManifestId, document)
			if err != nil {
				return nil, err
			}
			structureRow, err := repository.StructureRow(ctx, tx, current.CurrentStructureId, current.Id)
			if err != nil {
				return nil, err
			}
			if err = validateReadingUnits(structureRow, entries); err != nil {
				return nil, err
			}
			fixed.Main.Versions = make(map[uint64]uint64, len(entries))
			for _, entry := range entries {
				unit, _ := domain.CanonicalID(entry["unit_id"])
				version, _ := domain.CanonicalID(entry["content_version_id"])
				fixed.Main.Units = append(fixed.Main.Units, unit)
				fixed.Main.Versions[unit] = version
			}
		}
		if err = validatePublicationContext(fixed); err != nil {
````

## 第 20 步：Publications.Prepare（接着实现）

插入位置：第 19 步代码之后，紧接追加；完成稿约第 478 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：此处通过 <code>return nil, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>intent.Kind == &quot;RELEASE_CREATE&quot;</code>。条件成立时执行 <code>return map&#91;string&#93;any{&quot;operation_id&quot;: intent.Operation, &quot;state&quot;: &quot;PREPARING&quot;, &quot;poll_url&quot;: &quot;/v1/operations/&quot; + intent.Operation}, nil</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>intent.Kind == &quot;SWITCH&quot;</code>。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return s.applyPublication(ctx, tx, fixed, intent.Release)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return actor, run, present, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 20 bytes=19699:20454 -->
````go
			return nil, err
		}
		original, err := json.Marshal(fixed)
		if err != nil {
			return nil, err
		}
		if err = repository.RegisterPublication(ctx, tx, intent.Operation, original, intent.Kind == "RELEASE_CREATE"); err != nil {
			return nil, err
		}
		if intent.Kind == "RELEASE_CREATE" {
			return map[string]any{"operation_id": intent.Operation, "state": "PREPARING", "poll_url": "/v1/operations/" + intent.Operation}, nil
		}
		if intent.Kind == "SWITCH" {
			if _, err = repository.ReleaseManifest(ctx, tx, document, intent.Release); err != nil {
				return nil, err
			}
		}
		return s.applyPublication(ctx, tx, fixed, intent.Release)
	}
	return actor, run, present, nil
}

// publication_readiness.go：保留原实现，按业务合并文件。

````

## 第 21 步：Publications.Ready

插入位置：第 20 步代码之后，紧接追加；完成稿约第 502 行。

本步声明：`func (s *Publications) Ready(ctx context.Context) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.PendingPublicationBindings(ctx, s.db)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;PUBLICATION_RECOVERY_UNAVAILABLE&quot;)</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;PUBLICATION_RECOVERY_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 21 bytes=20454:20784 -->
````go
func (s *Publications) Ready(ctx context.Context) error {
	facts, err := repository.PendingPublicationBindings(ctx, s.db)
	if err != nil {
		return errors.New("PUBLICATION_RECOVERY_UNAVAILABLE")
	}
	for _, fact := range facts {
		if err := validatePendingPublicationBinding(fact); err != nil {
			return err
		}
	}
	return nil
}

````

## 第 22 步：validatePendingPublicationBinding

插入位置：第 21 步代码之后，紧接追加；完成稿约第 515 行。

本步声明：`func validatePendingPublicationBinding(fact repository.PendingPublicationBinding) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>fact</code> | <code>repository.PendingPublicationBinding</code> | 使用 <code>repository.PendingPublicationBinding</code> 类型；其定义处的职责：保存发布上下文、正式版本、发布指针与搜索outbox水位。<br>在此声明中的实际使用：<code>fact.Original</code>；<code>fact.Payload</code>；<code>fact.Target</code>；<code>fact.Kind</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;PUBLICATION_BINDING_UNKNOWN&quot;)</code>；<code>errors.New(&quot;PUBLICATION_RECOVERY_UNKNOWN&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>json.Unmarshal(fact.Original, &amp;original) != nil &#124;&#124; json.Unmarshal(fact.Payload, &amp;payload) != nil &#124;&#124; 		json.Unmarshal(fact.Target, &amp;target) != nil &#124;&#124; validatePublicationContext(orig …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return errors.New(&quot;PUBLICATION_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return errors.New(&quot;PUBLICATION_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; !bytes.Equal(left, right)</code>。条件成立时执行 <code>return errors.New(&quot;PUBLICATION_RECOVERY_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 22 bytes=20784:21847 -->
````go
func validatePendingPublicationBinding(fact repository.PendingPublicationBinding) error {
	var original, payload publicationContext
	var target struct {
		Path string `json:"path"`
	}
	if json.Unmarshal(fact.Original, &original) != nil || json.Unmarshal(fact.Payload, &payload) != nil ||
		json.Unmarshal(fact.Target, &target) != nil || validatePublicationContext(original) != nil ||
		fact.Kind != "RELEASE_CREATE" || fact.State != "PREPARING" || fact.Actor != original.Actor ||
		fact.Operation != original.Intent.Operation || original.Intent.Kind != fact.Kind || target.Path != original.Path ||
		!bytes.Equal(fact.Fingerprint, original.Main.Context.RequestFingerprint) {
		return errors.New("PUBLICATION_BINDING_UNKNOWN")
	}
	left, err := json.Marshal(original)
	if err != nil {
		return errors.New("PUBLICATION_BINDING_UNKNOWN")
	}
	right, err := json.Marshal(payload)
	if err != nil || !bytes.Equal(left, right) {
		return errors.New("PUBLICATION_RECOVERY_UNKNOWN")
	}
	return nil
}

// publication_readings.go：保留原实现，按业务合并文件。

````

## 第 23 步：Publications.presentation

插入位置：第 22 步代码之后，紧接追加；完成稿约第 540 行。

本步声明：`func (s *Publications) presentation(actor Actor, access DocumentAccess, operation string) MutationPresentation`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.readResult(ctx, tx, actor, access, operation)</code> |
| <code>access</code> | <code>DocumentAccess</code> | 使用 <code>DocumentAccess</code> 类型；其定义处的职责：把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。<br>在此声明中的实际使用：<code>s.readResult(ctx, tx, actor, access, operation)</code>；<code>access.Sources</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>s.readResult(ctx, tx, actor, access, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>MutationPresentation</code> | 使用 <code>MutationPresentation</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>func(ctx context.Context, tx *sql.Tx, _ map&#91;string&#93;any) (any, &#91;&#93;*pb.SourceEpoch, error) { 		value, err := s.readResult(ctx, tx, actor, access, operation) 		return value, access.Sources, err 	}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return func(ctx context.Context, tx *sql.Tx, _ map&#91;string&#93;any) (any, &#91;&#93;*pb.SourceEpoch, error) { 		value, err := s.readResult(ctx, tx, actor, access, operation) 		return value, access.Sources, err 	}</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：此处通过 <code>return value, access.Sources, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 23 bytes=21847:22164 -->
````go
func (s *Publications) presentation(actor Actor, access DocumentAccess, operation string) MutationPresentation {
	return func(ctx context.Context, tx *sql.Tx, _ map[string]any) (any, []*pb.SourceEpoch, error) {
		value, err := s.readResult(ctx, tx, actor, access, operation)
		return value, access.Sources, err
	}
}

````

## 第 24 步：Publications.Progress

插入位置：第 23 步代码之后，紧接追加；完成稿约第 547 行。

本步声明：`func (s *Publications) Progress(ctx context.Context, actor Actor, operation string) ([]*pb.SourceEpoch, ReadBody, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.Lookup(ctx, operation)</code>；<code>s.access.Authorize(ctx, actor, fixed.Document, &quot;PUBLICATION_RESULT&quot;)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor.ID</code>；<code>s.access.Authorize(ctx, actor, fixed.Document, &quot;PUBLICATION_RESULT&quot;)</code>；<code>s.readResult(ctx, tx, actor, access, operation)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>s.Lookup(ctx, operation)</code>；<code>s.readResult(ctx, tx, actor, access, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 实际返回表达式：<code>nil</code>；<code>access.Sources</code> |
| <code>(匿名/未命名)</code> | <code>ReadBody</code> | 使用 <code>ReadBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>func(ctx context.Context, tx *sql.Tx, _ string) (any, error) { 		return s.readResult(ctx, tx, actor, access, operation) 	}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.EntityNotFound, &quot;PUBLICATION_NOT_FOUND&quot;)</code>；<code>domain.Violation(domain.PermissionDenied, &quot;PUBLICATION_NOT_ALLOWED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>fixed.Actor != actor.ID</code>。条件成立时执行 <code>return nil, nil, domain.Violation(domain.EntityNotFound, &quot;PUBLICATION_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>!access.Permissions.Manage</code>。条件成立时执行 <code>return nil, nil, domain.Violation(domain.PermissionDenied, &quot;PUBLICATION_NOT_ALLOWED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return access.Sources, func(ctx context.Context, tx *sql.Tx, _ string) (any, error) { 		return s.readResult(ctx, tx, actor, access, operation) 	}, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：此处通过 <code>return s.readResult(ctx, tx, actor, access, operation)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 24 bytes=22164:22894 -->
````go
func (s *Publications) Progress(ctx context.Context, actor Actor, operation string) ([]*pb.SourceEpoch, ReadBody, error) {
	fixed, err := s.Lookup(ctx, operation)
	if err != nil {
		return nil, nil, err
	}
	if fixed.Actor != actor.ID {
		return nil, nil, domain.Violation(domain.EntityNotFound, "PUBLICATION_NOT_FOUND")
	}
	access, err := s.access.Authorize(ctx, actor, fixed.Document, "PUBLICATION_RESULT")
	if err != nil {
		return nil, nil, err
	}
	if !access.Permissions.Manage {
		return nil, nil, domain.Violation(domain.PermissionDenied, "PUBLICATION_NOT_ALLOWED")
	}
	return access.Sources, func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		return s.readResult(ctx, tx, actor, access, operation)
	}, nil
}

````

## 第 25 步：Publications.readResult

插入位置：第 24 步代码之后，紧接追加；完成稿约第 567 行。

本步声明：`func (s *Publications) readResult(ctx context.Context, tx *sql.Tx, actor Actor, access DocumentAccess, operation string) (any, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>ValidateAccess(ctx, tx, access)</code>；<code>repository.PublicationResult(ctx, tx, operation)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>ValidateAccess(ctx, tx, access)</code>；<code>repository.PublicationResult(ctx, tx, operation)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor.ID</code> |
| <code>access</code> | <code>DocumentAccess</code> | 使用 <code>DocumentAccess</code> 类型；其定义处的职责：把账号身份、文档ACL与来源门闩组合成准入，并驱动可恢复撤权。<br>在此声明中的实际使用：<code>access.Permissions</code>；<code>ValidateAccess(ctx, tx, access)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>repository.PublicationResult(ctx, tx, operation)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>any</code> | 实际返回表达式：<code>nil</code>；<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.PermissionDenied, &quot;PUBLICATION_NOT_ALLOWED&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.EntityNotFound, &quot;PUBLICATION_NOT_FOUND&quot;)</code>；<code>domain.Violation(domain.EntityNotFound, &quot;OPERATION_EXPIRED&quot;)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!access.Permissions.Manage</code>。条件成立时执行 <code>return nil, domain.Violation(domain.PermissionDenied, &quot;PUBLICATION_NOT_ALLOWED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>receipt.Actor != actor.ID</code>。条件成立时执行 <code>return nil, domain.Violation(domain.EntityNotFound, &quot;PUBLICATION_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 检查 <code>!receipt.Expires.After(time.Now())</code>。条件成立时执行 <code>return nil, domain.Violation(domain.EntityNotFound, &quot;OPERATION_EXPIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(receipt.Result, &amp;value) != nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;PUBLICATION_RESULT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return value, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 25 bytes=22894:23835 -->
````go
func (s *Publications) readResult(ctx context.Context, tx *sql.Tx, actor Actor, access DocumentAccess, operation string) (any, error) {
	if !access.Permissions.Manage {
		return nil, domain.Violation(domain.PermissionDenied, "PUBLICATION_NOT_ALLOWED")
	}
	if _, err := ValidateAccess(ctx, tx, access); err != nil {
		return nil, err
	}
	receipt, err := repository.PublicationResult(ctx, tx, operation)
	if err != nil {
		return nil, err
	}
	if receipt.Actor != actor.ID {
		return nil, domain.Violation(domain.EntityNotFound, "PUBLICATION_NOT_FOUND")
	}
	if !receipt.Expires.After(time.Now()) {
		return nil, domain.Violation(domain.EntityNotFound, "OPERATION_EXPIRED")
	}
	var value map[string]any
	if json.Unmarshal(receipt.Result, &value) != nil {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "PUBLICATION_RESULT_UNKNOWN")
	}
	return value, nil
}

// publication_recovery.go：保留原实现，按业务合并文件。

````

## 第 26 步：Publications.RecoverOne

插入位置：第 25 步代码之后，紧接追加；完成稿约第 593 行。

本步声明：`func (s *Publications) RecoverOne(parent context.Context)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>parent</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>context.WithTimeout(parent, 15*time.Second)</code>；<code>context.WithTimeout(parent, time.Second)</code> |

**跟着本段代码走一遍：**

- 执行 <code>context.WithTimeout(parent, 15*time.Second)</code>。
- 登记 <code>cancel()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 检查 <code>json.Unmarshal(task.Payload, &amp;fixed) != nil &#124;&#124; validatePublicationContext(fixed) != nil</code>。
- 检查 <code>lookupErr != nil &#124;&#124; leftErr != nil &#124;&#124; rightErr != nil &#124;&#124; string(left) != string(right)</code>。
- 检查 <code>errors.As(err, &amp;violation) &amp;&amp; (violation.Kind == domain.PermissionDenied &#124;&#124; violation.Kind == domain.EntityNotFound &#124;&#124; violation.Kind == domain.StateConflict)</code>。
- 检查 <code>err != nil</code>。
- 执行 <code>context.WithTimeout(parent, time.Second)</code>。
- 登记 <code>stop()</code>，当前函数退出时执行，提前 return 也会触发。

<!-- write-step: 26 bytes=23835:25350 -->
````go
func (s *Publications) RecoverOne(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer tx.Rollback()
	task, err := repository.ClaimPublicationTask(ctx, tx, s.instance)
	if err != nil {
		return
	}
	if err = tx.Commit(); err != nil {
		return
	}
	var fixed publicationContext
	if json.Unmarshal(task.Payload, &fixed) != nil || validatePublicationContext(fixed) != nil {
		err = domain.Violation(domain.TemporarilyUnavailable, "PUBLICATION_BINDING_UNKNOWN")
	} else {
		// 领取载荷还必须与独立原上下文相同；不能靠两份可变进度字段一致就继续业务。
		original, lookupErr := s.Lookup(ctx, task.Operation)
		left, leftErr := json.Marshal(original)
		right, rightErr := json.Marshal(fixed)
		if lookupErr != nil || leftErr != nil || rightErr != nil || string(left) != string(right) {
			err = domain.Violation(domain.TemporarilyUnavailable, "PUBLICATION_BINDING_UNKNOWN")
		} else {
			err = s.recoverPublication(ctx, task, fixed)
		}
		var violation domain.RuleViolation
		if errors.As(err, &violation) && (violation.Kind == domain.PermissionDenied || violation.Kind == domain.EntityNotFound || violation.Kind == domain.StateConflict) {
			err = s.reject(ctx, task, violation.Code)
		}
	}
	if err != nil {
		retry, stop := context.WithTimeout(parent, time.Second)
		defer stop()
		_ = repository.RetryMergeTask(retry, s.db, task, publicationRecoveryCode(err))
	}
}

````

## 第 27 步：Publications.recoverPublication

插入位置：第 26 步代码之后，紧接追加；完成稿约第 633 行。

本步声明：`func (s *Publications) recoverPublication(ctx context.Context, task domain.SubmissionTask, fixed publicationContext) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.resume(ctx, fixed)</code>；<code>s.checkCurrent(ctx, fixed, proofs)</code>；<code>s.client.CaptureMain(ctx, publicationRequest(fixed))</code>；<code>s.commit(ctx, task, fixed, main, proofs)</code> |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>s.commit(ctx, task, fixed, main, proofs)</code> |
| <code>fixed</code> | <code>publicationContext</code> | 在此声明中的实际使用：<code>s.resume(ctx, fixed)</code>；<code>s.checkCurrent(ctx, fixed, proofs)</code>；<code>publicationRequest(fixed)</code>；<code>s.commit(ctx, task, fixed, main, proofs)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>currentErr</code>；<code>publicationCaptureError(err)</code>；<code>s.commit(ctx, task, fixed, main, proofs)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 检查 <code>currentErr != nil</code>。
- 检查 <code>errors.As(currentErr, &amp;violation) &amp;&amp; (violation.Kind == domain.PermissionDenied &#124;&#124; violation.Kind == domain.EntityNotFound &#124;&#124; violation.Kind == domain.StateConflict)</code>。条件成立时执行 <code>return currentErr</code>。当前函数在这里结束。
- 此处通过 <code>return publicationCaptureError(err)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return s.commit(ctx, task, fixed, main, proofs)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 27 bytes=25350:26565 -->
````go
func (s *Publications) recoverPublication(ctx context.Context, task domain.SubmissionTask, fixed publicationContext) error {
	proofs, err := s.resume(ctx, fixed)
	if err != nil {
		return err
	}
	// 不可达是未知结果，权威标题/控制变化却是明确冲突。先在短事务核验原来源，
	// 避免已经无效的预约因Node持续不可达而无限等待；事务提交后才发捕获RPC。
	if err = s.checkCurrent(ctx, fixed, proofs); err != nil {
		return err
	}
	main, err := s.client.CaptureMain(ctx, publicationRequest(fixed))
	if err != nil {
		// 私有来源查询到Node之间仍可能变化；用Doc真实当前事实判定失效，
		// 不把一个远端错误码或deadline猜成最终业务拒绝。
		if currentErr := s.checkCurrent(ctx, fixed, proofs); currentErr != nil {
			var violation domain.RuleViolation
			if errors.As(currentErr, &violation) && (violation.Kind == domain.PermissionDenied || violation.Kind == domain.EntityNotFound || violation.Kind == domain.StateConflict) {
				return currentErr
			}
		}
		return publicationCaptureError(err)
	}
	proofs, err = s.resume(ctx, fixed)
	if err != nil {
		return err
	}
	return s.commit(ctx, task, fixed, main, proofs)
}

````

## 第 28 步：Publications.checkCurrent

插入位置：第 27 步代码之后，紧接追加；完成稿约第 662 行。

本步声明：`func (s *Publications) checkCurrent(ctx context.Context, fixed publicationContext, proofs []*pb.IdentityAdmission) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, nil)</code>；<code>s.validateCurrent(ctx, tx, fixed, proofs)</code> |
| <code>fixed</code> | <code>publicationContext</code> | 在此声明中的实际使用：<code>s.validateCurrent(ctx, tx, fixed, proofs)</code> |
| <code>proofs</code> | <code>&#91;&#93;*pb.IdentityAdmission</code> | 在此声明中的实际使用：<code>s.validateCurrent(ctx, tx, fixed, proofs)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>tx.Commit()</code> |

**跟着本段代码走一遍：**

- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 28 bytes=26565:26885 -->
````go
func (s *Publications) checkCurrent(ctx context.Context, fixed publicationContext, proofs []*pb.IdentityAdmission) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = s.validateCurrent(ctx, tx, fixed, proofs); err != nil {
		return err
	}
	return tx.Commit()
}

````

## 第 29 步：Publications.reject

插入位置：第 28 步代码之后，紧接追加；完成稿约第 674 行。

本步声明：`func (s *Publications) reject(ctx context.Context, task domain.SubmissionTask, code string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.db.BeginTx(ctx, nil)</code>；<code>repository.LockOperationGate(ctx, tx, task.Operation)</code>；<code>repository.ValidateMergeTask(ctx, tx, task)</code>；<code>repository.UpdateOperationResult(ctx, tx, task.Operation, &quot;REJECTED&quot;, result)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>task</code> | <code>domain.SubmissionTask</code> | 使用 <code>domain.SubmissionTask</code> 类型；其定义处的职责：定义送审、固定轮次、修改意见及合并按钮条件。<br>在此声明中的实际使用：<code>task.Operation</code>；<code>repository.ValidateMergeTask(ctx, tx, task)</code>；<code>repository.FinishMergeTask(ctx, tx, task, &quot;REJECTED&quot;)</code> |
| <code>code</code> | <code>string</code> | 在此声明中的实际使用：<code>&quot;code&quot;: code</code>；<code>&quot;message&quot;: code</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>tx.Commit()</code> |

**跟着本段代码走一遍：**

- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return tx.Commit()</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。

<!-- write-step: 29 bytes=26885:27731 -->
````go
func (s *Publications) reject(ctx context.Context, task domain.SubmissionTask, code string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = repository.LockOperationGate(ctx, tx, task.Operation); err != nil {
		return err
	}
	if err = repository.ValidateMergeTask(ctx, tx, task); err != nil {
		return err
	}
	result, err := json.Marshal(map[string]any{"operation_id": task.Operation, "state": "REJECTED",
		"error": map[string]string{"code": code, "message": code, "trace_id": task.Operation, "retry_action": "NONE"}})
	if err != nil {
		return err
	}
	if err = repository.UpdateOperationResult(ctx, tx, task.Operation, "REJECTED", result); err != nil {
		return err
	}
	if err = repository.FinishMergeTask(ctx, tx, task, "REJECTED"); err != nil {
		return err
	}
	return tx.Commit()
}

````

## 第 30 步：publicationRecoveryCode

插入位置：第 29 步代码之后，紧接追加；完成稿约第 700 行。

本步声明：`func publicationRecoveryCode(err error) string`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>err</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>在此声明中的实际使用：<code>errors.As(err, &amp;violation)</code>；<code>errors.As(err, &amp;database)</code>；<code>status.FromError(err)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>violation.Code</code>；<code>fmt.Sprintf(&quot;PUBLICATION_SQL_%d&quot;, database.Number)</code>；<code>&quot;PUBLICATION_RPC_&quot; + rpc.Code().String()</code>；<code>&quot;PUBLICATION_RECOVERY_UNKNOWN&quot;</code> |

**跟着本段代码走一遍：**

- 检查 <code>errors.As(err, &amp;violation)</code>。条件成立时执行 <code>return violation.Code</code>。当前函数在这里结束。
- 检查 <code>errors.As(err, &amp;database)</code>。条件成立时执行 <code>return fmt.Sprintf(&quot;PUBLICATION_SQL_%d&quot;, database.Number)</code>。当前函数在这里结束。
- 检查 <code>ok</code>。条件成立时执行 <code>return &quot;PUBLICATION_RPC_&quot; + rpc.Code().String()</code>。当前函数在这里结束。
- 此处通过 <code>return &quot;PUBLICATION_RECOVERY_UNKNOWN&quot;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 30 bytes=27731:28213 -->
````go
func publicationRecoveryCode(err error) string {
	var violation domain.RuleViolation
	if errors.As(err, &violation) {
		return violation.Code
	}
	var database *mysql.MySQLError
	if errors.As(err, &database) {
		return fmt.Sprintf("PUBLICATION_SQL_%d", database.Number)
	}
	if rpc, ok := status.FromError(err); ok {
		return "PUBLICATION_RPC_" + rpc.Code().String()
	}
	return "PUBLICATION_RECOVERY_UNKNOWN"
}

// publication_results.go：保留原实现，按业务合并文件。

````

## 第 31 步：Publications.OperationResult

插入位置：第 30 步代码之后，紧接追加；完成稿约第 717 行。

OperationResult供已认证的内部参与者核验原系统事实；业务成功后的关闭/清理
不再要求用户在线。浏览器进度仍走Progress，必须复核当前身份及管理权限。

本步声明：`func (s *Publications) OperationResult(ctx context.Context, query *pb.OperationQuery) (*pb.OperationResult, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.Lookup(ctx, query.OperationId)</code>；<code>repository.PublicationResult(ctx, s.db, query.OperationId)</code> |
| <code>query</code> | <code>*pb.OperationQuery</code> | 在此声明中的实际使用：<code>query == nil</code>；<code>query.Target</code>；<code>query.OperationId</code>；<code>query.RequestFingerprint</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.OperationResult</code> | 实际返回表达式：<code>nil</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.InvalidRequest, &quot;OPERATION_QUERY_INVALID&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>；<code>unknown</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>query == nil &#124;&#124; query.Target == nil &#124;&#124; !domain.ValidOperationID(query.OperationId) &#124;&#124; len(query.RequestFingerprint) != 32</code>。条件成立时执行 <code>return nil, domain.Violation(domain.InvalidRequest, &quot;OPERATION_QUERY_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>!proto.Equal(query.Target, fixed.Main.Context.Target) &#124;&#124; !bytes.Equal(query.RequestFingerprint, fixed.Main.Context.RequestFingerprint)</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>fact.Actor != fixed.Actor &#124;&#124; fact.Kind != fixed.Intent.Kind &#124;&#124; fact.Generation == 0 &#124;&#124; !bytes.Equal(fact.Fingerprint, query.RequestFingerprint)</code>。条件成立时执行 <code>return nil, unknown</code>。当前函数在这里结束。
- 检查 <code>fact.State == &quot;PREPARING&quot;</code>。条件成立时执行 <code>return result, nil</code>。当前函数在这里结束。
- 检查 <code>fact.State != &quot;SUCCEEDED&quot; &amp;&amp; fact.State != &quot;REJECTED&quot;</code>。条件成立时执行 <code>return nil, unknown</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(fact.Result, &amp;value) != nil &#124;&#124; value.Operation != query.OperationId &#124;&#124; value.State != fact.State</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;PUBLICATION_RESULT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>fact.State == &quot;SUCCEEDED&quot;</code>。
- 检查 <code>value.Result.Entity != &quot;&quot;</code>。

<!-- write-step: 31 bytes=28213:30315 -->
````go
// OperationResult供已认证的内部参与者核验原系统事实；业务成功后的关闭/清理
// 不再要求用户在线。浏览器进度仍走Progress，必须复核当前身份及管理权限。
func (s *Publications) OperationResult(ctx context.Context, query *pb.OperationQuery) (*pb.OperationResult, error) {
	if query == nil || query.Target == nil || !domain.ValidOperationID(query.OperationId) || len(query.RequestFingerprint) != 32 {
		return nil, domain.Violation(domain.InvalidRequest, "OPERATION_QUERY_INVALID")
	}
	fixed, err := s.Lookup(ctx, query.OperationId)
	if err != nil {
		return nil, err
	}
	if !proto.Equal(query.Target, fixed.Main.Context.Target) || !bytes.Equal(query.RequestFingerprint, fixed.Main.Context.RequestFingerprint) {
		return nil, domain.Violation(domain.StateConflict, "OPERATION_ID_CONFLICT")
	}
	fact, err := repository.PublicationResult(ctx, s.db, query.OperationId)
	if err != nil {
		return nil, err
	}
	unknown := domain.Violation(domain.TemporarilyUnavailable, "PUBLICATION_BINDING_UNKNOWN")
	if fact.Actor != fixed.Actor || fact.Kind != fixed.Intent.Kind || fact.Generation == 0 || !bytes.Equal(fact.Fingerprint, query.RequestFingerprint) {
		return nil, unknown
	}
	result := &pb.OperationResult{OperationId: query.OperationId, Generation: fact.Generation, State: pb.OperationResult_PROCESSING}
	if fact.State == "PREPARING" {
		return result, nil
	}
	if fact.State != "SUCCEEDED" && fact.State != "REJECTED" {
		return nil, unknown
	}
	var value struct {
		Operation string `json:"operation_id"`
		State     string `json:"state"`
		Result    struct {
			Entity string `json:"entity_id"`
		} `json:"result"`
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if json.Unmarshal(fact.Result, &value) != nil || value.Operation != query.OperationId || value.State != fact.State {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "PUBLICATION_RESULT_UNKNOWN")
	}
	result.State = pb.OperationResult_REJECTED
	if fact.State == "SUCCEEDED" {
		result.State = pb.OperationResult_SUCCEEDED
		if value.Result.Entity != "" {
````

## 第 32 步：Publications.OperationResult（接着实现）

插入位置：第 31 步代码之后，紧接追加；完成稿约第 762 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;PUBLICATION_RESULT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 32 bytes=30315:30931 -->
````go
			id, err := domain.CanonicalID(value.Result.Entity)
			if err != nil {
				return nil, domain.Violation(domain.TemporarilyUnavailable, "PUBLICATION_RESULT_UNKNOWN")
			}
			result.ResultIds = []uint64{id}
		}
	}
	// 摘要只来自原SQL固定字节，不用当前HTTP投影、当前正式指针或一次重算结果替换。
	digest := sha256.Sum256(fact.Result)
	result.FixedResultJsonUtf8, result.ResultDigest, result.TerminalRevision = fact.Result, digest[:], fact.Generation
	result.BusinessCode = value.Error.Code
	return result, nil
}

// publication_source.go：保留原实现，按业务合并文件。

````

## 第 33 步：Publications.validateCurrent

插入位置：第 32 步代码之后，紧接追加；完成稿约第 778 行。

本步声明：`func (s *Publications) validateCurrent(ctx context.Context, tx *sql.Tx, fixed publicationContext, proofs []*pb.IdentityAdmission) (dto.Document, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>LockSources(ctx, tx, publicationLockSources(fixed.Access))</code>；<code>ValidateAccess(ctx, tx, fixed.Access)</code>；<code>repository.LockedMergeBarrier(ctx, tx, fixed.Document)</code>；<code>repository.MainWorkspaceEpoch(ctx, tx, fixed.Document)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>LockSources(ctx, tx, publicationLockSources(fixed.Access))</code>；<code>ValidateAccess(ctx, tx, fixed.Access)</code>；<code>repository.LockedMergeBarrier(ctx, tx, fixed.Document)</code>；<code>repository.MainWorkspaceEpoch(ctx, tx, fixed.Document)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>fixed</code> | <code>publicationContext</code> | 在此声明中的实际使用：<code>validatePublicationContext(fixed)</code>；<code>fixed.Access</code>；<code>fixed.Intent</code>；<code>fixed.Main</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>proofs</code> | <code>&#91;&#93;*pb.IdentityAdmission</code> | 在此声明中的实际使用：<code>validatePublicationProofs(fixed, proofs)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>dto.Document</code> | 实际返回表达式：<code>current</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;MAIN_SOURCE_CHANGED&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;MAIN_COMMIT_PENDING&quot;)</code>；<code>validatePublicationProofs(fixed, proofs)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return current, err</code>。当前函数在这里结束。
- 检查 <code>current.Title != fixed.Access.Document.Title &#124;&#124; current.CurrentManifestId != domain.TextID(fixed.Main.Manifest) &#124;&#124; 		current.CurrentStructureId != domain.TextID(fixed.Main.Structur …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return current, domain.Violation(domain.StateConflict, &quot;MAIN_SOURCE_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return current, err</code>。当前函数在这里结束。
- 检查 <code>barrier.Phase != &quot;OPEN&quot;</code>。条件成立时执行 <code>return current, domain.Violation(domain.StateConflict, &quot;MAIN_COMMIT_PENDING&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return current, err</code>。当前函数在这里结束。
- 检查 <code>epoch != reliableMainEpoch(fixed.Main)</code>。条件成立时执行 <code>return current, domain.Violation(domain.StateConflict, &quot;MAIN_SOURCE_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return current, err</code>。当前函数在这里结束。
- 此处通过 <code>return current, validatePublicationProofs(fixed, proofs)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 33 bytes=30931:32547 -->
````go
func (s *Publications) validateCurrent(ctx context.Context, tx *sql.Tx, fixed publicationContext, proofs []*pb.IdentityAdmission) (dto.Document, error) {
	var current dto.Document
	if err := validatePublicationContext(fixed); err != nil {
		return current, err
	}
	if err := LockSources(ctx, tx, publicationLockSources(fixed.Access)); err != nil {
		return current, err
	}
	current, err := ValidateAccess(ctx, tx, fixed.Access)
	if err != nil {
		return current, err
	}
	if err = domain.ValidatePublication(fixed.Access.Permissions, current, fixed.Intent); err != nil {
		return current, err
	}
	if current.Title != fixed.Access.Document.Title || current.CurrentManifestId != domain.TextID(fixed.Main.Manifest) ||
		current.CurrentStructureId != domain.TextID(fixed.Main.Structure) || current.CurrentStyleId != domain.TextID(fixed.Main.Style) ||
		current.ControlRevision != domain.TextID(fixed.Main.Control) {
		return current, domain.Violation(domain.StateConflict, "MAIN_SOURCE_CHANGED")
	}
	barrier, err := repository.LockedMergeBarrier(ctx, tx, fixed.Document)
	if err != nil {
		return current, err
	}
	if barrier.Phase != "OPEN" {
		return current, domain.Violation(domain.StateConflict, "MAIN_COMMIT_PENDING")
	}
	epoch, err := repository.MainWorkspaceEpoch(ctx, tx, fixed.Document)
	if err != nil {
		return current, err
	}
	if epoch != reliableMainEpoch(fixed.Main) {
		return current, domain.Violation(domain.StateConflict, "MAIN_SOURCE_CHANGED")
	}
	if err = validatePublicationAdmission(ctx, tx, fixed); err != nil {
		return current, err
	}
	return current, validatePublicationProofs(fixed, proofs)
}

````

## 第 34 步：Publications.MainSource

插入位置：第 33 步代码之后，紧接追加；完成稿约第 818 行。

本步声明：`func (s *Publications) MainSource(ctx context.Context, input *pb.CaptureMainRequest) (map[string]any, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.Lookup(ctx, input.Context.OperationId)</code>；<code>s.resume(ctx, fixed)</code>；<code>s.db.BeginTx(ctx, nil)</code>；<code>s.validateCurrent(ctx, tx, fixed, proofs)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>input</code> | <code>*pb.CaptureMainRequest</code> | 在此声明中的实际使用：<code>input == nil</code>；<code>input.Context</code>；<code>proto.Equal(publicationRequest(fixed), input)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>map&#91;string&#93;any</code> | 实际返回表达式：<code>nil</code>；<code>map&#91;string&#93;any{&quot;manifest_id&quot;: current.CurrentManifestId, &quot;structure_id&quot;: current.CurrentStructureId, &quot;style_id&quot;: current.CurrentStyleId, &quot;control_revision&quot;: current.ControlRevision …</code>（长表达式，按本步完整代码继续） |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.InvalidRequest, &quot;MAIN_SOURCE_QUERY_INVALID&quot;)</code>；<code>err</code>；<code>domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>input == nil &#124;&#124; input.Context == nil &#124;&#124; input.Context.Target == nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.InvalidRequest, &quot;MAIN_SOURCE_QUERY_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>fixed.Intent.Kind != &quot;RELEASE_CREATE&quot; &#124;&#124; !proto.Equal(publicationRequest(fixed), input)</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;OPERATION_ID_CONFLICT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return map&#91;string&#93;any{&quot;manifest_id&quot;: current.CurrentManifestId, &quot;structure_id&quot;: current.CurrentStructureId, &quot;style_id&quot;: current.CurrentStyleId, &quot;control_revision&quot;: current.ControlR …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 34 bytes=32547:33812 -->
````go
func (s *Publications) MainSource(ctx context.Context, input *pb.CaptureMainRequest) (map[string]any, error) {
	if input == nil || input.Context == nil || input.Context.Target == nil {
		return nil, domain.Violation(domain.InvalidRequest, "MAIN_SOURCE_QUERY_INVALID")
	}
	fixed, err := s.Lookup(ctx, input.Context.OperationId)
	if err != nil {
		return nil, err
	}
	if fixed.Intent.Kind != "RELEASE_CREATE" || !proto.Equal(publicationRequest(fixed), input) {
		return nil, domain.Violation(domain.StateConflict, "OPERATION_ID_CONFLICT")
	}
	proofs, err := s.resume(ctx, fixed)
	if err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	current, err := s.validateCurrent(ctx, tx, fixed, proofs)
	if err != nil {
		return nil, err
	}
	units, epoch, err := mainSourceUnits(ctx, tx, fixed.Document, current.CurrentManifestId, fixed.Main.Control)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return map[string]any{"manifest_id": current.CurrentManifestId, "structure_id": current.CurrentStructureId, "style_id": current.CurrentStyleId, "control_revision": current.ControlRevision, "workspace_epoch": domain.TextID(epoch), "units": units}, nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/logic/document/publication_service.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/07-publication.md)。
