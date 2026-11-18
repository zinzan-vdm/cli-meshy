// Package sshtest is an SSH server in the test process with a filesystem in
// memory. It serves golang.org/x/crypto/ssh over a pipe, so a test that needs a
// node gets one with no port, no privilege, and no host.
package sshtest

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/zinzan-vdm/cli-meshy/internal/node"
)

// Timeout bounds one command against the server, so a hang fails the test
// instead of the CI job.
const Timeout = time.Second

// missingExit is the status that internal/sshx reports for a path that is not
// there. Read turns that status into node.ErrNoFile.
const missingExit = 3

// The kinds of filesystem operation that Ops records.
const (
	OpRead   = "read"
	OpWrite  = "write"
	OpRename = "rename"
)

// File is one path on the node.
type File struct {
	Body []byte
	Mode fs.FileMode
}

// Op is one filesystem operation. Seq orders it against the commands, which is
// how a test proves that the rename came last.
type Op struct {
	Seq  int
	Kind string
	Path string
	To   string
}

// Command is one command that the server ran, with the status it returned.
type Command struct {
	Seq   int
	Text  string
	Stdin []byte
	Exit  int
}

// Server is one node: a filesystem, a unit, and a log of what happened to them.
type Server struct {
	t *testing.T

	mu       sync.Mutex
	seq      int
	files    map[string]File
	ops      []Op
	cmds     []Command
	faults   []fault
	login    ssh.Signer
	active   bool
	stayDown bool
	version  string
	conns    []net.Conn
}

// fault is one staged failure. The next operation that matches consumes it.
type fault struct {
	kind   string
	key    string
	code   int
	stderr string
}

const (
	faultCommand = "command"
	faultWrite   = "write"
	faultRead    = "read"
	faultKeep    = "keep"
)

// Option changes the answers of a new server.
type Option func(*Server)

// WithVersion sets what `nebula -version` reports.
func WithVersion(v string) Option {
	return func(s *Server) { s.version = "Version: " + v + "\n" }
}

// New starts a node that stops when the test ends. The node lists one public
// key, and Connect presents the private half of it.
func New(t *testing.T, opts ...Option) *Server {
	t.Helper()
	login, _, err := keyPair()
	if err != nil {
		t.Fatalf("sshtest: login key: %v", err)
	}
	s := &Server{t: t, files: map[string]File{}, active: true, login: login}
	for _, opt := range opts {
		opt(s)
	}
	t.Cleanup(s.Close)
	return s
}

// Close drops every connection to the node.
func (s *Server) Close() {
	s.mu.Lock()
	conns := s.conns
	s.conns = nil
	s.mu.Unlock()
	for _, c := range conns {
		c.Close()
	}
}

// Put seeds a file that the node already holds.
func (s *Server) Put(path string, body []byte, mode fs.FileMode) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.files[path] = File{Body: append([]byte(nil), body...), Mode: mode}
}

// File returns what the node holds at path.
func (s *Server) File(path string) (File, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, ok := s.files[path]
	if !ok {
		return File{}, false
	}
	return File{Body: append([]byte(nil), f.Body...), Mode: f.Mode}, true
}

// Paths returns every path that the node holds.
func (s *Server) Paths() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.files))
	for p := range s.files {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// Ops returns the filesystem operations in order.
func (s *Server) Ops() []Op {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Op(nil), s.ops...)
}

// Commands returns the commands in order. Stdin is the data that the command
// read, which is how a test sees the bytes of a write.
func (s *Server) Commands() []Command {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Command(nil), s.cmds...)
}

// Seq returns the order number of the last recorded operation.
func (s *Server) Seq() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.seq
}

// FailCommand makes the next run of text return code and stderr.
func (s *Server) FailCommand(text string, code int, stderr string) {
	s.add(fault{kind: faultCommand, key: text, code: code, stderr: stderr})
}

// FailWrite makes the next write to path fail, as a full disk does.
func (s *Server) FailWrite(path, stderr string) {
	s.add(fault{kind: faultWrite, key: path, code: 1, stderr: stderr})
}

// FailRead makes the next read of path fail, as a file without permission does.
func (s *Server) FailRead(path, stderr string) {
	s.add(fault{kind: faultRead, key: path, code: 1, stderr: stderr})
}

// KeepRename makes the next rename onto path report success and leave the file
// as it was. It stages a node that does not keep what it was given.
func (s *Server) KeepRename(path string) {
	s.add(fault{kind: faultKeep, key: path})
}

// StopUnit makes the unit report inactive, as it does after bootstrap.
func (s *Server) StopUnit() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active = false
}

