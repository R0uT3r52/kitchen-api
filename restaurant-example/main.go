package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

var ErrConflict = errors.New("conflict: order cancelled or invalid state transition")

type MenuItemDTO struct {
	ExternalID  string `json:"external_id"`
	Name        string `json:"name"`
	PriceCents  int64  `json:"price_cents"`
	IsAvailable bool   `json:"is_available"`
}

type UpsertMenuRequest struct {
	Items []MenuItemDTO `json:"items"`
}

type UpdateStatusRequest struct {
	Status string `json:"status"`
}

type OrderItemDTO struct {
	ID             int64     `json:"id"`
	OrderID        int64     `json:"order_id"`
	MenuItemID     int64     `json:"menu_item_id"`
	Quantity       int       `json:"quantity"`
	UnitPriceCents int64     `json:"unit_price_cents"`
	CreatedAt      time.Time `json:"created_at"`
}

type OrderDTO struct {
	ID              int64          `json:"id"`
	UserID          string         `json:"user_id"`
	RestaurantID    int64          `json:"restaurant_id"`
	Status          string         `json:"status"`
	TotalPriceCents int64          `json:"total_price_cents"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	Items           []OrderItemDTO `json:"items,omitempty"`
}

var initialMenu = []MenuItemDTO{
	{ExternalID: "pizza_margherita", Name: "Пицца Маргарита", PriceCents: 450000, IsAvailable: true},
	{ExternalID: "pizza_pepperoni", Name: "Пицца Пепперони", PriceCents: 550000, IsAvailable: true},
	{ExternalID: "pizza_4cheeses", Name: "Пицца 4 сыра", PriceCents: 620000, IsAvailable: true},
	{ExternalID: "tiramisu", Name: "Десерт Тирамису", PriceCents: 280000, IsAvailable: true},
	{ExternalID: "cola_05", Name: "Кока-Кола 0.5л", PriceCents: 150000, IsAvailable: true},
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvDuration(key string, defaultVal time.Duration) time.Duration {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	d, err := time.ParseDuration(val)
	if err != nil {
		return defaultVal
	}
	return d
}

func waitKitchenAPI(ctx context.Context, client *http.Client, apiURL string, timeout time.Duration) error {
	endpoint := strings.TrimRight(apiURL, "/") + "/health"
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err == nil {
			resp, err := client.Do(req)
			if err == nil {
				_ = resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					return nil
				}
			}
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case t := <-ticker.C:
			if t.After(deadline) {
				return fmt.Errorf("kitchen-api not reachable at %s within %v", endpoint, timeout)
			}
		}
	}
}

func syncMenu(ctx context.Context, client *http.Client, apiURL, apiKey string, items []MenuItemDTO) error {
	payload, err := json.Marshal(UpsertMenuRequest{Items: items})
	if err != nil {
		return fmt.Errorf("failed to marshal menu: %w", err)
	}

	endpoint := strings.TrimRight(apiURL, "/") + "/api/v1/partner/menu"
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-KEY", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("menu sync request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func getOrders(ctx context.Context, client *http.Client, apiURL, apiKey, status string) ([]OrderDTO, error) {
	endpoint := strings.TrimRight(apiURL, "/") + "/api/v1/partner/orders"
	if status != "" {
		endpoint += "?status=" + status
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-KEY", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch orders: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var orders []OrderDTO
	if err := json.NewDecoder(resp.Body).Decode(&orders); err != nil {
		return nil, fmt.Errorf("failed to decode orders json: %w", err)
	}

	return orders, nil
}

func updateOrderStatus(ctx context.Context, client *http.Client, apiURL, apiKey string, orderID int64, newStatus string) error {
	payload, err := json.Marshal(UpdateStatusRequest{Status: newStatus})
	if err != nil {
		return fmt.Errorf("failed to marshal status: %w", err)
	}

	endpoint := fmt.Sprintf("%s/api/v1/partner/orders/%d/status", strings.TrimRight(apiURL, "/"), orderID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-KEY", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("status update request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode == http.StatusConflict {
		return ErrConflict
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func processOrder(ctx context.Context, client *http.Client, apiURL, apiKey string, order OrderDTO, stepDelay time.Duration, activeOrders *sync.Map, wg *sync.WaitGroup) {
	defer wg.Done()
	defer activeOrders.Delete(order.ID)

	orderID := order.ID
	log.Printf("[Order #%d] Received new order with %d item(s), total %d kopecks",
		orderID, len(order.Items), order.TotalPriceCents)

	if err := updateOrderStatus(ctx, client, apiURL, apiKey, orderID, "accepted"); err != nil {
		if errors.Is(err, ErrConflict) {
			log.Printf("[Order #%d] Order was cancelled by customer before acceptance, stopping processing", orderID)
			return
		}
		log.Printf("[Order #%d] Failed to accept order: %v", orderID, err)
		return
	}
	log.Printf("[Order #%d] Status updated: created -> accepted", orderID)

	if !sleepContext(ctx, stepDelay) {
		return
	}

	if err := updateOrderStatus(ctx, client, apiURL, apiKey, orderID, "cooking"); err != nil {
		if errors.Is(err, ErrConflict) {
			log.Printf("[Order #%d] Order was cancelled by customer before cooking, stopping processing", orderID)
			return
		}
		log.Printf("[Order #%d] Failed to start cooking: %v", orderID, err)
		return
	}
	log.Printf("[Order #%d] Status updated: accepted -> cooking", orderID)

	if !sleepContext(ctx, stepDelay) {
		return
	}

	if err := updateOrderStatus(ctx, client, apiURL, apiKey, orderID, "ready"); err != nil {
		log.Printf("[Order #%d] Failed to set status ready: %v", orderID, err)
		return
	}
	log.Printf("[Order #%d] Status updated: cooking -> ready", orderID)

	if !sleepContext(ctx, stepDelay) {
		return
	}

	if err := updateOrderStatus(ctx, client, apiURL, apiKey, orderID, "completed"); err != nil {
		log.Printf("[Order #%d] Failed to complete order: %v", orderID, err)
		return
	}
	log.Printf("[Order #%d] Status updated: ready -> completed. Order done!", orderID)
}

func sleepContext(ctx context.Context, d time.Duration) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(d):
		return true
	}
}

func pollOrders(ctx context.Context, client *http.Client, apiURL, apiKey string, stepDelay time.Duration, activeOrders *sync.Map, wg *sync.WaitGroup) {
	orders, err := getOrders(ctx, client, apiURL, apiKey, "created")
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			log.Printf("[polling] Error fetching created orders: %v", err)
		}
		return
	}

	for _, order := range orders {
		if _, loaded := activeOrders.LoadOrStore(order.ID, true); !loaded {
			wg.Add(1)
			go processOrder(ctx, client, apiURL, apiKey, order, stepDelay, activeOrders, wg)
		}
	}
}

func main() {
	apiURL := getEnv("API_URL", "http://localhost:8080")
	apiKey := getEnv("PARTNER_API_KEY", "partner_api_key_dominos")
	pollInterval := getEnvDuration("POLL_INTERVAL", 5*time.Second)
	stepDelay := getEnvDuration("STEP_DELAY", 2*time.Second)

	log.Printf("restaurant-example started for api_url=%s, api_key=%s, poll_interval=%v, step_delay=%v",
		apiURL, apiKey, pollInterval, stepDelay)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	httpClient := &http.Client{Timeout: 10 * time.Second}

	log.Println("Waiting for kitchen-api readiness...")
	if err := waitKitchenAPI(ctx, httpClient, apiURL, 30*time.Second); err != nil {
		log.Fatalf("failed waiting for kitchen-api: %v", err)
	}
	log.Println("kitchen-api is ready!")

	log.Printf("Synchronizing menu (%d items)...", len(initialMenu))
	if err := syncMenu(ctx, httpClient, apiURL, apiKey, initialMenu); err != nil {
		log.Fatalf("failed to sync menu: %v", err)
	}
	log.Println("Menu synchronized successfully!")

	var activeOrders sync.Map
	var wg sync.WaitGroup

	pollOrders(ctx, httpClient, apiURL, apiKey, stepDelay, &activeOrders, &wg)

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Termination signal received, waiting for ongoing orders to finish...")
			wg.Wait()
			log.Println("restaurant-example stopped cleanly")
			return
		case <-ticker.C:
			pollOrders(ctx, httpClient, apiURL, apiKey, stepDelay, &activeOrders, &wg)
		}
	}
}
