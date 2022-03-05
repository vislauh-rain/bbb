package http

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/vislauh-rain/bbb/bbb"
)

func New(config bbb.Config, http Config) (bbb.BBB, error) {
	config, err := bbb.Configure(config)
	if err != nil {
		return nil, err
	}
	return bbb.New(impl(config, http), config.Workers.GetWorkersCount(), config.Log), nil
}

func impl(config bbb.Config, http Config) bbb.NewImpl {
	return func() bbb.Impl {
		return &bb{config: config, http: http}
	}
}

type bb struct {
	ctx    context.Context
	config bbb.Config
	http   Config
}

func (b *bb) Init(ctx context.Context) error {
	b.ctx = ctx
	return nil
}

func (b *bb) Generate() <-chan bbb.Task {
	return bbb.Generator(b.ctx, b.config.Urls, b.config.Log)
}

func (b *bb) Worker(tasks <-chan bbb.Task) {
	tm, hasTm := b.config.Workers.GetTimeout()
	if !hasTm {
		tm = 0
	}
	for t := range tasks {
		doTask(b.ctx, t, tm, b.http, b.config.Log)
	}
}

func doTask(ctx context.Context, t bbb.Task, timeout time.Duration, config Config, log bbb.LogConfig) {
	if t.Body != nil {
		defer t.Body.Close()
	}
	log.LogFn(ctx, bbb.LogLevelVerbose, fmt.Sprintf("Requesting %s %s", t.Method, t.Url))
	rCtx := ctx
	if timeout > 0 {
		newCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		rCtx = newCtx
	}
	req, err := http.NewRequestWithContext(rCtx, t.Method, t.Url, t.Body)
	if err != nil {
		log.LogFn(ctx, bbb.LogLevelError, err.Error())
		return
	}
	for k, v := range config.Header {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.UpdateFn(ctx, bbb.UrlUpdate{
			Url:   t.UrlConfig,
			Index: t.Index,
			Err:   err,
		})
		return
	}
	defer resp.Body.Close()
	_, err = io.ReadAll(resp.Body)
	if err != nil {
		log.UpdateFn(ctx, bbb.UrlUpdate{
			Url:   t.UrlConfig,
			Index: t.Index,
			Err:   err,
		})
		return
	}
	if resp.StatusCode >= http.StatusBadRequest {
		log.UpdateFn(ctx, bbb.UrlUpdate{
			Url:   t.UrlConfig,
			Index: t.Index,
			Err:   errors.New(resp.Status),
		})
		return
	}
	log.UpdateFn(ctx, bbb.UrlUpdate{
		Url:   t.UrlConfig,
		Index: t.Index,
		Ok:    true,
	})
}

func (b *bb) Done() {}
