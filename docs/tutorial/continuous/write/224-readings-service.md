# 分步编写 `services/document/internal/logic/document/readings_service.go`

先按 [D02 · 创建工作区与可靠正文阅读](../lessons/document/02-workspace-content.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/224-readings-service.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

协调文档详情、MAIN全文、变化摘要和固定视图单元读取。

先固定清单与来源，再按视图读取单元，防止浏览器多次请求拼成不同版本。

写之前先按这条执行路线理解各段的连接：

1. Main通过MainReadViews取得可靠捕获
2. readingFixedManifest组装结构样式清单
3. Unit读取LockedReadView
4. Transactions.Read锁来源、插准入并登记输出后提交。

本文件需要守住的边界：

- Revision摘要不是授权凭据
- 敏感正文不能在事务提交及输出登记之前发送。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/logic/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/logic/document/readings_service.go') -and (Get-Item -LiteralPath 'services/document/internal/logic/document/readings_service.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/logic/document/readings_service.go')) { New-Item -ItemType File -Path 'services/document/internal/logic/document/readings_service.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:576 -->
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
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"time"
)

// manifest_read.go：保留原实现，按业务合并文件。

````

## 第 2 步：Readings.Manifest

插入位置：第 1 步代码之后，紧接追加；完成稿约第 24 行。

本步声明：`func (s *Readings) Manifest(ctx context.Context, actor Actor, document, manifest uint64) ([]*pb.SourceEpoch, ReadBody, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, document, &quot;READ_MANIFEST&quot;)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, document, &quot;READ_MANIFEST&quot;)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, document, &quot;READ_MANIFEST&quot;)</code>；<code>repository.ManifestRevision(ctx, tx, document, manifest)</code>；<code>repository.ManifestUnits(ctx, tx, domain.TextID(manifest), document)</code>；<code>historicalContentRangeCovered(ctx, tx, access, document, id, s.validator)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>manifest</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.ManifestRevision(ctx, tx, document, manifest)</code>；<code>domain.TextID(manifest)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 实际返回表达式：<code>nil</code>；<code>access.Sources</code> |
| <code>(匿名/未命名)</code> | <code>ReadBody</code> | 使用 <code>ReadBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>errors.Is(err, sql.ErrNoRows)</code>。条件成立时执行 <code>return nil, domain.Violation(domain.EntityNotFound, &quot;MANIFEST_NOT_FOUND&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>!access.Permissions.Manage &amp;&amp; !access.Permissions.Review &amp;&amp; !access.Permissions.Comment</code>。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>len(units) == 0</code>。条件成立时执行 <code>return nil, domain.Violation(domain.PermissionDenied, &quot;MANIFEST_SCOPE_NOT_ALLOWED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>!allowed</code>。条件成立时执行 <code>return nil, domain.Violation(domain.PermissionDenied, &quot;MANIFEST_SCOPE_NOT_ALLOWED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return readingFixedManifest(ctx, tx, current, document, domain.TextID(manifest), structure, style, admission)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=576:2459 -->
````go
func (s *Readings) Manifest(ctx context.Context, actor Actor, document, manifest uint64) ([]*pb.SourceEpoch, ReadBody, error) {
	access, err := s.access.Authorize(ctx, actor, document, "READ_MANIFEST")
	if err != nil {
		return nil, nil, err
	}
	run := func(ctx context.Context, tx *sql.Tx, admission string) (any, error) {
		current, err := ValidateAccess(ctx, tx, access)
		if err != nil {
			return nil, err
		}
		structure, style, err := repository.ManifestRevision(ctx, tx, document, manifest)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.Violation(domain.EntityNotFound, "MANIFEST_NOT_FOUND")
		}
		if err != nil {
			return nil, err
		}
		// 全文审核/批注可读已成为MAIN的共享历史；局部EDIT必须证明每个
		// 原单元的完整范围，不能凭一个当前子节读出拆分前整章或其它章节。
		if !access.Permissions.Manage && !access.Permissions.Review && !access.Permissions.Comment {
			units, err := repository.ManifestUnits(ctx, tx, domain.TextID(manifest), document)
			if err != nil {
				return nil, err
			}
			if len(units) == 0 {
				return nil, domain.Violation(domain.PermissionDenied, "MANIFEST_SCOPE_NOT_ALLOWED")
			}
			for _, unit := range units {
				id, err := domain.CanonicalID(unit["unit_id"])
				if err != nil {
					return nil, err
				}
				allowed, err := historicalContentRangeCovered(ctx, tx, access, document, id, s.validator)
				if err != nil {
					return nil, err
				}
				if !allowed {
					return nil, domain.Violation(domain.PermissionDenied, "MANIFEST_SCOPE_NOT_ALLOWED")
				}
			}
		}
		// 仅固定原清单，不Capture当前Yjs、不推进业务指针。Document元数据
		// 仍是当前事实，MainView独立指向原结构/样式，并绑定最终输出准入。
		return readingFixedManifest(ctx, tx, current, document, domain.TextID(manifest), structure, style, admission)
	}
````

## 第 3 步：Readings.Manifest（接着实现）

插入位置：第 2 步代码之后，紧接追加；完成稿约第 69 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return access.Sources, run, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=2459:2552 -->
````go
	return access.Sources, run, nil
}

// read.go：保留原实现，按业务合并文件。

````

## 第 4 步：Readings

插入位置：第 3 步代码之后，紧接追加；完成稿约第 74 行。

本步声明：`type Readings struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>access</code> | <code>*Access</code> | 使用 <code>Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。 |
| <code>mainViews</code> | <code>*MainReadViews</code> | 使用 <code>MainReadViews</code> 类型；其定义处的职责：从真实MAIN可靠尾部捕获整篇内容，物化成可安全输出的固定阅读快照。 |
| <code>validator</code> | <code>*codec.Validator</code> | 冻结协议的结构校验器，先限制请求形状，再由业务核验身份、权限、版本和事务条件。 |

