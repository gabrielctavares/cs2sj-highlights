package media

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gabrielctavares/cs2sj-highlights/internal/hudtheme"
)

type HUDFilterPlan struct {
	Filter string
	Inputs []string
}

func BuildHUDFilter(theme hudtheme.Theme, themeDir string, values hudtheme.Values, fontFallback string) (HUDFilterPlan, error) {
	if err := hudtheme.Validate(theme, themeDir); err != nil {
		return HUDFilterPlan{}, err
	}
	elements := append([]hudtheme.Element(nil), theme.Elements...)
	sort.SliceStable(elements, func(i, j int) bool { return elements[i].ZIndex < elements[j].ZIndex })
	parts := []string{"[0:v]scale=1920:1080:force_original_aspect_ratio=decrease,pad=1920:1080:(ow-iw)/2:(oh-ih)/2"}
	for _, element := range elements {
		if !element.Visible || element.Type != hudtheme.Box {
			continue
		}
		x, y, width, height := hudBounds(element)
		color := strings.TrimPrefix(element.Color, "#")
		if color == "" {
			color = "000000"
		}
		opacity := element.Opacity
		if opacity == 0 {
			opacity = 1
		}
		parts = append(parts, fmt.Sprintf("drawbox=x=%d:y=%d:w=%d:h=%d:color=0x%s@%.2f:t=fill", x, y, width, height, color, opacity))
	}
	return HUDFilterPlan{Filter: strings.Join(parts, ",") + "[hud]"}, nil
}

func hudBounds(element hudtheme.Element) (x, y, width, height int) {
	width = max(1, int(element.Width*19.2))
	height = max(1, int(element.Height*10.8))
	x = int(element.X * 19.2)
	y = int(element.Y * 10.8)
	return x, y, width, height
}

func themeAssetPath(themeDir, value string) string {
	if value == "" {
		return ""
	}
	return filepath.Join(themeDir, value)
}
