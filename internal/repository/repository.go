package repository

import (
	"context"
	"errors"
	"fmt"
	"kitchen-api/internal/domain"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func GetDB(ctx context.Context) (*pgxpool.Pool, error) {
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		user := os.Getenv("DB_USER")
		if user == "" {
			user = os.Getenv("POSTGRES_USER")
		}
		password := os.Getenv("DB_PASSWORD")
		if password == "" {
			password = os.Getenv("POSTGRES_PASSWORD")
		}
		host := os.Getenv("DB_HOST")
		if host == "" {
			host = "localhost"
		}
		port := os.Getenv("DB_PORT")
		if port == "" {
			port = "5432"
		}
		dbname := os.Getenv("DB_NAME")
		if dbname == "" {
			dbname = os.Getenv("POSTGRES_DB")
		}
		sslmode := os.Getenv("DB_SSLMODE")
		if sslmode == "" {
			sslmode = "disable"
		}

		connStr = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
			host, port, user, password, dbname, sslmode)
	}

	config, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse db config: %w", err)
	}

	db, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create db pool: %w", err)
	}

	if err := db.Ping(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping db: %w", err)
	}

	return db, nil
}

func NewRepo(db *pgxpool.Pool) *Repo {
	return &Repo{
		Data: db,
	}
}

func (r *Repo) GetActiveRestaurants(ctx context.Context) ([]domain.Restaurant, error) {
	sql := `SELECT * FROM restaurants WHERE is_active=TRUE;`

	rows, err := r.Data.Query(ctx, sql)
	if err != nil {
		return nil, err
	}

	ans, err := pgx.CollectRows(rows, pgx.RowToStructByName[restaurantRow])
	if err != nil {
		return nil, err
	}

	return toDomainRestaurants(ans), nil
}

func (r *Repo) GetRestaurantByID(ctx context.Context, id int64) (*domain.Restaurant, error) {
	sql := `SELECT * FROM restaurants WHERE id=$1;`

	rows, err := r.Data.Query(ctx, sql, id)
	if err != nil {
		return nil, err
	}

	ans, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[restaurantRow])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrRestaurantNotFound
		}
		return nil, err
	}

	dom := ans.toDomain()
	return &dom, nil
}

func (r *Repo) GetRestaurantByAPIKey(ctx context.Context, apiKey string) (*domain.Restaurant, error) {

	sql := `SELECT * FROM restaurants WHERE api_key=$1;`

	rows, err := r.Data.Query(ctx, sql, apiKey)
	if err != nil {
		return nil, err
	}

	ans, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[restaurantRow])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrRestaurantNotFound
		}
		return nil, err
	}

	dom := ans.toDomain()
	return &dom, nil
}

func (r *Repo) GetMenuByRestaurantID(ctx context.Context, restaurantID int64) ([]domain.MenuItem, error) {
	sql := `SELECT * FROM menu_items WHERE restaurant_id=$1;`

	rows, err := r.Data.Query(ctx, sql, restaurantID)
	if err != nil {
		return nil, err
	}

	ans, err := pgx.CollectRows(rows, pgx.RowToStructByName[menuItemRow])
	if err != nil {
		return nil, err
	}

	return toDomainMenuItems(ans), nil
}

func (r *Repo) UpsertMenuItems(ctx context.Context, restaurantID int64, items []domain.MenuItem) error {
	tx, err := r.Data.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	sql := `INSERT INTO menu_items (restaurant_id, external_id, name, price_cents, is_available)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (restaurant_id, external_id) DO UPDATE
			SET name=$3, price_cents=$4, is_available=$5;`

	// Better to use batch, instead of sending separate DB requests
	// Will be in assumption in README
	for _, elem := range items {
		_, err := r.Data.Exec(ctx, sql, restaurantID, elem.ExternalID, elem.Name, elem.PriceCents, elem.IsAvailable)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (r *Repo) CreateOrder(ctx context.Context, order *domain.Order) error {
	tx, err := r.Data.Begin(ctx)
	if err != nil {
		return err
	}

	// Will rollback on error
	defer tx.Rollback(ctx) //nolint:errcheck

	sqlOrder := `
		INSERT INTO orders (user_id, restaurant_id, status, total_price_cents)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, updated_at;
	`
	err = tx.QueryRow(ctx, sqlOrder,
		order.UserID,
		order.RestaurantID,
		order.Status,
		order.TotalPriceCents,
	).Scan(&order.ID, &order.CreatedAt, &order.UpdatedAt)
	if err != nil {
		return err
	}

	sqlItem := `
		INSERT INTO order_items (order_id, menu_item_id, quantity, unit_price_cents)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at;
	`
	for i := range order.Items {
		order.Items[i].OrderID = order.ID
		err = tx.QueryRow(ctx, sqlItem,
			order.ID,
			order.Items[i].MenuItemID,
			order.Items[i].Quantity,
			order.Items[i].UnitPriceCents,
		).Scan(&order.Items[i].ID, &order.Items[i].CreatedAt)
		if err != nil {
			return err
		}
	}

	sqlEvent := `
		INSERT INTO order_events (order_id, old_status, new_status)
		VALUES ($1, NULL, $2);
	`
	_, err = tx.Exec(ctx, sqlEvent, order.ID, order.Status)
	if err != nil {
		return err
	}

	// Commit transaction
	return tx.Commit(ctx)
}

func (r *Repo) GetOrderByID(ctx context.Context, orderID int64) (*domain.Order, error) {
	sqlOrder := `SELECT * FROM orders WHERE id=$1;`

	rows, err := r.Data.Query(ctx, sqlOrder, orderID)
	if err != nil {
		return nil, err
	}

	row, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[orderRow])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrOrderNotFound
		}
		return nil, err
	}

	sqlItems := `SELECT * FROM order_items WHERE order_id=$1;`

	itemsRows, err := r.Data.Query(ctx, sqlItems, orderID)
	if err != nil {
		return nil, err
	}

	items, err := pgx.CollectRows(itemsRows, pgx.RowToStructByName[orderItemRow])
	if err != nil {
		return nil, err
	}

	order := row.toDomain()
	order.Items = toDomainOrderItems(items)

	return &order, nil
}

