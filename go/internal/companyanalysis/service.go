package companyanalysis

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/kis-open-api/go/internal/dcf"
	"github.com/kis-open-api/go/internal/marketpremium"
)

const (
	defaultSECTickersURL       = "https://www.sec.gov/files/company_tickers.json"
	defaultSECCompanyFactsBase = "https://data.sec.gov/api/xbrl/companyfacts"
	defaultFRED10YearURL       = "https://fred.stlouisfed.org/graph/fredgraph.csv?id=DGS10"
	defaultStooqBaseURL        = "https://stooq.com/q/d/l/"
	defaultSECUserAgent        = "open-trading-api/1.0 contact@example.com"
	defaultBenchmarkSymbol     = "SPY"
	defaultForecastYears       = 5
	defaultTerminalGrowth      = 0.025
	defaultRiskFreeRate        = 0.04
	defaultMarketPremium       = 0.055
	defaultBetaLookbackDays    = 252
	defaultCostOfDebtSpread    = 0.015
	defaultPriceScale          = 1.0
	defaultPriceUnit           = "USD/share"
	minimumBetaObservations    = 60
	stooqInterval              = "d"
)

type InputStatus string

const (
	InputExact   InputStatus = "exact"
	InputDerived InputStatus = "derived"
	InputAssumed InputStatus = "assumed"
	InputMissing InputStatus = "missing"
)

type InputValue struct {
	Name     string      `json:"name"`
	Status   InputStatus `json:"status"`
	Value    float64     `json:"value,omitempty"`
	HasValue bool        `json:"has_value"`
	Source   string      `json:"source,omitempty"`
	Note     string      `json:"note,omitempty"`
}

type Config struct {
	SECTickersURL            string
	SECCompanyFactsBase      string
	FRED10YearURL            string
	StooqBaseURL             string
	SECUserAgent             string
	SECTickersCachePath      string
	SECCompanyFactsCachePath string
}

type AnalysisOptions struct {
	BenchmarkSymbol  string
	ForecastYears    int
	TerminalGrowth   float64
	PriceScale       float64
	PriceUnit        string
	BetaLookbackDays int
	RiskFreeRate     *float64
	Beta             *float64
	MarketPremium    *float64
	CostOfDebt       *float64
	NetDebt          *float64
}

type Quote struct {
	Symbol        string  `json:"symbol"`
	PriceDate     string  `json:"price_date"`
	Price         float64 `json:"price"`
	PreviousClose float64 `json:"previous_close"`
	Change        float64 `json:"change"`
	ChangePercent float64 `json:"change_percent"`
	Currency      string  `json:"currency"`
	Source        string  `json:"source"`
}

type AnnualRecord struct {
	FiscalYear             int     `json:"fiscal_year"`
	EndDate                string  `json:"end_date"`
	FiledDate              string  `json:"filed_date,omitempty"`
	Revenue                float64 `json:"revenue,omitempty"`
	EBIT                   float64 `json:"ebit,omitempty"`
	NetIncome              float64 `json:"net_income,omitempty"`
	TaxExpense             float64 `json:"tax_expense,omitempty"`
	EffectiveTax           float64 `json:"effective_tax,omitempty"`
	DnA                    float64 `json:"dna,omitempty"`
	CapEx                  float64 `json:"capex,omitempty"`
	ChangeInNWC            float64 `json:"change_in_nwc,omitempty"`
	CurrentAssets          float64 `json:"current_assets,omitempty"`
	CurrentLiabilities     float64 `json:"current_liabilities,omitempty"`
	TotalAssets            float64 `json:"total_assets,omitempty"`
	TotalDebt              float64 `json:"total_debt,omitempty"`
	Cash                   float64 `json:"cash,omitempty"`
	Equity                 float64 `json:"equity,omitempty"`
	SharesOutstanding      float64 `json:"shares_outstanding,omitempty"`
	InterestExpense        float64 `json:"interest_expense,omitempty"`
	PropertyPlantEquipment float64 `json:"property_plant_equipment,omitempty"`
}

type KeyMetrics struct {
	MarketCap       float64 `json:"market_cap"`
	EnterpriseValue float64 `json:"enterprise_value"`
	NetDebt         float64 `json:"net_debt"`
	RevenueGrowth   float64 `json:"revenue_growth"`
	OperatingMargin float64 `json:"operating_margin"`
	NetMargin       float64 `json:"net_margin"`
	ROE             float64 `json:"roe"`
	CurrentRatio    float64 `json:"current_ratio"`
	DebtToEquity    float64 `json:"debt_to_equity"`
	CashToDebt      float64 `json:"cash_to_debt"`
}

type Result struct {
	Symbol          string               `json:"symbol"`
	CompanyName     string               `json:"company_name"`
	CIK             string               `json:"cik"`
	BenchmarkSymbol string               `json:"benchmark_symbol"`
	Quote           Quote                `json:"quote"`
	Inputs          []InputValue         `json:"inputs"`
	Notes           []string             `json:"notes,omitempty"`
	Financials      []AnnualRecord       `json:"financials"`
	KeyMetrics      KeyMetrics           `json:"key_metrics"`
	Financial       dcf.FinancialData    `json:"financial"`
	Market          dcf.MarketData       `json:"market"`
	Assumptions     dcf.Assumptions      `json:"assumptions"`
	Projection      dcf.ProjectionModel  `json:"projection"`
	Valuation       *dcf.ValuationResult `json:"valuation,omitempty"`
}

