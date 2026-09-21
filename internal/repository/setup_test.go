package repository_test

import (
	"context"
	"fmt"
	"kitchen-api/internal/repository"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

var (
	testPool *pgxpool.Pool
	testRepo *repository.Repo
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	pgContainer, err := postgres.Run(
		ctx,
		"postgres:16.3-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(5*time.Second),
		),
	)
	if err != nil {
		fmt.Printf("Failed to start postgres container: %v\n", err)
		os.Exit(1)
	}

	defer func() {
		if err := testcontainers.TerminateContainer(pgContainer); err != nil {
			fmt.Printf("Failed terminating postgres container: %v\n", err)
		}
	}()

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fmt.Printf("Failed to get connection string: %v\n", err)
		os.Exit(1)
	}

	testPool, err = pgxpool.New(ctx, connStr)
	if err != nil {
		fmt.Printf("Failed to create connection pool with connstr: %s; %v\n", connStr, err)
		os.Exit(1)
	}
	defer testPool.Close()

	schemaPath := filepath.Join("..", "..", "migrations", "000001_init_schema.up.sql")
	schemaSQL, err := os.ReadFile(schemaPath)
	if err != nil {
		fmt.Printf("failed to read schema migration: %s\n", err)
		os.Exit(1)
	}
	if _, err := testPool.Exec(ctx, string(schemaSQL)); err != nil {
		fmt.Printf("failed to apply schema: %s\n", err)
		os.Exit(1)
	}

	testRepo = repository.NewRepo(testPool)

	code := m.Run()
	os.Exit(code)
}

func cleanDB(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	_, err := testPool.Exec(ctx, `
		TRUNCATE TABLE order_events, order_items, orders, menu_items, restaurants RESTART IDENTITY CASCADE;
	`)
	if err != nil {
		t.Fatalf("failed to truncate tables: %v", err)
	}
}

func createTestRestaurant(t *testing.T, name, apiKey string, isActive bool) int64 {
	t.Helper()
	var id int64
	err := testPool.QueryRow(context.Background(),
		`INSERT INTO restaurants (name, description, api_key, is_active)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id;`,
		name, "Description for "+name, apiKey, isActive,
	).Scan(&id)
	if err != nil {
		t.Fatalf("failed to create test restaurant: %v", err)
	}
	return id
}

func createTestMenuItem(t *testing.T, restaurantID int64, extID, name string, priceCents int64, isAvailable bool) int64 {
	t.Helper()
	var id int64
	err := testPool.QueryRow(context.Background(),
		`INSERT INTO menu_items (restaurant_id, external_id, name, price_cents, is_available)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id;`,
		restaurantID, extID, name, priceCents, isAvailable,
	).Scan(&id)
	if err != nil {
		t.Fatalf("failed to create test menu item: %v", err)
	}
	return id
}
