package grpc_test

import (
	"context"
	"net"
	"testing"

	appgrpc "kitchen-api/internal/grpc"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/test/bufconn"
)

func TestNewServer(t *testing.T) {
	mock := &mockPartnerUseCase{}
	server := appgrpc.NewServer(mock, grpc.MaxRecvMsgSize(1024*1024))
	if server == nil {
		t.Fatal("expected non-nil grpc server")
	}

	info := server.GetServiceInfo()
	if _, ok := info["partner.v1.MenuService"]; !ok {
		t.Fatalf("expected partner.v1.MenuService to be registered, got %v", info)
	}
	if _, ok := info["grpc.health.v1.Health"]; !ok {
		t.Fatalf("expected grpc.health.v1.Health to be registered, got %v", info)
	}

	lis := bufconn.Listen(1024 * 1024)
	go func() {
		_ = server.Serve(lis)
	}()
	defer server.GracefulStop()

	conn, err := grpc.NewClient(
		"passthrough://bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}
	defer conn.Close() //nolint:errcheck

	healthClient := grpc_health_v1.NewHealthClient(conn)
	resp, err := healthClient.Check(context.Background(), &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("health check failed: %v", err)
	}
	if resp.GetStatus() != grpc_health_v1.HealthCheckResponse_SERVING {
		t.Fatalf("expected SERVING status, got %v", resp.GetStatus())
	}
}
