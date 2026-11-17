package ca

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const Name = "ca-1"

type Authority struct {
	Name   string
	Cert   []byte
	Key    []byte
	Issued time.Time
}

type Options struct {
	Binary   string
	Name     string
	Networks string
	Lifetime time.Duration
	Now      time.Time
}

func Init(ctx context.Context, opts Options) (Authority, error) {
	if opts.Binary == "" {
		return Authority{}, errors.New("no nebula-cert binary")
	}
	if opts.Name == "" {
		return Authority{}, errors.New("no authority name")
	}
	if opts.Lifetime <= 0 {
		return Authority{}, errors.New("the lifetime must be more than zero")
	}

	dir, err := os.MkdirTemp("", "meshy-ca-")
	if err != nil {
		return Authority{}, err
	}
	defer os.RemoveAll(dir)

	keyPath := filepath.Join(dir, opts.Name+".key")
	crtPath := filepath.Join(dir, opts.Name+".crt")

	args := []string{
		"ca",
		"-name", opts.Name,
		"-duration", lifetime(opts.Lifetime),
		"-out-key", keyPath,
		"-out-crt", crtPath,
	}
	if opts.Networks != "" {
		args = append(args, "-networks", opts.Networks)
	}

	cmd := exec.CommandContext(ctx, opts.Binary, args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return Authority{}, fmt.Errorf("%s: %v: %s", filepath.Base(opts.Binary), err, strings.TrimSpace(string(out)))
	}

	key, err := os.ReadFile(keyPath)
	if err != nil {
		return Authority{}, fmt.Errorf("read the authority key: %w", err)
	}
	cert, err := os.ReadFile(crtPath)
	if err != nil {
		return Authority{}, fmt.Errorf("read the authority certificate: %w", err)
	}

	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}

	return Authority{
		Name:   opts.Name,
		Cert:   cert,
		Key:    key,
		Issued: now.UTC().Truncate(time.Second),
	}, nil
}

// Find locates nebula-cert in the pinned release cache, then on PATH. The cache
// is where meshy install puts the release, so a mesh that installed its
// binaries never needs the tool on PATH.
func Find(meshDir, version string) (string, error) {
	names := []string{"nebula-cert"}
	if runtime.GOOS == "windows" {
		names = []string{"nebula-cert.exe"}
	}

	var tried []string
	for _, name := range names {
		cached := filepath.Join(meshDir, ".meshy", "bin", version, runtime.GOOS+"-"+runtime.GOARCH, name)
		tried = append(tried, cached)
		if info, err := os.Stat(cached); err == nil && !info.IsDir() && info.Mode()&0o111 != 0 {
			return cached, nil
		}

		found, err := exec.LookPath(name)
		if err == nil {
			return found, nil
		}
		tried = append(tried, "PATH")
	}

	return "", fmt.Errorf("no nebula-cert for %s: looked in %s and on PATH", version, strings.Join(tried[:len(tried)-1], ", "))
}

// lifetime renders a duration in the hours that nebula-cert reads. A year is
// 365 days in this project, so twenty years is 175200h.
func lifetime(d time.Duration) string {
	hours := int64(d / time.Hour)
	if hours < 1 {
		hours = 1
	}
	return fmt.Sprintf("%dh", hours)
}
