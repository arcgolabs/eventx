package eventx

import (
	"context"
	"reflect"

	collectionmapping "github.com/arcgolabs/collectionx/mapping"
)

// Event is the common event contract for strongly typed publish/subscribe.
type Event interface {
	Name() string
}

// BusRuntime is the public event bus contract exposed to callers.
type BusRuntime interface {
	Publish(ctx context.Context, event Event) error
	PublishAsync(ctx context.Context, event Event) error
	Close() error
	SubscriberCount() int
	GetHandlersGroupedByEventType() *collectionmapping.MultiMap[reflect.Type, HandlerFunc]

	// subscribe is intentionally unexported so only eventx's own implementation
	// can satisfy BusRuntime.
	subscribe(eventType reflect.Type, handler HandlerFunc, subscriberMiddleware []Middleware, maxCalls int) (func(), error)
}

type subscription struct {
	id      uint64
	handler HandlerFunc
}

type publishTask struct {
	ctx      context.Context
	event    Event
	handlers []HandlerFunc
}

// subscriptionTable is a concurrent table for storing subscriptions by (event type, subscription id).
type subscriptionTable = *collectionmapping.ConcurrentTable[reflect.Type, uint64, *subscription]
