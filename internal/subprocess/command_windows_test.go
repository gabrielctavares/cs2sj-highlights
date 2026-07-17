package subprocess

import (
	"context"
	"testing"
)

func TestCommandContextHidesChildConsoleWindow(t *testing.T) {
	command := CommandContext(context.Background(), "cmd.exe", "/c", "exit", "0")
	if command.SysProcAttr == nil || !command.SysProcAttr.HideWindow {
		t.Fatalf("child process window is not hidden: %#v", command.SysProcAttr)
	}
	if command.SysProcAttr.CreationFlags&createNoWindow == 0 {
		t.Fatalf("CREATE_NO_WINDOW is missing: %#v", command.SysProcAttr)
	}
}
