package platform

import (
	"bytes"
	pb "collaboration/contracts/internalv1"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"regexp"
	"strconv"
	"strings"
)

var fenceID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{15,95}$`)
var sessionScope = regexp.MustCompile(`^session:[a-f0-9]{64}$`)
var credentialScope = regexp.MustCompile(`^credential:[1-9][0-9]{0,19}$`)

// 输出依赖包含会话哈希；组织/账号变更来源本身沿用ValidAccountScope的十进制身份规则。
func ValidAccountOutputScope(key string) bool {
	return sessionScope.MatchString(key) || ValidAccountScope(key)
}

func ValidAccountScope(key string) bool {
	parts := strings.Split(key, ":")
	count := 0
	switch parts[0] {
	case "identity", "credential", "platform-admin":
		count = 1
	case "org-member", "org-admin", "org-owner", "org-create":
		count = 2
	default:
		return false
	}
	if len(parts) != count+1 {
		return false
	}
	for _, part := range parts[1:] {
		value, err := strconv.ParseUint(part, 10, 64)
		if err != nil || value == 0 || strconv.FormatUint(value, 10) != part {
			return false
		}
	}
	return true
}

func ValidDocumentScope(key string) bool {
	parts := strings.Split(key, ":")
	// 正式版切换由Doc关闭旧publication水位；它不是账号身份准入来源。
	if len(parts) != 2 || (parts[0] != "doc-acl" && parts[0] != "kb-acl" && parts[0] != "publication") {
		return false
	}
	id, err := strconv.ParseUint(parts[1], 10, 64)
	return err == nil && id > 0 && strconv.FormatUint(id, 10) == parts[1]
}
func ValidFence(req *pb.FenceRequest) bool {
	if ValidBootstrapFence(req) {
		return true
	}
	return req != nil && fenceID.MatchString(req.RevocationId) && req.Generation == 1 && len(req.ClosedScopes) == 1 && req.ClosedScopes[0] != nil && req.ClosedScopes[0].Epoch > 0 && ValidDocumentScope(req.ClosedScopes[0].ScopeKey) && bytes.Equal(req.TargetDigest, SourceClosureDigest(req.ClosedScopes[0].ScopeKey, req.ClosedScopes[0].Epoch))
}

func SourceClosureDigest(scope string, epoch uint64) []byte {
	value := sha256.Sum256([]byte("AUTHORIZATION_CHANGE\n" + scope + "\n" + strconv.FormatUint(epoch, 10)))
	return value[:]
}

func LogoutTargetDigest(hash string) []byte {
	value := sha256.Sum256([]byte("SESSION_LOGOUT\n" + hash))
	return value[:]
}
func ValidBootstrapFence(req *pb.FenceRequest) bool {
	if req == nil || !fenceID.MatchString(req.RevocationId) || req.Generation != 1 || len(req.ClosedScopes) != 1 || req.ClosedScopes[0] == nil || req.ClosedScopes[0].Epoch == 0 {
		return false
	}
	s := req.ClosedScopes[0]
	if sessionScope.MatchString(s.ScopeKey) {
		return bytes.Equal(req.TargetDigest, LogoutTargetDigest(s.ScopeKey[len("session:"):]))
	}
	if ValidAccountScope(s.ScopeKey) {
		return bytes.Equal(req.TargetDigest, SourceClosureDigest(s.ScopeKey, s.Epoch))
	}
	return false
}
func BootstrapDigest(req *pb.FenceRequest, role string) []byte {
	s := req.ClosedScopes[0]
	value := sha256.Sum256([]byte("bootstrap-fence-v1\n" + role + "\n" + req.RevocationId + "\n" + s.ScopeKey + "\n" + strconv.FormatUint(s.Epoch, 10) + "\n" + strconv.FormatUint(req.Generation, 10) + "\n" + hex.EncodeToString(req.TargetDigest)))
	return value[:]
}
func BootstrapAck(req *pb.FenceRequest, role string) *pb.FenceResult {
	return &pb.FenceResult{State: pb.FenceResult_ACKNOWLEDGED, RevocationId: req.RevocationId, ClosedScopes: req.ClosedScopes, Generation: req.Generation, ResultDigest: BootstrapDigest(req, role)}
}
func MatchesBootstrapAck(req *pb.FenceRequest, reply *pb.FenceResult, role string) bool {
	return ValidFence(req) && reply != nil && reply.State == pb.FenceResult_ACKNOWLEDGED && reply.RevocationId == req.RevocationId && reply.Generation == req.Generation && len(reply.ClosedScopes) == 1 && reply.ClosedScopes[0] != nil && reply.ClosedScopes[0].ScopeKey == req.ClosedScopes[0].ScopeKey && reply.ClosedScopes[0].Epoch == req.ClosedScopes[0].Epoch && bytes.Equal(reply.ResultDigest, BootstrapDigest(req, role)) && len(reply.MissingInstances) == 0
}

type bootstrapBinding struct {
	Digest     string `json:"bootstrap_target_digest"`
	Generation string `json:"generation"`
}

// PrepareBootstrapFence只支持尚无业务输出的底座。后续引入admission/output时必须换真正排空器。
func PrepareBootstrapFence(ctx context.Context, store *Store, req *pb.FenceRequest) (bool, error) {
	if !ValidBootstrapFence(req) {
		return false, status.Error(codes.InvalidArgument, "BOOTSTRAP_FENCE_INVALID")
	}
	tx, err := store.DB.BeginTx(ctx, nil)
	if err != nil {
		return false, status.Error(codes.Unavailable, "FENCE_UNAVAILABLE")
	}
	defer tx.Rollback()
	scope := req.ClosedScopes[0]
	var previousScope, state string
	var epoch uint64
	var raw []byte
	err = tx.QueryRowContext(ctx, "SELECT scope_key,epoch,state,target_permission FROM authorization_change WHERE revocation_id=? FOR UPDATE", req.RevocationId).Scan(&previousScope, &epoch, &state, &raw)
	binding := bootstrapBinding{Digest: hex.EncodeToString(req.TargetDigest), Generation: strconv.FormatUint(req.Generation, 10)}
	if err == nil {
		var prior bootstrapBinding
		if json.Unmarshal(raw, &prior) != nil || prior != binding || previousScope != scope.ScopeKey || epoch != scope.Epoch {
			return false, status.Error(codes.AlreadyExists, "FENCE_ID_CONFLICT")
		}
		if state == "SUCCEEDED" {
			return true, nil
		}
	} else if err != sql.ErrNoRows {
		return false, status.Error(codes.Unavailable, "FENCE_UNAVAILABLE")
	}
	// 没有正文/下载handler也没有输出登记，才能证明无需排空。非空记录绝不空ACK。
	for _, table := range []string{"document_admission", "delivery_session", "output_instance"} {
		var count int
		if tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&count) != nil {
			return false, status.Error(codes.Unavailable, "FENCE_UNAVAILABLE")
		}
		if count != 0 {
			return false, status.Error(codes.FailedPrecondition, "FULL_OUTPUT_DRAIN_REQUIRED")
		}
	}
	if err == sql.ErrNoRows {
		encoded, _ := json.Marshal(binding)
		_, err = tx.ExecContext(ctx, "INSERT INTO authorization_change(revocation_id,scope_key,epoch,target_permission,state,participant_plan) VALUES(?,?,?,?,'FENCING','[\"collaboration\"]')", req.RevocationId, scope.ScopeKey, scope.Epoch, string(encoded))
		if err != nil {
			return false, status.Error(codes.Unavailable, "FENCE_UNAVAILABLE")
		}
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO access_gate(scope_key,max_epoch,state,last_revocation_id) VALUES(?,?,'BLOCKED',?) ON DUPLICATE KEY UPDATE max_epoch=GREATEST(max_epoch,VALUES(max_epoch)),state=IF(VALUES(max_epoch)>=max_epoch,'BLOCKED',state),last_revocation_id=IF(VALUES(max_epoch)>=max_epoch,VALUES(last_revocation_id),last_revocation_id),revision=revision+1", scope.ScopeKey, scope.Epoch, req.RevocationId)
	if err != nil || tx.Commit() != nil {
		return false, status.Error(codes.Unavailable, "FENCE_COMMIT_UNKNOWN")
	}
	return false, nil
}
func FinishBootstrapFence(ctx context.Context, store *Store, req *pb.FenceRequest, node *pb.FenceResult) error {
	if !MatchesBootstrapAck(req, node, "collaboration") {
		return status.Error(codes.Unavailable, "NODE_FENCE_NOT_CONFIRMED")
	}
	tx, err := store.DB.BeginTx(ctx, nil)
	if err != nil {
		return status.Error(codes.Unavailable, "FENCE_UNAVAILABLE")
	}
	defer tx.Rollback()
	scope := req.ClosedScopes[0]
	_, err = tx.ExecContext(ctx, "INSERT INTO authorization_ack(revocation_id,participant,scope_key,epoch,phase,result_digest,completed_at) VALUES(?,'collaboration',?,?,'OUTPUT_STOPPED',?,UTC_TIMESTAMP(6)) ON DUPLICATE KEY UPDATE result_digest=VALUES(result_digest)", req.RevocationId, scope.ScopeKey, scope.Epoch, node.ResultDigest)
	if err != nil {
		return status.Error(codes.Unavailable, "FENCE_UNAVAILABLE")
	}
	_, err = tx.ExecContext(ctx, "UPDATE authorization_change SET state='SUCCEEDED' WHERE revocation_id=?", req.RevocationId)
	if err != nil || tx.Commit() != nil {
		return status.Error(codes.Unavailable, "FENCE_COMMIT_UNKNOWN")
	}
	return nil
}