<!-- write-step: 4 bytes=2552:2651 -->
````go
type Readings struct {
	access    *Access
	mainViews *MainReadViews
	validator *codec.Validator
}

````

## 第 5 步：NewReadings

插入位置：第 4 步代码之后，紧接追加；完成稿约第 80 行。

本步声明：`func NewReadings(access *Access, mainViews *MainReadViews, validator *codec.Validator) *Readings`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>access</code> | <code>*Access</code> | 使用 <code>Access</code> 类型；其定义处的职责：把文档、空间、知识库和协作者授权事实汇总为文档能力，并收集对应来源代次；事务外准入用于准备，最终写入仍需事务内重核。<br>在此声明中的实际使用：<code>access: access</code> |
| <code>mainViews</code> | <code>*MainReadViews</code> | 使用 <code>MainReadViews</code> 类型；其定义处的职责：从真实MAIN可靠尾部捕获整篇内容，物化成可安全输出的固定阅读快照。<br>在此声明中的实际使用：<code>mainViews: mainViews</code> |
| <code>validator</code> | <code>*codec.Validator</code> | 冻结协议的结构校验器，先限制请求形状，再由业务核验身份、权限、版本和事务条件。<br>在此声明中的实际使用：<code>validator: validator</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*Readings</code> | 实际返回表达式：<code>&amp;Readings{access: access, mainViews: mainViews, validator: validator}</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &amp;Readings{access: access, mainViews: mainViews, validator: validator}</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=2651:2831 -->
````go
func NewReadings(access *Access, mainViews *MainReadViews, validator *codec.Validator) *Readings {
	return &Readings{access: access, mainViews: mainViews, validator: validator}
}

````

## 第 6 步：DocumentReading

插入位置：第 5 步代码之后，紧接追加；完成稿约第 84 行。

DocumentReading保存业务动作摘要；HTTP头的名称和编码由传输适配器处理。

