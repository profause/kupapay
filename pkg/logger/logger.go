package logger

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/profause/kupapay/pkg/redact"
	"github.com/rs/zerolog"
)

const (
	// Default rotation settings when WithRotation is used.
	DefaultRotateMaxSizeMB  = 100
	DefaultRotateMaxBackups = 3
	DefaultRotateMaxAgeDays = 30

	// callerSkip is the number of extra frames to skip beyond zerolog's default
	// so that the reported caller is the actual call site, not the wrapper method.
	callerSkip = 1
)

type options struct {
	rotateEnabled bool
	maxSizeMB     int
	maxBackups    int
	maxAgeDays    int
	compress      bool
	callerEnabled bool
}

// Option configures logger behavior.
type Option func(*options)

// WithCaller enables the caller (file:line) field on every log entry.
// Disabled by default to avoid the ~300ns runtime.Callers overhead in production.
func WithCaller() Option {
	return func(o *options) {
		o.callerEnabled = true
	}
}

// WithRotation enables lumberjack file rotation for file outputs.
// It is ignored when output is stdout/stderr.
func WithRotation(maxSizeMB, maxBackups, maxAgeDays int, compress bool) Option {
	return func(o *options) {
		o.rotateEnabled = true
		o.maxSizeMB = maxSizeMB
		o.maxBackups = maxBackups
		o.maxAgeDays = maxAgeDays
		o.compress = compress
	}
}

func defaultOptions() options {
	return options{
		maxSizeMB:  DefaultRotateMaxSizeMB,
		maxBackups: DefaultRotateMaxBackups,
		maxAgeDays: DefaultRotateMaxAgeDays,
	}
}

// Logger wraps zerolog logger
type Logger struct {
	logger   zerolog.Logger
	noCaller bool
}

// NewNop returns a no-op logger that drops all log events.
func NewNop() *Logger {
	return &Logger{
		logger: zerolog.Nop(),
	}
}

// Service provides logging functionality
type Service struct {
	logger *Logger
}

// NewService creates a new logger service
func NewService(level, format, output string, opts ...Option) (*Service, error) {
	cfg := defaultOptions()
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	logger, err := newLogger(level, format, output, cfg)
	if err != nil {
		return nil, err
	}

	return &Service{
		logger: logger,
	}, nil
}

// newLogger creates a new zerolog logger.
// Caller is added at the event level (not baked into the logger context) so that
// it can be suppressed selectively via WithoutCaller().
func newLogger(level, format, output string, cfg options) (*Logger, error) {
	output = strings.TrimSpace(output)
	format = strings.ToLower(strings.TrimSpace(format))

	// Parse log level
	logLevel := zerolog.InfoLevel
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "trace":
		logLevel = zerolog.TraceLevel
	case "debug":
		logLevel = zerolog.DebugLevel
	case "info":
		logLevel = zerolog.InfoLevel
	case "warn", "warning":
		logLevel = zerolog.WarnLevel
	case "error":
		logLevel = zerolog.ErrorLevel
	case "fatal":
		logLevel = zerolog.FatalLevel
	case "panic":
		logLevel = zerolog.PanicLevel
	}

	zerolog.SetGlobalLevel(logLevel)

	// Configure output
	var writer io.Writer
	switch output {
	case "", "stdout":
		writer = os.Stdout
	case "stderr":
		writer = os.Stderr
	default:
		dir := filepath.Dir(output)
		if dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, err
			}
		}

		if cfg.rotateEnabled {
			if cfg.maxSizeMB <= 0 {
				cfg.maxSizeMB = DefaultRotateMaxSizeMB
			}
			if cfg.maxBackups < 0 {
				cfg.maxBackups = DefaultRotateMaxBackups
			}
			if cfg.maxAgeDays < 0 {
				cfg.maxAgeDays = DefaultRotateMaxAgeDays
			}
			writer = &lumberjack.Logger{
				Filename:   output,
				MaxSize:    cfg.maxSizeMB,
				MaxBackups: cfg.maxBackups,
				MaxAge:     cfg.maxAgeDays,
				Compress:   cfg.compress,
			}
			break
		}

		file, err := os.OpenFile(output, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, err
		}
		writer = file
	}

	// Configure format — no Caller() in context; added per-event instead.
	var log zerolog.Logger
	if format == "json" {
		log = zerolog.New(writer).With().Timestamp().Logger()
	} else {
		isConsole := output == "stdout" || output == "stderr"
		if isConsole {
			writer = zerolog.ConsoleWriter{
				Out:        writer,
				TimeFormat: time.RFC3339,
				NoColor:    false,
			}
		}
		log = zerolog.New(writer).With().Timestamp().Logger()
	}

	return &Logger{logger: log, noCaller: !cfg.callerEnabled}, nil
}

// Get returns the logger
func (s *Service) Get() *Logger {
	return s.logger
}

// GetZerolog returns the underlying zerolog logger
func (s *Service) GetZerolog() *zerolog.Logger {
	return &s.logger.logger
}

// GetLogger returns the underlying zerolog logger (for compatibility)
func (l *Logger) GetLogger() *zerolog.Logger {
	return &l.logger
}

// WithFields creates a logger with additional fields
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	ctx := l.logger.With()
	for k, v := range fields {
		ctx = ctx.Interface(k, redact.Field(k, v))
	}
	return &Logger{logger: ctx.Logger(), noCaller: l.noCaller}
}

// WithField creates a logger with a single field
func (l *Logger) WithField(key string, value interface{}) *Logger {
	return &Logger{
		logger:   l.logger.With().Interface(key, redact.Field(key, value)).Logger(),
		noCaller: l.noCaller,
	}
}

