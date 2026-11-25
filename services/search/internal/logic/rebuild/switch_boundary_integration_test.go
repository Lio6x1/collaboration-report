package rebuild

import (
	"bytes"
	"collaboration/platform"
	"collaboration/search/internal/client/elastic"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
)

func TestActualRebuildTerminalCommitReceiptUnknownRetainsOriginalProof(t *testing.T) {
	f := newSwitchBoundaryFixture(t)
	fault := &switchCommitFault{metadata: f.metadata}
	faultDB := switchCommitDatabase(t, f.config, fault)
	coordinator := f.coordinator(faultDB, f.indexes)
	err := coordinator.finish(f.ctx, f.metadata.Operation)
	if err == nil || err.Error() != "SEARCH_REBUILD_COMMIT_UNKNOWN" {
		t.Fatal("ACTUAL_TERMINAL_COMMIT_UNKNOWN_NOT_RETURNED")
	}
	lost, cursor := fault.evidence()
	if lost != 1 || len(cursor) == 0 {
		t.Fatal("ACTUAL_TERMINAL_EXACT_COMMIT_FAULT_NOT_OBSERVED")
	}
	// 与注入连接独立的真实SQL读取证明：返回未知时CAS/终态已一起提交。
	f.assertSQL(t, f.db, "SUCCEEDED", f.metadata.Target, 2, 2)
	confirmed := f.result(t, f.db)
	var envelope terminalEnvelope
	switchBoundaryMust(t, "ACTUAL_TERMINAL_INJECTED_CURSOR_INVALID", json.Unmarshal(cursor, &envelope))
	if envelope.Version != 1 || !bytes.Equal(envelope.Fixed, confirmed.FixedResultJsonUtf8) {
		t.Fatal("ACTUAL_TERMINAL_LOST_RECEIPT_DID_NOT_BIND_ORIGINAL_FIXED_PROOF")
	}
	// 关闭故障池及原核验池，丢弃原协调器；新pool重新读取原job，不能沿内存答复成功。
	switchBoundaryMust(t, "ACTUAL_TERMINAL_FAULT_POOL_CLOSE_FAILED", faultDB.Close())
	switchBoundaryMust(t, "ACTUAL_TERMINAL_ORIGINAL_POOL_CLOSE_FAILED", f.db.Close())
	coordinator = nil
	store, err := platform.OpenStore(f.config)
	switchBoundaryMust(t, "ACTUAL_TERMINAL_FRESH_POOL_UNAVAILABLE", err)
	defer store.DB.Close()
	fresh := f.coordinator(store.DB, f.indexes)
	switchBoundaryMust(t, "ACTUAL_TERMINAL_ORIGINAL_JOB_RECOVERY_FAILED", fresh.finish(f.ctx, f.metadata.Operation))
	f.assertStable(t, store.DB, confirmed)
	switchBoundaryMust(t, "ACTUAL_TERMINAL_IDEMPOTENT_RECOVERY_FAILED", fresh.finish(f.ctx, f.metadata.Operation))
	f.assertStable(t, store.DB, confirmed)
	if after, _ := fault.evidence(); after != 1 {
		t.Fatal("ACTUAL_TERMINAL_RECEIPT_FAULT_REPEATED")
	}
	t.Log("real MySQL delegate Commit succeeded; exactly one original terminal transaction receipt was discarded; fresh pool retained the fixed result; no TCP disconnect was injected")
}

type switchProcessEvidence struct {
	PID       int    `json:"pid"`
	Operation string `json:"operation"`
	Alias     string `json:"alias"`
	Target    string `json:"target"`
	Stage     string `json:"stage"`
}

const switchProcessExitCode = 83

// os.Exit发生在生产finish调用真实SwitchAlias之后、finalizeRebuild之前。
// 它终止整个测试子进程（不执行Go defer），保留ES目标及SQL原SWITCHING；
// 父进程等待真实退出并再次读库，不能把换协调器指针称为进程故障。
type switchExitIndexes struct {
	SwitchIndexes
	fixture   *switchBoundaryFixture
	proofFile string
}

func (s *switchExitIndexes) SwitchAlias(ctx context.Context, alias, source, target string) error {
	m := s.fixture.metadata
	if alias != m.Alias || source != m.Source || target != m.Target {
		return errors.New("ACTUAL_PROCESS_SWITCH_BINDING_INVALID")
	}
	if err := s.SwitchIndexes.SwitchAlias(ctx, alias, source, target); err != nil {
		return err
	}
	actual, present, err := s.fixture.indexes.ResolveAlias(ctx, alias)
	if err != nil || !present || actual != target {
		return errors.New("ACTUAL_PROCESS_ALIAS_TARGET_UNKNOWN")
	}
	var physical, state string
	var generation, revision uint64
	err = s.fixture.db.QueryRowContext(ctx, `SELECT g.physical_index,g.generation,g.revision,j.state
		FROM index_generation g JOIN rebuild_binding b ON b.alias_name=g.alias_name JOIN rebuild_job j ON j.id=b.job_id
		WHERE g.alias_name=? AND b.operation_id=? AND j.id=?`, alias, m.Operation, m.JobID).
		Scan(&physical, &generation, &revision, &state)
	if err != nil || physical != source || generation != 1 || revision != 1 || state != "SWITCHING" {
		return errors.New("ACTUAL_PROCESS_EXIT_WINDOW_NOT_PROVEN")
	}
	raw, err := json.Marshal(switchProcessEvidence{os.Getpid(), m.Operation, alias, target, "ALIAS_TARGET_SQL_SWITCHING"})
	if err != nil || os.WriteFile(s.proofFile, raw, 0600) != nil {
		return errors.New("ACTUAL_PROCESS_EXIT_EVIDENCE_UNAVAILABLE")
	}
	os.Exit(switchProcessExitCode)
	return errors.New("ACTUAL_PROCESS_DID_NOT_EXIT")
}