本步声明：`type DocumentReading struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Document</code> | <code>dto.Document</code> | 阅读入口返回的当前文档元数据。 |
| <code>Actions</code> | <code>&#91;&#93;string</code> | 服务器按真实权限计算的可用操作名称，前端据此展示入口，接口仍须自行鉴权。 |

<!-- write-step: 6 bytes=2831:3004 -->
````go
// DocumentReading保存业务动作摘要；HTTP头的名称和编码由传输适配器处理。
type DocumentReading struct {
	Document dto.Document
	Actions  []string
}

````

## 第 7 步：Readings.Document

插入位置：第 6 步代码之后，紧接追加；完成稿约第 90 行。

本步声明：`func (s *Readings) Document(ctx context.Context, actor Actor, id uint64) ([]*pb.SourceEpoch, ReadBody, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, id, &quot;READ_DOCUMENT&quot;)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, id, &quot;READ_DOCUMENT&quot;)</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, id, &quot;READ_DOCUMENT&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 实际返回表达式：<code>nil</code>；<code>access.Sources</code> |
| <code>(匿名/未命名)</code> | <code>ReadBody</code> | 使用 <code>ReadBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>access.Permissions.Comment</code>。
- 在本段的匿名函数内：检查 <code>access.Permissions.Manage</code>。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>space.Kind == &quot;ORGANIZATION&quot;</code>。
- 在本段的匿名函数内：检查 <code>len(access.Permissions.Edit) &gt; 0</code>。
- 在本段的匿名函数内：此处通过 <code>return DocumentReading{Document: document, Actions: actions}, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return access.Sources, run, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=3004:4239 -->
````go
func (s *Readings) Document(ctx context.Context, actor Actor, id uint64) ([]*pb.SourceEpoch, ReadBody, error) {
	access, err := s.access.Authorize(ctx, actor, id, "READ_DOCUMENT")
	if err != nil {
		return nil, nil, err
	}
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		document, err := ValidateAccess(ctx, tx, access)
		if err != nil {
			return nil, err
		}
		actions := []string{"READ_MAIN"}
		// 前端只展示真实全文批注资格；单元 EDIT 不能隐含 COMMENT。
		if access.Permissions.Comment {
			actions = append(actions, "COMMENT")
		}
		if access.Permissions.Manage {
			actions = append(actions, "RENAME", "DELETE", "CHANGE_VISIBILITY", "CHANGE_AUTHORIZATION")
			spaceID, _ := domain.CanonicalID(document.SpaceId)
			space, err := repository.SpaceRow(ctx, tx, spaceID, false)
			if err != nil {
				return nil, err
			}
			if space.Kind == "ORGANIZATION" {
				actions = append(actions, "TRANSFER_RESPONSIBLE")
			} else {
				actions = append(actions, "MANAGE_INVITATIONS")
			}
		}
		if len(access.Permissions.Edit) > 0 {
			actions = append(actions, "VIEW_EDITABLE_UNITS")
		}
		return DocumentReading{Document: document, Actions: actions}, nil
	}
	return access.Sources, run, nil
}
````

## 第 8 步：Readings.Main

插入位置：第 7 步代码之后，紧接追加；完成稿约第 125 行。

