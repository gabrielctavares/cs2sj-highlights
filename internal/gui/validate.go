package gui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type FormValues struct {
	CS2Path   string
	InputDir  string
	OutputDir string
}

func ValidateForm(values FormValues) error {
	if strings.TrimSpace(values.CS2Path) == "" {
		return fmt.Errorf("informe o caminho do CS2")
	}
	if !strings.EqualFold(filepath.Base(values.CS2Path), "cs2.exe") || !regularFile(values.CS2Path) {
		return fmt.Errorf("o caminho do CS2 deve apontar para o arquivo cs2.exe")
	}
	if strings.TrimSpace(values.InputDir) == "" {
		return fmt.Errorf("informe a pasta das demos")
	}
	entries, err := os.ReadDir(values.InputDir)
	if err != nil {
		return fmt.Errorf("abrir pasta das demos: %w", err)
	}
	hasDemo := false
	for _, entry := range entries {
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), ".dem") {
			hasDemo = true
			break
		}
	}
	if !hasDemo {
		return fmt.Errorf("a pasta das demos não contém arquivos .dem")
	}
	if strings.TrimSpace(values.OutputDir) == "" {
		return fmt.Errorf("informe a pasta de saída")
	}
	if info, statErr := os.Stat(values.OutputDir); statErr == nil {
		if !info.IsDir() {
			return fmt.Errorf("a pasta de saída aponta para um arquivo")
		}
	} else if !os.IsNotExist(statErr) {
		return fmt.Errorf("consultar pasta de saída: %w", statErr)
	} else if mkdirErr := os.MkdirAll(values.OutputDir, 0o755); mkdirErr != nil {
		return fmt.Errorf("criar pasta de saída: %w", mkdirErr)
	}
	return nil
}
