package validation

import (
	"testing"
)

// Test structs
type TestUserDTO struct {
	Name  string `json:"name" validate:"required,min=2,max=50"`
	Email string `json:"email" validate:"required,email"`
	Age   int    `json:"age" validate:"gte=0,lte=150"`
}

type TestProductDTO struct {
	Name     string  `json:"name" validate:"required,min=1,max=255"`
	Price    float64 `json:"price" validate:"required,gt=0"`
	SKU      string  `json:"sku" validate:"required,min=1,max=100"`
	Quantity int     `json:"quantity" validate:"gte=0"`
	Category string  `json:"category" validate:"omitempty,oneof=electronics clothing food"`
}

type TestNestedDTO struct {
	User    TestUserDTO `json:"user" validate:"required"`
	Product string      `json:"product" validate:"required"`
}

type TestOptionalDTO struct {
	Name  string `json:"name" validate:"omitempty,min=2"`
	Email string `json:"email" validate:"omitempty,email"`
}

type TestCustomDTO struct {
	Slug     string `json:"slug" validate:"slug"`
	Phone    string `json:"phone" validate:"phone"`
	Currency string `json:"currency" validate:"currency"`
}

func TestNewValidator(t *testing.T) {
	v := NewValidator()
	if v == nil {
		t.Fatal("NewValidator returned nil")
	}

	// Test singleton
	v2 := NewValidator()
	if v != v2 {
		t.Error("NewValidator should return the same instance")
	}
}

func TestValidate_RequiredFields(t *testing.T) {
	v := NewValidator()

	tests := []struct {
		name      string
		dto       interface{}
		expectErr bool
		errField  string
	}{
		{
			name:      "valid user",
			dto:       TestUserDTO{Name: "John", Email: "john@example.com", Age: 25},
			expectErr: false,
		},
		{
			name:      "missing name",
			dto:       TestUserDTO{Email: "john@example.com", Age: 25},
			expectErr: true,
			errField:  "name",
		},
		{
			name:      "missing email",
			dto:       TestUserDTO{Name: "John", Age: 25},
			expectErr: true,
			errField:  "email",
		},
		{
			name:      "empty struct",
			dto:       TestUserDTO{},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.Validate(tt.dto)
			if tt.expectErr && err == nil {
				t.Error("expected error but got nil")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("expected no error but got: %v", err)
			}
			if tt.errField != "" && err != nil {
				ve, ok := AsValidationErrors(err)
				if !ok {
					t.Fatal("error is not ValidationErrors")
				}
				found := false
				for _, e := range ve.Errors {
					if e.Field == tt.errField {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected error for field %s, but not found in %v", tt.errField, ve.Fields())
				}
			}
		})
	}
}

func TestValidate_EmailFormat(t *testing.T) {
	v := NewValidator()

	tests := []struct {
		email     string
		expectErr bool
	}{
		{"valid@example.com", false},
		{"user.name@domain.co.uk", false},
		{"user+tag@example.com", false},
		{"invalid", true},
		{"@example.com", true},
		{"user@", true},
		{"user@.com", true},
	}

	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			dto := TestUserDTO{Name: "Test", Email: tt.email, Age: 25}
			err := v.Validate(dto)
			if tt.expectErr && err == nil {
				t.Errorf("expected error for email %q", tt.email)
			}
			if !tt.expectErr && err != nil {
				t.Errorf("did not expect error for email %q, got: %v", tt.email, err)
			}
		})
	}
}

func TestValidate_NumericBounds(t *testing.T) {
	v := NewValidator()

	tests := []struct {
		age       int
		expectErr bool
	}{
		{0, false},
		{25, false},
		{150, false},
		{-1, true},
		{151, true},
	}

	for _, tt := range tests {
		dto := TestUserDTO{Name: "Test", Email: "test@example.com", Age: tt.age}
		err := v.Validate(dto)
		if tt.expectErr && err == nil {
			t.Errorf("expected error for age %d", tt.age)
		}
		if !tt.expectErr && err != nil {
			t.Errorf("did not expect error for age %d, got: %v", tt.age, err)
		}
	}
}

