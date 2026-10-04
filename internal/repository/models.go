package repository

import (
	"time"

	"kitchen-api/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repo struct {
	Data        *pgxpool.Pool
	writerTopic string
	readerTopic string
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

type OutboxItemRow struct {
	ID          uuid.UUID  `db:"id"`
	Topic       string     `db:"topic"`
	Payload     []byte     `db:"payload"`
	EventKey    string     `db:"event_key"`
	Headers     []byte     `db:"headers"`
	CreatedAt   time.Time  `db:"created_at"`
	ProcessedAt *time.Time `db:"processed_at"`
}

type OrderCreatedEvent struct {
	EventID         string             `json:"event_id"`
	OrderID         int64              `json:"order_id"`
	RestaurantID    int64              `json:"restaurant_id"`
	Items           []domain.OrderItem `json:"items"`
	TotalPriceCents int64              `json:"total_price_cents"`
	CreatedAt       time.Time          `json:"created_at"`
}

type OrderStatusUpdatedEvent struct {
	EventID      string    `json:"event_id"`
	OrderID      int64     `json:"order_id"`
	RestaurantID int64     `json:"restaurant_id"`
	Status       string    `json:"status"`
	UpdatedAt    time.Time `json:"updated_at"`
}
