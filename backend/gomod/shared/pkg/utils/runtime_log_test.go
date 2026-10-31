package utils

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeLogPathCreatesServiceDir(t *testing.T) {
	dir := RuntimeLogDir("testsvc")
	if !strings.HasSuffix(filepath.ToSlash(dir), "/.runtime/logs/testsvc") {
		t.Fatalf("dir must live under .runtime/logs/testsvc, got %q", dir)
	}
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		t.Fatalf("service dir must be created: err=%v", err)
	}

	p := RuntimeLogPath("testsvc", "testsvc.log")
	if !strings.HasSuffix(filepath.ToSlash(p), "/.runtime/logs/testsvc/testsvc.log") {
		t.Fatalf("path must be .runtime/logs/testsvc/testsvc.log, got %q", p)
	}

	shared := RuntimeLogPath("", "sql.log")
	if !strings.HasSuffix(filepath.ToSlash(shared), "/.runtime/logs/sql.log") {
		t.Fatalf("empty service must land in the shared root, got %q", shared)
	}
}
