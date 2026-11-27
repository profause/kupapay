package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Environment string

const (
	ProductionEnvironment  Environment = "production"
	TestEnvironment        Environment = "test"
	StagingEnvironment     Environment = "staging"
	DevelopmentEnvironment Environment = "development"
	// SandboxEnvironment is the merchant/partner integration tier — see
	// k8s/sandbox. It was already being set via APP_ENV=sandbox without a
	// matching constant here, so nothing could branch on it by type.
	SandboxEnvironment Environment = "sandbox"
)

// Config holds all configuration for the application
type Config struct {
	App       AppConfig
	JWT       JWTConfig
	Database  DatabaseConfig
	Redis     RedisConfig
	Cache     CacheConfig
	Logger    LoggerConfig
	CORS      CORSConfig
	Queue     QueueConfig
	Passkey   PasskeyConfig
	RateLimit RateLimitConfig
}

// AppConfig holds application configuration
type AppConfig struct {
	Name              string
	Env               Environment
	Port              int
	Host              string
	APIPrefix         string
	APIVersion        string
	SecretKey         string
	BaseURL           string // Public base URL of this service used to build provider callback URLs (e.g. https://atlas.example.com)
	DashboardURL      string
	AdminDashboardURL string
}

// Validate validates the application configuration
func (c *AppConfig) Validate() error {
	// Validate Name
	if c.Name == "" {
		return fmt.Errorf("name must not be empty")
	}

	// Validate Env
	validEnvs := []Environment{ProductionEnvironment, TestEnvironment, StagingEnvironment, DevelopmentEnvironment, SandboxEnvironment}
	if !slices.Contains(validEnvs, c.Env) {
		return fmt.Errorf("environment variable %s is not valid", c.Env)
	}

	// Validate Port
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535 (got %d)", c.Port)
	}

	// Validate Host
	if c.Host == "" {
		return fmt.Errorf("host must not be empty")
	}

	// Validate APIPrefix (should start with /)
	if c.APIPrefix != "" && c.APIPrefix[0] != '/' {
		return fmt.Errorf("APIPrefix must start with '/' (got %s)", c.APIPrefix)
	}

	// Validate SecretKey
	if c.SecretKey == "" {
		return fmt.Errorf("SecretKey must not be empty")
	}

	return nil
}

func (c *AppConfig) IsProduction() bool {
	return c.Env == ProductionEnvironment
}

// IsSandbox returns true if environment is the merchant/partner sandbox. It
// is not the complement of IsProduction(): !IsProduction() also passes for
// staging and development. Code that must never call a real provider (as
// opposed to code that merely relaxes a limit outside production) should gate
// on this instead.
func (c *AppConfig) IsSandbox() bool {
	return c.Env == SandboxEnvironment
}

type JWTConfig struct {
	Secret           string
	ExpiresIn        time.Duration
	RefreshSecret    string
	RefreshExpiresIn string
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	WriteHost string // Primary database host for write operations
	ReadHost  string // Read replica host for read operations
	Port      int
	Name      string
	User      string
	Password  string
	SSLMode   string
}

// Validate validates the database configuration
func (c *DatabaseConfig) Validate() error {
	// If neither are set, fall back to WriteHost for ReadHost
	if c.ReadHost == "" && c.WriteHost != "" {
		c.ReadHost = c.WriteHost
	}

	// Validate WriteHost (primary requirement)
	if c.WriteHost == "" {
		return fmt.Errorf("WriteHost must not be empty (or set DATABASE_HOST for backward compatibility)")
	}

	// Validate ReadHost
	if c.ReadHost == "" {
		return fmt.Errorf("ReadHost must not be empty")
	}

	// Validate Port
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535 (got %d)", c.Port)
	}

	// Validate Name
	if c.Name == "" {
		return fmt.Errorf("name (database name) must not be empty")
	}

	// Validate User
	if c.User == "" {
		return fmt.Errorf("user must not be empty")
	}

	// Validate SSLMode
	validSSLModes := []string{"disable", "require", "verify-ca", "verify-full"}
	validMode := false
	for _, mode := range validSSLModes {
		if c.SSLMode == mode {
			validMode = true
			break
		}
	}
	if !validMode {
		return fmt.Errorf("SSLMode must be one of: %v (got %s)", validSSLModes, c.SSLMode)
	}

	return nil
}

