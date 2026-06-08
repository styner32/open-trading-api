package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/kis-open-api/go/internal/auth"
	"github.com/kis-open-api/go/internal/domesticstock"
)

// CreditBalanceReport is the per-stock credit balance JSON output.
type CreditBalanceReport struct {
	Symbol      string             `json:"symbol"`
	Name        string             `json:"name"`
	GeneratedAt time.Time          `json:"generated_at"`
	Price       PriceInfo          `json:"price"`
	Summary     CreditSummary      `json:"summary"`
	Daily       []CreditBalanceDay `json:"daily"`
}

type PriceInfo struct {
	CurrentPrice  int64   `json:"current_price"`
	PreviousClose int64   `json:"previous_close"`
	ChangePercent float64 `json:"change_percent"`
	Volume        int64   `json:"volume"`
}

type CreditSummary struct {
	BalanceQty      int64   `json:"balance_qty"`
	BalanceAmtEok   float64 `json:"balance_amt_eok"`
	CreditRatePct   float64 `json:"credit_rate_pct"`
	Trend5D         string  `json:"trend_5d"`
}

type CreditBalanceDay struct {
	Date        string `json:"date"`
	NewQty      int64  `json:"new_qty"`
	RepayQty    int64  `json:"repay_qty"`
	BalanceQty  int64  `json:"balance_qty"`
	BalanceAmt  int64  `json:"balance_amt"`
	NetNew      int64  `json:"net_new"`
	CreditRate  string `json:"credit_rate"`
}

func runCreditBalance(args []string) error {
	symbols, err := parseCreditArgs(args)
	if err != nil {
		return err
	}
	if len(symbols) == 0 {
		return fmt.Errorf("usage: go run ./cmd/agent report credit-balance [--watchlist FILE | SYMBOL ...]")
	}

	client, err := newKISClient()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if _, err := client.EnsureAuthToken(ctx); err != nil {
		return fmt.Errorf("auth token: %w", err)
	}

	svc := domesticstock.NewService(client)
	outDir := envDefault("CREDIT_BALANCE_OUTPUT_DIR", ".cache/credit_balance")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}

	date := time.Now().Format("20060102")
	var reports []CreditBalanceReport

	for i, symbol := range symbols {
		symbol = strings.TrimSpace(symbol)
		if symbol == "" {
			continue
		}
		fmt.Fprintf(os.Stderr, "[credit-balance] [%d/%d] %s: fetching...\n", i+1, len(symbols), symbol)

		report, err := buildCreditReport(ctx, svc, symbol)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[credit-balance] %s: ERROR: %v\n", symbol, err)
			continue
		}
		reports = append(reports, *report)
		fmt.Fprintf(os.Stderr, "[credit-balance] %s (%s): 잔고 %s주, %.1f억, 잔고율 %.2f%%\n",
			symbol, report.Name,
			formatInt(report.Summary.BalanceQty),
			report.Summary.BalanceAmtEok,
			report.Summary.CreditRatePct,
		)

		// Rate limit: 2 API calls per stock, 300ms between
		time.Sleep(300 * time.Millisecond)
	}

	// Save combined report
	if len(reports) > 0 {
		path := fmt.Sprintf("%s/credit_balance.%s.json", outDir, date)
		data, _ := json.MarshalIndent(reports, "", "  ")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "[credit-balance] save error: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "[credit-balance] saved: %s\n", path)
		}
	}

	printCreditTable(reports)
	return nil
}

func buildCreditReport(ctx context.Context, svc *domesticstock.Service, symbol string) (*CreditBalanceReport, error) {
	// 1. Get current price (non-fatal — may fail on weekends)
	var name string
	var currentPrice, prevClose, volume int64
	var changePct float64

	priceResp, priceErr := svc.InquirePrice(ctx, symbol)
	if priceErr == nil {
		priceOutput := firstOutputMap(priceResp, "output")
		name = strVal(priceOutput, "hts_kor_isnm")
		currentPrice = intVal(priceOutput, "stck_prpr")
		prevClose = intVal(priceOutput, "stck_sdpr")
		volume = intVal(priceOutput, "acml_vol")
		if prevClose > 0 {
			changePct = float64(currentPrice-prevClose) / float64(prevClose) * 100
		}
	} else {
		fmt.Fprintf(os.Stderr, "[credit-balance] %s: price skipped: %v\n", symbol, priceErr)
	}

	time.Sleep(300 * time.Millisecond)

	// 2. Get daily credit balance
	creditResp, err := svc.InquireDailyCreditBalance(ctx, symbol, "")
	if err != nil {
		// Debug: if API error, print details
		fmt.Fprintf(os.Stderr, "[credit-balance] %s: API error: %v\n", symbol, err)
		if creditResp != nil && creditResp.Body != nil {
			fmt.Fprintf(os.Stderr, "[credit-balance] %s: rt_cd=%v msg_cd=%s msg=%s\n",
				symbol, creditResp.Body["rt_cd"], creditResp.MessageCode(), creditResp.Message())
			if len(creditResp.RawBody) > 0 && len(creditResp.RawBody) < 500 {
				fmt.Fprintf(os.Stderr, "[credit-balance] %s: raw=%s\n", symbol, string(creditResp.RawBody))
			}
		}
		return nil, fmt.Errorf("credit: %w", err)
	}

	// Try output, output1, output2
	rows := outputArray(creditResp, "output")
	if len(rows) == 0 {
		rows = outputArray(creditResp, "output1")
	}
	if len(rows) == 0 {
		rows = outputArray(creditResp, "output2")
	}

	// Debug first entry if available
	if len(rows) > 0 && os.Getenv("DEBUG") != "" {
		first, _ := json.MarshalIndent(rows[0], "", "  ")
		fmt.Fprintf(os.Stderr, "[credit-balance] %s first row:\n%s\n", symbol, string(first))
	}

	var daily []CreditBalanceDay
	for _, row := range rows {
		d := CreditBalanceDay{
			Date:       strVal(row, "deal_date"),
			NewQty:     intVal(row, "whol_loan_new_stcn"),
			RepayQty:   intVal(row, "whol_loan_rdmp_stcn"),
			BalanceQty: intVal(row, "whol_loan_rmnd_stcn"),
			BalanceAmt: intVal(row, "whol_loan_rmnd_amt"),
			CreditRate: strVal(row, "whol_loan_rmnd_rate"),
		}
		d.NetNew = d.NewQty - d.RepayQty
		if d.Date != "" {
			daily = append(daily, d)
		}
	}

	// Summary from latest entry
	var summary CreditSummary
	if len(daily) > 0 {
		latest := daily[0]
		summary.BalanceQty = latest.BalanceQty
		summary.BalanceAmtEok = float64(latest.BalanceAmt) / 1e8
		summary.BalanceAmtEok = math.Round(summary.BalanceAmtEok*10) / 10
		if rate, err := strconv.ParseFloat(latest.CreditRate, 64); err == nil {
			summary.CreditRatePct = rate
		}
		// 5-day trend
		if len(daily) >= 2 {
			oldest := daily[len(daily)-1]
			if len(daily) > 5 {
				oldest = daily[4]
			}
			diff := latest.BalanceQty - oldest.BalanceQty
			switch {
			case diff > 0:
				summary.Trend5D = "증가"
			case diff < 0:
				summary.Trend5D = "감소"
			default:
				summary.Trend5D = "보합"
			}
		}
	}

	return &CreditBalanceReport{
		Symbol:      symbol,
		Name:        name,
		GeneratedAt: time.Now(),
		Price: PriceInfo{
			CurrentPrice:  currentPrice,
			PreviousClose: prevClose,
			ChangePercent: math.Round(changePct*100) / 100,
			Volume:        volume,
		},
		Summary: summary,
		Daily:   daily,
	}, nil
}

