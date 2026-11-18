package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zinzan-vdm/cli-meshy/internal/config"
	"github.com/zinzan-vdm/cli-meshy/internal/nebula"
	"github.com/zinzan-vdm/cli-meshy/internal/release"
	"github.com/zinzan-vdm/cli-meshy/internal/vault"
)

// readArchive returns every member of an archive and the mode of each header.
func readArchive(t *testing.T, path string) (map[string][]byte, map[string]int64) {
	t.Helper()

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the archive: %v", err)
	}
	gz, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("the archive is not gzip: %v", err)
	}
	defer gz.Close()

	files := map[string][]byte{}
	modes := map[string]int64{}
	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("read a header: %v", err)
		}
		content, err := io.ReadAll(tr)
		if err != nil {
			t.Fatalf("read %s: %v", header.Name, err)
		}
		files[header.Name] = content
		modes[header.Name] = header.Mode
	}
	return files, modes
}

// cacheBinary puts the bytes that meshy install would have kept in the cache,
// with the checksum file that a fetch writes beside them.
func cacheBinary(t *testing.T, meshDir, osName, arch string, body []byte) string {
	t.Helper()

	dir := filepath.Join(meshDir, vault.DirName, "bin", "v1.12.0", osName+"-"+arch)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("create the cache: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, release.Binary), body, 0o755); err != nil {
		t.Fatalf("write the binary: %v", err)
	}
	sum := sha256.Sum256(body)
	line := hex.EncodeToString(sum[:]) + "  " + release.Asset(osName, arch) + "/" + release.Binary + "\n"
	if err := os.WriteFile(filepath.Join(dir, release.ChecksumFile), []byte(line), 0o644); err != nil {
		t.Fatalf("write the checksum: %v", err)
	}
	return dir
}

// archivePath is the path that bundle prints and writes.
func archivePath(meshDir, node string) string {
	return filepath.Join(meshDir, "bundles", "homelab", node+".tar.gz")
}

func TestBundleWritesOneArchiveWithEveryFile(t *testing.T) {
	dir := writeApplyMesh(t)
	openApplyVault(t, dir, "laptop-1")
	stubSigner(t)
	setExpiry(t, time.Now().Add(300*24*time.Hour))
	cacheBinary(t, dir, "darwin", "arm64", []byte("PINNED BINARY"))

	out, errOut, code := captureBoth(t, func() int { return run([]string{"bundle", "laptop-1", "--mesh", dir}) })
	if code != 0 {
		t.Fatalf("exit = %d, want 0: %s", code, errOut)
	}

	path := archivePath(dir, "laptop-1")
	if strings.TrimSpace(out) != path {
		t.Errorf("the command printed %q, want the path %q", out, path)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat the archive: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("the archive has mode %o, want 0600", info.Mode().Perm())
	}

	files, modes := readArchive(t, path)
	for _, name := range []string{
		"config.yml", "ca.crt", "laptop-1.crt", "laptop-1.key",
		"nebula", "install.sh", "install.command", "install.ps1", "README.txt", "blocklist.txt",
	} {
		if _, ok := files[name]; !ok {
			t.Errorf("the archive holds no %s", name)
		}
	}

	mesh, err := config.Load(dir)
	if err != nil {
		t.Fatalf("load the mesh: %v", err)
	}
	target, found := applyTarget(*mesh, "laptop-1")
	if !found {
		t.Fatal("the mesh holds no laptop-1")
	}
	want, err := nebula.Render(*mesh, target)
	if err != nil {
		t.Fatalf("render the config: %v", err)
	}
	if !bytes.Equal(files["config.yml"], want) {
		t.Error("config.yml differs from the rendered config")
	}

	if got := string(files["laptop-1.key"]); got != "laptop-1 key\n" {
		t.Errorf("the key holds %q", got)
	}
	if got := string(files["laptop-1.crt"]); got != "laptop-1 certificate\n" {
		t.Errorf("the certificate holds %q", got)
	}
	if got := string(files["ca.crt"]); got != "authority certificate\n" {
		t.Errorf("the authority holds %q", got)
	}
	if got := string(files["nebula"]); got != "PINNED BINARY" {
		t.Errorf("the binary holds %q, want the cached bytes", got)
	}

	if modes["laptop-1.key"] != 0o600 {
		t.Errorf("the key has mode %o, want 0600", modes["laptop-1.key"])
	}
	if modes["nebula"] != 0o755 {
		t.Errorf("the binary has mode %o, want 0755", modes["nebula"])
	}
	if modes["install.sh"] != 0o755 {
		t.Errorf("install.sh has mode %o, want 0755", modes["install.sh"])
	}
	if modes["install.command"] != 0o755 {
		t.Errorf("install.command has mode %o, want 0755", modes["install.command"])
	}

	if _, err := os.Stat(dir + "/bundles/homelab/laptop-1.tar.gz.tmp"); err == nil {
		t.Error("the command left a temporary archive")
	}
}

