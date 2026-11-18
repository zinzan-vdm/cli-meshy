package sshx

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"strings"

	"github.com/zinzan-vdm/cli-meshy/internal/node"
)

// missingExit is the status that readCommand reports for a path that is not
// there. A separate code keeps a missing file apart from a failed read.
const missingExit = 3

// Write puts data at path with mode. The bytes travel on the session's stdin,
// so the node needs no scp or sftp binary.
func (c *Client) Write(ctx context.Context, path string, data []byte, mode fs.FileMode) error {
	res, err := c.run(ctx, writeCommand(path, mode), bytes.NewReader(data))
	if err != nil {
		return err
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("%s: write %s: exit %d: %s", c.host, path, res.ExitCode, strings.TrimSpace(res.Stderr))
	}
	return nil
}

// Read returns the bytes at path. A path that the node does not hold reports
// node.ErrNoFile.
func (c *Client) Read(ctx context.Context, path string) ([]byte, error) {
	res, err := c.Run(ctx, readCommand(path))
	if err != nil {
		return nil, err
	}
	switch res.ExitCode {
	case 0:
		return []byte(res.Stdout), nil
	case missingExit:
		return nil, node.ErrNoFile
	}
	return nil, fmt.Errorf("%s: read %s: exit %d: %s", c.host, path, res.ExitCode, strings.TrimSpace(res.Stderr))
}

// writeCommand keeps the mode of a new file away from the umask of the node
// user, because the config and the key must be readable by that user alone.
func writeCommand(path string, mode fs.FileMode) string {
	quoted := quote(path)
	return fmt.Sprintf("umask 077; cat > %s && chmod %04o %s", quoted, mode.Perm(), quoted)
}

func readCommand(path string) string {
	quoted := quote(path)
	return fmt.Sprintf("if [ ! -e %s ]; then exit %d; fi; cat %s", quoted, missingExit, quoted)
}

func quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
