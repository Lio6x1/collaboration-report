# services/account/internal/logic/account/operation_receipts_service.go

阶段02：[功能讲义](../02-account.md) · [分步编写](../write/064-operation-receipts-service.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：编排用例、权限、事务及未知结果恢复；跨服务调用不能替代持久事实。

主要声明：`OperationResults`、`NewOperationResults`、`minimalAccountResult`、`operationResultNotFound`、`operationResultUnavailable`、`sameOperationResult`、`Get`、`normalizeOperationResultError`、`Progress`、`OperationResult`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package account

import (
	"bytes"
	domain "collaboration/account/internal/domain/account"
	repository "collaboration/account/internal/repository/account"
	pb "collaboration/contracts/internalv1"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"
)

// account_operation_result.go：保留原实现，按业务合并文件。

type OperationResults struct {
	db       *sql.DB
	logout   *Logout
	closures *AccountClosures
}

func NewOperationResults(db *sql.DB, logout *Logout, closures *AccountClosures) *OperationResults {
	return &OperationResults{db: db, logout: logout, closures: closures}
}

type minimalAccountResult struct {
	Operation string `json:"operation_id"`
	State     string `json:"state"`
}

func operationResultNotFound() error {
	return domain.RuleViolation{Kind: domain.EntityNotFound, Code: "OPERATION_NOT_FOUND"}
}

func operationResultUnavailable() error {
	return domain.RuleViolation{Kind: domain.TemporarilyUnavailable, Code: "ACCOUNT_OPERATION_RESULT_UNAVAILABLE"}
}

func sameOperationResult(left, right repository.OperationResultSnapshot) bool {
	return left.Actor == right.Actor && left.Kind == right.Kind && left.State == right.State &&
		left.SessionHash == right.SessionHash && left.Generation == right.Generation && left.Terminal == right.Terminal &&
		bytes.Equal(left.Fingerprint, right.Fingerprint) && bytes.Equal(left.Result, right.Result)
}

// Get仅查询已存在的原操作。recovery_receipt沿HTTP恢复Cookie的真实语义处理：
// 它是原会话秘密，不是新发明的bearer；权限全部复用原Progress，不调用Resolve/Start。
func (s *OperationResults) Get(ctx context.Context, query *pb.AccountOperationQuery) (*pb.OperationResult, error) {
	if query == nil || !domain.ValidOperationID(query.OperationId) {
		return nil, domain.RuleViolation{Kind: domain.InvalidRequest, Code: "OPERATION_ID_INVALID"}
	}
	if s == nil || s.db == nil || s.logout == nil || s.closures == nil {
		return nil, operationResultUnavailable()
	}
	before, err := repository.ReadOperationResult(ctx, s.db, query.OperationId)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, operationResultNotFound()
	}
	if err != nil {
		return nil, err
	}
	primary, recovery := string(query.SessionToken), string(query.RecoveryReceipt)
	var authorized minimalAccountResult
	if before.Kind == "ACCOUNT_CLOSE" {
		plan, err := s.closures.Progress(ctx, query.OperationId, primary, recovery)
		if err != nil {
			return nil, normalizeOperationResultError(err)
		}
		authorized = minimalAccountResult{Operation: plan.OperationId, State: plan.State}
	} else {
		progress, err := s.logout.Progress(ctx, query.OperationId, primary, recovery)
		if err != nil {
			return nil, normalizeOperationResultError(err)
		}
		if progress.State != "" {
			authorized = minimalAccountResult{Operation: query.OperationId, State: progress.State}
		} else if json.Unmarshal(progress.Result, &authorized) != nil {
			return nil, operationResultUnavailable()
		}
	}
	// 原权限返回和本次RPC必须属于同一操作/会话绑定。读后比较整份持久快照，
	// 连同结果字节核验：即使关闭器没有递增generation也不能混用两阶段结果。
	after, err := repository.ReadOperationResult(ctx, s.db, query.OperationId)
	if err != nil || !sameOperationResult(before, after) {
		return nil, operationResultUnavailable()
	}
	if authorized.Operation != query.OperationId || after.Generation == 0 || len(after.Fingerprint) != 32 {
		return nil, operationResultUnavailable()
	}
	if len(after.Result) > 0 {
		var stored minimalAccountResult
		if json.Unmarshal(after.Result, &stored) != nil || stored != authorized {
			return nil, operationResultUnavailable()
		}
	} else if authorized.State != after.State || after.Kind == "ACCOUNT_CLOSE" {
		return nil, operationResultUnavailable()
	}
	result := &pb.OperationResult{OperationId: query.OperationId, Generation: after.Generation, State: pb.OperationResult_PROCESSING}
	terminal := true
	switch after.State {
	case "PREPARING":
		terminal = false
	case "SUCCEEDED":
		result.State = pb.OperationResult_SUCCEEDED
	case "REJECTED":
		result.State = pb.OperationResult_REJECTED
	case "CANCELLED":
		result.State = pb.OperationResult_CANCELLED
	default:
		return nil, operationResultUnavailable()
	}
	raw, err := json.Marshal(authorized)
	if err != nil {
		return nil, err
	}
	if !terminal {
		if after.Terminal {
			return nil, operationResultUnavailable()
		}
		result.CurrentStateJsonUtf8 = raw
		return result, nil
	}
	if !after.Terminal || len(after.Result) == 0 || authorized.State != after.State {
		return nil, operationResultUnavailable()
	}
	// generation来自原receipt；terminal_revision定义为持久终态的receipt代次，
	// 不采用后台租约代次或当前身份epoch。摘要只覆盖最小固定投影的JSON字节，
	// 不冒充完整result_json摘要，也不向已关闭会话交付实体/交接/保留清单。
	result.TerminalRevision = after.Generation
	result.FixedResultJsonUtf8 = raw
	digest := sha256.Sum256(raw)
	result.ResultDigest = digest[:]
	return result, nil
}