// WithRequestID creates a logger with a request ID field.
func (l *Logger) WithRequestID(requestID string) *Logger {
	return &Logger{
		logger:   l.logger.With().Str("request_id", requestID).Logger(),
		noCaller: l.noCaller,
	}
}

// WithUserID creates a logger with a user ID field.
func (l *Logger) WithUserID(userID string) *Logger {
	return &Logger{
		logger:   l.logger.With().Str("user_id", userID).Logger(),
		noCaller: l.noCaller,
	}
}

// WithContext creates a logger with multiple context fields.
func (l *Logger) WithContext(fields map[string]interface{}) *Logger {
	return l.WithFields(fields)
}

// WithComponent creates a logger with a component field.
func (l *Logger) WithComponent(component string) *Logger {
	return &Logger{
		logger:   l.logger.With().Str("component", component).Logger(),
		noCaller: l.noCaller,
	}
}

// WithoutCaller returns a logger that omits the caller field from every log line.
// Use this for structured logs where the call site is always the same
// (e.g. HTTP middleware) and the caller field adds no value.
func (l *Logger) WithoutCaller() *Logger {
	return &Logger{logger: l.logger, noCaller: true}
}

// addCaller attaches caller info to an event when the logger has caller enabled.
// callerSkip is the number of extra frames beyond zerolog's default to skip,
// accounting for this wrapper method and any intermediate calls.
func (l *Logger) addCaller(e *zerolog.Event) *zerolog.Event {
	if l.noCaller {
		return e
	}
	return e.Caller(callerSkip)
}

// appendKV appends key-value pairs to a zerolog event, using .Err() for error values.
func appendKV(e *zerolog.Event, keysAndValues []interface{}) *zerolog.Event {
	for i := 0; i+1 < len(keysAndValues); i += 2 {
		key, ok := keysAndValues[i].(string)
		if !ok || strings.TrimSpace(key) == "" {
			continue
		}

		if err, ok := keysAndValues[i+1].(error); ok {
			if val, ok := redact.Field(key, err.Error()).(string); ok {
				e = e.Str(key, val)
			} else {
				e = e.Interface(key, redact.Field(key, err.Error()))
			}
			continue
		}

		val := redact.Field(key, keysAndValues[i+1])
		if err, ok := val.(error); ok {
			e = e.Str(key, err.Error())
		} else {
			e = e.Interface(key, val)
		}
	}
	return e
}

// Trace logs a trace message
func (l *Logger) Trace(msg string) {
	l.addCaller(l.logger.Trace()).Msg(msg)
}

// Tracef logs a trace message with formatting
func (l *Logger) Tracef(format string, args ...interface{}) {
	l.addCaller(l.logger.Trace()).Msgf(format, args...)
}

// Tracew logs a trace message with fields
func (l *Logger) Tracew(msg string, keysAndValues ...interface{}) {
	appendKV(l.addCaller(l.logger.Trace()), keysAndValues).Msg(msg)
}

// Debug logs a debug message
func (l *Logger) Debug(msg string) {
	l.addCaller(l.logger.Debug()).Msg(msg)
}

// Debugf logs a debug message with formatting
func (l *Logger) Debugf(format string, args ...interface{}) {
	l.addCaller(l.logger.Debug()).Msgf(format, args...)
}

// Debugw logs a debug message with fields
func (l *Logger) Debugw(msg string, keysAndValues ...interface{}) {
	appendKV(l.addCaller(l.logger.Debug()), keysAndValues).Msg(msg)
}

// Info logs an info message
func (l *Logger) Info(msg string) {
	l.addCaller(l.logger.Info()).Msg(msg)
}

// Infof logs an info message with formatting
func (l *Logger) Infof(format string, args ...interface{}) {
	l.addCaller(l.logger.Info()).Msgf(format, args...)
}

// Infow logs an info message with fields
func (l *Logger) Infow(msg string, keysAndValues ...interface{}) {
	appendKV(l.addCaller(l.logger.Info()), keysAndValues).Msg(msg)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string) {
	l.addCaller(l.logger.Warn()).Msg(msg)
}

// Warnf logs a warning message with formatting
func (l *Logger) Warnf(format string, args ...interface{}) {
	l.addCaller(l.logger.Warn()).Msgf(format, args...)
}

// Warnw logs a warning message with fields
func (l *Logger) Warnw(msg string, keysAndValues ...interface{}) {
	appendKV(l.addCaller(l.logger.Warn()), keysAndValues).Msg(msg)
}

// Error logs an error message
func (l *Logger) Error(msg string) {
	l.addCaller(l.logger.Error()).Msg(msg)
}

// Errorf logs an error message with formatting
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.addCaller(l.logger.Error()).Msgf(format, args...)
}

// Errorw logs an error message with fields
func (l *Logger) Errorw(msg string, keysAndValues ...interface{}) {
	appendKV(l.addCaller(l.logger.Error()), keysAndValues).Msg(msg)
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(msg string) {
	l.addCaller(l.logger.Fatal()).Msg(msg)
}

// Fatalf logs a fatal message with formatting and exits
func (l *Logger) Fatalf(format string, args ...interface{}) {
	l.addCaller(l.logger.Fatal()).Msgf(format, args...)
}

// Panic logs a panic message and panics
func (l *Logger) Panic(msg string) {
	l.addCaller(l.logger.Panic()).Msg(msg)
}

// Panicf logs a panic message with formatting and panics
func (l *Logger) Panicf(format string, args ...interface{}) {
	l.addCaller(l.logger.Panic()).Msgf(format, args...)
}
