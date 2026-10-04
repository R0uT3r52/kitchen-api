package main

import (
	"context"
	"encoding/json"
	"errors"
	"kitchen-api/internal/domain"
	"kitchen-api/internal/kafka"
	"kitchen-api/internal/repository"
	"kitchen-api/internal/web"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

func outboxWorkers(ctx context.Context, workersWg *sync.WaitGroup, numWorkers int, repo *repository.Repo, producer *kafka.Producer) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	defer workersWg.Done()

	log.Printf("Starting outbox worker pool with %d workers", numWorkers)

	for {
		select {
		case <-ctx.Done():
			log.Println("Shutting down outbox workers...")
			return

		case <-ticker.C:
			events, err := repo.GetOutboxEvents(ctx, 50)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}
				log.Printf("[outboxWorker ERROR] failed to fetch outbox events: %v", err)
				continue
			}

			if len(events) == 0 {
				continue
			}

			jobs := make(chan repository.OutboxItemRow, len(events))
			for _, event := range events {
				jobs <- event
			}
			close(jobs)

			var wg sync.WaitGroup
			for i := range numWorkers {
				wg.Add(1)
				go func(workerID int) {
					defer wg.Done()

					for event := range jobs {
						select {
						case <-ctx.Done():
							return
						default:
							// Publish event to Kafka
							if err := producer.Publish(ctx, event.EventKey, event.Payload); err != nil {
								log.Printf("[outboxWorker %d ERROR] failed to publish event ID %v: %v", workerID, event.ID, err)
								continue
							}

							// Mark event as processed
							if err := repo.MarkOutboxEventProcessed(ctx, event.ID); err != nil {
								log.Printf("[outboxWorker %d ERROR] failed to mark event ID %v as processed: %v", workerID, event.ID, err)
							}
						}
					}
				}(i)
			}

			wg.Wait()
		}
	}
}

func ordersWorkers(ctx context.Context, workersWg *sync.WaitGroup, numWorkers int, repo *repository.Repo, consumer *kafka.Consumer) {
	defer workersWg.Done()

	log.Printf("Starting orders worker pool with %d workers", numWorkers)
	reader := consumer.GetReader()

	var wg sync.WaitGroup
	for i := range numWorkers {
		wg.Add(1)
		go func(wID int) {
			defer wg.Done()
			for {
				msg, err := reader.ReadMessage(ctx)
				if err != nil {
					if errors.Is(err, context.Canceled) || ctx.Err() != nil {
						log.Printf("[ordersWorker %d] Context canceled, stopping worker", wID)
						return
					}
					log.Printf("[ordersWorker %d ERROR] failed to read message: %v", wID, err)
					continue
				}

				var data repository.OrderStatusUpdatedEvent
				if err := json.Unmarshal(msg.Value, &data); err != nil {
					log.Printf("[ordersWorker %d ERROR] failed to unmarshal payload: %v", wID, err)
					continue
				}

				if err := repo.UpdateOrderStatus(ctx, data.OrderID, domain.OrderStatus(data.Status)); err != nil {
					if errors.Is(err, context.Canceled) {
						return
					}
					log.Printf("[ordersWorker %d ERROR] failed to update status for order #%d to %s in database: %v",
						wID, data.OrderID, data.Status, err)
					continue
				}

				log.Printf("[ordersWorker %d] Updated order #%d status to %s", wID, data.OrderID, data.Status)
			}
		}(i)
	}

	wg.Wait()
	log.Println("Orders workers stopped")
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dbPool, err := repository.GetDB(context.Background())
	if err != nil {
		log.Fatalf("failed to init db: %v", err)
	}
	defer dbPool.Close()

	kafkaAddr := os.Getenv("KAFKA_ADDR")
	if kafkaAddr == "" {
		kafkaAddr = "localhost:9092"
	}

	topicOrders := os.Getenv("TOPIC_ORDERS")
	if topicOrders == "" {
		topicOrders = "orders.created"
	}

	topicStatus := os.Getenv("TOPIC_STATUS")
	if topicStatus == "" {
		topicStatus = "orders.status.updates"
	}

	kafkaProducer, err := kafka.NewProducer(kafkaAddr, topicOrders)
	if err != nil {
		log.Fatalf("failed to init kafka producer: %v", err)
	}
	defer kafkaProducer.Close() //nolint:errcheck

	kafkaConsumer, err := kafka.NewConsumer(kafkaAddr, topicStatus, "order_status_reader")
	if err != nil {
		log.Fatalf("failed to init kafka consumer: %v", err)
	}
	defer kafkaConsumer.Close() //nolint:errcheck

	repo := repository.NewRepo(dbPool)
	orderService := domain.NewOrderService(repo)
	partnerService := domain.NewPartnerService(repo)

	handler := web.NewHandler(orderService, partnerService)

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      handler.InitRoutes(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("kitchen-api starting on port %s", port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	workersWg := &sync.WaitGroup{}
	workerCtx, cancelWorkers := context.WithCancel(context.Background())

	workersWg.Add(2)
	go outboxWorkers(workerCtx, workersWg, 1, repo, kafkaProducer)
	go ordersWorkers(workerCtx, workersWg, 1, repo, kafkaConsumer)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	cancelWorkers()

	log.Println("shutting down kitchen-api...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("server forced to shutdown: %v", err)
	}
	workersWg.Wait()
	log.Println("kitchen-api stopped")
}
