package health

import (
	"database/sql"

	"antdv-next-admin-go/internal/infrastructure/logger"
	"antdv-next-admin-go/internal/modules/health/handler"
)

type Module struct {
	Handler *handler.Handler
	logger  logger.Logger
}

func NewModule(db *sql.DB, cachePinger handler.CachePinger, log logger.Logger) *Module {
	return &Module{Handler: handler.New(db, cachePinger), logger: log.With(logger.String("module", "health"))}
}
