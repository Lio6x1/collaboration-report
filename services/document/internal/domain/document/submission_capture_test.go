package document

import (
	codec "collaboration/contracts/content"
	pb "collaboration/contracts/internalv1"
	"crypto/sha256"
	"testing"

	"google.golang.org/protobuf/proto"
)

func captureFixture(t *testing.T) (*codec.Validator, *pb.FixedContent, CaptureExpectation) {
	t.Helper()
	validator, err := codec.NewValidator()
	if err != nil {
		t.Fatal(err)
	}
	canonical := func(raw string) []byte {
		value, err := codec.Decode([]byte(raw))
		if err != nil {
			t.Fatal(err)
		}
		bytes, err := codec.Canonical(value)
		if err != nil {
			t.Fatal(err)
		}
		return bytes
	}
	body := canonical(`{"type":"doc","content":[{"type":"paragraph","attrs":{"id":"p"},"content":[{"type":"text","text":"中😀正文"}]}]}`)
	provenance := canonical(`{"schema_version":"provenance-v1","lineage_id":"11","base_manifest_id":"13","base_content_version_id":"17","source_operation_id":"submission-operation-0001","accepted_captures":[],"block_sources":[{"block_id":"p","origin_capture_ref":"submission-operation-0001","origin_block_id":"p","relation":"INSERTED","source_range":{"start":0,"end":0}}]}`)
	hash := sha256.Sum256(body)
	content := &pb.FixedContent{DocumentId: 3, UnitId: 5, BranchId: 7, StructureId: 9, CaptureId: "submission-operation-0001", OperationId: "submission-operation-0001", WorkspaceEpoch: 2,
		CutJsonUtf8: canonical(`{"room_id":"19","workspace_epoch":"2","durable_seq":"1"}`), LineageRef: "11", BodyJsonUtf8: body,
		SchemaVersion: "content-v1", CanonicalizationVersion: "canonical-v1", HashAlgorithm: "SHA-256", BodyHash: hash[:], ByteSize: uint64(len(body)),
		ProvenanceJsonUtf8: provenance, ProvenanceSchema: "provenance-v1"}
	expectation := CaptureExpectation{Request: &pb.CaptureSubmissionRequest{Context: &pb.OperationContext{OperationId: content.OperationId,
		Target: &pb.Target{DocumentId: 3, UnitId: 5, BranchId: 7, StructureId: 9, WorkspaceEpoch: 2, Purpose: "SUBMISSION"}}},
		Workspace: EditWorkspace{Workspace: 7, Epoch: 2, Lineage: 11, BaseManifest: 13, BaseVersion: 17}, Room: 19,
		LineageProvenance: provenance}
	return validator, content, expectation
}

func TestSubmissionCaptureRejectsSchemaValidBranchViolations(t *testing.T) {
	for _, name := range []string{"duplicate block identity", "branch font override"} {
		t.Run(name, func(t *testing.T) {
			validator, fixed, expectation := captureFixture(t)
			value, err := codec.Decode(fixed.BodyJsonUtf8)
			if err != nil {
				t.Fatal(err)
			}
			body := value.(map[string]any)
			blocks := body["content"].([]any)
			paragraph := blocks[0].(map[string]any)
			if name == "duplicate block identity" {
				body["content"] = append(blocks, paragraph)
			} else {
				text := paragraph["content"].([]any)[0].(map[string]any)
				text["marks"] = []any{map[string]any{"type": "font", "attrs": map[string]any{"family": "SimSun"}}}
			}
			// 全局正文 schema 允许这些结构；支线约束必须在领域校验中明确拒绝。
			if err := validator.Body(value); err != nil {
				t.Fatal(err)
			}
			fixed.BodyJsonUtf8, err = codec.Canonical(value)
			if err != nil {
				t.Fatal(err)
			}
			hash := sha256.Sum256(fixed.BodyJsonUtf8)
			fixed.BodyHash, fixed.ByteSize = hash[:], uint64(len(fixed.BodyJsonUtf8))
			if _, err := ValidateSubmissionCapture(validator, fixed, expectation); err == nil {
				t.Fatal("schema-valid branch violation accepted")
			}
		})
	}
}

func TestSubmissionCaptureRequiresExactFixedBytesAndBinding(t *testing.T) {
	validator, original, expectation := captureFixture(t)
	if _, err := ValidateSubmissionCapture(validator, original, expectation); err != nil {
		t.Fatal(err)
	}
	for _, change := range []struct {
		name string
		edit func(*pb.FixedContent)
	}{
		{"unit target", func(p *pb.FixedContent) { p.UnitId++ }},
		{"base lineage", func(p *pb.FixedContent) { p.LineageRef = "12" }},
		{"hash", func(p *pb.FixedContent) { p.BodyHash[0]++ }},
		{"size", func(p *pb.FixedContent) { p.ByteSize++ }},
		{"schema", func(p *pb.FixedContent) { p.SchemaVersion = "content-v2" }},
		{"canonical bytes", func(p *pb.FixedContent) { p.BodyJsonUtf8 = append(p.BodyJsonUtf8, ' ') }},
		{"cut", func(p *pb.FixedContent) {
			value, err := codec.Decode(p.CutJsonUtf8)
			if err != nil {
				t.Fatal(err)
			}
			value.(map[string]any)["room_id"] = "20"
			p.CutJsonUtf8, err = codec.Canonical(value)
			if err != nil {
				t.Fatal(err)
			}
		}},
		{"resources", func(p *pb.FixedContent) { p.ResourceIds = []uint64{23} }},
		{"provenance", func(p *pb.FixedContent) { p.ProvenanceJsonUtf8 = []byte(`{}`) }},
	} {
		t.Run(change.name, func(t *testing.T) {
			value := proto.Clone(original).(*pb.FixedContent)
			change.edit(value)
			if _, err := ValidateSubmissionCapture(validator, value, expectation); err == nil {
				t.Fatal("invalid fixed capture accepted")
			}
		})
	}
}
