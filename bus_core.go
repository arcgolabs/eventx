package eventx

import (
	"log/slog"
	"reflect"
	"sync"
	"sync/atomic"

	collectionlist "github.com/arcgolabs/collectionx/list"
	collectionmapping "github.com/arcgolabs/collectionx/mapping"
	"github.com/arcgolabs/observabilityx"
	"github.com/arcgolabs/pkg/option"
	"github.com/panjf2000/ants/v2"
)

// Bus is an in-memory strongly typed event bus.
type Bus struct {
	lifecycleMu    sync.Mutex
	closed         atomic.Bool
	subscriptionMu sync.Mutex
	nextID         uint64
	subsByType     subscriptionTable
	handlerCache   *collectionmapping.ConcurrentMap[reflect.Type, []HandlerFunc]
	parallel       bool
	parallelLimit  chan struct{}
	middleware     []Middleware
	onAsyncErr     asyncErrorHandler
	antsPool       *ants.Pool
	initErr        error
	dispatchWG     sync.WaitGroup
	observability  observabilityx.Observability
	logger         *slog.Logger
}

const (
	metricDispatchTotal           = "eventx_dispatch_total"
	metricDispatchDurationMS      = "eventx_dispatch_duration_ms"
	metricAsyncEnqueueTotal       = "eventx_async_enqueue_total"
	metricAsyncEnqueueDurationMS  = "eventx_async_enqueue_duration_ms"
	metricAsyncDispatchErrorTotal = "eventx_async_dispatch_error_total"
)

var (
	dispatchTotalSpec = observabilityx.NewCounterSpec(
		metricDispatchTotal,
		observabilityx.WithDescription("Total number of event dispatch operations."),
		observabilityx.WithLabelKeys("mode", "result", "event_name"),
	)
	dispatchDurationSpec = observabilityx.NewHistogramSpec(
		metricDispatchDurationMS,
		observabilityx.WithDescription("Duration of event dispatch operations in milliseconds."),
		observabilityx.WithUnit("ms"),
		observabilityx.WithLabelKeys("mode", "result", "event_name"),
	)
	asyncEnqueueTotalSpec = observabilityx.NewCounterSpec(
		metricAsyncEnqueueTotal,
		observabilityx.WithDescription("Total number of async enqueue attempts."),
		observabilityx.WithLabelKeys("result", "event_name"),
	)
	asyncEnqueueDurationSpec = observabilityx.NewHistogramSpec(
		metricAsyncEnqueueDurationMS,
		observabilityx.WithDescription("Duration of async enqueue attempts in milliseconds."),
		observabilityx.WithUnit("ms"),
		observabilityx.WithLabelKeys("result", "event_name"),
	)
	asyncDispatchErrorTotalSpec = observabilityx.NewCounterSpec(
		metricAsyncDispatchErrorTotal,
		observabilityx.WithDescription("Total number of async dispatch failures."),
		observabilityx.WithLabelKeys("event_name"),
	)
)

// New creates a new Bus runtime.
func New(opts ...Option) *Bus {
	cfg := defaultOptions()
	option.Apply(&cfg, opts...)

	b := &Bus{
		subsByType:    collectionmapping.NewConcurrentTable[reflect.Type, uint64, *subscription](),
		handlerCache:  collectionmapping.NewConcurrentMap[reflect.Type, []HandlerFunc](),
		parallel:      cfg.parallel,
		parallelLimit: newParallelLimiter(cfg.antsPoolSize),
		middleware:    cfg.middleware,
		onAsyncErr:    cfg.onAsyncError,
		observability: observabilityx.Normalize(cfg.observability, nil),
	}
	b.logger = b.observability.Logger().With("component", "eventx.bus")

	poolOpts := collectionlist.NewListWithCapacity[ants.Option](3,
		ants.WithPreAlloc(true),
		ants.WithNonblocking(false),
	)
	if cfg.antsMaxBlockingCalls > 0 {
		poolOpts.Add(ants.WithMaxBlockingTasks(cfg.antsMaxBlockingCalls))
	}

	pool, err := ants.NewPool(cfg.antsPoolSize, poolOpts.Values()...)
	if err != nil {
		b.initErr = err
		b.logger.Error("failed to create ants pool", "error", err)
	} else {
		b.antsPool = pool
	}

	b.logger.Debug("bus initialized",
		"parallel", b.parallel,
		"ants_pool_size", cfg.antsPoolSize,
		"ants_max_blocking_calls", cfg.antsMaxBlockingCalls,
		"middleware_count", len(cfg.middleware),
		"async_runtime_ready", b.antsPool != nil,
	)

	return b
}

