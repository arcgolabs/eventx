// Package main demonstrates eventx observability integration.
package main

import (
	"context"
	"fmt"

	"github.com/DaiYuANg/arcgo/httpx"
	"github.com/DaiYuANg/arcgo/httpx/adapter"
	"github.com/DaiYuANg/arcgo/httpx/adapter/std"
	"github.com/DaiYuANg/arcgo/observabilityx"
	otelobs "github.com/DaiYuANg/arcgo/observabilityx/otel"
	promobs "github.com/DaiYuANg/arcgo/observabilityx/prometheus"
	"github.com/arcgolabs/eventx"
)

type userCreated struct {
	ID int
}

func (e userCreated) Name() string {
	return "user.created"
}

func main() {
	prom := promobs.New(promobs.WithNamespace("eventx_example"))
	obs := observabilityx.Multi(otelobs.New(), prom)

	bus := eventx.New(
		eventx.WithObservability(obs),
		eventx.WithAntsPool(2),
		eventx.WithMiddleware(eventx.RecoverMiddleware()),
	)
	defer func() {
		if err := bus.Close(); err != nil {
			panic(err)
		}
	}()

	unsubscribe, err := eventx.Subscribe(bus, func(_ context.Context, evt userCreated) error {
		mustPrintln("user created:", evt.ID)
		return nil
	})
	if err != nil {
		panic(err)
	}
	defer unsubscribe()

	if err := bus.Publish(context.Background(), userCreated{ID: 1}); err != nil {
		panic(err)
	}
	if err := bus.PublishAsync(context.Background(), userCreated{ID: 2}); err != nil {
		panic(err)
	}

	stdAdapter := std.New(nil, adapter.HumaOptions{DisableDocsRoutes: true})
	metricsServer := httpx.New(
		httpx.WithAdapter(stdAdapter),
	)
	stdAdapter.Router().Handle("/metrics", prom.Handler())

	mustPrintln("httpx metrics route registered: GET /metrics")
	_ = metricsServer
}

func mustPrintln(args ...any) {
	if _, err := fmt.Println(args...); err != nil {
		panic(err)
	}
}
