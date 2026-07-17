package gui

import (
	"os"
	"path/filepath"
	"testing"
)

func touchFile(t *testing.T, path string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	return absolute
}

func TestResolveBundleFindsExecutableAndNestedHook(t *testing.T) {
	root := t.TempDir()
	executable := touchFile(t, filepath.Join(root, "CS2SJ-Demo.exe"))
	hlae := touchFile(t, filepath.Join(root, "tools", "hlae", "hlae.exe"))
	hook := touchFile(t, filepath.Join(root, "tools", "hlae", "x64", "AfxHookSource2.dll"))
	logo := touchFile(t, filepath.Join(root, "assets", "cs2sj-logo.jpg"))

	got, err := ResolveBundle(executable)
	if err != nil {
		t.Fatal(err)
	}
	if got.RootPath != filepath.Join(root, "tools", "hlae") || got.HLAEPath != hlae || got.HookDLLPath != hook || got.LogoPath != logo {
		t.Fatalf("got %#v", got)
	}
}

func TestResolveBundleSelectsShallowestHookCaseInsensitive(t *testing.T) {
	root := t.TempDir()
	executable := touchFile(t, filepath.Join(root, "CS2SJ-Demo.exe"))
	touchFile(t, filepath.Join(root, "tools", "hlae", "hlae.exe"))
	want := touchFile(t, filepath.Join(root, "tools", "hlae", "AfxHookSource2.DLL"))
	touchFile(t, filepath.Join(root, "tools", "hlae", "deep", "x64", "AfxHookSource2.dll"))

	got, err := ResolveBundle(executable)
	if err != nil {
		t.Fatal(err)
	}
	if got.HookDLLPath != want {
		t.Fatalf("got %q, want %q", got.HookDLLPath, want)
	}
}

func TestResolveBundleReturnsExactMissingHLAEError(t *testing.T) {
	_, err := ResolveBundle(filepath.Join(t.TempDir(), "CS2SJ-Demo.exe"))
	if err == nil || err.Error() != MissingHLAEMessage {
		t.Fatalf("got %v", err)
	}
	if HLAEDownloadURL != "https://github.com/advancedfx/advancedfx/releases/latest" {
		t.Fatal(HLAEDownloadURL)
	}
}

func TestResolveBundleMissingHookUsesExactHLAEError(t *testing.T) {
	root := t.TempDir()
	executable := touchFile(t, filepath.Join(root, "CS2SJ-Demo.exe"))
	touchFile(t, filepath.Join(root, "tools", "hlae", "hlae.exe"))

	_, err := ResolveBundle(executable)
	if err == nil || err.Error() != MissingHLAEMessage {
		t.Fatalf("got %v", err)
	}
}
