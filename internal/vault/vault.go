package vault

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"filippo.io/age"
)

// The vault is one sealed file in the mesh. Unlock writes its plaintext into DirName.
const (
	DirName      = ".meshy"
	FileName     = "secrets.age"
	IdentityFile = "age.identity"
	ShareCount   = 5
	Threshold    = 3
)

var (
	// ErrWrongShares reports shares that do not open the vault.
	ErrWrongShares = errors.New("the shares do not open this vault")
	// ErrLocked reports a vault that is not open. A command that reads or writes
	// the record needs the plaintext that Unlock writes.
	ErrLocked = errors.New("the vault is locked, run meshy unlock")
)

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

	sealed, err := encrypt(plaintext, identity.Recipient())
	if err != nil {
		return nil, nil, err
	}

	return sealed, shares, nil
}

// Open decrypts a sealed vault with at least Threshold shares.
func Open(ciphertext []byte, shares []string) ([]byte, error) {
	identity, err := identityFrom(shares)
	if err != nil {
		return nil, err
	}
	return decrypt(ciphertext, identity)
}

// Unlock writes the plaintext of the vault into <mesh>/.meshy and returns that path.
func Unlock(meshDir string, shares []string, force bool) (string, error) {
	identity, err := identityFrom(shares)
	if err != nil {
		return "", err
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

	plaintext, err := decrypt(ciphertext, identity)
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
	if err := writeIdentity(dir, identity); err != nil {
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

// Read returns the record of an open vault. One file becomes one entry, and the
// key that seals the vault is not part of the record.
func Read(dir string) (Record, error) {
	if _, err := readIdentity(dir); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, ErrLocked
	}

	record := Record{}
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == IdentityFile {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		body, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		record[entry.Name()] = body
	}

	return record, nil
}

// Write replaces the record of the open vault and seals it again with the key
// that opened it, so the shares that already exist keep working.
func Write(meshDir, dir string, record Record) error {
	identity, err := readIdentity(dir)
	if err != nil {
		return err
	}

	plaintext, err := Encode(record)
	if err != nil {
		return err
	}
	sealed, err := encrypt(plaintext, identity.Recipient())
	if err != nil {
		return fmt.Errorf("seal the vault: %w", err)
	}

	// The open vault is written first. A failure here leaves the sealed file
	// that a person already has untouched.
	if err := writeRecord(dir, record); err != nil {
		return err
	}
	if err := writeIdentity(dir, identity); err != nil {
		return err
	}

	return replace(filepath.Join(meshDir, FileName), sealed)
}

func encrypt(plaintext []byte, recipient age.Recipient) ([]byte, error) {
	var sealed bytes.Buffer
	writer, err := age.Encrypt(&sealed, recipient)
	if err != nil {
		return nil, err
	}
	if _, err := writer.Write(plaintext); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return sealed.Bytes(), nil
}

func decrypt(ciphertext []byte, identity *age.X25519Identity) ([]byte, error) {
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

// identityFrom rebuilds the key of the vault from the shares that open it.
func identityFrom(shares []string) (*age.X25519Identity, error) {
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
	return identity, nil
}

func readIdentity(dir string) (*age.X25519Identity, error) {
	path := filepath.Join(dir, IdentityFile)
	body, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrLocked
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	identity, err := age.ParseX25519Identity(strings.TrimSpace(string(body)))
	if err != nil {
		return nil, fmt.Errorf("%s holds no age identity", path)
	}
	return identity, nil
}

func writeIdentity(dir string, identity *age.X25519Identity) error {
	path := filepath.Join(dir, IdentityFile)
	if err := os.WriteFile(path, []byte(identity.String()+"\n"), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
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

// replace writes body to path through a temporary name, so a failed write leaves
// the file that is already there in place.
func replace(path string, body []byte) error {
	temp := path + ".new"
	if err := os.WriteFile(temp, body, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", temp, err)
	}
	if err := os.Rename(temp, path); err != nil {
		os.Remove(temp)
		return fmt.Errorf("replace %s: %w", path, err)
	}
	return nil
}
