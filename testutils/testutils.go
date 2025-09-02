// Package testutils provides test utilities for k6.
package testutils

import (
	"testing"
)

// NewLogger creates a new logger for testing
func NewLogger(t testing.TB) *Logger {
	return &Logger{t: t}
}

// Logger is a simple logger for testing
type Logger struct {
	t testing.TB
}

// WithField adds a field to the logger
func (l *Logger) WithField(_ string, _ interface{}) *Logger {
	return l
}

// Debug logs a debug message
func (l *Logger) Debug(args ...interface{}) {
	l.t.Log(args...)
}

// Debugf logs a formatted debug message
func (l *Logger) Debugf(format string, args ...interface{}) {
	l.t.Logf(format, args...)
}

// Info logs an info message
func (l *Logger) Info(args ...interface{}) {
	l.t.Log(args...)
}

// Infof logs a formatted info message
func (l *Logger) Infof(format string, args ...interface{}) {
	l.t.Logf(format, args...)
}

// Warn logs a warning message
func (l *Logger) Warn(args ...interface{}) {
	l.t.Log(args...)
}

// Warnf logs a formatted warning message
func (l *Logger) Warnf(format string, args ...interface{}) {
	l.t.Logf(format, args...)
}

// Error logs an error message
func (l *Logger) Error(args ...interface{}) {
	l.t.Log(args...)
}

// Errorf logs a formatted error message
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.t.Logf(format, args...)
}

// Fatal logs a fatal message
func (l *Logger) Fatal(args ...interface{}) {
	l.t.Fatal(args...)
}

// Fatalf logs a formatted fatal message
func (l *Logger) Fatalf(format string, args ...interface{}) {
	l.t.Fatalf(format, args...)
}
