package shamir

import (
	"bytes"
	"testing"
)

func TestThreeOfFiveRebuildsTheSecret(t *testing.T) {
	secret := []byte("an age identity that is longer than one byte")

	parts, err := Split(secret, 5, 3)
	if err != nil {
		t.Fatalf("split: %v", err)
	}
	if len(parts) != 5 {
		t.Fatalf("parts = %d, want 5", len(parts))
	}

	for _, set := range [][]int{
		{0, 1, 2},
		{0, 2, 4},
		{1, 3, 4},
	} {
		chosen := [][]byte{parts[set[0]], parts[set[1]], parts[set[2]]}
		got, err := Combine(chosen)
		if err != nil {
			t.Fatalf("combine %v: %v", set, err)
		}
		if !bytes.Equal(got, secret) {
			t.Fatalf("combine %v rebuilt %q, want %q", set, got, secret)
		}
	}
}

func TestTwoOfFiveDoesNotRebuildTheSecret(t *testing.T) {
	secret := []byte("an age identity that is longer than one byte")

	parts, err := Split(secret, 5, 3)
	if err != nil {
		t.Fatalf("split: %v", err)
	}

	got, err := Combine([][]byte{parts[0], parts[1]})
	if err != nil {
		t.Fatalf("combine: %v", err)
	}
	if bytes.Equal(got, secret) {
		t.Fatal("two shares rebuilt the secret, so the threshold is not enforced")
	}
}

func TestSplitIsRandom(t *testing.T) {
	secret := []byte("the same secret twice")

	first, err := Split(secret, 5, 3)
	if err != nil {
		t.Fatalf("split: %v", err)
	}
	second, err := Split(secret, 5, 3)
	if err != nil {
		t.Fatalf("split: %v", err)
	}

	if bytes.Equal(first[0], second[0]) {
		t.Fatal("two splits produced the same first share, so the polynomial is not random")
	}
}

func TestSplitRejectsBadInput(t *testing.T) {
	cases := map[string]struct {
		secret    []byte
		parts     int
		threshold int
	}{
		"empty secret":     {[]byte{}, 5, 3},
		"below threshold":  {[]byte("x"), 2, 3},
		"one part":         {[]byte("x"), 1, 2},
		"threshold of one": {[]byte("x"), 3, 1},
		"too many parts":   {[]byte("x"), 256, 3},
	}

	for name, c := range cases {
		if _, err := Split(c.secret, c.parts, c.threshold); err == nil {
			t.Errorf("%s: split returned no error", name)
		}
	}
}

func TestCombineRejectsBadInput(t *testing.T) {
	parts, err := Split([]byte("a secret"), 5, 3)
	if err != nil {
		t.Fatalf("split: %v", err)
	}

	if _, err := Combine([][]byte{parts[0]}); err == nil {
		t.Error("one part: combine returned no error")
	}

	unequal := [][]byte{parts[0], parts[1][:len(parts[1])-1], parts[2]}
	if _, err := Combine(unequal); err == nil {
		t.Error("unequal lengths: combine returned no error")
	}

	short := [][]byte{{0x01}, {0x02}}
	if _, err := Combine(short); err == nil {
		t.Error("parts of one byte: combine returned no error")
	}
}
