package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	pb "collaboration/contracts/internalv1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func TestAccountCollaboratorNameUsesOnlyOriginalBindingAndStrictResponse(t *testing.T) {
	// 适配只发送已取得的准入绑定，不转发浏览器Cookie；其他目标或额外邮箱字段必须关闭输出。
	proof := &pb.IdentityRevalidation{AdmissionId: "collaborator-admission-original", OperationId: "collaborator-operation-original",
		Purpose: "DOCUMENT_COLLABORATORS", Target: &pb.Target{DocumentId: 10, Purpose: "DOCUMENT_COLLABORATORS"},
		IdentityTarget: &pb.IdentityTarget{Kind: pb.IdentityTarget_USER, UserId: 2}}
	for _, body := range []string{
		`{"user_id":"2","display_name":"李小明"}`,
		`{"user_id":"99","display_name":"其他用户"}`,
		`{"user_id":"2","display_name":"李小明","email":"private@example.invalid"}`,
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/infra/identity/document-collaborator" || r.Method != http.MethodPost || r.Header.Get("Cookie") != "" {
				t.Error("collaborator adapter changed its internal boundary")
			}
			var raw json.RawMessage
			input := &pb.IdentityRevalidation{}
			if json.NewDecoder(r.Body).Decode(&raw) != nil || protojson.Unmarshal(raw, input) != nil || !proto.Equal(proof, input) {
				t.Error("collaborator adapter changed the original proof binding")
			}
			_, _ = w.Write([]byte(body))
		}))
		value, err := NewAccountDirectory(server.URL, server.Client()).Collaborator(context.Background(), proof)
		server.Close()
		if body == `{"user_id":"2","display_name":"李小明"}` {
			if err != nil || value.UserId != "2" || value.DisplayName != "李小明" {
				t.Fatalf("authoritative name response: %+v error=%v", value, err)
			}
		} else if err == nil {
			t.Fatalf("invalid collaborator response was accepted: %s", body)
		}
	}
}
