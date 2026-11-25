# 分步编写 `services/document/internal/domain/document/operations_models.go`

先按 [D03 · 固定主线操作与支线更新](../lessons/document/03-main-draft.md) 的场景和创建顺序学习，本页提供该文件的逐段落笔位置。完整文件答案在[章末核对](../source/096-operations-models.md)。所有相对文件路径都相对于自己的同一个学习工程。

## 为什么创建这个文件

定义操作回执和读取准入，并计算结构改变后历史单元范围是否仍完整受权。

单元ID相同不代表覆盖的内容范围相同；拆章合章后必须有真实分区边证明原范围。

写之前先按这条执行路线理解各段的连接：

1. SameStableUnitRange核固定节点、章、父节点与拆分模式
2. HistoricalRangeCovered遍历已核验转换边
3. completeHistoricalPartition检查边界完整分区。

本文件需要守住的边界：

- 块仍存在不是历史整单元访问证明
- 回执成功只是原结果，不授予新动作资格。

## 创建文件与录入规则

```powershell
# 工作目录：00 设置的自己的 lessonRoot；后续不换工程。
Set-Location -LiteralPath $lessonRoot
New-Item -ItemType Directory -Path 'services/document/internal/domain/document' -Force | Out-Null
if ((Test-Path -LiteralPath 'services/document/internal/domain/document/operations_models.go') -and (Get-Item -LiteralPath 'services/document/internal/domain/document/operations_models.go').Length -gt 0) { throw '文件已有内容，先核对当前学习进度，勿覆盖' }
if (-not (Test-Path -LiteralPath 'services/document/internal/domain/document/operations_models.go')) { New-Item -ItemType File -Path 'services/document/internal/domain/document/operations_models.go' | Out-Null }
```

本页是唯一录入清单：讲义中的同名声明用于理解，不提前另外插入；章末答案用于核对，不再次追加。文件已有部分内容时，跳过上面的创建块，对照最后完成的步骤继续追加；不要从第1步重写。以下片段按顺序写进这一份文件。第 1 步放在文件开头，其余步骤紧接上一步末尾；不重复输入 package/import，不为未写完的函数提前补右括号。长函数按真实语句边界分段，结束前暂不格式化或编译。片段首尾空行也保留；没有隐藏的省略实现。

## 第 1 步：package 与 import / 文件头

插入位置：空文件第 1 行。

<!-- write-step: 1 bytes=0:253 -->
````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	dto "collaboration/contracts/httpdto"
	"slices"
	"sort"
)

// historical_range.go：保留原实现，按业务合并文件。

````

## 第 2 步：SameStableUnitRange

插入位置：第 1 步代码之后，紧接追加；完成稿约第 12 行。

相同unit ID只是起点：固定节点、章、父节点、种类和拆分模式都必须证明仍覆盖同一完整范围。

