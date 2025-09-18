package logger

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// Environment variable to configure log file path.
const envLogPath = "WEB_MCP_LOG"

var (
	std           *log.Logger
	logFile       *os.File
	isInitialized bool
)

// InitFromEnv initializes the logger using WEB_MCP_LOG or a default path.
func InitFromEnv() error {
	path := os.Getenv(envLogPath)
	if path == "" {
		// Default to the directory where the executable is located
		if exePath, err := os.Executable(); err == nil {
			exeDir := filepath.Dir(exePath)
			path = filepath.Join(exeDir, "web-mcp.log")
		} else {
			// Fallback to current directory if executable path cannot be determined
			path = "./web-mcp.log"
		}
	}
	return Init(path)
}

// Init initializes the logger to write to the provided file path.
// It creates parent directories if needed and opens the file in append mode.
func Init(path string) error {
	if isInitialized {
		return nil
	}
	if err := ensureParentDir(path); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	logFile = f
	std = log.New(f, "", log.Ldate|log.Ltime|log.Lmicroseconds)
	isInitialized = true
	return nil
}

// Close closes the underlying log file, if open.
func Close() error {
	if logFile != nil {
		err := logFile.Close()
		logFile = nil
		return err
	}
	return nil
}

// Printf logs a formatted message at info level.
func Printf(format string, args ...any) { write("INFO", format, args...) }

// Infof logs informational messages.
func Infof(format string, args ...any) { write("INFO", format, args...) }

// Warnf logs warnings.
func Warnf(format string, args ...any) { write("WARN", format, args...) }

// Errorf logs errors.
func Errorf(format string, args ...any) { write("ERROR", format, args...) }

func write(level string, format string, args ...any) {
	if std == nil {
		// Fallback: initialize with default if not already.
		_ = InitFromEnv()
	}
	if std != nil {
		std.Printf("[%s] %s", level, fmt.Sprintf(format, args...))
	}
}

func ensureParentDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}