type Service struct {
	httpClient *http.Client
	cfg        Config
}

type secTickerEntry struct {
	CIK    int    `json:"cik_str"`
	Ticker string `json:"ticker"`
	Title  string `json:"title"`
}

type secCompanyFactsResponse struct {
	CIK        int    `json:"cik"`
	EntityName string `json:"entityName"`
	Facts      struct {
		DEI    map[string]secConcept `json:"dei"`
		USGAAP map[string]secConcept `json:"us-gaap"`
	} `json:"facts"`
}

type secConcept struct {
	Units map[string][]secObservation `json:"units"`
}

type secObservation struct {
	Start string  `json:"start"`
	End   string  `json:"end"`
	Val   float64 `json:"val"`
	Accn  string  `json:"accn"`
	FY    int     `json:"fy"`
	FP    string  `json:"fp"`
	Form  string  `json:"form"`
	Filed string  `json:"filed"`
	Frame string  `json:"frame"`
}

type stooqRow struct {
	Date  string
	Close float64
}

type rankedObservation struct {
	Observation secObservation
	Priority    int
}

func NewService(httpClient *http.Client, cfg Config) *Service {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	if strings.TrimSpace(cfg.SECTickersURL) == "" {
		cfg.SECTickersURL = defaultSECTickersURL
	}
	if strings.TrimSpace(cfg.SECCompanyFactsBase) == "" {
		cfg.SECCompanyFactsBase = defaultSECCompanyFactsBase
	}
	if strings.TrimSpace(cfg.FRED10YearURL) == "" {
		cfg.FRED10YearURL = defaultFRED10YearURL
	}
	if strings.TrimSpace(cfg.StooqBaseURL) == "" {
		cfg.StooqBaseURL = defaultStooqBaseURL
	}
	if strings.TrimSpace(cfg.SECUserAgent) == "" {
		cfg.SECUserAgent = defaultSECUserAgent
	}
	return &Service{httpClient: httpClient, cfg: cfg}
}

