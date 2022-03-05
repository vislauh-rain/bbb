package log

import (
	"context"

	"github.com/vislauh-rain/bbb/bbb"
)

type Logger interface {
	Log(ctx context.Context, level bbb.LogLevel, msg string)
	Update(ctx context.Context, update bbb.UrlUpdate)
	Stop()
}