本步声明：`func (s *Readings) Main(ctx context.Context, actor Actor, id uint64) ([]*pb.SourceEpoch, ReadBody, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, id, &quot;READ_MAIN&quot;)</code>；<code>s.mainViews.Capture(ctx, actor, access, id)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, id, &quot;READ_MAIN&quot;)</code>；<code>s.mainViews.Capture(ctx, actor, access, id)</code>；<code>s.mainViews.Publish(ctx, tx, actor, captured, admission)</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, id, &quot;READ_MAIN&quot;)</code>；<code>s.mainViews.Capture(ctx, actor, access, id)</code>；<code>readingMainView(ctx, tx, document, id, document.CurrentManifestId, admission)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 实际返回表达式：<code>nil</code>；<code>access.Sources</code> |
| <code>(匿名/未命名)</code> | <code>ReadBody</code> | 使用 <code>ReadBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>!access.Document.MainLocked</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>captured != nil</code>。条件成立时执行 <code>return s.mainViews.Publish(ctx, tx, actor, captured, admission)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>!document.MainLocked</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;MAIN_SOURCE_CHANGED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return readingMainView(ctx, tx, document, id, document.CurrentManifestId, admission)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return access.Sources, run, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=4239:5163 -->
````go
func (s *Readings) Main(ctx context.Context, actor Actor, id uint64) ([]*pb.SourceEpoch, ReadBody, error) {
	access, err := s.access.Authorize(ctx, actor, id, "READ_MAIN")
	if err != nil {
		return nil, nil, err
	}
	var captured *mainReadCapture
	if !access.Document.MainLocked {
		captured, err = s.mainViews.Capture(ctx, actor, access, id)
		if err != nil {
			return nil, nil, err
		}
		access = captured.Fixed.Access
	}
	run := func(ctx context.Context, tx *sql.Tx, admission string) (any, error) {
		if captured != nil {
			return s.mainViews.Publish(ctx, tx, actor, captured, admission)
		}
		document, err := ValidateAccess(ctx, tx, access)
		if err != nil {
			return nil, err
		}
		if !document.MainLocked {
			return nil, domain.Violation(domain.StateConflict, "MAIN_SOURCE_CHANGED")
		}
		return readingMainView(ctx, tx, document, id, document.CurrentManifestId, admission)
	}
	return access.Sources, run, nil
}

````

## 第 9 步：Readings.Revision

插入位置：第 8 步代码之后，紧接追加；完成稿约第 154 行。

Revision复用全文阅读资格及最终输出登记；变化摘要不成为浏览器的授权凭据。
Node调用在Doc事务外完成，避免跨服务等待占用来源/文档锁。

本步声明：`func (s *Readings) Revision(ctx context.Context, actor Actor, id uint64) ([]*pb.SourceEpoch, ReadBody, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, id, &quot;READ_MAIN&quot;)</code>；<code>s.mainViews.client.MainRevision(ctx, id)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, id, &quot;READ_MAIN&quot;)</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, id, &quot;READ_MAIN&quot;)</code>；<code>s.mainViews.client.MainRevision(ctx, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 实际返回表达式：<code>nil</code>；<code>access.Sources</code> |
| <code>(匿名/未命名)</code> | <code>ReadBody</code> | 使用 <code>ReadBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 检查 <code>!access.Document.MainLocked</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>current.MainLocked != original.MainLocked &#124;&#124; current.ControlRevision != original.ControlRevision &#124;&#124; current.CurrentManifestId != original.CurrentManifestId &#124;&#124; current.CurrentStyleId != original.CurrentStyleId</code>。条件成立时执行 <code>return nil, domain.Violation(domain.StateConflict, &quot;MAIN_SOURCE_CHANGED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return sources&#91;i&#93;.ScopeKey &lt; sources&#91;j&#93;.ScopeKey</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return dto.MainReadingRevision{Revision: hex.EncodeToString(digest&#91;:&#93;)}, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return access.Sources, run, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=5163:6962 -->
````go
// Revision复用全文阅读资格及最终输出登记；变化摘要不成为浏览器的授权凭据。
// Node调用在Doc事务外完成，避免跨服务等待占用来源/文档锁。
func (s *Readings) Revision(ctx context.Context, actor Actor, id uint64) ([]*pb.SourceEpoch, ReadBody, error) {
	access, err := s.access.Authorize(ctx, actor, id, "READ_MAIN")
	if err != nil {
		return nil, nil, err
	}
	tail := ""
	if !access.Document.MainLocked {
		tail, err = s.mainViews.client.MainRevision(ctx, id)
		if err != nil {
			return nil, nil, err
		}
	}
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		current, err := ValidateAccess(ctx, tx, access)
		if err != nil {
			return nil, err
		}
		original := access.Document
		// 跨服务采样期间主线代次发生变化就重试，不能把旧尾部套到新主线。
		if current.MainLocked != original.MainLocked || current.ControlRevision != original.ControlRevision || current.CurrentManifestId != original.CurrentManifestId || current.CurrentStyleId != original.CurrentStyleId {
			return nil, domain.Violation(domain.StateConflict, "MAIN_SOURCE_CHANGED")
		}
		// 身份证明中的来源是集合，返回顺序不代表权限发生变化。
		// 仅为摘要排序副本，保留原准入及输出所绑定的来源。
		sources := append([]*pb.SourceEpoch(nil), access.Sources...)
		sort.Slice(sources, func(i, j int) bool { return sources[i].ScopeKey < sources[j].ScopeKey })
		raw, err := json.Marshal(struct {
			Document dto.Document
			Tail     string
			Sources  []*pb.SourceEpoch
		}{current, tail, sources})
		if err != nil {
			return nil, err
		}
		digest := sha256.Sum256(raw)
		return dto.MainReadingRevision{Revision: hex.EncodeToString(digest[:])}, nil
	}
	return access.Sources, run, nil
}

````

## 第 10 步：validateReadingUnits

插入位置：第 9 步代码之后，紧接追加；完成稿约第 196 行。