本步声明：`func SameStableUnitRange(source, target dto.StructureRevision, unit string) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>source</code> | <code>dto.StructureRevision</code> | 在此声明中的实际使用：<code>source.DocumentId</code>；<code>find(source)</code> |
| <code>target</code> | <code>dto.StructureRevision</code> | 在此声明中的实际使用：<code>target.DocumentId</code>；<code>find(target)</code> |
| <code>unit</code> | <code>string</code> | 在此声明中的实际使用：<code>*entry.UnitId == unit</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>a.NodeId == b.NodeId &amp;&amp; a.ChapterId == b.ChapterId &amp;&amp; a.Kind == b.Kind &amp;&amp; 		parentID(a.ParentNodeId) == parentID(b.ParentNodeId) &amp;&amp; a.SplitLevel2 == b.SplitLevel2 &amp;&amp; 		ap.NodeId == …</code>（长表达式，按本步完整代码继续） |

**跟着本段代码走一遍：**

- 检查 <code>source.DocumentId == &quot;&quot; &#124;&#124; source.DocumentId != target.DocumentId</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>entry.UnitId != nil &amp;&amp; *entry.UnitId == unit</code>。
- 在本段的匿名函数内：检查 <code>count != 1</code>。条件成立时执行 <code>return found, parent, false</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>found.Kind == &quot;CHAPTER&quot;</code>。
- 在本段的匿名函数内：检查 <code>found.ParentNodeId != nil &#124;&#124; found.SplitLevel2</code>。条件成立时执行 <code>return found, parent, false</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>entry.ParentNodeId != nil &amp;&amp; *entry.ParentNodeId == found.NodeId</code>。条件成立时执行 <code>return found, parent, false</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return found, parent, true</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>found.ParentNodeId == nil &#124;&#124; (found.Kind != &quot;SECTION&quot; &amp;&amp; found.Kind != &quot;INTRODUCTION&quot;)</code>。条件成立时执行 <code>return found, parent, false</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>entry.NodeId == *found.ParentNodeId</code>。
- 在本段的匿名函数内：此处通过 <code>return found, parent, parent.Kind == &quot;CHAPTER&quot; &amp;&amp; parent.ParentNodeId == nil &amp;&amp; parent.SplitLevel2 &amp;&amp; parent.UnitId == nil &amp;&amp; parent.ChapterId == found.ChapterId</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>!ok</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。

<!-- write-step: 2 bytes=253:1736 -->
````go
// 相同unit ID只是起点：固定节点、章、父节点、种类和拆分模式都必须证明仍覆盖同一完整范围。
func SameStableUnitRange(source, target dto.StructureRevision, unit string) bool {
	if source.DocumentId == "" || source.DocumentId != target.DocumentId {
		return false
	}
	find := func(structure dto.StructureRevision) (dto.StructureEntry, dto.StructureEntry, bool) {
		var found, parent dto.StructureEntry
		count := 0
		for _, entry := range structure.Entries {
			if entry.UnitId != nil && *entry.UnitId == unit {
				found = entry
				count++
			}
		}
		if count != 1 {
			return found, parent, false
		}
		if found.Kind == "CHAPTER" {
			if found.ParentNodeId != nil || found.SplitLevel2 {
				return found, parent, false
			}
			for _, entry := range structure.Entries {
				if entry.ParentNodeId != nil && *entry.ParentNodeId == found.NodeId {
					return found, parent, false
				}
			}
			return found, parent, true
		}
		if found.ParentNodeId == nil || (found.Kind != "SECTION" && found.Kind != "INTRODUCTION") {
			return found, parent, false
		}
		for _, entry := range structure.Entries {
			if entry.NodeId == *found.ParentNodeId {
				parent = entry
			}
		}
		return found, parent, parent.Kind == "CHAPTER" && parent.ParentNodeId == nil && parent.SplitLevel2 && parent.UnitId == nil && parent.ChapterId == found.ChapterId
	}
	a, ap, ok := find(source)
	if !ok {
		return false
	}
	b, bp, ok := find(target)
	if !ok {
		return false
	}
````

## 第 3 步：SameStableUnitRange（接着实现）

插入位置：第 2 步代码之后，紧接追加；完成稿约第 58 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：检查 <code>value == nil</code>。条件成立时执行 <code>return &quot;&quot;</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return *value</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return a.NodeId == b.NodeId &amp;&amp; a.ChapterId == b.ChapterId &amp;&amp; a.Kind == b.Kind &amp;&amp; 		parentID(a.ParentNodeId) == parentID(b.ParentNodeId) &amp;&amp; a.SplitLevel2 == b.SplitLevel2 &amp;&amp; 		ap.No …</code>（长表达式，按本步完整代码继续） 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 3 bytes=1736:2179 -->
````go
	parentID := func(value *string) string {
		if value == nil {
			return ""
		}
		return *value
	}
	return a.NodeId == b.NodeId && a.ChapterId == b.ChapterId && a.Kind == b.Kind &&
		parentID(a.ParentNodeId) == parentID(b.ParentNodeId) && a.SplitLevel2 == b.SplitLevel2 &&
		ap.NodeId == bp.NodeId && ap.ChapterId == bp.ChapterId && ap.SplitLevel2 == bp.SplitLevel2
}

// historical_range_graph.go：保留原实现，按业务合并文件。

````

## 第 4 步：HistoricalRangeEdge

插入位置：第 3 步代码之后，紧接追加；完成稿约第 71 行。

仅接纳已核验实际版本/预览/退役事实的边；块是否仍存在不是历史整单元的授权证明。

本步声明：`type HistoricalRangeEdge struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>Source</code> | <code>uint64</code> | 原历史单元ID |
| <code>Target</code> | <code>uint64</code> | 迁移后单元ID |
| <code>Kind</code> | <code>string</code> | 整单元或区间转换类别 |
| <code>Order</code> | <code>int</code> | 分区顺序，辅助证明覆盖没有缺口 |
| <code>Start</code> | <code>*string</code> | 源范围开始二级标题边界，nil代表开头 |
| <code>End</code> | <code>*string</code> | 源范围结束边界，nil代表末尾 |
| <code>TargetStart</code> | <code>*string</code> | 目标中对应区间起点 |
| <code>TargetEnd</code> | <code>*string</code> | 目标中对应区间终点 |
| <code>Heads</code> | <code>&#91;&#93;string</code> | 实际原正文的顶层二级标题顺序 |
| <code>TargetInterval</code> | <code>bool</code> | 目标映射是否只覆盖区间而非整单元 |

