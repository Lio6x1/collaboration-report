# 分步编写 `services/document/internal/logic/document/historical_ranges_service.go`

先按 [D04 · 结构预览、拆章、合章与历史范围](../lessons/document/04-structure.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/206-historical-ranges-service.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

把历史映射声明验证成可信范围证明。

存有一条映射并不意味着安全，来源必须对应真实版本、原预览候选及成功安装。

写之前先按这条执行路线理解各段的连接：

1. readHistoricalStructureWitness读取原成功工件
2. actualHistoricalVersion核document/unit/structure/hash
3. proveHistoricalMapping逐项比对区间或WHOLE_UNIT映射。

本文件需要守住的边界：

- 无法证明返回HISTORICAL_RANGE_UNPROVEN
- 块ID存在本身不足以证明完整原范围。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/logic/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/logic/document/historical_ranges_service.go') -and (Get-Item -LiteralPath 'services/document/internal/logic/document/historical_ranges_service.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/logic/document/historical_ranges_service.go')) { New-Item -ItemType File -Path 'services/document/internal/logic/document/historical_ranges_service.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:587 -->
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
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"google.golang.org/protobuf/encoding/protojson"
	"slices"
	"strconv"
)

// historical_range_proof.go：保留原实现，按业务合并文件。

````

## 第 2 步：historicalRangeMapping

插入位置：第 1 步代码之后，紧接追加；完成稿约第 22 行。

本步声明：`type historicalRangeMapping struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Schema</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;schema_version&quot;`</code> |
| <code>Kind</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;type&quot;`</code> |
| <code>Chapter</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;chapter_id&quot;`</code> |
| <code>Operation</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;operation_id&quot;`</code> |
| <code>Source</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;source_unit_id&quot;`</code> |
| <code>Target</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;target_unit_id&quot;`</code> |
| <code>SourceVersion</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;source_version_id&quot;`</code> |
| <code>TargetVersion</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;target_version_id&quot;`</code> |
| <code>SourceStructure</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;source_structure_id&quot;`</code> |
| <code>TargetStructure</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;target_structure_id&quot;`</code> |
| <code>SourceHash</code> | <code>&#91;&#93;byte</code> | 序列化/校验标记：<code>`json:&quot;source_body_hash&quot;`</code> |
| <code>TargetHash</code> | <code>&#91;&#93;byte</code> | 序列化/校验标记：<code>`json:&quot;target_body_hash&quot;`</code> |
| <code>Scope</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;source_scope&quot;`</code> |
| <code>Start</code> | <code>*string</code> | 序列化/校验标记：<code>`json:&quot;start_boundary&quot;`</code> |
| <code>End</code> | <code>*string</code> | 序列化/校验标记：<code>`json:&quot;end_boundary&quot;`</code> |
| <code>MainBlocks</code> | <code>&#91;&#93;string</code> | 序列化/校验标记：<code>`json:&quot;main_block_ids&quot;`</code> |
| <code>DraftBlocks</code> | <code>&#91;&#93;string</code> | 序列化/校验标记：<code>`json:&quot;draft_block_ids&quot;`</code> |
| <code>Boundary</code> | <code>domain.StructureMigrationBoundary</code> | 使用 <code>domain.StructureMigrationBoundary</code> 类型；其定义处的职责：表达管理意图、拆合迁移候选和历史范围边界，并提供纯结构变换。<br>序列化/校验标记：<code>`json:&quot;boundary&quot;`</code> |
| <code>TargetKind</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;target_kind&quot;`</code> |
| <code>TargetKey</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;target_key&quot;`</code> |
| <code>TargetScope</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;target_scope&quot;`</code> |
| <code>TargetStart</code> | <code>*string</code> | 序列化/校验标记：<code>`json:&quot;target_start_boundary&quot;`</code> |
| <code>TargetEnd</code> | <code>*string</code> | 序列化/校验标记：<code>`json:&quot;target_end_boundary&quot;`</code> |
| <code>Preview</code> | <code>string</code> | 序列化/校验标记：<code>`json:&quot;preview_cut_ref&quot;`</code> |
| <code>Digest</code> | <code>&#91;&#93;byte</code> | 序列化/校验标记：<code>`json:&quot;preview_digest&quot;`</code> |

<!-- write-step: 2 bytes=587:2501 -->
````go
type historicalRangeMapping struct {
	Schema          string                            `json:"schema_version"`
	Kind            string                            `json:"type"`
	Chapter         string                            `json:"chapter_id"`
	Operation       string                            `json:"operation_id"`
	Source          string                            `json:"source_unit_id"`
	Target          string                            `json:"target_unit_id"`
	SourceVersion   string                            `json:"source_version_id"`
	TargetVersion   string                            `json:"target_version_id"`
	SourceStructure string                            `json:"source_structure_id"`
	TargetStructure string                            `json:"target_structure_id"`
	SourceHash      []byte                            `json:"source_body_hash"`
	TargetHash      []byte                            `json:"target_body_hash"`
	Scope           string                            `json:"source_scope"`
	Start           *string                           `json:"start_boundary"`
	End             *string                           `json:"end_boundary"`
	MainBlocks      []string                          `json:"main_block_ids"`
	DraftBlocks     []string                          `json:"draft_block_ids"`
	Boundary        domain.StructureMigrationBoundary `json:"boundary"`
	TargetKind      string                            `json:"target_kind"`
	TargetKey       string                            `json:"target_key"`
	TargetScope     string                            `json:"target_scope"`
	TargetStart     *string                           `json:"target_start_boundary"`
	TargetEnd       *string                           `json:"target_end_boundary"`
	Preview         string                            `json:"preview_cut_ref"`
	Digest          []byte                            `json:"preview_digest"`
}

````

## 第 3 步：proveHistoricalMapping

插入位置：第 2 步代码之后，紧接追加；完成稿约第 50 行。

上层已核验原终态/预览/manifest；本函数再把每条声明与实际身份、实际版本及原候选逐项对齐。

本步声明：`func proveHistoricalMapping(row repository.HistoricalUnitMapping, source, target *pb.FixedContent, witness historicalStructureWitness) (domain.HistoricalRangeEdge, bool)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>row</code> | <code>repository.HistoricalUnitMapping</code> | 使用 <code>repository.HistoricalUnitMapping</code> 类型；其定义处的职责：沿旧单元读取历史迁移映射，并取原结构动作终态与manifest绑定证据。<br>在此声明中的实际使用：<code>row.Schema</code>；<code>row.Source</code>；<code>row.Target</code>；<code>row.SourceDocument</code><br>另有 16 处使用，继续按下方完整代码的顺序处理。 |
| <code>source</code> | <code>*pb.FixedContent</code> | 在此声明中的实际使用：<code>actualHistoricalVersion(source, row.SourceDocument, row.Source, row.SourceStructure, row.Operation, row.SourceHash)</code>；<code>source.CaptureId</code>；<code>source.BodyJsonUtf8</code>；<code>source.BodyHash</code> |
| <code>target</code> | <code>*pb.FixedContent</code> | 在此声明中的实际使用：<code>actualHistoricalVersion(target, row.TargetDocument, row.Target, row.TargetStructure, row.Operation, row.TargetHash)</code>；<code>target.CaptureId</code>；<code>target.BodyJsonUtf8</code>；<code>target.BodyHash</code> |
| <code>witness</code> | <code>historicalStructureWitness</code> | 在此声明中的实际使用：<code>witness.Original</code>；<code>witness.Install</code>；<code>witness.Snapshot</code>；<code>witness.Migration</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>domain.HistoricalRangeEdge</code> | 使用 <code>domain.HistoricalRangeEdge</code> 类型；其定义处的职责：仅接纳已核验实际版本/预览/退役事实的边；块是否仍存在不是历史整单元的授权证明。<br>实际返回表达式：<code>edge</code> |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>matched</code> |

