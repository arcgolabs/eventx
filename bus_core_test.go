package eventx_test

import (
	"context"
	"errors"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/arcgolabs/eventx"
	"github.com/stretchr/testify/require"
)

type orderCreated struct {
	ID int
}

func (e orderCreated) Name() string {
	return "order.created"
}

func TestPublishSync(t *testing.T) {
	t.Parallel()

	bus := newTestBus(t)
	var got atomic.Int64

	unsubscribe, err := eventx.Subscribe(bus, func(_ context.Context, evt userCreated) error {
		got.Add(int64(evt.ID))
		return nil
	})
	require.NoError(t, err)
	defer unsubscribe()

	err = bus.Publish(context.Background(), userCreated{ID: 7})
	require.NoError(t, err)
	require.EqualValues(t, 7, got.Load())
}

func TestPublishNilEvent(t *testing.T) {
	t.Parallel()

	bus := newTestBus(t)

	var evt eventx.Event
	err := bus.Publish(context.Background(), evt)
	require.ErrorIs(t, err, eventx.ErrNilEvent)
}

func TestSubscribeNilHandler(t *testing.T) {
	t.Parallel()

	bus := newTestBus(t)

	_, err := eventx.Subscribe[userCreated](bus, nil)
	require.ErrorIs(t, err, eventx.ErrNilHandler)
}

func TestNilBus(t *testing.T) {
	t.Parallel()

	var nilBus *eventx.Bus

	_, err := eventx.Subscribe(nilBus, func(_ context.Context, _ userCreated) error { return nil })
	require.ErrorIs(t, err, eventx.ErrNilBus)

	err = nilBus.Publish(context.Background(), userCreated{ID: 1})
	require.ErrorIs(t, err, eventx.ErrNilBus)

	err = nilBus.PublishAsync(context.Background(), userCreated{ID: 1})
	require.ErrorIs(t, err, eventx.ErrNilBus)
}

func TestPublishNilContext(t *testing.T) {
	t.Parallel()

	bus := newTestBus(t)

	_, err := eventx.Subscribe(bus, func(ctx context.Context, _ userCreated) error {
		if ctx == nil {
			return errors.New("nil context")
		}
		return nil
	})
	require.NoError(t, err)

	err = bus.Publish(nilContext(), userCreated{ID: 1})
	require.NoError(t, err)
}

func TestUnsubscribe(t *testing.T) {
	t.Parallel()

	bus := newTestBus(t)
	var count atomic.Int64

	unsubscribe, err := eventx.Subscribe(bus, func(_ context.Context, _ userCreated) error {
		count.Add(1)
		return nil
	})
	require.NoError(t, err)

	require.NoError(t, bus.Publish(context.Background(), userCreated{ID: 1}))
	unsubscribe()
	require.NoError(t, bus.Publish(context.Background(), userCreated{ID: 1}))

	require.EqualValues(t, 1, count.Load())
}

func TestCloseRejectsNewRequests(t *testing.T) {
	t.Parallel()

	bus := newTestBus(t, eventx.WithAntsPool(1))
	closeBus(t, bus)

	err := bus.Publish(context.Background(), userCreated{ID: 1})
	require.ErrorIs(t, err, eventx.ErrBusClosed)

	err = bus.PublishAsync(context.Background(), userCreated{ID: 1})
	require.ErrorIs(t, err, eventx.ErrBusClosed)

	_, err = eventx.Subscribe(bus, func(_ context.Context, _ userCreated) error { return nil })
	require.ErrorIs(t, err, eventx.ErrBusClosed)
}

func TestSubscriberCount(t *testing.T) {
	t.Parallel()

	bus := newTestBus(t)

	unsub1, err := eventx.Subscribe(bus, func(_ context.Context, _ userCreated) error { return nil })
	require.NoError(t, err)
	unsub2, err := eventx.Subscribe(bus, func(_ context.Context, _ userCreated) error { return nil })
	require.NoError(t, err)

	require.Equal(t, 2, bus.SubscriberCount())
	unsub1()
	require.Equal(t, 1, bus.SubscriberCount())
	unsub2()
	require.Equal(t, 0, bus.SubscriberCount())
}

