---
title: 'eventx'
linkTitle: 'eventx'
description: 'In-Process Strongly Typed Event Bus'
weight: 4
---

## eventx

`eventx` is an in-process, strongly typed event bus for Go services.

## Install / Import

```bash
go get github.com/DaiYuANg/arcgo/eventx@latest
```

## Current capabilities

- Generic type subscription: `Subscribe[T Event]`
- Synchronous publishing: `Publish`
- Asynchronous publishing with queue/workers: `PublishAsync`
- Optional parallel dispatch for handlers of the same event type
- Middleware pipeline (global + per-subscriber)
- Graceful shutdown and in-flight draining (`Close`)

## Package layout

- Core bus: `github.com/DaiYuANg/arcgo/eventx`
- FX module (optional): `github.com/DaiYuANg/arcgo/eventx/fx`

## Documentation map

- Release notes: [eventx v0.3.0](./release-v0.3.0)
- Minimal sync pub/sub: [Getting Started](./getting-started)
- Async + middleware: [Async and middleware](./async-and-middleware)
- Errors, Close semantics, ordering notes: [Errors and lifecycle](./errors-and-lifecycle)

## Event contract

```go
type Event interface {
    Name() string
}
```

Routing is based on the event’s concrete Go type. `Name()` is semantic metadata only.

## Key API surface (summary)

- `eventx.New(opts...)`
- `eventx.Subscribe[T](bus, handler, subscriberOpts...)`
- `bus.Publish(ctx, event)`
- `bus.PublishAsync(ctx, event)`
- `bus.SubscriberCount()`
- `bus.Close()`

## Runnable examples (repository)

- [examples/eventx/basic](https://github.com/DaiYuANg/arcgo/tree/main/examples/eventx/basic)
- [examples/eventx/middleware](https://github.com/DaiYuANg/arcgo/tree/main/examples/eventx/middleware)
- [examples/eventx/observability](https://github.com/DaiYuANg/arcgo/tree/main/examples/eventx/observability)
- [examples/eventx/fx](https://github.com/DaiYuANg/arcgo/tree/main/examples/eventx/fx)

## Integration Guide

- With `dix`: build one bus per bounded context and manage lifecycle with app hooks.
- With `observabilityx`: attach observability middleware for event throughput, latency, and error metrics.
- With `logx`: emit structured event type and handler category logs around failure paths.
- With `httpx`: publish domain events from handlers after validation and service-layer commit points.

## Testing tips

- Use serial dispatch in unit tests for deterministic ordering.
- Call `defer bus.Close()` in each test to avoid worker leaks.
- Use explicit event types in tests to avoid accidental shared subscriptions.
## Production notes

- Define ownership boundaries up front; avoid one global bus for unrelated domains.
- Tune async dispatch capacity from real traffic, and define backpressure for critical events.
