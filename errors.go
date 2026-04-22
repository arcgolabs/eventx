package eventx

import "errors"

var (
	// ErrBusClosed indicates that the bus has been closed.
	ErrBusClosed = errors.New("eventx: bus is closed")
	// ErrNilEvent indicates that publish received a nil event.
	ErrNilEvent = errors.New("eventx: event is nil")
	// ErrNilHandler indicates that subscribe received a nil handler.
	ErrNilHandler = errors.New("eventx: handler is nil")
	// ErrNilBus indicates that operation received a nil bus.
	ErrNilBus = errors.New("eventx: bus is nil")
	// ErrAsyncRuntimeUnavailable indicates async runtime could not be initialized.
	ErrAsyncRuntimeUnavailable = errors.New("eventx: async runtime unavailable")
	// ErrInvalidSubscribeCount indicates subscribe call count limit is invalid.
	ErrInvalidSubscribeCount = errors.New("eventx: subscribe count must be greater than zero")
)
