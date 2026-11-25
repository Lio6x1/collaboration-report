package resource

import (
	"bytes"
	"collaboration/document/internal/storage/local"
	"collaboration/platform"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestActualUploadStorageAndQuotaSettlement(t *testing.T) {
	cfg, err := platform.LoadConfig(os.Getenv("COLLAB_TEST_DOCUMENT_CONFIG"))
	if err != nil {
		t.Fatal("ACTUAL_DOCUMENT_CONFIG_REQUIRED")
	}
	store, err := platform.OpenStore(cfg)
	if err != nil {
		t.Fatal("ACTUAL_DOCUMENT_DATABASE_UNAVAILABLE")
	}
	defer store.DB.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	tx, err := store.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	// 独立临时空间和文档只在本事务可见；不借用产品文档或改变现有配额。
	_, err = tx.ExecContext(ctx, `INSERT INTO space(kind,account_owner_id,lifecycle) VALUES('PERSONAL',1,'ACTIVE')`)
	must(err)
	var space, document uint64
	must(tx.QueryRowContext(ctx, `SELECT LAST_INSERT_ID()`).Scan(&space))
	_, err = tx.ExecContext(ctx, `INSERT INTO document(space_id,responsible_user_id,title,lifecycle,publication_state) VALUES(?,1,'resource transactional fixture','ACTIVE','UNPUBLISHED')`, space)
	must(err)
	must(tx.QueryRowContext(ctx, `SELECT LAST_INSERT_ID()`).Scan(&document))
	q, err := LockQuota(ctx, tx, space, 1024)
	must(err)
	body := []byte("资源上传与配额原子结算")
	op := fmt.Sprintf("resource-actual-%d", time.Now().UnixNano())
	expires := time.Now().UTC().Add(time.Minute)
	u := Upload{Space: space, Document: document, Uploader: 1, Operation: op, Size: uint64(len(body)), MediaType: "text/plain", Checksum: sha256.Sum256(body)}
	key := sha256.Sum256([]byte(op))
	u.Key = hex.EncodeToString(key[:])
	must(ReserveQuota(ctx, tx, q, op, u.Size, q.Revision, expires))
	must(ReserveTemporary(ctx, tx, 256<<20, u.Size))
	id, err := InsertUpload(ctx, tx, u)
	must(err)
	current, err := UploadRow(ctx, tx, id, true)
	must(err)
	if current.Document != document || current.Operation != op || current.State != "UPLOADING" || current.Checksum != u.Checksum {
		t.Fatal("UPLOAD_ORIGINAL_BINDING_NOT_PRESERVED")
	}
	files, err := local.New(t.TempDir())
	must(err)
	// 半份文件不允许发布为可读取对象，预留也不能提前进入used。
	if files.Put(ctx, u.Key, bytes.NewReader(body[:3]), u.Size, u.Checksum) == nil {
		t.Fatal("PARTIAL_UPLOAD_ACCEPTED")
	}
	if file, err := files.Open(ctx, u.Key, u.Size, u.Checksum); err == nil {
		file.Close()
		t.Fatal("PARTIAL_UPLOAD_VISIBLE")
	}
	must(files.Put(ctx, u.Key, bytes.NewReader(body), u.Size, u.Checksum))
	file, err := files.Open(ctx, u.Key, u.Size, u.Checksum)
	must(err)
	must(file.Close())
	q, err = LockQuota(ctx, tx, space, 1024)
	must(err)
	must(SettleQuota(ctx, tx, q, op, true))
	must(ReleaseTemporary(ctx, tx, u.Size))
	must(MarkReady(ctx, tx, current))
	ready, err := UploadRow(ctx, tx, id, false)
	must(err)
	q, err = LockQuota(ctx, tx, space, 1024)
	must(err)
	if ready.State != "READY" || ready.Revision != current.Revision+1 || q.Used != u.Size || q.Reserved != 0 {
		t.Fatal("READY_QUOTA_SETTLEMENT_INCONSISTENT")
	}
	must(SettleQuota(ctx, tx, q, op, true))
	// 未完成上传过期后，先进入不可再写的DELETING，再随删除确认释放两份预算。
	u.Operation += "-expired"
	u.Key = hex.EncodeToString(sha256Bytes(u.Operation))
	must(ReserveQuota(ctx, tx, q, u.Operation, u.Size, q.Revision, time.Now().Add(-time.Second)))
	must(ReserveTemporary(ctx, tx, 256<<20, u.Size))
	expiredID, err := InsertUpload(ctx, tx, u)
	must(err)
	expired, err := UploadRow(ctx, tx, expiredID, true)
	must(err)
	must(MarkDeleting(ctx, tx, expired))
	expired, err = UploadRow(ctx, tx, expiredID, true)
	must(err)
	if expired.State != "DELETING" {
		t.Fatal("EXPIRED_UPLOAD_NOT_CLOSED")
	}
	must(files.Delete(ctx, expired.Key))
	q, err = LockQuota(ctx, tx, space, 1024)
	must(err)
	must(MarkDeleted(ctx, tx, expired))
	must(SettleQuota(ctx, tx, q, u.Operation, false))
	must(ReleaseTemporary(ctx, tx, u.Size))
	// READY对象先保留插入宽限；其后任何共享引用或UNKNOWN保护都阻止删除。
	q, err = LockQuota(ctx, tx, space, 1024)
	must(err)
	changed, err := MarkReadyDeleting(ctx, tx, ready)
	must(err)
	if changed {
		t.Fatal("READY_INSERTION_GRACE_LOST")
	}
	_, err = tx.ExecContext(ctx, `UPDATE quota_reservation SET expires_at=DATE_SUB(UTC_TIMESTAMP(6),INTERVAL 2 HOUR) WHERE operation_id=?`, ready.Operation)
	must(err)
	_, err = tx.ExecContext(ctx, `INSERT INTO resource_reference(resource_id,document_id,owner_kind,owner_ref,purpose) VALUES(?,?,'VERSION','retained-history','CONTENT')`, ready.ID, document)
	must(err)
	changed, err = MarkReadyDeleting(ctx, tx, ready)
	must(err)
	if changed {
		t.Fatal("RETAINED_VERSION_RESOURCE_DELETED")
	}
	_, err = tx.ExecContext(ctx, `DELETE FROM resource_reference WHERE resource_id=? AND owner_kind='VERSION' AND owner_ref='retained-history'`, ready.ID)
	must(err)
	_, err = tx.ExecContext(ctx, `INSERT INTO resource_protection(operation_id,document_id,room_id,grant_id,workspace_epoch,request_fingerprint,resource_ids,source_epochs,state) VALUES(?,?,1,'fixture',1,?,JSON_ARRAY(?),JSON_ARRAY(),'UNKNOWN')`, ready.Operation+"-protection", document, ready.Checksum[:], ready.ID)
	must(err)
	changed, err = MarkReadyDeleting(ctx, tx, ready)
	must(err)
	if changed {
		t.Fatal("UNKNOWN_PROTECTION_RESOURCE_DELETED")
	}
	_, err = tx.ExecContext(ctx, `UPDATE resource_protection SET state='RELEASED' WHERE operation_id=?`, ready.Operation+"-protection")
	must(err)
	changed, err = MarkReadyDeleting(ctx, tx, ready)
	must(err)
	if !changed {
		t.Fatal("UNREFERENCED_READY_NOT_CLOSED")
	}
	deleting, err := UploadRow(ctx, tx, ready.ID, true)
	must(err)
	must(files.Delete(ctx, deleting.Key))
	must(MarkDeleted(ctx, tx, deleting))
	must(ReleaseUsedQuota(ctx, tx, q, deleting))
	if MarkDeleted(ctx, tx, deleting) == nil {
		t.Fatal("DUPLICATE_USED_RELEASE_ALLOWED")
	}
	q, err = LockQuota(ctx, tx, space, 1024)
	must(err)
	if q.Used != 0 || q.Reserved != 0 {
		t.Fatal("DELETED_RESOURCE_QUOTA_RETAINED")
	}
	must(tx.Rollback())
}

func sha256Bytes(value string) []byte {
	digest := sha256.Sum256([]byte(value))
	return digest[:]
}
