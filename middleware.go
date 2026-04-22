package eventx

import (
	"context"
	"reflect"
	"time"

	"github.com/samber/lo"
	"github.com/samber/oops"
)

// HandlerFunc is the runtime event handler signature after type adaptation.
type HandlerFunc func(context.Context, Event) error

// Middleware wraps HandlerFunc.
type Middleware func(HandlerFunc) HandlerFunc

func chain(handler HandlerFunc, mws []Middleware) HandlerFunc {
	return lo.ReduceRight(mws, func(out HandlerFunc, mw Middleware, _ int) HandlerFunc {
		if mw == nil {
			return out
		}
		return mw(out)
	}, handler)
}

// RecoverMiddleware turns panic into normal error so dispatch can continue.
func RecoverMiddleware() Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, event Event) (err error) {
			defer func() {
				if recovered := recover(); recovered != nil {
					if recoveredErr, ok := recovered.(error); ok {
						err = oops.In("eventx").
							With("op", "middleware_recover", "middleware", "recover", "event_type", reflect.TypeOf(event), "panic_type", reflect.TypeOf(recovered)).
							Wrapf(recoveredErr, "eventx: recovered panic")
						return
					}
					err = oops.In("eventx").
						With("op", "middleware_recover", "middleware", "recover", "event_type", reflect.TypeOf(event), "panic_type", reflect.TypeOf(recovered), "panic", recovered).
						Errorf("eventx: recovered panic: %v", recovered)
				}
			}()
			return next(ctx, event)
		}
	}
}

// ObserveMiddleware reports per-dispatch execution result.
func ObserveMiddleware(observer func(ctx context.Context, event Event, duration time.Duration, err error)) Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, event Event) error {
			start := time.Now()
			err := next(ctx, event)
			if observer != nil {
				observer(ctx, event, time.Since(start), err)
			}
			return err
		}
	}
}
