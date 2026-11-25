package rebuild

import (
	"bytes"
	pb "collaboration/contracts/internalv1"
	"collaboration/platform"
	"collaboration/platform/rebuildartifact"
	"collaboration/platform/rebuildmeta"
	"collaboration/search/internal/client/elastic"
	generationlogic "collaboration/search/internal/logic/generation"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
)

// 复用既有alias未知场景的空快照/空日志模型和生产Prepare/Import/Start。
// SQL、ES与签名真实执行；本夹具只证明切换恢复，不代表再次遍历产品整库。
type switchBoundaryFixture struct {
	config   platform.Config
	ctx      context.Context
	db       *sql.DB
	indexes  *elastic.Client
	metadata switchProcessMetadata
}

// 只传递原任务公开绑定及受控副本目录，不传DSN、证书正文或私钥。
type switchProcessMetadata struct {
	Operation   string   `json:"operation"`
	Alias       string   `json:"alias"`
	Source      string   `json:"source"`
	Target      string   `json:"target"`
	Snapshot    string   `json:"snapshot"`
	JobID       uint64   `json:"job_id"`
	Fingerprint [32]byte `json:"fingerprint"`
	Accepted    string   `json:"accepted"`
	Until       int64    `json:"until"`
}

func switchBoundaryMust(t *testing.T, stage string, err error) {
	t.Helper()
	if err != nil {
		// 底层驱动原文可能含连接信息；阶段固定码足够定位本独立场景。
		t.Fatal(stage)
	}
}

func switchBoundaryConfig(t *testing.T) (platform.Config, string) {
	t.Helper()
	file := os.Getenv("COLLAB_TEST_SEARCH_CONFIG")
	if file == "" {
		t.Fatal("ACTUAL_SEARCH_CONFIG_REQUIRED")
	}
	cfg, err := platform.LoadConfig(file)
	if err != nil || cfg.Role != "search" || cfg.Search == nil {
		t.Fatal("ACTUAL_SEARCH_CONFIG_INVALID")
	}
	return cfg, file
}

