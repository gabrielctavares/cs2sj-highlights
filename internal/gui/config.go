package gui

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/gabrielctavares/cs2sj-highlights/internal/model"
)

const ConfigSchemaVersion = 2

type Config struct {
	SchemaVersion   int           `json:"schema_version"`
	CS2Path         string        `json:"cs2_path"`
	InputDir        string        `json:"input_dir"`
	OutputDir       string        `json:"output_dir"`
	HUDMode         model.HUDMode `json:"hud_mode,omitempty"`
	HUDThemePath    string        `json:"hud_theme_path,omitempty"`
	FavoriteSteamID uint64        `json:"favorite_steam_id,omitempty"`
}

func ConfigPath(localAppData string) string {
	return filepath.Join(localAppData, "CS2SJ-Demo", "config.json")
}

func LoadConfig(path string) (Config, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return Config{SchemaVersion: ConfigSchemaVersion}, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("abrir configuração %q: %w", path, err)
	}
	defer file.Close()

	var config Config
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return Config{}, fmt.Errorf("ler configuração %q: %w", path, err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			err = fmt.Errorf("dados adicionais")
		}
		return Config{}, fmt.Errorf("ler configuração %q: %w", path, err)
	}
	if config.SchemaVersion == 1 {
		config.SchemaVersion = ConfigSchemaVersion
	}
	if config.SchemaVersion != ConfigSchemaVersion {
		return Config{}, fmt.Errorf("versão de configuração não suportada: %d", config.SchemaVersion)
	}
	if config.HUDMode == "" {
		config.HUDMode = model.HUDNone
	}
	if !config.HUDMode.Valid() {
		return Config{}, fmt.Errorf("modo de HUD não suportado: %q", config.HUDMode)
	}
	return config, nil
}

func SaveConfig(path string, config Config) (err error) {
	if path == "" {
		return fmt.Errorf("caminho da configuração não informado")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("criar pasta da configuração: %w", err)
	}

	config.SchemaVersion = ConfigSchemaVersion
	temporary := path + ".tmp"
	file, err := os.OpenFile(temporary, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("criar configuração temporária: %w", err)
	}
	closed := false
	defer func() {
		if !closed {
			if closeErr := file.Close(); err == nil && closeErr != nil {
				err = fmt.Errorf("fechar configuração temporária: %w", closeErr)
			}
		}
		if err != nil {
			_ = os.Remove(temporary)
		}
	}()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err = encoder.Encode(config); err != nil {
		return fmt.Errorf("gravar configuração temporária: %w", err)
	}
	if err = file.Sync(); err != nil {
		return fmt.Errorf("sincronizar configuração temporária: %w", err)
	}
	if err = file.Close(); err != nil {
		return fmt.Errorf("fechar configuração temporária: %w", err)
	}
	closed = true
	if err = os.Rename(temporary, path); err != nil {
		return fmt.Errorf("substituir configuração: %w", err)
	}
	return nil
}
