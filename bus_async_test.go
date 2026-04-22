package eventx_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/arcgolabs/eventx"
	"github.com/stretchr/testify/require"
)

func TestPublishAsyncNilContext(t *testing.T) {
	t.Parallel()

	bus := newTestBus(t, eventx.WithAntsPool(1))
	nilCtx := make(chan bool, 1)

	_, err := eventx.Subscribe(bus, func(ctx context.Context, _ userCreated) error {
		nilCtx <- ctx == nil
		return nil
	})
	require.NoError(t, err)

	require.NoError(t, bus.PublishAsync(nilContext(), userCreated{ID: 1}))
	closeBus(t, bus)

	select {
	case gotNil := <-nilCtx:
		require.False(t, gotNil)
	case <-time.After(time.Second):
		t.Fatal("async handler did not run in time")
	}
}

func TestPublishAsyncAndCloseDrain(t *testing.T) {
	t.Parallel()

	bus := newTestBus(t, eventx.WithAntsPool(2))
	var count atomic.Int64

	_, err := eventx.Subscribe(bus, func(_ context.Context, _ userCreated) error {
		count.Add(1)
		return nil
	})
	require.NoError(t, err)

	for i := range 10 {
		require.NoError(t, bus.PublishAsync(context.Background(), userCreated{ID: i}))
	}

	closeBus(t, bus)
	require.EqualValues(t, 10, count.Load())
}

func TestPublishAsyncWithDefaultAntsPool(t *testing.T) {
	t.Parallel()

	bus := newTestBus(t)
	var count atomic.Int64

	_, err := eventx.Subscribe(bus, func(_ context.Context, _ userCreated) error {
		count.Add(1)
		return nil
	})
	require.NoError(t, err)

	require.NoError(t, bus.PublishAsync(context.Background(), userCreated{ID: 1}))
	closeBus(t, bus)
	require.EqualValues(t, 1, count.Load())
}

func TestAsyncErrorHandler(t *testing.T) {
	t.Parallel()

	var got atomic.Int64
	bus := newTestBus(t,
		eventx.WithAntsPool(1),
		eventx.WithAsyncErrorHandler(func(_ context.Context, _ eventx.Event, err error) {
			if err != nil {
				got.Add(1)
			}
		}),
	)

	_, err := eventx.Subscribe(bus, func(_ context.Context, _ userCreated) error {
		return errors.New("boom")
	})
	require.NoError(t, err)

	require.NoError(t, bus.PublishAsync(context.Background(), userCreated{ID: 1}))
	closeBus(t, bus)
	require.EqualValues(t, 1, got.Load())
}

func TestAsyncCloseWhilePublishing(t *testing.T) {
	t.Parallel()

	bus := newTestBus(t, eventx.WithAntsPool(1))

	_, err := eventx.Subscribe(bus, func(_ context.Context, _ userCreated) error {
		time.Sleep(2 * time.Millisecond)
		return nil
	})
	require.NoError(t, err)

	errCh := make(chan error, 200)
	publishConcurrently(t, 200, func(id int) error {
		err := bus.PublishAsync(context.Background(), userCreated{ID: id})
		errCh <- err
		return nil
	})
	close(errCh)

	time.Sleep(5 * time.Millisecond)
	closeBus(t, bus)

	for err := range errCh {
		if err == nil || errors.Is(err, eventx.ErrBusClosed) {
			continue
		}
		require.NoError(t, err)
	}
}

func TestPublishAsyncUnavailableWhenAntsPoolInitFails(t *testing.T) {
	t.Parallel()

	bus := newTestBus(t, eventx.WithAntsPool(0))

	err := bus.PublishAsync(context.Background(), userCreated{ID: 1})
	require.ErrorIs(t, err, eventx.ErrAsyncRuntimeUnavailable)
}

func TestSubscribeOnceStrictUnderConcurrentPublish(t *testing.T) {
	t.Parallel()

	bus := newTestBus(t, eventx.WithAntsPool(4))
	var count atomic.Int64

	_, err := eventx.SubscribeOnce(bus, func(_ context.Context, _ userCreated) error {
		count.Add(1)
		time.Sleep(time.Millisecond)
		return nil
	})
	require.NoError(t, err)

	publishConcurrently(t, 16, func(id int) error {
		return bus.PublishAsync(context.Background(), userCreated{ID: id})
	})
	closeBus(t, bus)
	require.EqualValues(t, 1, count.Load())
}

func TestSubscribeNStrictUnderConcurrentPublish(t *testing.T) {
	t.Parallel()

	bus := newTestBus(t, eventx.WithAntsPool(4))
	var count atomic.Int64

	_, err := eventx.SubscribeN(bus, 2, func(_ context.Context, _ userCreated) error {
		count.Add(1)
		time.Sleep(time.Millisecond)
		return nil
	})
	require.NoError(t, err)

	publishConcurrently(t, 16, func(id int) error {
		return bus.PublishAsync(context.Background(), userCreated{ID: id})
	})
	closeBus(t, bus)
	require.EqualValues(t, 2, count.Load())
}

func TestParallelDispatchUsesGlobalLimiter(t *testing.T) {
	t.Parallel()

	bus := newTestBus(t, eventx.WithAntsPool(2), eventx.WithParallelDispatch(true))
	var active atomic.Int64
	var maxActive atomic.Int64
	started := make(chan struct{}, 4)
	release := make(chan struct{})

	for range 4 {
		_, err := eventx.Subscribe(bus, func(_ context.Context, _ userCreated) error {
			current := active.Add(1)
			started <- struct{}{}
			updateMax(&maxActive, current)
			<-release
			active.Add(-1)
			return nil
		})
		require.NoError(t, err)
	}

	require.NoError(t, bus.PublishAsync(context.Background(), userCreated{ID: 1}))
	waitForSignals(t, started, 2, time.Second, "handler did not start in time")

	select {
	case <-started:
		t.Fatal("parallel dispatch exceeded limiter before release")
	case <-time.After(20 * time.Millisecond):
	}

	close(release)
	closeBus(t, bus)
	require.LessOrEqual(t, maxActive.Load(), int64(2))
}
