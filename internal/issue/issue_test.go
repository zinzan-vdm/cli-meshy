package issue

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zinzan-vdm/cli-meshy/internal/node"
)

const printed = `[{"details":{"name":"core-1","groups":["core","edge"],"networks":["10.42.0.1/24"],"notAfter":"2027-11-17T00:00:00Z"},"fingerprint":"4f3a"}]`

// stubTool writes a nebula-cert that records its arguments, writes the two files
// a sign asks for, and answers a print with a fixed certificate.
func stubTool(t *testing.T) (string, string) {
	t.Helper()

	dir := t.TempDir()
	script := `#!/bin/sh
if [ "$1" = "print" ]; then
  printf '%s\n' "$STUB_PRINT"
  exit 0
fi
printf '%s\n' "$@" > "$STUB_ARGS"
n=0
if [ -f "$STUB_COUNT" ]; then n=$(cat "$STUB_COUNT"); fi
n=$((n + 1))
printf '%s' "$n" > "$STUB_COUNT"
while [ $# -gt 0 ]; do
  case "$1" in
    -out-key) printf 'STUB KEY %s\n' "$n" > "$2"; shift 2 ;;
    -out-crt) printf 'STUB CRT %s\n' "$n" > "$2"; shift 2 ;;
    *) shift ;;
  esac
done
`
	path := filepath.Join(dir, "nebula-cert")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}

	args := filepath.Join(dir, "args")
	t.Setenv("STUB_ARGS", args)
	t.Setenv("STUB_COUNT", filepath.Join(dir, "count"))
	t.Setenv("STUB_PRINT", printed)
	return path, args
}

func signWith(t *testing.T, binary string) (Certificate, error) {
	t.Helper()

	return Sign(context.Background(), Options{
		Binary:    binary,
		Authority: Authority{Cert: []byte("CA CRT"), Key: []byte("CA KEY")},
		Request: Request{
			Name:     "core-1",
			Networks: "10.42.0.1/24",
			Groups:   []string{"core", "edge"},
			Lifetime: 365 * 24 * time.Hour,
		},
	})
}

func TestDue(t *testing.T) {
	now := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	window := 60 * 24 * time.Hour

	cases := map[string]struct {
		notAfter time.Time
		force    bool
		want     bool
	}{
		"no certificate":       {notAfter: time.Time{}, want: true},
		"expired":              {notAfter: now.Add(-time.Hour), want: true},
		"inside the window":    {notAfter: now.Add(30 * 24 * time.Hour), want: true},
		"at the window edge":   {notAfter: now.Add(window), want: true},
		"outside the window":   {notAfter: now.Add(window + time.Hour), want: false},
		"far from the window":  {notAfter: now.Add(300 * 24 * time.Hour), want: false},
		"force on a valid one": {notAfter: now.Add(300 * 24 * time.Hour), force: true, want: true},
	}

	for name, c := range cases {
		if got := Due(c.notAfter, window, now, c.force); got != c.want {
			t.Errorf("%s: Due = %v, want %v", name, got, c.want)
		}
	}
}

func TestSignAsksForTheNodeFacts(t *testing.T) {
	binary, argsPath := stubTool(t)

	if _, err := signWith(t, binary); err != nil {
		t.Fatalf("Sign failed: %v", err)
	}

	body, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatalf("read args: %v", err)
	}
	args := string(body)

	for _, want := range []string{
		"sign\n",
		"-name\ncore-1\n",
		"-networks\n10.42.0.1/24\n",
		"-groups\ncore,edge\n",
		"-duration\n8760h\n",
		"-out-crt\n",
		"-out-key\n",
		"-ca-crt\n",
		"-ca-key\n",
	} {
		if !strings.Contains(args, want) {
			t.Errorf("arguments do not hold %q:\n%s", want, args)
		}
	}
}

func TestSignReturnsTheCertificateAndTheKey(t *testing.T) {
	binary, _ := stubTool(t)

	signed, err := signWith(t, binary)
	if err != nil {
		t.Fatalf("Sign failed: %v", err)
	}

	if string(signed.Cert) != "STUB CRT 1\n" {
		t.Errorf("cert = %q", signed.Cert)
	}
	if string(signed.Key) != "STUB KEY 1\n" {
		t.Errorf("key = %q", signed.Key)
	}
	if signed.Fingerprint != "4f3a" {
		t.Errorf("fingerprint = %q", signed.Fingerprint)
	}
	if want := time.Date(2027, 11, 17, 0, 0, 0, 0, time.UTC); !signed.NotAfter.Equal(want) {
		t.Errorf("notAfter = %s, want %s", signed.NotAfter, want)
	}
}

func TestSignLeavesTheGroupsOutWhenThereAreNone(t *testing.T) {
	binary, argsPath := stubTool(t)

	_, err := Sign(context.Background(), Options{
		Binary:    binary,
		Authority: Authority{Cert: []byte("CA CRT"), Key: []byte("CA KEY")},
		Request:   Request{Name: "core-1", Networks: "10.42.0.1/24", Lifetime: time.Hour},
	})
	if err != nil {
		t.Fatalf("Sign failed: %v", err)
	}

	body, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatalf("read args: %v", err)
	}
	if strings.Contains(string(body), "-groups") {
		t.Errorf("a node with no groups was given the flag:\n%s", body)
	}
}

func TestSignRefusesARequestWithNoNetwork(t *testing.T) {
	binary, _ := stubTool(t)

	_, err := Sign(context.Background(), Options{
		Binary:    binary,
		Authority: Authority{Cert: []byte("CA CRT"), Key: []byte("CA KEY")},
		Request:   Request{Name: "core-1", Lifetime: time.Hour},
	})
	if err == nil {
		t.Fatal("Sign accepted a request with no network")
	}
	if !strings.Contains(err.Error(), "core-1") {
		t.Errorf("error = %q, want it to name the node", err)
	}
}

func TestSignReportsTheMessageOfTheTool(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nebula-cert")
	if err := os.WriteFile(path, []byte("#!/bin/sh\necho 'no authority key' >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}

	_, err := signWith(t, path)
	if err == nil {
		t.Fatal("Sign accepted a failed tool")
	}
	if !strings.Contains(err.Error(), "no authority key") {
		t.Errorf("error = %q, want the message of the tool", err)
	}
}

func TestPrintReadsACertificateFromBytes(t *testing.T) {
	binary, _ := stubTool(t)

	cert, err := Print(context.Background(), binary, []byte("STUB CRT 1\n"))
	if err != nil {
		t.Fatalf("Print failed: %v", err)
	}
	if cert.Fingerprint != "4f3a" || cert.Name != "core-1" {
		t.Errorf("cert = %+v", cert)
	}
}

func TestExpiryRejectsADateTheToolCannotWrite(t *testing.T) {
	if _, err := Expiry(node.Cert{NotAfter: "17 November 2027"}); err == nil {
		t.Fatal("Expiry accepted a date that is not RFC 3339")
	}
}
