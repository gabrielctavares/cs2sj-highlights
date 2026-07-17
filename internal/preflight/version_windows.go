package preflight

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

func FileIdentity(path string) (string, error) {
	if version, err := fileVersion(path); err == nil && version != "" {
		return version, nil
	}
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open %q for identity: %w", path, err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("hash %q for identity: %w", path, err)
	}
	encoded := hex.EncodeToString(hash.Sum(nil))
	return "sha256:" + encoded[:12], nil
}

func fileVersion(path string) (string, error) {
	size, err := windows.GetFileVersionInfoSize(path, nil)
	if err != nil || size == 0 {
		return "", fmt.Errorf("version resource unavailable: %w", err)
	}
	buffer := make([]byte, size)
	if err := windows.GetFileVersionInfo(path, 0, size, unsafe.Pointer(&buffer[0])); err != nil {
		return "", fmt.Errorf("read version resource: %w", err)
	}
	var info *windows.VS_FIXEDFILEINFO
	var infoSize uint32
	if err := windows.VerQueryValue(unsafe.Pointer(&buffer[0]), `\`, unsafe.Pointer(&info), &infoSize); err != nil {
		return "", fmt.Errorf("query fixed file info: %w", err)
	}
	if info == nil || infoSize < uint32(unsafe.Sizeof(*info)) {
		return "", fmt.Errorf("invalid fixed file info")
	}
	return fmt.Sprintf("%d.%d.%d.%d", info.FileVersionMS>>16, info.FileVersionMS&0xffff, info.FileVersionLS>>16, info.FileVersionLS&0xffff), nil
}
