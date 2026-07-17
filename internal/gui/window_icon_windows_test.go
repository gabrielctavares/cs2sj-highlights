//go:build windows

package gui

import (
	"testing"

	"github.com/lxn/walk"
)

func TestLoadApplicationIconExtractsEmbeddedExecutableResource(t *testing.T) {
	wantPath := `G:\app\CS2SJ-Demo.exe`
	called := false
	_, err := loadApplicationIcon(wantPath, func(path string, index, size int) (*walk.Icon, error) {
		called = true
		if path != wantPath || index != 0 || size != 32 {
			t.Fatalf("loader called with path=%q index=%d size=%d", path, index, size)
		}
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("embedded icon loader was not called")
	}
}
