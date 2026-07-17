package hudtheme

import (
	"strings"
	"testing"
)

func TestValidateRejectsAssetOutsideThemeDirectory(t *testing.T) {
	theme := Theme{Version: 1, Name: "evento", Elements: []Element{{
		ID: "logo", Type: Image, Anchor: TopLeft, X: 1, Y: 1, Width: 10, Height: 10, Asset: "..\\outside.png",
	}}}
	err := Validate(theme, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "asset") {
		t.Fatalf("got %v", err)
	}
}

func TestValidateRejectsDuplicateIDsAndOutOfRangeBounds(t *testing.T) {
	theme := Theme{Version: 1, Name: "evento", Elements: []Element{
		{ID: "a", Type: Box, Anchor: TopLeft, Width: 101, Height: 10},
		{ID: "a", Type: Text, Anchor: TopLeft, Width: 10, Height: 10},
	}}
	if err := Validate(theme, t.TempDir()); err == nil {
		t.Fatal("expected validation error")
	}
}
