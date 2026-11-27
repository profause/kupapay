package redact

import (
	"encoding/json"
	"reflect"
	"strings"
	"unicode"
)

const Placeholder = "[REDACTED]"

var sensitiveExactKeys = map[string]struct{}{
	"accesskey":          {},
	"accesstoken":        {},
	"apikey":             {},
	"apisecret":          {},
	"authorization":      {},
	"authheader":         {},
	"bearer":             {},
	"bearertoken":        {},
	"clientsecret":       {},
	"consumerkey":        {},
	"consumersecret":     {},
	"credential":         {},
	"credentials":        {},
	"idtoken":            {},
	"jwt":                {},
	"password":           {},
	"passcode":           {},
	"passwd":             {},
	"privatekey":         {},
	"proxyauthorization": {},
	"pwd":                {},
	"refreshtoken":       {},
	"secret":             {},
	"secretid":           {},
	"secretkey":          {},
	"securitycredential": {},
	"signature":          {},
	"token":              {},
}

var sensitiveSuffixes = []string{
	"accesskey",
	"accesstoken",
	"apikey",
	"apisecret",
	"authorization",
	"bearertoken",
	"clientsecret",
	"consumerkey",
	"consumersecret",
	"credential",
	"credentials",
	"idtoken",
	"password",
	"passcode",
	"passwd",
	"privatekey",
	"refreshtoken",
	"secret",
	"secretid",
	"secretkey",
	"securitycredential",
	"signature",
	"token",
}

var safeExactKeys = map[string]struct{}{
	"publickey": {},
}

// Field redacts a value using the field key as a sensitivity hint.
func Field(key string, value any) any {
	return sanitize(key, value)
}

// Value redacts a value recursively without a top-level key hint.
func Value(value any) any {
	return sanitize("", value)
}

// Map returns a redacted clone of the provided map.
func Map(value map[string]any) map[string]any {
	if value == nil {
		return nil
	}
	redacted, _ := sanitize("", value).(map[string]any)
	return redacted
}

// IsSensitiveKey reports whether a field key should be redacted.
func IsSensitiveKey(key string) bool {
	normalized := normalizeKey(key)
	if normalized == "" {
		return false
	}
	if _, ok := safeExactKeys[normalized]; ok {
		return false
	}
	if _, ok := sensitiveExactKeys[normalized]; ok {
		return true
	}
	for _, suffix := range sensitiveSuffixes {
		if strings.HasSuffix(normalized, suffix) {
			return true
		}
	}
	return false
}

func sanitize(key string, value any) any {
	if value == nil {
		return nil
	}
	if IsSensitiveKey(key) {
		return Placeholder
	}

	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for childKey, childValue := range typed {
			out[childKey] = sanitize(childKey, childValue)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = sanitize("", item)
		}
		return out
	case json.RawMessage:
		if len(typed) == 0 {
			return ""
		}
		var parsed any
		if err := json.Unmarshal(typed, &parsed); err == nil {
			return sanitize("", parsed)
		}
		return string(typed)
	}

	return sanitizeReflect(key, reflect.ValueOf(value))
}

func sanitizeReflect(key string, value reflect.Value) any {
	if !value.IsValid() {
		return nil
	}
	if IsSensitiveKey(key) {
		return Placeholder
	}

	switch value.Kind() {
	case reflect.Interface, reflect.Pointer:
		if value.IsNil() {
			return nil
		}
		return sanitize(key, value.Elem().Interface())
	case reflect.Map:
		if value.Type().Key().Kind() != reflect.String {
			return value.Interface()
		}
		out := make(map[string]any, value.Len())
		iter := value.MapRange()
		for iter.Next() {
			childKey := iter.Key().String()
			out[childKey] = sanitize(childKey, iter.Value().Interface())
		}
		return out
	case reflect.Slice, reflect.Array:
		if value.Type().Elem().Kind() == reflect.Uint8 {
			bytes := make([]byte, value.Len())
			reflect.Copy(reflect.ValueOf(bytes), value)
			return bytes
		}
		out := make([]any, value.Len())
		for i := 0; i < value.Len(); i++ {
			out[i] = sanitize("", value.Index(i).Interface())
		}
		return out
	case reflect.Struct:
		out := make(map[string]any, value.NumField())
		typ := value.Type()
		for i := 0; i < value.NumField(); i++ {
			field := typ.Field(i)
			if field.PkgPath != "" {
				continue
			}
			name := fieldJSONName(field)
			if name == "-" {
				continue
			}
			if name == "" {
				name = field.Name
			}
			out[name] = sanitize(name, value.Field(i).Interface())
		}
		return out
	default:
		return value.Interface()
	}
}

func fieldJSONName(field reflect.StructField) string {
	tag := strings.TrimSpace(field.Tag.Get("json"))
	if tag == "" {
		return ""
	}
	name := strings.Split(tag, ",")[0]
	return strings.TrimSpace(name)
}

func normalizeKey(key string) string {
	if key == "" {
		return ""
	}

	var builder strings.Builder
	builder.Grow(len(key))
	for _, r := range key {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(unicode.ToLower(r))
		}
	}
	return builder.String()
}
