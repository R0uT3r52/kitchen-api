package web

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"kitchen-api/internal/domain"
)

type contextKey string

const restaurantIDContextKey contextKey = "restaurant_id"

func (h *Handler) PartnerAuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		apiKey := strings.TrimSpace(r.Header.Get("X-API-KEY"))
		if apiKey == "" {
			respondError(w, http.StatusUnauthorized, "missing or empty X-API-KEY header")
			return
		}

		restaurant, err := h.partnerService.GetRestaurantByAPIKey(r.Context(), apiKey)
		if err != nil {
			if errors.Is(err, domain.ErrRestaurantNotFound) {
				respondError(w, http.StatusUnauthorized, "invalid api key")
				return
			}
			if errors.Is(err, domain.ErrRestaurantUnavailable) {
				respondError(w, http.StatusForbidden, "restaurant is inactive")
				return
			}
			respondError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		ctx := context.WithValue(r.Context(), restaurantIDContextKey, restaurant.ID)
		next(w, r.WithContext(ctx))
	}
}

func getRestaurantIDFromContext(ctx context.Context) (int64, bool) {
	val, ok := ctx.Value(restaurantIDContextKey).(int64)
	return val, ok
}
