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
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	APIKey      string    `json:"-"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type MenuItem struct {
	ID           int64     `json:"id"`
	RestaurantID int64     `json:"restaurant_id"`
	ExternalID   string    `json:"external_id"`
	Name         string    `json:"name"`
	PriceCents   int64     `json:"price_cents"`
	IsAvailable  bool      `json:"is_available"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Order struct {
	ID              int64       `json:"id"`
	UserID          string      `json:"user_id"`
	RestaurantID    int64       `json:"restaurant_id"`
	Status          OrderStatus `json:"status"`
	TotalPriceCents int64       `json:"total_price_cents"`
	CreatedAt       time.Time   `json:"created_at"`
	UpdatedAt       time.Time   `json:"updated_at"`
	Items           []OrderItem `json:"items,omitempty"`
}

type OrderItem struct {
	ID             int64     `json:"id"`
	OrderID        int64     `json:"order_id"`
	MenuItemID     int64     `json:"menu_item_id"`
	Quantity       int       `json:"quantity"`
	UnitPriceCents int64     `json:"unit_price_cents"`
	CreatedAt      time.Time `json:"created_at"`
}
