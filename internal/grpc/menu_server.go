package grpc

import (
	"context"
	"errors"

	partnerv1 "kitchen-api/api/proto/partner/v1"
	"kitchen-api/internal/domain"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// implements partnerv1.MenuServiceServer
type MenuServer struct {
	partnerv1.UnimplementedMenuServiceServer
	partnerService domain.PartnerUseCase
}

// creates a new MenuServer instance
func NewMenuServer(partnerService domain.PartnerUseCase) *MenuServer {
	return &MenuServer{
		partnerService: partnerService,
	}
}

// registers the MenuServer with a gRPC service registrar
func (s *MenuServer) Register(server grpc.ServiceRegistrar) {
	partnerv1.RegisterMenuServiceServer(server, s)
}

// handles batch synchronization of restaurant menu items
func (s *MenuServer) SyncMenu(ctx context.Context, req *partnerv1.SyncMenuRequest) (*partnerv1.SyncMenuResponse, error) {
	restaurantID, ok := RestaurantIDFromContext(ctx)
	if !ok || restaurantID <= 0 {
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}

	if req == nil || len(req.GetItems()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "items cannot be empty")
	}

	items := req.GetItems()
	domainItems := make([]domain.MenuItem, 0, len(items))
	for _, item := range items {
		if item == nil {
			return nil, status.Error(codes.InvalidArgument, "menu item cannot be nil")
		}
		domainItems = append(domainItems, domain.MenuItem{
			RestaurantID: restaurantID,
			ExternalID:   item.GetExternalId(),
			Name:         item.GetName(),
			PriceCents:   item.GetPriceCents(),
			IsAvailable:  item.GetIsAvailable(),
		})
	}

	err := s.partnerService.UpsertMenu(ctx, restaurantID, domainItems)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidData):
			return nil, status.Errorf(codes.InvalidArgument, "%v", err)
		case errors.Is(err, domain.ErrRestaurantNotFound):
			return nil, status.Errorf(codes.NotFound, "%v", err)
		case errors.Is(err, domain.ErrRestaurantUnavailable):
			return nil, status.Errorf(codes.FailedPrecondition, "%v", err)
		default:
			return nil, status.Errorf(codes.Internal, "internal server error: %v", err)
		}
	}

	return &partnerv1.SyncMenuResponse{
		Status:  "ok",
		Message: "menu synced successfully",
		Count:   int32(len(domainItems)),
	}, nil
}
