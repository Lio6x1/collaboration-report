# services/document/internal/logic/document/historical_ranges_service.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/206-historical-ranges-service.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：编排用例、权限、事务及未知结果恢复；跨服务调用不能替代持久事实。

主要声明：`historicalRangeMapping`、`proveHistoricalMapping`、`actualHistoricalVersion`、`equalBoundary`、`historicalStructureWitness`、`readHistoricalStructureWitness`、`equalHistoricalJSON`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

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
			matched = true
		}
	}
	return edge, matched
}

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

func equalBoundary(a, b *string) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}

// historical_range_witness.go：保留原实现，按业务合并文件。

type historicalStructureWitness struct {
	Original  mainControlContext
	Install   mainInstallContext
	Preview   structurePreviewContext
	Snapshot  structurePreviewSnapshot
	Migration domain.StructureMigration
	Bytes     int
}

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
	result.Migration, err = domain.ParseStructureMigration(envelope.Migration, input, scopes, validator)
	if err != nil {
		return result, invalid
	}
	result.Bytes = len(fact.Original) + len(fact.Payload) + len(preview.Snapshot)
	return result, nil
}

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

UTF-8/LF全文SHA-256：`df7560d3bbbff26501dd3ac5b85b4488704d0de27c8af49eb0faab0c903c913c`。[参考实现](../../../../services/document/internal/logic/document/historical_ranges_service.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
