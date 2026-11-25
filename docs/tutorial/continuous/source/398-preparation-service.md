# services/search/internal/logic/rebuild/preparation_service.go

阶段05：[功能讲义](../05-search.md) · [分步编写](../write/398-preparation-service.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：编排用例、权限、事务及未知结果恢复；跨服务调用不能替代持久事实。

主要声明：`SwitchIndexes`、`Barrier`、`Seal`、`Coordinator`、`Step`、`finish`、`JobService`、`StartRebuild`、`GetRebuild`、`run`、`jobResult`、`Logs`、`Aliases`、`Preparer`、`Prepare`、`ValidationIndexes`、`Validator`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// 同一业务的实现集中在本文件；事务、权限和恢复边界沿用原有用例。
package rebuild

import (
	"bytes"
	pb "collaboration/contracts/internalv1"
	"collaboration/platform/rebuildartifact"
	"collaboration/platform/rebuildmeta"
	"collaboration/search/internal/client/elastic"
	"collaboration/search/internal/client/logsource"
	events "collaboration/search/internal/domain/event"
	index "collaboration/search/internal/domain/index"
	event "collaboration/search/internal/repository/event"
	generation "collaboration/search/internal/repository/generation"
	repository "collaboration/search/internal/repository/rebuild"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// barrier.go：保留原实现，按业务合并文件。

type SwitchIndexes interface {
	InspectAliasSwitch(context.Context, string, string, string) (elastic.AliasSwitchState, error)
	SwitchAlias(context.Context, string, string, string) error
}

// Barrier只封存切换屏障，不执行alias变更。调用方必须先装配完整恢复器，
// 因为提交后普通消费及查询会由既有CheckSwitching明确暂停。
type Barrier struct {
	Seed    *Seeder
	Indexes SwitchIndexes
}

func (b *Barrier) Seal(ctx context.Context, operation string) error {
	if b == nil || b.Seed == nil || b.Seed.DB == nil || b.Seed.Logs == nil || b.Indexes == nil {
		return errors.New("SEARCH_REBUILD_DEPENDENCIES_UNAVAILABLE")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := b.Seed.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return errors.New("SEARCH_REBUILD_TRANSACTION_UNAVAILABLE")
	}
	defer tx.Rollback()
	// 排他锁先等待普通消费和重建的完整记录退出，再读取原任务；
	// 不在持有共享锁后升级，避免两个协调器同时升级产生死锁。
	_, found, err := generation.Find(ctx, tx, b.Seed.Alias, true)
	if err != nil {
		return err
	}
	if !found {
		return errors.New("SEARCH_GENERATION_MISSING")
	}
	input, err := b.Seed.lock(ctx, tx, operation)
	if err != nil {
		return err
	}
	if input.job.State == "SWITCHING" {
		_, err = repository.LoadFrontier(ctx, tx, input.job.ID)
		return err
	}
	if input.job.State != "REPLAYING" {
		return errors.New("SEARCH_REBUILD_PROGRESS_CONFLICT")
	}
	state, err := b.Indexes.InspectAliasSwitch(ctx, b.Seed.Alias, input.job.Source, input.job.Target)
	if err != nil {
		return err
	}
	if state != elastic.AliasAtSource {
		return errors.New("SEARCH_ALIAS_SWITCH_CONFLICT")
	}
	identity := logsource.Identity{Cluster: input.prepare.Cluster, Topic: input.prepare.Topic}
	uuid, err := hex.DecodeString(input.prepare.TopicID)
	if err != nil || len(uuid) != len(identity.TopicID) {
		return errors.New("SEARCH_REBUILD_LOG_IDENTITY_INVALID")
	}
	copy(identity.TopicID[:], uuid)
	window, err := b.Seed.Logs.Capture(ctx, &identity)
	if err != nil {
		return err
	}
	if window.Identity != identity {
		return errors.New("SEARCH_LOG_IDENTITY_CHANGED")
	}
	var ends [events.PartitionCount]int64
	for p, bounds := range window.Partitions {
		start, err := strconv.ParseInt(input.prepare.Starts[p], 10, 64)
		if err != nil || bounds.First > start || bounds.Last < start {
			return errors.New("SEARCH_REBUILD_LOG_WINDOW_LOST")
		}
		ends[p] = bounds.Last
		var next int64
		if tx.QueryRowContext(ctx, `SELECT next_offset FROM consumer_checkpoint WHERE consumer_name=? AND kafka_topic=? AND partition_id=?`, repository.ConsumerName(input.job.ID), input.prepare.Topic, p).Scan(&next) != nil {
			return errors.New("SEARCH_REBUILD_CHECKPOINT_MISSING")
		}
		if next < start || next > bounds.Last {
			return errors.New("SEARCH_REBUILD_CHECKPOINT_UNBOUND")
		}
		// 尚未追近时继续后台重放，不提前暂停当前搜索。
		if bounds.Last-next > 32 {
			return nil
		}
	}
	if err := repository.SealFrontier(ctx, tx, input.job.ID, ends); err != nil {
		return err
	}
	if tx.Commit() != nil {
		return errors.New("SEARCH_REBUILD_COMMIT_UNKNOWN")
	}
	return nil
}

// coordinator.go：保留原实现，按业务合并文件。

type Coordinator struct {
	Barrier   *Barrier
	Validator *Validator
}

func (c *Coordinator) Step(ctx context.Context) error {
	if c == nil || c.Barrier == nil || c.Barrier.Seed == nil || c.Validator == nil {
		return errors.New("SEARCH_REBUILD_DEPENDENCIES_UNAVAILABLE")
	}
	var operation, state string
	err := c.Barrier.Seed.DB.QueryRowContext(ctx, `SELECT b.operation_id,j.state FROM rebuild_binding b JOIN rebuild_job j ON j.id=b.job_id WHERE b.state='ACCEPTED' AND j.state IN ('REPLAYING','SWITCHING') ORDER BY j.id LIMIT 1`).Scan(&operation, &state)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return errors.New("SEARCH_REBUILD_JOB_READ_FAILED")
	}
	if state == "REPLAYING" {
		return c.Barrier.Seal(ctx, operation)
	}
	if err := c.Validator.Step(ctx, operation); err != nil {
		return err
	}
	return c.finish(ctx, operation)
}

// finish专用恢复路径不调用拒绝SWITCHING的普通Guard。ES已在目标时只完成
// 原SQL事务，绝不再次增加代次；未知回执留在同一任务下由下一步重读。
func (c *Coordinator) finish(parent context.Context, operation string) error {
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	s := c.Barrier.Seed
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return errors.New("SEARCH_REBUILD_TRANSACTION_UNAVAILABLE")
	}
	defer tx.Rollback()
	if _, found, err := generation.Find(ctx, tx, s.Alias, true); err != nil {
		return err
	} else if !found {
		return errors.New("SEARCH_GENERATION_MISSING")
	}
	input, err := s.lock(ctx, tx, operation)
	if err != nil {
		return err
	}
	if input.job.State == "SUCCEEDED" {
		return nil
	}
	if input.job.State != "SWITCHING" {
		return errors.New("SEARCH_REBUILD_PROGRESS_CONFLICT")
	}
	progress, err := repository.LoadValidation(ctx, tx, input.job.ID)
	if err != nil {
		return err
	}
	if !progress.Complete {
		return nil
	}
	state, err := c.Barrier.Indexes.InspectAliasSwitch(ctx, s.Alias, input.job.Source, input.job.Target)
	if err != nil {
		return err
	}
	if state == elastic.AliasAtSource {
		if err := c.Barrier.Indexes.SwitchAlias(ctx, s.Alias, input.job.Source, input.job.Target); err != nil {
			return err
		}
	} else if state != elastic.AliasAtTarget {
		return errors.New("SEARCH_ALIAS_SWITCH_CONFLICT")
	}
	if _, err := finalizeRebuild(ctx, tx, operation, input); err != nil {
		return err
	}
	if tx.Commit() != nil {
		return errors.New("SEARCH_REBUILD_COMMIT_UNKNOWN")
	}
	return nil
}

