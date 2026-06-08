package snapshot

import (
	"context"
	"math"

	"github.com/kis-open-api/go/internal/external/yahoo"
)

// RegimeSection은 매크로 채널/시장 국면 분류를 담습니다.
type RegimeSection struct {
	Phase           string  // "지정학 risk-off", "성장 둔화 risk-off" 등
	KOSPINASDAQCorr float64 // KOSPI-NASDAQ 30일 상관계수
	KOSPINIKKEICorr float64 // KOSPI-NIKKEI 30일 상관계수
	RiskAversionIdx float64 // 위험회피지수 0~10
	Reason          string
}

// collectRegime은 Yahoo 30일 히스토리로 상관계수와 시장 국면을 계산합니다.
// TODO: Yahoo Finance 장애 시 대체 제공업체(EODHD 등) 연동 검토.
func collectRegime(ctx context.Context, yClient YahooQuotes, volatility *VolatilitySection, impact *ImpactSection, macro *MacroSection) *RegimeSection {
	s := &RegimeSection{}
	if yClient == nil {
		s.Reason = "yahoo dependency is nil"
		return s
	}

	// 30일 일별 종가 조회
	symbols := []string{"^KS11", "NQ=F", "^N225", "CL=F", "KRW=X"}
	history := map[string][]float64{}
	var firstErr string
	for _, sym := range symbols {
		closes, err := yClient.GetChartHistory(ctx, sym, "1mo", "1d")
		if err != nil || len(closes) < 5 {
			if firstErr == "" && err != nil {
				firstErr = sym + ": " + err.Error()
			}
			continue
		}
		history[sym] = dailyReturns(closes)
	}
	if len(history) == 0 {
		s.Reason = "chart history unavailable: " + firstErr
		return s
	}

	// 시장 국면 분류 (매크로 섹션 현재 데이터 활용)
	s.Phase = classifyPhase(macro)
	// 상관계수
	if ks, ok := history["^KS11"]; ok {
		if nq, ok2 := history["NQ=F"]; ok2 {
			s.KOSPINASDAQCorr = pearsonCorr(ks, nq)
		}
		if n225, ok2 := history["^N225"]; ok2 {
			s.KOSPINIKKEICorr = pearsonCorr(ks, n225)
		}
	}
	s.RiskAversionIdx = calcRiskAversionIdx(volatility, impact, s.KOSPINASDAQCorr, macro)
	if firstErr != "" {
		s.Reason = "partial: " + firstErr
	}
	return s
}

func dailyReturns(closes []yahoo.DailyClose) []float64 {
	if len(closes) < 2 {
		return nil
	}
	returns := make([]float64, len(closes)-1)
	for i := 1; i < len(closes); i++ {
		if closes[i-1].Close == 0 {
			continue
		}
		returns[i-1] = (closes[i].Close - closes[i-1].Close) / closes[i-1].Close * 100
	}
	return returns
}

func pearsonCorr(x, y []float64) float64 {
	n := min2(len(x), len(y))
	if n < 3 {
		return 0
	}
	xm, ym := mean(x[:n]), mean(y[:n])
	var num, dX, dY float64
	for i := 0; i < n; i++ {
		dx, dy := x[i]-xm, y[i]-ym
		num += dx * dy
		dX += dx * dx
		dY += dy * dy
	}
	denom := math.Sqrt(dX * dY)
	if denom == 0 {
		return 0
	}
	return num / denom
}

func mean(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	s := 0.0
	for _, x := range v {
		s += x
	}
	return s / float64(len(v))
}

func min2(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// classifyPhase: 유가↑/위험자산↓/달러↑ 조합으로 시장 국면 분류
func classifyPhase(macro *MacroSection) string {
	if macro == nil {
		return "데이터 없음"
	}
	oilUp := macro.Quotes["CL=F"].ChangePercent > 1.0
	krwUp := macro.Quotes["KRW=X"].ChangePercent > 0.3 // USD/KRW 상승 = 달러 강세
	// 위험자산 하락 여부는 Global에서 가져와야 하나 여기서는 macro만 사용
	switch {
	case oilUp && krwUp:
		return "지정학 risk-off"
	case !oilUp && krwUp:
		return "성장 둔화 risk-off"
	case oilUp && !krwUp:
		return "리플레이션"
	case krwUp:
		return "달러 강세"
	default:
		return "특이 신호 없음"
	}
}

// calcRiskAversionIdx: 0~10 위험회피 지수 (가중치 임시)
// TODO: 백테스트 후 가중치 조정 필요
func calcRiskAversionIdx(vol *VolatilitySection, imp *ImpactSection, corrKOSPINASDAQ float64, macro *MacroSection) float64 {
	score := 0.0
	// VKOSPI 백분위 대신 절대값 정규화 (0~50 → 0~10)
	if vol != nil && vol.VKOSPI > 0 {
		score += math.Min(vol.VKOSPI/50.0, 1.0) * 10 * 0.3
	}
	// 외인 거래대금 비중 (0~30% → 0~10)
	if imp != nil && imp.ForeignSellTradingValuePercent != nil {
		score += math.Min(*imp.ForeignSellTradingValuePercent/30.0, 1.0) * 10 * 0.3
	}
	// 디커플링 (0~1 → 0~10)
	score += (1 - math.Max(-1, math.Min(1, corrKOSPINASDAQ))) / 2 * 10 * 0.2
	// 환율 변동 (0~2% → 0~10)
	if macro != nil {
		krwChg := math.Abs(macro.Quotes["KRW=X"].ChangePercent)
		score += math.Min(krwChg/2.0, 1.0) * 10 * 0.2
	}
	return math.Round(score*10) / 10
}
