package eventx

import (
	"context"

	"github.com/arcgolabs/observabilityx"
	"github.com/samber/lo"
)

const (
	defaultAsyncWorkers = 4
)

type asyncErrorHandler func(ctx context.Context, event Event, err error)

// Options configures bus construction.
type Options struct {
	antsPoolSize         int
	antsMaxBlockingCalls int

	parallel      bool
	middleware    []Middleware
	onAsyncError  asyncErrorHandler
	observability observabilityx.Observability
}

type options = Options

func defaultOptions() Options {
	return Options{
		antsPoolSize:         defaultAsyncWorkers,
		antsMaxBlockingCalls: -1, // -1 means infinite blocking calls

		parallel:      false,
		middleware:    nil,
		onAsyncError:  nil,
		observability: observabilityx.Nop(),
	}
}

// Option configures Bus.
type Option func(*Options)

// WithAntsPool enables ants goroutine pool with the given size.
// This is the recommended way for async event dispatch.
func WithAntsPool(size int) Option {
	return func(o *options) {
		o.antsPoolSize = size
	}
}

// WithAntsPoolWithMaxBlockingCalls configures ants pool with max blocking calls limit.
// maxBlockingCalls <= 0 means infinite.
func WithAntsPoolWithMaxBlockingCalls(size, maxBlockingCalls int) Option {
	return func(o *options) {
		o.antsPoolSize = size
		o.antsMaxBlockingCalls = maxBlockingCalls
	}
}

// WithParallelDispatch controls whether handlers of the same event are dispatched in parallel.
// Default is false (serial dispatch).
func WithParallelDispatch(enabled bool) Option {
	return func(o *options) {
		o.parallel = enabled
	}
}

// WithMiddleware appends global middleware.
func WithMiddleware(mw ...Middleware) Option {
	filtered := lo.Filter(mw, func(item Middleware, _ int) bool {
		return item != nil
	})
	return func(o *options) {
		o.middleware = lo.Concat(o.middleware, filtered)
	}
}

// WithAsyncErrorHandler sets callback for async dispatch errors.
func WithAsyncErrorHandler(handler func(ctx context.Context, event Event, err error)) Option {
	return func(o *options) {
		o.onAsyncError = handler
	}
}

// WithObservability sets optional observability integration for bus runtime.
func WithObservability(obs observabilityx.Observability) Option {
	return func(o *options) {
		o.observability = obs
	}
}

// SubscribeOptions configures per-subscription behavior.
type SubscribeOptions struct {
	middleware []Middleware
}

type subscribeOptions = SubscribeOptions

func defaultSubscribeOptions() SubscribeOptions {
	return SubscribeOptions{
		middleware: nil,
	}
}

// SubscribeOption configures per-subscription behavior.
type SubscribeOption func(*SubscribeOptions)

// WithSubscriberMiddleware appends subscription-level middleware.
func WithSubscriberMiddleware(mw ...Middleware) SubscribeOption {
	filtered := lo.Filter(mw, func(item Middleware, _ int) bool {
		return item != nil
	})
	return func(o *subscribeOptions) {
		o.middleware = lo.Concat(o.middleware, filtered)
	}
}
