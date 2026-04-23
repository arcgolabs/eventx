package fx

import (
	"github.com/arcgolabs/eventx"
	pkgfx "github.com/arcgolabs/pkg/fx"
	"go.uber.org/fx"
)

// EventParams defines parameters for eventx module.
type EventParams struct {
	fx.In

	// Options for creating event bus.
	Options []eventx.Option `group:"eventx_options,soft"`
}

// EventResult defines result for eventx module.
type EventResult struct {
	fx.Out

	// Bus is the created event bus.
	Bus eventx.BusRuntime
}

// NewEventBus creates a new event bus.
func NewEventBus(params EventParams) EventResult {
	bus := eventx.New(params.Options...)
	return EventResult{Bus: bus}
}

// NewEventxModule creates a eventx module.
func NewEventxModule(opts ...eventx.Option) fx.Option {
	return fx.Module("eventx",
		pkgfx.ProvideOptionGroup[eventx.Options, eventx.Option]("eventx_options", opts...),
		fx.Provide(NewEventBus),
	)
}

// NewEventxModuleWithAsync creates a eventx module with ants pool enabled.
func NewEventxModuleWithAsync(poolSize int) fx.Option {
	return NewEventxModule(eventx.WithAntsPool(poolSize))
}

// NewEventxModuleWithParallel creates a eventx module with parallel dispatch enabled.
func NewEventxModuleWithParallel() fx.Option {
	return NewEventxModule(eventx.WithParallelDispatch(true))
}
