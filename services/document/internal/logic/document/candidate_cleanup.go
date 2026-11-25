package document

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"google.golang.org/protobuf/proto"
)

type CandidateCleanup struct{ db *sql.DB }

func NewCandidateCleanup(db *sql.DB) *CandidateCleanup { return &CandidateCleanup{db: db} }

type candidateCleanupProof struct {
	Binding *pb.CleanupAuthorization `json:"binding"`
	Version uint64                   `json:"version"`
	Digest  []byte                   `json:"digest"`
}

// 引用格式明确绑定原操作；capture代次为原guard_generation，merge代次为不可复用的attempt_id。
// 它们均不是后台worker租约代次，清理重试不能随着重领任务改变候选身份。
func parseCleanupReference(request *pb.CleanupAuthorization) (string, uint64, error) {
	if request == nil || !domain.ValidOperationID(request.OperationId) || request.Generation == 0 || len(request.CandidateDigest) != sha256.Size {
		return "", 0, domain.Violation(domain.InvalidRequest, "CANDIDATE_BINDING_INVALID")
	}
	parts := strings.Split(request.CandidateRef, ":")
	if len(parts) == 2 && parts[0] == "capture" && parts[1] == request.OperationId {
		return "capture", 0, nil
	}
	if len(parts) == 3 && parts[0] == "merge" && parts[1] == request.OperationId {
		attempt, err := strconv.ParseUint(parts[2], 10, 64)
		if err == nil && attempt > 0 && strconv.FormatUint(attempt, 10) == parts[2] && attempt == request.Generation {
			return "merge", attempt, nil
		}
	}
	return "", 0, domain.Violation(domain.InvalidRequest, "CANDIDATE_REFERENCE_INVALID")
}

func cleanupPermitReply(request *pb.CleanupAuthorization, state pb.CleanupPermit_State, raw []byte) *pb.CleanupPermit {
	reply := &pb.CleanupPermit{OperationId: request.OperationId, Generation: request.Generation, State: state}
	if state == pb.CleanupPermit_CLEANABLE {
		digest := sha256.Sum256(raw)
		reply.PermitDigest = digest[:]
	}
	return reply
}

func (s *CandidateCleanup) Authorize(ctx context.Context, request *pb.CleanupAuthorization) (*pb.CleanupPermit, error) {
	kind, attempt, err := parseCleanupReference(request)
	if err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = repository.LockOperationGate(ctx, tx, request.OperationId); err != nil {
		return nil, err
	}
	// 已签出的许可保存原证明。后来正文按合法生命周期删除，不会破坏同号清理回执重放。
	raw, err := repository.CandidateCleanupPermit(ctx, tx, request.CandidateRef)
	if err == nil {
		var proof candidateCleanupProof
		if json.Unmarshal(raw, &proof) != nil || proof.Binding == nil || proof.Version == 0 || len(proof.Digest) != sha256.Size {
			return nil, errors.New("CANDIDATE_PERMIT_UNKNOWN")
		}
		if !proto.Equal(proof.Binding, request) || !bytes.Equal(proof.Digest, request.CandidateDigest) {
			return nil, domain.Violation(domain.StateConflict, "CANDIDATE_BINDING_CHANGED")
		}
		return cleanupPermitReply(request, pb.CleanupPermit_CLEANABLE, raw), nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	var generation, version uint64
	var state string
	var digest, actual []byte
	var copied bool
	if kind == "capture" {
		generation, state, version, digest, actual, copied, err = repository.SubmissionCleanupCopy(ctx, tx, request.OperationId)
	} else {
		generation = attempt
		state, version, digest, actual, copied, err = repository.MergeCleanupCopy(ctx, tx, request.OperationId, attempt)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return cleanupPermitReply(request, pb.CleanupPermit_UNKNOWN, nil), nil
	}
	if err != nil {
		return nil, err
	}
	if generation != request.Generation {
		return nil, domain.Violation(domain.StateConflict, "CANDIDATE_GENERATION_CHANGED")
	}
	expectedState := "SUCCEEDED"
	if kind == "merge" {
		expectedState = "COMMITTED"
	}
	if state != expectedState || version == 0 || !copied {
		return cleanupPermitReply(request, pb.CleanupPermit_RETAIN, nil), nil
	}
	if len(digest) != sha256.Size || !bytes.Equal(digest, actual) {
		return cleanupPermitReply(request, pb.CleanupPermit_UNKNOWN, nil), nil
	}
	if !bytes.Equal(digest, request.CandidateDigest) {
		return nil, domain.Violation(domain.StateConflict, "CANDIDATE_DIGEST_CHANGED")
	}
	complete, err := repository.CandidateCleanupTerminal(ctx, tx, request.OperationId)
	if err != nil {
		return nil, err
	}
	if !complete {
		return cleanupPermitReply(request, pb.CleanupPermit_RETAIN, nil), nil
	}
	raw, err = json.Marshal(candidateCleanupProof{Binding: proto.Clone(request).(*pb.CleanupAuthorization), Version: version, Digest: digest})
	if err != nil {
		return nil, err
	}
	if err = repository.SaveCandidateCleanupPermit(ctx, tx, request.CandidateRef, request.OperationId, raw); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, errors.New("CANDIDATE_PERMIT_COMMIT_UNKNOWN")
	}
	// MySQL JSON列会规范化空白/键序，首次回复也必须读取真正持久的字节。
	// 不能以写入前Marshal的摘要签许可，否则Node重查原号必然得到不同摘要。
	raw, err = repository.CandidateCleanupPermit(ctx, s.db, request.CandidateRef)
	if err != nil {
		return nil, errors.New("CANDIDATE_PERMIT_READ_UNKNOWN")
	}
	return cleanupPermitReply(request, pb.CleanupPermit_CLEANABLE, raw), nil
}
