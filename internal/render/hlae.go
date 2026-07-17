package render

import (
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"unicode"

	"github.com/gabrielctavares/cs2sj-highlights/internal/model"
)

func cleanHUDCommandChain(model.HUDMode) string {
	return "cl_drawhud 1; cl_draw_only_deathnotices 1; cl_drawhud_force_deathnotices 1; cl_drawhud_force_radar -1; cl_drawhud_force_teamid_overhead -1; cl_trueview_show_status 0"
}

func BuildCFG(demoPath string, pass RenderPass, tickRate float64, hudMode model.HUDMode) (string, error) {
	if len(pass.Clips) == 0 {
		return "", fmt.Errorf("render pass is empty")
	}
	if tickRate <= 0 {
		return "", fmt.Errorf("tick rate must be positive")
	}
	if err := validateConsolePath(demoPath); err != nil {
		return "", fmt.Errorf("invalid demo path: %w", err)
	}
	clips := slices.Clone(pass.Clips)
	sort.SliceStable(clips, func(i, j int) bool {
		if clips[i].StartTick != clips[j].StartTick {
			return clips[i].StartTick < clips[j].StartTick
		}
		return clips[i].ID < clips[j].ID
	})

	var builder strings.Builder
	builder.WriteString("mirv_cmd clear\n")
	builder.WriteString("mirv_streams settings edit afxDefault settings afxFfmpeg\n")
	builder.WriteString("mirv_streams record screen enabled 1\n")
	builder.WriteString("mirv_streams record fps 60\n")
	builder.WriteString("mirv_streams record startMovieWav 1\n")
	builder.WriteString("voice_modenable 0\n")
	builder.WriteString("demo_ui_mode 0\n")
	builder.WriteString("cl_showdemooverlay 0\n")
	builder.WriteString("cl_showfps 0\n")
	builder.WriteString("cl_showpos 0\n")
	builder.WriteString("cl_showtick 0\n")
	builder.WriteString("cl_showmem 0\n")
	builder.WriteString("cl_showframenumber 0\n")
	builder.WriteString("cl_trueview_show_status 0\n")
	builder.WriteString("r_show_build_info 0\n")
	builder.WriteString("r_show_time_info 0\n")
	builder.WriteString("cl_hud_telemetry_frametime_show 0\n")
	builder.WriteString("cl_hud_telemetry_ping_show 0\n")
	builder.WriteString("cl_hud_telemetry_net_detailed 0\n")
	builder.WriteString("cl_hud_telemetry_net_misdelivery_show 0\n")
	builder.WriteString("cl_hud_telemetry_net_quality_graph_show 0\n")
	builder.WriteString("cl_hud_telemetry_serverrecvmargin_graph_show 0\n")
	if hudMode == model.HUDGame {
		builder.WriteString("cl_drawhud 1\n")
	} else {
		builder.WriteString("sv_cheats 1\n")
		builder.WriteString("cl_drawhud 1\n")
		builder.WriteString("cl_draw_only_deathnotices 1\n")
		builder.WriteString("cl_drawhud_force_deathnotices 1\n")
		builder.WriteString("cl_drawhud_force_radar -1\n")
		builder.WriteString("cl_drawhud_force_teamid_overhead -1\n")
	}
	builder.WriteString("spec_mode 1\n")
	initialSeek := max(1, clips[0].StartTick-int(10*tickRate))
	if hudMode == model.HUDGame {
		fmt.Fprintf(&builder, "mirv_cmd addAtTick 1 \"demo_gototick %d; demo_timescale 10\"\n", initialSeek)
	} else {
		fmt.Fprintf(&builder, "mirv_cmd addAtTick 1 \"%s; demo_gototick %d; demo_timescale 10\"\n", cleanHUDCommandChain(hudMode), initialSeek)
	}
	lastStop := 0
	for index, clip := range clips {
		if clip.Player.SteamID == 0 {
			return "", fmt.Errorf("clip %q has no player Steam ID", clip.ID)
		}
		if err := validateConsolePath(clip.Player.Name); err != nil {
			return "", fmt.Errorf("clip %q has invalid player name: %w", clip.ID, err)
		}
		if clip.EndTick <= clip.StartTick {
			return "", fmt.Errorf("clip %q has invalid interval %d-%d", clip.ID, clip.StartTick, clip.EndTick)
		}
		if clip.MasterPath == "" {
			return "", fmt.Errorf("clip %q has no master path", clip.ID)
		}
		recordingPath := filepath.Dir(clip.MasterPath)
		if err := validateConsolePath(recordingPath); err != nil {
			return "", fmt.Errorf("clip %q has invalid recording path: %w", clip.ID, err)
		}
		cameraTick := max(1, clip.StartTick-int(5*tickRate), lastStop+1)
		fmt.Fprintf(&builder, "mirv_cmd addAtTick %d \"demo_timescale 1; spec_lock_to_accountid %d; spec_mode 1\"\n", cameraTick, clip.Player.SteamID)
		cfgName := filepath.ToSlash(filepath.Join("cs2-highlights", clipStartCFGFileName(pass.Index, clip.ID)))
		fmt.Fprintf(&builder, "mirv_cmd addAtTick %d \"exec %s\"\n", clip.StartTick, cfgName)
		if index+1 < len(clips) {
			nextSeek := clips[index+1].StartTick - int(10*tickRate)
			if nextSeek > clip.EndTick {
				fmt.Fprintf(&builder, "mirv_cmd addAtTick %d \"mirv_streams record end; demo_gototick %d; demo_timescale 10\"\n", clip.EndTick, nextSeek)
			} else {
				fmt.Fprintf(&builder, "mirv_cmd addAtTick %d \"mirv_streams record end; demo_timescale 10\"\n", clip.EndTick)
			}
		} else {
			fmt.Fprintf(&builder, "mirv_cmd addAtTick %d \"mirv_streams record end; demo_timescale 10\"\n", clip.EndTick)
		}
		lastStop = max(lastStop, clip.EndTick)
	}
	fmt.Fprintf(&builder, "mirv_cmd addAtTick %d \"quit\"\n", lastStop+int(2*tickRate))
	fmt.Fprintf(&builder, "playdemo \"%s\"\n", demoPath)
	return builder.String(), nil
}

