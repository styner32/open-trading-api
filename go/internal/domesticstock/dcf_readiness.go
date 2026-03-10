package domesticstock

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/kis-open-api/go/internal/auth"
)

const (
	financeBalanceSheetPath    = "/uapi/domestic-stock/v1/finance/balance-sheet"
	financeIncomeStatementPath = "/uapi/domestic-stock/v1/finance/income-statement"
	defaultFinanceDivClsCode   = "0"
	defaultFinanceMarketDiv    = "J"
)

const (
	financeBalanceSheetTRID    = "FHKST66430100"
	financeIncomeStatementTRID = "FHKST66430200"
)

type DCFInputStatus string

const (
	DCFInputExact   DCFInputStatus = "exact"
	DCFInputDerived DCFInputStatus = "derived"
	DCFInputAssumed DCFInputStatus = "assumed"
	DCFInputMissing DCFInputStatus = "missing"
)

type DCFInputValue struct {
	Name     string
	Status   DCFInputStatus
	Value    float64
	HasValue bool
	Source   string
	Note     string
}

type DCFReadinessResult struct {
	Symbol                    string
	Division                  string
	CanProjectFCF             bool
	CanComputeWACC            bool
	CanComputeEnterpriseValue bool
	CanComputeTargetPrice     bool
	BalancePeriods            int
	IncomePeriods             int
	Inputs                    []DCFInputValue
	MissingForFCF             []string
	MissingForWACC            []string
	MissingForTargetPrice     []string
}

func (s *Service) FinanceBalanceSheet(ctx context.Context, symbol string, divClsCode string) (*auth.RESTResponse, error) {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return nil, errors.New("symbol is required")
	}

	divClsCode = strings.TrimSpace(divClsCode)
	if divClsCode == "" {
		divClsCode = defaultFinanceDivClsCode
	}

	params := map[string]string{
		"FID_DIV_CLS_CODE":       divClsCode,
		"fid_cond_mrkt_div_code": defaultFinanceMarketDiv,
		"fid_input_iscd":         symbol,
	}
	return s.client.Get(ctx, financeBalanceSheetPath, financeBalanceSheetTRID, "", params)
}

func (s *Service) FinanceIncomeStatement(ctx context.Context, symbol string, divClsCode string) (*auth.RESTResponse, error) {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return nil, errors.New("symbol is required")
	}

	divClsCode = strings.TrimSpace(divClsCode)
	if divClsCode == "" {
		divClsCode = defaultFinanceDivClsCode
	}

	params := map[string]string{
		"FID_DIV_CLS_CODE":       divClsCode,
		"fid_cond_mrkt_div_code": defaultFinanceMarketDiv,
		"fid_input_iscd":         symbol,
	}
	return s.client.Get(ctx, financeIncomeStatementPath, financeIncomeStatementTRID, "", params)
}

func (s *Service) DCFReadiness(ctx context.Context, symbol string, divClsCode string) (*DCFReadinessResult, error) {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return nil, errors.New("symbol is required")
	}

	bundle, err := s.buildDCFInputBundle(ctx, symbol, DCFValuationOptions{DivClsCode: divClsCode})
	if err != nil {
		return nil, err
	}

	result := &DCFReadinessResult{
		Symbol:         bundle.Symbol,
		Division:       bundle.Division,
		BalancePeriods: len(bundle.BalanceRows),
		IncomePeriods:  len(bundle.IncomeRows),
		Inputs:         bundle.Inputs,
	}
	result.MissingForFCF = missingInputNames(result.Inputs, "Revenue", "EBIT", "EffectiveTax", "DnA", "CapEx", "ChangeInNWC")
	result.MissingForWACC = missingInputNames(result.Inputs, "RiskFreeRate", "Beta", "MarketPremium", "CostOfDebt", "EquityWeight", "DebtWeight", "EffectiveTax")
	result.MissingForTargetPrice = missingInputNames(result.Inputs, "SharesOut", "NetDebt")
	result.CanProjectFCF = len(result.MissingForFCF) == 0
	result.CanComputeWACC = len(result.MissingForWACC) == 0
	result.CanComputeEnterpriseValue = result.CanProjectFCF && result.CanComputeWACC
	result.CanComputeTargetPrice = result.CanComputeEnterpriseValue && len(result.MissingForTargetPrice) == 0
	return result, nil
}

