package web_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"kitchen-api/internal/domain"
	"kitchen-api/internal/web"
)

type mockOrderUseCase struct {
	getActiveRestaurantsFn func(ctx context.Context) ([]domain.Restaurant, error)
	getRestaurantMenuFn    func(ctx context.Context, restaurantID int64) ([]domain.MenuItem, error)
	createOrderFn          func(ctx context.Context, userID string, restaurantID int64, items []domain.OrderItemCreate) (*domain.Order, error)
	getOrderByIDFn         func(ctx context.Context, orderID int64) (*domain.Order, error)
	getUserOrdersFn        func(ctx context.Context, userID string) ([]domain.Order, error)
	cancelOrderFn          func(ctx context.Context, orderID int64, userID string) error
}

func (m *mockOrderUseCase) GetActiveRestaurants(ctx context.Context) ([]domain.Restaurant, error) {
	if m.getActiveRestaurantsFn != nil {
		return m.getActiveRestaurantsFn(ctx)
	}
	return nil, nil
}

func (m *mockOrderUseCase) GetRestaurantMenu(ctx context.Context, restaurantID int64) ([]domain.MenuItem, error) {
	if m.getRestaurantMenuFn != nil {
		return m.getRestaurantMenuFn(ctx, restaurantID)
	}
	return nil, nil
}

func (m *mockOrderUseCase) CreateOrder(ctx context.Context, userID string, restaurantID int64, items []domain.OrderItemCreate) (*domain.Order, error) {
	if m.createOrderFn != nil {
		return m.createOrderFn(ctx, userID, restaurantID, items)
	}
	return nil, nil
}

func (m *mockOrderUseCase) GetOrderByID(ctx context.Context, orderID int64) (*domain.Order, error) {
	if m.getOrderByIDFn != nil {
		return m.getOrderByIDFn(ctx, orderID)
	}
	return nil, nil
}

func (m *mockOrderUseCase) GetUserOrders(ctx context.Context, userID string) ([]domain.Order, error) {
	if m.getUserOrdersFn != nil {
		return m.getUserOrdersFn(ctx, userID)
	}
	return nil, nil
}

func (m *mockOrderUseCase) CancelOrder(ctx context.Context, orderID int64, userID string) error {
	if m.cancelOrderFn != nil {
		return m.cancelOrderFn(ctx, orderID, userID)
	}
	return nil
}

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

func setupTestServer(orderUC domain.OrderUseCase, partnerUC domain.PartnerUseCase) http.Handler {
	h := web.NewHandler(orderUC, partnerUC)
	return h.InitRoutes()
}

func TestHealthCheck(t *testing.T) {
	router := setupTestServer(&mockOrderUseCase{}, &mockPartnerUseCase{})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("expected status 'ok', got '%s'", body["status"])
	}
}

