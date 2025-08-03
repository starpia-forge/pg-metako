// Purpose    : Provide structured logging with debug, info, error levels using log/slog
// Context    : Application-wide logging system for pg-metako with consistent structured output
// Constraints: No heap allocation, structured JSON output only

package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
)

// Logger wraps slog.Logger to provide structured logging functionality
type Logger struct {
	slogger *slog.Logger
}

// New creates a new Logger instance with the specified writer and log level
func New(writer io.Writer, level slog.Level) *Logger {
	opts := &slog.HandlerOptions{
		Level: level,
	}
	handler := slog.NewJSONHandler(writer, opts)
	slogger := slog.New(handler)

	return &Logger{
		slogger: slogger,
	}
}

// Debug logs a debug level message with structured key-value pairs
func (l *Logger) Debug(msg string, args ...any) {
	l.slogger.Debug(msg, args...)
}

// Info logs an info level message with structured key-value pairs
func (l *Logger) Info(msg string, args ...any) {
	l.slogger.Info(msg, args...)
}

// Error logs an error level message with structured key-value pairs
func (l *Logger) Error(msg string, args ...any) {
	l.slogger.Error(msg, args...)
}

// Printf provides compatibility with standard log.Printf for formatted messages
func (l *Logger) Printf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	l.slogger.Info(msg)
}

// Println provides compatibility with standard log.Println for simple messages
func (l *Logger) Println(args ...any) {
	msg := fmt.Sprint(args...)
	l.slogger.Info(msg)
}

// Fatalf logs an error message and exits the program (compatibility with log.Fatalf)
func (l *Logger) Fatalf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	l.slogger.Error(msg)
	panic(msg) // Use panic instead of os.Exit for testability
}

// Global logger instance
var defaultLogger *Logger

// init initializes the default logger with INFO level to os.Stderr
func init() {
	defaultLogger = New(os.Stderr, slog.LevelInfo)
}

// SetDefault sets the default logger instance
func SetDefault(logger *Logger) {
	defaultLogger = logger
}

// Global convenience functions for drop-in replacement of standard log package

// Debug logs a debug level message using the default logger
func Debug(msg string, args ...any) {
	defaultLogger.Debug(msg, args...)
}

// Info logs an info level message using the default logger
func Info(msg string, args ...any) {
	defaultLogger.Info(msg, args...)
}

// Error logs an error level message using the default logger
func Error(msg string, args ...any) {
	defaultLogger.Error(msg, args...)
}

// Printf logs a formatted message using the default logger
func Printf(format string, args ...any) {
	defaultLogger.Printf(format, args...)
}

// Println logs a simple message using the default logger
func Println(args ...any) {
	defaultLogger.Println(args...)
}

// Fatalf logs an error message and exits using the default logger
func Fatalf(format string, args ...any) {
	defaultLogger.Fatalf(format, args...)
}
