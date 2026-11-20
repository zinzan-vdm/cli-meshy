package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zinzan-vdm/cli-meshy/internal/config"
	"github.com/zinzan-vdm/cli-meshy/internal/vault"
)

// authorityPrint is the answer that `nebula-cert print -json` gives for a trust
// bundle that holds names. A real print has more fields, and the command reads
// the name and the fingerprint only.
func authorityPrint(names ...string) string {
	docs := make([]string, 0, len(names))
	for _, name := range names {
		docs = append(docs, `{"details":{"name":"`+name+`","isCa":true},"fingerprint":"ff`+name+`"}`)
	}
	return "[" + strings.Join(docs, ",") + "]"
}

// rotateNode sets the answer that the node gives for its trust bundle, so the
// read back compares the list that the test names.
func rotateNode(nodes map[string]*fakeNode, name string, trust ...string) {
	nodes[name].prints["/etc/nebula/meshy-homelab/ca.crt"] = authorityPrint(trust...)
}

func TestCARotatePhase1CreatesTheNextAuthority(t *testing.T) {
	dir := writeApplyMesh(t)
	openApplyVault(t, dir, "core-1", "web-1")
	stubBinary(t)
	t.Setenv("MESHY_KNOWN_HOSTS", filepath.Join(dir, "known_hosts"))
	nodes := useFakes(t, "core-1", "web-1")
	rotateNode(nodes, "core-1", "ca-1", "ca-2")
	rotateNode(nodes, "web-1", "ca-1", "ca-2")

	out, errOut, code := captureBoth(t, func() int { return run([]string{"ca", "rotate", "--phase", "1", "--mesh", dir}) })
	if code != 0 {
		t.Fatalf("exit = %d, want 0: %s%s", code, out, errOut)
	}
	for _, want := range []string{
		"authority ca-2 is sealed in " + vault.FileName,
		"mesh.yml now trusts ca-1 and ca-2",
		"next: meshy ca rotate --phase 2",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the output does not hold %q:\n%s", want, out)
		}
	}

	record, err := vault.Read(filepath.Join(dir, vault.DirName))
	if err != nil {
		t.Fatalf("read the vault: %v", err)
	}
	if _, _, found := vault.Certificate(record, "ca-2"); !found {
		t.Error("the vault holds no ca-2")
	}
	if _, ok := record["ca-2.issued"]; !ok {
		t.Error("the vault holds no issue time for ca-2")
	}
	if _, _, found := vault.Certificate(record, "ca-1"); !found {
		t.Error("the run removed ca-1 from the vault")
	}

	mesh, err := config.Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(mesh.CA.Trust) != 2 || mesh.CA.Trust[0] != "ca-1" || mesh.CA.Trust[1] != "ca-2" {
		t.Errorf("trust = %v, want [ca-1 ca-2]", mesh.CA.Trust)
	}
	if mesh.CA.Active != "ca-1" {
		t.Errorf("active = %q, want ca-1", mesh.CA.Active)
	}

	for _, name := range []string{"core-1", "web-1"} {
		bundle := nodes[name].read("/etc/nebula/meshy-homelab/ca.crt")
		if !strings.Contains(bundle, "authority certificate") || !strings.Contains(bundle, "STUB CRT") {
			t.Errorf("%s holds the bundle %q", name, bundle)
		}
		if nodes[name].reloads != 1 {
			t.Errorf("%s reloaded %d times, want 1", name, nodes[name].reloads)
		}
	}
}

func TestCARotatePhase1IssuesNoCertificate(t *testing.T) {
	dir := writeApplyMesh(t)
	openApplyVault(t, dir, "core-1", "web-1")
	stubBinary(t)
	t.Setenv("MESHY_KNOWN_HOSTS", filepath.Join(dir, "known_hosts"))
	nodes := useFakes(t, "core-1", "web-1")
	rotateNode(nodes, "core-1", "ca-1", "ca-2")
	rotateNode(nodes, "web-1", "ca-1", "ca-2")

	out, errOut, code := captureBoth(t, func() int {
		return run([]string{"ca", "rotate", "--phase", "1", "--mesh", dir})
	})
	if code != 0 {
		t.Fatalf("exit = %d, want 0: %s%s", code, out, errOut)
	}

	body, err := os.ReadFile(os.Getenv("STUB_ARGS"))
	if err != nil {
		t.Fatalf("read the arguments: %v", err)
	}
	args := string(body)
	if !strings.HasPrefix(args, "ca\n") {
		t.Errorf("the tool was not asked for an authority: %s", args)
	}
	if strings.Contains(args, "sign") {
		t.Errorf("the tool was asked to sign: %s", args)
	}
	if !strings.Contains(args, "-name\nca-2\n") {
		t.Errorf("the tool was not asked for ca-2: %s", args)
	}
	if got := nodes["core-1"].read("/etc/nebula/meshy-homelab/core-1.crt"); got != "core-1 certificate\n" {
		t.Errorf("the node certificate is %q, want the certificate from the vault", got)
	}
}

