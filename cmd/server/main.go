package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/Abraxas-365/freerouter/internal/bootstrap"
	"github.com/Abraxas-365/freerouter/internal/config"
)

func main() {
	cfg := config.Load()

	container, err := bootstrap.New(cfg)
	if err != nil {
		log.Fatalf("failed to initialize: %v", err)
	}
	defer container.Cleanup()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Printf("FreeRouter starting on :%s (env=%s)", cfg.Server.Port, cfg.Server.Env)
	if err := container.Server.Start(ctx); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