**跟着本段代码走一遍：**

- 检查 <code>original.StructurePreview == nil &#124;&#124; install.Transition == nil &#124;&#124; row.Schema != &quot;full-range-v1&quot; &#124;&#124; row.Source == 0 &#124;&#124; row.Target == 0 &#124;&#124; row.Source == row.Target &#124;&#124; 		row.SourceDocu …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return edge, false</code>。当前函数在这里结束。
- 检查 <code>len(row.Mapping) &gt; 4&lt;&lt;20 &#124;&#124; json.Unmarshal(row.Mapping, &amp;mapping) != nil &#124;&#124; mapping.Schema != row.Schema &#124;&#124; mapping.Operation != row.Operation &#124;&#124; 		mapping.Source != domain.TextID( …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return edge, false</code>。当前函数在这里结束。
- 检查 <code>branch.Unit == row.Target</code>。
- 检查 <code>key != &quot;&quot;</code>。条件成立时执行 <code>return edge, false</code>。当前函数在这里结束。
- 检查 <code>key == &quot;&quot; &#124;&#124; target.CaptureId != key &#124;&#124; (mapping.TargetKey != &quot;&quot; &amp;&amp; mapping.TargetKey != key)</code>。条件成立时执行 <code>return edge, false</code>。当前函数在这里结束。
- 检查 <code>unit != nil &amp;&amp; unit.UnitId == row.Target</code>。
- 检查 <code>foundVersion &#124;&#124; unit.ContentVersionId != row.TargetVersion &#124;&#124; unit.Content == nil &#124;&#124; !bytes.Equal(unit.Content.BodyJsonUtf8, target.BodyJsonUtf8) &#124;&#124; !bytes.Equal(unit.Content.BodyHash, target.BodyHash)</code>。条件成立时执行 <code>return edge, false</code>。当前函数在这里结束。
- 检查 <code>!foundVersion</code>。条件成立时执行 <code>return edge, false</code>。当前函数在这里结束。

