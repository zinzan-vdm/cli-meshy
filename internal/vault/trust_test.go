package vault

import (
	"strings"
	"testing"
)

func TestTrustBundleJoinsTheAuthoritiesInOrder(t *testing.T) {
	record := Record{
		"ca-1.crt": []byte("-----BEGIN CA 1-----\n"),
		"ca-2.crt": []byte("-----BEGIN CA 2-----\n"),
	}

	bundle, err := TrustBundle(record, []string{"ca-1", "ca-2"})
	if err != nil {
		t.Fatalf("TrustBundle: %v", err)
	}
	want := "-----BEGIN CA 1-----\n-----BEGIN CA 2-----\n"
	if string(bundle) != want {
		t.Errorf("bundle = %q, want %q", bundle, want)
	}
}

func TestTrustBundleAddsTheNewlineBetweenTwoCertificates(t *testing.T) {
	record := Record{
		"ca-1.crt": []byte("first"),
		"ca-2.crt": []byte("second"),
	}

	bundle, err := TrustBundle(record, []string{"ca-1", "ca-2"})
	if err != nil {
		t.Fatalf("TrustBundle: %v", err)
	}
	if string(bundle) != "first\nsecond\n" {
		t.Errorf("bundle = %q", bundle)
	}
}

func TestTrustBundleNamesTheAuthorityThatIsMissing(t *testing.T) {
	record := Record{"ca-1.crt": []byte("first\n")}

	_, err := TrustBundle(record, []string{"ca-1", "ca-2"})
	if err == nil {
		t.Fatal("TrustBundle accepted a trust list that the vault cannot supply")
	}
	if !strings.Contains(err.Error(), "ca-2") {
		t.Errorf("error does not name the authority: %v", err)
	}
}

func TestTrustBundleRefusesAnEmptyList(t *testing.T) {
	if _, err := TrustBundle(Record{}, nil); err == nil {
		t.Error("TrustBundle accepted an empty trust list")
	}
}
