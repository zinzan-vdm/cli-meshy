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

// nebula-cert v1.12.0 prints an array of one, not an object. A plan against a
// real node reads this shape.
const certArrayJSON = `[{"curve":"CURVE25519","details":{"groups":["core"],"isCa":false,"issuer":"5af9","name":"core-1","networks":null,"notAfter":"2027-11-17T22:47:57Z","notBefore":"2026-11-17T22:47:57Z","unsafeNetworks":null},"fingerprint":"aa11bb22","publicKey":"f7c7","signature":"448a","version":2}]`

func TestParseCertReadsTheArrayTheToolPrints(t *testing.T) {
	c, err := ParseCert(certArrayJSON)
	if err != nil {
		t.Fatalf("ParseCert: %v", err)
	}
	if c.Name != "core-1" || c.Fingerprint != "aa11bb22" || c.NotAfter != "2027-11-17T22:47:57Z" {
		t.Errorf("unexpected cert: %+v", c)
	}
	if len(c.Groups) != 1 || c.Groups[0] != "core" {
		t.Errorf("groups = %v", c.Groups)
	}
}

func TestParseCertRejectsAnEmptyArray(t *testing.T) {
	if _, err := ParseCert("[]"); err == nil {
		t.Error("ParseCert accepted an empty array")
	}
}

// nebula-cert v1.12.0 prints one document for each PEM block in the file, so a
// trust bundle with two authorities prints an array of two. This text is the
// output of the pinned tool on a bundle of ca-1 and ca-2.
const bundleJSON = `[{"curve":"CURVE25519","details":{"groups":null,"isCa":true,"issuer":"","name":"ca-1","networks":["10.42.0.0/24"],"notAfter":"2046-11-13T14:37:52Z","notBefore":"2026-11-18T14:37:52Z"},"fingerprint":"cded78a3"},{"curve":"CURVE25519","details":{"groups":null,"isCa":true,"issuer":"","name":"ca-2","networks":["10.42.0.0/24"],"notAfter":"2046-11-13T14:37:52Z","notBefore":"2026-11-18T14:37:52Z"},"fingerprint":"cf7ac451"}]`

func TestParseCertsReadsEveryDocumentOfABundle(t *testing.T) {
	certs, err := ParseCerts(bundleJSON)
	if err != nil {
		t.Fatalf("ParseCerts: %v", err)
	}
	if len(certs) != 2 {
		t.Fatalf("ParseCerts returned %d certificates, want 2", len(certs))
	}
	if certs[0].Name != "ca-1" || certs[1].Name != "ca-2" {
		t.Errorf("names = %q %q", certs[0].Name, certs[1].Name)
	}
	if !certs[0].IsCA || !certs[1].IsCA {
		t.Errorf("a bundle entry is not marked as an authority: %+v", certs)
	}
}

func TestParseCertsRejectsAnEmptyArray(t *testing.T) {
	if _, err := ParseCerts("[]"); err == nil {
		t.Error("ParseCerts accepted an empty array")
	}
}
