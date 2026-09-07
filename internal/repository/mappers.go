package repository

import "kitchen-api/internal/domain"

func (r restaurantRow) toDomain() domain.Restaurant {
	return domain.Restaurant{
		ID:          r.ID,
		Name:        r.Name,
		Description: r.Description,
		APIKey:      r.APIKey,
		IsActive:    r.IsActive,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

func toDomainRestaurants(rows []restaurantRow) []domain.Restaurant {
	result := make([]domain.Restaurant, len(rows))
	for i, r := range rows {
		result[i] = r.toDomain()
	}
	return result
}

func (r menuItemRow) toDomain() domain.MenuItem {
	return domain.MenuItem{
		ID:           r.ID,
		RestaurantID: r.RestaurantID,
		ExternalID:   r.ExternalID,
		Name:         r.Name,
		PriceCents:   r.PriceCents,
		IsAvailable:  r.IsAvailable,
		CreatedAt:    r.CreatedAt,
		UpdatedAt:    r.UpdatedAt,
	}
}

func toDomainMenuItems(rows []menuItemRow) []domain.MenuItem {
	result := make([]domain.MenuItem, len(rows))
	for i, r := range rows {
		result[i] = r.toDomain()
	}
	return result
}

func (r orderRow) toDomain() domain.Order {
	return domain.Order{
		ID:              r.ID,
		UserID:          r.UserID,
		RestaurantID:    r.RestaurantID,
		Status:          domain.OrderStatus(r.Status),
		TotalPriceCents: r.TotalPriceCents,
		CreatedAt:       r.CreatedAt,
		UpdatedAt:       r.UpdatedAt,
	}
}

func toDomainOrders(rows []orderRow) []domain.Order {
	result := make([]domain.Order, len(rows))
	for i, r := range rows {
		result[i] = r.toDomain()
	}
	return result
}

func (r orderItemRow) toDomain() domain.OrderItem {
	return domain.OrderItem{
		ID:             r.ID,
		OrderID:        r.OrderID,
		MenuItemID:     r.MenuItemID,
		Quantity:       r.Quantity,
		UnitPriceCents: r.UnitPriceCents,
		CreatedAt:      r.CreatedAt,
	}
}

func toDomainOrderItems(rows []orderItemRow) []domain.OrderItem {
	result := make([]domain.OrderItem, len(rows))
	for i, r := range rows {
		result[i] = r.toDomain()
	}
	return result
}
