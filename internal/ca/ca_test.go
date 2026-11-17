package ca

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func writeExecutable(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func cachePath(dir, version string) string {
	name := "nebula-cert"
	if runtime.GOOS == "windows" {
		name = "nebula-cert.exe"
	}
	return filepath.Join(dir, ".meshy", "bin", version, runtime.GOOS+"-"+runtime.GOARCH, name)
}

func TestFindPrefersTheCache(t *testing.T) {
	dir := t.TempDir()
	cached := cachePath(dir, "v1.12.0")
	writeExecutable(t, cached)

	got, err := Find(dir, "v1.12.0")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got != cached {
		t.Errorf("binary = %q, want %q", got, cached)
	}
}

func TestFindFallsBackToPath(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(t.TempDir(), "nebula-cert")
	writeExecutable(t, bin)
	t.Setenv("PATH", filepath.Dir(bin))

	got, err := Find(dir, "v1.12.0")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got != bin {
		t.Errorf("binary = %q, want %q", got, bin)
	}
}

func TestFindNamesBothPlacesWhenItFindsNothing(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	_, err := Find(t.TempDir(), "v1.12.0")
	if err == nil {
		t.Fatal("find returned no error")
	}
	if !strings.Contains(err.Error(), ".meshy/bin/v1.12.0") || !strings.Contains(err.Error(), "PATH") {
		t.Errorf("error does not name both places: %v", err)
	}
}

func TestLifetimeIsRenderedInHours(t *testing.T) {
	cases := map[time.Duration]string{
		20 * 365 * 24 * time.Hour: "175200h",
		time.Hour:                 "1h",
		30 * time.Minute:          "1h",
	}
	for in, want := range cases {
		if got := lifetime(in); got != want {
			t.Errorf("lifetime(%s) = %q, want %q", in, got, want)
		}
	}
}
