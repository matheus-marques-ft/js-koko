package localcommand

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/creack/pty"
)

type LocalCommand struct {
	command string
	argv    []string
	workDir string

	env           []string
	cmdCredential *syscall.Credential

	cmd       *exec.Cmd
	ptyFd     *os.File
	ptyClosed chan struct{}
	waitErr   error

	ptyWin *pty.Winsize
}

func New(command string, argv []string, options ...Option) (*LocalCommand, error) {
	ptyClosed := make(chan struct{})
	lcmd := &LocalCommand{
		command:   command,
		argv:      argv,
		ptyClosed: ptyClosed,
	}

	for _, option := range options {
		option(lcmd)
	}
	cmd := exec.Command(command, argv...)
	if lcmd.env != nil {
		cmd.Env = lcmd.env
	}
	if lcmd.cmdCredential != nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
		cmd.SysProcAttr.Credential = lcmd.cmdCredential
	}
	if lcmd.workDir != "" {
		cmd.Dir = lcmd.workDir
	}
	ptyFd, err := pty.StartWithSize(cmd, lcmd.ptyWin)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	lcmd.cmd = cmd
	lcmd.ptyFd = ptyFd
	// When the process is closed by the user,
	// close pty so that Read() on the pty breaks with an EOF.
	go func() {
		defer func() {
			lcmd.ptyFd.Close()
			close(lcmd.ptyClosed)
		}()
		lcmd.waitErr = lcmd.cmd.Wait()
	}()

	return lcmd, nil
}

// Wait blocks until the underlying process has exited.
func (lcmd *LocalCommand) Wait() error {
	<-lcmd.ptyClosed
	return lcmd.waitErr
}

// ExitCode returns the process exit code once it has exited, or -1 if it
// hasn't exited yet or the exit code couldn't be determined (e.g. killed by
// a signal).
func (lcmd *LocalCommand) ExitCode() int {
	select {
	case <-lcmd.ptyClosed:
	default:
		return -1
	}
	if lcmd.waitErr == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(lcmd.waitErr, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}

func (lcmd *LocalCommand) Read(p []byte) (n int, err error) {
	return lcmd.ptyFd.Read(p)
}

func (lcmd *LocalCommand) Write(p []byte) (n int, err error) {
	return lcmd.ptyFd.Write(p)
}

func (lcmd *LocalCommand) Close() error {
	select {
	case <-lcmd.ptyClosed:
		return nil
	default:
		if lcmd.cmd != nil && lcmd.cmd.Process != nil {
			return lcmd.cmd.Process.Signal(syscall.SIGKILL)
		}
	}
	return nil
}

func (lcmd *LocalCommand) SetWinSize(width int, height int) error {
	win := pty.Winsize{
		Rows: uint16(height),
		Cols: uint16(width),
	}
	return pty.Setsize(lcmd.ptyFd, &win)
}
