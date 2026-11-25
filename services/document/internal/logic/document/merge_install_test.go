package document

import (
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	"encoding/json"
	"google.golang.org/protobuf/proto"
	"testing"
)

// READY必须确认原清单、原房间及原代次，不能仅因下游返回一个成功枚举就释放Doc屏障。
func TestMainInstallReplyRequiresTheExactManifestAndEpochs(t *testing.T) {
	fixed := mainInstallContext{
		Context: &pb.OperationContext{OperationId: "exact-install-operation"}, Generation: 3, Manifest: 5,
		Digest: make([]byte, 32), Units: []*pb.ManifestUnit{{Content: &pb.FixedContent{CutJsonUtf8: []byte(`{"room_id":"7","workspace_epoch":"11","durable_seq":"0"}`)}}},
	}
	reply := &pb.InstallReply{State: pb.InstallReply_READY, OperationId: fixed.Context.OperationId, BarrierGeneration: 3,
		ManifestId: 5, ManifestDigest: make([]byte, 32), InstalledEpochs: []*pb.RoomCut{{RoomId: 7, WorkspaceEpoch: 11, DurableSeq: 0}}}
	if !validMainInstallReply(fixed, true, reply) {
		t.Fatal("exact installation rejected")
	}
	for _, change := range []struct {
		name string
		edit func(*pb.InstallReply)
	}{
		{"manifest", func(r *pb.InstallReply) { r.ManifestId++ }},
		{"digest", func(r *pb.InstallReply) { r.ManifestDigest[0]++ }},
		{"epoch", func(r *pb.InstallReply) { r.InstalledEpochs[0].WorkspaceEpoch++ }},
		{"room", func(r *pb.InstallReply) { r.InstalledEpochs[0].RoomId++ }},
		{"cut", func(r *pb.InstallReply) { r.InstalledEpochs[0].DurableSeq++ }},
		{"missing rooms", func(r *pb.InstallReply) { r.InstalledEpochs = nil }},
		{"duplicate rooms", func(r *pb.InstallReply) { r.InstalledEpochs = append(r.InstalledEpochs, r.InstalledEpochs[0]) }},
	} {
		t.Run(change.name, func(t *testing.T) {
			changed := proto.Clone(reply).(*pb.InstallReply)
			change.edit(changed)
			if validMainInstallReply(fixed, true, changed) {
				t.Fatal("different installation released the barrier")
			}
		})
	}
}

func TestEmptyMainControlAckRequiresActualEmptyDigestAndControlScope(t *testing.T) {
	digest, err := domain.MainManifestDigest(2, []*pb.ManifestUnit{})
	if err != nil {
		t.Fatal(err)
	}
	fixed := mainInstallContext{Context: &pb.OperationContext{OperationId: "empty-controlled-install", Target: &pb.Target{DocumentId: 1, Purpose: "MAIN_CONTROL"}}, Generation: 1, Manifest: 2, Structure: 3, Style: 4, Control: 5, Digest: digest[:]}
	reply := &pb.InstallReply{State: pb.InstallReply_READY, OperationId: fixed.Context.OperationId, BarrierGeneration: 1, ManifestId: 2, ManifestDigest: digest[:]}
	if !validMainInstallReply(fixed, true, reply) {
		t.Fatal("exact empty control installation rejected")
	}
	fixed.Context.Target.Purpose = "MERGE"
	if validMainInstallReply(fixed, true, reply) {
		t.Fatal("empty merge installation released main")
	}
	fixed.Context.Target.Purpose = "MAIN_CONTROL"
	fixed.Digest = make([]byte, 32)
	reply.ManifestDigest = make([]byte, 32)
	if validMainInstallReply(fixed, true, reply) {
		t.Fatal("equal fake digests proved an empty manifest")
	}
}

func TestControlManifestMembershipRejectsMissingAndWrongFixedUnits(t *testing.T) {
	var structure dto.StructureRevision
	if json.Unmarshal([]byte(`{"id":"2","document_id":"1","entries":[{"node_id":"3","chapter_id":"3","unit_id":"4","kind":"CHAPTER"}]}`), &structure) != nil {
		t.Fatal("fixture")
	}
	rows := []map[string]string{{"unit_id": "4", "content_version_id": "5"}}
	units := []*pb.ManifestUnit{{UnitId: 4, ContentVersionId: 5}}
	if !controlManifestUnitSetMatches(structure, rows, units) {
		t.Fatal("actual unit set rejected")
	}
	if controlManifestUnitSetMatches(structure, nil, nil) {
		t.Fatal("empty DTO hid an actual chapter")
	}
	if controlManifestUnitSetMatches(structure, rows, nil) {
		t.Fatal("missing installed unit accepted")
	}
	units[0].ContentVersionId = 6
	if controlManifestUnitSetMatches(structure, rows, units) {
		t.Fatal("wrong fixed version accepted")
	}
	structure.Entries = nil
	if !controlManifestUnitSetMatches(structure, nil, nil) {
		t.Fatal("actual empty structure rejected")
	}
}

func TestStructureInstallAckIncludesNewDraftRooms(t *testing.T) {
	fixed := mainInstallContext{Context: &pb.OperationContext{OperationId: "structure-install-original"}, Generation: 1, Manifest: 2, Digest: make([]byte, 32), Units: []*pb.ManifestUnit{{Content: &pb.FixedContent{CutJsonUtf8: []byte(`{"room_id":"3","workspace_epoch":"2","durable_seq":"0"}`)}}}, Transition: &structureInstallation{Branches: []structureInstalledBranch{{Room: 4, Epoch: 1, Content: &pb.FixedContent{CutJsonUtf8: []byte(`{"room_id":"4","workspace_epoch":"1","durable_seq":"0"}`)}}}}}
	reply := &pb.InstallReply{State: pb.InstallReply_READY, OperationId: fixed.Context.OperationId, BarrierGeneration: 1, ManifestId: 2, ManifestDigest: make([]byte, 32), InstalledEpochs: []*pb.RoomCut{{RoomId: 3, WorkspaceEpoch: 2}}}
	if validMainInstallReply(fixed, true, reply) {
		t.Fatal("MAIN-only ACK opened a new draft")
	}
	reply.InstalledEpochs = append(reply.InstalledEpochs, &pb.RoomCut{RoomId: 4, WorkspaceEpoch: 1})
	if !validMainInstallReply(fixed, true, reply) {
		t.Fatal("full MAIN+draft ACK rejected")
	}
	reply.InstalledEpochs[1].DurableSeq = 1
	if validMainInstallReply(fixed, true, reply) {
		t.Fatal("nonzero initial draft cut accepted")
	}
}
