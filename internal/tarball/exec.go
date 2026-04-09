package tarball

import (
	"io"
	"os/exec"
)

// realCommand wraps exec.Cmd to implement the command interface.
type realCommand struct {
	cmd *exec.Cmd
}

func (c *realCommand) StdoutPipe() (io.ReadCloser, error) { return c.cmd.StdoutPipe() }
func (c *realCommand) Start() error                        { return c.cmd.Start() }
func (c *realCommand) Wait() error                         { return c.cmd.Wait() }

// newRealCommand creates a real exec.Cmd wrapped in the command interface.
func newRealCommand(name string, args ...string) command {
	return &realCommand{cmd: exec.Command(name, args...)}
}
