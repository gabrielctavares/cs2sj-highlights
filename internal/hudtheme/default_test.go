package hudtheme

import "testing"

func TestDefaultThemeUsesExternalTeamLogos(t *testing.T) {
	theme := DefaultTheme()
	if err := Validate(theme, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	bindings := map[Binding]bool{}
	for _, element := range theme.Elements {
		bindings[element.Binding] = true
	}
	if !bindings[TeamALogo] || !bindings[TeamBLogo] {
		t.Fatalf("team logo bindings missing: %#v", theme.Elements)
	}
}