本步声明：`func validateReadingUnits(structure dto.StructureRevision, units []map[string]string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>structure</code> | <code>dto.StructureRevision</code> | 在此声明中的实际使用：<code>structure.Entries</code> |
| <code>units</code> | <code>&#91;&#93;map&#91;string&#93;string</code> | 在此声明中的实际使用：<code>len(units)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;MANIFEST_INCOMPLETE&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>entry.UnitId != nil</code>。
- 检查 <code>len(expected) != len(units) &#124;&#124; (len(expected) == 0 &amp;&amp; len(structure.Entries) != 0)</code>。条件成立时执行 <code>return errors.New(&quot;MANIFEST_INCOMPLETE&quot;)</code>。当前函数在这里结束。
- 检查 <code>!expected&#91;unit&#91;&quot;unit_id&quot;&#93;&#93;</code>。条件成立时执行 <code>return errors.New(&quot;MANIFEST_INCOMPLETE&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=6962:7503 -->
````go
func validateReadingUnits(structure dto.StructureRevision, units []map[string]string) error {
	expected := make(map[string]bool)
	for _, entry := range structure.Entries {
		if entry.UnitId != nil {
			expected[*entry.UnitId] = true
		}
	}
	if len(expected) != len(units) || (len(expected) == 0 && len(structure.Entries) != 0) {
		return errors.New("MANIFEST_INCOMPLETE")
	}
	for _, unit := range units {
		if !expected[unit["unit_id"]] {
			return errors.New("MANIFEST_INCOMPLETE")
		}
		delete(expected, unit["unit_id"])
	}
	return nil
}

````

## 第 11 步：readingMainView

插入位置：第 10 步代码之后，紧接追加；完成稿约第 215 行。

本步声明：`func readingMainView(ctx context.Context, tx *sql.Tx, document dto.Document, id uint64, manifest, admission string) (dto.MainView, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>readingFixedManifest(ctx, tx, document, id, manifest, document.CurrentStructureId, document.CurrentStyleId, admission)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>readingFixedManifest(ctx, tx, document, id, manifest, document.CurrentStructureId, document.CurrentStyleId, admission)</code> |
| <code>document</code> | <code>dto.Document</code> | 在此声明中的实际使用：<code>readingFixedManifest(ctx, tx, document, id, manifest, document.CurrentStructureId, document.CurrentStyleId, admission)</code>；<code>document.CurrentStructureId</code>；<code>document.CurrentStyleId</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>readingFixedManifest(ctx, tx, document, id, manifest, document.CurrentStructureId, document.CurrentStyleId, admission)</code> |
| <code>manifest</code> | <code>string</code> | 在此声明中的实际使用：<code>readingFixedManifest(ctx, tx, document, id, manifest, document.CurrentStructureId, document.CurrentStyleId, admission)</code> |
| <code>admission</code> | <code>string</code> | 在此声明中的实际使用：<code>readingFixedManifest(ctx, tx, document, id, manifest, document.CurrentStructureId, document.CurrentStyleId, admission)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>dto.MainView</code> | 由 <code>readingFixedManifest(ctx, tx, document, id, manifest, document.CurrentStructureId, document.CurrentStyleId, admission)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>readingFixedManifest(ctx, tx, document, id, manifest, document.CurrentStructureId, document.CurrentStyleId, admission)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return readingFixedManifest(ctx, tx, document, id, manifest, document.CurrentStructureId, document.CurrentStyleId, admission)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=7503:7773 -->
````go
func readingMainView(ctx context.Context, tx *sql.Tx, document dto.Document, id uint64, manifest, admission string) (dto.MainView, error) {
	return readingFixedManifest(ctx, tx, document, id, manifest, document.CurrentStructureId, document.CurrentStyleId, admission)
}

````

## 第 12 步：readingFixedManifest

插入位置：第 11 步代码之后，紧接追加；完成稿约第 219 行。

本步声明：`func readingFixedManifest(ctx context.Context, tx *sql.Tx, document dto.Document, id uint64, manifest, structureID, styleID, admission string) (dto.MainView, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.StructureRow(ctx, tx, structureID, document.Id)</code>；<code>repository.StyleRow(ctx, tx, styleID, document.Id)</code>；<code>repository.ManifestUnits(ctx, tx, manifest, id)</code>；<code>repository.LockedAdmission(ctx, tx, admission)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.StructureRow(ctx, tx, structureID, document.Id)</code>；<code>repository.StyleRow(ctx, tx, styleID, document.Id)</code>；<code>repository.ManifestUnits(ctx, tx, manifest, id)</code>；<code>repository.LockedAdmission(ctx, tx, admission)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>document</code> | <code>dto.Document</code> | 在此声明中的实际使用：<code>document.Id</code>；<code>Document: document</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.ManifestUnits(ctx, tx, manifest, id)</code>；<code>Document: id</code> |
| <code>manifest</code> | <code>string</code> | 在此声明中的实际使用：<code>repository.ManifestUnits(ctx, tx, manifest, id)</code>；<code>Manifest: manifest</code>；<code>ManifestId: manifest</code> |
| <code>structureID</code> | <code>string</code> | 在此声明中的实际使用：<code>repository.StructureRow(ctx, tx, structureID, document.Id)</code>；<code>StructureId: structureID</code> |
| <code>styleID</code> | <code>string</code> | 在此声明中的实际使用：<code>repository.StyleRow(ctx, tx, styleID, document.Id)</code>；<code>StyleId: styleID</code> |
| <code>admission</code> | <code>string</code> | 在此声明中的实际使用：<code>Admission: admission</code>；<code>repository.LockedAdmission(ctx, tx, admission)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>dto.MainView</code> | 实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>domain.Violation(domain.PermissionDenied, &quot;READ_VIEW_NOT_CURRENT&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return value, err</code>。当前函数在这里结束。
- 检查 <code>original.State != &quot;OPEN&quot; &#124;&#124; !original.Expires.After(time.Now())</code>。条件成立时执行 <code>return value, domain.Violation(domain.PermissionDenied, &quot;READ_VIEW_NOT_CURRENT&quot;)</code>。当前函数在这里结束。
- 检查 <code>original.Expires.Before(view.Expires)</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return value, err</code>。当前函数在这里结束。
- 此处通过 <code>return value, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=7773:9807 -->
````go
func readingFixedManifest(ctx context.Context, tx *sql.Tx, document dto.Document, id uint64, manifest, structureID, styleID, admission string) (dto.MainView, error) {
	var value dto.MainView
	structure, err := repository.StructureRow(ctx, tx, structureID, document.Id)
	if err != nil {
		return value, err
	}
	style, err := repository.StyleRow(ctx, tx, styleID, document.Id)
	if err != nil {
		return value, err
	}
	units, err := repository.ManifestUnits(ctx, tx, manifest, id)
	if err != nil {
		return value, err
	}
	if err = validateReadingUnits(structure, units); err != nil {
		return value, err
	}
	// 固定read view绑定最终输出的真实准入，当前业务指针与阅读清单分别表达各自事实。
	view := domain.ReadView{ID: platform.NewTraceID(), Document: id, Manifest: manifest, Admission: admission, Expires: time.Now().UTC().Add(30 * time.Second)}
	original, err := repository.LockedAdmission(ctx, tx, admission)
	if err != nil {
		return value, err
	}
	if original.State != "OPEN" || !original.Expires.After(time.Now()) {
		return value, domain.Violation(domain.PermissionDenied, "READ_VIEW_NOT_CURRENT")
	}
	// 对外声明的期限不能晚于原证明的实际期限；最长仍是既定30秒。
	if original.Expires.Before(view.Expires) {
		view.Expires = original.Expires
	}
	if err = repository.InsertReadView(ctx, tx, view); err != nil {
		return value, err
	}
	value = dto.MainView{ReadViewId: view.ID, ExpiresAt: view.Expires.Format(time.RFC3339Nano), Document: document, ManifestId: manifest, StructureId: structureID, StyleId: styleID, Structure: structure, Style: style}
	value.Units = make([]struct {
		UnitId           string `json:"unit_id"`
		ContentVersionId string `json:"content_version_id"`
	}, 0, len(units))
	for _, unit := range units {
		value.Units = append(value.Units, struct {
			UnitId           string `json:"unit_id"`
			ContentVersionId string `json:"content_version_id"`
		}{UnitId: unit["unit_id"], ContentVersionId: unit["content_version_id"]})
	}
	return value, nil
}
````

## 第 13 步：Readings.Unit

插入位置：第 12 步代码之后，紧接追加；完成稿约第 265 行。

本步声明：`func (s *Readings) Unit(ctx context.Context, actor Actor, id, unit uint64, view string) ([]*pb.SourceEpoch, ReadBody, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, id, &quot;READ_FIXED_UNIT&quot;)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, id, &quot;READ_FIXED_UNIT&quot;)</code>；<code>actor.ID</code>；<code>actor.Proof</code> |
| <code>id</code> | <code>uint64</code> | 在此声明中的实际使用：<code>s.access.Authorize(ctx, actor, id, &quot;READ_FIXED_UNIT&quot;)</code>；<code>repository.LockedReadView(ctx, tx, view, id)</code>；<code>repository.ReadViewVersion(ctx, tx, fixed.Manifest, id, unit)</code>；<code>domain.TextID(id)</code> |
| <code>unit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>repository.ReadViewVersion(ctx, tx, fixed.Manifest, id, unit)</code> |
| <code>view</code> | <code>string</code> | 在此声明中的实际使用：<code>repository.LockedReadView(ctx, tx, view, id)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 实际返回表达式：<code>nil</code>；<code>access.Sources</code> |
| <code>(匿名/未命名)</code> | <code>ReadBody</code> | 使用 <code>ReadBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>实际返回表达式：<code>nil</code>；<code>run</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil &#124;&#124; fixed.Actor != actor.ID &#124;&#124; !bytes.Equal(fixed.Session, actor.Proof.SessionRef) &#124;&#124; fixed.State != &quot;OPEN&quot; &#124;&#124; fixed.AdmissionState != &quot;OPEN&quot; &#124;&#124; !fixed.Expires.After(time.Now())</code>。条件成立时执行 <code>return nil, domain.Violation(domain.PermissionDenied, &quot;READ_VIEW_NOT_CURRENT&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return nil, domain.Violation(domain.EntityNotFound, &quot;UNIT_NOT_IN_READ_VIEW&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return repository.ContentRow(ctx, tx, version, domain.TextID(id))</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return access.Sources, run, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=9807:10902 -->
````go
func (s *Readings) Unit(ctx context.Context, actor Actor, id, unit uint64, view string) ([]*pb.SourceEpoch, ReadBody, error) {
	access, err := s.access.Authorize(ctx, actor, id, "READ_FIXED_UNIT")
	if err != nil {
		return nil, nil, err
	}
	run := func(ctx context.Context, tx *sql.Tx, _ string) (any, error) {
		if _, err := ValidateAccess(ctx, tx, access); err != nil {
			return nil, err
		}
		fixed, err := repository.LockedReadView(ctx, tx, view, id)
		if err != nil || fixed.Actor != actor.ID || !bytes.Equal(fixed.Session, actor.Proof.SessionRef) || fixed.State != "OPEN" || fixed.AdmissionState != "OPEN" || !fixed.Expires.After(time.Now()) {
			return nil, domain.Violation(domain.PermissionDenied, "READ_VIEW_NOT_CURRENT")
		}
		version, err := repository.ReadViewVersion(ctx, tx, fixed.Manifest, id, unit)
		if err != nil {
			return nil, domain.Violation(domain.EntityNotFound, "UNIT_NOT_IN_READ_VIEW")
		}
		return repository.ContentRow(ctx, tx, version, domain.TextID(id))
	}
	return access.Sources, run, nil
}

