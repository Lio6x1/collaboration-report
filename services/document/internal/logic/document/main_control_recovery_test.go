package document

import (
	"errors"
	"testing"

	domain "collaboration/document/internal/domain/document"
	"github.com/go-sql-driver/mysql"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// 持久重试只保存可定位的类别和编号，不能把SQL正文、私有密码或RPC详情写入操作状态。
func TestMainControlRecoveryCode(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{domain.Violation(domain.StateConflict, "MAIN_CONTEXT_CHANGED"), "MAIN_CONTEXT_CHANGED"},
		{&mysql.MySQLError{Number: 1406, Message: "private statement"}, "MAIN_CONTROL_SQL_1406"},
		{status.Error(codes.Unavailable, "private RPC detail"), "MAIN_CONTROL_RPC_Unavailable"},
		{errors.New("private credentials"), "MAIN_CONTROL_RECOVERY_UNKNOWN"},
	}
	for _, c := range cases {
		if got := mainControlRecoveryCode(c.err); got != c.want {
			t.Errorf("code = %q, want %q", got, c.want)
		}
	}
}
