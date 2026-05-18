package snapshot

import (
	"context"
	"fmt"
)

type PriceSection struct {
	Date              string
	Open              float64
	High              float64
	Low               float64
	Close             float64
	PreviousClose     float64
	RangePoints       float64
	RangePercent      float64
	YearHigh          bool
	TradingValueEok   float64 // 일간 총 거래대금 (억원), 0 = 미확인
}

// collectPrice computes intraday range as high-low and range percent as
// (high-low)/previous close*100.
func collectPrice(ctx context.Context, stock DomesticStock, date string) (*PriceSection, error) {
	if stock == nil {
		return nil, fmt.Errorf("domestic stock dependency is nil")
	}
	dailyRows, err := stock.InquireIndexDailyPrice(ctx, "0001", date)
	if err == nil {
		if section, ok := priceFromRows(dailyRows, date); ok {
			return section, nil
		}
	}
	today, _ := normalizeDate("")
	if date != today {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("KOSPI daily row not found for %s", date)
	}
	resp, fallbackErr := stock.InquireIndexPrice(ctx, "0001")
	if fallbackErr != nil {
		if err != nil {
			return nil, fmt.Errorf("%v; fallback: %w", err, fallbackErr)
		}
		return nil, fallbackErr
	}
	if section, ok := priceFromRow(firstRow(resp, "output"), date); ok {
		return section, nil
	}
	return nil, fmt.Errorf("KOSPI index price output missing")
}

func priceFromRows(dailyRows []map[string]any, date string) (*PriceSection, bool) {
	for _, row := range dailyRows {
		if fmt.Sprintf("%v", row["stck_bsop_date"]) == date {
			return priceFromRow(row, date)
		}
	}
	return nil, false
}

func priceFromRow(row map[string]any, date string) (*PriceSection, bool) {
	closeValue, closeOK := num(row, "bstp_nmix_prpr", "stck_clpr", "stck_prpr")
	openValue, openOK := num(row, "bstp_nmix_oprc", "stck_oprc")
	highValue, highOK := num(row, "bstp_nmix_hgpr", "stck_hgpr")
	lowValue, lowOK := num(row, "bstp_nmix_lwpr", "stck_lwpr")
	if !closeOK || !openOK || !highOK || !lowOK {
		return nil, false
	}
	prevClose, ok := num(row, "stck_prdy_clpr", "bstp_nmix_prdy_clpr")
	if !ok {
		if diff, diffOK := num(row, "bstp_nmix_prdy_vrss", "prdy_vrss"); diffOK {
			prevClose = closeValue - diff
		}
	}
	if prevClose <= 0 {
		return nil, false
	}
	yearHigh, _ := num(row, "dryy_bstp_nmix_hgpr")
	rangePoints := highValue - lowValue
	// acml_tr_pbmn: 누적거래대금 (백만원), divide by 100 → 억원
	tradingMillionKRW, _ := num(row, "acml_tr_pbmn")
	return &PriceSection{
		Date: date, Open: openValue, High: highValue, Low: lowValue,
		Close: closeValue, PreviousClose: prevClose, RangePoints: rangePoints,
		RangePercent: rangePoints / prevClose * 100, YearHigh: yearHigh > 0 && highValue >= yearHigh,
		TradingValueEok: tradingMillionKRW / tradeAmountMillionToEok,
	}, true
}
