package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"go-gin-template/internal/app"
	"go-gin-template/internal/config"
)

func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func run() error {
	configPath := flag.String("config", "configs/config.yaml", "path to YAML config")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	application, err := app.New(ctx, cfg)
	if err != nil {
		return fmt.Errorf("initialize application: %w", err)
	}
	return application.Run(ctx)
}
