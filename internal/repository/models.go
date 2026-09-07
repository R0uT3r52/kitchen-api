package repository

import (
	"time"

	"kitchen-api/internal/domain"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repo struct {
	Data *pgxpool.Pool
}

type OrderEvent struct {
	ID        int64               `db:"id"`
	OrderID   int64               `db:"order_id"`
	OldStatus *domain.OrderStatus `db:"old_status"`
	NewStatus domain.OrderStatus  `db:"new_status"`
	CreatedAt time.Time           `db:"created_at"`
}

type restaurantRow struct {
	ID          int64     `db:"id"`
	Name        string    `db:"name"`
	Description string    `db:"description"`
	APIKey      string    `db:"api_key"`
	IsActive    bool      `db:"is_active"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

type menuItemRow struct {
	ID           int64     `db:"id"`
	RestaurantID int64     `db:"restaurant_id"`
	ExternalID   string    `db:"external_id"`
	Name         string    `db:"name"`
	PriceCents   int64     `db:"price_cents"`
	IsAvailable  bool      `db:"is_available"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`
}

type orderRow struct {
	ID              int64     `db:"id"`
	UserID          string    `db:"user_id"`
	RestaurantID    int64     `db:"restaurant_id"`
	Status          string    `db:"status"`
	TotalPriceCents int64     `db:"total_price_cents"`
	CreatedAt       time.Time `db:"created_at"`
	UpdatedAt       time.Time `db:"updated_at"`
}

type orderItemRow struct {
	ID             int64     `db:"id"`
	OrderID        int64     `db:"order_id"`
	MenuItemID     int64     `db:"menu_item_id"`
	Quantity       int       `db:"quantity"`
	UnitPriceCents int64     `db:"unit_price_cents"`
	CreatedAt      time.Time `db:"created_at"`
}
