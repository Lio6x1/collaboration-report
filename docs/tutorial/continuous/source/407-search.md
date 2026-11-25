# services/search/internal/bootstrap/search.go

阶段05：[功能讲义](../05-search.md) · [分步编写](../write/407-search.md) · [主路线](../README.md)

这是**写完后的完整答案**。学习时先进入功能讲义和逐段操作页；本页用于核对遗漏、查阅最终调用关系，不作为整文件输入任务。不要把本页全文追加到已经分步编写的文件末尾。

职责：注册路由、RPC及恢复器，连接入口和完整业务。

主要声明：`RegisterSearch`。字段和参数、执行顺序、正常/错误输入、事务原因、接线与测试在功能讲义中展开；逐段操作页提供每步的落笔位置和完整本步代码。

````go
package bootstrap

import (
	pb "collaboration/contracts/internalv1"
	"collaboration/platform"
	"collaboration/search/internal/server"
	"collaboration/search/internal/svc"
	"collaboration/search/internal/worker"
	"context"
	"google.golang.org/grpc"
)

// RegisterSearch仅组合当前RPC适配，底座就绪继续依赖本服务私有Store的实际检查。
func RegisterSearch(rpc *grpc.Server, store *platform.Store, configuration platform.Config) {
	consumer, err := svc.NewConsumer(store, configuration)
	candidates, candidateErr := svc.NewCandidates(store, configuration)
	rebuild, rebuildErr := svc.NewRebuild(store, configuration)
	if err != nil {
		store.ReadyExtra = func(context.Context) error { return err }
		if candidates != nil {
			candidates.Close()
		}
	} else if candidateErr != nil {
		store.ReadyExtra = func(context.Context) error { return candidateErr }
		consumer.Close()
	} else if rebuildErr != nil {
		store.ReadyExtra = func(context.Context) error { return rebuildErr }
		consumer.Close()
		candidates.Close()
	} else {
		rebuildWorker, seedErr := svc.NewRebuildWorker(store, configuration)
		if seedErr != nil {
			store.ReadyExtra = func(context.Context) error { return seedErr }
			consumer.Close()
			candidates.Close()
			pb.RegisterSearchInternalServer(rpc, server.NewSearch())
			return
		}
		store.ReadyExtra = func(ctx context.Context) error {
			if err := consumer.Ready(ctx); err != nil {
				return err
			}
			return candidates.Ready(ctx)
		}
		go worker.RunAfterRecovery(store.Context, consumer, rebuildWorker.RecoveryReady)
		go worker.RunCursors(store.Context, candidates)
		go worker.RunRebuild(store.Context, rebuildWorker)
	}
	if err != nil || candidateErr != nil || rebuildErr != nil {
		pb.RegisterSearchInternalServer(rpc, server.NewSearch())
		return
	}
	pb.RegisterSearchInternalServer(rpc, server.NewSearchWithRebuild(candidates, rebuild))
}
````

UTF-8/LF全文SHA-256：`f577e95443dd514fbf21c0d3dce34219d59fd896db1c28016111ac02314ad692`。[参考实现](../../../../services/search/internal/bootstrap/search.go)仅用于对照。变式有合理差异时看实际测试；摘要相同不表示业务验收通过。
