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
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/kis-open-api/go/internal/auth"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/transform"
)

const (
	marketTimePath               = "/uapi/domestic-stock/v1/quotations/market-time"
	inquirePricePath             = "/uapi/domestic-stock/v1/quotations/inquire-price"
	inquireIndexPricePath        = "/uapi/domestic-stock/v1/quotations/inquire-index-price"
	programTradeTodayPath        = "/uapi/domestic-stock/v1/quotations/comp-program-trade-today"
	viStatusPath                 = "/uapi/domestic-stock/v1/quotations/inquire-vi-status"
	marketFundsPath              = "/uapi/domestic-stock/v1/quotations/mktfunds"
	dailyItemChartPricePath      = "/uapi/domestic-stock/v1/quotations/inquire-daily-itemchartprice"
	indexMasterDownloadURL       = "https://new.real.download.dws.co.kr/common/master/idxcode.mst.zip"
	kospiMasterDownloadURL       = "https://new.real.download.dws.co.kr/common/master/kospi_code.mst.zip"
	kospiMasterFilename          = "kospi_code.mst"
	vkospiCodeEnvKey             = "VKOSPI_INDEX_CODE"
	kospiMasterCacheEnvKey       = "KOSPI_MASTER_CACHE_FILE"
	kospiActualPBRCacheEnvKey    = "KOSPI_ACTUAL_PBR_CACHE_FILE"
	kospiActualPBRRPMEnvKey      = "KOSPI_ACTUAL_PBR_RATE_LIMIT_RPM"
	kospiActualPBRDebugEnvKey    = "KOSPI_ACTUAL_PBR_DEBUG"
	kospiActualPBRProgressEnvKey = "KOSPI_ACTUAL_PBR_PROGRESS"
	kospiProxyScaleEnvKey        = "KOSPI_PROXY_PBR_EARNINGS_SCALE"
	defaultProgramMarketClass    = "K"
	defaultKOSPIMasterCache      = ".cache/kospi_code.mst"
	defaultKOSPIActualPBRCache   = ".cache/kospi_actual_pbr.json"
)

const (
	marketTimeTRID          = "HHMCM000002C0"
	inquirePriceTRID        = "FHKST01010100"
	inquireIndexPriceTRID   = "FHPUP02100000"
	programTradeTodayTRID   = "FHPPG04600101"
	viStatusTRID            = "FHPST01390000"
	marketFundsTRID         = "FHKST649100C0"
	dailyItemChartPriceTRID = "FHKST03010100"
)

var (
	defaultVKOSPICandidates = []string{"2050"}
	indexCodePattern        = regexp.MustCompile(`^\s*(\d{5})`)
	kospiPart2Width         = 227
	kospiPart2FieldWidths   = []int{
		2, 1, 4, 4, 4,
		1, 1, 1, 1, 1,
		1, 1, 1, 1, 1,
		1, 1, 1, 1, 1,
		1, 1, 1, 1, 1,
		1, 1, 1, 1, 1,
		1, 9, 5, 5, 1,
		1, 1, 2, 1, 1,
		1, 2, 2, 2, 3,
		1, 3, 12, 12, 8,
		15, 21, 2, 7, 1,
		1, 1, 1, 1, 9,
		9, 9, 5, 9, 8,
		9, 3, 1, 1, 1,
	}
)

const defaultKOSPIActualPBRRPM = 18

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

type ProxyPBRConstituent struct {
	Code       string
	Name       string
	MarketCap  float64
	ROE        float64
	NetIncome  float64
	BookEquity float64
	ProxyPBR   float64
	Coverage   float64
	BaseDate   string
}

type ProxyPBRResult struct {
	Market             string
	Method             string
	TargetCoverage     float64
	RawCoverage        float64
	UsedCoverage       float64
	SelectedCount      int
	UsedCount          int
	SkippedCount       int
	TotalCount         int
	TotalMarketCap     float64
	RawSelectedCap     float64
	UsedMarketCap      float64
	SkippedMarketCap   float64
	AggregateBookValue float64
	ProxyPBR           float64
	EarningsScale      float64
	CalibrationSymbol  string
	BasisDate          string
	Constituents       []ProxyPBRConstituent
}

type ActualPBRConstituent struct {
	Code       string
	Name       string
	MarketCap  float64
	PBR        float64
	BookEquity float64
	Coverage   float64
	BaseDate   string
	CacheHit   bool
}

