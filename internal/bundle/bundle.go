// Package bundle builds the one archive that a user node joins with.
package bundle

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Names of the files that every archive holds.
const (
	ConfigFile    = "config.yml"
	AuthorityFile = "ca.crt"
	BlocklistFile = "blocklist.txt"
	ReadmeFile    = "README.txt"
	ShScript      = "install.sh"
	CommandScript = "install.command"
	PowerShell    = "install.ps1"
)

// Archive is every file that one archive holds. A nil Binary leaves the binary
// out, which is the --no-binary form.
type Archive struct {
	Mesh       string
	Node       string
	Config     []byte
	Authority  []byte
	Cert       []byte
	Key        []byte
	Blocklist  []byte
	Binary     []byte
	BinaryName string
	Installers map[string][]byte
	Readme     []byte
}

// Build returns the bytes of the gzip archive, flat at the top level. It writes
// nothing, so the whole archive exists before a disk sees it.
func Build(a Archive) ([]byte, error) {
	if a.Mesh == "" || a.Node == "" {
		return nil, fmt.Errorf("an archive needs a mesh name and a node name")
	}

	files := []file{
		{name: BlocklistFile, mode: 0o644, body: a.Blocklist},
		{name: ConfigFile, mode: 0o600, body: a.Config},
		{name: AuthorityFile, mode: 0o644, body: a.Authority},
		{name: a.Node + ".crt", mode: 0o644, body: a.Cert},
		{name: a.Node + ".key", mode: 0o600, body: a.Key},
	}
	if a.Binary != nil {
		files = append(files, file{name: a.BinaryName, mode: 0o755, body: a.Binary})
	}
	for _, name := range installerNames() {
		body, ok := a.Installers[name]
		if !ok {
			return nil, fmt.Errorf("the archive holds no %s", name)
		}
		mode := int64(0o644)
		if name != PowerShell {
			mode = 0o755
		}
		files = append(files, file{name: name, mode: mode, body: body})
	}
	files = append(files, file{name: ReadmeFile, mode: 0o644, body: a.Readme})

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, f := range files {
		header := &tar.Header{
			Name:     f.name,
			Mode:     f.mode,
			Size:     int64(len(f.body)),
			Typeflag: tar.TypeReg,
			ModTime:  time.Unix(0, 0),
		}
		if err := tw.WriteHeader(header); err != nil {
			return nil, fmt.Errorf("write the header of %s: %w", f.name, err)
		}
		if _, err := tw.Write(f.body); err != nil {
			return nil, fmt.Errorf("write %s: %w", f.name, err)
		}
	}
	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("close the archive: %w", err)
	}
	if err := gz.Close(); err != nil {
		return nil, fmt.Errorf("close the archive: %w", err)
	}
	return buf.Bytes(), nil
}

// Write puts the archive at path with mode 0600 in one call. A failed write
// removes the file, so a partial archive never stays on disk.
func Write(path string, body []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		os.Remove(path)
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("set the mode of %s: %w", path, err)
	}
	return nil
}

// Readme returns the five lines that a person reads first.
func Readme(mesh, node string) []byte {
	return []byte(fmt.Sprintf(`Unpack this archive where you can find it: tar xzf %[2]s.tar.gz
Run the installer for this machine: %[3]s on Linux, %[4]s on macOS, %[5]s on Windows.
The installer needs root. On Linux and macOS run it with sudo. On Windows use PowerShell as administrator.
Choose "Run anyway" if Windows says the installer is unsigned.
Wait for the line meshy-%[1]s is running. The machine is then on the mesh.
`, mesh, node, ShScript, CommandScript, PowerShell))
}

// installerNames is the order that the installers take in the archive. The
// order is fixed, so the same input makes the same bytes.
func installerNames() []string {
	return []string{CommandScript, PowerShell, ShScript}
}

type file struct {
	name string
	mode int64
	body []byte
}