// jobs.go：保留原实现，按业务合并文件。

type JobService struct {
	DB                        *sql.DB
	Owner, Alias, AcceptedDir string
}

func (s *JobService) StartRebuild(ctx context.Context, request *pb.RebuildRequest) (*pb.OperationResult, error) {
	if request == nil || len(request.ProtoReflect().GetUnknown()) != 0 {
		return nil, errors.New("SEARCH_REBUILD_REQUEST_INVALID")
	}
	fingerprint, err := rebuildmeta.RequestFingerprint(request.OperationId, request.Generation, request.SnapshotRef, request.WatermarkJsonUtf8)
	if err != nil {
		return nil, err
	}
	return s.run(ctx, request.OperationId, fingerprint[:], request)
}

func (s *JobService) GetRebuild(ctx context.Context, query *pb.OperationQuery) (*pb.OperationResult, error) {
	if query == nil || !rebuildmeta.OperationValid(query.OperationId) || len(query.RequestFingerprint) != 32 || len(query.ProtoReflect().GetUnknown()) != 0 {
		return nil, errors.New("SEARCH_REBUILD_QUERY_INVALID")
	}
	// 重建没有文档级Target；即使已知字段全零，未知字段也不能悄悄改变查询语义。
	t := query.Target
	if t != nil && (t.DocumentId != 0 || t.UnitId != 0 || t.BranchId != 0 || t.StructureId != 0 || t.WorkspaceEpoch != 0 || t.Purpose != "" || len(t.ProtoReflect().GetUnknown()) != 0) {
		return nil, errors.New("SEARCH_REBUILD_QUERY_INVALID")
	}
	return s.run(ctx, query.OperationId, query.RequestFingerprint, nil)
}

