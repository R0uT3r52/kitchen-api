package web

import (
	"encoding/json"
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
	// TODO: implement handlers
	mux.HandleFunc("GET /api/v1/restaurants", h.placeholder("GET /api/v1/restaurants"))
	mux.HandleFunc("GET /api/v1/restaurants/{id}/menu", h.placeholder("GET /api/v1/restaurants/{id}/menu"))
	mux.HandleFunc("POST /api/v1/orders", h.placeholder("POST /api/v1/orders"))
	mux.HandleFunc("POST /api/v1/orders/{id}/cancel", h.placeholder("POST /api/v1/orders/{id}/cancel"))
	mux.HandleFunc("GET /api/v1/orders/{id}", h.placeholder("GET /api/v1/orders/{id}"))
	mux.HandleFunc("GET /api/v1/orders", h.placeholder("GET /api/v1/orders"))

	// Partner API
	// TODO: add X-API-KEY middleware and implement handlers
	mux.HandleFunc("PUT /api/v1/partner/menu", h.placeholder("PUT /api/v1/partner/menu"))
	mux.HandleFunc("GET /api/v1/partner/orders", h.placeholder("GET /api/v1/partner/orders"))
	mux.HandleFunc("PATCH /api/v1/partner/orders/{id}/status", h.placeholder("PATCH /api/v1/partner/orders/{id}/status"))

	return mux
}

func (h *Handler) healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *Handler) placeholder(route string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotImplemented)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"message": "endpoint under development",
			"route":   route,
		})
	}
}
