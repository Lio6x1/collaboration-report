package index

import (
	"math"
	"testing"
)

func TestFullUnsignedWatermarksSortWithoutSignedWrap(t *testing.T) {
	values := []uint64{0, 1, 9, 10, math.MaxInt64, math.MaxInt64 + 1, math.MaxUint64}
	previous := ""
	for _, value := range values {
		actual := Watermark(value)
		if len(actual) != 20 || (previous != "" && actual <= previous) {
			t.Fatal("UNSIGNED_WATERMARK_ORDER_CHANGED")
		}
		previous = actual
	}
}

func TestSameSequenceDifferentAuthorityCannotOverwriteAndOldCannotRevive(t *testing.T) {
	current := State{Sequence: math.MaxUint64, Digest: [32]byte{1}, Visible: false, Policy: "PUBLIC"}
	older := Authority{Sequence: math.MaxInt64, Digest: [32]byte{2}, Visible: true, Release: 7, Policy: "PUBLIC"}
	apply, err := ShouldApply(current, older)
	if err != nil || apply {
		t.Fatal("OLD_VISIBLE_EVENT_REVIVED_TOMBSTONE")
	}
	same := Authority{Sequence: math.MaxUint64, Digest: current.Digest, Policy: "PUBLIC"}
	apply, err = ShouldApply(current, same)
	if err != nil || apply {
		t.Fatal("IDENTICAL_AUTHORITY_NOT_IDEMPOTENT")
	}
	same.Digest = [32]byte{3}
	if _, err = ShouldApply(current, same); err == nil {
		t.Fatal("SAME_SEQUENCE_AUTHORITY_CONFLICT_ACCEPTED")
	}
	newer := Authority{Sequence: 2, Digest: [32]byte{4}, Visible: true, Release: 7, Policy: "COLLABORATORS"}
	apply, err = ShouldApply(State{Sequence: 1, Digest: [32]byte{1}, Policy: "PUBLIC"}, newer)
	if err != nil || !apply {
		t.Fatal("NEW_CURRENT_AUTHORITY_NOT_APPLIED")
	}
}
