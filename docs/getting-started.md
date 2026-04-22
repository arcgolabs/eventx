---
title: 'eventx Getting Started'
linkTitle: 'getting-started'
description: 'Define a typed event, subscribe, and publish synchronously'
weight: 2
---

## Getting Started

`eventx` is an in-process, strongly typed event bus. Subscriptions are routed by the event’s **concrete Go type**. `Event.Name()` is semantic metadata for logs/metrics.

This page shows a minimal program with:

- one typed event
- one subscriber
- one synchronous publish (`Publish`)

## 1) Install

```bash
go get github.com/DaiYuANg/arcgo/eventx@latest
```

## 2) Create `main.go`

```go
package main

import (
	"context"
	"fmt"

	"github.com/DaiYuANg/arcgo/eventx"
)

type UserCreated struct {
	ID string
}

func (UserCreated) Name() string { return "user.created" }

func main() {
	bus := eventx.New()
	defer func() { _ = bus.Close() }()

	unsub, err := eventx.Subscribe[UserCreated](bus, func(ctx context.Context, evt UserCreated) error {
		_ = ctx
		fmt.Println("user created:", evt.ID)
		return nil
	})
	if err != nil {
		panic(err)
	}
	defer unsub()

	if err := bus.Publish(context.Background(), UserCreated{ID: "u-1"}); err != nil {
		panic(err)
	}
}
```

## 3) Run

```bash
go mod init example.com/eventx-hello
go get github.com/DaiYuANg/arcgo/eventx@latest
go run .
```

## Next

- Async publishing and backpressure: [Async and middleware](./async-and-middleware)
- Error model, shutdown, and ordering guarantees: [Errors and lifecycle](./errors-and-lifecycle)

