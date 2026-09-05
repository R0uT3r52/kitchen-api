package repository

import (
	"context"

	"kitchen-api/internal/domain"
)

type Repository interface {
	GetActiveRestaurants(ctx context.Context) ([]domain.Restaurant, error)
	GetRestaurantByID(ctx context.Context, id int64) (*domain.Restaurant, error)
	GetRestaurantByAPIKey(ctx context.Context, apiKey string) (*domain.Restaurant, error)

	GetMenuByRestaurantID(ctx context.Context, restaurantID int64) ([]domain.MenuItem, error)
	UpsertMenuItems(ctx context.Context, restaurantID int64, items []domain.MenuItem) error

	CreateOrder(ctx context.Context, order *domain.Order) error
	GetOrderByID(ctx context.Context, orderID int64) (*domain.Order, error)
	GetOrdersByUserID(ctx context.Context, userID string) ([]domain.Order, error)
	GetOrdersByRestaurantAndStatus(ctx context.Context, restaurantID int64, status domain.OrderStatus) ([]domain.Order, error)
	UpdateOrderStatus(ctx context.Context, orderID int64, newStatus domain.OrderStatus) error
}