func newSwitchBoundaryFixture(t *testing.T) *switchBoundaryFixture {
	t.Helper()
	cfg, file := switchBoundaryConfig(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	store, err := platform.OpenStore(cfg)
	switchBoundaryMust(t, "ACTUAL_SWITCH_STORE_UNAVAILABLE", err)
	t.Cleanup(func() { store.DB.Close() })
	indexes, err := elastic.New(cfg.Search)
	switchBoundaryMust(t, "ACTUAL_SWITCH_ES_UNAVAILABLE", err)
	t.Cleanup(func() { indexes.Close() })
	var pending int
	switchBoundaryMust(t, "ACTUAL_SWITCH_PENDING_READ_FAILED", store.DB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM rebuild_job WHERE state NOT IN ('SUCCEEDED','REJECTED')").Scan(&pending))
	if pending != 0 {
		t.Fatal("ACTUAL_REBUILD_REQUIRES_NO_PENDING_JOBS_AND_STOPPED_SEARCH")
	}
	doc, err := platform.LoadConfig(filepath.Join(filepath.Dir(file), "document.json"))
	if err != nil || doc.Role != "document" {
		t.Fatal("ACTUAL_DOCUMENT_SIGNER_CONFIG_INVALID")
	}
	signer, err := rebuildartifact.NewSigner(cfg.CertFile, cfg.KeyFile, cfg.CAFile, "search")
	switchBoundaryMust(t, "ACTUAL_SWITCH_SEARCH_SIGNER_INVALID", err)
	docSigner, err := rebuildartifact.NewSigner(doc.CertFile, doc.KeyFile, doc.CAFile, "document")
	switchBoundaryMust(t, "ACTUAL_SWITCH_DOC_SIGNER_INVALID", err)
	verifier, err := rebuildartifact.NewVerifier(cfg.CAFile)
	switchBoundaryMust(t, "ACTUAL_SWITCH_VERIFIER_INVALID", err)
	alias := fmt.Sprintf("actual-boundary-%d", time.Now().UnixNano())
	if alias == cfg.Search.Alias {
		t.Fatal("ACTUAL_SWITCH_ALIAS_NOT_ISOLATED")
	}
	guard, err := generationlogic.New(store.DB, indexes, alias, alias+"-g1")
	switchBoundaryMust(t, "ACTUAL_SWITCH_GUARD_INVALID", err)
	switchBoundaryMust(t, "ACTUAL_SWITCH_INITIALIZE_FAILED", guard.Initialize(ctx))
	prepare := &Preparer{DB: store.DB, Logs: recoveryEmptyLogs{}, Aliases: indexes, Signer: signer,
		Verifier: verifier, Owner: cfg.Search.Owner, Alias: alias}
	envelope, err := prepare.Prepare(ctx, alias, 2)
	switchBoundaryMust(t, "ACTUAL_SWITCH_PREPARE_FAILED", err)
	verified, err := verifier.Verify(envelope, rebuildartifact.PrepareV1)
	switchBoundaryMust(t, "ACTUAL_SWITCH_PREPARE_VERIFY_FAILED", err)
	original, err := rebuildmeta.ParsePrepare(verified.Payload)
	switchBoundaryMust(t, "ACTUAL_SWITCH_PREPARE_BINDING_INVALID", err)
	prepareRef, err := rebuildartifact.Reference(envelope, rebuildartifact.PrepareV1)
	switchBoundaryMust(t, "ACTUAL_SWITCH_PREPARE_REF_INVALID", err)
	empty := sha256.Sum256(nil)
	payload, err := (rebuildmeta.Seal{Version: 1, PrepareRef: prepareRef, Count: "0", EntriesDigest: hex.EncodeToString(empty[:])}).Bytes()
	switchBoundaryMust(t, "ACTUAL_SWITCH_SEAL_PAYLOAD_INVALID", err)
	sealed, err := docSigner.Sign(rebuildartifact.SnapshotV1, payload)
	switchBoundaryMust(t, "ACTUAL_SWITCH_SEAL_SIGN_FAILED", err)
	snapshotRef, err := rebuildartifact.Reference(sealed, rebuildartifact.SnapshotV1)
	switchBoundaryMust(t, "ACTUAL_SWITCH_SNAPSHOT_REF_INVALID", err)
	name, err := rebuildartifact.FileName(snapshotRef)
	switchBoundaryMust(t, "ACTUAL_SWITCH_FILE_REF_INVALID", err)
	inbox, accepted := t.TempDir(), t.TempDir()
	switchBoundaryMust(t, "ACTUAL_SWITCH_SEAL_WRITE_FAILED", os.WriteFile(filepath.Join(inbox, name), sealed, 0600))
	switchBoundaryMust(t, "ACTUAL_SWITCH_ENTRIES_WRITE_FAILED", os.WriteFile(filepath.Join(inbox, strings.TrimSuffix(name, ".snapshot.json")+".entries.ndjson"), nil, 0600))
	importer := &Importer{DB: store.DB, Verifier: verifier, Owner: cfg.Search.Owner, Alias: alias, Inbox: inbox, Accepted: accepted}
	imported, err := importer.Import(ctx, alias, snapshotRef)
	switchBoundaryMust(t, "ACTUAL_SWITCH_IMPORT_FAILED", err)
	if !bytes.Equal(imported, sealed) {
		t.Fatal("ACTUAL_SWITCH_ORIGINAL_SEAL_CHANGED")
	}
	descriptor, err := rebuildmeta.DescriptorFor(original, prepareRef, snapshotRef)
	switchBoundaryMust(t, "ACTUAL_SWITCH_DESCRIPTOR_INVALID", err)
	watermark, err := descriptor.Bytes()
	switchBoundaryMust(t, "ACTUAL_SWITCH_WATERMARK_INVALID", err)
	fingerprint, err := rebuildmeta.RequestFingerprint(alias, 2, snapshotRef, watermark)
	switchBoundaryMust(t, "ACTUAL_SWITCH_FINGERPRINT_INVALID", err)
	jobs := &JobService{DB: store.DB, Owner: cfg.Search.Owner, Alias: alias, AcceptedDir: accepted}
	started, err := jobs.StartRebuild(ctx, &pb.RebuildRequest{OperationId: alias, Generation: 2, SnapshotRef: snapshotRef, WatermarkJsonUtf8: watermark})
	switchBoundaryMust(t, "ACTUAL_SWITCH_START_FAILED", err)
	if started == nil || started.State != pb.OperationResult_PROCESSING {
		t.Fatal("ACTUAL_SWITCH_ORIGINAL_JOB_NOT_ACCEPTED")
	}
	var state struct {
		JobID string `json:"job_id"`
	}
	switchBoundaryMust(t, "ACTUAL_SWITCH_START_STATE_INVALID", json.Unmarshal(started.CurrentStateJsonUtf8, &state))
	id, err := strconv.ParseUint(state.JobID, 10, 64)
	if err != nil || id == 0 {
		t.Fatal("ACTUAL_SWITCH_JOB_ID_INVALID")
	}
	until, _ := ctx.Deadline()
	f := &switchBoundaryFixture{config: cfg, ctx: ctx, db: store.DB, indexes: indexes,
		metadata: switchProcessMetadata{alias, alias, original.Source, original.Target, snapshotRef, id, fingerprint, accepted, until.UnixMilli()}}
	// 失败也只恢复本原号；保留全部数据库/索引事实，不删原receipt或回退alias。
	t.Cleanup(func() { f.restoreOwnJob(t) })
	coordinator := f.coordinator(store.DB, indexes)
	switchBoundaryMust(t, "ACTUAL_SWITCH_EMPTY_SEED_FAILED", coordinator.Barrier.Seed.Step(ctx))
	switchBoundaryMust(t, "ACTUAL_SWITCH_BARRIER_FAILED", coordinator.Barrier.Seal(ctx, alias))
	for i := 0; i < 3; i++ {
		switchBoundaryMust(t, "ACTUAL_SWITCH_VALIDATION_FAILED", coordinator.Validator.Step(ctx, alias))
	}
	f.assertSQL(t, store.DB, "SWITCHING", original.Source, 1, 1)
	return f
}

func switchBoundaryCoordinator(db *sql.DB, cfg platform.Config, m switchProcessMetadata, indexes *elastic.Client, switches SwitchIndexes) *Coordinator {
	seed := &Seeder{DB: db, Owner: cfg.Search.Owner, Alias: m.Alias, AcceptedDir: m.Accepted,
		Writer: recoveryNoSeed{}, Indexes: indexes, Logs: recoveryEmptyLogs{}}
	return &Coordinator{Barrier: &Barrier{Seed: seed, Indexes: switches}, Validator: &Validator{Seed: seed, Indexes: indexes}}
}

func (f *switchBoundaryFixture) coordinator(db *sql.DB, switches SwitchIndexes) *Coordinator {
	return switchBoundaryCoordinator(db, f.config, f.metadata, f.indexes, switches)
}

func (f *switchBoundaryFixture) assertSQL(t *testing.T, db *sql.DB, wantState, wantPhysical string, wantGeneration, wantRevision uint64) {
	t.Helper()
	var physical, state, operation string
	var generation, revision, id uint64
	var fingerprint []byte
	err := db.QueryRowContext(f.ctx, `SELECT g.physical_index,g.generation,g.revision,j.id,j.state,b.operation_id,b.request_fingerprint
		FROM index_generation g JOIN rebuild_binding b ON b.alias_name=g.alias_name JOIN rebuild_job j ON j.id=b.job_id
		WHERE g.alias_name=? AND b.operation_id=? AND j.id=?`, f.metadata.Alias, f.metadata.Operation, f.metadata.JobID).
		Scan(&physical, &generation, &revision, &id, &state, &operation, &fingerprint)
	switchBoundaryMust(t, "ACTUAL_SWITCH_PERSISTED_FACTS_UNAVAILABLE", err)
	if physical != wantPhysical || state != wantState || generation != wantGeneration || revision != wantRevision || id != f.metadata.JobID || operation != f.metadata.Operation || !bytes.Equal(fingerprint, f.metadata.Fingerprint[:]) {
		t.Fatal("ACTUAL_SWITCH_ORIGINAL_SQL_BINDING_CHANGED")
	}
}

func (f *switchBoundaryFixture) result(t *testing.T, db *sql.DB) *pb.OperationResult {
	t.Helper()
	jobs := &JobService{DB: db, Owner: f.config.Search.Owner, Alias: f.metadata.Alias, AcceptedDir: f.metadata.Accepted}
	result, err := jobs.GetRebuild(f.ctx, &pb.OperationQuery{OperationId: f.metadata.Operation, RequestFingerprint: f.metadata.Fingerprint[:]})
	switchBoundaryMust(t, "ACTUAL_SWITCH_GET_FAILED", err)
	if result == nil || result.State != pb.OperationResult_SUCCEEDED || result.OperationId != f.metadata.Operation || result.Generation != 2 || result.TerminalRevision != 2 || len(result.ResultIds) != 1 || result.ResultIds[0] != f.metadata.JobID {
		t.Fatal("ACTUAL_SWITCH_ORIGINAL_RESULT_INVALID")
	}
	var proof terminalProof
	switchBoundaryMust(t, "ACTUAL_SWITCH_FIXED_PROOF_INVALID", json.Unmarshal(result.FixedResultJsonUtf8, &proof))
	digest := sha256.Sum256(result.FixedResultJsonUtf8)
	validation := sha256.Sum256([]byte("search-rebuild-empty-validation-v1"))
	if proof.Version != 1 || proof.State != "SUCCEEDED" || proof.Operation != f.metadata.Operation || proof.JobID != strconv.FormatUint(f.metadata.JobID, 10) || proof.Generation != "2" || proof.Revision != "2" || proof.Alias != f.metadata.Alias || proof.Source != f.metadata.Source || proof.Target != f.metadata.Target || proof.Snapshot != f.metadata.Snapshot || proof.Fingerprint != hex.EncodeToString(f.metadata.Fingerprint[:]) || proof.Ends != ([3]string{"0", "0", "0"}) || proof.Validation != hex.EncodeToString(validation[:]) || !bytes.Equal(digest[:], result.ResultDigest) {
		t.Fatal("ACTUAL_SWITCH_FIXED_PROOF_BINDING_CHANGED")
	}
	return result
}

func (f *switchBoundaryFixture) assertStable(t *testing.T, db *sql.DB, result *pb.OperationResult) {
	t.Helper()
	if !proto.Equal(result, f.result(t, db)) {
		t.Fatal("ACTUAL_SWITCH_REPEATED_GET_CHANGED_FIXED_RESULT")
	}
	f.assertSQL(t, db, "SUCCEEDED", f.metadata.Target, 2, 2)
	actual, present, err := f.indexes.ResolveAlias(f.ctx, f.metadata.Alias)
	switchBoundaryMust(t, "ACTUAL_SWITCH_ALIAS_READ_FAILED", err)
	if !present || actual != f.metadata.Target {
		t.Fatal("ACTUAL_SWITCH_TARGET_ALIAS_NOT_RETAINED")
	}
	t.Logf("retained evidence: operation=%s job=%d alias=%s source=%s target=%s generation=2 revision=2 proof_sha256=%x", f.metadata.Operation, f.metadata.JobID, f.metadata.Alias, f.metadata.Source, f.metadata.Target, sha256.Sum256(result.FixedResultJsonUtf8))
}

func (f *switchBoundaryFixture) restoreOwnJob(t *testing.T) {
	// 原场景结束后仅做有界恢复；不延长任何产品调用或替失败改为成功。
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	store, err := platform.OpenStore(f.config)
	if err != nil {
		t.Error("ACTUAL_SWITCH_CLEANUP_POOL_UNAVAILABLE")
		return
	}
	defer store.DB.Close()
	var state string
	if store.DB.QueryRowContext(ctx, "SELECT state FROM rebuild_job WHERE id=?", f.metadata.JobID).Scan(&state) != nil {
		t.Error("ACTUAL_SWITCH_CLEANUP_STATE_UNAVAILABLE")
		return
	}
	if state == "SUCCEEDED" {
		return
	}
	c := f.coordinator(store.DB, f.indexes)
	if state == "SNAPSHOT" {
		err = c.Barrier.Seed.Step(ctx)
	}
	if err == nil && (state == "SNAPSHOT" || state == "REPLAYING") {
		err = c.Barrier.Seal(ctx, f.metadata.Operation)
	}
	for i := 0; i < 3 && err == nil; i++ {
		err = c.Validator.Step(ctx, f.metadata.Operation)
	}
	if err == nil {
		err = c.finish(ctx, f.metadata.Operation)
	}
	if err != nil {
		t.Error("ACTUAL_SWITCH_OWN_JOB_CLEANUP_REMAINS_UNKNOWN")
	}
}
