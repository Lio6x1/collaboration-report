# 分步编写 `services/document/internal/logic/document/editing_service.go`

先按 [D02 · 创建工作区与可靠正文阅读](../lessons/document/02-workspace-content.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/204-editing-service.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

发放一次编辑票据、兑换固定协作grant，并安全读取正文版本与可编辑单元。

文档权限只能在特定workspace/epoch/room中编辑；旧票据不能在新工作区继续用。

写之前先按这条执行路线理解各段的连接：

1. Create锁文档/支线后登记房间与加密票据结果
2. Redeem校验原client/connection并允许原grant重取
3. Bootstrap提供可信基线及lineage
4. ContentVersions.Read验证版本实际业务范围。

本文件需要守住的边界：

- 丢失兑换响应可以重取原grant，不能换连接重复消费
- Go不能解析Yjs或访问Node私表。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/logic/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/logic/document/editing_service.go') -and (Get-Item -LiteralPath 'services/document/internal/logic/document/editing_service.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/logic/document/editing_service.go')) { New-Item -ItemType File -Path 'services/document/internal/logic/document/editing_service.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:591 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	codec "collaboration/contracts/content"
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"collaboration/platform"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// content_versions.go：保留原实现，按业务合并文件。

````

## 第 2 步：ContentVersions

插入位置：第 1 步代码之后，紧接追加；完成稿约第 24 行。

本步声明：`type ContentVersions struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>access</code> | <code>*Access</code> | 使用 <code>Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。 |
| <code>validator</code> | <code>*codec.Validator</code> | 冻结协议的结构校验器，先限制请求形状，再由业务核验身份、权限、版本和事务条件。 |

<!-- write-step: 2 bytes=591:690 -->
````go
type ContentVersions struct {
	db        *sql.DB
	access    *Access
	validator *codec.Validator
}

````

## 第 3 步：NewContentVersions

插入位置：第 2 步代码之后，紧接追加；完成稿约第 30 行。

本步声明：`func NewContentVersions(db *sql.DB, access *Access, validator *codec.Validator) *ContentVersions`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |
| <code>access</code> | <code>*Access</code> | 使用 <code>Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。<br>在此声明中的实际使用：<code>access: access</code> |
| <code>validator</code> | <code>*codec.Validator</code> | 冻结协议的结构校验器，先限制请求形状，再由业务核验身份、权限、版本和事务条件。<br>在此声明中的实际使用：<code>validator: validator</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*ContentVersions</code> | 实际返回表达式：<code>&amp;ContentVersions{db: db, access: access, validator: validator}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;ContentVersions{db: db, access: access, validator: validator}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=690:863 -->
````go
func NewContentVersions(db *sql.DB, access *Access, validator *codec.Validator) *ContentVersions {
	return &ContentVersions{db: db, access: access, validator: validator}
}

````

## 第 4 步：ContentVersions.Lookup

插入位置：第 3 步代码之后，紧接追加；完成稿约第 34 行。

本步声明：`func (s *ContentVersions) Lookup(ctx context.Context, id uint64) (domain.ContentVersionScope, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.ContentVersionScope(ctx, s.db, id)</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.ContentVersionScope(ctx, s.db, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.ContentVersionScope</code> | 使用 <code>domain.ContentVersionScope</code> 类型；其定义处的职责：版本号只标识不可变正文；当前可读范围仍由文档和原单元归属决定。<br>实际返回表达式：<code>scope</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.EntityNotFound, &quot;CONTENT_VERSION_NOT_FOUND&quot;)</code>；<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>err == sql.ErrNoRows</code>。条件成立时执行 <code>return scope, domain.Violation(domain.EntityNotFound, &quot;CONTENT_VERSION_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return scope, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=863:1164 -->
````go
func (s *ContentVersions) Lookup(ctx context.Context, id uint64) (domain.ContentVersionScope, error) {
	scope, err := repository.ContentVersionScope(ctx, s.db, id)
	if err == sql.ErrNoRows {
		return scope, domain.Violation(domain.EntityNotFound, "CONTENT_VERSION_NOT_FOUND")
	}
	return scope, err
}

````

## 第 5 步：ContentVersions.Read

插入位置：第 4 步代码之后，紧接追加；完成稿约第 42 行。

本步声明：`func (s *ContentVersions) Read(ctx context.Context, actor Actor, scope domain.ContentVersionScope) ([]*pb.SourceEpoch, ReadBody, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, scope.Document, &quot;READ_CONTENT_VERSION&quot;)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, scope.Document, &quot;READ_CONTENT_VERSION&quot;)</code> |
| <code>scope</code> | <code>domain.ContentVersionScope</code> | 使用 <code>domain.ContentVersionScope</code> 类型；其定义处的职责：版本号只标识不可变正文；当前可读范围仍由文档和原单元归属决定。<br>在此声明中的实际使用：<code>scope.Document</code>；<code>scope.Version</code>；<code>current != scope</code>；<code>scope.Unit</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 实际返回表达式：<code>nil</code>；<code>access.Sources</code> |
| <code>(匿名/未命名)</code> | <code>ReadBody</code> | 使用 <code>ReadBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil &#124;&#124; current != scope</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;CONTENT_VERSION_SCOPE_CHANGED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>!allowed &amp;&amp; (access.Permissions.Comment &#124;&#124; access.Permissions.Review)</code>。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>!allowed</code>。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>!allowed</code>。条件成立时执行 <code>return nil, domain.Violation(domain.PermissionDenied, &quot;CONTENT_VERSION_SCOPE_NOT_ALLOWED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return repository.ContentRow(ctx, tx, domain.TextID(scope.Version), domain.TextID(scope.Document))</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return access.Sources, run, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1164:2678 -->
````go
func (s *ContentVersions) Read(ctx context.Context, actor Actor, scope domain.ContentVersionScope) ([]*pb.SourceEpoch, ReadBody, error) {
	access, err := s.access.Authorize(ctx, actor, scope.Document, "READ_CONTENT_VERSION")
	if err != nil {
		return nil, nil, err
	}
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		// 归属查询只取得鉴权目标。正文交付前，在最终输出的同一事务和来源锁下重新核验。
		if _, err := ValidateAccess(ctx, tx, access); err != nil {
			return nil, err
		}
		current, err := repository.ContentVersionScope(ctx, tx, scope.Version)
		if err != nil || current != scope {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "CONTENT_VERSION_SCOPE_CHANGED")
		}
		allowed := access.Permissions.Manage || access.Permissions.Edit[scope.Unit]
		if !allowed && (access.Permissions.Comment || access.Permissions.Review) {
			allowed, err = repository.SharedContentVersion(ctx, tx, scope)
			if err != nil {
				return nil, err
			}
		}
		if !allowed {
			allowed, err = historicalContentRangeCovered(ctx, tx, access, scope.Document, scope.Unit, s.validator)
			if err != nil {
				return nil, err
			}
		}
		if !allowed {
			return nil, domain.Violation(domain.PermissionDenied, "CONTENT_VERSION_SCOPE_NOT_ALLOWED")
		}
		return repository.ContentRow(ctx, tx, domain.TextID(scope.Version), domain.TextID(scope.Document))
	}
	return access.Sources, run, nil
}

// edit_bootstrap.go：保留原实现，按业务合并文件。

````

## 第 6 步：EditSessions.Bootstrap

插入位置：第 5 步代码之后，紧接追加；完成稿约第 79 行。

Node seed仅来自当前Doc准入的固定基线；Go不解析Yjs，也不读取Node私有表。

本步声明：`func (s *EditSessions) Bootstrap(ctx context.Context, admission, grant string) (map[string]any, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.AdmissionSources(ctx, s.db, admission)</code>；<code>s.db.BeginTx(ctx, nil)</code>；<code>LockSources(ctx, tx, sources)</code>；<code>repository.LockedEditingAdmission(ctx, tx, admission)</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |
| <code>admission</code> | <code>string</code> | 在此声明中的实际使用：<code>repository.AdmissionSources(ctx, s.db, admission)</code>；<code>repository.LockedEditingAdmission(ctx, tx, admission)</code> |
| <code>grant</code> | <code>string</code> | 在此声明中的实际使用：<code>binding.GrantID != grant</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>map&#91;string&#93;any</code> | 实际返回表达式：<code>nil</code>；<code>map&#91;string&#93;any{&quot;workspace_kind&quot;: binding.WorkspaceKind, &quot;workspace_id&quot;: domain.TextID(binding.WorkspaceID), &quot;body&quot;: json.RawMessage(body), &quot;base_version_id&quot;: domain.TextID(version) …</code>（长表达式，按本步完整代码继续） |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.PermissionDenied, &quot;ADMISSION_CLOSED&quot;)</code>；<code>domain.Violation(domain.PermissionDenied, &quot;GRANT_BINDING_INVALID&quot;)</code>；<code>domain.Violation(domain.StateConflict, &quot;WORKSPACE_NOT_CURRENT&quot;)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; current.State != &quot;OPEN&quot; &#124;&#124; !current.Expires.After(time.Now())</code>。条件成立时执行 <code>return nil, domain.Violation(domain.PermissionDenied, &quot;ADMISSION_CLOSED&quot;)</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(current.TargetJSON, &amp;binding) != nil &#124;&#124; binding.GrantID != grant &#124;&#124; binding.Target == nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.PermissionDenied, &quot;GRANT_BINDING_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; lifecycle != &quot;ACTIVE&quot; &#124;&#124; structure != binding.Target.StructureId</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;WORKSPACE_NOT_CURRENT&quot;)</code>。当前函数在这里结束。
- 检查 <code>binding.WorkspaceKind == &quot;BRANCH&quot;</code>。
- 检查 <code>err == nil</code>。
- 检查 <code>err == nil &amp;&amp; json.Valid(provenance)</code>。
- 检查 <code>err == nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;LINEAGE_INVALID&quot;)</code>。当前函数在这里结束。

<!-- write-step: 6 bytes=2678:4793 -->
````go
// Node seed仅来自当前Doc准入的固定基线；Go不解析Yjs，也不读取Node私有表。
func (s *EditSessions) Bootstrap(ctx context.Context, admission, grant string) (map[string]any, error) {
	sources, err := repository.AdmissionSources(ctx, s.db, admission)
	if err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = LockSources(ctx, tx, sources); err != nil {
		return nil, err
	}
	current, err := repository.LockedEditingAdmission(ctx, tx, admission)
	if err != nil || current.State != "OPEN" || !current.Expires.After(time.Now()) {
		return nil, domain.Violation(domain.PermissionDenied, "ADMISSION_CLOSED")
	}
	var binding EditBinding
	if json.Unmarshal(current.TargetJSON, &binding) != nil || binding.GrantID != grant || binding.Target == nil {
		return nil, domain.Violation(domain.PermissionDenied, "GRANT_BINDING_INVALID")
	}
	lifecycle, structure, err := repository.LockedEditingStructure(ctx, tx, binding.Target.DocumentId)
	if err != nil || lifecycle != "ACTIVE" || structure != binding.Target.StructureId {
		return nil, domain.Violation(domain.StateConflict, "WORKSPACE_NOT_CURRENT")
	}
	var body []byte
	var version uint64
	var lineageContext any
	var mainContext map[string]string
	if binding.WorkspaceKind == "BRANCH" {
		body, version, err = repository.BranchBootstrapBody(ctx, tx, binding.WorkspaceID, binding.Target.DocumentId, binding.Target.UnitId, binding.Target.WorkspaceEpoch)
		if err == nil {
			body, err = domain.BranchBaselineBody(body)
		}
		if err == nil {
			var lineage, manifest uint64
			var provenance []byte
			lineage, manifest, provenance, err = repository.BranchBootstrapLineage(ctx, tx, binding.WorkspaceID)
			if err == nil && json.Valid(provenance) {
				lineageContext = map[string]any{"lineage_id": domain.TextID(lineage), "base_manifest_id": domain.TextID(manifest), "base_version_id": domain.TextID(version), "provenance": json.RawMessage(provenance)}
			} else if err == nil {
				return nil, domain.Violation(domain.StateConflict, "LINEAGE_INVALID")
			}
		}
	} else {
````

## 第 7 步：EditSessions.Bootstrap（接着实现）

插入位置：第 6 步代码之后，紧接追加；完成稿约第 125 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err == nil</code>。
- 检查 <code>err != nil &#124;&#124; !json.Valid(body)</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;WORKSPACE_NOT_CURRENT&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return map&#91;string&#93;any{&quot;workspace_kind&quot;: binding.WorkspaceKind, &quot;workspace_id&quot;: domain.TextID(binding.WorkspaceID), &quot;body&quot;: json.RawMessage(body), &quot;base_version_id&quot;: domain.TextID(v …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=4793:5774 -->
````go
		body, version, err = repository.MainBootstrapBody(ctx, tx, binding.Target.DocumentId, binding.Target.UnitId)
		if err == nil {
			mainContext, err = repository.BootstrapMainContext(ctx, tx, binding.Target.DocumentId)
		}
	}
	if err != nil || !json.Valid(body) {
		return nil, domain.Violation(domain.StateConflict, "WORKSPACE_NOT_CURRENT")
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	// HTTP BIGINT保持十进制字符串，不能经过Node的JSON Number损失房间或版本身份。
	return map[string]any{"workspace_kind": binding.WorkspaceKind, "workspace_id": domain.TextID(binding.WorkspaceID), "body": json.RawMessage(body), "base_version_id": domain.TextID(version), "actor_id": domain.TextID(current.Actor), "session_ref": base64.RawStdEncoding.EncodeToString(current.SessionRef), "sources": SourcesWire(sources), "lineage_context": lineageContext, "main_context": mainContext}, nil
}

// edit_protocol.go：保留原实现，按业务合并文件。

````

## 第 8 步：EditBinding

插入位置：第 7 步代码之后，紧接追加；完成稿约第 142 行。

EditBinding是持久准入的固定绑定，沿用生成Target；它不包含可变Yjs正文。

本步声明：`type EditBinding struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Target</code> | <code>*pb.Target</code> | 序列化/校验标记：<code>`json:&quot;target&quot;`</code> |
| <code>RoomID</code> | <code>uint64</code> | 序列化/校验标记：<code>`json:&quot;room_id&quot;`</code> |
| <code>WorkspaceKind</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;workspace_kind&quot;`</code> |
| <code>WorkspaceID</code> | <code>uint64</code> | 序列化/校验标记：<code>`json:&quot;workspace_id&quot;`</code> |
| <code>ClientSessionID</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;client_session_id&quot;`</code> |
| <code>ConnectionID</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;connection_id,omitempty&quot;`</code> |
| <code>GrantID</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;grant_id,omitempty&quot;`</code> |

<!-- write-step: 8 bytes=5774:6329 -->
````go
// EditBinding是持久准入的固定绑定，沿用生成Target；它不包含可变Yjs正文。
type EditBinding struct {
	Target          *pb.Target `json:"target"`
	RoomID          uint64     `json:"room_id"`
	WorkspaceKind   string     `json:"workspace_kind"`
	WorkspaceID     uint64     `json:"workspace_id"`
	ClientSessionID string     `json:"client_session_id"`
	ConnectionID    string     `json:"connection_id,omitempty"`
	GrantID         string     `json:"grant_id,omitempty"`
}

// edit_redeem.go：保留原实现，按业务合并文件。

````

## 第 9 步：ticketHash

插入位置：第 8 步代码之后，紧接追加；完成稿约第 155 行。

本步声明：`func ticketHash(token []byte) ([32]byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>token</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>string(token)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;32&#93;byte</code> | 实际返回表达式：<code>&#91;32&#93;byte{}</code>；<code>sha256.Sum256(raw)</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>domain.Violation(domain.InvalidRequest, &quot;TICKET_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil &#124;&#124; len(raw) != 32 &#124;&#124; base64.RawURLEncoding.EncodeToString(raw) != string(token)</code>。条件成立时执行 <code>return &#91;32&#93;byte{}, domain.Violation(domain.InvalidRequest, &quot;TICKET_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return sha256.Sum256(raw), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=6329:6656 -->
````go
func ticketHash(token []byte) ([32]byte, error) {
	raw, err := base64.RawURLEncoding.DecodeString(string(token))
	if err != nil || len(raw) != 32 || base64.RawURLEncoding.EncodeToString(raw) != string(token) {
		return [32]byte{}, domain.Violation(domain.InvalidRequest, "TICKET_INVALID")
	}
	return sha256.Sum256(raw), nil
}

````

## 第 10 步：EditSessions.Redeem

插入位置：第 9 步代码之后，紧接追加；完成稿约第 163 行。

一次票据只可被原client/connection重取原grant，丢回执不会变成多人可复用凭据。

本步声明：`func (s *EditSessions) Redeem(ctx context.Context, req *pb.RedeemTicketRequest) (*pb.CollaborationGrant, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.TicketAdmission(ctx, s.db, hash&#91;:&#93;)</code>；<code>repository.AdmissionSources(ctx, s.db, admission)</code>；<code>s.db.BeginTx(ctx, nil)</code>；<code>LockSources(ctx, tx, sources)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>req</code> | <code>*pb.RedeemTicketRequest</code> | 在此声明中的实际使用：<code>req.Ticket</code>；<code>req.ClientSessionId</code>；<code>req.ConnectionId</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.CollaborationGrant</code> | 实际返回表达式：<code>nil</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.PermissionDenied, &quot;TICKET_NOT_CURRENT&quot;)</code>；<code>domain.Violation(domain.PermissionDenied, &quot;SOURCES_REQUIRED&quot;)</code>；<code>domain.Violation(domain.PermissionDenied, &quot;SOURCE_CLOSED&quot;)</code><br>另有 5 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.PermissionDenied, &quot;TICKET_NOT_CURRENT&quot;)</code>。当前函数在这里结束。
- 检查 <code>errors.Is(err, domain.ErrSourcesRequired)</code>。条件成立时执行 <code>return nil, domain.Violation(domain.PermissionDenied, &quot;SOURCES_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.PermissionDenied, &quot;SOURCE_CLOSED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; ticket.Admission.State != &quot;OPEN&quot; &#124;&#124; ticket.Client != req.ClientSessionId &#124;&#124; !ticket.Admission.Expires.After(time.Now()) &#124;&#124; ticket.State == &quot;CLOSED&quot;</code>。条件成立时执行 <code>return nil, domain.Violation(domain.PermissionDenied, &quot;TICKET_NOT_CURRENT&quot;)</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(ticket.Admission.TargetJSON, &amp;binding) != nil &#124;&#124; binding.Target == nil &#124;&#124; binding.ClientSessionID != ticket.Client</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;TICKET_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>ticket.State == &quot;REDEEMED&quot;</code>。
- 检查 <code>binding.ConnectionID != req.ConnectionId &#124;&#124; binding.GrantID == &quot;&quot;</code>。条件成立时执行 <code>return nil, domain.Violation(domain.PermissionDenied, &quot;TICKET_ALREADY_REDEEMED&quot;)</code>。当前函数在这里结束。
- 检查 <code>!ticket.Expires.After(time.Now())</code>。条件成立时执行 <code>return nil, domain.Violation(domain.PermissionDenied, &quot;TICKET_EXPIRED&quot;)</code>。当前函数在这里结束。

<!-- write-step: 10 bytes=6656:8676 -->
````go
// 一次票据只可被原client/connection重取原grant，丢回执不会变成多人可复用凭据。
func (s *EditSessions) Redeem(ctx context.Context, req *pb.RedeemTicketRequest) (*pb.CollaborationGrant, error) {
	hash, err := ticketHash(req.Ticket)
	if err != nil {
		return nil, err
	}
	admission, err := repository.TicketAdmission(ctx, s.db, hash[:])
	if err != nil {
		return nil, domain.Violation(domain.PermissionDenied, "TICKET_NOT_CURRENT")
	}
	sources, err := repository.AdmissionSources(ctx, s.db, admission)
	if errors.Is(err, domain.ErrSourcesRequired) {
		return nil, domain.Violation(domain.PermissionDenied, "SOURCES_REQUIRED")
	}
	if err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	// 先锁真实来源，再锁准入与票据，保持与关闭路径一致的锁顺序。
	if err = LockSources(ctx, tx, sources); err != nil {
		return nil, domain.Violation(domain.PermissionDenied, "SOURCE_CLOSED")
	}
	ticket, err := repository.LockedEditingTicket(ctx, tx, hash[:])
	if err != nil || ticket.Admission.State != "OPEN" || ticket.Client != req.ClientSessionId || !ticket.Admission.Expires.After(time.Now()) || ticket.State == "CLOSED" {
		return nil, domain.Violation(domain.PermissionDenied, "TICKET_NOT_CURRENT")
	}
	var binding EditBinding
	if json.Unmarshal(ticket.Admission.TargetJSON, &binding) != nil || binding.Target == nil || binding.ClientSessionID != ticket.Client {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "TICKET_BINDING_UNKNOWN")
	}
	if ticket.State == "REDEEMED" {
		if binding.ConnectionID != req.ConnectionId || binding.GrantID == "" {
			return nil, domain.Violation(domain.PermissionDenied, "TICKET_ALREADY_REDEEMED")
		}
	} else {
		if !ticket.Expires.After(time.Now()) {
			return nil, domain.Violation(domain.PermissionDenied, "TICKET_EXPIRED")
		}
		binding.ConnectionID = req.ConnectionId
		binding.GrantID = platform.NewTraceID()
		encoded, _ := json.Marshal(binding)
````

## 第 11 步：EditSessions.Redeem（接着实现）

插入位置：第 10 步代码之后，紧接追加；完成稿约第 208 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.TemporarilyUnavailable, &quot;REDEEM_COMMIT_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=8676:9584 -->
````go
		if err = repository.BindEditingGrant(ctx, tx, admission, binding.GrantID, hash[:], encoded); err != nil {
			return nil, err
		}
	}
	fingerprint := sha256.Sum256(append(append([]byte{}, hash[:]...), []byte(req.ClientSessionId+"\n"+req.ConnectionId)...))
	operation := &pb.OperationContext{OperationId: binding.GrantID, ActorId: ticket.Admission.Actor, DocumentAdmissionRef: admission, Target: binding.Target, Sources: sources, RequestFingerprint: fingerprint[:]}
	result := &pb.CollaborationGrant{GrantId: binding.GrantID, Context: operation, RoomId: binding.RoomID, Capabilities: []string{"EDIT", "READ"}, ExpiresUnixMs: ticket.Admission.Expires.UnixMilli(), ClientSessionId: ticket.Client}
	if err = tx.Commit(); err != nil {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "REDEEM_COMMIT_UNKNOWN")
	}
	return result, nil
}

// edit_session.go：保留原实现，按业务合并文件。

````

## 第 12 步：EditSessions

插入位置：第 11 步代码之后，紧接追加；完成稿约第 223 行。

本步声明：`type EditSessions struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。 |
| <code>access</code> | <code>*Access</code> | 使用 <code>Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。 |
| <code>tokens</code> | <code>InvitationCipher</code> | 使用 <code>InvitationCipher</code> 类型；其定义处的职责：服务器需再次展示邀请链接时保存密文；查询匹配另用摘要，不在数据库公开原文令牌。 |
| <code>origin</code> | <code>string</code> | 用户站点的固定源地址，用于构建同源编辑/资源/邀请入口；不能信任任意客户端Host。 |

<!-- write-step: 12 bytes=9584:9685 -->
````go
type EditSessions struct {
	db     *sql.DB
	access *Access
	tokens InvitationCipher
	origin string
}
````

## 第 13 步：editResult

插入位置：第 12 步代码之后，紧接追加；完成稿约第 229 行。

本步声明：`type editResult struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Session</code> | <code>dto.EditSession</code> | 序列化/校验标记：<code>`json:&quot;session&quot;`</code> |
| <code>Ciphertext</code> | <code>&#91;&#93;byte</code> | 序列化/校验标记：<code>`json:&quot;ciphertext&quot;`</code> |
| <code>Binding</code> | <code>EditBinding</code> | 使用 <code>EditBinding</code> 类型；其定义处的职责：EditBinding是持久准入的固定绑定，沿用生成Target；它不包含可变Yjs正文。<br>序列化/校验标记：<code>`json:&quot;binding&quot;`</code> |

<!-- write-step: 13 bytes=9685:9851 -->
````go
type editResult struct {
	Session    dto.EditSession `json:"session"`
	Ciphertext []byte          `json:"ciphertext"`
	Binding    EditBinding     `json:"binding"`
}

````

## 第 14 步：NewEditSessions

插入位置：第 13 步代码之后，紧接追加；完成稿约第 235 行。

本步声明：`func NewEditSessions(db *sql.DB, access *Access, tokens InvitationCipher, origin string) *EditSessions`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>db</code> | <code>*sql.DB</code> | 服务共享的数据库连接池；一次独立查询可从池取连接，需要原子提交的一组语句则应共用事务。<br>在此声明中的实际使用：<code>db: db</code> |
| <code>access</code> | <code>*Access</code> | 使用 <code>Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。<br>在此声明中的实际使用：<code>access: access</code> |
| <code>tokens</code> | <code>InvitationCipher</code> | 使用 <code>InvitationCipher</code> 类型；其定义处的职责：服务器需再次展示邀请链接时保存密文；查询匹配另用摘要，不在数据库公开原文令牌。<br>在此声明中的实际使用：<code>tokens: tokens</code> |
| <code>origin</code> | <code>string</code> | 在此声明中的实际使用：<code>origin: origin</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*EditSessions</code> | 实际返回表达式：<code>&amp;EditSessions{db: db, access: access, tokens: tokens, origin: origin}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;EditSessions{db: db, access: access, tokens: tokens, origin: origin}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=9851:10037 -->
````go
func NewEditSessions(db *sql.DB, access *Access, tokens InvitationCipher, origin string) *EditSessions {
	return &EditSessions{db: db, access: access, tokens: tokens, origin: origin}
}

````

## 第 15 步：collaborationRoom

插入位置：第 14 步代码之后，紧接追加；完成稿约第 239 行。

房间身份由Doc持久登记，同一workspace/epoch复用；Go不查询Node的私有库。

本步声明：`func collaborationRoom(ctx context.Context, tx *sql.Tx, kind string, workspace, unit, epoch uint64) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.EnsureCollaborationRoom(ctx, tx, key, raw)</code>；<code>repository.LockedCollaborationRoom(ctx, tx, key)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.EnsureCollaborationRoom(ctx, tx, key, raw)</code>；<code>repository.LockedCollaborationRoom(ctx, tx, key)</code> |
| <code>kind</code> | <code>string</code> | 在此声明中的实际使用：<code>kind + &quot;:&quot;</code> |
| <code>workspace</code> | <code>uint64</code> | 在此声明中的实际使用：<code>domain.TextID(workspace)</code> |
| <code>unit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>domain.TextID(unit)</code> |
| <code>epoch</code> | <code>uint64</code> | 在此声明中的实际使用：<code>domain.TextID(epoch)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 由 <code>domain.CanonicalID(found&#91;&quot;room_id&quot;&#93;)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>0</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>domain.CanonicalID(found&#91;&quot;room_id&quot;&#93;)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.TemporarilyUnavailable, &quot;ROOM_BINDING_UNAVAILABLE&quot;)</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return 0, err</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(raw, &amp;found) != nil</code>。条件成立时执行 <code>return 0, domain.Violation(domain.TemporarilyUnavailable, &quot;ROOM_BINDING_UNAVAILABLE&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return domain.CanonicalID(found&#91;&quot;room_id&quot;&#93;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=10037:10896 -->
````go
// 房间身份由Doc持久登记，同一workspace/epoch复用；Go不查询Node的私有库。
func collaborationRoom(ctx context.Context, tx *sql.Tx, kind string, workspace, unit, epoch uint64) (uint64, error) {
	key := kind + ":" + domain.TextID(workspace) + ":" + domain.TextID(unit) + ":" + domain.TextID(epoch)
	room, err := NewEntityID()
	if err != nil {
		return 0, err
	}
	raw, _ := json.Marshal(map[string]string{"room_id": domain.TextID(room)})
	if err = repository.EnsureCollaborationRoom(ctx, tx, key, raw); err != nil {
		return 0, err
	}
	raw, err = repository.LockedCollaborationRoom(ctx, tx, key)
	if err != nil {
		return 0, err
	}
	var found map[string]string
	if json.Unmarshal(raw, &found) != nil {
		return 0, domain.Violation(domain.TemporarilyUnavailable, "ROOM_BINDING_UNAVAILABLE")
	}
	return domain.CanonicalID(found["room_id"])
}
````

## 第 16 步：EditSessions.Create

插入位置：第 15 步代码之后，紧接追加；完成稿约第 260 行。

本步声明：`func (s *EditSessions) Create(ctx context.Context, actor Actor, document, unit uint64, input dto.EditSessionCreate) (Actor, MutationBody, MutationPresentation, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, document, &quot;EDIT_SESSION&quot;)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, document, &quot;EDIT_SESSION&quot;)</code>；<code>return actor, nil, nil, err</code>；<code>return actor, nil, nil, domain.Violation(domain.PermissionDenied, &quot;UNIT_EDIT_REQUIRED&quot;)</code>；<code>actor.Sources</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, document, &quot;EDIT_SESSION&quot;)</code>；<code>repository.LockedActiveBranch(ctx, tx, unit, document)</code>；<code>repository.LockedEditWorkspace(ctx, tx, branch, document, unit)</code>；<code>repository.MainWorkspaceEpoch(ctx, tx, document)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |
| <code>unit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>!access.Permissions.Edit&#91;unit&#93;</code>；<code>repository.LockedActiveBranch(ctx, tx, unit, document)</code>；<code>Workspace: unit</code>；<code>repository.LockedEditWorkspace(ctx, tx, branch, document, unit)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>input</code> | <code>dto.EditSessionCreate</code> | 在此声明中的实际使用：<code>input.ExpectedStructureId</code>；<code>input.WorkspaceKind</code>；<code>input.ClientSessionId</code>；<code>input.OperationId</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>实际返回表达式：<code>actor</code> |
| <code>(匿名/未命名)</code> | <code>MutationBody</code> | 使用 <code>MutationBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>MutationPresentation</code> | 使用 <code>MutationPresentation</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>present</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.PermissionDenied, &quot;UNIT_EDIT_REQUIRED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return actor, nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>!access.Permissions.Edit&#91;unit&#93;</code>。条件成立时执行 <code>return actor, nil, nil, domain.Violation(domain.PermissionDenied, &quot;UNIT_EDIT_REQUIRED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>current.CurrentStructureId != input.ExpectedStructureId</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;STRUCTURE_CHANGED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>input.WorkspaceKind == &quot;MAIN&quot; &amp;&amp; current.MainLocked</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;MAIN_LOCKED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;UNIT_CHANGED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>input.WorkspaceKind == &quot;BRANCH&quot;</code>。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;BRANCH_NOT_READY&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>sessionExpires.Before(grantExpires)</code>。

<!-- write-step: 16 bytes=10896:12795 -->
````go
func (s *EditSessions) Create(ctx context.Context, actor Actor, document, unit uint64, input dto.EditSessionCreate) (Actor, MutationBody, MutationPresentation, error) {
	access, err := s.access.Authorize(ctx, actor, document, "EDIT_SESSION")
	if err != nil {
		return actor, nil, nil, err
	}
	if !access.Permissions.Edit[unit] {
		return actor, nil, nil, domain.Violation(domain.PermissionDenied, "UNIT_EDIT_REQUIRED")
	}
	actor.Sources = access.Sources
	run := func(ctx context.Context, tx *sql.Tx) (map[string]any, error) {
		current, err := ValidateAccess(ctx, tx, access)
		if err != nil {
			return nil, err
		}
		if current.CurrentStructureId != input.ExpectedStructureId {
			return nil, domain.Violation(domain.StateConflict, "STRUCTURE_CHANGED")
		}
		if input.WorkspaceKind == "MAIN" && current.MainLocked {
			return nil, domain.Violation(domain.StateConflict, "MAIN_LOCKED")
		}
		branch, err := repository.LockedActiveBranch(ctx, tx, unit, document)
		if err != nil {
			return nil, domain.Violation(domain.StateConflict, "UNIT_CHANGED")
		}
		workspace := domain.EditWorkspace{Workspace: unit, Epoch: 1}
		if input.WorkspaceKind == "BRANCH" {
			workspace, err = repository.LockedEditWorkspace(ctx, tx, branch, document, unit)
			if err != nil {
				return nil, domain.Violation(domain.StateConflict, "BRANCH_NOT_READY")
			}
		} else {
			workspace.Epoch, err = repository.MainWorkspaceEpoch(ctx, tx, document)
			if err != nil {
				return nil, err
			}
			branch = 0
		}
		room, err := collaborationRoom(ctx, tx, input.WorkspaceKind, workspace.Workspace, unit, workspace.Epoch)
		if err != nil {
			return nil, err
		}
		// grant不超过实际登录会话期限；兑换票据另有一分钟期限，二者不混用。
		grantExpires := time.Now().UTC().Add(5 * time.Minute)
		sessionExpires := time.UnixMilli(actor.SessionExpiresUnixMs)
		if sessionExpires.Before(grantExpires) {
````

## 第 17 步：EditSessions.Create（接着实现）

插入位置：第 16 步代码之后，紧接追加；完成稿约第 305 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：检查 <code>grantExpires.Before(ticketExpires)</code>。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>input.WorkspaceKind == &quot;BRANCH&quot;</code>。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return OperationResult(input.OperationId, &quot;SUCCEEDED&quot;, room), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 17 bytes=12795:15308 -->
````go
			grantExpires = sessionExpires
		}
		ticketExpires := time.Now().UTC().Add(time.Minute)
		if grantExpires.Before(ticketExpires) {
			ticketExpires = grantExpires
		}
		var secret [32]byte
		if _, err = rand.Read(secret[:]); err != nil {
			return nil, err
		}
		token := base64.RawURLEncoding.EncodeToString(secret[:])
		hash := sha256.Sum256(secret[:])
		admission := platform.NewTraceID()
		structure, _ := domain.CanonicalID(current.CurrentStructureId)
		binding := EditBinding{Target: &pb.Target{DocumentId: document, UnitId: unit, BranchId: branch, StructureId: structure, WorkspaceEpoch: workspace.Epoch, Purpose: "EDIT_" + input.WorkspaceKind}, RoomID: room, WorkspaceKind: input.WorkspaceKind, WorkspaceID: workspace.Workspace, ClientSessionID: input.ClientSessionId}
		if err = InsertAdmission(ctx, tx, actor, access.Sources, document, "EDIT_"+input.WorkspaceKind, admission); err != nil {
			return nil, err
		}
		target, _ := json.Marshal(binding)
		if err = repository.ConfigureEditAdmission(ctx, tx, unit, admission, target, grantExpires); err != nil {
			return nil, err
		}
		if err = repository.InsertEditTicket(ctx, tx, hash[:], admission, input.ClientSessionId, input.WorkspaceKind, workspace.Workspace, workspace.Epoch, ticketExpires); err != nil {
			return nil, err
		}
		encrypted, err := s.tokens.Seal(token, "edit:"+domain.TextID(actor.ID)+":"+input.OperationId)
		if err != nil {
			return nil, err
		}
		result := editResult{Session: dto.EditSession{TicketExpiresAt: ticketExpires.Format(time.RFC3339Nano), WssUrl: strings.Replace(s.origin, "https://", "wss://", 1) + "/collaboration", RoomId: domain.TextID(room), WorkspaceEpoch: domain.TextID(workspace.Epoch)}, Ciphertext: encrypted, Binding: binding}
		if input.WorkspaceKind == "BRANCH" {
			b, l, m, v := domain.TextID(branch), domain.TextID(workspace.Lineage), domain.TextID(workspace.BaseManifest), domain.TextID(workspace.BaseVersion)
			result.Session.BranchId = &b
			result.Session.LineageId = &l
			result.Session.BaseManifestId = &m
			result.Session.BaseVersionId = &v
		}
		encoded, _ := json.Marshal(result)
		if err = repository.InsertEditSessionResult(ctx, tx, input.OperationId, encoded); err != nil {
			return nil, err
		}
		return OperationResult(input.OperationId, "SUCCEEDED", room), nil
	}
	// 原号重试返回同一票据密文，但当前EDIT资格与原期限仍须成立，不发行第二张票据。
	present := func(ctx context.Context, tx *sql.Tx, _ map[string]any) (any, []*pb.SourceEpoch, error) {
````

## 第 18 步：EditSessions.Create（接着实现）

插入位置：第 17 步代码之后，紧接追加；完成稿约第 350 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, domain.Violation(domain.TemporarilyUnavailable, &quot;EDIT_SESSION_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>json.Unmarshal(raw, &amp;result) != nil</code>。条件成立时执行 <code>return nil, nil, domain.Violation(domain.TemporarilyUnavailable, &quot;EDIT_SESSION_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil &#124;&#124; !expiry.After(time.Now())</code>。条件成立时执行 <code>return nil, nil, domain.Violation(domain.StateConflict, &quot;TICKET_EXPIRED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return result.Session, access.Sources, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return actor, run, present, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=15308:16215 -->
````go
		if _, err := ValidateAccess(ctx, tx, access); err != nil {
			return nil, nil, err
		}
		raw, err := repository.EditSessionResult(ctx, tx, input.OperationId)
		if err != nil {
			return nil, nil, domain.Violation(domain.TemporarilyUnavailable, "EDIT_SESSION_UNKNOWN")
		}
		var result editResult
		if json.Unmarshal(raw, &result) != nil {
			return nil, nil, domain.Violation(domain.TemporarilyUnavailable, "EDIT_SESSION_UNKNOWN")
		}
		expiry, err := time.Parse(time.RFC3339Nano, result.Session.TicketExpiresAt)
		if err != nil || !expiry.After(time.Now()) {
			return nil, nil, domain.Violation(domain.StateConflict, "TICKET_EXPIRED")
		}
		result.Session.Ticket, err = s.tokens.Open(result.Ciphertext, "edit:"+domain.TextID(actor.ID)+":"+input.OperationId)
		return result.Session, access.Sources, err
	}
	return actor, run, present, nil
}

// editable.go：保留原实现，按业务合并文件。

````

## 第 19 步：EditableUnits

插入位置：第 18 步代码之后，紧接追加；完成稿约第 373 行。

本步声明：`type EditableUnits struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>access</code> | <code>*Access</code> | 使用 <code>Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。 |
| <code>pagination</code> | <code>*Pagination</code> | 使用 <code>Pagination</code> 类型；其定义处的职责：生成并校验绑定查询上下文的签名翻页游标。 |

<!-- write-step: 19 bytes=16215:16290 -->
````go
type EditableUnits struct {
	access     *Access
	pagination *Pagination
}

````

## 第 20 步：NewEditableUnits

插入位置：第 19 步代码之后，紧接追加；完成稿约第 378 行。

本步声明：`func NewEditableUnits(access *Access, pagination *Pagination) *EditableUnits`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>access</code> | <code>*Access</code> | 使用 <code>Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。<br>在此声明中的实际使用：<code>access: access</code> |
| <code>pagination</code> | <code>*Pagination</code> | 使用 <code>Pagination</code> 类型；其定义处的职责：生成并校验绑定查询上下文的签名翻页游标。<br>在此声明中的实际使用：<code>pagination: pagination</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*EditableUnits</code> | 实际返回表达式：<code>&amp;EditableUnits{access: access, pagination: pagination}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;EditableUnits{access: access, pagination: pagination}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 20 bytes=16290:16434 -->
````go
func NewEditableUnits(access *Access, pagination *Pagination) *EditableUnits {
	return &EditableUnits{access: access, pagination: pagination}
}
````

## 第 21 步：EditableUnits.Prepare

插入位置：第 20 步代码之后，紧接追加；完成稿约第 381 行。

本步声明：`func (s *EditableUnits) Prepare(ctx context.Context, actor Actor, id uint64, limitText, cursorText string) ([]*pb.SourceEpoch, ReadBody, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, id, &quot;EDITABLE_UNITS&quot;)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, id, &quot;EDITABLE_UNITS&quot;)</code>；<code>actor.ID</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, id, &quot;EDITABLE_UNITS&quot;)</code>；<code>domain.TextID(id)</code>；<code>repository.VisitEditableUnits(ctx, tx, access.Document.CurrentStructureId, id, last, func(fact domain.EditableUnitFact) bool { 			unitID, _ := domain.CanonicalID(fact.ID) 			if !ac …</code>（长表达式，按本步完整代码继续） |
| <code>limitText</code> | <code>string</code> | 在此声明中的实际使用：<code>s.pagination.Parse(limitText, cursorText, binding)</code> |
| <code>cursorText</code> | <code>string</code> | 在此声明中的实际使用：<code>s.pagination.Parse(limitText, cursorText, binding)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 实际返回表达式：<code>nil</code>；<code>access.Sources</code> |
| <code>(匿名/未命名)</code> | <code>ReadBody</code> | 使用 <code>ReadBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>last == &quot;&quot;</code>。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>!access.Permissions.Edit&#91;unitID&#93;</code>。条件成立时执行 <code>return true</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>fact.Lifecycle != nil &amp;&amp; *fact.Lifecycle == &quot;ACTIVE&quot;</code>。
- 在本段的匿名函数内：检查 <code>len(page.Items) &gt; limit</code>。
- 在本段的匿名函数内：此处通过 <code>return false</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：此处通过 <code>return true</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：此处通过 <code>return page, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return access.Sources, run, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 21 bytes=16434:18156 -->
````go
func (s *EditableUnits) Prepare(ctx context.Context, actor Actor, id uint64, limitText, cursorText string) ([]*pb.SourceEpoch, ReadBody, error) {
	access, err := s.access.Authorize(ctx, actor, id, "EDITABLE_UNITS")
	if err != nil {
		return nil, nil, err
	}
	// 游标绑定当前文档ACL、结构和库代次；翻页不能沿用已失效的可编辑范围。
	binding := "editable:" + domain.TextID(actor.ID) + ":" + domain.TextID(id) + ":" + access.Document.AclRevision + ":" + access.Document.CurrentStructureId + ":" + domain.TextID(access.KBRevision)
	limit, last, err := s.pagination.Parse(limitText, cursorText, binding)
	if err != nil {
		return nil, nil, err
	}
	if last == "" {
		last = "0"
	}
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		if _, err := ValidateAccess(ctx, tx, access); err != nil {
			return nil, err
		}
		page := dto.EditableUnitPage{Items: []dto.EditableUnit{}}
		err := repository.VisitEditableUnits(ctx, tx, access.Document.CurrentStructureId, id, last, func(fact domain.EditableUnitFact) bool {
			unitID, _ := domain.CanonicalID(fact.ID)
			if !access.Permissions.Edit[unitID] {
				return true
			}
			unit := dto.EditableUnit{UnitId: fact.ID, Path: fact.Title, BranchId: fact.Branch, WorkspaceEpoch: fact.Epoch, LineageId: fact.Lineage, State: "PREPARING"}
			if fact.Lifecycle != nil && *fact.Lifecycle == "ACTIVE" {
				unit.State = "READY"
			}
			page.Items = append(page.Items, unit)
			if len(page.Items) > limit {
				page.HasMore = true
				page.Items = page.Items[:limit]
				page.NextCursor = s.pagination.Next(binding, page.Items[limit-1].UnitId)
				return false
			}
			return true
		})
		return page, err
	}
	return access.Sources, run, nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/logic/document/editing_service.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/02-workspace-content.md)。
