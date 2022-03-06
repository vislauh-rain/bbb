package log_acc

import (
	"container/ring"
	"context"
	"errors"
	"fmt"
	print "log"
	"net"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/vislauh-rain/bbb/bbb"
	"github.com/vislauh-rain/bbb/log"
)

type logger struct {
	log        *ring.Ring
	logLen     int
	once       sync.Once
	done       chan struct{}
	finished   chan struct{}
	logCh      chan string
	updateCh   chan bbb.UrlUpdate
	dumpTicker *time.Ticker
	printer    *print.Logger
	buffer     *strings.Builder
	updates    map[int]accUpdate
}

//TODO customize

const logLen = 2

func New() log.Logger {
	l := &logger{
		log:        ring.New(logLen),
		logLen:     logLen,
		done:       make(chan struct{}),
		finished:   make(chan struct{}),
		logCh:      make(chan string),
		updateCh:   make(chan bbb.UrlUpdate),
		dumpTicker: time.NewTicker(5 * time.Second),
		buffer:     &strings.Builder{},
		updates:    map[int]accUpdate{},
	}
	l.printer = print.New(l.buffer, print.Prefix(), print.Flags())
	go l.routine()
	return l
}

func (l *logger) Log(ctx context.Context, _ bbb.LogLevel, msg string) {
	select {
	case l.logCh <- msg:
	case <-l.done:
	case <-ctx.Done():
	}
}

func (l *logger) Update(ctx context.Context, update bbb.UrlUpdate) {
	select {
	case l.updateCh <- update:
	case <-l.done:
	case <-ctx.Done():
	}
}

func (l *logger) routine() {
	defer close(l.finished)
	for {
		select {
		case msg := <-l.logCh:
			l.msg(msg)
		case update := <-l.updateCh:
			l.update(update)
		case <-l.dumpTicker.C:
			l.dump()
		case <-l.done:
			l.dump()
			return
		}
	}
}

func (l *logger) Stop() {
	l.once.Do(func() {
		l.dumpTicker.Stop()
		close(l.done)
	})
	<-l.finished
}

func (l *logger) msg(m string) {
	l.buffer.Reset()
	l.printer.Println(m)
	l.log.Value = l.buffer.String()
	l.log = l.log.Next()
}

type accUpdate struct {
	index    int
	url      string
	oks      int
	timeouts int
	errs     int
	lastE    string
	lastETm  time.Time
	started  time.Time
}

func (a accUpdate) total() int {
	return a.oks + a.timeouts + a.errs
}

func (a accUpdate) rate(now time.Time) float64 {
	return float64(a.total()) / now.Sub(a.started).Seconds()
}

func (a accUpdate) lastErr() string {
	if a.lastE == "" {
		return ""
	}
	if a.lastETm.IsZero() {
		return a.lastE
	}
	if time.Since(a.lastETm) > 30*time.Second {
		return ""
	}
	return a.lastE
}

func (l *logger) update(u bbb.UrlUpdate) {
	if u.Err != nil && strings.Contains(u.Err.Error(), "net::ERR_NAME_NOT_RESOLVED") {
		l.msg("waiting for DNS server...")
		if len(l.updates) > 0 {
			l.updates = map[int]accUpdate{}
		}
		return
	}

	acc := l.updates[u.Index]
	if acc.started.IsZero() {
		acc.started = time.Now()
	}
	acc.url = u.Url.Url
	if templateStart := strings.Index(acc.url, "{{"); templateStart >= 0 {
		acc.url = acc.url[:templateStart]
	}
	acc.index = u.Index
	if u.Err == nil {
		acc.oks++
	} else if errors.Is(u.Err, context.Canceled) ||
		errors.Is(u.Err, context.DeadlineExceeded) {
		acc.timeouts++
	} else if opErr := new(net.OpError); errors.As(u.Err, &opErr) && opErr.Timeout() {
		acc.timeouts++
	} else {
		acc.errs++
		acc.lastE = u.Err.Error()
		acc.lastETm = time.Now()
	}
	l.updates[u.Index] = acc
}

const header = "#   url\ttotal\toks\terrs\ttimeouts\treq/sec\tlast error\n"

func (l *logger) dump() {
	var tableBuilder strings.Builder
	if len(l.updates) > 0 {
		urls := make([]accUpdate, 0, len(l.updates))
		for _, acc := range l.updates {
			urls = append(urls, acc)
		}
		sort.Slice(urls, func(i, j int) bool {
			return urls[i].index < urls[j].index
		})

		tableBuilder.WriteString(header)
		now := time.Now()
		for _, acc := range urls {
			tableBuilder.WriteString(fmt.Sprintf("%d. %s\t%d\t%d\t%d\t%d\t%.2f\t%s\n",
				acc.index+1, acc.url, acc.total(), acc.oks, acc.errs, acc.timeouts, acc.rate(now), acc.lastErr()))
		}
	}

	var logBuilder strings.Builder
	var cur = l.log
	for i := 0; i < l.logLen; i++ {
		if cur.Value != nil {
			logBuilder.WriteString(cur.Value.(string))
			cur.Value = nil
		}
		cur = cur.Next()
	}

	if tableBuilder.Len() > 0 {
		_, _ = os.Stdout.WriteString(tableBuilder.String())
	}
	if logBuilder.Len() > 0 {
		_, _ = os.Stderr.WriteString(logBuilder.String())
	}
}