func (s *Service) Analyze(ctx context.Context, symbol string, options AnalysisOptions) (*Result, error) {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol == "" {
		return nil, fmt.Errorf("symbol is required")
	}
	options = resolveOptions(options)

	stockHistory, err := s.fetchStooqHistory(ctx, symbol)
	if err != nil {
		return nil, fmt.Errorf("stock history failed: %w", err)
	}
	quote, err := buildQuoteFromHistory(symbol, stockHistory)
	if err != nil {
		return nil, err
	}

	cik, companyName, err := s.resolveSECTicker(ctx, symbol)
	if err != nil {
		return nil, err
	}

	facts, err := s.fetchCompanyFacts(ctx, cik)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(companyName) == "" {
		companyName = strings.TrimSpace(facts.EntityName)
	}

	financials, latestShares, notes, err := s.buildAnnualRecords(facts)
	if err != nil {
		return nil, err
	}
	if len(financials) < 2 {
		return nil, fmt.Errorf("not enough annual financial history for %s", symbol)
	}
	latest := financials[0]

	inputs := make([]InputValue, 0, 16)
	if latest.Revenue <= 0 || latest.EBIT == 0 {
		return nil, fmt.Errorf("required SEC financial metrics missing for %s", symbol)
	}

	fin := dcf.FinancialData{
		Revenue: latest.Revenue,
		EBIT:    latest.EBIT,
		DnA:     latest.DnA,
		CapEx:   latest.CapEx,
	}
	inputs = append(inputs,
		InputValue{Name: "Revenue", Status: InputExact, Value: latest.Revenue, HasValue: true, Source: "sec.companyfacts"},
		InputValue{Name: "EBIT", Status: InputExact, Value: latest.EBIT, HasValue: true, Source: "sec.companyfacts"},
		InputValue{Name: "DnA", Status: inputStatusForValue(latest.DnA), Value: latest.DnA, HasValue: latest.DnA != 0, Source: "sec.companyfacts"},
		InputValue{Name: "CapEx", Status: inputStatusForValue(latest.CapEx), Value: latest.CapEx, HasValue: latest.CapEx != 0, Source: "sec.companyfacts / derived from PPE"},
	)

	if latest.EffectiveTax > 0 {
		fin.EffectiveTax = latest.EffectiveTax
		inputs = append(inputs, InputValue{Name: "EffectiveTax", Status: InputExact, Value: latest.EffectiveTax, HasValue: true, Source: "sec.companyfacts"})
	} else {
		fin.EffectiveTax = 0.21
		inputs = append(inputs, InputValue{Name: "EffectiveTax", Status: InputAssumed, Value: fin.EffectiveTax, HasValue: true, Source: "default", Note: "fallback assumption"})
	}

	if latest.ChangeInNWC != 0 {
		fin.ChangeInNWC = latest.ChangeInNWC
		inputs = append(inputs, InputValue{Name: "ChangeInNWC", Status: InputDerived, Value: latest.ChangeInNWC, HasValue: true, Source: "sec.companyfacts current assets/current liabilities"})
	} else {
		inputs = append(inputs, InputValue{Name: "ChangeInNWC", Status: InputMissing, Source: "sec.companyfacts", Note: "could not derive from annual balance-sheet history"})
	}

	if latestShares > 0 {
		fin.SharesOut = latestShares
		inputs = append(inputs, InputValue{Name: "SharesOut", Status: InputExact, Value: latestShares, HasValue: true, Source: "sec.companyfacts dei"})
	} else if latest.SharesOutstanding > 0 {
		fin.SharesOut = latest.SharesOutstanding
		inputs = append(inputs, InputValue{Name: "SharesOut", Status: InputExact, Value: latest.SharesOutstanding, HasValue: true, Source: "sec.companyfacts dei"})
	} else {
		return nil, fmt.Errorf("shares outstanding missing for %s", symbol)
	}

	netDebt := latest.TotalDebt - latest.Cash
	if options.NetDebt != nil {
		netDebt = *options.NetDebt
		inputs = append(inputs, InputValue{Name: "NetDebt", Status: InputAssumed, Value: netDebt, HasValue: true, Source: "env override"})
	} else {
		inputs = append(inputs, InputValue{Name: "NetDebt", Status: InputDerived, Value: netDebt, HasValue: true, Source: "sec.companyfacts debt - cash"})
	}
	fin.NetDebt = netDebt

	market := dcf.MarketData{}
	if options.RiskFreeRate != nil {
		market.RiskFreeRate = *options.RiskFreeRate
		inputs = append(inputs, InputValue{Name: "RiskFreeRate", Status: InputAssumed, Value: market.RiskFreeRate, HasValue: true, Source: "env override"})
	} else if value, ok := s.fetchRiskFreeRate(ctx); ok {
		market.RiskFreeRate = value
		inputs = append(inputs, InputValue{Name: "RiskFreeRate", Status: InputExact, Value: value, HasValue: true, Source: "fred.DGS10"})
	} else {
		market.RiskFreeRate = defaultRiskFreeRate
		inputs = append(inputs, InputValue{Name: "RiskFreeRate", Status: InputAssumed, Value: market.RiskFreeRate, HasValue: true, Source: "default", Note: "fallback assumption"})
		notes = append(notes, "risk free rate fallback used because FRED fetch failed")
	}

	if options.MarketPremium != nil {
		market.MarketPremium = *options.MarketPremium
		inputs = append(inputs, InputValue{Name: "MarketPremium", Status: InputAssumed, Value: market.MarketPremium, HasValue: true, Source: "env override"})
	} else if value, note, ok := s.resolveMarketPremium(ctx); ok {
		market.MarketPremium = value
		inputs = append(inputs, InputValue{Name: "MarketPremium", Status: InputExact, Value: value, HasValue: true, Source: "external-market-premium.damodaran", Note: note})
	} else {
		market.MarketPremium = defaultMarketPremium
		inputs = append(inputs, InputValue{Name: "MarketPremium", Status: InputAssumed, Value: market.MarketPremium, HasValue: true, Source: "default", Note: "fallback assumption"})
		notes = append(notes, "market premium fallback used because Damodaran provider was unavailable")
	}

	if options.Beta != nil {
		market.Beta = *options.Beta
		inputs = append(inputs, InputValue{Name: "Beta", Status: InputAssumed, Value: market.Beta, HasValue: true, Source: "env override"})
	} else {
		benchmarkHistory, benchErr := s.fetchStooqHistory(ctx, options.BenchmarkSymbol)
		if benchErr != nil {
			market.Beta = 1.0
			inputs = append(inputs, InputValue{Name: "Beta", Status: InputAssumed, Value: market.Beta, HasValue: true, Source: "default", Note: "benchmark history unavailable"})
			notes = append(notes, "beta fallback used because benchmark history fetch failed: "+benchErr.Error())
		} else if beta, observations, ok := deriveBetaFromHistories(stockHistory, benchmarkHistory, options.BetaLookbackDays); ok {
			market.Beta = beta
			inputs = append(inputs, InputValue{Name: "Beta", Status: InputDerived, Value: beta, HasValue: true, Source: "stooq stock/benchmark daily history", Note: fmt.Sprintf("derived from %d matched daily returns", observations)})
		} else {
			market.Beta = 1.0
			inputs = append(inputs, InputValue{Name: "Beta", Status: InputAssumed, Value: market.Beta, HasValue: true, Source: "default", Note: "not enough matched returns"})
			notes = append(notes, "beta fallback used because there were not enough matched daily returns")
		}
	}

	if options.CostOfDebt != nil {
		market.CostOfDebt = *options.CostOfDebt
		inputs = append(inputs, InputValue{Name: "CostOfDebt", Status: InputAssumed, Value: market.CostOfDebt, HasValue: true, Source: "env override"})
	} else if costOfDebt, ok := deriveCostOfDebt(latest, financials); ok {
		if costOfDebt < market.RiskFreeRate {
			costOfDebt = market.RiskFreeRate
		}
		market.CostOfDebt = costOfDebt
		inputs = append(inputs, InputValue{Name: "CostOfDebt", Status: InputDerived, Value: costOfDebt, HasValue: true, Source: "sec.companyfacts interest expense / average debt"})
	} else {
		market.CostOfDebt = market.RiskFreeRate + defaultCostOfDebtSpread
		inputs = append(inputs, InputValue{Name: "CostOfDebt", Status: InputAssumed, Value: market.CostOfDebt, HasValue: true, Source: "risk free + spread", Note: fmt.Sprintf("fallback spread %.2f%%", defaultCostOfDebtSpread*100)})
	}

	marketCap := quote.Price * fin.SharesOut
	totalDebtForWeights := latest.TotalDebt
	if totalDebtForWeights < 0 {
		totalDebtForWeights = 0
	}
	totalCapital := marketCap + totalDebtForWeights
	if totalCapital <= 0 {
		return nil, fmt.Errorf("capital structure could not be derived for %s", symbol)
	}
	market.EquityWeight = marketCap / totalCapital
	market.DebtWeight = totalDebtForWeights / totalCapital
	inputs = append(inputs,
		InputValue{Name: "EquityWeight", Status: InputDerived, Value: market.EquityWeight, HasValue: true, Source: "market cap / (market cap + debt)"},
		InputValue{Name: "DebtWeight", Status: InputDerived, Value: market.DebtWeight, HasValue: true, Source: "debt / (market cap + debt)"},
	)

	assumptions := dcf.Assumptions{
		ForecastYears:    options.ForecastYears,
		TerminalGrowth:   options.TerminalGrowth,
		TargetPriceScale: options.PriceScale,
		TargetPriceUnit:  options.PriceUnit,
	}
	projection := deriveProjectionModel(financials)
	valuation, err := dcf.Value(fin, market, assumptions, projection)
	if err != nil {
		return nil, err
	}

	return &Result{
		Symbol:          symbol,
		CompanyName:     companyName,
		CIK:             cik,
		BenchmarkSymbol: options.BenchmarkSymbol,
		Quote:           quote,
		Inputs:          inputs,
		Notes:           notes,
		Financials:      financials,
		KeyMetrics:      buildKeyMetrics(quote, latest, marketCap, fin.NetDebt, projection.RevenueGrowth),
		Financial:       fin,
		Market:          market,
		Assumptions:     assumptions,
		Projection:      projection,
		Valuation:       valuation,
	}, nil
}

