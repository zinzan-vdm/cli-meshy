package vault

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"filippo.io/age"
)

// The vault is one sealed file in the mesh. Unlock writes its plaintext into DirName.
const (
	DirName    = ".meshy"
	FileName   = "secrets.age"
	ShareCount = 5
	Threshold  = 3
)

// ErrWrongShares reports shares that do not open the vault.
var ErrWrongShares = errors.New("the shares do not open this vault")

// Seal encrypts plaintext with a new key and returns the ciphertext and the shares of that key.
func Seal(plaintext []byte) ([]byte, []string, error) {
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		return nil, nil, err
	}

	shares, err := split([]byte(identity.String()))
	if err != nil {
		return nil, nil, err
	}

	var sealed bytes.Buffer
	writer, err := age.Encrypt(&sealed, identity.Recipient())
	if err != nil {
		return nil, nil, err
	}
	if _, err := writer.Write(plaintext); err != nil {
		return nil, nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, nil, err
	}

	return sealed.Bytes(), shares, nil
}

// Open decrypts a sealed vault with at least Threshold shares.
func Open(ciphertext []byte, shares []string) ([]byte, error) {
	if len(shares) < Threshold {
		return nil, fmt.Errorf("need %d shares, got %d", Threshold, len(shares))
	}

	secret, err := combine(shares[:Threshold])
	if err != nil {
		return nil, err
	}

	identity, err := age.ParseX25519Identity(string(secret))
	if err != nil {
		return nil, ErrWrongShares
	}

	reader, err := age.Decrypt(bytes.NewReader(ciphertext), identity)
	if err != nil {
		return nil, ErrWrongShares
	}
	plaintext, err := io.ReadAll(reader)
	if err != nil {
		return nil, ErrWrongShares
	}

	return plaintext, nil
}

// Unlock writes the plaintext of the vault into <mesh>/.meshy and returns that path.
func Unlock(meshDir string, shares []string, force bool) (string, error) {
	if len(shares) < Threshold {
		return "", fmt.Errorf("need %d shares, got %d", Threshold, len(shares))
	}

	dir := filepath.Join(meshDir, DirName)
	if !force {
		if entries, err := os.ReadDir(dir); err == nil && len(entries) > 0 {
			return "", fmt.Errorf("%s already holds plaintext, use --force to replace it", dir)
		}
	}

	path := filepath.Join(meshDir, FileName)
	ciphertext, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}

	plaintext, err := Open(ciphertext, shares)
	if err != nil {
		return "", err
	}

	record, err := Decode(plaintext)
	if err != nil {
		return "", err
	}

	if err := writeRecord(dir, record); err != nil {
		return "", err
	}

	return dir, nil
}

// Lock removes <mesh>/.meshy. A vault that is already locked stays locked.
func Lock(meshDir string) error {
	dir := filepath.Join(meshDir, DirName)
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("remove %s: %w", dir, err)
	}
	return nil
}

func writeRecord(dir string, record Record) error {
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("clear %s: %w", dir, err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}

	names := make([]string, 0, len(record))
	for name := range record {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, record[name], 0o600); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}

	return nil
}
