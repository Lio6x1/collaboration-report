package document

import (
	codec "collaboration/contracts/content"
	dto "collaboration/contracts/httpdto"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"encoding/json"
	"strings"
	"testing"
)

func TestResolutionInstallPreservesCanonicalIntegerSources(t *testing.T) {
	value := resolutionInstall{}
	value.LineageContext = map[string]any{"body": map[string]any{"type": "doc", "content": []any{map[string]any{"type": "heading", "attrs": map[string]any{"id": "title", "level": json.Number("2")}}}}, "provenance": map[string]any{"source_range": map[string]any{"start": json.Number("0"), "end": json.Number("0")}}}
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeResolutionInstall(raw)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := codec.Canonical(decoded.LineageContext); err != nil {
		t.Fatal("integer provenance and heading fields were converted to floats", err)
	}
}

func TestResolutionReadinessAcceptsExactCommittedContextAfterProtoComparison(t *testing.T) {
	fixed, fact := pendingResolutionFixture(t)
	install := resolutionInstall{Fixed: fixed, Version: 23, draftInstall: draftInstall{Context: fixed.Context, Branch: fixed.Scope.Workspace.Workspace, OldRoom: fixed.Scope.Room, NewRoom: 24, Epoch: fixed.Scope.Workspace.Epoch + 1, Lineage: 25, Preview: fact.Operation, Content: &pb.FixedContent{OperationId: fact.Operation, CaptureId: fact.Operation + ":candidate"}}}
	fact.State, fact.Kind = "SUCCEEDED", "RESOLUTION_RECONCILE"
	fact.Payload, _ = json.Marshal(install)
	fact.Committed = fact.Payload
	// 完整原绑定相同；protobuf比较会初始化内部状态，不应把其缓存当成业务字段差异。
	if _, err := validatePendingResolutionBinding(fact); err != nil {
		t.Fatal(err)
	}
}

func pendingResolutionFixture(t *testing.T) (resolutionContext, repository.PendingResolutionBinding) {
	t.Helper()
	seed, _ := pendingDiscardPreviewFixture(t)
	input := dto.ConflictResolution{OperationId: "manual-resolution-original", ExpectedAttemptId: domain.ConflictAttemptRef(42), ExpectedInputDigest: strings.Repeat("a", 64), ExpectedRoundId: "17", ResolvedConflictIds: []string{domain.ConflictItemRef(42, 1)}, ResolutionContent: map[string]json.RawMessage{"type": json.RawMessage(`"doc"`), "content": json.RawMessage(`[{"type":"paragraph","attrs":{"id":"original"},"content":[{"type":"text","text":"人工修复"}]}]`)}}
	intent, err := domain.ParseConflictResolution("original-conflict-operation", input)
	if err != nil {
		t.Fatal(err)
	}
	fixed := resolutionContext{Actor: seed.Actor, SessionExpires: seed.SessionExpires, Intent: intent, Access: seed.Access, Scope: seed.Scope, Context: seed.Context, BaselineVersion: 21,
		Origin:   domain.MergeFact{Operation: intent.Conflict, State: "REJECTED", Actor: 7, Request: 16, Round: 17, Version: 20, Document: 3, Unit: 4, Branch: 6},
		Versions: conflictVersions{Attempt: 42, Base: 19, Submitted: 20, Main: 21, Digest: intent.Digest}, Review: domain.ReviewPointer{ID: 16, Document: 3, Unit: 4, Branch: 6, Round: 17, Status: "WITHDRAWN"}, Guard: domain.ReviewGuard{Resumable: 16, Generation: 3}}
	fixed.Access.Permissions.Edit = map[uint64]bool{4: true}
	fixed.Context.OperationId = intent.Operation
	fixed.Context.Target.Purpose = "CONFLICT_RESOLUTION"
	path := "/v1/conflict-contexts/" + intent.Conflict + "/resolutions"
	fingerprint, err := domain.ConflictResolutionFingerprint(path, intent)
	if err != nil {
		t.Fatal(err)
	}
	fixed.Context.RequestFingerprint = fingerprint[:]
	for _, proof := range fixed.Access.Proofs {
		proof.OperationId, proof.Purpose, proof.Target = intent.Operation, "CONFLICT_RESOLUTION", fixed.Context.Target
	}
	raw, _ := json.Marshal(fixed)
	target, _ := json.Marshal(map[string]string{"path": path})
	return fixed, repository.PendingResolutionBinding{Operation: intent.Operation, Kind: "RESOLUTION_RECOVER", State: "PREPARING", Owner: intent.Operation, Actor: fixed.Actor, Original: raw, Payload: raw, Target: target, Fingerprint: fingerprint[:]}
}

func TestResolutionReadinessBindsOriginalIntentEvenWhenTwoTasksAgree(t *testing.T) {
	_, valid := pendingResolutionFixture(t)
	if _, err := validatePendingResolutionBinding(valid); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		name string
		edit func(*resolutionContext)
	}{
		{"different body", func(f *resolutionContext) {
			f.Intent.Input.ResolutionContent["content"] = json.RawMessage(`[{"type":"paragraph","attrs":{"id":"original"},"content":[{"type":"text","text":"另一份正文"}]}]`)
		}},
		{"different original round", func(f *resolutionContext) { f.Origin.Round++ }},
		{"different digest metadata", func(f *resolutionContext) { f.Intent.Digest[0]++ }},
		{"missing edit", func(f *resolutionContext) { f.Access.Permissions.Edit[4] = false }},
		{"different original source", func(f *resolutionContext) { f.Access.Proofs[0].Sources[0].Epoch++ }},
		{"different withdrawn request", func(f *resolutionContext) { f.Guard.Resumable++ }},
		{"different target purpose", func(f *resolutionContext) { f.Context.Target.Purpose = "PULL_MAIN" }},
		{"missing proof", func(f *resolutionContext) { f.Access.Proofs[0] = nil }},
	} {
		t.Run(item.name, func(t *testing.T) {
			fixed, fact := pendingResolutionFixture(t)
			item.edit(&fixed)
			raw, _ := json.Marshal(fixed)
			fact.Original, fact.Payload = raw, raw
			if _, err := validatePendingResolutionBinding(fact); err == nil {
				t.Fatal("changed input must remain unknown even when two mutable tasks agree")
			}
		})
	}
	valid.Owner = "another-branch-control-owner"
	if _, err := validatePendingResolutionBinding(valid); err == nil {
		t.Fatal("another owner accepted")
	}
}
