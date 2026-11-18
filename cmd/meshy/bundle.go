package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/zinzan-vdm/cli-meshy/internal/bootstrap"
	"github.com/zinzan-vdm/cli-meshy/internal/bundle"
	"github.com/zinzan-vdm/cli-meshy/internal/config"
	"github.com/zinzan-vdm/cli-meshy/internal/install"
	"github.com/zinzan-vdm/cli-meshy/internal/nebula"
	"github.com/zinzan-vdm/cli-meshy/internal/node"
	"github.com/zinzan-vdm/cli-meshy/internal/release"
	"github.com/zinzan-vdm/cli-meshy/internal/revoke"
	"github.com/zinzan-vdm/cli-meshy/internal/scaffold/installers"
	"github.com/zinzan-vdm/cli-meshy/internal/vault"
)

// bundleDir is where a mesh keeps the archives it builds. The path holds a key,
// so the directory is 0700 and the archive is 0600.
const bundleDir = "bundles"

// windowsData is where a Windows host keeps the files of one mesh.
const windowsData = `C:\ProgramData\nebula`

func cmdBundle(args []string) int {
	flags := flag.NewFlagSet("bundle", flag.ContinueOnError)
	noBinary := flags.Bool("no-binary", false, "leave the pinned binary out")
	meshDir := flags.String("mesh", ".", "directory that holds mesh.yml")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	// The synopsis puts the node before the flags, and flag.Parse stops at the
	// first name it sees, so the arguments after the node are read as flags.
	name := flags.Arg(0)
	rest := flags.Args()
	if len(rest) > 0 {
		rest = rest[1:]
	}
	if err := flags.Parse(rest); err != nil {
		return 2
	}
	if flags.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "meshy bundle: unexpected argument %q\n", flags.Arg(0))
		return 2
	}
	if name == "" {
		fmt.Fprintln(os.Stderr, "meshy bundle: name a node")
		return 2
	}

	mesh, err := config.Load(*meshDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "meshy bundle: %v\n", err)
		return 2
	}
	if problems := mesh.Validate(); len(problems) > 0 {
		fmt.Fprintf(os.Stderr, "mesh %q has %d problems:\n", mesh.Name, len(problems))
		for _, problem := range problems {
			fmt.Fprintf(os.Stderr, "  - %s\n", problem)
		}
		return 2
	}

	target, found := applyTarget(*mesh, name)
	if !found {
		fmt.Fprintf(os.Stderr, "meshy bundle: node %q is not in %s\n", name, config.FileName)
		return 2
	}
	if target.Kind != config.KindUser {
		fmt.Fprintf(os.Stderr, "meshy bundle: %s is a server node, apply configures it\n", target.Name)
		return 1
	}

	dir := filepath.Join(*meshDir, vault.DirName)
	record, err := vault.Read(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "meshy bundle: %v\n", err)
		if errors.Is(err, vault.ErrLocked) {
			return 1
		}
		return 2
	}

	archive, err := buildArchive(*mesh, *meshDir, dir, record, target, *noBinary)
	if err != nil {
		fmt.Fprintf(os.Stderr, "meshy bundle: %v\n", err)
		return 2
	}

	path := filepath.Join(*meshDir, bundleDir, mesh.Name, target.Name+".tar.gz")
	if err := bundle.Write(path, archive); err != nil {
		fmt.Fprintf(os.Stderr, "meshy bundle: %v\n", err)
		return 2
	}

	fmt.Println(path)
	return 0
}

