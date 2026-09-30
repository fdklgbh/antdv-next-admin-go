package errors

import "fmt"

type AppError struct {
	Code       int
	HTTPStatus int
	Message    string
	Err        error
}

func (e *AppError) Error() string {
	if e.Err == nil {
		return e.Message
	}
	return fmt.Sprintf("%s: %v", e.Message, e.Err)
}
func (e *AppError) Unwrap() error { return e.Err }
func New(code, status int, message string, err error) *AppError {
	return &AppError{Code: code, HTTPStatus: status, Message: message, Err: err}
}
