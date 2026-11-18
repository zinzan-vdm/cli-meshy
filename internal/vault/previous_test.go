package vault

import "testing"

func TestKeepPreviousKeepsThePairAndTheCertificateListIgnoresIt(t *testing.T) {
	record := Record{}
	SetCertificate(record, "core-1", []byte("new certificate\n"), []byte("new key\n"))
	KeepPrevious(record, "core-1", "ca-1", []byte("old certificate\n"), []byte("old key\n"))

	cert, key, ok := Previous(record, "core-1", "ca-1")
	if !ok {
		t.Fatal("the record dropped the pair that the rotation replaced")
	}
	if string(cert) != "old certificate\n" || string(key) != "old key\n" {
		t.Errorf("the record kept %q and %q", cert, key)
	}

	current, _, ok := Certificate(record, "core-1")
	if !ok || string(current) != "new certificate\n" {
		t.Errorf("the current certificate is %q", current)
	}

	for _, name := range Certificates(record) {
		if name != "core-1" {
			t.Errorf("Certificates returned %q, a kept pair is not a certificate of a node", name)
		}
	}
}

func TestPreviousReportsAPairThatTheRecordDoesNotHold(t *testing.T) {
	record := Record{}
	if _, _, ok := Previous(record, "core-1", "ca-1"); ok {
		t.Error("Previous reported a pair that the record does not hold")
	}
}

func TestPreviousAuthoritiesListsTheKeptPairsOfOneNode(t *testing.T) {
	record := Record{}
	KeepPrevious(record, "core-1", "ca-2", []byte("second"), []byte("second key"))
	KeepPrevious(record, "core-1", "ca-1", []byte("first"), []byte("first key"))
	KeepPrevious(record, "web-1", "ca-1", []byte("other"), []byte("other key"))

	names := PreviousAuthorities(record, "core-1")
	if len(names) != 2 || names[0] != "ca-1" || names[1] != "ca-2" {
		t.Errorf("PreviousAuthorities = %v, want [ca-1 ca-2]", names)
	}
}
