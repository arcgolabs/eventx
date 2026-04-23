package eventx

import (
	"context"
	"reflect"
	"strings"
	"time"

	"github.com/arcgolabs/observabilityx"
)

func (b *Bus) observabilitySafe() observabilityx.Observability {
	if b == nil {
		return observabilityx.Nop()
	}
	return observabilityx.Normalize(b.observability, b.logger)
}

func eventName(event Event) string {
	if event == nil {
		return ""
	}

	name := strings.TrimSpace(event.Name())
	if name != "" {
		return name
	}
	return reflect.TypeOf(event).String()
}

func backgroundContext() context.Context {
	return context.Background()
}

func normalizeContext(ctx context.Context) context.Context {
	if ctx != nil {
		return ctx
	}
	return backgroundContext()
}

func recordAsyncEnqueueMetrics(
	ctx context.Context,
	obs observabilityx.Observability,
	start time.Time,
	event string,
	result string,
) {
	obs.Counter(asyncEnqueueTotalSpec).Add(ctx, 1,
		observabilityx.String("result", result),
		observabilityx.String("event_name", event),
	)
	obs.Histogram(asyncEnqueueDurationSpec).Record(ctx, float64(time.Since(start).Milliseconds()),
		observabilityx.String("result", result),
		observabilityx.String("event_name", event),
	)
}
