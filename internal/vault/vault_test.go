package vault

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sealedMesh(t *testing.T, record Record) (string, []string) {
	t.Helper()

	plaintext, err := Encode(record)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	ciphertext, shares, err := Seal(plaintext)
	if err != nil {
		t.Fatalf("Seal failed: %v", err)
	}

	meshDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(meshDir, FileName), ciphertext, 0o600); err != nil {
		t.Fatalf("write the vault: %v", err)
	}

	return meshDir, shares
}

func TestSealAndOpenWithThreeShares(t *testing.T) {
	plaintext := []byte("ca-1 key material")

	ciphertext, shares, err := Seal(plaintext)
	if err != nil {
		t.Fatalf("Seal failed: %v", err)
	}
	if len(shares) != ShareCount {
		t.Fatalf("Seal returned %d shares, want %d", len(shares), ShareCount)
	}

	for _, chosen := range [][]string{
		{shares[0], shares[1], shares[2]},
		{shares[4], shares[0], shares[2]},
	} {
		opened, err := Open(ciphertext, chosen)
		if err != nil {
			t.Fatalf("Open with three shares failed: %v", err)
		}
		if !bytes.Equal(opened, plaintext) {
			t.Fatalf("Open returned %q, want %q", opened, plaintext)
		}
	}
}

func TestOpenNeedsThreeShares(t *testing.T) {
	ciphertext, shares, err := Seal([]byte("payload"))
	if err != nil {
		t.Fatalf("Seal failed: %v", err)
	}

	_, err = Open(ciphertext, shares[:2])
	if err == nil {
		t.Fatal("Open with two shares succeeded")
	}
	if want := "need 3 shares, got 2"; err.Error() != want {
		t.Fatalf("error = %q, want %q", err, want)
	}
}

func TestOpenRejectsSharesFromAnotherVault(t *testing.T) {
	ciphertext, _, err := Seal([]byte("payload"))
	if err != nil {
		t.Fatalf("Seal failed: %v", err)
	}
	_, other, err := Seal([]byte("payload"))
	if err != nil {
		t.Fatalf("Seal failed: %v", err)
	}

	_, err = Open(ciphertext, other[:3])
	if !errors.Is(err, ErrWrongShares) {
		t.Fatalf("error = %v, want ErrWrongShares", err)
	}
}

func TestOpenRejectsSharesThatDoNotCombine(t *testing.T) {
	ciphertext, shares, err := Seal([]byte("payload"))
	if err != nil {
		t.Fatalf("Seal failed: %v", err)
	}

	for name, chosen := range map[string][]string{
		"the same share twice": {shares[0], shares[0], shares[1]},
		"a truncated share":    {shares[0], shares[1], "meshy-3-c2hvcnQ"},
	} {
		if _, err := Open(ciphertext, chosen); !errors.Is(err, ErrWrongShares) {
			t.Fatalf("%s: error = %v, want ErrWrongShares", name, err)
		}
	}
}

func TestOpenRejectsAShareThatIsNotMeshy(t *testing.T) {
	ciphertext, shares, err := Seal([]byte("payload"))
	if err != nil {
		t.Fatalf("Seal failed: %v", err)
	}

	_, err = Open(ciphertext, []string{"hunter2", shares[1], shares[2]})
	if err == nil {
		t.Fatal("Open accepted a share that is not in the meshy format")
	}
	if !strings.Contains(err.Error(), "share 1") {
		t.Fatalf("error = %q, want it to name share 1", err)
	}
}

func TestShareNamesItsIndex(t *testing.T) {
	_, shares, err := Seal([]byte("payload"))
	if err != nil {
		t.Fatalf("Seal failed: %v", err)
	}

	for i, share := range shares {
		prefix := fmt.Sprintf("%s-%d-", sharePrefix, i+1)
		if !strings.HasPrefix(share, prefix) {
			t.Fatalf("share %d = %q, want the prefix %q", i+1, share, prefix)
		}
		if _, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(share, prefix)); err != nil {
			t.Fatalf("share %d does not hold base64: %v", i+1, err)
		}
	}
}

