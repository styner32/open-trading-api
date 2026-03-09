package domesticstock

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/kis-open-api/go/internal/auth"
)

const (
	marketTimePath            = "/uapi/domestic-stock/v1/quotations/market-time"
	inquireIndexPricePath     = "/uapi/domestic-stock/v1/quotations/inquire-index-price"
	programTradeTodayPath     = "/uapi/domestic-stock/v1/quotations/comp-program-trade-today"
	viStatusPath              = "/uapi/domestic-stock/v1/quotations/inquire-vi-status"
	marketFundsPath           = "/uapi/domestic-stock/v1/quotations/mktfunds"
	dailyItemChartPricePath   = "/uapi/domestic-stock/v1/quotations/inquire-daily-itemchartprice"
	indexMasterDownloadURL    = "https://new.real.download.dws.co.kr/common/master/idxcode.mst.zip"
	vkospiCodeEnvKey          = "VKOSPI_INDEX_CODE"
	defaultProgramMarketClass = "K"
)

const (
	marketTimeTRID          = "HHMCM000002C0"
	inquireIndexPriceTRID   = "FHPUP02100000"
	programTradeTodayTRID   = "FHPPG04600101"
	viStatusTRID            = "FHPST01390000"
	marketFundsTRID         = "FHKST649100C0"
	dailyItemChartPriceTRID = "FHKST03010100"
)

var (
	defaultVKOSPICandidates = []string{"2050"}
	indexCodePattern        = regexp.MustCompile(`^\s*(\d{5})`)
)

type Service struct {
	client *auth.KIClient
}

type RSIResult struct {
	Symbol     string
	Period     int
	Last       float64
	Signal     string
	SampleSize int
}

func NewService(client *auth.KIClient) *Service {
	return &Service{client: client}
}

func (s *Service) MarketTime(ctx context.Context) (*auth.RESTResponse, error) {
	return s.client.Get(ctx, marketTimePath, marketTimeTRID, "", map[string]string{})
}

func (s *Service) InquireIndexPrice(ctx context.Context, indexCode string) (*auth.RESTResponse, error) {
	params := map[string]string{
		"FID_COND_MRKT_DIV_CODE": "U",
		"FID_INPUT_ISCD":         strings.TrimSpace(indexCode),
	}
	return s.client.Get(ctx, inquireIndexPricePath, inquireIndexPriceTRID, "", params)
}

func (s *Service) CompProgramTradeToday(ctx context.Context, marketClassCode string) (*auth.RESTResponse, error) {
	marketClassCode = strings.TrimSpace(marketClassCode)
	if marketClassCode == "" {
		marketClassCode = defaultProgramMarketClass
	}

	params := map[string]string{
		"FID_COND_MRKT_DIV_CODE":  "J",
		"FID_MRKT_CLS_CODE":       marketClassCode,
		"FID_SCTN_CLS_CODE":       "",
		"FID_INPUT_ISCD":          "",
		"FID_COND_MRKT_DIV_CODE1": "",
		"FID_INPUT_HOUR_1":        "",
	}
	return s.client.Get(ctx, programTradeTodayPath, programTradeTodayTRID, "", params)
}

func (s *Service) InquireVIStatus(ctx context.Context, yyyymmdd string) (*auth.RESTResponse, error) {
	if strings.TrimSpace(yyyymmdd) == "" {
		return nil, errors.New("yyyymmdd is required")
	}

	params := map[string]string{
		"FID_DIV_CLS_CODE":       "0",
		"FID_COND_SCR_DIV_CODE":  "20139",
		"FID_MRKT_CLS_CODE":      "0",
		"FID_INPUT_ISCD":         "",
		"FID_RANK_SORT_CLS_CODE": "0",
		"FID_INPUT_DATE_1":       yyyymmdd,
		"FID_TRGT_CLS_CODE":      "",
		"FID_TRGT_EXLS_CLS_CODE": "",
	}
	return s.client.Get(ctx, viStatusPath, viStatusTRID, "", params)
}