func TestCARotatePhase1RefusesATrustOfTwo(t *testing.T) {
	dir := writeApplyMesh(t)
	path := filepath.Join(dir, config.FileName)
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read mesh: %v", err)
	}
	broken := strings.Replace(string(body), "trust: [ca-1]", "trust: [ca-1, ca-2]", 1)
	if err := os.WriteFile(path, []byte(broken), 0o600); err != nil {
		t.Fatalf("write mesh: %v", err)
	}

	stderr, code := captureErr(t, func() int {
		return run([]string{"ca", "rotate", "--phase", "1", "--mesh", dir})
	})
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if !strings.Contains(stderr, "ca.trust already holds two authorities") {
		t.Errorf("message = %q, want the refusal", stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, vault.FileName)); err == nil {
		t.Error("a refused run sealed a vault")
	}
}

func TestCARotatePhase1SaysUnlockWhenTheVaultIsClosed(t *testing.T) {
	dir := writeApplyMesh(t)
	t.Setenv("MESHY_KNOWN_HOSTS", filepath.Join(dir, "known_hosts"))

	stderr, code := captureErr(t, func() int {
		return run([]string{"ca", "rotate", "--phase", "1", "--mesh", dir})
	})
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if !strings.Contains(stderr, "the vault is locked, run meshy unlock") {
		t.Errorf("message = %q, want the unlock line", stderr)
	}

	mesh, err := config.Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(mesh.CA.Trust) != 1 {
		t.Errorf("a refused run changed ca.trust: %v", mesh.CA.Trust)
	}
}

func TestCARotatePhase1ContinuesAfterANodeFails(t *testing.T) {
	dir := writeApplyMesh(t)
	openApplyVault(t, dir, "core-1", "web-1")
	stubBinary(t)
	t.Setenv("MESHY_KNOWN_HOSTS", filepath.Join(dir, "known_hosts"))
	nodes := useFakes(t, "core-1", "web-1")
	nodes["core-1"].failAll = true
	rotateNode(nodes, "web-1", "ca-1", "ca-2")

	out, _, code := captureBoth(t, func() int {
		return run([]string{"ca", "rotate", "--phase", "1", "--mesh", dir})
	})
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if !strings.Contains(out, "core-1: failed: ") {
		t.Errorf("the report does not name the failed node:\n%s", out)
	}
	if !strings.Contains(out, "web-1: wrote") {
		t.Errorf("the run stopped before the second node:\n%s", out)
	}
	if nodes["web-1"].reloads != 1 {
		t.Errorf("web-1 reloaded %d times, want 1", nodes["web-1"].reloads)
	}
}

func TestCARotatePhase1ReportsANodeThatDidNotTakeTheBundle(t *testing.T) {
	dir := writeApplyMesh(t)
	openApplyVault(t, dir, "core-1")
	stubBinary(t)
	t.Setenv("MESHY_KNOWN_HOSTS", filepath.Join(dir, "known_hosts"))
	nodes := useFakes(t, "core-1")
	rotateNode(nodes, "core-1", "ca-1")

	out, _, code := captureBoth(t, func() int {
		return run([]string{"ca", "rotate", "--phase", "1", "--mesh", dir})
	})
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if !strings.Contains(out, "does not hold ca-2") {
		t.Errorf("the report does not name the missing authority:\n%s", out)
	}
}

func TestCARotateNeedsAPhase(t *testing.T) {
	dir := writeApplyMesh(t)

	stderr, code := captureErr(t, func() int { return run([]string{"ca", "rotate", "--mesh", dir}) })
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if !strings.Contains(stderr, "--phase 1") {
		t.Errorf("message = %q, want the phase", stderr)
	}
}

