package companyanalysis

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kis-open-api/go/internal/dcf"
)

type Export struct {
	GeneratedAt   string                `json:"generated_at"`
	BusinessDate  string                `json:"business_date,omitempty"`
	Symbol        string                `json:"symbol"`
	Result        *Result               `json:"result,omitempty"`
	ReverseDCF    *dcf.ReverseDCFResult `json:"reverse_dcf,omitempty"`
	MonteCarloCfg dcf.MonteCarloConfig  `json:"monte_carlo_config"`
	MonteCarlo    *dcf.MonteCarloResult `json:"monte_carlo,omitempty"`
}

func WriteExport(path string, payload Export) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("path is required")
	}

	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal company analysis export: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to create export directory: %w", err)
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, raw, 0o600); err != nil {
		return fmt.Errorf("failed to write export temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("failed to replace export file: %w", err)
	}
	return nil
}
