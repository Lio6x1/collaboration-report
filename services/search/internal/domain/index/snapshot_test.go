package index

import (
	codec "collaboration/contracts/content"
	pb "collaboration/contracts/internalv1"
	"crypto/sha256"
	"strings"
	"testing"
)

func snapshotFixture(t *testing.T) (*codec.Validator, *pb.PublicSnapshot) {
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
		result, err := codec.Canonical(value)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	body := canonical(`{"type":"doc","content":[{"type":"heading","attrs":{"id":"h1","level":2},"content":[{"type":"text","text":"实验章节"}]},{"type":"paragraph","attrs":{"id":"p1"},"content":[{"type":"text","text":"中文 code 表格"},{"type":"formula","attrs":{"latex":"x+y","atomStrong":false,"atomEm":false,"atomCode":false,"atomStrike":false,"atomLink":null}}]}]}`)
	provenance := canonical(`{"schema_version":"provenance-v1","lineage_id":"1","base_manifest_id":"2","base_content_version_id":"3","source_operation_id":"fixture-main","accepted_captures":[],"block_sources":[]}`)
	digest := sha256.Sum256(body)
	fixed := &pb.FixedContent{DocumentId: 17, UnitId: 11, StructureId: 7, SchemaVersion: "content-v1", CanonicalizationVersion: "canonical-v1",
		HashAlgorithm: "SHA-256", BodyJsonUtf8: body, BodyHash: digest[:], ByteSize: uint64(len(body)),
		ProvenanceSchema: "provenance-v1", ProvenanceJsonUtf8: provenance}
	return validator, &pb.PublicSnapshot{DocumentId: 17, AggregateSeq: 3, ReleaseId: 29, PublicationEpoch: 4, StructureId: 7,
		Visible: true, Title: "正式标题", VisibilityPolicy: "COLLABORATORS", VisibilityRevision: 1,
		Units: []*pb.ManifestUnit{{UnitId: 11, ContentVersionId: 31, Content: fixed}}}
}

func TestSnapshotExtractsOnlyValidatedFixedBodyAndStableAuthority(t *testing.T) {
	validator, snapshot := snapshotFixture(t)
	value, err := FromSnapshot(validator, snapshot, 17, 2)
	if err != nil {
		t.Fatal(err)
	}
	if value.Document != 17 || value.Sequence != 3 || value.Release != 29 || !value.Visible || value.Policy != "COLLABORATORS" ||
		len(value.Chapters) != 1 || value.Chapters[0].Unit != 11 || value.Chapters[0].Title != "实验章节" ||
		!strings.Contains(value.Text, "中文 code 表格") || !strings.Contains(value.Text, "x+y") {
		t.Fatal("FIXED_TEXT_PROJECTION_CHANGED")
	}
	again, err := FromSnapshot(validator, snapshot, 17, 1)
	if err != nil || again.Digest != value.Digest {
		t.Fatal("EVENT_HINT_CHANGED_CURRENT_AUTHORITY_DIGEST")
	}
	snapshot.Units[0].Content.BodyHash = make([]byte, 32)
	if _, err = FromSnapshot(validator, snapshot, 17, 2); err == nil {
		t.Fatal("INVALID_FIXED_BODY_HASH_INDEXED")
	}
}

func TestSnapshotRejectsUnboundCurrentStateAndHiddenBodyFallback(t *testing.T) {
	for _, mutate := range []func(*pb.PublicSnapshot){
		func(value *pb.PublicSnapshot) { value.DocumentId = 18 },
		func(value *pb.PublicSnapshot) { value.AggregateSeq = 1 },
		func(value *pb.PublicSnapshot) { value.ReleaseId = 0 },
		func(value *pb.PublicSnapshot) { value.Units[0].Content.UnitId = 12 },
		func(value *pb.PublicSnapshot) { value.Units = append(value.Units, value.Units[0]) },
		func(value *pb.PublicSnapshot) { value.Visible = false },
	} {
		validator, snapshot := snapshotFixture(t)
		mutate(snapshot)
		if _, err := FromSnapshot(validator, snapshot, 17, 2); err == nil {
			t.Fatal("UNBOUND_SNAPSHOT_INDEXED")
		}
	}
	validator, _ := snapshotFixture(t)
	hidden := &pb.PublicSnapshot{DocumentId: 17, AggregateSeq: 4, PublicationEpoch: 5, VisibilityPolicy: "PUBLIC", VisibilityRevision: 1}
	value, err := FromSnapshot(validator, hidden, 17, 2)
	if err != nil || value.Visible || value.Release != 0 || value.Text != "" {
		t.Fatal("VALID_HIDDEN_AUTHORITY_REJECTED")
	}
}
