package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"

	"whatsapp/src/core/event"
	httpgateway "whatsapp/src/core/gateway/http"
	"whatsapp/src/core/whatsapp"
	"whatsapp/src/plugins/webhook"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
	waLog "go.mau.fi/whatsmeow/util/log"
)

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		parsed, err := strconv.ParseBool(value)
		if err == nil {
			return parsed
		}
	}
	return defaultValue
}

func main() {
	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		log.Printf("No .env file found or error loading it: %v", err)
	}

	// Read configuration from environment
	httpPort := getEnv("HTTP_PORT", "8081")
	dbPath := getEnv("DB_PATH", "file:./whatsapp.db?_foreign_keys=on")
	logLevel := getEnv("WHATSAPP_LOG_LEVEL", "INFO")
	wmDebug := getEnvBool("WATERMILL_DEBUG", false)
	wmTrace := getEnvBool("WATERMILL_TRACE", false)

	// Create logger
	waLogger := waLog.Stdout("WhatsApp", logLevel, true)
	wmLogger := watermill.NewStdLogger(wmDebug, wmTrace)

	// Create event bus
	eventBus := event.NewEventBus(wmLogger)
	webhookPlugin, err := webhook.NewPlugin(eventBus)
	if err != nil {
		log.Fatalf("Failed to initialize webhook plugin: %v", err)
	}
	// Start webhook plugin
	if err := webhookPlugin.Start(); err != nil {
		log.Fatalf("Failed to start webhook plugin: %v", err)
	}
	defer eventBus.Close()

	// Create WhatsApp manager
	manager, err := whatsapp.NewManager(dbPath, waLogger)
	if err != nil {
		log.Fatalf("Failed to create WhatsApp manager: %v", err)
	}
	defer manager.Close()

	// Set event publisher
	manager.SetEventPublisher(eventBus)

	// Restore previously connected devices (failover recovery)
	fmt.Println("Restoring previously connected devices...")
	if err := manager.RestoreDevices(context.Background()); err != nil {
		log.Printf("Warning: Failed to restore devices: %v", err)
	}

	// Create Fiber HTTP server
	server := httpgateway.NewServer(manager)

	// Start HTTP server
	if httpPort[0] != ':' {
		httpPort = ":" + httpPort
	}
	fmt.Printf("Starting Fiber HTTP server on %s\n", httpPort)
	if err := server.Listen(httpPort); err != nil {
		log.Fatalf("Failed to start HTTP server: %v", err)
	}
}
