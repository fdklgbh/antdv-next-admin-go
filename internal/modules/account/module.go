package account

import "antdv-next-admin-go/internal/modules/account/handler"

type Module struct{ Handler *handler.Handler }

func NewModule() *Module { return &Module{Handler: handler.New()} }
