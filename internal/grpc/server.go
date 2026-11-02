package grpc

import (
	"kitchen-api/internal/domain"

	"google.golang.org/grpc"
)

// NewServer creates and configures a ready-to-run gRPC server with all interceptors and services registered.
func NewServer(partnerService domain.PartnerUseCase, opts ...grpc.ServerOption) *grpc.Server {
	authInterceptor := NewAuthInterceptor(partnerService)

	serverOpts := []grpc.ServerOption{
		grpc.UnaryInterceptor(authInterceptor),
	}
	serverOpts = append(serverOpts, opts...)

	srv := grpc.NewServer(serverOpts...)

	menuServer := NewMenuServer(partnerService)
	menuServer.Register(srv)

	return srv
}
