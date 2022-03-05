package bbb

import (
	"context"
)

func LogNone(_ context.Context, _ LogLevel, _ string) {}

func UpdateNone(_ context.Context, _ UrlUpdate) {}

func logFilter(level LogLevel, log LogFn) LogFn {
	return func(ctx context.Context, l LogLevel, msg string) {
		if ctx.Err() != nil {
			return
		}
		if l <= level {
			log(ctx, l, msg)
		}
	}
}

func updateFilter(log UpdateFn) UpdateFn {
	return func(ctx context.Context, update UrlUpdate) {
		if ctx.Err() != nil {
			return
		}
		log(ctx, update)
	}
}
