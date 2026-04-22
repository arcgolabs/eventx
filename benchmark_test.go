package eventx_test

import (
	"context"
	"testing"

	"github.com/arcgolabs/eventx"
)

type benchmarkEvent struct {
	ID int
}

func (e benchmarkEvent) Name() string {
	return "benchmark.event"
}

func benchmarkBusWithSubscribers(b *testing.B, parallelDispatch bool, subscribers int) eventx.BusRuntime {
	b.Helper()

	bus := eventx.New(eventx.WithParallelDispatch(parallelDispatch))
	for range subscribers {
		_, err := eventx.Subscribe(bus, func(_ context.Context, _ benchmarkEvent) error {
			return nil
		})
		if err != nil {
			b.Fatalf("subscribe failed: %v", err)
		}
	}

	b.Cleanup(func() {
		if err := bus.Close(); err != nil {
			b.Errorf("close bus: %v", err)
		}
	})
	return bus
}

func BenchmarkBusPublishSerial(b *testing.B) {
	bus := benchmarkBusWithSubscribers(b, false, 1)
	ctx := context.Background()
	evt := benchmarkEvent{ID: 1}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		if err := bus.Publish(ctx, evt); err != nil {
			b.Fatalf("publish failed: %v", err)
		}
	}
}

func BenchmarkBusPublishParallelDispatch(b *testing.B) {
	bus := benchmarkBusWithSubscribers(b, true, 4)
	ctx := context.Background()
	evt := benchmarkEvent{ID: 1}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		if err := bus.Publish(ctx, evt); err != nil {
			b.Fatalf("publish failed: %v", err)
		}
	}
}

func BenchmarkBusPublishConcurrentPublishers(b *testing.B) {
	bus := benchmarkBusWithSubscribers(b, false, 2)
	evt := benchmarkEvent{ID: 1}

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		ctx := context.Background()
		for pb.Next() {
			if err := bus.Publish(ctx, evt); err != nil {
				b.Fatalf("publish failed: %v", err)
			}
		}
	})
}