func (s *JobService) run(ctx context.Context, operation string, fingerprint []byte, request *pb.RebuildRequest) (*pb.OperationResult, error) {
	if s == nil || s.DB == nil {
		return nil, errors.New("SEARCH_REBUILD_UNAVAILABLE")
	}
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, errors.New("SEARCH_REBUILD_TRANSACTION_UNAVAILABLE")
	}
	defer tx.Rollback()
	// 统一generation→binding→job锁序；原号恢复不要求当前代次仍是旧源代次。
	current, exists, err := generation.Find(ctx, tx, s.Alias, true)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.New("SEARCH_GENERATION_MISSING")
	}
	binding, found, err := repository.Find(ctx, tx, operation)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errors.New("SEARCH_REBUILD_OPERATION_NOT_FOUND")
	}
	p := binding.Prepare
	if p.Owner != s.Owner || p.Alias != s.Alias {
		return nil, errors.New("SEARCH_REBUILD_OPERATION_CONFLICT")
	}
	if binding.State != "IMPORTED" && binding.State != "ACCEPTED" {
		return nil, errors.New("SEARCH_REBUILD_NOT_IMPORTED")
	}
	imported, found, err := repository.FindImport(ctx, tx, operation)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errors.New("SEARCH_REBUILD_NOT_IMPORTED")
	}
	prepareDigest := sha256.Sum256(binding.Envelope)
	prepareRef := "rebuild-prepare-v1:" + hex.EncodeToString(prepareDigest[:])
	snapshotRef := "rebuild-snapshot-v1:" + hex.EncodeToString(imported.Digest[:])
	if imported.Seal.PrepareRef != prepareRef {
		return nil, errors.New("SEARCH_REBUILD_OPERATION_CONFLICT")
	}
	descriptor, err := rebuildmeta.DescriptorFor(p, prepareRef, snapshotRef)
	if err != nil {
		return nil, err
	}
	watermark, err := descriptor.Bytes()
	if err != nil {
		return nil, err
	}
	targetGeneration, _ := rebuildmeta.Decimal(p.TargetGeneration, true)
	expected, err := rebuildmeta.RequestFingerprint(operation, targetGeneration, snapshotRef, watermark)
	if err != nil || !bytes.Equal(expected[:], fingerprint) {
		return nil, errors.New("SEARCH_REBUILD_OPERATION_CONFLICT")
	}
	row, err := repository.LoadJobBinding(ctx, tx, operation)
	if err != nil {
		return nil, err
	}
	var job repository.Job
	if binding.State == "ACCEPTED" {
		id, parseErr := rebuildmeta.Decimal(row.JobID.String, true)
		if !bytes.Equal(row.Fingerprint, fingerprint) || !row.JobID.Valid || parseErr != nil {
			return nil, errors.New("SEARCH_REBUILD_OPERATION_CONFLICT")
		}
		job, err = repository.LoadJob(ctx, tx, id)
		if err != nil {
			return nil, err
		}
		// MySQL JSON会重排键；重新规范化后比较原descriptor，不对数据库文本验签。
		var persisted rebuildmeta.Descriptor
		if json.Unmarshal(job.Watermark, &persisted) != nil || persisted != descriptor || job.Generation != targetGeneration || job.Source != p.Source || job.Target != p.Target || job.SnapshotRef != snapshotRef {
			return nil, errors.New("SEARCH_REBUILD_JOB_INVALID")
		}
	} else {
		if request == nil {
			return nil, errors.New("SEARCH_REBUILD_NOT_ACCEPTED")
		}
		if current.Physical != p.Source || strconv.FormatUint(current.Generation, 10) != p.SourceGeneration || strconv.FormatUint(current.Revision, 10) != p.SourceRevision {
			return nil, errors.New("SEARCH_REBUILD_GENERATION_CHANGED")
		}
		if err := generation.CheckSwitching(ctx, tx); err != nil {
			return nil, err
		}
		// RPC仅核对已导入的受控副本可用；整库摘要由import检查，worker仍须流式核验。
		if !filepath.IsAbs(s.AcceptedDir) {
			return nil, errors.New("SEARCH_REBUILD_SNAPSHOT_UNAVAILABLE")
		}
		path := filepath.Join(s.AcceptedDir, strings.TrimPrefix(snapshotRef, "rebuild-snapshot-v1:")+".entries.ndjson")
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() {
			return nil, errors.New("SEARCH_REBUILD_SNAPSHOT_UNAVAILABLE")
		}
		job = repository.Job{State: "SNAPSHOT", SnapshotRef: snapshotRef, Watermark: watermark, Source: p.Source, Target: p.Target, Generation: targetGeneration}
		job.ID, err = repository.AcceptJob(ctx, tx, operation, fingerprint, job, p.Topic, p.Starts)
		if err != nil {
			return nil, err
		}
	}
	var result *pb.OperationResult
	if job.State == "SUCCEEDED" {
		result, err = loadTerminalResult(ctx, tx, operation, p, job, fingerprint)
	} else {
		result, err = jobResult(operation, job)
	}
	if err != nil {
		return nil, err
	}
	if tx.Commit() != nil {
		return nil, errors.New("SEARCH_REBUILD_COMMIT_UNKNOWN")
	}
	return result, nil
}

