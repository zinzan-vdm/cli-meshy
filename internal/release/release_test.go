package release

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// archive builds the tarball that the project publishes, with the members the
// caller names.
func archive(t *testing.T, members map[string]string) []byte {
	t.Helper()

	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	for name, body := range members {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatalf("header %s: %v", name, err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatalf("body %s: %v", name, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	return buf.Bytes()
}

func digest(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

// checksums writes the file the project publishes, naming the archive and each
// member as archive/member.
func checksums(asset string, body []byte, members map[string]string) []byte {
	var b strings.Builder
	b.WriteString(digest(body) + "  " + asset + "\n")
	for name, member := range members {
		b.WriteString(digest([]byte(member)) + "  " + asset + "/" + name + "\n")
	}
	return []byte(b.String())
}

func published(t *testing.T) (asset string, body []byte, sums []byte) {
	t.Helper()
	asset = Asset("linux", "amd64")
	members := map[string]string{"nebula": "the nebula binary", "nebula-cert": "the signing tool"}
	body = archive(t, members)
	return asset, body, checksums(asset, body, members)
}

func serve(t *testing.T, files map[string][]byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := files[filepath.Base(r.URL.Path)]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func withBase(t *testing.T, url string) {
	t.Helper()
	was := BaseURL
	BaseURL = url
	t.Cleanup(func() { BaseURL = was })
}

func TestFetchKeepsOnlyTheBinary(t *testing.T) {
	asset, body, sums := published(t)
	srv := serve(t, map[string][]byte{asset: body, ChecksumFile: sums})
	withBase(t, srv.URL)

	dir := filepath.Join(t.TempDir(), "linux-amd64")
	path, err := Fetch(context.Background(), http.DefaultClient, "v1.12.0", "linux", "amd64", dir)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	if want := filepath.Join(dir, "nebula"); path != want {
		t.Errorf("path = %q, want %q", path, want)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the binary: %v", err)
	}
	if string(got) != "the nebula binary" {
		t.Errorf("the binary holds %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "nebula-cert")); err == nil {
		t.Error("the cache holds nebula-cert")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat the binary: %v", err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Errorf("mode = %o, want 755", info.Mode().Perm())
	}
}

func TestFetchRejectsAnArchiveThatDoesNotMatch(t *testing.T) {
	asset, body, sums := published(t)
	// The checksums name the real archive, and the server hands out another one.
	other := archive(t, map[string]string{"nebula": "a binary from somewhere else"})
	srv := serve(t, map[string][]byte{asset: other, ChecksumFile: sums})
	withBase(t, srv.URL)

	dir := filepath.Join(t.TempDir(), "linux-amd64")
	_, err := Fetch(context.Background(), http.DefaultClient, "v1.12.0", "linux", "amd64", dir)
	if err == nil {
		t.Fatal("Fetch accepted an archive that does not match")
	}
	if !strings.Contains(err.Error(), "checksum mismatch for "+asset) {
		t.Errorf("error = %v, want a checksum mismatch", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "nebula")); err == nil {
		t.Error("a rejected archive left a binary in the cache")
	}
	_ = body
}

func TestFetchReportsAReleaseThatDoesNotExist(t *testing.T) {
	srv := serve(t, map[string][]byte{})
	withBase(t, srv.URL)

	_, err := Fetch(context.Background(), http.DefaultClient, "v9.9.9", "linux", "amd64", t.TempDir())
	if err == nil {
		t.Fatal("Fetch found a release that does not exist")
	}
	if !strings.Contains(err.Error(), "no release v9.9.9") {
		t.Errorf("error = %v, want no release v9.9.9", err)
	}
}

func TestFetchKeepsABinaryThatAlreadyMatches(t *testing.T) {
	asset, body, sums := published(t)
	dir := filepath.Join(t.TempDir(), "linux-amd64")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "nebula"), []byte("the nebula binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	// The archive is absent, so a run that downloads it fails. Only the checksums
	// are served.
	srv := serve(t, map[string][]byte{ChecksumFile: sums})
	withBase(t, srv.URL)

	path, err := Fetch(context.Background(), http.DefaultClient, "v1.12.0", "linux", "amd64", dir)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if want := filepath.Join(dir, "nebula"); path != want {
		t.Errorf("path = %q, want %q", path, want)
	}
	_ = asset
	_ = body
}

func TestLocalMakesNoNetworkCall(t *testing.T) {
	asset, body, sums := published(t)
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, asset), body, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, ChecksumFile), sums, 0o644); err != nil {
		t.Fatal(err)
	}

	// A base URL that cannot answer proves the run reads the directory only.
	withBase(t, "http://127.0.0.1:1")

	dir := filepath.Join(t.TempDir(), "linux-amd64")
	path, err := Local(src, "v1.12.0", "linux", "amd64", dir)
	if err != nil {
		t.Fatalf("Local: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("the binary is not in the cache: %v", err)
	}
}

func TestLocalRejectsAnArchiveThatDoesNotMatch(t *testing.T) {
	asset, _, sums := published(t)
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, asset), archive(t, map[string]string{"nebula": "another build"}), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, ChecksumFile), sums, 0o644); err != nil {
		t.Fatal(err)
	}

	dir := filepath.Join(t.TempDir(), "linux-amd64")
	_, err := Local(src, "v1.12.0", "linux", "amd64", dir)
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("error = %v, want a checksum mismatch", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "nebula")); err == nil {
		t.Error("a rejected archive left a binary in the cache")
	}
}

func TestNormalizeAndAsset(t *testing.T) {
	if got := Normalize(" v1.12.0 "); got != "1.12.0" {
		t.Errorf("Normalize = %q, want 1.12.0", got)
	}
	if got := Asset("linux", "arm64"); got != "nebula-linux-arm64.tar.gz" {
		t.Errorf("Asset = %q", got)
	}
	for _, p := range [][2]string{{"linux", "amd64"}, {"linux", "arm64"}, {"darwin", "arm64"}} {
		if !Supported(p[0], p[1]) {
			t.Errorf("%s/%s is not supported", p[0], p[1])
		}
	}
	for _, p := range [][2]string{{"windows", "amd64"}, {"linux", "riscv64"}, {"linux", "386"}} {
		if Supported(p[0], p[1]) {
			t.Errorf("%s/%s is supported, and the release has no asset for it", p[0], p[1])
		}
	}
}

// TestLocalKeepsTheRealArchive reads the archive that the project publishes and
// runs the binary it finds. It runs only when MESHY_RELEASE_DIR names a
// directory that holds nebula-linux-amd64.tar.gz and SHASUM256.txt, so CI
// downloads nothing.
func TestLocalKeepsTheRealArchive(t *testing.T) {
	src := os.Getenv("MESHY_RELEASE_DIR")
	if src == "" {
		t.Skip("MESHY_RELEASE_DIR is not set")
	}

	dir := filepath.Join(t.TempDir(), "linux-amd64")
	path, err := Local(src, "v1.12.0", "linux", "amd64", dir)
	if err != nil {
		t.Fatalf("Local: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Size() < 10<<20 {
		t.Errorf("the binary is %d bytes, which is not the real one", info.Size())
	}

	out, err := exec.Command(path, "-version").Output()
	if err != nil {
		t.Fatalf("run the binary: %v", err)
	}
	if !strings.Contains(string(out), "1.12.0") {
		t.Errorf("the binary reports %q", out)
	}
}
