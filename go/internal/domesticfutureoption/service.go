package domesticfutureoption

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/kis-open-api/go/internal/auth"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/transform"
)

const (
	inquirePricePath                = "/uapi/domestic-futureoption/v1/quotations/inquire-price"
	inquireTimeFuopCCNLPath         = "/uapi/domestic-futureoption/v1/quotations/inquire-time-fuopccnl"
	inquireMemberPath               = "/uapi/domestic-futureoption/v1/quotations/inquire-member"
	inquireTimeFuopChartPricePath   = "/uapi/domestic-futureoption/v1/quotations/inquire-time-fuopchartprice"
	displayBoardTopPath             = "/uapi/domestic-futureoption/v1/quotations/display-board-top"
	displayBoardFuturesPath         = "/uapi/domestic-futureoption/v1/quotations/display-board-futures"
	expPriceTrendPath               = "/uapi/domestic-futureoption/v1/quotations/exp-price-trend"
	indexFutureMasterDownloadURL    = "https://new.real.download.dws.co.kr/common/master/fo_idx_code_mts.mst.zip"
	indexFutureMasterFilename       = "fo_idx_code_mts.mst"
	defaultIndexFutureMasterCache   = ".cache/fo_idx_code_mts.mst"
	quadWitchingFuturesCodeEnvKey   = "QUAD_WITCHING_FUTURES_CODE"
	indexFutureMasterCacheEnvKey    = "INDEX_FUTURE_MASTER_CACHE_FILE"
	defaultFuturesMarketDivCode     = "F"
	defaultBoardScreenCode          = "20503"
	defaultBoardMarketClassCode     = "MKI"
	defaultTimeChartHourClassCode   = "60"
	defaultTimeChartIncludePastData = "Y"
	defaultTimeChartIncludeFakeTick = "N"
)

const (
	inquirePriceTRID              = "FHMIF10000000"
	inquireTimeFuopCCNLTRID       = "FHMIF10020000"
	inquireMemberTRID             = "FHMIF10070000"
	inquireTimeFuopChartPriceTRID = "FHKIF03020200"
	displayBoardTopTRID           = "FHPIF05030000"
	displayBoardFuturesTRID       = "FHPIF05030200"
	expPriceTrendTRID             = "FHPIF05110100"
)

type Service struct {
	client *auth.KIClient
}

type MasterRecord struct {
	InfoType            string `json:"info_type"`
	ShortCode           string `json:"short_code"`
	StandardCode        string `json:"standard_code"`
	Name                string `json:"name"`
	ATMClassCode        string `json:"atm_class_code"`
	StrikePrice         string `json:"strike_price"`
	MonthClassCode      string `json:"month_class_code"`
	UnderlyingShortCode string `json:"underlying_short_code"`
	UnderlyingName      string `json:"underlying_name"`
}

type ResolvedContract struct {
	BusinessDate    string       `json:"business_date"`
	Source          string       `json:"source"`
	MasterCachePath string       `json:"master_cache_path,omitempty"`
	MasterJSONPath  string       `json:"master_json_path,omitempty"`
	Record          MasterRecord `json:"record"`
}

type masterJSONField struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type masterJSONCache struct {
	BusinessDate string            `json:"business_date"`
	GeneratedAt  string            `json:"generated_at"`
	SourcePath   string            `json:"source_path"`
	RecordCount  int               `json:"record_count"`
	Fields       []masterJSONField `json:"fields"`
	Records      []MasterRecord    `json:"records"`
}

func NewService(client *auth.KIClient) *Service {
	return &Service{client: client}
}

func (s *Service) InquirePrice(ctx context.Context, marketDivCode string, inputISCD string) (*auth.RESTResponse, error) {
	marketDivCode = strings.TrimSpace(marketDivCode)
	inputISCD = strings.TrimSpace(inputISCD)
	if marketDivCode == "" {
		marketDivCode = defaultFuturesMarketDivCode
	}
	if inputISCD == "" {
		return nil, errors.New("inputISCD is required")
	}

	params := map[string]string{
		"FID_COND_MRKT_DIV_CODE": marketDivCode,
		"FID_INPUT_ISCD":         inputISCD,
	}
	return s.client.Get(ctx, inquirePricePath, inquirePriceTRID, "", params)
}

