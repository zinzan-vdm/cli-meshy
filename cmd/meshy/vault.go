package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/zinzan-vdm/cli-meshy/internal/vault"
)

func cmdUnlock(args []string) int {
	flags := flag.NewFlagSet("unlock", flag.ContinueOnError)
	fromEnv := flags.Bool("from-env", false, "read shares from MESHY_SHARE_1 to MESHY_SHARE_5")
	force := flags.Bool("force", false, "replace the plaintext of a vault that is already open")
	meshDir := flags.String("mesh", ".", "directory that holds mesh.yml")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	shares, err := collectShares(*fromEnv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "meshy unlock: %v\n", err)
		return 1
	}

	dir, err := vault.Unlock(*meshDir, shares, *force)
	if err != nil {
		fmt.Fprintf(os.Stderr, "meshy unlock: %v\n", err)
		return 1
	}

	fmt.Println(dir)
	return 0
}

func cmdLock(args []string) int {
	flags := flag.NewFlagSet("lock", flag.ContinueOnError)
	meshDir := flags.String("mesh", ".", "directory that holds mesh.yml")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	if err := vault.Lock(*meshDir); err != nil {
		fmt.Fprintf(os.Stderr, "meshy lock: %v\n", err)
		return 1
	}

	fmt.Println("locked")
	return 0
}

func collectShares(fromEnv bool) ([]string, error) {
	if fromEnv {
		return envShares(), nil
	}
	return promptShares()
}

func envShares() []string {
	shares := make([]string, 0, vault.ShareCount)
	for i := 1; i <= vault.ShareCount; i++ {
		share := strings.TrimSpace(os.Getenv(fmt.Sprintf("MESHY_SHARE_%d", i)))
		if share != "" {
			shares = append(shares, share)
		}
	}
	return shares
}

func promptShares() ([]string, error) {
	shares := make([]string, 0, vault.Threshold)
	reader := bufio.NewReader(os.Stdin)

	for len(shares) < vault.Threshold {
		fmt.Fprintf(os.Stderr, "share %d of %d: ", len(shares)+1, vault.Threshold)
		share, err := readSecret(reader)
		if err != nil {
			return nil, err
		}
		if share == "" {
			continue
		}
		shares = append(shares, share)
	}

	return shares, nil
}

func readSecret(reader *bufio.Reader) (string, error) {
	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		secret, err := term.ReadPassword(fd)
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(secret)), nil
	}

	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return "", err
	}
	return strings.TrimSpace(line), nil
}
