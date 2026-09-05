package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	apiURL := os.Getenv("API_URL")
	if apiURL == "" {
		apiURL = "http://localhost:8080"
	}

	apiKey := os.Getenv("PARTNER_API_KEY")
	if apiKey == "" {
		apiKey = "partner_api_key_dominos"
	}

	log.Printf("restaurant-example started for api_url=%s, api_key=%s", apiURL, apiKey)

	// TODO: send menu with PUT /api/v1/partner/menu
	// TODO: run polling GET /api/v1/partner/orders?status=created for every N seconds
	// TODO: implement status changing (created -> accepted -> cooking -> ready -> completed)

	testUrl := fmt.Sprintf("%s/health", apiURL)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("restaurant-example stopped")
			return
		case <-ticker.C:
			log.Println("polling kitchen-api for new orders...")
			resp, _ := http.Get(testUrl)
			body, _ := io.ReadAll(resp.Body)

			log.Println("Health response: ", string(body))
		}
	}
}