<!-- write-step: 3 bytes=2501:5452 -->
````go
// 上层已核验原终态/预览/manifest；本函数再把每条声明与实际身份、实际版本及原候选逐项对齐。
func proveHistoricalMapping(row repository.HistoricalUnitMapping, source, target *pb.FixedContent, witness historicalStructureWitness) (domain.HistoricalRangeEdge, bool) {
	edge := domain.HistoricalRangeEdge{}
	original, install := witness.Original, witness.Install
	if original.StructurePreview == nil || install.Transition == nil || row.Schema != "full-range-v1" || row.Source == 0 || row.Target == 0 || row.Source == row.Target ||
		row.SourceDocument != original.Document || row.TargetDocument != original.Document || row.Chapter != row.TargetChapter ||
		row.SourceRetired != install.Structure || row.TargetCreated != install.Structure || row.SourceStructure != original.Structure || row.TargetStructure != install.Structure ||
		row.SourceKind != "MAIN_CAPTURE" || row.VersionKind != "STRUCTURE" || row.Operation != original.Intent.Operation ||
		!actualHistoricalVersion(source, row.SourceDocument, row.Source, row.SourceStructure, row.Operation, row.SourceHash) ||
		!actualHistoricalVersion(target, row.TargetDocument, row.Target, row.TargetStructure, row.Operation, row.TargetHash) ||
		source.CaptureId != "structure-source:"+domain.TextID(row.Source) {
		return edge, false
	}
	var mapping historicalRangeMapping
	if len(row.Mapping) > 4<<20 || json.Unmarshal(row.Mapping, &mapping) != nil || mapping.Schema != row.Schema || mapping.Operation != row.Operation ||
		mapping.Source != domain.TextID(row.Source) || mapping.Target != domain.TextID(row.Target) || mapping.SourceVersion != domain.TextID(row.SourceVersion) ||
		mapping.TargetVersion != domain.TextID(row.TargetVersion) || mapping.SourceStructure != domain.TextID(row.SourceStructure) || mapping.TargetStructure != domain.TextID(row.TargetStructure) ||
		mapping.Chapter != domain.TextID(row.Chapter) || mapping.TargetKind != row.TargetKind || mapping.Scope != "WHOLE_UNIT" ||
		!bytes.Equal(mapping.SourceHash, row.SourceHash) || !bytes.Equal(mapping.TargetHash, row.TargetHash) ||
		mapping.Preview != original.StructurePreview.Reference || !bytes.Equal(mapping.Digest, original.StructurePreview.Digest) {
		return edge, false
	}
	key := ""
	for _, branch := range install.Transition.Branches {
		if branch.Unit == row.Target {
			if key != "" {
				return edge, false
			}
			key = branch.Key
		}
	}
	if key == "" || target.CaptureId != key || (mapping.TargetKey != "" && mapping.TargetKey != key) {
		return edge, false
	}
	foundVersion := false
	for _, unit := range install.Units {
		if unit != nil && unit.UnitId == row.Target {
			if foundVersion || unit.ContentVersionId != row.TargetVersion || unit.Content == nil || !bytes.Equal(unit.Content.BodyJsonUtf8, target.BodyJsonUtf8) || !bytes.Equal(unit.Content.BodyHash, target.BodyHash) {
				return edge, false
			}
			foundVersion = true
		}
	}
	if !foundVersion {
		return edge, false
	}
````

## 第 4 步：proveHistoricalMapping（接着实现）

插入位置：第 3 步代码之后，紧接追加；完成稿约第 96 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>content != nil &amp;&amp; content.UnitId == row.Source</code>。
- 检查 <code>foundSource &#124;&#124; content.DocumentId != row.SourceDocument &#124;&#124; content.StructureId != row.SourceStructure &#124;&#124; !bytes.Equal(content.BodyJsonUtf8, source.BodyJsonUtf8) &#124;&#124; !bytes.Equal(content.BodyHash, source.BodyHash)</code>。条件成立时执行 <code>return edge, false</code>。当前函数在这里结束。
- 检查 <code>!foundSource</code>。条件成立时执行 <code>return edge, false</code>。当前函数在这里结束。
- 检查 <code>candidate.Key != key</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>matched &#124;&#124; change.Type != mapping.Kind &#124;&#124; change.Chapter != mapping.Chapter &#124;&#124; candidate.Kind != mapping.TargetKind &#124;&#124; 				!bytes.Equal(candidate.Main, target.BodyJsonUtf8) &#124;&#124; !equ …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return edge, false</code>。当前函数在这里结束。
- 检查 <code>segment.Source != mapping.Source</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>rangeFound &#124;&#124; segment.Scope != mapping.Scope &#124;&#124; !equalBoundary(segment.Start, mapping.Start) &#124;&#124; !equalBoundary(segment.End, mapping.End) &#124;&#124; 					!slices.Equal(segment.MainBlocks, m …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return edge, false</code>。当前函数在这里结束。
- 检查 <code>!rangeFound</code>。条件成立时执行 <code>return edge, false</code>。当前函数在这里结束。
- 检查 <code>change.Type == &quot;SPLIT_LEVEL2&quot;</code>。
- 检查 <code>!ok</code>。条件成立时执行 <code>return edge, false</code>。当前函数在这里结束。

<!-- write-step: 4 bytes=5452:7438 -->
````go
	foundSource := false
	for _, content := range witness.Snapshot.Main {
		if content != nil && content.UnitId == row.Source {
			if foundSource || content.DocumentId != row.SourceDocument || content.StructureId != row.SourceStructure || !bytes.Equal(content.BodyJsonUtf8, source.BodyJsonUtf8) || !bytes.Equal(content.BodyHash, source.BodyHash) {
				return edge, false
			}
			foundSource = true
		}
	}
	if !foundSource {
		return edge, false
	}
	matched := false
	for _, change := range witness.Migration.Changes {
		for i, candidate := range change.Targets {
			if candidate.Key != key {
				continue
			}
			if matched || change.Type != mapping.Kind || change.Chapter != mapping.Chapter || candidate.Kind != mapping.TargetKind ||
				!bytes.Equal(candidate.Main, target.BodyJsonUtf8) || !equalHistoricalJSON(candidate.Boundary, mapping.Boundary) {
				return edge, false
			}
			rangeFound := false
			for _, segment := range candidate.Ranges {
				if segment.Source != mapping.Source {
					continue
				}
				if rangeFound || segment.Scope != mapping.Scope || !equalBoundary(segment.Start, mapping.Start) || !equalBoundary(segment.End, mapping.End) ||
					!slices.Equal(segment.MainBlocks, mapping.MainBlocks) || !slices.Equal(segment.DraftBlocks, mapping.DraftBlocks) ||
					segment.TargetScope != mapping.TargetScope || !equalBoundary(segment.TargetStart, mapping.TargetStart) || !equalBoundary(segment.TargetEnd, mapping.TargetEnd) {
					return edge, false
				}
				rangeFound = true
			}
			if !rangeFound {
				return edge, false
			}
			edge = domain.HistoricalRangeEdge{Source: row.Source, Target: row.Target, Kind: change.Type, Order: i, Start: mapping.Start, End: mapping.End,
				TargetInterval: mapping.TargetScope == "INTERVAL", TargetStart: mapping.TargetStart, TargetEnd: mapping.TargetEnd}
			if change.Type == "SPLIT_LEVEL2" {
				var ok bool
				edge.Heads, ok = domain.StructureTopLevel2IDs(source.BodyJsonUtf8)
				if !ok {
					return edge, false
				}
			}
````

## 第 5 步：proveHistoricalMapping（接着实现）

插入位置：第 4 步代码之后，紧接追加；完成稿约第 142 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return edge, matched</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 5 bytes=7438:7488 -->
````go
			matched = true
		}
	}
	return edge, matched
}