// RedisConfig holds Redis configuration
type RedisConfig struct {
	Host      string // Redis host for standalone mode
	Port      int    // Redis port for standalone mode
	Username  string // Redis username (optional)
	Password  string // Redis password
	DB        int    // Redis database number (standalone only, ignored in cluster)
	KeyPrefix string // Global prefix applied to Redis cache keys

	// Cluster mode configuration
	ClusterEnabled bool     // Enable cluster mode
	TLSEnabled     bool     // Enable TLS for cluster connections
	ClusterAddrs   []string // Cluster node addresses (e.g., ["node1:6379", "node2:6379"])

	// Sentinel mode configuration
	SentinelEnabled    bool     // Enable sentinel mode
	SentinelMasterName string   // Name of the master set
	SentinelAddrs      []string // Sentinel node addresses
}

// RateLimitConfig configures the pkg/ratelimit middleware. The global block is
// the IP-keyed default applied to every /internal/v2 and /api/v2 route.
// Per-endpoint policies (login, register, etc.) are defined in code, not config.
type RateLimitConfig struct {
	// Enabled toggles the global middleware. When false, the global handler
	// is a no-op (per-endpoint policies still apply where wired explicitly).
	Enabled bool
	// GlobalRate is the request budget allowed inside GlobalPeriod, keyed by
	// client IP. 0 disables the global limit but leaves Enabled honored for
	// per-endpoint use.
	GlobalRate int
	// GlobalPeriod is parsed as a Go duration ("1m", "10s"). Falls back to 1m.
	GlobalPeriod string
	// KeyPrefix is the cache-key namespace for rate-limit buckets. Empty
	// uses pkg/ratelimit.DefaultKeyPrefix ("rate_limit:").
	KeyPrefix string
}

// CacheConfig holds cache configuration
type CacheConfig struct {
	Type                string // redis, memory, or hybrid
	UseFallback         bool   // Use memory cache as fallback when Redis fails
	MemoryMaxSize       int    // Max items in memory cache
	MemoryMaxBytes      int64  // Max memory usage in bytes (0 = unlimited)
	MemoryCleanupSec    int    // Memory cache cleanup interval in seconds
	MemoryDefaultTTLSec int    // Default TTL for memory cache items in seconds
}

// LoggerConfig holds logger configuration
type LoggerConfig struct {
	Level             string
	Format            string
	Output            string
	Caller            bool // include caller file:line in every log entry
	RotateEnabled     bool
	RotateMaxSizeMB   int
	RotateMaxAgeDays  int
	RotateMaxBackups  int
	RotateCompressLog bool
}

// CORSConfig holds CORS configuration
type CORSConfig struct {
	Origin      string
	Credentials bool
}

// QueueConfig holds queue configuration
type QueueConfig struct {
	Backend     string // sqs or go
	RedisAddr   string
	Concurrency int
}

type PasskeyConfig struct {
	Admin  PasskeyAppConfig
	Client PasskeyAppConfig
}

// PasskeyAppConfig is one relying-party profile (one frontend app).
type PasskeyAppConfig struct {
	RPID        string
	DisplayName string
	Origins     []string
}

// Configured reports whether this profile has been set up (either field present).
func (c PasskeyAppConfig) Configured() bool {
	return strings.TrimSpace(c.RPID) != "" || len(c.Origins) > 0
}

type Service interface {
	Get() *Config
	Load() error
}

// Service provides configuration management
type service struct {
	config *Config
}

// NewService creates a new config service
func NewService() Service {
	return &service{}
}