func jobResult(operation string, job repository.Job) (*pb.OperationResult, error) {
	switch job.State {
	case "SNAPSHOT", "REPLAYING", "VALIDATING", "SWITCHING":
	case "SUCCEEDED", "REJECTED":
		// 终态必须由worker持久证明封存后实现；仅状态字符串不足以制造固定成功结果。
		return nil, errors.New("SEARCH_REBUILD_TERMINAL_PROOF_UNAVAILABLE")
	default:
		return nil, errors.New("SEARCH_REBUILD_JOB_INVALID")
	}
	current, _ := json.Marshal(struct {
		Version       int    `json:"version"`
		JobID         string `json:"job_id"`
		State         string `json:"state"`
		SnapshotAfter string `json:"snapshot_after"`
	}{1, strconv.FormatUint(job.ID, 10), job.State, strconv.FormatUint(job.SnapshotAfter, 10)})
	return &pb.OperationResult{OperationId: operation, Generation: job.Generation, State: pb.OperationResult_PROCESSING, CurrentStateJsonUtf8: current}, nil
}

// prepare.go：保留原实现，按业务合并文件。

type Logs interface {
	Capture(context.Context, *logsource.Identity) (logsource.Window, error)
}
type Aliases interface {
	ResolveAlias(context.Context, string) (string, bool, error)
}
type Preparer struct {
	DB           *sql.DB
	Logs         Logs
	Aliases      Aliases
	Signer       *rebuildartifact.Signer
	LoadSigner   func() (*rebuildartifact.Signer, error)
	Verifier     *rebuildartifact.Verifier
	Owner, Alias string
}

