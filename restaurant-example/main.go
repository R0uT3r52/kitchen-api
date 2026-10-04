// Package main provides a demonstration partner restaurant service for Avito Kitchen.
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
	"restaurant-example/kafka"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type MenuItemDTO struct {
	ExternalID  string `json:"external_id"`
	Name        string `json:"name"`
	PriceCents  int64  `json:"price_cents"`
	IsAvailable bool   `json:"is_available"`
}

type UpsertMenuRequest struct {
	Items []MenuItemDTO `json:"items"`
}

type OrderItemDTO struct {
	ID             int64     `json:"id"`
	OrderID        int64     `json:"order_id"`
	MenuItemID     int64     `json:"menu_item_id"`
	Quantity       int       `json:"quantity"`
	UnitPriceCents int64     `json:"unit_price_cents"`
	CreatedAt      time.Time `json:"created_at"`
}

// Kafka Event DTOs
type OrderCreatedDTO struct {
	EventID         string         `json:"event_id"`
	OrderID         int64          `json:"order_id"`
	RestaurantID    int64          `json:"restaurant_id"`
	Items           []OrderItemDTO `json:"items"`
	TotalPriceCents int64          `json:"total_price_cents"`
	CreatedAt       time.Time      `json:"created_at"`
}

type OrderStatusUpdatedDTO struct {
	EventID      string    `json:"event_id"`
	OrderID      int64     `json:"order_id"`
	RestaurantID int64     `json:"restaurant_id"`
	Status       string    `json:"status"`
	UpdatedAt    time.Time `json:"updated_at"`
}

var initialMenu = []MenuItemDTO{
	{ExternalID: "pizza_margharita", Name: "Пицца Маргарита", PriceCents: 450000, IsAvailable: true},
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

func sendOrderStatusUpdate(ctx context.Context, producer *kafka.Producer, orderID, restaurantID int64, status string) error {
	event := OrderStatusUpdatedDTO{
		EventID:      fmt.Sprintf("%d-%s-%d", orderID, status, time.Now().UnixMilli()),
		OrderID:      orderID,
		RestaurantID: restaurantID,
		Status:       status,
		UpdatedAt:    time.Now(),
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}

	return producer.Publish(ctx, strconv.Itoa(int(orderID)), payload)
}

func processOrder(ctx context.Context, producer *kafka.Producer, order OrderCreatedDTO, stepDelay time.Duration, activeOrders *sync.Map, wg *sync.WaitGroup) {
	defer wg.Done()
	defer activeOrders.Delete(order.OrderID)

	orderID := order.OrderID
	log.Printf("[Order #%d] Received new order with %d item(s), total %d cents",
		orderID, len(order.Items), order.TotalPriceCents)

	// 1. accepted
	if err := sendOrderStatusUpdate(ctx, producer, orderID, order.RestaurantID, "accepted"); err != nil {
		log.Printf("[Order #%d] Failed to send status accepted: %v", orderID, err)
		return
	}
	log.Printf("[Order #%d] Status updated: created -> accepted", orderID)

	if !sleepContext(ctx, stepDelay) {
		return
	}

	// 2. cooking
	if err := sendOrderStatusUpdate(ctx, producer, orderID, order.RestaurantID, "cooking"); err != nil {
		log.Printf("[Order #%d] Failed to send status cooking: %v", orderID, err)
		return
	}
	log.Printf("[Order #%d] Status updated: accepted -> cooking", orderID)

	if !sleepContext(ctx, stepDelay) {
		return
	}

	// 3. ready
	if err := sendOrderStatusUpdate(ctx, producer, orderID, order.RestaurantID, "ready"); err != nil {
		log.Printf("[Order #%d] Failed to send status ready: %v", orderID, err)
		return
	}
	log.Printf("[Order #%d] Status updated: cooking -> ready", orderID)

	if !sleepContext(ctx, stepDelay) {
		return
	}

	// 4. completed
	if err := sendOrderStatusUpdate(ctx, producer, orderID, order.RestaurantID, "completed"); err != nil {
		log.Printf("[Order #%d] Failed to send status completed: %v", orderID, err)
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

func pollKafkaOrders(ctx context.Context, wg *sync.WaitGroup, stepDelay time.Duration, consumer *kafka.Consumer, producer *kafka.Producer, activeOrders *sync.Map) {
	defer wg.Done()
	reader := consumer.GetReader()

	log.Println("[polling] Started polling Kafka for new orders...")

	for {
		msg, err := reader.ReadMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || ctx.Err() != nil {
				log.Println("[polling] Context canceled, stopping Kafka order consumer")
				return
			}
			log.Printf("[polling] Error reading message from Kafka: %v", err)
			continue
		}

		var order OrderCreatedDTO
		if err := json.Unmarshal(msg.Value, &order); err != nil {
			log.Printf("[polling] Error unmarshaling order created event: %v", err)
			continue
		}

		if _, loaded := activeOrders.LoadOrStore(order.OrderID, true); loaded {
			log.Printf("[Order #%d] Already in progress, skipping duplicate event", order.OrderID)
			continue
		}

		wg.Add(1)
		go processOrder(ctx, producer, order, stepDelay, activeOrders, wg)
	}
}

func main() {
	apiURL := getEnv("API_URL", "http://localhost:8080")
	apiKey := getEnv("PARTNER_API_KEY", "partner_api_key_dominos")
	stepDelay := getEnvDuration("STEP_DELAY", 2*time.Second)
	kafkaAddr := getEnv("KAFKA_ADDR", "localhost:9092")
	readerTopic := getEnv("TOPIC_ORDERS", "orders.created")
	writerTopic := getEnv("TOPIC_STATUS", "orders.status.updates")
	consumerGroupID := getEnv("KAFKA_GROUP_ID", "restaurant-example-dominos")

	log.Printf("restaurant-example started for api_url=%s, api_key=%s, step_delay=%v, kafka_addr=%s, reader_topic=%s, writer_topic=%s",
		apiURL, apiKey, stepDelay, kafkaAddr, readerTopic, writerTopic)

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

	consumer, err := kafka.NewConsumer(kafkaAddr, readerTopic, consumerGroupID)
	if err != nil {
		log.Fatalf("failed to initialize kafka consumer: %v", err)
	}
	defer consumer.Close() //nolint:errcheck

	producer, err := kafka.NewProducer(kafkaAddr, writerTopic)
	if err != nil {
		log.Fatalf("failed to initialize kafka producer: %v", err)
	}
	defer producer.Close() //nolint:errcheck

	var activeOrders sync.Map
	var wg sync.WaitGroup

	wg.Add(1)
	go pollKafkaOrders(ctx, &wg, stepDelay, consumer, producer, &activeOrders)

	<-ctx.Done()
	log.Println("Termination signal received, waiting for ongoing orders to finish...")
	wg.Wait()
	log.Println("restaurant-example stopped cleanly")
}
