package preflight

import (
	"fmt"
	"path/filepath"

	"golang.org/x/sys/windows"
)

func diskFreeBytes(path string) (uint64, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return 0, fmt.Errorf("resolve output volume: %w", err)
	}
	pointer, err := windows.UTF16PtrFromString(absolute)
	if err != nil {
		return 0, fmt.Errorf("encode output volume path: %w", err)
	}
	var available uint64
	if err := windows.GetDiskFreeSpaceEx(pointer, &available, nil, nil); err != nil {
		return 0, fmt.Errorf("GetDiskFreeSpaceEx(%q): %w", absolute, err)
	}
	return available, nil
}