// Prepare先封存Kafka原L再允许Doc建立S。代次锁只覆盖有界准备，不跨Doc的整库扫描。
func (p *Preparer) Prepare(ctx context.Context, operation string, target uint64) ([]byte, error) {
	if p.DB == nil || p.Logs == nil || p.Aliases == nil || !rebuildmeta.OperationValid(operation) || target == 0 {
		return nil, errors.New("SEARCH_REBUILD_PREPARE_INVALID")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	tx, err := p.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, errors.New("SEARCH_REBUILD_TRANSACTION_UNAVAILABLE")
	}
	defer tx.Rollback()
	source, exists, err := generation.Find(ctx, tx, p.Alias, true)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.New("SEARCH_GENERATION_MISSING")
	}
	original, found, err := repository.Find(ctx, tx, operation)
	if err != nil {
		return nil, err
	}
	if found {
		if original.Prepare.Owner != p.Owner || original.Prepare.Alias != p.Alias || original.Prepare.TargetGeneration != strconv.FormatUint(target, 10) {
			return nil, errors.New("SEARCH_REBUILD_OPERATION_CONFLICT")
		}
		if original.State == "REJECTED" {
			return nil, errors.New("SEARCH_REBUILD_PREPARE_REJECTED")
		}
		return original.Envelope, nil
	}
	if source.Generation == math.MaxUint64 || source.Revision == math.MaxUint64 || target != source.Generation+1 {
		return nil, errors.New("SEARCH_REBUILD_GENERATION_INVALID")
	}
	if err := generation.CheckSwitching(ctx, tx); err != nil {
		return nil, err
	}
	if err := repository.CheckAvailable(ctx, tx, p.Alias); err != nil {
		return nil, err
	}
	actual, present, err := p.Aliases.ResolveAlias(ctx, p.Alias)
	if err != nil {
		return nil, err
	}
	if !present || actual != source.Physical {
		return nil, errors.New("SEARCH_ALIAS_GENERATION_MISMATCH")
	}
	window, err := p.Logs.Capture(ctx, nil)
	if err != nil {
		return nil, err
	}
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, errors.New("SEARCH_REBUILD_NONCE_UNAVAILABLE")
	}
	name := sha256.Sum256([]byte(p.Owner + "\x00" + p.Alias + "\x00" + operation))
	input := rebuildmeta.Prepare{Version: 1, Operation: operation, Owner: p.Owner, Alias: p.Alias, Source: source.Physical, SourceGeneration: strconv.FormatUint(source.Generation, 10), SourceRevision: strconv.FormatUint(source.Revision, 10),
		Target: "rebuild-" + hex.EncodeToString(name[:]), TargetGeneration: strconv.FormatUint(target, 10), Cluster: window.Identity.Cluster, Topic: window.Identity.Topic, TopicID: hex.EncodeToString(window.Identity.TopicID[:]), Nonce: hex.EncodeToString(nonce[:])}
	for i, part := range window.Partitions {
		input.Starts[i] = strconv.FormatInt(part.Last, 10)
	}
	payload, err := input.Bytes()
	if err != nil {
		return nil, err
	}
	signer := p.Signer
	if signer == nil && p.LoadSigner != nil {
		signer, err = p.LoadSigner()
		if err != nil {
			return nil, err
		}
	}
	if signer == nil || p.Verifier == nil {
		return nil, errors.New("SEARCH_REBUILD_SIGNER_UNAVAILABLE")
	}
	envelope, err := signer.Sign(rebuildartifact.PrepareV1, payload)
	if err != nil {
		return nil, err
	}
	verified, err := p.Verifier.Verify(envelope, rebuildartifact.PrepareV1)
	if err != nil {
		return nil, err
	}
	if err := repository.Insert(ctx, tx, input, envelope, verified.SignerFingerprint); err != nil {
		return nil, err
	}
	if tx.Commit() != nil {
		return nil, errors.New("SEARCH_REBUILD_PREPARE_COMMIT_UNKNOWN")
	}
	return envelope, nil
}

// validate.go：保留原实现，按业务合并文件。

type ValidationIndexes interface {
	Get(context.Context, string, uint64) (index.Authority, error)
	RefreshCount(context.Context, string) (uint64, error)
}

type Validator struct {
	Seed    *Seeder
	Indexes ValidationIndexes
}

