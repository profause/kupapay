package validation

import (
	"reflect"
	"strings"
	"sync"

	"github.com/go-playground/validator/v10"
)

var (
	once     sync.Once
	instance *Validator
)

// Validator wraps go-playground/validator with custom configuration
type Validator struct {
	validate *validator.Validate
}

// NewValidator creates a new Validator instance (singleton)
func NewValidator() *Validator {
	once.Do(func() {
		v := validator.New(validator.WithRequiredStructEnabled())

		// Use JSON tag names for field names in error messages
		v.RegisterTagNameFunc(func(fld reflect.StructField) string {
			name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
			if name == "-" {
				return fld.Name
			}
			if name == "" {
				return fld.Name
			}
			return name
		})

		// Register custom validators
		registerCustomValidators(v)

		instance = &Validator{validate: v}
	})

	return instance
}

// Validate validates a struct and returns structured errors
func (v *Validator) Validate(s interface{}) error {
	err := v.validate.Struct(s)
	if err == nil {
		return nil
	}

	// Convert validator errors to our structured format
	validatorErrs, ok := err.(validator.ValidationErrors)
	if !ok {
		return err
	}

	return NewValidationErrors(validatorErrs)
}

// ValidateField validates a single field value against a tag
func (v *Validator) ValidateField(field interface{}, tag string) error {
	err := v.validate.Var(field, tag)
	if err == nil {
		return nil
	}

	validatorErrs, ok := err.(validator.ValidationErrors)
	if !ok {
		return err
	}

	return NewValidationErrors(validatorErrs)
}

// ValidatePartial validates only the fields that are set (non-zero)
// Useful for PATCH/update operations
func (v *Validator) ValidatePartial(s interface{}) error {
	err := v.validate.StructExcept(s)
	if err == nil {
		return nil
	}

	validatorErrs, ok := err.(validator.ValidationErrors)
	if !ok {
		return err
	}

	return NewValidationErrors(validatorErrs)
}

// RegisterValidation registers a custom validation function
func (v *Validator) RegisterValidation(tag string, fn validator.Func) error {
	return v.validate.RegisterValidation(tag, fn)
}

// MustRegisterValidation registers a custom validation function and panics on error
func (v *Validator) MustRegisterValidation(tag string, fn validator.Func) {
	if err := v.validate.RegisterValidation(tag, fn); err != nil {
		panic(err)
	}
}

// Engine returns the underlying validator instance for advanced use cases
func (v *Validator) Engine() *validator.Validate {
	return v.validate
}