// read_transaction.go：保留原实现，按业务合并文件。

````

## 第 14 步：Transactions.Read

插入位置：第 13 步代码之后，紧接追加；完成稿约第 289 行。

目录和正文都在来源锁下读取并登记输出，不能在提交之前释放敏感结果。

本步声明：`func (s *Transactions) Read(ctx context.Context, actor Actor, sources []*pb.SourceEpoch, document uint64, purpose, handler string, run ReadBody, bind BindOutput) (any, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>s.identity.Revalidate(ctx, actor.Proof)</code>；<code>s.outputs.EdgeInstance(ctx, handler)</code>；<code>s.db.BeginTx(ctx, nil)</code>；<code>LockSources(ctx, tx, sources)</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor.Proof</code>；<code>InsertAdmission(ctx, tx, actor, sources, document, purpose, admission)</code> |
| <code>sources</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 在此声明中的实际使用：<code>LockSources(ctx, tx, sources)</code>；<code>InsertAdmission(ctx, tx, actor, sources, document, purpose, admission)</code>；<code>s.outputs.Register(ctx, tx, sources, document, admission, handler, edge)</code>；<code>bind(registration, sources)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>document != 0</code>；<code>InsertAdmission(ctx, tx, actor, sources, document, purpose, admission)</code>；<code>s.outputs.Register(ctx, tx, sources, document, admission, handler, edge)</code> |
| <code>purpose</code> | <code>string</code> | 在此声明中的实际使用：<code>InsertAdmission(ctx, tx, actor, sources, document, purpose, admission)</code> |
| <code>handler</code> | <code>string</code> | 在此声明中的实际使用：<code>s.outputs.EdgeInstance(ctx, handler)</code>；<code>s.outputs.Register(ctx, tx, sources, document, admission, handler, edge)</code> |
| <code>run</code> | <code>ReadBody</code> | 使用 <code>ReadBody</code> 类型；其定义处的职责：统一业务变更幂等回执、敏感结果重投影及提交前输出绑定。<br>在此声明中的实际使用：<code>run(ctx, tx, admission)</code> |
| <code>bind</code> | <code>BindOutput</code> | 使用 <code>BindOutput</code> 类型；其定义处的职责：BindOutput把固定租约绑定到真实handler，必须在COMMIT前完成；业务层不持有ResponseWriter。<br>在此声明中的实际使用：<code>bind(registration, sources)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>any</code> | 实际返回表达式：<code>nil</code>；<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 执行 <code>s.db.BeginTx(ctx, nil)</code>。从这里建立事务，后续受保护读写须沿用返回的事务对象。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 登记 <code>tx.Rollback()</code>，当前函数退出时执行，提前 return 也会触发。
- 执行 <code>tx.Rollback()</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>document != 0</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。
- 执行 <code>tx.Commit()</code>。提交结果需要检查；收到错误不一定证明数据库没有提交。
- 此处通过 <code>return nil, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return value, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=10902:12068 -->
````go
// 目录和正文都在来源锁下读取并登记输出，不能在提交之前释放敏感结果。
func (s *Transactions) Read(ctx context.Context, actor Actor, sources []*pb.SourceEpoch, document uint64, purpose, handler string, run ReadBody, bind BindOutput) (any, error) {
	if err := s.identity.Revalidate(ctx, actor.Proof); err != nil {
		return nil, err
	}
	edge, err := s.outputs.EdgeInstance(ctx, handler)
	if err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err := LockSources(ctx, tx, sources); err != nil {
		return nil, err
	}
	admission := ""
	if document != 0 {
		admission = platform.NewTraceID()
		if err := InsertAdmission(ctx, tx, actor, sources, document, purpose, admission); err != nil {
			return nil, err
		}
	}
	value, err := run(ctx, tx, admission)
	if err != nil {
		return nil, err
	}
	registration, err := s.outputs.Register(ctx, tx, sources, document, admission, handler, edge)
	if err != nil {
		return nil, err
	}
	output := bind(registration, sources)
	if err := tx.Commit(); err != nil {
		output.Abort()
		return nil, err
	}
	return value, nil
}
````

## 第 15 步：InsertAdmission

插入位置：第 14 步代码之后，紧接追加；完成稿约第 328 行。

本步声明：`func InsertAdmission(ctx context.Context, tx *sql.Tx, actor Actor, sources []*pb.SourceEpoch, document uint64, purpose, admission string) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.InsertReadAdmission(ctx, tx, record)</code>；<code>repository.InsertAdmissionSource(ctx, tx, admission, source.ScopeKey, source.Epoch)</code> |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.InsertReadAdmission(ctx, tx, record)</code>；<code>repository.InsertAdmissionSource(ctx, tx, admission, source.ScopeKey, source.Epoch)</code> |
| <code>actor</code> | <code>Actor</code> | 使用 <code>Actor</code> 类型；其定义处的职责：Actor保存已核验身份与跨服务证明；请求对象不进入用例。<br>在此声明中的实际使用：<code>actor.Proof</code>；<code>actor.ID</code> |
| <code>sources</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 此次文档准入依赖的权限来源与代次，持久写入后供输出和恢复核验。 |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>DocumentID: document</code> |
| <code>purpose</code> | <code>string</code> | 在此声明中的实际使用：<code>json.Marshal(&#91;&#93;string{purpose})</code>；<code>Purpose: purpose</code> |
| <code>admission</code> | <code>string</code> | 在此声明中的实际使用：<code>ID: admission</code>；<code>repository.InsertAdmissionSource(ctx, tx, admission, source.ScopeKey, source.Epoch)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=12068:12902 -->
````go
func InsertAdmission(ctx context.Context, tx *sql.Tx, actor Actor, sources []*pb.SourceEpoch, document uint64, purpose, admission string) error {
	target, _ := json.Marshal(actor.Proof.Target)
	capabilities, _ := json.Marshal([]string{purpose})
	record := domain.ReadAdmission{ID: admission, IdentityAdmissionRef: actor.Proof.AdmissionId, ActorID: actor.ID, SessionRef: actor.Proof.SessionRef, DocumentID: document, Purpose: purpose, CapabilitiesJSON: capabilities, TargetJSON: target, OperationID: actor.Proof.OperationId, ExpiresUnixMs: actor.Proof.ExpiresUnixMs}
	if err := repository.InsertReadAdmission(ctx, tx, record); err != nil {
		return err
	}
	for _, source := range sources {
		if err := repository.InsertAdmissionSource(ctx, tx, admission, source.ScopeKey, source.Epoch); err != nil {
			return err
		}
	}
	return nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/logic/document/readings_service.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/02-workspace-content.md)。
