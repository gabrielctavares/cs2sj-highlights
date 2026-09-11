package gui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/gabrielctavares/cs2sj-highlights/internal/hudtheme"
)

type HUDThemeFile struct {
	Path  string
	Theme hudtheme.Theme
}

func ListHUDThemes(root string) ([]HUDThemeFile, error) {
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list HUD themes: %w", err)
	}
	themes := make([]HUDThemeFile, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(root, entry.Name(), "hud.json")
		theme, loadErr := hudtheme.Load(path)
		if os.IsNotExist(loadErr) {
			continue
		}
		if loadErr != nil {
			return nil, fmt.Errorf("load HUD theme %q: %w", path, loadErr)
		}
		themes = append(themes, HUDThemeFile{Path: path, Theme: theme})
	}
	sort.Slice(themes, func(i, j int) bool {
		return strings.ToLower(themes[i].Theme.Name) < strings.ToLower(themes[j].Theme.Name)
	})
	return themes, nil
}

func CreateHUDTheme(root, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("theme name is empty")
	}
	directory := filepath.Join(root, themeDirectoryName(name))
	path := filepath.Join(directory, "hud.json")
	if _, err := os.Stat(path); err == nil {
		return "", fmt.Errorf("HUD theme already exists: %q", name)
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return "", fmt.Errorf("create HUD theme directory: %w", err)
	}
	theme := hudtheme.DefaultTheme()
	theme.Name = name
	ApplyGuidedLayout(&theme)
	if err := hudtheme.Save(path, theme); err != nil {
		return "", err
	}
	return path, nil
}

func DefaultHUDThemePath(root string) (string, error) {
	themes, err := ListHUDThemes(root)
	if err != nil {
		return "", err
	}
	if len(themes) > 0 {
		return themes[0].Path, nil
	}
	return CreateHUDTheme(root, "Meu HUD")
}

func themeDirectoryName(name string) string {
	var result strings.Builder
	previousDash := false
	for _, value := range strings.ToLower(strings.TrimSpace(name)) {
		if unicode.IsLetter(value) || unicode.IsDigit(value) {
			result.WriteRune(value)
			previousDash = false
		} else if !previousDash {
			result.WriteByte('-')
			previousDash = true
		}
	}
	slug := strings.Trim(result.String(), "-")
	if slug == "" {
		return "tema"
	}
	return slug
}
