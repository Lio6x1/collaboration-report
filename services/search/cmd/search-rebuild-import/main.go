// search-rebuild-import只读取Search自己的配置和受控inbox中的Doc制品。
package main

import (
	"collaboration/platform"
	"collaboration/platform/rebuildartifact"
	"collaboration/platform/rebuildmeta"
	"collaboration/search/internal/logic/rebuild"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
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
	flags := flag.NewFlagSet("search-rebuild-import", flag.ContinueOnError)
	// 不回显路径、配置或用户参数，错误出口只允许固定码。
	flags.SetOutput(io.Discard)
	config := flags.String("config", "", "Search本服务配置")
	operation := flags.String("operation", "", "原操作号")
	reference := flags.String("snapshot-ref", "", "原Doc快照引用")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *config == "" || !rebuildmeta.OperationValid(*operation) {
		return "SEARCH_REBUILD_ARGUMENT_INVALID"
	}
	name, err := rebuildartifact.FileName(*reference)
	if err != nil || !strings.HasSuffix(name, ".snapshot.json") {
		return "SEARCH_REBUILD_ARGUMENT_INVALID"
	}
	cfg, err := platform.LoadConfig(*config)
	if err != nil || cfg.Role != "search" || cfg.Search == nil || cfg.Root == "" || !filepath.IsAbs(cfg.Root) {
		return "SEARCH_REBUILD_CONFIG_INVALID"
	}
	store, err := platform.OpenStore(cfg)
	if err != nil {
		return "SEARCH_REBUILD_DATABASE_UNAVAILABLE"
	}
	defer store.DB.Close()
	verifier, err := rebuildartifact.NewVerifier(cfg.CAFile)
	if err != nil {
		return "REBUILD_ARTIFACT_CONFIG_INVALID"
	}
	root := filepath.Join(cfg.Root, ".runtime", "rebuild", "search")
	service := rebuild.Importer{DB: store.DB, Verifier: verifier, Owner: cfg.Search.Owner, Alias: cfg.Search.Alias, Inbox: filepath.Join(root, "inbox"), Accepted: filepath.Join(root, "accepted")}
	_, err = service.Import(ctx, *operation, *reference)
	if err != nil {
		if ctx.Err() != nil {
			return "SEARCH_REBUILD_CANCELLED"
		}
		switch err.Error() {
		case "SEARCH_REBUILD_OPERATION_CONFLICT", "SEARCH_REBUILD_IMPORT_COMMIT_UNKNOWN", "SEARCH_REBUILD_PREPARE_MISSING", "SEARCH_REBUILD_GENERATION_INVALID", "SEARCH_REBUILD_ENTRIES_MISMATCH":
			return err.Error()
		default:
			return "SEARCH_REBUILD_IMPORT_FAILED"
		}
	}
	if json.NewEncoder(output).Encode(struct {
		Reference string `json:"reference"`
	}{*reference}) != nil {
		return "SEARCH_REBUILD_OUTPUT_FAILED"
	}
	return ""
}