type ActualPBRResult struct {
	Market             string
	Method             string
	TargetCoverage     float64
	RawCoverage        float64
	UsedCoverage       float64
	SelectedCount      int
	UsedCount          int
	SkippedCount       int
	TotalCount         int
	CacheHitCount      int
	APICallCount       int
	TotalMarketCap     float64
	RawSelectedCap     float64
	UsedMarketCap      float64
	SkippedMarketCap   float64
	AggregateBookValue float64
	WeightedPBR        float64
	BusinessDate       string
	ActualPBRCachePath string
	MasterCachePath    string
	MasterJSONPath     string
	MasterLoadTime     time.Duration
	CacheLoadTime      time.Duration
	RateLimitWaitTime  time.Duration
	PriceFetchTime     time.Duration
	CacheSaveTime      time.Duration
	TotalDuration      time.Duration
	Constituents       []ActualPBRConstituent
}

type kospiMasterRecord struct {
	Code      string  `json:"code"`
	Name      string  `json:"name"`
	MarketCap float64 `json:"market_cap"`
	NetIncome float64 `json:"net_income"`
	ROE       float64 `json:"roe"`
	BaseDate  string  `json:"base_date"`
}

type kospiMasterJSONField struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Source      string `json:"source"`
}

type kospiMasterJSONCache struct {
	BusinessDate string                 `json:"business_date"`
	GeneratedAt  string                 `json:"generated_at"`
	SourcePath   string                 `json:"source_path"`
	RecordCount  int                    `json:"record_count"`
	Fields       []kospiMasterJSONField `json:"fields"`
	Records      []kospiMasterRecord    `json:"records"`
}

type actualPBRCacheEntry struct {
	PBR          float64 `json:"pbr"`
	MarketCap    float64 `json:"market_cap"`
	BusinessDate string  `json:"business_date"`
	UpdatedAt    string  `json:"updated_at"`
	Name         string  `json:"name,omitempty"`
}

type actualPBRCache map[string]actualPBRCacheEntry

type requestPacer struct {
	interval    time.Duration
	nextAllowed time.Time
}

type actualPBRLookupResult struct {
	PBR           float64
	MarketCap     float64
	CacheHit      bool
	WaitDuration  time.Duration
	FetchDuration time.Duration
	CacheSaveTime time.Duration
}

func NewService(client *auth.KIClient) *Service {
	return &Service{client: client}
}

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

func (s *Service) KOSPIProxyPBR(ctx context.Context, targetCoverage float64) (*ProxyPBRResult, error) {
	if targetCoverage <= 0 || targetCoverage > 1 {
		return nil, errors.New("targetCoverage must be > 0 and <= 1")
	}

	records, err := s.loadKOSPIMaster(ctx, time.Now().Format("20060102"))
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, errors.New("kospi master file did not contain usable records")
	}

	sort.Slice(records, func(i, j int) bool {
		return records[i].MarketCap > records[j].MarketCap
	})

	totalMarketCap := 0.0
	for _, record := range records {
		totalMarketCap += record.MarketCap
	}
	if totalMarketCap <= 0 {
		return nil, errors.New("total KOSPI market cap is zero")
	}

	scale, calibrationSymbol, err := s.resolveKOSPIProxyScale(ctx, records)
	if err != nil {
		return nil, err
	}

	targetMarketCap := totalMarketCap * targetCoverage

	result := &ProxyPBRResult{
		Market:            "KOSPI",
		Method:            "master-file-proxy",
		TargetCoverage:    targetCoverage,
		TotalCount:        len(records),
		TotalMarketCap:    totalMarketCap,
		EarningsScale:     scale,
		CalibrationSymbol: calibrationSymbol,
	}

	for _, record := range records {
		result.SelectedCount++
		result.RawSelectedCap += record.MarketCap

		bookEquity, stockProxyPBR, ok := proxyBookEquity(record, scale)
		if !ok {
			result.SkippedCount++
			result.SkippedMarketCap += record.MarketCap
		} else {
			result.UsedCount++
			result.UsedMarketCap += record.MarketCap
			result.AggregateBookValue += bookEquity
			result.Constituents = append(result.Constituents, ProxyPBRConstituent{
				Code:       record.Code,
				Name:       record.Name,
				MarketCap:  record.MarketCap,
				ROE:        record.ROE,
				NetIncome:  record.NetIncome * scale,
				BookEquity: bookEquity,
				ProxyPBR:   stockProxyPBR,
				Coverage:   record.MarketCap / totalMarketCap,
				BaseDate:   record.BaseDate,
			})
			if result.BasisDate == "" {
				result.BasisDate = record.BaseDate
			}
		}

		if result.UsedMarketCap >= targetMarketCap {
			break
		}
	}

	if result.AggregateBookValue <= 0 {
		return nil, errors.New("aggregate book value is zero; unable to compute proxy PBR")
	}

	result.RawCoverage = result.RawSelectedCap / totalMarketCap
	result.UsedCoverage = result.UsedMarketCap / totalMarketCap
	result.ProxyPBR = result.UsedMarketCap / result.AggregateBookValue

	return result, nil
}