func TestValidate_StringLength(t *testing.T) {
	v := NewValidator()

	tests := []struct {
		name      string
		value     string
		expectErr bool
	}{
		{"min length ok", "AB", false},
		{"exact min", "AB", false},
		{"too short", "A", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dto := TestUserDTO{Name: tt.value, Email: "test@example.com", Age: 25}
			err := v.Validate(dto)
			if tt.expectErr && err == nil {
				t.Errorf("expected error for name %q", tt.value)
			}
			if !tt.expectErr && err != nil {
				t.Errorf("did not expect error for name %q, got: %v", tt.value, err)
			}
		})
	}
}

func TestValidate_Oneof(t *testing.T) {
	v := NewValidator()

	tests := []struct {
		category  string
		expectErr bool
	}{
		{"electronics", false},
		{"clothing", false},
		{"food", false},
		{"", false}, // omitempty
		{"invalid", true},
		{"Electronics", true}, // case sensitive
	}

	for _, tt := range tests {
		t.Run(tt.category, func(t *testing.T) {
			dto := TestProductDTO{
				Name:     "Test Product",
				Price:    9.99,
				SKU:      "TEST-001",
				Category: tt.category,
			}
			err := v.Validate(dto)
			if tt.expectErr && err == nil {
				t.Errorf("expected error for category %q", tt.category)
			}
			if !tt.expectErr && err != nil {
				t.Errorf("did not expect error for category %q, got: %v", tt.category, err)
			}
		})
	}
}

func TestValidate_NestedStruct(t *testing.T) {
	v := NewValidator()

	tests := []struct {
		name      string
		dto       TestNestedDTO
		expectErr bool
		errField  string
	}{
		{
			name: "valid nested",
			dto: TestNestedDTO{
				User:    TestUserDTO{Name: "John", Email: "john@example.com", Age: 25},
				Product: "Widget",
			},
			expectErr: false,
		},
		{
			name: "invalid nested user email",
			dto: TestNestedDTO{
				User:    TestUserDTO{Name: "John", Email: "invalid", Age: 25},
				Product: "Widget",
			},
			expectErr: true,
			errField:  "user.email",
		},
		{
			name: "missing nested user name",
			dto: TestNestedDTO{
				User:    TestUserDTO{Email: "john@example.com", Age: 25},
				Product: "Widget",
			},
			expectErr: true,
			errField:  "user.name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.Validate(tt.dto)
			if tt.expectErr && err == nil {
				t.Error("expected error but got nil")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("expected no error but got: %v", err)
			}
			if tt.errField != "" && err != nil {
				ve, ok := AsValidationErrors(err)
				if !ok {
					t.Fatal("error is not ValidationErrors")
				}
				found := false
				for _, e := range ve.Errors {
					if e.Field == tt.errField {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected error for field %s, but not found in %v", tt.errField, ve.Fields())
				}
			}
		})
	}
}

func TestValidationError_Error(t *testing.T) {
	v := NewValidator()

	// Single error
	dto := TestUserDTO{Name: "A", Email: "test@example.com", Age: 25}
	err := v.Validate(dto)
	if err == nil {
		t.Fatal("expected error")
	}

	errStr := err.Error()
	if errStr == "" {
		t.Error("error string should not be empty")
	}

	// Multiple errors
	dto2 := TestUserDTO{Age: -1}
	err2 := v.Validate(dto2)
	if err2 == nil {
		t.Fatal("expected error")
	}

	ve, ok := AsValidationErrors(err2)
	if !ok {
		t.Fatal("error is not ValidationErrors")
	}

	if len(ve.Errors) < 2 {
		t.Errorf("expected multiple errors, got %d", len(ve.Errors))
	}
}

func TestValidationErrors_ByField(t *testing.T) {
	v := NewValidator()

	dto := TestUserDTO{Age: -1} // Missing name, email, and invalid age
	err := v.Validate(dto)
	if err == nil {
		t.Fatal("expected error")
	}

	ve, ok := AsValidationErrors(err)
	if !ok {
		t.Fatal("error is not ValidationErrors")
	}

	nameErrors := ve.ByField("name")
	if len(nameErrors) == 0 {
		t.Error("expected errors for 'name' field")
	}

	unknownErrors := ve.ByField("unknown")
	if len(unknownErrors) != 0 {
		t.Error("expected no errors for 'unknown' field")
	}
}