````

## 第 6 步：actualHistoricalVersion

插入位置：第 5 步代码之后，紧接追加；完成稿约第 148 行。

本步声明：`func actualHistoricalVersion(content *pb.FixedContent, document, unit, structure uint64, operation string, expectedHash []byte) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>content</code> | <code>*pb.FixedContent</code> | 在此声明中的实际使用：<code>content == nil</code>；<code>content.DocumentId</code>；<code>content.UnitId</code>；<code>content.StructureId</code><br>另有 7 处使用，继续按下方完整代码的顺序处理。 |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>content.DocumentId != document</code> |
| <code>unit</code> | <code>uint64</code> | 在此声明中的实际使用：<code>content.UnitId != unit</code> |
| <code>structure</code> | <code>uint64</code> | 在此声明中的实际使用：<code>content.StructureId != structure</code> |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>content.OperationId != operation</code> |
| <code>expectedHash</code> | <code>&#91;&#93;byte</code> | 在此声明中的实际使用：<code>len(expectedHash)</code>；<code>bytes.Equal(content.BodyHash, expectedHash)</code>；<code>bytes.Equal(digest&#91;:&#93;, expectedHash)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>err == nil &amp;&amp; bytes.Equal(canonical, content.BodyJsonUtf8) &amp;&amp; bytes.Equal(digest&#91;:&#93;, expectedHash)</code> |

**跟着本段代码走一遍：**

- 检查 <code>content == nil &#124;&#124; content.DocumentId != document &#124;&#124; content.UnitId != unit &#124;&#124; content.StructureId != structure &#124;&#124; content.OperationId != operation &#124;&#124; 		content.SchemaVersion != &quot;co …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 此处通过 <code>return err == nil &amp;&amp; bytes.Equal(canonical, content.BodyJsonUtf8) &amp;&amp; bytes.Equal(digest&#91;:&#93;, expectedHash)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 6 bytes=7488:8373 -->
````go
func actualHistoricalVersion(content *pb.FixedContent, document, unit, structure uint64, operation string, expectedHash []byte) bool {
	if content == nil || content.DocumentId != document || content.UnitId != unit || content.StructureId != structure || content.OperationId != operation ||
		content.SchemaVersion != "content-v1" || content.CanonicalizationVersion != "canonical-v1" || content.HashAlgorithm != "SHA-256" ||
		len(content.BodyJsonUtf8) > 16<<20 || content.ByteSize != uint64(len(content.BodyJsonUtf8)) || len(expectedHash) != 32 || !bytes.Equal(content.BodyHash, expectedHash) {
		return false
	}
	value, err := codec.Decode(content.BodyJsonUtf8)
	if err != nil {
		return false
	}
	canonical, err := codec.Canonical(value)
	digest := sha256.Sum256(canonical)
	return err == nil && bytes.Equal(canonical, content.BodyJsonUtf8) && bytes.Equal(digest[:], expectedHash)
}

````

## 第 7 步：equalBoundary

插入位置：第 6 步代码之后，紧接追加；完成稿约第 163 行。

本步声明：`func equalBoundary(a, b *string) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>a</code> | <code>*string</code> | 在此声明中的实际使用：<code>a == nil</code>；<code>a != nil</code>；<code>*a == *b</code> |
| <code>b</code> | <code>*string</code> | 在此声明中的实际使用：<code>b == nil</code>；<code>b != nil</code>；<code>*a == *b</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>a == nil &amp;&amp; b == nil &#124;&#124; a != nil &amp;&amp; b != nil &amp;&amp; *a == *b</code> |

