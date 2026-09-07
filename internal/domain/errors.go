package domain

import "errors"

var (
	ErrInvalidData             = errors.New("invalid request data")
	ErrRestaurantNotFound      = errors.New("restaurant not found")
	ErrRestaurantUnavailable   = errors.New("restaurant is unavailable")
	ErrMenuItemNotFound        = errors.New("menu item not found")
	ErrMenuItemUnavailable     = errors.New("menu item is unavailable")
	ErrInvalidQuantity         = errors.New("incorrect order item quantity")
	ErrOrderNotFound           = errors.New("order not found")
	ErrAccessDenied            = errors.New("access denied")
	ErrInvalidStatusTransition = errors.New("invalid order status transition")
	ErrOrderAlreadyCancelled   = errors.New("order has already been cancelled")
)
