// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package account

import (
	"bytes"
	domain "collaboration/account/internal/domain/account"
	repository "collaboration/account/internal/repository/account"
	dto "collaboration/contracts/httpdto"
	"collaboration/platform"
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"time"
)

// account_close_cancel.go：保留原实现，按业务合并文件。

type AccountCloseCancelPeer interface {
	Cancel(context.Context, platform.AccountClosureBinding) error
}

type AccountCloseCancellations struct {
	db         *sql.DB
	repository *repository.AccountCloseCancellations
	peer       AccountCloseCancelPeer
}

func NewAccountCloseCancellations(db *sql.DB, peer AccountCloseCancelPeer) *AccountCloseCancellations {
	return &AccountCloseCancellations{db: db, repository: repository.NewAccountCloseCancellations(db), peer: peer}
}

func (s *AccountCloseCancellations) Ready(ctx context.Context) error { return s.repository.Ready(ctx) }

func (s *AccountCloseCancellations) RecoverOne(parent context.Context, instance string) {
	ctx, cancel := context.WithTimeout(parent, 4*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer tx.Rollback()
	task, err := s.repository.Claim(ctx, tx, instance)
	if err != nil || tx.Commit() != nil {
		return
	}
	binding, err := s.repository.Binding(ctx, s.db, task)
	if err != nil {
		s.repository.Retry(ctx, task)
		return
	}
	// 只有Account同事务保存的REJECTED绑定能调用取消；成功响应来自Doc再次
	// 查询权威证明并完成本地提交。未知网络回复重试原号，不自造释放确认。
	rpcCtx, done := context.WithTimeout(ctx, 2*time.Second)
	err = s.peer.Cancel(rpcCtx, binding)
	done()
	if err != nil {
		s.repository.Retry(ctx, task)
		return
	}
	finished, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		s.repository.Retry(ctx, task)
		return
	}
	defer finished.Rollback()
	if err = s.repository.Finish(ctx, finished, task, binding); err != nil {
		_ = finished.Rollback()
		s.repository.Retry(ctx, task)
		return
	}
	if finished.Commit() != nil {
		s.repository.Retry(ctx, task)
	}
}

// account_close_confirmation.go：保留原实现，按业务合并文件。

func (s *AccountClosures) Confirmed(ctx context.Context, binding platform.AccountClosureBinding) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	record, capture, _, err := s.lockBinding(ctx, tx, binding)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if capture.Binding == nil || (record.State != "FROZEN" && record.State != "PLANNED" && record.State != "CLEANING") {
		return false, nil
	}
	user, err := s.repository.LockUser(ctx, tx, binding.User)
	if err != nil {
		return false, err
	}
	if binding.IdentityEpoch == ^uint64(0) || user.State != "REVOKING" || user.IdentityEpoch != binding.IdentityEpoch+1 || user.Epoch != strconv.FormatUint(capture.CredentialEpoch, 10) {
		return false, accountCloseConflict("ACCOUNT_CLOSE_FROZEN_IDENTITY_CHANGED")
	}
	return s.repository.ConfirmedScopes(ctx, tx, binding, capture)
}

// account_close_finalize.go：保留原实现，按业务合并文件。

