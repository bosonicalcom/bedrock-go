package grpcx

import (
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

// HealthController implements [Controller] and exposes the standard gRPC health
// checking protocol, initialising the global serving status to SERVING.
type HealthController struct {
	healthSrv *health.Server
}

var (
	_ Controller = (*HealthController)(nil)
)

// NewHealthController creates a HealthController whose global serving status is
// set to SERVING immediately, making it safe to register before the server starts.
func NewHealthController() HealthController {
	srv := health.NewServer()
	srv.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING) // set initial global status to SERVING
	return HealthController{
		healthSrv: srv,
	}
}

// RegisterServers implements [Controller] by registering the gRPC health server.
func (c HealthController) RegisterServers(registrar grpc.ServiceRegistrar) {
	grpc_health_v1.RegisterHealthServer(registrar, c.healthSrv)
}
