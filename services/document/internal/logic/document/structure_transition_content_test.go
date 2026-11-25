package document

import (
	"bytes"
	codec "collaboration/contracts/content"
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	"crypto/sha256"
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"
)

func TestStructureCandidateNewDraftBlockIsUnacceptedOriginalPreviewInput(t *testing.T) {
	validator, err := codec.NewValidator()
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := buildStructureCandidate(validator, []byte(`{"type":"doc","content":[{"type":"paragraph","attrs":{"id":"pending-S2"},"content":[{"type":"text","text":"尚未审核"}]}]}`), structureCandidateContext{Document: 1, Unit: 2, Structure: 3, Branch: 4, Epoch: 1, Lineage: 5, Manifest: 6, BaseVersion: 7, Operation: "confirm-original", Preview: "preview-original", Key: "chapter:SPLIT:0", Relation: "SPLIT"})
	if err != nil {
		t.Fatal(err)
	}
	var proof struct {
		Accepted []json.RawMessage `json:"accepted_captures"`
		Blocks   []struct {
			Capture  string `json:"origin_capture_ref"`
			Relation string `json:"relation"`
		} `json:"block_sources"`
	}
	if json.Unmarshal(candidate.ProvenanceJsonUtf8, &proof) != nil || len(proof.Accepted) != 0 || len(proof.Blocks) != 1 || proof.Blocks[0].Capture != "preview-original" || proof.Blocks[0].Relation != "INSERTED" {
		t.Fatal("new draft input falsely accepted")
	}
}

// 图片是无块ID的行内节点；块谱系遍历不能代替资源收集，也不能读取不存在的resource_id字段。
func TestStructureCandidateCollectsInlineImagesWithoutBlockIDs(t *testing.T) {
	validator, err := codec.NewValidator()
	if err != nil {
		t.Fatal(err)
	}
	image := func(id string) map[string]any {
		return map[string]any{"type": "image", "attrs": map[string]any{"resourceId": id, "alt": nil, "title": nil, "width": nil, "height": nil, "atomStrong": false, "atomEm": false, "atomCode": false, "atomStrike": false, "atomLink": nil}}
	}
	body := map[string]any{"type": "doc", "content": []any{
		map[string]any{"type": "paragraph", "attrs": map[string]any{"id": "inline-images"}, "content": []any{image("100"), image("9"), image("100")}},
	}}
	raw, err := codec.Canonical(body)
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := buildStructureCandidate(validator, raw, structureCandidateContext{Document: 1, Unit: 2, Structure: 3, Branch: 4, Epoch: 1, Lineage: 5, Manifest: 6, BaseVersion: 7, Operation: "image-confirm-original", Preview: "image-preview-original", Key: "chapter:SPLIT:0", Relation: "SPLIT"})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(candidate.ResourceIds, []uint64{9, 100}) {
		t.Fatal("inline image references missing or not canonical")
	}
}

