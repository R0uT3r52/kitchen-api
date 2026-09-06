package repository

import (
	"time"

	"kitchen-api/internal/domain"

	"github.com/jackc/pgx/v5/pgxpool"
)

type OrderEvent struct {
	ID        int64               `db:"id"`
	OrderID   int64               `db:"order_id"`
	OldStatus *domain.OrderStatus `db:"old_status"`
	NewStatus domain.OrderStatus  `db:"new_status"`
	CreatedAt time.Time           `db:"created_at"`
}

type Repo struct {
	Data *pgxpool.Pool
}

// TODO: DTO and mappers for Domain-Repository and Repository-Domain