func (s *Service) InquireTimeFuopCCNL(ctx context.Context, marketDivCode string, inputISCD string) (*auth.RESTResponse, error) {
	params, err := futureCodeParams(marketDivCode, inputISCD)
	if err != nil {
		return nil, err
	}
	return s.client.Get(ctx, inquireTimeFuopCCNLPath, inquireTimeFuopCCNLTRID, "", params)
}

func (s *Service) InquireMember(ctx context.Context, marketDivCode string, inputISCD string) (*auth.RESTResponse, error) {
	params, err := futureCodeParams(marketDivCode, inputISCD)
	if err != nil {
		return nil, err
	}
	return s.client.Get(ctx, inquireMemberPath, inquireMemberTRID, "", params)
}

func (s *Service) InquireTimeFuopChartPrice(
	ctx context.Context,
	marketDivCode string,
	inputISCD string,
	hourClsCode string,
	includePastData string,
	includeFakeTick string,
	inputDate string,
	inputHour string,
) (*auth.RESTResponse, error) {
	marketDivCode = strings.TrimSpace(marketDivCode)
	inputISCD = strings.TrimSpace(inputISCD)
	hourClsCode = strings.TrimSpace(hourClsCode)
	includePastData = strings.TrimSpace(includePastData)
	includeFakeTick = strings.TrimSpace(includeFakeTick)
	inputDate = strings.TrimSpace(inputDate)
	inputHour = strings.TrimSpace(inputHour)

	if marketDivCode == "" {
		marketDivCode = defaultFuturesMarketDivCode
	}
	if inputISCD == "" {
		return nil, errors.New("inputISCD is required")
	}
	if hourClsCode == "" {
		hourClsCode = defaultTimeChartHourClassCode
	}
	if includePastData == "" {
		includePastData = defaultTimeChartIncludePastData
	}
	if includeFakeTick == "" {
		includeFakeTick = defaultTimeChartIncludeFakeTick
	}
	if inputDate == "" {
		inputDate = time.Now().Format("20060102")
	}
	if inputHour == "" {
		inputHour = time.Now().Format("150405")
	}

	params := map[string]string{
		"FID_COND_MRKT_DIV_CODE": marketDivCode,
		"FID_INPUT_ISCD":         inputISCD,
		"FID_HOUR_CLS_CODE":      hourClsCode,
		"FID_PW_DATA_INCU_YN":    includePastData,
		"FID_FAKE_TICK_INCU_YN":  includeFakeTick,
		"FID_INPUT_DATE_1":       inputDate,
		"FID_INPUT_HOUR_1":       inputHour,
	}
	return s.client.Get(ctx, inquireTimeFuopChartPricePath, inquireTimeFuopChartPriceTRID, "", params)
}

func (s *Service) DisplayBoardTop(
	ctx context.Context,
	marketDivCode string,
	inputISCD string,
	condMarketDivCode1 string,
	screenDivCode string,
	maturityCount string,
	condMarketClassCode string,
) (*auth.RESTResponse, error) {
	marketDivCode = strings.TrimSpace(marketDivCode)
	inputISCD = strings.TrimSpace(inputISCD)
	if marketDivCode == "" {
		marketDivCode = defaultFuturesMarketDivCode
	}
	if inputISCD == "" {
		return nil, errors.New("inputISCD is required")
	}

	params := map[string]string{
		"FID_COND_MRKT_DIV_CODE":  marketDivCode,
		"FID_INPUT_ISCD":          inputISCD,
		"FID_COND_MRKT_DIV_CODE1": strings.TrimSpace(condMarketDivCode1),
		"FID_COND_SCR_DIV_CODE":   strings.TrimSpace(screenDivCode),
		"FID_MTRT_CNT":            strings.TrimSpace(maturityCount),
		"FID_COND_MRKT_CLS_CODE":  strings.TrimSpace(condMarketClassCode),
	}
	return s.client.Get(ctx, displayBoardTopPath, displayBoardTopTRID, "", params)
}