**跟着本段代码走一遍：**

- 此处通过 <code>return a == nil &amp;&amp; b == nil &#124;&#124; a != nil &amp;&amp; b != nil &amp;&amp; *a == *b</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 7 bytes=8373:8558 -->
````go
func equalBoundary(a, b *string) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}

// historical_range_witness.go：保留原实现，按业务合并文件。

````

## 第 8 步：historicalStructureWitness

插入位置：第 7 步代码之后，紧接追加；完成稿约第 169 行。

本步声明：`type historicalStructureWitness struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Original</code> | <code>mainControlContext</code> | 原结构变更控制上下文。 |
| <code>Install</code> | <code>mainInstallContext</code> | 原已提交安装上下文。 |
| <code>Preview</code> | <code>structurePreviewContext</code> | 原预览绑定的文档/单元事实。 |
| <code>Snapshot</code> | <code>structurePreviewSnapshot</code> | 原预览固定的主线和草稿正文。 |
| <code>Migration</code> | <code>domain.StructureMigration</code> | 使用 <code>domain.StructureMigration</code> 类型；其定义处的职责：这是系统内部固定候选与范围证据，不复制浏览器HTTP输入，也不预先分配正式单元身份。 |
| <code>Bytes</code> | <code>int</code> | 核验历史见证时累计的字节量，用于限制解析成本。 |

<!-- write-step: 8 bytes=8558:8785 -->
````go
type historicalStructureWitness struct {
	Original  mainControlContext
	Install   mainInstallContext
	Preview   structurePreviewContext
	Snapshot  structurePreviewSnapshot
	Migration domain.StructureMigration
	Bytes     int
}

````

## 第 9 步：readHistoricalStructureWitness

插入位置：第 8 步代码之后，紧接追加；完成稿约第 178 行。

本步声明：`func readHistoricalStructureWitness(ctx context.Context, tx *sql.Tx, operation string, document uint64, validator *codec.Validator) (historicalStructureWitness, error)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ctx</code> | <code>context.Context</code> | 此次调用的取消信号和截止时间，传入 SQL/RPC，使调用者取消后下游也能结束等待；它不携带可信用户身份。<br>在此声明中的实际使用：<code>repository.HistoricalStructureBinding(ctx, tx, operation)</code>；<code>repository.LockedStructureConfirmation(ctx, tx, result.Original.StructurePreview.Reference)</code>；<code>repository.StructureRow(ctx, tx, source.StructureID, domain.TextID(document))</code>；<code>repository.HistoricalManifestBinding(ctx, tx, document, result.Install.Manifest, result.Install.Structure, operation)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>tx</code> | <code>*sql.Tx</code> | 调用者已经开启的事务。沿用它才能让行锁和后续读写处于同一次提交；本函数不能另开连接替换它。<br>在此声明中的实际使用：<code>repository.HistoricalStructureBinding(ctx, tx, operation)</code>；<code>repository.LockedStructureConfirmation(ctx, tx, result.Original.StructurePreview.Reference)</code>；<code>repository.StructureRow(ctx, tx, source.StructureID, domain.TextID(document))</code>；<code>repository.HistoricalManifestBinding(ctx, tx, document, result.Install.Manifest, result.Install.Structure, operation)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>operation</code> | <code>string</code> | 在此声明中的实际使用：<code>repository.HistoricalStructureBinding(ctx, tx, operation)</code>；<code>repository.HistoricalManifestBinding(ctx, tx, document, result.Install.Manifest, result.Install.Structure, operation)</code> |
| <code>document</code> | <code>uint64</code> | 在此声明中的实际使用：<code>result.Original.Document != document</code>；<code>result.Preview.Document != document</code>；<code>domain.TextID(document)</code>；<code>repository.HistoricalManifestBinding(ctx, tx, document, result.Install.Manifest, result.Install.Structure, operation)</code><br>另有 1 处使用，继续按下方完整代码的顺序处理。 |
| <code>validator</code> | <code>*codec.Validator</code> | 冻结协议的结构校验器，先限制请求形状，再由业务核验身份、权限、版本和事务条件。<br>在此声明中的实际使用：<code>validateStructurePreviewSnapshot(validator, preview.Snapshot, result.Preview, reply)</code>；<code>domain.ParseStructureMigration(envelope.Migration, input, scopes, validator)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>historicalStructureWitness</code> | 实际返回表达式：<code>result</code> |
| <code>(匿名/未命名)</code> | <code>error</code> | 失败原因。nil 只表示本函数没有报告错误；非 nil 要沿错误路径处理，事务提交报错尤其不能直接当成已回滚。<br>实际返回表达式：<code>invalid</code>；<code>err</code>；<code>nil</code> |