func (s *Service) KOSPIActualPBR(ctx context.Context, targetCoverage float64, businessDate string) (*ActualPBRResult, error) {
	if targetCoverage <= 0 || targetCoverage > 1 {
		return nil, errors.New("targetCoverage must be > 0 and <= 1")
	}

	startedAt := time.Now()
	businessDate = strings.TrimSpace(businessDate)
	if businessDate == "" {
		businessDate = time.Now().Format("20060102")
	}
	debug := actualPBRDebugEnabled()
	progress := actualPBRProgressEnabled()
	actualPBRProgressLog(progress, "start target=%.2f%% business_date=%s", targetCoverage*100, businessDate)
	actualPBRLog(debug, "start target=%.2f%% business_date=%s", targetCoverage*100, businessDate)

	masterStartedAt := time.Now()
	records, err := s.loadKOSPIMaster(ctx, businessDate)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, errors.New("kospi master file did not contain usable records")
	}
	masterLoadTime := time.Since(masterStartedAt)
	actualPBRProgressLog(progress, "master loaded records=%d took=%s", len(records), masterLoadTime)
	actualPBRLog(debug, "master loaded records=%d took=%s", len(records), masterLoadTime)

	sort.Slice(records, func(i, j int) bool {
		return records[i].MarketCap > records[j].MarketCap
	})

	totalMarketCap := 0.0
	for _, record := range records {
		totalMarketCap += record.MarketCap
	}
	if totalMarketCap <= 0 {
		return nil, errors.New("total KOSPI market cap is zero")
	}

	cachePath := strings.TrimSpace(os.Getenv(kospiActualPBRCacheEnvKey))
	if cachePath == "" {
		cachePath = defaultKOSPIActualPBRCache
	}
	cachePath = resolveKOSPIActualPBRCachePath(cachePath, businessDate)
	masterCachePath := resolveKOSPIMasterCachePath(getOrDefaultEnv(kospiMasterCacheEnvKey, defaultKOSPIMasterCache), businessDate)

	cacheStartedAt := time.Now()
	cache, err := loadActualPBRCache(cachePath)
	if err != nil {
		return nil, err
	}
	cacheLoadTime := time.Since(cacheStartedAt)
	actualPBRProgressLog(progress, "cache loaded entries=%d path=%s took=%s", len(cache), cachePath, cacheLoadTime)
	actualPBRLog(debug, "cache loaded entries=%d path=%s took=%s", len(cache), cachePath, cacheLoadTime)

	pacer := newRequestPacer(getEnvInt(kospiActualPBRRPMEnvKey, defaultKOSPIActualPBRRPM))
	targetMarketCap := totalMarketCap * targetCoverage

	result := &ActualPBRResult{
		Market:             "KOSPI",
		Method:             "master-cap-actual-pbr",
		TargetCoverage:     targetCoverage,
		TotalCount:         len(records),
		TotalMarketCap:     totalMarketCap,
		BusinessDate:       businessDate,
		ActualPBRCachePath: cachePath,
		MasterCachePath:    masterCachePath,
		MasterJSONPath:     resolveKOSPIMasterJSONPath(masterCachePath),
		MasterLoadTime:     masterLoadTime,
		CacheLoadTime:      cacheLoadTime,
	}

	for _, record := range records {
		result.SelectedCount++
		result.RawSelectedCap += record.MarketCap

		lookup, lookupErr := s.lookupActualPBR(ctx, record, businessDate, cache, cachePath, pacer, progress)
		result.RateLimitWaitTime += lookup.WaitDuration
		result.PriceFetchTime += lookup.FetchDuration
		result.CacheSaveTime += lookup.CacheSaveTime
		if lookupErr != nil || lookup.PBR <= 0 {
			result.SkippedCount++
			result.SkippedMarketCap += record.MarketCap
			actualPBRLog(debug, "skip code=%s name=%s err=%v", record.Code, record.Name, lookupErr)
			continue
		}

		pbr := lookup.PBR
		effectiveMarketCap := lookup.MarketCap
		if effectiveMarketCap <= 0 {
			effectiveMarketCap = record.MarketCap
		}

		bookEquity := effectiveMarketCap / pbr
		if bookEquity <= 0 || math.IsNaN(bookEquity) || math.IsInf(bookEquity, 0) {
			result.SkippedCount++
			result.SkippedMarketCap += record.MarketCap
			continue
		}

		if lookup.CacheHit {
			result.CacheHitCount++
		} else {
			result.APICallCount++
		}

		result.UsedCount++
		result.UsedMarketCap += effectiveMarketCap
		result.AggregateBookValue += bookEquity
		result.Constituents = append(result.Constituents, ActualPBRConstituent{
			Code:       record.Code,
			Name:       record.Name,
			MarketCap:  effectiveMarketCap,
			PBR:        pbr,
			BookEquity: bookEquity,
			Coverage:   effectiveMarketCap / totalMarketCap,
			BaseDate:   record.BaseDate,
			CacheHit:   lookup.CacheHit,
		})

		if progress && (!lookup.CacheHit || result.SelectedCount%10 == 0 || result.UsedMarketCap >= targetMarketCap) {
			actualPBRProgressLog(
				progress,
				"progress selected=%d used=%d cache=%d api=%d raw=%.2f%% used=%.2f%% last=%s",
				result.SelectedCount,
				result.UsedCount,
				result.CacheHitCount,
				result.APICallCount,
				(result.RawSelectedCap/totalMarketCap)*100,
				(result.UsedMarketCap/totalMarketCap)*100,
				record.Code,
			)
		}

		if debug && (!lookup.CacheHit || result.SelectedCount%10 == 0 || result.UsedMarketCap >= targetMarketCap) {
			actualPBRLog(
				debug,
				"progress selected=%d used=%d cache=%d api=%d raw=%.2f%% used=%.2f%% last=%s wait=%s fetch=%s",
				result.SelectedCount,
				result.UsedCount,
				result.CacheHitCount,
				result.APICallCount,
				(result.RawSelectedCap/totalMarketCap)*100,
				(result.UsedMarketCap/totalMarketCap)*100,
				record.Code,
				lookup.WaitDuration,
				lookup.FetchDuration,
			)
		}

		if result.UsedMarketCap >= targetMarketCap {
			break
		}
	}

	if result.AggregateBookValue <= 0 {
		return nil, errors.New("aggregate book value is zero; unable to compute weighted PBR")
	}

	result.RawCoverage = result.RawSelectedCap / totalMarketCap
	result.UsedCoverage = result.UsedMarketCap / totalMarketCap
	result.WeightedPBR = result.UsedMarketCap / result.AggregateBookValue
	result.TotalDuration = time.Since(startedAt)
	actualPBRProgressLog(
		progress,
		"done weighted_pbr=%.4f used=%.2f%% cache=%d api=%d total=%s",
		result.WeightedPBR,
		result.UsedCoverage*100,
		result.CacheHitCount,
		result.APICallCount,
		result.TotalDuration,
	)
	actualPBRLog(
		debug,
		"done weighted_pbr=%.4f used=%.2f%% cache=%d api=%d wait=%s fetch=%s total=%s",
		result.WeightedPBR,
		result.UsedCoverage*100,
		result.CacheHitCount,
		result.APICallCount,
		result.RateLimitWaitTime,
		result.PriceFetchTime,
		result.TotalDuration,
	)

	if result.UsedCoverage < targetCoverage {
		return nil, fmt.Errorf(
			"insufficient KOSPI actual PBR coverage: used %.2f%% target %.2f%%",
			result.UsedCoverage*100,
			targetCoverage*100,
		)
	}

	return result, nil
}

