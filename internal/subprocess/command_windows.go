package subprocess

import (
	"context"
	"os/exec"
	"syscall"
)

const createNoWindow = 0x08000000

// CommandContext creates a child process without flashing a console window.
func CommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	command := exec.CommandContext(ctx, name, args...)
	command.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
	return command
}
