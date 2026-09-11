//go:build windows

package gui

import "testing"

func TestNativeColorConversionPreservesRGB(t *testing.T) {
	for _, want := range []string{"#1D4ED8", "#0E7490", "#FEDC34", "#000000", "#FFFFFF"} {
		if got := hexColor(colorRef(want)); got != want {
			t.Fatalf("round trip %s = %s", want, got)
		}
	}
}

func TestNativeColorPickerOffersCustomPalette(t *testing.T) {
	colors := defaultCustomColors()
	if len(colors) != 16 || hexColor(colors[0]) != "#1D4ED8" || hexColor(colors[15]) != "#000000" {
		t.Fatalf("unexpected custom colors: %#v", colors)
	}
}