**跟着本段代码走一遍：**

- 检查 <code>err == sql.ErrNoRows</code>。条件成立时执行 <code>return result, invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>fact.State != &quot;SUCCEEDED&quot; &#124;&#124; len(fact.Payload) &gt; 24&lt;&lt;20 &#124;&#124; len(fact.Original) &gt; 24&lt;&lt;20 &#124;&#124; validatePendingMainControlBinding(fact) != nil &#124;&#124; 		json.Unmarshal(fact.Original, &amp;result. …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return result, invalid</code>。当前函数在这里结束。
- 检查 <code>err == sql.ErrNoRows</code>。条件成立时执行 <code>return result, invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>preview.Binding.State != &quot;SUCCEEDED&quot; &#124;&#124; len(preview.Snapshot) &gt; 24&lt;&lt;20</code>。条件成立时执行 <code>return result, invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; result.Preview.Document != document &#124;&#124; result.Preview.Structure != result.Original.Structure</code>。条件成立时执行 <code>return result, invalid</code>。当前函数在这里结束。
- 检查 <code>result.Original.Intent.Structure == nil &#124;&#124; result.Original.Actor != result.Preview.Actor &#124;&#124; 		domain.ValidateStructureConfirmation(*result.Original.Intent.Structure, result.Preview.Intent.Input) != nil</code>。条件成立时执行 <code>return result, invalid</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(preview.Snapshot, &amp;envelope) != nil &#124;&#124; !bytes.Equal(envelope.Digest, result.Original.StructurePreview.Digest)</code>。条件成立时执行 <code>return result, invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil &#124;&#124; expires &lt;= 0 &#124;&#124; strconv.FormatInt(expires, 10) != envelope.Expires</code>。条件成立时执行 <code>return result, invalid</code>。当前函数在这里结束。