func TestActualRebuildProcessExitAfterAliasRecoversSameJob(t *testing.T) {
	f := newSwitchBoundaryFixture(t)
	file := filepath.Join(t.TempDir(), "original-job.json")
	raw, err := json.Marshal(f.metadata)
	switchBoundaryMust(t, "ACTUAL_PROCESS_METADATA_INVALID", err)
	switchBoundaryMust(t, "ACTUAL_PROCESS_METADATA_WRITE_FAILED", os.WriteFile(file, raw, 0600))
	// 两个独立OS进程只读同一原metadata。recover分支绝不再Prepare/Import/Start。
	exitPID, err := runSwitchBoundaryProcess(t, f.ctx, file, "exit")
	var exited *exec.ExitError
	if !errors.As(err, &exited) || exited.ExitCode() != switchProcessExitCode || exitPID == 0 || exitPID == os.Getpid() {
		t.Fatal("ACTUAL_SWITCH_CHILD_DID_NOT_EXIT_AT_EXACT_BOUNDARY")
	}
	exitProof := readSwitchProcessEvidence(t, file+".exit.json")
	if exitProof.PID != exitPID || exitProof.Operation != f.metadata.Operation || exitProof.Alias != f.metadata.Alias || exitProof.Target != f.metadata.Target || exitProof.Stage != "ALIAS_TARGET_SQL_SWITCHING" {
		t.Fatal("ACTUAL_SWITCH_CHILD_EXIT_PROOF_MISMATCH")
	}
	f.assertSQL(t, f.db, "SWITCHING", f.metadata.Source, 1, 1)
	actual, present, err := f.indexes.ResolveAlias(f.ctx, f.metadata.Alias)
	switchBoundaryMust(t, "ACTUAL_SWITCH_EXIT_ALIAS_READ_FAILED", err)
	if !present || actual != f.metadata.Target {
		t.Fatal("ACTUAL_SWITCH_EXIT_LOST_ES_TARGET")
	}
	// 新进程执行生产Coordinator.Step恢复原SWITCHING，另有原号finish/Get幂等核验。
	recoveryPID, err := runSwitchBoundaryProcess(t, f.ctx, file, "recover")
	switchBoundaryMust(t, "ACTUAL_SWITCH_FRESH_PROCESS_RECOVERY_FAILED", err)
	proof := readSwitchProcessEvidence(t, file+".recovered.json")
	if proof.PID != recoveryPID || recoveryPID == 0 || recoveryPID == exitPID || recoveryPID == os.Getpid() || proof.Operation != f.metadata.Operation || proof.Alias != f.metadata.Alias || proof.Target != f.metadata.Target || proof.Stage != "ORIGINAL_JOB_SUCCEEDED" {
		t.Fatal("ACTUAL_SWITCH_RECOVERY_PROCESS_NOT_INDEPENDENT")
	}
	result := f.result(t, f.db)
	childResult, err := os.ReadFile(file + ".result.pb")
	switchBoundaryMust(t, "ACTUAL_SWITCH_CHILD_RESULT_UNAVAILABLE", err)
	encoded, err := proto.MarshalOptions{Deterministic: true}.Marshal(result)
	switchBoundaryMust(t, "ACTUAL_SWITCH_RESULT_ENCODING_FAILED", err)
	if !bytes.Equal(childResult, encoded) {
		t.Fatal("ACTUAL_SWITCH_CHILD_AND_PARENT_FIXED_RESULTS_DIFFER")
	}
	f.assertStable(t, f.db, result)
	t.Logf("actual process boundary: exited_pid=%d exit_code=%d recovery_pid=%d parent_pid=%d original_job=%d; real SQL/ES retained", exitPID, switchProcessExitCode, recoveryPID, os.Getpid(), f.metadata.JobID)
}