func normalizeOperationResultError(err error) error {
	var rule domain.RuleViolation
	if errors.Is(err, sql.ErrNoRows) || (errors.As(err, &rule) && rule.Kind == domain.EntityNotFound) {
		return operationResultNotFound()
	}
	return err
}

// operation_progress.go：保留原实现，按业务合并文件。

// Progress对已关闭的原会话只返回该操作的最小进度，不交付目标或其他业务投影。
func (s *Logout) Progress(ctx context.Context, id, primaryToken, recoveryToken string) (domain.OperationProgress, error) {
	receipt, err := s.receipts.Lookup(ctx, id)
	notFound := domain.RuleViolation{Kind: domain.EntityNotFound, Code: "OPERATION_NOT_FOUND"}
	if errors.Is(err, sql.ErrNoRows) {
		return domain.OperationProgress{}, notFound
	}
	if err != nil {
		return domain.OperationProgress{}, err
	}
	boundOperation := receipt.Kind == "SESSION_LOGOUT" || receipt.Kind == "CHANGE_PASSWORD" || receipt.Kind == "CHANGE_EMAIL"
	for _, token := range []string{primaryToken, recoveryToken} {
		session, err := s.authentication.Session(ctx, token)
		if err == nil && boundOperation && receipt.ActorID == session.UserID && receipt.SessionHash == hex.EncodeToString(session.ID) {
			return domain.OperationProgress{State: receipt.State}, nil
		}
	}
	if !boundOperation {
		session, err := s.authentication.Session(ctx, primaryToken)
		if err == nil && session.State == "ACTIVE" && session.Expires.After(time.Now()) && receipt.ActorID == session.UserID {
			credential, err := s.receipts.CurrentCredential(ctx, session.UserID)
			if err == nil && credential.State == "ACTIVE" && credential.Epoch == session.CredentialEpoch {
				raw, err := s.receipts.Result(ctx, id)
				if err == nil && len(raw) > 0 {
					return domain.OperationProgress{Result: raw}, nil
				}
			}
		}
	}
	return domain.OperationProgress{}, notFound
}

// operation_result.go：保留原实现，按业务合并文件。

func OperationResult(id, state, entity string) map[string]any {
	value := map[string]any{"operation_id": id, "state": state}
	if entity != "" {
		value["result"] = map[string]string{"entity_id": entity}
	}
	if state == "PREPARING" {
		value["poll_url"] = "/v1/account/operations/" + id
	}
	return value
}
````

UTF-8/LF全文SHA-256：`c746ac3fcf6b1616492e20180fd4386201ea02698ef330cf55bcf8fd038d156f`。[参考实现](../../../../services/account/internal/logic/account/operation_receipts_service.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
