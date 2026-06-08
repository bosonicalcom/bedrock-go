// Package grpcx provides gRPC transport primitives, including the Controller interface and health checking.
package grpcx

//go:generate go tool mockgen -source=controller.go -destination=grpctest/controller_mock.go -package=grpctest

import "google.golang.org/grpc"

// A Controller is a system component responsible for registering gRPC service servers and
// interconnecting transport-layer mechanics with internal system components.
type Controller interface {
	// RegisterServers registers all gRPC servers with the given registrar.
	RegisterServers(registrar grpc.ServiceRegistrar)
}