// LeaveUnitDown makes a restart report success and leave the unit inactive.
func (s *Server) LeaveUnitDown() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stayDown = true
}

func (s *Server) add(f fault) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.faults = append(s.faults, f)
}

// take consumes the first fault of a kind that matches key.
func (s *Server) take(kind, key string) (fault, bool) {
	for i, f := range s.faults {
		if f.kind == kind && f.key == key {
			s.faults = append(s.faults[:i], s.faults[i+1:]...)
			return f, true
		}
	}
	return fault{}, false
}

func (s *Server) next() int {
	s.seq++
	return s.seq
}

// The commands that the client sends. A handler reads one path, or two for a
// rename, from the text of the command.
const (
	movePrefix    = "mv -f "
	hashPrefix    = "sha256sum "
	writePrefix   = "umask 077; cat > "
	readPrefix    = "if [ ! -e "
	certPrefix    = "nebula-cert print -json -path "
	reloadPrefix  = "sudo -n systemctl reload "
	restartPrefix = "sudo -n systemctl restart "
)

// exec answers one command and changes the filesystem as the command asks. The
// caller holds no lock, and this takes it for the whole command.
func (s *Server) exec(cmd string, stdin []byte) node.Result {
	s.mu.Lock()
	defer s.mu.Unlock()

	if f, ok := s.take(faultCommand, cmd); ok {
		return node.Result{ExitCode: f.code, Stderr: f.stderr}
	}

	switch {
	case cmd == "nebula -version":
		return node.Result{Stdout: s.version}
	case strings.HasPrefix(cmd, "ip link show "):
		return node.Result{}
	case strings.HasPrefix(cmd, "systemctl cat "):
		return node.Result{}
	case strings.HasPrefix(cmd, "systemctl is-active "):
		if s.active {
			return node.Result{Stdout: "active\n"}
		}
		return node.Result{Stdout: "inactive\n", ExitCode: 3}
	case strings.HasPrefix(cmd, reloadPrefix):
		if !s.active {
			return node.Result{ExitCode: 1, Stderr: "Unit is not active, cannot reload."}
		}
		return node.Result{}
	case strings.HasPrefix(cmd, restartPrefix):
		if !s.stayDown {
			s.active = true
		}
		return node.Result{}
	case strings.HasPrefix(cmd, movePrefix):
		return s.move(cmd)
	case strings.HasPrefix(cmd, hashPrefix):
		return s.hash(cmd)
	case strings.HasPrefix(cmd, writePrefix):
		return s.write(cmd, stdin)
	case strings.HasPrefix(cmd, readPrefix):
		return s.read(cmd)
	case strings.HasPrefix(cmd, certPrefix):
		return s.printCert(cmd)
	}
	return unknown(cmd)
}

func (s *Server) move(cmd string) node.Result {
	w := words(strings.TrimPrefix(cmd, movePrefix))
	if len(w) != 2 {
		return unknown(cmd)
	}
	from, to := w[0], w[1]
	f, ok := s.files[from]
	if !ok {
		return node.Result{ExitCode: 1, Stderr: "mv: cannot stat " + from + ": No such file or directory"}
	}
	s.ops = append(s.ops, Op{Seq: s.next(), Kind: OpRename, Path: from, To: to})
	if _, ok := s.take(faultKeep, to); ok {
		return node.Result{}
	}
	delete(s.files, from)
	s.files[to] = f
	return node.Result{}
}

func (s *Server) hash(cmd string) node.Result {
	w := words(strings.TrimPrefix(cmd, hashPrefix))
	if len(w) != 1 {
		return unknown(cmd)
	}
	f, ok := s.files[w[0]]
	if !ok {
		return node.Result{ExitCode: 1, Stderr: "sha256sum: " + w[0] + ": No such file or directory"}
	}
	sum := sha256.Sum256(f.Body)
	return node.Result{Stdout: hex.EncodeToString(sum[:]) + "  " + w[0] + "\n"}
}

func (s *Server) write(cmd string, stdin []byte) node.Result {
	w := words(strings.TrimPrefix(cmd, writePrefix))
	if len(w) != 5 || w[1] != "&&" || w[2] != "chmod" || w[4] != w[0] {
		return unknown(cmd)
	}
	mode, err := strconv.ParseUint(w[3], 8, 32)
	if err != nil {
		return unknown(cmd)
	}
	path := w[0]
	if f, ok := s.take(faultWrite, path); ok {
		return node.Result{ExitCode: f.code, Stderr: f.stderr}
	}
	s.ops = append(s.ops, Op{Seq: s.next(), Kind: OpWrite, Path: path})
	s.files[path] = File{Body: append([]byte(nil), stdin...), Mode: fs.FileMode(mode)}
	return node.Result{}
}