func TestGetHandlersGroupedByEventType(t *testing.T) {
	t.Parallel()

	bus := newTestBus(t)

	_, err := eventx.Subscribe(bus, func(_ context.Context, _ userCreated) error { return nil })
	require.NoError(t, err)
	unsubscribe, err := eventx.Subscribe(bus, func(_ context.Context, _ userCreated) error { return nil })
	require.NoError(t, err)
	_, err = eventx.Subscribe(bus, func(_ context.Context, _ orderCreated) error { return nil })
	require.NoError(t, err)

	grouped := bus.GetHandlersGroupedByEventType()
	require.Equal(t, 2, grouped.Len())
	require.Equal(t, 3, grouped.ValueCount())
	require.Len(t, grouped.Get(reflect.TypeFor[userCreated]()), 2)
	require.Len(t, grouped.Get(reflect.TypeFor[orderCreated]()), 1)

	grouped.Delete(reflect.TypeFor[userCreated]())
	refreshed := bus.GetHandlersGroupedByEventType()
	require.Len(t, refreshed.Get(reflect.TypeFor[userCreated]()), 2)

	unsubscribe()
	refreshed = bus.GetHandlersGroupedByEventType()
	require.Len(t, refreshed.Get(reflect.TypeFor[userCreated]()), 1)
}

func TestSubscribeOnce(t *testing.T) {
	t.Parallel()

	bus := newTestBus(t)
	var count atomic.Int64

	_, err := eventx.SubscribeOnce(bus, func(_ context.Context, _ userCreated) error {
		count.Add(1)
		return nil
	})
	require.NoError(t, err)

	require.NoError(t, bus.Publish(context.Background(), userCreated{ID: 1}))
	require.NoError(t, bus.Publish(context.Background(), userCreated{ID: 2}))
	require.EqualValues(t, 1, count.Load())
}

func TestSubscribeN(t *testing.T) {
	t.Parallel()

	bus := newTestBus(t)
	var count atomic.Int64

	_, err := eventx.SubscribeN(bus, 2, func(_ context.Context, _ userCreated) error {
		count.Add(1)
		return nil
	})
	require.NoError(t, err)

	require.NoError(t, bus.Publish(context.Background(), userCreated{ID: 1}))
	require.NoError(t, bus.Publish(context.Background(), userCreated{ID: 2}))
	require.NoError(t, bus.Publish(context.Background(), userCreated{ID: 3}))
	require.EqualValues(t, 2, count.Load())
}

func TestSubscribeNInvalidCount(t *testing.T) {
	t.Parallel()

	bus := newTestBus(t)

	_, err := eventx.SubscribeN(bus, 0, func(_ context.Context, _ userCreated) error { return nil })
	require.ErrorIs(t, err, eventx.ErrInvalidSubscribeCount)
}

func TestUnsubscribeInvalidatesHandlerSnapshot(t *testing.T) {
	t.Parallel()

	bus := newTestBus(t)
	var first atomic.Int64
	var second atomic.Int64

	unsub1, err := eventx.Subscribe(bus, func(_ context.Context, _ userCreated) error {
		first.Add(1)
		return nil
	})
	require.NoError(t, err)
	_, err = eventx.Subscribe(bus, func(_ context.Context, _ userCreated) error {
		second.Add(1)
		return nil
	})
	require.NoError(t, err)

	require.NoError(t, bus.Publish(context.Background(), userCreated{ID: 1}))
	require.EqualValues(t, 1, first.Load())
	require.EqualValues(t, 1, second.Load())

	unsub1()

	require.NoError(t, bus.Publish(context.Background(), userCreated{ID: 1}))
	require.EqualValues(t, 1, first.Load())
	require.EqualValues(t, 2, second.Load())
}