// 直接消费Node生成且由其回归锁定的候选，覆盖跨服务确认、MAIN/草稿分别物化及新支线安装谱系。
func TestVirtuallyAlignedSplitCandidatesMaterializeWithoutAcceptingPendingDraft(t *testing.T) {
	raw, err := os.ReadFile("../../../../../tests/fixtures/structure-aligned-split.json")
	if err != nil {
		t.Fatal(err)
	}
	var vector struct {
		Input dto.StructurePreviewCreate `json:"input"`
		Fixed struct {
			Main []struct {
				Body []byte `json:"body_json_utf8"`
			} `json:"main_units"`
		} `json:"fixed"`
		Migration json.RawMessage `json:"migration"`
	}
	if err = json.Unmarshal(raw, &vector); err != nil {
		t.Fatal(err)
	}
	validator, err := codec.NewValidator()
	if err != nil {
		t.Fatal(err)
	}
	migration, err := domain.ParseStructureMigration(vector.Migration, vector.Input, []domain.StructureMigrationSource{{Unit: 4, Chapter: 3}}, validator)
	if err != nil {
		t.Fatal("Node对齐候选不能通过Go确认校验:", err)
	}
	source, err := buildStructureCandidate(validator, vector.Fixed.Main[0].Body, structureCandidateContext{Document: 1, Unit: 4, Structure: 2, Epoch: 1, Lineage: 6, Manifest: 7, BaseVersion: 8, Operation: vector.Input.OperationId, Preview: vector.Input.OperationId, Key: "source", Relation: "SPLIT"})
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(raw)
	original := mainControlContext{Document: 1, Intent: domain.MainControlIntent{Operation: "aligned-confirm-original"}, Access: DocumentAccess{Document: dto.Document{AclRevision: "1"}}, StructurePreview: &structureConfirmationBinding{Reference: vector.Input.OperationId, Digest: digest[:]}}
	installation := &structureInstallation{Preview: vector.Input.OperationId, Digest: digest[:], ACLEpoch: 1}
	install := mainInstallContext{Manifest: 10, Structure: 11, Transition: installation}
	joinedMain := []any{}
	for i, target := range migration.Changes[0].Targets {
		unit, version := uint64(20+i), uint64(30+i)
		context := structureCandidateContext{Document: 1, Unit: unit, Structure: 11, Epoch: 2, Lineage: uint64(40 + i), Manifest: 10, BaseVersion: 8, Operation: original.Intent.Operation, Preview: vector.Input.OperationId, Key: target.Key, Relation: "SPLIT", SourceVersions: map[string]uint64{"4": 8}, Sources: []*pb.FixedContent{source}}
		accepted, err := buildStructureCandidate(validator, target.Main, context)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(accepted.BodyJsonUtf8), "支线独立改动") || strings.Contains(string(accepted.BodyJsonUtf8), "尚未审核追加") {
			t.Fatal("虚拟对齐把尚未审核正文升为MAIN")
		}
		context.Branch, context.Epoch, context.BaseVersion = uint64(50+i), 1, version
		draft, err := buildStructureCandidate(validator, target.Draft, context)
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 && (!strings.Contains(string(draft.BodyJsonUtf8), "支线独立改动") || !strings.Contains(string(draft.BodyJsonUtf8), "尚未审核追加")) {
			t.Fatal("未审核支线改动丢失")
		}
		proof, err := codec.Decode(draft.ProvenanceJsonUtf8)
		if err != nil {
			t.Fatal(err)
		}
		provenance := proof.(map[string]any)
		if len(provenance["accepted_captures"].([]any)) != 0 {
			t.Fatal("未审核来源产生了已接纳capture")
		}
		for _, item := range provenance["block_sources"].([]any) {
			block := item.(map[string]any)
			if block["block_id"] == "pending-only" && (block["origin_capture_ref"] != vector.Input.OperationId || block["origin_content_version_id"] != nil || block["relation"] != "INSERTED") {
				t.Fatal("支线新增块必须继续引用原预览，不能假称MAIN版本来源")
			}
		}
		if i == 1 && (!slices.Equal(draft.ResourceIds, []uint64{9007199254740997}) || !slices.Equal(accepted.ResourceIds, draft.ResourceIds)) {
			t.Fatal("对齐引入的主线图片未进入新MAIN和草稿资源保护")
		}
		room := uint64(60 + i)
		draft.CutJsonUtf8, _ = codec.Canonical(map[string]any{"room_id": domain.TextID(room), "workspace_epoch": "1", "durable_seq": "0"})
		baselineRaw, err := domain.BranchBaselineBody(accepted.BodyJsonUtf8)
		if err != nil {
			t.Fatal(err)
		}
		baseline, _ := codec.Decode(baselineRaw)
		installation.Branches = append(installation.Branches, structureInstalledBranch{Key: target.Key, Unit: unit, Branch: context.Branch, Room: room, Epoch: 1, Lineage: context.Lineage, Content: draft, Context: map[string]any{"lineage_id": domain.TextID(context.Lineage), "base_manifest_id": "10", "base_version_id": domain.TextID(version), "body": baseline, "provenance": provenance}})
		install.Units = append(install.Units, &pb.ManifestUnit{UnitId: unit, ContentVersionId: version})
		heading, _ := codec.Decode(target.Boundary.Heading)
		body, _ := codec.Decode(accepted.BodyJsonUtf8)
		joinedMain = append(joinedMain, heading)
		joinedMain = append(joinedMain, body.(map[string]any)["content"].([]any)...)
	}
	rebuilt, err := codec.Canonical(map[string]any{"type": "doc", "content": joinedMain})
	if err != nil || !bytes.Equal(rebuilt, source.BodyJsonUtf8) {
		t.Fatal("拆分后MAIN未完整保持原固定MAIN")
	}
	if !validStructureInstallation(original, install) {
		t.Fatal("对齐后的候选无法通过正式Go安装校验")
	}
	installation.Branches[0].Context["base_version_id"] = "8"
	if validStructureInstallation(original, install) {
		t.Fatal("新支线谱系不能继续假称旧整章版本为baseline")
	}
}
