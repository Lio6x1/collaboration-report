// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	"bytes"
	domain "collaboration/document/internal/domain/document"
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
)

// fence.go：保留原实现，按业务合并文件。

type Fences struct{ db *sql.DB }

func NewFences(db *sql.DB) *Fences { return &Fences{db: db} }
func BlockFenceScope(ctx context.Context, tx *sql.Tx, key string, epoch uint64, id string) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO access_gate(scope_key,max_epoch,state,last_revocation_id) VALUES(?,?,'BLOCKED',?) ON DUPLICATE KEY UPDATE max_epoch=GREATEST(max_epoch,VALUES(max_epoch)),state=IF(VALUES(max_epoch)>=max_epoch,'BLOCKED',state),last_revocation_id=IF(VALUES(max_epoch)>=max_epoch,VALUES(last_revocation_id),last_revocation_id),revision=revision+1", key, epoch, id)
	return err
}
func LockedFence(ctx context.Context, tx *sql.Tx, id string) (domain.FenceState, error) {
	var current domain.FenceState
	err := tx.QueryRowContext(ctx, "SELECT scope_key,epoch,state,target_permission FROM authorization_change WHERE revocation_id=? FOR UPDATE", id).Scan(&current.Scope, &current.Epoch, &current.State, &current.TargetPermission)
	return current, err
}
func InsertFence(ctx context.Context, tx *sql.Tx, id, key string, epoch uint64, binding []byte) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO authorization_change(revocation_id,scope_key,epoch,target_permission,state,participant_plan) VALUES(?,?,?,?,'FENCING','["edge","collaboration"]')`, id, key, epoch, string(binding))
	return err
}
func CloseSourceAdmissions(ctx context.Context, tx *sql.Tx, key string, epoch uint64) error {
	_, err := tx.ExecContext(ctx, "UPDATE document_admission a JOIN admission_source x ON x.admission_id=a.admission_id SET a.state='CLOSED' WHERE x.scope_key=? AND x.epoch<=? AND a.state<>'CLOSED'", key, epoch)
	if err != nil {
		return err
	}
	// 关闭只处理这次原范围及旧代次的派生能力。重扫全部已关闭历史不仅增加成本，
	// 还会等在另一个范围的行锁上，令独立关闭任务彼此阻塞。
	_, err = tx.ExecContext(ctx, "UPDATE read_view v JOIN document_admission a ON a.admission_id=v.admission_id JOIN admission_source x ON x.admission_id=a.admission_id SET v.state='CLOSED' WHERE x.scope_key=? AND x.epoch<=? AND a.state='CLOSED' AND v.state<>'CLOSED'", key, epoch)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "UPDATE collaboration_ticket t JOIN document_admission a ON a.admission_id=t.admission_id JOIN admission_source x ON x.admission_id=a.admission_id SET t.state='CLOSED' WHERE x.scope_key=? AND x.epoch<=? AND a.state='CLOSED' AND t.state<>'CLOSED'", key, epoch)
	return err
}
func (r *Fences) Unsettled(ctx context.Context) ([]domain.PersistedOutput, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT business_key,payload FROM durable_task WHERE kind='OUTPUT_LEASE' AND state<>'SUCCEEDED'")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []domain.PersistedOutput
	for rows.Next() {
		var output domain.PersistedOutput
		if rows.Scan(&output.ID, &output.Payload) != nil {
			return nil, errors.New("OUTPUT_RECORD_INVALID")
		}
		list = append(list, output)
	}
	return list, rows.Err()
}
func (r *Fences) RecordEdgeAck(ctx context.Context, id, key string, epoch uint64, digest []byte) error {
	_, err := r.db.ExecContext(ctx, "INSERT INTO authorization_ack(revocation_id,participant,scope_key,epoch,phase,result_digest,completed_at) VALUES(?,'edge',?,?,'OUTPUT_STOPPED',?,UTC_TIMESTAMP(6)) ON DUPLICATE KEY UPDATE result_digest=VALUES(result_digest)", id, key, epoch, digest)
	return err
}
func RecordNodeAck(ctx context.Context, tx *sql.Tx, id, key string, epoch uint64, digest []byte) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO authorization_ack(revocation_id,participant,scope_key,epoch,phase,result_digest,completed_at) VALUES(?,'collaboration',?,?,'OUTPUT_STOPPED',?,UTC_TIMESTAMP(6)) ON DUPLICATE KEY UPDATE result_digest=VALUES(result_digest)", id, key, epoch, digest)
	return err
}
func CompleteFence(ctx context.Context, tx *sql.Tx, id string) error {
	_, err := tx.ExecContext(ctx, "UPDATE authorization_change SET state='SUCCEEDED' WHERE revocation_id=?", id)
	return err
}

// output.go：保留原实现，按业务合并文件。

type Outputs struct{ db *sql.DB }

func NewOutputs(db *sql.DB) *Outputs { return &Outputs{db: db} }

// 所有写方法只使用用例传入的tx，来源锁和输出记录不能分散到另一连接。
func EnsureInstance(ctx context.Context, tx *sql.Tx, i domain.OutputInstance) error {
	digest, err := hex.DecodeString(i.Boot)
	if err != nil || len(digest) != 32 || i.ID == "" || i.PID == 0 {
		return errors.New("OUTPUT_INSTANCE_INVALID")
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO output_instance(instance_id,boot_digest,pid,state,last_heartbeat_at) VALUES(?,?,?,'RUNNING',UTC_TIMESTAMP(6)) ON DUPLICATE KEY UPDATE last_heartbeat_at=UTC_TIMESTAMP(6)", i.ID, digest, i.PID)
	if err != nil {
		return err
	}
	var pid uint64
	var prior []byte
	var state string
	if tx.QueryRowContext(ctx, "SELECT pid,boot_digest,state FROM output_instance WHERE instance_id=? FOR UPDATE", i.ID).Scan(&pid, &prior, &state) != nil || pid != i.PID || !bytes.Equal(prior, digest) || state != "RUNNING" {
		return errors.New("OUTPUT_INSTANCE_CONFLICT")
	}
	return nil
}

func InsertOutputLease(ctx context.Context, tx *sql.Tx, l domain.OutputLease) error {
	var epoch, release uint64
	public := l.PublicationEpoch != "" || l.Release != ""
	if public {
		var err error
		epoch, err = domain.CanonicalID(l.PublicationEpoch)
		if err != nil {
			return errors.New("OUTPUT_PUBLICATION_INVALID")
		}
		release, err = domain.CanonicalID(l.Release)
		if err != nil || l.Document == 0 || l.Admission != "" {
			return errors.New("OUTPUT_PUBLICATION_INVALID")
		}
	}
	raw, _ := json.Marshal(l)
	_, err := tx.ExecContext(ctx, "INSERT INTO durable_task(kind,business_key,payload,state,instance_id,lease_until,retry_at) VALUES('OUTPUT_LEASE',?,?,'RUNNING',?,DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 30 SECOND),UTC_TIMESTAMP(6))", l.ID, string(raw), l.Instance)
	if err != nil {
		return err
	}
	if l.Document != 0 {
		if public {
			// 匿名和已登录正式读者都绑定当前正式指针；身份依赖另存sources，不伪造Doc准入。
			_, err = tx.ExecContext(ctx, "INSERT INTO delivery_session(delivery_id,admission_id,document_id,publication_epoch,release_id,target_kind,target_ref,instance_id,handler_id,state,expires_at) VALUES(?,NULL,?,?,?,'HTTP',?,?,?,'OPEN',DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 30 SECOND))", l.ID, l.Document, epoch, release, l.ID, l.Instance, l.Handler)
		} else {
			_, err = tx.ExecContext(ctx, "INSERT INTO delivery_session(delivery_id,admission_id,document_id,target_kind,target_ref,instance_id,handler_id,state,expires_at) VALUES(?,?,?,'HTTP',?,?,?,'OPEN',DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 30 SECOND))", l.ID, l.Admission, l.Document, l.ID, l.Instance, l.Handler)
		}
	}
	return err
}
func (m *Outputs) Settle(ctx context.Context, tx *sql.Tx, id, instance string) error {
	result, err := tx.ExecContext(ctx, "UPDATE durable_task SET state='SUCCEEDED' WHERE kind='OUTPUT_LEASE' AND business_key=? AND instance_id=?", id, instance)
	if err != nil {
		return err
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		var existing string
		if tx.QueryRowContext(ctx, "SELECT state FROM durable_task WHERE kind='OUTPUT_LEASE' AND business_key=? AND instance_id=? FOR UPDATE", id, instance).Scan(&existing) != nil || existing != "SUCCEEDED" {
			return errors.New("OUTPUT_LEASE_UNKNOWN")
		}
	}
	if _, err = tx.ExecContext(ctx, "UPDATE delivery_session SET state='SETTLED' WHERE delivery_id=? AND instance_id=?", id, instance); err != nil {
		return err
	}
	return nil
}

type OutputProcess struct {
	ID  string
	PID uint64
}

func (r *Outputs) ActiveInstances(ctx context.Context) ([]OutputProcess, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT instance_id,pid FROM output_instance WHERE state<>'EXIT_CONFIRMED'")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []OutputProcess
	for rows.Next() {
		var instance OutputProcess
		if rows.Scan(&instance.ID, &instance.PID) != nil {
			return nil, errors.New("OUTPUT_RECOVERY_INVALID")
		}
		list = append(list, instance)
	}
	return list, rows.Err()
}
func (r *Outputs) ConfirmExit(ctx context.Context, tx *sql.Tx, id, state string) error {
	_, err := tx.ExecContext(ctx, "UPDATE output_instance SET state='EXIT_CONFIRMED',exit_proof_ref=? WHERE instance_id=?", "PID_CHECK:"+state, id)
	if err == nil {
		_, err = tx.ExecContext(ctx, "UPDATE delivery_session SET state='SETTLED',stop_reason='EXIT_CONFIRMED' WHERE instance_id=? AND state<>'SETTLED'", id)
	}
	if err == nil {
		_, err = tx.ExecContext(ctx, "UPDATE durable_task SET state='SUCCEEDED',last_error='OUTPUT_EXIT_CONFIRMED' WHERE kind='OUTPUT_LEASE' AND instance_id=? AND state<>'SUCCEEDED'", id)
	}
	return err
}
func (r *Outputs) UnrecoveredCount(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM durable_task t LEFT JOIN output_instance i ON i.instance_id=t.instance_id WHERE t.kind='OUTPUT_LEASE' AND t.state<>'SUCCEEDED' AND (i.state IS NULL OR i.state<>'RUNNING')").Scan(&count)
	return count, err
}

type PendingOutput struct{ ID, Instance string }

// Pending和本地handler不是同一时点；重新查询时仍须绑定原输出及原实例。
// 只接受持久SUCCEEDED，缺记录、读取失败和过期租约都不能证明输出已结束。
func (r *Outputs) Settled(ctx context.Context, id, instance string) (bool, error) {
	var state string
	err := r.db.QueryRowContext(ctx, "SELECT state FROM durable_task WHERE kind='OUTPUT_LEASE' AND business_key=? AND instance_id=?", id, instance).Scan(&state)
	return state == "SUCCEEDED", err
}

func (r *Outputs) Pending(ctx context.Context) ([]PendingOutput, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT business_key,instance_id FROM durable_task WHERE kind='OUTPUT_LEASE' AND state<>'SUCCEEDED'")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []PendingOutput
	for rows.Next() {
		var output PendingOutput
		if rows.Scan(&output.ID, &output.Instance) != nil {
			return nil, errors.New("OUTPUT_RECORD_INVALID")
		}
		list = append(list, output)
	}
	return list, rows.Err()
}
