// Package release fetches a pinned Nebula release and keeps only the binary.
package release

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// BaseURL is where the project publishes every release.
var BaseURL = "https://github.com/slackhq/nebula/releases/download"

const (
	// ChecksumFile is the name the project gives the checksums of a release.
	ChecksumFile = "SHASUM256.txt"
	// Binary is the only member of an archive that a node runs.
	Binary = "nebula"
	// maxArchive stops a download that is not the file we asked for.
	maxArchive = 64 << 20
)

// Asset names the archive for one platform.
func Asset(goos, goarch string) string {
	return "nebula-" + goos + "-" + goarch + ".tar.gz"
}

// Normalize drops the leading v, so a pin and a node agree on the release.
func Normalize(version string) string {
	return strings.TrimPrefix(strings.TrimSpace(version), "v")
}

// Supported reports whether the project publishes an archive for a platform.
func Supported(goos, goarch string) bool {
	if goos != "linux" && goos != "darwin" {
		return false
	}
	return goarch == "amd64" || goarch == "arm64"
}

// Checksums reads SHASUM256.txt into a map from name to digest. The file names
// the archive and each member as archive/member.
func Checksums(r io.Reader) (map[string]string, error) {
	sums := map[string]string{}
	lines := bufio.NewScanner(r)
	for lines.Scan() {
		line := strings.TrimSpace(lines.Text())
		if line == "" {
			continue
		}
		f := strings.Fields(line)
		if len(f) != 2 || len(f[0]) != sha256.Size*2 {
			return nil, fmt.Errorf("checksum line %q is not a digest and a name", line)
		}
		sums[strings.TrimPrefix(f[1], "*")] = strings.ToLower(f[0])
	}
	if err := lines.Err(); err != nil {
		return nil, err
	}
	if len(sums) == 0 {
		return nil, fmt.Errorf("the checksum file is empty")
	}
	return sums, nil
}

// Fetch downloads the archive and the checksums of one platform, verifies the
// archive, and writes the binary into dir. A binary that already matches the
// published digest is kept and nothing is downloaded but the checksums.
func Fetch(ctx context.Context, c *http.Client, version, goos, goarch, dir string) (string, error) {
	asset := Asset(goos, goarch)

	sums, err := fetchChecksums(ctx, c, version, dir)
	if err != nil {
		return "", err
	}
	if path, ok := cached(dir, asset, sums); ok {
		return path, nil
	}

	archive, err := get(ctx, c, version, asset, asset)
	if err != nil {
		return "", err
	}
	return place(archive, sums, asset, dir)
}

// Local does what Fetch does, from a directory that holds the archive and the
// checksums. It makes no network call.
func Local(srcDir, version, goos, goarch, dir string) (string, error) {
	asset := Asset(goos, goarch)

	body, err := os.ReadFile(filepath.Join(srcDir, ChecksumFile))
	if err != nil {
		return "", fmt.Errorf("release %s: %w", version, err)
	}
	sums, err := Checksums(bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("release %s: %w", version, err)
	}
	if path, ok := cached(dir, asset, sums); ok {
		return path, nil
	}

	archive, err := os.ReadFile(filepath.Join(srcDir, asset))
	if err != nil {
		return "", fmt.Errorf("release %s: %w", version, err)
	}
	return place(archive, sums, asset, dir)
}

// fetchChecksums keeps the published checksums beside the binary, so a later
// run can tell whether the cache still matches without a download.
func fetchChecksums(ctx context.Context, c *http.Client, version, dir string) (map[string]string, error) {
	body, err := get(ctx, c, version, ChecksumFile, ChecksumFile)
	if err != nil {
		return nil, err
	}
	sums, err := Checksums(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("release %s: %w", version, err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("release %s: %w", version, err)
	}
	if err := os.WriteFile(filepath.Join(dir, ChecksumFile), body, 0o644); err != nil {
		return nil, fmt.Errorf("release %s: %w", version, err)
	}
	return sums, nil
}

// get reads one file of a release. A missing archive means the release does not
// exist, which is the error the caller can act on.
func get(ctx context.Context, c *http.Client, version, name, asset string) ([]byte, error) {
	url := BaseURL + "/" + version + "/" + name
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("release %s: %w", version, err)
	}

	res, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("release %s: %w", version, err)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusNotFound {
		// Every release publishes its checksums, so a missing file means the
		// release itself is not there.
		return nil, fmt.Errorf("no release %s", version)
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("release %s: %s: %s", version, name, res.Status)
	}

	body, err := io.ReadAll(io.LimitReader(res.Body, maxArchive+1))
	if err != nil {
		return nil, fmt.Errorf("release %s: %s: %w", version, name, err)
	}
	if len(body) > maxArchive {
		return nil, fmt.Errorf("release %s: %s is larger than %d bytes", version, name, maxArchive)
	}
	return body, nil
}

// place checks the archive against the published digest before anything reaches
// the cache, then keeps only the binary.
func place(archive []byte, sums map[string]string, asset, dir string) (string, error) {
	want, ok := sums[asset]
	if !ok {
		return "", fmt.Errorf("checksums have no line for %s", asset)
	}
	sum := sha256.Sum256(archive)
	if got := hex.EncodeToString(sum[:]); got != want {
		return "", fmt.Errorf("checksum mismatch for %s", asset)
	}
	return unpack(archive, sums, asset, dir)
}

// unpack writes the binary out of the archive and checks it against the digest
// the project publishes for that member.
func unpack(archive []byte, sums map[string]string, asset, dir string) (string, error) {
	want, ok := sums[asset+"/"+Binary]
	if !ok {
		return "", fmt.Errorf("checksums have no line for %s/%s", asset, Binary)
	}

	body, err := member(archive, Binary)
	if err != nil {
		return "", fmt.Errorf("%s: %w", asset, err)
	}
	sum := sha256.Sum256(body)
	if got := hex.EncodeToString(sum[:]); got != want {
		return "", fmt.Errorf("checksum mismatch for %s/%s", asset, Binary)
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(dir, Binary)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, body, 0o755); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return "", err
	}
	return path, nil
}

// member reads one file out of a tar.gz archive.
func member(archive []byte, name string) ([]byte, error) {
	zr, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, err
	}
	defer zr.Close()

	tr := tar.NewReader(zr)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil, fmt.Errorf("the archive holds no %s", name)
		}
		if err != nil {
			return nil, err
		}
		if strings.TrimPrefix(h.Name, "./") != name || h.Typeflag != tar.TypeReg {
			continue
		}
		body, err := io.ReadAll(io.LimitReader(tr, maxArchive+1))
		if err != nil {
			return nil, err
		}
		if len(body) > maxArchive {
			return nil, fmt.Errorf("%s is larger than %d bytes", name, maxArchive)
		}
		return body, nil
	}
}

// cached reports the binary in dir when it matches the published digest.
func cached(dir, asset string, sums map[string]string) (string, bool) {
	want, ok := sums[asset+"/"+Binary]
	if !ok {
		return "", false
	}
	body, err := os.ReadFile(filepath.Join(dir, Binary))
	if err != nil {
		return "", false
	}
	sum := sha256.Sum256(body)
	if hex.EncodeToString(sum[:]) != want {
		return "", false
	}
	return filepath.Join(dir, Binary), true
}
