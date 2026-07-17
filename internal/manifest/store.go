package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/gabrielctavares/cs2sj-highlights/internal/model"
)

type Store struct{}

func SHA256File(path string) (result string, err error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open %q for SHA-256: %w", path, err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close %q after SHA-256: %w", path, closeErr)
		}
	}()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("hash %q: %w", path, err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func Fingerprint(config any) (string, error) {
	data, err := json.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("encode configuration fingerprint: %w", err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func (Store) Load(path string) (result model.Manifest, err error) {
	file, err := os.Open(path)
	if err != nil {
		return model.Manifest{}, fmt.Errorf("open manifest %q: %w", path, err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close manifest %q: %w", path, closeErr)
		}
	}()
	if err := json.NewDecoder(file).Decode(&result); err != nil {
		return model.Manifest{}, fmt.Errorf("decode manifest %q: %w", path, err)
	}
	return result, nil
}

func (Store) Save(path string, value model.Manifest) (err error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create manifest directory for %q: %w", path, err)
	}
	temporary := path + ".tmp"
	defer func() {
		if err != nil {
			_ = os.Remove(temporary)
		}
	}()
	file, err := os.OpenFile(temporary, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create temporary manifest %q: %w", temporary, err)
	}
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if encodeErr := encoder.Encode(value); encodeErr != nil {
		_ = file.Close()
		return fmt.Errorf("encode temporary manifest %q: %w", temporary, encodeErr)
	}
	if syncErr := file.Sync(); syncErr != nil {
		_ = file.Close()
		return fmt.Errorf("sync temporary manifest %q: %w", temporary, syncErr)
	}
	if closeErr := file.Close(); closeErr != nil {
		return fmt.Errorf("close temporary manifest %q: %w", temporary, closeErr)
	}
	if renameErr := os.Rename(temporary, path); renameErr != nil {
		return fmt.Errorf("replace manifest %q: %w", path, renameErr)
	}
	return nil
}

func DemoCompatible(manifest model.Manifest, demoHash, configFingerprint string) bool {
	supportedSchema := manifest.SchemaVersion == "manifest-v1" || manifest.SchemaVersion == model.ManifestSchemaVersion
	return supportedSchema && manifest.DemoSHA256 == demoHash && manifest.ConfigFingerprint == configFingerprint
}

func CatalogCurrent(manifest model.Manifest) bool {
	return manifest.SchemaVersion == model.ManifestSchemaVersion &&
		manifest.RulesVersion == model.RulesVersion &&
		manifest.DemoMetadata == model.DemoMetadataVersion &&
		manifest.CandidateVersion == model.CandidateVersion &&
		manifest.ScoringVersion == model.ScoringVersion &&
		manifest.DiversityVersion == model.DiversityVersion
}

func Compatible(manifest model.Manifest, demoHash, configFingerprint string) bool {
	return DemoCompatible(manifest, demoHash, configFingerprint) && CatalogCurrent(manifest)
}
