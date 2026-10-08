package admin

import "antdv-next-admin-go/internal/modules/admin/handler"

type Module struct{ Handler *handler.Handler }

func NewModule() *Module { return &Module{Handler: handler.New()} }
