package domain

import (
	"context"
	"strings"
	"time"
)

type OrderService struct {
	repo Repository
}

type PartnerService struct {
	repo Repository
}

func NewOrderService(repo Repository) *OrderService {
	return &OrderService{
		repo: repo,
	}
}

func NewPartnerService(repo Repository) *PartnerService {
	return &PartnerService{
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

	ans, err := o.repo.GetActiveMenuByRestaurantID(ctx, restaurantID)
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
		if !ok {
			return nil, ErrMenuItemNotFound
		}
		if !menuItem.IsAvailable {
			return nil, ErrMenuItemUnavailable
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
	if len(userID) == 0 || orderID <= 0 {
		return ErrInvalidData
	}

	order, err := o.repo.GetOrderByID(ctx, orderID)
	if err != nil {
		return err
	}
	if order.UserID != userID {
		return ErrAccessDenied
	}
	if order.Status == StatusCancelled {
		return ErrOrderAlreadyCancelled
	}
	if !CanTransition(order.Status, StatusCancelled) {
		return ErrInvalidStatusTransition
	}

	err = o.repo.UpdateOrderStatus(ctx, orderID, StatusCancelled)
	return err
}

func (p *PartnerService) GetOrders(ctx context.Context, restaurantID int64, status *OrderStatus) ([]Order, error) {
	if restaurantID <= 0 {
		return nil, ErrInvalidData
	}
	if status != nil && !status.Validate() {
		return nil, ErrInvalidData
	}

	ans, err := p.repo.GetOrdersByRestaurantAndStatus(ctx, restaurantID, status)
	if err != nil {
		return nil, err
	}

	return ans, nil
}

func (p *PartnerService) UpdateOrderStatus(ctx context.Context, restaurantID int64, orderID int64, newStatus OrderStatus) error {
	if restaurantID <= 0 || orderID <= 0 || !newStatus.Validate() {
		return ErrInvalidData
	}

	order, err := p.repo.GetOrderByID(ctx, orderID)
	if err != nil {
		return err
	}
	if order.RestaurantID != restaurantID {
		return ErrAccessDenied
	}
	if order.Status == StatusCancelled {
		return ErrOrderAlreadyCancelled
	}
	if order.Status == newStatus {
		return nil
	}
	if !CanTransition(order.Status, newStatus) {
		return ErrInvalidStatusTransition
	}

	err = p.repo.UpdateOrderStatus(ctx, orderID, newStatus)
	return err
}

func (p *PartnerService) GetRestaurantByAPIKey(ctx context.Context, apiKey string) (*Restaurant, error) {
	apiKey = strings.TrimSpace(apiKey)

	if len(apiKey) == 0 {
		return nil, ErrInvalidData
	}

	ans, err := p.repo.GetRestaurantByAPIKey(ctx, apiKey)
	if err != nil {
		return nil, err
	}
	if !ans.IsActive {
		return nil, ErrRestaurantUnavailable
	}
	return ans, nil
}

func (p *PartnerService) UpsertMenu(ctx context.Context, restaurantID int64, items []MenuItem) error {
	if restaurantID <= 0 || len(items) == 0 {
		return ErrInvalidData
	}

	seen := make(map[string]struct{}, len(items))
	for i := range items {
		extID := strings.TrimSpace(items[i].ExternalID)
		name := strings.TrimSpace(items[i].Name)
		if extID == "" || name == "" || items[i].PriceCents < 0 {
			return ErrInvalidData
		}
		if _, exists := seen[extID]; exists {
			return ErrInvalidData
		}
		seen[extID] = struct{}{}

		items[i].ExternalID = extID
		items[i].Name = name
		items[i].RestaurantID = restaurantID
	}

	err := p.repo.UpsertMenuItems(ctx, restaurantID, items)
	return err
}