// 仅接受原计划最后一页的真实Doc SUCCEEDED；没有页面/清理资格、网络未知或
// 本人冻结/任一ACK不完整都继续等待。最后个人信息、组织关系、回执和原任务同事务。
func (s *AccountCloseRecoveries) finalize(ctx context.Context, task domain.AccountCloseFenceTask, expected domain.AccountCloseRecord, capture domain.AccountCloseCapture, plan dto.AccountClosePlan, proof platform.AccountClosurePage) error {
	if capture.Binding == nil || capture.Retention == nil {
		return errors.New("ACCOUNT_CLOSE_FINAL_PROOF_REQUIRED")
	}
	request := platform.AccountClosurePageRequest{AccountClosureBinding: *capture.Binding, PlanDigest: capture.Retention.PlanDigest, AfterDocument: capture.Retention.AfterDocument}
	if !proof.Matches(request) || proof.State != "SUCCEEDED" || !proof.Done || len(proof.Documents) != 0 {
		return errors.New("ACCOUNT_CLOSE_FINAL_PROOF_INVALID")
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	current, original, _, err := s.closures.lockBinding(ctx, tx, *capture.Binding)
	if err != nil {
		return err
	}
	if current.State != "CLEANING" || current.State != expected.State || !bytes.Equal(current.Capture, expected.Capture) {
		return errors.New("ACCOUNT_CLOSE_FINAL_STATE_CHANGED")
	}
	user, err := s.closures.repository.LockUser(ctx, tx, capture.Binding.User)
	if err != nil {
		return err
	}
	if capture.IdentityEpoch == ^uint64(0) || user.State != "REVOKING" || user.IdentityEpoch != capture.IdentityEpoch+1 || user.Epoch != strconv.FormatUint(capture.CredentialEpoch, 10) {
		return errors.New("ACCOUNT_CLOSE_FINAL_IDENTITY_CHANGED")
	}
	confirmed, err := s.closures.repository.ConfirmedScopes(ctx, tx, *capture.Binding, original)
	if err != nil {
		return err
	}
	if !confirmed {
		return accountCloseConflict("ACCOUNT_CLOSE_FINAL_DRAIN_PENDING")
	}
	if err = repository.FinalizeAccountCloseData(ctx, tx, *capture.Binding, capture); err != nil {
		return err
	}
	if err = repository.ValidateAccountCloseRecovery(ctx, tx, task); err != nil {
		return err
	}
	plan.State = "SUCCEEDED"
	capture.Retention.DocCompleted = true
	rawCapture, err := json.Marshal(capture)
	if err != nil {
		return err
	}
	rawPlan, err := json.Marshal(plan)
	if err != nil {
		return err
	}
	if err = repository.SaveAccountCloseRetention(ctx, tx, task.Operation, current.State, plan.State, rawCapture, rawPlan); err != nil {
		return err
	}
	if err = repository.CompleteAccountCloseReceipt(ctx, tx, *capture.Binding, proof.PlanDigest, rawPlan); err != nil {
		return err
	}
	if err = repository.FinishAccountCloseRecovery(ctx, tx, task, true); err != nil {
		return err
	}
	return tx.Commit()
}

// account_close_progress.go：保留原实现，按业务合并文件。

type AccountCloseSession struct {
	Session domain.Session
	Token   string
}

func accountCloseNotFound() error {
	return domain.RuleViolation{Kind: domain.EntityNotFound, Code: "ACCOUNT_CLOSE_NOT_FOUND"}
}

// 冻结后只能用原会话秘密恢复这一条最小进度，不能把恢复Cookie提升成登录权限。
// 新操作只能用当前有效主会话；恢复Cookie不能发起另一次注销。
func (s *AccountClosures) Resolve(ctx context.Context, operation, primary, recovery string) (AccountCloseSession, error) {
	if !domain.ValidOperationID(operation) {
		return AccountCloseSession{}, domain.RuleViolation{Kind: domain.InvalidRequest, Code: "OPERATION_ID_INVALID"}
	}
	receipt, err := s.repository.ProgressBinding(ctx, operation)
	if errors.Is(err, sql.ErrNoRows) {
		authenticated, err := s.authentication.Authenticate(ctx, primary)
		if errors.Is(err, domain.ErrSessionInvalid) {
			err = domain.RuleViolation{Kind: domain.SessionRequired, Code: "SESSION_INVALID"}
		}
		return AccountCloseSession{Session: authenticated.Session, Token: primary}, err
	}
	if err != nil {
		return AccountCloseSession{}, err
	}
	for _, token := range []string{primary, recovery} {
		session, err := s.authentication.Session(ctx, token)
		if err == nil && receipt.Kind == "ACCOUNT_CLOSE" && strconv.FormatUint(receipt.User, 10) == session.UserID && receipt.SessionHash == hex.EncodeToString(session.ID) {
			return AccountCloseSession{Session: session, Token: token}, nil
		}
	}
	return AccountCloseSession{}, accountCloseNotFound()
}

func (s *AccountClosures) Progress(ctx context.Context, operation, primary, recovery string) (dto.AccountClosePlan, error) {
	var plan dto.AccountClosePlan
	selection, err := s.Resolve(ctx, operation, primary, recovery)
	if err != nil {
		// 匿名、其他会话和不存在的操作都不暴露注销是否存在。
		var rule domain.RuleViolation
		if errors.As(err, &rule) || errors.Is(err, sql.ErrNoRows) {
			return plan, accountCloseNotFound()
		}
		return plan, err
	}
	user, err := strconv.ParseUint(selection.Session.UserID, 10, 64)
	if err != nil {
		return plan, accountCloseNotFound()
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return plan, err
	}
	defer tx.Rollback()
	if _, err = s.repository.LockUser(ctx, tx, user); err != nil {
		return plan, err
	}
	receipt, err := s.repository.Receipt(ctx, tx, operation)
	if errors.Is(err, sql.ErrNoRows) {
		return plan, accountCloseNotFound()
	}
	if err != nil {
		return plan, err
	}
	if receipt.User != user || receipt.Kind != "ACCOUNT_CLOSE" || receipt.SessionHash != hex.EncodeToString(selection.Session.ID) {
		return plan, accountCloseNotFound()
	}
	record, err := s.repository.Record(ctx, tx, operation)
	if err != nil {
		return plan, err
	}
	capture, err := domain.DecodeAccountCloseCapture(record.Capture)
	if err != nil {
		return plan, err
	}
	if capture.SessionHash != receipt.SessionHash || strconv.FormatUint(capture.CredentialEpoch, 10) != selection.Session.CredentialEpoch {
		return plan, accountCloseNotFound()
	}
	plan, err = accountCloseSavedPlan(operation, record, receipt)
	if err != nil {
		return plan, err
	}
	return plan, tx.Commit()
}

// account_close_recovery.go：保留原实现，按业务合并文件。

type AccountCloseRecoveryPeer interface {
	AccountClosurePreparer
	Plan(context.Context, platform.AccountClosureBinding) (platform.AccountClosureRetention, error)
	Relations(context.Context, platform.AccountClosureBinding) (platform.AccountClosureRetention, error)
	Page(context.Context, platform.AccountClosurePageRequest) (platform.AccountClosurePage, error)
	CleanDocument(context.Context, platform.AccountClosureBinding, string) error
	Complete(context.Context, platform.AccountClosureBinding) error
}

type AccountCloseRecoveries struct {
	db       *sql.DB
	closures *AccountClosures
	peer     AccountCloseRecoveryPeer
}

func NewAccountCloseRecoveries(db *sql.DB, closures *AccountClosures, peer AccountCloseRecoveryPeer) *AccountCloseRecoveries {
	return &AccountCloseRecoveries{db: db, closures: closures, peer: peer}
}

func accountCloseRecoveryUser(task domain.AccountCloseFenceTask) (uint64, error) {
	var payload repository.AccountCloseRecoveryPayload
	decoder := json.NewDecoder(bytes.NewReader(task.Payload))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&payload) != nil || decoder.Decode(new(any)) != io.EOF || !domain.ValidOperationID(task.Operation) || payload.Operation != task.Operation || task.Revocation != task.Operation || payload.User == 0 {
		return 0, errors.New("ACCOUNT_CLOSE_RECOVERY_BINDING_INVALID")
	}
	return payload.User, nil
}

