package document

import (
	"bytes"
	pb "collaboration/contracts/internalv1"
	"testing"
)

func TestStructurePreparationCutKeepsAllMainAndAffectedRealBranch(t *testing.T) {
	original := &pb.CutVector{DocumentId: 1, StructureId: 2, StyleId: 3, ControlRevision: 4, Rooms: []*pb.RoomCut{
		{RoomId: 10, WorkspaceEpoch: 4, DurableSeq: 1}, {RoomId: 20, WorkspaceEpoch: 4, DurableSeq: 2},
		{RoomId: 30, WorkspaceEpoch: 1, DurableSeq: 3}, {RoomId: 40, WorkspaceEpoch: 1, DurableSeq: 4},
	}}
	cut, err := StructurePreparationCut(original, []uint64{30, 40}, []uint64{30})
	if err != nil || len(cut.Rooms) != 3 || cut.Rooms[2].RoomId != 30 || len(cut.DependencyDigest) != 32 || len(original.Rooms) != 4 {
		t.Fatalf("wrong fixed scope: %+v, %v", cut, err)
	}
	absent, err := StructurePreparationCut(original, []uint64{30, 40, 50}, []uint64{50})
	if err != nil || len(absent.Rooms) != 2 || bytes.Equal(absent.DependencyDigest, cut.DependencyDigest) {
		t.Fatalf("virtual branch scope: %+v, %v", absent, err)
	}
	for _, rooms := range [][]uint64{{30, 30}, {99}} {
		if _, err := StructurePreparationCut(original, []uint64{30, 40}, rooms); err == nil {
			t.Fatal("unproven branch accepted")
		}
	}
}