func discoverVKOSPICodeFromMaster(ctx context.Context, client *auth.KIClient) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, indexMasterDownloadURL, nil)
	if err != nil {
		return "", err
	}

	var resp *http.Response
	if client != nil {
		resp, err = client.Do(req)
	} else {
		resp, err = http.DefaultClient.Do(req)
	}
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

func (s *Service) lookupActualPBR(
	ctx context.Context,
	record kospiMasterRecord,
	businessDate string,
	cache actualPBRCache,
	cachePath string,
	pacer *requestPacer,
	progress bool,
) (actualPBRLookupResult, error) {
	if entry, ok := cache[record.Code]; ok && entry.BusinessDate == businessDate && entry.PBR > 0 {
		return actualPBRLookupResult{
			PBR:       entry.PBR,
			MarketCap: entry.MarketCap,
			CacheHit:  true,
		}, nil
	}

	actualPBRProgressLog(progress, "fetch start code=%s name=%s", record.Code, record.Name)
	waitDuration, err := pacer.Wait(ctx)
	if err != nil {
		return actualPBRLookupResult{WaitDuration: waitDuration}, err
	}

	fetchStartedAt := time.Now()
	resp, err := s.InquirePrice(ctx, record.Code)
	fetchDuration := time.Since(fetchStartedAt)
	if err != nil {
		return actualPBRLookupResult{
			WaitDuration:  waitDuration,
			FetchDuration: fetchDuration,
		}, err
	}
	if !resp.IsOK() {
		return actualPBRLookupResult{
			WaitDuration:  waitDuration,
			FetchDuration: fetchDuration,
		}, fmt.Errorf("inquire-price error for %s: msg_cd=%s msg1=%s", record.Code, resp.MessageCode(), resp.Message())
	}

	row := firstOutputRow(resp, "output")
	if row == nil {
		return actualPBRLookupResult{
			WaitDuration:  waitDuration,
			FetchDuration: fetchDuration,
		}, fmt.Errorf("inquire-price output missing for %s", record.Code)
	}

	pbr, ok := parseFloat(row["pbr"])
	if !ok || pbr <= 0 {
		return actualPBRLookupResult{
			WaitDuration:  waitDuration,
			FetchDuration: fetchDuration,
		}, fmt.Errorf("invalid pbr for %s", record.Code)
	}

	effectiveMarketCap := record.MarketCap
	if actualMarketCap, ok := parseFloat(row["hts_avls"]); ok && actualMarketCap > 0 {
		effectiveMarketCap = actualMarketCap
	}

	cacheSaveStartedAt := time.Now()
	cache[record.Code] = actualPBRCacheEntry{
		PBR:          pbr,
		MarketCap:    effectiveMarketCap,
		BusinessDate: businessDate,
		UpdatedAt:    time.Now().Format(time.RFC3339),
		Name:         record.Name,
	}

	if err := saveActualPBRCache(cachePath, cache); err != nil {
		return actualPBRLookupResult{
			WaitDuration:  waitDuration,
			FetchDuration: fetchDuration,
		}, err
	}
	cacheSaveTime := time.Since(cacheSaveStartedAt)

	return actualPBRLookupResult{
		PBR:           pbr,
		MarketCap:     effectiveMarketCap,
		CacheHit:      false,
		WaitDuration:  waitDuration,
		FetchDuration: fetchDuration,
		CacheSaveTime: cacheSaveTime,
	}, nil
}

