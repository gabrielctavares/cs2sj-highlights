package hudtheme

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var colorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func Validate(theme Theme, themeDir string) error {
	if theme.Version != 1 {
		return fmt.Errorf("unsupported theme version %d", theme.Version)
	}
	if strings.TrimSpace(theme.Name) == "" {
		return fmt.Errorf("theme name is empty")
	}
	if len(theme.Elements) == 0 {
		return fmt.Errorf("theme has no elements")
	}
	ids := make(map[string]struct{}, len(theme.Elements))
	for _, element := range theme.Elements {
		if err := validateElement(element, themeDir); err != nil {
			return fmt.Errorf("element %q: %w", element.ID, err)
		}
		if _, exists := ids[element.ID]; exists {
			return fmt.Errorf("element %q has a duplicate id", element.ID)
		}
		ids[element.ID] = struct{}{}
	}
	return nil
}

func validateElement(element Element, themeDir string) error {
	if strings.TrimSpace(element.ID) == "" {
		return fmt.Errorf("id is empty")
	}
	if !validType(element.Type) || !validAnchor(element.Anchor) || !validBinding(element.Binding) {
		return fmt.Errorf("type, anchor or binding is invalid")
	}
	if element.X < 0 || element.Y < 0 || element.Width <= 0 || element.Height <= 0 || element.X+element.Width > 100 || element.Y+element.Height > 100 {
		return fmt.Errorf("bounds must stay inside 0-100")
	}
	if element.Opacity < 0 || element.Opacity > 1 {
		return fmt.Errorf("opacity must be between 0 and 1")
	}
	if element.Color != "" && !colorPattern.MatchString(element.Color) {
		return fmt.Errorf("color is invalid")
	}
	for _, value := range []string{element.Asset, element.Font} {
		if value == "" {
			continue
		}
		path, err := ResolveAsset(themeDir, value)
		if err != nil {
			return fmt.Errorf("asset %q: %w", value, err)
		}
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("asset %q is not a regular file", value)
		}
	}
	if element.Type == Text && element.FontSize < 0 {
		return fmt.Errorf("font size is invalid")
	}
	return nil
}

func ResolveAsset(themeDir, value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", nil
	}
	root, err := filepath.Abs(themeDir)
	if err != nil {
		return "", err
	}
	candidate, err := filepath.Abs(filepath.Join(root, value))
	if err != nil {
		return "", err
	}
	prefix := root + string(filepath.Separator)
	if candidate != root && !strings.HasPrefix(candidate, prefix) {
		return "", fmt.Errorf("asset escapes theme directory")
	}
	return candidate, nil
}

func validType(value ElementType) bool { return value == Box || value == Text || value == Image }

func validAnchor(value Anchor) bool {
	switch value {
	case TopLeft, TopCenter, TopRight, Center, BottomLeft, BottomCenter, BottomRight:
		return true
	default:
		return false
	}
}

func validBinding(value Binding) bool {
	switch value {
	case "", Event, TeamAName, TeamBName, TeamALogo, TeamBLogo, ScoreA, ScoreB, Map, Round, Player, Highlight:
		return true
	default:
		return false
	}
}
