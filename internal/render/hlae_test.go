package render

import (
	"slices"
	"strings"
	"testing"

	"github.com/gabrielctavares/cs2sj-highlights/internal/model"
)

func TestBuildCFG(t *testing.T) {
	pass := RenderPass{Index: 1, Clips: []model.Highlight{{
		ID: "r03-p7-t1200", StartTick: 1200, EndTick: 1800,
		Player: model.Player{SteamID: 76561198000000007, Slot: 7, Name: "gabi"}, MasterPath: `C:\videos\demo\masters\r03-p7-t1200\video.mp4`,
	}}}
	got, err := BuildCFG(`C:\demos\final.dem`, pass, 64, model.HUDNone)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{
		"mirv_cmd clear",
		"mirv_streams settings edit afxDefault settings afxFfmpeg",
		"mirv_streams record screen enabled 1",
		"mirv_streams record fps 60",
		"mirv_streams record startMovieWav 1",
		"voice_modenable 0",
		"demo_ui_mode 0",
		"cl_showdemooverlay 0",
		`mirv_cmd addAtTick 1 "cl_drawhud 1; cl_draw_only_deathnotices 1; cl_drawhud_force_deathnotices 1; cl_drawhud_force_radar -1; cl_drawhud_force_teamid_overhead -1; cl_trueview_show_status 0; demo_gototick 560; demo_timescale 10"`,
		`mirv_cmd addAtTick 880 "demo_timescale 1; spec_lock_to_accountid 39734279; spec_mode 1"`,
		`mirv_cmd addAtTick 1200 "exec cs2-highlights/pass-01-r03-p7-t1200-start.cfg"`,
		`mirv_cmd addAtTick 1800 "mirv_streams record end; demo_timescale 10"`,
		`mirv_cmd addAtTick 1928 "quit"`,
		`playdemo "C:\demos\final.dem"`,
	} {
		if !strings.Contains(got, line) {
			t.Errorf("missing %q in:\n%s", line, got)
		}
	}
	if !strings.HasSuffix(got, `playdemo "C:\demos\final.dem"`+"\n") {
		t.Fatalf("playdemo must be final line:\n%s", got)
	}
}

func TestBuildCFGSlowsDownBeforeRecordingStarts(t *testing.T) {
	pass := RenderPass{Index: 1, Clips: []model.Highlight{{
		ID: "clip", StartTick: 1200, EndTick: 1800,
		Player: model.Player{SteamID: 76561198000000001, Name: "peteR"}, MasterPath: `C:\videos\clip\video.mp4`,
	}}}
	got, err := BuildCFG(`C:\demos\match.dem`, pass, 64, model.HUDCustom)
	if err != nil {
		t.Fatal(err)
	}
	want := `mirv_cmd addAtTick 880 "demo_timescale 1; spec_lock_to_accountid 39734273; spec_mode 1"`
	if !strings.Contains(got, want) {
		t.Fatalf("capture must stabilize at 1x five seconds before recording; missing %q in:\n%s", want, got)
	}
}

func TestBuildCFGSeeksDirectlyBetweenDistantClips(t *testing.T) {
	pass := RenderPass{Index: 1, Clips: []model.Highlight{
		{
			ID: "first", StartTick: 1200, EndTick: 1800,
			Player: model.Player{SteamID: 76561198000000001, Name: "first"}, MasterPath: `C:\videos\first\video.mp4`,
		},
		{
			ID: "second", StartTick: 6400, EndTick: 7000,
			Player: model.Player{SteamID: 76561198000000002, Name: "second"}, MasterPath: `C:\videos\second\video.mp4`,
		},
	}}

	got, err := BuildCFG(`C:\demos\final.dem`, pass, 64, model.HUDNone)
	if err != nil {
		t.Fatal(err)
	}

	want := `mirv_cmd addAtTick 1800 "mirv_streams record end; demo_gototick 5760; demo_timescale 10"`
	if !strings.Contains(got, want) {
		t.Fatalf("missing direct seek between distant clips %q in:\n%s", want, got)
	}
}

func TestBuildCFGDoesNotSeekBackwardsBetweenCloseClips(t *testing.T) {
	pass := RenderPass{Index: 1, Clips: []model.Highlight{
		{
			ID: "first", StartTick: 1200, EndTick: 1800,
			Player: model.Player{SteamID: 76561198000000001, Name: "first"}, MasterPath: `C:\videos\first\video.mp4`,
		},
		{
			ID: "second", StartTick: 2000, EndTick: 2400,
			Player: model.Player{SteamID: 76561198000000002, Name: "second"}, MasterPath: `C:\videos\second\video.mp4`,
		},
	}}

	got, err := BuildCFG(`C:\demos\final.dem`, pass, 64, model.HUDNone)
	if err != nil {
		t.Fatal(err)
	}

	want := `mirv_cmd addAtTick 1800 "mirv_streams record end; demo_timescale 10"`
	if !strings.Contains(got, want) {
		t.Fatalf("close clips must keep fast-forwarding %q in:\n%s", want, got)
	}
	if strings.Contains(got, `mirv_cmd addAtTick 1800 "mirv_streams record end; demo_gototick`) {
		t.Fatalf("close clips must not seek backwards:\n%s", got)
	}
}

