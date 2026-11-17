package vault

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/vault/shamir"
)

// A share carries a label so that a person can tell five password manager entries apart.
const sharePrefix = "meshy"

func split(secret []byte) ([]string, error) {
	parts, err := shamir.Split(secret, ShareCount, Threshold)
	if err != nil {
		return nil, err
	}

	shares := make([]string, 0, len(parts))
	for i, part := range parts {
		shares = append(shares, formatShare(i+1, part))
	}

	return shares, nil
}

func combine(shares []string) ([]byte, error) {
	parts := make([][]byte, 0, len(shares))
	for i, share := range shares {
		part, err := parseShare(share)
		if err != nil {
			return nil, fmt.Errorf("share %d: %w", i+1, err)
		}
		parts = append(parts, part)
	}

	secret, err := shamir.Combine(parts)
	if err != nil {
		return nil, ErrWrongShares
	}

	return secret, nil
}

func formatShare(label int, part []byte) string {
	return fmt.Sprintf("%s-%d-%s", sharePrefix, label, base64.RawURLEncoding.EncodeToString(part))
}

func parseShare(share string) ([]byte, error) {
	fields := strings.SplitN(strings.TrimSpace(share), "-", 3)
	if len(fields) != 3 || fields[0] != sharePrefix {
		return nil, fmt.Errorf("%q is not a meshy share", strings.TrimSpace(share))
	}

	label, err := strconv.Atoi(fields[1])
	if err != nil || label < 1 || label > ShareCount {
		return nil, fmt.Errorf("%q has no share label", strings.TrimSpace(share))
	}

	part, err := base64.RawURLEncoding.DecodeString(fields[2])
	if err != nil {
		return nil, fmt.Errorf("share %d is not base64", label)
	}
	if len(part) < 2 {
		return nil, fmt.Errorf("share %d is too short", label)
	}

	return part, nil
}
