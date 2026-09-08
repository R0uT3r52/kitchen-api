package web

import (
	"net/http"

	"kitchen-api/internal/domain"
)

type Handler struct {
	orderService   domain.OrderUseCase
	partnerService domain.PartnerUseCase
}

func NewHandler(orderService domain.OrderUseCase, partnerService domain.PartnerUseCase) *Handler {
	return &Handler{
		orderService:   orderService,
		partnerService: partnerService,
	}
}

func (h *Handler) InitRoutes() http.Handler {
	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("GET /health", h.healthCheck)

	// Client API
	mux.HandleFunc("GET /api/v1/restaurants", h.handleGetRestaurants)
	mux.HandleFunc("GET /api/v1/restaurants/{id}/menu", h.handleGetRestaurantMenu)
	mux.HandleFunc("POST /api/v1/orders", h.handleCreateOrder)
	mux.HandleFunc("POST /api/v1/orders/{id}/cancel", h.handleCancelOrder)
	mux.HandleFunc("GET /api/v1/orders/{id}", h.handleGetOrderByID)
	mux.HandleFunc("GET /api/v1/orders", h.handleGetUserOrders)

	// Partner API
	mux.HandleFunc("PUT /api/v1/partner/menu", h.PartnerAuthMiddleware(h.handleUpsertMenu))
	mux.HandleFunc("GET /api/v1/partner/orders", h.PartnerAuthMiddleware(h.handleGetPartnerOrders))
	mux.HandleFunc("PATCH /api/v1/partner/orders/{id}/status", h.PartnerAuthMiddleware(h.handleUpdateOrderStatus))

	return mux
}

func (h *Handler) healthCheck(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