func TestBuildClipCFGQuotesRecordingPath(t *testing.T) {
	clip := model.Highlight{
		ID: "r03-p7-t1200", StartTick: 1200, EndTick: 1800,
		Player: model.Player{SteamID: 76561198000000007, Name: "gabi"}, MasterPath: `C:\videos folder\demo\masters\r03-p7-t1200\video.mp4`,
	}
	got, err := BuildClipCFG(clip)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{
		"demo_timescale 1",
		`spec_player "gabi"`,
		"spec_lock_to_accountid 39734279",
		"spec_mode 1",
		`mirv_streams record name "C:\videos folder\demo\masters\r03-p7-t1200"`,
		"mirv_streams record start",
	} {
		if !strings.Contains(got, line) {
			t.Errorf("missing %q in:\n%s", line, got)
		}
	}
}

func TestBuildCFGRejectsUnsafeValues(t *testing.T) {
	valid := RenderPass{Index: 1, Clips: []model.Highlight{{ID: "a", StartTick: 1, EndTick: 2, Player: model.Player{SteamID: 76561198000000001, Slot: 1, Name: "gabi"}, MasterPath: `C:\out\a\video.mp4`}}}
	tests := []struct {
		name string
		demo string
		pass RenderPass
	}{
		{"empty pass", `C:\a.dem`, RenderPass{}},
		{"newline", "C:\\bad\n.dem", valid},
		{"quote", `C:\"bad.dem`, valid},
		{"missing steam id", `C:\a.dem`, RenderPass{Clips: []model.Highlight{{StartTick: 1, EndTick: 2, Player: model.Player{Slot: 1, Name: "gabi"}, MasterPath: `C:\out\a\video.mp4`}}}},
		{"unsafe player name", `C:\a.dem`, RenderPass{Clips: []model.Highlight{{StartTick: 1, EndTick: 2, Player: model.Player{SteamID: 76561198000000001, Name: `ga"bi`}, MasterPath: `C:\out\a\video.mp4`}}}},
		{"unsafe recording path", `C:\a.dem`, RenderPass{Clips: []model.Highlight{{StartTick: 1, EndTick: 2, Player: model.Player{SteamID: 76561198000000001, Slot: 1, Name: "gabi"}, MasterPath: `C:\out"bad\a\video.mp4`}}}},
		{"invalid interval", `C:\a.dem`, RenderPass{Clips: []model.Highlight{{StartTick: 2, EndTick: 2, Player: model.Player{SteamID: 76561198000000001, Name: "gabi"}, MasterPath: `C:\out\a\video.mp4`}}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := BuildCFG(test.demo, test.pass, 64, model.HUDNone); err == nil {
				t.Fatal("expected error")
			}
		})
	}
	if _, err := BuildCFG(`C:\a.dem`, valid, 0, model.HUDNone); err == nil {
		t.Fatal("non-positive tick rate must fail")
	}
}

func TestBuildCFGHUDModesAndDisablesPersonalDiagnostics(t *testing.T) {
	pass := RenderPass{Index: 1, Clips: []model.Highlight{{ID: "clip", StartTick: 100, EndTick: 200, Player: model.Player{SteamID: 1, Name: "Ana"}, MasterPath: `C:\out\video.mp4`}}}
	for _, test := range []struct {
		mode model.HUDMode
		draw string
	}{{model.HUDNone, "cl_drawhud 1"}, {model.HUDCustom, "cl_drawhud 1"}, {model.HUDGame, "cl_drawhud 1"}} {
		got, err := BuildCFG(`C:\demo.dem`, pass, 64, test.mode)
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{test.draw, "demo_ui_mode 0", "cl_showdemooverlay 0", "cl_showfps 0", "r_show_build_info 0", "cl_hud_telemetry_frametime_show 0", "cl_hud_telemetry_ping_show 0", "cl_hud_telemetry_net_misdelivery_show 0", "cl_hud_telemetry_net_quality_graph_show 0"} {
			if !strings.Contains(got, want) {
				t.Errorf("mode %s missing %q in:\n%s", test.mode, want, got)
			}
		}
	}
}

