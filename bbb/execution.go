package bbb

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"reflect"
	"sync"
	"time"
)

type BBB interface {
	Go(ctx context.Context) <-chan error
}

type Impl interface {
	Init(context.Context) error
	Generate() <-chan Task
	Worker(<-chan Task)
	Done()
}

type NewImpl = func() Impl

func New(impl NewImpl, workers int, log LogConfig) BBB {
	return &bb{workers, impl, log}
}

func Configure(config Config) (Config, error) {
	var err error
	if err = validate.Struct(config); err != nil {
		return Config{}, err
	}
	config.Workers.timeout, config.Workers.hasTimeout, err = getTimeout(config.Workers)
	if err != nil {
		return Config{}, err
	}
	if config.Log.Level == 0 {
		config.Log.Level = LogLevelDefault
	}
	if config.Log.LogFn == nil {
		config.Log.LogFn = LogNone
	} else {
		config.Log.LogFn = logFilter(config.Log.Level, config.Log.LogFn)
	}
	if config.Log.UpdateFn == nil {
		config.Log.UpdateFn = UpdateNone
	} else {
		config.Log.UpdateFn = updateFilter(config.Log.UpdateFn)
	}
	return config, nil
}

type bb struct {
	workers int
	impl    NewImpl
	log     LogConfig
}

func (b *bb) Go(ctx context.Context) <-chan error {
	wk := b.workers
	impl := b.impl()
	result := make(chan error, 1)
	wg := sync.WaitGroup{}
	wg.Add(wk)

	err := impl.Init(ctx)
	if err != nil {
		result <- err
		close(result)
		return result
	}

	ch := impl.Generate()
	for i := 0; i < wk; i++ {
		go func() {
			defer wg.Done()
			impl.Worker(ch)
		}()
	}
	go func() {
		wg.Wait()
		b.log.LogFn(context.Background(), LogLevelVerbose, "wait finished")
		for range ch {
		}
		impl.Done()
		b.log.LogFn(context.Background(), LogLevelVerbose, "impl Done")
		close(result)
	}()

	return result
}

type Task struct {
	Method    string
	Url       string
	UrlConfig UrlConfig
	Body      io.ReadCloser
	Index     int
}

func Generator(ctx context.Context, urls []UrlConfig, log LogConfig) <-chan Task {
	ch := make(chan Task)
	go func() {
		defer close(ch)
		generator(ctx, ch, urls, log)
		log.LogFn(context.Background(), LogLevelVerbose, "generator finished")
	}()
	return ch
}

func generator(ctx context.Context, out chan<- Task, urls []UrlConfig, log LogConfig) {
	invalidUrls := map[int]bool{}

	i := 0
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		cur := i
		i++
		if i >= len(urls) {
			i = 0
		}

		if invalidUrls[cur] {
			continue
		}
		t, ok, err := genTask(&urls[cur], cur)
		if !ok || err != nil {
			if err != nil {
				log.LogFn(ctx, LogLevelError, fmt.Sprintf("invalid url %q at %d: %s", urls[cur].Url, cur, err))
			}
			invalidUrls[cur] = true
			if len(invalidUrls) == len(urls) {
				log.LogFn(ctx, LogLevelError, "generator finished as all urls are invalid or disabled")
				return
			}
			continue
		}

		select {
		case out <- t:
		case <-ctx.Done():
			return
		}
	}
}

func genTask(urlConfig *UrlConfig, index int) (Task, bool, error) {
	if urlConfig.Disable {
		return Task{}, false, nil
	}
	url, err := urlConfig.GenUrl()
	if err != nil {
		return Task{}, false, err
	}
	return Task{
		Method:    urlConfig.randMethod(),
		UrlConfig: *urlConfig,
		Url:       url,
		Body:      nil,
		Index:     index,
	}, true, nil
}

var r = rand.New(rand.NewSource(time.Now().Unix()))

func (u UrlConfig) randMethod() string {
	switch u.Methods {
	case nil:
		return http.MethodGet
	case "all":
		return allMethod[r.Intn(len(allMethod))]
	default:
		slice := reflect.ValueOf(u.Methods)
		return slice.Index(r.Intn(slice.Len())).String()
	}
}