func (s *Service) loadKOSPIMaster(ctx context.Context, businessDate string) ([]kospiMasterRecord, error) {
	cachePath := strings.TrimSpace(os.Getenv(kospiMasterCacheEnvKey))
	if cachePath == "" {
		cachePath = defaultKOSPIMasterCache
	}
	cachePath = resolveKOSPIMasterCachePath(cachePath, businessDate)

	if err := s.ensureKOSPIMasterCache(ctx, cachePath); err != nil {
		return nil, err
	}

	raw, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read KOSPI master cache: %w", err)
	}

	records, err := parseKOSPIMaster(raw)
	if err != nil {
		return nil, err
	}

	if err := ensureKOSPIMasterJSONCache(cachePath, businessDate, records); err != nil {
		return nil, err
	}

	return records, nil
}

func loadActualPBRCache(cachePath string) (actualPBRCache, error) {
	if strings.TrimSpace(cachePath) == "" {
		return actualPBRCache{}, nil
	}

	raw, err := os.ReadFile(cachePath)
	if errors.Is(err, os.ErrNotExist) {
		return actualPBRCache{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read actual PBR cache: %w", err)
	}
	if len(raw) == 0 {
		return actualPBRCache{}, nil
	}

	cache := actualPBRCache{}
	if err := json.Unmarshal(raw, &cache); err != nil {
		return nil, fmt.Errorf("failed to unmarshal actual PBR cache: %w", err)
	}
	return cache, nil
}

func saveActualPBRCache(cachePath string, cache actualPBRCache) error {
	if strings.TrimSpace(cachePath) == "" {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		return fmt.Errorf("failed to create actual PBR cache dir: %w", err)
	}

	raw, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal actual PBR cache: %w", err)
	}

	tmpPath := cachePath + ".tmp"
	if err := os.WriteFile(tmpPath, raw, 0o644); err != nil {
		return fmt.Errorf("failed to write actual PBR cache temp file: %w", err)
	}
	if err := os.Rename(tmpPath, cachePath); err != nil {
		return fmt.Errorf("failed to replace actual PBR cache: %w", err)
	}

	return nil
}

