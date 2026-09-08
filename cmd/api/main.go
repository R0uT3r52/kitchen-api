package main

import (
	"context"
	"errors"
	"kitchen-api/internal/domain"
	"kitchen-api/internal/repository"
	"kitchen-api/internal/web"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

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

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down kitchen-api...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("server forced to shutdown: %v", err)
	}
	log.Println("kitchen-api stopped")
}
