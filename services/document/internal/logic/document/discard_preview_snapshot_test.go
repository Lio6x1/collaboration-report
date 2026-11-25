package document

import (
	codec "collaboration/contracts/content"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"testing"
	"time"
)

func discardSnapshotFixture(t *testing.T) (*codec.Validator, discardPreviewContext, *pb.DraftPreviewReply, map[string]any) {
	t.Helper()
	validator, err := codec.NewValidator()
	if err != nil {
		t.Fatal(err)
	}
	fixed, _ := pendingDiscardPreviewFixture(t)
	canonical := func(raw string) []byte {
		value, err := codec.Decode([]byte(raw))
		if err != nil {
			t.Fatal(err)
		}
		result, err := codec.Canonical(value)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	body := canonical(`{"type":"doc","content":[{"type":"paragraph","attrs":{"id":"p"},"content":[{"type":"text","text":"可靠正文"}]}]}`)
	provenance := canonical(`{"schema_version":"provenance-v1","lineage_id":"13","base_manifest_id":"10","base_content_version_id":"14","source_operation_id":"discard-preview-original","accepted_captures":[],"block_sources":[{"block_id":"p","origin_capture_ref":"discard-preview-original","origin_block_id":"p","relation":"INSERTED","source_range":{"start":0,"end":0}}]}`)
	bodyHash := sha256.Sum256(body)
	cut := &pb.RoomCut{RoomId: 12, WorkspaceEpoch: 1, DurableSeq: 2}
	cutBytes := canonical(`{"room_id":"12","workspace_epoch":"1","durable_seq":"2"}`)
	draft := &pb.FixedContent{DocumentId: 3, UnitId: 4, BranchId: 6, StructureId: 5, WorkspaceEpoch: 1, CaptureId: fixed.Intent.Operation, OperationId: fixed.Intent.Operation,
		LineageRef: "13", CutJsonUtf8: cutBytes, BodyJsonUtf8: body, BodyHash: bodyHash[:], ByteSize: uint64(len(body)), SchemaVersion: "content-v1", CanonicalizationVersion: "canonical-v1", HashAlgorithm: "SHA-256", ProvenanceJsonUtf8: provenance, ProvenanceSchema: "provenance-v1"}
	main := proto.Clone(draft).(*pb.FixedContent)
	main.BranchId, main.WorkspaceEpoch, main.CaptureId = 0, 2, fixed.Intent.Operation+":main"
	main.CutJsonUtf8 = canonical(`{"room_id":"0","workspace_epoch":"2","durable_seq":"0"}`)
	candidate := proto.Clone(draft).(*pb.FixedContent)
	candidate.CaptureId += ":candidate"
	wire := func(value proto.Message) map[string]any {
		raw, err := (protojson.MarshalOptions{UseProtoNames: true, EmitUnpopulated: true}).Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		var result map[string]any
		if err = json.Unmarshal(raw, &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	observed := &pb.CutVector{DocumentId: 3, StructureId: 5, StyleId: 11, ControlRevision: 2, Rooms: []*pb.RoomCut{cut}}
	rooms, _ := codec.Canonical(domain.CutBinding(observed)["rooms"])
	digest := sha256.Sum256(rooms)
	observed.DependencyDigest = digest[:]
	expires := time.Now().Add(time.Minute).UnixMilli()
	lineage, _ := codec.Decode(provenance)
	value := map[string]any{"binding": wire(discardPreviewRequest(fixed)), "room_id": "12", "draft_cut": wire(cut), "main_cut": []any{}, "source": map[string]any{"purpose": "DISCARD_PREVIEW", "room_id": "12", "main_manifest_id": "10", "main_version_id": "14", "structure_id": "5", "style_id": "11", "control_revision": "2", "lineage": map[string]any{"lineage_id": "13", "base_manifest_id": "10", "base_version_id": "14", "provenance": lineage}},
		"draft": wire(draft), "main": wire(main), "candidate": wire(candidate), "conflicts": []any{}, "preview_cut_ref": fixed.Intent.Operation, "expires_unix_ms": domain.TextID(uint64(expires))}
	reply := &pb.DraftPreviewReply{State: pb.CaptureReply_FIXED, PreviewCutRef: fixed.Intent.Operation, Draft: draft, MainManifestId: 10, ObservedCut: observed, ExpiresUnixMs: expires}
	return validator, fixed, reply, value
}

func signedDiscardFixture(t *testing.T, value map[string]any, reply *pb.DraftPreviewReply) []byte {
	t.Helper()
	delete(value, "input_digest")
	raw, err := codec.Canonical(value)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(raw)
	reply.InputDigest = digest[:]
	value["input_digest"] = base64.StdEncoding.EncodeToString(digest[:])
	raw, _ = json.Marshal(value)
	return raw
}

func TestDiscardPreviewSnapshotBindsActualCutAndMainCandidate(t *testing.T) {
	validator, fixed, reply, value := discardSnapshotFixture(t)
	if _, err := validateDiscardPreviewSnapshot(validator, signedDiscardFixture(t, value, reply), fixed, reply); err != nil {
		t.Fatal(err)
	}
	// 即使错误生产者重算了整包摘要，也不能把另一支线、切点或任意候选当成原丢弃预览。
	for _, edit := range []func(map[string]any){
		func(v map[string]any) { v["candidate"].(map[string]any)["branch_id"] = "99" },
		func(v map[string]any) { v["source"].(map[string]any)["main_manifest_id"] = "99" },
		func(v map[string]any) { v["draft_cut"].(map[string]any)["durable_seq"] = "99" },
		func(v map[string]any) { v["main"].(map[string]any)["workspace_epoch"] = "99" },
		func(v map[string]any) {
			v["candidate"].(map[string]any)["body_json_utf8"] = base64.StdEncoding.EncodeToString([]byte(`{"type":"doc","content":[]}`))
		},
		func(v map[string]any) { v["conflicts"] = []any{map[string]any{"conflict_id": "unexpected"}} },
	} {
		validator, fixed, reply, value := discardSnapshotFixture(t)
		edit(value)
		if _, err := validateDiscardPreviewSnapshot(validator, signedDiscardFixture(t, value, reply), fixed, reply); err == nil {
			t.Fatal("changed reliable discard snapshot accepted")
		}
	}
}

// 内容仍绑定原管理修订，物理cut却必须绑定真正保存的MAIN代次，且重签摘要不能隐藏篡改。
func TestDiscardPreviewSnapshotSeparatesMetadataAndPhysicalMainEpoch(t *testing.T) {
	validator, fixed, reply, value := discardSnapshotFixture(t)
	fixed.Scope.MainEpoch = 1
	value["source"].(map[string]any)["workspace_epoch"] = "1"
	main := value["main"].(map[string]any)
	main["cut_json_utf8"] = base64.StdEncoding.EncodeToString([]byte(`{"durable_seq":"4","room_id":"19","workspace_epoch":"1"}`))
	value["main_cut"] = []any{map[string]any{"room_id": "19", "workspace_epoch": "1", "durable_seq": "4"}}
	reply.ObservedCut.Rooms = append(reply.ObservedCut.Rooms, &pb.RoomCut{RoomId: 19, WorkspaceEpoch: 1, DurableSeq: 4})
	rooms, _ := codec.Canonical(domain.CutBinding(reply.ObservedCut)["rooms"])
	digest := sha256.Sum256(rooms)
	reply.ObservedCut.DependencyDigest = digest[:]
	if _, err := validateDiscardPreviewSnapshot(validator, signedDiscardFixture(t, value, reply), fixed, reply); err != nil {
		t.Fatal(err)
	}
	value["source"].(map[string]any)["workspace_epoch"] = "2"
	if _, err := validateDiscardPreviewSnapshot(validator, signedDiscardFixture(t, value, reply), fixed, reply); err == nil {
		t.Fatal("re-signed metadata epoch cannot replace original physical MAIN epoch")
	}
}
