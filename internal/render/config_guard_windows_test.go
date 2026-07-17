package render

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSteamConfigGuardRestoresOriginalAndRemovesNewConvars(t *testing.T) {
	root := t.TempDir()
	cfg := filepath.Join(root, "userdata", "123", "730", "local", "cfg")
	if err := os.MkdirAll(cfg, 0o755); err != nil {
		t.Fatal(err)
	}
	original := filepath.Join(cfg, "cs2_user_convars_0_slot0.vcfg")
	autoexec := filepath.Join(cfg, "autoexec.cfg")
	if err := os.WriteFile(original, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(autoexec, []byte("personal"), 0o600); err != nil {
		t.Fatal(err)
	}
	guard := SteamConfigGuard{SteamRoot: func() (string, error) { return root, nil }}
	restore, err := guard.Protect(context.Background(), `C:\game\cs2.exe`)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(original, []byte("changed"), 0o600); err != nil {
		t.Fatal(err)
	}
	created := filepath.Join(cfg, "cs2_user_convars_1_slot0.vcfg")
	if err := os.WriteFile(created, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := restore(); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(original)
	if err != nil || string(got) != "original" {
		t.Fatalf("original was not restored: %q %v", got, err)
	}
	if _, err := os.Stat(created); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("new convars file remains: %v", err)
	}
	got, err = os.ReadFile(autoexec)
	if err != nil || string(got) != "personal" {
		t.Fatalf("autoexec was touched: %q %v", got, err)
	}
}

func TestSteamConfigGuardRejectsMissingUserConfig(t *testing.T) {
	guard := SteamConfigGuard{SteamRoot: func() (string, error) { return t.TempDir(), nil }}
	if _, err := guard.Protect(context.Background(), `C:\game\cs2.exe`); err == nil {
		t.Fatal("expected protection error")
	}
}