func TestValidateField(t *testing.T) {
	v := NewValidator()

	// Valid email
	err := v.ValidateField("test@example.com", "email")
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}

	// Invalid email
	err = v.ValidateField("invalid", "email")
	if err == nil {
		t.Error("expected error for invalid email")
	}

	// UUID validation
	err = v.ValidateField("550e8400-e29b-41d4-a716-446655440000", "uuid")
	if err != nil {
		t.Errorf("expected no error for valid UUID, got: %v", err)
	}

	err = v.ValidateField("not-a-uuid", "uuid")
	if err == nil {
		t.Error("expected error for invalid UUID")
	}
}

func TestCustomValidators_Slug(t *testing.T) {
	v := NewValidator()

	tests := []struct {
		slug      string
		expectErr bool
	}{
		{"valid-slug", false},
		{"another-valid-slug-123", false},
		{"simple", false},
		{"", false}, // empty is valid (not required)
		{"Invalid-Slug", true},
		{"invalid slug", true},
		{"invalid_slug", true},
		{"-invalid", true},
		{"invalid-", true},
	}

	for _, tt := range tests {
		t.Run(tt.slug, func(t *testing.T) {
			dto := TestCustomDTO{Slug: tt.slug}
			err := v.Validate(dto)
			if tt.expectErr && err == nil {
				t.Errorf("expected error for slug %q", tt.slug)
			}
			if !tt.expectErr && err != nil {
				t.Errorf("did not expect error for slug %q, got: %v", tt.slug, err)
			}
		})
	}
}

func TestCustomValidators_Phone(t *testing.T) {
	v := NewValidator()

	tests := []struct {
		phone     string
		expectErr bool
	}{
		{"+1234567890", false},
		{"+44 20 7946 0958", false},
		{"+234-801-234-5678", false},
		{"+1 (555) 123-4567", false},
		{"", false}, // empty is valid
		{"1234567890", true},
		{"+123", true}, // too short
		{"not-a-phone", true},
	}

	for _, tt := range tests {
		t.Run(tt.phone, func(t *testing.T) {
			dto := TestCustomDTO{Phone: tt.phone}
			err := v.Validate(dto)
			if tt.expectErr && err == nil {
				t.Errorf("expected error for phone %q", tt.phone)
			}
			if !tt.expectErr && err != nil {
				t.Errorf("did not expect error for phone %q, got: %v", tt.phone, err)
			}
		})
	}
}

func TestCustomValidators_Currency(t *testing.T) {
	v := NewValidator()

	tests := []struct {
		currency  string
		expectErr bool
	}{
		{"USD", false},
		{"EUR", false},
		{"NGN", false},
		{"USDT", false},
		{"USDC", false},
		{"", false}, // empty is valid
		{"INVALID", true},
		// USDT/USDC are the only supported crypto; BTC and ETH were removed.
		{"BTC", true},
		{"ETH", true},
		{"usd", false}, // pkg/currency parses case-insensitively
		{"US", true},
	}

	for _, tt := range tests {
		t.Run(tt.currency, func(t *testing.T) {
			dto := TestCustomDTO{Currency: tt.currency}
			err := v.Validate(dto)
			if tt.expectErr && err == nil {
				t.Errorf("expected error for currency %q", tt.currency)
			}
			if !tt.expectErr && err != nil {
				t.Errorf("did not expect error for currency %q, got: %v", tt.currency, err)
			}
		})
	}
}

func TestIsValidationError(t *testing.T) {
	v := NewValidator()

	err := v.Validate(TestUserDTO{})
	if !IsValidationError(err) {
		t.Error("expected IsValidationError to return true")
	}
}

func TestHumanReadableMessages(t *testing.T) {
	v := NewValidator()

	dto := TestUserDTO{Name: "A", Email: "invalid", Age: -1}
	err := v.Validate(dto)
	if err == nil {
		t.Fatal("expected error")
	}

	ve, _ := AsValidationErrors(err)

	// Check that messages are human-readable (not just tag names)
	for _, e := range ve.Errors {
		if e.Message == "" {
			t.Errorf("error message should not be empty for field %s", e.Field)
		}
		// Message should be more descriptive than just the tag
		if e.Message == e.Tag {
			t.Errorf("error message should be human-readable, not just the tag: %s", e.Message)
		}
	}
}
