# services/document/cmd/search-snapshot/main.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/283-main.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：本模块的公共规则、适配或进程入口；按阶段讲解核对调用者和接线位置。

主要声明：`main`、`run`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// search-snapshot 只使用 Document 本服务身份与私有库；制品交接经受控 inbox/outbox。
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"collaboration/document/internal/logic/snapshot"
	"collaboration/platform"
	"collaboration/platform/rebuildartifact"
	"collaboration/platform/rebuildmeta"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if code := run(ctx, os.Args[1:], os.Stdout); code != "" {
		fmt.Fprintln(os.Stderr, code)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, output io.Writer) string {
	flags := flag.NewFlagSet("search-snapshot", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	config := flags.String("config", "", "Document 本服务配置文件")
	operation := flags.String("operation", "", "原操作号")
	prepareRef := flags.String("prepare-ref", "", "受控 inbox 内的 prepare 内容引用")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *config == "" || !rebuildmeta.OperationValid(*operation) ||
		!strings.HasPrefix(*prepareRef, "rebuild-prepare-v1:") {
		return "REBUILD_SNAPSHOT_ARGUMENT_INVALID"
	}
	cfg, err := platform.LoadConfig(*config)
	if err != nil || cfg.Role != "document" || cfg.Root == "" || !filepath.IsAbs(cfg.Root) {
		return "REBUILD_SNAPSHOT_CONFIG_INVALID"
	}
	// prepare-ref 是内容地址而不是路径，ReadFile 同时限制文件名与核对原始字节摘要。
	inbox := filepath.Join(cfg.Root, ".runtime", "rebuild", "document", "inbox")
	prepare, err := rebuildartifact.ReadFile(inbox, *prepareRef)
	if err != nil {
		return "REBUILD_SNAPSHOT_INPUT_INVALID"
	}
	store, err := platform.OpenStore(cfg)
	if err != nil {
		return "REBUILD_SNAPSHOT_DATABASE_UNAVAILABLE"
	}
	defer store.DB.Close()
	verifier, err := rebuildartifact.NewVerifier(cfg.CAFile)
	if err != nil {
		return "REBUILD_ARTIFACT_CONFIG_INVALID"
	}
	service := snapshot.Service{DB: store.DB, Verifier: verifier,
		LoadSigner: func() (*rebuildartifact.Signer, error) {
			return rebuildartifact.NewSigner(cfg.CertFile, cfg.KeyFile, cfg.CAFile, "document")
		},
	}
	seal, err := service.Capture(ctx, *operation, prepare)
	if err != nil {
		if errors.Is(err, snapshot.ErrCommitUnknown) {
			return "REBUILD_SNAPSHOT_COMMIT_UNKNOWN_RETRY_SAME_OPERATION"
		}
		if ctx.Err() != nil {
			return "REBUILD_SNAPSHOT_CANCELLED"
		}
		if errors.Is(err, snapshot.ErrConflict) {
			return "REBUILD_OPERATION_CONFLICT"
		}
		return "REBUILD_SNAPSHOT_CAPTURE_FAILED"
	}
	reference, err := rebuildartifact.Reference(seal, rebuildartifact.SnapshotV1)
	if err != nil {
		return "REBUILD_ARTIFACT_ENVELOPE_INVALID"
	}
	name, err := rebuildartifact.FileName(reference)
	if err != nil || !strings.HasSuffix(name, ".snapshot.json") {
		return "REBUILD_ARTIFACT_REFERENCE_INVALID"
	}
	outbox := filepath.Join(cfg.Root, ".runtime", "rebuild", "document", "outbox")
	entriesName := strings.TrimSuffix(name, ".snapshot.json") + ".entries.ndjson"
	if err := rebuildartifact.PublishFile(outbox, entriesName, func(w io.Writer) error {
		original, err := service.Export(ctx, *operation, w)
		if err != nil {
			return err
		}
		if !bytes.Equal(original, seal) {
			return errors.New("REBUILD_SNAPSHOT_SEAL_CHANGED")
		}
		return ctx.Err()
	}); err != nil {
		return "REBUILD_SNAPSHOT_OUTPUT_FAILED"
	}
	// 条目先完整校验、Sync 并发布，原 seal 最后发布；中途失败可按原号重生，
	// 不能用重扫 document 或重签名补齐文件，也不能把 SQL 成功冒充文件发布成功。
	if err := rebuildartifact.PublishFile(outbox, name, func(w io.Writer) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, err := w.Write(seal)
		if err == nil && n != len(seal) {
			err = io.ErrShortWrite
		}
		return err
	}); err != nil {
		return "REBUILD_SNAPSHOT_OUTPUT_FAILED"
	}
	if ctx.Err() != nil {
		return "REBUILD_SNAPSHOT_CANCELLED"
	}
	if json.NewEncoder(output).Encode(struct {
		Reference string `json:"reference"`
	}{reference}) != nil {
		return "REBUILD_SNAPSHOT_OUTPUT_FAILED"
	}
	return ""
}
````

UTF-8/LF全文SHA-256：`3140219a287d5fea86fda4691a1b6cde48ff3403cc417a93fbf862ec5e11dd7f`。[参考实现](../../../../services/document/cmd/search-snapshot/main.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