// 一次领取只推进一个阶段或一个文档，避免大账号占满同步HTTP/任务预算。
// 跨服务调用不持SQL锁；每步本地进度和原任务重新排队同事务，回复丢失原号续接。
func (s *AccountCloseRecoveries) RecoverOne(parent context.Context, instance string) {
	ctx, cancel := context.WithTimeout(parent, 4*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return
	}
	task, err := repository.ClaimAccountCloseRecovery(ctx, tx, instance)
	if err != nil {
		tx.Rollback()
		return
	}
	if tx.Commit() != nil {
		return
	}
	if err = s.advance(ctx, task); err != nil {
		// 本地提交未知不能改写已成功任务；条件重试只作用原领取代次。
		retry, done := context.WithTimeout(parent, time.Second)
		defer done()
		_ = repository.RetryAccountCloseRecovery(retry, s.db, task, "ACCOUNT_CLOSE_STEP_UNCONFIRMED")
	}
}

func (s *AccountCloseRecoveries) read(ctx context.Context, task domain.AccountCloseFenceTask) (domain.AccountCloseRecord, domain.AccountCloseCapture, dto.AccountClosePlan, error) {
	var record domain.AccountCloseRecord
	var capture domain.AccountCloseCapture
	var plan dto.AccountClosePlan
	user, err := accountCloseRecoveryUser(task)
	if err != nil {
		return record, capture, plan, err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return record, capture, plan, err
	}
	defer tx.Rollback()
	if _, err = s.closures.repository.LockUser(ctx, tx, user); err != nil {
		return record, capture, plan, err
	}
	receipt, err := s.closures.repository.Receipt(ctx, tx, task.Operation)
	if err != nil {
		return record, capture, plan, err
	}
	record, err = s.closures.repository.Record(ctx, tx, task.Operation)
	if err != nil {
		return record, capture, plan, err
	}
	capture, err = domain.DecodeAccountCloseCapture(record.Capture)
	if err != nil {
		return record, capture, plan, err
	}
	if record.User != user || receipt.User != user || receipt.SessionHash != capture.SessionHash {
		return record, capture, plan, errors.New("ACCOUNT_CLOSE_RECOVERY_BINDING_INVALID")
	}
	plan, err = accountCloseSavedPlan(task.Operation, record, receipt)
	if err != nil {
		return record, capture, plan, err
	}
	if err = repository.ValidateAccountCloseRecovery(ctx, tx, task); err != nil {
		return record, capture, plan, err
	}
	return record, capture, plan, tx.Commit()
}