// Load loads configuration from .env file and environment variables
func (s *service) Load() error {
	// Load .env file first, overriding any existing environment variables
	// Use Overload() instead of Load() to ensure .env values take precedence
	_ = godotenv.Overload()

	v := viper.New()

	// Set defaults first
	s.setDefaults(v)

	// Enable environment variables (will override defaults)
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Explicit bindings for app
	if err := v.BindEnv("app.name", "APP_NAME"); err != nil {
		return fmt.Errorf("error binding app.name env var: %w", err)
	}
	if err := v.BindEnv("app.env", "APP_ENV"); err != nil {
		return fmt.Errorf("error binding app.env env var: %w", err)
	}
	if err := v.BindEnv("app.port", "APP_PORT"); err != nil {
		return fmt.Errorf("error binding app.port env var: %w", err)
	}
	if err := v.BindEnv("app.host", "APP_HOST"); err != nil {
		return fmt.Errorf("error binding app.host env var: %w", err)
	}
	if err := v.BindEnv("app.apiprefix", "APP_APIPREFIX"); err != nil {
		return fmt.Errorf("error binding app.apiprefix env var: %w", err)
	}
	if err := v.BindEnv("app.apiversion", "APP_APIVERSION"); err != nil {
		return fmt.Errorf("error binding app.apiversion env var: %w", err)
	}

	if err := v.BindEnv("app.secretkey", "APP_SECRET", "APP_SECRETKEY", "APP_SECRET_KEY"); err != nil {
		return fmt.Errorf("error binding app.secretkey env var: %w", err)
	}
	if err := v.BindEnv("app.baseurl", "APP_BASE_URL"); err != nil {
		return fmt.Errorf("error binding app.baseurl env var: %w", err)
	}

	if err := v.BindEnv("app.dashboardurl", "APP_DASHBOARD_URL", "DASHBOARD_URL"); err != nil {
		return fmt.Errorf("error binding app.dashboardurl env var: %w", err)
	}
	if err := v.BindEnv("app.admindashboardurl", "APP_ADMIN_DASHBOARD_URL", "ADMIN_DASHBOARD_URL"); err != nil {
		return fmt.Errorf("error binding app.admindashboardurl env var: %w", err)
	}

	// Explicit bindings for JWT. JWT_SECRET_KEY is the legacy Nest variable;
	// JWT_SECRET is accepted for the existing Atlas config file.
	if err := v.BindEnv("jwt.secret", "JWT_SECRET", "JWT_SECRET"); err != nil {
		return fmt.Errorf("error binding jwt.secret env var: %w", err)
	}
	if err := v.BindEnv("jwt.expiresin", "JWT_EXPIRATION", "JWT_EXPIRESIN"); err != nil {
		return fmt.Errorf("error binding jwt.expiresin env var: %w", err)
	}

	// Explicit bindings for database
	if err := v.BindEnv("database.host", "DB_HOST"); err != nil {
		return fmt.Errorf("error binding database.host env var: %w", err)
	}
	if err := v.BindEnv("database.writehost", "DB_WRITE_HOST"); err != nil {
		return fmt.Errorf("error binding database.writehost env var: %w", err)
	}
	if err := v.BindEnv("database.readhost", "DB_READ_HOST"); err != nil {
		return fmt.Errorf("error binding database.readhost env var: %w", err)
	}
	if err := v.BindEnv("database.port", "DB_PORT"); err != nil {
		return fmt.Errorf("error binding database.port env var: %w", err)
	}
	if err := v.BindEnv("database.name", "DB_NAME"); err != nil {
		return fmt.Errorf("error binding database.name env var: %w", err)
	}
	if err := v.BindEnv("database.user", "DB_USER"); err != nil {
		return fmt.Errorf("error binding database.user env var: %w", err)
	}
	if err := v.BindEnv("database.password", "DB_PASSWORD"); err != nil {
		return fmt.Errorf("error binding database.password env var: %w", err)
	}
	if err := v.BindEnv("database.sslmode", "DB_SSLMODE"); err != nil {
		return fmt.Errorf("error binding database.sslmode env var: %w", err)
	}

	// Explicit bindings for Redis
	if err := v.BindEnv("redis.host", "REDIS_HOST"); err != nil {
		return fmt.Errorf("error binding redis.host env var: %w", err)
	}
	if err := v.BindEnv("redis.port", "REDIS_PORT"); err != nil {
		return fmt.Errorf("error binding redis.port env var: %w", err)
	}
	if err := v.BindEnv("redis.username", "REDIS_USERNAME"); err != nil {
		return fmt.Errorf("error binding redis.username env var: %w", err)
	}
	if err := v.BindEnv("redis.password", "REDIS_PASSWORD"); err != nil {
		return fmt.Errorf("error binding redis.password env var: %w", err)
	}
	if err := v.BindEnv("redis.db", "REDIS_DB"); err != nil {
		return fmt.Errorf("error binding redis.db env var: %w", err)
	}
	if err := v.BindEnv("redis.keyprefix", "REDIS_KEY_PREFIX"); err != nil {
		return fmt.Errorf("error binding redis.keyprefix env var: %w", err)
	}
	if err := v.BindEnv("redis.clusterenabled", "REDIS_CLUSTER_ENABLED"); err != nil {
		return fmt.Errorf("error binding redis.clusterenabled env var: %w", err)
	}
	if err := v.BindEnv("redis.tlsenabled", "REDIS_TLS_ENABLED"); err != nil {
		return fmt.Errorf("error binding redis.tlsenabled env var: %w", err)
	}
	if err := v.BindEnv("redis.clusteraddrs", "REDIS_CLUSTER_ADDRS"); err != nil {
		return fmt.Errorf("error binding redis.clusteraddrs env var: %w", err)
	}
	if err := v.BindEnv("redis.sentinelenabled", "REDIS_SENTINEL_ENABLED"); err != nil {
		return fmt.Errorf("error binding redis.sentinelenabled env var: %w", err)
	}
	if err := v.BindEnv("redis.sentinelmastername", "REDIS_SENTINEL_MASTER_NAME"); err != nil {
		return fmt.Errorf("error binding redis.sentinelmastername env var: %w", err)
	}
	if err := v.BindEnv("redis.sentineladdrs", "REDIS_SENTINEL_ADDRS"); err != nil {
		return fmt.Errorf("error binding redis.sentineladdrs env var: %w", err)
	}

	// Explicit bindings for logger
	if err := v.BindEnv("logger.level", "LOGGER_LEVEL"); err != nil {
		return fmt.Errorf("error binding logger.level env var: %w", err)
	}
	if err := v.BindEnv("logger.format", "LOGGER_FORMAT"); err != nil {
		return fmt.Errorf("error binding logger.format env var: %w", err)
	}
	if err := v.BindEnv("logger.output", "LOGGER_OUTPUT"); err != nil {
		return fmt.Errorf("error binding logger.output env var: %w", err)
	}
	if err := v.BindEnv("logger.rotateenabled", "LOGGER_ROTATE_ENABLED"); err != nil {
		return fmt.Errorf("error binding logger.rotateenabled env var: %w", err)
	}
	if err := v.BindEnv("logger.rotatemaxsizemb", "LOGGER_ROTATE_MAX_SIZE_MB"); err != nil {
		return fmt.Errorf("error binding logger.rotatemaxsizemb env var: %w", err)
	}
	if err := v.BindEnv("logger.rotatemaxagedays", "LOGGER_ROTATE_MAX_AGE_DAYS"); err != nil {
		return fmt.Errorf("error binding logger.rotatemaxagedays env var: %w", err)
	}
	if err := v.BindEnv("logger.rotatemaxbackups", "LOGGER_ROTATE_MAX_BACKUPS"); err != nil {
		return fmt.Errorf("error binding logger.rotatemaxbackups env var: %w", err)
	}
	if err := v.BindEnv("logger.rotatecompresslog", "LOGGER_ROTATE_COMPRESS"); err != nil {
		return fmt.Errorf("error binding logger.rotatecompresslog env var: %w", err)
	}
	if err := v.BindEnv("logger.caller", "LOGGER_CALLER"); err != nil {
		return fmt.Errorf("error binding logger.caller env var: %w", err)
	}

	// Explicit bindings for CORS
	if err := v.BindEnv("cors.origin", "CORS_ORIGIN"); err != nil {
		return fmt.Errorf("error binding cors.origin env var: %w", err)
	}
	if err := v.BindEnv("cors.credentials", "CORS_CREDENTIALS"); err != nil {
		return fmt.Errorf("error binding cors.credentials env var: %w", err)
	}

	// Explicit bindings for queue
	if err := v.BindEnv("queue.backend", "QUEUE_BACKEND"); err != nil {
		return fmt.Errorf("error binding queue.backend env var: %w", err)
	}
	if err := v.BindEnv("queue.redisaddr", "QUEUE_REDIS_ADDR"); err != nil {
		return fmt.Errorf("error binding queue.redisaddr env var: %w", err)
	}
	if err := v.BindEnv("queue.concurrency", "QUEUE_CONCURRENCY"); err != nil {
		return fmt.Errorf("error binding queue.concurrency env var: %w", err)
	}

	// Optionally merge config.yaml if it exists (env vars can be referenced using ${VAR:-default})
	v.SetConfigType("yaml")
	if err := s.mergeConfigFile(v); err != nil {
		return err
	}

	jwtExpiresIn, err := normalizeJWTExpiration(v.Get("jwt.expiresin"))
	if err != nil {
		return err
	}
	v.Set("jwt.expiresin", jwtExpiresIn)

	// Unmarshal config
	s.config = &Config{}
	if err := v.Unmarshal(s.config); err != nil {
		return fmt.Errorf("error unmarshaling config: %w", err)
	}
	s.normalizeAuthConfig(v)

	// Parse Redis cluster addresses from comma-separated string
	if clusterAddrsStr := v.GetString("redis.clusteraddrs"); clusterAddrsStr != "" {
		clusterAddrs := strings.Split(clusterAddrsStr, ",")
		trimmedAddrs := make([]string, 0, len(clusterAddrs))
		for _, addr := range clusterAddrs {
			if trimmed := strings.TrimSpace(addr); trimmed != "" {
				trimmedAddrs = append(trimmedAddrs, trimmed)
			}
		}
		s.config.Redis.ClusterAddrs = trimmedAddrs
	}

	// Parse Redis sentinel addresses from comma-separated string
	if sentinelAddrsStr := v.GetString("redis.sentineladdrs"); sentinelAddrsStr != "" {
		sentinelAddrs := strings.Split(sentinelAddrsStr, ",")
		trimmedAddrs := make([]string, 0, len(sentinelAddrs))
		for _, addr := range sentinelAddrs {
			if trimmed := strings.TrimSpace(addr); trimmed != "" {
				trimmedAddrs = append(trimmedAddrs, trimmed)
			}
		}
		s.config.Redis.SentinelAddrs = trimmedAddrs
	}

	return nil
}