func runSwitchBoundaryProcess(t *testing.T, ctx context.Context, file, phase string) (int, error) {
	t.Helper()
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestRebuildSwitchBoundaryProcessChild$", "-test.timeout=90s", "-test.v")
	// 覆盖而非重复添加变量，避免父环境留下旧phase导致新子进程运行错误分支。
	for _, value := range os.Environ() {
		key, _, _ := strings.Cut(value, "=")
		if !strings.EqualFold(key, "COLLAB_REBUILD_CHILD_PHASE") && !strings.EqualFold(key, "COLLAB_REBUILD_CHILD_METADATA") {
			command.Env = append(command.Env, value)
		}
	}
	command.Env = append(command.Env, "COLLAB_REBUILD_CHILD_PHASE="+phase, "COLLAB_REBUILD_CHILD_METADATA="+file)
	var output bytes.Buffer
	command.Stdout, command.Stderr = &output, &output
	if err := command.Start(); err != nil {
		return 0, err
	}
	pid := command.Process.Pid
	err := command.Wait()
	// 子进程只输出固定测试码及无秘密的原绑定日志；不打印环境、DSN或metadata正文。
	if output.Len() != 0 {
		t.Logf("child phase=%s pid=%d\n%s", phase, pid, output.String())
	}
	return pid, err
}

func readSwitchProcessEvidence(t *testing.T, file string) switchProcessEvidence {
	t.Helper()
	raw, err := os.ReadFile(file)
	switchBoundaryMust(t, "ACTUAL_SWITCH_PROCESS_EVIDENCE_UNAVAILABLE", err)
	var proof switchProcessEvidence
	switchBoundaryMust(t, "ACTUAL_SWITCH_PROCESS_EVIDENCE_INVALID", json.Unmarshal(raw, &proof))
	return proof
}

// 仅exec专用入口，父进程不会在这里创建夹具或连接真实环境。
func TestRebuildSwitchBoundaryProcessChild(t *testing.T) {
	phase := os.Getenv("COLLAB_REBUILD_CHILD_PHASE")
	if phase == "" {
		return
	}
	if phase != "exit" && phase != "recover" {
		t.Fatal("ACTUAL_SWITCH_CHILD_PHASE_INVALID")
	}
	file := os.Getenv("COLLAB_REBUILD_CHILD_METADATA")
	raw, err := os.ReadFile(file)
	switchBoundaryMust(t, "ACTUAL_SWITCH_CHILD_METADATA_UNAVAILABLE", err)
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var m switchProcessMetadata
	if decoder.Decode(&m) != nil || decoder.Decode(new(any)) != io.EOF || m.Operation == "" || m.Alias == "" || m.JobID == 0 || !filepath.IsAbs(m.Accepted) || m.Until <= time.Now().UnixMilli() {
		t.Fatal("ACTUAL_SWITCH_CHILD_METADATA_INVALID")
	}
	cfg, _ := switchBoundaryConfig(t)
	if m.Alias == cfg.Search.Alias {
		t.Fatal("ACTUAL_SWITCH_CHILD_ALIAS_NOT_ISOLATED")
	}
	ctx, cancel := context.WithDeadline(context.Background(), time.UnixMilli(m.Until))
	defer cancel()
	store, err := platform.OpenStore(cfg)
	switchBoundaryMust(t, "ACTUAL_SWITCH_CHILD_POOL_UNAVAILABLE", err)
	defer store.DB.Close()
	indexes, err := elastic.New(cfg.Search)
	switchBoundaryMust(t, "ACTUAL_SWITCH_CHILD_ES_UNAVAILABLE", err)
	defer indexes.Close()
	f := &switchBoundaryFixture{config: cfg, ctx: ctx, db: store.DB, indexes: indexes, metadata: m}
	f.assertSQL(t, store.DB, "SWITCHING", m.Source, 1, 1)
	if phase == "exit" {
		wrapper := &switchExitIndexes{SwitchIndexes: indexes, fixture: f, proofFile: file + ".exit.json"}
		_ = f.coordinator(store.DB, wrapper).finish(ctx, m.Operation)
		t.Fatal("ACTUAL_SWITCH_CHILD_EXIT_BOUNDARY_NOT_REACHED")
	}
	c := f.coordinator(store.DB, indexes)
	switchBoundaryMust(t, "ACTUAL_SWITCH_CHILD_PRODUCTION_RECOVERY_FAILED", c.Step(ctx))
	result := f.result(t, store.DB)
	switchBoundaryMust(t, "ACTUAL_SWITCH_CHILD_IDEMPOTENT_FINISH_FAILED", c.finish(ctx, m.Operation))
	f.assertStable(t, store.DB, result)
	encoded, err := proto.MarshalOptions{Deterministic: true}.Marshal(result)
	switchBoundaryMust(t, "ACTUAL_SWITCH_CHILD_RESULT_ENCODING_FAILED", err)
	switchBoundaryMust(t, "ACTUAL_SWITCH_CHILD_RESULT_WRITE_FAILED", os.WriteFile(file+".result.pb", encoded, 0600))
	proof, err := json.Marshal(switchProcessEvidence{os.Getpid(), m.Operation, m.Alias, m.Target, "ORIGINAL_JOB_SUCCEEDED"})
	switchBoundaryMust(t, "ACTUAL_SWITCH_CHILD_RECOVERY_EVIDENCE_INVALID", err)
	switchBoundaryMust(t, "ACTUAL_SWITCH_CHILD_RECOVERY_EVIDENCE_WRITE_FAILED", os.WriteFile(file+".recovered.json", proof, 0600))
	t.Log("original operation restored by a fresh OS process; generation/revision and fixed proof verified")
}
