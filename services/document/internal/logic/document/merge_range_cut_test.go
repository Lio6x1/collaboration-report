package document

import (
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	"encoding/json"
	"testing"
)

func TestRangeProjectionKeepsTheExactOriginalSubmissionCut(t *testing.T) {
	unit := "2"
	source := dto.StructureRevision{Id: "5", DocumentId: "4", Entries: []dto.StructureEntry{{NodeId: "3", ChapterId: "3", Kind: "CHAPTER", UnitId: &unit}}}
	target := source
	target.Id = "8"
	original := []byte(`{"room_id":"7","workspace_epoch":"1","durable_seq":"2"}`)
	attempt := mergeAttemptContext{Submitted: &pb.FixedContent{DocumentId: 4, UnitId: 2, StructureId: 5, CutJsonUtf8: original}, Main: &pb.CaptureMainRequest{ExpectedStructureId: 8}}
	value := map[string]any{"room_id": "7", "workspace_epoch": "1", "durable_seq": "2", "range_projection": map[string]any{"source_structure": source, "target_structure": target, "unit_id": unit}}
	projected, _ := json.Marshal(value)
	if !validProjectedSubmissionCut(projected, attempt) {
		t.Fatal("valid range projection rejected")
	}
	value["durable_seq"] = "3"
	projected, _ = json.Marshal(value)
	if validProjectedSubmissionCut(projected, attempt) {
		t.Fatal("later draft cut substituted for fixed submission")
	}
}
