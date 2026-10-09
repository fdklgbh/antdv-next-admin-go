package health

import (
	"antdv-next-admin-go/internal/infrastructure/logger"
	"antdv-next-admin-go/internal/modules/health/handler"
	"database/sql"
)

type Module struct {
	Handler *handler.Handler
	logger  logger.Logger
}

func NewModule(db *sql.DB, cachePinger handler.CachePinger, log logger.Logger) *Module {
	return &Module{Handler: handler.New(db, cachePinger), logger: log.With(logger.String("module", "health"))}
}