func (s *Service) DisplayBoardFutures(ctx context.Context, marketDivCode string, screenDivCode string, marketClassCode string) (*auth.RESTResponse, error) {
	marketDivCode = strings.TrimSpace(marketDivCode)
	screenDivCode = strings.TrimSpace(screenDivCode)
	marketClassCode = strings.TrimSpace(marketClassCode)
	if marketDivCode == "" {
		marketDivCode = defaultFuturesMarketDivCode
	}
	if screenDivCode == "" {
		screenDivCode = defaultBoardScreenCode
	}
	if marketClassCode == "" {
		marketClassCode = defaultBoardMarketClassCode
	}

	params := map[string]string{
		"FID_COND_MRKT_DIV_CODE": marketDivCode,
		"FID_COND_SCR_DIV_CODE":  screenDivCode,
		"FID_COND_MRKT_CLS_CODE": marketClassCode,
	}
	return s.client.Get(ctx, displayBoardFuturesPath, displayBoardFuturesTRID, "", params)
}

func (s *Service) ExpPriceTrend(ctx context.Context, inputISCD string, marketDivCode string) (*auth.RESTResponse, error) {
	marketDivCode = strings.TrimSpace(marketDivCode)
	inputISCD = strings.TrimSpace(inputISCD)
	if marketDivCode == "" {
		marketDivCode = defaultFuturesMarketDivCode
	}
	if inputISCD == "" {
		return nil, errors.New("inputISCD is required")
	}

	params := map[string]string{
		"FID_INPUT_ISCD":         inputISCD,
		"FID_COND_MRKT_DIV_CODE": marketDivCode,
	}
	return s.client.Get(ctx, expPriceTrendPath, expPriceTrendTRID, "", params)
}

func (s *Service) ResolveNearMonthKOSPI200Futures(ctx context.Context, businessDate string) (*ResolvedContract, error) {
	override := strings.TrimSpace(os.Getenv(quadWitchingFuturesCodeEnvKey))
	if override != "" {
		return &ResolvedContract{
			BusinessDate: normalizeBusinessDate(businessDate),
			Source:       "env",
			Record: MasterRecord{
				ShortCode: override,
				Name:      "ENV override",
			},
		}, nil
	}

	records, err := s.LoadIndexFutureMaster(ctx, businessDate)
	if err != nil {
		return nil, err
	}

	record, err := selectNearMonthKOSPI200Future(records)
	if err != nil {
		return nil, err
	}

	cachePath := resolveIndexFutureMasterCachePath(getOrDefaultEnv(indexFutureMasterCacheEnvKey, defaultIndexFutureMasterCache), businessDate)
	return &ResolvedContract{
		BusinessDate:    normalizeBusinessDate(businessDate),
		Source:          "master",
		MasterCachePath: cachePath,
		MasterJSONPath:  resolveIndexFutureMasterJSONPath(cachePath),
		Record:          record,
	}, nil
}

func (s *Service) LoadIndexFutureMaster(ctx context.Context, businessDate string) ([]MasterRecord, error) {
	cachePath := resolveIndexFutureMasterCachePath(getOrDefaultEnv(indexFutureMasterCacheEnvKey, defaultIndexFutureMasterCache), businessDate)
	if err := s.ensureIndexFutureMasterCache(ctx, cachePath); err != nil {
		return nil, err
	}

	raw, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read index future master cache: %w", err)
	}

	records, err := parseIndexFutureMaster(raw)
	if err != nil {
		return nil, err
	}

	if err := ensureIndexFutureMasterJSONCache(cachePath, businessDate, records); err != nil {
		return nil, err
	}

	return records, nil
}

