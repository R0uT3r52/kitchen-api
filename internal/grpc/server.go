package grpc

import (
	partnerv1 "kitchen-api/api/proto/partner/v1"
	"kitchen-api/internal/domain"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

// creates and configures a gRPC server with all interceptors and services registered
func NewServer(partnerService domain.PartnerUseCase, opts ...grpc.ServerOption) *grpc.Server {
	authInterceptor := NewAuthInterceptor(partnerService)

	serverOpts := []grpc.ServerOption{
		grpc.UnaryInterceptor(authInterceptor),
	}
	serverOpts = append(serverOpts, opts...)

	srv := grpc.NewServer(serverOpts...)

	// gRPC health checking service
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(srv, healthServer)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	healthServer.SetServingStatus(partnerv1.MenuService_ServiceDesc.ServiceName, grpc_health_v1.HealthCheckResponse_SERVING)

	menuServer := NewMenuServer(partnerService)
	menuServer.Register(srv)

	return srv
}