func resolveOptions(options AnalysisOptions) AnalysisOptions {
	if strings.TrimSpace(options.BenchmarkSymbol) == "" {
		options.BenchmarkSymbol = defaultBenchmarkSymbol
	}
	if options.ForecastYears <= 0 {
		options.ForecastYears = defaultForecastYears
	}
	if options.TerminalGrowth == 0 {
		options.TerminalGrowth = defaultTerminalGrowth
	}
	if options.PriceScale == 0 {
		options.PriceScale = defaultPriceScale
	}
	if strings.TrimSpace(options.PriceUnit) == "" {
		options.PriceUnit = defaultPriceUnit
	}
	if options.BetaLookbackDays <= 0 {
		options.BetaLookbackDays = defaultBetaLookbackDays
	}
	return options
}

func (s *Service) resolveSECTicker(ctx context.Context, symbol string) (string, string, error) {
	raw, err := s.fetchSECJSON(ctx, s.cfg.SECTickersURL, strings.TrimSpace(s.cfg.SECTickersCachePath))
	if err != nil {
		return "", "", err
	}
	var payload map[string]secTickerEntry
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", "", fmt.Errorf("failed to decode SEC ticker list: %w", err)
	}

	for _, entry := range payload {
		if strings.EqualFold(strings.TrimSpace(entry.Ticker), symbol) {
			return fmt.Sprintf("%010d", entry.CIK), strings.TrimSpace(entry.Title), nil
		}
	}
	return "", "", fmt.Errorf("SEC ticker mapping not found for %s", symbol)
}

func (s *Service) fetchCompanyFacts(ctx context.Context, cik string) (*secCompanyFactsResponse, error) {
	url := strings.TrimRight(s.cfg.SECCompanyFactsBase, "/") + "/CIK" + strings.TrimSpace(cik) + ".json"
	raw, err := s.fetchSECJSON(ctx, url, resolveCompanyFactsCachePath(s.cfg.SECCompanyFactsCachePath, cik))
	if err != nil {
		return nil, err
	}
	var payload secCompanyFactsResponse
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("failed to decode SEC company facts: %w", err)
	}
	return &payload, nil
}

func (s *Service) fetchSECJSON(ctx context.Context, url string, cachePath string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", s.cfg.SECUserAgent)

	resp, err := s.httpClient.Do(req)
	if err == nil {
		defer resp.Body.Close()
		raw, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			err = readErr
		} else if resp.StatusCode == http.StatusOK && json.Valid(raw) && !isSECFairAccessBody(raw) {
			writeCacheFile(cachePath, raw)
			return raw, nil
		} else if cached, ok := readCacheFile(cachePath); ok {
			return cached, nil
		} else if isSECFairAccessBody(raw) {
			return nil, fmt.Errorf("SEC blocked automated access. Set COMPANY_ANALYSIS_SEC_USER_AGENT to a descriptive identifier with contact email and retry")
		} else if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("SEC request failed with status %d", resp.StatusCode)
		} else {
			return nil, fmt.Errorf("SEC response was not valid JSON")
		}
	}

	if cached, ok := readCacheFile(cachePath); ok {
		return cached, nil
	}
	if err != nil {
		return nil, fmt.Errorf("SEC request failed: %w", err)
	}
	return nil, fmt.Errorf("SEC request failed")
}

