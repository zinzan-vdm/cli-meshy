package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/zinzan-vdm/cli-meshy/internal/ca"
	"github.com/zinzan-vdm/cli-meshy/internal/config"
	"github.com/zinzan-vdm/cli-meshy/internal/vault"
)

func cmdCA(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "meshy ca: name a subcommand: init, rotate")
		return 2
	}

	switch args[0] {
	case "init":
		return cmdCAInit(args[1:])
	case "rotate":
		return cmdCARotate(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "meshy ca: unknown subcommand %q\n", args[0])
		return 2
	}
}

func cmdCAInit(args []string) int {
	flags := flag.NewFlagSet("ca init", flag.ContinueOnError)
	force := flags.Bool("force", false, "replace a vault that already holds an authority")
	meshDir := flags.String("mesh", ".", "directory that holds mesh.yml")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	mesh, err := config.Load(*meshDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "meshy ca init: %v\n", err)
		return 1
	}
	if problems := mesh.Validate(); len(problems) > 0 {
		fmt.Fprintf(os.Stderr, "mesh %q has %d problems:\n", mesh.Name, len(problems))
		for _, problem := range problems {
			fmt.Fprintf(os.Stderr, "  - %s\n", problem)
		}
		return 1
	}

	sealed := filepath.Join(*meshDir, vault.FileName)
	if _, err := os.Stat(sealed); err == nil && !*force {
		fmt.Fprintf(os.Stderr, "meshy ca init: %s already holds a vault, use --force to replace it\n", sealed)
		return 1
	}

	binary, err := ca.Find(*meshDir, mesh.Nebula.Version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "meshy ca init: %v\n", err)
		return 2
	}

	authority, err := ca.Init(context.Background(), ca.Options{
		Binary:   binary,
		Name:     ca.Name,
		Networks: mesh.CIDR,
		Lifetime: mesh.CA.Duration.Duration,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "meshy ca init: %v\n", err)
		return 1
	}

	record := vault.Record{
		authority.Name + ".key":    authority.Key,
		authority.Name + ".crt":    authority.Cert,
		authority.Name + ".issued": []byte(authority.Issued.Format("2006-01-02T15:04:05Z")),
	}
	plaintext, err := vault.Encode(record)
	if err != nil {
		fmt.Fprintf(os.Stderr, "meshy ca init: %v\n", err)
		return 1
	}

	ciphertext, shares, err := vault.Seal(plaintext)
	if err != nil {
		fmt.Fprintf(os.Stderr, "meshy ca init: %v\n", err)
		return 1
	}
	if err := os.WriteFile(sealed, ciphertext, 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "meshy ca init: %v\n", err)
		return 1
	}

	changed, err := config.SetAuthority(*meshDir, authority.Name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "meshy ca init: %v\n", err)
		return 1
	}

	fmt.Printf("authority %s is sealed in %s\n", authority.Name, vault.FileName)
	fmt.Printf("lifetime %s, networks %s\n", config.FormatDuration(mesh.CA.Duration.Duration), mesh.CIDR)
	if changed {
		fmt.Printf("mesh.yml now trusts %s and makes it active\n", authority.Name)
	}
	fmt.Printf("\nThese five shares are the only copy. Any three open the vault.\n\n")
	for _, share := range shares {
		fmt.Printf("  %s\n", share)
	}
	fmt.Printf("\nStore them apart. Anyone with three of them can read every key in this mesh.\n")

	return 0
}
