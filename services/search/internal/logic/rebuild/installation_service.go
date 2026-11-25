// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package rebuild

import (
	"bytes"
	"collaboration/platform/rebuildartifact"
	"collaboration/platform/rebuildmeta"
	"collaboration/search/internal/client/logsource"
	generation "collaboration/search/internal/repository/generation"
	repository "collaboration/search/internal/repository/rebuild"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// import.go：保留原实现，按业务合并文件。

type Importer struct {
	DB                            *sql.DB
	Verifier                      *rebuildartifact.Verifier
	Owner, Alias, Inbox, Accepted string
}

// Import先恢复原绑定，再在事务外验证并发布完整流，最后短事务复核原绑定后接受。
// 文件先于SQL发布：崩溃最多留下内容地址孤儿，不能留下指向半份文件的IMPORTED。
func (s *Importer) Import(ctx context.Context, operation, reference string) ([]byte, error) {
	name, err := rebuildartifact.FileName(reference)
	if s == nil || s.DB == nil || !rebuildmeta.OperationValid(operation) || err != nil || !strings.HasSuffix(name, ".snapshot.json") || s.Inbox == "" || s.Accepted == "" {
		return nil, errors.New("SEARCH_REBUILD_IMPORT_INVALID")
	}
	binding, original, found, err := s.inspect(ctx, operation)
	if err != nil {
		return nil, err
	}
	var row repository.Imported
	if found {
		actual, _ := rebuildartifact.Reference(original.Envelope, rebuildartifact.SnapshotV1)
		if actual != reference {
			return nil, errors.New("SEARCH_REBUILD_OPERATION_CONFLICT")
		}
		row = original
	} else {
		raw, err := rebuildartifact.ReadFile(s.Inbox, reference)
		if err != nil {
			return nil, err
		}
		if s.Verifier == nil {
			return nil, rebuildartifact.ErrConfig
		}
		verified, err := s.Verifier.Verify(raw, rebuildartifact.SnapshotV1)
		if err != nil {
			return nil, err
		}
		row.Seal, err = rebuildmeta.ParseSeal(verified.Payload)
		if err != nil {
			return nil, err
		}
		row.Envelope = raw
		row.Digest = sha256.Sum256(raw)
		signer, err := hex.DecodeString(verified.SignerFingerprint)
		if err != nil || len(signer) != 32 {
			return nil, rebuildartifact.ErrIdentity
		}
		copy(row.SignerDigest[:], signer)
	}
	prepareRef, _ := rebuildartifact.Reference(binding.Envelope, rebuildartifact.PrepareV1)
	if row.Seal.PrepareRef != prepareRef {
		return nil, errors.New("SEARCH_REBUILD_OPERATION_CONFLICT")
	}
	entriesName := strings.TrimSuffix(name, ".snapshot.json") + ".entries.ndjson"
	// 已接受副本优先恢复；丢失时允许从相同inbox重建，但仍核完整原摘要，绝不重签。
	inputDir := s.Inbox
	if found {
		if _, err := os.Lstat(filepath.Join(s.Accepted, entriesName)); err == nil {
			inputDir = s.Accepted
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, errors.New("SEARCH_REBUILD_INPUT_FAILED")
		}
	}
	if err := rebuildartifact.PublishFile(s.Accepted, entriesName, func(w io.Writer) error {
		info, err := os.Lstat(filepath.Join(inputDir, entriesName))
		if err != nil || !info.Mode().IsRegular() {
			return errors.New("SEARCH_REBUILD_INPUT_FAILED")
		}
		f, err := os.Open(filepath.Join(inputDir, entriesName))
		if err != nil {
			return errors.New("SEARCH_REBUILD_INPUT_FAILED")
		}
		defer f.Close()
		count, digest, err := rebuildmeta.Scan(io.TeeReader(&importReader{ctx: ctx, reader: f}, w), nil)
		if err != nil {
			return err
		}
		if strconv.FormatUint(count, 10) != row.Seal.Count || hex.EncodeToString(digest[:]) != row.Seal.EntriesDigest {
			return errors.New("SEARCH_REBUILD_ENTRIES_MISMATCH")
		}
		return nil
	}); err != nil {
		return nil, err
	}
	if err := rebuildartifact.PublishFile(s.Accepted, name, func(w io.Writer) error { _, err := io.Copy(w, bytes.NewReader(row.Envelope)); return err }); err != nil {
		return nil, err
	}
	// 全流IO已退出；此处只复核不可变绑定，不在代次锁内重扫文件。
	short, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := s.DB.BeginTx(short, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, errors.New("SEARCH_REBUILD_TRANSACTION_UNAVAILABLE")
	}
	defer tx.Rollback()
	current, accepted, has, err := s.lock(short, tx, operation)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(binding.Envelope, current.Envelope) {
		return nil, errors.New("SEARCH_REBUILD_OPERATION_CONFLICT")
	}
	if has {
		if !bytes.Equal(accepted.Envelope, row.Envelope) {
			return nil, errors.New("SEARCH_REBUILD_OPERATION_CONFLICT")
		}
		return accepted.Envelope, nil
	}
	if err := repository.SaveImport(short, tx, operation, row); err != nil {
		return nil, err
	}
	if tx.Commit() != nil {
		return nil, errors.New("SEARCH_REBUILD_IMPORT_COMMIT_UNKNOWN")
	}
	return row.Envelope, nil
}

func (s *Importer) inspect(ctx context.Context, operation string) (repository.Binding, repository.Imported, bool, error) {
	short, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := s.DB.BeginTx(short, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return repository.Binding{}, repository.Imported{}, false, errors.New("SEARCH_REBUILD_TRANSACTION_UNAVAILABLE")
	}
	defer tx.Rollback()
	return s.lock(short, tx, operation)
}

func (s *Importer) lock(ctx context.Context, tx *sql.Tx, operation string) (repository.Binding, repository.Imported, bool, error) {
	fail := func(err error) (repository.Binding, repository.Imported, bool, error) {
		return repository.Binding{}, repository.Imported{}, false, err
	}
	source, exists, err := generation.Find(ctx, tx, s.Alias, true)
	if err != nil {
		return fail(err)
	}
	if !exists {
		return fail(errors.New("SEARCH_GENERATION_MISSING"))
	}
	binding, exists, err := repository.Find(ctx, tx, operation)
	if err != nil {
		return fail(err)
	}
	if !exists {
		return fail(errors.New("SEARCH_REBUILD_PREPARE_MISSING"))
	}
	p := binding.Prepare
	if p.Owner != s.Owner || p.Alias != s.Alias || binding.State == "REJECTED" {
		return fail(errors.New("SEARCH_REBUILD_OPERATION_CONFLICT"))
	}
	row, found, err := repository.FindImport(ctx, tx, operation)
	if err != nil {
		return fail(err)
	}
	if found {
		ref, _ := rebuildartifact.Reference(binding.Envelope, rebuildartifact.PrepareV1)
		if row.Seal.PrepareRef != ref || (binding.State != "IMPORTED" && binding.State != "ACCEPTED") {
			return fail(errors.New("SEARCH_REBUILD_IMPORT_INVALID"))
		}
	} else if binding.State != "PREPARED" || source.Physical != p.Source || strconv.FormatUint(source.Generation, 10) != p.SourceGeneration || strconv.FormatUint(source.Revision, 10) != p.SourceRevision {
		return fail(errors.New("SEARCH_REBUILD_GENERATION_INVALID"))
	}
	return binding, row, found, nil
}

type importReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *importReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

// seed.go：保留原实现，按业务合并文件。

type SeedWriter interface {
	SeedInTransaction(context.Context, *sql.Tx, string, rebuildmeta.Entry) error
}
type SeedIndex interface {
	EnsureIndex(context.Context, string) error
}

type Seeder struct {
	DB                        *sql.DB
	Owner, Alias, AcceptedDir string
	Writer                    SeedWriter
	Indexes                   SeedIndex
	Logs                      Logs
}

type seedJob struct {
	prepare  rebuildmeta.Prepare
	imported repository.Imported
	job      repository.Job
}

// Step只取一个已验证批次。整库文件校验在事务外，逐文档事务串行保护状态，
// 不使用到期租约推断另一个写步骤已停止，也不持有整库级长事务。
func (s *Seeder) Step(ctx context.Context) error {
	if s == nil || s.DB == nil || s.Writer == nil || s.Indexes == nil || s.Logs == nil {
		return errors.New("SEARCH_REBUILD_DEPENDENCIES_UNAVAILABLE")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	operation, err := repository.PendingSnapshot(ctx, s.DB)
	if err != nil || operation == "" {
		return err
	}
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return errors.New("SEARCH_REBUILD_TRANSACTION_UNAVAILABLE")
	}
	input, err := s.lock(ctx, tx, operation)
	tx.Rollback()
	if err != nil {
		return err
	}
	if input.job.State != "SNAPSHOT" {
		return nil
	}
	identity := logsource.Identity{Cluster: input.prepare.Cluster, Topic: input.prepare.Topic}
	uuid, _ := hex.DecodeString(input.prepare.TopicID)
	copy(identity.TopicID[:], uuid)
	window, err := s.Logs.Capture(ctx, &identity)
	if err != nil {
		return err
	}
	for partition, raw := range input.prepare.Starts {
		start, _ := strconv.ParseInt(raw, 10, 64)
		if window.Partitions[partition].First > start || window.Partitions[partition].Last < start {
			return errors.New("SEARCH_REBUILD_LOG_WINDOW_LOST")
		}
	}
	page, err := snapshotPage(ctx, s.AcceptedDir, input.job.SnapshotRef, input.imported.Seal, input.job.SnapshotAfter)
	if err != nil {
		return err
	}
	if err := s.Indexes.EnsureIndex(ctx, input.job.Target); err != nil {
		return err
	}
	after := input.job.SnapshotAfter
	for _, entry := range page {
		if err := s.apply(ctx, operation, input.job.ID, after, &entry); err != nil {
			return err
		}
		after = entry.Document
	}
	if len(page) == 0 {
		return s.apply(ctx, operation, input.job.ID, after, nil)
	}
	return nil
}

func (s *Seeder) lock(ctx context.Context, tx *sql.Tx, operation string) (seedJob, error) {
	var result seedJob
	current, found, err := generation.Find(ctx, tx, s.Alias, false)
	if err != nil {
		return result, err
	}
	if !found {
		return result, errors.New("SEARCH_GENERATION_MISSING")
	}
	binding, found, err := repository.Find(ctx, tx, operation)
	if err != nil {
		return result, err
	}
	if !found || binding.State != "ACCEPTED" || binding.Prepare.Owner != s.Owner || binding.Prepare.Alias != s.Alias {
		return result, errors.New("SEARCH_REBUILD_OPERATION_CONFLICT")
	}
	bound, err := repository.LoadJobBinding(ctx, tx, operation)
	if err != nil {
		return result, err
	}
	id, err := rebuildmeta.Decimal(bound.JobID.String, true)
	if err != nil || !bound.JobID.Valid {
		return result, errors.New("SEARCH_REBUILD_JOB_INVALID")
	}
	result.job, err = repository.LoadJob(ctx, tx, id)
	if err != nil {
		return result, err
	}
	result.prepare = binding.Prepare
	if result.job.State != "SNAPSHOT" && result.job.State != "REPLAYING" && result.job.State != "SWITCHING" {
		return result, nil
	}
	p := result.prepare
	if current.Physical != p.Source || strconv.FormatUint(current.Generation, 10) != p.SourceGeneration || strconv.FormatUint(current.Revision, 10) != p.SourceRevision || result.job.Target != p.Target {
		return result, errors.New("SEARCH_REBUILD_GENERATION_CHANGED")
	}
	result.imported, found, err = repository.FindImport(ctx, tx, operation)
	if err != nil {
		return result, err
	}
	if !found {
		return result, errors.New("SEARCH_REBUILD_NOT_IMPORTED")
	}
	return result, nil
}

func (s *Seeder) apply(parent context.Context, operation string, id, after uint64, entry *rebuildmeta.Entry) error {
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return errors.New("SEARCH_REBUILD_TRANSACTION_UNAVAILABLE")
	}
	defer tx.Rollback()
	current, err := s.lock(ctx, tx, operation)
	if err != nil {
		return err
	}
	if current.job.ID != id || current.job.State != "SNAPSHOT" || current.job.SnapshotAfter != after {
		return errors.New("SEARCH_REBUILD_PROGRESS_CONFLICT")
	}
	if entry == nil {
		err = repository.FinishSnapshot(ctx, tx, id)
	} else {
		if entry.Document <= after {
			return errors.New("SEARCH_REBUILD_PROGRESS_CONFLICT")
		}
		if err = s.Writer.SeedInTransaction(ctx, tx, current.job.Target, *entry); err == nil {
			err = repository.AdvanceSnapshot(ctx, tx, id, after, entry.Document)
		}
	}
	if err != nil {
		return err
	}
	if tx.Commit() != nil {
		return errors.New("SEARCH_REBUILD_COMMIT_UNKNOWN")
	}
	return nil
}

