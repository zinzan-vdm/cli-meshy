package installers_test

import (
	"strings"
	"testing"

	"github.com/zinzan-vdm/cli-meshy/internal/scaffold/installers"
)

func values() installers.Values {
	return installers.Values{
		Mesh:        "homelab",
		Node:        "laptop-1",
		Interface:   "meshy-homelab",
		Dir:         "/etc/nebula/meshy-homelab",
		ConfigPath:  "/etc/nebula/meshy-homelab/config.yml",
		Unit:        "nebula@meshy-homelab.service",
		UnitFile:    "/etc/systemd/system/nebula@meshy-homelab.service",
		Plist:       "/Library/LaunchDaemons/com.meshy.homelab.plist",
		BinaryPath:  "/usr/local/bin/nebula",
		Binary:      "nebula",
		Version:     "v1.12.0",
		Asset:       "nebula-linux-amd64.tar.gz",
		Checksum:    "SHASUM256.txt",
		BaseURL:     "https://example.invalid/download",
		Destination: `C:\ProgramData\nebula\meshy-homelab`,
	}
}

func render(t *testing.T) map[string][]byte {
	t.Helper()
	bodies, err := installers.Render(values())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	for _, name := range installers.Names() {
		if _, ok := bodies[name]; !ok {
			t.Fatalf("Render returned no %s", name)
		}
		if strings.Contains(string(bodies[name]), "{{") {
			t.Errorf("%s holds an unsubstituted placeholder", name)
		}
	}
	return bodies
}

// TestLinuxInstallerNamesEveryStep asserts the structure that the Linux
// installer must have. TestLinuxInstallerPlacesTheFilesAndStartsTheUnit runs
// it.
func TestLinuxInstallerNamesEveryStep(t *testing.T) {
	body := string(render(t)["install.sh"])

	for _, want := range []string{
		"#!/bin/sh",
		`if [ "$(id -u)" -ne 0 ]`,
		"run this installer as root",
		"/etc/nebula/meshy-homelab",
		"/etc/systemd/system/nebula@meshy-homelab.service",
		"install -m 0644 \"$HERE/blocklist.txt\" \"$DIR/blocklist.txt\"",
		"install -m 0600 \"$HERE/config.yml\" \"$DIR/config.yml\"",
		"install -m 0600 \"$HERE/laptop-1.key\" \"$DIR/laptop-1.key\"",
		"ExecStart=$BINARY -config $DIR/config.yml",
		"systemctl daemon-reload",
		"systemctl enable --now \"$UNIT\"",
		"https://example.invalid/download/v1.12.0/SHASUM256.txt",
		"https://example.invalid/download/v1.12.0/nebula-linux-amd64.tar.gz",
		"sha256sum",
		"echo \"meshy-homelab is running\"",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("install.sh does not hold %q", want)
		}
	}
	if strings.Contains(body, "nebula@meshy-.service") {
		t.Error("install.sh does not name the mesh in the unit")
	}
}

// TestMacInstallerClearsQuarantineAndLoadsThePlist proves the macOS installer
// by structure, because this host cannot run it.
func TestMacInstallerClearsQuarantineAndLoadsThePlist(t *testing.T) {
	body := string(render(t)["install.command"])

	for _, want := range []string{
		"#!/bin/sh",
		"run this installer as root",
		"/Library/LaunchDaemons/com.meshy.homelab.plist",
		"xattr -dr com.apple.quarantine",
		"launchctl load -w \"$PLIST\"",
		"<key>RunAtLoad</key>",
		"<key>KeepAlive</key>",
		"<string>/usr/local/bin/nebula</string>",
		"<string>/etc/nebula/meshy-homelab/config.yml</string>",
		"shasum -a 256",
		"install -m 0600 \"$HERE/laptop-1.key\"",
		"echo \"meshy-homelab is running\"",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("install.command does not hold %q", want)
		}
	}

	quarantine := strings.Index(body, "xattr -dr com.apple.quarantine")
	load := strings.Index(body, "launchctl load -w")
	rights := strings.Index(body, `if [ "$(id -u)" -ne 0 ]`)
	if quarantine < 0 || load < 0 || rights < 0 {
		t.Fatal("install.command lost a step")
	}
	if rights > quarantine {
		t.Error("install.command clears the quarantine flag before it checks the rights")
	}
	if quarantine > load {
		t.Error("install.command loads the daemon before it clears the quarantine flag")
	}
}

// TestWindowsInstallerRegistersTheService proves the Windows installer by
// structure, because this host cannot run PowerShell.
func TestWindowsInstallerRegistersTheService(t *testing.T) {
	body := string(render(t)["install.ps1"])

	for _, want := range []string{
		"$ErrorActionPreference = 'Stop'",
		"WindowsBuiltInRole]::Administrator",
		"run this script as Administrator",
		`C:\ProgramData\nebula\meshy-homelab`,
		"-service install -config $Config",
		"-service start",
		"Get-Service",
		"Expand-Archive",
		"Get-FileHash -Algorithm SHA256",
		"https://example.invalid/download/v1.12.0",
		"nebula-linux-amd64.tar.gz",
		"Write-Output 'meshy-homelab is running'",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("install.ps1 does not hold %q", want)
		}
	}
}

func TestRenderUsesTheNodePlatformForTheBinary(t *testing.T) {
	v := values()
	v.Binary = "nebula.exe"
	v.Asset = "nebula-windows-amd64.zip"
	v.Destination = `C:\ProgramData\nebula\meshy-homelab`

	bodies, err := installers.Render(v)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if !strings.Contains(string(bodies["install.ps1"]), "nebula-windows-amd64.zip") {
		t.Error("install.ps1 does not name the Windows archive")
	}
}