func (s *Server) read(cmd string) node.Result {
	w := words(strings.TrimPrefix(cmd, readPrefix))
	if len(w) == 0 {
		return unknown(cmd)
	}
	path := w[0]
	if f, ok := s.take(faultRead, path); ok {
		return node.Result{ExitCode: f.code, Stderr: f.stderr}
	}
	f, ok := s.files[path]
	if !ok {
		return node.Result{ExitCode: missingExit}
	}
	s.ops = append(s.ops, Op{Seq: s.next(), Kind: OpRead, Path: path})
	return node.Result{Stdout: string(f.Body)}
}

func (s *Server) printCert(cmd string) node.Result {
	w := words(strings.TrimPrefix(cmd, certPrefix))
	if len(w) != 1 {
		return unknown(cmd)
	}
	f, ok := s.files[w[0]]
	if !ok {
		return node.Result{ExitCode: 1, Stderr: "open " + w[0] + ": no such file or directory"}
	}
	return node.Result{Stdout: string(f.Body)}
}

func unknown(cmd string) node.Result {
	return node.Result{ExitCode: 127, Stderr: "command not found: " + cmd}
}

// words splits a command tail into its arguments. Each argument is either a
// single quoted string, with '\” for a quote inside it, or bare words up to a
// space.
func words(s string) []string {
	var out []string
	for i := 0; i < len(s); {
		if s[i] == ' ' {
			i++
			continue
		}
		if s[i] == '\'' {
			var b strings.Builder
			i++
			for i < len(s) {
				if strings.HasPrefix(s[i:], `'\''`) {
					b.WriteByte('\'')
					i += 4
					continue
				}
				if s[i] == '\'' {
					i++
					break
				}
				b.WriteByte(s[i])
				i++
			}
			out = append(out, b.String())
			continue
		}
		start := i
		for i < len(s) && s[i] != ' ' {
			i++
		}
		out = append(out, s[start:i])
	}
	return out
}

// serve answers one connection from a client on the other end of a pipe.
func (s *Server) serve(conn net.Conn, cfg *ssh.ServerConfig) {
	sconn, chans, reqs, err := ssh.NewServerConn(conn, cfg)
	if err != nil {
		conn.Close()
		return
	}
	defer sconn.Close()
	go ssh.DiscardRequests(reqs)

	for nc := range chans {
		if nc.ChannelType() != "session" {
			nc.Reject(ssh.UnknownChannelType, "only sessions")
			continue
		}
		ch, requests, err := nc.Accept()
		if err != nil {
			continue
		}
		go s.session(ch, requests)
	}
}

func (s *Server) session(ch ssh.Channel, requests <-chan *ssh.Request) {
	defer ch.Close()
	for req := range requests {
		if req.Type != "exec" {
			req.Reply(false, nil)
			continue
		}
		var payload struct{ Command string }
		ssh.Unmarshal(req.Payload, &payload)
		req.Reply(true, nil)

		// Only a write carries data. Reading stdin for every command would
		// block, because a command with no data sends no end of file.
		var stdin []byte
		if strings.HasPrefix(payload.Command, writePrefix) {
			stdin, _ = io.ReadAll(ch)
		}

		res := s.exec(payload.Command, stdin)
		s.mu.Lock()
		s.cmds = append(s.cmds, Command{Seq: s.next(), Text: payload.Command, Stdin: stdin, Exit: res.ExitCode})
		s.mu.Unlock()

		if res.Stdout != "" {
			ch.Write([]byte(res.Stdout))
		}
		if res.Stderr != "" {
			ch.Stderr().Write([]byte(res.Stderr))
		}
		ch.SendRequest("exit-status", false, ssh.Marshal(struct{ C uint32 }{uint32(res.ExitCode)}))
		return
	}
}

// hostSigner and clientSigner build a throwaway key pair for one connection.
func keyPair() (ssh.Signer, ssh.PublicKey, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		return nil, nil, err
	}
	key, err := ssh.NewPublicKey(pub)
	if err != nil {
		return nil, nil, err
	}
	return signer, key, nil
}

func hostKey(hostSigner ssh.Signer) ssh.HostKeyCallback {
	want := hostSigner.PublicKey().Marshal()
	return func(_ string, _ net.Addr, key ssh.PublicKey) error {
		if string(key.Marshal()) != string(want) {
			return fmt.Errorf("host key does not match the server")
		}
		return nil
	}
}
