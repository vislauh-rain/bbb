package bbb

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"reflect"
	"runtime"
	"strings"
	"text/template"
	"time"

	"github.com/go-playground/validator/v10"
)

type Config struct {
	Urls    []UrlConfig `binding:"gte=1"`
	Workers WorkerConfig
	Log     LogConfig
}

type UrlConfig struct {
	Url      string      `binding:"url"`
	Methods  interface{} `binding:"methods"`
	Disable  bool
	template *template.Template
}

func (u *UrlConfig) GenUrl() (string, error) {
	if u.template == nil {
		var err error
		u.template, err = template.New(u.Url).Parse(u.Url)
		if err != nil {
			return "", err
		}
	}
	var builder strings.Builder
	err := u.template.Execute(&builder, urlContext{
		Now:      time.Now(),
		RandDate: randTime(),
	})
	return builder.String(), err
}

type WorkerConfig struct {
	WorkersCount int `json:"Count" binding:"gte=0"`
	Timeout      interface{}
	timeout      time.Duration
	hasTimeout   bool
}

func (c WorkerConfig) GetWorkersCount() int {
	if c.WorkersCount == 0 {
		return runtime.NumCPU()
	} else {
		return c.WorkersCount
	}
}

func (c WorkerConfig) GetTimeout() (time.Duration, bool) {
	return c.timeout, c.hasTimeout
}

var float64Tp = reflect.TypeOf(float64(0))

func getTimeout(c WorkerConfig) (time.Duration, bool, error) {
	if c.Timeout == nil {
		return 0, false, nil
	}
	val := reflect.ValueOf(c.Timeout)
	if val.CanConvert(float64Tp) {
		d := val.Convert(float64Tp).Float()
		return time.Duration(d * float64(time.Second)), true, nil
	}
	if val.Kind() == reflect.String {
		d, e := time.ParseDuration(val.String())
		if e != nil {
			return d, false, e
		}
		return d, true, nil
	}
	return 0, false, fmt.Errorf("unknown duration type %T", c.Timeout)
}

type LogConfig struct {
	Level    LogLevel
	LogFn    LogFn
	UpdateFn UpdateFn
}

type (
	LogFn    = func(ctx context.Context, level LogLevel, msg string)
	UpdateFn = func(ctx context.Context, update UrlUpdate)
)

//go:generate stringer -type=LogLevel
type LogLevel int

const (
	LogLevelNone LogLevel = iota + 1
	LogLevelError
	LogLevelWarning
	LogLevelDefault
	LogLevelVerbose
)

var logLevelPref = []string{"log", "loglevel"}

func (i *LogLevel) UnmarshalText(buf []byte) error {
	text := string(buf)
	for l := LogLevelNone; l <= LogLevelVerbose; l++ {
		level := strings.ToLower(l.String())
		if level == text {
			*i = l
			return nil
		}
		for _, p := range logLevelPref {
			if strings.TrimPrefix(level, p) == text {
				*i = l
				return nil
			}
		}
	}
	return fmt.Errorf("unknown log level %q", text)
}

type UrlUpdate struct {
	Url   UrlConfig
	Index int
	Ok    bool
	Err   error
}

var validate = validator.New()

func init() {
	validate.SetTagName("binding")
	err := validate.RegisterValidation("methods", func(fl validator.FieldLevel) bool {
		if fl.Field().IsNil() || !fl.Field().IsValid() || fl.Field().IsZero() {
			return true
		}
		switch fl.Field().Kind() {
		case reflect.String:
			return fl.Field().String() == "all"
		case reflect.Slice, reflect.Array:
			if fl.Field().Len() == 0 {
				return false
			}
			for i := 0; i < fl.Field().Len(); i++ {
				el := fl.Field().Index(i)
				if el.Kind() != reflect.String {
					return false
				}
				if !allMethodsSet[el.String()] {
					return false
				}
			}
			return true
		default:
			return false
		}
	}, true)
	if err != nil {
		log.Fatalln(err)
	}
}

var (
	allMethod = [...]string{
		http.MethodGet,
		http.MethodHead,
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete,
		http.MethodConnect,
		http.MethodOptions,
		http.MethodTrace,
	}
	allMethodsSet = map[string]bool{}
)

func init() {
	for _, m := range allMethod {
		allMethodsSet[m] = true
	}
}