func TestCARotateRefusesAPhaseThatIsNotBuilt(t *testing.T) {
	dir := writeApplyMesh(t)

	stderr, code := captureErr(t, func() int {
		return run([]string{"ca", "rotate", "--phase", "4", "--mesh", dir})
	})
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if !strings.Contains(stderr, "phase 4") {
		t.Errorf("message = %q, want the phase", stderr)
	}
}

func TestCARotateIsInTheDispatchAndTheUsage(t *testing.T) {
	out, _, code := captureBoth(t, func() int { return run([]string{"help"}) })
	if code != 0 {
		t.Fatalf("help exit = %d, want 0", code)
	}
	if !strings.Contains(out, "ca rotate  rotate the trust anchor") {
		t.Errorf("the usage does not list ca rotate:\n%s", out)
	}
	if strings.Contains(out, "status, ca rotate") {
		t.Error("the usage still lists ca rotate as planned")
	}
}

// rotatedMeshDir writes the mesh that phase 1 leaves behind: two trusted
// authorities, and the one that the argument names as active.
func rotatedMeshDir(t *testing.T, active string) string {
	t.Helper()

	dir := t.TempDir()
	body := strings.Replace(issueMesh, "trust: [ca-1]", "trust: [ca-1, ca-2]", 1)
	body = strings.Replace(body, "active: ca-1", "active: "+active, 1)
	if err := os.WriteFile(filepath.Join(dir, config.FileName), []byte(body), 0o600); err != nil {
		t.Fatalf("write mesh: %v", err)
	}
	return dir
}

// openRotateVault seals a vault that holds both authorities and the named
// nodes, then opens it.
func openRotateVault(t *testing.T, meshDir string, names ...string) {
	t.Helper()

	record := vault.Record{
		"ca-1.crt": []byte("authority certificate\n"),
		"ca-1.key": []byte("authority key\n"),
		"ca-2.crt": []byte("second authority certificate\n"),
		"ca-2.key": []byte("second authority key\n"),
	}
	for _, name := range names {
		vault.SetCertificate(record, name, []byte(name+" certificate\n"), []byte(name+" key\n"))
	}

	plaintext, err := vault.Encode(record)
	if err != nil {
		t.Fatalf("encode the vault: %v", err)
	}
	sealed, shares, err := vault.Seal(plaintext)
	if err != nil {
		t.Fatalf("seal the vault: %v", err)
	}
	if err := os.WriteFile(filepath.Join(meshDir, vault.FileName), sealed, 0o600); err != nil {
		t.Fatalf("write the vault: %v", err)
	}
	if _, err := vault.Unlock(meshDir, shares[:vault.Threshold], false); err != nil {
		t.Fatalf("unlock the vault: %v", err)
	}
}

// rotateCert sets the answer that the node gives for its own certificate, so
// the read back compares the issuer with the authority of the phase.
func rotateCert(nodes map[string]*fakeNode, name, issuer string) {
	nodes[name].prints["/etc/nebula/meshy-homelab/"+name+".crt"] =
		`[{"details":{"name":"` + name + `","issuer":"` + issuer + `","isCa":false},"fingerprint":"bb` + name + `"}]`
}

// rotateCertAfter sets the answer that the node gives for its own certificate
// once the run has written it, so the read back proves the write.
func rotateCertAfter(nodes map[string]*fakeNode, name, issuer string) {
	path := "/etc/nebula/meshy-homelab/" + name + ".crt"
	// The answer of the read back applies once the run has written the file, so
	// a file from an earlier run must not count as a write of this one.
	delete(nodes[name].files, path)
	nodes[name].printsAfter[path] =
		`[{"details":{"name":"` + name + `","issuer":"` + issuer + `","isCa":false},"fingerprint":"cc` + name + `"}]`
}

