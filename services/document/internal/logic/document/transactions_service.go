// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	"bytes"
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	repository "collaboration/document/internal/repository/document"
	"collaboration/platform"
	"context"
	"database/sql"
	"encoding/json"
)

// mutation_transaction.go：保留原实现，按业务合并文件。

// 通用receipt只存固定结果；敏感投影在原事务内重新鉴权后构造，并登记它实际依赖的来源。
func (s *Transactions) Mutation(ctx context.Context, actor Actor, command OperationCommand, handler string, run MutationBody, present MutationPresentation, bind BindOutput) (any, error) {
	if !domain.ValidOperationID(command.ID) {
		return nil, domain.Violation(domain.InvalidRequest, "OPERATION_ID_INVALID")
	}
	if err := s.identity.Revalidate(ctx, actor.Proof); err != nil {
		return nil, err
	}
	edge, err := s.outputs.EdgeInstance(ctx, handler)
	if err != nil {
		return nil, err
	}
	fingerprint, err := domain.OperationFingerprint(command.Path, command.Input)
	if err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err := repository.LockOperationGate(ctx, tx, command.ID); err != nil {
		return nil, err
	}
	sources := actor.Sources
	if sources == nil {
		sources = actor.Proof.Sources
	}
	if err := LockSources(ctx, tx, sources); err != nil {
		return nil, err
	}
	receipt, err := repository.LockedOperation(ctx, tx, command.ID)
	var value map[string]any
	if err == nil {
		if receipt.ActorID != actor.ID || receipt.Kind != command.Kind || !bytes.Equal(receipt.Fingerprint, fingerprint[:]) {
			return nil, domain.Violation(domain.StateConflict, "OPERATION_ID_CONFLICT")
		}
		if json.Unmarshal(receipt.Result, &value) != nil {
			return nil, domain.Violation(domain.TemporarilyUnavailable, "OPERATION_PENDING")
		}
	} else if err == sql.ErrNoRows {
		if command.InitialResult != nil {
			if err := insertMutationReceipt(ctx, tx, actor, command, fingerprint, command.InitialResult); err != nil {
				return nil, err
			}
		}
		value, err = run(ctx, tx)
		if err != nil {
			return nil, err
		}
		if command.InitialResult != nil {
			result, err := platform.EncodeOperationReceipt(command.ID, value)
			if err != nil {
				return nil, err
			}
			if err = repository.UpdateOperationResult(ctx, tx, command.ID, result.State, result.JSON); err != nil {
				return nil, err
			}
		} else if err := insertMutationReceipt(ctx, tx, actor, command, fingerprint, value); err != nil {
			return nil, err
		}
	} else {
		return nil, err
	}
	var response any = value
	responseSources := actor.Proof.Sources
	if present != nil {
		response, responseSources, err = present(ctx, tx, value)
		if err != nil {
			return nil, err
		}
	}
	registration, err := s.outputs.Register(ctx, tx, responseSources, 0, "", handler, edge)
	if err != nil {
		return nil, err
	}
	output := bind(registration, responseSources)
	if err := tx.Commit(); err != nil {
		output.Abort()
		return nil, err
	}
	return response, nil
}

func insertMutationReceipt(ctx context.Context, tx *sql.Tx, actor Actor, command OperationCommand, fingerprint [32]byte, value map[string]any) error {
	result, err := platform.EncodeOperationReceipt(command.ID, value)
	if err != nil {
		return err
	}
	target, err := json.Marshal(map[string]string{"path": command.Path})
	if err != nil {
		return err
	}
	record := domain.OperationRecord{ID: command.ID, ActorID: actor.ID, Kind: command.Kind, TargetJSON: target, Fingerprint: fingerprint[:], State: result.State, IdentityAdmissionRef: actor.Proof.AdmissionId, ResultJSON: result.JSON}
	return repository.InsertOperation(ctx, tx, record)
}
func OperationResult(id, state string, entity uint64) map[string]any {
	value := map[string]any{"operation_id": id, "state": state}
	if entity != 0 {
		value["result"] = map[string]string{"entity_id": domain.TextID(entity)}
	}
	if state == "PREPARING" {
		value["poll_url"] = "/v1/authorization-changes/" + id
	}
	return value
}

// operation_replay.go：保留原实现，按业务合并文件。

type ReplayPolicy uint8

const (
	ValidateOperationFirst ReplayPolicy = iota
	LookupReceiptFirst
)

// TryReplay保留两类旧入口的验证顺序；存在原号时只用当前账号证明读取固定回执。
// 清除业务来源不会绕过新动作资格：不存在原号时返回未处理，调用者仍需执行完整新用例。
func (s *Transactions) TryReplay(ctx context.Context, actor Actor, command OperationCommand, handler string, policy ReplayPolicy, present MutationPresentation, bind BindOutput) (bool, any, error) {
	if policy == ValidateOperationFirst && !domain.ValidOperationID(command.ID) {
		return true, nil, domain.Violation(domain.InvalidRequest, "OPERATION_ID_INVALID")
	}
	exists, err := s.operations.Exists(ctx, command.ID)
	if err != nil {
		return true, nil, err
	}
	if !exists {
		return false, nil, nil
	}
	actor.Sources = nil
	result, err := s.Mutation(ctx, actor, command, handler, func(context.Context, *sql.Tx) (map[string]any, error) {
		return nil, domain.Violation(domain.TemporarilyUnavailable, "ORIGINAL_OPERATION_UNAVAILABLE")
	}, present, bind)
	return true, result, err
}

// transactions.go：保留原实现，按业务合并文件。

type ReadBody func(context.Context, *sql.Tx, string) (any, error)
type MutationBody func(context.Context, *sql.Tx) (map[string]any, error)
type MutationPresentation func(context.Context, *sql.Tx, map[string]any) (any, []*pb.SourceEpoch, error)
type OutputAborter interface{ Abort() }

// BindOutput把固定租约绑定到真实handler，必须在COMMIT前完成；业务层不持有ResponseWriter。
type BindOutput func(*ResponseRegistration, []*pb.SourceEpoch) OutputAborter

type OperationCommand struct {
	ID, Kind, Path string
	Input          any
	// 有外键依赖原号的流程先在同一事务登记准备态，再执行业务写入；失败仍整体回滚。
	InitialResult map[string]any
}
type Transactions struct {
	db         *sql.DB
	operations *repository.Operations
	identity   *Identity
	outputs    *Outputs
}

func NewTransactions(db *sql.DB, operations *repository.Operations, identity *Identity, outputs *Outputs) *Transactions {
	return &Transactions{db: db, operations: operations, identity: identity, outputs: outputs}
}