<!-- write-step: 4 bytes=2179:2591 -->
````go
// 仅接纳已核验实际版本/预览/退役事实的边；块是否仍存在不是历史整单元的授权证明。
type HistoricalRangeEdge struct {
	Source, Target                     uint64
	Kind                               string
	Order                              int
	Start, End, TargetStart, TargetEnd *string
	Heads                              []string
	TargetInterval                     bool
}
````

## 第 5 步：historicalRange

插入位置：第 4 步代码之后，紧接追加；完成稿约第 80 行。

本步声明：`type historicalRange struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>whole</code> | <code>bool</code> | 是否要求整个单元范围 |
| <code>start</code> | <code>*string</code> | 区间起点，nil为正文开头 |
| <code>end</code> | <code>*string</code> | 区间终点，nil为正文结尾 |

<!-- write-step: 5 bytes=2591:2660 -->
````go
type historicalRange struct {
	whole      bool
	start, end *string
}
````

## 第 6 步：historicalRangeKey

插入位置：第 5 步代码之后，紧接追加；完成稿约第 84 行。

本步声明：`type historicalRangeKey struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>unit</code> | <code>uint64</code> | 历史遍历到的单元ID |
| <code>whole</code> | <code>bool</code> | 整单元访问状态 |
| <code>start</code> | <code>string</code> | 标准化的起点文本 |
| <code>end</code> | <code>string</code> | 标准化的终点文本 |
| <code>startNil</code> | <code>bool</code> | 区分未设边界与空字符串 |
| <code>endNil</code> | <code>bool</code> | 区分未设终点与空字符串 |

<!-- write-step: 6 bytes=2660:2792 -->
````go
type historicalRangeKey struct {
	unit             uint64
	whole            bool
	start, end       string
	startNil, endNil bool
}

````

## 第 7 步：HistoricalRangeCovered

插入位置：第 6 步代码之后，紧接追加；完成稿约第 91 行。

