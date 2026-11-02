// Package main provides a demonstration partner restaurant service for Avito Kitchen.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	partnerv1 "restaurant-example/api/proto/partner/v1"
	"restaurant-example/kafka"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
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

type OrderStatusCancelledDTO struct {
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

func waitKitchenAPIGRPC(ctx context.Context, conn *grpc.ClientConn, timeout time.Duration) error {
	healthClient := grpc_health_v1.NewHealthClient(conn)
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	var lastErr error
	for {
		checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		resp, err := healthClient.Check(checkCtx, &grpc_health_v1.HealthCheckRequest{})
		cancel()

		if err == nil && resp.GetStatus() == grpc_health_v1.HealthCheckResponse_SERVING {
			return nil
		}
		if err != nil {
			lastErr = err
		} else {
			lastErr = fmt.Errorf("gRPC health status is %s", resp.GetStatus())
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case t := <-ticker.C:
			if t.After(deadline) {
				return fmt.Errorf("kitchen-api gRPC not healthy within %v: %w", timeout, lastErr)
			}
		}
	}
}

func syncMenuGRPC(ctx context.Context, client partnerv1.MenuServiceClient, apiKey string, items []MenuItemDTO) error {
	newCtx := metadata.AppendToOutgoingContext(ctx, "x-api-key", apiKey)

	syncCtx, cancel := context.WithTimeout(newCtx, 5*time.Second)
	defer cancel()

	data := make([]*partnerv1.MenuItem, 0, len(items))
	for i := range items {
		data = append(data, &partnerv1.MenuItem{
			ExternalId:  items[i].ExternalID,
			Name:        items[i].Name,
			PriceCents:  items[i].PriceCents,
			IsAvailable: items[i].IsAvailable,
		})
	}

	_, err := client.SyncMenu(syncCtx, &partnerv1.SyncMenuRequest{
		Items: data,
	})
	if err != nil {
		return fmt.Errorf("menu sync request failed: %v", err)
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
	defer func() {
		cancel, loaded := activeOrders.Load(order.OrderID)
		if loaded {
			cancelFunc := cancel.(context.CancelFunc)
			activeOrders.Delete(order.OrderID)
			cancelFunc()
		}
	}()

	orderID := order.OrderID
	log.Printf("[Order #%d] Received new order with %d item(s), total %d cents",
		orderID, len(order.Items), order.TotalPriceCents)

	if ctx.Err() != nil {
		log.Printf("[Order #%d] Order cancelled before processing started", order.OrderID)
		return
	}

	// 1. accepted
	if err := sendOrderStatusUpdate(ctx, producer, orderID, order.RestaurantID, "accepted"); err != nil {
		log.Printf("[Order #%d] Failed to send status accepted: %v", orderID, err)
		return
	}
	log.Printf("[Order #%d] Status updated: created -> accepted", orderID)

	if !sleepContext(ctx, stepDelay) {
		log.Printf("[Order #%d] Cooking pipeline terminated due to order cancellation", order.OrderID)
		return
	}

	// 2. cooking
	if err := sendOrderStatusUpdate(ctx, producer, orderID, order.RestaurantID, "cooking"); err != nil {
		log.Printf("[Order #%d] Failed to send status cooking: %v", orderID, err)
		return
	}
	log.Printf("[Order #%d] Status updated: accepted -> cooking", orderID)

	if !sleepContext(ctx, stepDelay) {
		log.Printf("[Order #%d] Cooking pipeline terminated due to order cancellation", order.OrderID)
		return
	}

	// 3. ready
	if err := sendOrderStatusUpdate(ctx, producer, orderID, order.RestaurantID, "ready"); err != nil {
		log.Printf("[Order #%d] Failed to send status ready: %v", orderID, err)
		return
	}
	log.Printf("[Order #%d] Status updated: cooking -> ready", orderID)

	if !sleepContext(ctx, stepDelay) {
		log.Printf("[Order #%d] Cooking pipeline terminated due to order cancellation", order.OrderID)
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

func pollKafkaOrders(ctx context.Context, wg *sync.WaitGroup, stepDelay time.Duration, consumer *kafka.Consumer, producer *kafka.Producer, activeOrders, cancelledOrders *sync.Map) {
	defer wg.Done()

	log.Println("[polling] Started polling Kafka for new orders...")

	for {
		msg, err := consumer.ReadMessage(ctx)
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

		if _, loaded := activeOrders.Load(order.OrderID); loaded {
			log.Printf("[Order #%d] Already in progress, skipping duplicate event", order.OrderID)
			continue
		}

		if _, loaded := cancelledOrders.Load(order.OrderID); loaded {
			log.Printf("[Order #%d] Order was cancelled before cooking", order.OrderID)
			continue
		}

		orderCtx, cancel := context.WithCancel(ctx)
		activeOrders.Store(order.OrderID, cancel)

		wg.Add(1)
		// context.Background() to prevent cooking process stop
		go processOrder(orderCtx, producer, order, stepDelay, activeOrders, wg)
	}
}

func pollKafkaCancelledOrders(ctx context.Context, wg *sync.WaitGroup, consumer *kafka.Consumer, activeOrders, cancelledOrders *sync.Map) {
	defer wg.Done()

	log.Println("[cancel polling] Start polling kafka for cancelled orders...")

	for {
		msg, err := consumer.ReadMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || ctx.Err() != nil {
				log.Println("[cancel polling] Stopped polling kafka for cancelled orders...")
				return
			}
			log.Printf("[cancel polling] Error reading message from Kafka: %v", err)
			continue
		}

		var order OrderStatusCancelledDTO
		if err := json.Unmarshal(msg.Value, &order); err != nil {
			log.Printf("[cancel polling] Failed to unmarshal cancelled order: %v", err)
			continue
		}

		if cancel, loaded := activeOrders.Load(order.OrderID); loaded {
			cancelFunc := cancel.(context.CancelFunc)
			log.Printf("[cancel polling] Cancelling Order #%d with status %s", order.OrderID, order.Status)
			cancelFunc()
			continue
		}

		cancelledOrders.Store(order.OrderID, struct{}{})
		log.Printf("[cancel polling] Got cancel message, but no active order found")
	}
}

func main() {
	apiURL := getEnv("API_URL", "http://localhost:8080")
	apiKey := getEnv("PARTNER_API_KEY", "partner_api_key_dominos")
	stepDelay := getEnvDuration("STEP_DELAY", 2*time.Second)
	kafkaAddr := getEnv("KAFKA_ADDR", "localhost:9092")
	ordersTopic := getEnv("TOPIC_ORDERS", "orders.created")
	statusTopic := getEnv("TOPIC_STATUS", "orders.status.updates")
	cancelTopic := getEnv("TOPIC_CANCEL", "orders.cancelled")
	consumerGroupID := getEnv("KAFKA_GROUP_ID", "restaurant-example-dominos")
	grpcAddr := getEnv("GRPC_ADDR", "localhost:50051")

	log.Printf("restaurant-example started for api_url=%s, api_key=%s, step_delay=%v, kafka_addr=%s, orders_topic=%s, status_topic=%s, cancel_topic=%s",
		apiURL, apiKey, stepDelay, kafkaAddr, ordersTopic, statusTopic, cancelTopic)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	httpClient := &http.Client{Timeout: 10 * time.Second}

	conn, err := grpc.NewClient(
		grpcAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("Failed to create connection to gRPC server: %v", err)
	}
	defer conn.Close() //nolint:errcheck

	client := partnerv1.NewMenuServiceClient(conn)

	log.Println("Waiting for kitchen-api readiness (HTTP)...")
	if err := waitKitchenAPI(ctx, httpClient, apiURL, 30*time.Second); err != nil {
		log.Fatalf("failed waiting for kitchen-api HTTP: %v", err)
	}

	log.Println("Waiting for kitchen-api readiness (gRPC)...")
	if err := waitKitchenAPIGRPC(ctx, conn, 30*time.Second); err != nil {
		log.Fatalf("failed waiting for kitchen-api gRPC: %v", err)
	}
	log.Println("kitchen-api is ready!")

	log.Printf("Synchronizing menu (%d items)...", len(initialMenu))
	if err := syncMenuGRPC(ctx, client, apiKey, initialMenu); err != nil {
		log.Fatalf("failed to synchronize menu: %v", err)
	}
	log.Println("Menu synchronized successfully!")

	consumer, err := kafka.NewConsumer(kafkaAddr, ordersTopic, consumerGroupID)
	if err != nil {
		log.Fatalf("failed to initialize kafka consumer: %v", err)
	}
	defer consumer.Close() //nolint:errcheck

	consumerCancel, err := kafka.NewConsumer(kafkaAddr, cancelTopic, consumerGroupID+"-cancel")
	if err != nil {
		log.Fatalf("failed to initialize kafka consumer: %v", err)
	}

	producer, err := kafka.NewProducer(kafkaAddr, statusTopic)
	if err != nil {
		log.Fatalf("failed to initialize kafka producer: %v", err)
	}
	defer producer.Close() //nolint:errcheck

	var activeOrders, cancelledOrders sync.Map
	var wg sync.WaitGroup

	pollCtx, cancelPoll := context.WithCancel(context.Background())

	wg.Add(2)
	go pollKafkaOrders(pollCtx, &wg, stepDelay, consumer, producer, &activeOrders, &cancelledOrders)
	go pollKafkaCancelledOrders(pollCtx, &wg, consumerCancel, &activeOrders, &cancelledOrders)

	<-ctx.Done()
	cancelPoll()

	cookCtx, cancelCooking := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelCooking()

	done := make(chan struct{})

	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("restaurant-example stopped cleanly")
	case <-cookCtx.Done():
		log.Println("Cooking timeout exited, terminating...")
	}
}