func TestGetRestaurants(t *testing.T) {
	t.Run("success with restaurants", func(t *testing.T) {
		orderUC := &mockOrderUseCase{
			getActiveRestaurantsFn: func(ctx context.Context) ([]domain.Restaurant, error) {
				return []domain.Restaurant{
					{ID: 1, Name: "Dominos", Description: "Pizza place", IsActive: true},
				}, nil
			},
		}
		router := setupTestServer(orderUC, &mockPartnerUseCase{})

		req := httptest.NewRequest(http.MethodGet, "/api/v1/restaurants", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		var list []domain.Restaurant
		if err := json.NewDecoder(rec.Body).Decode(&list); err != nil {
			t.Fatalf("failed to decode json: %v", err)
		}
		if len(list) != 1 || list[0].Name != "Dominos" {
			t.Errorf("unexpected list content: %+v", list)
		}
	})

	t.Run("success empty list", func(t *testing.T) {
		orderUC := &mockOrderUseCase{
			getActiveRestaurantsFn: func(ctx context.Context) ([]domain.Restaurant, error) {
				return nil, nil
			},
		}
		router := setupTestServer(orderUC, &mockPartnerUseCase{})

		req := httptest.NewRequest(http.MethodGet, "/api/v1/restaurants", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		if rec.Body.String() != "[]\n" {
			t.Errorf("expected '[]', got %q", rec.Body.String())
		}
	})

	t.Run("internal server error", func(t *testing.T) {
		orderUC := &mockOrderUseCase{
			getActiveRestaurantsFn: func(ctx context.Context) ([]domain.Restaurant, error) {
				return nil, errors.New("db error")
			},
		}
		router := setupTestServer(orderUC, &mockPartnerUseCase{})

		req := httptest.NewRequest(http.MethodGet, "/api/v1/restaurants", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", rec.Code)
		}
	})
}

func TestGetRestaurantMenu(t *testing.T) {
	t.Run("invalid id", func(t *testing.T) {
		router := setupTestServer(&mockOrderUseCase{}, &mockPartnerUseCase{})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/restaurants/invalid/menu", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("restaurant not found", func(t *testing.T) {
		orderUC := &mockOrderUseCase{
			getRestaurantMenuFn: func(ctx context.Context, restaurantID int64) ([]domain.MenuItem, error) {
				return nil, domain.ErrRestaurantNotFound
			},
		}
		router := setupTestServer(orderUC, &mockPartnerUseCase{})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/restaurants/999/menu", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rec.Code)
		}
	})

	t.Run("restaurant unavailable", func(t *testing.T) {
		orderUC := &mockOrderUseCase{
			getRestaurantMenuFn: func(ctx context.Context, restaurantID int64) ([]domain.MenuItem, error) {
				return nil, domain.ErrRestaurantUnavailable
			},
		}
		router := setupTestServer(orderUC, &mockPartnerUseCase{})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/restaurants/1/menu", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("success", func(t *testing.T) {
		orderUC := &mockOrderUseCase{
			getRestaurantMenuFn: func(ctx context.Context, restaurantID int64) ([]domain.MenuItem, error) {
				return []domain.MenuItem{
					{ID: 10, RestaurantID: 1, Name: "Pizza", PriceCents: 50000, IsAvailable: true},
				}, nil
			},
		}
		router := setupTestServer(orderUC, &mockPartnerUseCase{})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/restaurants/1/menu", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		var menu []domain.MenuItem
		if err := json.NewDecoder(rec.Body).Decode(&menu); err != nil {
			t.Fatalf("failed to decode json: %v", err)
		}
		if len(menu) != 1 || menu[0].Name != "Pizza" {
			t.Errorf("unexpected menu content: %+v", menu)
		}
	})
}

func TestCreateOrder(t *testing.T) {
	t.Run("malformed body", func(t *testing.T) {
		router := setupTestServer(&mockOrderUseCase{}, &mockPartnerUseCase{})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewBufferString("{bad json"))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("empty user_id", func(t *testing.T) {
		router := setupTestServer(&mockOrderUseCase{}, &mockPartnerUseCase{})
		body := web.CreateOrderRequest{
			UserID:       " ",
			RestaurantID: 1,
			Items:        []web.OrderItemRequest{{MenuItemID: 1, Quantity: 2}},
		}
		payload, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewReader(payload))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("invalid restaurant_id", func(t *testing.T) {
		router := setupTestServer(&mockOrderUseCase{}, &mockPartnerUseCase{})
		body := web.CreateOrderRequest{
			UserID:       "user123",
			RestaurantID: 0,
			Items:        []web.OrderItemRequest{{MenuItemID: 1, Quantity: 2}},
		}
		payload, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewReader(payload))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("empty items", func(t *testing.T) {
		router := setupTestServer(&mockOrderUseCase{}, &mockPartnerUseCase{})
		body := web.CreateOrderRequest{
			UserID:       "user123",
			RestaurantID: 1,
			Items:        []web.OrderItemRequest{},
		}
		payload, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewReader(payload))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("invalid quantity", func(t *testing.T) {
		router := setupTestServer(&mockOrderUseCase{}, &mockPartnerUseCase{})
		body := web.CreateOrderRequest{
			UserID:       "user123",
			RestaurantID: 1,
			Items:        []web.OrderItemRequest{{MenuItemID: 1, Quantity: 0}},
		}
		payload, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewReader(payload))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("menu item unavailable (stop-list)", func(t *testing.T) {
		orderUC := &mockOrderUseCase{
			createOrderFn: func(ctx context.Context, userID string, restaurantID int64, items []domain.OrderItemCreate) (*domain.Order, error) {
				return nil, domain.ErrMenuItemUnavailable
			},
		}
		router := setupTestServer(orderUC, &mockPartnerUseCase{})
		body := web.CreateOrderRequest{
			UserID:       "user123",
			RestaurantID: 1,
			Items:        []web.OrderItemRequest{{MenuItemID: 1, Quantity: 1}},
		}
		payload, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewReader(payload))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("success", func(t *testing.T) {
		now := time.Now()
		orderUC := &mockOrderUseCase{
			createOrderFn: func(ctx context.Context, userID string, restaurantID int64, items []domain.OrderItemCreate) (*domain.Order, error) {
				return &domain.Order{
					ID:              42,
					UserID:          userID,
					RestaurantID:    restaurantID,
					Status:          domain.StatusCreated,
					TotalPriceCents: 50000,
					CreatedAt:       now,
					UpdatedAt:       now,
					Items: []domain.OrderItem{
						{ID: 1, OrderID: 42, MenuItemID: 1, Quantity: 1, UnitPriceCents: 50000},
					},
				}, nil
			},
		}
		router := setupTestServer(orderUC, &mockPartnerUseCase{})
		body := web.CreateOrderRequest{
			UserID:       "user123",
			RestaurantID: 1,
			Items:        []web.OrderItemRequest{{MenuItemID: 1, Quantity: 1}},
		}
		payload, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewReader(payload))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d", rec.Code)
		}
		var created domain.Order
		if err := json.NewDecoder(rec.Body).Decode(&created); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if created.ID != 42 || created.Status != domain.StatusCreated {
			t.Errorf("unexpected created order: %+v", created)
		}
	})
}

func TestCancelOrder(t *testing.T) {
	t.Run("invalid order id", func(t *testing.T) {
		router := setupTestServer(&mockOrderUseCase{}, &mockPartnerUseCase{})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/orders/invalid/cancel", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("missing user_id", func(t *testing.T) {
		router := setupTestServer(&mockOrderUseCase{}, &mockPartnerUseCase{})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/orders/1/cancel", bytes.NewBufferString("{}"))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("user_id from query fallback", func(t *testing.T) {
		orderUC := &mockOrderUseCase{
			cancelOrderFn: func(ctx context.Context, orderID int64, userID string) error {
				if orderID == 1 && userID == "user123" {
					return nil
				}
				return domain.ErrInvalidData
			},
		}
		router := setupTestServer(orderUC, &mockPartnerUseCase{})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/orders/1/cancel?user_id=user123", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("order not found", func(t *testing.T) {
		orderUC := &mockOrderUseCase{
			cancelOrderFn: func(ctx context.Context, orderID int64, userID string) error {
				return domain.ErrOrderNotFound
			},
		}
		router := setupTestServer(orderUC, &mockPartnerUseCase{})
		body := web.CancelOrderRequest{UserID: "user123"}
		payload, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/orders/1/cancel", bytes.NewReader(payload))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rec.Code)
		}
	})

	t.Run("access denied", func(t *testing.T) {
		orderUC := &mockOrderUseCase{
			cancelOrderFn: func(ctx context.Context, orderID int64, userID string) error {
				return domain.ErrAccessDenied
			},
		}
		router := setupTestServer(orderUC, &mockPartnerUseCase{})
		body := web.CancelOrderRequest{UserID: "other_user"}
		payload, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/orders/1/cancel", bytes.NewReader(payload))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", rec.Code)
		}
	})

	t.Run("already cancelled", func(t *testing.T) {
		orderUC := &mockOrderUseCase{
			cancelOrderFn: func(ctx context.Context, orderID int64, userID string) error {
				return domain.ErrOrderAlreadyCancelled
			},
		}
		router := setupTestServer(orderUC, &mockPartnerUseCase{})
		body := web.CancelOrderRequest{UserID: "user123"}
		payload, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/orders/1/cancel", bytes.NewReader(payload))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusConflict {
			t.Fatalf("expected 409, got %d", rec.Code)
		}
	})

	t.Run("invalid status transition (cooking)", func(t *testing.T) {
		orderUC := &mockOrderUseCase{
			cancelOrderFn: func(ctx context.Context, orderID int64, userID string) error {
				return domain.ErrInvalidStatusTransition
			},
		}
		router := setupTestServer(orderUC, &mockPartnerUseCase{})
		body := web.CancelOrderRequest{UserID: "user123"}
		payload, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/orders/1/cancel", bytes.NewReader(payload))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusConflict {
			t.Fatalf("expected 409, got %d", rec.Code)
		}
	})

	t.Run("success", func(t *testing.T) {
		orderUC := &mockOrderUseCase{
			cancelOrderFn: func(ctx context.Context, orderID int64, userID string) error {
				return nil
			},
		}
		router := setupTestServer(orderUC, &mockPartnerUseCase{})
		body := web.CancelOrderRequest{UserID: "user123"}
		payload, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/orders/1/cancel", bytes.NewReader(payload))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		var resp web.StatusResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Status != "cancelled" {
			t.Errorf("expected status 'cancelled', got '%s'", resp.Status)
		}
	})
}

