package domain

import (
	"context"
	"errors"
	"testing"
)

type mockRepo struct {
	getActiveRestaurants           func(ctx context.Context) ([]Restaurant, error)
	getRestaurantByID              func(ctx context.Context, id int64) (*Restaurant, error)
	getRestaurantByAPIKey          func(ctx context.Context, apiKey string) (*Restaurant, error)
	getMenuByRestaurantID          func(ctx context.Context, restaurantID int64) ([]MenuItem, error)
	upsertMenuItems                func(ctx context.Context, restaurantID int64, items []MenuItem) error
	createOrder                    func(ctx context.Context, order *Order) error
	getOrderByID                   func(ctx context.Context, orderID int64) (*Order, error)
	getOrdersByUserID              func(ctx context.Context, userID string) ([]Order, error)
	getOrdersByRestaurantAndStatus func(ctx context.Context, restaurantID int64, status *OrderStatus) ([]Order, error)
	updateOrderStatus              func(ctx context.Context, orderID int64, newStatus OrderStatus) error
}

func (m *mockRepo) GetActiveRestaurants(ctx context.Context) ([]Restaurant, error) {
	if m.getActiveRestaurants != nil {
		return m.getActiveRestaurants(ctx)
	}
	return nil, nil
}

func (m *mockRepo) GetRestaurantByID(ctx context.Context, id int64) (*Restaurant, error) {
	if m.getRestaurantByID != nil {
		return m.getRestaurantByID(ctx, id)
	}
	return nil, nil
}

func (m *mockRepo) GetRestaurantByAPIKey(ctx context.Context, apiKey string) (*Restaurant, error) {
	if m.getRestaurantByAPIKey != nil {
		return m.getRestaurantByAPIKey(ctx, apiKey)
	}
	return nil, nil
}

func (m *mockRepo) GetMenuByRestaurantID(ctx context.Context, restaurantID int64) ([]MenuItem, error) {
	if m.getMenuByRestaurantID != nil {
		return m.getMenuByRestaurantID(ctx, restaurantID)
	}
	return nil, nil
}

func (m *mockRepo) UpsertMenuItems(ctx context.Context, restaurantID int64, items []MenuItem) error {
	if m.upsertMenuItems != nil {
		return m.upsertMenuItems(ctx, restaurantID, items)
	}
	return nil
}

func (m *mockRepo) CreateOrder(ctx context.Context, order *Order) error {
	if m.createOrder != nil {
		return m.createOrder(ctx, order)
	}
	return nil
}

func (m *mockRepo) GetOrderByID(ctx context.Context, orderID int64) (*Order, error) {
	if m.getOrderByID != nil {
		return m.getOrderByID(ctx, orderID)
	}
	return nil, nil
}

func (m *mockRepo) GetOrdersByUserID(ctx context.Context, userID string) ([]Order, error) {
	if m.getOrdersByUserID != nil {
		return m.getOrdersByUserID(ctx, userID)
	}
	return nil, nil
}

func (m *mockRepo) GetOrdersByRestaurantAndStatus(ctx context.Context, restaurantID int64, status *OrderStatus) ([]Order, error) {
	if m.getOrdersByRestaurantAndStatus != nil {
		return m.getOrdersByRestaurantAndStatus(ctx, restaurantID, status)
	}
	return nil, nil
}

func (m *mockRepo) UpdateOrderStatus(ctx context.Context, orderID int64, newStatus OrderStatus) error {
	if m.updateOrderStatus != nil {
		return m.updateOrderStatus(ctx, orderID, newStatus)
	}
	return nil
}

func TestGetRestaurantMenu(t *testing.T) {
	t.Run("restaurant not found", func(t *testing.T) {
		repo := &mockRepo{
			getRestaurantByID: func(ctx context.Context, id int64) (*Restaurant, error) {
				return nil, ErrRestaurantNotFound
			},
		}
		service := NewOrderService(repo)
		_, err := service.GetRestaurantMenu(context.Background(), 1)
		if !errors.Is(err, ErrRestaurantNotFound) {
			t.Fatalf("expected ErrRestaurantNotFound, got %v", err)
		}
	})

	t.Run("restaurant inactive", func(t *testing.T) {
		repo := &mockRepo{
			getRestaurantByID: func(ctx context.Context, id int64) (*Restaurant, error) {
				return &Restaurant{ID: 1, IsActive: false}, nil
			},
		}
		service := NewOrderService(repo)
		_, err := service.GetRestaurantMenu(context.Background(), 1)
		if !errors.Is(err, ErrRestaurantUnavailable) {
			t.Fatalf("expected ErrRestaurantUnavailable, got %v", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		repo := &mockRepo{
			getRestaurantByID: func(ctx context.Context, id int64) (*Restaurant, error) {
				return &Restaurant{ID: 1, IsActive: true}, nil
			},
			getMenuByRestaurantID: func(ctx context.Context, restaurantID int64) ([]MenuItem, error) {
				return []MenuItem{
					{ID: 10, Name: "Burger", PriceCents: 500, IsAvailable: true},
				}, nil
			},
		}
		service := NewOrderService(repo)
		items, err := service.GetRestaurantMenu(context.Background(), 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(items) != 1 || items[0].Name != "Burger" {
			t.Fatalf("unexpected items: %+v", items)
		}
	})
}

func TestCancelOrder(t *testing.T) {
	t.Run("access denied if user mismatch", func(t *testing.T) {
		repo := &mockRepo{
			getOrderByID: func(ctx context.Context, orderID int64) (*Order, error) {
				return &Order{ID: 1, UserID: "user-1", Status: StatusCreated}, nil
			},
		}
		service := NewOrderService(repo)
		err := service.CancelOrder(context.Background(), 1, "user-2")
		if !errors.Is(err, ErrAccessDenied) {
			t.Fatalf("expected ErrAccessDenied, got %v", err)
		}
	})

	t.Run("invalid status when already cooking", func(t *testing.T) {
		repo := &mockRepo{
			getOrderByID: func(ctx context.Context, orderID int64) (*Order, error) {
				return &Order{ID: 1, UserID: "user-1", Status: StatusCooking}, nil
			},
		}
		service := NewOrderService(repo)
		err := service.CancelOrder(context.Background(), 1, "user-1")
		if !errors.Is(err, ErrInvalidOrderStatus) {
			t.Fatalf("expected ErrInvalidOrderStatus, got %v", err)
		}
	})

	t.Run("success when created", func(t *testing.T) {
		updated := false
		repo := &mockRepo{
			getOrderByID: func(ctx context.Context, orderID int64) (*Order, error) {
				return &Order{ID: 1, UserID: "user-1", Status: StatusCreated}, nil
			},
			updateOrderStatus: func(ctx context.Context, orderID int64, newStatus OrderStatus) error {
				if orderID == 1 && newStatus == StatusCancelled {
					updated = true
					return nil
				}
				return errors.New("unexpected params")
			},
		}
		service := NewOrderService(repo)
		err := service.CancelOrder(context.Background(), 1, "user-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !updated {
			t.Fatal("expected order to be updated to cancelled")
		}
	})
}
