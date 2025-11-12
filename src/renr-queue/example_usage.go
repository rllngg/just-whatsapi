package renrqueue

import (
	"fmt"
	"log"
)

// ExampleUsage demonstrates how to use the queue client
func ExampleUsage() {
	// Option 1: Load .env file and create client from environment variables
	if err := LoadEnv(); err != nil {
		log.Printf("Warning: Could not load .env file: %v", err)
	}

	client, err := NewClientFromEnv()
	if err != nil {
		log.Fatalf("Failed to create client from environment: %v", err)
	}

	// Option 2: Create client manually with explicit credentials
	// client := NewClient("https://api.renr.biz.id", "username", "password")

	// Enqueue a message
	enqueueReq := EnqueueRequest{
		Ref:     StringPtr("order-12345"),
		Payload: `{"to":"user@example.com","subject":"Order Confirmation"}`,
	}

	item, err := client.Enqueue("email", enqueueReq)
	if err != nil {
		log.Fatalf("Failed to enqueue message: %v", err)
	}
	fmt.Printf("Message enqueued: ID=%d, Status=%s\n", item.ID, item.Status)

	// Pull next message from queue
	msg, err := client.Pull("email", nil)
	if err != nil {
		log.Fatalf("Failed to pull message: %v", err)
	}

	if msg == nil {
		fmt.Println("No messages available in queue")
		return
	}

	fmt.Printf("Processing message ID=%d\n", msg.ID)

	// Simulate processing...
	// If successful:
	completed, err := client.Complete("email", msg.ID)
	if err != nil {
		log.Fatalf("Failed to complete message: %v", err)
	}
	fmt.Printf("Message completed: ID=%d, Status=%s\n", completed.ID, completed.Status)

	// If failed (will automatically retry if retries available):
	// failed, err := client.MarkAsFailed("email", msg.ID, MarkFailedRequest{
	// 	Reason: "Connection timeout after 30 seconds",
	// })
	// if err != nil {
	// 	log.Fatalf("Failed to mark message as failed: %v", err)
	// }
	// fmt.Printf("Message failed: ID=%d, WillRetry=%v, NextRetryAt=%v\n",
	// 	failed.ID, failed.WillRetry, failed.NextRetryAt)

	// Get queue statistics
	stats, err := client.GetStats("email")
	if err != nil {
		log.Fatalf("Failed to get stats: %v", err)
	}
	fmt.Printf("Queue stats: Pending=%d, Processing=%d, Completed=%d, Failed=%d\n",
		stats.Pending, stats.Processing, stats.Completed, stats.Failed)

	// Get failed messages (dead letter queue)
	failedMsgs, err := client.GetFailedMessages("email", 0, 20)
	if err != nil {
		log.Fatalf("Failed to get failed messages: %v", err)
	}
	fmt.Printf("Failed messages: %d total, %d pages\n",
		failedMsgs.TotalElements, failedMsgs.TotalPages)

	// Retry a specific failed message
	if len(failedMsgs.Content) > 0 {
		retryResult, err := client.RetryFailed("email", failedMsgs.Content[0].ID)
		if err != nil {
			log.Fatalf("Failed to retry message: %v", err)
		}
		fmt.Printf("Message retried: OriginalID=%d, NewID=%d\n",
			retryResult.OriginalID, retryResult.NewQueueItem.ID)
	}
}
