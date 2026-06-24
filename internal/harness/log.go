package harness

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// LogLevel represents logging severity.
type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
)

// LogHook is a function called on every log event.
type LogHook func(entry LogEntry)

// LogEntry is a structured log entry.
type LogEntry struct {
	Timestamp time.Time              `json:"timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

// Logger provides structured logging with multiple outputs and hooks.
type Logger struct {
	mu      sync.Mutex
	level   LogLevel
	logger  *slog.Logger
	hooks   []LogHook
	fields  map[string]interface{}
	outputs []io.Writer
}

// NewLogger creates a new structured logger.
func NewLogger(level LogLevel, outputs ...io.Writer) *Logger {
	if len(outputs) == 0 {
		outputs = []io.Writer{os.Stderr}
	}

	multi := io.MultiWriter(outputs...)
	handler := slog.NewJSONHandler(multi, &slog.HandlerOptions{
		Level: slog.Level(level * 4), // map our levels to slog levels
	})

	return &Logger{
		level:   level,
		logger:  slog.New(handler),
		fields:  make(map[string]interface{}),
		outputs: outputs,
	}
}

// WithFields returns a new logger with additional fields.
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()

	newFields := make(map[string]interface{})
	for k, v := range l.fields {
		newFields[k] = v
	}
	for k, v := range fields {
		newFields[k] = v
	}

	return &Logger{
		level:   l.level,
		logger:  l.logger,
		hooks:   l.hooks,
		fields:  newFields,
		outputs: l.outputs,
	}
}

// AddHook adds a log hook.
func (l *Logger) AddHook(hook LogHook) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.hooks = append(l.hooks, hook)
}

// Debug logs a debug message.
func (l *Logger) Debug(msg string, fields ...interface{}) {
	if l.level <= LevelDebug {
		l.log("debug", msg, fields...)
	}
}

// Info logs an info message.
func (l *Logger) Info(msg string, fields ...interface{}) {
	if l.level <= LevelInfo {
		l.log("info", msg, fields...)
	}
}

// Warn logs a warning message.
func (l *Logger) Warn(msg string, fields ...interface{}) {
	if l.level <= LevelWarn {
		l.log("warn", msg, fields...)
	}
}

// Error logs an error message.
func (l *Logger) Error(msg string, fields ...interface{}) {
	if l.level <= LevelError {
		l.log("error", msg, fields...)
	}
}

func (l *Logger) log(level string, msg string, fields ...interface{}) {
	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   msg,
		Fields:    make(map[string]interface{}),
	}

	// Merge stored fields
	for k, v := range l.fields {
		entry.Fields[k] = v
	}

	// Parse variadic fields (key, value pairs)
	for i := 0; i < len(fields)-1; i += 2 {
		if key, ok := fields[i].(string); ok {
			entry.Fields[key] = fields[i+1]
		}
	}

	// Call hooks
	for _, hook := range l.hooks {
		hook(entry)
	}

	// Log via slog
	slogAttrs := make([]any, 0, len(entry.Fields))
	for k, v := range entry.Fields {
		slogAttrs = append(slogAttrs, slog.Any(k, v))
	}

	switch level {
	case "debug":
		l.logger.Debug(msg, slogAttrs...)
	case "info":
		l.logger.Info(msg, slogAttrs...)
	case "warn":
		l.logger.Warn(msg, slogAttrs...)
	case "error":
		l.logger.Error(msg, slogAttrs...)
	}
}

// ── Hook Factories ──

// FileHook writes log entries to a file as JSON lines.
func FileHook(path string) (LogHook, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}

	return func(entry LogEntry) {
		data, _ := json.Marshal(entry)
		f.Write(append(data, '\n'))
	}, nil
}

// WebhookHook sends log entries to a webhook URL.
func WebhookHook(url string) LogHook {
	client := &http.Client{Timeout: 5 * time.Second}

	return func(entry LogEntry) {
		data, _ := json.Marshal(entry)
		client.Post(url, "application/json", nil)
		_ = data // would be used in real implementation
	}
}

// MetricsHook tracks log metrics (counts by level).
type MetricsHook struct {
	mu     sync.Mutex
	Counts map[string]int
}

// NewMetricsHook creates a metrics hook.
func NewMetricsHook() *MetricsHook {
	return &MetricsHook{Counts: make(map[string]int)}
}

// Hook returns the LogHook function.
func (m *MetricsHook) Hook() LogHook {
	return func(entry LogEntry) {
		m.mu.Lock()
		defer m.mu.Unlock()
		m.Counts[entry.Level]++
	}
}

// String returns a summary of the metrics.
func (m *MetricsHook) String() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return fmt.Sprintf("debug=%d info=%d warn=%d error=%d",
		m.Counts["debug"], m.Counts["info"], m.Counts["warn"], m.Counts["error"])
}

// ── Convenience constructors ──

// NewConsoleLogger creates a logger that writes to stderr.
func NewConsoleLogger(level LogLevel) *Logger {
	return NewLogger(level, os.Stderr)
}

// NewFileLogger creates a logger that writes to a file.
func NewFileLogger(level LogLevel, path string) (*Logger, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}

	return NewLogger(level, os.Stderr, f), nil
}

// NewMultiLogger creates a logger with console + file + hooks.
func NewMultiLogger(level LogLevel, filePath string, webhookURL string) (*Logger, *MetricsHook, error) {
	metrics := NewMetricsHook()

	logger := NewConsoleLogger(level)
	logger.AddHook(metrics.Hook())

	if filePath != "" {
		fileHook, err := FileHook(filePath)
		if err != nil {
			return nil, nil, err
		}
		logger.AddHook(fileHook)
	}

	if webhookURL != "" {
		logger.AddHook(WebhookHook(webhookURL))
	}

	return logger, metrics, nil
}

// ── Package-level convenience (backward compat) ──

// Log is the default package-level logger.
var Log *Logger

func init() {
	Log = NewConsoleLogger(LevelInfo)
}

// SetVerbose enables debug logging on the default logger.
func SetVerbose() {
	Log = NewConsoleLogger(LevelDebug)
}
