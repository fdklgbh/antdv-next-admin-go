package system

import (
	"antdv-next-admin-go/internal/infrastructure/logger"
	"antdv-next-admin-go/internal/modules/system/handler"
	"antdv-next-admin-go/internal/modules/system/repository"
	"antdv-next-admin-go/internal/modules/system/service"
	"gorm.io/gorm"
)

type Module struct {
	handler *handler.Handler
	logger  logger.Logger
}

func NewModule(db *gorm.DB, log logger.Logger) *Module {
	moduleLogger := log.With(logger.String("module", "system"))
	return &Module{handler: handler.New(service.New(repository.New(db))), logger: moduleLogger}
}
