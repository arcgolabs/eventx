// Package main demonstrates eventx async throughput with multiple consumers.
package main

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/arcgolabs/eventx"
)

type stockEvent struct {
	ProductID string
	Change    int
	Reason    string
}

func (e stockEvent) Name() string {
	return "stock.change"
}

func main() {
	mustPrintln("=== EventX benchmark example ===")

	var (
		totalProcessed atomic.Int64
		totalErrors    atomic.Int64
	)

	bus := newBenchmarkBus(&totalErrors)
	defer func() {
		if err := bus.Close(); err != nil {
			panic(err)
		}
	}()

	if err := registerConsumers(bus, &totalProcessed); err != nil {
		panic(err)
	}

	mustPrintln("start publishing stock events...")
	batchSize := 100
	elapsed := publishStockEvents(bus, batchSize)
	printSummary(elapsed, batchSize, totalProcessed.Load(), totalErrors.Load(), bus.SubscriberCount())
}

func newBenchmarkBus(totalErrors *atomic.Int64) eventx.BusRuntime {
	return eventx.New(
		eventx.WithAntsPool(10),
		eventx.WithParallelDispatch(true),
		eventx.WithMiddleware(eventx.RecoverMiddleware()),
		eventx.WithAsyncErrorHandler(func(_ context.Context, event eventx.Event, err error) {
			totalErrors.Add(1)
			mustPrintf("async handler error [%s]: %v\n", event.Name(), err)
		}),
	)
}

func registerConsumers(bus eventx.BusRuntime, totalProcessed *atomic.Int64) error {
	for consumerID := range 3 {
		if err := subscribeConsumer(bus, totalProcessed, consumerID); err != nil {
			return err
		}
	}
	return nil
}

func subscribeConsumer(bus eventx.BusRuntime, totalProcessed *atomic.Int64, consumerID int) error {
	_, err := eventx.Subscribe[stockEvent](bus, func(_ context.Context, event stockEvent) error {
		totalProcessed.Add(1)

		time.Sleep(10 * time.Millisecond)

		if event.Change < -100 {
			return fmt.Errorf("stock change too large: %d", event.Change)
		}

		if consumerID == 0 {
			mustPrintf(
				"consumer%d: product %s stock %+d (%s)\n",
				consumerID,
				event.ProductID,
				event.Change,
				event.Reason,
			)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("subscribe consumer %d: %w", consumerID, err)
	}
	return nil
}

func publishStockEvents(bus eventx.BusRuntime, batchSize int) time.Duration {
	startTime := time.Now()

	for i := range batchSize {
		event := stockEvent{
			ProductID: fmt.Sprintf("PROD-%03d", i%10),
			Change:    -1,
			Reason:    "订单扣减",
		}

		if err := bus.PublishAsync(context.Background(), event); err != nil {
			mustPrintf("publish failed: %v\n", err)
		}
	}

	time.Sleep(2 * time.Second)
	return time.Since(startTime)
}

func printSummary(elapsed time.Duration, batchSize int, processed, totalErrors int64, subscriberCount int) {
	mustPrintf("\n=== done ===\n")
	mustPrintf("elapsed: %v\n", elapsed)
	mustPrintf("processed events: %d\n", processed)
	mustPrintf("errors: %d\n", totalErrors)
	mustPrintf("throughput: %.0f events/sec\n", float64(batchSize)/elapsed.Seconds())
	mustPrintf("subscriber count: %d\n", subscriberCount)
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