func buildDCFInput(name string, row map[string]any, key string, status DCFInputStatus, source string, note string) DCFInputValue {
	value, ok := parseFloat(row[key])
	if !ok {
		return DCFInputValue{
			Name:   name,
			Status: DCFInputMissing,
			Source: source,
			Note:   note,
		}
	}

	return DCFInputValue{
		Name:     name,
		Status:   status,
		Value:    value,
		HasValue: true,
		Source:   source,
		Note:     note,
	}
}

func sortedFinanceRows(resp *auth.RESTResponse) []map[string]any {
	rows := toRows(resp.Body["output"])
	sort.SliceStable(rows, func(i, j int) bool {
		return fieldStringMap(rows[i], "stac_yymm") > fieldStringMap(rows[j], "stac_yymm")
	})
	return rows
}

func fieldStringMap(row map[string]any, key string) string {
	if row == nil {
		return ""
	}
	value, ok := row[key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(toString(value))
}

func deriveEffectiveTax(row map[string]any) (float64, bool) {
	preTaxIncome, ok := parseFloat(row["op_prfi"])
	if !ok || preTaxIncome <= 0 {
		return 0, false
	}

	netIncome, ok := parseFloat(row["thtr_ntin"])
	if !ok {
		return 0, false
	}

	tax := 1 - (netIncome / preTaxIncome)
	if tax < 0 {
		tax = 0
	}
	if tax > 1 {
		tax = 1
	}
	return tax, true
}

func deriveCapEx(balanceRows []map[string]any, incomeRow map[string]any) (float64, bool) {
	if len(balanceRows) < 2 {
		return 0, false
	}

	latestFixedAssets, ok := parseFloat(balanceRows[0]["fxas"])
	if !ok {
		return 0, false
	}
	prevFixedAssets, ok := parseFloat(balanceRows[1]["fxas"])
	if !ok {
		return 0, false
	}
	depreciation, ok := parseFloat(incomeRow["depr_cost"])
	if !ok {
		return 0, false
	}

	capex := (latestFixedAssets - prevFixedAssets) + depreciation
	if capex < 0 {
		capex = 0
	}
	return capex, true
}

func deriveChangeInNWC(balanceRows []map[string]any) (float64, bool) {
	if len(balanceRows) < 2 {
		return 0, false
	}

	latestCurrentAssets, ok := parseFloat(balanceRows[0]["cras"])
	if !ok {
		return 0, false
	}
	latestCurrentLiabilities, ok := parseFloat(balanceRows[0]["flow_lblt"])
	if !ok {
		return 0, false
	}
	prevCurrentAssets, ok := parseFloat(balanceRows[1]["cras"])
	if !ok {
		return 0, false
	}
	prevCurrentLiabilities, ok := parseFloat(balanceRows[1]["flow_lblt"])
	if !ok {
		return 0, false
	}

	latestNWC := latestCurrentAssets - latestCurrentLiabilities
	prevNWC := prevCurrentAssets - prevCurrentLiabilities
	return latestNWC - prevNWC, true
}

func deriveCapitalWeights(row map[string]any) (float64, float64, bool) {
	totalAssets, ok := parseFloat(row["total_aset"])
	if !ok || totalAssets <= 0 {
		return 0, 0, false
	}
	totalCapital, ok := parseFloat(row["total_cptl"])
	if !ok {
		return 0, 0, false
	}
	totalLiabilities, ok := parseFloat(row["total_lblt"])
	if !ok {
		return 0, 0, false
	}

	return totalCapital / totalAssets, totalLiabilities / totalAssets, true
}

func missingInputNames(inputs []DCFInputValue, names ...string) []string {
	lookup := make(map[string]DCFInputValue, len(inputs))
	for _, input := range inputs {
		lookup[input.Name] = input
	}

	missing := make([]string, 0, len(names))
	for _, name := range names {
		input, ok := lookup[name]
		if !ok || input.Status == DCFInputMissing {
			missing = append(missing, name)
		}
	}
	return missing
}
