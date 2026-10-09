package cache

import "errors"

var (
	ErrMiss       = errors.New("cache: miss")
	ErrFull       = errors.New("cache: capacity reached")
	ErrInvalidTTL = errors.New("cache: invalid TTL")
)
