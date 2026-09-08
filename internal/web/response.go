package web

import (
	"encoding/json"
	"errors"
	"net/http"

	"kitchen-api/internal/domain"
)

func respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		_ = json.NewEncoder(w).Encode(data)
	}
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, ErrorResponse{Error: message})
}

func handleDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidData),
		errors.Is(err, domain.ErrInvalidQuantity),
		errors.Is(err, domain.ErrRestaurantUnavailable),
		errors.Is(err, domain.ErrMenuItemUnavailable):
		respondError(w, http.StatusBadRequest, err.Error())

	case errors.Is(err, domain.ErrRestaurantNotFound),
		errors.Is(err, domain.ErrMenuItemNotFound),
		errors.Is(err, domain.ErrOrderNotFound):
		respondError(w, http.StatusNotFound, err.Error())

	case errors.Is(err, domain.ErrAccessDenied):
		respondError(w, http.StatusForbidden, err.Error())

	case errors.Is(err, domain.ErrOrderAlreadyCancelled),
		errors.Is(err, domain.ErrInvalidStatusTransition):
		respondError(w, http.StatusConflict, err.Error())

	default:
		respondError(w, http.StatusInternalServerError, "internal server error")
	}
}
