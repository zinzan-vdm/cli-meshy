package revoke

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/zinzan-vdm/cli-meshy/internal/config"
	"github.com/zinzan-vdm/cli-meshy/internal/vault"
)

func testMesh() config.Mesh {
	m := config.Mesh{
		Name: "homelab",
		CA:   config.CA{Active: "ca-1", Trust: []string{"ca-1"}},
		Nodes: []config.Node{
			{Name: "core-1", Kind: config.KindServer, IP: "10.42.0.1"},
			{Name: "web-1", Kind: config.KindServer, IP: "10.42.0.2"},
		},
	}
	return m
}

func testRecord() vault.Record {
	record := vault.Record{}
	vault.SetCertificate(record, "ca-1", []byte("ca certificate\n"), []byte("ca key\n"))
	vault.SetCertificate(record, "core-1", []byte("core certificate\n"), []byte("core key\n"))
	vault.SetCertificate(record, "web-1", []byte("web certificate\n"), []byte("web key\n"))
	vault.SetCertificate(record, "old-1", []byte("old certificate\n"), []byte("old key\n"))
	return record
}

func TestNamesFindsTheCertificatesThatTheFileDoesNotName(t *testing.T) {
	names := Names(testRecord(), testMesh())

	if len(names) != 1 || names[0] != "old-1" {
		t.Errorf("names = %v, want old-1 alone", names)
	}
}

func TestNamesLeavesATrustedAuthorityAlone(t *testing.T) {
	record := testRecord()
	vault.SetCertificate(record, "ca-2", []byte("next authority\n"), []byte("next key\n"))
	m := testMesh()
	m.CA.Trust = []string{"ca-1", "ca-2"}

	for _, name := range Names(record, m) {
		if name == "ca-2" {
			t.Error("a trusted authority is not a revocation")
		}
	}
}

func TestNamesIsEmptyForAMeshThatNamesEveryCertificate(t *testing.T) {
	record := vault.Record{}
	vault.SetCertificate(record, "ca-1", []byte("ca\n"), []byte("ca key\n"))
	vault.SetCertificate(record, "core-1", []byte("core\n"), []byte("core key\n"))
	vault.SetCertificate(record, "web-1", []byte("web\n"), []byte("web key\n"))

	if names := Names(record, testMesh()); len(names) != 0 {
		t.Errorf("names = %v, want none", names)
	}
}

func TestTextWritesOneFingerprintPerLine(t *testing.T) {
	body := Text(vault.Record{}, []Node{
		{Name: "old-2", Fingerprint: "2222"},
		{Name: "old-1", Fingerprint: "1111"},
	})
	if string(body) != "1111\n2222\n" {
		t.Errorf("blocklist = %q", body)
	}
}

func TestTextIsEmptyWhenNothingIsRevoked(t *testing.T) {
	if body := Text(vault.Record{}, nil); len(body) != 0 {
		t.Errorf("blocklist = %q, want nothing", body)
	}
}

func TestTextKeepsABlockThatTheRecordAlreadyHolds(t *testing.T) {
	record := vault.Record{}
	vault.SetRevoked(record, "gone-1", "abcd")

	body := Text(record, nil)
	if string(body) != "abcd\n" {
		t.Errorf("blocklist = %q, want the fingerprint from the record", body)
	}
}

func TestTextCountsAFingerprintOnce(t *testing.T) {
	record := vault.Record{}
	vault.SetRevoked(record, "gone-1", "abcd")

	body := Text(record, []Node{{Name: "old-1", Fingerprint: "abcd"}})
	if string(body) != "abcd\n" {
		t.Errorf("blocklist = %q, want one line", body)
	}
}

// stubTool writes a nebula-cert that answers a print with one document. A tool
// that fails exits 1, which is what a missing or broken tool does.
func stubTool(t *testing.T, doc string) string {
	t.Helper()

	dir := t.TempDir()
	script := "#!/bin/sh\nprintf '%s\\n' '" + doc + "'\n"
	path := filepath.Join(dir, "nebula-cert")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write the stub tool: %v", err)
	}
	return path
}

const nodeDoc = `[{"details":{"name":"old-1","groups":[],"networks":[],"notAfter":"2030-01-01T00:00:00Z","isCa":false},"fingerprint":"4f3a"}]`
const authorityDoc = `[{"details":{"name":"ca-2","groups":[],"networks":[],"notAfter":"2030-01-01T00:00:00Z","isCa":true},"fingerprint":"9c9c"}]`

func TestListReadsTheFingerprintFromTheTool(t *testing.T) {
	nodes, err := List(context.Background(), stubTool(t, nodeDoc), testRecord(), testMesh())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(nodes) != 1 || nodes[0].Name != "old-1" {
		t.Fatalf("nodes = %+v, want old-1", nodes)
	}
	if nodes[0].Fingerprint != "4f3a" {
		t.Errorf("fingerprint = %q, want 4f3a", nodes[0].Fingerprint)
	}
}

func TestListSkipsACertificateThatSignsOthers(t *testing.T) {
	record := testRecord()
	vault.SetCertificate(record, "ca-2", []byte("next authority\n"), []byte("next key\n"))

	nodes, err := List(context.Background(), stubTool(t, authorityDoc), record, testMesh())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, n := range nodes {
		if n.Name == "ca-2" {
			t.Error("the run would block an authority")
		}
	}
}

func TestListStopsOnACertificateThatItCannotRead(t *testing.T) {
	failing := "#!/bin/sh\nexit 1\n"
	dir := t.TempDir()
	path := filepath.Join(dir, "nebula-cert")
	if err := os.WriteFile(path, []byte(failing), 0o755); err != nil {
		t.Fatalf("write the failing tool: %v", err)
	}

	if _, err := List(context.Background(), path, testRecord(), testMesh()); err == nil {
		t.Fatal("List accepted a certificate that it could not read")
	}
}
