package document

import (
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"crypto/sha256"
	"encoding/json"
	"testing"
)

func historicalMappingFixture(t *testing.T) (repository.HistoricalUnitMapping, *pb.FixedContent, *pb.FixedContent, historicalStructureWitness) {
	t.Helper()
	start := "a"
	body := []byte(`{"content":[{"attrs":{"id":"a","level":2},"content":[{"text":"节","type":"text"}],"type":"heading"},{"attrs":{"id":"p"},"content":[{"text":"正文","type":"text"}],"type":"paragraph"}],"type":"doc"}`)
	targetBody := []byte(`{"content":[{"attrs":{"id":"p"},"content":[{"text":"正文","type":"text"}],"type":"paragraph"}],"type":"doc"}`)
	sourceHash, targetHash := sha256.Sum256(body), sha256.Sum256(targetBody)
	source := &pb.FixedContent{DocumentId: 1, UnitId: 2, StructureId: 3, OperationId: "confirm", BodyJsonUtf8: body, BodyHash: sourceHash[:], ByteSize: uint64(len(body)), SchemaVersion: "content-v1", CanonicalizationVersion: "canonical-v1", HashAlgorithm: "SHA-256"}
	target := &pb.FixedContent{DocumentId: 1, UnitId: 4, StructureId: 5, OperationId: "confirm", BodyJsonUtf8: targetBody, BodyHash: targetHash[:], ByteSize: uint64(len(targetBody)), SchemaVersion: "content-v1", CanonicalizationVersion: "canonical-v1", HashAlgorithm: "SHA-256"}
	segment := domain.StructureMigrationRange{Source: "2", Scope: "WHOLE_UNIT", Start: &start, MainBlocks: []string{"a", "p"}, DraftBlocks: []string{"a", "p"}}
	candidate := domain.StructureMigrationTarget{Key: "6:SPLIT:0", Kind: "SECTION", Main: targetBody, Ranges: []domain.StructureMigrationRange{segment}}
	source.CaptureId = "structure-source:2"
	target.CaptureId = "6:SPLIT:0"
	witness := historicalStructureWitness{Original: mainControlContext{Document: 1, Structure: 3, Intent: domain.MainControlIntent{Operation: "confirm"}, StructurePreview: &structureConfirmationBinding{Reference: "preview", Digest: make([]byte, 32)}},
		Install:   mainInstallContext{Structure: 5, Units: []*pb.ManifestUnit{{UnitId: 4, ContentVersionId: 8, Content: target}}, Transition: &structureInstallation{Branches: []structureInstalledBranch{{Key: candidate.Key, Unit: 4}}}},
		Snapshot:  structurePreviewSnapshot{Main: []*pb.FixedContent{{DocumentId: 1, UnitId: 2, StructureId: 3, BodyJsonUtf8: body, BodyHash: sourceHash[:]}}},
		Migration: domain.StructureMigration{Schema: "structure-migration-v1", Changes: []domain.StructureMigrationChange{{Type: "SPLIT_LEVEL2", Chapter: "6", Sources: []string{"2"}, Targets: []domain.StructureMigrationTarget{candidate}}}}}
	row := repository.HistoricalUnitMapping{UnitMapping: repository.UnitMapping{Source: 2, Target: 4, SourceVersion: 7, TargetVersion: 8, SourceStructure: 3, TargetStructure: 5, Operation: "confirm", Schema: "full-range-v1", SourceHash: sourceHash[:], TargetHash: targetHash[:]}, SourceDocument: 1, TargetDocument: 1, Chapter: 6, TargetChapter: 6, SourceRetired: 5, TargetCreated: 5, TargetKind: "SECTION", SourceKind: "MAIN_CAPTURE", VersionKind: "STRUCTURE"}
	row.Mapping, _ = json.Marshal(map[string]any{"schema_version": "full-range-v1", "type": "SPLIT_LEVEL2", "chapter_id": "6", "operation_id": "confirm", "source_unit_id": "2", "target_unit_id": "4", "source_version_id": "7", "target_version_id": "8", "source_structure_id": "3", "target_structure_id": "5", "source_body_hash": sourceHash[:], "target_body_hash": targetHash[:], "source_scope": "WHOLE_UNIT", "start_boundary": "a", "end_boundary": nil, "main_block_ids": segment.MainBlocks, "draft_block_ids": segment.DraftBlocks, "boundary": candidate.Boundary, "target_kind": "SECTION", "target_key": candidate.Key, "preview_cut_ref": "preview", "preview_digest": make([]byte, 32)})
	return row, source, target, witness
}

func TestHistoricalMappingOnlyProvesActualFixedOriginalRange(t *testing.T) {
	row, source, target, witness := historicalMappingFixture(t)
	if edge, ok := proveHistoricalMapping(row, source, target, witness); !ok || edge.Source != 2 || edge.Target != 4 || len(edge.Heads) != 1 {
		t.Fatal("actual full-source edge rejected")
	}
	for _, item := range []struct {
		name string
		edit func(*repository.HistoricalUnitMapping, *pb.FixedContent, *historicalStructureWitness)
	}{
		{"foreign document", func(r *repository.HistoricalUnitMapping, _ *pb.FixedContent, _ *historicalStructureWitness) {
			r.TargetDocument = 99
		}},
		{"wrong retirement", func(r *repository.HistoricalUnitMapping, _ *pb.FixedContent, _ *historicalStructureWitness) {
			r.SourceRetired = 9
		}},
		{"wrong source version", func(r *repository.HistoricalUnitMapping, _ *pb.FixedContent, _ *historicalStructureWitness) {
			r.SourceVersion++
		}},
		{"wrong operation", func(r *repository.HistoricalUnitMapping, _ *pb.FixedContent, _ *historicalStructureWitness) {
			r.Operation = "other"
		}},
		{"wrong actual body", func(_ *repository.HistoricalUnitMapping, s *pb.FixedContent, _ *historicalStructureWitness) {
			s.BodyJsonUtf8 = []byte(`{"type":"doc"}`)
		}},
		{"wrong preview digest", func(_ *repository.HistoricalUnitMapping, _ *pb.FixedContent, w *historicalStructureWitness) {
			w.Original.StructurePreview.Digest[0] = 1
		}},
		{"missing installed target", func(_ *repository.HistoricalUnitMapping, _ *pb.FixedContent, w *historicalStructureWitness) {
			w.Install.Units = nil
		}},
		{"guessed boundary", func(_ *repository.HistoricalUnitMapping, _ *pb.FixedContent, w *historicalStructureWitness) {
			other := "new"
			w.Migration.Changes[0].Targets[0].Ranges[0].Start = &other
		}},
	} {
		t.Run(item.name, func(t *testing.T) {
			r, s, target, w := historicalMappingFixture(t)
			item.edit(&r, s, &w)
			if _, ok := proveHistoricalMapping(r, s, target, w); ok {
				t.Fatal("unproven history authorized")
			}
		})
	}
}
