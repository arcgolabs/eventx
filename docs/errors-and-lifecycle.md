---
title: 'eventx Errors and Lifecycle'
linkTitle: 'errors-lifecycle'
description: 'Error aggregation, Close semantics, and ordering notes'
weight: 4
---

## Errors and lifecycle

Key behavior:

- `Publish` executes handlers and returns **aggregated** handler errors (via `errors.Join` semantics).
- `RecoverMiddleware` can convert panics into errors on the error path.
- `Close` stops new publishes, drains async work, and waits for in-flight dispatches. Calling `Close` multiple times is safe.
- Serial dispatch (default) is deterministic for a snapshot of subscriptions; parallel dispatch does not guarantee ordering between handlers.

## Typed errors you may check

- `eventx.ErrBusClosed`
- `eventx.ErrNilEvent`
- `eventx.ErrNilBus`
- `eventx.ErrNilHandler`
- `eventx.ErrAsyncQueueFull`

## Minimal example: Close + subscriber count

```go
package main

import (
	"context"
	"fmt"

	"github.com/arcgolabs/eventx"
)

type Ping struct{}

func (Ping) Name() string { return "ping" }

func main() {
	bus := eventx.New()

	unsub, err := eventx.Subscribe[Ping](bus, func(ctx context.Context, evt Ping) error {
		_ = ctx
		_ = evt
		return nil
	})
	if err != nil {
		panic(err)
	}

	fmt.Println("subs:", bus.SubscriberCount())
	unsub()
	fmt.Println("subs:", bus.SubscriberCount())

	_ = bus.Publish(context.Background(), Ping{})
	_ = bus.Close()
}
```

## Related

- [Getting Started](./getting-started)
- [Async and middleware](./async-and-middleware)
