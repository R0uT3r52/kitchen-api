package grpc_test

import (
	"context"
	"errors"
	"net"
	"testing"

	partnerv1 "kitchen-api/api/proto/partner/v1"
	"kitchen-api/internal/domain"
	appgrpc "kitchen-api/internal/grpc"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func TestMenuServer_SyncMenu(t *testing.T) {
	validItems := []*partnerv1.MenuItem{
		{
			ExternalId:  "item-1",
			Name:        "Burger",
			PriceCents:  500,
			IsAvailable: true,
		},
		{
			ExternalId:  "item-2",
			Name:        "Fries",
			PriceCents:  250,
			IsAvailable: false,
		},
	}

	tests := []struct {
		name         string
		ctx          context.Context
		req          *partnerv1.SyncMenuRequest
		mock         *mockPartnerUseCase
		expectedCode codes.Code
		checkResp    func(t *testing.T, resp *partnerv1.SyncMenuResponse)
	}{
		{
			name:         "unauthenticated - missing restaurant id",
			ctx:          context.Background(),
			req:          &partnerv1.SyncMenuRequest{Items: validItems},
			mock:         &mockPartnerUseCase{},
			expectedCode: codes.Unauthenticated,
		},
		{
			name:         "unauthenticated - invalid restaurant id <= 0",
			ctx:          appgrpc.ContextWithRestaurantID(context.Background(), 0),
			req:          &partnerv1.SyncMenuRequest{Items: validItems},
			mock:         &mockPartnerUseCase{},
			expectedCode: codes.Unauthenticated,
		},
		{
			name:         "nil request",
			ctx:          appgrpc.ContextWithRestaurantID(context.Background(), 1),
			req:          nil,
			mock:         &mockPartnerUseCase{},
			expectedCode: codes.InvalidArgument,
		},
		{
			name:         "empty items list",
			ctx:          appgrpc.ContextWithRestaurantID(context.Background(), 1),
			req:          &partnerv1.SyncMenuRequest{Items: []*partnerv1.MenuItem{}},
			mock:         &mockPartnerUseCase{},
			expectedCode: codes.InvalidArgument,
		},
		{
			name: "nil menu item in list",
			ctx:  appgrpc.ContextWithRestaurantID(context.Background(), 1),
			req: &partnerv1.SyncMenuRequest{
				Items: []*partnerv1.MenuItem{nil},
			},
			mock:         &mockPartnerUseCase{},
			expectedCode: codes.InvalidArgument,
		},
		{
			name: "domain error - ErrInvalidData",
			ctx:  appgrpc.ContextWithRestaurantID(context.Background(), 1),
			req:  &partnerv1.SyncMenuRequest{Items: validItems},
			mock: &mockPartnerUseCase{
				upsertMenuFn: func(ctx context.Context, restaurantID int64, items []domain.MenuItem) error {
					return domain.ErrInvalidData
				},
			},
			expectedCode: codes.InvalidArgument,
		},
		{
			name: "domain error - ErrRestaurantNotFound",
			ctx:  appgrpc.ContextWithRestaurantID(context.Background(), 1),
			req:  &partnerv1.SyncMenuRequest{Items: validItems},
			mock: &mockPartnerUseCase{
				upsertMenuFn: func(ctx context.Context, restaurantID int64, items []domain.MenuItem) error {
					return domain.ErrRestaurantNotFound
				},
			},
			expectedCode: codes.NotFound,
		},
		{
			name: "domain error - ErrRestaurantUnavailable",
			ctx:  appgrpc.ContextWithRestaurantID(context.Background(), 1),
			req:  &partnerv1.SyncMenuRequest{Items: validItems},
			mock: &mockPartnerUseCase{
				upsertMenuFn: func(ctx context.Context, restaurantID int64, items []domain.MenuItem) error {
					return domain.ErrRestaurantUnavailable
				},
			},
			expectedCode: codes.FailedPrecondition,
		},
		{
			name: "domain error - internal failure",
			ctx:  appgrpc.ContextWithRestaurantID(context.Background(), 1),
			req:  &partnerv1.SyncMenuRequest{Items: validItems},
			mock: &mockPartnerUseCase{
				upsertMenuFn: func(ctx context.Context, restaurantID int64, items []domain.MenuItem) error {
					return errors.New("database connection failed")
				},
			},
			expectedCode: codes.Internal,
		},
		{
			name: "success sync menu",
			ctx:  appgrpc.ContextWithRestaurantID(context.Background(), 10),
			req:  &partnerv1.SyncMenuRequest{Items: validItems},
			mock: &mockPartnerUseCase{
				upsertMenuFn: func(ctx context.Context, restaurantID int64, items []domain.MenuItem) error {
					if restaurantID != 10 {
						t.Fatalf("expected restaurant ID 10, got %d", restaurantID)
					}
					if len(items) != 2 {
						t.Fatalf("expected 2 items, got %d", len(items))
					}
					if items[0].ExternalID != "item-1" || items[0].PriceCents != 500 || !items[0].IsAvailable {
						t.Fatalf("unexpected item 0: %+v", items[0])
					}
					if items[1].ExternalID != "item-2" || items[1].PriceCents != 250 || items[1].IsAvailable {
						t.Fatalf("unexpected item 1: %+v", items[1])
					}
					return nil
				},
			},
			expectedCode: codes.OK,
			checkResp: func(t *testing.T, resp *partnerv1.SyncMenuResponse) {
				if resp.GetStatus() != "ok" {
					t.Fatalf("expected status 'ok', got '%s'", resp.GetStatus())
				}
				if resp.GetCount() != 2 {
					t.Fatalf("expected count 2, got %d", resp.GetCount())
				}
				if resp.GetMessage() == "" {
					t.Fatal("expected non-empty message")
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := appgrpc.NewMenuServer(tc.mock)
			resp, err := server.SyncMenu(tc.ctx, tc.req)
			st, _ := status.FromError(err)

			if st.Code() != tc.expectedCode {
				t.Fatalf("expected code %v, got %v (err: %v)", tc.expectedCode, st.Code(), err)
			}

			if tc.expectedCode == codes.OK && tc.checkResp != nil {
				tc.checkResp(t, resp)
			}
		})
	}
}

func TestMenuService_E2EIntegration(t *testing.T) {
	bufferSize := 1024 * 1024
	lis := bufconn.Listen(bufferSize)

	mock := &mockPartnerUseCase{
		getRestaurantByAPIKeyFn: func(ctx context.Context, apiKey string) (*domain.Restaurant, error) {
			if apiKey == "valid-api-key" {
				return &domain.Restaurant{ID: 15, IsActive: true}, nil
			}
			return nil, domain.ErrRestaurantNotFound
		},
		upsertMenuFn: func(ctx context.Context, restaurantID int64, items []domain.MenuItem) error {
			if restaurantID != 15 {
				t.Fatalf("expected restaurant ID 15, got %d", restaurantID)
			}
			return nil
		},
	}

	grpcServer := appgrpc.NewServer(mock)

	go func() {
		if err := grpcServer.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			t.Errorf("grpc server error: %v", err)
		}
	}()
	defer grpcServer.GracefulStop()

	dialer := func(context.Context, string) (net.Conn, error) {
		return lis.Dial()
	}

	conn, err := grpc.NewClient(
		"passthrough://bufnet",
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("failed to dial bufnet: %v", err)
	}
	defer conn.Close() //nolint:errcheck

	client := partnerv1.NewMenuServiceClient(conn)

	req := &partnerv1.SyncMenuRequest{
		Items: []*partnerv1.MenuItem{
			{
				ExternalId:  "e2e-item-1",
				Name:        "Pasta",
				PriceCents:  1200,
				IsAvailable: true,
			},
		},
	}

	// 1. Without auth header -> Unauthenticated
	_, err = client.SyncMenu(context.Background(), req)
	st, _ := status.FromError(err)
	if st.Code() != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated without header, got: %v", st.Code())
	}

	// 2. With wrong auth header -> Unauthenticated
	badCtx := metadata.AppendToOutgoingContext(context.Background(), "x-api-key", "wrong-key")
	_, err = client.SyncMenu(badCtx, req)
	st, _ = status.FromError(err)
	if st.Code() != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated with wrong key, got: %v", st.Code())
	}

	// 3. With valid auth header -> Success
	goodCtx := metadata.AppendToOutgoingContext(context.Background(), "x-api-key", "valid-api-key")
	resp, err := client.SyncMenu(goodCtx, req)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if resp.GetStatus() != "ok" || resp.GetCount() != 1 {
		t.Fatalf("unexpected response: %+v", resp)
	}
}
