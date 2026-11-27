package config

import (
	"testing"
	"time"
)

func TestNormalizeJWTExpiration(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		want    time.Duration
		wantErr bool
	}{
		{name: "legacy numeric minutes", value: "60", want: time.Hour},
		{name: "duration string", value: "90m", want: 90 * time.Minute},
		{name: "numeric duration", value: 3600, want: 3600 * time.Minute},
		{name: "invalid duration", value: "one hour", wantErr: true},
		{name: "zero duration", value: "0", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeJWTExpiration(tt.value)
			if tt.wantErr {
				if err == nil {
					t.Fatal("normalizeJWTExpiration() expected an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeJWTExpiration() error = %v", err)
			}
			duration, err := time.ParseDuration(got)
			if err != nil {
				t.Fatalf("normalized duration %q is invalid: %v", got, err)
			}
			if duration != tt.want {
				t.Fatalf("normalizeJWTExpiration() = %s, want %s", duration, tt.want)
			}
		})
	}
}

func TestLoadAcceptsLegacyJWTExpirationMinutes(t *testing.T) {
	t.Setenv("JWT_EXPIRATION", "60")

	cfg := NewService()
	if err := cfg.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got := cfg.Get().JWT.ExpiresIn; got != time.Hour {
		t.Fatalf("JWT.ExpiresIn = %s, want %s", got, time.Hour)
	}
}
