// Purpose    : Test structured logging functionality with debug, info, error levels
// Context    : Unit tests for logger package using log/slog backend
// Constraints: No heap allocation, structured output only

package logger

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

func TestLogger_InfoLevel(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, slog.LevelInfo)

	logger.Info("test message", "key", "value")

	output := buf.String()
	if !strings.Contains(output, "test message") {
		t.Errorf("Expected log output to contain 'test message', got: %s", output)
	}
	if !strings.Contains(output, "INFO") {
		t.Errorf("Expected log output to contain 'INFO', got: %s", output)
	}
}

func TestLogger_DebugLevel_WhenDebugEnabled(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, slog.LevelDebug)

	logger.Debug("debug message", "debug_key", "debug_value")

	output := buf.String()
	if !strings.Contains(output, "debug message") {
		t.Errorf("Expected log output to contain 'debug message', got: %s", output)
	}
	if !strings.Contains(output, "DEBUG") {
		t.Errorf("Expected log output to contain 'DEBUG', got: %s", output)
	}
}

func TestLogger_DebugLevel_WhenDebugDisabled(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, slog.LevelInfo)

	logger.Debug("debug message", "debug_key", "debug_value")

	output := buf.String()
	if strings.Contains(output, "debug message") {
		t.Errorf("Expected debug message to be filtered out when level is INFO, got: %s", output)
	}
}

func TestLogger_ErrorLevel(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, slog.LevelInfo)

	logger.Error("error message", "error_key", "error_value")

	output := buf.String()
	if !strings.Contains(output, "error message") {
		t.Errorf("Expected log output to contain 'error message', got: %s", output)
	}
	if !strings.Contains(output, "ERROR") {
		t.Errorf("Expected log output to contain 'ERROR', got: %s", output)
	}
}

func TestLogger_StructuredOutput(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, slog.LevelInfo)

	logger.Info("structured test", "string_field", "value", "int_field", 42, "bool_field", true)

	output := buf.String()

	// Parse as JSON to verify structure
	var logEntry map[string]interface{}
	if err := json.Unmarshal([]byte(output), &logEntry); err != nil {
		t.Errorf("Expected structured JSON output, but got invalid JSON: %v", err)
	}

	// Verify required fields exist
	if _, exists := logEntry["time"]; !exists {
		t.Error("Expected 'time' field in structured output")
	}
	if _, exists := logEntry["level"]; !exists {
		t.Error("Expected 'level' field in structured output")
	}
	if _, exists := logEntry["msg"]; !exists {
		t.Error("Expected 'msg' field in structured output")
	}
}

func TestLogger_Printf_Compatibility(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, slog.LevelInfo)

	logger.Printf("formatted message: %s %d", "test", 123)

	output := buf.String()
	if !strings.Contains(output, "formatted message: test 123") {
		t.Errorf("Expected formatted message, got: %s", output)
	}
}

func TestLogger_Println_Compatibility(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, slog.LevelInfo)

	logger.Println("simple message")

	output := buf.String()
	if !strings.Contains(output, "simple message") {
		t.Errorf("Expected simple message, got: %s", output)
	}
}

func TestLogger_Fatalf(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, slog.LevelInfo)

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected Fatalf to panic")
		}
	}()

	logger.Fatalf("fatal error: %s", "test")
}

func TestGlobalFunctions(t *testing.T) {
	var buf bytes.Buffer
	originalLogger := defaultLogger
	defer func() {
		SetDefault(originalLogger)
	}()

	// Set a test logger
	testLogger := New(&buf, slog.LevelDebug)
	SetDefault(testLogger)

	// Test global Info
	Info("global info", "key", "value")
	output := buf.String()
	if !strings.Contains(output, "global info") {
		t.Errorf("Expected global info message, got: %s", output)
	}

	// Reset buffer
	buf.Reset()

	// Test global Debug
	Debug("global debug", "debug_key", "debug_value")
	output = buf.String()
	if !strings.Contains(output, "global debug") {
		t.Errorf("Expected global debug message, got: %s", output)
	}

	// Reset buffer
	buf.Reset()

	// Test global Error
	Error("global error", "error_key", "error_value")
	output = buf.String()
	if !strings.Contains(output, "global error") {
		t.Errorf("Expected global error message, got: %s", output)
	}

	// Reset buffer
	buf.Reset()

	// Test global Printf
	Printf("formatted: %s %d", "test", 42)
	output = buf.String()
	if !strings.Contains(output, "formatted: test 42") {
		t.Errorf("Expected formatted message, got: %s", output)
	}

	// Reset buffer
	buf.Reset()

	// Test global Println
	Println("simple global message")
	output = buf.String()
	if !strings.Contains(output, "simple global message") {
		t.Errorf("Expected simple global message, got: %s", output)
	}
}
