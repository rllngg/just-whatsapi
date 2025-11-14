package renrqueue

import (
	"context"
	"fmt"
	"log"
	"time"
)

// MessageHandler is a callback function that processes queue messages
// Return nil to mark the message as completed
// Return error to mark the message as failed with the error message
type MessageHandler func(msg *QueueItemResponse) error

// Subscriber manages polling and processing of queue messages
type Subscriber struct {
	client   *Client
	topic    string
	ref      *string
	interval time.Duration
	handler  MessageHandler
	logger   Logger
	ctx      context.Context
	cancel   context.CancelFunc
}

// Logger interface for subscriber logging
type Logger interface {
	Printf(format string, v ...interface{})
	Errorf(format string, v ...interface{})
	Infof(format string, v ...interface{})
}

// defaultLogger implements a basic logger using standard log package
type defaultLogger struct{}

func (l *defaultLogger) Printf(format string, v ...interface{}) {
	log.Printf(format, v...)
}

func (l *defaultLogger) Errorf(format string, v ...interface{}) {
	log.Printf("ERROR: "+format, v...)
}

func (l *defaultLogger) Infof(format string, v ...interface{}) {
	log.Printf("INFO: "+format, v...)
}

// SubscriberOption configures a Subscriber
type SubscriberOption func(*Subscriber)

// WithRef filters messages by reference
func WithRef(ref string) SubscriberOption {
	return func(s *Subscriber) {
		s.ref = &ref
	}
}

// WithLogger sets a custom logger for the subscriber
func WithLogger(logger Logger) SubscriberOption {
	return func(s *Subscriber) {
		s.logger = logger
	}
}

// Subscribe creates and starts a new queue subscriber
// topic: The queue topic to subscribe to
// interval: Polling interval (e.g., "1s", "500ms", "2s")
// handler: Callback function that processes messages
// opts: Optional configuration (WithRef, WithLogger)
//
// Example:
//
//	subscriber, err := client.Subscribe("my-topic", "1s", func(msg *QueueItemResponse) error {
//	    fmt.Printf("Processing message: %d\n", msg.ID)
//	    // Process the message...
//	    return nil // Return nil to mark as completed, or error to mark as failed
//	}, WithRef("my-ref"))
func (c *Client) Subscribe(topic, interval string, handler MessageHandler, opts ...SubscriberOption) (*Subscriber, error) {
	duration, err := time.ParseDuration(interval)
	if err != nil {
		return nil, fmt.Errorf("invalid interval: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	subscriber := &Subscriber{
		client:   c,
		topic:    topic,
		interval: duration,
		handler:  handler,
		logger:   &defaultLogger{},
		ctx:      ctx,
		cancel:   cancel,
	}

	// Apply options
	for _, opt := range opts {
		opt(subscriber)
	}

	// Start in background
	go subscriber.start()

	subscriber.logger.Infof("Subscriber started for topic: %s (interval: %s)", topic, interval)

	return subscriber, nil
}

// start begins polling the queue
func (s *Subscriber) start() {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			s.logger.Infof("Subscriber stopped for topic: %s", s.topic)
			return
		case <-ticker.C:
			s.processNextMessage()
		}
	}
}

// processNextMessage pulls and processes a single message from the queue
func (s *Subscriber) processNextMessage() {
	// Pull next message
	msg, err := s.client.Pull(s.topic, s.ref)
	if err != nil {
		s.logger.Errorf("Failed to pull message from queue '%s': %v", s.topic, err)
		return
	}

	// No messages available
	if msg == nil {
		return
	}

	s.logger.Infof("Processing message: topic=%s, id=%d, status=%s", s.topic, msg.ID, msg.Status)

	// Call the handler
	err = s.handler(msg)

	if err != nil {
		// Handler returned error - mark as failed
		s.logger.Errorf("Handler failed for message %d: %v", msg.ID, err)
		_, failErr := s.client.MarkAsFailed(s.topic, msg.ID, MarkFailedRequest{
			Reason: fmt.Sprintf("Handler error: %v", err),
		})
		if failErr != nil {
			s.logger.Errorf("Failed to mark message %d as failed: %v", msg.ID, failErr)
		}
	} else {
		// Handler succeeded - mark as completed
		_, completeErr := s.client.Complete(s.topic, msg.ID)
		if completeErr != nil {
			s.logger.Errorf("Failed to complete message %d: %v", msg.ID, completeErr)
		} else {
			s.logger.Infof("Message completed: topic=%s, id=%d", s.topic, msg.ID)
		}
	}
}

// Stop gracefully stops the subscriber
func (s *Subscriber) Stop() {
	s.logger.Infof("Stopping subscriber for topic: %s", s.topic)
	s.cancel()
}

// Topic returns the topic this subscriber is listening to
func (s *Subscriber) Topic() string {
	return s.topic
}

// IsRunning returns true if the subscriber is still running
func (s *Subscriber) IsRunning() bool {
	select {
	case <-s.ctx.Done():
		return false
	default:
		return true
	}
}
