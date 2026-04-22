package eventx

import (
	"context"
	"reflect"
	"sync"
	"sync/atomic"

	"github.com/samber/oops"
)

// Subscribe registers a strongly typed handler and returns an unsubscribe function.
func Subscribe[T Event](b BusRuntime, handler func(context.Context, T) error, opts ...SubscribeOption) (func(), error) {
	eventType := reflect.TypeFor[T]()
	if b == nil {
		return nil, oops.In("eventx").
			With("op", "subscribe", "expected_event_type", eventType.String()).
			Wrapf(ErrNilBus, "eventx: validate subscribe bus")
	}
	if handler == nil {
		return nil, oops.In("eventx").
			With("op", "subscribe", "expected_event_type", eventType.String()).
			Wrapf(ErrNilHandler, "eventx: validate subscribe handler")
	}

	cfg := buildSubscribeOptions(opts...)
	_, base := typedEventHandler(handler)

	return b.subscribe(eventType, base, cfg.middleware, 0)
}

// SubscribeOnce registers a strongly typed handler that will auto-unsubscribe
// after handling one event.
func SubscribeOnce[T Event](b BusRuntime, handler func(context.Context, T) error, opts ...SubscribeOption) (func(), error) {
	return SubscribeN(b, 1, handler, opts...)
}

// SubscribeN registers a strongly typed handler that will auto-unsubscribe
// after handling n events.
func SubscribeN[T Event](b BusRuntime, n int, handler func(context.Context, T) error, opts ...SubscribeOption) (func(), error) {
	eventType := reflect.TypeFor[T]()
	if n <= 0 {
		return nil, oops.In("eventx").
			With("op", "subscribe", "expected_event_type", eventType.String(), "max_calls", n).
			Wrapf(ErrInvalidSubscribeCount, "eventx: validate subscribe count")
	}
	if b == nil {
		return nil, oops.In("eventx").
			With("op", "subscribe", "expected_event_type", eventType.String(), "max_calls", n).
			Wrapf(ErrNilBus, "eventx: validate subscribe bus")
	}
	if handler == nil {
		return nil, oops.In("eventx").
			With("op", "subscribe", "expected_event_type", eventType.String(), "max_calls", n).
			Wrapf(ErrNilHandler, "eventx: validate subscribe handler")
	}

	cfg := buildSubscribeOptions(opts...)
	_, base := typedEventHandler(handler)

	return b.subscribe(eventType, base, cfg.middleware, n)
}

func typedEventHandler[T Event](handler func(context.Context, T) error) (reflect.Type, HandlerFunc) {
	eventType := reflect.TypeFor[T]()
	return eventType, func(ctx context.Context, event Event) error {
		typed, ok := any(event).(T)
		if !ok {
			return oops.In("eventx").
				With("op", "dispatch", "expected_event_type", eventType.String(), "actual_event_type", reflect.TypeOf(event)).
				Errorf("eventx: event type mismatch, expect %v, got %T", eventType, event)
		}
		return handler(ctx, typed)
	}
}

func (b *Bus) subscribe(eventType reflect.Type, base HandlerFunc, subscriberMiddleware []Middleware, maxCalls int) (func(), error) {
	if b == nil {
		return nil, oops.In("eventx").
			With("op", "subscribe", "expected_event_type", eventType.String(), "max_calls", maxCalls).
			Wrapf(ErrNilBus, "eventx: validate subscribe bus")
	}

	finalHandler := b.subscriptionHandler(base, subscriberMiddleware)
	id, err := b.registerSubscription(eventType, func(id uint64) HandlerFunc {
		return b.subscriptionDispatchHandler(eventType, id, finalHandler, maxCalls)
	})
	if err != nil {
		return nil, err
	}

	return b.unsubscribeFunc(eventType, id), nil
}

func (b *Bus) snapshotHandlersByEventType(eventType reflect.Type) []HandlerFunc {
	if cached, ok := b.handlerCache.Get(eventType); ok {
		return cached
	}

	row := b.subsByType.Row(eventType)
	if len(row) == 0 {
		return nil
	}

	snapshot := make([]HandlerFunc, 0, len(row))
	for _, sub := range row {
		if sub == nil || sub.handler == nil {
			continue
		}
		snapshot = append(snapshot, sub.handler)
	}
	b.handlerCache.Set(eventType, snapshot)
	b.logger.Debug("handler snapshot rebuilt",
		"event_type", eventType.String(),
		"handler_count", len(snapshot),
	)
	return snapshot
}

func (b *Bus) subscriptionHandler(base HandlerFunc, subscriberMiddleware []Middleware) HandlerFunc {
	return chain(chain(base, subscriberMiddleware), b.middleware)
}

func (b *Bus) subscriptionDispatchHandler(
	eventType reflect.Type,
	id uint64,
	handler HandlerFunc,
	maxCalls int,
) HandlerFunc {
	if maxCalls <= 0 {
		return handler
	}
	return b.limitedSubscriptionHandler(eventType, id, handler, maxCalls)
}

func (b *Bus) limitedSubscriptionHandler(
	eventType reflect.Type,
	id uint64,
	handler HandlerFunc,
	maxCalls int,
) HandlerFunc {
	var remaining atomic.Int64
	remaining.Store(int64(maxCalls))

	return func(ctx context.Context, event Event) error {
		current, ok := consumeSubscriptionCall(&remaining)
		if !ok {
			return nil
		}
		if current == 1 {
			b.deleteSubscription(eventType, id)
		}
		return handler(ctx, event)
	}
}

func consumeSubscriptionCall(remaining *atomic.Int64) (int64, bool) {
	for {
		current := remaining.Load()
		if current <= 0 {
			return 0, false
		}
		if remaining.CompareAndSwap(current, current-1) {
			return current, true
		}
	}
}

func (b *Bus) unsubscribeFunc(eventType reflect.Type, id uint64) func() {
	var once sync.Once
	return func() {
		once.Do(func() {
			b.deleteSubscription(eventType, id)
		})
	}
}
