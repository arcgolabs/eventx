package eventx

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/DaiYuANg/arcgo/collectionx"
	"github.com/DaiYuANg/arcgo/observabilityx"
	"github.com/samber/lo"
)

func (b *Bus) dispatch(ctx context.Context, event Event, handlers []HandlerFunc, mode string) error {
	if len(handlers) == 0 {
		b.logger.Debug("dispatch skipped",
			"mode", mode,
			"event_name", eventName(event),
			"reason", "no_handlers",
		)
		return nil
	}
	ctx = normalizeContext(ctx)

	obs := b.observabilitySafe()
	start := time.Now()
	ctx, span := obs.StartSpan(ctx, "eventx.dispatch",
		observabilityx.String("mode", mode),
		observabilityx.String("event_name", eventName(event)),
		observabilityx.Int64("handlers", int64(len(handlers))),
	)
	defer span.End()

	result := "success"
	defer func() {
		obs.Counter(dispatchTotalSpec).Add(ctx, 1,
			observabilityx.String("mode", mode),
			observabilityx.String("result", result),
			observabilityx.String("event_name", eventName(event)),
		)
		obs.Histogram(dispatchDurationSpec).Record(ctx, float64(time.Since(start).Milliseconds()),
			observabilityx.String("mode", mode),
			observabilityx.String("result", result),
			observabilityx.String("event_name", eventName(event)),
		)
	}()

	var err error
	if b.parallel {
		err = b.dispatchParallel(ctx, event, handlers)
	} else {
		err = b.dispatchSerial(ctx, event, handlers)
	}

	if err != nil {
		result = "error"
		span.RecordError(err)
	}
	b.logger.Debug("dispatch completed",
		"mode", mode,
		"event_name", eventName(event),
		"handler_count", len(handlers),
		"result", result,
	)
	return err
}

func (b *Bus) dispatchSerial(ctx context.Context, event Event, handlers []HandlerFunc) error {
	errs := lo.FilterMap(handlers, func(handler HandlerFunc, _ int) (error, bool) {
		if handler == nil {
			return nil, false
		}
		err := handler(ctx, event)
		return err, err != nil
	})
	return errors.Join(errs...)
}

func (b *Bus) dispatchParallel(ctx context.Context, event Event, handlers []HandlerFunc) error {
	errCh := make(chan error, len(handlers))
	var wg sync.WaitGroup

	lo.ForEach(handlers, func(handler HandlerFunc, _ int) {
		if handler == nil {
			return
		}
		wg.Add(1)
		go func(h HandlerFunc) {
			defer wg.Done()
			b.acquireParallelSlot()
			defer b.releaseParallelSlot()
			if err := h(ctx, event); err != nil {
				errCh <- err
			}
		}(handler)
	})

	wg.Wait()
	close(errCh)

	errs := collectionx.NewListWithCapacity[error](len(handlers))
	for err := range errCh {
		errs.Add(err)
	}
	return errors.Join(errs.Values()...)
}

func (b *Bus) acquireParallelSlot() {
	if b == nil || b.parallelLimit == nil {
		return
	}
	b.parallelLimit <- struct{}{}
}

func (b *Bus) releaseParallelSlot() {
	if b == nil || b.parallelLimit == nil {
		return
	}
	<-b.parallelLimit
}
