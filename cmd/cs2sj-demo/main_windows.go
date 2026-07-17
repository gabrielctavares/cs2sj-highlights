//go:build windows

package main

import (
	"context"
	"os"

	"github.com/gabrielctavares/cs2sj-highlights/internal/gui"

	"github.com/lxn/walk"
)

func main() {
	executable, err := os.Executable()
	if err == nil {
		err = gui.Run(context.Background(), executable, os.Getenv("LOCALAPPDATA"))
	}
	if err != nil {
		walk.MsgBox(nil, "CS2SJ Demo", err.Error(), walk.MsgBoxOK|walk.MsgBoxIconError)
	}
}
