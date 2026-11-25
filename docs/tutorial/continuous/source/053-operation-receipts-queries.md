# services/account/internal/repository/account/operation_receipts_queries.go

阶段02：[功能讲义](../02-account.md) · [分步编写](../write/053-operation-receipts-queries.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：集中SQL和行锁；使用用例传入的同一事务，不隐式提交。

主要声明：`OperationReceipts`、`NewOperationReceipts`、`Lookup`、`ConcurrentLogout`、`CurrentCredential`、`Result`、`OperationResultSnapshot`、`ReadOperationResult`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package account

import (
	domain "collaboration/account/internal/domain/account"
	"context"
	"database/sql"
)

// operation_receipt.go：保留原实现，按业务合并文件。

type OperationReceipts struct{ db *sql.DB }

func NewOperationReceipts(db *sql.DB) *OperationReceipts { return &OperationReceipts{db: db} }

func (r *OperationReceipts) Lookup(ctx context.Context, id string) (domain.OperationReceipt, error) {
	var value domain.OperationReceipt
	err := r.db.QueryRowContext(ctx, `
		SELECT CAST(actor_id AS CHAR),kind,state,
		       COALESCE(JSON_UNQUOTE(JSON_EXTRACT(target_json,'$.session_hash')),'')
		FROM operation_receipt WHERE operation_id=?`, id).
		Scan(&value.ActorID, &value.Kind, &value.State, &value.SessionHash)
	return value, err
}

func (r *OperationReceipts) ConcurrentLogout(ctx context.Context, tx *sql.Tx, id string) (string, string, error) {
	var sessionHash, state string
	err := tx.QueryRowContext(ctx, `
		SELECT JSON_UNQUOTE(JSON_EXTRACT(target_json,'$.session_hash')),state
		FROM operation_receipt WHERE operation_id=?`, id).Scan(&sessionHash, &state)
	return sessionHash, state, err
}

func (r *OperationReceipts) CurrentCredential(ctx context.Context, user string) (domain.Credential, error) {
	var credential domain.Credential
	err := r.db.QueryRowContext(ctx, "SELECT state,CAST(credential_epoch AS CHAR) FROM user WHERE id=?", user).Scan(&credential.State, &credential.Epoch)
	return credential, err
}

func (r *OperationReceipts) Result(ctx context.Context, id string) ([]byte, error) {
	var result []byte
	err := r.db.QueryRowContext(ctx, "SELECT result_json FROM operation_receipt WHERE operation_id=?", id).Scan(&result)
	return result, err
}

// operation_result.go：保留原实现，按业务合并文件。

// OperationResultSnapshot包括原不可变绑定和同一行的版本/结果，不能把多次查询的
// state与generation拼接；授权前后由用例核验该快照仍为同一个持久事实。
type OperationResultSnapshot struct {
	Actor, Kind, State, SessionHash string
	Fingerprint, Result             []byte
	Generation                      uint64
	Terminal                        bool
}

func ReadOperationResult(ctx context.Context, db *sql.DB, operation string) (OperationResultSnapshot, error) {
	var value OperationResultSnapshot
	err := db.QueryRowContext(ctx, `SELECT CAST(actor_id AS CHAR),kind,state,
	 COALESCE(JSON_UNQUOTE(JSON_EXTRACT(target_json,'$.session_hash')),''),
	 request_fingerprint,generation,result_json,terminal_at IS NOT NULL
	 FROM operation_receipt WHERE operation_id=?`, operation).
		Scan(&value.Actor, &value.Kind, &value.State, &value.SessionHash, &value.Fingerprint, &value.Generation, &value.Result, &value.Terminal)
	return value, err
}
````

UTF-8/LF全文SHA-256：`fac26b7d73a8da928469958932e700825ea26523733ad6126fd6dc5dda004a41`。[参考实现](../../../../services/account/internal/repository/account/operation_receipts_queries.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
