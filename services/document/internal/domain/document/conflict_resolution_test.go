package document

import (
	"bytes"
	dto "collaboration/contracts/httpdto"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

func TestConflictResolutionMeasuresCanonicalContentWithoutHTMLTransportExpansion(t *testing.T) {
	input := conflictResolutionInput()
	paragraphs := []any{}
	for i := 0; i < 16; i++ {
		paragraphs = append(paragraphs, map[string]any{"type": "paragraph", "attrs": map[string]string{"id": "literal-" + strconv.Itoa(i)}, "content": []any{map[string]string{"type": "text", "text": strings.Repeat("<&>", 1<<16)}}})
	}
	var raw bytes.Buffer
	encoder := json.NewEncoder(&raw)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(paragraphs); err != nil {
		t.Fatal(err)
	}
	input.ResolutionContent["content"] = json.RawMessage(bytes.TrimSpace(raw.Bytes()))
	// 合法文本只是三MiB。encoding/json默认HTML转义会膨胀到十八MiB，不能把它当成规范正文超限。
	intent, err := ParseConflictResolution("original-conflict-operation", input)
	if err != nil || len(intent.Body) > 4<<20 || !bytes.Contains(intent.Body, []byte("<&>")) {
		t.Fatal("literal text rejected or changed by transport escaping", err)
	}
}

func TestConflictResolutionFingerprintSurvivesDatabaseJSONKeyOrder(t *testing.T) {
	left := conflictResolutionInput()
	right := conflictResolutionInput()
	// MySQL JSON合法重排嵌套正文对象的键；原号指纹必须绑定同一规范正文，而不是键的输入顺序。
	right.ResolutionContent["content"] = json.RawMessage(`[{"content":[{"text":"人工修复","type":"text"}],"attrs":{"id":"original"},"type":"paragraph"}]`)
	a, err := ParseConflictResolution("original-conflict-operation", left)
	if err != nil {
		t.Fatal(err)
	}
	b, err := ParseConflictResolution("original-conflict-operation", right)
	if err != nil {
		t.Fatal(err)
	}
	first, err := ConflictResolutionFingerprint("/v1/conflict-contexts/original-conflict-operation/resolutions", a)
	if err != nil {
		t.Fatal(err)
	}
	second, err := ConflictResolutionFingerprint("/v1/conflict-contexts/original-conflict-operation/resolutions", b)
	if err != nil || !bytes.Equal(first[:], second[:]) {
		t.Fatal("database key order changed original body fingerprint", err)
	}
	right.ResolutionContent["content"] = json.RawMessage(`[{"type":"paragraph","attrs":{"id":"original"},"content":[{"type":"text","text":"另一份正文"}]}]`)
	changed, err := ParseConflictResolution("original-conflict-operation", right)
	if err != nil {
		t.Fatal(err)
	}
	third, err := ConflictResolutionFingerprint("/v1/conflict-contexts/original-conflict-operation/resolutions", changed)
	if err != nil || bytes.Equal(first[:], third[:]) {
		t.Fatal("different proposed body reused original fingerprint", err)
	}
}

func conflictResolutionInput() dto.ConflictResolution {
	return dto.ConflictResolution{
		OperationId:         "resolution-operation-01",
		ExpectedAttemptId:   "merge-attempt-00000000000000000042",
		ExpectedInputDigest: strings.Repeat("a", 64),
		ExpectedRoundId:     "17",
		ResolutionContent: map[string]json.RawMessage{
			"type":    json.RawMessage(`"doc"`),
			"content": json.RawMessage(`[{"type":"paragraph","attrs":{"id":"original"},"content":[{"type":"text","text":"人工修复"}]}]`),
		},
		ResolvedConflictIds: []string{"merge-conflict-00000000000000000042-0001"},
	}
}

func TestConflictReferencesUseExactRealAttemptAndOriginalOrdinal(t *testing.T) {
	if ConflictAttemptRef(42) != "merge-attempt-00000000000000000042" || ConflictItemRef(42, 1) != "merge-conflict-00000000000000000042-0001" {
		t.Fatal("public references must encode the real attempt without random aliases")
	}
	value, err := ParseConflictResolution("original-conflict-operation", conflictResolutionInput())
	if err != nil || value.Attempt != 42 || value.Round != 17 || len(value.Digest) != 32 || value.Conflict != "original-conflict-operation" || value.Operation != "resolution-operation-01" {
		t.Fatalf("valid fixed resolution rejected: %#v, %v", value, err)
	}
}

func TestConflictResolutionRejectsMalformedReferencesAndIncompleteSets(t *testing.T) {
	for _, change := range []struct {
		name string
		run  func(*dto.ConflictResolution)
	}{
		{"short operation", func(v *dto.ConflictResolution) { v.OperationId = "short" }},
		{"database key alias", func(v *dto.ConflictResolution) { v.ExpectedAttemptId = "42" }},
		{"zero attempt", func(v *dto.ConflictResolution) { v.ExpectedAttemptId = "merge-attempt-00000000000000000000" }},
		{"overflow attempt", func(v *dto.ConflictResolution) { v.ExpectedAttemptId = "merge-attempt-18446744073709551616" }},
		{"noncanonical alias", func(v *dto.ConflictResolution) { v.ExpectedAttemptId = "merge-attempt-42" }},
		{"uppercase digest", func(v *dto.ConflictResolution) { v.ExpectedInputDigest = strings.Repeat("A", 64) }},
		{"wrong digest size", func(v *dto.ConflictResolution) { v.ExpectedInputDigest = "aa" }},
		{"round alias", func(v *dto.ConflictResolution) { v.ExpectedRoundId = "017" }},
		{"no conflict IDs", func(v *dto.ConflictResolution) { v.ResolvedConflictIds = nil }},
		{"duplicate conflict", func(v *dto.ConflictResolution) {
			v.ResolvedConflictIds = append(v.ResolvedConflictIds, v.ResolvedConflictIds[0])
		}},
		{"cross attempt item", func(v *dto.ConflictResolution) { v.ResolvedConflictIds[0] = "merge-conflict-00000000000000000043-0001" }},
		{"zero ordinal", func(v *dto.ConflictResolution) { v.ResolvedConflictIds[0] = "merge-conflict-00000000000000000042-0000" }},
		{"missing body", func(v *dto.ConflictResolution) { v.ResolutionContent = nil }},
	} {
		t.Run(change.name, func(t *testing.T) {
			input := conflictResolutionInput()
			change.run(&input)
			if _, err := ParseConflictResolution("original-conflict-operation", input); err == nil {
				t.Fatal("unsafe resolution intent accepted")
			}
		})
	}
}

func TestConflictResolutionChecksEntireOriginalConflictSet(t *testing.T) {
	value, err := ParseConflictResolution("original-conflict-operation", conflictResolutionInput())
	if err != nil {
		t.Fatal(err)
	}
	if value.ValidateConflictSet(1) != nil || value.ValidateConflictSet(2) == nil {
		t.Fatal("original conflict set cannot be truncated")
	}
}
