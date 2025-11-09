package event

import (
	"context"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/pubsub/gochannel"
)

// EventBus handles event publishing and subscribing using Watermill
type EventBus struct {
	pubSub *gochannel.GoChannel
	logger watermill.LoggerAdapter
}

// NewEventBus creates a new EventBus instance
func NewEventBus(logger watermill.LoggerAdapter) *EventBus {
	pubSub := gochannel.NewGoChannel(
		gochannel.Config{},
		logger,
	)

	return &EventBus{
		pubSub: pubSub,
		logger: logger,
	}
}

// Publish publishes an event to a topic
func (eb *EventBus) Publish(topic string, messages ...*message.Message) error {
	return eb.pubSub.Publish(topic, messages...)
}

// Subscribe subscribes to a topic
func (eb *EventBus) Subscribe(ctx context.Context, topic string) (<-chan *message.Message, error) {
	return eb.pubSub.Subscribe(ctx, topic)
}

// Close closes the event bus
func (eb *EventBus) Close() error {
	return eb.pubSub.Close()
}
