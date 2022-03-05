package rod

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/go-rod/stealth"
	"github.com/vislauh-rain/bbb/bbb"
)

func New(config bbb.Config, rod Config) (bbb.BBB, error) {
	config, err := bbb.Configure(config)
	if err != nil {
		return nil, err
	}
	return bbb.New(impl(config, rod), config.Workers.GetWorkersCount(), config.Log), nil
}

func impl(config bbb.Config, rod Config) bbb.NewImpl {
	return func() bbb.Impl {
		return &bb{config: config, rod: rod}
	}
}

type bb struct {
	config   bbb.Config
	rod      Config
	launcher *launcher.Launcher
	browser  *rod.Browser
	ctx      context.Context
}

func (b *bb) Init(ctx context.Context) error {
	b.ctx = ctx

	b.launcher = launcher.New().Context(ctx)
	if b.rod.ShowUi {
		b.launcher.Headless(false)
	}
	url, err := b.launcher.Launch()
	if err != nil {
		b.Done()
		return err
	}

	b.browser = rod.New().Context(ctx).ControlURL(url)
	err = b.browser.Connect()
	if err != nil {
		b.browser = nil
		b.Done()
		return err
	}
	//TODO detect browser close in some way
	return nil
}

func (b *bb) Generate() <-chan bbb.Task {
	return bbb.Generator(b.ctx, b.config.Urls, b.config.Log)
}

func (b *bb) Worker(tasks <-chan bbb.Task) {
	page := stealth.MustPage(b.browser).Context(b.ctx)
	errP := &errPusher{}
	ddos := &ddosDetector{}

	go page.EachEvent(func(e *proto.NetworkResponseReceived) {
		b.config.Log.LogFn(b.ctx, bbb.LogLevelVerbose, fmt.Sprintf("Fetched %q, status %d", e.Response.URL, e.Response.Status))
		if strings.Contains(e.Response.URL, "ddos") {
			ddos.setDdos()
		}
		if e.Type == "Document" && e.Response.Status >= http.StatusBadRequest {
			if e.Response.StatusText != "" {
				errP.Push(errors.New(e.Response.StatusText))
			} else if e.Response.Status >= http.StatusInternalServerError {
				errP.Push(fmt.Errorf("status %d", e.Response.Status))
			}
		}
	})()

	for t := range tasks {
		doTask(page, t, b.config, errP, ddos)
	}
}

func doTask(page *rod.Page, t bbb.Task, config bbb.Config, errP *errPusher, ddos *ddosDetector) {
	defer rec(page.GetContext(), t, config.Log)

	errc := make(chan error, 1)
	errP.Install(errc)
	defer errP.Cleanup()

	timeout, hasTimeout := config.Workers.GetTimeout()
	if hasTimeout {
		p, cancel := ddos.withTimeout(page, timeout, config.Log)
		defer cancel()
		page = p
	}

	config.Log.LogFn(page.GetContext(), bbb.LogLevelVerbose, "Navigating to "+t.Url)
	page.MustNavigate(t.Url)
	page.MustWaitLoad()

	select {
	case err := <-errc:
		config.Log.UpdateFn(page.GetContext(), bbb.UrlUpdate{
			Url:   t.UrlConfig,
			Index: t.Index,
			Err:   err,
		})
	default:
		config.Log.UpdateFn(page.GetContext(), bbb.UrlUpdate{
			Url:   t.UrlConfig,
			Index: t.Index,
			Ok:    true,
		})
	}
}

type errPusher struct {
	ch atomic.Value
}

func (e *errPusher) Push(err error) {
	i := e.ch.Load()
	if i == nil {
		return
	}
	ch := i.(chan error)
	if ch == nil {
		return
	}
	select {
	case ch <- err:
	default:
	}
}

func (e *errPusher) Install(ch chan error) {
	e.ch.Store(ch)
}

func (e *errPusher) Cleanup() {
	var ch chan error
	e.ch.Store(ch)
}

type ddosDetector struct {
	ch atomic.Value
}

func (d *ddosDetector) withTimeout(parent *rod.Page, dur time.Duration, log bbb.LogConfig) (*rod.Page, context.CancelFunc) {
	//ctx, cancel := context.WithCancel(parent)
	page, cancel := parent.WithCancel()
	done := make(chan struct{})
	go func() {
		timer := time.NewTimer(dur)
		defer timer.Stop()

		select {
		case <-timer.C:
			cancel()
		case <-done:
		case <-d.ddos():
			log.LogFn(parent.GetContext(), bbb.LogLevelVerbose, "timeout cancelled 'cause of ddos protection")
		}
	}()
	return page, func() {
		close(done)
		cancel()
	}
}

func (d *ddosDetector) ddos() <-chan struct{} {
	ch := make(chan struct{}, 1)
	d.ch.Store(ch)
	return ch
}

func (d *ddosDetector) setDdos() {
	i := d.ch.Load()
	if i == nil {
		return
	}
	ch := i.(chan struct{})
	if ch == nil {
		return
	}
	select {
	case ch <- struct{}{}:
	default:
	}
}

func rec(ctx context.Context, t bbb.Task, log bbb.LogConfig) {
	crash := recover()
	if crash == nil {
		return
	}
	var err error
	if e, ok := crash.(error); ok {
		err = e
	} else {
		err = fmt.Errorf("%v", crash)
	}
	log.UpdateFn(ctx, bbb.UrlUpdate{
		Url:   t.UrlConfig,
		Index: t.Index,
		Err:   err,
	})
}

func (b *bb) Done() {
	if b.browser != nil {
		b.browser.MustClose()
		b.browser = nil
	}
	b.config.Log.LogFn(context.Background(), bbb.LogLevelVerbose, "browser closed")
	if b.launcher != nil {
		//b.launcher.Cleanup() TODO hungs???
		b.launcher = nil
	}
	b.config.Log.LogFn(context.Background(), bbb.LogLevelVerbose, "launcher cleaned")
}
