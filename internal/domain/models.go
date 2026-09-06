package domain

import "time"

type OrderStatus string

const (
	StatusCreated   OrderStatus = "created"
	StatusAccepted  OrderStatus = "accepted"
	StatusRejected  OrderStatus = "rejected"
	StatusCooking   OrderStatus = "cooking"
	StatusReady     OrderStatus = "ready"
	StatusCompleted OrderStatus = "completed"
	StatusCancelled OrderStatus = "cancelled"
)

type Restaurant struct {
	ID          int64     `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	Description string    `json:"description" db:"description"`
	APIKey      string    `json:"-" db:"api_key"`
	IsActive    bool      `json:"is_active" db:"is_active"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

type MenuItem struct {
	ID           int64     `json:"id" db:"id"`
	RestaurantID int64     `json:"restaurant_id" db:"restaurant_id"`
	ExternalID   string    `json:"external_id" db:"external_id"`
	Name         string    `json:"name" db:"name"`
	PriceCents   int64     `json:"price_cents" db:"price_cents"`
	IsAvailable  bool      `json:"is_available" db:"is_available"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

type Order struct {
	ID              int64       `json:"id" db:"id"`
	UserID          string      `json:"user_id" db:"user_id"`
	RestaurantID    int64       `json:"restaurant_id" db:"restaurant_id"`
	Status          OrderStatus `json:"status" db:"status"`
	TotalPriceCents int64       `json:"total_price_cents" db:"total_price_cents"`
	CreatedAt       time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time   `json:"updated_at" db:"updated_at"`
	Items           []OrderItem `json:"items,omitempty" db:"items,omitempty"`
}

type OrderItem struct {
	ID             int64     `json:"id" db:"id"`
	OrderID        int64     `json:"order_id" db:"order_id"`
	MenuItemID     int64     `json:"menu_item_id" db:"menu_item_id"`
	Quantity       int       `json:"quantity" db:"quantity"`
	UnitPriceCents int64     `json:"unit_price_cents" db:"unit_price_cents"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
}

type OrderItemCreate struct {
	MenuItemID int64
	Quantity   int
}
