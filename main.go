package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/vislauh-rain/bbb/bbb"
	"github.com/vislauh-rain/bbb/http"
	"github.com/vislauh-rain/bbb/log/log_acc"
	"github.com/vislauh-rain/bbb/rod"
)

var configFile = flag.String("config", "config/config.json", "config file path")

type Config struct {
	bbb.Config
	Mode string `binding:"eq=|eq=http|eq=rod"`
	Http http.Config
	Rod  rod.Config
}

func main() {
	flag.Parse()

	configFile, err := os.Open(*configFile)
	if err != nil {
		log.Fatalln(err)
	}
	var config Config
	err = json.NewDecoder(configFile).Decode(&config)
	_ = configFile.Close()
	if err != nil {
		log.Fatalln(err)
	}

	logger := log_acc.New()
	config.Log.LogFn = logger.Log
	config.Log.UpdateFn = logger.Update

	var b bbb.BBB
	switch config.Mode {
	case "http":
		b, err = http.New(config.Config, config.Http)
	default:
		b, err = rod.New(config.Config, config.Rod)
	}
	if err != nil {
		log.Fatalln(err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	err = <-b.Go(ctx)
	logger.Stop()
	if err != nil {
		log.Fatalln(err)
	}
}
