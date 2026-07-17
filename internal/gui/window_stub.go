//go:build !windows

package gui

import (
	"context"
	"fmt"
)

func Run(context.Context, string, string) error {
	return fmt.Errorf("Windows 10 ou mais recente é obrigatório")
}