func (s *Service) buildAnnualRecords(facts *secCompanyFactsResponse) ([]AnnualRecord, float64, []string, error) {
	if facts == nil {
		return nil, 0, nil, fmt.Errorf("company facts are required")
	}

	revenueByFY := selectAnnualByFY(facts.Facts.USGAAP, []string{
		"RevenueFromContractWithCustomerExcludingAssessedTax",
		"Revenues",
		"SalesRevenueNet",
	}, "USD", true)
	if len(revenueByFY) == 0 {
		return nil, 0, nil, fmt.Errorf("annual revenue series missing")
	}

	ebitByFY := selectAnnualByFY(facts.Facts.USGAAP, []string{"OperatingIncomeLoss"}, "USD", true)
	netIncomeByFY := selectAnnualByFY(facts.Facts.USGAAP, []string{"NetIncomeLoss"}, "USD", true)
	taxExpenseByFY := selectAnnualByFY(facts.Facts.USGAAP, []string{"IncomeTaxExpenseBenefit"}, "USD", true)
	effectiveTaxByFY := selectAnnualByFY(facts.Facts.USGAAP, []string{"EffectiveIncomeTaxRateContinuingOperations"}, "pure", true)
	dnaByFY := selectAnnualByFY(facts.Facts.USGAAP, []string{
		"DepreciationDepletionAndAmortization",
		"DepreciationAndAmortization",
		"Depreciation",
	}, "USD", true)
	directCapExByFY := selectAnnualByFY(facts.Facts.USGAAP, []string{"PaymentsToAcquirePropertyPlantAndEquipment"}, "USD", true)
	currentAssetsByFY := selectAnnualByFY(facts.Facts.USGAAP, []string{"AssetsCurrent"}, "USD", false)
	currentLiabilitiesByFY := selectAnnualByFY(facts.Facts.USGAAP, []string{"LiabilitiesCurrent"}, "USD", false)
	totalAssetsByFY := selectAnnualByFY(facts.Facts.USGAAP, []string{"Assets"}, "USD", false)
	cashByFY := selectAnnualByFY(facts.Facts.USGAAP, []string{"CashAndCashEquivalentsAtCarryingValue"}, "USD", false)
	longTermDebtByFY := selectAnnualByFY(facts.Facts.USGAAP, []string{"LongTermDebt", "LongTermDebtNoncurrent"}, "USD", false)
	currentDebtByFY := selectAnnualByFY(facts.Facts.USGAAP, []string{"LongTermDebtCurrent", "DebtCurrent", "ConvertibleDebtCurrent"}, "USD", false)
	equityByFY := selectAnnualByFY(facts.Facts.USGAAP, []string{"StockholdersEquity"}, "USD", false)
	interestExpenseByFY := selectAnnualByFY(facts.Facts.USGAAP, []string{"InterestExpense", "InterestExpenseDebt", "InterestExpenseNonoperating"}, "USD", true)
	ppeByFY := selectAnnualByFY(facts.Facts.USGAAP, []string{"PropertyPlantAndEquipmentNet"}, "USD", false)
	sharesByFY := selectAnnualByFY(facts.Facts.DEI, []string{"EntityCommonStockSharesOutstanding"}, "shares", false)
	latestShares, sharesOK := selectLatestObservation(facts.Facts.DEI, []string{"EntityCommonStockSharesOutstanding"}, "shares", false)

	years := make([]int, 0, len(revenueByFY))
	for year := range revenueByFY {
		years = append(years, year)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(years)))

	financials := make([]AnnualRecord, 0, len(years))
	for _, year := range years {
		revenueObs := revenueByFY[year]
		record := AnnualRecord{
			FiscalYear: year,
			EndDate:    revenueObs.End,
			FiledDate:  revenueObs.Filed,
			Revenue:    revenueObs.Val,
		}
		if obs, ok := ebitByFY[year]; ok {
			record.EBIT = obs.Val
		}
		if obs, ok := netIncomeByFY[year]; ok {
			record.NetIncome = obs.Val
		}
		if obs, ok := taxExpenseByFY[year]; ok {
			record.TaxExpense = obs.Val
		}
		if obs, ok := effectiveTaxByFY[year]; ok {
			record.EffectiveTax = obs.Val
		}
		if obs, ok := dnaByFY[year]; ok {
			record.DnA = math.Abs(obs.Val)
		}
		if obs, ok := directCapExByFY[year]; ok {
			record.CapEx = math.Abs(obs.Val)
		}
		if obs, ok := currentAssetsByFY[year]; ok {
			record.CurrentAssets = obs.Val
		}
		if obs, ok := currentLiabilitiesByFY[year]; ok {
			record.CurrentLiabilities = obs.Val
		}
		if obs, ok := totalAssetsByFY[year]; ok {
			record.TotalAssets = obs.Val
		}
		if obs, ok := cashByFY[year]; ok {
			record.Cash = obs.Val
		}
		if obs, ok := longTermDebtByFY[year]; ok {
			record.TotalDebt += obs.Val
		}
		if obs, ok := currentDebtByFY[year]; ok {
			record.TotalDebt += obs.Val
		}
		if obs, ok := equityByFY[year]; ok {
			record.Equity = obs.Val
		}
		if obs, ok := interestExpenseByFY[year]; ok {
			record.InterestExpense = math.Abs(obs.Val)
		}
		if obs, ok := ppeByFY[year]; ok {
			record.PropertyPlantEquipment = obs.Val
		}
		if obs, ok := sharesByFY[year]; ok {
			record.SharesOutstanding = obs.Val
		}
		if record.EffectiveTax == 0 {
			if derivedTax, ok := deriveEffectiveTax(record); ok {
				record.EffectiveTax = derivedTax
			}
		}
		financials = append(financials, record)
	}

	for i := 0; i < len(financials); i++ {
		if financials[i].CapEx == 0 && i+1 < len(financials) {
			deltaPPE := financials[i].PropertyPlantEquipment - financials[i+1].PropertyPlantEquipment
			capex := deltaPPE + financials[i].DnA
			if capex > 0 {
				financials[i].CapEx = capex
			}
		}
		if i+1 < len(financials) {
			currentNWC := financials[i].CurrentAssets - financials[i].CurrentLiabilities
			prevNWC := financials[i+1].CurrentAssets - financials[i+1].CurrentLiabilities
			financials[i].ChangeInNWC = currentNWC - prevNWC
		}
	}

	notes := make([]string, 0, 2)
	if !sharesOK {
		notes = append(notes, "latest SEC shares outstanding observation missing; annual FY figure used when available")
	}
	var latestShareValue float64
	if sharesOK {
		latestShareValue = latestShares.Val
	}
	return financials, latestShareValue, notes, nil
}

