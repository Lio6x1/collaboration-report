package main

import (
	pb "collaboration/contracts/internalv1"
	"collaboration/document/internal/bootstrap"
	"collaboration/document/internal/server"
	"collaboration/platform"
	"google.golang.org/grpc"
)

func main() {
	rpc := server.NewDocument()
	platform.Main("document", func(grpcServer *grpc.Server, _ *platform.Store, _ platform.Config) {
		pb.RegisterDocumentInternalServer(grpcServer, rpc)
	}, bootstrap.RegisterDocument(rpc))
}