func (r *Repo) GetOrdersByUserID(ctx context.Context, userID string) ([]domain.Order, error) {
	sql := `SELECT * FROM orders WHERE user_id=$1 ORDER BY created_at DESC;`

	rows, err := r.Data.Query(ctx, sql, userID)
	if err != nil {
		return nil, err
	}

	orderRows, err := pgx.CollectRows(rows, pgx.RowToStructByName[orderRow])
	if err != nil {
		return nil, err
	}

	if len(orderRows) == 0 {
		return []domain.Order{}, nil
	}

	orderIDs := make([]int64, 0, len(orderRows))
	for i := range orderRows {
		orderIDs = append(orderIDs, orderRows[i].ID)
	}

	sql2 := `SELECT * FROM order_items WHERE order_id = ANY($1);`
	itemsRows, err := r.Data.Query(ctx, sql2, orderIDs)
	if err != nil {
		return nil, err
	}

	orderItems, err := pgx.CollectRows(itemsRows, pgx.RowToStructByName[orderItemRow])
	if err != nil {
		return nil, err
	}

	itemsMap := make(map[int64][]domain.OrderItem)
	for _, item := range orderItems {
		itemsMap[item.OrderID] = append(itemsMap[item.OrderID], item.toDomain())
	}

	ans := toDomainOrders(orderRows)
	for i := range ans {
		if items, ok := itemsMap[ans[i].ID]; ok {
			ans[i].Items = items
		}
	}

	return ans, nil
}

func (r *Repo) GetOrdersByRestaurantAndStatus(ctx context.Context, restaurantID int64, status *domain.OrderStatus) ([]domain.Order, error) {
	var rows pgx.Rows
	var err error

	if status != nil {
		sql := `SELECT * FROM orders WHERE restaurant_id=$1 AND status=$2 ORDER BY created_at DESC;`
		rows, err = r.Data.Query(ctx, sql, restaurantID, string(*status))
	} else {
		sql := `SELECT * FROM orders WHERE restaurant_id=$1 ORDER BY created_at DESC;`
		rows, err = r.Data.Query(ctx, sql, restaurantID)
	}
	if err != nil {
		return nil, err
	}

	orderRows, err := pgx.CollectRows(rows, pgx.RowToStructByName[orderRow])
	if err != nil {
		return nil, err
	}

	if len(orderRows) == 0 {
		return []domain.Order{}, nil
	}

	orderIDs := make([]int64, 0, len(orderRows))
	for i := range orderRows {
		orderIDs = append(orderIDs, orderRows[i].ID)
	}

	sql2 := `SELECT * FROM order_items WHERE order_id = ANY($1);`
	itemsRows, err := r.Data.Query(ctx, sql2, orderIDs)
	if err != nil {
		return nil, err
	}

	orderItems, err := pgx.CollectRows(itemsRows, pgx.RowToStructByName[orderItemRow])
	if err != nil {
		return nil, err
	}

	itemsMap := make(map[int64][]domain.OrderItem)
	for _, item := range orderItems {
		itemsMap[item.OrderID] = append(itemsMap[item.OrderID], item.toDomain())
	}

	ans := toDomainOrders(orderRows)
	for i := range ans {
		if items, ok := itemsMap[ans[i].ID]; ok {
			ans[i].Items = items
		}
	}

	return ans, nil
}

func (r *Repo) UpdateOrderStatus(ctx context.Context, orderID int64, newStatus domain.OrderStatus) error {
	tx, err := r.Data.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var oldStatus domain.OrderStatus
	sqlSelect := `SELECT status FROM orders WHERE id=$1 FOR UPDATE;`
	err = tx.QueryRow(ctx, sqlSelect, orderID).Scan(&oldStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrOrderNotFound
		}
		return err
	}

	if oldStatus == newStatus {
		return tx.Commit(ctx)
	}

	if !domain.CanTransition(oldStatus, newStatus) {
		if oldStatus == domain.StatusCancelled {
			return domain.ErrOrderAlreadyCancelled
		}
		return domain.ErrInvalidStatusTransition
	}

	sqlUpdate := `UPDATE orders SET status=$1 WHERE id=$2;`
	_, err = tx.Exec(ctx, sqlUpdate, newStatus, orderID)
	if err != nil {
		return err
	}

	sqlEvent := `
		INSERT INTO order_events (order_id, old_status, new_status)
		VALUES ($1, $2, $3);
	`
	_, err = tx.Exec(ctx, sqlEvent, orderID, oldStatus, newStatus)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}
