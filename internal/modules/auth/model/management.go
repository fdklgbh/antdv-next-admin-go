package model

import "errors"

var ErrNotFound = errors.New("resource not found")
var ErrInvalid = errors.New("invalid management input")
var ErrConflict = errors.New("resource already exists or is in use")
var ErrProtected = errors.New("protected account or role")
