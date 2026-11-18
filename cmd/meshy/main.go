package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/zinzan-vdm/cli-meshy/internal/config"
	"github.com/zinzan-vdm/cli-meshy/internal/scaffold"
)

// version is set at build time.
var version = "dev"

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		usage(os.Stderr)
		return 2
	}

	switch args[0] {
	case "init":
		return cmdInit(args[1:])
	case "validate":
		return cmdValidate(args[1:])
	case "plan":
		return cmdPlan(args[1:])
	case "apply":
		return cmdApply(args[1:])
	case "ca":
		return cmdCA(args[1:])
	case "issue":
		return cmdIssue(args[1:])
	case "bootstrap":
		return cmdBootstrap(args[1:])
	case "install":
		return cmdInstall(args[1:])
	case "bundle":
		return cmdBundle(args[1:])
	case "unlock":
		return cmdUnlock(args[1:])
	case "lock":
		return cmdLock(args[1:])
	case "version", "--version":
		fmt.Printf("meshy %s\n", version)
		return 0
	case "help", "-h", "--help":
		usage(os.Stdout)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "meshy: unknown command %q\n\n", args[0])
		usage(os.Stderr)
		return 2
	}
}

func usage(w *os.File) {
	fmt.Fprintf(w, `meshy manages one overlay network per git repository.

Usage:
  meshy <command> [flags]

Commands:
  init       create a mesh repository in a directory
  validate   check a mesh file and report every problem
  plan       compare a mesh file with the nodes and report every difference
  apply      make every node match mesh.yml
  ca         create the certificate authority of the mesh
  ca rotate  rotate the trust anchor, one phase at a time
  issue      mint a certificate for a node
  bootstrap  print the script that prepares a server node
  install    put the pinned Nebula release on the server nodes
  bundle     write the one archive that a user node joins with
  unlock     open the vault and write its plaintext to .meshy
  lock       remove the plaintext of the vault
  version    print the build version

Flags:
  --mesh <dir>   directory that holds mesh.yml (default ".")

Planned:
  status
`)
}

func cmdInit(args []string) int {
	flags := flag.NewFlagSet("init", flag.ContinueOnError)
	name := flags.String("name", "", "mesh name, defaults to the directory name")
	cidr := flags.String("cidr", "", "overlay network, defaults to 10.42.0.0/24")
	lighthouse := flags.String("lighthouse", "", "name of the first node, defaults to core-1")
	owner := flags.String("owner", "", "GitHub handle that owns mesh.yml in CODEOWNERS")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	dir := flags.Arg(0)
	if dir == "" {
		dir = "."
	}

	written, err := scaffold.Init(scaffold.Options{
		Dir:        dir,
		Name:       *name,
		CIDR:       *cidr,
		Lighthouse: *lighthouse,
		Owner:      *owner,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "meshy init: %v\n", err)
		return 1
	}

	if len(written) == 0 {
		fmt.Println("nothing written, every file already exists")
		return 0
	}
	for _, path := range written {
		fmt.Printf("created %s\n", path)
	}
	fmt.Println("\nNext: edit the node block, then run meshy validate")
	return 0
}

func cmdValidate(args []string) int {
	flags := flag.NewFlagSet("validate", flag.ContinueOnError)
	meshDir := flags.String("mesh", ".", "directory that holds mesh.yml")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	mesh, err := config.Load(*meshDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "meshy validate: %v\n", err)
		return 1
	}

	problems := mesh.Validate()
	if len(problems) == 0 {
		fmt.Printf("mesh %q is valid: %d nodes, %d rules\n", mesh.Name, len(mesh.Nodes), len(mesh.Rules))
		return 0
	}

	fmt.Fprintf(os.Stderr, "mesh %q has %d problems:\n", mesh.Name, len(problems))
	for _, problem := range problems {
		fmt.Fprintf(os.Stderr, "  - %s\n", problem)
	}
	return 1
}
