package domain

import (
	"context"
)

type OrderUseCase interface {
	GetActiveRestaurants(ctx context.Context) ([]Restaurant, error)
	GetRestaurantMenu(ctx context.Context, restaurantID int64) ([]MenuItem, error)
	CreateOrder(ctx context.Context, userID string, restaurantID int64, items []OrderItemCreate) (*Order, error)
	GetOrderByID(ctx context.Context, orderID int64) (*Order, error)
	GetUserOrders(ctx context.Context, userID string) ([]Order, error)
	CancelOrder(ctx context.Context, orderID int64, userID string) error
}

type PartnerUseCase interface {
	UpsertMenu(ctx context.Context, restaurantID int64, items []MenuItem) error
	GetPendingOrders(ctx context.Context, restaurantID int64) ([]Order, error)
	UpdateOrderStatus(ctx context.Context, restaurantID int64, orderID int64, newStatus OrderStatus) error
}

type Repository interface {
	GetActiveRestaurants(ctx context.Context) ([]Restaurant, error)
	GetRestaurantByID(ctx context.Context, id int64) (*Restaurant, error)
	GetRestaurantByAPIKey(ctx context.Context, apiKey string) (*Restaurant, error)

	GetMenuByRestaurantID(ctx context.Context, restaurantID int64) ([]MenuItem, error)
	UpsertMenuItems(ctx context.Context, restaurantID int64, items []MenuItem) error

	CreateOrder(ctx context.Context, order *Order) error
	GetOrderByID(ctx context.Context, orderID int64) (*Order, error)
	GetOrdersByUserID(ctx context.Context, userID string) ([]Order, error)
	GetOrdersByRestaurantAndStatus(ctx context.Context, restaurantID int64, status OrderStatus) ([]Order, error)
	UpdateOrderStatus(ctx context.Context, orderID int64, newStatus OrderStatus) error
}
