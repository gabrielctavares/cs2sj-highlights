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

type Confidence string

const (
	ConfidenceLow    Confidence = "low"
	ConfidenceMedium Confidence = "medium"
	ConfidenceHigh   Confidence = "high"
)

type ScoreFactor struct {
	Code   string `json:"code"`
	Points int    `json:"points"`
}

type Evaluation struct {
	Score       int           `json:"score"`
	Confidence  Confidence    `json:"confidence"`
	Factors     []ScoreFactor `json:"factors,omitempty"`
	Explanation string        `json:"explanation,omitempty"`
}

type CandidateContext struct {
	KillCount          int  `json:"kill_count"`
	AssistCount        int  `json:"assist_count,omitempty"`
	FlashAssistCount   int  `json:"flash_assist_count,omitempty"`
	ClutchOpponents    int  `json:"clutch_opponents,omitempty"`
	TradeKills         int  `json:"trade_kills,omitempty"`
	LowestKillerHealth int  `json:"lowest_killer_health,omitempty"`
	WonRound           bool `json:"won_round,omitempty"`
	OpeningKill        bool `json:"opening_kill,omitempty"`
	FastSequence       bool `json:"fast_sequence,omitempty"`
	MatchPoint         bool `json:"match_point,omitempty"`
	MatchEnd           bool `json:"match_end,omitempty"`
	Overtime           int  `json:"overtime,omitempty"`
}

type CandidateDiscard struct {
	Round  int    `json:"round"`
	Player Player `json:"player"`
	Code   string `json:"code"`
}

type Kill struct {
	Tick              int     `json:"tick"`
	Killer            Player  `json:"killer"`
	Victim            Player  `json:"victim"`
	Assister          Player  `json:"assister,omitempty"`
	Weapon            string  `json:"weapon"`
	IsGrenadeKill     bool    `json:"is_grenade_kill"`
	PenetratedObjects int     `json:"penetrated_objects,omitempty"`
	IsHeadshot        bool    `json:"is_headshot,omitempty"`
	AssistedFlash     bool    `json:"assisted_flash,omitempty"`
	AttackerBlind     bool    `json:"attacker_blind,omitempty"`
	NoScope           bool    `json:"no_scope,omitempty"`
	ThroughSmoke      bool    `json:"through_smoke,omitempty"`
	Distance          float64 `json:"distance,omitempty"`
	DistanceKnown     bool    `json:"distance_known,omitempty"`
	KillerHealth      int     `json:"killer_health,omitempty"`
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
	Overtime   int      `json:"overtime,omitempty"`
	MatchPoint bool     `json:"match_point,omitempty"`
	MatchEnd   bool     `json:"match_end"`
	Players    []Player `json:"players"`
	Kills      []Kill   `json:"kills"`
}

type Timeline struct {
	DemoPath            string  `json:"demo_path"`
	Map                 string  `json:"map"`
	TeamA               string  `json:"team_a,omitempty"`
	TeamB               string  `json:"team_b,omitempty"`
	TickRate            float64 `json:"tick_rate"`
	RegulationMaxRounds int     `json:"regulation_max_rounds,omitempty"`
	OvertimeMaxRounds   int     `json:"overtime_max_rounds,omitempty"`
	Rounds              []Round `json:"rounds"`
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
	ID              string           `json:"id"`
	Round           int              `json:"round"`
	Player          Player           `json:"player"`
	Tags            []string         `json:"tags"`
	StartTick       int              `json:"start_tick"`
	EndTick         int              `json:"end_tick"`
	Priority        int              `json:"priority"`
	Status          ClipStatus       `json:"status"`
	Attempts        int              `json:"attempts"`
	MasterPath      string           `json:"master_path,omitempty"`
	MasterAudioPath string           `json:"master_audio_path,omitempty"`
	MasterMode      string           `json:"master_mode,omitempty"`
	MasterVersion   string           `json:"master_version,omitempty"`
	OutputHUDMode   HUDMode          `json:"output_hud_mode,omitempty"`
	OutputVersion   string           `json:"output_version,omitempty"`
	Outputs         OutputPaths      `json:"outputs"`
	HUD             HUDMetadata      `json:"hud"`
	LastError       string           `json:"last_error,omitempty"`
	ActionOffsets   []float64        `json:"action_offsets,omitempty"`
	Actions         []Kill           `json:"actions,omitempty"`
	Context         CandidateContext `json:"context"`
	Individual      Evaluation       `json:"individual"`
	Editorial       Evaluation       `json:"editorial"`
}

type Manifest struct {
	SchemaVersion        string             `json:"schema_version"`
	RulesVersion         string             `json:"rules_version"`
	ConfigFingerprint    string             `json:"config_fingerprint"`
	DemoSHA256           string             `json:"demo_sha256"`
	DemoPath             string             `json:"demo_path"`
	Map                  string             `json:"map"`
	TickRate             float64            `json:"tick_rate,omitempty"`
	TeamA                string             `json:"team_a,omitempty"`
	TeamB                string             `json:"team_b,omitempty"`
	DemoMetadata         string             `json:"demo_metadata,omitempty"`
	CandidateVersion     string             `json:"candidate_version,omitempty"`
	ScoringVersion       string             `json:"scoring_version,omitempty"`
	DiversityVersion     string             `json:"diversity_version,omitempty"`
	State                DemoState          `json:"state"`
	Highlights           []Highlight        `json:"highlights"`
	SelectedHighlightIDs []string           `json:"selected_highlight_ids"`
	DiscardedCandidates  []CandidateDiscard `json:"discarded_candidates,omitempty"`
	Summary              OutputPaths        `json:"summary"`
	LastError            string             `json:"last_error,omitempty"`
}

func NewManifest(t Timeline, demoHash, configFingerprint string, highlights []Highlight) Manifest {
	state := DemoPending
	if len(highlights) == 0 {
		state = DemoNoHighlights
	}
	return Manifest{
		SchemaVersion:     ManifestSchemaVersion,
		RulesVersion:      RulesVersion,
		ConfigFingerprint: configFingerprint,
		DemoSHA256:        demoHash,
		DemoPath:          t.DemoPath,
		Map:               t.Map,
		TickRate:          t.TickRate,
		TeamA:             t.TeamA,
		TeamB:             t.TeamB,
		DemoMetadata:      DemoMetadataVersion,
		CandidateVersion:  CandidateVersion,
		ScoringVersion:    ScoringVersion,
		DiversityVersion:  DiversityVersion,
		State:             state,
		Highlights:        highlights,
	}
}
