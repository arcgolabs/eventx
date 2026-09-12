package eventx_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/arcgolabs/eventx"
	"github.com/stretchr/testify/require"
)

type otherEvent struct{}

func (otherEvent) Name() string {
	return "other"
}

type pointerEvent struct {
	ID int
}

func (*pointerEvent) Name() string {
	return "pointer"
}

func TestPublishLazySkipsFactoryWithoutMatchingSubscribers(t *testing.T) {
	t.Parallel()

	bus := eventx.New()
	t.Cleanup(func() { require.NoError(t, bus.Close()) })

	_, err := bus.Subscribe(func(_ context.Context, _ otherEvent) error { return nil })
	require.NoError(t, err)

	var factoryCalls atomic.Int64
	err = bus.PublishLazy(t.Context(), func() userCreated {
		factoryCalls.Add(1)
		return userCreated{ID: 1}
	})

	require.NoError(t, err)
	require.Zero(t, factoryCalls.Load())
}

func TestPublishLazyDispatchesFactoryResultThroughMiddleware(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("handler failed")
	var middlewareCalls atomic.Int64
	bus := eventx.New(eventx.WithMiddleware(func(next eventx.HandlerFunc) eventx.HandlerFunc {
		return func(ctx context.Context, event eventx.Event) error {
			middlewareCalls.Add(1)
			return next(ctx, event)
		}
	}))
	t.Cleanup(func() { require.NoError(t, bus.Close()) })

	var handledID atomic.Int64
	_, err := bus.Subscribe(func(ctx context.Context, event userCreated) error {
		require.NotNil(t, ctx)
		handledID.Store(int64(event.ID))
		return wantErr
	})
	require.NoError(t, err)

	var factoryCalls atomic.Int64
	err = bus.PublishLazy(nilContext(), func() userCreated {
		factoryCalls.Add(1)
		return userCreated{ID: 42}
	})

	require.ErrorIs(t, err, wantErr)
	require.EqualValues(t, 1, factoryCalls.Load())
	require.EqualValues(t, 1, middlewareCalls.Load())
	require.EqualValues(t, 42, handledID.Load())
}

func TestPublishLazyRejectsNilFactory(t *testing.T) {
	t.Parallel()

	bus := eventx.New()
	t.Cleanup(func() { require.NoError(t, bus.Close()) })

	var factory func() userCreated
	err := bus.PublishLazy(t.Context(), factory)

	require.ErrorIs(t, err, eventx.ErrNilEventFactory)
}

func TestPublishLazyRejectsTypedNilEvent(t *testing.T) {
	t.Parallel()

	bus := eventx.New()
	t.Cleanup(func() { require.NoError(t, bus.Close()) })

	var handled atomic.Bool
	_, err := bus.Subscribe(func(_ context.Context, _ *pointerEvent) error {
		handled.Store(true)
		return nil
	})
	require.NoError(t, err)

	err = bus.PublishLazy(t.Context(), func() *pointerEvent { return nil })
	require.ErrorIs(t, err, eventx.ErrNilEvent)
	require.False(t, handled.Load())
}

func TestPublishLazyClosedBusDoesNotCallFactory(t *testing.T) {
	t.Parallel()

	bus := eventx.New()
	require.NoError(t, bus.Close())

	var factoryCalls atomic.Int64
	err := bus.PublishLazy(t.Context(), func() userCreated {
		factoryCalls.Add(1)
		return userCreated{ID: 1}
	})

	require.ErrorIs(t, err, eventx.ErrBusClosed)
	require.Zero(t, factoryCalls.Load())
}

func TestPublishLazyUsesHandlerSnapshotSelectedBeforeFactory(t *testing.T) {
	t.Parallel()

	bus := eventx.New()
	t.Cleanup(func() { require.NoError(t, bus.Close()) })

	var handled atomic.Bool
	unsubscribe, err := bus.Subscribe(func(_ context.Context, _ userCreated) error {
		handled.Store(true)
		return nil
	})
	require.NoError(t, err)

	factoryStarted := make(chan struct{})
	releaseFactory := make(chan struct{})
	publishDone := make(chan error, 1)
	go func() {
		publishDone <- bus.PublishLazy(t.Context(), func() userCreated {
			close(factoryStarted)
			<-releaseFactory
			return userCreated{ID: 1}
		})
	}()

	<-factoryStarted
	unsubscribe()
	close(releaseFactory)

	require.NoError(t, <-publishDone)
	require.True(t, handled.Load())
}

func TestHasSubscribersTracksTypedSubscriptions(t *testing.T) {
	t.Parallel()

	bus := eventx.New()
	t.Cleanup(func() { require.NoError(t, bus.Close()) })

	require.False(t, bus.HasSubscribers[userCreated]())
	require.False(t, bus.HasSubscribers[otherEvent]())

	unsubscribe, err := bus.Subscribe(func(_ context.Context, _ userCreated) error { return nil })
	require.NoError(t, err)
	require.True(t, bus.HasSubscribers[userCreated]())
	require.False(t, bus.HasSubscribers[otherEvent]())

	unsubscribe()
	require.False(t, bus.HasSubscribers[userCreated]())
}