func (s *AccountCloseRecoveries) advance(ctx context.Context, task domain.AccountCloseFenceTask) error {
	record, capture, plan, err := s.read(ctx, task)
	if err != nil {
		return err
	}
	if record.State == "SUCCEEDED" || record.State == "REJECTED" {
		return s.finishTerminal(ctx, task, record.User)
	}
	if record.State == "PREPARING" {
		result, err := s.closures.PrepareClose(ctx, task.Operation, s.peer)
		if err != nil {
			return err
		}
		if result.State == "PREPARING" && len(result.RequiredSuccessions) > 0 {
			return accountCloseConflict("ACCOUNT_CLOSE_HANDOFF_REQUIRED")
		}
		return s.requeueCurrent(ctx, task, record.User)
	}
	if capture.Binding == nil {
		return errors.New("ACCOUNT_CLOSE_RECOVERY_BINDING_INVALID")
	}
	binding := *capture.Binding
	switch record.State {
	case "FROZEN":
		confirmed, err := s.closures.Confirmed(ctx, binding)
		if err != nil {
			return err
		}
		if !confirmed {
			return accountCloseConflict("ACCOUNT_CLOSE_DRAIN_PENDING")
		}
		if _, err = s.peer.Plan(ctx, binding); err != nil {
			return err
		}
		request := platform.AccountClosurePageRequest{AccountClosureBinding: binding}
		page, err := s.peer.Page(ctx, request)
		if err != nil {
			return err
		}
		if !page.Matches(request) {
			return errors.New("ACCOUNT_CLOSE_PAGE_INVALID")
		}
		capture.Retention = &domain.AccountCloseRetention{PlanDigest: page.PlanDigest}
		plan.State = "PLANNED"
	case "PLANNED":
		if capture.Retention == nil {
			return errors.New("ACCOUNT_CLOSE_RETENTION_REQUIRED")
		}
		if _, err = s.peer.Relations(ctx, binding); err != nil {
			return err
		}
		plan.State = "CLEANING"
	case "CLEANING":
		if capture.Retention == nil {
			return errors.New("ACCOUNT_CLOSE_RETENTION_REQUIRED")
		}
		request := platform.AccountClosurePageRequest{AccountClosureBinding: binding, AfterDocument: capture.Retention.AfterDocument, PlanDigest: capture.Retention.PlanDigest}
		page, err := s.peer.Page(ctx, request)
		if err != nil {
			return err
		}
		if !page.Matches(request) {
			return errors.New("ACCOUNT_CLOSE_PAGE_INVALID")
		}
		if len(page.Documents) > 0 {
			if capture.Retention.DocCompleted {
				return errors.New("ACCOUNT_CLOSE_COMPLETION_CURSOR_CHANGED")
			}
			document := page.Documents[0]
			if err = s.peer.CleanDocument(ctx, binding, document.Document); err != nil {
				return err
			}
			capture.Retention.AfterDocument = document.Document
			if document.Release != "0" {
				plan.RetainedReleaseIds = append(plan.RetainedReleaseIds, document.Release)
			}
		} else {
			if !page.Done {
				return errors.New("ACCOUNT_CLOSE_PAGE_INCOMPLETE")
			}
			if page.State == "SUCCEEDED" {
				return s.finalize(ctx, task, record, capture, plan, page)
			}
			if capture.Retention.DocCompleted {
				return errors.New("ACCOUNT_CLOSE_DOC_COMPLETION_CHANGED")
			}
			if err = s.peer.Complete(ctx, binding); err != nil {
				return err
			}
			capture.Retention.DocCompleted = true
		}
	default:
		return errors.New("ACCOUNT_CLOSE_STATE_INVALID")
	}
	return s.persist(ctx, task, record, capture, plan)
}

