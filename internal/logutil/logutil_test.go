package logutil

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDebugDisabledByDefault(t *testing.T) {
	t.Setenv("INCT_DEBUG", "")
	t.Setenv("INCT_LOG", "")
	if Init() {
		t.Fatal("logging must be off without INCT_DEBUG")
		t.Cleanup(Close)
	}
}

func TestLogPathOverride(t *testing.T) {
	t.Setenv("INCT_DEBUG", "1")
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "debug.log")
	t.Setenv("INCT_LOG", path)
	if !Init() {
		t.Fatal("expected logging enabled")
	}
	t.Cleanup(Close)
	if Path != path {
		t.Fatalf("Path = %q, want %q", Path, path)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("log file not created: %v", err)
	}
}

func TestDefaultLogDir(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux-only default path check")
	}
	t.Setenv("XDG_STATE_HOME", "/tmp/xdg-state")
	got, err := logDir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join("/tmp/xdg-state", "incantations")
	if got != want {
		t.Fatalf("logDir = %q, want %q", got, want)
	}
}

func TestDebugfWritesWhenEnabled(t *testing.T) {
	t.Setenv("INCT_DEBUG", "1")
	path := filepath.Join(t.TempDir(), "i.log")
	t.Setenv("INCT_LOG", path)
	if !Init() {
		t.Fatal("expected logging enabled")
	}
	Debugf("hello %s", "world")
	Close()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "hello world") {
		t.Fatalf("log file missing message: %q", data)
	}
}