func selectAnnualByFY(concepts map[string]secConcept, tags []string, unit string, duration bool) map[int]secObservation {
	selected := make(map[int]rankedObservation)
	for priority, tag := range tags {
		concept, ok := concepts[tag]
		if !ok {
			continue
		}
		for _, observation := range concept.Units[unit] {
			if !isAnnualObservation(observation, duration) {
				continue
			}
			current, exists := selected[observation.FY]
			if !exists || priority < current.Priority || (priority == current.Priority && filedLater(observation, current.Observation)) {
				selected[observation.FY] = rankedObservation{Observation: observation, Priority: priority}
			}
		}
	}

	result := make(map[int]secObservation, len(selected))
	for year, ranked := range selected {
		result[year] = ranked.Observation
	}
	return result
}

func selectLatestObservation(concepts map[string]secConcept, tags []string, unit string, annualOnly bool) (secObservation, bool) {
	var best secObservation
	found := false
	for _, tag := range tags {
		concept, ok := concepts[tag]
		if !ok {
			continue
		}
		for _, observation := range concept.Units[unit] {
			if annualOnly && !isAnnualForm(observation.Form) {
				continue
			}
			if strings.TrimSpace(observation.End) == "" {
				continue
			}
			if !found || latestObservation(observation, best) {
				best = observation
				found = true
			}
		}
	}
	return best, found
}

func isAnnualObservation(observation secObservation, duration bool) bool {
	if observation.FY == 0 || !isAnnualForm(observation.Form) {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(observation.FP), "FY") {
		return true
	}
	if !duration {
		return strings.TrimSpace(observation.End) != ""
	}
	if observation.Start == "" || observation.End == "" {
		return false
	}
	start, err := time.Parse("2006-01-02", observation.Start)
	if err != nil {
		return false
	}
	end, err := time.Parse("2006-01-02", observation.End)
	if err != nil {
		return false
	}
	days := end.Sub(start).Hours() / 24
	return days >= 300 && days <= 380
}

func isAnnualForm(form string) bool {
	switch strings.ToUpper(strings.TrimSpace(form)) {
	case "10-K", "10-K/A", "20-F", "20-F/A", "40-F", "40-F/A":
		return true
	default:
		return false
	}
}

func filedLater(left secObservation, right secObservation) bool {
	if left.Filed == right.Filed {
		return left.End > right.End
	}
	return left.Filed > right.Filed
}

func latestObservation(left secObservation, right secObservation) bool {
	if left.End == right.End {
		return left.Filed > right.Filed
	}
	return left.End > right.End
}

func isSECFairAccessBody(raw []byte) bool {
	body := strings.ToLower(strings.TrimSpace(string(raw)))
	return strings.Contains(body, "automated access to our sites") &&
		strings.Contains(body, "privacy and security policy")
}

func readCacheFile(path string) ([]byte, bool) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, false
	}
	raw, err := os.ReadFile(path)
	if err != nil || len(raw) == 0 {
		return nil, false
	}
	return raw, true
}

func writeCacheFile(path string, raw []byte) {
	path = strings.TrimSpace(path)
	if path == "" || len(raw) == 0 {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, raw, 0o600); err != nil {
		return
	}
	_ = os.Rename(tmpPath, path)
}

func resolveCompanyFactsCachePath(basePath string, cik string) string {
	basePath = strings.TrimSpace(basePath)
	if basePath == "" {
		return ""
	}
	dir := filepath.Dir(basePath)
	filename := filepath.Base(basePath)
	ext := filepath.Ext(filename)
	stem := filename
	if ext != "" {
		stem = strings.TrimSuffix(filename, ext)
	}
	if ext == "" {
		ext = ".json"
	}
	cik = strings.TrimSpace(cik)
	if cik != "" {
		stem += "." + cik
	}
	return filepath.Join(dir, stem+ext)
}

