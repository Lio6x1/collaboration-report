# 分步编写 `services/document/internal/domain/document/structure_models.go`

先按 [D04 · 结构预览、拆章、合章与历史范围](../lessons/document/04-structure.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/103-structure-models.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

表达管理意图、拆合迁移候选和历史范围边界，并提供纯结构变换。

目录移动可保留原单元，拆合会改变内容范围；必须分清稳定目录变化与需要完整预览的范围变化。

写之前先按这条执行路线理解各段的连接：

1. ParseStructureChange解析动作
2. StructureConfirmationReference要求共用同一预览
3. ValidateStructureConfirmation逐项比较原动作
4. ParseStructureMigration核正文与完整区间
5. ApplyStructureMigration使用事务分配的新身份。

本文件需要守住的边界：

- 不能拼接多个预览的cut
- MainWorkspaceFact区分管理revision与实际MAIN物理epoch
- 虚拟支线不能伪造room cut。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/domain/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/domain/document/structure_models.go') -and (Get-Item -LiteralPath 'services/document/internal/domain/document/structure_models.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/domain/document/structure_models.go')) { New-Item -ItemType File -Path 'services/document/internal/domain/document/structure_models.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:448 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	"bytes"
	codec "collaboration/contracts/content"
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	"crypto/sha256"
	"encoding/json"
	"google.golang.org/protobuf/proto"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// main_control.go：保留原实现，按业务合并文件。

````

## 第 2 步：MainControlIntent

插入位置：第 1 步代码之后，紧接追加；完成稿约第 21 行。

MainControlIntent固定管理动作；它不借用审核请求，也不隐含任何批准或支线接纳。

本步声明：`type MainControlIntent struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Operation</code> | <code>string</code> | 原管理操作号<br>序列化/校验标记：<code>`json:&quot;operation&quot;`</code> |
| <code>Kind</code> | <code>string</code> | MAIN_LOCK、STYLE_CHANGE、CHAPTER_CREATE或STRUCTURE_CHANGE<br>序列化/校验标记：<code>`json:&quot;kind&quot;`</code> |
| <code>Control</code> | <code>uint64</code> | 页面期待的control revision<br>序列化/校验标记：<code>`json:&quot;control&quot;`</code> |
| <code>Style</code> | <code>uint64</code> | 页面期待的style revision<br>序列化/校验标记：<code>`json:&quot;style&quot;`</code> |
| <code>Locked</code> | <code>bool</code> | 目标主线锁定状态<br>序列化/校验标记：<code>`json:&quot;locked&quot;`</code> |
| <code>StyleJSON</code> | <code>json.RawMessage</code> | 固定新样式字节，仅样式动作使用<br>序列化/校验标记：<code>`json:&quot;style_json,omitempty&quot;`</code> |
| <code>Chapter</code> | <code>*dto.ChapterCreate</code> | 新增章节的规范HTTP意图<br>序列化/校验标记：<code>`json:&quot;chapter,omitempty&quot;`</code> |
| <code>Structure</code> | <code>*dto.StructureChange</code> | 目录或拆合变更的完整规范意图<br>序列化/校验标记：<code>`json:&quot;structure,omitempty&quot;`</code> |

<!-- write-step: 2 bytes=448:1017 -->
````go
// MainControlIntent固定管理动作；它不借用审核请求，也不隐含任何批准或支线接纳。
type MainControlIntent struct {
	Operation string               `json:"operation"`
	Kind      string               `json:"kind"`
	Control   uint64               `json:"control"`
	Style     uint64               `json:"style"`
	Locked    bool                 `json:"locked"`
	StyleJSON json.RawMessage      `json:"style_json,omitempty"`
	Chapter   *dto.ChapterCreate   `json:"chapter,omitempty"`
	Structure *dto.StructureChange `json:"structure,omitempty"`
}

````

## 第 3 步：CanonicalRevision

插入位置：第 2 步代码之后，紧接追加；完成稿约第 33 行。

本步声明：`func CanonicalRevision(value string) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>string</code> | 在此声明中的实际使用：<code>strconv.ParseUint(value, 10, 64)</code>；<code>TextID(revision) != value</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>0</code>；<code>revision</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>Violation(InvalidRequest, &quot;REVISION_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil &#124;&#124; TextID(revision) != value</code>。条件成立时执行 <code>return 0, Violation(InvalidRequest, &quot;REVISION_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return revision, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=1017:1255 -->
````go
func CanonicalRevision(value string) (uint64, error) {
	revision, err := strconv.ParseUint(value, 10, 64)
	if err != nil || TextID(revision) != value {
		return 0, Violation(InvalidRequest, "REVISION_INVALID")
	}
	return revision, nil
}

````

## 第 4 步：ParseMainLock

插入位置：第 3 步代码之后，紧接追加；完成稿约第 41 行。

本步声明：`func ParseMainLock(input dto.MainLock) (MainControlIntent, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>input</code> | <code>dto.MainLock</code> | 在此声明中的实际使用：<code>input.OperationId</code>；<code>input.Locked</code>；<code>input.ExpectedControlRevision</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>MainControlIntent</code> | 使用 <code>MainControlIntent</code> 类型；其定义处的职责：MainControlIntent固定管理动作；它不借用审核请求，也不隐含任何批准或支线接纳。<br>实际返回表达式：<code>intent</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>Violation(InvalidRequest, &quot;MAIN_CONTROL_INPUT_INVALID&quot;)</code>；<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>!ValidOperationID(intent.Operation)</code>。条件成立时执行 <code>return intent, Violation(InvalidRequest, &quot;MAIN_CONTROL_INPUT_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return intent, err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=1255:1651 -->
````go
func ParseMainLock(input dto.MainLock) (MainControlIntent, error) {
	intent := MainControlIntent{Operation: input.OperationId, Kind: "MAIN_LOCK", Locked: input.Locked}
	if !ValidOperationID(intent.Operation) {
		return intent, Violation(InvalidRequest, "MAIN_CONTROL_INPUT_INVALID")
	}
	var err error
	intent.Control, err = CanonicalRevision(input.ExpectedControlRevision)
	return intent, err
}

````

## 第 5 步：ParseStyleChange

插入位置：第 4 步代码之后，紧接追加；完成稿约第 51 行。

本步声明：`func ParseStyleChange(input dto.StyleChange) (MainControlIntent, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>input</code> | <code>dto.StyleChange</code> | 在此声明中的实际使用：<code>input.OperationId</code>；<code>input.ExpectedControlRevision</code>；<code>input.ExpectedStyleId</code>；<code>input.Style</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>MainControlIntent</code> | 使用 <code>MainControlIntent</code> 类型；其定义处的职责：MainControlIntent固定管理动作；它不借用审核请求，也不隐含任何批准或支线接纳。<br>实际返回表达式：<code>intent</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>Violation(InvalidRequest, &quot;MAIN_CONTROL_INPUT_INVALID&quot;)</code>；<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>!ValidOperationID(intent.Operation)</code>。条件成立时执行 <code>return intent, Violation(InvalidRequest, &quot;MAIN_CONTROL_INPUT_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return intent, err</code>。当前函数在这里结束。

<!-- write-step: 5 bytes=1651:2494 -->
````go
func ParseStyleChange(input dto.StyleChange) (MainControlIntent, error) {
	intent := MainControlIntent{Operation: input.OperationId, Kind: "STYLE_CHANGE"}
	if !ValidOperationID(intent.Operation) {
		return intent, Violation(InvalidRequest, "MAIN_CONTROL_INPUT_INVALID")
	}
	var err error
	if intent.Control, err = CanonicalRevision(input.ExpectedControlRevision); err != nil {
		return intent, err
	}
	if intent.Style, err = CanonicalID(input.ExpectedStyleId); err != nil {
		return intent, err
	}
	raw, err := json.Marshal(input.Style)
	if err != nil {
		return intent, err
	}
	value, err := codec.Decode(raw)
	if err != nil {
		return intent, err
	}
	// HTTP入口已经按冻结StyleChange schema核验字段；固定规范字节用于不可变样式与同号绑定。
	intent.StyleJSON, err = codec.Canonical(value)
	return intent, err
}

````

## 第 6 步：ValidateMainControl

插入位置：第 5 步代码之后，紧接追加；完成稿约第 76 行。

本步声明：`func ValidateMainControl(permissions Permissions, control, style string, locked bool, intent MainControlIntent) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>permissions</code> | <code>Permissions</code> | 使用 <code>Permissions</code> 类型；其定义处的职责：把账号会话事实、文档授权来源和撤权关闭计划表达成领域类型。<br>在此声明中的实际使用：<code>permissions.Manage</code> |
| <code>control</code> | <code>string</code> | 在此声明中的实际使用：<code>control != TextID(intent.Control)</code> |
| <code>style</code> | <code>string</code> | 在此声明中的实际使用：<code>style != TextID(intent.Style)</code> |
| <code>locked</code> | <code>bool</code> | 在此声明中的实际使用：<code>(intent.Kind == &quot;STYLE_CHANGE&quot; &#124;&#124; intent.Kind == &quot;STRUCTURE_CHANGE&quot;) &amp;&amp; locked</code> |
| <code>intent</code> | <code>MainControlIntent</code> | 使用 <code>MainControlIntent</code> 类型；其定义处的职责：MainControlIntent固定管理动作；它不借用审核请求，也不隐含任何批准或支线接纳。<br>在此声明中的实际使用：<code>intent.Control</code>；<code>intent.Kind</code>；<code>intent.Style</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>Violation(PermissionDenied, &quot;MAIN_CONTROL_NOT_ALLOWED&quot;)</code>；<code>Violation(StateConflict, &quot;MAIN_CONTROL_CHANGED&quot;)</code>；<code>Violation(StateConflict, &quot;MAIN_STYLE_CHANGED&quot;)</code>；<code>Violation(StateConflict, &quot;MAIN_LOCKED&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!permissions.Manage</code>。条件成立时执行 <code>return Violation(PermissionDenied, &quot;MAIN_CONTROL_NOT_ALLOWED&quot;)</code>。当前函数在这里结束。
- 检查 <code>control != TextID(intent.Control)</code>。条件成立时执行 <code>return Violation(StateConflict, &quot;MAIN_CONTROL_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>intent.Kind == &quot;STYLE_CHANGE&quot; &amp;&amp; style != TextID(intent.Style)</code>。条件成立时执行 <code>return Violation(StateConflict, &quot;MAIN_STYLE_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>(intent.Kind == &quot;STYLE_CHANGE&quot; &#124;&#124; intent.Kind == &quot;STRUCTURE_CHANGE&quot;) &amp;&amp; locked</code>。条件成立时执行 <code>return Violation(StateConflict, &quot;MAIN_LOCKED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=2494:3156 -->
````go
func ValidateMainControl(permissions Permissions, control, style string, locked bool, intent MainControlIntent) error {
	if !permissions.Manage {
		return Violation(PermissionDenied, "MAIN_CONTROL_NOT_ALLOWED")
	}
	if control != TextID(intent.Control) {
		return Violation(StateConflict, "MAIN_CONTROL_CHANGED")
	}
	if intent.Kind == "STYLE_CHANGE" && style != TextID(intent.Style) {
		return Violation(StateConflict, "MAIN_STYLE_CHANGED")
	}
	if (intent.Kind == "STYLE_CHANGE" || intent.Kind == "STRUCTURE_CHANGE") && locked {
		return Violation(StateConflict, "MAIN_LOCKED")
	}
	return nil
}

// main_workspace.go：保留原实现，按业务合并文件。

````

## 第 7 步：MainWorkspaceFact

插入位置：第 6 步代码之后，紧接追加；完成稿约第 94 行。

MainWorkspaceFact只保存原安装的标量证据，不携带正文。管理元数据修订和
物理MAIN代次可以不同；只有改变MAIN并完成实际安装，才产生新的物理代次。

本步声明：`type MainWorkspaceFact struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Manifest</code> | <code>uint64</code> | 当前文档清单 |
| <code>Structure</code> | <code>uint64</code> | 当前结构 |
| <code>Style</code> | <code>uint64</code> | 当前样式 |
| <code>MetadataControl</code> | <code>uint64</code> | 文档管理元数据修订 |
| <code>Product</code> | <code>string</code> | 生成此清单的动作种类 |
| <code>Parent</code> | <code>bool</code> | 是否存在父清单 |
| <code>Fixed</code> | <code>bool</code> | 原控制安装工件是否固定 |
| <code>Succeeded</code> | <code>bool</code> | 原业务是否真实成功 |
| <code>Installed</code> | <code>bool</code> | 原MAIN安装是否完成 |
| <code>OriginalMatched</code> | <code>bool</code> | 原始操作绑定是否一致 |
| <code>RecordedManifest</code> | <code>uint64</code> | 安装工件记录的清单 |
| <code>RecordedStructure</code> | <code>uint64</code> | 安装工件结构 |
| <code>RecordedStyle</code> | <code>uint64</code> | 安装工件样式 |
| <code>Control</code> | <code>uint64</code> | 实际安装的MAIN控制代次，不能直接以MetadataControl替代 |

<!-- write-step: 7 bytes=3156:3665 -->
````go
// MainWorkspaceFact只保存原安装的标量证据，不携带正文。管理元数据修订和
// 物理MAIN代次可以不同；只有改变MAIN并完成实际安装，才产生新的物理代次。
type MainWorkspaceFact struct {
	Manifest, Structure, Style, MetadataControl                 uint64
	Product                                                     string
	Parent, Fixed, Succeeded, Installed, OriginalMatched        bool
	RecordedManifest, RecordedStructure, RecordedStyle, Control uint64
}

````

## 第 8 步：ResolveMainWorkspace

插入位置：第 7 步代码之后，紧接追加；完成稿约第 103 行。

本步声明：`func ResolveMainWorkspace(fact MainWorkspaceFact) (uint64, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>fact</code> | <code>MainWorkspaceFact</code> | 使用 <code>MainWorkspaceFact</code> 类型；其定义处的职责：MainWorkspaceFact只保存原安装的标量证据，不携带正文。管理元数据修订和 物理MAIN代次可以不同；只有改变MAIN并完成实际安装，才产生新的物理代次。<br>在此声明中的实际使用：<code>fact.Manifest</code>；<code>fact.Structure</code>；<code>fact.Style</code>；<code>fact.MetadataControl</code><br>另有 10 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>uint64</code> | 实际返回表达式：<code>0</code>；<code>1</code>；<code>fact.Control</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>unknown</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>fact.Manifest == 0 &#124;&#124; fact.Structure == 0 &#124;&#124; fact.Style == 0 &#124;&#124; fact.MetadataControl == 0</code>。条件成立时执行 <code>return 0, unknown</code>。当前函数在这里结束。
- 检查 <code>fact.Product == &quot;INITIAL&quot;</code>。
- 检查 <code>fact.Parent &#124;&#124; fact.Fixed &#124;&#124; fact.Control != 0</code>。条件成立时执行 <code>return 0, unknown</code>。当前函数在这里结束。
- 此处通过 <code>return 1, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!fact.Parent &#124;&#124; !fact.Fixed &#124;&#124; !fact.Succeeded &#124;&#124; !fact.Installed &#124;&#124; !fact.OriginalMatched &#124;&#124; 		fact.RecordedManifest != fact.Manifest &#124;&#124; fact.RecordedStructure != fact.Structure &#124; …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return 0, unknown</code>。当前函数在这里结束。
- 此处通过 <code>return fact.Control, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 8 bytes=3665:4706 -->
````go
func ResolveMainWorkspace(fact MainWorkspaceFact) (uint64, error) {
	unknown := Violation(TemporarilyUnavailable, "MAIN_WORKSPACE_BINDING_UNKNOWN")
	if fact.Manifest == 0 || fact.Structure == 0 || fact.Style == 0 || fact.MetadataControl == 0 {
		return 0, unknown
	}
	if fact.Product == "INITIAL" {
		// INITIAL来自文档创建的固定空基线；不能把别的缺失安装事实伪装成这个分支。
		if fact.Parent || fact.Fixed || fact.Control != 0 {
			return 0, unknown
		}
		return 1, nil
	}
	switch fact.Product {
	case "MAIN_LOCK", "STYLE_CHANGE", "CHAPTER_CREATE", "STRUCTURE_CHANGE", "MERGE":
	default:
		return 0, unknown
	}
	if !fact.Parent || !fact.Fixed || !fact.Succeeded || !fact.Installed || !fact.OriginalMatched ||
		fact.RecordedManifest != fact.Manifest || fact.RecordedStructure != fact.Structure || fact.RecordedStyle != fact.Style ||
		fact.Control == 0 || fact.Control > fact.MetadataControl {
		return 0, unknown
	}
	return fact.Control, nil
}

// structure.go：保留原实现，按业务合并文件。

````

## 第 9 步：ParseStructureChange

插入位置：第 8 步代码之后，紧接追加；完成稿约第 130 行。

本步声明：`func ParseStructureChange(input dto.StructureChange) (MainControlIntent, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>input</code> | <code>dto.StructureChange</code> | 在此声明中的实际使用：<code>parseStructureChange(input, false)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>MainControlIntent</code> | 使用 <code>MainControlIntent</code> 类型；其定义处的职责：MainControlIntent固定管理动作；它不借用审核请求，也不隐含任何批准或支线接纳。<br>由 <code>parseStructureChange(input, false)</code> 的第 1 个返回项原样转交。 |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>由 <code>parseStructureChange(input, false)</code> 的第 2 个返回项原样转交。 |

**跟着本段代码走一遍：**

- 此处通过 <code>return parseStructureChange(input, false)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=4706:4834 -->
````go
func ParseStructureChange(input dto.StructureChange) (MainControlIntent, error) {
	return parseStructureChange(input, false)
}

````

## 第 10 步：parseStructureChange

插入位置：第 9 步代码之后，紧接追加；完成稿约第 134 行。

本步声明：`func parseStructureChange(input dto.StructureChange, preview bool) (MainControlIntent, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>input</code> | <code>dto.StructureChange</code> | 在此声明中的实际使用：<code>input.OperationId</code>；<code>&amp;input</code>；<code>input.Changes</code>；<code>input.ExpectedStructureId</code><br>另有 3 处使用，继续按下方完整代码的顺序处理。 |
| <code>preview</code> | <code>bool</code> | 在此声明中的实际使用：<code>preview &amp;&amp; change.PreviewCutRef != nil</code>；<code>!preview</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>MainControlIntent</code> | 使用 <code>MainControlIntent</code> 类型；其定义处的职责：MainControlIntent固定管理动作；它不借用审核请求，也不隐含任何批准或支线接纳。<br>实际返回表达式：<code>intent</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>Violation(InvalidRequest, &quot;STRUCTURE_INPUT_INVALID&quot;)</code>；<code>err</code>；<code>Violation(InvalidRequest, &quot;CHAPTER_SELF_ANCHOR&quot;)</code>；<code>Violation(StateConflict, &quot;STRUCTURE_PREVIEW_REQUIRED&quot;)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>!ValidOperationID(input.OperationId) &#124;&#124; len(input.Changes) == 0 &#124;&#124; len(input.Changes) &gt; 100</code>。条件成立时执行 <code>return intent, Violation(InvalidRequest, &quot;STRUCTURE_INPUT_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return intent, err</code>。当前函数在这里结束。
- 检查 <code>change.Title == nil &#124;&#124; strings.TrimSpace(*change.Title) == &quot;&quot; &#124;&#124; !utf8.ValidString(*change.Title) &#124;&#124; utf8.RuneCountInString(*change.Title) &gt; 240 &#124;&#124; change.AfterChapterId != nil &#124;&#124; change.PreviewCutRef != nil</code>。条件成立时执行 <code>return intent, Violation(InvalidRequest, &quot;STRUCTURE_INPUT_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>change.Title != nil &#124;&#124; change.PreviewCutRef != nil</code>。条件成立时执行 <code>return intent, Violation(InvalidRequest, &quot;STRUCTURE_INPUT_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>change.AfterChapterId != nil</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return intent, err</code>。当前函数在这里结束。
- 检查 <code>*change.AfterChapterId == change.ChapterId</code>。条件成立时执行 <code>return intent, Violation(InvalidRequest, &quot;CHAPTER_SELF_ANCHOR&quot;)</code>。当前函数在这里结束。
- 检查 <code>change.Title != nil &#124;&#124; change.AfterChapterId != nil &#124;&#124; (preview &amp;&amp; change.PreviewCutRef != nil)</code>。条件成立时执行 <code>return intent, Violation(InvalidRequest, &quot;STRUCTURE_INPUT_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>!preview &amp;&amp; (change.PreviewCutRef == nil &#124;&#124; !ValidOperationID(*change.PreviewCutRef))</code>。条件成立时执行 <code>return intent, Violation(StateConflict, &quot;STRUCTURE_PREVIEW_REQUIRED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return intent, Violation(InvalidRequest, &quot;STRUCTURE_INPUT_INVALID&quot;)</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!preview &amp;&amp; HasStructureRangeChange(input)</code>。

<!-- write-step: 10 bytes=4834:6838 -->
````go
func parseStructureChange(input dto.StructureChange, preview bool) (MainControlIntent, error) {
	intent := MainControlIntent{Operation: input.OperationId, Kind: "STRUCTURE_CHANGE", Structure: &input}
	if !ValidOperationID(input.OperationId) || len(input.Changes) == 0 || len(input.Changes) > 100 {
		return intent, Violation(InvalidRequest, "STRUCTURE_INPUT_INVALID")
	}
	if _, err := CanonicalID(input.ExpectedStructureId); err != nil {
		return intent, err
	}
	var err error
	if intent.Control, err = CanonicalRevision(input.ExpectedControlRevision); err != nil {
		return intent, err
	}
	for _, change := range input.Changes {
		if _, err = CanonicalID(change.ChapterId); err != nil {
			return intent, err
		}
		switch change.Type {
		case "RENAME":
			if change.Title == nil || strings.TrimSpace(*change.Title) == "" || !utf8.ValidString(*change.Title) || utf8.RuneCountInString(*change.Title) > 240 || change.AfterChapterId != nil || change.PreviewCutRef != nil {
				return intent, Violation(InvalidRequest, "STRUCTURE_INPUT_INVALID")
			}
		case "MOVE":
			if change.Title != nil || change.PreviewCutRef != nil {
				return intent, Violation(InvalidRequest, "STRUCTURE_INPUT_INVALID")
			}
			if change.AfterChapterId != nil {
				if _, err = CanonicalID(*change.AfterChapterId); err != nil {
					return intent, err
				}
				if *change.AfterChapterId == change.ChapterId {
					return intent, Violation(InvalidRequest, "CHAPTER_SELF_ANCHOR")
				}
			}
		case "SPLIT_LEVEL2", "JOIN_CHAPTER", "REMOVE":
			if change.Title != nil || change.AfterChapterId != nil || (preview && change.PreviewCutRef != nil) {
				return intent, Violation(InvalidRequest, "STRUCTURE_INPUT_INVALID")
			}
			if !preview && (change.PreviewCutRef == nil || !ValidOperationID(*change.PreviewCutRef)) {
				return intent, Violation(StateConflict, "STRUCTURE_PREVIEW_REQUIRED")
			}
		default:
			return intent, Violation(InvalidRequest, "STRUCTURE_INPUT_INVALID")
		}
	}
	if !preview && HasStructureRangeChange(input) {
````

## 第 11 步：parseStructureChange（接着实现）

插入位置：第 10 步代码之后，紧接追加；完成稿约第 179 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return intent, err</code>。当前函数在这里结束。
- 此处通过 <code>return intent, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=6838:6957 -->
````go
		if _, err := StructureConfirmationReference(input); err != nil {
			return intent, err
		}
	}
	return intent, nil
}

````

## 第 12 步：ApplyStableStructureChanges

插入位置：第 11 步代码之后，紧接追加；完成稿约第 186 行。

纯目录动作移动完整章组。复制条目后修改，不写穿不可变历史，也不改变孩子的局部顺序和单元身份。

本步声明：`func ApplyStableStructureChanges(entries []dto.StructureEntry, input dto.StructureChange) ([]dto.StructureEntry, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>entries</code> | <code>&#91;&#93;dto.StructureEntry</code> | 在此声明中的实际使用：<code>len(entries)</code> |
| <code>input</code> | <code>dto.StructureChange</code> | 在此声明中的实际使用：<code>HasStructureRangeChange(input)</code>；<code>ParseStructureChange(input)</code>；<code>input.Changes</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;dto.StructureEntry</code> | 实际返回表达式：<code>nil</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>Violation(StateConflict, &quot;STRUCTURE_PREVIEW_REQUIRED&quot;)</code>；<code>err</code>；<code>Violation(StateConflict, &quot;STRUCTURE_SHAPE_UNKNOWN&quot;)</code>；<code>Violation(StateConflict, &quot;CHAPTER_CHANGED&quot;)</code><br>另有 2 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>HasStructureRangeChange(input)</code>。条件成立时执行 <code>return nil, Violation(StateConflict, &quot;STRUCTURE_PREVIEW_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>entry.ParentNodeId == nil &amp;&amp; entry.Kind == &quot;CHAPTER&quot;</code>。
- 检查 <code>exists</code>。条件成立时执行 <code>return nil, Violation(StateConflict, &quot;STRUCTURE_SHAPE_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>entry.ParentNodeId == nil &amp;&amp; entry.Kind == &quot;CHAPTER&quot;</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>!exists &#124;&#124; entry.ParentNodeId == nil &#124;&#124; *entry.ParentNodeId != group&#91;0&#93;.NodeId</code>。条件成立时执行 <code>return nil, Violation(StateConflict, &quot;STRUCTURE_SHAPE_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return roots&#91;i&#93;.SortOrder &lt; roots&#91;j&#93;.SortOrder</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!exists</code>。条件成立时执行 <code>return nil, Violation(StateConflict, &quot;CHAPTER_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>change.Type == &quot;RENAME&quot;</code>。

<!-- write-step: 12 bytes=6957:8679 -->
````go
// 纯目录动作移动完整章组。复制条目后修改，不写穿不可变历史，也不改变孩子的局部顺序和单元身份。
func ApplyStableStructureChanges(entries []dto.StructureEntry, input dto.StructureChange) ([]dto.StructureEntry, error) {
	if HasStructureRangeChange(input) {
		return nil, Violation(StateConflict, "STRUCTURE_PREVIEW_REQUIRED")
	}
	if _, err := ParseStructureChange(input); err != nil {
		return nil, err
	}
	groups := make(map[string][]dto.StructureEntry)
	roots := make([]dto.StructureEntry, 0)
	for _, entry := range entries {
		if entry.ParentNodeId == nil && entry.Kind == "CHAPTER" {
			if _, exists := groups[entry.ChapterId]; exists {
				return nil, Violation(StateConflict, "STRUCTURE_SHAPE_UNKNOWN")
			}
			groups[entry.ChapterId] = []dto.StructureEntry{entry}
			roots = append(roots, entry)
		}
	}
	for _, entry := range entries {
		if entry.ParentNodeId == nil && entry.Kind == "CHAPTER" {
			continue
		}
		group, exists := groups[entry.ChapterId]
		if !exists || entry.ParentNodeId == nil || *entry.ParentNodeId != group[0].NodeId {
			return nil, Violation(StateConflict, "STRUCTURE_SHAPE_UNKNOWN")
		}
		groups[entry.ChapterId] = append(group, entry)
	}
	sort.SliceStable(roots, func(i, j int) bool { return roots[i].SortOrder < roots[j].SortOrder })
	order := make([]string, len(roots))
	for i, root := range roots {
		order[i] = root.ChapterId
	}
	for _, change := range input.Changes {
		group, exists := groups[change.ChapterId]
		if !exists {
			return nil, Violation(StateConflict, "CHAPTER_CHANGED")
		}
		if change.Type == "RENAME" {
			group[0].Title = *change.Title
			continue
		}
		next := make([]string, 0, len(order))
		for _, id := range order {
````

## 第 13 步：ApplyStableStructureChanges（接着实现）

插入位置：第 12 步代码之后，紧接追加；完成稿约第 231 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>id != change.ChapterId</code>。
- 检查 <code>change.AfterChapterId != nil</code>。
- 检查 <code>id == *change.AfterChapterId</code>。
- 检查 <code>index &lt; 0</code>。条件成立时执行 <code>return nil, Violation(StateConflict, &quot;CHAPTER_ANCHOR_CHANGED&quot;)</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return group&#91;a+1&#93;.SortOrder &lt; group&#91;b+1&#93;.SortOrder</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=8679:9587 -->
````go
			if id != change.ChapterId {
				next = append(next, id)
			}
		}
		// 无after锚点表示放到首位；有锚点则紧随该完整章组。
		index := 0
		if change.AfterChapterId != nil {
			index = -1
			for i, id := range next {
				if id == *change.AfterChapterId {
					index = i + 1
					break
				}
			}
			if index < 0 {
				return nil, Violation(StateConflict, "CHAPTER_ANCHOR_CHANGED")
			}
		}
		next = append(next, "")
		copy(next[index+1:], next[index:])
		next[index] = change.ChapterId
		order = next
	}
	result := make([]dto.StructureEntry, 0, len(entries))
	for i, id := range order {
		group := groups[id]
		group[0].SortOrder = int64(i + 1)
		sort.SliceStable(group[1:], func(a, b int) bool { return group[a+1].SortOrder < group[b+1].SortOrder })
		result = append(result, group...)
	}
	return result, nil
}

// structure_confirmation.go：保留原实现，按业务合并文件。

````

## 第 14 步：HasStructureRangeChange

插入位置：第 13 步代码之后，紧接追加；完成稿约第 266 行。

本步声明：`func HasStructureRangeChange(input dto.StructureChange) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>input</code> | <code>dto.StructureChange</code> | 在此声明中的实际使用：<code>input.Changes</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>true</code>；<code>false</code> |

**跟着本段代码走一遍：**

- 检查 <code>change.Type == &quot;SPLIT_LEVEL2&quot; &#124;&#124; change.Type == &quot;JOIN_CHAPTER&quot; &#124;&#124; change.Type == &quot;REMOVE&quot;</code>。条件成立时执行 <code>return true</code>。当前函数在这里结束。
- 此处通过 <code>return false</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 14 bytes=9587:9826 -->
````go
func HasStructureRangeChange(input dto.StructureChange) bool {
	for _, change := range input.Changes {
		if change.Type == "SPLIT_LEVEL2" || change.Type == "JOIN_CHAPTER" || change.Type == "REMOVE" {
			return true
		}
	}
	return false
}

````

## 第 15 步：StructureConfirmationReference

插入位置：第 14 步代码之后，紧接追加；完成稿约第 275 行。

全部范围动作共享同一份完整预览；不能拼接多个操作各自看见的目录、正文与切点。

本步声明：`func StructureConfirmationReference(input dto.StructureChange) (string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>input</code> | <code>dto.StructureChange</code> | 在此声明中的实际使用：<code>input.OperationId</code>；<code>input.Changes</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;&quot;</code>；<code>reference</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>Violation(InvalidRequest, &quot;STRUCTURE_INPUT_INVALID&quot;)</code>；<code>Violation(StateConflict, &quot;STRUCTURE_PREVIEW_REQUIRED&quot;)</code>；<code>Violation(StateConflict, &quot;STRUCTURE_PREVIEW_MISMATCH&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>!ValidOperationID(input.OperationId)</code>。条件成立时执行 <code>return &quot;&quot;, Violation(InvalidRequest, &quot;STRUCTURE_INPUT_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>change.PreviewCutRef == nil &#124;&#124; !ValidOperationID(*change.PreviewCutRef)</code>。条件成立时执行 <code>return &quot;&quot;, Violation(StateConflict, &quot;STRUCTURE_PREVIEW_REQUIRED&quot;)</code>。当前函数在这里结束。
- 检查 <code>reference != &quot;&quot; &amp;&amp; reference != *change.PreviewCutRef</code>。条件成立时执行 <code>return &quot;&quot;, Violation(StateConflict, &quot;STRUCTURE_PREVIEW_MISMATCH&quot;)</code>。当前函数在这里结束。
- 检查 <code>change.PreviewCutRef != nil</code>。条件成立时执行 <code>return &quot;&quot;, Violation(InvalidRequest, &quot;STRUCTURE_INPUT_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>reference == &quot;&quot;</code>。条件成立时执行 <code>return &quot;&quot;, Violation(StateConflict, &quot;STRUCTURE_PREVIEW_REQUIRED&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return reference, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 15 bytes=9826:10841 -->
````go
// 全部范围动作共享同一份完整预览；不能拼接多个操作各自看见的目录、正文与切点。
func StructureConfirmationReference(input dto.StructureChange) (string, error) {
	if !ValidOperationID(input.OperationId) {
		return "", Violation(InvalidRequest, "STRUCTURE_INPUT_INVALID")
	}
	reference := ""
	for _, change := range input.Changes {
		switch change.Type {
		case "SPLIT_LEVEL2", "JOIN_CHAPTER", "REMOVE":
			if change.PreviewCutRef == nil || !ValidOperationID(*change.PreviewCutRef) {
				return "", Violation(StateConflict, "STRUCTURE_PREVIEW_REQUIRED")
			}
			if reference != "" && reference != *change.PreviewCutRef {
				return "", Violation(StateConflict, "STRUCTURE_PREVIEW_MISMATCH")
			}
			reference = *change.PreviewCutRef
		default:
			if change.PreviewCutRef != nil {
				return "", Violation(InvalidRequest, "STRUCTURE_INPUT_INVALID")
			}
		}
	}
	if reference == "" {
		return "", Violation(StateConflict, "STRUCTURE_PREVIEW_REQUIRED")
	}
	return reference, nil
}

````

## 第 16 步：ValidateStructureConfirmation

插入位置：第 15 步代码之后，紧接追加；完成稿约第 303 行。

只去掉确认入口自己新增的原号/引用；动作集合、顺序、目录标题与期待修订均必须等于原预览。

本步声明：`func ValidateStructureConfirmation(input dto.StructureChange, preview dto.StructurePreviewCreate) error`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>input</code> | <code>dto.StructureChange</code> | 在此声明中的实际使用：<code>StructureConfirmationReference(input)</code>；<code>input.OperationId</code>；<code>input.ExpectedStructureId</code>；<code>input.ExpectedControlRevision</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>preview</code> | <code>dto.StructurePreviewCreate</code> | 在此声明中的实际使用：<code>preview.OperationId</code>；<code>ParseStructurePreview(preview)</code>；<code>json.Marshal(preview)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code>；<code>Violation(StateConflict, &quot;STRUCTURE_PREVIEW_MISMATCH&quot;)</code>；<code>Violation(TemporarilyUnavailable, &quot;STRUCTURE_PREVIEW_BINDING_UNKNOWN&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>reference != preview.OperationId &#124;&#124; input.OperationId == preview.OperationId</code>。条件成立时执行 <code>return Violation(StateConflict, &quot;STRUCTURE_PREVIEW_MISMATCH&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return Violation(TemporarilyUnavailable, &quot;STRUCTURE_PREVIEW_BINDING_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return err</code>。当前函数在这里结束。
- 检查 <code>!bytes.Equal(original, confirmed)</code>。条件成立时执行 <code>return Violation(StateConflict, &quot;STRUCTURE_PREVIEW_MISMATCH&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 16 bytes=10841:12229 -->
````go
// 只去掉确认入口自己新增的原号/引用；动作集合、顺序、目录标题与期待修订均必须等于原预览。
func ValidateStructureConfirmation(input dto.StructureChange, preview dto.StructurePreviewCreate) error {
	reference, err := StructureConfirmationReference(input)
	if err != nil {
		return err
	}
	if reference != preview.OperationId || input.OperationId == preview.OperationId {
		return Violation(StateConflict, "STRUCTURE_PREVIEW_MISMATCH")
	}
	if _, err = ParseStructurePreview(preview); err != nil {
		return Violation(TemporarilyUnavailable, "STRUCTURE_PREVIEW_BINDING_UNKNOWN")
	}
	normalized := dto.StructurePreviewCreate{
		OperationId:             preview.OperationId,
		ExpectedStructureId:     input.ExpectedStructureId,
		ExpectedControlRevision: input.ExpectedControlRevision,
		Changes:                 slices.Clone(input.Changes),
	}
	for i := range normalized.Changes {
		normalized.Changes[i].PreviewCutRef = nil
	}
	if _, err = ParseStructurePreview(normalized); err != nil {
		return err
	}
	original, err := json.Marshal(preview)
	if err != nil {
		return err
	}
	confirmed, err := json.Marshal(normalized)
	if err != nil {
		return err
	}
	if !bytes.Equal(original, confirmed) {
		return Violation(StateConflict, "STRUCTURE_PREVIEW_MISMATCH")
	}
	return nil
}

// structure_migration.go：保留原实现，按业务合并文件。

````

## 第 17 步：StructureMigration

插入位置：第 16 步代码之后，紧接追加；完成稿约第 343 行。

这是系统内部固定候选与范围证据，不复制浏览器HTTP输入，也不预先分配正式单元身份。

本步声明：`type StructureMigration struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Schema</code> | <code>string</code> | 结构迁移固定协议版本structure-migration-v1<br>序列化/校验标记：<code>`json:&quot;schema_version&quot;`</code> |
| <code>Changes</code> | <code>&#91;&#93;StructureMigrationChange</code> | 按原动作顺序固定的迁移变更<br>序列化/校验标记：<code>`json:&quot;changes&quot;`</code> |

<!-- write-step: 17 bytes=12229:12499 -->
````go
// 这是系统内部固定候选与范围证据，不复制浏览器HTTP输入，也不预先分配正式单元身份。
type StructureMigration struct {
	Schema  string                     `json:"schema_version"`
	Changes []StructureMigrationChange `json:"changes"`
}
````

## 第 18 步：StructureMigrationChange

插入位置：第 17 步代码之后，紧接追加；完成稿约第 348 行。

本步声明：`type StructureMigrationChange struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Type</code> | <code>string</code> | SPLIT_LEVEL2、JOIN_CHAPTER或REMOVE<br>序列化/校验标记：<code>`json:&quot;type&quot;`</code> |
| <code>Chapter</code> | <code>string</code> | 被操作的原章节ID<br>序列化/校验标记：<code>`json:&quot;chapter_id&quot;`</code> |
| <code>Sources</code> | <code>&#91;&#93;string</code> | 该章节完整源单元集合<br>序列化/校验标记：<code>`json:&quot;source_units&quot;`</code> |
| <code>Targets</code> | <code>&#91;&#93;StructureMigrationTarget</code> | 固定目标候选，REMOVE必须为空<br>序列化/校验标记：<code>`json:&quot;targets&quot;`</code> |

<!-- write-step: 18 bytes=12499:12757 -->
````go
type StructureMigrationChange struct {
	Type    string                     `json:"type"`
	Chapter string                     `json:"chapter_id"`
	Sources []string                   `json:"source_units"`
	Targets []StructureMigrationTarget `json:"targets"`
}
````

## 第 19 步：StructureMigrationTarget

插入位置：第 18 步代码之后，紧接追加；完成稿约第 354 行。

本步声明：`type StructureMigrationTarget struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Key</code> | <code>string</code> | 预览内唯一候选键，提交时映射新正式身份<br>序列化/校验标记：<code>`json:&quot;key&quot;`</code> |
| <code>Kind</code> | <code>string</code> | INTRODUCTION、SECTION或CHAPTER<br>序列化/校验标记：<code>`json:&quot;kind&quot;`</code> |
| <code>Title</code> | <code>string</code> | 候选目录标题<br>序列化/校验标记：<code>`json:&quot;title&quot;`</code> |
| <code>Main</code> | <code>json.RawMessage</code> | 仅主线来源构成的规范候选正文<br>序列化/校验标记：<code>`json:&quot;main_body&quot;`</code> |
| <code>Draft</code> | <code>json.RawMessage</code> | 包含未审核修改的候选草稿，不能标作已接纳MAIN<br>序列化/校验标记：<code>`json:&quot;draft_body&quot;`</code> |
| <code>Boundary</code> | <code>StructureMigrationBoundary</code> | 目标标题或空块的固定边界<br>序列化/校验标记：<code>`json:&quot;boundary&quot;`</code> |
| <code>Ranges</code> | <code>&#91;&#93;StructureMigrationRange</code> | 各原单元贡献的完整区间证据<br>序列化/校验标记：<code>`json:&quot;source_ranges&quot;`</code> |

<!-- write-step: 19 bytes=12757:13179 -->
````go
type StructureMigrationTarget struct {
	Key      string                     `json:"key"`
	Kind     string                     `json:"kind"`
	Title    string                     `json:"title"`
	Main     json.RawMessage            `json:"main_body"`
	Draft    json.RawMessage            `json:"draft_body"`
	Boundary StructureMigrationBoundary `json:"boundary"`
	Ranges   []StructureMigrationRange  `json:"source_ranges"`
}
````

## 第 20 步：StructureMigrationBoundary

插入位置：第 19 步代码之后，紧接追加；完成稿约第 363 行。

本步声明：`type StructureMigrationBoundary struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Heading</code> | <code>json.RawMessage</code> | 作为分割边界的实际二级标题节点JSON<br>序列化/校验标记：<code>`json:&quot;heading,omitempty&quot;`</code> |
| <code>EmptyBlock</code> | <code>string</code> | 空引言等候选使用的固定块ID<br>序列化/校验标记：<code>`json:&quot;empty_block_id,omitempty&quot;`</code> |

<!-- write-step: 20 bytes=13179:13339 -->
````go
type StructureMigrationBoundary struct {
	Heading    json.RawMessage `json:"heading,omitempty"`
	EmptyBlock string          `json:"empty_block_id,omitempty"`
}
````

## 第 21 步：StructureMigrationRange

插入位置：第 20 步代码之后，紧接追加；完成稿约第 367 行。

本步声明：`type StructureMigrationRange struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Source</code> | <code>string</code> | 原单元文本ID<br>序列化/校验标记：<code>`json:&quot;source_unit_id&quot;`</code> |
| <code>Scope</code> | <code>string</code> | 原范围是WHOLE_UNIT还是INTERVAL<br>序列化/校验标记：<code>`json:&quot;source_scope&quot;`</code> |
| <code>Start</code> | <code>*string</code> | 源起点标题ID，nil为开头<br>序列化/校验标记：<code>`json:&quot;start_boundary&quot;`</code> |
| <code>End</code> | <code>*string</code> | 源终点标题ID，nil为结尾<br>序列化/校验标记：<code>`json:&quot;end_boundary&quot;`</code> |
| <code>MainBlocks</code> | <code>&#91;&#93;string</code> | 原MAIN实际贡献块ID<br>序列化/校验标记：<code>`json:&quot;main_block_ids&quot;`</code> |
| <code>DraftBlocks</code> | <code>&#91;&#93;string</code> | 原草稿实际贡献块ID<br>序列化/校验标记：<code>`json:&quot;draft_block_ids&quot;`</code> |
| <code>TargetScope</code> | <code>string</code> | 目标范围类别<br>序列化/校验标记：<code>`json:&quot;target_scope,omitempty&quot;`</code> |
| <code>TargetStart</code> | <code>*string</code> | 合章后目标对应区间起点<br>序列化/校验标记：<code>`json:&quot;target_start_boundary&quot;`</code> |
| <code>TargetEnd</code> | <code>*string</code> | 合章后目标对应区间终点<br>序列化/校验标记：<code>`json:&quot;target_end_boundary&quot;`</code> |

<!-- write-step: 21 bytes=13339:13810 -->
````go
type StructureMigrationRange struct {
	Source      string   `json:"source_unit_id"`
	Scope       string   `json:"source_scope"`
	Start       *string  `json:"start_boundary"`
	End         *string  `json:"end_boundary"`
	MainBlocks  []string `json:"main_block_ids"`
	DraftBlocks []string `json:"draft_block_ids"`
	TargetScope string   `json:"target_scope,omitempty"`
	TargetStart *string  `json:"target_start_boundary"`
	TargetEnd   *string  `json:"target_end_boundary"`
}
````

## 第 22 步：StructureMigrationSource

插入位置：第 21 步代码之后，紧接追加；完成稿约第 378 行。

本步声明：`type StructureMigrationSource struct{ Unit, Chapter uint64 }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Unit</code> | <code>uint64</code> | 真实源单元ID |
| <code>Chapter</code> | <code>uint64</code> | 真实所属章ID，防候选伪造跨章来源 |

<!-- write-step: 22 bytes=13810:13872 -->
````go
type StructureMigrationSource struct{ Unit, Chapter uint64 }

````

## 第 23 步：ParseStructureMigration

插入位置：第 22 步代码之后，紧接追加；完成稿约第 380 行。

本步声明：`func ParseStructureMigration(raw []byte, input dto.StructurePreviewCreate, sources []StructureMigrationSource, validator *codec.Validator) (StructureMigration, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>len(raw)</code>；<code>codec.Decode(raw)</code>；<code>json.Unmarshal(raw, &amp;migration)</code> |
| <code>input</code> | <code>dto.StructurePreviewCreate</code> | 在此声明中的实际使用：<code>input.Changes</code> |
| <code>sources</code> | <code>&#91;&#93;StructureMigrationSource</code> | 预览时固定的原单元、章节、主线与草稿来源事实；用来逐项验证迁移映射，不能只信Node返回的目标列表。 |
| <code>validator</code> | <code>*codec.Validator</code> | 冻结协议的结构校验器，先限制请求形状，再由业务核验身份、权限、版本和事务条件。<br>在此声明中的实际使用：<code>validator == nil</code>；<code>canonicalStructureCandidateBody(validator, target.Main, false)</code>；<code>canonicalStructureCandidateBody(validator, target.Draft, true)</code>；<code>structureMigrationHeadingID(validator, target.Boundary.Heading)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>StructureMigration</code> | 使用 <code>StructureMigration</code> 类型；其定义处的职责：这是系统内部固定候选与范围证据，不复制浏览器HTTP输入，也不预先分配正式单元身份。<br>实际返回表达式：<code>migration</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>invalid</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>validator == nil &#124;&#124; len(raw) &gt; 24&lt;&lt;20</code>。条件成立时执行 <code>return migration, invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return migration, invalid</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(raw, &amp;migration) != nil &#124;&#124; migration.Schema != &quot;structure-migration-v1&quot;</code>。条件成立时执行 <code>return migration, invalid</code>。当前函数在这里结束。
- 检查 <code>source.Unit == 0 &#124;&#124; source.Chapter == 0 &#124;&#124; seen&#91;source.Unit&#93;</code>。条件成立时执行 <code>return migration, invalid</code>。当前函数在这里结束。
- 检查 <code>chapters&#91;chapter&#93; == nil</code>。
- 检查 <code>intent.Type != &quot;SPLIT_LEVEL2&quot; &amp;&amp; intent.Type != &quot;JOIN_CHAPTER&quot; &amp;&amp; intent.Type != &quot;REMOVE&quot;</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>index &gt;= len(migration.Changes)</code>。条件成立时执行 <code>return migration, invalid</code>。当前函数在这里结束。
- 检查 <code>change.Type != intent.Type &#124;&#124; change.Chapter != intent.ChapterId &#124;&#124; !exactMigrationSources(change.Sources, chapters&#91;change.Chapter&#93;)</code>。条件成立时执行 <code>return migration, invalid</code>。当前函数在这里结束。
- 检查 <code>change.Type == &quot;REMOVE&quot;</code>。
- 检查 <code>len(change.Targets) != 0</code>。条件成立时执行 <code>return migration, invalid</code>。当前函数在这里结束。

<!-- write-step: 23 bytes=13872:15423 -->
````go
func ParseStructureMigration(raw []byte, input dto.StructurePreviewCreate, sources []StructureMigrationSource, validator *codec.Validator) (StructureMigration, error) {
	var migration StructureMigration
	invalid := Violation(TemporarilyUnavailable, "STRUCTURE_MIGRATION_UNKNOWN")
	if validator == nil || len(raw) > 24<<20 {
		return migration, invalid
	}
	if _, err := codec.Decode(raw); err != nil {
		return migration, invalid
	}
	if json.Unmarshal(raw, &migration) != nil || migration.Schema != "structure-migration-v1" {
		return migration, invalid
	}
	chapters := map[string]map[string]bool{}
	seen := map[uint64]bool{}
	for _, source := range sources {
		if source.Unit == 0 || source.Chapter == 0 || seen[source.Unit] {
			return migration, invalid
		}
		seen[source.Unit] = true
		chapter := TextID(source.Chapter)
		if chapters[chapter] == nil {
			chapters[chapter] = map[string]bool{}
		}
		chapters[chapter][TextID(source.Unit)] = true
	}
	index := 0
	keys := map[string]bool{}
	for _, intent := range input.Changes {
		if intent.Type != "SPLIT_LEVEL2" && intent.Type != "JOIN_CHAPTER" && intent.Type != "REMOVE" {
			continue
		}
		if index >= len(migration.Changes) {
			return migration, invalid
		}
		change := &migration.Changes[index]
		index++
		if change.Type != intent.Type || change.Chapter != intent.ChapterId || !exactMigrationSources(change.Sources, chapters[change.Chapter]) {
			return migration, invalid
		}
		if change.Type == "REMOVE" {
			if len(change.Targets) != 0 {
				return migration, invalid
			}
			continue
		}
````

## 第 24 步：ParseStructureMigration（接着实现）

插入位置：第 23 步代码之后，紧接追加；完成稿约第 425 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>len(change.Targets) == 0 &#124;&#124; len(change.Targets) &gt; 10000 &#124;&#124; (change.Type == &quot;SPLIT_LEVEL2&quot; &amp;&amp; len(change.Sources) != 1) &#124;&#124; (change.Type == &quot;JOIN_CHAPTER&quot; &amp;&amp; len(change.Targets) != 1)</code>。条件成立时执行 <code>return migration, invalid</code>。当前函数在这里结束。
- 检查 <code>target.Key == &quot;&quot; &#124;&#124; len(target.Key) &gt; 160 &#124;&#124; keys&#91;target.Key&#93; &#124;&#124; !utf8.ValidString(target.Title) &#124;&#124; utf8.RuneCountInString(target.Title) &gt; 240</code>。条件成立时执行 <code>return migration, invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return migration, invalid</code>。当前函数在这里结束。
- 检查 <code>change.Type == &quot;JOIN_CHAPTER&quot;</code>。
- 检查 <code>target.Kind != &quot;CHAPTER&quot; &#124;&#124; len(target.Boundary.Heading) != 0 &#124;&#124; !exactMigrationRanges(target.Ranges, change.Sources, true) &#124;&#124; !validJoinedTargetIntervals(target)</code>。条件成立时执行 <code>return migration, invalid</code>。当前函数在这里结束。
- 检查 <code>!exactMigrationRanges(target.Ranges, change.Sources, false)</code>。条件成立时执行 <code>return migration, invalid</code>。当前函数在这里结束。
- 检查 <code>target.Kind == &quot;INTRODUCTION&quot;</code>。
- 检查 <code>i != 0 &#124;&#124; target.Title != &quot;&quot; &#124;&#124; len(target.Boundary.Heading) != 0 &#124;&#124; target.Ranges&#91;0&#93;.Start != nil</code>。条件成立时执行 <code>return migration, invalid</code>。当前函数在这里结束。
- 检查 <code>target.Kind == &quot;SECTION&quot;</code>。
- 检查 <code>err != nil &#124;&#124; target.Title == &quot;&quot; &#124;&#124; target.Ranges&#91;0&#93;.Start == nil &#124;&#124; *target.Ranges&#91;0&#93;.Start != id</code>。条件成立时执行 <code>return migration, invalid</code>。当前函数在这里结束。
- 检查 <code>i &gt; 0</code>。
- 检查 <code>previous == nil &#124;&#124; target.Ranges&#91;0&#93;.Start == nil &#124;&#124; *previous != *target.Ranges&#91;0&#93;.Start</code>。条件成立时执行 <code>return migration, invalid</code>。当前函数在这里结束。

<!-- write-step: 24 bytes=15423:17317 -->
````go
		if len(change.Targets) == 0 || len(change.Targets) > 10000 || (change.Type == "SPLIT_LEVEL2" && len(change.Sources) != 1) || (change.Type == "JOIN_CHAPTER" && len(change.Targets) != 1) {
			return migration, invalid
		}
		sections := 0
		for i := range change.Targets {
			target := &change.Targets[i]
			if target.Key == "" || len(target.Key) > 160 || keys[target.Key] || !utf8.ValidString(target.Title) || utf8.RuneCountInString(target.Title) > 240 {
				return migration, invalid
			}
			keys[target.Key] = true
			var err error
			if target.Main, err = canonicalStructureCandidateBody(validator, target.Main, false); err != nil {
				return migration, invalid
			}
			if target.Draft, err = canonicalStructureCandidateBody(validator, target.Draft, true); err != nil {
				return migration, invalid
			}
			if change.Type == "JOIN_CHAPTER" {
				if target.Kind != "CHAPTER" || len(target.Boundary.Heading) != 0 || !exactMigrationRanges(target.Ranges, change.Sources, true) || !validJoinedTargetIntervals(target) {
					return migration, invalid
				}
				continue
			}
			if !exactMigrationRanges(target.Ranges, change.Sources, false) {
				return migration, invalid
			}
			if target.Kind == "INTRODUCTION" {
				if i != 0 || target.Title != "" || len(target.Boundary.Heading) != 0 || target.Ranges[0].Start != nil {
					return migration, invalid
				}
			} else if target.Kind == "SECTION" {
				id, err := structureMigrationHeadingID(validator, target.Boundary.Heading)
				if err != nil || target.Title == "" || target.Ranges[0].Start == nil || *target.Ranges[0].Start != id {
					return migration, invalid
				}
				sections++
			} else {
				return migration, invalid
			}
			if i > 0 {
				previous := change.Targets[i-1].Ranges[0].End
				if previous == nil || target.Ranges[0].Start == nil || *previous != *target.Ranges[0].Start {
					return migration, invalid
				}
			}
		}
````

## 第 25 步：ParseStructureMigration（接着实现）

插入位置：第 24 步代码之后，紧接追加；完成稿约第 471 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>change.Type == &quot;SPLIT_LEVEL2&quot; &amp;&amp; (sections == 0 &#124;&#124; change.Targets&#91;len(change.Targets)-1&#93;.Ranges&#91;0&#93;.End != nil)</code>。条件成立时执行 <code>return migration, invalid</code>。当前函数在这里结束。
- 检查 <code>index == 0 &#124;&#124; index != len(migration.Changes)</code>。条件成立时执行 <code>return migration, invalid</code>。当前函数在这里结束。
- 此处通过 <code>return migration, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 25 bytes=17317:17580 -->
````go
		if change.Type == "SPLIT_LEVEL2" && (sections == 0 || change.Targets[len(change.Targets)-1].Ranges[0].End != nil) {
			return migration, invalid
		}
	}
	if index == 0 || index != len(migration.Changes) {
		return migration, invalid
	}
	return migration, nil
}

````

## 第 26 步：validJoinedTargetIntervals

插入位置：第 25 步代码之后，紧接追加；完成稿约第 481 行。

本步声明：`func validJoinedTargetIntervals(target *StructureMigrationTarget) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>target</code> | <code>*StructureMigrationTarget</code> | 在此声明中的实际使用：<code>target.Ranges</code>；<code>target.Main</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>true</code>；<code>false</code> |

**跟着本段代码走一遍：**

- 检查 <code>segment.TargetScope != &quot;&quot;</code>。
- 检查 <code>!withInterval</code>。条件成立时执行 <code>return true</code>。当前函数在这里结束。
- 检查 <code>!ok &#124;&#124; len(heads) == 0</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>segment.TargetScope != &quot;INTERVAL&quot;</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>!ok &#124;&#124; start &lt;= previous &#124;&#124; (i &gt; 0 &amp;&amp; segment.TargetStart == nil) &#124;&#124; (i == 0 &amp;&amp; start &gt; 0)</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>i == len(target.Ranges)-1</code>。
- 检查 <code>segment.TargetEnd != nil</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>segment.TargetEnd == nil &#124;&#124; !equalMigrationBoundary(segment.TargetEnd, target.Ranges&#91;i+1&#93;.TargetStart)</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 此处通过 <code>return true</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 26 bytes=17580:18614 -->
````go
func validJoinedTargetIntervals(target *StructureMigrationTarget) bool {
	// 首轮旧固定候选没有目标区间：允许沿原事实完成安装，历史读权限只能要求整个目标覆盖。
	withInterval := false
	for _, segment := range target.Ranges {
		if segment.TargetScope != "" {
			withInterval = true
		}
	}
	if !withInterval {
		return true
	}
	heads, ok := StructureTopLevel2IDs(target.Main)
	if !ok || len(heads) == 0 {
		return false
	}
	previous := -2
	for i, segment := range target.Ranges {
		if segment.TargetScope != "INTERVAL" {
			return false
		}
		start, ok := historicalBoundaryIndex(heads, segment.TargetStart, false)
		if !ok || start <= previous || (i > 0 && segment.TargetStart == nil) || (i == 0 && start > 0) {
			return false
		}
		previous = start
		if i == len(target.Ranges)-1 {
			if segment.TargetEnd != nil {
				return false
			}
		} else if segment.TargetEnd == nil || !equalMigrationBoundary(segment.TargetEnd, target.Ranges[i+1].TargetStart) {
			return false
		}
	}
	return true
}

````

## 第 27 步：equalMigrationBoundary

插入位置：第 26 步代码之后，紧接追加；完成稿约第 517 行。

本步声明：`func equalMigrationBoundary(a, b *string) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>a</code> | <code>*string</code> | 在此声明中的实际使用：<code>a != nil</code>；<code>*a == *b</code> |
| <code>b</code> | <code>*string</code> | 在此声明中的实际使用：<code>b != nil</code>；<code>*a == *b</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>a != nil &amp;&amp; b != nil &amp;&amp; *a == *b</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return a != nil &amp;&amp; b != nil &amp;&amp; *a == *b</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 27 bytes=18614:18706 -->
````go
func equalMigrationBoundary(a, b *string) bool { return a != nil && b != nil && *a == *b }

````

## 第 28 步：StructureTopLevel2IDs

插入位置：第 27 步代码之后，紧接追加；完成稿约第 519 行。

授权区间只读取不可变整份MAIN中的顶层二级标题身份，拒绝重复和不完整的正文。

本步声明：`func StructureTopLevel2IDs(raw []byte) ([]string, bool)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>codec.Decode(raw)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;string</code> | 实际返回表达式：<code>nil</code>；<code>heads</code> |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, false</code>。当前函数在这里结束。
- 检查 <code>!ok &#124;&#124; root&#91;&quot;type&quot;&#93; != &quot;doc&quot;</code>。条件成立时执行 <code>return nil, false</code>。当前函数在这里结束。
- 检查 <code>!ok</code>。条件成立时执行 <code>return nil, false</code>。当前函数在这里结束。
- 检查 <code>node&#91;&quot;type&quot;&#93; != &quot;heading&quot; &#124;&#124; attrs&#91;&quot;level&quot;&#93; != json.Number(&quot;2&quot;)</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>!ok &#124;&#124; id == &quot;&quot; &#124;&#124; seen&#91;id&#93;</code>。条件成立时执行 <code>return nil, false</code>。当前函数在这里结束。
- 此处通过 <code>return heads, true</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 28 bytes=18706:19524 -->
````go
// 授权区间只读取不可变整份MAIN中的顶层二级标题身份，拒绝重复和不完整的正文。
func StructureTopLevel2IDs(raw []byte) ([]string, bool) {
	value, err := codec.Decode(raw)
	if err != nil {
		return nil, false
	}
	root, ok := value.(map[string]any)
	if !ok || root["type"] != "doc" {
		return nil, false
	}
	heads, seen := []string{}, map[string]bool{}
	for _, item := range structureMigrationArray(root["content"]) {
		node, ok := item.(map[string]any)
		if !ok {
			return nil, false
		}
		attrs, _ := node["attrs"].(map[string]any)
		if node["type"] != "heading" || attrs["level"] != json.Number("2") {
			continue
		}
		id, ok := attrs["id"].(string)
		if !ok || id == "" || seen[id] {
			return nil, false
		}
		seen[id] = true
		heads = append(heads, id)
	}
	return heads, true
}

````

## 第 29 步：exactMigrationSources

插入位置：第 28 步代码之后，紧接追加；完成稿约第 549 行。

本步声明：`func exactMigrationSources(items []string, expected map[string]bool) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>items</code> | <code>&#91;&#93;string</code> | 在此声明中的实际使用：<code>len(items)</code> |
| <code>expected</code> | <code>map&#91;string&#93;bool</code> | 在此声明中的实际使用：<code>len(expected)</code>；<code>!expected&#91;id&#93;</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(items) == 0 &#124;&#124; len(items) != len(expected)</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>!expected&#91;id&#93; &#124;&#124; seen&#91;id&#93;</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 此处通过 <code>return true</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 29 bytes=19524:19816 -->
````go
func exactMigrationSources(items []string, expected map[string]bool) bool {
	if len(items) == 0 || len(items) != len(expected) {
		return false
	}
	seen := map[string]bool{}
	for _, id := range items {
		if !expected[id] || seen[id] {
			return false
		}
		seen[id] = true
	}
	return true
}

````

## 第 30 步：exactMigrationRanges

插入位置：第 29 步代码之后，紧接追加；完成稿约第 563 行。

本步声明：`func exactMigrationRanges(ranges []StructureMigrationRange, sources []string, joining bool) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ranges</code> | <code>&#91;&#93;StructureMigrationRange</code> | 在此声明中的实际使用：<code>len(ranges)</code> |
| <code>sources</code> | <code>&#91;&#93;string</code> | 在此声明中的实际使用：<code>len(sources)</code>；<code>segment.Source != sources&#91;i&#93;</code> |
| <code>joining</code> | <code>bool</code> | 在此声明中的实际使用：<code>joining &amp;&amp; (segment.Start != nil &#124;&#124; segment.End != nil)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(ranges) != len(sources)</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>segment.Source != sources&#91;i&#93; &#124;&#124; segment.Scope != &quot;WHOLE_UNIT&quot; &#124;&#124; (joining &amp;&amp; (segment.Start != nil &#124;&#124; segment.End != nil))</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 此处通过 <code>return true</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 30 bytes=19816:20303 -->
````go
func exactMigrationRanges(ranges []StructureMigrationRange, sources []string, joining bool) bool {
	if len(ranges) != len(sources) {
		return false
	}
	for i, segment := range ranges {
		// WHOLE_UNIT绑定被固定的原完整来源；start/end界定这条迁移边的区间，不能把当前块列表当全覆盖。
		if segment.Source != sources[i] || segment.Scope != "WHOLE_UNIT" || (joining && (segment.Start != nil || segment.End != nil)) {
			return false
		}
	}
	return true
}

````

## 第 31 步：structureMigrationHeadingID

插入位置：第 30 步代码之后，紧接追加；完成稿约第 576 行。

本步声明：`func structureMigrationHeadingID(validator *codec.Validator, raw []byte) (string, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>validator</code> | <code>*codec.Validator</code> | 冻结协议的结构校验器，先限制请求形状，再由业务核验身份、权限、版本和事务条件。<br>在此声明中的实际使用：<code>validator.Body</code> |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>codec.Decode(raw)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;&quot;</code>；<code>id</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>invalid</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return &quot;&quot;, invalid</code>。当前函数在这里结束。
- 检查 <code>!ok &#124;&#124; node&#91;&quot;type&quot;&#93; != &quot;heading&quot;</code>。条件成立时执行 <code>return &quot;&quot;, invalid</code>。当前函数在这里结束。
- 检查 <code>!ok &#124;&#124; attrs&#91;&quot;level&quot;&#93; != json.Number(&quot;2&quot;) &#124;&#124; validator.Body(map&#91;string&#93;any{&quot;type&quot;: &quot;doc&quot;, &quot;content&quot;: &#91;&#93;any{node}}) != nil</code>。条件成立时执行 <code>return &quot;&quot;, invalid</code>。当前函数在这里结束。
- 检查 <code>!ok</code>。条件成立时执行 <code>return &quot;&quot;, invalid</code>。当前函数在这里结束。
- 此处通过 <code>return id, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 31 bytes=20303:20926 -->
````go
func structureMigrationHeadingID(validator *codec.Validator, raw []byte) (string, error) {
	invalid := Violation(TemporarilyUnavailable, "STRUCTURE_MIGRATION_UNKNOWN")
	value, err := codec.Decode(raw)
	if err != nil {
		return "", invalid
	}
	node, ok := value.(map[string]any)
	if !ok || node["type"] != "heading" {
		return "", invalid
	}
	attrs, ok := node["attrs"].(map[string]any)
	if !ok || attrs["level"] != json.Number("2") || validator.Body(map[string]any{"type": "doc", "content": []any{node}}) != nil {
		return "", invalid
	}
	id, ok := attrs["id"].(string)
	if !ok {
		return "", invalid
	}
	return id, nil
}

````

## 第 32 步：canonicalStructureCandidateBody

插入位置：第 31 步代码之后，紧接追加；完成稿约第 597 行。

本步声明：`func canonicalStructureCandidateBody(validator *codec.Validator, raw []byte, branch bool) ([]byte, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>validator</code> | <code>*codec.Validator</code> | 冻结协议的结构校验器，先限制请求形状，再由业务核验身份、权限、版本和事务条件。<br>在此声明中的实际使用：<code>validator.Body</code> |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>codec.Decode(raw)</code> |
| <code>branch</code> | <code>bool</code> | 在此声明中的实际使用：<code>visit = func(node map&#91;string&#93;any, depth int) bool { 		count++ 		if depth &gt; 64 &#124;&#124; count &gt; 100000 { 			return false 		} 		if attrs, ok := node&#91;&quot;attrs&quot;&#93;.(map&#91;string&#93;any); ok { 			if i …</code>（长表达式，按本步完整代码继续） |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;byte</code> | 实际返回表达式：<code>nil</code>；<code>canonical</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>invalid</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil &#124;&#124; validator.Body(value) != nil</code>。条件成立时执行 <code>return nil, invalid</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>depth &gt; 64 &#124;&#124; count &gt; 100000</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>ok</code>。
- 在本段的匿名函数内：检查 <code>blocks&#91;id&#93;</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>node&#91;&quot;type&quot;&#93; == &quot;image&quot;</code>。
- 在本段的匿名函数内：检查 <code>!ok</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>err != nil</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>branch</code>。
- 在本段的匿名函数内：检查 <code>mark.(map&#91;string&#93;any)&#91;&quot;type&quot;&#93; == &quot;font&quot;</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>!visit(child.(map&#91;string&#93;any), depth+1)</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return true</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!visit(value.(map&#91;string&#93;any), 0)</code>。条件成立时执行 <code>return nil, invalid</code>。当前函数在这里结束。

<!-- write-step: 32 bytes=20926:22138 -->
````go
func canonicalStructureCandidateBody(validator *codec.Validator, raw []byte, branch bool) ([]byte, error) {
	invalid := Violation(TemporarilyUnavailable, "STRUCTURE_MIGRATION_UNKNOWN")
	value, err := codec.Decode(raw)
	if err != nil || validator.Body(value) != nil {
		return nil, invalid
	}
	count, blocks := 0, map[string]bool{}
	var visit func(map[string]any, int) bool
	visit = func(node map[string]any, depth int) bool {
		count++
		if depth > 64 || count > 100000 {
			return false
		}
		if attrs, ok := node["attrs"].(map[string]any); ok {
			if id, ok := attrs["id"].(string); ok {
				if blocks[id] {
					return false
				}
				blocks[id] = true
			}
			if node["type"] == "image" {
				id, ok := attrs["resourceId"].(string)
				if !ok {
					return false
				}
				if _, err := CanonicalID(id); err != nil {
					return false
				}
			}
		}
		if branch {
			for _, mark := range structureMigrationArray(node["marks"]) {
				if mark.(map[string]any)["type"] == "font" {
					return false
				}
			}
		}
		for _, child := range structureMigrationArray(node["content"]) {
			if !visit(child.(map[string]any), depth+1) {
				return false
			}
		}
		return true
	}
	if !visit(value.(map[string]any), 0) {
````

## 第 33 步：canonicalStructureCandidateBody（接着实现）

插入位置：第 32 步代码之后，紧接追加；完成稿约第 642 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return nil, invalid</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>err != nil &#124;&#124; len(canonical) &gt; 16&lt;&lt;20</code>。条件成立时执行 <code>return nil, invalid</code>。当前函数在这里结束。
- 此处通过 <code>return canonical, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 33 bytes=22138:22300 -->
````go
		return nil, invalid
	}
	canonical, err := codec.Canonical(value)
	if err != nil || len(canonical) > 16<<20 {
		return nil, invalid
	}
	return canonical, nil
}

````

## 第 34 步：structureMigrationArray

插入位置：第 33 步代码之后，紧接追加；完成稿约第 651 行。

本步声明：`func structureMigrationArray(value any) []any`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>any</code> | 在此声明中的实际使用：<code>result, _ := value.(&#91;&#93;any)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;any</code> | 实际返回表达式：<code>result</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return result</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 34 bytes=22300:22467 -->
````go
func structureMigrationArray(value any) []any { result, _ := value.([]any); return result }

// structure_preparation.go：保留原实现，按业务合并文件。

````

## 第 35 步：StructurePreparationCut

插入位置：第 34 步代码之后，紧接追加；完成稿约第 655 行。

只缩小原固定cut的支线范围。虚拟支线不产生假room cut，但其入口仍由Node持久屏障保护。

本步声明：`func StructurePreparationCut(original *pb.CutVector, branchRooms, affectedRooms []uint64) (*pb.CutVector, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>original</code> | <code>*pb.CutVector</code> | 在此声明中的实际使用：<code>original == nil</code>；<code>proto.Clone(original)</code>；<code>original.Rooms</code> |
| <code>branchRooms</code> | <code>&#91;&#93;uint64</code> | 预览包含的全部支线房间编号，无零值且不重复。 |
| <code>affectedRooms</code> | <code>&#91;&#93;uint64</code> | 本次实际受结构修改影响的支线房间，必须是branchRooms的无重复子集；准备cut按此范围筛选，不能无故冻结其他支线。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>*pb.CutVector</code> | 实际返回表达式：<code>nil</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>invalid</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>original == nil</code>。条件成立时执行 <code>return nil, invalid</code>。当前函数在这里结束。
- 检查 <code>id == 0 &#124;&#124; branches&#91;id&#93;</code>。条件成立时执行 <code>return nil, invalid</code>。当前函数在这里结束。
- 检查 <code>!branches&#91;id&#93; &#124;&#124; affected&#91;id&#93;</code>。条件成立时执行 <code>return nil, invalid</code>。当前函数在这里结束。
- 检查 <code>len(affected) == 0</code>。条件成立时执行 <code>return nil, invalid</code>。当前函数在这里结束。
- 检查 <code>cut == nil &#124;&#124; cut.RoomId == 0 &#124;&#124; seen&#91;cut.RoomId&#93;</code>。条件成立时执行 <code>return nil, invalid</code>。当前函数在这里结束。
- 检查 <code>!branches&#91;cut.RoomId&#93; &#124;&#124; affected&#91;cut.RoomId&#93;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 35 bytes=22467:23852 -->
````go
// 只缩小原固定cut的支线范围。虚拟支线不产生假room cut，但其入口仍由Node持久屏障保护。
func StructurePreparationCut(original *pb.CutVector, branchRooms, affectedRooms []uint64) (*pb.CutVector, error) {
	invalid := Violation(TemporarilyUnavailable, "STRUCTURE_PREPARATION_UNKNOWN")
	if original == nil {
		return nil, invalid
	}
	branches, affected := map[uint64]bool{}, map[uint64]bool{}
	for _, id := range branchRooms {
		if id == 0 || branches[id] {
			return nil, invalid
		}
		branches[id] = true
	}
	for _, id := range affectedRooms {
		if !branches[id] || affected[id] {
			return nil, invalid
		}
		affected[id] = true
	}
	if len(affected) == 0 {
		return nil, invalid
	}
	result := proto.Clone(original).(*pb.CutVector)
	result.Rooms = make([]*pb.RoomCut, 0, len(original.Rooms))
	seen := map[uint64]bool{}
	for _, cut := range original.Rooms {
		if cut == nil || cut.RoomId == 0 || seen[cut.RoomId] {
			return nil, invalid
		}
		seen[cut.RoomId] = true
		if !branches[cut.RoomId] || affected[cut.RoomId] {
			result.Rooms = append(result.Rooms, proto.Clone(cut).(*pb.RoomCut))
		}
	}
	raw, err := codec.Canonical(CutBinding(result)["rooms"])
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(raw)
	result.DependencyDigest = digest[:]
	return result, nil
}

// structure_preview.go：保留原实现，按业务合并文件。

````

## 第 36 步：StructurePreviewIntent

插入位置：第 35 步代码之后，紧接追加；完成稿约第 700 行。

本步声明：`type StructurePreviewIntent struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Operation</code> | <code>string</code> | 原结构预览操作号 |
| <code>Structure</code> | <code>uint64</code> | 期待结构修订 |
| <code>Control</code> | <code>uint64</code> | 期待控制修订 |
| <code>Input</code> | <code>dto.StructurePreviewCreate</code> | 完整原HTTP动作集合，确认必须一致 |

<!-- write-step: 36 bytes=23852:23993 -->
````go
type StructurePreviewIntent struct {
	Operation          string
	Structure, Control uint64
	Input              dto.StructurePreviewCreate
}

````

## 第 37 步：ParseStructurePreview

插入位置：第 36 步代码之后，紧接追加；完成稿约第 706 行。

本步声明：`func ParseStructurePreview(input dto.StructurePreviewCreate) (StructurePreviewIntent, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>input</code> | <code>dto.StructurePreviewCreate</code> | 在此声明中的实际使用：<code>input.OperationId</code>；<code>Input: input</code>；<code>input.ExpectedStructureId</code>；<code>input.ExpectedControlRevision</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>StructurePreviewIntent</code> | 实际返回表达式：<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。

<!-- write-step: 37 bytes=23993:24764 -->
````go
func ParseStructurePreview(input dto.StructurePreviewCreate) (StructurePreviewIntent, error) {
	result := StructurePreviewIntent{Operation: input.OperationId, Input: input}
	// 共享结构输入的字段规则，保留生成DTO；预览允许声明拆合目标，但不能携带确认引用。
	change := dto.StructureChange{OperationId: input.OperationId, ExpectedStructureId: input.ExpectedStructureId, ExpectedControlRevision: input.ExpectedControlRevision, Changes: input.Changes}
	parsed, err := parseStructureChange(change, true)
	if err != nil {
		return result, err
	}
	result.Control = parsed.Control
	result.Structure, err = CanonicalID(input.ExpectedStructureId)
	return result, err
}

// structure_transition.go：保留原实现，按业务合并文件。

````

## 第 38 步：StructureMigrationIdentity

插入位置：第 37 步代码之后，紧接追加；完成稿约第 721 行。

本步声明：`type StructureMigrationIdentity struct{ Unit, Node string }`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Unit</code> | <code>string</code> | 提交事务分配的目标协作单元ID |
| <code>Node</code> | <code>string</code> | 提交事务分配的结构节点ID |

<!-- write-step: 38 bytes=24764:24825 -->
````go
type StructureMigrationIdentity struct{ Unit, Node string }

````

## 第 39 步：ApplyStructureMigration

插入位置：第 38 步代码之后，紧接追加；完成稿约第 723 行。

按原HTTP动作次序变换完整章组；新unit身份由事务分配，章身份与历史条目保持不变。

本步声明：`func ApplyStructureMigration(original []dto.StructureEntry, input dto.StructureChange, migration StructureMigration, identities map[string]StructureMigrationIdentity) ([]dto.StructureEntry, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>original</code> | <code>&#91;&#93;dto.StructureEntry</code> | 在此声明中的实际使用：<code>slices.Clone(original)</code> |
| <code>input</code> | <code>dto.StructureChange</code> | 在此声明中的实际使用：<code>input.Changes</code>；<code>one := input</code> |
| <code>migration</code> | <code>StructureMigration</code> | 使用 <code>StructureMigration</code> 类型；其定义处的职责：这是系统内部固定候选与范围证据，不复制浏览器HTTP输入，也不预先分配正式单元身份。<br>在此声明中的实际使用：<code>migration.Changes</code> |
| <code>identities</code> | <code>map&#91;string&#93;StructureMigrationIdentity</code> | 在此声明中的实际使用：<code>identity := identities&#91;change.Targets&#91;0&#93;.Key&#93;</code>；<code>identity := identities&#91;target.Key&#93;</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>&#91;&#93;dto.StructureEntry</code> | 实际返回表达式：<code>nil</code>；<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>Violation(StateConflict, &quot;STRUCTURE_RANGE_OVERLAP&quot;)</code>；<code>err</code>；<code>Violation(TemporarilyUnavailable, &quot;STRUCTURE_MIGRATION_UNKNOWN&quot;)</code>；<code>Violation(StateConflict, &quot;STRUCTURE_SHAPE_UNKNOWN&quot;)</code><br>另有 4 处使用，继续按下方完整代码的顺序处理。 |

**跟着本段代码走一遍：**

- 检查 <code>exists</code>。条件成立时执行 <code>return nil, Violation(StateConflict, &quot;STRUCTURE_RANGE_OVERLAP&quot;)</code>。当前函数在这里结束。
- 检查 <code>intent.Type == &quot;RENAME&quot; &#124;&#124; intent.Type == &quot;MOVE&quot;</code>。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>!ok &#124;&#124; change.Type != intent.Type</code>。条件成立时执行 <code>return nil, Violation(TemporarilyUnavailable, &quot;STRUCTURE_MIGRATION_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>entry.ChapterId != change.Chapter</code>。
- 检查 <code>entry.ParentNodeId != nil</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>found &#124;&#124; entry.Kind != &quot;CHAPTER&quot;</code>。条件成立时执行 <code>return nil, Violation(StateConflict, &quot;STRUCTURE_SHAPE_UNKNOWN&quot;)</code>。当前函数在这里结束。
- 检查 <code>change.Type == &quot;REMOVE&quot;</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>change.Type == &quot;JOIN_CHAPTER&quot;</code>。

<!-- write-step: 39 bytes=24825:26421 -->
````go
// 按原HTTP动作次序变换完整章组；新unit身份由事务分配，章身份与历史条目保持不变。
func ApplyStructureMigration(original []dto.StructureEntry, input dto.StructureChange, migration StructureMigration, identities map[string]StructureMigrationIdentity) ([]dto.StructureEntry, error) {
	result := slices.Clone(original)
	changes := map[string]StructureMigrationChange{}
	for _, change := range migration.Changes {
		if _, exists := changes[change.Chapter]; exists {
			return nil, Violation(StateConflict, "STRUCTURE_RANGE_OVERLAP")
		}
		changes[change.Chapter] = change
	}
	for _, intent := range input.Changes {
		if intent.Type == "RENAME" || intent.Type == "MOVE" {
			one := input
			one.Changes = slices.Clone(input.Changes[:1])
			one.Changes[0] = intent
			var err error
			result, err = ApplyStableStructureChanges(result, one)
			if err != nil {
				return nil, err
			}
			continue
		}
		change, ok := changes[intent.ChapterId]
		if !ok || change.Type != intent.Type {
			return nil, Violation(TemporarilyUnavailable, "STRUCTURE_MIGRATION_UNKNOWN")
		}
		next := make([]dto.StructureEntry, 0, len(result)+len(change.Targets))
		found := false
		for _, entry := range result {
			if entry.ChapterId != change.Chapter {
				next = append(next, entry)
				continue
			}
			if entry.ParentNodeId != nil {
				continue
			}
			if found || entry.Kind != "CHAPTER" {
				return nil, Violation(StateConflict, "STRUCTURE_SHAPE_UNKNOWN")
			}
			found = true
			if change.Type == "REMOVE" {
				continue
			}
			root := entry
			if change.Type == "JOIN_CHAPTER" {
````

## 第 40 步：ApplyStructureMigration（接着实现）

插入位置：第 39 步代码之后，紧接追加；完成稿约第 768 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>!root.SplitLevel2 &#124;&#124; len(change.Targets) != 1</code>。条件成立时执行 <code>return nil, Violation(StateConflict, &quot;STRUCTURE_MODE_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>change.Type != &quot;SPLIT_LEVEL2&quot; &#124;&#124; root.SplitLevel2 &#124;&#124; root.UnitId == nil</code>。条件成立时执行 <code>return nil, Violation(StateConflict, &quot;STRUCTURE_MODE_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return nil, err</code>。当前函数在这里结束。
- 检查 <code>!found</code>。条件成立时执行 <code>return nil, Violation(StateConflict, &quot;CHAPTER_CHANGED&quot;)</code>。当前函数在这里结束。
- 检查 <code>next&#91;i&#93;.ParentNodeId == nil</code>。
- 检查 <code>len(result) &gt; 10000</code>。条件成立时执行 <code>return nil, Violation(StateConflict, &quot;STRUCTURE_ENTRY_LIMIT&quot;)</code>。当前函数在这里结束。

<!-- write-step: 40 bytes=26421:27837 -->
````go
				if !root.SplitLevel2 || len(change.Targets) != 1 {
					return nil, Violation(StateConflict, "STRUCTURE_MODE_CHANGED")
				}
				identity := identities[change.Targets[0].Key]
				if _, err := CanonicalID(identity.Unit); err != nil {
					return nil, err
				}
				root.SplitLevel2 = false
				root.UnitId = &identity.Unit
				next = append(next, root)
				continue
			}
			if change.Type != "SPLIT_LEVEL2" || root.SplitLevel2 || root.UnitId == nil {
				return nil, Violation(StateConflict, "STRUCTURE_MODE_CHANGED")
			}
			root.SplitLevel2 = true
			root.UnitId = nil
			next = append(next, root)
			for i, target := range change.Targets {
				identity := identities[target.Key]
				if _, err := CanonicalID(identity.Unit); err != nil {
					return nil, err
				}
				if _, err := CanonicalID(identity.Node); err != nil {
					return nil, err
				}
				parent := root.NodeId
				next = append(next, dto.StructureEntry{NodeId: identity.Node, ParentNodeId: &parent, ChapterId: root.ChapterId, Kind: target.Kind, Title: target.Title, SortOrder: int64(i + 1), UnitId: &identity.Unit})
			}
		}
		if !found {
			return nil, Violation(StateConflict, "CHAPTER_CHANGED")
		}
		order := int64(0)
		for i := range next {
			if next[i].ParentNodeId == nil {
				order++
				next[i].SortOrder = order
			}
		}
		result = next
	}
	if len(result) > 10000 {
		return nil, Violation(StateConflict, "STRUCTURE_ENTRY_LIMIT")
	}
````

## 第 41 步：ApplyStructureMigration（接着实现）

插入位置：第 40 步代码之后，紧接追加；完成稿约第 813 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 41 bytes=27837:27859 -->
````go
	return result, nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/domain/document/structure_models.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/04-structure.md)。
