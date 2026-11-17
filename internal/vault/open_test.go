package vault

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteKeepsTheSharesThatOpenedTheVault(t *testing.T) {
	meshDir, shares := sealedMesh(t, Record{
		"ca-1.key": []byte("the authority key"),
		"ca-1.crt": []byte("the authority certificate"),
	})

	dir, err := Unlock(meshDir, shares[:3], false)
	if err != nil {
		t.Fatalf("Unlock failed: %v", err)
	}

	record, err := Read(dir)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	SetCertificate(record, "core-1", []byte("the node certificate"), []byte("the node key"))
	if err := Write(meshDir, dir, record); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	ciphertext, err := os.ReadFile(filepath.Join(meshDir, FileName))
	if err != nil {
		t.Fatalf("read the vault: %v", err)
	}
	plaintext, err := Open(ciphertext, shares[:3])
	if err != nil {
		t.Fatalf("the shares that opened the vault no longer open it: %v", err)
	}

	decoded, err := Decode(plaintext)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	for name, want := range map[string]string{
		"ca-1.key":   "the authority key",
		"ca-1.crt":   "the authority certificate",
		"core-1.key": "the node key",
		"core-1.crt": "the node certificate",
	} {
		if string(decoded[name]) != want {
			t.Errorf("%s = %q, want %q", name, decoded[name], want)
		}
	}
}

func TestReadHoldsTheRecordAndNotTheKey(t *testing.T) {
	meshDir, shares := sealedMesh(t, Record{
		"ca-1.key": []byte("the authority key"),
		"ca-1.crt": []byte("the authority certificate"),
	})

	dir, err := Unlock(meshDir, shares[:3], false)
	if err != nil {
		t.Fatalf("Unlock failed: %v", err)
	}

	record, err := Read(dir)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if len(record) != 2 {
		t.Fatalf("Read returned %d entries, want 2: %v", len(record), record)
	}
	if _, found := record[IdentityFile]; found {
		t.Errorf("Read returned the key that seals the vault")
	}
}

func TestReadRefusesALockedVault(t *testing.T) {
	_, err := Read(filepath.Join(t.TempDir(), DirName))
	if !errors.Is(err, ErrLocked) {
		t.Fatalf("error = %v, want ErrLocked", err)
	}
}

func TestWriteRefusesALockedVault(t *testing.T) {
	meshDir, shares := sealedMesh(t, Record{"ca-1.key": []byte("key")})
	dir, err := Unlock(meshDir, shares[:3], false)
	if err != nil {
		t.Fatalf("Unlock failed: %v", err)
	}
	if err := Lock(meshDir); err != nil {
		t.Fatalf("Lock failed: %v", err)
	}

	if err := Write(meshDir, dir, Record{"ca-1.key": []byte("key")}); !errors.Is(err, ErrLocked) {
		t.Fatalf("error = %v, want ErrLocked", err)
	}
}

func TestWriteLeavesTheOpenVaultReadable(t *testing.T) {
	meshDir, shares := sealedMesh(t, Record{"ca-1.key": []byte("the authority key")})
	dir, err := Unlock(meshDir, shares[:3], false)
	if err != nil {
		t.Fatalf("Unlock failed: %v", err)
	}

	record, err := Read(dir)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	SetCertificate(record, "core-1", []byte("cert"), []byte("key"))
	if err := Write(meshDir, dir, record); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	again, err := Read(dir)
	if err != nil {
		t.Fatalf("Read after Write failed: %v", err)
	}
	if _, _, found := Certificate(again, "core-1"); !found {
		t.Errorf("the open vault lost the node entry")
	}
}

func TestUnlockWritesTheKeyThatSealsTheVault(t *testing.T) {
	meshDir, shares := sealedMesh(t, Record{"ca-1.key": []byte("key")})
	dir, err := Unlock(meshDir, shares[:3], false)
	if err != nil {
		t.Fatalf("Unlock failed: %v", err)
	}

	path := filepath.Join(dir, IdentityFile)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("%s mode = %o, want 600", IdentityFile, perm)
	}
}

func TestCertificateNeedsBothEntries(t *testing.T) {
	record := Record{"core-1.crt": []byte("cert")}
	if _, _, found := Certificate(record, "core-1"); found {
		t.Error("a record with no key reported a certificate")
	}

	SetCertificate(record, "core-1", []byte("cert"), []byte("key"))
	cert, key, found := Certificate(record, "core-1")
	if !found || string(cert) != "cert" || string(key) != "key" {
		t.Errorf("Certificate = %q, %q, %v", cert, key, found)
	}
}

func TestEncodeRejectsTheReservedName(t *testing.T) {
	if _, err := Encode(Record{IdentityFile: []byte("a key")}); err == nil {
		t.Fatal("Encode accepted the name of the file that seals the vault")
	}
}
