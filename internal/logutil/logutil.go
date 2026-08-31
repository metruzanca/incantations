// Package logutil provides debug logging to a file. Logs never go to stdout or
// stderr so generated shell integration (a copy of stdout) stays clean.
//
// Logging is disabled unless INCT_DEBUG is set to a truthy value (or INCT_LOG
// is set). The log file location can be overridden with INCT_LOG; otherwise it
// defaults to $XDG_STATE_HOME/incantations/incantations.log (~/.local/state on
// Linux, ~/Library/Logs on macOS).
package logutil

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

var (
	mu     sync.Mutex
	on     bool
	file   *os.File
	logger *log.Logger
	Path   string
)

// Init enables file logging when INCT_DEBUG is set and opens the log file.
// Returns whether logging is active.
func Init() bool {
	mu.Lock()
	defer mu.Unlock()
	if on {
		return true
	}
	if !debugEnabled() {
		return false
	}
	dir, err := logDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "incantations: debug log: %v\n", err)
		return false
	}
	p := os.Getenv("INCT_LOG")
	if p != "" {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "incantations: debug log: %v\n", err)
			return false
		}
	} else {
		p = filepath.Join(dir, "incantations.log")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "incantations: debug log: %v\n", err)
			return false
		}
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "incantations: debug log: %v\n", err)
		return false
	}
	on = true
	file = f
	Path = p
	logger = log.New(f, "", log.LstdFlags|log.Lmicroseconds)
	fmt.Fprintf(os.Stderr, "incantations: debug log: %s\n", p)
	// Log directly: Init already holds the mutex.
	logger.Printf("DEBUG started pid=%d args=%q", os.Getpid(), os.Args[1:])
	return true
}

// Close closes the log file. Safe to call multiple times.
func Close() {
	mu.Lock()
	defer mu.Unlock()
	if file != nil {
		file.Close()
		file = nil
	}
	on = false
	logger = nil
}

// Debugf logs a debug message when logging is enabled.
func Debugf(format string, args ...any) {
	logf("DEBUG", format, args...)
}

// Errorf logs an error message when logging is enabled.
func Errorf(format string, args ...any) {
	logf("ERROR", format, args...)
}

func logf(level, format string, args ...any) {
	mu.Lock()
	defer mu.Unlock()
	if !on {
		return
	}
	logger.Printf("%s "+format, append([]any{level}, args...)...)
}

func debugEnabled() bool {
	// INCT_LOG alone is enough to enable file logging at that path.
	if os.Getenv("INCT_LOG") != "" {
		return true
	}
	v := os.Getenv("INCT_DEBUG")
	return v != "" && v != "0"
}

func logDir() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Logs", "incantations"), nil
	case "windows":
		if ad := os.Getenv("LOCALAPPDATA"); ad != "" {
			return filepath.Join(ad, "incantations", "logs"), nil
		}
		cache, err := os.UserCacheDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(cache, "incantations"), nil
	default:
		if xdg := os.Getenv("XDG_STATE_HOME"); xdg != "" {
			return filepath.Join(xdg, "incantations"), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".local", "state", "incantations"), nil
	}
}
