package model

type Team string
type HUDMode string

const (
	TeamT  Team = "T"
	TeamCT Team = "CT"
)

const (
	HUDNone   HUDMode = "none"
	HUDGame   HUDMode = "game"
	HUDCustom HUDMode = "custom"
)

func (mode HUDMode) Valid() bool {
	return mode == HUDNone || mode == HUDGame || mode == HUDCustom
}

func (mode HUDMode) CaptureMode() string {
	if mode == HUDGame {
		return "game"
	}
	return "clean"
}

type DemoState string
type ClipStatus string

const (
	DemoPending      DemoState = "pending"
	DemoRendering    DemoState = "rendering"
	DemoCompleted    DemoState = "completed"
	DemoPartial      DemoState = "partial"
	DemoNoHighlights DemoState = "completed_without_highlights"
	DemoFailed       DemoState = "failed"

	ClipPending    ClipStatus = "pending"
	ClipCaptured   ClipStatus = "captured"
	ClipProcessing ClipStatus = "processing"
	ClipCompleted  ClipStatus = "completed"
	ClipFailed     ClipStatus = "failed"
)

type Player struct {
	SteamID  uint64 `json:"steam_id"`
	UserID   int    `json:"user_id"`
	Slot     int    `json:"slot"`
	Name     string `json:"name"`
	Team     Team   `json:"team"`
	TeamName string `json:"team_name,omitempty"`
}

type Kill struct {
	Tick          int    `json:"tick"`
	Killer        Player `json:"killer"`
	Victim        Player `json:"victim"`
	Weapon        string `json:"weapon"`
	IsGrenadeKill bool   `json:"is_grenade_kill"`
}

type Round struct {
	Number     int      `json:"number"`
	StartTick  int      `json:"start_tick"`
	LiveTick   int      `json:"live_tick"`
	EndTick    int      `json:"end_tick"`
	Winner     Team     `json:"winner"`
	ScoreT     int      `json:"score_t"`
	ScoreCT    int      `json:"score_ct"`
	ScoreA     int      `json:"score_a,omitempty"`
	ScoreB     int      `json:"score_b,omitempty"`
	ScoreKnown bool     `json:"score_known,omitempty"`
	MatchEnd   bool     `json:"match_end"`
	Players    []Player `json:"players"`
	Kills      []Kill   `json:"kills"`
}

type Timeline struct {
	DemoPath string  `json:"demo_path"`
	Map      string  `json:"map"`
	TeamA    string  `json:"team_a,omitempty"`
	TeamB    string  `json:"team_b,omitempty"`
	TickRate float64 `json:"tick_rate"`
	Rounds   []Round `json:"rounds"`
}

type OutputPaths struct {
	Horizontal string `json:"horizontal"`
	Vertical   string `json:"vertical"`
}

type HUDMetadata struct {
	Event      string `json:"event,omitempty"`
	TeamA      string `json:"team_a,omitempty"`
	TeamB      string `json:"team_b,omitempty"`
	Map        string `json:"map,omitempty"`
	ScoreA     int    `json:"score_a,omitempty"`
	ScoreB     int    `json:"score_b,omitempty"`
	ScoreKnown bool   `json:"score_known,omitempty"`
}

type Highlight struct {
	ID              string      `json:"id"`
	Round           int         `json:"round"`
	Player          Player      `json:"player"`
	Tags            []string    `json:"tags"`
	StartTick       int         `json:"start_tick"`
	EndTick         int         `json:"end_tick"`
	Priority        int         `json:"priority"`
	Status          ClipStatus  `json:"status"`
	Attempts        int         `json:"attempts"`
	MasterPath      string      `json:"master_path,omitempty"`
	MasterAudioPath string      `json:"master_audio_path,omitempty"`
	MasterMode      string      `json:"master_mode,omitempty"`
	MasterVersion   string      `json:"master_version,omitempty"`
	OutputHUDMode   HUDMode     `json:"output_hud_mode,omitempty"`
	OutputVersion   string      `json:"output_version,omitempty"`
	Outputs         OutputPaths `json:"outputs"`
	HUD             HUDMetadata `json:"hud"`
	LastError       string      `json:"last_error,omitempty"`
	ActionOffsets   []float64   `json:"action_offsets,omitempty"`
}

type Manifest struct {
	SchemaVersion     string      `json:"schema_version"`
	RulesVersion      string      `json:"rules_version"`
	ConfigFingerprint string      `json:"config_fingerprint"`
	DemoSHA256        string      `json:"demo_sha256"`
	DemoPath          string      `json:"demo_path"`
	Map               string      `json:"map"`
	TickRate          float64     `json:"tick_rate,omitempty"`
	TeamA             string      `json:"team_a,omitempty"`
	TeamB             string      `json:"team_b,omitempty"`
	DemoMetadata      string      `json:"demo_metadata,omitempty"`
	State             DemoState   `json:"state"`
	Highlights        []Highlight `json:"highlights"`
	Summary           OutputPaths `json:"summary"`
	LastError         string      `json:"last_error,omitempty"`
}

func NewManifest(t Timeline, demoHash, configFingerprint string, highlights []Highlight) Manifest {
	state := DemoPending
	if len(highlights) == 0 {
		state = DemoNoHighlights
	}
	return Manifest{
		SchemaVersion:     "manifest-v1",
		RulesVersion:      "rules-v2",
		ConfigFingerprint: configFingerprint,
		DemoSHA256:        demoHash,
		DemoPath:          t.DemoPath,
		Map:               t.Map,
		TickRate:          t.TickRate,
		TeamA:             t.TeamA,
		TeamB:             t.TeamB,
		DemoMetadata:      "demo-v5",
		State:             state,
		Highlights:        highlights,
	}
}
