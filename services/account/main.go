package main

import (
	"collaboration/account/internal/bootstrap"
	"collaboration/account/internal/server"
	"collaboration/account/internal/svc"
	pb "collaboration/contracts/internalv1"
	"collaboration/platform"

	"github.com/zeromicro/go-zero/rest"
	"google.golang.org/grpc"
)

func main() {
	var identity *server.Identity
	platform.Main("account", func(rpc *grpc.Server, store *platform.Store, _ platform.Config) {
		identity = server.NewIdentity(svc.NewIdentity(store))
		identity.BindAuthorizationResults(svc.NewAuthorizationResults(store))
		pb.RegisterIdentityInternalServer(rpc, identity)
	}, func(http *rest.Server, store *platform.Store, cfg platform.Config) error {
		return bootstrap.RegisterAccount(http, store, cfg, identity.BindOperationResults)
	})
}
