package document

import (
	"bytes"
	codec "collaboration/contracts/content"
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"crypto/sha256"
	"encoding/json"
	"google.golang.org/protobuf/proto"
	"testing"
	"time"
)

func mainReadBindingFixture(t *testing.T) (mainReadContext, repository.OriginalAdmissionBinding) {
	t.Helper()
	expires := time.Now().Add(30 * time.Second).UnixMilli()
	proof := &pb.IdentityAdmission{
		AdmissionId: "original-read-identity", OperationId: "original-main-reading",
		ActorId: 7, Purpose: "READ_MAIN", Target: &pb.Target{DocumentId: 3, Purpose: "READ_MAIN"},
		SessionRef: bytes.Repeat([]byte{1}, 32), ExpiresUnixMs: expires,
	}
	sources := []*pb.SourceEpoch{{ScopeKey: "doc-acl:3", Epoch: 1}}
	target := &pb.Target{DocumentId: 3, StructureId: 5, WorkspaceEpoch: 2, Purpose: "READ_MAIN"}
	fingerprint, err := domain.OperationFingerprint("/v1/documents/3/main", map[string]any{"manifest_id": "10", "structure_id": "5", "style_id": "11", "control_revision": "2"})
	if err != nil {
		t.Fatal(err)
	}
	fixed := mainReadContext{
		Actor: 7, SessionExpires: expires + 3600000, ExpiresUnixMs: expires,
		Manifest: 10, Structure: 5, Style: 11, Control: 2,
		Units: []uint64{4}, Versions: map[uint64]uint64{4: 21},
		Access: DocumentAccess{
			Document: dto.Document{
				Id: "3", Lifecycle: "ACTIVE", CurrentManifestId: "10",
				CurrentStructureId: "5", CurrentStyleId: "11", ControlRevision: "2",
			},
			Permissions: domain.Permissions{Read: true},
			Sources:     sources, Proofs: []*pb.IdentityAdmission{proof},
		},
		Context: &pb.OperationContext{
			OperationId: proof.OperationId, ActorId: 7,
			IdentityAdmissionRef: proof.AdmissionId, DocumentAdmissionRef: "original-document-reading",
			Target: target, Sources: sources, RequestFingerprint: fingerprint[:],
		},
	}
	raw, _ := json.Marshal(target)
	actual := repository.OriginalAdmissionBinding{
		Actor: 7, Document: 3, Identity: proof.AdmissionId, Operation: proof.OperationId,
		Purpose: "READ_MAIN", Target: raw, Session: proof.SessionRef, SessionExpires: expires,
	}
	return fixed, actual
}

func mainReadCaptureFixture(t *testing.T) (*MainReadViews, mainReadContext, *pb.MainCaptureReply) {
	t.Helper()
	fixed, _ := mainReadBindingFixture(t)
	validator, err := codec.NewValidator()
	if err != nil {
		t.Fatal(err)
	}
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
	body := canonical(`{"type":"doc","content":[{"type":"paragraph","attrs":{"id":"p"},"content":[{"type":"text","text":"可靠读取"}]}]}`)
	provenance := canonical(`{"schema_version":"provenance-v1","lineage_id":"8","base_manifest_id":"10","base_content_version_id":"21","source_operation_id":"original-main-reading","accepted_captures":[],"block_sources":[{"block_id":"p","origin_capture_ref":"original-main-reading","origin_block_id":"p","relation":"INSERTED","source_range":{"start":0,"end":0}}]}`)
	hash := sha256.Sum256(body)
	content := &pb.FixedContent{
		DocumentId: 3, UnitId: 4, StructureId: 5, WorkspaceEpoch: 2,
		OperationId: fixed.Context.OperationId, CaptureId: fixed.Context.OperationId + ":MAIN_READ_VIEW:1:4",
		LineageRef: "8", BodyJsonUtf8: body, BodyHash: hash[:], ByteSize: uint64(len(body)),
		SchemaVersion: "content-v1", CanonicalizationVersion: "canonical-v1", HashAlgorithm: "SHA-256",
		ProvenanceSchema: "provenance-v1", ProvenanceJsonUtf8: provenance,
		CutJsonUtf8: canonical(`{"room_id":"19","workspace_epoch":"2","durable_seq":"1"}`),
	}
	main := &pb.MainCaptureReply{
		State: pb.CaptureReply_FIXED, Units: []*pb.FixedContent{content},
		Cut: &pb.CutVector{
			DocumentId: 3, StructureId: 5, StyleId: 11, ControlRevision: 2,
			Rooms: []*pb.RoomCut{{RoomId: 19, WorkspaceEpoch: 2, DurableSeq: 1}},
		},
	}
	sealMainReadCapture(t, main)
	return &MainReadViews{validator: validator}, fixed, main
}

func sealMainReadCapture(t *testing.T, main *pb.MainCaptureReply) {
	t.Helper()
	cut := domain.CutBinding(main.Cut)
	rooms, _ := codec.Canonical(cut["rooms"])
	dependencies := sha256.Sum256(rooms)
	main.Cut.DependencyDigest = dependencies[:]
	units := make([]any, 0, len(main.Units))
	for _, unit := range main.Units {
		wire, err := fixedWire(unit)
		if err != nil {
			t.Fatal(err)
		}
		units = append(units, wire)
	}
	raw, _ := codec.Canonical(map[string]any{"cut": domain.CutBinding(main.Cut), "units": units})
	digest := sha256.Sum256(raw)
	main.ResultDigest = digest[:]
}

