package vault

import (
	"os"
	"path/filepath"
	"testing"

	"filippo.io/age"
)

// writeTestVault seals a record into meshDir and leaves the plaintext in
// meshDir/.meshy, the state a command finds after unlock.
func writeTestVault(t *testing.T, meshDir string, record Record) string {
	t.Helper()

	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("generate identity: %v", err)
	}
	plaintext, err := Encode(record)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	sealed, err := encrypt(plaintext, identity.Recipient())
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(meshDir, FileName), sealed, 0o600); err != nil {
		t.Fatalf("write sealed file: %v", err)
	}

	dir := filepath.Join(meshDir, DirName)
	if err := writeRecord(dir, record); err != nil {
		t.Fatalf("write record: %v", err)
	}
	if err := writeIdentity(dir, identity); err != nil {
		t.Fatalf("write identity: %v", err)
	}

	return dir
}

func TestWriteKeepsTheToolCache(t *testing.T) {
	meshDir := t.TempDir()
	dir := writeTestVault(t, meshDir, Record{"ca-1.key": []byte("key"), "ca-1.crt": []byte("cert")})

	cache := filepath.Join(dir, "bin", "1.12.0", "linux-amd64")
	if err := os.MkdirAll(cache, 0o700); err != nil {
		t.Fatalf("create cache: %v", err)
	}
	tool := filepath.Join(cache, "nebula-cert")
	if err := os.WriteFile(tool, []byte("binary"), 0o755); err != nil {
		t.Fatalf("write tool: %v", err)
	}

	record := Record{"ca-1.key": []byte("key"), "ca-1.crt": []byte("cert"), "core-1.crt": []byte("c"), "core-1.key": []byte("k")}
	if err := Write(meshDir, dir, record); err != nil {
		t.Fatalf("write: %v", err)
	}

	if _, err := os.Stat(tool); err != nil {
		t.Errorf("the tool cache did not survive the write: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "core-1.key")); err != nil {
		t.Errorf("the record file is missing: %v", err)
	}
}

func TestWriteDropsAFileTheRecordNoLongerNames(t *testing.T) {
	meshDir := t.TempDir()
	dir := writeTestVault(t, meshDir, Record{"ca-1.key": []byte("key"), "old.key": []byte("stale")})

	if err := Write(meshDir, dir, Record{"ca-1.key": []byte("key")}); err != nil {
		t.Fatalf("write: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "old.key")); !os.IsNotExist(err) {
		t.Errorf("a stale record file survived the write: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, IdentityFile)); err != nil {
		t.Errorf("the identity did not survive the write: %v", err)
	}
}

func TestWriteModesArePrivate(t *testing.T) {
	meshDir := t.TempDir()
	dir := writeTestVault(t, meshDir, Record{"ca-1.key": []byte("key")})

	if err := Write(meshDir, dir, Record{"core-1.key": []byte("k")}); err != nil {
		t.Fatalf("write: %v", err)
	}

	info, err := os.Stat(filepath.Join(dir, "core-1.key"))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("mode = %o, want 600", perm)
	}
}

func TestLockRemovesTheToolCache(t *testing.T) {
	meshDir := t.TempDir()
	dir := writeTestVault(t, meshDir, Record{"ca-1.key": []byte("key")})

	cache := filepath.Join(dir, "bin", "1.12.0")
	if err := os.MkdirAll(cache, 0o700); err != nil {
		t.Fatalf("create cache: %v", err)
	}

	if err := Lock(meshDir); err != nil {
		t.Fatalf("lock: %v", err)
	}

	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("lock left the plaintext directory behind: %v", err)
	}
}
