package validation

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/go-playground/validator/v10"
)

// registerCustomValidators registers all custom validation functions
func registerCustomValidators(v *validator.Validate) {
	// Register each custom validator - ignore errors since these are startup registrations
	_ = v.RegisterValidation("slug", validateSlug)
	_ = v.RegisterValidation("password", validatePassword)
	_ = v.RegisterValidation("phone", validatePhone)
	_ = v.RegisterValidation("nowhitespace", validateNoWhitespace)
	_ = v.RegisterValidation("nohtml", validateNoHTML)
	_ = v.RegisterValidation("required_doc_number", ValidateDocumentNumber)
}

// validateSlug validates a URL slug (lowercase alphanumeric with hyphens)
func validateSlug(fl validator.FieldLevel) bool {
	slug := fl.Field().String()
	if slug == "" {
		return true // empty is valid (use required for mandatory)
	}
	matched, _ := regexp.MatchString(`^[a-z0-9]+(?:-[a-z0-9]+)*$`, slug)
	return matched
}

// validatePassword validates password strength:
// - At least 8 characters
// - At least one uppercase letter
// - At least one lowercase letter
// - At least one digit
// - At least one special character
func validatePassword(fl validator.FieldLevel) bool {
	password := fl.Field().String()
	if len(password) < 8 {
		return false
	}

	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, char := range password {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsDigit(char):
			hasDigit = true
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSpecial = true
		}
	}

	return hasUpper && hasLower && hasDigit && hasSpecial
}

// validatePhone validates international phone number format
// Accepts formats like: +1234567890, +44 20 7946 0958, +234-801-234-5678
func validatePhone(fl validator.FieldLevel) bool {
	phone := fl.Field().String()
	if phone == "" {
		return true
	}

	// Remove spaces, hyphens, and parentheses for validation
	cleaned := regexp.MustCompile(`[\s\-\(\)]+`).ReplaceAllString(phone, "")

	// Must start with + and have 7-15 digits
	matched, _ := regexp.MatchString(`^\+[1-9]\d{6,14}$`, cleaned)
	return matched
}

// validateNoWhitespace ensures a string contains no whitespace
func validateNoWhitespace(fl validator.FieldLevel) bool {
	value := fl.Field().String()
	for _, char := range value {
		if unicode.IsSpace(char) {
			return false
		}
	}
	return true
}

// validateNoHTML ensures a string contains no HTML tags
func validateNoHTML(fl validator.FieldLevel) bool {
	value := fl.Field().String()
	matched, _ := regexp.MatchString(`<[^>]*>`, value)
	return !matched
}

var DocsRequiringNumber = map[string]bool{
	"certificate of incorporation":   true,
	"certificate of registration":    true,
	"tax identification certificate": true,
}

func ValidateDocumentNumber(fl validator.FieldLevel) bool {
	documentNumber := strings.TrimSpace(fl.Field().String())

	parent := fl.Parent()

	nameField := parent.FieldByName("Name")
	if !nameField.IsValid() {
		return false
	}

	docName := strings.ToLower(strings.TrimSpace(nameField.String()))

	// Document number is required only for specific documents
	if _, exists := DocsRequiringNumber[docName]; exists {
		return documentNumber != ""
	}

	return true
}
