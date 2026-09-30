package health

import (
	"database/sql"
	"go-gin-template/internal/infrastructure/logger"
	"go-gin-template/internal/modules/health/handler"
)

type Module struct{ Handler *handler.Handler }

func NewModule(db *sql.DB, redisPinger handler.RedisPinger, log logger.Logger) *Module {
	return &Module{Handler: handler.New(db, redisPinger, log.With(logger.String("module", "health")))}
}
