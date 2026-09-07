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
	getActiveMenuByRestaurantID    func(ctx context.Context, restaurantID int64) ([]MenuItem, error)
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

func (m *mockRepo) GetActiveMenuByRestaurantID(ctx context.Context, restaurantID int64) ([]MenuItem, error) {
	if m.getActiveMenuByRestaurantID != nil {
		return m.getActiveMenuByRestaurantID(ctx, restaurantID)
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
			getActiveMenuByRestaurantID: func(ctx context.Context, restaurantID int64) ([]MenuItem, error) {
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
		if !errors.Is(err, ErrInvalidStatusTransition) {
			t.Fatalf("expected ErrInvalidStatusTransition, got %v", err)
		}
	})

	t.Run("already cancelled", func(t *testing.T) {
		repo := &mockRepo{
			getOrderByID: func(ctx context.Context, orderID int64) (*Order, error) {
				return &Order{ID: 1, UserID: "user-1", Status: StatusCancelled}, nil
			},
		}
		service := NewOrderService(repo)
		err := service.CancelOrder(context.Background(), 1, "user-1")
		if !errors.Is(err, ErrOrderAlreadyCancelled) {
			t.Fatalf("expected ErrOrderAlreadyCancelled, got %v", err)
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

func TestCreateOrder(t *testing.T) {
	t.Run("invalid data", func(t *testing.T) {
		service := NewOrderService(&mockRepo{})
		_, err := service.CreateOrder(context.Background(), "", 1, []OrderItemCreate{{MenuItemID: 1, Quantity: 1}})
		if !errors.Is(err, ErrInvalidData) {
			t.Fatalf("expected ErrInvalidData, got %v", err)
		}

		_, err = service.CreateOrder(context.Background(), "user-1", 1, nil)
		if !errors.Is(err, ErrInvalidData) {
			t.Fatalf("expected ErrInvalidData, got %v", err)
		}
	})

	t.Run("inactive restaurant", func(t *testing.T) {
		repo := &mockRepo{
			getRestaurantByID: func(ctx context.Context, id int64) (*Restaurant, error) {
				return &Restaurant{ID: 1, IsActive: false}, nil
			},
		}
		service := NewOrderService(repo)
		_, err := service.CreateOrder(context.Background(), "user-1", 1, []OrderItemCreate{{MenuItemID: 1, Quantity: 1}})
		if !errors.Is(err, ErrRestaurantUnavailable) {
			t.Fatalf("expected ErrRestaurantUnavailable, got %v", err)
		}
	})

	t.Run("menu item not found", func(t *testing.T) {
		repo := &mockRepo{
			getRestaurantByID: func(ctx context.Context, id int64) (*Restaurant, error) {
				return &Restaurant{ID: 1, IsActive: true}, nil
			},
			getMenuByRestaurantID: func(ctx context.Context, restaurantID int64) ([]MenuItem, error) {
				return []MenuItem{}, nil
			},
		}
		service := NewOrderService(repo)
		_, err := service.CreateOrder(context.Background(), "user-1", 1, []OrderItemCreate{{MenuItemID: 999, Quantity: 1}})
		if !errors.Is(err, ErrMenuItemNotFound) {
			t.Fatalf("expected ErrMenuItemNotFound, got %v", err)
		}
	})

	t.Run("menu item unavailable in stop-list", func(t *testing.T) {
		repo := &mockRepo{
			getRestaurantByID: func(ctx context.Context, id int64) (*Restaurant, error) {
				return &Restaurant{ID: 1, IsActive: true}, nil
			},
			getMenuByRestaurantID: func(ctx context.Context, restaurantID int64) ([]MenuItem, error) {
				return []MenuItem{
					{ID: 10, Name: "Pizza", PriceCents: 1000, IsAvailable: false},
				}, nil
			},
		}
		service := NewOrderService(repo)
		_, err := service.CreateOrder(context.Background(), "user-1", 1, []OrderItemCreate{{MenuItemID: 10, Quantity: 1}})
		if !errors.Is(err, ErrMenuItemUnavailable) {
			t.Fatalf("expected ErrMenuItemUnavailable, got %v", err)
		}
	})

	t.Run("success with deduplication and price calculation", func(t *testing.T) {
		created := false
		repo := &mockRepo{
			getRestaurantByID: func(ctx context.Context, id int64) (*Restaurant, error) {
				return &Restaurant{ID: 1, IsActive: true}, nil
			},
			getMenuByRestaurantID: func(ctx context.Context, restaurantID int64) ([]MenuItem, error) {
				return []MenuItem{
					{ID: 10, Name: "Burger", PriceCents: 500, IsAvailable: true},
					{ID: 20, Name: "Cola", PriceCents: 200, IsAvailable: true},
				}, nil
			},
			createOrder: func(ctx context.Context, order *Order) error {
				created = true
				order.ID = 42
				return nil
			},
		}
		service := NewOrderService(repo)
		items := []OrderItemCreate{
			{MenuItemID: 10, Quantity: 2},
			{MenuItemID: 10, Quantity: 1}, // duplicate
			{MenuItemID: 20, Quantity: 2},
		}
		order, err := service.CreateOrder(context.Background(), "user-1", 1, items)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !created || order.ID != 42 {
			t.Fatalf("expected order to be created with ID 42, got %v", order)
		}
		if order.TotalPriceCents != 1900 {
			t.Fatalf("expected TotalPriceCents 1900, got %d", order.TotalPriceCents)
		}
		if len(order.Items) != 2 {
			t.Fatalf("expected 2 unique items, got %d", len(order.Items))
		}
		if order.Items[0].Quantity != 3 || order.Items[0].UnitPriceCents != 500 {
			t.Fatalf("unexpected burger item: %+v", order.Items[0])
		}
		if order.Items[1].Quantity != 2 || order.Items[1].UnitPriceCents != 200 {
			t.Fatalf("unexpected cola item: %+v", order.Items[1])
		}
	})
}

func TestPartnerService(t *testing.T) {
	t.Run("UpdateOrderStatus - order already cancelled", func(t *testing.T) {
		repo := &mockRepo{
			getOrderByID: func(ctx context.Context, orderID int64) (*Order, error) {
				return &Order{ID: 1, RestaurantID: 10, Status: StatusCancelled}, nil
			},
		}
		service := NewPartnerService(repo)
		err := service.UpdateOrderStatus(context.Background(), 10, 1, StatusAccepted)
		if !errors.Is(err, ErrOrderAlreadyCancelled) {
			t.Fatalf("expected ErrOrderAlreadyCancelled, got %v", err)
		}
	})

	t.Run("UpdateOrderStatus - idempotent same status", func(t *testing.T) {
		repo := &mockRepo{
			getOrderByID: func(ctx context.Context, orderID int64) (*Order, error) {
				return &Order{ID: 1, RestaurantID: 10, Status: StatusAccepted}, nil
			},
		}
		service := NewPartnerService(repo)
		err := service.UpdateOrderStatus(context.Background(), 10, 1, StatusAccepted)
		if err != nil {
			t.Fatalf("expected nil for same status, got %v", err)
		}
	})

	t.Run("UpdateOrderStatus - invalid transition", func(t *testing.T) {
		repo := &mockRepo{
			getOrderByID: func(ctx context.Context, orderID int64) (*Order, error) {
				return &Order{ID: 1, RestaurantID: 10, Status: StatusCreated}, nil
			},
		}
		service := NewPartnerService(repo)
		err := service.UpdateOrderStatus(context.Background(), 10, 1, StatusCompleted)
		if !errors.Is(err, ErrInvalidStatusTransition) {
			t.Fatalf("expected ErrInvalidStatusTransition, got %v", err)
		}
	})

	t.Run("GetRestaurantByAPIKey - inactive restaurant", func(t *testing.T) {
		repo := &mockRepo{
			getRestaurantByAPIKey: func(ctx context.Context, apiKey string) (*Restaurant, error) {
				return &Restaurant{ID: 1, IsActive: false}, nil
			},
		}
		service := NewPartnerService(repo)
		_, err := service.GetRestaurantByAPIKey(context.Background(), "key1")
		if !errors.Is(err, ErrRestaurantUnavailable) {
			t.Fatalf("expected ErrRestaurantUnavailable, got %v", err)
		}
	})

	t.Run("UpsertMenu - duplicate external ID or negative price", func(t *testing.T) {
		service := NewPartnerService(&mockRepo{})
		err := service.UpsertMenu(context.Background(), 1, []MenuItem{
			{ExternalID: "item1", Name: "Burger", PriceCents: -10},
		})
		if !errors.Is(err, ErrInvalidData) {
			t.Fatalf("expected ErrInvalidData for negative price, got %v", err)
		}

		err = service.UpsertMenu(context.Background(), 1, []MenuItem{
			{ExternalID: "item1", Name: "Burger", PriceCents: 500},
			{ExternalID: "item1 ", Name: "Burger Duplicate", PriceCents: 600},
		})
		if !errors.Is(err, ErrInvalidData) {
			t.Fatalf("expected ErrInvalidData for duplicate external_id, got %v", err)
		}
	})

	t.Run("UpsertMenu - success", func(t *testing.T) {
		upserted := false
		repo := &mockRepo{
			upsertMenuItems: func(ctx context.Context, restaurantID int64, items []MenuItem) error {
				if restaurantID == 1 && len(items) == 1 && items[0].RestaurantID == 1 {
					upserted = true
					return nil
				}
				return errors.New("unexpected params")
			},
		}
		service := NewPartnerService(repo)
		err := service.UpsertMenu(context.Background(), 1, []MenuItem{
			{ExternalID: "item1", Name: "Burger", PriceCents: 500},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !upserted {
			t.Fatal("expected menu items to be upserted")
		}
	})
}
