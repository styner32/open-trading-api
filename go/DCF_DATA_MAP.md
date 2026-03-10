# DCF Data Map

## Current Go Coverage

The current Go implementation can now build a practical end-to-end DCF valuation from KIS domestic stock APIs plus a small assumption layer.

Implemented code:
- `go/internal/domesticstock/dcf_readiness.go`
- `go/internal/domesticstock/dcf_valuation.go`
- `go/internal/dcf/engine.go`
- `go/internal/dcf/advanced.go`
- `go/cmd/main.go`

## Input Mapping

| DCF field | Current status | Source | Notes |
| --- | --- | --- | --- |
| `Revenue` | exact | `income-statement.output.sale_account` | Latest annual revenue |
| `EBIT` | exact | `income-statement.output.bsop_prti` | Uses operating profit as EBIT proxy |
| `EffectiveTax` | derived | `income-statement.output.(op_prfi, thtr_ntin)` | `1 - thtr_ntin / op_prfi` |
| `DnA` | exact | `income-statement.output.depr_cost` | Depreciation field confirmed |
| `CapEx` | derived | `balance-sheet.output.fxas` + `income-statement.output.depr_cost` | Proxy: `delta fixed assets + depreciation` |
| `ChangeInNWC` | derived | `balance-sheet.output.(cras, flow_lblt)` | Proxy: delta of `(current assets - current liabilities)` |
| `SharesOut` | exact | `inquire-price.output.lstn_stcn` | Listed shares |
| `NetDebt` | derived | `other-major-ratios.output.(ebitda, ev_ebitda)` + `inquire-price.output.hts_avls` | Proxy: `EV - market cap`; fallback uses debt dependence |
| `RiskFreeRate` | exact / assumed | `comp-interest.output.(hts_kor_isnm, bond_mnrt_prpr)` or `DCF_RISK_FREE_RATE` | Prefers KIS domestic bond rate snapshot; can tighten row selection with `DCF_RISK_FREE_NAME_HINT` |
| `Beta` | derived / assumed | `inquire-daily-itemchartprice` + `inquire-index-daily-price` or `DCF_BETA` | Rolling beta from matched daily returns |
| `MarketPremium` | assumed | `DCF_MARKET_PREMIUM` | Defaults to `5.5%` if not overridden |
| `CostOfDebt` | derived / assumed | `income-statement.output.bsop_non_expn` + `stability-ratio.output.bram_depn` or `DCF_COST_OF_DEBT` | Proxy from non-operating expense over debt proxy; fallback uses risk-free + spread |
| `EquityWeight` | derived | market cap + net debt proxy | Prefers market-value capital structure |
| `DebtWeight` | derived | market cap + net debt proxy | Falls back to book-capital proxy |

## What This Means

- `FCF projection`: possible
- `WACC`: possible
- `Enterprise Value`: possible
- `Target Price`: possible
- `Reverse DCF`: possible
- `Monte Carlo`: possible

Important caveat:
- `NetDebt`, `CostOfDebt`, and `MarketPremium` are not fully clean accounting-market facts in KIS alone. They remain proxy / assumption inputs and must be read that way.

## API Calls Used By DCF Readiness / Valuation

Required company calls:
1. `/uapi/domestic-stock/v1/quotations/inquire-price`
2. `/uapi/domestic-stock/v1/finance/balance-sheet`
3. `/uapi/domestic-stock/v1/finance/income-statement`

Additional calls for market / proxy inputs:
4. `/uapi/domestic-stock/v1/finance/other-major-ratios`
5. `/uapi/domestic-stock/v1/finance/stability-ratio`
6. `/uapi/domestic-stock/v1/quotations/comp-interest`
7. `/uapi/domestic-stock/v1/quotations/inquire-daily-itemchartprice`
8. `/uapi/domestic-stock/v1/quotations/inquire-index-daily-price`

## Assumption / Override Environment Variables

- `DCF_RISK_FREE_RATE`
- `DCF_RISK_FREE_NAME_HINT`
- `DCF_BETA`
- `DCF_MARKET_PREMIUM`
- `DCF_COST_OF_DEBT`
- `DCF_NET_DEBT`
- `DCF_FORECAST_YEARS`
- `DCF_TERMINAL_GROWTH`
- `DCF_INDEX_CODE`
- `DCF_BETA_LOOKBACK_DAYS`
- `DCF_CREDIT_SPREAD`
- `DCF_MONTE_CARLO_ITERATIONS`
- `DCF_MONTE_CARLO_WORKERS`
- `DCF_MONTE_CARLO_GROWTH_STDDEV`
- `DCF_MONTE_CARLO_WACC_STDDEV`
- `DCF_MONTE_CARLO_TERMINAL_STDDEV`
