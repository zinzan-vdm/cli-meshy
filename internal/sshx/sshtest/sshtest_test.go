package sshtest

import (
	"context"
	"testing"
)

func readFile(t *testing.T, c *Client, path string) string {
	t.Helper()
	body, err := c.Read(context.Background(), path)
	if err != nil {
		t.Fatalf("Read %s: %v", path, err)
	}
	return string(body)
}

func TestConnectServesAFileFromMemory(t *testing.T) {
	s := New(t)
	s.Put("/etc/nebula/meshy-homelab/config.yml", []byte("pki:\n"), 0o600)
	c := s.Connect()

	if got := readFile(t, c, "/etc/nebula/meshy-homelab/config.yml"); got != "pki:\n" {
		t.Errorf("read = %q, want %q", got, "pki:\n")
	}
	if _, err := c.Read(context.Background(), "/etc/nebula/meshy-homelab/ca.crt"); err == nil {
		t.Error("read of an absent file reported no error")
	}
}

func TestConnectRefusesAKeyTheNodeDoesNotList(t *testing.T) {
	s := New(t)
	other, _, err := keyPair()
	if err != nil {
		t.Fatalf("key pair: %v", err)
	}
	if _, err := s.dial(other); err == nil {
		t.Fatal("the server accepted a key that it does not list")
	}
}

func TestWriteKeepsTheBytesAndTheMode(t *testing.T) {
	s := New(t)
	c := s.Connect()

	if err := c.Write(context.Background(), "/tmp/config.yml.tmp", []byte("pki:\n"), 0o600); err != nil {
		t.Fatalf("Write: %v", err)
	}
	f, ok := s.File("/tmp/config.yml.tmp")
	if !ok {
		t.Fatal("the write left no file")
	}
	if string(f.Body) != "pki:\n" || f.Mode != 0o600 {
		t.Errorf("file = %q mode %o", f.Body, f.Mode)
	}
	ops := s.Ops()
	if len(ops) != 1 || ops[0].Kind != OpWrite {
		t.Errorf("ops = %+v, want one write", ops)
	}
}

func TestRenameComesAfterTheWrite(t *testing.T) {
	s := New(t)
	c := s.Connect()

	if err := c.Write(context.Background(), "/tmp/a.tmp", []byte("x"), 0o644); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if _, err := c.Run(context.Background(), "mv -f '/tmp/a.tmp' '/tmp/a'"); err != nil {
		t.Fatalf("Run: %v", err)
	}

	ops := s.Ops()
	if len(ops) != 2 {
		t.Fatalf("ops = %+v, want a write and a rename", ops)
	}
	if ops[0].Kind != OpWrite || ops[1].Kind != OpRename {
		t.Errorf("ops = %+v, want the write before the rename", ops)
	}
	if ops[1].To != "/tmp/a" {
		t.Errorf("rename target = %q", ops[1].To)
	}
	if f, ok := s.File("/tmp/a"); !ok || string(f.Body) != "x" {
		t.Errorf("the rename lost the bytes: %+v", f)
	}
}

func TestFailCommandFailsOneCommandOnce(t *testing.T) {
	s := New(t)
	c := s.Connect()
	s.FailCommand("systemctl cat 'nebula@meshy-homelab.service'", 1, "No files found")

	res, err := c.Run(context.Background(), "systemctl cat 'nebula@meshy-homelab.service'")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.ExitCode != 1 || res.Stderr != "No files found" {
		t.Errorf("first run = %+v, want the staged failure", res)
	}

	res, err = c.Run(context.Background(), "systemctl cat 'nebula@meshy-homelab.service'")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.ExitCode != 0 {
		t.Errorf("second run = %+v, want success", res)
	}
	if len(s.Commands()) != 2 {
		t.Errorf("the log holds %d commands, want 2", len(s.Commands()))
	}
}

func TestFailWriteLeavesTheOldFile(t *testing.T) {
	const path = "/etc/nebula/meshy-homelab/config.yml.tmp"
	s := New(t)
	c := s.Connect()
	s.FailWrite(path, "no space left on device")

	err := c.Write(context.Background(), path, []byte("new"), 0o600)
	if err == nil {
		t.Fatal("Write succeeded although the node refused it")
	}
	if f, ok := s.File(path); ok {
		t.Errorf("the failed write left %q on the node", f.Body)
	}
}

func TestKeepRenameReportsSuccessAndLeavesTheOldBytes(t *testing.T) {
	const (
		temp = "/etc/nebula/meshy-homelab/config.yml.tmp"
		path = "/etc/nebula/meshy-homelab/config.yml"
	)
	s := New(t)
	c := s.Connect()
	s.Put(path, []byte("old"), 0o600)
	s.KeepRename(path)

	if err := c.Write(context.Background(), temp, []byte("new"), 0o600); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if _, err := c.Run(context.Background(), "mv -f '"+temp+"' '"+path+"'"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	f, ok := s.File(path)
	if !ok {
		t.Fatal("the file left the node")
	}
	if string(f.Body) != "old" {
		t.Errorf("body = %q, want the old bytes", f.Body)
	}
}
