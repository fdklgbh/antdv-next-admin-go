package validator

import validatorlib "github.com/go-playground/validator/v10"

type FieldError struct {
	Field string `json:"field"`
	Rule  string `json:"rule"`
}
type Validator struct{ validate *validatorlib.Validate }

func New() *Validator {
	return &Validator{validate: validatorlib.New(validatorlib.WithRequiredStructEnabled())}
}
func (v *Validator) Struct(value any) error { return v.validate.Struct(value) }
func Fields(err error) []FieldError {
	validationErrors, ok := err.(validatorlib.ValidationErrors)
	if !ok {
		return nil
	}
	fields := make([]FieldError, 0, len(validationErrors))
	for _, field := range validationErrors {
		fields = append(fields, FieldError{Field: field.Field(), Rule: field.Tag()})
	}
	return fields
}