// Close stops accepting new events, drains async queue, and waits in-flight handlers.
func (b *Bus) Close() error {
	if b == nil {
		return nil
	}

	var pool *ants.Pool
	b.lifecycleMu.Lock()
	if b.closed.Load() {
		b.lifecycleMu.Unlock()
		return nil
	}
	b.closed.Store(true)
	pool = b.antsPool
	b.lifecycleMu.Unlock()

	b.logger.Debug("closing bus",
		"subscriber_count", b.subsByType.Len(),
	)

	if pool != nil {
		pool.Release()
	}
	b.dispatchWG.Wait()
	b.logger.Debug("bus closed")
	return nil
}

// SubscriberCount returns active subscriber count.
func (b *Bus) SubscriberCount() int {
	if b == nil {
		return 0
	}
	return b.subsByType.Len()
}

func (b *Bus) beginDispatch() bool {
	if b == nil {
		return false
	}

	b.lifecycleMu.Lock()
	defer b.lifecycleMu.Unlock()
	if b.closed.Load() {
		return false
	}
	b.dispatchWG.Add(1)
	return true
}

func (b *Bus) registerSubscription(eventType reflect.Type, buildHandler func(id uint64) HandlerFunc) (uint64, error) {
	if b == nil {
		return 0, ErrNilBus
	}

	b.lifecycleMu.Lock()
	defer b.lifecycleMu.Unlock()
	if b.closed.Load() {
		return 0, ErrBusClosed
	}
	b.nextID++
	id := b.nextID
	var handler HandlerFunc
	if buildHandler != nil {
		handler = buildHandler(id)
	}

	b.subscriptionMu.Lock()
	defer b.subscriptionMu.Unlock()
	b.subsByType.Put(eventType, id, &subscription{
		id:      id,
		handler: handler,
	})
	b.rebuildHandlerSnapshotLocked(eventType)
	b.logger.Debug("subscription registered",
		"event_type", eventType.String(),
		"subscription_id", id,
	)
	return id, nil
}

func (b *Bus) rebuildHandlerSnapshotLocked(eventType reflect.Type) {
	if b == nil || b.handlerCache == nil {
		return
	}

	row := b.subsByType.Row(eventType)
	snapshot := make([]HandlerFunc, 0, len(row))
	for _, sub := range row {
		if sub == nil || sub.handler == nil {
			continue
		}
		snapshot = append(snapshot, sub.handler)
	}
	b.handlerCache.Set(eventType, snapshot)
	b.logger.Debug("handler snapshot rebuilt",
		"event_type", eventType.String(),
		"handler_count", len(snapshot),
	)
}

func (b *Bus) deleteSubscription(eventType reflect.Type, id uint64) {
	if b == nil {
		return
	}
	b.subscriptionMu.Lock()
	defer b.subscriptionMu.Unlock()
	if b.subsByType.Delete(eventType, id) {
		b.rebuildHandlerSnapshotLocked(eventType)
		b.logger.Debug("subscription removed",
			"event_type", eventType.String(),
			"subscription_id", id,
		)
	}
}

func newParallelLimiter(size int) chan struct{} {
	if size <= 0 {
		return nil
	}
	return make(chan struct{}, size)
}
