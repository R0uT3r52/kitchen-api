package domain

import "context"

type OrderUseCase interface {
	GetActiveRestaurants(ctx context.Context) ([]Restaurant, error)
	GetRestaurantMenu(ctx context.Context, restaurantID int64) ([]MenuItem, error)
	CreateOrder(ctx context.Context, userID string, restaurantID int64, items []OrderItem) (*Order, error)
	GetOrderByID(ctx context.Context, orderID int64) (*Order, error)
	GetUserOrders(ctx context.Context, userID string) ([]Order, error)
	CancelOrder(ctx context.Context, orderID int64, userID string) error
}

type PartnerUseCase interface {
	UpsertMenu(ctx context.Context, apiKey string, items []MenuItem) error
	GetPendingOrders(ctx context.Context, apiKey string) ([]Order, error)
	UpdateOrderStatus(ctx context.Context, apiKey string, orderID int64, newStatus OrderStatus) error
}