func BuildClipCFG(clip model.Highlight) (string, error) {
	if clip.Player.SteamID == 0 {
		return "", fmt.Errorf("clip %q has no player Steam ID", clip.ID)
	}
	if err := validateConsolePath(clip.Player.Name); err != nil {
		return "", fmt.Errorf("clip %q has invalid player name: %w", clip.ID, err)
	}
	if clip.MasterPath == "" {
		return "", fmt.Errorf("clip %q has no master path", clip.ID)
	}
	recordingPath := filepath.Dir(clip.MasterPath)
	if err := validateConsolePath(recordingPath); err != nil {
		return "", fmt.Errorf("clip %q has invalid recording path: %w", clip.ID, err)
	}
	return fmt.Sprintf("demo_timescale 1\nspec_lock_to_accountid 0\nspec_mode 1\nspec_player \"%s\"\nspec_lock_to_accountid %d\nmirv_streams record name \"%s\"\nmirv_streams record start\n", clip.Player.Name, clip.Player.SteamID, recordingPath), nil
}

func clipStartCFGFileName(passIndex int, clipID string) string {
	return fmt.Sprintf("pass-%02d-%s-start.cfg", passIndex, safeCFGComponent(clipID))
}

func safeCFGComponent(value string) string {
	var builder strings.Builder
	separator := false
	for _, char := range strings.ToLower(value) {
		if char < 128 && (unicode.IsLetter(char) || unicode.IsDigit(char)) {
			if separator && builder.Len() > 0 {
				builder.WriteByte('-')
			}
			builder.WriteRune(char)
			separator = false
		} else {
			separator = true
		}
	}
	if builder.Len() == 0 {
		return "clip"
	}
	return builder.String()
}

func validateConsolePath(path string) error {
	if path == "" {
		return fmt.Errorf("path is empty")
	}
	if strings.ContainsAny(path, "\r\n\x00\"") {
		return fmt.Errorf("path contains a quote or control character")
	}
	return nil
}

func CustomLoaderArgs(hookDLL, cs2Path, cfgName string) []string {
	cmdLine := fmt.Sprintf(`-steam -insecure -novid -console -windowed -w 1920 -h 1080 +exec %s`, cfgName)
	return []string{
		"-customLoader", "-noGui", "-autoStart",
		"-hookDllPath", hookDLL,
		"-programPath", cs2Path,
		"-cmdLine", cmdLine,
	}
}
