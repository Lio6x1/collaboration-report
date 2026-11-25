package rebuild

import (
	"bytes"
	pb "collaboration/contracts/internalv1"
	"collaboration/platform"
	"collaboration/platform/rebuildartifact"
	"collaboration/platform/rebuildmeta"
	"collaboration/search/internal/client/elastic"
	"collaboration/search/internal/client/logsource"
	generationlogic "collaboration/search/internal/logic/generation"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"google.golang.org/protobuf/proto"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// 固定空日志和空快照是本场景的显式fixture，不代表Doc整库快照或Kafka重放验收。
// 它使反例只落在真实ES已切换、真实SQL尚未完成的跨系统未知结果边界。
type recoveryEmptyLogs struct{}

func (recoveryEmptyLogs) Capture(_ context.Context, expected *logsource.Identity) (logsource.Window, error) {
	identity := logsource.Identity{Cluster: "actual-recovery-fixture", Topic: "actual-recovery-fixture", TopicID: [16]byte{1}}
	if expected != nil && *expected != identity {
		return logsource.Window{}, errors.New("FIXTURE_LOG_IDENTITY_CHANGED")
	}
	return logsource.Window{Identity: identity}, nil
}

type recoveryNoSeed struct{}

func (recoveryNoSeed) SeedInTransaction(context.Context, *sql.Tx, string, rebuildmeta.Entry) error {
	return errors.New("EMPTY_FIXTURE_MUST_NOT_SEED_DOCUMENT")
}

type recoveryLostAck struct {
	SwitchIndexes
	injected bool
}

func (s *recoveryLostAck) SwitchAlias(ctx context.Context, alias, source, target string) error {
	if err := s.SwitchIndexes.SwitchAlias(ctx, alias, source, target); err != nil {
		return err
	}
	if !s.injected {
		s.injected = true
		return errors.New("SEARCH_ALIAS_SWITCH_UNKNOWN")
	}
	return nil
}

func TestActualRebuildAliasUnknownRecoversOriginalJob(t *testing.T) {
	file := os.Getenv("COLLAB_TEST_SEARCH_CONFIG")
	if file == "" {
		t.Fatal("ACTUAL_SEARCH_CONFIG_REQUIRED")
	}
	cfg, err := platform.LoadConfig(file)
	if err != nil || cfg.Search == nil || cfg.Role != "search" {
		t.Fatal("ACTUAL_SEARCH_CONFIG_INVALID")
	}
	// 仅测试fixture签名读取同目录Doc配置；绝不连接Doc私有库。
	doc, err := platform.LoadConfig(filepath.Join(filepath.Dir(file), "document.json"))
	if err != nil || doc.Role != "document" {
		t.Fatal("ACTUAL_DOCUMENT_SIGNER_CONFIG_INVALID")
	}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	signer, err := rebuildartifact.NewSigner(cfg.CertFile, cfg.KeyFile, cfg.CAFile, "search")
	must(err)
	docSigner, err := rebuildartifact.NewSigner(doc.CertFile, doc.KeyFile, doc.CAFile, "document")
	must(err)
	verifier, err := rebuildartifact.NewVerifier(cfg.CAFile)
	must(err)
	store, err := platform.OpenStore(cfg)
	must(err)
	defer store.DB.Close()
	indexes, err := elastic.New(cfg.Search)
	must(err)
	defer indexes.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	// PendingSnapshot是全局选择器：主控须先停止Search，且环境不能有其他未完成任务。
	var pending int
	must(store.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM rebuild_job WHERE state NOT IN ('SUCCEEDED','REJECTED')`).Scan(&pending))
	if pending != 0 {
		t.Fatal("ACTUAL_REBUILD_REQUIRES_NO_PENDING_JOBS")
	}
	alias := fmt.Sprintf("actual-recovery-%d", time.Now().UnixNano())
	source, operation := alias+"-g1", alias
	if alias == cfg.Search.Alias {
		t.Fatal("ACTUAL_RECOVERY_ALIAS_MUST_BE_INDEPENDENT")
	}
	guard, err := generationlogic.New(store.DB, indexes, alias, source)
	must(err)
	must(guard.Initialize(ctx))
	preparer := &Preparer{DB: store.DB, Logs: recoveryEmptyLogs{}, Aliases: indexes, Signer: signer, Verifier: verifier, Owner: cfg.Search.Owner, Alias: alias}
	prepared, err := preparer.Prepare(ctx, operation, 2)
	must(err)
	verified, err := verifier.Verify(prepared, rebuildartifact.PrepareV1)
	must(err)
	original, err := rebuildmeta.ParsePrepare(verified.Payload)
	must(err)
	prepareRef, err := rebuildartifact.Reference(prepared, rebuildartifact.PrepareV1)
	must(err)
	emptyDigest := sha256.Sum256(nil)
	payload, err := (rebuildmeta.Seal{Version: 1, PrepareRef: prepareRef, Count: "0", EntriesDigest: hex.EncodeToString(emptyDigest[:])}).Bytes()
	must(err)
	sealed, err := docSigner.Sign(rebuildartifact.SnapshotV1, payload)
	must(err)
	snapshotRef, err := rebuildartifact.Reference(sealed, rebuildartifact.SnapshotV1)
	must(err)
	name, err := rebuildartifact.FileName(snapshotRef)
	must(err)
	inbox, accepted := t.TempDir(), t.TempDir()
	must(os.WriteFile(filepath.Join(inbox, name), sealed, 0600))
	must(os.WriteFile(filepath.Join(inbox, strings.TrimSuffix(name, ".snapshot.json")+".entries.ndjson"), nil, 0600))
	importer := &Importer{DB: store.DB, Verifier: verifier, Owner: cfg.Search.Owner, Alias: alias, Inbox: inbox, Accepted: accepted}
	imported, err := importer.Import(ctx, operation, snapshotRef)
	must(err)
	if !bytes.Equal(imported, sealed) {
		t.Fatal("ORIGINAL_SEAL_CHANGED")
	}
	descriptor, err := rebuildmeta.DescriptorFor(original, prepareRef, snapshotRef)
	must(err)
	watermark, err := descriptor.Bytes()
	must(err)
	jobs := &JobService{DB: store.DB, Owner: cfg.Search.Owner, Alias: alias, AcceptedDir: accepted}
	request := &pb.RebuildRequest{OperationId: operation, Generation: 2, SnapshotRef: snapshotRef, WatermarkJsonUtf8: watermark}
	started, err := jobs.StartRebuild(ctx, request)
	must(err)
	if started.State != pb.OperationResult_PROCESSING {
		t.Fatal("ORIGINAL_JOB_NOT_ACCEPTED")
	}
	var acceptedState struct {
		JobID string `json:"job_id"`
	}
	must(json.Unmarshal(started.CurrentStateJsonUtf8, &acceptedState))
	jobID, err := strconv.ParseUint(acceptedState.JobID, 10, 64)
	must(err)
	if jobID == 0 {
		t.Fatal("ORIGINAL_JOB_ID_MISSING")
	}
	fingerprint, err := rebuildmeta.RequestFingerprint(operation, 2, snapshotRef, watermark)
	must(err)
	newCoordinator := func(switches SwitchIndexes) *Coordinator {
		seed := &Seeder{DB: store.DB, Owner: cfg.Search.Owner, Alias: alias, AcceptedDir: accepted, Writer: recoveryNoSeed{}, Indexes: indexes, Logs: recoveryEmptyLogs{}}
		return &Coordinator{Barrier: &Barrier{Seed: seed, Indexes: switches}, Validator: &Validator{Seed: seed, Indexes: indexes}}
	}
	coordinator := newCoordinator(indexes)
	// 失败也只恢复本任务；不DELETE无权限的绑定，不撤销已完成的真实alias切换。
	defer func() {
		cleanup, done := context.WithTimeout(context.Background(), 30*time.Second)
		defer done()
		var state string
		if err := store.DB.QueryRowContext(cleanup, `SELECT state FROM rebuild_job WHERE id=?`, jobID).Scan(&state); err != nil {
			t.Error("RECOVERY_CLEANUP_STATE_UNAVAILABLE")
			return
		}
		if state == "SWITCHING" {
			fresh := newCoordinator(indexes)
			for i := 0; i < 3; i++ {
				if err := fresh.Validator.Step(cleanup, operation); err != nil {
					t.Errorf("RECOVERY_CLEANUP_VALIDATE: %v", err)
					return
				}
			}
			if err := fresh.finish(cleanup, operation); err != nil {
				t.Errorf("RECOVERY_CLEANUP_SWITCHING_REMAINS: %v", err)
			}
		}
	}()
	must(coordinator.Barrier.Seed.Step(ctx))
	must(coordinator.Barrier.Seal(ctx, operation))
	for i := 0; i < 3; i++ {
		must(coordinator.Validator.Step(ctx, operation))
	}
	lost := &recoveryLostAck{SwitchIndexes: indexes}
	coordinator.Barrier.Indexes = lost
	err = coordinator.finish(ctx, operation)
	if err == nil || err.Error() != "SEARCH_ALIAS_SWITCH_UNKNOWN" || !lost.injected {
		t.Fatalf("EXPECTED_ACTUAL_ALIAS_ACK_UNKNOWN: %v", err)
	}
	actual, present, err := indexes.ResolveAlias(ctx, alias)
	must(err)
	if !present || actual != original.Target {
		t.Fatal("ACTUAL_ALIAS_DID_NOT_SWITCH")
	}
	var physical, state string
	var generation, revision uint64
	must(store.DB.QueryRowContext(ctx, `SELECT physical_index,generation,revision FROM index_generation WHERE alias_name=?`, alias).Scan(&physical, &generation, &revision))
	must(store.DB.QueryRowContext(ctx, `SELECT state FROM rebuild_job WHERE id=?`, jobID).Scan(&state))
	if physical != source || generation != 1 || revision != 1 || state != "SWITCHING" {
		t.Fatal("UNKNOWN_ACK_MUST_KEEP_ORIGINAL_SQL_BARRIER")
	}
	// 丢弃原协调器及注入器，模拟进程内状态全部丢失；恢复仅依赖真实SQL/ES。
	fresh := newCoordinator(indexes)
	must(fresh.finish(ctx, operation))
	query := &pb.OperationQuery{OperationId: operation, RequestFingerprint: fingerprint[:]}
	result, err := jobs.GetRebuild(ctx, query)
	must(err)
	if result.State != pb.OperationResult_SUCCEEDED || result.Generation != 2 || result.TerminalRevision != 2 || len(result.ResultIds) != 1 || result.ResultIds[0] != jobID {
		t.Fatal("RECOVERY_DID_NOT_FINISH_ORIGINAL_JOB_EXACTLY_ONCE")
	}
	var proof terminalProof
	must(json.Unmarshal(result.FixedResultJsonUtf8, &proof))
	digest := sha256.Sum256(result.FixedResultJsonUtf8)
	validation := sha256.Sum256([]byte("search-rebuild-empty-validation-v1"))
	if proof.Operation != operation || proof.JobID != fmt.Sprint(jobID) || proof.Generation != "2" || proof.Revision != "2" || proof.Alias != alias || proof.Source != source || proof.Target != original.Target || proof.Snapshot != snapshotRef || proof.Fingerprint != hex.EncodeToString(fingerprint[:]) || proof.Ends != ([3]string{"0", "0", "0"}) || proof.Validation != hex.EncodeToString(validation[:]) || !bytes.Equal(digest[:], result.ResultDigest) {
		t.Fatal("ORIGINAL_FIXED_PROOF_INVALID")
	}
	must(fresh.finish(ctx, operation))
	again, err := jobs.GetRebuild(ctx, query)
	must(err)
	if !proto.Equal(result, again) {
		t.Fatal("REPEATED_GET_CHANGED_FIXED_RESULT")
	}
	must(store.DB.QueryRowContext(ctx, `SELECT physical_index,generation,revision FROM index_generation WHERE alias_name=?`, alias).Scan(&physical, &generation, &revision))
	if physical != original.Target || generation != 2 || revision != 2 {
		t.Fatal("RECOVERY_INCREMENTED_GENERATION_MORE_THAN_ONCE")
	}
	t.Logf("retained evidence: operation=%s job=%d alias=%s source=%s target=%s generation=2 revision=2", operation, jobID, alias, source, original.Target)
}
