package document

import (
	dto "collaboration/contracts/httpdto"
	"strings"
	"testing"
)

func TestReverseChangeRequiresCanonicalOriginalSourceAndReason(t *testing.T) {
	valid := dto.ReverseChange{OperationId: "reverse-original-01", ExpectedManifestId: "11", MergeRecordId: "22", Reason: "保留旧合并，创建新审核"}
	intent, err := ParseReverseChange(valid)
	if err != nil || intent.Operation != valid.OperationId || intent.Manifest != 11 || intent.Merge != 22 || intent.Reason != valid.Reason {
		t.Fatalf("valid original input: %#v %v", intent, err)
	}
	for _, mutate := range []func(*dto.ReverseChange){
		func(v *dto.ReverseChange) { v.OperationId = "" },
		func(v *dto.ReverseChange) { v.ExpectedManifestId = "0" },
		func(v *dto.ReverseChange) { v.ExpectedManifestId = "011" },
		func(v *dto.ReverseChange) { v.MergeRecordId = "18446744073709551616" },
		func(v *dto.ReverseChange) { v.Reason = " \t " },
		func(v *dto.ReverseChange) { v.Reason = strings.Repeat("反", 2001) },
	} {
		bad := valid
		mutate(&bad)
		if _, err = ParseReverseChange(bad); err == nil {
			t.Fatalf("invalid original input accepted: %#v", bad)
		}
	}
}
