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
