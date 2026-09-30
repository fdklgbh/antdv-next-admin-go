package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"go-gin-template/internal/config"
	"go-gin-template/internal/infrastructure/database"
	"go-gin-template/migrations"
)

func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}
func run() error {
	configPath := flag.String("config", "configs/config.yaml", "path to YAML config")
	action := flag.String("action", "up", "migration action: up or down")
	flag.Parse()
	cfg, err := config.Load(*configPath)
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.StartupTimeout)
	defer cancel()
	db, closeDB, err := database.Open(ctx, cfg.Database)
	if err != nil {
		return err
	}
	defer closeDB()
	switch *action {
	case "up":
		err = migrations.Migrator(db).Migrate()
	case "down":
		err = migrations.Migrator(db).RollbackLast()
	default:
		return fmt.Errorf("unsupported migration action: %s", *action)
	}
	if err != nil {
		return fmt.Errorf("run migration %s: %w", *action, err)
	}
	return nil
}
