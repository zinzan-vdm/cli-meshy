package main

import (
	"strings"
	"testing"

	"github.com/zinzan-vdm/cli-meshy/internal/config"
	"github.com/zinzan-vdm/cli-meshy/internal/install"
)

func installMesh() config.Mesh {
	return config.Mesh{
		Name:   "homelab",
		Nebula: config.Nebula{Version: "v1.12.0"},
		Nodes: []config.Node{
			{Name: "core-1", Kind: config.KindServer, IP: "10.42.0.1", SSH: &config.SSH{User: "meshy", Host: "203.0.113.10"}},
			{Name: "web-1", Kind: config.KindServer, IP: "10.42.0.2", SSH: &config.SSH{User: "meshy", Host: "203.0.113.11"}},
			{Name: "laptop", Kind: config.KindUser, IP: "10.42.0.9", Platform: "darwin/arm64"},
		},
	}
}

func TestInstallTargetsTakesEveryServerNode(t *testing.T) {
	targets, code := installTargets(installMesh(), "")
	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}
	if len(targets) != 2 {
		t.Fatalf("got %d targets, want 2", len(targets))
	}
	for _, target := range targets {
		if target.Kind != config.KindServer {
			t.Errorf("%s is not a server node", target.Name)
		}
	}
}

func TestInstallTargetsTakesOneNode(t *testing.T) {
	targets, code := installTargets(installMesh(), "web-1")
	if code != 0 || len(targets) != 1 || targets[0].Name != "web-1" {
		t.Fatalf("targets = %v, code = %d", targets, code)
	}
}

func TestInstallTargetsRefusesAnUnknownNode(t *testing.T) {
	_, code := installTargets(installMesh(), "core-9")
	if code != 2 {
		t.Errorf("code = %d, want 2", code)
	}
}

func TestInstallTargetsSendsAUserNodeToBundle(t *testing.T) {
	_, code := installTargets(installMesh(), "laptop")
	if code != 1 {
		t.Errorf("code = %d, want 1", code)
	}
}

func TestInstallTargetsNeedsAServerNode(t *testing.T) {
	mesh := config.Mesh{Name: "homelab", Nebula: config.Nebula{Version: "v1.12.0"}}
	_, code := installTargets(mesh, "")
	if code != 2 {
		t.Errorf("code = %d, want 2", code)
	}
}

func TestReportInstallNamesTheMove(t *testing.T) {
	mesh := config.Mesh{Name: "homelab", Nebula: config.Nebula{Version: "v1.12.0"}}

	out, _, code := captureBoth(t, func() int {
		return reportInstall(mesh, install.Result{Node: "core-1", From: "1.11.0", Moved: true})
	})
	if code != 0 {
		t.Errorf("code = %d, want 0", code)
	}
	want := "core-1: v1.12.0, was 1.11.0, restarted nebula@meshy-homelab\n"
	if out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}
}

func TestReportInstallSaysNothingForANodeOnThePin(t *testing.T) {
	mesh := config.Mesh{Name: "homelab", Nebula: config.Nebula{Version: "v1.12.0"}}

	out, _, code := captureBoth(t, func() int {
		return reportInstall(mesh, install.Result{Node: "core-1", From: "1.12.0"})
	})
	if code != 0 {
		t.Errorf("code = %d, want 0", code)
	}
	if out != "core-1: v1.12.0\n" {
		t.Errorf("stdout = %q", out)
	}
}

func TestReportInstallNamesANodeWithNoNebula(t *testing.T) {
	mesh := config.Mesh{Name: "homelab", Nebula: config.Nebula{Version: "v1.12.0"}}

	out, _, _ := captureBoth(t, func() int {
		return reportInstall(mesh, install.Result{Node: "core-1", Moved: true})
	})
	if !strings.Contains(out, "was nothing") {
		t.Errorf("stdout = %q, want the empty start named", out)
	}
}

func TestReportInstallFailsTheRun(t *testing.T) {
	mesh := config.Mesh{Name: "homelab", Nebula: config.Nebula{Version: "v1.12.0"}}

	_, errOut, code := captureBoth(t, func() int {
		return reportInstall(mesh, install.Result{Node: "core-1", Err: errInstallFailed})
	})
	if code != 1 {
		t.Errorf("code = %d, want 1", code)
	}
	if !strings.Contains(errOut, "meshy install: node core-1 is down") {
		t.Errorf("stderr = %q", errOut)
	}
}

type installError string

func (e installError) Error() string { return string(e) }

const errInstallFailed = installError("node core-1 is down")

func TestPlatformCacheReadsTheSourceAndKeepsOneCopy(t *testing.T) {
	mesh := config.Mesh{Name: "homelab", Nebula: config.Nebula{Version: "v1.12.0"}}
	cache := platformCache(mesh, t.TempDir(), t.TempDir())

	// The source directory holds no archive, so the call fails. The point is that
	// it fails on the source and not on a download.
	_, err := cache(t.Context(), install.Platform{OS: "linux", Arch: "amd64"})
	if err == nil {
		t.Fatal("the cache found an archive that is not there")
	}
	if !strings.Contains(err.Error(), "no such file or directory") {
		t.Errorf("error = %v, want a missing file", err)
	}
}
