package document

import (
	"errors"
	"testing"
)

// 启动时要能定位退出证明失败，但驱动信息可能含私有SQL/配置，不能原样写入日志。
func TestOutputRecoveryDiagnosticUsesOnlyStableCodes(t *testing.T) {
	for _, sample := range []struct{ input, want string }{
		{"OUTPUT_EXIT_PROOF_UNAVAILABLE", "OUTPUT_EXIT_PROOF_UNAVAILABLE"},
		{"OUTPUT_EXIT_PROOF_DEADLINE", "OUTPUT_EXIT_PROOF_DEADLINE"},
		{"OLD_OUTPUT_PROCESS_STILL_RUNNING", "OLD_OUTPUT_PROCESS_STILL_RUNNING"},
		{"OUTPUT_INSTANCE_UNKNOWN", "OUTPUT_INSTANCE_UNKNOWN"},
		{"mysql: private-password in statement", "OUTPUT_RECOVERY_UNKNOWN"},
		{"UNKNOWN_DRIVER_SECRET", "OUTPUT_RECOVERY_UNKNOWN"},
	} {
		if got := OutputRecoveryCode(errors.New(sample.input)); got != sample.want {
			t.Fatalf("unexpected safe diagnostic code: %s", got)
		}
	}
}
