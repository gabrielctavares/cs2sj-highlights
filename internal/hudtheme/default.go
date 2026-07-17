package hudtheme

func DefaultTheme() Theme {
	return Theme{
		Version: 1,
		Name:    "HUD padrão CS2SJ",
		Elements: []Element{
			{ID: "event-bar", Type: Box, Anchor: TopCenter, X: 22, Y: 1, Width: 56, Height: 3, ZIndex: 1, Visible: true, Color: "#0D1219", Opacity: 0.94},
			{ID: "score-bar", Type: Box, Anchor: TopCenter, X: 18, Y: 4, Width: 64, Height: 8, ZIndex: 1, Visible: true, Color: "#111923", Opacity: 0.94},
			{ID: "team-a-logo", Type: Image, Anchor: TopLeft, X: 18, Y: 4, Width: 5, Height: 8, ZIndex: 3, Visible: true, Binding: TeamALogo},
			{ID: "team-b-logo", Type: Image, Anchor: TopRight, X: 77, Y: 4, Width: 5, Height: 8, ZIndex: 3, Visible: true, Binding: TeamBLogo},
			{ID: "event", Type: Text, Anchor: TopCenter, X: 22, Y: 1, Width: 56, Height: 3, ZIndex: 2, Visible: true, Binding: Event, FontSize: 18, Color: "#FFFFFF"},
			{ID: "team-a", Type: Text, Anchor: TopLeft, X: 24, Y: 6, Width: 15, Height: 4, ZIndex: 2, Visible: true, Binding: TeamAName, FontSize: 31, Color: "#FFFFFF"},
			{ID: "score-a", Type: Text, Anchor: TopCenter, X: 39, Y: 5, Width: 8, Height: 5, ZIndex: 2, Visible: true, Binding: ScoreA, FontSize: 36, Color: "#FFFFFF"},
			{ID: "score-b", Type: Text, Anchor: TopCenter, X: 53, Y: 5, Width: 8, Height: 5, ZIndex: 2, Visible: true, Binding: ScoreB, FontSize: 36, Color: "#FFFFFF"},
			{ID: "team-b", Type: Text, Anchor: TopRight, X: 61, Y: 6, Width: 15, Height: 4, ZIndex: 2, Visible: true, Binding: TeamBName, FontSize: 31, Color: "#FFFFFF"},
			{ID: "map", Type: Text, Anchor: TopCenter, X: 44, Y: 5, Width: 12, Height: 5, ZIndex: 2, Visible: true, Binding: Map, FontSize: 22, Color: "#FFFFFF"},
			{ID: "round", Type: Text, Anchor: TopCenter, X: 44, Y: 8, Width: 12, Height: 3, ZIndex: 2, Visible: true, Binding: Round, FontSize: 16, Color: "#FFFFFF"},
			{ID: "highlight-card", Type: Box, Anchor: BottomCenter, X: 37, Y: 90, Width: 26, Height: 7, ZIndex: 1, Visible: true, Color: "#0D1219", Opacity: 0.92},
			{ID: "player", Type: Text, Anchor: BottomCenter, X: 37, Y: 90, Width: 26, Height: 4, ZIndex: 2, Visible: true, Binding: Player, FontSize: 26, Color: "#FFFFFF"},
			{ID: "highlight", Type: Text, Anchor: BottomCenter, X: 37, Y: 94, Width: 26, Height: 3, ZIndex: 2, Visible: true, Binding: Highlight, FontSize: 18, Color: "#FFFFFF"},
		},
	}
}
