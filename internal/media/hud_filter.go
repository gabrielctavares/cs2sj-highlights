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

// BuildHUDFilter compiles a 16:9 theme into an FFmpeg filter. Theme image
// inputs are placed directly after the master video input by ClipBuilder.
func BuildHUDFilter(theme hudtheme.Theme, themeDir string, values hudtheme.Values, fontFallback string) (HUDFilterPlan, error) {
	if err := hudtheme.Validate(theme, themeDir); err != nil {
		return HUDFilterPlan{}, err
	}
	elements := append([]hudtheme.Element(nil), theme.Elements...)
	sort.SliceStable(elements, func(i, j int) bool { return elements[i].ZIndex < elements[j].ZIndex })
	filter := "[0:v]scale=1920:1080:force_original_aspect_ratio=decrease,pad=1920:1080:(ow-iw)/2:(oh-ih)/2[base]"
	current := "base"
	imageIndex := 1
	inputs := make([]string, 0)
	for index, element := range elements {
		if !element.Visible {
			continue
		}
		next := fmt.Sprintf("layer%d", index)
		x, y, width, height := hudBounds(element)
		switch element.Type {
		case hudtheme.Box:
			color := strings.TrimPrefix(element.Color, "#")
			if color == "" {
				color = "000000"
			}
			opacity := element.Opacity
			if opacity == 0 {
				opacity = 1
			}
			filter += fmt.Sprintf(";[%s]drawbox=x=%d:y=%d:w=%d:h=%d:color=0x%s@%.2f:t=fill[%s]", current, x, y, width, height, color, opacity, next)
		case hudtheme.Text:
			text := element.Text
			if element.Binding != "" {
				text = values[element.Binding]
			}
			if strings.TrimSpace(text) == "" {
				continue
			}
			font := fontFallback
			if element.Font != "" {
				font, _ = hudtheme.ResolveAsset(themeDir, element.Font)
			}
			color := strings.TrimPrefix(element.Color, "#")
			if color == "" {
				color = "FFFFFF"
			}
			fontSize := element.FontSize
			if fontSize == 0 {
				fontSize = 24
			}
			filter += fmt.Sprintf(";[%s]drawtext=fontfile='%s':text='%s':fontcolor=0x%s:fontsize=%d:x=%d+(%d-text_w)/2:y=%d+(%d-text_h)/2[%s]", current, escapeFilterPath(font), escapeDrawText(text), color, fontSize, x, width, y, height, next)
		case hudtheme.Image:
			if element.Asset == "" {
				continue
			}
			asset, _ := hudtheme.ResolveAsset(themeDir, element.Asset)
			inputs = append(inputs, asset)
			filter += fmt.Sprintf(";[%d:v]scale=%d:%d[theme%d];[%s][theme%d]overlay=x=%d:y=%d:format=auto[%s]", imageIndex, width, height, imageIndex, current, imageIndex, x, y, next)
			imageIndex++
		default:
			continue
		}
		current = next
	}
	return HUDFilterPlan{Filter: filter + fmt.Sprintf(";[%s]null[v]", current), Inputs: inputs}, nil
}

func hudBounds(element hudtheme.Element) (x, y, width, height int) {
	width = max(1, int(element.Width*19.2))
	height = max(1, int(element.Height*10.8))
	x = int(element.X * 19.2)
	y = int(element.Y * 10.8)
	return x, y, width, height
}

func escapeDrawText(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, "'", `\'`)
	value = strings.ReplaceAll(value, ":", `\:`)
	return strings.ReplaceAll(value, "%", `\%`)
}

func themeAssetPath(themeDir, value string) string {
	if value == "" {
		return ""
	}
	return filepath.Join(themeDir, value)
}
