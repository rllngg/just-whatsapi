package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"whatsapp/src/core/event"
	httpgateway "whatsapp/src/core/gateway/http"
	"whatsapp/src/core/whatsapp"

	"github.com/ThreeDotsLabs/watermill"
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
	defer eventBus.Close()

	// Create WhatsApp manager
	manager, err := whatsapp.NewManager(dbPath, waLogger)
	if err != nil {
		log.Fatalf("Failed to create WhatsApp manager: %v", err)
	}
	defer manager.Close()

	// Set event publisher
	manager.SetEventPublisher(eventBus)

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
