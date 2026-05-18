package snapshot

import (
	"context"
	"fmt"
	"math"
	"strings"
)

type ImpactSection struct {
	// Fix 2: 거래대금 기반 외인 비중 (주요 지표)
	ForeignSellTradingValuePercent *float64
	ForeignSellTradingValueLabel   string // "정상"/"주의"/"위험"
	ForeignSellTradingValueReason  string
	// 시총 대비 (참고용 유지)
	ForeignSellMarketCapPercent *float64
	ForeignSellReason           string
	// 반도체
	SemiconductorSellConcentrationPct *float64
	SemiconductorReason               string
	// Fix 3: 선물 + 베이시스
	FuturesChangePercent *float64
	FuturesPrice         *float64
	FuturesCode          string
	FuturesReason        string
	BasisPoints          *float64 // 선물가 - 코스피200 지수
	BasisPercent         *float64 // 베이시스율
	BasisReason          string
	// 사이드카
	SidecarStatus string
	SidecarTime   string
}

// collectImpact computes trading-value sell pressure and semiconductor concentration.
// price is used for KOSPI daily trading value (Fix 2).
func collectImpact(ctx context.Context, deps Deps, date string, flow *FlowSection, price *PriceSection, opts Options) *ImpactSection {
	section := &ImpactSection{SidecarStatus: normalizeSidecar(opts.SidecarStatus), SidecarTime: strings.TrimSpace(opts.SidecarTime)}
	if flow == nil {
		section.ForeignSellReason = "flow section unavailable"
		section.ForeignSellTradingValueReason = "flow section unavailable"
		section.SemiconductorReason = "flow section unavailable"
	} else {
		// Fix 2: 거래대금 분모
		if price != nil && price.TradingValueEok > 0 {
			pct := math.Abs(flow.ForeignEok) / price.TradingValueEok * 100
			section.ForeignSellTradingValuePercent = ptr(pct)
			section.ForeignSellTradingValueLabel = tradingValueLabel(pct)
		} else {
			section.ForeignSellTradingValueReason = "trading value unavailable"
		}
		// 시총 대비 (참고)
		if deps.DomesticStock == nil {
			section.ForeignSellReason = "domestic stock dependency is nil"
		} else if summary, err := deps.DomesticStock.KOSPIMarketCapSummary(ctx, date); err != nil {
			section.ForeignSellReason = err.Error()
		} else {
			section.ForeignSellMarketCapPercent = absPercent(flow.ForeignEok, summary.TotalMarketCap)
		}
		// 반도체
		if opts.SemiconductorForeignNetSellEok != nil {
			section.SemiconductorSellConcentrationPct = absPercent(*opts.SemiconductorForeignNetSellEok, flow.ForeignEok)
		} else {
			section.SemiconductorReason = "manual input not provided"
		}
	}
	section.collectFutures(ctx, deps.DomesticFuture, date)
	// Fix 3: 베이시스 계산
	section.collectBasis(ctx, deps.DomesticStock)
	return section
}

func tradingValueLabel(pct float64) string {
	switch {
	case pct < 10:
		return "정상"
	case pct < 20:
		return "주의"
	default:
		return "위험"
	}
}

func (s *ImpactSection) collectFutures(ctx context.Context, futures DomesticFuture, date string) {
	if futures == nil {
		s.FuturesReason = "domestic future dependency is nil"
		return
	}
	resolved, err := futures.ResolveNearMonthKOSPI200Futures(ctx, date)
	if err != nil {
		s.FuturesReason = err.Error()
		return
	}
	s.FuturesCode = resolved.Record.ShortCode
	resp, err := futures.InquirePrice(ctx, "F", resolved.Record.ShortCode)
	if err != nil {
		s.FuturesReason = err.Error()
		return
	}
	row := firstRow(resp, "output1", "output")
	if row == nil {
		s.FuturesReason = "futures price output missing"
		return
	}
	if value, ok := num(row, "futs_prdy_ctrt", "bstp_nmix_prdy_ctrt"); ok {
		s.FuturesChangePercent = ptr(value)
	} else {
		s.FuturesReason = "futs_prdy_ctrt missing"
	}
	// Fix 3: 선물 실제가 추출
	if price, ok := num(row, "futs_prpr", "bstp_nmix_prpr"); ok {
		s.FuturesPrice = ptr(price)
	}
}

func (s *ImpactSection) collectBasis(ctx context.Context, stock DomesticStock) {
	if s.FuturesPrice == nil {
		s.BasisReason = "futures price unavailable"
		return
	}
	if stock == nil {
		s.BasisReason = "domestic stock dependency is nil"
		return
	}
	resp, err := stock.InquireIndexPrice(ctx, "0002")
	if err != nil {
		s.BasisReason = fmt.Sprintf("KOSPI200 index: %v", err)
		return
	}
	row := firstRow(resp, "output")
	if row == nil {
		s.BasisReason = "KOSPI200 output missing"
		return
	}
	indexPrice, ok := num(row, "bstp_nmix_prpr", "stck_prpr")
	if !ok || indexPrice <= 0 {
		s.BasisReason = "KOSPI200 price missing"
		return
	}
	basis := *s.FuturesPrice - indexPrice
	s.BasisPoints = ptr(basis)
	s.BasisPercent = ptr(basis / indexPrice * 100)
}

func normalizeSidecar(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "triggered", "not-triggered":
		return strings.ToLower(strings.TrimSpace(value))
	case "", "unknown":
		return "unknown"
	default:
		return fmt.Sprintf("unknown (%s)", strings.TrimSpace(value))
	}
}
