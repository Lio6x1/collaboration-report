// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package document

import (
	domain "collaboration/document/internal/domain/document"
	"context"
	"database/sql"
	"encoding/json"
)

// pull_main.go：保留原实现，按业务合并文件。

func PullResult(ctx context.Context, q Queryer, operation string) (MainControlReceipt, error) {
	var fact MainControlReceipt
	err := q.QueryRowContext(ctx, "SELECT actor_id,kind,state,request_fingerprint,result_json,generation,expires_at FROM operation_receipt WHERE operation_id=? AND kind='PULL_MAIN'", operation).Scan(&fact.Actor, &fact.Kind, &fact.State, &fact.Fingerprint, &fact.Result, &fact.Generation, &fact.Expires)
	return fact, err
}

// 支线控制不写submission_operation_id：该列有真实SUBMISSION外键，不能塞入另一种业务原号。
func LockedBranchControl(ctx context.Context, tx *sql.Tx, branch uint64) (string, error) {
	key := domain.TextID(branch)
	_, err := tx.ExecContext(ctx, "INSERT INTO durable_task(kind,business_key,payload,state,retry_at) VALUES('BRANCH_CONTROL_GATE',?,'{}','SUCCEEDED',UTC_TIMESTAMP(6)) ON DUPLICATE KEY UPDATE business_key=VALUES(business_key)", key)
	if err != nil {
		return "", err
	}
	var raw []byte
	if err = tx.QueryRowContext(ctx, "SELECT payload FROM durable_task WHERE kind='BRANCH_CONTROL_GATE' AND business_key=? FOR UPDATE", key).Scan(&raw); err != nil {
		return "", err
	}
	var value struct {
		Operation string `json:"operation_id"`
	}
	if json.Unmarshal(raw, &value) != nil {
		return "", domain.Violation(domain.TemporarilyUnavailable, "BRANCH_CONTROL_UNKNOWN")
	}
	return value.Operation, nil
}
func RequireBranchControlOpen(ctx context.Context, tx *sql.Tx, branch uint64) error {
	operation, err := LockedBranchControl(ctx, tx, branch)
	if err != nil {
		return err
	}
	if operation != "" {
		return domain.Violation(domain.StateConflict, "BRANCH_CONTROL_PENDING")
	}
	return nil
}
func SetBranchControl(ctx context.Context, tx *sql.Tx, branch uint64, operation string) error {
	raw, _ := json.Marshal(map[string]string{"operation_id": operation})
	_, err := tx.ExecContext(ctx, "UPDATE durable_task SET payload=? WHERE kind='BRANCH_CONTROL_GATE' AND business_key=?", string(raw), domain.TextID(branch))
	return err
}
func RegisterPull(ctx context.Context, tx *sql.Tx, branch uint64, operation string, payload []byte) error {
	if err := SetBranchControl(ctx, tx, branch, operation); err != nil {
		return err
	}
	if err := InsertSubmissionTask(ctx, tx, "PULL_MAIN_CONTEXT", operation, operation, payload, "SUCCEEDED"); err != nil {
		return err
	}
	return InsertSubmissionTask(ctx, tx, "PULL_MAIN_RECOVER", operation, operation, payload, "QUEUED")
}
func ClaimPullTask(ctx context.Context, tx *sql.Tx, instance string) (domain.SubmissionTask, error) {
	return claimOperationTask(ctx, tx, instance, "PULL_MAIN_RECOVER", "PULL_MAIN_RECONCILE")
}
func MainUnitVersion(ctx context.Context, q Queryer, document, unit, manifest uint64) (uint64, error) {
	var version uint64
	err := q.QueryRowContext(ctx, "SELECT content_version_id FROM manifest_entry WHERE document_id=? AND manifest_id=? AND unit_id=?", document, manifest, unit).Scan(&version)
	return version, err
}
func InsertPullLineage(ctx context.Context, tx *sql.Tx, id, branch, document, unit, previous, manifest, version uint64, operation, capture string, provenance []byte) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO branch_lineage(id,branch_id,unit_id,document_id,previous_id,event_kind,source_operation_id,base_manifest_id,base_version_id,capture_ref,provenance_json,provenance_schema) VALUES(?,?,?,?,?,'PULL_MAIN',?,?,?,?,?,'provenance-v1')", id, branch, unit, document, previous, operation, manifest, version, capture, string(provenance))
	return err
}
func AdvancePulledBranch(ctx context.Context, tx *sql.Tx, branch, lineage, epoch uint64) error {
	_, err := tx.ExecContext(ctx, "UPDATE branch SET current_lineage_id=?,workspace_epoch=?,lifecycle='RESTORING',revision=revision+1 WHERE id=? AND lifecycle='ACTIVE'", lineage, epoch, branch)
	return err
}
func CompletePull(ctx context.Context, tx *sql.Tx, task domain.SubmissionTask, result, install []byte) error {
	if err := UpdateOperationResult(ctx, tx, task.Operation, "SUCCEEDED", result); err != nil {
		return err
	}
	if err := InsertSubmissionTask(ctx, tx, "PULL_MAIN_COMMITTED", task.Operation, task.Operation, install, "SUCCEEDED"); err != nil {
		return err
	}
	if err := InsertSubmissionTask(ctx, tx, "PULL_MAIN_RECONCILE", task.Operation, task.Operation, install, "QUEUED"); err != nil {
		return err
	}
	return FinishMergeTask(ctx, tx, task, "SUCCEEDED")
}
func CompletePullInstallation(ctx context.Context, tx *sql.Tx, task domain.SubmissionTask, branch, epoch uint64, succeeded bool) error {
	if succeeded {
		if _, err := tx.ExecContext(ctx, "UPDATE branch SET lifecycle='ACTIVE' WHERE id=? AND workspace_epoch=? AND lifecycle='RESTORING'", branch, epoch); err != nil {
			return err
		}
	}
	if err := SetBranchControl(ctx, tx, branch, ""); err != nil {
		return err
	}
	if err := InsertSubmissionTask(ctx, tx, "PULL_MAIN_INSTALLED", task.Operation, task.Operation, []byte(`{}`), "SUCCEEDED"); err != nil {
		return err
	}
	return FinishMergeTask(ctx, tx, task, "SUCCEEDED")
}
func PullInstalled(ctx context.Context, q Queryer, operation string) (bool, error) {
	var count uint64
	err := q.QueryRowContext(ctx, "SELECT COUNT(*) FROM durable_task WHERE kind='PULL_MAIN_INSTALLED' AND business_key=? AND state='SUCCEEDED'", operation).Scan(&count)
	return count == 1, err
}