var envPlaceholderRegexp = regexp.MustCompile(`\$\{([A-Z0-9_]+)(?::-(.*?))?\}`)

// mergeConfigFile loads config.yaml (if present), expanding ${VAR:-default} placeholders with environment values.
func (s *service) mergeConfigFile(v *viper.Viper) error {
	configPaths := []string{
		"config.yaml",
		filepath.Join("configs", "config.yaml"),
	}

	if home, err := os.UserHomeDir(); err == nil {
		configPaths = append(configPaths, filepath.Join(home, ".config", "app", "config.yaml"))
	}

	for _, path := range configPaths {
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("error reading config file %s: %w", path, err)
		}

		expanded := expandEnvPlaceholders(string(data))
		if err := v.MergeConfig(strings.NewReader(expanded)); err != nil {
			return fmt.Errorf("error parsing config file %s: %w", path, err)
		}
		return nil
	}

	return nil
}

func expandEnvPlaceholders(content string) string {
	return envPlaceholderRegexp.ReplaceAllStringFunc(content, func(match string) string {
		submatches := envPlaceholderRegexp.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match
		}

		envName := submatches[1]
		defaultVal := ""
		if len(submatches) >= 3 {
			defaultVal = submatches[2]
		}

		if val, ok := os.LookupEnv(envName); ok {
			return val
		}
		return defaultVal
	})
}