// 即使整个RPC摘要重新自洽，单元正文也不能借另一个房间切点或清单来源。
func TestMainReadCaptureRequiresActualPerUnitCutAndProvenance(t *testing.T) {
	s, fixed, valid := mainReadCaptureFixture(t)
	if err := s.validateCapture(fixed, valid); err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(*pb.MainCaptureReply){
		"different durable cut": func(m *pb.MainCaptureReply) {
			m.Units[0].CutJsonUtf8 = []byte(`{"durable_seq":"2","room_id":"19","workspace_epoch":"2"}`)
		},
		"different room": func(m *pb.MainCaptureReply) {
			m.Units[0].CutJsonUtf8 = []byte(`{"durable_seq":"1","room_id":"20","workspace_epoch":"2"}`)
		},
		"unclaimed room": func(m *pb.MainCaptureReply) {
			m.Cut.Rooms = append(m.Cut.Rooms, &pb.RoomCut{RoomId: 20, WorkspaceEpoch: 2, DurableSeq: 1})
		},
		"duplicate room": func(m *pb.MainCaptureReply) {
			m.Cut.Rooms = append(m.Cut.Rooms, proto.Clone(m.Cut.Rooms[0]).(*pb.RoomCut))
		},
		"different provenance base": func(m *pb.MainCaptureReply) {
			m.Units[0].ProvenanceJsonUtf8 = bytes.ReplaceAll(m.Units[0].ProvenanceJsonUtf8, []byte(`"base_manifest_id":"10"`), []byte(`"base_manifest_id":"12"`))
		},
		"different provenance operation": func(m *pb.MainCaptureReply) {
			m.Units[0].ProvenanceJsonUtf8 = bytes.ReplaceAll(m.Units[0].ProvenanceJsonUtf8, []byte(`"source_operation_id":"original-main-reading"`), []byte(`"source_operation_id":"other-main-reading"`))
		},
		"invalid resource set": func(m *pb.MainCaptureReply) { m.Units[0].ResourceIds = []uint64{999} },
	}
	for name, edit := range cases {
		t.Run(name, func(t *testing.T) {
			s, fixed, main := mainReadCaptureFixture(t)
			edit(main)
			sealMainReadCapture(t, main)
			if s.validateCapture(fixed, main) == nil {
				t.Fatal("self-consistent replacement accepted")
			}
		})
	}
}

// 改写一份任务元数据不能把原READ准入变成另一个actor/清单/管理用途。
// 反例同时覆盖真实原准入和内部固定字段，不能仅比较两份来自同一payload的对象。
func TestMainReadAdmissionRequiresOriginalProofAndWholeBinding(t *testing.T) {
	fixed, actual := mainReadBindingFixture(t)
	if err := validateMainReadAdmission(fixed, actual); err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(*mainReadContext, *repository.OriginalAdmissionBinding){
		"context actor": func(f *mainReadContext, _ *repository.OriginalAdmissionBinding) { f.Context.ActorId++ },
		"proof actor":   func(f *mainReadContext, _ *repository.OriginalAdmissionBinding) { f.Access.Proofs[0].ActorId++ },
		"proof operation": func(f *mainReadContext, _ *repository.OriginalAdmissionBinding) {
			f.Access.Proofs[0].OperationId = "different-original"
		},
		"proof purpose": func(f *mainReadContext, _ *repository.OriginalAdmissionBinding) {
			f.Access.Proofs[0].Purpose = "MAIN_CONTROL"
		},
		"manifest":    func(f *mainReadContext, _ *repository.OriginalAdmissionBinding) { f.Manifest++ },
		"control":     func(f *mainReadContext, _ *repository.OriginalAdmissionBinding) { f.Control++ },
		"fingerprint": func(f *mainReadContext, _ *repository.OriginalAdmissionBinding) { f.Context.RequestFingerprint[0] ^= 1 },
		"expired scope": func(f *mainReadContext, _ *repository.OriginalAdmissionBinding) {
			f.ExpiresUnixMs = f.SessionExpires + 1
		},
		"read permission": func(f *mainReadContext, _ *repository.OriginalAdmissionBinding) { f.Access.Permissions.Read = false },
		"session": func(_ *mainReadContext, a *repository.OriginalAdmissionBinding) {
			a.Session = bytes.Repeat([]byte{2}, 32)
		},
		"unit":          func(_ *mainReadContext, a *repository.OriginalAdmissionBinding) { a.Unit = 4 },
		"missing proof": func(f *mainReadContext, _ *repository.OriginalAdmissionBinding) { f.Access.Proofs[0] = nil },
	}
	for name, edit := range cases {
		t.Run(name, func(t *testing.T) {
			fixed, actual := mainReadBindingFixture(t)
			edit(&fixed, &actual)
			if validateMainReadAdmission(fixed, actual) == nil {
				t.Fatal("changed original reading binding accepted")
			}
		})
	}
}
