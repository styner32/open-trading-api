package dcf

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type MonteCarloExport struct {
	GeneratedAt   string            `json:"generated_at"`
	BusinessDate  string            `json:"business_date,omitempty"`
	Symbol        string            `json:"symbol"`
	CurrentPrice  float64           `json:"current_price"`
	Financial     FinancialData     `json:"financial"`
	Market        MarketData        `json:"market"`
	Assumptions   Assumptions       `json:"assumptions"`
	Projection    ProjectionModel   `json:"projection"`
	Valuation     *ValuationResult  `json:"valuation,omitempty"`
	ReverseDCF    *ReverseDCFResult `json:"reverse_dcf,omitempty"`
	MonteCarloCfg MonteCarloConfig  `json:"monte_carlo_config"`
	MonteCarlo    *MonteCarloResult `json:"monte_carlo,omitempty"`
}

func WriteMonteCarloExport(path string, payload MonteCarloExport) error {
	if path == "" {
		return fmt.Errorf("path is required")
	}

	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal monte carlo export: %w", err)
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
