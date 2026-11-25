package document

import (
	dto "collaboration/contracts/httpdto"
	"errors"
	"math"
	"testing"
)

const publicationOperation = "publication-operation-01"

func TestPublicationParsesCanonicalIntent(t *testing.T) {
	// 发布初始水位允许零；版本编号不允许零。超过JS安全整数仍保存完整uint64。
	created, err := ParseReleaseCreate(dto.ReleaseCreate{
		OperationId: publicationOperation, ExpectedPublicationRevision: "0",
		ExpectedControlRevision: "18446744073709551615",
	})
	if err != nil || created.Kind != "RELEASE_CREATE" || created.Publication != 0 || created.Control != math.MaxUint64 || created.Release != 0 {
		t.Fatalf("release intent changed: %+v, %v", created, err)
	}
	release := "9007199254740993"
	switched, err := ParsePublication(dto.Publication{
		OperationId: publicationOperation, ExpectedPublicationRevision: "18446744073709551615",
		Action: "SWITCH", ReleaseId: &release,
	})
	if err != nil || switched.Kind != "SWITCH" || switched.Release != 9007199254740993 || switched.Publication != math.MaxUint64 {
		t.Fatalf("switch intent lost precision: %+v, %v", switched, err)
	}
	unpublished, err := ParsePublication(dto.Publication{OperationId: publicationOperation, ExpectedPublicationRevision: "3", Action: "UNPUBLISH"})
	if err != nil || unpublished.Kind != "UNPUBLISH" || unpublished.Release != 0 {
		t.Fatalf("unpublish intent changed: %+v, %v", unpublished, err)
	}
}

func TestPublicationRejectsAmbiguousInput(t *testing.T) {
	for _, value := range []string{"", "01", "+1", "-1", " 1", "1 ", "1.0", "1e3", "１", "18446744073709551616"} {
		t.Run("revision_"+value, func(t *testing.T) {
			_, err := ParseReleaseCreate(dto.ReleaseCreate{OperationId: publicationOperation, ExpectedPublicationRevision: value, ExpectedControlRevision: "0"})
			publicationFailure(t, err, InvalidRequest, "REVISION_INVALID")
			_, err = ParsePublication(dto.Publication{OperationId: publicationOperation, ExpectedPublicationRevision: value, Action: "UNPUBLISH"})
			publicationFailure(t, err, InvalidRequest, "REVISION_INVALID")
		})
	}
	for _, control := range []string{"", "01", "+1", "18446744073709551616"} {
		_, err := ParseReleaseCreate(dto.ReleaseCreate{OperationId: publicationOperation, ExpectedPublicationRevision: "0", ExpectedControlRevision: control})
		publicationFailure(t, err, InvalidRequest, "REVISION_INVALID")
	}
	for _, operation := range []string{"", "short", "../publication-operation", "publication operation"} {
		_, err := ParseReleaseCreate(dto.ReleaseCreate{OperationId: operation, ExpectedPublicationRevision: "0", ExpectedControlRevision: "0"})
		publicationFailure(t, err, InvalidRequest, "PUBLICATION_INPUT_INVALID")
		_, err = ParsePublication(dto.Publication{OperationId: operation, ExpectedPublicationRevision: "0", Action: "UNPUBLISH"})
		publicationFailure(t, err, InvalidRequest, "PUBLICATION_INPUT_INVALID")
	}
	for _, release := range []string{"", "0", "01", "+1", "18446744073709551616"} {
		_, err := ParsePublication(dto.Publication{OperationId: publicationOperation, ExpectedPublicationRevision: "0", Action: "SWITCH", ReleaseId: &release})
		publicationFailure(t, err, InvalidRequest, "ID_INVALID")
	}
	for _, input := range []dto.Publication{
		{Action: "SWITCH"}, {Action: "UNPUBLISH", ReleaseId: stringPointer("1")},
		{Action: "UNPUBLISH", ReleaseId: stringPointer("")}, {Action: "switch"}, {Action: "RELEASE_CREATE"},
	} {
		input.OperationId, input.ExpectedPublicationRevision = publicationOperation, "0"
		_, err := ParsePublication(input)
		publicationFailure(t, err, InvalidRequest, "PUBLICATION_INPUT_INVALID")
	}
}

