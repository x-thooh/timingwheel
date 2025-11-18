package log

import "context"

// Logger is used for logging formatted messages.
type Logger interface {
	// Debug logs a message at debug level.
	Debug(ctx context.Context, format string, a ...interface{})

	// Info logs a message at info level.
	Info(ctx context.Context, format string, a ...interface{})

	// Warn logs a message at warnf level.
	Warn(ctx context.Context, format string, a ...interface{})

	// Error logs a message at error level.
	Error(ctx context.Context, format string, a ...interface{})
}