func TestCARotatePhase2SignsEveryNodeWithTheNewAuthority(t *testing.T) {
	dir := rotatedMeshDir(t, "ca-1")
	openRotateVault(t, dir, "core-1", "web-1")
	stubSigner(t)
	t.Setenv("MESHY_KNOWN_HOSTS", filepath.Join(dir, "known_hosts"))
	nodes := useFakes(t, "core-1", "web-1")
	rotateNode(nodes, "core-1", "ca-1", "ca-2")
	rotateNode(nodes, "web-1", "ca-1", "ca-2")
	rotateCert(nodes, "core-1", "the-old-authority")
	rotateCert(nodes, "web-1", "the-old-authority")
	rotateCertAfter(nodes, "core-1", "4f3a")
	rotateCertAfter(nodes, "web-1", "4f3a")

	out, errOut, code := captureBoth(t, func() int {
		return run([]string{"ca", "rotate", "--phase", "2", "--mesh", dir})
	})
	if code != 0 {
		t.Fatalf("exit = %d, want 0: %s%s", code, out, errOut)
	}
	for _, want := range []string{
		"authority ca-2 now signs new certificates",
		"mesh.yml now makes ca-2 active",
		"core-1: signed by ca-2, renewed, wrote",
		"web-1: signed by ca-2, renewed, wrote",
		"next: meshy ca rotate --phase 3",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the output does not hold %q:\n%s", want, out)
		}
	}

	mesh, err := config.Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if mesh.CA.Active != "ca-2" {
		t.Errorf("active = %q, want ca-2", mesh.CA.Active)
	}
	if len(mesh.CA.Trust) != 2 {
		t.Errorf("trust = %v, want two authorities", mesh.CA.Trust)
	}

	record, err := vault.Read(filepath.Join(dir, vault.DirName))
	if err != nil {
		t.Fatalf("read the vault: %v", err)
	}
	cert, _, found := vault.Certificate(record, "core-1")
	if !found || !strings.Contains(string(cert), "STUB CRT") {
		t.Errorf("the vault holds the certificate %q, want the one that ca-2 signed", cert)
	}
	kept := vault.PreviousAuthorities(record, "core-1")
	if len(kept) != 1 {
		t.Fatalf("the vault holds %d kept pairs for core-1, want one", len(kept))
	}
	old, oldKey, found := vault.Previous(record, "core-1", kept[0])
	if !found {
		t.Fatal("the vault dropped the certificate that the old authority signed")
	}
	if string(old) != "core-1 certificate\n" || string(oldKey) != "core-1 key\n" {
		t.Errorf("the vault kept %q and %q, want the old pair byte for byte", old, oldKey)
	}

	for _, name := range []string{"core-1", "web-1"} {
		if got := nodes[name].read("/etc/nebula/meshy-homelab/" + name + ".crt"); !strings.Contains(got, "STUB CRT") {
			t.Errorf("%s holds %q, want the new certificate", name, got)
		}
		if nodes[name].reloads != 1 {
			t.Errorf("%s reloaded %d times, want 1", name, nodes[name].reloads)
		}
	}
}

func TestCARotatePhase2RefusesAKeptCertificateWithNoIssuer(t *testing.T) {
	dir := rotatedMeshDir(t, "ca-1")
	openRotateVault(t, dir, "core-1")
	stubSigner(t)
	t.Setenv("STUB_ISSUER", "")
	t.Setenv("MESHY_KNOWN_HOSTS", filepath.Join(dir, "known_hosts"))
	nodes := useFakes(t, "core-1")
	rotateNode(nodes, "core-1", "ca-1", "ca-2")
	rotateCert(nodes, "core-1", "the-old-authority")

	out, errOut, code := captureBoth(t, func() int {
		return run([]string{"ca", "rotate", "--phase", "2", "--mesh", dir})
	})
	if code != 1 {
		t.Fatalf("exit = %d, want 1: %s%s", code, out, errOut)
	}
	if !strings.Contains(out, "names no issuer") {
		t.Errorf("the report does not name the refusal:\n%s", out)
	}

	record := readVault(t, dir)
	if kept := vault.PreviousAuthorities(record, "core-1"); len(kept) != 0 {
		t.Errorf("the refused run kept a pair under %v, want none", kept)
	}
	cert, _, found := vault.Certificate(record, "core-1")
	if !found || string(cert) != "core-1 certificate\n" {
		t.Errorf("the vault holds %q, want the pair that it held before the run", cert)
	}
	if nodes["core-1"].writes != 0 || nodes["core-1"].reloads != 0 {
		t.Errorf("core-1 saw %d writes and %d reloads, want none",
			nodes["core-1"].writes, nodes["core-1"].reloads)
	}
}

