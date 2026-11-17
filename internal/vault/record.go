package vault

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

// Record is the plaintext that the vault holds. Each entry becomes one file under .meshy.
type Record map[string][]byte

// Encode turns a record into the bytes that Seal encrypts.
func Encode(record Record) ([]byte, error) {
	for name := range record {
		if err := checkName(name); err != nil {
			return nil, err
		}
	}

	data, err := json.Marshal(record)
	if err != nil {
		return nil, fmt.Errorf("encode the vault record: %w", err)
	}

	return data, nil
}

// Decode reads the plaintext of a vault. It rejects a name that would write outside .meshy.
func Decode(data []byte) (Record, error) {
	record := Record{}
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("decode the vault record: %w", err)
	}
	if len(record) == 0 {
		return nil, fmt.Errorf("the vault record is empty")
	}

	for name := range record {
		if err := checkName(name); err != nil {
			return nil, err
		}
	}

	return record, nil
}

func checkName(name string) error {
	switch {
	case name == "" || name == "." || name == "..":
		return fmt.Errorf("the vault holds the invalid name %q", name)
	case name == IdentityFile:
		return fmt.Errorf("the vault holds the reserved name %q", name)
	case strings.ContainsAny(name, `/\`):
		return fmt.Errorf("the vault holds the invalid name %q", name)
	case filepath.Base(name) != name:
		return fmt.Errorf("the vault holds the invalid name %q", name)
	}
	return nil
}