func TestGetOrderByID(t *testing.T) {
	t.Run("invalid id", func(t *testing.T) {
		router := setupTestServer(&mockOrderUseCase{}, &mockPartnerUseCase{})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/orders/abc", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("order not found", func(t *testing.T) {
		orderUC := &mockOrderUseCase{
			getOrderByIDFn: func(ctx context.Context, orderID int64) (*domain.Order, error) {
				return nil, domain.ErrOrderNotFound
			},
		}
		router := setupTestServer(orderUC, &mockPartnerUseCase{})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/orders/999", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rec.Code)
		}
	})

	t.Run("success", func(t *testing.T) {
		orderUC := &mockOrderUseCase{
			getOrderByIDFn: func(ctx context.Context, orderID int64) (*domain.Order, error) {
				return &domain.Order{
					ID:              1,
					UserID:          "user123",
					RestaurantID:    2,
					Status:          domain.StatusAccepted,
					TotalPriceCents: 1000,
				}, nil
			},
		}
		router := setupTestServer(orderUC, &mockPartnerUseCase{})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/orders/1", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		var order domain.Order
		if err := json.NewDecoder(rec.Body).Decode(&order); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if order.ID != 1 || order.Status != domain.StatusAccepted {
			t.Errorf("unexpected order: %+v", order)
		}
	})
}

