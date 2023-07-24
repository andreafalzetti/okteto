//go:build !windows
// +build !windows

package up

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"syscall"
)

// GetCommandToExec for non-windows GOOS
func (he *hybridExecutor) GetCommandToExec(ctx context.Context, cmd []string) (*exec.Cmd, error) {
	var c *exec.Cmd
	if runtime.GOOS != "windows" {
		c = exec.Command(cmd[0], cmd[1:]...)
	} else {
		binary, err := expandExecutableInCurrentDirectory(cmd[0], he.workdir)
		if err != nil {
			return nil, err
		}
		c = exec.Command(binary, cmd[1:]...)
	}

	c.Env = he.envs

	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr

	c.Dir = he.workdir

	// https://docs.studygolang.com/pkg/syscall/#SysProcAttr
	c.SysProcAttr = &syscall.SysProcAttr{
		Setpgid:    true,
		Foreground: true,
	}

	return c, nil
}