func (s *service) normalizeAuthConfig(v *viper.Viper) {
	if secret := v.GetString("jwt.secret"); secret != "" {
		s.config.JWT.Secret = secret
	}
	if s.config.JWT.Secret == "" {
		s.config.JWT.Secret = s.config.App.SecretKey
	}
	if s.config.App.SecretKey == "" {
		s.config.App.SecretKey = s.config.JWT.Secret
	}
}

// hostIsRPIDOrSubdomain reports whether host equals rpid or is a subdomain of it.
func hostIsRPIDOrSubdomain(host, rpid string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	rpid = strings.ToLower(strings.TrimSpace(rpid))
	return host == rpid || strings.HasSuffix(host, "."+rpid)
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// setDefaults sets default configuration values
func (s *service) setDefaults(v *viper.Viper) {
	// App defaults
	v.SetDefault("app.name", "kupapay")
	v.SetDefault("app.env", DevelopmentEnvironment)
	v.SetDefault("app.port", 3000)
	v.SetDefault("app.host", "0.0.0.0")
	v.SetDefault("app.apiprefix", "/api")
	v.SetDefault("app.apiversion", "v1")
	v.SetDefault("app.secretkey", "")
	v.SetDefault("app.baseurl", "")
	v.SetDefault("app.dashboardurl", "")
	v.SetDefault("app.admindashboardurl", "")

	// JWT defaults
	v.SetDefault("jwt.secret", "")
	v.SetDefault("jwt.expiresin", "24h")
	v.SetDefault("jwt.refreshsecret", "")
	v.SetDefault("jwt.refreshexpiresin", "7d")

	// Session defaults — 30m mirrors legacy prod idle window (30 × 1min).
	v.SetDefault("session.idleduration", "30m")

	// Database defaults (PostgreSQL)
	v.SetDefault("database.writehost", "localhost")
	v.SetDefault("database.readhost", "localhost")
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.name", "kupapay")
	v.SetDefault("database.user", "postgres")
	v.SetDefault("database.password", "")
	v.SetDefault("database.sslmode", "disable")

	// Redis defaults
	v.SetDefault("redis.host", "localhost")
	v.SetDefault("redis.port", 6379)
	v.SetDefault("redis.db", 0)
	v.SetDefault("redis.keyprefix", "")
	v.SetDefault("redis.clusterenabled", false)
	v.SetDefault("redis.tlsenabled", false)
	v.SetDefault("redis.clusteraddrs", []string{})
	v.SetDefault("redis.sentinelenabled", false)
	v.SetDefault("redis.sentinelmastername", "mymaster")
	v.SetDefault("redis.sentineladdrs", []string{})

	// Cache defaults
	v.SetDefault("cache.type", "hybrid") // redis, memory, or hybrid
	v.SetDefault("cache.usefallback", true)
	v.SetDefault("cache.memorymaxsize", 1000)
	v.SetDefault("cache.memorymaxbytes", 100*1024*1024) // 100MB
	v.SetDefault("cache.memorycleanupsec", 60)
	v.SetDefault("cache.memorydefaultttlsec", 300)

	// Logger defaults
	v.SetDefault("logger.level", "debug")
	v.SetDefault("logger.format", "json")
	v.SetDefault("logger.output", "stdout")
	v.SetDefault("logger.rotateenabled", false)
	v.SetDefault("logger.rotatemaxsizemb", 100)
	v.SetDefault("logger.rotatemaxagedays", 30)
	v.SetDefault("logger.rotatemaxbackups", 3)
	v.SetDefault("logger.rotatecompresslog", false)
	v.SetDefault("logger.caller", false)

	// CORS defaults
	v.SetDefault("cors.origin", "*")
	v.SetDefault("cors.credentials", false)

	// Queue defaults
	v.SetDefault("queue.backend", "sqs")
	v.SetDefault("queue.redisaddr", "localhost:6379")
	v.SetDefault("queue.concurrency", 10)

}

func normalizeJWTExpiration(value any) (string, error) {
	raw := strings.TrimSpace(fmt.Sprint(value))
	duration, err := time.ParseDuration(raw)
	if err != nil {
		// JWT_EXPIRATION has historically been expressed in minutes.
		duration, err = time.ParseDuration(raw + "m")
		if err != nil {
			return "", fmt.Errorf("invalid JWT.ExpiresIn %q: use a duration such as 60m (numeric values are interpreted as minutes)", raw)
		}
	}
	if duration <= 0 {
		return "", fmt.Errorf("invalid JWT.ExpiresIn %q: duration must be positive", raw)
	}

	return duration.String(), nil
}

// Get returns the configuration
func (s *service) Get() *Config {
	return s.config
}

// GetApp returns app configuration
func (s *service) GetApp() *AppConfig {
	return &s.config.App
}

// GetDatabase returns database configuration
func (s *service) GetDatabase() *DatabaseConfig {
	return &s.config.Database
}

// GetRedis returns Redis configuration
func (s *service) GetRedis() *RedisConfig {
	return &s.config.Redis
}

// GetCache returns cache configuration
func (s *service) GetCache() *CacheConfig {
	return &s.config.Cache
}

// GetLogger returns logger configuration
func (s *service) GetLogger() *LoggerConfig {
	return &s.config.Logger
}

// GetCORS returns CORS configuration
func (s *service) GetCORS() *CORSConfig {
	return &s.config.CORS
}

// GetQueue returns queue configuration
func (s *service) GetQueue() *QueueConfig {
	return &s.config.Queue
}

// IsProduction returns true if environment is production
func (s *service) IsProduction() bool {
	return s.config.App.Env == "production"
}

// IsDevelopment returns true if environment is development
func (s *service) IsDevelopment() bool {
	return s.config.App.Env == "development"
}

// IsTest returns true if environment is test
func (s *service) IsTest() bool {
	return s.config.App.Env == "test"
}

// IsSandbox returns true if environment is the merchant/partner sandbox.
func (s *service) IsSandbox() bool {
	return s.config.App.Env == SandboxEnvironment
}
