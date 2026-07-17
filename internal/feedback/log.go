package feedback

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gabrielctavares/cs2sj-highlights/internal/model"
)

type Decision struct {
	Timestamp     time.Time           `json:"timestamp"`
	DemoName      string              `json:"demo_name"`
	HighlightID   string              `json:"highlight_id"`
	Perspective   string              `json:"perspective"`
	Breadth       string              `json:"breadth"`
	PlayerSteamID uint64              `json:"player_steam_id,omitempty"`
	Score         int                 `json:"score"`
	Factors       []model.ScoreFactor `json:"factors,omitempty"`
	Confidence    model.Confidence    `json:"confidence,omitempty"`
	Selected      bool                `json:"selected"`
}

func Append(path string, decisions []Decision) (err error) {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("caminho do feedback não informado")
	}
	for index := range decisions {
		if err := validate(decisions[index]); err != nil {
			return fmt.Errorf("decisão %d inválida: %w", index+1, err)
		}
	}
	if len(decisions) == 0 {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("criar pasta do feedback: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("abrir feedback local: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); err == nil && closeErr != nil {
			err = fmt.Errorf("fechar feedback local: %w", closeErr)
		}
	}()
	if err := file.Chmod(0o600); err != nil {
		return fmt.Errorf("proteger feedback local: %w", err)
	}
	for _, decision := range decisions {
		if decision.Timestamp.IsZero() {
			decision.Timestamp = time.Now().UTC()
		} else {
			decision.Timestamp = decision.Timestamp.UTC()
		}
		line, marshalErr := json.Marshal(decision)
		if marshalErr != nil {
			return fmt.Errorf("codificar feedback local: %w", marshalErr)
		}
		line = append(line, '\n')
		if _, writeErr := file.Write(line); writeErr != nil {
			return fmt.Errorf("gravar feedback local: %w", writeErr)
		}
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sincronizar feedback local: %w", err)
	}
	return nil
}

func validate(decision Decision) error {
	if strings.TrimSpace(decision.DemoName) == "" || strings.ContainsAny(decision.DemoName, `/\\`) || filepath.Base(decision.DemoName) != decision.DemoName {
		return fmt.Errorf("nome da demo deve ser apenas o nome base")
	}
	if strings.TrimSpace(decision.HighlightID) == "" || strings.ContainsAny(decision.HighlightID, `/\\:`) {
		return fmt.Errorf("ID do highlight inválido")
	}
	if decision.Perspective != "editorial" && decision.Perspective != "individual" {
		return fmt.Errorf("perspectiva não suportada: %q", decision.Perspective)
	}
	if decision.Breadth != "restricted" && decision.Breadth != "balanced" && decision.Breadth != "broad" {
		return fmt.Errorf("abrangência não suportada: %q", decision.Breadth)
	}
	if decision.Score < 0 || decision.Score > 100 {
		return fmt.Errorf("nota fora do intervalo: %d", decision.Score)
	}
	return nil
}
