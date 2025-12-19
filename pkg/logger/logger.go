package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

// LogLevel represents the severity of a log message
type LogLevel int

const (
	// DebugLevel for detailed debugging information
	DebugLevel LogLevel = iota
	// InfoLevel for general informational messages
	InfoLevel
	// WarnLevel for warning messages
	WarnLevel
	// ErrorLevel for error messages
	ErrorLevel
)

// String returns the string representation of the log level
func (l LogLevel) String() string {
	switch l {
	case DebugLevel:
		return "DEBUG"
	case InfoLevel:
		return "INFO"
	case WarnLevel:
		return "WARN"
	case ErrorLevel:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// Logger provides structured logging with different log levels
type Logger struct {
	logger   *log.Logger
	level    LogLevel
	prefix   string
	useColor bool
}

// New creates a new Logger instance
func New(out io.Writer, prefix string, level LogLevel) *Logger {
	return &Logger{
		logger:   log.New(out, "", log.LstdFlags),
		level:    level,
		prefix:   prefix,
		useColor: isTerminal(out),
	}
}

// NewDefault creates a logger with default settings (INFO level)
func NewDefault(prefix string) *Logger {
	return New(os.Stdout, prefix, InfoLevel)
}

// SetLevel sets the minimum log level
func (l *Logger) SetLevel(level LogLevel) {
	l.level = level
}

// Debug logs a debug message
func (l *Logger) Debug(format string, v ...any) {
	if l.level <= DebugLevel {
		l.log(DebugLevel, format, v...)
	}
}

// Info logs an informational message
func (l *Logger) Info(format string, v ...any) {
	if l.level <= InfoLevel {
		l.log(InfoLevel, format, v...)
	}
}

// Warn logs a warning message
func (l *Logger) Warn(format string, v ...any) {
	if l.level <= WarnLevel {
		l.log(WarnLevel, format, v...)
	}
}

// Error logs an error message
func (l *Logger) Error(format string, v ...any) {
	if l.level <= ErrorLevel {
		l.log(ErrorLevel, format, v...)
	}
}

// Printf provides backward compatibility with standard log.Logger
func (l *Logger) Printf(format string, v ...any) {
	l.Info(format, v...)
}

// Println provides backward compatibility with standard log.Logger
func (l *Logger) Println(v ...any) {
	l.Info("%s", fmt.Sprint(v...))
}

// log is the internal logging method
func (l *Logger) log(level LogLevel, format string, v ...any) {
	levelStr := level.String()
	if l.useColor {
		levelStr = colorize(level, levelStr)
	}

	message := fmt.Sprintf(format, v...)
	l.logger.Printf("%s [%s] %s", l.prefix, levelStr, message)
}

// colorize adds ANSI color codes to the log level
func colorize(level LogLevel, text string) string {
	const (
		colorReset  = "\033[0m"
		colorGray   = "\033[90m"
		colorGreen  = "\033[32m"
		colorYellow = "\033[33m"
		colorRed    = "\033[31m"
	)

	switch level {
	case DebugLevel:
		return colorGray + text + colorReset
	case InfoLevel:
		return colorGreen + text + colorReset
	case WarnLevel:
		return colorYellow + text + colorReset
	case ErrorLevel:
		return colorRed + text + colorReset
	default:
		return text
	}
}

// isTerminal checks if the writer is a terminal
func isTerminal(w io.Writer) bool {
	if w == os.Stdout || w == os.Stderr {
		// Simple heuristic: check if TERM is set
		term := os.Getenv("TERM")
		return term != "" && !strings.Contains(term, "dumb")
	}
	return false
}
