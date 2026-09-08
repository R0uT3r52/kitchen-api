package web

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"kitchen-api/internal/domain"
)

// GET /api/v1/restaurants
func (h *Handler) handleGetRestaurants(w http.ResponseWriter, r *http.Request) {
	restaurants, err := h.orderService.GetActiveRestaurants(r.Context())
	if err != nil {
		handleDomainError(w, err)
		return
	}

	if restaurants == nil {
		restaurants = []domain.Restaurant{}
	}

	respondJSON(w, http.StatusOK, restaurants)
}

// GET /api/v1/restaurants/{id}/menu
func (h *Handler) handleGetRestaurantMenu(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	restaurantID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || restaurantID <= 0 {
		respondError(w, http.StatusBadRequest, "invalid restaurant id")
		return
	}

	menu, err := h.orderService.GetRestaurantMenu(r.Context(), restaurantID)
	if err != nil {
		handleDomainError(w, err)
		return
	}

	if menu == nil {
		menu = []domain.MenuItem{}
	}

	respondJSON(w, http.StatusOK, menu)
}

// POST /api/v1/orders
func (h *Handler) handleCreateOrder(w http.ResponseWriter, r *http.Request) {
	var req CreateOrderRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "malformed request body")
		return
	}

	req.UserID = strings.TrimSpace(req.UserID)
	if req.UserID == "" {
		respondError(w, http.StatusBadRequest, "user_id is required")
		return
	}

	if req.RestaurantID <= 0 {
		respondError(w, http.StatusBadRequest, "invalid restaurant_id")
		return
	}

	if len(req.Items) == 0 {
		respondError(w, http.StatusBadRequest, "order must contain at least one item")
		return
	}

	domainItems := make([]domain.OrderItemCreate, 0, len(req.Items))
	for _, item := range req.Items {
		if item.Quantity <= 0 {
			respondError(w, http.StatusBadRequest, "incorrect order item quantity")
			return
		}
		domainItems = append(domainItems, domain.OrderItemCreate{
			MenuItemID: item.MenuItemID,
			Quantity:   item.Quantity,
		})
	}

	order, err := h.orderService.CreateOrder(r.Context(), req.UserID, req.RestaurantID, domainItems)
	if err != nil {
		handleDomainError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, order)
}

// POST /api/v1/orders/{id}/cancel
func (h *Handler) handleCancelOrder(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	orderID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || orderID <= 0 {
		respondError(w, http.StatusBadRequest, "invalid order id")
		return
	}

	var req CancelOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.UserID == "" {
		req.UserID = r.URL.Query().Get("user_id")
	}

	req.UserID = strings.TrimSpace(req.UserID)
	if req.UserID == "" {
		respondError(w, http.StatusBadRequest, "user_id is required")
		return
	}

	err = h.orderService.CancelOrder(r.Context(), orderID, req.UserID)
	if err != nil {
		handleDomainError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, StatusResponse{
		Status:  "cancelled",
		Message: "order cancelled successfully",
	})
}

// GET /api/v1/orders/{id}
func (h *Handler) handleGetOrderByID(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	orderID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || orderID <= 0 {
		respondError(w, http.StatusBadRequest, "invalid order id")
		return
	}

	order, err := h.orderService.GetOrderByID(r.Context(), orderID)
	if err != nil {
		handleDomainError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, order)
}

// GET /api/v1/orders?user_id=...
func (h *Handler) handleGetUserOrders(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	if userID == "" {
		respondError(w, http.StatusBadRequest, "user_id query parameter is required")
		return
	}

	orders, err := h.orderService.GetUserOrders(r.Context(), userID)
	if err != nil {
		handleDomainError(w, err)
		return
	}

	if orders == nil {
		orders = []domain.Order{}
	}

	respondJSON(w, http.StatusOK, orders)
}