func (s *Service) MarketFunds(ctx context.Context, yyyymmdd string) (*auth.RESTResponse, error) {
	params := map[string]string{
		"FID_INPUT_DATE_1": strings.TrimSpace(yyyymmdd),
	}
	return s.client.Get(ctx, marketFundsPath, marketFundsTRID, "", params)
}

func (s *Service) InquireDailyItemChartPrice(
	ctx context.Context,
	symbol string,
	fromDate string,
	toDate string,
	period string,
	orgAdjPrice string,
) (*auth.RESTResponse, error) {
	symbol = strings.TrimSpace(symbol)
	fromDate = strings.TrimSpace(fromDate)
	toDate = strings.TrimSpace(toDate)
	period = strings.TrimSpace(period)
	orgAdjPrice = strings.TrimSpace(orgAdjPrice)

	if symbol == "" {
		return nil, errors.New("symbol is required")
	}
	if fromDate == "" || toDate == "" {
		return nil, errors.New("fromDate and toDate are required")
	}
	if period == "" {
		period = "D"
	}
	if orgAdjPrice == "" {
		orgAdjPrice = "1"
	}

	params := map[string]string{
		"FID_COND_MRKT_DIV_CODE": "J",
		"FID_INPUT_ISCD":         symbol,
		"FID_INPUT_DATE_1":       fromDate,
		"FID_INPUT_DATE_2":       toDate,
		"FID_PERIOD_DIV_CODE":    period,
		"FID_ORG_ADJ_PRC":        orgAdjPrice,
	}
	return s.client.Get(ctx, dailyItemChartPricePath, dailyItemChartPriceTRID, "", params)
}

func (s *Service) ResolveVKOSPICode(ctx context.Context, candidates []string) (string, error) {
	envCode := strings.TrimSpace(os.Getenv(vkospiCodeEnvKey))
	if envCode != "" {
		return envCode, nil
	}

	discoveredCode, err := discoverVKOSPICodeFromMaster(ctx)
	if err == nil && discoveredCode != "" {
		return discoveredCode, nil
	}

	candidates = normalizeCandidates(candidates)
	for _, code := range candidates {
		resp, reqErr := s.InquireIndexPrice(ctx, code)
		if reqErr != nil {
			continue
		}
		if !resp.IsOK() {
			continue
		}
		if len(toRows(resp.Body["output"])) > 0 {
			return code, nil
		}
	}

	return "", fmt.Errorf("unable to resolve VKOSPI code (set %s env var)", vkospiCodeEnvKey)
}

func (s *Service) RSIFromDailyChart(
	ctx context.Context,
	symbol string,
	period int,
	fromDate string,
	toDate string,
) (*RSIResult, error) {
	if period <= 0 {
		return nil, errors.New("period must be > 0")
	}

	resp, err := s.InquireDailyItemChartPrice(ctx, symbol, fromDate, toDate, "D", "1")
	if err != nil {
		return nil, err
	}
	if !resp.IsOK() {
		return nil, fmt.Errorf("RSI source API returned error: msg_cd=%s msg1=%s", resp.MessageCode(), resp.Message())
	}

	rows := toRows(resp.Body["output2"])
	if len(rows) == 0 {
		return nil, errors.New("daily chart output2 is empty")
	}

	closes, err := extractCloseSeries(rows)
	if err != nil {
		return nil, err
	}

	rsiValue, err := calculateRSI(closes, period)
	if err != nil {
		return nil, err
	}

	return &RSIResult{
		Symbol:     strings.TrimSpace(symbol),
		Period:     period,
		Last:       rsiValue,
		Signal:     rsiSignal(rsiValue),
		SampleSize: len(closes),
	}, nil
}

