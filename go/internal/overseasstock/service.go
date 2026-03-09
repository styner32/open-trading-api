package overseasstock

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/kis-open-api/go/internal/auth"
)

const (
	pricePath              = "/uapi/overseas-price/v1/quotations/price"
	priceTRID              = "HHDFS00000300"
	ewyExchangeCodeEnvKey  = "EWY_EXCD"
	defaultEWYSymbol       = "EWY"
	defaultUSMasterBaseURL = "https://new.real.download.dws.co.kr/common/master/%smst.cod.zip"
)

type Service struct {
	client *auth.KIClient
}

type MasterMarket struct {
	FilePrefix   string
	ExchangeCode string
}

type MasterRecord struct {
	MarketCode   string
	ExchangeCode string
	Symbol       string
	KoreaName    string
	EnglishName  string
}

var usMarkets = []MasterMarket{
	{FilePrefix: "nas", ExchangeCode: "NAS"},
	{FilePrefix: "nys", ExchangeCode: "NYS"},
	{FilePrefix: "ams", ExchangeCode: "AMS"},
}

func NewService(client *auth.KIClient) *Service {
	return &Service{client: client}
}

func (s *Service) Price(ctx context.Context, exchangeCode string, symbol string) (*auth.RESTResponse, error) {
	exchangeCode = strings.TrimSpace(exchangeCode)
	symbol = strings.TrimSpace(symbol)
	if exchangeCode == "" {
		return nil, errors.New("exchangeCode is required")
	}
	if symbol == "" {
		return nil, errors.New("symbol is required")
	}

	params := map[string]string{
		"AUTH": "",
		"EXCD": exchangeCode,
		"SYMB": symbol,
	}

	return s.client.Get(ctx, pricePath, priceTRID, "", params)
}

func (s *Service) ResolveEWYExchangeCode(ctx context.Context) (string, error) {
	if override := strings.TrimSpace(os.Getenv(ewyExchangeCodeEnvKey)); override != "" {
		return override, nil
	}

	return s.ResolveUSExchangeCode(ctx, defaultEWYSymbol)
}

func (s *Service) ResolveUSExchangeCode(ctx context.Context, symbol string) (string, error) {
	record, err := s.FindUSSymbol(ctx, symbol)
	if err != nil {
		return "", err
	}

	return record.ExchangeCode, nil
}

func (s *Service) FindUSSymbol(ctx context.Context, symbol string) (*MasterRecord, error) {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol == "" {
		return nil, errors.New("symbol is required")
	}

	for _, market := range usMarkets {
		record, err := s.findSymbolInMarket(ctx, market, symbol)
		if err != nil {
			return nil, err
		}
		if record != nil {
			return record, nil
		}
	}

	return nil, fmt.Errorf("symbol %s not found in US market master files", symbol)
}

func (s *Service) findSymbolInMarket(ctx context.Context, market MasterMarket, symbol string) (*MasterRecord, error) {
	rawZip, err := s.downloadMaster(ctx, market)
	if err != nil {
		return nil, err
	}

	rawFile, err := extractStockMaster(rawZip, market.FilePrefix+"mst.cod")
	if err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(bytes.NewReader(rawFile))
	for scanner.Scan() {
		record, ok := parseStockMasterLine(scanner.Text())
		if !ok {
			continue
		}
		if !strings.EqualFold(record.Symbol, symbol) {
			continue
		}

		record.MarketCode = market.FilePrefix
		if record.ExchangeCode == "" {
			record.ExchangeCode = market.ExchangeCode
		}
		return &record, nil
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return nil, nil
}

func (s *Service) downloadMaster(ctx context.Context, market MasterMarket) ([]byte, error) {
	url := fmt.Sprintf(defaultUSMasterBaseURL, market.FilePrefix)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	httpClient := http.DefaultClient
	if s.client != nil && s.client.Client != nil {
		httpClient = s.client.Client
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s master download failed (status %d)", market.FilePrefix, resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

func extractStockMaster(rawZip []byte, fileName string) ([]byte, error) {
	readerAt := bytes.NewReader(rawZip)
	zipReader, err := zip.NewReader(readerAt, int64(len(rawZip)))
	if err != nil {
		return nil, err
	}

	for _, file := range zipReader.File {
		if !strings.EqualFold(file.Name, fileName) {
			continue
		}

		fileReader, err := file.Open()
		if err != nil {
			return nil, err
		}
		defer fileReader.Close()

		return io.ReadAll(fileReader)
	}

	return nil, fmt.Errorf("%s not found in stock master archive", fileName)
}

func parseStockMasterLine(line string) (MasterRecord, bool) {
	fields := strings.Split(line, "\t")
	if len(fields) < 8 {
		return MasterRecord{}, false
	}

	symbol := strings.TrimSpace(fields[4])
	if symbol == "" {
		return MasterRecord{}, false
	}

	return MasterRecord{
		ExchangeCode: strings.TrimSpace(fields[2]),
		Symbol:       symbol,
		KoreaName:    strings.TrimSpace(fields[6]),
		EnglishName:  strings.TrimSpace(fields[7]),
	}, true
}
