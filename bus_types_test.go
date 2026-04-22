// Package eventx_test tests eventx through its public API.
package eventx_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/arcgolabs/eventx"
	"github.com/stretchr/testify/require"
)

type userCreated struct {
	ID int
}

func nilContext() context.Context {
	return nil
}

func (e userCreated) Name() string {
	return "user.created"
}

func newTestBus(tb testing.TB, opts ...eventx.Option) eventx.BusRuntime {
	tb.Helper()
	bus := eventx.New(opts...)
	tb.Cleanup(func() {
		closeBus(tb, bus)
	})
	return bus
}

func closeBus(tb testing.TB, bus eventx.BusRuntime) {
	tb.Helper()
	require.NoError(tb, bus.Close())
}

func publishConcurrently(tb testing.TB, total int, publish func(int) error) {
	tb.Helper()
	var wg sync.WaitGroup
	errCh := make(chan error, total)

	for i := range total {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			errCh <- publish(id)
		}(i)
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		require.NoError(tb, err)
	}
}

func waitForSignals(tb testing.TB, started <-chan struct{}, total int, timeout time.Duration, message string) {
	tb.Helper()
	for range total {
		select {
		case <-started:
		case <-time.After(timeout):
			tb.Fatal(message)
		}
	}
}

func updateMax(maxValue *atomic.Int64, current int64) {
	for {
		seen := maxValue.Load()
		if current <= seen || maxValue.CompareAndSwap(seen, current) {
			return
		}
	}
}