func TestGetUserOrders(t *testing.T) {
	t.Run("missing user_id", func(t *testing.T) {
		router := setupTestServer(&mockOrderUseCase{}, &mockPartnerUseCase{})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/orders", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("success with orders", func(t *testing.T) {
		orderUC := &mockOrderUseCase{
			getUserOrdersFn: func(ctx context.Context, userID string) ([]domain.Order, error) {
				return []domain.Order{
					{ID: 1, UserID: userID, Status: domain.StatusCreated},
					{ID: 2, UserID: userID, Status: domain.StatusCompleted},
				}, nil
			},
		}
		router := setupTestServer(orderUC, &mockPartnerUseCase{})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/orders?user_id=user123", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		var orders []domain.Order
		if err := json.NewDecoder(rec.Body).Decode(&orders); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if len(orders) != 2 {
			t.Fatalf("expected 2 orders, got %d", len(orders))
		}
	})
}

func TestPartnerAuthMiddleware(t *testing.T) {
	t.Run("missing X-API-KEY header", func(t *testing.T) {
		router := setupTestServer(&mockOrderUseCase{}, &mockPartnerUseCase{})
		req := httptest.NewRequest(http.MethodGet, "/api/v1/partner/orders", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("invalid api key", func(t *testing.T) {
		partnerUC := &mockPartnerUseCase{
			getRestaurantByAPIKeyFn: func(ctx context.Context, apiKey string) (*domain.Restaurant, error) {
				return nil, domain.ErrRestaurantNotFound
			},
		}
		router := setupTestServer(&mockOrderUseCase{}, partnerUC)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/partner/orders", nil)
		req.Header.Set("X-API-KEY", "bad_key")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("inactive restaurant", func(t *testing.T) {
		partnerUC := &mockPartnerUseCase{
			getRestaurantByAPIKeyFn: func(ctx context.Context, apiKey string) (*domain.Restaurant, error) {
				return nil, domain.ErrRestaurantUnavailable
			},
		}
		router := setupTestServer(&mockOrderUseCase{}, partnerUC)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/partner/orders", nil)
		req.Header.Set("X-API-KEY", "key_inactive")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", rec.Code)
		}
	})

	t.Run("internal server error in auth", func(t *testing.T) {
		partnerUC := &mockPartnerUseCase{
			getRestaurantByAPIKeyFn: func(ctx context.Context, apiKey string) (*domain.Restaurant, error) {
				return nil, errors.New("connection failed")
			},
		}
		router := setupTestServer(&mockOrderUseCase{}, partnerUC)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/partner/orders", nil)
		req.Header.Set("X-API-KEY", "some_key")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", rec.Code)
		}
	})
}

func TestPartnerUpsertMenu(t *testing.T) {
	partnerUC := &mockPartnerUseCase{
		getRestaurantByAPIKeyFn: func(ctx context.Context, apiKey string) (*domain.Restaurant, error) {
			return &domain.Restaurant{ID: 5, Name: "Pizza House", IsActive: true}, nil
		},
		upsertMenuFn: func(ctx context.Context, restaurantID int64, items []domain.MenuItem) error {
			if restaurantID != 5 {
				return domain.ErrInvalidData
			}
			return nil
		},
	}
	router := setupTestServer(&mockOrderUseCase{}, partnerUC)

	t.Run("malformed body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/v1/partner/menu", bytes.NewBufferString("{bad}"))
		req.Header.Set("X-API-KEY", "secret")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("empty items", func(t *testing.T) {
		body := web.UpsertMenuRequest{Items: []web.MenuItemDTO{}}
		payload, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPut, "/api/v1/partner/menu", bytes.NewReader(payload))
		req.Header.Set("X-API-KEY", "secret")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("success", func(t *testing.T) {
		body := web.UpsertMenuRequest{
			Items: []web.MenuItemDTO{
				{ExternalID: "p1", Name: "Pizza", PriceCents: 50000, IsAvailable: true},
			},
		}
		payload, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPut, "/api/v1/partner/menu", bytes.NewReader(payload))
		req.Header.Set("X-API-KEY", "secret")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		var resp web.StatusResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Status != "ok" {
			t.Errorf("expected status 'ok', got '%s'", resp.Status)
		}
	})
}

func TestPartnerGetOrders(t *testing.T) {
	partnerUC := &mockPartnerUseCase{
		getRestaurantByAPIKeyFn: func(ctx context.Context, apiKey string) (*domain.Restaurant, error) {
			return &domain.Restaurant{ID: 5, Name: "Pizza House", IsActive: true}, nil
		},
		getOrdersFn: func(ctx context.Context, restaurantID int64, status *domain.OrderStatus) ([]domain.Order, error) {
			if status != nil && *status == domain.StatusCreated {
				return []domain.Order{{ID: 10, RestaurantID: restaurantID, Status: domain.StatusCreated}}, nil
			}
			return []domain.Order{
				{ID: 10, RestaurantID: restaurantID, Status: domain.StatusCreated},
				{ID: 11, RestaurantID: restaurantID, Status: domain.StatusAccepted},
			}, nil
		},
	}
	router := setupTestServer(&mockOrderUseCase{}, partnerUC)

	t.Run("invalid status param", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/partner/orders?status=bad_status", nil)
		req.Header.Set("X-API-KEY", "secret")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("with valid status filter", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/partner/orders?status=created", nil)
		req.Header.Set("X-API-KEY", "secret")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		var orders []domain.Order
		if err := json.NewDecoder(rec.Body).Decode(&orders); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if len(orders) != 1 || orders[0].Status != domain.StatusCreated {
			t.Errorf("unexpected orders: %+v", orders)
		}
	})

	t.Run("all orders without filter", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/partner/orders", nil)
		req.Header.Set("X-API-KEY", "secret")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		var orders []domain.Order
		if err := json.NewDecoder(rec.Body).Decode(&orders); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if len(orders) != 2 {
			t.Errorf("expected 2 orders, got %d", len(orders))
		}
	})
}

func TestPartnerUpdateOrderStatus(t *testing.T) {
	partnerUC := &mockPartnerUseCase{
		getRestaurantByAPIKeyFn: func(ctx context.Context, apiKey string) (*domain.Restaurant, error) {
			return &domain.Restaurant{ID: 5, Name: "Pizza House", IsActive: true}, nil
		},
		updateOrderStatusFn: func(ctx context.Context, restaurantID int64, orderID int64, newStatus domain.OrderStatus) error {
			if orderID == 999 {
				return domain.ErrOrderNotFound
			}
			if orderID == 777 {
				return domain.ErrAccessDenied
			}
			if orderID == 888 {
				return domain.ErrOrderAlreadyCancelled
			}
			if orderID == 666 {
				return domain.ErrInvalidStatusTransition
			}
			return nil
		},
	}
	router := setupTestServer(&mockOrderUseCase{}, partnerUC)

	t.Run("invalid order id", func(t *testing.T) {
		body := web.UpdateStatusRequest{Status: domain.StatusAccepted}
		payload, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/partner/orders/invalid/status", bytes.NewReader(payload))
		req.Header.Set("X-API-KEY", "secret")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("malformed body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/partner/orders/1/status", bytes.NewBufferString("{bad}"))
		req.Header.Set("X-API-KEY", "secret")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("invalid status", func(t *testing.T) {
		body := map[string]string{"status": "flying"}
		payload, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/partner/orders/1/status", bytes.NewReader(payload))
		req.Header.Set("X-API-KEY", "secret")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("access denied", func(t *testing.T) {
		body := web.UpdateStatusRequest{Status: domain.StatusAccepted}
		payload, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/partner/orders/777/status", bytes.NewReader(payload))
		req.Header.Set("X-API-KEY", "secret")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", rec.Code)
		}
	})

	t.Run("conflict order already cancelled", func(t *testing.T) {
		body := web.UpdateStatusRequest{Status: domain.StatusAccepted}
		payload, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/partner/orders/888/status", bytes.NewReader(payload))
		req.Header.Set("X-API-KEY", "secret")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusConflict {
			t.Fatalf("expected 409, got %d", rec.Code)
		}
	})

	t.Run("conflict invalid transition", func(t *testing.T) {
		body := web.UpdateStatusRequest{Status: domain.StatusCompleted}
		payload, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/partner/orders/666/status", bytes.NewReader(payload))
		req.Header.Set("X-API-KEY", "secret")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusConflict {
			t.Fatalf("expected 409, got %d", rec.Code)
		}
	})

	t.Run("success", func(t *testing.T) {
		body := web.UpdateStatusRequest{Status: domain.StatusAccepted}
		payload, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/partner/orders/1/status", bytes.NewReader(payload))
		req.Header.Set("X-API-KEY", "secret")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		var resp web.StatusResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Status != "ok" {
			t.Errorf("expected status 'ok', got '%s'", resp.Status)
		}
	})
}