func TestCARotatePhase2ChangesOneLineOfMeshYml(t *testing.T) {
	dir := rotatedMeshDir(t, "ca-1")
	path := filepath.Join(dir, config.FileName)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read mesh: %v", err)
	}
	openRotateVault(t, dir, "core-1", "web-1")
	stubSigner(t)
	t.Setenv("MESHY_KNOWN_HOSTS", filepath.Join(dir, "known_hosts"))
	nodes := useFakes(t, "core-1", "web-1")
	rotateNode(nodes, "core-1", "ca-1", "ca-2")
	rotateNode(nodes, "web-1", "ca-1", "ca-2")
	rotateCert(nodes, "core-1", "the-old-authority")
	rotateCert(nodes, "web-1", "the-old-authority")
	rotateCertAfter(nodes, "core-1", "4f3a")
	rotateCertAfter(nodes, "web-1", "4f3a")

	out, errOut, code := captureBoth(t, func() int {
		return run([]string{"ca", "rotate", "--phase", "2", "--mesh", dir})
	})
	if code != 0 {
		t.Fatalf("exit = %d, want 0: %s%s", code, out, errOut)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read mesh: %v", err)
	}
	want := strings.Replace(string(before), "active: ca-1", "active: ca-2", 1)
	if string(after) != want {
		t.Errorf("the file changed by more than the active line:\n%s", after)
	}
}

func TestCARotatePhase2RefusesWhileTrustHoldsOneAuthority(t *testing.T) {
	dir := writeApplyMesh(t)
	openApplyVault(t, dir, "core-1")

	stderr, code := captureErr(t, func() int {
		return run([]string{"ca", "rotate", "--phase", "2", "--mesh", dir})
	})
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if !strings.Contains(stderr, "run phase 1 first") {
		t.Errorf("message = %q, want the refusal", stderr)
	}

	mesh, err := config.Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if mesh.CA.Active != "ca-1" {
		t.Errorf("a refused run changed ca.active to %q", mesh.CA.Active)
	}
}

func TestCARotatePhase2SaysUnlockWhenTheVaultIsClosed(t *testing.T) {
	dir := rotatedMeshDir(t, "ca-1")
	t.Setenv("MESHY_KNOWN_HOSTS", filepath.Join(dir, "known_hosts"))

	stderr, code := captureErr(t, func() int {
		return run([]string{"ca", "rotate", "--phase", "2", "--mesh", dir})
	})
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if !strings.Contains(stderr, "the vault is locked, run meshy unlock") {
		t.Errorf("message = %q, want the unlock line", stderr)
	}

	mesh, err := config.Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if mesh.CA.Active != "ca-1" {
		t.Errorf("a refused run changed ca.active to %q", mesh.CA.Active)
	}
}

func TestCARotatePhase2StopsAtANodeThatFailsAndLeavesTheRest(t *testing.T) {
	dir := rotatedMeshDir(t, "ca-1")
	openRotateVault(t, dir, "core-1", "web-1")
	stubSigner(t)
	t.Setenv("MESHY_KNOWN_HOSTS", filepath.Join(dir, "known_hosts"))
	nodes := useFakes(t, "core-1", "web-1")
	nodes["core-1"].failAll = true
	rotateNode(nodes, "web-1", "ca-1", "ca-2")
	rotateCert(nodes, "web-1", "4f3a")

	out, _, code := captureBoth(t, func() int {
		return run([]string{"ca", "rotate", "--phase", "2", "--mesh", dir})
	})
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if !strings.Contains(out, "core-1: failed: ") {
		t.Errorf("the report does not name the failed node:\n%s", out)
	}
	if !strings.Contains(out, "web-1: not reached") {
		t.Errorf("the report does not name the node that the phase left alone:\n%s", out)
	}
	if !strings.Contains(out, "phase 2 stopped at core-1") {
		t.Errorf("the report does not name where the phase stopped:\n%s", out)
	}
	if strings.Contains(out, "next: meshy ca rotate --phase 3") {
		t.Error("a stopped phase named the next phase")
	}
	if len(nodes["web-1"].log) != 0 {
		t.Errorf("web-1 saw %d commands after the phase stopped", len(nodes["web-1"].log))
	}

	record, err := vault.Read(filepath.Join(dir, vault.DirName))
	if err != nil {
		t.Fatalf("read the vault: %v", err)
	}
	if cert, _, _ := vault.Certificate(record, "web-1"); string(cert) != "web-1 certificate\n" {
		t.Errorf("the vault holds %q for web-1, want the certificate that ca-1 signed", cert)
	}
	if _, _, found := vault.Previous(record, "web-1", "ca-1"); found {
		t.Error("the vault moved the certificate of a node that the phase left alone")
	}
}

