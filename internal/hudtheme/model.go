package hudtheme

type Anchor string

const (
	TopLeft      Anchor = "top_left"
	TopCenter    Anchor = "top_center"
	TopRight     Anchor = "top_right"
	Center       Anchor = "center"
	BottomLeft   Anchor = "bottom_left"
	BottomCenter Anchor = "bottom_center"
	BottomRight  Anchor = "bottom_right"
)

type ElementType string

const (
	Box   ElementType = "box"
	Text  ElementType = "text"
	Image ElementType = "image"
)

type Binding string

const (
	Event     Binding = "event"
	TeamAName Binding = "team_a_name"
	TeamBName Binding = "team_b_name"
	TeamALogo Binding = "team_a_logo"
	TeamBLogo Binding = "team_b_logo"
	ScoreA    Binding = "score_a"
	ScoreB    Binding = "score_b"
	Map       Binding = "map"
	Round     Binding = "round"
	Player    Binding = "player"
	Highlight Binding = "highlight"
)

type Element struct {
	ID       string      `json:"id"`
	Type     ElementType `json:"type"`
	Anchor   Anchor      `json:"anchor"`
	X        float64     `json:"x"`
	Y        float64     `json:"y"`
	Width    float64     `json:"width"`
	Height   float64     `json:"height"`
	ZIndex   int         `json:"z_index"`
	Visible  bool        `json:"visible"`
	Binding  Binding     `json:"binding,omitempty"`
	Text     string      `json:"text,omitempty"`
	Asset    string      `json:"asset,omitempty"`
	Font     string      `json:"font,omitempty"`
	FontSize int         `json:"font_size,omitempty"`
	Color    string      `json:"color,omitempty"`
	Opacity  float64     `json:"opacity,omitempty"`
}

type Theme struct {
	Version  int       `json:"version"`
	Name     string    `json:"name"`
	Elements []Element `json:"elements"`
}

type Values map[Binding]string