本步声明：`func HistoricalRangeCovered(edit map[uint64]bool, original uint64, current map[uint64]bool, edges []HistoricalRangeEdge) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>edit</code> | <code>map&#91;uint64&#93;bool</code> | 在此声明中的实际使用：<code>allowed = edit&#91;unit&#93;</code> |
| <code>original</code> | <code>uint64</code> | 在此声明中的实际使用：<code>original == 0</code>；<code>cover(original, historicalRange{whole: true})</code> |
| <code>current</code> | <code>map&#91;uint64&#93;bool</code> | 在此声明中的实际使用：<code>cover = func(unit uint64, scope historicalRange) bool { 		// 块ID可能包含分隔符，结构化key避免把不同区间拼成同一个缓存键。 		key := historicalRangeKey{unit: unit, whole: scope.whole, start: boundaryText(scope …</code>（长表达式，按本步完整代码继续） |
| <code>edges</code> | <code>&#91;&#93;HistoricalRangeEdge</code> | 使用 <code>HistoricalRangeEdge</code> 类型；其定义处的职责：仅接纳已核验实际版本/预览/退役事实的边；块是否仍存在不是历史整单元的授权证明。<br>在此声明中的实际使用：<code>len(edges)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>cover(original, historicalRange{whole: true})</code> |

**跟着本段代码走一遍：**

- 检查 <code>original == 0 &#124;&#124; len(edges) &gt; 100000</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>known&#91;key&#93;</code>。条件成立时执行 <code>return memo&#91;key&#93;</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>visiting&#91;key&#93; &#124;&#124; states &gt;= 100000 &#124;&#124; len(visiting) &gt;= 512</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 在本段的匿名函数内：登记 <code>delete(visiting, key)</code>，当前函数退出时执行，提前 return 也会触发。
- 在本段的匿名函数内：登记 <code>func() { known&#91;key&#93; = true; memo&#91;key&#93; = allowed }()</code>，当前函数退出时执行，提前 return 也会触发。
- 在本段的匿名函数内：检查 <code>current&#91;unit&#93;</code>。
- 在本段的匿名函数内：此处通过 <code>return allowed</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>len(outgoing) == 0</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>edge.Target == 0 &#124;&#124; edge.Target == unit &#124;&#124; edge.Source != unit &#124;&#124; edge.Kind != outgoing&#91;0&#93;.Kind</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>len(outgoing) != 1</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。

<!-- write-step: 7 bytes=2792:4463 -->
````go
func HistoricalRangeCovered(edit map[uint64]bool, original uint64, current map[uint64]bool, edges []HistoricalRangeEdge) bool {
	if original == 0 || len(edges) > 100000 {
		return false
	}
	bySource := map[uint64][]HistoricalRangeEdge{}
	for _, edge := range edges {
		bySource[edge.Source] = append(bySource[edge.Source], edge)
	}
	visiting, memo, known := map[historicalRangeKey]bool{}, map[historicalRangeKey]bool{}, map[historicalRangeKey]bool{}
	states := 0
	var cover func(uint64, historicalRange) bool
	cover = func(unit uint64, scope historicalRange) bool {
		// 块ID可能包含分隔符，结构化key避免把不同区间拼成同一个缓存键。
		key := historicalRangeKey{unit: unit, whole: scope.whole, start: boundaryText(scope.start), end: boundaryText(scope.end), startNil: scope.start == nil, endNil: scope.end == nil}
		if known[key] {
			return memo[key]
		}
		if visiting[key] || states >= 100000 || len(visiting) >= 512 {
			return false
		}
		states++
		visiting[key] = true
		defer delete(visiting, key)
		allowed := false
		defer func() { known[key] = true; memo[key] = allowed }()
		if current[unit] {
			allowed = edit[unit]
			return allowed
		}
		outgoing := bySource[unit]
		if len(outgoing) == 0 {
			return false
		}
		for _, edge := range outgoing {
			if edge.Target == 0 || edge.Target == unit || edge.Source != unit || edge.Kind != outgoing[0].Kind {
				return false
			}
		}
		switch outgoing[0].Kind {
		case "JOIN_CHAPTER":
			if len(outgoing) != 1 {
				return false
			}
			edge := outgoing[0]
			// 整个当前合章的EDIT可以覆盖旧节；只有目标区间也有实际证明时，才沿后续拆分缩小到该节。
````

## 第 8 步：HistoricalRangeCovered（接着实现）

插入位置：第 7 步代码之后，紧接追加；完成稿约第 136 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：检查 <code>cover(edge.Target, historicalRange{whole: true})</code>。
- 在本段的匿名函数内：此处通过 <code>return true</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>!scope.whole &#124;&#124; !edge.TargetInterval</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return allowed</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：此处通过 <code>return ordered&#91;i&#93;.Order &lt; ordered&#91;j&#93;.Order</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>!completeHistoricalPartition(ordered)</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>!cover(edge.Target, historicalRange{whole: true})</code>。
- 在本段的匿名函数内：检查 <code>all</code>。
- 在本段的匿名函数内：此处通过 <code>return true</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 在本段的匿名函数内：检查 <code>scope.whole</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>!ok</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>!ok &#124;&#124; start &gt;= end</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 在本段的匿名函数内：检查 <code>i == 0</code>。
- 在本段的匿名函数内：检查 <code>begin &lt; end &amp;&amp; finish &gt; start</code>。

<!-- write-step: 8 bytes=4463:5708 -->
````go
			if cover(edge.Target, historicalRange{whole: true}) {
				allowed = true
				return true
			}
			if !scope.whole || !edge.TargetInterval {
				return false
			}
			allowed = cover(edge.Target, historicalRange{start: edge.TargetStart, end: edge.TargetEnd})
			return allowed
		case "SPLIT_LEVEL2":
			ordered := slices.Clone(outgoing)
			sort.Slice(ordered, func(i, j int) bool { return ordered[i].Order < ordered[j].Order })
			if !completeHistoricalPartition(ordered) {
				return false
			}
			all := true
			for _, edge := range ordered {
				if !cover(edge.Target, historicalRange{whole: true}) {
					all = false
				}
			}
			if all {
				allowed = true
				return true
			}
			if scope.whole {
				return false
			}
			start, ok := historicalBoundaryIndex(ordered[0].Heads, scope.start, false)
			if !ok {
				return false
			}
			end, ok := historicalBoundaryIndex(ordered[0].Heads, scope.end, true)
			if !ok || start >= end {
				return false
			}
			selected := 0
			for i, edge := range ordered {
				begin, _ := historicalBoundaryIndex(edge.Heads, edge.Start, false)
				finish, _ := historicalBoundaryIndex(edge.Heads, edge.End, true)
				if i == 0 {
					begin = -1
				}
				if begin < end && finish > start {
					selected++
````

## 第 9 步：HistoricalRangeCovered（接着实现）

插入位置：第 8 步代码之后，紧接追加；完成稿约第 181 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 在本段的匿名函数内：检查 <code>!cover(edge.Target, historicalRange{whole: true})</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 在本段的匿名函数内：此处通过 <code>return allowed</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 此处通过 <code>return cover(original, historicalRange{whole: true})</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 9 bytes=5708:5940 -->
````go
					if !cover(edge.Target, historicalRange{whole: true}) {
						return false
					}
				}
			}
			allowed = selected > 0
			return allowed
		default:
			return false
		}
	}
	return cover(original, historicalRange{whole: true})
}

````

## 第 10 步：completeHistoricalPartition

插入位置：第 9 步代码之后，紧接追加；完成稿约第 195 行。

本步声明：`func completeHistoricalPartition(edges []HistoricalRangeEdge) bool`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>edges</code> | <code>&#91;&#93;HistoricalRangeEdge</code> | 使用 <code>HistoricalRangeEdge</code> 类型；其定义处的职责：仅接纳已核验实际版本/预览/退役事实的边；块是否仍存在不是历史整单元的授权证明。<br>在此声明中的实际使用：<code>edges&#91;0&#93;.Heads</code>；<code>edges&#91;0&#93;.Start</code>；<code>len(edges)</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>false</code>；<code>true</code> |

**跟着本段代码走一遍：**

- 检查 <code>len(heads) == 0</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>head == &quot;&quot; &#124;&#124; seen&#91;head&#93;</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>introduction</code>。
- 检查 <code>len(edges) != expected</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>edge.Order != i &#124;&#124; targets&#91;edge.Target&#93; &#124;&#124; !slices.Equal(heads, edge.Heads)</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>introduction &amp;&amp; i == 0</code>。
- 检查 <code>edge.End == nil &#124;&#124; *edge.End != heads&#91;0&#93;</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>introduction</code>。
- 检查 <code>edge.Start == nil &#124;&#124; *edge.Start != heads&#91;section&#93;</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>section == len(heads)-1</code>。
- 检查 <code>edge.End != nil</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。
- 检查 <code>edge.End == nil &#124;&#124; *edge.End != heads&#91;section+1&#93;</code>。条件成立时执行 <code>return false</code>。当前函数在这里结束。

<!-- write-step: 10 bytes=5940:6954 -->
````go
func completeHistoricalPartition(edges []HistoricalRangeEdge) bool {
	heads := edges[0].Heads
	if len(heads) == 0 {
		return false
	}
	seen := map[string]bool{}
	for _, head := range heads {
		if head == "" || seen[head] {
			return false
		}
		seen[head] = true
	}
	introduction := edges[0].Start == nil
	expected := len(heads)
	if introduction {
		expected++
	}
	if len(edges) != expected {
		return false
	}
	targets := map[uint64]bool{}
	for i, edge := range edges {
		if edge.Order != i || targets[edge.Target] || !slices.Equal(heads, edge.Heads) {
			return false
		}
		targets[edge.Target] = true
		if introduction && i == 0 {
			if edge.End == nil || *edge.End != heads[0] {
				return false
			}
			continue
		}
		section := i
		if introduction {
			section--
		}
		if edge.Start == nil || *edge.Start != heads[section] {
			return false
		}
		if section == len(heads)-1 {
			if edge.End != nil {
				return false
			}
		} else if edge.End == nil || *edge.End != heads[section+1] {
			return false
		}
	}
````

## 第 11 步：completeHistoricalPartition（接着实现）

插入位置：第 10 步代码之后，紧接追加；完成稿约第 242 行。

继续同一个声明，沿用上一步的参数和局部变量，不重新声明函数。

**跟着本段代码走一遍：**

- 此处通过 <code>return true</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 11 bytes=6954:6969 -->
````go
	return true
}
````

## 第 12 步：historicalBoundaryIndex

插入位置：第 11 步代码之后，紧接追加；完成稿约第 244 行。

本步声明：`func historicalBoundaryIndex(heads []string, boundary *string, end bool) (int, bool)`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>heads</code> | <code>&#91;&#93;string</code> | 在此声明中的实际使用：<code>len(heads)</code> |
| <code>boundary</code> | <code>*string</code> | 在此声明中的实际使用：<code>boundary == nil</code>；<code>head == *boundary</code> |
| <code>end</code> | <code>bool</code> | 是否解析结束边界；boundary为nil时结束边界用len(heads)，起始边界用-1，保留标题前引言范围。 |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>int</code> | 实际返回表达式：<code>len(heads)</code>；<code>-1</code>；<code>i</code>；<code>0</code> |
| <code>(匿名/未命名)</code> | <code>bool</code> | 实际返回表达式：<code>true</code>；<code>false</code> |

**跟着本段代码走一遍：**

- 检查 <code>boundary == nil</code>。
- 检查 <code>end</code>。条件成立时执行 <code>return len(heads), true</code>。当前函数在这里结束。
- 此处通过 <code>return -1, true</code> 结束当前函数；注意返回的是哪一个阶段的结果。
- 检查 <code>head == *boundary</code>。条件成立时执行 <code>return i, true</code>。当前函数在这里结束。
- 此处通过 <code>return 0, false</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 12 bytes=6969:7240 -->
````go
func historicalBoundaryIndex(heads []string, boundary *string, end bool) (int, bool) {
	if boundary == nil {
		if end {
			return len(heads), true
		}
		return -1, true
	}
	for i, head := range heads {
		if head == *boundary {
			return i, true
		}
	}
	return 0, false
}
````

## 第 13 步：boundaryText

插入位置：第 12 步代码之后，紧接追加；完成稿约第 258 行。

本步声明：`func boundaryText(value *string) string`。


输入参数：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>value</code> | <code>*string</code> | 在此声明中的实际使用：<code>value == nil</code>；<code>return *value</code> |


返回值（从左到右）：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>(匿名/未命名)</code> | <code>string</code> | 实际返回表达式：<code>&quot;&quot;</code>；<code>*value</code> |

**跟着本段代码走一遍：**

- 检查 <code>value == nil</code>。条件成立时执行 <code>return &quot;&quot;</code>。当前函数在这里结束。
- 此处通过 <code>return *value</code> 结束当前函数；注意返回的是哪一个阶段的结果。

<!-- write-step: 13 bytes=7240:7396 -->
````go
func boundaryText(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// operation.go：保留原实现，按业务合并文件。

````

## 第 14 步：OperationReceipt

插入位置：第 13 步代码之后，紧接追加；完成稿约第 267 行。

OperationReceipt保存原号/发起者/请求指纹与固定结果，原结果不等于新动作资格。

本步声明：`type OperationReceipt struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ActorID</code> | <code>uint64</code> | 原操作发起人 |
| <code>Kind</code> | <code>string</code> | 原业务动作种类 |
| <code>Fingerprint</code> | <code>&#91;&#93;byte</code> | 路径和规范输入的摘要 |
| <code>Result</code> | <code>&#91;&#93;byte</code> | 固定业务结果JSON，敏感展示仍需当前授权 |