func TestBuildCFGCleanModesForceEveryNativeHUDLayerOff(t *testing.T) {
	pass := RenderPass{Index: 1, Clips: []model.Highlight{{ID: "clip", StartTick: 100, EndTick: 200, Player: model.Player{SteamID: 1, Name: "Ana"}, MasterPath: `C:\out\video.mp4`}}}
	for _, mode := range []model.HUDMode{model.HUDNone, model.HUDCustom} {
		got, err := BuildCFG(`C:\demo.dem`, pass, 64, mode)
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{
			"cl_drawhud 1",
			"cl_draw_only_deathnotices 1",
			"cl_drawhud_force_radar -1",
			"cl_drawhud_force_teamid_overhead -1",
			"cl_trueview_show_status 0",
			"cl_showpos 0",
			"cl_showtick 0",
			"cl_showmem 0",
			"cl_showframenumber 0",
			"r_show_time_info 0",
		} {
			if !strings.Contains(got, want+"\n") {
				t.Errorf("mode %s missing %q in:\n%s", mode, want, got)
			}
		}
		deathNotices := "cl_drawhud_force_deathnotices 1\n"
		if !strings.Contains(got, deathNotices) {
			t.Errorf("mode %s missing %q in:\n%s", mode, strings.TrimSpace(deathNotices), got)
		}
	}
}

func TestBuildCFGReappliesCleanHUDCommandsAfterDemoLoads(t *testing.T) {
	pass := RenderPass{Index: 1, Clips: []model.Highlight{{ID: "clip", StartTick: 1200, EndTick: 1800, Player: model.Player{SteamID: 1, Name: "Ana"}, MasterPath: `C:\out\video.mp4`}}}
	got, err := BuildCFG(`C:\demo.dem`, pass, 64, model.HUDCustom)
	if err != nil {
		t.Fatal(err)
	}
	want := `mirv_cmd addAtTick 1 "cl_drawhud 1; cl_draw_only_deathnotices 1; cl_drawhud_force_deathnotices 1; cl_drawhud_force_radar -1; cl_drawhud_force_teamid_overhead -1; cl_trueview_show_status 0; demo_gototick 560; demo_timescale 10"`
	if !strings.Contains(got, want) {
		t.Fatalf("clean HUD commands are not reapplied after playdemo:\n%s", got)
	}
}

func TestBuildCFGCleanModesKeepNativeKillFeed(t *testing.T) {
	pass := RenderPass{Index: 1, Clips: []model.Highlight{{ID: "clip", StartTick: 1200, EndTick: 1800, Player: model.Player{SteamID: 1, Name: "Ana"}, MasterPath: `C:\out\video.mp4`}}}
	custom, err := BuildCFG(`C:\demo.dem`, pass, 64, model.HUDCustom)
	if err != nil {
		t.Fatal(err)
	}
	none, err := BuildCFG(`C:\demo.dem`, pass, 64, model.HUDNone)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(custom, "cl_drawhud_force_deathnotices 1\n") || strings.Contains(custom, "cl_drawhud_force_deathnotices -1\n") {
		t.Fatalf("custom HUD must keep the kill feed only:\n%s", custom)
	}
	if !strings.Contains(none, "cl_drawhud_force_deathnotices 1\n") {
		t.Fatalf("none mode must keep the kill feed:\n%s", none)
	}
}

func TestBuildCFGCleanModesKeepNativeCrosshairAndKillFeed(t *testing.T) {
	pass := RenderPass{Index: 1, Clips: []model.Highlight{{ID: "clip", StartTick: 1200, EndTick: 1800, Player: model.Player{SteamID: 1, Name: "Ana"}, MasterPath: `C:\out\video.mp4`}}}
	for _, mode := range []model.HUDMode{model.HUDNone, model.HUDCustom} {
		got, err := BuildCFG(`C:\demo.dem`, pass, 64, mode)
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"cl_drawhud 1\n", "cl_draw_only_deathnotices 1\n", "cl_drawhud_force_deathnotices 1\n"} {
			if !strings.Contains(got, want) {
				t.Fatalf("mode %s must keep native crosshair and kill feed; missing %q in:\n%s", mode, strings.TrimSpace(want), got)
			}
		}
	}
}

func TestCustomLoaderArgsKeepPathsSeparateAndInsecure(t *testing.T) {
	args := CustomLoaderArgs(`C:\HLAE Folder\AfxHookSource2.dll`, `C:\Steam Library\cs2.exe`, "cs2-highlights-pass-01.cfg")
	if !slices.Contains(args, `C:\HLAE Folder\AfxHookSource2.dll`) || !slices.Contains(args, `C:\Steam Library\cs2.exe`) {
		t.Fatalf("paths were not kept as argument values: %#v", args)
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "-insecure") || !strings.Contains(joined, "+exec cs2-highlights-pass-01.cfg") {
		t.Fatalf("unexpected arguments: %#v", args)
	}
}
