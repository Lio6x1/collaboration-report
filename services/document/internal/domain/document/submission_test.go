package document

import (
	"bytes"
	"testing"

	dto "collaboration/contracts/httpdto"
)

func TestSubmissionInputHasCanonicalBusinessIdentity(t *testing.T) {
	first, err := ParseSubmission(dto.Submission(`{"operation_id":"submit-operation-0001","branch_id":"9007199254740993","expected_structure_id":"5","flush_receipt_id":"flush-operation-0001","mode":"NEW"}`))
	if err != nil {
		t.Fatal(err)
	}
	second, err := ParseSubmission(dto.Submission(`{ "mode":"NEW", "flush_receipt_id":"flush-operation-0001", "expected_structure_id":"5", "branch_id":"9007199254740993", "operation_id":"submit-operation-0001" }`))
	if err != nil {
		t.Fatal(err)
	}
	if first.Branch != 9007199254740993 || !bytes.Equal(first.Canonical, second.Canonical) {
		t.Fatal("same public intent differs")
	}
	for _, input := range []string{
		`{"operation_id":"submit-operation-0001","branch_id":"03","expected_structure_id":"5","flush_receipt_id":"flush-operation-0001","mode":"NEW"}`,
		`{"operation_id":"submit-operation-0001","branch_id":"3","expected_structure_id":"5","flush_receipt_id":"flush-operation-0001","mode":"RESUBMIT"}`,
		`{"operation_id":"submit-operation-0001","branch_id":"3","expected_structure_id":"5","flush_receipt_id":"flush-operation-0001","mode":"NEW","request_id":"1"}`,
		`{"operation_id":"submit-operation-0001","branch_id":"3","expected_structure_id":"5","flush_receipt_id":"flush-operation-0001","mode":"NEW","body_json":{}}`,
	} {
		if _, err := ParseSubmission(dto.Submission(input)); err == nil {
			t.Fatal("invalid submission accepted")
		}
	}
}
