# 分步编写 `services/document/internal/domain/document/account_lifecycle_models.go`

先按 [D13 · 账号注销、组织关闭与公开留存](../lessons/document/13-account-retirement.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/089-account-lifecycle-models.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

定义账号注销固定绑定、公开版依赖留存计划与历史图清理算法。

保留公开版只能保留其真实依赖，旧祖先可留骨架但不应携带旧私有正文。

写之前先按这条执行路线理解各段的连接：

1. AccountRetirementBinding核权威组织集合
2. CleanupDocument从原计划取Doc和关闭scope
3. KeepHistoryAncestors保留必要祖先
4. HistoryDeletionOrder从真实叶节点排序删除。

本文件需要守住的边界：

- Release=0表示整篇清理
- 不能按ID大小猜parent顺序
- 计划不存活跃MAIN或正文副本。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/domain/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/domain/document/account_lifecycle_models.go') -and (Get-Item -LiteralPath 'services/document/internal/domain/document/account_lifecycle_models.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/domain/document/account_lifecycle_models.go')) { New-Item -ItemType File -Path 'services/document/internal/domain/document/account_lifecycle_models.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:315 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	"bytes"
	"collaboration/platform"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"slices"
)

// account_document_cleanup.go：保留原实现，按业务合并文件。

````

## 第 2 步：AccountRetirementPlan.CleanupDocument

插入位置：第 1 步代码之后，紧接追加；完成稿约第 17 行。

CleanupDocument从已保存的不可变计划取文档及其真实关闭来源，不能用调用者参数
拼出缺失的屏障。保留正式版的文档同样必须清掉Node私有工作区。

