# 分步编写 `services/document/internal/domain/document/workspace_models.go`

先按 [D02 · 创建工作区与可靠正文阅读](../lessons/document/02-workspace-content.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/105-workspace-models.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

定义工作区初始正文、固定阅读视图及章节插入算法。

新文档也需要真实版本、lineage和manifest，不能用零号占位再补齐来源。

写之前先按这条执行路线理解各段的连接：

1. BuildInitialContent生成规范初始正文与provenance
2. InsertChapterEntry按锚点插章并保留孩子局部顺序
3. BranchBaselineBody只对可信MAIN投影字体。

本文件需要守住的边界：

- 不能修改不可变MAIN字节
- 支线基线的字体投影不是放宽客户端正文校验。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/domain/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/domain/document/workspace_models.go') -and (Get-Item -LiteralPath 'services/document/internal/domain/document/workspace_models.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/domain/document/workspace_models.go')) { New-Item -ItemType File -Path 'services/document/internal/domain/document/workspace_models.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:353 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	"bytes"
	codec "collaboration/contracts/content"
	dto "collaboration/contracts/httpdto"
	"crypto/sha256"
	"encoding/json"
	"strings"
	"time"
	"unicode/utf8"
)

// branch_baseline.go：保留原实现，按业务合并文件。

````

## 第 2 步：BranchBaselineBody

插入位置：第 1 步代码之后，紧接追加；完成稿约第 17 行。

只用于已核验真实MAIN版本的支线基线投影。支线字体由样式控制；正文、块ID和其他格式保持。
Decode得到新树，绝不重写不可变MAIN版本，也不用于放宽客户端UPDATE/送审正文的校验。

