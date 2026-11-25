# 分步编写 `services/document/internal/domain/document/export.go`

先按 [D10 · 固定导出、转换器与进程退出](../lessons/document/10-exports.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/093-export.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

定义导出union输入、固定计划、作业记录和转换器输入正文。

导出任务执行时页面可能已变，必须只读取登记时manifest与实际资源依赖。

写之前先按这条执行路线理解各段的连接：

1. ParseExportInput解析冻结四字段
2. Validate核MAIN/PUBLIC与格式组合
3. ExportSnapshot固定版本和权威来源
4. DecodeExportSnapshot严格验证持久计划。

本文件需要守住的边界：

- RUNNING任务保护历史依赖
- ExportDocument只接受已验证正文树，不能含任意URL或主机文件路径。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/domain/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/domain/document/export.go') -and (Get-Item -LiteralPath 'services/document/internal/domain/document/export.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/domain/document/export.go')) { New-Item -ItemType File -Path 'services/document/internal/domain/document/export.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:176 -->
````go
package document

import (
	codec "collaboration/contracts/content"
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	"encoding/json"
	"time"
)

````

## 第 2 步：ExportInput

插入位置：第 1 步代码之后，紧接追加；完成稿约第 11 行。

ExportInput 是冻结 union 的服务端判别结果，不替代生成 DTO；解码仍拒绝未知字段。

本步声明：`type ExportInput struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Operation</code> | <code>string</code> | 创建导出的幂等操作号 |
| <code>Target</code> | <code>string</code> | MAIN或PUBLIC，决定权限与固定依据 |
| <code>Format</code> | <code>string</code> | DOCX、PDF或MARKDOWN |
| <code>Manifest</code> | <code>string</code> | MAIN导出的固定阅读清单引用 |
| <code>Release</code> | <code>string</code> | PUBLIC导出的固定正式版引用 |

<!-- write-step: 2 bytes=176:369 -->
````go
// ExportInput 是冻结 union 的服务端判别结果，不替代生成 DTO；解码仍拒绝未知字段。
type ExportInput struct {
	Operation, Target, Format, Manifest, Release string
}

````

## 第 3 步：ParseExportInput

插入位置：第 2 步代码之后，紧接追加；完成稿约第 16 行。

Go生成器将oneOf保留为原始JSON；这里只判别四个冻结字段，不另造HTTP DTO。

本步声明：`func ParseExportInput(raw dto.ExportCreate) (ExportInput, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>raw</code> | <code>dto.ExportCreate</code> | 在此声明中的实际使用：<code>&#91;&#93;byte(raw)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>ExportInput</code> | 使用 <code>ExportInput</code> 类型；其定义处的职责：ExportInput 是冻结 union 的服务端判别结果，不替代生成 DTO；解码仍拒绝未知字段。<br>实际返回表达式：<code>value</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>Violation(InvalidRequest, &quot;EXPORT_INPUT_INVALID&quot;)</code>；<code>value.Validate()</code> |

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return value, Violation(InvalidRequest, &quot;EXPORT_INPUT_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>!ok &#124;&#124; len(object) != 4</code>。条件成立时执行 <code>return value, Violation(InvalidRequest, &quot;EXPORT_INPUT_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>value.Target == &quot;PUBLIC&quot;</code>。
- 检查 <code>!ok</code>。条件成立时执行 <code>return value, Violation(InvalidRequest, &quot;EXPORT_INPUT_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>value.Target == &quot;MAIN&quot;</code>。
- 此处通过 <code>return value, value.Validate()</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=369:1304 -->
````go
// Go生成器将oneOf保留为原始JSON；这里只判别四个冻结字段，不另造HTTP DTO。
func ParseExportInput(raw dto.ExportCreate) (ExportInput, error) {
	var value ExportInput
	decoded, err := codec.Decode([]byte(raw))
	if err != nil {
		return value, Violation(InvalidRequest, "EXPORT_INPUT_INVALID")
	}
	object, ok := decoded.(map[string]any)
	if !ok || len(object) != 4 {
		return value, Violation(InvalidRequest, "EXPORT_INPUT_INVALID")
	}
	value.Operation, _ = object["operation_id"].(string)
	value.Target, _ = object["target"].(string)
	value.Format, _ = object["format"].(string)
	field := "expected_manifest_id"
	if value.Target == "PUBLIC" {
		field = "expected_release_id"
	}
	id, ok := object[field].(string)
	if !ok {
		return value, Violation(InvalidRequest, "EXPORT_INPUT_INVALID")
	}
	if value.Target == "MAIN" {
		value.Manifest = id
	} else {
		value.Release = id
	}
	return value, value.Validate()
}

````

## 第 4 步：ExportInput.Validate

插入位置：第 3 步代码之后，紧接追加；完成稿约第 46 行。

本步声明：`func (v ExportInput) Validate() error`。


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>Violation(InvalidRequest, &quot;EXPORT_INPUT_INVALID&quot;)</code>；<code>err</code> |

**跟着本段代码走一遍：**

- 检查 <code>!ValidOperationID(v.Operation) &#124;&#124; (v.Format != &quot;DOCX&quot; &amp;&amp; v.Format != &quot;MARKDOWN&quot; &amp;&amp; v.Format != &quot;PDF&quot;)</code>。条件成立时执行 <code>return Violation(InvalidRequest, &quot;EXPORT_INPUT_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>v.Release != &quot;&quot;</code>。条件成立时执行 <code>return Violation(InvalidRequest, &quot;EXPORT_INPUT_INVALID&quot;)</code>。当前函数在这里结束。
- 检查 <code>v.Manifest != &quot;&quot;</code>。条件成立时执行 <code>return Violation(InvalidRequest, &quot;EXPORT_INPUT_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return err</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 4 bytes=1304:1902 -->
````go
func (v ExportInput) Validate() error {
	if !ValidOperationID(v.Operation) || (v.Format != "DOCX" && v.Format != "MARKDOWN" && v.Format != "PDF") {
		return Violation(InvalidRequest, "EXPORT_INPUT_INVALID")
	}
	var id string
	switch v.Target {
	case "MAIN":
		if v.Release != "" {
			return Violation(InvalidRequest, "EXPORT_INPUT_INVALID")
		}
		id = v.Manifest
	case "PUBLIC":
		if v.Manifest != "" {
			return Violation(InvalidRequest, "EXPORT_INPUT_INVALID")
		}
		id = v.Release
	default:
		return Violation(InvalidRequest, "EXPORT_INPUT_INVALID")
	}
	_, err := CanonicalID(id)
	return err
}

````

## 第 5 步：ExportSnapshot

插入位置：第 4 步代码之后，紧接追加；完成稿约第 69 行。

固定计划只存依赖及权威来源；正文从不可变manifest装配，RUNNING任务阻止提前清理历史。

本步声明：`type ExportSnapshot struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Document</code> | <code>uint64</code> | 原文档ID |
| <code>Space</code> | <code>uint64</code> | 固定计费空间 |
| <code>Manifest</code> | <code>uint64</code> | 导出完整清单 |
| <code>Structure</code> | <code>uint64</code> | 清单固定结构 |
| <code>Style</code> | <code>uint64</code> | 清单固定样式 |
| <code>Release</code> | <code>uint64</code> | 正式导出release，MAIN时按协议为空值 |
| <code>Publication</code> | <code>uint64</code> | 固定正式发布代次 |
| <code>Title</code> | <code>string</code> | 固定导出标题 |
| <code>Target</code> | <code>string</code> | MAIN或PUBLIC来源 |
| <code>Format</code> | <code>string</code> | 输出格式 |
| <code>Captured</code> | <code>time.Time</code> | 计划固定时刻 |
| <code>Sources</code> | <code>&#91;&#93;*pb.SourceEpoch</code> | 该导出实际依赖的权威来源代次 |

<!-- write-step: 5 bytes=1902:2329 -->
````go
// 固定计划只存依赖及权威来源；正文从不可变manifest装配，RUNNING任务阻止提前清理历史。
type ExportSnapshot struct {
	Document, Space, Manifest, Structure, Style uint64
	Release, Publication                        uint64
	Title, Target, Format                       string
	Captured                                    time.Time
	Sources                                     []*pb.SourceEpoch
}

````

## 第 6 步：ExportRecord

插入位置：第 5 步代码之后，紧接追加；完成稿约第 78 行。

本步声明：`type ExportRecord struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Job</code> | <code>dto.ExportJob</code> | 公开作业DTO及状态 |
| <code>Requester</code> | <code>uint64</code> | 发起用户，匿名用协议约定零值 |
| <code>Operation</code> | <code>string</code> | 原创建操作号 |
| <code>Fingerprint</code> | <code>&#91;32&#93;byte</code> | 请求绑定摘要 |
| <code>Snapshot</code> | <code>ExportSnapshot</code> | 不可变导出计划 |
| <code>Generation</code> | <code>uint64</code> | worker执行代次 |
| <code>ArtifactKey</code> | <code>string</code> | 完成制品不可变存储键 |
| <code>Size</code> | <code>uint64</code> | 制品实际字节数 |
| <code>Checksum</code> | <code>&#91;32&#93;byte</code> | 制品SHA-256 |
| <code>Expires</code> | <code>time.Time</code> | 作业/下载的绝对期限 |

<!-- write-step: 6 bytes=2329:2581 -->
````go
type ExportRecord struct {
	Job         dto.ExportJob
	Requester   uint64
	Operation   string
	Fingerprint [32]byte
	Snapshot    ExportSnapshot
	Generation  uint64
	ArtifactKey string
	Size        uint64
	Checksum    [32]byte
	Expires     time.Time
}

````

## 第 7 步：ExportDocument

插入位置：第 6 步代码之后，紧接追加；完成稿约第 91 行。

固定输入交给受控转换器；每种输出共享相同正文、资源与版本说明。

本步声明：`type ExportDocument struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Title</code> | <code>string</code> | 固定标题<br>序列化/校验标记：<code>`json:&quot;,omitempty&quot;`</code> |
| <code>Target</code> | <code>string</code> | 固定来源类型<br>序列化/校验标记：<code>`json:&quot;,omitempty&quot;`</code> |
| <code>Format</code> | <code>string</code> | 目标输出格式<br>序列化/校验标记：<code>`json:&quot;,omitempty&quot;`</code> |
| <code>Manifest</code> | <code>string</code> | 版本说明中的清单文本ID<br>序列化/校验标记：<code>`json:&quot;,omitempty&quot;`</code> |
| <code>Release</code> | <code>string</code> | 版本说明中的正式版文本ID<br>序列化/校验标记：<code>`json:&quot;,omitempty&quot;`</code> |
| <code>Captured</code> | <code>time.Time</code> | 计划固定时刻 |
| <code>Structure</code> | <code>dto.StructureRevision</code> | 完整固定结构 |
| <code>Style</code> | <code>dto.StyleRevision</code> | 固定样式修订 |
| <code>Units</code> | <code>&#91;&#93;ExportUnit</code> | 已完整校验的单元正文树 |
| <code>Assets</code> | <code>&#91;&#93;ExportAsset</code> | 已按固定引用读验的附件 |

<!-- write-step: 7 bytes=2581:3066 -->
````go
// 固定输入交给受控转换器；每种输出共享相同正文、资源与版本说明。
type ExportDocument struct {
	Title, Target, Format, Manifest, Release string `json:",omitempty"`
	Captured                                 time.Time
	Structure                                dto.StructureRevision
	Style                                    dto.StyleRevision
	Units                                    []ExportUnit
	Assets                                   []ExportAsset
}

````

## 第 8 步：ExportUnit

插入位置：第 7 步代码之后，紧接追加；完成稿约第 101 行。

转换器只消费已经完整校验的正文树；生成HTTP DTO的RawMessage字段在业务边界解码。

本步声明：`type ExportUnit struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ID</code> | <code>string</code> | 单元文本ID，与结构和书签绑定 |
| <code>Body</code> | <code>map&#91;string&#93;any</code> | 通过content-v1验证的规范正文树 |

<!-- write-step: 8 bytes=3066:3240 -->
````go
// 转换器只消费已经完整校验的正文树；生成HTTP DTO的RawMessage字段在业务边界解码。
type ExportUnit struct {
	ID   string
	Body map[string]any
}

````

## 第 9 步：ExportAsset

插入位置：第 8 步代码之后，紧接追加；完成稿约第 107 行。

本步声明：`type ExportAsset struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ID</code> | <code>string</code> | 资源文本ID |
| <code>Key</code> | <code>string</code> | SQL固定对象键 |
| <code>MediaType</code> | <code>string</code> | 已验证媒体类型 |
| <code>Size</code> | <code>uint64</code> | 原对象实际长度 |
| <code>Checksum</code> | <code>&#91;32&#93;byte</code> | 原资源SHA-256 |
| <code>Data</code> | <code>&#91;&#93;byte</code> | 已验证资源字节，不序列化进持久JSON<br>序列化/校验标记：<code>`json:&quot;-&quot;`</code> |

<!-- write-step: 9 bytes=3240:3390 -->
````go
type ExportAsset struct {
	ID, Key, MediaType string
	Size               uint64
	Checksum           [32]byte
	Data               []byte `json:"-"`
}

````

## 第 10 步：DecodeExportSnapshot

插入位置：第 9 步代码之后，紧接追加；完成稿约第 114 行。

本步声明：`func DecodeExportSnapshot(raw []byte) (ExportSnapshot, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>raw</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>len(raw)</code>；<code>json.Unmarshal(raw, &amp;fixed)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>ExportSnapshot</code> | 使用 <code>ExportSnapshot</code> 类型；其定义处的职责：固定计划只存依赖及权威来源；正文从不可变manifest装配，RUNNING任务阻止提前清理历史。<br>实际返回表达式：<code>fixed</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>Violation(TemporarilyUnavailable, &quot;EXPORT_FIXED_PLAN_INVALID&quot;)</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(raw) &gt; 128&lt;&lt;10 &#124;&#124; json.Unmarshal(raw, &amp;fixed) != nil &#124;&#124; fixed.Document == 0 &#124;&#124; fixed.Manifest == 0 &#124;&#124; fixed.Structure == 0 &#124;&#124; fixed.Style == 0 &#124;&#124; len(fixed.Sources) == 0 &#124;&#124; len(fixed.Sources) &gt; 512</code>。条件成立时执行 <code>return fixed, Violation(TemporarilyUnavailable, &quot;EXPORT_FIXED_PLAN_INVALID&quot;)</code>。当前函数在这里结束。
- 此处通过 <code>return fixed, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 10 bytes=3390:3791 -->
````go
func DecodeExportSnapshot(raw []byte) (ExportSnapshot, error) {
	var fixed ExportSnapshot
	if len(raw) > 128<<10 || json.Unmarshal(raw, &fixed) != nil || fixed.Document == 0 || fixed.Manifest == 0 || fixed.Structure == 0 || fixed.Style == 0 || len(fixed.Sources) == 0 || len(fixed.Sources) > 512 {
		return fixed, Violation(TemporarilyUnavailable, "EXPORT_FIXED_PLAN_INVALID")
	}
	return fixed, nil
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/domain/document/export.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/10-exports.md)。