func deriveEffectiveTax(record AnnualRecord) (float64, bool) {
	preTaxIncome := record.NetIncome + record.TaxExpense
	if preTaxIncome <= 0 {
		return 0, false
	}
	tax := record.TaxExpense / preTaxIncome
	if tax < 0 {
		tax = 0
	}
	if tax > 1 {
		tax = 1
	}
	return tax, true
}

func (s *Service) fetchRiskFreeRate(ctx context.Context) (float64, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.cfg.FRED10YearURL, nil)
	if err != nil {
		return 0, false
	}
	req.Header.Set("Accept", "text/csv")
	req.Header.Set("User-Agent", s.cfg.SECUserAgent)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return 0, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, false
	}

	reader := csv.NewReader(resp.Body)
	_, err = reader.Read()
	if err != nil {
		return 0, false
	}

	var last float64
	found := false
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil || len(record) < 2 {
			return 0, false
		}
		value := strings.TrimSpace(record[1])
		if value == "" || value == "." {
			continue
		}
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			continue
		}
		last = parsed / 100
		found = true
	}
	return last, found
}

func (s *Service) resolveMarketPremium(ctx context.Context) (float64, string, bool) {
	value, err := marketpremium.Resolver{HTTPClient: s.httpClient}.Resolve(ctx)
	if err != nil || value == nil || value.Rate <= 0 {
		return 0, "", false
	}
	note := strings.TrimSpace(value.Note)
	if value.AsOf != "" {
		if note == "" {
			note = "as of " + value.AsOf
		} else {
			note = note + " | as of " + value.AsOf
		}
	}
	return value.Rate, note, true
}

func (s *Service) fetchStooqHistory(ctx context.Context, symbol string) ([]stooqRow, error) {
	url := strings.TrimRight(s.cfg.StooqBaseURL, "/") + "/?s=" + stooqSymbol(symbol) + "&i=" + stooqInterval
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/csv")
	req.Header.Set("User-Agent", s.cfg.SECUserAgent)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("stooq status %d", resp.StatusCode)
	}

	reader := csv.NewReader(resp.Body)
	headers, err := reader.Read()
	if err != nil {
		return nil, err
	}
	if len(headers) < 5 {
		return nil, fmt.Errorf("stooq CSV header is incomplete")
	}

	rows := make([]stooqRow, 0, 512)
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if len(record) < 5 {
			continue
		}
		closeValue, err := strconv.ParseFloat(strings.TrimSpace(record[4]), 64)
		if err != nil || closeValue <= 0 {
			continue
		}
		rows = append(rows, stooqRow{
			Date:  strings.TrimSpace(record[0]),
			Close: closeValue,
		})
	}
	if len(rows) < 2 {
		return nil, fmt.Errorf("stooq returned too few rows for %s", symbol)
	}
	return rows, nil
}

func stooqSymbol(symbol string) string {
	symbol = strings.ToLower(strings.TrimSpace(symbol))
	if strings.Contains(symbol, ".") {
		return symbol
	}
	return symbol + ".us"
}

func buildQuoteFromHistory(symbol string, rows []stooqRow) (Quote, error) {
	if len(rows) < 2 {
		return Quote{}, fmt.Errorf("stooq history for %s is too short", symbol)
	}
	last := rows[len(rows)-1]
	prev := rows[len(rows)-2]
	change := last.Close - prev.Close
	changePercent := 0.0
	if prev.Close > 0 {
		changePercent = (change / prev.Close) * 100
	}
	return Quote{
		Symbol:        symbol,
		PriceDate:     last.Date,
		Price:         last.Close,
		PreviousClose: prev.Close,
		Change:        change,
		ChangePercent: changePercent,
		Currency:      "USD",
		Source:        "stooq daily close",
	}, nil
}

func deriveBetaFromHistories(stockRows []stooqRow, benchmarkRows []stooqRow, lookbackDays int) (float64, int, bool) {
	stockReturns := dailyReturnSeries(stockRows)
	benchmarkReturns := dailyReturnSeries(benchmarkRows)
	if len(stockReturns) == 0 || len(benchmarkReturns) == 0 {
		return 0, 0, false
	}

	dates := make([]string, 0, minInt(len(stockReturns), len(benchmarkReturns)))
	for date := range stockReturns {
		if _, ok := benchmarkReturns[date]; ok {
			dates = append(dates, date)
		}
	}
	sort.Strings(dates)
	if len(dates) > lookbackDays {
		dates = dates[len(dates)-lookbackDays:]
	}

	minObservations := minimumBetaObservations
	if lookbackDays > 0 && lookbackDays < minObservations {
		minObservations = lookbackDays
	}
	if minObservations < 30 {
		minObservations = 30
	}
	if len(dates) < minObservations {
		return 0, len(dates), false
	}

	stock := make([]float64, 0, len(dates))
	benchmark := make([]float64, 0, len(dates))
	for _, date := range dates {
		stock = append(stock, stockReturns[date])
		benchmark = append(benchmark, benchmarkReturns[date])
	}

	meanStock := averageFloatSlice(stock)
	meanBenchmark := averageFloatSlice(benchmark)
	var covariance float64
	var variance float64
	for i := range stock {
		stockDelta := stock[i] - meanStock
		benchmarkDelta := benchmark[i] - meanBenchmark
		covariance += stockDelta * benchmarkDelta
		variance += benchmarkDelta * benchmarkDelta
	}
	if variance == 0 {
		return 0, len(dates), false
	}
	beta := covariance / variance
	if math.IsNaN(beta) || math.IsInf(beta, 0) {
		return 0, len(dates), false
	}
	return beta, len(dates), true
}