<!-- write-step: 14 bytes=7396:7622 -->
````go
// OperationReceipt保存原号/发起者/请求指纹与固定结果，原结果不等于新动作资格。
type OperationReceipt struct {
	ActorID             uint64
	Kind                string
	Fingerprint, Result []byte
}
````

## 第 15 步：OperationRecord

插入位置：第 14 步代码之后，紧接追加；完成稿约第 273 行。

本步声明：`type OperationRecord struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ID</code> | <code>string</code> | 业务幂等操作号 |
| <code>ActorID</code> | <code>uint64</code> | 原发起用户 |
| <code>Kind</code> | <code>string</code> | 业务动作类型 |
| <code>TargetJSON</code> | <code>&#91;&#93;byte</code> | 原目标绑定JSON |
| <code>Fingerprint</code> | <code>&#91;&#93;byte</code> | 规范请求指纹 |
| <code>ResultJSON</code> | <code>&#91;&#93;byte</code> | 固定公开业务结果 |
| <code>State</code> | <code>string</code> | 回执阶段 |
| <code>IdentityAdmissionRef</code> | <code>string</code> | 登记此原号的账号准入引用 |

<!-- write-step: 15 bytes=7622:7875 -->
````go
type OperationRecord struct {
	ID                                  string
	ActorID                             uint64
	Kind                                string
	TargetJSON, Fingerprint, ResultJSON []byte
	State, IdentityAdmissionRef         string
}

````

## 第 16 步：ReadAdmission

插入位置：第 15 步代码之后，紧接追加；完成稿约第 281 行。

ReadAdmission只保存固定证明绑定；权限来源由另一个反向索引在原事务内登记。

本步声明：`type ReadAdmission struct {`。


字段或接口方法：

| 名称 | 类型/签名 | 含义、原因与实际使用 |
| --- | --- | --- |
| <code>ID</code> | <code>string</code> | 文档读取准入ID |
| <code>IdentityAdmissionRef</code> | <code>string</code> | Account权威准入引用 |
| <code>ActorID</code> | <code>uint64</code> | 真实用户ID |
| <code>SessionRef</code> | <code>&#91;&#93;byte</code> | 原会话不可逆引用 |
| <code>DocumentID</code> | <code>uint64</code> | 准入文档 |
| <code>Purpose</code> | <code>string</code> | 本次业务用途，禁止跨用途复用 |
| <code>CapabilitiesJSON</code> | <code>&#91;&#93;byte</code> | 允许能力集合 |
| <code>TargetJSON</code> | <code>&#91;&#93;byte</code> | 固定目标完整绑定 |
| <code>OperationID</code> | <code>string</code> | 所属操作号 |
| <code>ExpiresUnixMs</code> | <code>int64</code> | 准入绝对截止毫秒值 |

<!-- write-step: 16 bytes=7875:8307 -->
````go
// ReadAdmission只保存固定证明绑定；权限来源由另一个反向索引在原事务内登记。
type ReadAdmission struct {
	ID, IdentityAdmissionRef     string
	ActorID                      uint64
	SessionRef                   []byte
	DocumentID                   uint64
	Purpose                      string
	CapabilitiesJSON, TargetJSON []byte
	OperationID                  string
	ExpiresUnixMs                int64
}
````

## 文件完成后的检查

```powershell
gofmt -w 'services/document/internal/domain/document/operations_models.go'
if ($LASTEXITCODE -ne 0) { throw '本文件语法尚未写完整' }
```

gofmt 只检查当前文件语法并排版，不证明依赖、路由或业务测试通过。同包其他文件尚未完成时，先继续功能讲义；到讲义指定的包/阶段关口再 build/test。

若出现重复声明，检查是否把完整答案又追加了一遍；若文件结尾缺括号，回看最后一个分段。功能结果、参数语义和失败排查回到[本课讲义](../lessons/document/03-main-draft.md)。
