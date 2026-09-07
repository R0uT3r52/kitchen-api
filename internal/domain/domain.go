package domain

import (
	"context"
	"strings"
	"time"
)

type OrderService struct {
	repo Repository
}

func NewOrderService(repo Repository) *OrderService {
	return &OrderService{
		repo: repo,
	}
}

func (o *OrderService) GetActiveRestaurants(ctx context.Context) ([]Restaurant, error) {
	ans, err := o.repo.GetActiveRestaurants(ctx)
	if err != nil {
		return nil, err
	}

	return ans, nil
}

func (o *OrderService) GetRestaurantMenu(ctx context.Context, restaurantID int64) ([]MenuItem, error) {
	r, err := o.repo.GetRestaurantByID(ctx, restaurantID)
	if err != nil {
		return nil, err
	}
	if !r.IsActive {
		return nil, ErrRestaurantUnavailable
	}

	ans, err := o.repo.GetMenuByRestaurantID(ctx, restaurantID)
	if err != nil {
		return nil, err
	}

	return ans, nil
}

func (o *OrderService) CreateOrder(ctx context.Context, userID string, restaurantID int64, items []OrderItemCreate) (*Order, error) {
	userID = strings.TrimSpace(userID)

	if len(items) == 0 || len(userID) == 0 {
		return nil, ErrInvalidData
	}

	r, err := o.repo.GetRestaurantByID(ctx, restaurantID)
	if err != nil {
		return nil, err
	}
	if !r.IsActive {
		return nil, ErrRestaurantUnavailable
	}

	menu, err := o.repo.GetMenuByRestaurantID(ctx, restaurantID)
	if err != nil {
		return nil, err
	}

	menuMap := make(map[int64]MenuItem, len(menu))
	for i := range menu {
		menuMap[menu[i].ID] = menu[i]
	}

	dedupMap := make(map[int64]int)
	uniqueIDs := make([]int64, 0, len(items))

	for _, item := range items {
		if item.Quantity <= 0 {
			return nil, ErrInvalidQuantity
		}
		if _, exists := dedupMap[item.MenuItemID]; !exists {
			uniqueIDs = append(uniqueIDs, item.MenuItemID)
		}
		dedupMap[item.MenuItemID] += item.Quantity
	}

	totalPrice := int64(0)
	orderItems := make([]OrderItem, 0, len(uniqueIDs))

	for _, menuItemID := range uniqueIDs {
		qty := dedupMap[menuItemID]
		menuItem, ok := menuMap[menuItemID]
		if !ok || !menuItem.IsAvailable {
			return nil, ErrMenuItemNotFound
		}

		totalPrice += menuItem.PriceCents * int64(qty)

		orderItems = append(orderItems, OrderItem{
			MenuItemID:     menuItem.ID,
			Quantity:       qty,
			UnitPriceCents: menuItem.PriceCents,
		})
	}

	now := time.Now()
	order := Order{
		UserID:          userID,
		RestaurantID:    restaurantID,
		Status:          StatusCreated,
		TotalPriceCents: totalPrice,
		CreatedAt:       now,
		UpdatedAt:       now,
		Items:           orderItems,
	}

	err = o.repo.CreateOrder(ctx, &order)
	if err != nil {
		return nil, err
	}

	return &order, nil
}

func (o *OrderService) GetOrderByID(ctx context.Context, orderID int64) (*Order, error) {
	ans, err := o.repo.GetOrderByID(ctx, orderID)
	if err != nil {
		return nil, err
	}

	return ans, nil
}

func (o *OrderService) GetUserOrders(ctx context.Context, userID string) ([]Order, error) {
	userID = strings.TrimSpace(userID)
	if len(userID) == 0 {
		return nil, ErrInvalidData
	}
	ans, err := o.repo.GetOrdersByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	return ans, nil
}

func (o *OrderService) CancelOrder(ctx context.Context, orderID int64, userID string) error {
	userID = strings.TrimSpace(userID)
	if len(userID) == 0 {
		return ErrInvalidData
	}

	order, err := o.repo.GetOrderByID(ctx, orderID)
	if err != nil {
		return err
	}
	if order.UserID != userID {
		return ErrAccessDenied
	}
	if order.Status != StatusCreated && order.Status != StatusAccepted {
		return ErrInvalidOrderStatus
	}

	err = o.repo.UpdateOrderStatus(ctx, orderID, StatusCancelled)
	return err
}
