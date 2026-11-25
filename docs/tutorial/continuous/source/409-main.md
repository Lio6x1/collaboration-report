# services/search/cmd/search-rebuild-prepare/main.go

阶段05：[功能讲义](../05-search.md) · [分步编写](../write/409-main.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：本模块的公共规则、适配或进程入口；按阶段讲解核对调用者和接线位置。

主要声明：`main`、`run`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// search-rebuild-prepare 只读取 Search 自己的配置并导出已提交的原准备制品。
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"

	"collaboration/platform"
	"collaboration/platform/rebuildartifact"
	"collaboration/platform/rebuildmeta"
	"collaboration/search/internal/client/elastic"
	"collaboration/search/internal/client/logsource"
	"collaboration/search/internal/logic/rebuild"
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
	flags := flag.NewFlagSet("search-rebuild-prepare", flag.ContinueOnError)
	// flag 默认错误会回显用户参数；这里只输出固定错误码，配置内容与路径不进日志。
	flags.SetOutput(io.Discard)
	config := flags.String("config", "", "Search 本服务配置文件")
	operation := flags.String("operation", "", "原操作号")
	generation := flags.String("generation", "", "规范十进制目标代次")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *config == "" || !rebuildmeta.OperationValid(*operation) {
		return "SEARCH_REBUILD_ARGUMENT_INVALID"
	}
	target, err := rebuildmeta.Decimal(*generation, true)
	if err != nil {
		return "SEARCH_REBUILD_GENERATION_INVALID"
	}
	cfg, err := platform.LoadConfig(*config)
	if err != nil || cfg.Role != "search" || cfg.Root == "" || !filepath.IsAbs(cfg.Root) || cfg.Search == nil {
		return "SEARCH_REBUILD_CONFIG_INVALID"
	}
	store, err := platform.OpenStore(cfg)
	if err != nil {
		return "SEARCH_REBUILD_DATABASE_UNAVAILABLE"
	}
	defer store.DB.Close()
	logs, err := logsource.New(cfg.Search.Brokers, cfg.Search.Topic)
	if err != nil {
		return "SEARCH_REBUILD_LOG_CONFIG_INVALID"
	}
	defer logs.Close()
	aliases, err := elastic.New(cfg.Search)
	if err != nil {
		return "SEARCH_REBUILD_ES_CONFIG_INVALID"
	}
	defer aliases.Close()
	verifier, err := rebuildartifact.NewVerifier(cfg.CAFile)
	if err != nil {
		return "REBUILD_ARTIFACT_CONFIG_INVALID"
	}
	preparer := rebuild.Preparer{DB: store.DB, Logs: logs, Aliases: aliases, Verifier: verifier,
		Owner: cfg.Search.Owner, Alias: cfg.Search.Alias,
		// 已接受原 prepare 恢复不应因当前叶证书过期而失败；只在新任务时读私钥。
		LoadSigner: func() (*rebuildartifact.Signer, error) {
			return rebuildartifact.NewSigner(cfg.CertFile, cfg.KeyFile, cfg.CAFile, "search")
		},
	}
	envelope, err := preparer.Prepare(ctx, *operation, target)
	if err != nil {
		if err.Error() == "SEARCH_REBUILD_PREPARE_COMMIT_UNKNOWN" {
			return "SEARCH_REBUILD_PREPARE_COMMIT_UNKNOWN"
		}
		if ctx.Err() != nil {
			return "SEARCH_REBUILD_CANCELLED"
		}
		// 已知提交未知必须明确提示原号重试，其他底层原因不回显 DSN/网络响应。
		switch err.Error() {
		case "SEARCH_REBUILD_OPERATION_CONFLICT", "SEARCH_REBUILD_PREPARE_REJECTED", "SEARCH_REBUILD_GENERATION_INVALID":
			return err.Error()
		default:
			return "SEARCH_REBUILD_PREPARE_FAILED"
		}
	}
	reference, err := rebuildartifact.Reference(envelope, rebuildartifact.PrepareV1)
	if err != nil {
		return "REBUILD_ARTIFACT_ENVELOPE_INVALID"
	}
	name, err := rebuildartifact.FileName(reference)
	if err != nil {
		return "REBUILD_ARTIFACT_REFERENCE_INVALID"
	}
	if ctx.Err() != nil {
		return "SEARCH_REBUILD_CANCELLED"
	}
	outbox := filepath.Join(cfg.Root, ".runtime", "rebuild", "search", "outbox")
	if err := rebuildartifact.PublishFile(outbox, name, func(w io.Writer) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, err := w.Write(envelope)
		if err == nil && n != len(envelope) {
			err = io.ErrShortWrite
		}
		return err
	}); err != nil {
		return "SEARCH_REBUILD_OUTPUT_FAILED"
	}
	if ctx.Err() != nil {
		return "SEARCH_REBUILD_CANCELLED"
	}
	if json.NewEncoder(output).Encode(struct {
		Reference string `json:"reference"`
	}{reference}) != nil {
		return "SEARCH_REBUILD_OUTPUT_FAILED"
	}
	return ""
}
````

UTF-8/LF全文SHA-256：`5d773d7bf6e753a26466f5ff23c89e57333cb35c439d2d4b447bca1edd773e3e`。[参考实现](../../../../services/search/cmd/search-rebuild-prepare/main.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
