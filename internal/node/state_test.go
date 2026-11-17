package node

import "testing"

func TestParseVersion(t *testing.T) {
	cases := map[string]string{
		"Version: 1.12.0\n":      "1.12.0",
		"Nebula v1.9.5\n":        "1.9.5",
		"1.12.0\n":               "1.12.0",
		"Version: 1.12.0-beta\n": "1.12.0",
	}
	for in, want := range cases {
		got, err := ParseVersion(in)
		if err != nil {
			t.Fatalf("ParseVersion(%q): %v", in, err)
		}
		if got != want {
			t.Errorf("ParseVersion(%q) = %q, want %q", in, got, want)
		}
	}
	if _, err := ParseVersion("no release here"); err == nil {
		t.Error("ParseVersion accepted output with no version")
	}
}

func TestParseService(t *testing.T) {
	cases := map[string]string{
		"active\n":   "active",
		"inactive\n": "inactive",
		"failed\n":   "failed",
		"":           "",
	}
	for in, want := range cases {
		if got := ParseService(in); got != want {
			t.Errorf("ParseService(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseHash(t *testing.T) {
	digest := "3f2a1b" + "0000000000000000000000000000000000000000000000000000000000000000"[:58]
	got, err := ParseHash(digest + "  /etc/nebula/meshy-homelab/config.yml\n")
	if err != nil {
		t.Fatalf("ParseHash: %v", err)
	}
	if got != digest {
		t.Errorf("ParseHash = %q, want %q", got, digest)
	}
	if _, err := ParseHash("not a digest"); err == nil {
		t.Error("ParseHash accepted a line with no digest")
	}
}

const certJSON = `{
  "details": {
    "name": "core-1",
    "groups": ["core", "admin"],
    "ips": ["10.42.0.1/24"],
    "notAfter": "2027-11-17T20:00:00Z",
    "isCA": false
  },
  "fingerprint": "aa11bb22"
}`

func TestParseCert(t *testing.T) {
	c, err := ParseCert(certJSON)
	if err != nil {
		t.Fatalf("ParseCert: %v", err)
	}
	if !c.Present || c.Name != "core-1" || c.Fingerprint != "aa11bb22" {
		t.Fatalf("unexpected cert: %+v", c)
	}
	if len(c.Groups) != 2 || c.Groups[0] != "core" {
		t.Errorf("groups = %v", c.Groups)
	}
	if len(c.Addresses) != 1 || c.Addresses[0] != "10.42.0.1/24" {
		t.Errorf("addresses = %v", c.Addresses)
	}
	if c.NotAfter != "2027-11-17T20:00:00Z" || c.IsCA {
		t.Errorf("unexpected cert: %+v", c)
	}
}

func TestParseCertRejects(t *testing.T) {
	if _, err := ParseCert("not json"); err == nil {
		t.Error("ParseCert accepted text that is not json")
	}
	if _, err := ParseCert(`{"details":{"name":"core-1"}}`); err == nil {
		t.Error("ParseCert accepted a certificate with no fingerprint")
	}
}