// snapshot_page.go：保留原实现，按业务合并文件。

// snapshotPage先核完整原副本，再交付一个有界批次。不能在Scan回调中直接写ES，
// 否则文件尾部损坏会在摘要失败前产生未经封存证明的可见副作用。
func snapshotPage(ctx context.Context, dir, reference string, seal rebuildmeta.Seal, after uint64) ([]rebuildmeta.Entry, error) {
	name, err := rebuildartifact.FileName(reference)
	if err != nil || !filepath.IsAbs(dir) || !strings.HasSuffix(name, ".snapshot.json") {
		return nil, errors.New("SEARCH_REBUILD_SNAPSHOT_UNAVAILABLE")
	}
	path := filepath.Join(dir, strings.TrimSuffix(name, ".snapshot.json")+".entries.ndjson")
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return nil, errors.New("SEARCH_REBUILD_SNAPSHOT_UNAVAILABLE")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, errors.New("SEARCH_REBUILD_SNAPSHOT_UNAVAILABLE")
	}
	defer f.Close()
	page := make([]rebuildmeta.Entry, 0, 32)
	count, digest, err := rebuildmeta.Scan(&importReader{ctx: ctx, reader: f}, func(entry rebuildmeta.Entry) error {
		if entry.Document > after && len(page) < 32 {
			page = append(page, entry)
		}
		return nil
	})
	if err != nil || strconv.FormatUint(count, 10) != seal.Count || hex.EncodeToString(digest[:]) != seal.EntriesDigest {
		return nil, errors.New("SEARCH_REBUILD_SNAPSHOT_INVALID")
	}
	return page, nil
}