本步声明：`func (p AccountRetirementPlan) CleanupDocument(document uint64) (AccountRetentionDocument, []AccountRetentionScope, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>cleanupRetentionDocument(p.Retention, document, &quot;ACCOUNT&quot;)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>AccountRetentionDocument</code> | 使用 <code>AccountRetentionDocument</code> 类型；其定义处的职责：保留计划只记录不可变依赖编号，绝不把活跃MAIN或整份私有正文写进回执。 Release为零表示整个文档待清理；非零只保留该正式版自己的依赖。<br>由 <code>cleanupRetentionDocument(p.Retention, document, &quot;ACCOUNT&quot;)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;AccountRetentionScope</code> | 由 <code>cleanupRetentionDocument(p.Retention, document, &quot;ACCOUNT&quot;)</code> 的第 2 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>cleanupRetentionDocument(p.Retention, document, &quot;ACCOUNT&quot;)</code> 的第 3 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return cleanupRetentionDocument(p.Retention, document, &quot;ACCOUNT&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=315:711 -->
````go
// CleanupDocument从已保存的不可变计划取文档及其真实关闭来源，不能用调用者参数
// 拼出缺失的屏障。保留正式版的文档同样必须清掉Node私有工作区。
func (p AccountRetirementPlan) CleanupDocument(document uint64) (AccountRetentionDocument, []AccountRetentionScope, error) {
	return cleanupRetentionDocument(p.Retention, document, "ACCOUNT")
}

````

## 第 3 步：cleanupRetentionDocument

插入位置：第 2 步代码之后，紧接追加；完成稿约第 23 行。

本步声明：`func cleanupRetentionDocument(retention *AccountRetentionPlan, document uint64, kind string) (AccountRetentionDocument, []AccountRetentionScope, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>retention</code> | <code>*AccountRetentionPlan</code> | 在此声明中的实际使用：<code>retention == nil</code>；<code>retention.Documents</code>；<code>retention.Scopes</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>candidate.Document == document</code>；<code>TextID(document)</code> |
| <code>kind</code> | <code>string</code> | 在此声明中的实际使用：<code>kind + &quot;_RETENTION_PLAN_REQUIRED&quot;</code>；<code>kind+&quot;_DOCUMENT_NOT_IN_PLAN&quot;</code>；<code>kind + &quot;_DOCUMENT_SCOPE_MISSING&quot;</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>AccountRetentionDocument</code> | 使用 <code>AccountRetentionDocument</code> 类型；其定义处的职责：保留计划只记录不可变依赖编号，绝不把活跃MAIN或整份私有正文写进回执。 Release为零表示整个文档待清理；非零只保留该正式版自己的依赖。<br>实际返回表达式：<code>item</code> |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;AccountRetentionScope</code> | 实际返回表达式：<code>nil</code>；<code>scopes</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(kind + &quot;_RETENTION_PLAN_REQUIRED&quot;)</code>；<code>Violation(StateConflict, kind+&quot;_DOCUMENT_NOT_IN_PLAN&quot;)</code>；<code>errors.New(kind + &quot;_DOCUMENT_SCOPE_MISSING&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>retention == nil</code>。条件成立时执行 <code>return item, nil, errors.New(kind + &quot;_RETENTION_PLAN_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>candidate.Document == document</code>。
- 检查 <code>!found</code>。条件成立时执行 <code>return item, nil, Violation(StateConflict, kind+&quot;_DOCUMENT_NOT_IN_PLAN&quot;)</code>。当前函数在这里结束。
- 检查 <code>item.KnowledgeBase != 0</code>。
- 检查 <code>keys&#91;scope.Key&#93;</code>。
- 检查 <code>len(keys) != 0</code>。条件成立时执行 <code>return item, nil, errors.New(kind + &quot;_DOCUMENT_SCOPE_MISSING&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return item, scopes, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=711:1786 -->
````go
func cleanupRetentionDocument(retention *AccountRetentionPlan, document uint64, kind string) (AccountRetentionDocument, []AccountRetentionScope, error) {
	var item AccountRetentionDocument
	if retention == nil {
		return item, nil, errors.New(kind + "_RETENTION_PLAN_REQUIRED")
	}
	found := false
	for _, candidate := range retention.Documents {
		if candidate.Document == document {
			item, found = candidate, true
			break
		}
	}
	if !found {
		return item, nil, Violation(StateConflict, kind+"_DOCUMENT_NOT_IN_PLAN")
	}
	keys := map[string]bool{"doc-acl:" + TextID(document): true, "publication:" + TextID(document): true}
	if item.KnowledgeBase != 0 {
		keys["kb-acl:"+TextID(item.KnowledgeBase)] = true
	}
	var scopes []AccountRetentionScope
	for _, scope := range retention.Scopes {
		if keys[scope.Key] {
			scopes = append(scopes, scope)
			delete(keys, scope.Key)
		}
	}
	if len(keys) != 0 {
		return item, nil, errors.New(kind + "_DOCUMENT_SCOPE_MISSING")
	}
	return item, scopes, nil
}

// account_history_cleanup.go：保留原实现，按业务合并文件。

````

## 第 4 步：AccountHistoryRevision

插入位置：第 3 步代码之后，紧接追加；完成稿约第 57 行。

留存依赖树只保存编号；旧manifest/结构的必要祖先可留骨架，不能保留旧正文条目。

本步声明：`type AccountHistoryRevision struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ID</code> | <code>uint64</code> | 不可变历史修订ID |
| <code>Parent</code> | <code>uint64</code> | 原父修订ID，不可为删除便利改写 |
| <code>Structure</code> | <code>uint64</code> | manifest关联结构 |
| <code>Style</code> | <code>uint64</code> | manifest关联样式 |

<!-- write-step: 4 bytes=1786:1978 -->
````go
// 留存依赖树只保存编号；旧manifest/结构的必要祖先可留骨架，不能保留旧正文条目。
type AccountHistoryRevision struct {
	ID, Parent, Structure, Style uint64
}

````

## 第 5 步：KeepHistoryAncestors

插入位置：第 4 步代码之后，紧接追加；完成稿约第 62 行。

本步声明：`func KeepHistoryAncestors(rows []AccountHistoryRevision, keep map[uint64]bool) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>rows</code> | <code>&#91;&#93;AccountHistoryRevision</code> | 使用 <code>AccountHistoryRevision</code> 类型；其定义处的职责：留存依赖树只保存编号；旧manifest/结构的必要祖先可留骨架，不能保留旧正文条目。 |
| <code>keep</code> | <code>map&#91;uint64&#93;bool</code> | 在此声明中的实际使用：<code>seen&#91;id&#93;, keep&#91;id&#93; = true, true</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;ACCOUNT_HISTORY_DEPENDENCY_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>!found &#124;&#124; seen&#91;id&#93;</code>。条件成立时执行 <code>return errors.New(&quot;ACCOUNT_HISTORY_DEPENDENCY_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=1978:2424 -->
````go
func KeepHistoryAncestors(rows []AccountHistoryRevision, keep map[uint64]bool) error {
	byID := map[uint64]AccountHistoryRevision{}
	for _, row := range rows {
		byID[row.ID] = row
	}
	for id := range keep {
		seen := map[uint64]bool{}
		for id != 0 {
			row, found := byID[id]
			if !found || seen[id] {
				return errors.New("ACCOUNT_HISTORY_DEPENDENCY_INVALID")
			}
			seen[id], keep[id] = true, true
			id = row.Parent
		}
	}
	return nil
}

````

## 第 6 步：HistoryDeletionOrder

插入位置：第 5 步代码之后，紧接追加；完成稿约第 81 行。

不改不可变parent_id：从真实叶节点删除，不因ID大小猜历史先后。

本步声明：`func HistoryDeletionOrder(rows []AccountHistoryRevision, keep map[uint64]bool) ([]uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>rows</code> | <code>&#91;&#93;AccountHistoryRevision</code> | 使用 <code>AccountHistoryRevision</code> 类型；其定义处的职责：留存依赖树只保存编号；旧manifest/结构的必要祖先可留骨架，不能保留旧正文条目。 |
| <code>keep</code> | <code>map&#91;uint64&#93;bool</code> | 在此声明中的实际使用：<code>!keep&#91;id&#93;</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;uint64</code> | 实际返回表达式：<code>nil</code>；<code>order</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;ACCOUNT_HISTORY_DEPENDENCY_CYCLE&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>row.Parent != 0</code>。
- 检查 <code>parents&#91;id&#93;</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>!keep&#91;id&#93;</code>。
- 检查 <code>!progress</code>。条件成立时执行 <code>return nil, errors.New(&quot;ACCOUNT_HISTORY_DEPENDENCY_CYCLE&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return order, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=2424:3263 -->
````go
// 不改不可变parent_id：从真实叶节点删除，不因ID大小猜历史先后。
func HistoryDeletionOrder(rows []AccountHistoryRevision, keep map[uint64]bool) ([]uint64, error) {
	remaining := map[uint64]AccountHistoryRevision{}
	for _, row := range rows {
		remaining[row.ID] = row
	}
	order := []uint64{}
	for len(remaining) > 0 {
		parents := map[uint64]bool{}
		for _, row := range remaining {
			if row.Parent != 0 {
				parents[row.Parent] = true
			}
		}
		progress := false
		for id := range remaining {
			if parents[id] {
				continue
			}
			if !keep[id] {
				order = append(order, id)
			}
			delete(remaining, id)
			progress = true
		}
		if !progress {
			return nil, errors.New("ACCOUNT_HISTORY_DEPENDENCY_CYCLE")
		}
	}
	return order, nil
}

// account_retention.go：保留原实现，按业务合并文件。

````

## 第 7 步：AccountRetentionDocument

插入位置：第 6 步代码之后，紧接追加；完成稿约第 115 行。

保留计划只记录不可变依赖编号，绝不把活跃MAIN或整份私有正文写进回执。
Release为零表示整个文档待清理；非零只保留该正式版自己的依赖。

本步声明：`type AccountRetentionDocument struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Document</code> | <code>uint64</code> | 固定计划中的原文档<br>序列化/校验标记：<code>`json:&quot;document_id&quot;`</code> |
| <code>Space</code> | <code>uint64</code> | 原父空间<br>序列化/校验标记：<code>`json:&quot;space_id&quot;`</code> |
| <code>KnowledgeBase</code> | <code>uint64</code> | 原父知识库<br>序列化/校验标记：<code>`json:&quot;knowledge_base_id&quot;`</code> |
| <code>Release</code> | <code>uint64</code> | 固定保留公开版，零表示整篇清理<br>序列化/校验标记：<code>`json:&quot;release_id&quot;`</code> |
| <code>Manifest</code> | <code>uint64</code> | 该公开版清单<br>序列化/校验标记：<code>`json:&quot;manifest_id&quot;`</code> |
| <code>Structure</code> | <code>uint64</code> | 该清单结构<br>序列化/校验标记：<code>`json:&quot;structure_id&quot;`</code> |
| <code>Style</code> | <code>uint64</code> | 该清单样式<br>序列化/校验标记：<code>`json:&quot;style_id&quot;`</code> |
| <code>Versions</code> | <code>&#91;&#93;uint64</code> | 该清单实际固定正文版本集合<br>序列化/校验标记：<code>`json:&quot;version_ids&quot;`</code> |
| <code>Resources</code> | <code>&#91;&#93;uint64</code> | 该公开版实际附件集合<br>序列化/校验标记：<code>`json:&quot;resource_ids&quot;`</code> |

<!-- write-step: 7 bytes=3263:3907 -->
````go
// 保留计划只记录不可变依赖编号，绝不把活跃MAIN或整份私有正文写进回执。
// Release为零表示整个文档待清理；非零只保留该正式版自己的依赖。
type AccountRetentionDocument struct {
	Document      uint64   `json:"document_id"`
	Space         uint64   `json:"space_id"`
	KnowledgeBase uint64   `json:"knowledge_base_id"`
	Release       uint64   `json:"release_id"`
	Manifest      uint64   `json:"manifest_id"`
	Structure     uint64   `json:"structure_id"`
	Style         uint64   `json:"style_id"`
	Versions      []uint64 `json:"version_ids"`
	Resources     []uint64 `json:"resource_ids"`
}

````

## 第 8 步：AccountRetentionScope

插入位置：第 7 步代码之后，紧接追加；完成稿约第 129 行。

本步声明：`type AccountRetentionScope struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Key</code> | <code>string</code> | 计划内待关闭来源<br>序列化/校验标记：<code>`json:&quot;scope_key&quot;`</code> |
| <code>Epoch</code> | <code>uint64</code> | 原来源代次<br>序列化/校验标记：<code>`json:&quot;epoch&quot;`</code> |
| <code>Operation</code> | <code>string</code> | 原号派生的子屏障ID<br>序列化/校验标记：<code>`json:&quot;revocation_id&quot;`</code> |

<!-- write-step: 8 bytes=3907:4057 -->
````go
type AccountRetentionScope struct {
	Key       string `json:"scope_key"`
	Epoch     uint64 `json:"epoch"`
	Operation string `json:"revocation_id"`
}

````

## 第 9 步：AccountRetentionPlan

插入位置：第 8 步代码之后，紧接追加；完成稿约第 135 行。

本步声明：`type AccountRetentionPlan struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Spaces</code> | <code>&#91;&#93;uint64</code> | 固定将关闭或留存的空间集合<br>序列化/校验标记：<code>`json:&quot;space_ids&quot;`</code> |
| <code>KnowledgeBases</code> | <code>&#91;&#93;uint64</code> | 固定KB集合<br>序列化/校验标记：<code>`json:&quot;knowledge_base_ids&quot;`</code> |
| <code>Documents</code> | <code>&#91;&#93;AccountRetentionDocument</code> | 逐文档保留依赖，不能恢复时选新正式版<br>序列化/校验标记：<code>`json:&quot;documents&quot;`</code> |
| <code>Scopes</code> | <code>&#91;&#93;AccountRetentionScope</code> | 此计划精确关闭来源集合<br>序列化/校验标记：<code>`json:&quot;scopes&quot;`</code> |

<!-- write-step: 9 bytes=4057:4349 -->
````go
type AccountRetentionPlan struct {
	Spaces         []uint64                   `json:"space_ids"`
	KnowledgeBases []uint64                   `json:"knowledge_base_ids"`
	Documents      []AccountRetentionDocument `json:"documents"`
	Scopes         []AccountRetentionScope    `json:"scopes"`
}

````

## 第 10 步：AccountRetentionRevocation

插入位置：第 9 步代码之后，紧接追加；完成稿约第 142 行。

子屏障使用固定原号派生，和Account身份屏障及用户操作回执互不混用。

本步声明：`func AccountRetentionRevocation(operation, scope string) string`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>&quot;ACCOUNT_RETENTION\n&quot; + operation</code> |
| <code>scope</code> | <code>string</code> | 在此声明中的实际使用：<code>&quot;ACCOUNT_RETENTION\n&quot; + operation + &quot;\n&quot; + scope</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;retention-&quot; + hex.EncodeToString(digest&#91;:&#93;)</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return &quot;retention-&quot; + hex.EncodeToString(digest&#91;:&#93;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=4349:4652 -->
````go
// 子屏障使用固定原号派生，和Account身份屏障及用户操作回执互不混用。
func AccountRetentionRevocation(operation, scope string) string {
	digest := sha256.Sum256([]byte("ACCOUNT_RETENTION\n" + operation + "\n" + scope))
	return "retention-" + hex.EncodeToString(digest[:])
}

````

## 第 11 步：orderedPositive

插入位置：第 10 步代码之后，紧接追加；完成稿约第 148 行。

本步声明：`func orderedPositive(ids []uint64) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ids</code> | <code>&#91;&#93;uint64</code> | 在此声明中的实际使用：<code>ids == nil</code>；<code>ids&#91;i-1&#93; &gt;= id</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |

**跟着本段代码走一遍：**

- 检查 <code>ids == nil</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>id == 0 &#124;&#124; (i &gt; 0 &amp;&amp; ids&#91;i-1&#93; &gt;= id)</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 此处通过 <code>return true</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=4652:4838 -->
````go
func orderedPositive(ids []uint64) bool {
	if ids == nil {
		return false
	}
	for i, id := range ids {
		if id == 0 || (i > 0 && ids[i-1] >= id) {
			return false
		}
	}
	return true
}

````

## 第 12 步：AccountRetentionPlan.Valid

插入位置：第 11 步代码之后，紧接追加；完成稿约第 160 行。

本步声明：`func (p AccountRetentionPlan) Valid(operation string) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>AccountRetentionRevocation(operation, scope.Key)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>len(expected) == 0</code> |

**跟着本段代码走一遍：**

- 检查 <code>!orderedPositive(p.Spaces) &#124;&#124; !orderedPositive(p.KnowledgeBases) &#124;&#124; p.Documents == nil &#124;&#124; p.Scopes == nil</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>d.Document == 0 &#124;&#124; !slices.Contains(p.Spaces, d.Space) &#124;&#124; (d.KnowledgeBase != 0 &amp;&amp; !slices.Contains(p.KnowledgeBases, d.KnowledgeBase)) &#124;&#124; (i &gt; 0 &amp;&amp; p.Documents&#91;i-1&#93;.Document &gt;= d. …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>d.Release == 0</code>。
- 检查 <code>d.Manifest != 0 &#124;&#124; d.Structure != 0 &#124;&#124; d.Style != 0 &#124;&#124; len(d.Versions) != 0 &#124;&#124; len(d.Resources) != 0</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>d.Manifest == 0 &#124;&#124; d.Structure == 0 &#124;&#124; d.Style == 0</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>!expected&#91;scope.Key&#93; &#124;&#124; !platform.ValidDocumentScope(scope.Key) &#124;&#124; scope.Epoch == 0 &#124;&#124; scope.Operation != AccountRetentionRevocation(operation, scope.Key) &#124;&#124; (i &gt; 0 &amp;&amp; p.Scopes&#91;i-1&#93;.Key &gt;= scope.Key)</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 此处通过 <code>return len(expected) == 0</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=4838:6196 -->
````go
func (p AccountRetentionPlan) Valid(operation string) bool {
	if !orderedPositive(p.Spaces) || !orderedPositive(p.KnowledgeBases) || p.Documents == nil || p.Scopes == nil {
		return false
	}
	expected := map[string]bool{}
	for _, kb := range p.KnowledgeBases {
		expected["kb-acl:"+TextID(kb)] = true
	}
	for i, d := range p.Documents {
		if d.Document == 0 || !slices.Contains(p.Spaces, d.Space) || (d.KnowledgeBase != 0 && !slices.Contains(p.KnowledgeBases, d.KnowledgeBase)) || (i > 0 && p.Documents[i-1].Document >= d.Document) || !orderedPositive(d.Versions) || !orderedPositive(d.Resources) {
			return false
		}
		if d.Release == 0 {
			if d.Manifest != 0 || d.Structure != 0 || d.Style != 0 || len(d.Versions) != 0 || len(d.Resources) != 0 {
				return false
			}
		} else if d.Manifest == 0 || d.Structure == 0 || d.Style == 0 {
			return false
		}
		expected["doc-acl:"+TextID(d.Document)] = true
		expected["publication:"+TextID(d.Document)] = true
	}
	for i, scope := range p.Scopes {
		if !expected[scope.Key] || !platform.ValidDocumentScope(scope.Key) || scope.Epoch == 0 || scope.Operation != AccountRetentionRevocation(operation, scope.Key) || (i > 0 && p.Scopes[i-1].Key >= scope.Key) {
			return false
		}
		delete(expected, scope.Key)
	}
	return len(expected) == 0
}

// account_retirement.go：保留原实现，按业务合并文件。

````

## 第 13 步：AccountRetirementBinding

插入位置：第 12 步代码之后，紧接追加；完成稿约第 193 行。

AccountRetirementBinding由Account在锁定原注销操作和组织事实后给出。
ClosingOrganizations只能是权威决定随账号关闭的组织，不能由浏览器指定。

本步声明：`type AccountRetirementBinding platform.AccountClosureBinding`。

<!-- write-step: 13 bytes=6196:6446 -->
````go
// AccountRetirementBinding由Account在锁定原注销操作和组织事实后给出。
// ClosingOrganizations只能是权威决定随账号关闭的组织，不能由浏览器指定。
type AccountRetirementBinding platform.AccountClosureBinding

````

## 第 14 步：AccountRetirementPlan

插入位置：第 13 步代码之后，紧接追加；完成稿约第 197 行。

本步声明：`type AccountRetirementPlan struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Binding</code> | <code>AccountRetirementBinding</code> | Account原账号注销与权威关闭组织集合<br>序列化/校验标记：<code>`json:&quot;binding&quot;`</code> |
| <code>ReservedKnowledgeBases</code> | <code>&#91;&#93;uint64</code> | 已为此次退出预留的KB管理角色<br>序列化/校验标记：<code>`json:&quot;reserved_knowledge_bases&quot;`</code> |
| <code>Retention</code> | <code>*AccountRetentionPlan</code> | 身份排空后固定的公开留存/清理范围，准备阶段可空<br>序列化/校验标记：<code>`json:&quot;retention,omitempty&quot;`</code> |

<!-- write-step: 14 bytes=6446:6712 -->
````go
type AccountRetirementPlan struct {
	Binding                AccountRetirementBinding `json:"binding"`
	ReservedKnowledgeBases []uint64                 `json:"reserved_knowledge_bases"`
	Retention              *AccountRetentionPlan    `json:"retention,omitempty"`
}

````

## 第 15 步：AccountRetirementResult

插入位置：第 14 步代码之后，紧接追加；完成稿约第 203 行。

本步声明：`type AccountRetirementResult struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>State</code> | <code>string</code> | Document准备阶段结果，不是全局注销终态 |
| <code>RequiredKnowledgeBases</code> | <code>&#91;&#93;uint64</code> | 仍须先交接管理员职责的知识库 |
| <code>ReservedKnowledgeBases</code> | <code>&#91;&#93;uint64</code> | 已成功为原号预留的知识库 |

<!-- write-step: 15 bytes=6712:6850 -->
````go
type AccountRetirementResult struct {
	State                  string
	RequiredKnowledgeBases []uint64
	ReservedKnowledgeBases []uint64
}

````

## 第 16 步：AccountRetirementBinding.Valid

插入位置：第 15 步代码之后，紧接追加；完成稿约第 209 行。

本步声明：`func (b AccountRetirementBinding) Valid() bool`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>platform.ValidateAccountClosureBinding(platform.AccountClosureBinding(b))</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return platform.ValidateAccountClosureBinding(platform.AccountClosureBinding(b))</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=6850:6984 -->
````go
func (b AccountRetirementBinding) Valid() bool {
	return platform.ValidateAccountClosureBinding(platform.AccountClosureBinding(b))
}

````

## 第 17 步：AccountRetirementBinding.Same

插入位置：第 16 步代码之后，紧接追加；完成稿约第 213 行。

本步声明：`func (b AccountRetirementBinding) Same(other AccountRetirementBinding) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>other</code> | <code>AccountRetirementBinding</code> | 使用 <code>AccountRetirementBinding</code> 类型；其定义处的职责：AccountRetirementBinding由Account在锁定原注销操作和组织事实后给出。 ClosingOrganizations只能是权威决定随账号关闭的组织，不能由浏览器指定。<br>在此声明中的实际使用：<code>platform.AccountClosureBinding(other)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>platform.EqualAccountClosureBinding(platform.AccountClosureBinding(b), platform.AccountClosureBinding(other))</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return platform.EqualAccountClosureBinding(platform.AccountClosureBinding(b), platform.AccountClosureBinding(other))</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 17 bytes=6984:7183 -->
````go
func (b AccountRetirementBinding) Same(other AccountRetirementBinding) bool {
	return platform.EqualAccountClosureBinding(platform.AccountClosureBinding(b), platform.AccountClosureBinding(other))
}

````

## 第 18 步：DecodeAccountRetirementPlan

插入位置：第 17 步代码之后，紧接追加；完成稿约第 217 行。

本步声明：`func DecodeAccountRetirementPlan(raw []byte) (AccountRetirementPlan, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>bytes.NewReader(raw)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>AccountRetirementPlan</code> | 实际返回表达式：<code>plan</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>errors.New(&quot;ACCOUNT_RETIREMENT_PLAN_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>decoder.Decode(&amp;plan) != nil &#124;&#124; decoder.Decode(new(any)) != io.EOF &#124;&#124; !plan.Binding.Valid() &#124;&#124; plan.ReservedKnowledgeBases == nil</code>。条件成立时执行 <code>return plan, errors.New(&quot;ACCOUNT_RETIREMENT_PLAN_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>id &lt;= previous</code>。条件成立时执行 <code>return plan, errors.New(&quot;ACCOUNT_RETIREMENT_PLAN_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>plan.Retention != nil &amp;&amp; !plan.Retention.Valid(plan.Binding.Operation)</code>。条件成立时执行 <code>return plan, errors.New(&quot;ACCOUNT_RETIREMENT_PLAN_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return plan, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 18 bytes=7183:7917 -->
````go
func DecodeAccountRetirementPlan(raw []byte) (AccountRetirementPlan, error) {
	var plan AccountRetirementPlan
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&plan) != nil || decoder.Decode(new(any)) != io.EOF || !plan.Binding.Valid() || plan.ReservedKnowledgeBases == nil {
		return plan, errors.New("ACCOUNT_RETIREMENT_PLAN_INVALID")
	}
	previous := uint64(0)
	for _, id := range plan.ReservedKnowledgeBases {
		if id <= previous {
			return plan, errors.New("ACCOUNT_RETIREMENT_PLAN_INVALID")
		}
		previous = id
	}
	if plan.Retention != nil && !plan.Retention.Valid(plan.Binding.Operation) {
		return plan, errors.New("ACCOUNT_RETIREMENT_PLAN_INVALID")
	}
	return plan, nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/domain/document/account_lifecycle_models.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/13-account-retirement.md)。
