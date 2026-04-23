---
title: 'eventx Async and Middleware'
linkTitle: 'async-middleware'
description: 'Async publishing plus global and per-subscriber middleware'
weight: 3
---

## Async and middleware

`eventx` supports synchronous `Publish` and asynchronous `PublishAsync`.

- Async dispatch is backed by an internal worker/queue implementation. The recommended option path is `WithAntsPool(...)`.
- Global middleware wraps per-subscriber middleware. Middleware order is preserved.

## 1) Install

```bash
go get github.com/arcgolabs/eventx@latest
```

## 2) Create `main.go`

This example:

- enables async dispatch
- observes async failures via `WithAsyncErrorHandler`
- adds global middleware
- adds per-subscriber middleware

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/arcgolabs/eventx"
)

type OrderPaid struct {
	OrderID string
}

func (OrderPaid) Name() string { return "order.paid" }

func main() {
	bus := eventx.New(
		eventx.WithAntsPool(4),
		eventx.WithParallelDispatch(true),
		eventx.WithAsyncErrorHandler(func(ctx context.Context, evt eventx.Event, err error) {
			_ = ctx
			fmt.Println("async error:", evt.Name(), err)
		}),
		eventx.WithMiddleware(
			eventx.RecoverMiddleware(),
			eventx.ObserveMiddleware(func(ctx context.Context, evt eventx.Event, d time.Duration, err error) {
				_ = ctx
				fmt.Println("handled:", evt.Name(), "dur", d, "err", err)
			}),
		),
	)
	defer func() { _ = bus.Close() }()

	_, err := eventx.Subscribe[OrderPaid](
		bus,
		func(ctx context.Context, evt OrderPaid) error {
			_ = ctx
			fmt.Println("inventory update:", evt.OrderID)
			return nil
		},
		eventx.WithSubscriberMiddleware(eventx.RecoverMiddleware()),
	)
	if err != nil {
		panic(err)
	}

	err = bus.PublishAsync(context.Background(), OrderPaid{OrderID: "ORD-001"})
	if errors.Is(err, eventx.ErrAsyncQueueFull) {
		// apply backpressure/retry or fall back to Publish for critical events
		fmt.Println("queue full")
	}
	if err != nil {
		panic(err)
	}

	time.Sleep(100 * time.Millisecond)
}
```

## Related

- [Getting Started](./getting-started)
- [Errors and lifecycle](./errors-and-lifecycle)
- Runnable examples in repo: [examples](https://github.com/arcgolabs/eventx/tree/main/examples)