// buildArchive collects every file that the archive holds and returns its
// bytes. It renews the certificate first when the vault holds one inside the
// window, and it writes the vault before the archive, so the archive never
// carries a certificate that the vault does not hold.
func buildArchive(mesh config.Mesh, meshDir, dir string, record vault.Record, target config.Node, noBinary bool) ([]byte, error) {
	sign := &certifier{mesh: &mesh, meshDir: meshDir, record: record, now: time.Now()}
	cert, key, _, err := sign.certificate(context.Background(), target)
	if err != nil {
		return nil, err
	}

	authority, _, found := vault.Certificate(record, mesh.CA.Active)
	if !found {
		return nil, fmt.Errorf("the vault holds no certificate for authority %s", mesh.CA.Active)
	}

	rendered, err := nebula.Render(mesh, target)
	if err != nil {
		return nil, err
	}

	if sign.changed {
		if err := vault.Write(meshDir, dir, record); err != nil {
			return nil, err
		}
	}

	name, err := platformOf(target)
	if err != nil {
		return nil, err
	}

	values := installerValues(mesh, target, name)
	bodies, err := installers.Render(values)
	if err != nil {
		return nil, err
	}

	held := bundle.Archive{
		Mesh:       mesh.Name,
		Node:       target.Name,
		Config:     rendered,
		Authority:  authority,
		Cert:       cert,
		Key:        key,
		Blocklist:  revoke.Text(record, nil),
		BinaryName: values.Binary,
		Installers: bodies,
		Readme:     bundle.Readme(mesh.Name, target.Name),
	}
	if !noBinary {
		body, err := pinnedBinary(mesh, meshDir, name)
		if err != nil {
			return nil, err
		}
		held.Binary = body
	}

	return bundle.Build(held)
}

// pinnedBinary returns the bytes of the release that the mesh pins, from the
// cache that meshy install fills, or through the install path when the cache is
// empty.
func pinnedBinary(mesh config.Mesh, meshDir string, p install.Platform) ([]byte, error) {
	if !release.Supported(p.OS, p.Arch) {
		return nil, fmt.Errorf("the pinned release has no binary for %s, use --no-binary", p)
	}

	dir := filepath.Join(meshDir, vault.DirName, "bin", mesh.Nebula.Version, p.OS+"-"+p.Arch)
	path, ok := release.Cached(dir, p.OS, p.Arch)
	if !ok {
		var err error
		path, err = platformCache(mesh, meshDir, "")(context.Background(), p)
		if err != nil {
			return nil, err
		}
	}
	return os.ReadFile(path)
}

// platformOf returns the platform that a user node runs, in the words that the
// release uses.
func platformOf(target config.Node) (install.Platform, error) {
	osName, arch, found := strings.Cut(target.Platform, "/")
	if !found || osName == "" || arch == "" {
		return install.Platform{}, fmt.Errorf("node %s has platform %q, use a form such as linux/amd64", target.Name, target.Platform)
	}
	return install.Platform{OS: osName, Arch: arch}, nil
}

// installerValues are the names and paths that the three installers write.
func installerValues(mesh config.Mesh, target config.Node, p install.Platform) installers.Values {
	unit := node.Unit(mesh.Name) + ".service"
	return installers.Values{
		Mesh:        mesh.Name,
		Node:        target.Name,
		Interface:   mesh.Interface,
		Dir:         nebula.ConfigDir(mesh.Name),
		ConfigPath:  nebula.ConfigPath(mesh.Name, target.Name),
		Unit:        unit,
		UnitFile:    "/etc/systemd/system/" + unit,
		Plist:       "/Library/LaunchDaemons/com.meshy." + mesh.Name + ".plist",
		BinaryPath:  bootstrap.Binary,
		Binary:      binaryName(p.OS),
		Version:     mesh.Nebula.Version,
		Asset:       assetName(p),
		Checksum:    release.ChecksumFile,
		BaseURL:     release.BaseURL,
		Destination: windowsData + `\meshy-` + mesh.Name,
	}
}

// binaryName is the file that a platform runs.
func binaryName(osName string) string {
	if osName == "windows" {
		return release.Binary + ".exe"
	}
	return release.Binary
}

// assetName is the archive that a platform downloads. The project publishes a
// zip for Windows and a tar.gz for the other platforms.
func assetName(p install.Platform) string {
	if p.OS == "windows" {
		return "nebula-" + p.OS + "-" + p.Arch + ".zip"
	}
	return release.Asset(p.OS, p.Arch)
}
