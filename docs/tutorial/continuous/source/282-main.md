# services/document/cmd/search-rebuild/main.go

阶段03：[功能讲义](../03-document.md) · [分步编写](../write/282-main.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：本模块的公共规则、适配或进程入口；按阶段讲解核对调用者和接线位置。

主要声明：`main`、`run`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
// search-rebuild以真实Doc身份接收/查询已导入的原重建操作。
package main

import (
	pb "collaboration/contracts/internalv1"
	"collaboration/document/internal/logic/snapshot"
	"collaboration/platform"
	"collaboration/platform/rebuildmeta"
	"context"
	"flag"
	"fmt"
	"google.golang.org/protobuf/encoding/protojson"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"time"
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
	flags := flag.NewFlagSet("search-rebuild", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	config := flags.String("config", "", "Doc本服务配置")
	operation := flags.String("operation", "", "原操作号")
	ref := flags.String("snapshot-ref", "", "原封存引用")
	action := flags.String("action", "status", "start或status")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *config == "" || !rebuildmeta.OperationValid(*operation) || (*action != "start" && *action != "status") {
		return "REBUILD_ARGUMENT_INVALID"
	}
	cfg, err := platform.LoadConfig(*config)
	if err != nil || cfg.Role != "document" || !filepath.IsAbs(cfg.Root) {
		return "REBUILD_CONFIG_INVALID"
	}
	store, err := platform.OpenStore(cfg)
	if err != nil {
		return "REBUILD_DATABASE_UNAVAILABLE"
	}
	defer store.DB.Close()
	local, cancel := context.WithTimeout(ctx, 3*time.Second)
	request, fingerprint, err := snapshot.RebuildRequest(local, store.DB, *operation, *ref)
	cancel()
	if err != nil {
		return "REBUILD_ORIGINAL_REQUEST_UNAVAILABLE"
	}
	connection, err := platform.DialPeer(cfg, "search")
	if err != nil {
		return "REBUILD_PEER_UNAVAILABLE"
	}
	defer connection.Close()
	client := pb.NewSearchInternalClient(connection)
	rpcCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var result *pb.OperationResult
	if *action == "start" {
		result, err = client.StartRebuild(rpcCtx, request)
	} else {
		result, err = client.GetRebuild(rpcCtx, &pb.OperationQuery{OperationId: *operation, RequestFingerprint: fingerprint[:]})
	}
	if err != nil {
		// 包括网络中断在内都不推断未接收；操作者用同一参数status查询或start重试。
		return "REBUILD_RPC_UNKNOWN_RETRY_SAME_OPERATION"
	}
	if result == nil || result.OperationId != *operation || result.Generation != request.Generation {
		return "REBUILD_REPLY_INVALID"
	}
	raw, err := (protojson.MarshalOptions{UseProtoNames: true}).Marshal(result)
	if err != nil {
		return "REBUILD_REPLY_INVALID"
	}
	if _, err := fmt.Fprintln(output, string(raw)); err != nil {
		return "REBUILD_OUTPUT_FAILED"
	}
	return ""
}
````

UTF-8/LF全文SHA-256：`e9626cd79436067798ae3cf711a170298f7dd1c8561d5b7cd3bc09ee39ab6612`。[参考实现](../../../../services/document/cmd/search-rebuild/main.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