func (s *AccountCloseRecoveries) persist(ctx context.Context, task domain.AccountCloseFenceTask, expected domain.AccountCloseRecord, capture domain.AccountCloseCapture, plan dto.AccountClosePlan) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	current, _, _, err := s.closures.lockBinding(ctx, tx, *capture.Binding)
	if err != nil {
		return err
	}
	if current.State != expected.State || !bytes.Equal(current.Capture, expected.Capture) {
		return errors.New("ACCOUNT_CLOSE_STEP_CHANGED")
	}
	if err = repository.ValidateAccountCloseRecovery(ctx, tx, task); err != nil {
		return err
	}
	rawCapture, err := json.Marshal(capture)
	if err != nil {
		return err
	}
	rawPlan, err := json.Marshal(plan)
	if err != nil {
		return err
	}
	if err = repository.SaveAccountCloseRetention(ctx, tx, task.Operation, current.State, plan.State, rawCapture, rawPlan); err != nil {
		return err
	}
	if err = repository.FinishAccountCloseRecovery(ctx, tx, task, false); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *AccountCloseRecoveries) requeueCurrent(ctx context.Context, task domain.AccountCloseFenceTask, user uint64) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = s.closures.repository.LockUser(ctx, tx, user); err != nil {
		return err
	}
	if _, err = s.closures.repository.Receipt(ctx, tx, task.Operation); err != nil {
		return err
	}
	if _, err = s.closures.repository.Record(ctx, tx, task.Operation); err != nil {
		return err
	}
	if err = repository.ValidateAccountCloseRecovery(ctx, tx, task); err != nil {
		return err
	}
	if err = repository.FinishAccountCloseRecovery(ctx, tx, task, false); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *AccountCloseRecoveries) finishTerminal(ctx context.Context, task domain.AccountCloseFenceTask, user uint64) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = s.closures.repository.LockUser(ctx, tx, user); err != nil {
		return err
	}
	if _, err = s.closures.repository.Receipt(ctx, tx, task.Operation); err != nil {
		return err
	}
	record, err := s.closures.repository.Record(ctx, tx, task.Operation)
	if err != nil {
		return err
	}
	if record.State != "SUCCEEDED" && record.State != "REJECTED" {
		return errors.New("ACCOUNT_CLOSE_NOT_TERMINAL")
	}
	if err = repository.ValidateAccountCloseRecovery(ctx, tx, task); err != nil {
		return err
	}
	if err = repository.FinishAccountCloseRecovery(ctx, tx, task, true); err != nil {
		return err
	}
	return tx.Commit()
}

// account_close_recovery_ready.go：保留原实现，按业务合并文件。

// 待恢复任务必须仍绑定原账号、原回执及固定计划。逐页核验持久事实，
// READY不能用忽略孤立任务的INNER JOIN或把损坏任务改成功来掩盖恢复缺口。
func (s *AccountCloseRecoveries) Ready(ctx context.Context) error {
	after := uint64(0)
	for {
		page, err := repository.AccountCloseRecoveryPage(ctx, s.db, after)
		if err != nil {
			return err
		}
		for _, row := range page {
			if err = validateRecoveryPlan(row.Task, row.Record, row.Receipt); err != nil {
				return err
			}
			after = row.Task.ID
		}
		if len(page) < 100 {
			return nil
		}
	}
}

func validateRecoveryPlan(task domain.AccountCloseFenceTask, record domain.AccountCloseRecord, receipt domain.AccountCloseReceipt) error {
	user, err := accountCloseRecoveryUser(task)
	if err != nil {
		return err
	}
	capture, err := domain.DecodeAccountCloseCapture(record.Capture)
	if err != nil {
		return err
	}
	if record.User != user || receipt.User != user || receipt.SessionHash != capture.SessionHash || !json.Valid(receipt.Result) {
		return errors.New("ACCOUNT_CLOSE_RECOVERY_BINDING_INVALID")
	}
	if _, err = accountCloseSavedPlan(task.Operation, record, receipt); err != nil {
		return err
	}
	switch record.State {
	case "PREPARING", "REJECTED":
	case "FROZEN":
		if capture.Binding == nil {
			return errors.New("ACCOUNT_CLOSE_RECOVERY_BINDING_INVALID")
		}
	case "PLANNED", "CLEANING", "SUCCEEDED":
		if capture.Binding == nil || capture.Retention == nil {
			return errors.New("ACCOUNT_CLOSE_RETENTION_REQUIRED")
		}
	default:
		return errors.New("ACCOUNT_CLOSE_STATE_INVALID")
	}
	if capture.Binding != nil && (capture.Binding.User != user || capture.Binding.Operation != task.Operation) {
		return errors.New("ACCOUNT_CLOSE_RECOVERY_BINDING_INVALID")
	}
	return nil
}
