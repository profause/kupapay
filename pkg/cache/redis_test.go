package cache

import (
	"strings"
	"testing"
)

func TestNormalizeRedisKeyPrefix(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		output string
	}{
		{
			name:   "empty",
			input:  "",
			output: "",
		},
		{
			name:   "trimmed_without_separator",
			input:  "  kupapay  ",
			output: "kupapay:",
		},
		{
			name:   "preserves_existing_separator",
			input:  "cache:",
			output: "cache:",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeRedisKeyPrefix(tt.input); got != tt.output {
				t.Fatalf("expected %q, got %q", tt.output, got)
			}
		})
	}
}

func TestRedisPrefixedKey(t *testing.T) {
	redis := &Redis{keyPrefix: "kupapay:"}

	if got := redis.PrefixedKey("settings:usd"); got != "kupapay:settings:usd" {
		t.Fatalf("expected prefixed key, got %q", got)
	}
}

func TestRedisPrefixedPattern(t *testing.T) {
	tests := []struct {
		name     string
		redis    *Redis
		pattern  string
		expected string
	}{
		{
			name:     "empty_pattern_without_prefix",
			redis:    &Redis{},
			pattern:  "",
			expected: "*",
		},
		{
			name:     "empty_pattern_with_prefix",
			redis:    &Redis{keyPrefix: "kupapay:"},
			pattern:  "",
			expected: "kupapay:*",
		},
		{
			name:     "scoped_pattern_with_prefix",
			redis:    &Redis{keyPrefix: "kupapay:"},
			pattern:  "settings:*",
			expected: "kupapay:settings:*",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.redis.prefixedPattern(tt.pattern); got != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestRedisConfigValidate_StandaloneOK(t *testing.T) {
	cfg := &RedisConfig{
		Host: "localhost",
		Port: 6379,
		DB:   0,
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestRedisConfigValidate_ClusterMissingAddrs(t *testing.T) {
	cfg := &RedisConfig{
		ClusterEnabled: true,
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error, got nil")
	} else if !strings.Contains(err.Error(), "ClusterAddrs must not be empty") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRedisConfigValidate_SentinelMissingFields(t *testing.T) {
	tests := []struct {
		name    string
		cfg     RedisConfig
		wantErr string
	}{
		{
			name: "missing_master",
			cfg: RedisConfig{
				SentinelEnabled: true,
				SentinelAddrs:   []string{"127.0.0.1:26379"},
			},
			wantErr: "SentinelMasterName must not be empty",
		},
		{
			name: "missing_addrs",
			cfg: RedisConfig{
				SentinelEnabled:    true,
				SentinelMasterName: "mymaster",
			},
			wantErr: "SentinelAddrs must not be empty",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.cfg.Validate(); err == nil {
				t.Fatalf("expected error, got nil")
			} else if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestRedisConfigValidate_StandaloneErrors(t *testing.T) {
	tests := []struct {
		name    string
		cfg     RedisConfig
		wantErr string
	}{
		{
			name: "missing_host",
			cfg: RedisConfig{
				Port: 6379,
				DB:   0,
			},
			wantErr: "Host must not be empty",
		},
		{
			name: "invalid_port",
			cfg: RedisConfig{
				Host: "localhost",
				Port: 0,
				DB:   0,
			},
			wantErr: "Port must be between",
		},
		{
			name: "invalid_db_low",
			cfg: RedisConfig{
				Host: "localhost",
				Port: 6379,
				DB:   -1,
			},
			wantErr: "DB must be between",
		},
		{
			name: "invalid_db_high",
			cfg: RedisConfig{
				Host: "localhost",
				Port: 6379,
				DB:   16,
			},
			wantErr: "DB must be between",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.cfg.Validate(); err == nil {
				t.Fatalf("expected error, got nil")
			} else if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}