func TestUnlockWritesTheRecord(t *testing.T) {
	meshDir, shares := sealedMesh(t, Record{
		"ca-1.key": []byte("the authority key"),
		"ca-1.crt": []byte("the authority certificate"),
	})

	dir, err := Unlock(meshDir, shares[:3], false)
	if err != nil {
		t.Fatalf("Unlock failed: %v", err)
	}
	if want := filepath.Join(meshDir, DirName); dir != want {
		t.Fatalf("Unlock returned %q, want %q", dir, want)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat %s: %v", dir, err)
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Fatalf("directory mode = %o, want 700", perm)
	}

	for name, want := range map[string]string{
		"ca-1.key": "the authority key",
		"ca-1.crt": "the authority certificate",
	} {
		path := filepath.Join(dir, name)
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %s: %v", path, err)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Fatalf("%s mode = %o, want 600", name, perm)
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if string(got) != want {
			t.Fatalf("%s = %q, want %q", name, got, want)
		}
	}
}

func TestUnlockRefusesAnOpenVault(t *testing.T) {
	meshDir, shares := sealedMesh(t, Record{"ca-1.key": []byte("key")})

	if _, err := Unlock(meshDir, shares[:3], false); err != nil {
		t.Fatalf("first Unlock failed: %v", err)
	}

	_, err := Unlock(meshDir, shares[:3], false)
	if err == nil {
		t.Fatal("the second Unlock succeeded without --force")
	}
	if !strings.Contains(err.Error(), "already holds plaintext") {
		t.Fatalf("error = %q, want it to report plaintext", err)
	}

	if _, err := Unlock(meshDir, shares[:3], true); err != nil {
		t.Fatalf("Unlock with force failed: %v", err)
	}
}

func TestUnlockAcceptsAnEmptyDirectory(t *testing.T) {
	meshDir, shares := sealedMesh(t, Record{"ca-1.key": []byte("key")})
	if err := os.Mkdir(filepath.Join(meshDir, DirName), 0o700); err != nil {
		t.Fatalf("create the empty directory: %v", err)
	}

	if _, err := Unlock(meshDir, shares[:3], false); err != nil {
		t.Fatalf("Unlock refused an empty directory: %v", err)
	}
}

func TestUnlockNeedsAVaultFile(t *testing.T) {
	_, shares, err := Seal([]byte("payload"))
	if err != nil {
		t.Fatalf("Seal failed: %v", err)
	}

	_, err = Unlock(t.TempDir(), shares[:3], false)
	if err == nil {
		t.Fatal("Unlock succeeded without a vault file")
	}
	if !strings.Contains(err.Error(), FileName) {
		t.Fatalf("error = %q, want it to name %s", err, FileName)
	}
}

func TestUnlockRejectsANameOutsideTheDirectory(t *testing.T) {
	meshDir := t.TempDir()
	ciphertext, shares, err := Seal([]byte(`{"../escape":"a2V5"}`))
	if err != nil {
		t.Fatalf("Seal failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(meshDir, FileName), ciphertext, 0o600); err != nil {
		t.Fatalf("write the vault: %v", err)
	}

	if _, err := Unlock(meshDir, shares[:3], false); err == nil {
		t.Fatal("Unlock accepted a record that writes outside .meshy")
	}
	if _, err := os.Stat(filepath.Join(meshDir, "escape")); !os.IsNotExist(err) {
		t.Fatalf("Unlock wrote outside .meshy: %v", err)
	}
}

func TestLockRemovesThePlaintext(t *testing.T) {
	meshDir, shares := sealedMesh(t, Record{"ca-1.key": []byte("key")})
	dir, err := Unlock(meshDir, shares[:3], false)
	if err != nil {
		t.Fatalf("Unlock failed: %v", err)
	}

	if err := Lock(meshDir); err != nil {
		t.Fatalf("Lock failed: %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("%s still exists after Lock: %v", dir, err)
	}

	if err := Lock(meshDir); err != nil {
		t.Fatalf("a second Lock failed: %v", err)
	}
}

func TestRecordRoundTrip(t *testing.T) {
	record := Record{"ca-1.key": []byte("key"), "ca-1.crt": []byte("crt")}

	data, err := Encode(record)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	decoded, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if len(decoded) != len(record) {
		t.Fatalf("Decode returned %d entries, want %d", len(decoded), len(record))
	}
	for name, want := range record {
		if !bytes.Equal(decoded[name], want) {
			t.Fatalf("%s = %q, want %q", name, decoded[name], want)
		}
	}
}

func TestDecodeRejectsAnEmptyRecord(t *testing.T) {
	if _, err := Decode([]byte(`{}`)); err == nil {
		t.Fatal("Decode accepted an empty record")
	}
}
