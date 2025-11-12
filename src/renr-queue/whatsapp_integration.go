package renrqueue

import (
	"encoding/json"
	"fmt"
	"log"
)

// This file demonstrates simple integration examples for using the generic subscriber

// Example 1: Basic usage - subscribe to a topic with a simple handler
func ExampleBasicUsage() {
	// Create client from environment variables
	client, err := NewClientFromEnv()
	if err != nil {
		log.Fatalf("Failed to create queue client: %v", err)
	}

	// Subscribe to a topic with a callback that runs every 1 second
	subscriber, err := client.Subscribe("my-topic", "1s", func(msg *QueueItemResponse) error {
		fmt.Printf("Received message: ID=%d\n", msg.ID)

		// Parse the payload
		if msg.Payload != nil {
			fmt.Printf("Payload: %s\n", *msg.Payload)
		}

		// Process your message here...

		// Return nil to mark as completed, or error to mark as failed
		return nil
	})

	if err != nil {
		log.Fatalf("Failed to subscribe: %v", err)
	}

	// Subscriber is now running in the background
	// To stop it later:
	defer subscriber.Stop()

	// Your application continues running...
	select {}
}

// Example 2: Subscribe with reference filter
func ExampleWithRef() {
	client, err := NewClientFromEnv()
	if err != nil {
		log.Fatalf("Failed to create queue client: %v", err)
	}

	// Only process messages with ref="user-123"
	subscriber, err := client.Subscribe("notifications", "500ms", func(msg *QueueItemResponse) error {
		fmt.Printf("Processing notification for user-123: %d\n", msg.ID)
		return nil
	}, WithRef("user-123"))

	if err != nil {
		log.Fatalf("Failed to subscribe: %v", err)
	}

	defer subscriber.Stop()
	select {}
}

// Example 3: Multiple subscribers
func ExampleMultipleSubscribers() {
	client, err := NewClientFromEnv()
	if err != nil {
		log.Fatalf("Failed to create queue client: %v", err)
	}

	// Subscribe to multiple topics
	sub1, _ := client.Subscribe("emails", "1s", func(msg *QueueItemResponse) error {
		fmt.Println("Processing email...")
		return nil
	})

	sub2, _ := client.Subscribe("sms", "500ms", func(msg *QueueItemResponse) error {
		fmt.Println("Processing SMS...")
		return nil
	})

	sub3, _ := client.Subscribe("push-notifications", "2s", func(msg *QueueItemResponse) error {
		fmt.Println("Processing push notification...")
		return nil
	})

	// Stop all subscribers on shutdown
	defer func() {
		sub1.Stop()
		sub2.Stop()
		sub3.Stop()
	}()

	select {}
}

// Example 4: Error handling - returning error marks message as failed
func ExampleErrorHandling() {
	client, err := NewClientFromEnv()
	if err != nil {
		log.Fatalf("Failed to create queue client: %v", err)
	}

	subscriber, err := client.Subscribe("processing-tasks", "1s", func(msg *QueueItemResponse) error {
		// Parse payload
		var task map[string]interface{}
		if msg.Payload != nil {
			if err := json.Unmarshal([]byte(*msg.Payload), &task); err != nil {
				// Return error to mark as failed - the queue will retry automatically
				return fmt.Errorf("invalid payload: %w", err)
			}
		}

		// Process the task...
		taskType, ok := task["type"].(string)
		if !ok {
			return fmt.Errorf("missing task type")
		}

		fmt.Printf("Processing task type: %s\n", taskType)

		// Simulate processing...
		// If processing fails, return error:
		// return fmt.Errorf("processing failed: %v", someError)

		// Success - return nil to mark as completed
		return nil
	})

	if err != nil {
		log.Fatalf("Failed to subscribe: %v", err)
	}

	defer subscriber.Stop()
	select {}
}

// Example 5: Custom logger
type customLogger struct{}

func (l *customLogger) Printf(format string, v ...interface{}) {
	log.Printf("[QUEUE] "+format, v...)
}

func (l *customLogger) Errorf(format string, v ...interface{}) {
	log.Printf("[QUEUE ERROR] "+format, v...)
}

func (l *customLogger) Infof(format string, v ...interface{}) {
	log.Printf("[QUEUE INFO] "+format, v...)
}

func ExampleCustomLogger() {
	client, err := NewClientFromEnv()
	if err != nil {
		log.Fatalf("Failed to create queue client: %v", err)
	}

	logger := &customLogger{}

	subscriber, err := client.Subscribe("my-topic", "1s", func(msg *QueueItemResponse) error {
		fmt.Printf("Processing message %d\n", msg.ID)
		return nil
	}, WithLogger(logger))

	if err != nil {
		log.Fatalf("Failed to subscribe: %v", err)
	}

	defer subscriber.Stop()
	select {}
}