func printCreditTable(reports []CreditBalanceReport) {
	if len(reports) == 0 {
		fmt.Println("\n신용잔고 조회 결과가 없습니다.")
		return
	}
	fmt.Println()
	fmt.Println("# 종목별 신용잔고 요약")
	fmt.Println()
	fmt.Println("| 종목 | 현재가 | 등락률 | 신용잔고(주) | 잔고금액(억) | 잔고율(%) | 5일추세 |")
	fmt.Println("|---|---|---|---|---|---|---|")
	for _, r := range reports {
		fmt.Printf("| %s %s | %s | %+.2f%% | %s | %.1f | %.2f%% | %s |\n",
			r.Symbol, r.Name,
			formatInt(r.Price.CurrentPrice),
			r.Price.ChangePercent,
			formatInt(r.Summary.BalanceQty),
			r.Summary.BalanceAmtEok,
			r.Summary.CreditRatePct,
			r.Summary.Trend5D,
		)
	}
	fmt.Println()
}

// helpers

func firstOutputMap(resp *auth.RESTResponse, key string) map[string]any {
	if resp == nil || resp.Body == nil {
		return nil
	}
	switch v := resp.Body[key].(type) {
	case map[string]any:
		return v
	case []any:
		if len(v) > 0 {
			if m, ok := v[0].(map[string]any); ok {
				return m
			}
		}
	}
	return nil
}

func outputArray(resp *auth.RESTResponse, key string) []map[string]any {
	if resp == nil || resp.Body == nil {
		return nil
	}
	arr, ok := resp.Body[key].([]any)
	if !ok {
		return nil
	}
	var result []map[string]any
	for _, item := range arr {
		if m, ok := item.(map[string]any); ok {
			result = append(result, m)
		}
	}
	return result
}

func strVal(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, _ := m[key].(string)
	return strings.TrimSpace(v)
}

func intVal(m map[string]any, key string) int64 {
	if m == nil {
		return 0
	}
	switch v := m[key].(type) {
	case float64:
		return int64(v)
	case string:
		s := strings.TrimSpace(strings.ReplaceAll(v, ",", ""))
		n, _ := strconv.ParseInt(s, 10, 64)
		return n
	}
	return 0
}

func formatInt(v int64) string {
	s := strconv.FormatInt(v, 10)
	if v < 0 {
		s = s[1:]
	}
	var result []byte
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}
	out := string(result)
	if v < 0 {
		out = "-" + out
	}
	return out
}

// parseCreditArgs parses CLI args: either --watchlist FILE or bare SYMBOL args.
func parseCreditArgs(args []string) ([]string, error) {
	for i, arg := range args {
		if arg == "--watchlist" && i+1 < len(args) {
			return parseWatchlistFile(args[i+1])
		}
	}
	var symbols []string
	for _, arg := range args {
		arg = strings.TrimSpace(arg)
		if arg != "" && !strings.HasPrefix(arg, "--") {
			symbols = append(symbols, arg)
		}
	}
	return symbols, nil
}

func parseWatchlistFile(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("watchlist: %w", err)
	}
	defer f.Close()

	var symbols []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if idx := strings.Index(line, "#"); idx >= 0 {
			line = strings.TrimSpace(line[:idx])
		}
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) > 0 {
			symbols = append(symbols, fields[0])
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("watchlist scan: %w", err)
	}
	fmt.Fprintf(os.Stderr, "[credit-balance] watchlist: %d symbols from %s\n", len(symbols), path)
	return symbols, nil
}