func (s *Service) ensureKOSPIMasterCache(ctx context.Context, cachePath string) error {
	_, err := os.Stat(cachePath)
	if err == nil {
		return nil
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to stat KOSPI master cache: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, kospiMasterDownloadURL, nil)
	if err != nil {
		return err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download KOSPI master zip: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("KOSPI master download failed (status %d)", resp.StatusCode)
	}

	zipBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	masterBytes, err := unzipSingleFile(zipBytes, kospiMasterFilename)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		return fmt.Errorf("failed to create KOSPI master cache dir: %w", err)
	}

	tmpPath := cachePath + ".tmp"
	if err := os.WriteFile(tmpPath, masterBytes, 0o644); err != nil {
		return fmt.Errorf("failed to write KOSPI master temp file: %w", err)
	}
	if err := os.Rename(tmpPath, cachePath); err != nil {
		return fmt.Errorf("failed to replace KOSPI master cache: %w", err)
	}

	return nil
}

func resolveKOSPIMasterCachePath(cachePath string, businessDate string) string {
	return resolveBusinessDateCachePath(cachePath, businessDate, defaultKOSPIMasterCache)
}

func resolveKOSPIActualPBRCachePath(cachePath string, businessDate string) string {
	return resolveBusinessDateCachePath(cachePath, businessDate, defaultKOSPIActualPBRCache)
}

func resolveBusinessDateCachePath(cachePath string, businessDate string, defaultPath string) string {
	cachePath = strings.TrimSpace(cachePath)
	if cachePath == "" {
		cachePath = defaultPath
	}

	businessDate = strings.TrimSpace(businessDate)
	if !isYYYYMMDD(businessDate) {
		businessDate = time.Now().Format("20060102")
	}

	ext := filepath.Ext(cachePath)
	if ext == "" {
		return cachePath + "." + businessDate
	}

	base := strings.TrimSuffix(cachePath, ext)
	return base + "." + businessDate + ext
}

func resolveKOSPIMasterJSONPath(cachePath string) string {
	cachePath = strings.TrimSpace(cachePath)
	if cachePath == "" {
		cachePath = defaultKOSPIMasterCache
	}

	ext := filepath.Ext(cachePath)
	if ext == "" {
		return cachePath + ".json"
	}

	return strings.TrimSuffix(cachePath, ext) + ".json"
}