func TestPublicationRequiresCurrentManagementAndPosition(t *testing.T) {
	current := dto.Document{Lifecycle: "ACTIVE", PublicationState: "PUBLISHED", PublicationEpoch: "7", ControlRevision: "9", CurrentReleaseId: stringPointer("42")}
	create := PublicationIntent{Operation: publicationOperation, Kind: "RELEASE_CREATE", Publication: 7, Control: 9}
	if err := ValidatePublication(Permissions{Manage: true}, current, create); err != nil {
		t.Fatal(err)
	}
	publicationFailure(t, ValidatePublication(Permissions{Read: true, Review: true}, current, create), PermissionDenied, "PUBLICATION_NOT_ALLOWED")
	for _, lifecycle := range []string{"TRASHED", "PURGING", "RETAINED_PUBLIC", "DELETED"} {
		changed := current
		changed.Lifecycle = lifecycle
		publicationFailure(t, ValidatePublication(Permissions{Manage: true}, changed, create), StateConflict, "DOCUMENT_NOT_ACTIVE")
	}
	stale := create
	stale.Publication = 6
	publicationFailure(t, ValidatePublication(Permissions{Manage: true}, current, stale), StateConflict, "PUBLICATION_CHANGED")
	stale = create
	stale.Control = 8
	publicationFailure(t, ValidatePublication(Permissions{Manage: true}, current, stale), StateConflict, "MAIN_CONTROL_CHANGED")
	// 切回历史仅改正式展示指针，无须当前MAIN控制修订；归属须在最终事务另行验证。
	switchIntent := PublicationIntent{Operation: publicationOperation, Kind: "SWITCH", Publication: 7, Release: 41}
	if err := ValidatePublication(Permissions{Manage: true}, current, switchIntent); err != nil {
		t.Fatal(err)
	}
	switchIntent.Release = 42
	publicationFailure(t, ValidatePublication(Permissions{Manage: true}, current, switchIntent), StateConflict, "PUBLICATION_ALREADY_CURRENT")
	unpublish := PublicationIntent{Operation: publicationOperation, Kind: "UNPUBLISH", Publication: 7}
	if err := ValidatePublication(Permissions{Manage: true}, current, unpublish); err != nil {
		t.Fatal(err)
	}
	current.PublicationState, current.CurrentReleaseId = "UNPUBLISHED", nil
	publicationFailure(t, ValidatePublication(Permissions{Manage: true}, current, unpublish), StateConflict, "PUBLICATION_NOT_PUBLISHED")
	current.PublicationEpoch = "18446744073709551615"
	create.Publication = math.MaxUint64
	publicationFailure(t, ValidatePublication(Permissions{Manage: true}, current, create), StateConflict, "PUBLICATION_SEQUENCE_EXHAUSTED")
}

func TestPublicationSequencesNeverWrap(t *testing.T) {
	for _, input := range [][2]uint64{{0, 0}, {9007199254740993, 9223372036854775808}, {math.MaxUint64 - 1, math.MaxUint64 - 1}} {
		epoch, event, err := NextPublicationSequences(input[0], input[1])
		if err != nil || epoch != input[0]+1 || event != input[1]+1 {
			t.Fatalf("publication sequence changed: %d/%d, %v", epoch, event, err)
		}
	}
	for _, input := range [][2]uint64{{math.MaxUint64, 1}, {1, math.MaxUint64}, {math.MaxUint64, math.MaxUint64}} {
		_, _, err := NextPublicationSequences(input[0], input[1])
		publicationFailure(t, err, StateConflict, "PUBLICATION_SEQUENCE_EXHAUSTED")
	}
}

func publicationFailure(t *testing.T, err error, kind FailureKind, code string) {
	t.Helper()
	var violation RuleViolation
	if !errors.As(err, &violation) || violation.Kind != kind || violation.Code != code {
		t.Fatalf("expected %v/%s, got %v", kind, code, err)
	}
}

func stringPointer(value string) *string { return &value }