本步声明：`func BranchBaselineBody(raw []byte) ([]byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>codec.Decode(raw)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 由 <code>codec.Canonical(value)</code> 的第 1 个返回项原样转交。<br>实际返回表达式：<code>nil</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>codec.Canonical(value)</code> 的第 2 个返回项原样转交。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>!ok</code>。条件成立时执行 <code>return</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>ok</code>。
- 在本段的匿名函数内：检查 <code>ok &amp;&amp; object&#91;&quot;type&quot;&#93; == &quot;font&quot;</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 在本段的匿名函数内：检查 <code>len(permitted) == 0</code>。
- 在本段的匿名函数内：检查 <code>ok</code>。
- 此处通过 <code>return codec.Canonical(value)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 2 bytes=353:1427 -->
````go
// 只用于已核验真实MAIN版本的支线基线投影。支线字体由样式控制；正文、块ID和其他格式保持。
// Decode得到新树，绝不重写不可变MAIN版本，也不用于放宽客户端UPDATE/送审正文的校验。
func BranchBaselineBody(raw []byte) ([]byte, error) {
	value, err := codec.Decode(raw)
	if err != nil {
		return nil, err
	}
	var visit func(any)
	visit = func(value any) {
		node, ok := value.(map[string]any)
		if !ok {
			return
		}
		if marks, ok := node["marks"].([]any); ok {
			permitted := make([]any, 0, len(marks))
			for _, mark := range marks {
				if object, ok := mark.(map[string]any); ok && object["type"] == "font" {
					continue
				}
				permitted = append(permitted, mark)
			}
			if len(permitted) == 0 {
				delete(node, "marks")
			} else {
				node["marks"] = permitted
			}
		}
		if children, ok := node["content"].([]any); ok {
			for _, child := range children {
				visit(child)
			}
		}
	}
	visit(value)
	return codec.Canonical(value)
}

// chapter.go：保留原实现，按业务合并文件。

````

## 第 3 步：ParseChapterCreate

插入位置：第 2 步代码之后，紧接追加；完成稿约第 56 行。

本步声明：`func ParseChapterCreate(input dto.ChapterCreate) (MainControlIntent, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>input</code> | <code>dto.ChapterCreate</code> | 在此声明中的实际使用：<code>input.OperationId</code>；<code>&amp;input</code>；<code>input.Title</code>；<code>input.ExpectedStructureId</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>MainControlIntent</code> | 使用 <code>MainControlIntent</code> 类型；其定义处的职责：MainControlIntent固定管理动作；它不借用审核请求，也不隐含任何批准或支线接纳。<br>实际返回表达式：<code>intent</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>Violation(InvalidRequest, &quot;CHAPTER_INPUT_INVALID&quot;)</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>!ValidOperationID(input.OperationId) &#124;&#124; strings.TrimSpace(input.Title) == &quot;&quot; &#124;&#124; 		!utf8.ValidString(input.Title) &#124;&#124; utf8.RuneCountInString(input.Title) &gt; 240</code>。条件成立时执行 <code>return intent, Violation(InvalidRequest, &quot;CHAPTER_INPUT_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return intent, err</code>。当前函数在这里结束。
- 检查 <code>input.AfterChapterId != nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return intent, err</code>。当前函数在这里结束。
- 此处通过 <code>return intent, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=1427:2081 -->
````go
func ParseChapterCreate(input dto.ChapterCreate) (MainControlIntent, error) {
	intent := MainControlIntent{Operation: input.OperationId, Kind: "CHAPTER_CREATE", Chapter: &input}
	if !ValidOperationID(input.OperationId) || strings.TrimSpace(input.Title) == "" ||
		!utf8.ValidString(input.Title) || utf8.RuneCountInString(input.Title) > 240 {
		return intent, Violation(InvalidRequest, "CHAPTER_INPUT_INVALID")
	}
	if _, err := CanonicalID(input.ExpectedStructureId); err != nil {
		return intent, err
	}
	if input.AfterChapterId != nil {
		if _, err := CanonicalID(*input.AfterChapterId); err != nil {
			return intent, err
		}
	}
	return intent, nil
}

````

## 第 4 步：InsertChapterEntry

插入位置：第 3 步代码之后，紧接追加；完成稿约第 73 行。

结构修订只追加新快照；一级章节移动不改变其二级孩子的局部顺序。

本步声明：`func InsertChapterEntry(entries []dto.StructureEntry, input dto.ChapterCreate, chapter, unit string) ([]dto.StructureEntry, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>entries</code> | <code>&#91;&#93;dto.StructureEntry</code> | 在此声明中的实际使用：<code>len(entries)</code>；<code>entries&#91;index&#93;.ChapterId</code>；<code>append(result, entries&#91;:index&#93;...)</code>；<code>append(result, entries&#91;index:&#93;...)</code> |
| <code>input</code> | <code>dto.ChapterCreate</code> | 在此声明中的实际使用：<code>ParseChapterCreate(input)</code>；<code>input.AfterChapterId</code>；<code>input.Title</code> |
| <code>chapter</code> | <code>string</code> | 在此声明中的实际使用：<code>NodeId: chapter</code>；<code>ChapterId: chapter</code>；<code>entry.NodeId != chapter</code> |
| <code>unit</code> | <code>string</code> | 在此声明中的实际使用：<code>&amp;unit</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;dto.StructureEntry</code> | 实际返回表达式：<code>nil</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>Violation(StateConflict, &quot;CHAPTER_ANCHOR_CHANGED&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>entry.ParentNodeId != nil &#124;&#124; entry.Kind != &quot;CHAPTER&quot;</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>input.AfterChapterId == nil</code>。
- 检查 <code>order &lt;= entry.SortOrder</code>。
- 检查 <code>entry.ChapterId == *input.AfterChapterId</code>。
- 检查 <code>!found</code>。条件成立时执行 <code>return nil, Violation(StateConflict, &quot;CHAPTER_ANCHOR_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>entry.NodeId != chapter &amp;&amp; entry.ParentNodeId == nil &amp;&amp; entry.SortOrder &gt;= order</code>。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=2081:3524 -->
````go
// 结构修订只追加新快照；一级章节移动不改变其二级孩子的局部顺序。
func InsertChapterEntry(entries []dto.StructureEntry, input dto.ChapterCreate, chapter, unit string) ([]dto.StructureEntry, error) {
	if _, err := ParseChapterCreate(input); err != nil {
		return nil, err
	}
	index, order := len(entries), int64(1)
	found := input.AfterChapterId == nil
	for i, entry := range entries {
		if entry.ParentNodeId != nil || entry.Kind != "CHAPTER" {
			continue
		}
		if input.AfterChapterId == nil {
			if order <= entry.SortOrder {
				order = entry.SortOrder + 1
			}
		} else if entry.ChapterId == *input.AfterChapterId {
			found, order, index = true, entry.SortOrder+1, i+1
			for index < len(entries) && entries[index].ChapterId == entry.ChapterId {
				index++
			}
		}
	}
	if !found {
		return nil, Violation(StateConflict, "CHAPTER_ANCHOR_CHANGED")
	}
	result := make([]dto.StructureEntry, 0, len(entries)+1)
	result = append(result, entries[:index]...)
	result = append(result, dto.StructureEntry{NodeId: chapter, ChapterId: chapter, UnitId: &unit,
		Kind: "CHAPTER", Title: input.Title, SortOrder: order})
	result = append(result, entries[index:]...)
	for i := range result {
		entry := &result[i]
		if entry.NodeId != chapter && entry.ParentNodeId == nil && entry.SortOrder >= order {
			entry.SortOrder++
		}
	}
	return result, nil
}

// content_version.go：保留原实现，按业务合并文件。

````

## 第 5 步：ContentVersionScope

插入位置：第 4 步代码之后，紧接追加；完成稿约第 114 行。

版本号只标识不可变正文；当前可读范围仍由文档和原单元归属决定。

本步声明：`type ContentVersionScope struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Version</code> | <code>uint64</code> | 不可变content_version编号 |
| <code>Document</code> | <code>uint64</code> | 版本实际所属文档 |
| <code>Unit</code> | <code>uint64</code> | 版本原协作单元 |
| <code>Structure</code> | <code>uint64</code> | 版本产生时的结构修订 |

<!-- write-step: 5 bytes=3524:3760 -->
````go
// 版本号只标识不可变正文；当前可读范围仍由文档和原单元归属决定。
type ContentVersionScope struct {
	Version, Document, Unit, Structure uint64
}

// cursor.go：保留原实现，按业务合并文件。

````

## 第 6 步：Cursor

插入位置：第 5 步代码之后，紧接追加；完成稿约第 121 行。

本步声明：`type Cursor struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Binding</code> | <code>string</code> | 签名绑定的用户和列表上下文<br>序列化/校验标记：<code>`json:&quot;binding&quot;`</code> |
| <code>Last</code> | <code>string</code> | 最后已消费实体的规范文本ID<br>序列化/校验标记：<code>`json:&quot;last&quot;`</code> |
| <code>Until</code> | <code>int64</code> | 游标绝对到期Unix秒值；普通Pagination.Next签发时设为当前时间加五分钟<br>序列化/校验标记：<code>`json:&quot;until&quot;`</code> |

<!-- write-step: 6 bytes=3760:3940 -->
````go
type Cursor struct {
	Binding string `json:"binding"`
	Last    string `json:"last"`
	Until   int64  `json:"until"`
}

// directory.go：保留原实现，按业务合并文件。

````

## 第 7 步：DirectorySource

插入位置：第 6 步代码之后，紧接追加；完成稿约第 129 行。

本步声明：`type DirectorySource struct{ ID, Epoch uint64 }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ID</code> | <code>uint64</code> | 派生目录可见性的文档编号 |
| <code>Epoch</code> | <code>uint64</code> | 对应文档ACL代次，撤权时可关闭目录输出 |

<!-- write-step: 7 bytes=3940:4050 -->
````go
type DirectorySource struct{ ID, Epoch uint64 }

// editable.go：保留原实现，按业务合并文件。

````

## 第 8 步：EditableUnitFact

插入位置：第 7 步代码之后，紧接追加；完成稿约第 133 行。

本步声明：`type EditableUnitFact struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ID</code> | <code>string</code> | 可编辑单元文本ID |
| <code>Title</code> | <code>string</code> | 结构中显示的单元标题 |
| <code>Branch</code> | <code>*string</code> | 活动支线ID，可空表示尚无 |
| <code>Lifecycle</code> | <code>*string</code> | 支线状态，可空不能视为ACTIVE |
| <code>Lineage</code> | <code>*string</code> | 支线当前谱系ID |
| <code>Epoch</code> | <code>string</code> | 当前工作区代次文本 |

<!-- write-step: 8 bytes=4050:4258 -->
````go
type EditableUnitFact struct {
	ID, Title                  string
	Branch, Lifecycle, Lineage *string
	Epoch                      string
}

// initial_content.go：保留原实现，按业务合并文件。

````

## 第 9 步：InitialStyle

插入位置：第 8 步代码之后，紧接追加；完成稿约第 141 行。

本步声明：`const InitialStyle = \`{"body_font":"SimSun","western_font":"Times New Roman","body_size_pt":12,"heading_sizes_pt":[22,18,16,14,12,12],"page":"A4","margin_cm":"2.54","title_font":"SimHei","title_size_pt":22}\``。

<!-- write-step: 9 bytes=4258:4467 -->
````go
const InitialStyle = `{"body_font":"SimSun","western_font":"Times New Roman","body_size_pt":12,"heading_sizes_pt":[22,18,16,14,12,12],"page":"A4","margin_cm":"2.54","title_font":"SimHei","title_size_pt":22}`

````

## 第 10 步：InitialContent

插入位置：第 9 步代码之后，紧接追加；完成稿约第 143 行。

本步声明：`type InitialContent struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Version</code> | <code>uint64</code> | 预先分配且将真实插入的初始正文版本ID |
| <code>Lineage</code> | <code>uint64</code> | 初始支线谱系ID |
| <code>Canonical</code> | <code>&#91;&#93;byte</code> | 规范空正文的固定字节 |
| <code>Provenance</code> | <code>&#91;&#93;byte</code> | 绑定实际版本、单元、manifest的来源字节 |
| <code>Hash</code> | <code>&#91;32&#93;byte</code> | Canonical的SHA-256 |

<!-- write-step: 10 bytes=4467:4590 -->
````go
type InitialContent struct {
	Version, Lineage      uint64
	Canonical, Provenance []byte
	Hash                  [32]byte
}
````

## 第 11 步：CreationKnowledgeBase

插入位置：第 10 步代码之后，紧接追加；完成稿约第 148 行。

本步声明：`type CreationKnowledgeBase struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Space</code> | <code>uint64</code> | 目标KB实际所属空间 |
| <code>Revision</code> | <code>uint64</code> | 创建准入预读时的ACL revision |
| <code>Lifecycle</code> | <code>string</code> | KB当前生命周期，创建时必须符合活动条件 |

<!-- write-step: 11 bytes=4590:4677 -->
````go
type CreationKnowledgeBase struct {
	Space, Revision uint64
	Lifecycle       string
}

````

## 第 12 步：CanonicalJSON

插入位置：第 11 步代码之后，紧接追加；完成稿约第 153 行。

本步声明：`func CanonicalJSON(value any) ([]byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>any</code> | 在此声明中的实际使用：<code>encoder.Encode(value)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>nil</code>；<code>bytes.TrimSuffix(buffer.Bytes(), &#91;&#93;byte(&quot;\n&quot;))</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return bytes.TrimSuffix(buffer.Bytes(), &#91;&#93;byte(&quot;\n&quot;)), nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=4677:4948 -->
````go
func CanonicalJSON(value any) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buffer.Bytes(), []byte("\n")), nil
}

````

## 第 13 步：BuildInitialContent

插入位置：第 12 步代码之后，紧接追加；完成稿约第 163 行。

初始正文和provenance绑定真实版本、lineage及manifest；不使用不存在的占位编号。

本步声明：`func BuildInitialContent(version, lineage, unit, manifest uint64, operation string) (InitialContent, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>version</code> | <code>uint64</code> | 在此声明中的实际使用：<code>TextID(version)</code>；<code>Version: version</code> |
| <code>lineage</code> | <code>uint64</code> | 在此声明中的实际使用：<code>TextID(lineage)</code>；<code>Lineage: lineage</code> |
| <code>unit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>TextID(unit)</code> |
| <code>manifest</code> | <code>uint64</code> | 在此声明中的实际使用：<code>TextID(manifest)</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>&quot;source_operation_id&quot;: operation</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>InitialContent</code> | 实际返回表达式：<code>InitialContent{}</code>；<code>InitialContent{Version: version, Lineage: lineage, Canonical: canonical, Provenance: raw, Hash: sha256.Sum256(canonical)}</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return InitialContent{}, err</code>。当前函数在这里结束。
- 此处通过 <code>return InitialContent{Version: version, Lineage: lineage, Canonical: canonical, Provenance: raw, Hash: sha256.Sum256(canonical)}, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=4948:6128 -->
````go
// 初始正文和provenance绑定真实版本、lineage及manifest；不使用不存在的占位编号。
func BuildInitialContent(version, lineage, unit, manifest uint64, operation string) (InitialContent, error) {
	block := "initial-" + TextID(unit)
	body := map[string]any{"type": "doc", "content": []any{map[string]any{"type": "paragraph", "attrs": map[string]string{"id": block}, "content": []any{}}}}
	canonical, err := CanonicalJSON(body)
	if err != nil {
		return InitialContent{}, err
	}
	provenance := map[string]any{"schema_version": "provenance-v1", "lineage_id": TextID(lineage), "base_manifest_id": TextID(manifest), "base_content_version_id": TextID(version), "source_operation_id": operation, "accepted_captures": []any{}, "block_sources": []any{map[string]any{"block_id": block, "origin_content_version_id": TextID(version), "origin_block_id": block, "relation": "INSERTED", "source_range": map[string]int{"start": 0, "end": 0}}}}
	raw, _ := json.Marshal(provenance)
	return InitialContent{Version: version, Lineage: lineage, Canonical: canonical, Provenance: raw, Hash: sha256.Sum256(canonical)}, nil
}

// name.go：保留原实现，按业务合并文件。

````

## 第 14 步：ValidName

插入位置：第 13 步代码之后，紧接追加；完成稿约第 178 行。

名称按Unicode字符限制，保留用户输入；空白名称不形成有效业务对象。

本步声明：`func ValidName(text string, maximum int) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>text</code> | <code>string</code> | 在此声明中的实际使用：<code>strings.TrimSpace(text)</code>；<code>&#91;&#93;rune(text)</code> |
| <code>maximum</code> | <code>int</code> | 在此声明中的实际使用：<code>len(&#91;&#93;rune(text)) &lt;= maximum</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>strings.TrimSpace(text) != &quot;&quot; &amp;&amp; len(&#91;&#93;rune(text)) &lt;= maximum</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return strings.TrimSpace(text) != &quot;&quot; &amp;&amp; len(&#91;&#93;rune(text)) &lt;= maximum</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=6128:6346 -->
````go
// 名称按Unicode字符限制，保留用户输入；空白名称不形成有效业务对象。
func ValidName(text string, maximum int) bool {
	return strings.TrimSpace(text) != "" && len([]rune(text)) <= maximum
}
````

## 第 15 步：Visibility

插入位置：第 14 步代码之后，紧接追加；完成稿约第 182 行。

本步声明：`func Visibility(value *string) string`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>*string</code> | 在此声明中的实际使用：<code>value == nil</code>；<code>return *value</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;COLLABORATORS&quot;</code>；<code>*value</code> |

**跟着本段代码走一遍：**

- 检查 <code>value == nil</code>。条件成立时执行 <code>return &quot;COLLABORATORS&quot;</code>。当前函数在这里结束。
- 此处通过 <code>return *value</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=6346:6450 -->
````go
func Visibility(value *string) string {
	if value == nil {
		return "COLLABORATORS"
	}
	return *value
}
````

## 第 16 步：ValidVisibility

插入位置：第 15 步代码之后，紧接追加；完成稿约第 188 行。

本步声明：`func ValidVisibility(value string) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>string</code> | 在此声明中的实际使用：<code>value == &quot;COLLABORATORS&quot;</code>；<code>value == &quot;SPACE_MEMBERS&quot;</code>；<code>value == &quot;PUBLIC&quot;</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>value == &quot;COLLABORATORS&quot; &#124;&#124; value == &quot;SPACE_MEMBERS&quot; &#124;&#124; value == &quot;PUBLIC&quot;</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return value == &quot;COLLABORATORS&quot; &#124;&#124; value == &quot;SPACE_MEMBERS&quot; &#124;&#124; value == &quot;PUBLIC&quot;</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=6450:6639 -->
````go
func ValidVisibility(value string) bool {
	return value == "COLLABORATORS" || value == "SPACE_MEMBERS" || value == "PUBLIC"
}

// read_view.go：保留原实现，按业务合并文件。

````

## 第 17 步：ReadView

插入位置：第 16 步代码之后，紧接追加；完成稿约第 194 行。

本步声明：`type ReadView struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ID</code> | <code>string</code> | 这次固定阅读视图ID |
| <code>Document</code> | <code>uint64</code> | 视图所属文档 |
| <code>Manifest</code> | <code>string</code> | 固定全文清单文本ID |
| <code>Admission</code> | <code>string</code> | 真实读取准入ID |
| <code>Expires</code> | <code>time.Time</code> | 视图期限，后续单元读取需复核 |

<!-- write-step: 17 bytes=6639:6779 -->
````go
type ReadView struct {
	ID                  string
	Document            uint64
	Manifest, Admission string
	Expires             time.Time
}
````

## 第 18 步：FixedReadView

插入位置：第 17 步代码之后，紧接追加；完成稿约第 200 行。

本步声明：`type FixedReadView struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Manifest</code> | <code>uint64</code> | 已固定阅读清单ID |
| <code>Actor</code> | <code>uint64</code> | 创建视图的用户ID |
| <code>Session</code> | <code>&#91;&#93;byte</code> | 原真实会话引用，不是Cookie token |
| <code>State</code> | <code>string</code> | 阅读视图状态 |
| <code>AdmissionState</code> | <code>string</code> | 关联准入状态，关闭时不得继续输出 |
| <code>Expires</code> | <code>time.Time</code> | 原视图到期时间 |

<!-- write-step: 18 bytes=6779:6932 -->
````go
type FixedReadView struct {
	Manifest, Actor       uint64
	Session               []byte
	State, AdmissionState string
	Expires               time.Time
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/domain/document/workspace_models.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/02-workspace-content.md)。
