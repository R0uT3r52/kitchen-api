package grpc_test

import (
	"context"
	"errors"
	"testing"

	"kitchen-api/internal/domain"
	appgrpc "kitchen-api/internal/grpc"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type mockPartnerUseCase struct {
	upsertMenuFn            func(ctx context.Context, restaurantID int64, items []domain.MenuItem) error
	getOrdersFn             func(ctx context.Context, restaurantID int64, status *domain.OrderStatus) ([]domain.Order, error)
	updateOrderStatusFn     func(ctx context.Context, restaurantID int64, orderID int64, newStatus domain.OrderStatus) error
	getRestaurantByAPIKeyFn func(ctx context.Context, apiKey string) (*domain.Restaurant, error)
}

func (m *mockPartnerUseCase) UpsertMenu(ctx context.Context, restaurantID int64, items []domain.MenuItem) error {
	if m.upsertMenuFn != nil {
		return m.upsertMenuFn(ctx, restaurantID, items)
	}
	return nil
}

func (m *mockPartnerUseCase) GetOrders(ctx context.Context, restaurantID int64, status *domain.OrderStatus) ([]domain.Order, error) {
	if m.getOrdersFn != nil {
		return m.getOrdersFn(ctx, restaurantID, status)
	}
	return nil, nil
}

func (m *mockPartnerUseCase) UpdateOrderStatus(ctx context.Context, restaurantID int64, orderID int64, newStatus domain.OrderStatus) error {
	if m.updateOrderStatusFn != nil {
		return m.updateOrderStatusFn(ctx, restaurantID, orderID, newStatus)
	}
	return nil
}

func (m *mockPartnerUseCase) GetRestaurantByAPIKey(ctx context.Context, apiKey string) (*domain.Restaurant, error) {
	if m.getRestaurantByAPIKeyFn != nil {
		return m.getRestaurantByAPIKeyFn(ctx, apiKey)
	}
	return nil, nil
}

func TestUnaryAuthInterceptor(t *testing.T) {
	dummyHandler := func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	}

	info := &grpc.UnaryServerInfo{
		FullMethod: "/partner.v1.MenuService/SyncMenu",
	}

	tests := []struct {
		name          string
		ctx           context.Context
		mock          *mockPartnerUseCase
		expectedCode  codes.Code
		expectedRest  int64
		handlerCalled bool
	}{
		{
			name:         "missing metadata",
			ctx:          context.Background(),
			mock:         &mockPartnerUseCase{},
			expectedCode: codes.Unauthenticated,
		},
		{
			name: "missing x-api-key header",
			ctx: metadata.NewIncomingContext(
				context.Background(),
				metadata.Pairs("other-header", "val"),
			),
			mock:         &mockPartnerUseCase{},
			expectedCode: codes.Unauthenticated,
		},
		{
			name: "empty x-api-key header",
			ctx: metadata.NewIncomingContext(
				context.Background(),
				metadata.Pairs("x-api-key", "   "),
			),
			mock:         &mockPartnerUseCase{},
			expectedCode: codes.Unauthenticated,
		},
		{
			name: "invalid api key - restaurant not found",
			ctx: metadata.NewIncomingContext(
				context.Background(),
				metadata.Pairs("x-api-key", "invalid-key"),
			),
			mock: &mockPartnerUseCase{
				getRestaurantByAPIKeyFn: func(ctx context.Context, apiKey string) (*domain.Restaurant, error) {
					return nil, domain.ErrRestaurantNotFound
				},
			},
			expectedCode: codes.Unauthenticated,
		},
		{
			name: "inactive restaurant",
			ctx: metadata.NewIncomingContext(
				context.Background(),
				metadata.Pairs("x-api-key", "inactive-key"),
			),
			mock: &mockPartnerUseCase{
				getRestaurantByAPIKeyFn: func(ctx context.Context, apiKey string) (*domain.Restaurant, error) {
					return nil, domain.ErrRestaurantUnavailable
				},
			},
			expectedCode: codes.PermissionDenied,
		},
		{
			name: "internal error from service",
			ctx: metadata.NewIncomingContext(
				context.Background(),
				metadata.Pairs("x-api-key", "err-key"),
			),
			mock: &mockPartnerUseCase{
				getRestaurantByAPIKeyFn: func(ctx context.Context, apiKey string) (*domain.Restaurant, error) {
					return nil, errors.New("db failure")
				},
			},
			expectedCode: codes.Internal,
		},
		{
			name: "successful authentication",
			ctx: metadata.NewIncomingContext(
				context.Background(),
				metadata.Pairs("x-api-key", "valid-key"),
			),
			mock: &mockPartnerUseCase{
				getRestaurantByAPIKeyFn: func(ctx context.Context, apiKey string) (*domain.Restaurant, error) {
					return &domain.Restaurant{ID: 42, IsActive: true}, nil
				},
			},
			expectedCode:  codes.OK,
			expectedRest:  42,
			handlerCalled: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			interceptor := appgrpc.UnaryAuthInterceptor(tc.mock)

			called := false
			handler := func(ctx context.Context, req any) (any, error) {
				called = true
				restID, ok := appgrpc.RestaurantIDFromContext(ctx)
				if !ok || restID != tc.expectedRest {
					t.Fatalf("expected restaurant ID %d, got %d (ok=%v)", tc.expectedRest, restID, ok)
				}
				return dummyHandler(ctx, req)
			}

			resp, err := interceptor(tc.ctx, "test_req", info, handler)
			st, _ := status.FromError(err)

			if st.Code() != tc.expectedCode {
				t.Fatalf("expected code %v, got %v (err: %v)", tc.expectedCode, st.Code(), err)
			}

			if tc.handlerCalled && !called {
				t.Fatal("expected handler to be called")
			}
			if !tc.handlerCalled && called {
				t.Fatal("expected handler not to be called")
			}
			if tc.expectedCode == codes.OK && resp != "ok" {
				t.Fatalf("expected response 'ok', got %v", resp)
			}
		})
	}
}
