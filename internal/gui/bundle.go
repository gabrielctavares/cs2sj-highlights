package gui

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const MissingHLAEMessage = "HLAE não encontrado em tools\\hlae. Extraia todo o conteúdo do ZIP antes de executar o programa. Se o arquivo continuar ausente, baixe o HLAE somente pela página oficial."
const HLAEDownloadURL = "https://github.com/advancedfx/advancedfx/releases/latest"

type Bundle struct {
	RootPath    string
	HLAEPath    string
	HookDLLPath string
	LogoPath    string
}

func ResolveBundle(executablePath string) (Bundle, error) {
	absoluteExecutable, err := filepath.Abs(executablePath)
	if err != nil {
		return Bundle{}, fmt.Errorf("resolver caminho do aplicativo: %w", err)
	}
	root := filepath.Join(filepath.Dir(absoluteExecutable), "tools", "hlae")
	hlae := filepath.Join(root, "hlae.exe")
	if !regularFile(hlae) {
		return Bundle{}, fmt.Errorf("%s", MissingHLAEMessage)
	}

	matches := make([]string, 0, 1)
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && strings.EqualFold(entry.Name(), "AfxHookSource2.dll") {
			absolute, absoluteErr := filepath.Abs(path)
			if absoluteErr != nil {
				return absoluteErr
			}
			matches = append(matches, absolute)
		}
		return nil
	})
	if err != nil {
		return Bundle{}, fmt.Errorf("examinar HLAE em %q: %w", root, err)
	}
	if len(matches) == 0 {
		return Bundle{}, fmt.Errorf("%s", MissingHLAEMessage)
	}
	sort.Slice(matches, func(i, j int) bool {
		relativeI, _ := filepath.Rel(root, matches[i])
		relativeJ, _ := filepath.Rel(root, matches[j])
		depthI := strings.Count(filepath.Clean(relativeI), string(filepath.Separator))
		depthJ := strings.Count(filepath.Clean(relativeJ), string(filepath.Separator))
		if depthI != depthJ {
			return depthI < depthJ
		}
		return strings.ToLower(matches[i]) < strings.ToLower(matches[j])
	})
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return Bundle{}, fmt.Errorf("resolver pasta do HLAE: %w", err)
	}
	absoluteHLAE, err := filepath.Abs(hlae)
	if err != nil {
		return Bundle{}, fmt.Errorf("resolver hlae.exe: %w", err)
	}
	logoPath := filepath.Join(filepath.Dir(absoluteExecutable), "assets", "cs2sj-logo.jpg")
	if !regularFile(logoPath) {
		logoPath = ""
	}
	return Bundle{RootPath: absoluteRoot, HLAEPath: absoluteHLAE, HookDLLPath: matches[0], LogoPath: logoPath}, nil
}

func regularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