func ensureKOSPIMasterJSONCache(cachePath string, businessDate string, records []kospiMasterRecord) error {
	jsonPath := resolveKOSPIMasterJSONPath(cachePath)

	masterInfo, err := os.Stat(cachePath)
	if err != nil {
		return fmt.Errorf("failed to stat KOSPI master cache for json export: %w", err)
	}

	if jsonInfo, err := os.Stat(jsonPath); err == nil && !masterInfo.ModTime().After(jsonInfo.ModTime()) {
		return nil
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to stat KOSPI master json cache: %w", err)
	}

	payload := kospiMasterJSONCache{
		BusinessDate: strings.TrimSpace(businessDate),
		GeneratedAt:  time.Now().Format(time.RFC3339),
		SourcePath:   cachePath,
		RecordCount:  len(records),
		Fields: []kospiMasterJSONField{
			{Name: "code", Description: "6자리 단축 종목코드", Source: "part1[0:9]"},
			{Name: "name", Description: "한글 종목명", Source: "part1[21:]"},
			{Name: "market_cap", Description: "전일기준 시가총액(억)", Source: "field[65]"},
			{Name: "net_income", Description: "당기순이익", Source: "field[62]"},
			{Name: "roe", Description: "ROE(자기자본이익률)", Source: "field[63]"},
			{Name: "base_date", Description: "재무 기준년월", Source: "field[64]"},
		},
		Records: records,
	}

	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal KOSPI master json cache: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(jsonPath), 0o755); err != nil {
		return fmt.Errorf("failed to create KOSPI master json dir: %w", err)
	}

	tmpPath := jsonPath + ".tmp"
	if err := os.WriteFile(tmpPath, raw, 0o644); err != nil {
		return fmt.Errorf("failed to write KOSPI master json temp file: %w", err)
	}
	if err := os.Rename(tmpPath, jsonPath); err != nil {
		return fmt.Errorf("failed to replace KOSPI master json cache: %w", err)
	}

	return nil
}

func (s *Service) resolveKOSPIProxyScale(ctx context.Context, records []kospiMasterRecord) (float64, string, error) {
	if raw := strings.TrimSpace(os.Getenv(kospiProxyScaleEnvKey)); raw != "" {
		scale, ok := parseFloat(raw)
		if !ok || scale <= 0 {
			return 0, "", fmt.Errorf("%s must be a positive number", kospiProxyScaleEnvKey)
		}
		return scale, "ENV", nil
	}

	for _, record := range records {
		if record.MarketCap <= 0 || record.NetIncome <= 0 || record.ROE <= 0 {
			continue
		}

		resp, err := s.InquirePrice(ctx, record.Code)
		if err != nil {
			continue
		}
		if !resp.IsOK() {
			continue
		}

		row := firstOutputRow(resp, "output")
		if row == nil {
			continue
		}

		pbr, ok := parseFloat(row["pbr"])
		if !ok || pbr <= 0 {
			continue
		}

		scale := (record.MarketCap * record.ROE) / (100 * record.NetIncome * pbr)
		if scale <= 0 || math.IsNaN(scale) || math.IsInf(scale, 0) {
			continue
		}

		return scale, record.Code, nil
	}

	return 0, "", errors.New("unable to calibrate KOSPI proxy PBR scale from inquire-price")
}

