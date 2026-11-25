# services/document/internal/domain/document/operations_models.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/096-operations-models.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：定义模型、输入输出和纯规则；把状态与副作用分开。

主要声明：`SameStableUnitRange`、`HistoricalRangeEdge`、`historicalRange`、`historicalRangeKey`、`HistoricalRangeCovered`、`completeHistoricalPartition`、`historicalBoundaryIndex`、`boundaryText`、`OperationReceipt`、`OperationRecord`、`ReadAdmission`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	dto "collaboration/contracts/httpdto"
	"slices"
	"sort"
)

// historical_range.go：保留原实现，按业务合并文件。

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

// 仅接纳已核验实际版本/预览/退役事实的边；块是否仍存在不是历史整单元的授权证明。
type HistoricalRangeEdge struct {
	Source, Target                     uint64
	Kind                               string
	Order                              int
	Start, End, TargetStart, TargetEnd *string
	Heads                              []string
	TargetInterval                     bool
}
type historicalRange struct {
	whole      bool
	start, end *string
}
type historicalRangeKey struct {
	unit             uint64
	whole            bool
	start, end       string
	startNil, endNil bool
}

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
	return true
}
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
func boundaryText(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// operation.go：保留原实现，按业务合并文件。

// OperationReceipt保存原号/发起者/请求指纹与固定结果，原结果不等于新动作资格。
type OperationReceipt struct {
	ActorID             uint64
	Kind                string
	Fingerprint, Result []byte
}
type OperationRecord struct {
	ID                                  string
	ActorID                             uint64
	Kind                                string
	TargetJSON, Fingerprint, ResultJSON []byte
	State, IdentityAdmissionRef         string
}

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

UTF-8/LF全文SHA-256：`65495203bc10b99bc6c10ed4b4548951b99f04151b15b12ba5d2765ed6879a3e`。[参考实现](../../../../services/document/internal/domain/document/operations_models.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
