package web

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"kitchen-api/internal/domain"
)

// PUT /api/v1/partner/menu
func (h *Handler) handleUpsertMenu(w http.ResponseWriter, r *http.Request) {
	restaurantID, ok := getRestaurantIDFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req UpsertMenuRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "incorrect request body")
		return
	}

	if len(req.Items) == 0 {
		respondError(w, http.StatusBadRequest, "items cannot be empty")
		return
	}

	domainItems := make([]domain.MenuItem, 0, len(req.Items))
	for _, item := range req.Items {
		domainItems = append(domainItems, domain.MenuItem{
			RestaurantID: restaurantID,
			ExternalID:   item.ExternalID,
			Name:         item.Name,
			PriceCents:   item.PriceCents,
			IsAvailable:  item.IsAvailable,
		})
	}

	err := h.partnerService.UpsertMenu(r.Context(), restaurantID, domainItems)
	if err != nil {
		handleDomainError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, StatusResponse{
		Status:  "ok",
		Message: "menu updated successfully",
	})
}

// GET /api/v1/partner/orders?status=...
func (h *Handler) handleGetPartnerOrders(w http.ResponseWriter, r *http.Request) {
	restaurantID, ok := getRestaurantIDFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var statusPtr *domain.OrderStatus
	statusStr := strings.TrimSpace(r.URL.Query().Get("status"))
	if statusStr != "" {
		st := domain.OrderStatus(statusStr)
		if !st.Validate() {
			respondError(w, http.StatusBadRequest, "invalid status parameter")
			return
		}
		statusPtr = &st
	}

	orders, err := h.partnerService.GetOrders(r.Context(), restaurantID, statusPtr)
	if err != nil {
		handleDomainError(w, err)
		return
	}

	if orders == nil {
		orders = []domain.Order{}
	}

	respondJSON(w, http.StatusOK, orders)
}

// PATCH /api/v1/partner/orders/{id}/status
func (h *Handler) handleUpdateOrderStatus(w http.ResponseWriter, r *http.Request) {
	restaurantID, ok := getRestaurantIDFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	idStr := r.PathValue("id")
	orderID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || orderID <= 0 {
		respondError(w, http.StatusBadRequest, "invalid order id")
		return
	}

	var req UpdateStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "incorrect request body")
		return
	}

	if !req.Status.Validate() {
		respondError(w, http.StatusBadRequest, "invalid order status")
		return
	}

	err = h.partnerService.UpdateOrderStatus(r.Context(), restaurantID, orderID, req.Status)
	if err != nil {
		handleDomainError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, StatusResponse{
		Status: "ok",
	})
}
