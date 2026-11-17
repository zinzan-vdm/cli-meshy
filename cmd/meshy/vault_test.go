package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/zinzan-vdm/cli-meshy/internal/vault"
)

func meshWithVault(t *testing.T) (string, []string) {
	t.Helper()

	plaintext, err := vault.Encode(vault.Record{
		"ca-1.key": []byte("the authority key"),
		"ca-1.crt": []byte("the authority certificate"),
	})
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	ciphertext, shares, err := vault.Seal(plaintext)
	if err != nil {
		t.Fatalf("Seal failed: %v", err)
	}

	meshDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(meshDir, vault.FileName), ciphertext, 0o600); err != nil {
		t.Fatalf("write the vault: %v", err)
	}

	return meshDir, shares
}

func setShares(t *testing.T, shares []string) {
	t.Helper()

	for i := 1; i <= vault.ShareCount; i++ {
		value := ""
		if i <= len(shares) {
			value = shares[i-1]
		}
		t.Setenv(fmt.Sprintf("MESHY_SHARE_%d", i), value)
	}
}

func TestUnlockFromEnvNeedsNoPrompt(t *testing.T) {
	meshDir, shares := meshWithVault(t)
	setShares(t, shares[:3])

	if code := run([]string{"unlock", "--from-env", "--mesh", meshDir}); code != 0 {
		t.Fatalf("unlock exited %d, want 0", code)
	}

	path := filepath.Join(meshDir, vault.DirName, "ca-1.key")
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(got) != "the authority key" {
		t.Fatalf("ca-1.key = %q, want the plaintext of the vault", got)
	}
}

func TestUnlockFromEnvNeedsThreeShares(t *testing.T) {
	meshDir, shares := meshWithVault(t)
	setShares(t, shares[:2])

	if code := run([]string{"unlock", "--from-env", "--mesh", meshDir}); code != 1 {
		t.Fatalf("unlock exited %d, want 1", code)
	}
	if _, err := os.Stat(filepath.Join(meshDir, vault.DirName)); !os.IsNotExist(err) {
		t.Fatalf("unlock wrote plaintext with two shares: %v", err)
	}
}

func TestUnlockWithWrongSharesExitsOne(t *testing.T) {
	meshDir, _ := meshWithVault(t)
	_, other := meshWithVault(t)
	setShares(t, other[:3])

	if code := run([]string{"unlock", "--from-env", "--mesh", meshDir}); code != 1 {
		t.Fatalf("unlock exited %d, want 1", code)
	}
}

func TestLockRemovesThePlaintext(t *testing.T) {
	meshDir, shares := meshWithVault(t)
	setShares(t, shares[:3])
	if code := run([]string{"unlock", "--from-env", "--mesh", meshDir}); code != 0 {
		t.Fatalf("unlock exited %d, want 0", code)
	}

	if code := run([]string{"lock", "--mesh", meshDir}); code != 0 {
		t.Fatalf("lock exited %d, want 0", code)
	}
	if _, err := os.Stat(filepath.Join(meshDir, vault.DirName)); !os.IsNotExist(err) {
		t.Fatalf(".meshy still exists after lock: %v", err)
	}

	if code := run([]string{"lock", "--mesh", meshDir}); code != 0 {
		t.Fatalf("a second lock exited %d, want 0", code)
	}
}
