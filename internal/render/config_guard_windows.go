package render

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

type ConfigGuard interface {
	Protect(context.Context, string) (func() error, error)
}

type SteamConfigGuard struct {
	SteamRoot func() (string, error)
}

type configSnapshot struct {
	path   string
	backup string
	mode   fs.FileMode
}

func (guard SteamConfigGuard) Protect(ctx context.Context, _ string) (func() error, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	resolve := guard.SteamRoot
	if resolve == nil {
		resolve = steamRootFromRegistry
	}
	root, err := resolve()
	if err != nil {
		return nil, fmt.Errorf("localizar Steam para proteger configurações do CS2: %w", err)
	}
	directories, err := filepath.Glob(filepath.Join(root, "userdata", "*", "730", "local", "cfg"))
	if err != nil {
		return nil, fmt.Errorf("procurar configurações do CS2: %w", err)
	}
	if len(directories) == 0 {
		return nil, fmt.Errorf("nenhuma pasta Steam userdata/*/730/local/cfg encontrada em %q", root)
	}
	backupRoot, err := os.MkdirTemp("", "cs2sj-config-backup-")
	if err != nil {
		return nil, fmt.Errorf("criar cópia de segurança das configurações do CS2: %w", err)
	}
	if err := os.Chmod(backupRoot, 0o700); err != nil {
		_ = os.RemoveAll(backupRoot)
		return nil, fmt.Errorf("proteger pasta da cópia de segurança: %w", err)
	}

	initial := make(map[string]struct{})
	var snapshots []configSnapshot
	for _, directory := range directories {
		matches, globErr := filepath.Glob(filepath.Join(directory, "cs2_user_convars*.vcfg"))
		if globErr != nil {
			_ = os.RemoveAll(backupRoot)
			return nil, fmt.Errorf("procurar convars em %q: %w", directory, globErr)
		}
		for _, path := range matches {
			info, statErr := os.Stat(path)
			if statErr != nil || !info.Mode().IsRegular() {
				_ = os.RemoveAll(backupRoot)
				if statErr == nil {
					statErr = fmt.Errorf("não é arquivo regular")
				}
				return nil, fmt.Errorf("proteger configuração %q: %w", path, statErr)
			}
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				_ = os.RemoveAll(backupRoot)
				return nil, fmt.Errorf("ler configuração %q: %w", path, readErr)
			}
			backup := filepath.Join(backupRoot, fmt.Sprintf("%03d.vcfg", len(snapshots)))
			if writeErr := os.WriteFile(backup, data, 0o600); writeErr != nil {
				_ = os.RemoveAll(backupRoot)
				return nil, fmt.Errorf("copiar configuração %q: %w", path, writeErr)
			}
			clean := filepath.Clean(path)
			initial[clean] = struct{}{}
			snapshots = append(snapshots, configSnapshot{path: clean, backup: backup, mode: info.Mode().Perm()})
		}
	}
	if len(snapshots) == 0 {
		_ = os.RemoveAll(backupRoot)
		return nil, fmt.Errorf("nenhum arquivo cs2_user_convars*.vcfg encontrado abaixo de %q", root)
	}

	var once sync.Once
	var restoreErr error
	restore := func() error {
		once.Do(func() {
			var failures []error
			for _, snapshot := range snapshots {
				data, readErr := os.ReadFile(snapshot.backup)
				if readErr != nil {
					failures = append(failures, fmt.Errorf("ler backup %q: %w", snapshot.backup, readErr))
					continue
				}
				if writeErr := replaceFile(snapshot.path, data, snapshot.mode); writeErr != nil {
					failures = append(failures, fmt.Errorf("restaurar %q: %w", snapshot.path, writeErr))
				}
			}
			for _, directory := range directories {
				matches, _ := filepath.Glob(filepath.Join(directory, "cs2_user_convars*.vcfg"))
				for _, path := range matches {
					if _, existed := initial[filepath.Clean(path)]; existed {
						continue
					}
					if removeErr := os.Remove(path); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
						failures = append(failures, fmt.Errorf("remover configuração criada %q: %w", path, removeErr))
					}
				}
			}
			if len(failures) > 0 {
				restoreErr = fmt.Errorf("restauração incompleta; backup preservado em %q: %w", backupRoot, errors.Join(failures...))
				return
			}
			if removeErr := os.RemoveAll(backupRoot); removeErr != nil {
				restoreErr = fmt.Errorf("remover backup restaurado %q: %w", backupRoot, removeErr)
			}
		})
		return restoreErr
	}
	return restore, nil
}

func steamRootFromRegistry() (string, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Valve\Steam`, registry.QUERY_VALUE)
	if err != nil {
		return "", err
	}
	defer key.Close()
	value, _, err := key.GetStringValue("SteamPath")
	if err != nil {
		return "", err
	}
	if value == "" {
		return "", fmt.Errorf("SteamPath está vazio")
	}
	return filepath.Clean(filepath.FromSlash(value)), nil
}

func replaceFile(path string, data []byte, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temporary := path + ".cs2sj-restore"
	if err := os.WriteFile(temporary, data, mode); err != nil {
		return err
	}
	from, err := windows.UTF16PtrFromString(temporary)
	if err != nil {
		_ = os.Remove(temporary)
		return err
	}
	to, err := windows.UTF16PtrFromString(path)
	if err != nil {
		_ = os.Remove(temporary)
		return err
	}
	if err := windows.MoveFileEx(from, to, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	return nil
}