<!-- write-step: 9 bytes=8785:11011 -->
````go
func readHistoricalStructureWitness(ctx context.Context, tx *sql.Tx, operation string, document uint64, validator *codec.Validator) (historicalStructureWitness, error) {
	var result historicalStructureWitness
	invalid := domain.Violation(domain.PermissionDenied, "HISTORICAL_RANGE_UNPROVEN")
	fact, err := repository.HistoricalStructureBinding(ctx, tx, operation)
	if err == sql.ErrNoRows {
		return result, invalid
	}
	if err != nil {
		return result, err
	}
	if fact.State != "SUCCEEDED" || len(fact.Payload) > 24<<20 || len(fact.Original) > 24<<20 || validatePendingMainControlBinding(fact) != nil ||
		json.Unmarshal(fact.Original, &result.Original) != nil || json.Unmarshal(fact.Payload, &result.Install) != nil || result.Original.Document != document || result.Original.StructurePreview == nil {
		return result, invalid
	}
	preview, err := repository.LockedStructureConfirmation(ctx, tx, result.Original.StructurePreview.Reference)
	if err == sql.ErrNoRows {
		return result, invalid
	}
	if err != nil {
		return result, err
	}
	if preview.Binding.State != "SUCCEEDED" || len(preview.Snapshot) > 24<<20 {
		return result, invalid
	}
	result.Preview, err = validateOriginalStructurePreviewBinding(preview.Binding)
	if err != nil || result.Preview.Document != document || result.Preview.Structure != result.Original.Structure {
		return result, invalid
	}
	if result.Original.Intent.Structure == nil || result.Original.Actor != result.Preview.Actor ||
		domain.ValidateStructureConfirmation(*result.Original.Intent.Structure, result.Preview.Intent.Input) != nil {
		return result, invalid
	}
	var envelope struct {
		Preview   string          `json:"preview_cut_ref"`
		Expires   string          `json:"expires_unix_ms"`
		Digest    []byte          `json:"input_digest"`
		Cut       json.RawMessage `json:"observed_cut"`
		Source    json.RawMessage `json:"source"`
		Migration json.RawMessage `json:"migration"`
	}
	if json.Unmarshal(preview.Snapshot, &envelope) != nil || !bytes.Equal(envelope.Digest, result.Original.StructurePreview.Digest) {
		return result, invalid
	}
	expires, err := strconv.ParseInt(envelope.Expires, 10, 64)
	if err != nil || expires <= 0 || strconv.FormatInt(expires, 10) != envelope.Expires {
````

## 第 10 步：readHistoricalStructureWitness（接着实现）

插入位置：第 9 步代码之后，紧接追加；完成稿约第 223 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return result, invalid</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>protojson.Unmarshal(envelope.Cut, cut) != nil</code>。条件成立时执行 <code>return result, invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, invalid</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(envelope.Source, &amp;source) != nil &#124;&#124; source.Manifest != domain.TextID(result.Preview.Manifest) &#124;&#124; source.StructureID != domain.TextID(result.Preview.Structure) &#124;&#124; 		s …</code>（长表达式，按本步完整代码继续）。条件成立时执行 <code>return result, invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>!equalHistoricalJSON(actual, source.Structure)</code>。条件成立时执行 <code>return result, invalid</code>。当前函数在这里结束。
- 检查 <code>entry.UnitId == nil</code>。条件成立时执行 <code>continue</code>。跳过当前循环本轮剩余语句。
- 检查 <code>err != nil &#124;&#124; actualUnits&#91;unit&#93; != 0</code>。条件成立时执行 <code>return result, invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, invalid</code>。当前函数在这里结束。

<!-- write-step: 10 bytes=11011:12859 -->
````go
		return result, invalid
	}
	cut := &pb.CutVector{}
	if protojson.Unmarshal(envelope.Cut, cut) != nil {
		return result, invalid
	}
	// 到期只影响原预览是否还能确认；已成功产生的不可变范围事实不会因短期准入到期失效。
	reply := &pb.DraftPreviewReply{State: pb.CaptureReply_FIXED, PreviewCutRef: envelope.Preview, MainManifestId: result.Preview.Manifest,
		ObservedCut: cut, ExpiresUnixMs: expires, InputDigest: envelope.Digest}
	result.Snapshot, err = validateStructurePreviewSnapshot(validator, preview.Snapshot, result.Preview, reply)
	if err != nil {
		return result, invalid
	}
	var source struct {
		Manifest    string                `json:"manifest_id"`
		StructureID string                `json:"structure_id"`
		Style       string                `json:"style_id"`
		Control     string                `json:"control_revision"`
		Structure   dto.StructureRevision `json:"structure"`
	}
	if json.Unmarshal(envelope.Source, &source) != nil || source.Manifest != domain.TextID(result.Preview.Manifest) || source.StructureID != domain.TextID(result.Preview.Structure) ||
		source.Style != domain.TextID(result.Preview.Style) || source.Control != domain.TextID(result.Preview.Control) {
		return result, invalid
	}
	actual, err := repository.StructureRow(ctx, tx, source.StructureID, domain.TextID(document))
	if err != nil {
		return result, err
	}
	if !equalHistoricalJSON(actual, source.Structure) {
		return result, invalid
	}
	actualUnits := map[uint64]uint64{}
	for _, entry := range actual.Entries {
		if entry.UnitId == nil {
			continue
		}
		unit, err := domain.CanonicalID(*entry.UnitId)
		if err != nil || actualUnits[unit] != 0 {
			return result, invalid
		}
		chapter, err := domain.CanonicalID(entry.ChapterId)
		if err != nil {
			return result, invalid
		}
		actualUnits[unit] = chapter
	}
````

## 第 11 步：readHistoricalStructureWitness（接着实现）

插入位置：第 10 步代码之后，紧接追加；完成稿约第 269 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>len(actualUnits) != len(result.Preview.Units)</code>。条件成立时执行 <code>return result, invalid</code>。当前函数在这里结束。
- 检查 <code>actualUnits&#91;unit.Unit&#93; != unit.Chapter</code>。条件成立时执行 <code>return result, invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>!valid</code>。条件成立时执行 <code>return result, invalid</code>。当前函数在这里结束。
- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, err</code>。当前函数在这里结束。
- 检查 <code>len(manifest) != len(result.Install.Units)</code>。条件成立时执行 <code>return result, invalid</code>。当前函数在这里结束。
- 检查 <code>installed&#91;unit&#91;&quot;unit_id&quot;&#93;&#93; != unit&#91;&quot;content_version_id&quot;&#93;</code>。条件成立时执行 <code>return result, invalid</code>。当前函数在这里结束。
- 检查 <code>json.Unmarshal(envelope.Source, &amp;input) != nil</code>。条件成立时执行 <code>return result, invalid</code>。当前函数在这里结束。
- 检查 <code>!equalHistoricalJSON(input, result.Preview.Intent.Input)</code>。条件成立时执行 <code>return result, invalid</code>。当前函数在这里结束。

<!-- write-step: 11 bytes=12859:14342 -->
````go
	if len(actualUnits) != len(result.Preview.Units) {
		return result, invalid
	}
	for _, unit := range result.Preview.Units {
		if actualUnits[unit.Unit] != unit.Chapter {
			return result, invalid
		}
	}
	valid, err := repository.HistoricalManifestBinding(ctx, tx, document, result.Install.Manifest, result.Install.Structure, operation)
	if err != nil {
		return result, err
	}
	if !valid {
		return result, invalid
	}
	manifest, err := repository.ManifestUnits(ctx, tx, domain.TextID(result.Install.Manifest), document)
	if err != nil {
		return result, err
	}
	if len(manifest) != len(result.Install.Units) {
		return result, invalid
	}
	installed := map[string]string{}
	for _, unit := range result.Install.Units {
		installed[domain.TextID(unit.UnitId)] = domain.TextID(unit.ContentVersionId)
	}
	for _, unit := range manifest {
		if installed[unit["unit_id"]] != unit["content_version_id"] {
			return result, invalid
		}
	}
	var input dto.StructurePreviewCreate
	if json.Unmarshal(envelope.Source, &input) != nil {
		return result, invalid
	}
	input.OperationId = result.Preview.Intent.Operation
	input.ExpectedStructureId = source.StructureID
	input.ExpectedControlRevision = source.Control
	if !equalHistoricalJSON(input, result.Preview.Intent.Input) {
		return result, invalid
	}
	scopes := []domain.StructureMigrationSource{}
	for _, unit := range result.Preview.Units {
		scopes = append(scopes, domain.StructureMigrationSource{Unit: unit.Unit, Chapter: unit.Chapter})
	}
````

## 第 12 步：readHistoricalStructureWitness（接着实现）

插入位置：第 11 步代码之后，紧接追加；完成稿约第 314 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 检查 <code>err != nil</code>。条件成立时执行 <code>return result, invalid</code>。当前函数在这里结束。
- 此处通过 <code>return result, nil</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=14342:14591 -->
````go
	result.Migration, err = domain.ParseStructureMigration(envelope.Migration, input, scopes, validator)
	if err != nil {
		return result, invalid
	}
	result.Bytes = len(fact.Original) + len(fact.Payload) + len(preview.Snapshot)
	return result, nil
}