func TestCARotatePhase2ReportsNoChangeWhenTheNewAuthorityIsActive(t *testing.T) {
	dir := rotatedMeshDir(t, "ca-2")
	path := filepath.Join(dir, config.FileName)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read mesh: %v", err)
	}
	openRotateVault(t, dir, "core-1", "web-1")
	stubSigner(t)
	t.Setenv("MESHY_KNOWN_HOSTS", filepath.Join(dir, "known_hosts"))
	nodes := useFakes(t, "core-1", "web-1")
	rotateNode(nodes, "core-1", "ca-1", "ca-2")
	rotateNode(nodes, "web-1", "ca-1", "ca-2")
	rotateCert(nodes, "core-1", "4f3a")
	rotateCert(nodes, "web-1", "4f3a")

	out, errOut, code := captureBoth(t, func() int {
		return run([]string{"ca", "rotate", "--phase", "2", "--mesh", dir})
	})
	if code != 0 {
		t.Fatalf("exit = %d, want 0: %s%s", code, out, errOut)
	}
	if !strings.Contains(out, "no change") {
		t.Errorf("the report does not say no change:\n%s", out)
	}
	if !strings.Contains(out, "next: meshy ca rotate --phase 3") {
		t.Errorf("the report does not name the next phase:\n%s", out)
	}
	// The run reads the issuer of each node, because that read is the proof
	// that the new authority already signed every certificate.
	for name, node := range nodes {
		if node.writes != 0 {
			t.Errorf("%s saw %d writes, want none", name, node.writes)
		}
		if node.reloads != 0 {
			t.Errorf("%s reloaded %d times, want none", name, node.reloads)
		}
		want := "nebula-cert print -json -path /etc/nebula/meshy-homelab/" + name + ".crt"
		if len(node.log) != 1 || node.log[0] != want {
			t.Errorf("%s saw %v, want the one read %q", name, node.log, want)
		}
	}

	record, err := vault.Read(filepath.Join(dir, vault.DirName))
	if err != nil {
		t.Fatalf("read the vault: %v", err)
	}
	if cert, _, _ := vault.Certificate(record, "core-1"); string(cert) != "core-1 certificate\n" {
		t.Errorf("the vault holds %q, want the certificate that it held before the run", cert)
	}
	if _, _, found := vault.Previous(record, "core-1", "ca-1"); found {
		t.Error("a run with nothing to do moved a certificate")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read mesh: %v", err)
	}
	if string(after) != string(before) {
		t.Errorf("a run with nothing to do changed mesh.yml:\n%s", after)
	}
}

func TestCARotatePhase2ReportsANodeThatKeptTheOldCertificate(t *testing.T) {
	dir := rotatedMeshDir(t, "ca-1")
	openRotateVault(t, dir, "core-1")
	stubSigner(t)
	t.Setenv("MESHY_KNOWN_HOSTS", filepath.Join(dir, "known_hosts"))
	nodes := useFakes(t, "core-1")
	rotateNode(nodes, "core-1", "ca-1", "ca-2")
	rotateCert(nodes, "core-1", "old-authority")

	out, _, code := captureBoth(t, func() int {
		return run([]string{"ca", "rotate", "--phase", "2", "--mesh", dir})
	})
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if !strings.Contains(out, "ca-2 did not sign its certificate") {
		t.Errorf("the report does not name the issuer that the node reported:\n%s", out)
	}
}

func TestCARotatePhase2FinishesANodeThatAStoppedRunLeft(t *testing.T) {
	dir := rotatedMeshDir(t, "ca-2")
	path := filepath.Join(dir, config.FileName)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read mesh: %v", err)
	}
	openRotateVault(t, dir, "core-1", "web-1")
	stubSigner(t)
	t.Setenv("MESHY_KNOWN_HOSTS", filepath.Join(dir, "known_hosts"))
	nodes := useFakes(t, "core-1", "web-1")
	rotateNode(nodes, "core-1", "ca-1", "ca-2")
	rotateNode(nodes, "web-1", "ca-1", "ca-2")
	rotateCert(nodes, "core-1", "4f3a")
	rotateCert(nodes, "web-1", "the-old-authority")
	nodes["web-1"].printsAfter["/etc/nebula/meshy-homelab/web-1.crt"] =
		`[{"details":{"name":"web-1","issuer":"4f3a","isCa":false},"fingerprint":"bbweb-1"}]`

	out, errOut, code := captureBoth(t, func() int {
		return run([]string{"ca", "rotate", "--phase", "2", "--mesh", dir})
	})
	if code != 0 {
		t.Fatalf("exit = %d, want 0: %s%s", code, out, errOut)
	}
	if !strings.Contains(out, "web-1: signed by ca-2") {
		t.Errorf("the run did not finish web-1:\n%s", out)
	}
	if strings.Contains(out, "no change") {
		t.Errorf("a run that changed a node reported no change:\n%s", out)
	}
	if nodes["core-1"].writes != 0 {
		t.Errorf("core-1 saw %d writes, want none", nodes["core-1"].writes)
	}
	if nodes["web-1"].reloads != 1 {
		t.Errorf("web-1 reloaded %d times, want 1", nodes["web-1"].reloads)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read mesh: %v", err)
	}
	if string(after) != string(before) {
		t.Errorf("the run changed a mesh file that already named ca-2:\n%s", after)
	}
}

