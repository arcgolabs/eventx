// Package main demonstrates eventx global and subscriber middleware.
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/arcgolabs/eventx"
)

type userEvent struct {
	EventName string
	UserID    string
	Timestamp time.Time
}

func (e userEvent) Name() string {
	return e.EventName
}

type userRegisteredEvent struct {
	userEvent
	Email    string
	UserName string
}

func (e userRegisteredEvent) Name() string {
	return e.userEvent.Name()
}

type userLoginEvent struct {
	userEvent
	IPAddress string
}

func (e userLoginEvent) Name() string {
	return e.userEvent.Name()
}

func main() {
	mustPrintln("=== EventX middleware example ===")

	bus := newMiddlewareBus()
	defer func() {
		if err := bus.Close(); err != nil {
			panic(err)
		}
	}()

	if err := registerMiddlewareSubscribers(bus); err != nil {
		panic(err)
	}

	publishMiddlewareDemo(bus)
	mustPrintln("\nbus keeps running after middleware recovered the panic")
}

func newMiddlewareBus() eventx.BusRuntime {
	return eventx.New(
		eventx.WithAntsPool(4),
		eventx.WithMiddleware(func(next eventx.HandlerFunc) eventx.HandlerFunc {
			return func(ctx context.Context, event eventx.Event) error {
				start := time.Now()
				mustPrintf("[middleware] start handling event: %s\n", event.Name())
				err := next(ctx, event)
				duration := time.Since(start)
				mustPrintf("[middleware] event completed: %s, duration: %v\n", event.Name(), duration)
				return err
			}
		}),
		eventx.WithMiddleware(eventx.RecoverMiddleware()),
	)
}

func registerMiddlewareSubscribers(bus eventx.BusRuntime) error {
	if err := subscribeUserRegistration(bus); err != nil {
		return err
	}
	if err := subscribeUserLogin(bus); err != nil {
		return err
	}
	return nil
}

func subscribeUserRegistration(bus eventx.BusRuntime) error {
	_, err := eventx.Subscribe[userRegisteredEvent](bus,
		func(_ context.Context, event userRegisteredEvent) error {
			mustPrintf("  handle user registration: %s (%s)\n", event.UserName, event.Email)

			if event.Email == "panic@example.com" {
				panic("simulate handler panic")
			}

			time.Sleep(100 * time.Millisecond)
			return nil
		},
		eventx.WithSubscriberMiddleware(func(next eventx.HandlerFunc) eventx.HandlerFunc {
			return func(ctx context.Context, event eventx.Event) error {
				mustPrintf("  [auth] validate event permissions for %s\n", event.Name())
				return next(ctx, event)
			}
		}),
	)
	if err != nil {
		return fmt.Errorf("subscribe user registration: %w", err)
	}
	return nil
}

func subscribeUserLogin(bus eventx.BusRuntime) error {
	_, err := eventx.Subscribe[userLoginEvent](bus, func(_ context.Context, event userLoginEvent) error {
		mustPrintf("  handle user login: %s from %s\n", event.UserID, event.IPAddress)
		time.Sleep(50 * time.Millisecond)
		return nil
	})
	if err != nil {
		return fmt.Errorf("subscribe user login: %w", err)
	}
	return nil
}

func publishMiddlewareDemo(bus eventx.BusRuntime) {
	publishUserEvent(
		"--- publish user registered event (normal) ---",
		func() error {
			return bus.Publish(context.Background(), userRegisteredEvent{
				userEvent: userEvent{
					EventName: "user.registered",
					UserID:    "user-001",
					Timestamp: time.Now(),
				},
				Email:    "john@example.com",
				UserName: "John Doe",
			})
		},
	)

	publishUserEvent(
		"\n--- publish user login event ---",
		func() error {
			return bus.Publish(context.Background(), userLoginEvent{
				userEvent: userEvent{
					EventName: "user.loggedin",
					UserID:    "user-001",
					Timestamp: time.Now(),
				},
				IPAddress: "192.168.1.100",
			})
		},
	)

	publishUserEvent(
		"\n--- publish user registered event (panic case) ---",
		func() error {
			return bus.Publish(context.Background(), userRegisteredEvent{
				userEvent: userEvent{
					EventName: "user.registered",
					UserID:    "user-002",
					Timestamp: time.Now(),
				},
				Email:    "panic@example.com",
				UserName: "Panic User",
			})
		},
	)
}

func publishUserEvent(title string, publish func() error) {
	mustPrintln(title)
	if err := publish(); err != nil {
		mustPrintf("error: %v\n", err)
	}
}

func mustPrintln(args ...any) {
	if _, err := fmt.Println(args...); err != nil {
		panic(err)
	}
}

func mustPrintf(format string, args ...any) {
	if _, err := fmt.Printf(format, args...); err != nil {
		panic(err)
	}
}
