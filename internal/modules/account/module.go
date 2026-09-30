package account

import "go-gin-template/internal/modules/account/handler"

type Module struct{ Handler *handler.Handler }

func NewModule() *Module { return &Module{Handler: handler.New()} }