````

## 第 13 步：equalHistoricalJSON

插入位置：第 12 步代码之后，紧接追加；完成稿约第 322 行。

本步声明：`func equalHistoricalJSON(a, b any) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>a</code> | <code>any</code> | 在此声明中的实际使用：<code>json.Marshal(a)</code> |
| <code>b</code> | <code>any</code> | 在此声明中的实际使用：<code>json.Marshal(b)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>leftErr == nil &amp;&amp; rightErr == nil &amp;&amp; bytes.Equal(left, right)</code> |

**跟着本段代码走一遍：**

- 检查 <code>leftErr != nil &#124;&#124; rightErr != nil</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 此处通过 <code>return leftErr == nil &amp;&amp; rightErr == nil &amp;&amp; bytes.Equal(left, right)</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=14591:15069 -->
````go
func equalHistoricalJSON(a, b any) bool {
	left, leftErr := json.Marshal(a)
	right, rightErr := json.Marshal(b)
	if leftErr != nil || rightErr != nil {
		return false
	}
	leftValue, leftErr := codec.Decode(left)
	rightValue, rightErr := codec.Decode(right)
	if leftErr != nil || rightErr != nil {
		return false
	}
	left, leftErr = codec.Canonical(leftValue)
	right, rightErr = codec.Canonical(rightValue)
	return leftErr == nil && rightErr == nil && bytes.Equal(left, right)
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/logic/document/historical_ranges_service.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/04-structure.md)。
