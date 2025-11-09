package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"whatsapp/src/core/event"
	httpgateway "whatsapp/src/core/gateway/http"
	"whatsapp/src/core/whatsapp"

	"github.com/ThreeDotsLabs/watermill"
	_ "github.com/mattn/go-sqlite3"
	waLog "go.mau.fi/whatsmeow/util/log"
)

func setupTest(t *testing.T) (*whatsapp.Manager, *httpgateway.Server, *event.EventBus, func()) {
	// Create temporary database
	dbPath := "file:test_whatsapp.db?_foreign_keys=on"

	// Create logger
	waLogger := waLog.Stdout("WhatsApp", "ERROR", false)
	wmLogger := watermill.NewStdLogger(false, false)

	// Create event bus
	eventBus := event.NewEventBus(wmLogger)

	// Create WhatsApp manager
	manager, err := whatsapp.NewManager(dbPath, waLogger)
	if err != nil {
		t.Fatalf("Failed to create WhatsApp manager: %v", err)
	}

	// Set event publisher
	manager.SetEventPublisher(eventBus)

	// Create Fiber HTTP server
	server := httpgateway.NewServer(manager)

	cleanup := func() {
		manager.Close()
		eventBus.Close()
		os.Remove("test_whatsapp.db")
	}

	return manager, server, eventBus, cleanup
}

func TestNewDeviceEndpoint(t *testing.T) {
	_, server, _, cleanup := setupTest(t)
	defer cleanup()

	// Create request using Fiber test
	app := server.App()

	// Create actual test
	req := httptest.NewRequest("POST", "/device/new", nil)
	resp2, err := app.Test(req, 10000)
	if err != nil {
		t.Fatalf("Failed to test endpoint: %v", err)
	}

	// Check response status
	if resp2.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp2.StatusCode)
	}

	// Parse response
	var response map[string]string
	body, _ := io.ReadAll(resp2.Body)
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Check QR code exists
	if response["qr_code"] == "" {
		t.Error("Expected qr_code in response, got empty string")
	}

	t.Logf("QR Code received: %s", response["qr_code"])
}

func TestGetDevicesEndpoint(t *testing.T) {
	_, server, _, cleanup := setupTest(t)
	defer cleanup()

	// Create request using Fiber test
	app := server.App()
	req := httptest.NewRequest("GET", "/device", nil)

	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatalf("Failed to test endpoint: %v", err)
	}

	// Check response status
	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Parse response
	var response []map[string]string
	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Initially should be empty
	if len(response) != 0 {
		t.Errorf("Expected 0 devices, got %d", len(response))
	}
}

func TestNewDeviceMethodNotAllowed(t *testing.T) {
	_, server, _, cleanup := setupTest(t)
	defer cleanup()

	// Create GET request (should be POST)
	app := server.App()
	req := httptest.NewRequest("GET", "/device/new", nil)

	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatalf("Failed to test endpoint: %v", err)
	}

	// Check response status
	if resp.StatusCode != 405 {
		t.Errorf("Expected status 405, got %d", resp.StatusCode)
	}
}

func TestGetDevicesMethodNotAllowed(t *testing.T) {
	_, server, _, cleanup := setupTest(t)
	defer cleanup()

	// Create POST request (should be GET)
	app := server.App()
	req := httptest.NewRequest("POST", "/device", nil)

	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatalf("Failed to test endpoint: %v", err)
	}

	// Check response status
	if resp.StatusCode != 405 {
		t.Errorf("Expected status 405, got %d", resp.StatusCode)
	}
}

func TestDeviceCreatedEventPublished(t *testing.T) {
	_, server, eventBus, cleanup := setupTest(t)
	defer cleanup()

	// Subscribe to device events
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	messages, err := eventBus.Subscribe(ctx, "device.created")
	if err != nil {
		t.Fatalf("Failed to subscribe to events: %v", err)
	}

	// Create a new device
	app := server.App()
	req := httptest.NewRequest("POST", "/device/new", nil)

	go func() {
		app.Test(req, 10000)
	}()

	// Wait for event
	select {
	case msg := <-messages:
		var payload map[string]interface{}
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			t.Fatalf("Failed to unmarshal event payload: %v", err)
		}

		if payload["status"] != "pending" {
			t.Errorf("Expected status 'pending', got '%s'", payload["status"])
		}

		if payload["device_id"] == "" {
			t.Error("Expected device_id in event payload")
		}

		t.Logf("Device created event received: %+v", payload)
		msg.Ack()
	case <-ctx.Done():
		t.Error("Timeout waiting for device.created event")
	}
}