func TestBundleCarriesTheBlocklistThatTheConfigNames(t *testing.T) {
	dir := writeApplyMesh(t)
	openApplyVault(t, dir, "laptop-1")
	stubSigner(t)
	setExpiry(t, time.Now().Add(300*24*time.Hour))

	// A block that an earlier apply already recorded.
	sealed := filepath.Join(dir, vault.DirName)
	record, err := vault.Read(sealed)
	if err != nil {
		t.Fatalf("read the vault: %v", err)
	}
	vault.SetRevoked(record, "old-1", "4f3a")
	if err := vault.Write(dir, sealed, record); err != nil {
		t.Fatalf("write the vault: %v", err)
	}

	_, errOut, code := captureBoth(t, func() int { return run([]string{"bundle", "laptop-1", "--no-binary", "--mesh", dir}) })
	if code != 0 {
		t.Fatalf("exit = %d, want 0: %s", code, errOut)
	}

	files, _ := readArchive(t, archivePath(dir, "laptop-1"))
	if got := string(files["blocklist.txt"]); got != "4f3a\n" {
		t.Errorf("the blocklist holds %q, want the fingerprint", got)
	}
	if !strings.Contains(string(files["config.yml"]), "/etc/nebula/meshy-homelab/blocklist.txt") {
		t.Error("the config does not name the blocklist that the archive carries")
	}
	if !strings.Contains(string(files["install.sh"]), "blocklist.txt") {
		t.Error("install.sh does not place the blocklist")
	}
}

func TestBundleRefusesAServerNode(t *testing.T) {
	dir := writeApplyMesh(t)
	openApplyVault(t, dir, "web-1")

	_, errOut, code := captureBoth(t, func() int { return run([]string{"bundle", "web-1", "--mesh", dir}) })
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if !strings.Contains(errOut, "web-1 is a server node, apply configures it") {
		t.Errorf("the message is %q", errOut)
	}
	if _, err := os.Stat(dir + "/bundles"); err == nil {
		t.Error("a refused bundle wrote an archive")
	}
}

func TestBundleRefusesALockedVault(t *testing.T) {
	dir := writeApplyMesh(t)

	_, errOut, code := captureBoth(t, func() int { return run([]string{"bundle", "laptop-1", "--mesh", dir}) })
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if !strings.Contains(errOut, "the vault is locked, run meshy unlock") {
		t.Errorf("the message is %q", errOut)
	}
}

func TestBundleRefusesANodeThatIsNotInTheMesh(t *testing.T) {
	dir := writeApplyMesh(t)
	openApplyVault(t, dir, "laptop-1")

	_, errOut, code := captureBoth(t, func() int { return run([]string{"bundle", "phone-1", "--mesh", dir}) })
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if !strings.Contains(errOut, "phone-1") {
		t.Errorf("the message is %q", errOut)
	}
}

func TestBundleNeedsANodeName(t *testing.T) {
	dir := writeApplyMesh(t)

	_, errOut, code := captureBoth(t, func() int { return run([]string{"bundle", "--mesh", dir}) })
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if !strings.Contains(errOut, "name a node") {
		t.Errorf("the message is %q", errOut)
	}
}

func TestBundleRefusesAMeshWithProblems(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, config.FileName), []byte("schema: 1\nname: homelab\n"), 0o600); err != nil {
		t.Fatalf("write the mesh: %v", err)
	}

	_, errOut, code := captureBoth(t, func() int { return run([]string{"bundle", "laptop-1", "--mesh", dir}) })
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if !strings.Contains(errOut, "problems") {
		t.Errorf("the message is %q", errOut)
	}
}

func TestBundleRenewsACertificateInsideTheWindow(t *testing.T) {
	dir := writeApplyMesh(t)
	openApplyVault(t, dir, "laptop-1")
	stubSigner(t)
	setExpiry(t, time.Now().Add(10*24*time.Hour))
	cacheBinary(t, dir, "darwin", "arm64", []byte("PINNED BINARY"))

	_, errOut, code := captureBoth(t, func() int { return run([]string{"bundle", "laptop-1", "--mesh", dir}) })
	if code != 0 {
		t.Fatalf("exit = %d, want 0: %s", code, errOut)
	}

	files, _ := readArchive(t, archivePath(dir, "laptop-1"))
	if !strings.Contains(string(files["laptop-1.crt"]), "STUB CRT") {
		t.Error("the archive holds no renewed certificate")
	}

	record, err := vault.Read(filepath.Join(dir, vault.DirName))
	if err != nil {
		t.Fatalf("read the vault: %v", err)
	}
	cert, _, found := vault.Certificate(record, "laptop-1")
	if !found || !strings.Contains(string(cert), "STUB CRT") {
		t.Error("the vault does not hold the certificate that the archive carries")
	}
}

func TestBundleLeavesACertificateOutsideTheWindow(t *testing.T) {
	dir := writeApplyMesh(t)
	openApplyVault(t, dir, "laptop-1")
	stubSigner(t)
	setExpiry(t, time.Now().Add(300*24*time.Hour))
	cacheBinary(t, dir, "darwin", "arm64", []byte("PINNED BINARY"))

	_, errOut, code := captureBoth(t, func() int { return run([]string{"bundle", "laptop-1", "--mesh", dir}) })
	if code != 0 {
		t.Fatalf("exit = %d, want 0: %s", code, errOut)
	}

	files, _ := readArchive(t, archivePath(dir, "laptop-1"))
	if got := string(files["laptop-1.crt"]); got != "laptop-1 certificate\n" {
		t.Errorf("the archive holds %q, want the certificate from the vault", got)
	}
}

