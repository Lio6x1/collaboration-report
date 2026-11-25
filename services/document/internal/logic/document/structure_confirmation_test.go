package document

import (
	dto "collaboration/contracts/httpdto"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"encoding/json"
	"testing"
	"time"
)

func structureConfirmationBindingFixture(t *testing.T) (Actor, DocumentAccess, dto.StructureChange, repository.PendingStructurePreviewBinding) {
	t.Helper()
	preview, fact := pendingStructurePreviewFixture(t)
	preview.Intent.Input.Changes[0].Type, preview.Intent.Input.Changes[0].Title = "REMOVE", nil
	intent, err := domain.ParseStructurePreview(preview.Intent.Input)
	if err != nil {
		t.Fatal(err)
	}
	preview.Intent = intent
	fingerprint, err := domain.OperationFingerprint("/v1/documents/3/structure-previews", intent.Input)
	if err != nil {
		t.Fatal(err)
	}
	preview.Context.RequestFingerprint = fingerprint[:]
	fact.Fingerprint = fingerprint[:]
	fact.State = "SUCCEEDED"
	fact.Original, _ = json.Marshal(preview)
	var input dto.StructureChange
	if err = json.Unmarshal([]byte(`{"operation_id":"structure-confirm-original","expected_structure_id":"5","expected_control_revision":"2","changes":[{"type":"REMOVE","chapter_id":"9","preview_cut_ref":"structure-preview-original"}]}`), &input); err != nil {
		t.Fatal(err)
	}
	var access DocumentAccess
	copy, _ := json.Marshal(preview.Access)
	if err = json.Unmarshal(copy, &access); err != nil {
		t.Fatal(err)
	}
	access.Proofs[0].Purpose = "MAIN_CONTROL"
	access.Proofs[0].Target.Purpose = "MAIN_CONTROL"
	access.Proofs[0].OperationId = input.OperationId
	access.Proofs[0].ExpiresUnixMs = time.Now().Add(time.Minute).UnixMilli()
	actor := Actor{ID: preview.Actor, SessionExpiresUnixMs: preview.SessionExpires, Proof: access.Proofs[0]}
	return actor, access, input, fact
}

func TestStructureConfirmationCannotTransferOriginalSessionOrSource(t *testing.T) {
	actor, access, input, fact := structureConfirmationBindingFixture(t)
	if _, err := validateStructureConfirmationActor(actor, access, input, fact, time.Now()); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		name string
		edit func(*Actor, *DocumentAccess, *repository.PendingStructurePreviewBinding)
	}{
		{"actor", func(a *Actor, _ *DocumentAccess, _ *repository.PendingStructurePreviewBinding) { a.ID++ }},
		{"session", func(_ *Actor, a *DocumentAccess, _ *repository.PendingStructurePreviewBinding) {
			a.Proofs[0].SessionRef[0] = 1
		}},
		{"session expiry", func(a *Actor, _ *DocumentAccess, _ *repository.PendingStructurePreviewBinding) {
			a.SessionExpiresUnixMs++
		}},
		{"expired session", func(a *Actor, _ *DocumentAccess, _ *repository.PendingStructurePreviewBinding) {
			a.SessionExpiresUnixMs = time.Now().Add(-time.Second).UnixMilli()
		}},
		{"source epoch", func(_ *Actor, a *DocumentAccess, _ *repository.PendingStructurePreviewBinding) { a.Sources[1].Epoch++ }},
		{"missing source", func(_ *Actor, a *DocumentAccess, _ *repository.PendingStructurePreviewBinding) {
			a.Sources = a.Sources[:1]
		}},
		{"missing proof", func(_ *Actor, a *DocumentAccess, _ *repository.PendingStructurePreviewBinding) { a.Proofs = nil }},
		{"nil proof", func(_ *Actor, a *DocumentAccess, _ *repository.PendingStructurePreviewBinding) { a.Proofs[0] = nil }},
		{"expired proof", func(_ *Actor, a *DocumentAccess, _ *repository.PendingStructurePreviewBinding) {
			a.Proofs[0].ExpiresUnixMs = time.Now().Add(-time.Second).UnixMilli()
		}},
		{"not manager", func(_ *Actor, a *DocumentAccess, _ *repository.PendingStructurePreviewBinding) {
			a.Permissions.Manage = false
		}},
		{"locked main", func(_ *Actor, a *DocumentAccess, _ *repository.PendingStructurePreviewBinding) {
			a.Document.MainLocked = true
		}},
		{"document", func(_ *Actor, a *DocumentAccess, _ *repository.PendingStructurePreviewBinding) { a.Document.Id = "99" }},
		{"manifest", func(_ *Actor, a *DocumentAccess, _ *repository.PendingStructurePreviewBinding) {
			a.Document.CurrentManifestId = "99"
		}},
		{"style", func(_ *Actor, a *DocumentAccess, _ *repository.PendingStructurePreviewBinding) {
			a.Document.CurrentStyleId = "99"
		}},
		{"structure", func(_ *Actor, a *DocumentAccess, _ *repository.PendingStructurePreviewBinding) {
			a.Document.CurrentStructureId = "99"
		}},
		{"control", func(_ *Actor, a *DocumentAccess, _ *repository.PendingStructurePreviewBinding) {
			a.Document.ControlRevision = "99"
		}},
		{"unfinished preview", func(_ *Actor, _ *DocumentAccess, f *repository.PendingStructurePreviewBinding) { f.State = "PREPARING" }},
		{"rejected preview", func(_ *Actor, _ *DocumentAccess, f *repository.PendingStructurePreviewBinding) { f.State = "REJECTED" }},
		{"original fingerprint", func(_ *Actor, _ *DocumentAccess, f *repository.PendingStructurePreviewBinding) {
			f.Fingerprint = make([]byte, 32)
		}},
	} {
		t.Run(item.name, func(t *testing.T) {
			actor, access, input, fact := structureConfirmationBindingFixture(t)
			item.edit(&actor, &access, &fact)
			if _, err := validateStructureConfirmationActor(actor, access, input, fact, time.Now()); err == nil {
				t.Fatal("changed original binding accepted")
			}
		})
	}
}
