package grpc

import (
	"context"
	"errors"
	"strings"

	"kitchen-api/internal/domain"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type contextKey string

const (
	apiKeyHeader               = "x-api-key"
	restaurantIDKey contextKey = "restaurant_id"
)

func RestaurantIDFromContext(ctx context.Context) (int64, bool) {
	val, ok := ctx.Value(restaurantIDKey).(int64)
	return val, ok
}

func ContextWithRestaurantID(ctx context.Context, restaurantID int64) context.Context {
	return context.WithValue(ctx, restaurantIDKey, restaurantID)
}

// requests using the x-api-key metadata header and saves the restaurant ID into context
func UnaryAuthInterceptor(partnerService domain.PartnerUseCase) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		// health check calls without authentication
		if info != nil && strings.HasPrefix(info.FullMethod, "/grpc.health.v1.Health/") {
			return handler(ctx, req)
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}

		values := md.Get(apiKeyHeader)
		if len(values) == 0 || strings.TrimSpace(values[0]) == "" {
			return nil, status.Error(codes.Unauthenticated, "missing or empty x-api-key")
		}

		apiKey := strings.TrimSpace(values[0])
		restaurant, err := partnerService.GetRestaurantByAPIKey(ctx, apiKey)
		if err != nil {
			if errors.Is(err, domain.ErrRestaurantNotFound) || errors.Is(err, domain.ErrInvalidData) {
				return nil, status.Error(codes.Unauthenticated, "invalid api key")
			}
			if errors.Is(err, domain.ErrRestaurantUnavailable) {
				return nil, status.Error(codes.PermissionDenied, "restaurant is inactive")
			}
			return nil, status.Errorf(codes.Internal, "internal server error: %v", err)
		}

		newCtx := ContextWithRestaurantID(ctx, restaurant.ID)
		return handler(newCtx, req)
	}
}

func NewAuthInterceptor(partnerService domain.PartnerUseCase) grpc.UnaryServerInterceptor {
	return UnaryAuthInterceptor(partnerService)
}
