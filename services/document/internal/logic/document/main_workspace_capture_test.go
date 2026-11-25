package document

import (
	codec "collaboration/contracts/content"
	"testing"
)

func TestReliableMainCutBindsItsPhysicalEpochSeparatelyFromMetadataControl(t *testing.T) {
	service, fixed, main := mainReadCaptureFixture(t)
	fixed.MainEpoch = 1
	main.Cut.Rooms[0].WorkspaceEpoch = 1
	cut, err := codec.Canonical(map[string]any{"room_id": "19", "workspace_epoch": "1", "durable_seq": "1"})
	if err != nil {
		t.Fatal(err)
	}
	main.Units[0].CutJsonUtf8 = cut
	sealMainReadCapture(t, main)
	if err := validateReliableMainCapture(service.validator, mainReadBinding(fixed), main, "MAIN_READ_VIEW:1", "MAIN_READ"); err != nil {
		t.Fatal("actual physical epoch must remain bound when metadata advances")
	}
	for _, wrong := range []uint64{0, 2, 3} {
		main.Cut.Rooms[0].WorkspaceEpoch = wrong
		sealMainReadCapture(t, main)
		if err := validateReliableMainCapture(service.validator, mainReadBinding(fixed), main, "MAIN_READ_VIEW:1", "MAIN_READ"); err == nil {
			t.Fatal("another physical room epoch cannot enter the fixed MAIN cut")
		}
	}
}