// pull_readiness.go：保留原实现，按业务合并文件。

type PendingPullBinding struct {
	Kind, Operation, State, Lifecycle                    string
	Actor, Epoch, Lineage                                uint64
	Original, Payload, Fingerprint, Target, Result, Gate []byte
	Admission                                            OriginalAdmissionBinding
}

func PendingPullBindings(ctx context.Context, db *sql.DB) ([]PendingPullBinding, error) {
	rows, err := db.QueryContext(ctx, `SELECT t.kind,t.operation_id,COALESCE(o.state,''),COALESCE(o.actor_id,0),
 original.payload,t.payload,o.request_fingerprint,o.target_json,o.result_json,gate.payload,
 COALESCE(b.lifecycle,''),COALESCE(b.workspace_epoch,0),COALESCE(b.current_lineage_id,0),
 COALESCE(a.actor_id,0),COALESCE(a.document_id,0),COALESCE(a.unit_id,0),
 COALESCE(a.identity_admission_ref,''),COALESCE(a.operation_id,''),COALESCE(a.purpose,''),
 a.target_json,a.session_ref,COALESCE(CAST(UNIX_TIMESTAMP(a.expires_at)*1000 AS UNSIGNED),0)
 FROM durable_task t
 LEFT JOIN durable_task original ON original.kind='PULL_MAIN_CONTEXT' AND original.business_key=t.operation_id AND original.operation_id=t.operation_id
 LEFT JOIN operation_receipt o ON o.operation_id=t.operation_id AND o.kind='PULL_MAIN'
 LEFT JOIN branch b ON b.id=JSON_UNQUOTE(JSON_EXTRACT(original.payload,'$.context.target.branch_id'))
 LEFT JOIN durable_task gate ON gate.kind='BRANCH_CONTROL_GATE' AND gate.business_key=CAST(b.id AS CHAR)
 LEFT JOIN document_admission a ON a.admission_id=JSON_UNQUOTE(JSON_EXTRACT(original.payload,'$.context.document_admission_ref'))
 WHERE t.kind IN ('PULL_MAIN_RECOVER','PULL_MAIN_RECONCILE') AND t.state NOT IN ('SUCCEEDED','REJECTED') ORDER BY t.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []PendingPullBinding
	for rows.Next() {
		var row PendingPullBinding
		if err = rows.Scan(&row.Kind, &row.Operation, &row.State, &row.Actor, &row.Original, &row.Payload, &row.Fingerprint, &row.Target, &row.Result, &row.Gate, &row.Lifecycle, &row.Epoch, &row.Lineage,
			&row.Admission.Actor, &row.Admission.Document, &row.Admission.Unit,
			&row.Admission.Identity, &row.Admission.Operation, &row.Admission.Purpose,
			&row.Admission.Target, &row.Admission.Session, &row.Admission.SessionExpires); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}
