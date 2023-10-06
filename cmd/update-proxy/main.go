package main

import (
	"context"
	"fmt"
	"github.com/lukeelten/openshift-update-proxy/pkg/proxy"
	"log"
	"net/http"
	"os/signal"
	"syscall"

	"github.com/lukeelten/openshift-update-proxy/pkg/config"
	"go.uber.org/zap"
)

func main() {
	cfg := config.LoadConfig()
	var err error

	var logger *zap.Logger
	if cfg.Debug {
		logger, err = zap.NewDevelopment()
	} else {
		zapConfig := zap.NewProductionConfig()
		zapConfig.Encoding = "console"
		zapConfig.EncoderConfig = zap.NewDevelopmentEncoderConfig()
		logger, err = zapConfig.Build()
	}

	if err != nil {
		log.Fatal(err)
	}

	defer logger.Sync()
	app := proxy.NewOpenShiftUpdateProxy(cfg, logger.Sugar())

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT, syscall.SIGKILL, syscall.SIGHUP)
	defer stop()

	globalContext, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Entkopplung vom Signal Context
	go func() {
		<-ctx.Done()
		stop()
		cancel() // Cancel global context when signals are received
	}()

	err = app.Run(globalContext)
	if err != nil && err != http.ErrServerClosed {
		logger.Fatal(fmt.Sprintf("got runtime error: %s", err.Error()))
	}
}