func parseKOSPIMaster(raw []byte) ([]kospiMasterRecord, error) {
	reader := transform.NewReader(bytes.NewReader(raw), korean.EUCKR.NewDecoder())
	scanner := bufio.NewScanner(reader)

	records := make([]kospiMasterRecord, 0)
	for scanner.Scan() {
		record, ok, err := parseKOSPIMasterLine(scanner.Text())
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		records = append(records, record)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return records, nil
}

func parseKOSPIMasterLine(line string) (kospiMasterRecord, bool, error) {
	runes := []rune(strings.TrimRight(line, "\r\n"))
	if len(runes) < kospiPart2Width {
		return kospiMasterRecord{}, false, nil
	}

	part1 := runes[:len(runes)-kospiPart2Width]
	part2 := runes[len(runes)-kospiPart2Width:]
	if len(part1) < 21 {
		return kospiMasterRecord{}, false, nil
	}

	code := strings.TrimSpace(string(part1[:9]))
	name := strings.TrimSpace(string(part1[21:]))
	if code == "" || name == "" {
		return kospiMasterRecord{}, false, nil
	}

	fields, err := splitFixedWidthFields(part2, kospiPart2FieldWidths)
	if err != nil {
		return kospiMasterRecord{}, false, err
	}
	if len(fields) < 70 {
		return kospiMasterRecord{}, false, fmt.Errorf("unexpected KOSPI master field count: %d", len(fields))
	}

	if strings.ToUpper(strings.TrimSpace(fields[58])) != "Y" {
		return kospiMasterRecord{}, false, nil
	}

	marketCap, ok := parseFloat(fields[65])
	if !ok || marketCap <= 0 {
		return kospiMasterRecord{}, false, nil
	}

	netIncome, ok := parseFloat(fields[62])
	if !ok {
		return kospiMasterRecord{}, false, nil
	}

	roe, ok := parseFloat(fields[63])
	if !ok {
		return kospiMasterRecord{}, false, nil
	}

	return kospiMasterRecord{
		Code:      normalizeShortCode(code),
		Name:      name,
		MarketCap: marketCap,
		NetIncome: netIncome,
		ROE:       roe,
		BaseDate:  strings.TrimSpace(fields[64]),
	}, true, nil
}

func splitFixedWidthFields(runes []rune, widths []int) ([]string, error) {
	fields := make([]string, 0, len(widths))
	offset := 0
	for _, width := range widths {
		if offset+width > len(runes) {
			return nil, errors.New("fixed-width data shorter than expected")
		}
		fields = append(fields, strings.TrimSpace(string(runes[offset:offset+width])))
		offset += width
	}
	return fields, nil
}

func normalizeShortCode(code string) string {
	code = strings.TrimSpace(code)
	code = strings.TrimPrefix(code, "A")
	if len(code) > 6 {
		code = code[len(code)-6:]
	}
	return code
}

func unzipSingleFile(zipBytes []byte, targetName string) ([]byte, error) {
	readerAt := bytes.NewReader(zipBytes)
	zipReader, err := zip.NewReader(readerAt, int64(len(zipBytes)))
	if err != nil {
		return nil, err
	}

	for _, file := range zipReader.File {
		if !strings.EqualFold(file.Name, targetName) {
			continue
		}

		fileReader, err := file.Open()
		if err != nil {
			return nil, err
		}
		defer fileReader.Close()

		return io.ReadAll(fileReader)
	}

	return nil, fmt.Errorf("%s not found in zip archive", targetName)
}

func proxyBookEquity(record kospiMasterRecord, scale float64) (float64, float64, bool) {
	if record.MarketCap <= 0 || record.NetIncome <= 0 || record.ROE <= 0 || scale <= 0 {
		return 0, 0, false
	}

	bookEquity := (record.NetIncome * scale) / (record.ROE / 100)
	if bookEquity <= 0 || math.IsNaN(bookEquity) || math.IsInf(bookEquity, 0) {
		return 0, 0, false
	}

	return bookEquity, record.MarketCap / bookEquity, true
}

func firstOutputRow(resp *auth.RESTResponse, outputKey string) map[string]any {
	if resp == nil {
		return nil
	}

	rows := toRows(resp.Body[outputKey])
	if len(rows) == 0 {
		return nil
	}
	return rows[0]
}

func newRequestPacer(rpm int) *requestPacer {
	if rpm <= 0 {
		return &requestPacer{}
	}

	return &requestPacer{
		interval: time.Minute / time.Duration(rpm),
	}
}

func (p *requestPacer) Wait(ctx context.Context) (time.Duration, error) {
	if p == nil || p.interval <= 0 {
		return 0, nil
	}

	now := time.Now()
	if p.nextAllowed.After(now) {
		waitDuration := p.nextAllowed.Sub(now)
		timer := time.NewTimer(waitDuration)
		defer timer.Stop()

		select {
		case <-ctx.Done():
			return waitDuration, ctx.Err()
		case <-timer.C:
		}

		p.nextAllowed = time.Now().Add(p.interval)
		return waitDuration, nil
	}

	p.nextAllowed = time.Now().Add(p.interval)
	return 0, nil
}

func getEnvInt(key string, defaultValue int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return defaultValue
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}
	return parsed
}

func getOrDefaultEnv(key string, defaultValue string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return defaultValue
	}
	return value
}

func actualPBRDebugEnabled() bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(kospiActualPBRDebugEnvKey)))
	switch value {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func actualPBRProgressEnabled() bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(kospiActualPBRProgressEnvKey)))
	switch value {
	case "", "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return true
	}
}

func actualPBRProgressLog(enabled bool, format string, args ...any) {
	if !enabled {
		return
	}
	log.Printf("[KOSPIActualPBR] "+format, args...)
}

func actualPBRLog(enabled bool, format string, args ...any) {
	if !enabled {
		return
	}
	log.Printf("[KOSPIActualPBR] "+format, args...)
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

func isYYYYMMDD(value string) bool {
	if len(value) != 8 {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