func TestBundleNoBinaryLeavesTheBinaryOut(t *testing.T) {
	dir := writeApplyMesh(t)
	openApplyVault(t, dir, "laptop-1")
	stubSigner(t)
	setExpiry(t, time.Now().Add(300*24*time.Hour))

	_, errOut, code := captureBoth(t, func() int { return run([]string{"bundle", "laptop-1", "--no-binary", "--mesh", dir}) })
	if code != 0 {
		t.Fatalf("exit = %d, want 0: %s", code, errOut)
	}

	files, _ := readArchive(t, archivePath(dir, "laptop-1"))
	if _, ok := files["nebula"]; ok {
		t.Error("--no-binary wrote a binary")
	}
	install := string(files["install.sh"])
	if !strings.Contains(install, release.BaseURL+"/v1.12.0/"+release.ChecksumFile) {
		t.Error("install.sh does not fetch the checksum file")
	}
	if !strings.Contains(install, release.Asset("darwin", "arm64")) {
		t.Error("install.sh does not name the archive for the node")
	}
	if !strings.Contains(install, "sha256sum") {
		t.Error("install.sh does not check the checksum")
	}
}

func TestBundleFetchesTheBinaryWhenTheCacheIsEmpty(t *testing.T) {
	dir := writeApplyMesh(t)
	openApplyVault(t, dir, "laptop-1")
	stubSigner(t)
	setExpiry(t, time.Now().Add(300*24*time.Hour))

	binary := []byte("DOWNLOADED NEBULA")
	archive := tarGz(t, release.Binary, binary)
	asset := release.Asset("darwin", "arm64")
	sum := sha256.Sum256(archive)
	memberSum := sha256.Sum256(binary)
	sums := hex.EncodeToString(sum[:]) + "  " + asset + "\n" +
		hex.EncodeToString(memberSum[:]) + "  " + asset + "/" + release.Binary + "\n"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch filepath.Base(r.URL.Path) {
		case release.ChecksumFile:
			io.WriteString(w, sums)
		case asset:
			w.Write(archive)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	old := release.BaseURL
	release.BaseURL = server.URL
	t.Cleanup(func() { release.BaseURL = old })

	_, errOut, code := captureBoth(t, func() int { return run([]string{"bundle", "laptop-1", "--mesh", dir}) })
	if code != 0 {
		t.Fatalf("exit = %d, want 0: %s", code, errOut)
	}

	files, _ := readArchive(t, archivePath(dir, "laptop-1"))
	if got := string(files["nebula"]); got != string(binary) {
		t.Errorf("the archive holds %q, want the fetched bytes", got)
	}

	kept := filepath.Join(dir, vault.DirName, "bin", "v1.12.0", "darwin-arm64", release.Binary)
	if _, err := os.Stat(kept); err != nil {
		t.Errorf("the fetch did not fill the cache: %v", err)
	}
}

func TestBundleRefusesAPlatformWithoutARelease(t *testing.T) {
	dir := t.TempDir()
	body := strings.Replace(issueMesh, "platform: darwin/arm64", "platform: windows/amd64", 1)
	if err := os.WriteFile(filepath.Join(dir, config.FileName), []byte(body), 0o600); err != nil {
		t.Fatalf("write the mesh: %v", err)
	}
	openApplyVault(t, dir, "laptop-1")
	stubSigner(t)
	setExpiry(t, time.Now().Add(300*24*time.Hour))

	_, errOut, code := captureBoth(t, func() int { return run([]string{"bundle", "laptop-1", "--mesh", dir}) })
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if !strings.Contains(errOut, "--no-binary") {
		t.Errorf("the message does not name the flag: %q", errOut)
	}
}

func TestBundleIsInTheDispatchAndTheUsage(t *testing.T) {
	out, _, code := captureBoth(t, func() int { return run([]string{"help"}) })
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.Contains(out, "bundle") {
		t.Errorf("the usage does not list bundle:\n%s", out)
	}
	if strings.Contains(out, "bundle, status") {
		t.Error("the usage still lists bundle as planned")
	}
}

func TestBundleRefusesAStrayArgument(t *testing.T) {
	dir := writeApplyMesh(t)

	_, errOut, code := captureBoth(t, func() int { return run([]string{"bundle", "laptop-1", "extra", "--mesh", dir}) })
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if !strings.Contains(errOut, "unexpected argument") {
		t.Errorf("the message is %q", errOut)
	}
}

// tarGz builds a one member archive, which is the shape of a release asset.
func tarGz(t *testing.T, name string, body []byte) []byte {
	t.Helper()

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatalf("write a header: %v", err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatalf("write a member: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close the tar: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close the gzip: %v", err)
	}
	return buf.Bytes()
}
