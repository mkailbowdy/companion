package voice

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

type CommandRunner interface {
	Run(context.Context, string, ...string) ([]byte, error)
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := commandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, &ProcessError{Command: filepath.Base(name), Err: err}
	}
	return output, nil
}

type ProcessError struct {
	Command string
	Err     error
}

func (e *ProcessError) Error() string {
	return fmt.Sprintf("%s failed: %v", e.Command, e.Err)
}

func (e *ProcessError) Unwrap() error {
	return e.Err
}

func commandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.WaitDelay = 2 * time.Second
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	return cmd
}