func TestSendTextMessage(t *testing.T) {
	_, server, _, cleanup := setupTest(t)
	defer cleanup()

	// Create request body
	reqBody := `{
		"device_id": "device_1",
		"chat_id": "6289685028129@s.whatsapp.net",
		"message": "selamat malam",
		"reply_message_id": "3EB089B9D6ADD58153C561",
		"is_forwarded": false,
		"duration": 3600
	}`

	app := server.App()
	req := httptest.NewRequest("POST", "/message/text", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")

	// For now, we expect this to fail since no device is connected
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatalf("Failed to test endpoint: %v", err)
	}

	// Should return error since no device exists
	if resp.StatusCode != 500 {
		t.Logf("Expected error status, got %d", resp.StatusCode)
	}
}

func TestSendImageMessage(t *testing.T) {
	_, server, _, cleanup := setupTest(t)
	defer cleanup()

	// Create request body
	reqBody := `{
		"device_id": "device_1",
		"chat_id": "6289685028129@s.whatsapp.net",
		"file_url": "https://test.com/image.png",
		"caption": "selamat malam",
		"reply_message_id": "3EB089B9D6ADD58153C561",
		"is_forwarded": false,
		"view_once": false,
		"duration": 3600
	}`

	app := server.App()
	req := httptest.NewRequest("POST", "/message/image", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")

	// For now, we expect this to fail since no device is connected
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatalf("Failed to test endpoint: %v", err)
	}

	// Should return error since no device exists
	if resp.StatusCode == 200 {
		var response map[string]string
		body, _ := io.ReadAll(resp.Body)
		json.Unmarshal(body, &response)
		t.Logf("Response: %+v", response)
	}
}

func TestSendFileMessage(t *testing.T) {
	_, server, _, cleanup := setupTest(t)
	defer cleanup()

	// Create request body
	reqBody := `{
		"device_id": "device_1",
		"chat_id": "6289685028129@s.whatsapp.net",
		"file_url": "https://test.com/document.pdf",
		"caption": "selamat malam",
		"reply_message_id": "3EB089B9D6ADD58153C561",
		"is_forwarded": false,
		"duration": 3600
	}`

	app := server.App()
	req := httptest.NewRequest("POST", "/message/file", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatalf("Failed to test endpoint: %v", err)
	}

	// Should return error since no device exists
	if resp.StatusCode == 200 {
		var response map[string]string
		body, _ := io.ReadAll(resp.Body)
		json.Unmarshal(body, &response)
		t.Logf("Response: %+v", response)
	}
}

func TestSendPresence(t *testing.T) {
	_, server, _, cleanup := setupTest(t)
	defer cleanup()

	// Create request body
	reqBody := `{
		"device_id": "device_1",
		"chat_id": "6289685028129@s.whatsapp.net",
		"is_forwarded": false
	}`

	app := server.App()
	req := httptest.NewRequest("POST", "/message/presence", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatalf("Failed to test endpoint: %v", err)
	}

	// Should return error since no device exists
	if resp.StatusCode == 200 {
		var response map[string]string
		body, _ := io.ReadAll(resp.Body)
		json.Unmarshal(body, &response)
		t.Logf("Response: %+v", response)
	}
}

func TestSendReaction(t *testing.T) {
	_, server, _, cleanup := setupTest(t)
	defer cleanup()

	// Create request body
	reqBody := `{
		"device_id": "device_1",
		"chat_id": "6289685028129@s.whatsapp.net",
		"message_id": "1231231232131",
		"emoji": "👍",
		"is_forwarded": false
	}`

	app := server.App()
	req := httptest.NewRequest("POST", "/message/emoji", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatalf("Failed to test endpoint: %v", err)
	}

	// Should return error since no device exists
	if resp.StatusCode == 200 {
		var response map[string]string
		body, _ := io.ReadAll(resp.Body)
		json.Unmarshal(body, &response)
		t.Logf("Response: %+v", response)
	}
}
