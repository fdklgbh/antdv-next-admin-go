package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"go-gin-template/internal/config"
	"go-gin-template/internal/httpserver"
	"go-gin-template/internal/infrastructure/logger"
)

type App struct {
	config         config.Config
	server         *http.Server
	logger         logger.Logger
	closeResources func() error
}

func New(ctx context.Context, cfg config.Config) (*App, error) {
	startupCtx, cancel := context.WithTimeout(ctx, cfg.Server.StartupTimeout)
	defer cancel()
	deps, err := buildDependencies(startupCtx, cfg)
	if err != nil {
		return nil, err
	}
	router, err := httpserver.NewRouter(cfg.Server.Mode, deps.routerDeps)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("create router: %w", err), closeAll(deps.closers))
	}
	server := &http.Server{Addr: cfg.Server.Address, Handler: router, ReadHeaderTimeout: cfg.Server.ReadTimeout, ReadTimeout: cfg.Server.ReadTimeout, WriteTimeout: cfg.Server.WriteTimeout, MaxHeaderBytes: cfg.Server.MaxHeaderBytes}
	return &App{config: cfg, server: server, logger: deps.routerDeps.Logger, closeResources: func() error { return closeAll(deps.closers) }}, nil
}
func (a *App) Run(ctx context.Context) (runErr error) {
	defer func() { runErr = errors.Join(runErr, a.closeResources()) }()
	serverErrors := make(chan error, 1)
	go func() {
		a.logger.Info(context.Background(), "http_server_started")
		serverErrors <- a.server.ListenAndServe()
	}()
	select {
	case err := <-serverErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve HTTP: %w", err)
	case <-ctx.Done():
		a.logger.Info(context.Background(), "http_server_shutdown_started")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), a.config.Server.ShutdownTimeout)
		defer cancel()
		if err := a.server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown HTTP server: %w", err)
		}
		a.logger.Info(context.Background(), "http_server_stopped")
		return nil
	}
}
func closeAll(closers []func() error) error {
	var err error
	for index := len(closers) - 1; index >= 0; index-- {
		err = errors.Join(err, closers[index]())
	}
	return err
}