func (s *Service) ensureIndexFutureMasterCache(ctx context.Context, cachePath string) error {
	_, err := os.Stat(cachePath)
	if err == nil {
		return nil
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to stat index future master cache: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, indexFutureMasterDownloadURL, nil)
	if err != nil {
		return err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download index future master zip: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("index future master download failed (status %d)", resp.StatusCode)
	}

	zipBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	masterBytes, err := unzipSingleFile(zipBytes, indexFutureMasterFilename)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		return fmt.Errorf("failed to create index future master cache dir: %w", err)
	}

	tmpPath := cachePath + ".tmp"
	if err := os.WriteFile(tmpPath, masterBytes, 0o644); err != nil {
		return fmt.Errorf("failed to write index future master temp file: %w", err)
	}
	if err := os.Rename(tmpPath, cachePath); err != nil {
		return fmt.Errorf("failed to replace index future master cache: %w", err)
	}

	return nil
}

func parseIndexFutureMaster(raw []byte) ([]MasterRecord, error) {
	reader := transform.NewReader(bytes.NewReader(raw), korean.EUCKR.NewDecoder())
	scanner := bufio.NewScanner(reader)

	records := make([]MasterRecord, 0, 512)
	for scanner.Scan() {
		record, ok := parseIndexFutureMasterLine(scanner.Text())
		if !ok {
			continue
		}
		records = append(records, record)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if len(records) == 0 {
		return nil, errors.New("index future master did not contain any parsable records")
	}

	return records, nil
}

func parseIndexFutureMasterLine(line string) (MasterRecord, bool) {
	parts := strings.Split(strings.TrimRight(line, "\r\n"), "|")
	if len(parts) < 9 {
		return MasterRecord{}, false
	}

	record := MasterRecord{
		InfoType:            strings.TrimSpace(parts[0]),
		ShortCode:           strings.TrimSpace(parts[1]),
		StandardCode:        strings.TrimSpace(parts[2]),
		Name:                strings.TrimSpace(parts[3]),
		ATMClassCode:        strings.TrimSpace(parts[4]),
		StrikePrice:         strings.TrimSpace(parts[5]),
		MonthClassCode:      strings.TrimSpace(parts[6]),
		UnderlyingShortCode: strings.TrimSpace(parts[7]),
		UnderlyingName:      strings.TrimSpace(parts[8]),
	}

	if record.ShortCode == "" || record.Name == "" {
		return MasterRecord{}, false
	}

	return record, true
}

func selectNearMonthKOSPI200Future(records []MasterRecord) (MasterRecord, error) {
	candidates := make([]MasterRecord, 0, len(records))
	for _, record := range records {
		if record.ShortCode == "" {
			continue
		}
		if strings.TrimSpace(record.InfoType) != "1" {
			continue
		}
		if !strings.HasPrefix(record.ShortCode, "101") && !strings.Contains(strings.ToUpper(record.UnderlyingName), "KOSPI") {
			continue
		}
		candidates = append(candidates, record)
	}

	if len(candidates) == 0 {
		return MasterRecord{}, errors.New("unable to find KOSPI200 index futures contract in master file")
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		leftScore := futureCandidateScore(candidates[i])
		rightScore := futureCandidateScore(candidates[j])
		if leftScore != rightScore {
			return leftScore > rightScore
		}
		if candidates[i].MonthClassCode != candidates[j].MonthClassCode {
			return candidates[i].MonthClassCode < candidates[j].MonthClassCode
		}
		return candidates[i].ShortCode < candidates[j].ShortCode
	})

	return candidates[0], nil
}

func futureCandidateScore(record MasterRecord) int {
	score := 0
	if record.InfoType == "1" {
		score += 100
	}
	if strings.HasPrefix(record.ShortCode, "101") {
		score += 50
	}
	switch record.MonthClassCode {
	case "1":
		score += 30
	case "2":
		score += 20
	case "3":
		score += 10
	}
	if strings.Contains(strings.ToUpper(record.UnderlyingName), "KOSPI") {
		score += 5
	}
	return score
}