func discoverVKOSPICodeFromMaster(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, indexMasterDownloadURL, nil)
	if err != nil {
		return "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("master download failed (status %d)", resp.StatusCode)
	}

	zipBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	readerAt := bytes.NewReader(zipBytes)
	zipReader, err := zip.NewReader(readerAt, int64(len(zipBytes)))
	if err != nil {
		return "", err
	}

	for _, file := range zipReader.File {
		if strings.EqualFold(file.Name, "idxcode.mst") {
			fileReader, openErr := file.Open()
			if openErr != nil {
				return "", openErr
			}
			defer fileReader.Close()

			scanner := bufio.NewScanner(fileReader)
			for scanner.Scan() {
				line := scanner.Text()
				upperLine := strings.ToUpper(line)
				if !strings.Contains(upperLine, "VKOSPI") && !strings.Contains(upperLine, "V-KOSPI") {
					continue
				}
				code := extractIndexCode(line)
				if code != "" {
					return code, nil
				}
			}

			if err := scanner.Err(); err != nil {
				return "", err
			}
		}
	}

	return "", errors.New("VKOSPI code not found in idxcode master")
}

func extractIndexCode(line string) string {
	matches := indexCodePattern.FindStringSubmatch(line)
	if len(matches) < 2 {
		return ""
	}
	fullCode := matches[1]
	if len(fullCode) != 5 {
		return ""
	}
	return fullCode[1:]
}

func normalizeCandidates(candidates []string) []string {
	if len(candidates) == 0 {
		return defaultVKOSPICandidates
	}

	seen := map[string]struct{}{}
	out := make([]string, 0, len(candidates))
	for _, code := range candidates {
		code = strings.TrimSpace(code)
		if code == "" {
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		out = append(out, code)
	}

	if len(out) == 0 {
		return defaultVKOSPICandidates
	}
	return out
}

func toRows(value any) []map[string]any {
	switch typed := value.(type) {
	case []any:
		rows := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			row, ok := item.(map[string]any)
			if !ok {
				continue
			}
			rows = append(rows, row)
		}
		return rows
	case map[string]any:
		return []map[string]any{typed}
	default:
		return nil
	}
}

func extractCloseSeries(rows []map[string]any) ([]float64, error) {
	type point struct {
		Date  string
		Close float64
	}

	points := make([]point, 0, len(rows))
	for _, row := range rows {
		closeValue, ok := parseFloat(row["stck_clpr"])
		if !ok {
			continue
		}

		date := strings.TrimSpace(toString(row["stck_bsop_date"]))
		points = append(points, point{Date: date, Close: closeValue})
	}

	if len(points) == 0 {
		return nil, errors.New("no valid close prices found in output2")
	}

	sort.Slice(points, func(i, j int) bool {
		return points[i].Date < points[j].Date
	})

	closes := make([]float64, 0, len(points))
	for _, p := range points {
		closes = append(closes, p.Close)
	}
	return closes, nil
}

func calculateRSI(closes []float64, period int) (float64, error) {
	if len(closes) <= period {
		return 0, fmt.Errorf("not enough closes for RSI(%d): got %d", period, len(closes))
	}

	var gainSum, lossSum float64
	for i := 1; i <= period; i++ {
		delta := closes[i] - closes[i-1]
		if delta > 0 {
			gainSum += delta
		} else if delta < 0 {
			lossSum += -delta
		}
	}

	avgGain := gainSum / float64(period)
	avgLoss := lossSum / float64(period)

	for i := period + 1; i < len(closes); i++ {
		delta := closes[i] - closes[i-1]

		gain := math.Max(delta, 0)
		loss := math.Max(-delta, 0)

		avgGain = ((avgGain * float64(period-1)) + gain) / float64(period)
		avgLoss = ((avgLoss * float64(period-1)) + loss) / float64(period)
	}

	if avgLoss == 0 {
		if avgGain == 0 {
			return 50, nil
		}
		return 100, nil
	}

	rs := avgGain / avgLoss
	rsi := 100 - (100 / (1 + rs))
	return rsi, nil
}

func rsiSignal(rsi float64) string {
	switch {
	case rsi >= 70:
		return "OVERBOUGHT"
	case rsi <= 30:
		return "OVERSOLD"
	default:
		return "NEUTRAL"
	}
}

func parseFloat(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil
	case string:
		clean := strings.ReplaceAll(strings.TrimSpace(typed), ",", "")
		if clean == "" {
			return 0, false
		}
		parsed, err := strconv.ParseFloat(clean, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func toString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return fmt.Sprintf("%v", value)
	}
}
