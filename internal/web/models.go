package web

import "kitchen-api/internal/domain"

type OrderItemRequest struct {
	MenuItemID int64 `json:"menu_item_id"`
	Quantity   int   `json:"quantity"`
}

type CreateOrderRequest struct {
	UserID       string             `json:"user_id"`
	RestaurantID int64              `json:"restaurant_id"`
	Items        []OrderItemRequest `json:"items"`
}

type UpdateStatusRequest struct {
	Status domain.OrderStatus `json:"status"`
}

type MenuItemDTO struct {
	ExternalID  string `json:"external_id"`
	Name        string `json:"name"`
	PriceCents  int64  `json:"price_cents"`
	IsAvailable bool   `json:"is_available"`
}

type UpsertMenuRequest struct {
	Items []MenuItemDTO `json:"items"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}
