package domesticstock

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/kis-open-api/go/internal/auth"
)

func (s *Service) MarketTime(ctx context.Context) (*auth.RESTResponse, error) {
	return s.client.Get(ctx, marketTimePath, marketTimeTRID, "", map[string]string{})
}

func (s *Service) InquirePrice(ctx context.Context, symbol string) (*auth.RESTResponse, error) {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return nil, errors.New("symbol is required")
	}

	params := map[string]string{
		"FID_COND_MRKT_DIV_CODE": "J",
		"FID_INPUT_ISCD":         symbol,
	}
	return s.client.Get(ctx, inquirePricePath, inquirePriceTRID, "", params)
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

	discoveredCode, err := discoverVKOSPICodeFromMaster(ctx, s.client)
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
