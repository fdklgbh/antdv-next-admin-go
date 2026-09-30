package admin

import "go-gin-template/internal/modules/admin/handler"

type Module struct{ Handler *handler.Handler }

func NewModule() *Module { return &Module{Handler: handler.New()} }
