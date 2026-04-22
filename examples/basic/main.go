// Package main demonstrates basic eventx publish and subscribe flows.
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/arcgolabs/eventx"
)

type orderCreatedEvent struct {
	OrderID   string
	UserID    string
	Amount    float64
	CreatedAt time.Time
}

func (e orderCreatedEvent) Name() string {
	return "order.created"
}

type orderPaidEvent struct {
	OrderID string
	PaidAt  time.Time
}

func (e orderPaidEvent) Name() string {
	return "order.paid"
}

func main() {
	bus := eventx.New(
		eventx.WithAntsPool(4),
		eventx.WithParallelDispatch(true),
	)
	defer func() {
		if err := bus.Close(); err != nil {
			panic(err)
		}
	}()

	_, err := eventx.Subscribe[orderCreatedEvent](bus, func(_ context.Context, event orderCreatedEvent) error {
		mustPrintf("send welcome email to %s (order: %s)\n", event.UserID, event.OrderID)
		time.Sleep(100 * time.Millisecond)
		return nil
	})
	if err != nil {
		panic(err)
	}

	_, err = eventx.Subscribe[orderCreatedEvent](bus, func(_ context.Context, event orderCreatedEvent) error {
		mustPrintf("init order analytics: %s, amount: %.2f\n", event.OrderID, event.Amount)
		return nil
	})
	if err != nil {
		panic(err)
	}

	_, err = eventx.Subscribe[orderPaidEvent](bus, func(_ context.Context, event orderPaidEvent) error {
		mustPrintf("update inventory for order: %s\n", event.OrderID)
		return nil
	})
	if err != nil {
		panic(err)
	}

	_, err = eventx.Subscribe[orderPaidEvent](bus, func(_ context.Context, event orderPaidEvent) error {
		mustPrintf("send payment confirmation for order: %s\n", event.OrderID)
		return nil
	})
	if err != nil {
		panic(err)
	}

	mustPrintln("=== publish order created event (sync) ===")
	err = bus.Publish(context.Background(), orderCreatedEvent{
		OrderID:   "ORD-001",
		UserID:    "USER-123",
		Amount:    299.99,
		CreatedAt: time.Now(),
	})
	if err != nil {
		mustPrintf("publish event failed: %v\n", err)
	}

	mustPrintln("\n=== publish order paid event (async) ===")
	err = bus.PublishAsync(context.Background(), orderPaidEvent{
		OrderID: "ORD-001",
		PaidAt:  time.Now(),
	})
	if err != nil {
		mustPrintf("publish async event failed: %v\n", err)
	}

	time.Sleep(500 * time.Millisecond)

	mustPrintln("\n=== subscriber count:", bus.SubscriberCount(), "===")
}

func mustPrintln(args ...any) {
	if _, err := fmt.Println(args...); err != nil {
		panic(err)
	}
}

func mustPrintf(format string, args ...any) {
	if _, err := fmt.Printf(format, args...); err != nil {
		panic(err)
	}
}