func resolveIndexFutureMasterCachePath(cachePath string, businessDate string) string {
	return resolveBusinessDateCachePath(cachePath, businessDate, defaultIndexFutureMasterCache)
}

func resolveIndexFutureMasterJSONPath(cachePath string) string {
	cachePath = strings.TrimSpace(cachePath)
	if cachePath == "" {
		cachePath = defaultIndexFutureMasterCache
	}

	ext := filepath.Ext(cachePath)
	if ext == "" {
		return cachePath + ".json"
	}
	return strings.TrimSuffix(cachePath, ext) + ".json"
}

func resolveBusinessDateCachePath(cachePath string, businessDate string, defaultPath string) string {
	cachePath = strings.TrimSpace(cachePath)
	if cachePath == "" {
		cachePath = defaultPath
	}

	businessDate = normalizeBusinessDate(businessDate)
	ext := filepath.Ext(cachePath)
	if ext == "" {
		return cachePath + "." + businessDate
	}

	base := strings.TrimSuffix(cachePath, ext)
	return base + "." + businessDate + ext
}

func normalizeBusinessDate(value string) string {
	value = strings.TrimSpace(value)
	if len(value) != 8 {
		return time.Now().Format("20060102")
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return time.Now().Format("20060102")
		}
	}
	return value
}

func ensureIndexFutureMasterJSONCache(cachePath string, businessDate string, records []MasterRecord) error {
	jsonPath := resolveIndexFutureMasterJSONPath(cachePath)

	masterInfo, err := os.Stat(cachePath)
	if err != nil {
		return fmt.Errorf("failed to stat index future master cache for json export: %w", err)
	}

	if jsonInfo, err := os.Stat(jsonPath); err == nil && !masterInfo.ModTime().After(jsonInfo.ModTime()) {
		return nil
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to stat index future master json cache: %w", err)
	}

	payload := masterJSONCache{
		BusinessDate: normalizeBusinessDate(businessDate),
		GeneratedAt:  time.Now().Format(time.RFC3339),
		SourcePath:   cachePath,
		RecordCount:  len(records),
		Fields: []masterJSONField{
			{Name: "info_type", Description: "1=지수선물, 5=지수콜옵션, 6=지수풋옵션 등"},
			{Name: "short_code", Description: "단축 종목코드"},
			{Name: "standard_code", Description: "표준 종목코드"},
			{Name: "name", Description: "한글 종목명"},
			{Name: "month_class_code", Description: "1=최근월물, 2=차근월물"},
			{Name: "underlying_short_code", Description: "기초자산 단축코드"},
			{Name: "underlying_name", Description: "기초자산명"},
		},
		Records: records,
	}

	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal index future master json cache: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(jsonPath), 0o755); err != nil {
		return fmt.Errorf("failed to create index future master json dir: %w", err)
	}

	tmpPath := jsonPath + ".tmp"
	if err := os.WriteFile(tmpPath, raw, 0o644); err != nil {
		return fmt.Errorf("failed to write index future master json temp file: %w", err)
	}
	if err := os.Rename(tmpPath, jsonPath); err != nil {
		return fmt.Errorf("failed to replace index future master json cache: %w", err)
	}

	return nil
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

func getOrDefaultEnv(key string, defaultValue string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return defaultValue
	}
	return value
}

func futureCodeParams(marketDivCode string, inputISCD string) (map[string]string, error) {
	marketDivCode = strings.TrimSpace(marketDivCode)
	inputISCD = strings.TrimSpace(inputISCD)
	if inputISCD == "" {
		return nil, errors.New("inputISCD is required")
	}

	params := map[string]string{
		"FID_INPUT_ISCD": inputISCD,
	}
	if marketDivCode != "" {
		params["FID_COND_MRKT_DIV_CODE"] = marketDivCode
	}
	return params, nil
}