func TestSignerOfNamesTheAuthorityAndKeepsAnUnknownFingerprint(t *testing.T) {
	signers := map[string]string{"aa11": "ca-1"}

	if got := signerOf(signers, "AA11"); got != "ca-1" {
		t.Errorf("signerOf = %q, want ca-1", got)
	}
	if got := signerOf(signers, "bb22"); got != "bb22" {
		t.Errorf("signerOf = %q, want the fingerprint that it cannot name", got)
	}
}

// phase3Mesh is the state that phase 2 leaves behind: two trusted authorities,
// ca-2 active, and a certificate on each node that the active authority signed.
func phase3Mesh(t *testing.T, issuers map[string]string) (string, map[string]*fakeNode) {
	t.Helper()

	dir := rotatedMeshDir(t, "ca-2")
	openRotateVault(t, dir, "core-1", "web-1")
	stubSigner(t)
	t.Setenv("MESHY_KNOWN_HOSTS", filepath.Join(dir, "known_hosts"))

	nodes := useFakes(t, "core-1", "web-1")
	for _, name := range []string{"core-1", "web-1"} {
		rotateNode(nodes, name, "ca-1", "ca-2")
		rotateCert(nodes, name, issuers[name])
		nodes[name].printsAfter["/etc/nebula/meshy-homelab/ca.crt"] = authorityPrint("ca-2")
	}
	return dir, nodes
}

func TestCARotatePhase3DropsTheOldAuthority(t *testing.T) {
	dir, nodes := phase3Mesh(t, map[string]string{"core-1": "4f3a", "web-1": "4f3a"})

	out, errOut, code := captureBoth(t, func() int {
		return run([]string{"ca", "rotate", "--phase", "3", "--mesh", dir})
	})
	if code != 0 {
		t.Fatalf("exit = %d, want 0: %s%s", code, out, errOut)
	}
	for _, want := range []string{
		"mesh.yml no longer trusts ca-1",
		"ca-1.key stays in " + vault.FileName,
		"the rotation is complete",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the output does not hold %q:\n%s", want, out)
		}
	}

	mesh, err := config.Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(mesh.CA.Trust) != 1 || mesh.CA.Trust[0] != "ca-2" {
		t.Errorf("trust = %v, want ca-2 alone", mesh.CA.Trust)
	}
	if mesh.CA.Active != "ca-2" {
		t.Errorf("active = %q, want ca-2", mesh.CA.Active)
	}

	// The key stays, because a deleted key cannot explain a certificate that
	// survives in a backup.
	record, err := vault.Read(filepath.Join(dir, vault.DirName))
	if err != nil {
		t.Fatalf("read the vault: %v", err)
	}
	cert, key, found := vault.Certificate(record, "ca-1")
	if !found {
		t.Fatal("the vault dropped ca-1")
	}
	if string(cert) != "authority certificate\n" || string(key) != "authority key\n" {
		t.Errorf("the vault holds %q and %q, want the old pair byte for byte", cert, key)
	}

	for _, name := range []string{"core-1", "web-1"} {
		if nodes[name].writes == 0 {
			t.Errorf("%s took no bundle", name)
		}
	}
}

func TestCARotatePhase3ChangesOneLineOfMeshYml(t *testing.T) {
	dir, _ := phase3Mesh(t, map[string]string{"core-1": "4f3a", "web-1": "4f3a"})
	path := filepath.Join(dir, config.FileName)

	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read mesh: %v", err)
	}

	if _, errOut, code := captureBoth(t, func() int {
		return run([]string{"ca", "rotate", "--phase", "3", "--mesh", dir})
	}); code != 0 {
		t.Fatalf("exit = %d, want 0: %s", code, errOut)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read mesh: %v", err)
	}
	want := strings.Replace(string(before), "trust: [ca-1, ca-2]", "trust: [ca-2]", 1)
	if string(after) != want {
		t.Errorf("mesh.yml =\n%s\nwant\n%s", after, want)
	}
}