// Step先证明原三个分区已经到H，再按原快照覆盖和全部物理应用两阶段核验。
// 每页与原job锁共同提交；正文逐篇读取，不把整个索引加载进内存。
func (v *Validator) Step(ctx context.Context, operation string) error {
	if v == nil || v.Seed == nil || v.Seed.DB == nil || v.Indexes == nil {
		return errors.New("SEARCH_REBUILD_DEPENDENCIES_UNAVAILABLE")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	tx, err := v.Seed.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return errors.New("SEARCH_REBUILD_TRANSACTION_UNAVAILABLE")
	}
	defer tx.Rollback()
	input, err := v.Seed.lock(ctx, tx, operation)
	if err != nil {
		return err
	}
	if input.job.State != "SWITCHING" {
		return errors.New("SEARCH_REBUILD_PROGRESS_CONFLICT")
	}
	frontier, err := repository.LoadFrontier(ctx, tx, input.job.ID)
	if err != nil {
		return err
	}
	consumer := repository.ConsumerName(input.job.ID)
	for p, end := range frontier {
		var next int64
		if tx.QueryRowContext(ctx, `SELECT next_offset FROM consumer_checkpoint WHERE consumer_name=? AND kafka_topic=? AND partition_id=?`, consumer, input.prepare.Topic, p).Scan(&next) != nil {
			return errors.New("SEARCH_REBUILD_CHECKPOINT_MISSING")
		}
		if next < end {
			return nil
		}
		if next != end {
			return errors.New("SEARCH_REBUILD_FRONTIER_INVALID")
		}
	}
	progress, err := repository.LoadValidation(ctx, tx, input.job.ID)
	if err != nil || progress.Complete {
		return err
	}
	if progress.Phase == "SNAPSHOT" {
		page, err := snapshotPage(ctx, v.Seed.AcceptedDir, input.job.SnapshotRef, input.imported.Seal, progress.After)
		if err != nil {
			return err
		}
		for _, entry := range page {
			var applied uint64
			if tx.QueryRowContext(ctx, `SELECT applied_seq FROM index_application WHERE physical_index=? AND document_id=?`, input.job.Target, entry.Document).Scan(&applied) != nil || applied < entry.Minimum {
				return errors.New("SEARCH_REBUILD_SNAPSHOT_COVERAGE_FAILED")
			}
			progress.After = entry.Document
		}
		if len(page) == 0 {
			progress.Phase = "APPLICATION"
			progress.After = 0
		}
		if err := repository.SaveValidation(ctx, tx, input.job.ID, progress, false); err != nil {
			return err
		}
	} else {
		page, err := repository.ValidationApplications(ctx, tx, input.job.Target, progress.After)
		if err != nil {
			return err
		}
		count, _ := strconv.ParseUint(progress.Count, 10, 64)
		for _, application := range page {
			actual, err := v.Indexes.Get(ctx, input.job.Target, application.Document)
			if err != nil {
				return err
			}
			s := application.State
			if actual.Sequence != s.Sequence || actual.Digest != s.Digest || actual.Release != s.Release || actual.Visible != s.Visible || actual.Policy != s.Policy || s.Digest == ([32]byte{}) {
				return errors.New("SEARCH_REBUILD_TARGET_MISMATCH")
			}
			// Get已验证隐藏墓碑无正文；摘要链同时覆盖实际完整正文及逐文档SQL绑定。
			raw, err := json.Marshal(actual)
			if err != nil {
				return errors.New("SEARCH_REBUILD_VALIDATION_INVALID")
			}
			previous, _ := hex.DecodeString(progress.Digest)
			h := sha256.New()
			h.Write(previous)
			h.Write(raw)
			progress.Digest = hex.EncodeToString(h.Sum(nil))
			if count == ^uint64(0) {
				return errors.New("SEARCH_REBUILD_VALIDATION_INVALID")
			}
			count++
			progress.After = application.Document
		}
		progress.Count = strconv.FormatUint(count, 10)
		complete := len(page) == 0
		if complete {
			if err := event.CheckRecovery(ctx, tx, consumer, input.prepare.Topic); err != nil {
				return err
			}
			var pending uint64
			if tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM consumed_event e LEFT JOIN event_origin o ON o.consumer_name=e.consumer_name AND o.event_id=e.event_id WHERE e.consumer_name=? AND (e.state<>'SUCCEEDED' OR o.applied_index IS NULL OR o.applied_index<>?)`, consumer, input.job.Target).Scan(&pending) != nil || pending != 0 {
				return errors.New("SEARCH_REBUILD_LOG_COVERAGE_FAILED")
			}
			actualCount, err := v.Indexes.RefreshCount(ctx, input.job.Target)
			if err != nil {
				return err
			}
			if actualCount != count {
				return errors.New("SEARCH_REBUILD_TARGET_COUNT_MISMATCH")
			}
			// 空快照/空日志同样有非零封存证明，不能让全零摘要冒充未验证状态。
			if count == 0 {
				digest := sha256.Sum256([]byte("search-rebuild-empty-validation-v1"))
				progress.Digest = hex.EncodeToString(digest[:])
			}
		}
		if err := repository.SaveValidation(ctx, tx, input.job.ID, progress, complete); err != nil {
			return err
		}
	}
	if tx.Commit() != nil {
		return errors.New("SEARCH_REBUILD_COMMIT_UNKNOWN")
	}
	return nil
}
````

UTF-8/LF全文SHA-256：`f014b692bbf7b5c8040d7c44c0b2c74fb4c024639d09b462dd812db3897885fa`。[参考实现](../../../../services/search/internal/logic/rebuild/preparation_service.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
