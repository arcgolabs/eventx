// Package main demonstrates using eventx with fx and logx modules.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/DaiYuANg/arcgo/collectionx"
	"github.com/DaiYuANg/arcgo/logx"
	logxfx "github.com/DaiYuANg/arcgo/logx/fx"
	"github.com/arcgolabs/eventx"
	eventxfx "github.com/arcgolabs/eventx/fx"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
)

type notificationEvent struct {
	Type    string
	UserID  string
	Message string
}

func (e notificationEvent) Name() string {
	return "notification." + e.Type
}

func main() {
	mustPrintln("=== EventX + FX + LogX example ===")

	app := newApp()
	if err := app.Start(context.Background()); err != nil {
		panic(err)
	}
}

func newApp() *fx.App {
	return fx.New(
		fx.WithLogger(func(log *slog.Logger) fxevent.Logger {
			return &fxevent.SlogLogger{Logger: log}
		}),
		logxfx.NewLogxModuleWithSlog(
			logx.WithLevel(logx.DebugLevel),
			logx.WithCaller(true),
		),
		eventxfx.NewEventxModule(
			eventx.WithAntsPool(4),
			eventx.WithParallelDispatch(true),
		),
		fx.Invoke(registerNotificationSubscribers),
		fx.Invoke(registerPublishHook),
	)
}

func registerNotificationSubscribers(bus eventx.BusRuntime, logger *slog.Logger) error {
	logger.Info("registering notification subscribers")

	for _, cfg := range []subscriberConfig{
		{msgType: "email", logLabel: "send email"},
		{msgType: "sms", logLabel: "send sms"},
		{msgType: "push", logLabel: "send push"},
	} {
		if err := subscribeNotificationType(bus, logger, cfg); err != nil {
			return err
		}
	}

	logger.Info("all notification subscribers registered")
	return nil
}

type subscriberConfig struct {
	msgType  string
	logLabel string
}

func subscribeNotificationType(bus eventx.BusRuntime, logger *slog.Logger, cfg subscriberConfig) error {
	_, err := eventx.Subscribe[notificationEvent](bus, func(_ context.Context, event notificationEvent) error {
		if event.Type != cfg.msgType {
			return nil
		}

		logx.WithFields(logger, collectionx.NewMapFrom(map[string]any{
			"user_id":  event.UserID,
			"msg_type": cfg.msgType,
		})).Info(cfg.logLabel)
		mustPrintf("   %s to %s: %s\n", cfg.logLabel, event.UserID, event.Message)
		time.Sleep(100 * time.Millisecond)
		return nil
	})
	if err != nil {
		return fmt.Errorf("subscribe %s notifications: %w", cfg.msgType, err)
	}
	return nil
}

func registerPublishHook(lc fx.Lifecycle, bus eventx.BusRuntime, logger *slog.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return publishNotifications(ctx, bus, logger)
		},
	})
}

func publishNotifications(ctx context.Context, bus eventx.BusRuntime, logger *slog.Logger) error {
	logger.Info("publishing notification events")
	mustPrintln("\n=== publish notification events ===")

	events := []notificationEvent{
		{Type: "email", UserID: "user-001", Message: "欢迎注册！"},
		{Type: "sms", UserID: "user-001", Message: "验证码：123456"},
		{Type: "push", UserID: "user-002", Message: "您有新的消息"},
		{Type: "email", UserID: "user-002", Message: "订单已发货"},
	}

	for i, event := range events {
		logx.WithFields(logger, collectionx.NewMapFrom(map[string]any{
			"index":   i + 1,
			"type":    event.Type,
			"user_id": event.UserID,
			"total":   len(events),
		})).Info("publish notification event")

		if err := bus.PublishAsync(ctx, event); err != nil {
			logx.WithError(logx.WithFields(logger, collectionx.NewMapFrom(map[string]any{
				"event": event,
			})), err).Error("publish event failed")
		}
	}

	logger.Info("all notifications published to the async queue")
	mustPrintln("\nall notifications published to the async queue")
	time.Sleep(500 * time.Millisecond)
	return nil
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