func TestCARotatePhase3RefusesWhileANodeHoldsTheOldCertificate(t *testing.T) {
	dir, nodes := phase3Mesh(t, map[string]string{"core-1": "ffca-1", "web-1": "4f3a"})
	path := filepath.Join(dir, config.FileName)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read mesh: %v", err)
	}

	_, errOut, code := captureBoth(t, func() int {
		return run([]string{"ca", "rotate", "--phase", "3", "--mesh", dir})
	})
	if code != 1 {
		t.Fatalf("exit = %d, want 1: %s", code, errOut)
	}
	if !strings.Contains(errOut, "core-1 still holds a certificate from") {
		t.Errorf("message = %q, want the node and its authority", errOut)
	}

	// A refusal leaves the file and the nodes exactly as they were.
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read mesh: %v", err)
	}
	if string(after) != string(before) {
		t.Error("the refusal changed mesh.yml")
	}
	for _, name := range []string{"core-1", "web-1"} {
		if nodes[name].writes != 0 {
			t.Errorf("%s took a write from a refused phase", name)
		}
	}
}

func TestCARotatePhase3RefusesATrustOfOne(t *testing.T) {
	dir := writeApplyMesh(t)
	openApplyVault(t, dir, "core-1")
	stubSigner(t)
	t.Setenv("MESHY_KNOWN_HOSTS", filepath.Join(dir, "known_hosts"))
	useFakes(t, "core-1")

	stderr, code := captureErr(t, func() int {
		return run([]string{"ca", "rotate", "--phase", "3", "--mesh", dir})
	})
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if !strings.Contains(stderr, "the rotation is already complete") {
		t.Errorf("message = %q, want the completed rotation", stderr)
	}
}

func TestCARotatePhase3ReportsANodeThatKeptTheOldAuthority(t *testing.T) {
	dir, nodes := phase3Mesh(t, map[string]string{"core-1": "4f3a", "web-1": "4f3a"})
	// web-1 reports a bundle that still holds the authority the file dropped.
	nodes["web-1"].printsAfter["/etc/nebula/meshy-homelab/ca.crt"] = authorityPrint("ca-1", "ca-2")

	out, _, code := captureBoth(t, func() int {
		return run([]string{"ca", "rotate", "--phase", "3", "--mesh", dir})
	})
	if code != 1 {
		t.Errorf("exit = %d, want 1: %s", code, out)
	}
	if !strings.Contains(out, "web-1: failed: the trust bundle on the node still holds ca-1") {
		t.Errorf("output =\n%s\nwant the node and the authority that it still holds", out)
	}
	if strings.Contains(out, "the rotation is complete") {
		t.Error("the phase called a rotation complete while a bundle still held the old authority")
	}
}

func TestCARotatePhase3RefusesBeforePhase2(t *testing.T) {
	// Both authorities are trusted and ca-1 still signs, which is the state
	// that phase 1 leaves behind. Phase 3 would drop ca-2, the authority that
	// the mesh is moving to.
	dir := rotatedMeshDir(t, "ca-1")
	openRotateVault(t, dir, "core-1", "web-1")
	stubSigner(t)
	t.Setenv("MESHY_KNOWN_HOSTS", filepath.Join(dir, "known_hosts"))
	nodes := useFakes(t, "core-1", "web-1")
	for _, name := range []string{"core-1", "web-1"} {
		rotateNode(nodes, name, "ca-1", "ca-2")
		rotateCert(nodes, name, "4f3a")
	}

	path := filepath.Join(dir, config.FileName)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read mesh: %v", err)
	}

	stderr, code := captureErr(t, func() int {
		return run([]string{"ca", "rotate", "--phase", "3", "--mesh", dir})
	})
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if !strings.Contains(stderr, "run phase 2 first") {
		t.Errorf("message = %q, want the phase before this one", stderr)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read mesh: %v", err)
	}
	if string(after) != string(before) {
		t.Error("the refused phase changed mesh.yml")
	}
	for _, name := range []string{"core-1", "web-1"} {
		if nodes[name].writes != 0 {
			t.Errorf("%s took a write from a refused phase", name)
		}
	}
}
