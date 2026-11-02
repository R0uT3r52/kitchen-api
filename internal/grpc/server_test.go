package grpc_test

import (
	"testing"

	appgrpc "kitchen-api/internal/grpc"

	"google.golang.org/grpc"
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
}
