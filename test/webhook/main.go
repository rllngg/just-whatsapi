package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

// WebhookPayload represents a received webhook
type WebhookPayload struct {
	Timestamp time.Time              `json:"timestamp"`
	Headers   map[string][]string    `json:"headers"`
	Body      map[string]interface{} `json:"body"`
}

// WebhookStore manages received webhooks
type WebhookStore struct {
	mu       sync.RWMutex
	webhooks []WebhookPayload
}

var store = &WebhookStore{
	webhooks: make([]WebhookPayload, 0),
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	http.HandleFunc("/webhook", handleWebhook)
	http.HandleFunc("/webhooks", handleGetWebhooks)
	http.HandleFunc("/webhooks/clear", handleClearWebhooks)
	http.HandleFunc("/health", handleHealth)

	log.Printf("Webhook receiver starting on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}

// handleWebhook receives and stores webhooks
func handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	payload := WebhookPayload{
		Timestamp: time.Now(),
		Headers:   r.Header,
		Body:      body,
	}

	store.mu.Lock()
	store.webhooks = append(store.webhooks, payload)
	store.mu.Unlock()

	// Log the webhook
	log.Printf("[%s] Webhook received", payload.Timestamp.Format(time.RFC3339))
	log.Printf("Headers: %v", r.Header)
	log.Printf("Body: %v", body)

	response := map[string]interface{}{
		"success":   true,
		"message":   "Webhook received",
		"timestamp": payload.Timestamp,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleGetWebhooks returns all received webhooks
func handleGetWebhooks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	store.mu.RLock()
	defer store.mu.RUnlock()

	response := map[string]interface{}{
		"count":    len(store.webhooks),
		"webhooks": store.webhooks,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleClearWebhooks clears all stored webhooks
func handleClearWebhooks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	store.mu.Lock()
	count := len(store.webhooks)
	store.webhooks = make([]WebhookPayload, 0)
	store.mu.Unlock()

	response := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Cleared %d webhooks", count),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleHealth returns health status
func handleHealth(w http.ResponseWriter, r *http.Request) {
	response := map[string]string{
		"status": "ok",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
