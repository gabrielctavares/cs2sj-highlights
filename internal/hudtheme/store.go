package hudtheme

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func Load(path string) (Theme, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Theme{}, fmt.Errorf("read HUD theme %q: %w", path, err)
	}
	var theme Theme
	if err := json.Unmarshal(data, &theme); err != nil {
		return Theme{}, fmt.Errorf("decode HUD theme %q: %w", path, err)
	}
	if err := Validate(theme, filepath.Dir(path)); err != nil {
		return Theme{}, fmt.Errorf("validate HUD theme %q: %w", path, err)
	}
	return theme, nil
}

func Save(path string, theme Theme) (err error) {
	if err := Validate(theme, filepath.Dir(path)); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create HUD theme directory: %w", err)
	}
	temporary := path + ".tmp"
	defer func() {
		if err != nil {
			_ = os.Remove(temporary)
		}
	}()
	file, err := os.OpenFile(temporary, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create temporary HUD theme: %w", err)
	}
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(theme); err != nil {
		_ = file.Close()
		return fmt.Errorf("encode temporary HUD theme: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("sync temporary HUD theme: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close temporary HUD theme: %w", err)
	}
	if err := os.Rename(temporary, path); err != nil {
		return fmt.Errorf("replace HUD theme: %w", err)
	}
	return nil
}
