package main

import (
	"fmt"
	"log"
	"net"

	publishingv1 "github.com/dipen1510/marketplace-workflow-platform/publishing-status-controller/api"
	"github.com/dipen1510/marketplace-workflow-platform/publishing-status-controller/internal/publishing"

	"google.golang.org/grpc"
)

func main() {

	address := ":50051"

	listener, err :=
		net.Listen(
			"tcp",
			address,
		)

	if err != nil {
		log.Fatalf(
			"failed to listen: %v",
			err,
		)
	}

	grpcServer :=
		grpc.NewServer()

	service :=
		publishing.NewService()

	publishingv1.
		RegisterMarketplacePublishingServiceServer(
			grpcServer,
			service,
		)

	fmt.Printf(
		"Marketplace Publishing Service listening on %s\n",
		address,
	)

	if err :=
		grpcServer.Serve(listener); err != nil {

		log.Fatalf(
			"gRPC server failed: %v",
			err,
		)
	}
}
