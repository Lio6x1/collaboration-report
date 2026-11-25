// Package snapshot 编排 Document 自有的一致快照，不读取 Search 的私有库。
package snapshot

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"strconv"

	repo "collaboration/document/internal/repository/snapshot"
	"collaboration/platform/rebuildartifact"
	"collaboration/platform/rebuildmeta"
)

var (
	ErrInput         = errors.New("REBUILD_SNAPSHOT_INPUT_INVALID")
	ErrConflict      = errors.New("REBUILD_OPERATION_CONFLICT")
	ErrCommitUnknown = errors.New("REBUILD_SNAPSHOT_COMMIT_UNKNOWN_RETRY_SAME_OPERATION")
	ErrIncomplete    = errors.New("REBUILD_SNAPSHOT_INCOMPLETE")
)

// Service 的证书与数据库只能由本服务配置装配。恢复已封存任务不使用 Signer，
// 因而 CLI 可先恢复，再在确需新封存时加载当前有效的本服务签名身份。
type Service struct {
	DB         *sql.DB
	Verifier   *rebuildartifact.Verifier
	Signer     *rebuildartifact.Signer
	LoadSigner func() (*rebuildartifact.Signer, error)
}

// Capture 在一次 RR 事务内同时保存完整条目和签名。任何扫描/签名错误均回滚；
// commit 回执未知时不导出本次内存结果，调用者只能带完全相同的原号和输入重试。
func (s *Service) Capture(ctx context.Context, operation string, prepareEnvelope []byte) ([]byte, error) {
	if s == nil || s.DB == nil || !rebuildmeta.OperationValid(operation) || len(prepareEnvelope) == 0 || len(prepareEnvelope) > rebuildartifact.MaxEnvelopeBytes {
		return nil, ErrInput
	}
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, errors.New("REBUILD_SNAPSHOT_BEGIN_FAILED")
	}
	defer tx.Rollback()

	// 锁定读取不建立业务表的一致视图；先检查持久接受事实，证书到期不能使
	// 已接受的原操作失去恢复能力。只有明确不存在才允许按当前信任链首次准入。
	job, err := repo.Lock(ctx, tx, operation)
	var accepted rebuildartifact.Verified
	if errors.Is(err, repo.ErrNotFound) {
		if s.Verifier == nil {
			return nil, rebuildartifact.ErrConfig
		}
		accepted, err = s.Verifier.Verify(prepareEnvelope, rebuildartifact.PrepareV1)
		if err != nil {
			return nil, err
		}
		if err := checkOperation(accepted, operation); err != nil {
			return nil, err
		}
		fingerprint, err := hex.DecodeString(accepted.SignerFingerprint)
		if err != nil || len(fingerprint) != 32 {
			return nil, rebuildartifact.ErrIdentity
		}
		var signerDigest [32]byte
		copy(signerDigest[:], fingerprint)
		job, err = repo.Reserve(ctx, tx, operation, prepareEnvelope, signerDigest)
		if err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	} else {
		if job.Operation != operation || !bytes.Equal(job.Prepare, prepareEnvelope) || sha256.Sum256(prepareEnvelope) != job.PrepareDigest {
			return nil, ErrConflict
		}
		accepted, err = rebuildartifact.RestoreAccepted(job.Prepare, rebuildartifact.PrepareV1, job.PrepareDigest, job.SignerDigest)
		if err != nil {
			return nil, err
		}
		if err := checkOperation(accepted, operation); err != nil {
			return nil, err
		}
	}
	if job.State == "SEALED" {
		// 只返回已经提交的原签名；回滚释放读取锁即可，不重签也不重扫业务表。
		return append([]byte(nil), job.Seal...), nil
	}
	// 当前签名身份仅用于新封存。延迟加载让证书轮换/过期与原制品恢复分离，
	// 同时在整库扫描前报告配置错误，避免已知无法签名仍持有长事务。
	signer := s.Signer
	if signer == nil && s.LoadSigner != nil {
		signer, err = s.LoadSigner()
		if err != nil {
			return nil, err
		}
	}
	if signer == nil {
		return nil, rebuildartifact.ErrConfig
	}

	hash := sha256.New()
	var count, after uint64
	for {
		// 首次普通 SELECT 在 prepare 已验签并绑定后建立 S，后续所有页仍在同一事务。
		entries, err := repo.CurrentPage(ctx, tx, after, 500)
		if err != nil {
			return nil, err
		}
		if len(entries) == 0 {
			break
		}
		for _, entry := range entries {
			if entry.Document <= after || count == ^uint64(0) {
				return nil, ErrIncomplete
			}
			line, err := entry.Line()
			if err != nil {
				return nil, err
			}
			hash.Write(line)
			after, count = entry.Document, count+1
		}
		if err := repo.InsertEntries(ctx, tx, operation, entries); err != nil {
			return nil, err
		}
	}
	var digest [32]byte
	copy(digest[:], hash.Sum(nil))
	payload, err := (rebuildmeta.Seal{Version: 1, PrepareRef: accepted.Reference,
		Count: strconv.FormatUint(count, 10), EntriesDigest: hex.EncodeToString(digest[:])}).Bytes()
	if err != nil {
		return nil, err
	}
	envelope, err := signer.Sign(rebuildartifact.SnapshotV1, payload)
	if err != nil {
		return nil, err
	}
	if err := repo.Seal(ctx, tx, operation, count, digest, envelope); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, ErrCommitUnknown
	}
	return envelope, nil
}

func checkOperation(accepted rebuildartifact.Verified, operation string) error {
	prepare, err := rebuildmeta.ParsePrepare(accepted.Payload)
	if err != nil {
		return err
	}
	if prepare.Operation != operation {
		return ErrConflict
	}
	return nil
}