func dailyReturnSeries(rows []stooqRow) map[string]float64 {
	if len(rows) < 2 {
		return nil
	}
	returns := make(map[string]float64, len(rows)-1)
	for i := 1; i < len(rows); i++ {
		prev := rows[i-1].Close
		if prev <= 0 {
			continue
		}
		returns[rows[i].Date] = (rows[i].Close / prev) - 1
	}
	return returns
}

func deriveCostOfDebt(latest AnnualRecord, financials []AnnualRecord) (float64, bool) {
	if latest.InterestExpense <= 0 || latest.TotalDebt <= 0 {
		return 0, false
	}
	avgDebt := latest.TotalDebt
	if len(financials) > 1 && financials[1].TotalDebt > 0 {
		avgDebt = (latest.TotalDebt + financials[1].TotalDebt) / 2
	}
	if avgDebt <= 0 {
		return 0, false
	}
	costOfDebt := latest.InterestExpense / avgDebt
	if costOfDebt <= 0 || costOfDebt > 0.30 {
		return 0, false
	}
	return costOfDebt, true
}

func deriveProjectionModel(financials []AnnualRecord) dcf.ProjectionModel {
	model := dcf.ProjectionModel{
		RevenueGrowth: 0.05,
		EBITMargin:    0.10,
		DNAMargin:     0.03,
		CapExMargin:   0.04,
		NWCMargin:     0.01,
	}

	if growth, ok := deriveRevenueGrowth(financials); ok {
		model.RevenueGrowth = growth
	}
	if margin, ok := averageMargin(financials, func(record AnnualRecord) float64 { return record.EBIT }); ok {
		model.EBITMargin = margin
	}
	if margin, ok := averageMargin(financials, func(record AnnualRecord) float64 { return record.DnA }); ok {
		model.DNAMargin = math.Max(margin, 0)
	}
	if margin, ok := averageMargin(financials, func(record AnnualRecord) float64 { return record.CapEx }); ok {
		model.CapExMargin = math.Max(margin, 0)
	}
	if margin, ok := averageMargin(financials, func(record AnnualRecord) float64 { return record.ChangeInNWC }); ok {
		model.NWCMargin = margin
	}
	return model
}

func deriveRevenueGrowth(financials []AnnualRecord) (float64, bool) {
	values := make([]float64, 0, len(financials))
	for _, record := range financials {
		if record.Revenue <= 0 {
			continue
		}
		values = append(values, record.Revenue)
		if len(values) >= 5 {
			break
		}
	}
	if len(values) < 2 {
		return 0, false
	}
	latest := values[0]
	oldest := values[len(values)-1]
	years := float64(len(values) - 1)
	if oldest <= 0 || years <= 0 {
		return 0, false
	}
	growth := math.Pow(latest/oldest, 1/years) - 1
	if math.IsNaN(growth) || math.IsInf(growth, 0) {
		return 0, false
	}
	return growth, true
}

func averageMargin(financials []AnnualRecord, numerator func(AnnualRecord) float64) (float64, bool) {
	var sum float64
	var count int
	for _, record := range financials {
		if record.Revenue <= 0 {
			continue
		}
		sum += numerator(record) / record.Revenue
		count++
		if count >= 4 {
			break
		}
	}
	if count == 0 {
		return 0, false
	}
	return sum / float64(count), true
}

func buildKeyMetrics(quote Quote, latest AnnualRecord, marketCap float64, netDebt float64, revenueGrowth float64) KeyMetrics {
	enterpriseValue := marketCap + latest.TotalDebt - latest.Cash
	currentRatio := 0.0
	if latest.CurrentLiabilities > 0 {
		currentRatio = latest.CurrentAssets / latest.CurrentLiabilities
	}
	debtToEquity := 0.0
	if latest.Equity > 0 {
		debtToEquity = latest.TotalDebt / latest.Equity
	}
	cashToDebt := 0.0
	if latest.TotalDebt > 0 {
		cashToDebt = latest.Cash / latest.TotalDebt
	}
	operatingMargin := 0.0
	netMargin := 0.0
	roe := 0.0
	if latest.Revenue > 0 {
		operatingMargin = latest.EBIT / latest.Revenue
		netMargin = latest.NetIncome / latest.Revenue
	}
	if latest.Equity > 0 {
		roe = latest.NetIncome / latest.Equity
	}
	return KeyMetrics{
		MarketCap:       marketCap,
		EnterpriseValue: enterpriseValue,
		NetDebt:         netDebt,
		RevenueGrowth:   revenueGrowth,
		OperatingMargin: operatingMargin,
		NetMargin:       netMargin,
		ROE:             roe,
		CurrentRatio:    currentRatio,
		DebtToEquity:    debtToEquity,
		CashToDebt:      cashToDebt,
	}
}

func inputStatusForValue(value float64) InputStatus {
	if value == 0 {
		return InputMissing
	}
	return InputExact
}

func averageFloatSlice(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sum float64
	for _, value := range values {
		sum += value
	}
	return sum / float64(len(values))
}

func minInt(left int, right int) int {
	if left < right {
		return left
	}
	return right
}
