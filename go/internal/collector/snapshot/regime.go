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
func collectRegime(ctx context.Context, yClient YahooQuotes, price *PriceSection, volatility *VolatilitySection, impact *ImpactSection, macro *MacroSection) *RegimeSection {
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

	// 시장 국면 분류
	s.Phase = classifyPhase(price, impact, macro)
	// 상관계수
	if ks, ok := history["^KS11"]; ok {
		if nq, ok2 := history["NQ=F"]; ok2 {
			s.KOSPINASDAQCorr = pearsonCorr(ks, nq)
		}
		if n225, ok2 := history["^N225"]; ok2 {
			s.KOSPINIKKEICorr = pearsonCorr(ks, n225)
		}
	}
	s.RiskAversionIdx = calcRiskAversionIdx(price, volatility, impact, s.KOSPINASDAQCorr, macro)
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

// classifyPhase: 지수 급락 및 사이드카 발동 감지 포함한 시장 국면 분류
func classifyPhase(price *PriceSection, impact *ImpactSection, macro *MacroSection) string {
	var isPanic bool
	var panicLabel string

	if price != nil && price.PreviousClose > 0 {
		chg := (price.Close - price.PreviousClose) / price.PreviousClose * 100
		if chg <= -8.0 {
			isPanic = true
			panicLabel = "시장 패닉 (CB 발동 급락)"
		} else if chg <= -5.0 {
			isPanic = true
			panicLabel = "시장 패닉 (급락 국면)"
		} else if chg <= -3.0 {
			isPanic = true
			panicLabel = "급락 국면"
		}
	}

	if impact != nil && impact.SidecarStatus == "triggered" {
		isPanic = true
		if panicLabel == "" {
			panicLabel = "시장 패닉 (사이드카 발동)"
		} else {
			panicLabel += " & 사이드카 발동"
		}
	}

	if isPanic {
		return panicLabel
	}

	if macro == nil {
		return "데이터 없음"
	}
	oilUp := macro.Quotes["CL=F"].ChangePercent > 1.0
	krwUp := macro.Quotes["KRW=X"].ChangePercent > 0.3 // USD/KRW 상승 = 달러 강세
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

// calcRiskAversionIdx: 0~10 위험회피 지수 계산
func calcRiskAversionIdx(price *PriceSection, vol *VolatilitySection, imp *ImpactSection, corrKOSPINASDAQ float64, macro *MacroSection) float64 {
	score := 0.0

	// 1. KOSPI 일간 수익률 / 패닉 컴포넌트 (Weight 0.25)
	kospiChg := 0.0
	if price != nil && price.PreviousClose > 0 {
		kospiChg = (price.Close - price.PreviousClose) / price.PreviousClose * 100
		if kospiChg < 0 {
			score += math.Min(math.Abs(kospiChg)/4.0, 1.0) * 10 * 0.25
		}
	}

	// 2. VKOSPI 변동성 컴포넌트 (Weight 0.25) - 없으면 일간 변동폭 프록시 사용
	vkospiVal := 0.0
	if vol != nil && vol.VKOSPI > 0 {
		vkospiVal = vol.VKOSPI
	} else if price != nil && price.Low > 0 {
		// VKOSPI가 없을 경우 당일 고저 변동률 * 5배로 근사 (변동성 프록시)
		intradayRangePct := (price.High - price.Low) / price.Low * 100
		vkospiVal = math.Min(intradayRangePct*5.0, 50.0)
	}
	if vkospiVal > 0 {
		score += math.Min(vkospiVal/40.0, 1.0) * 10 * 0.25
	}

	// 3. 외인 거래대금 비중 (Weight 0.20)
	if imp != nil && imp.ForeignSellTradingValuePercent != nil {
		score += math.Min(*imp.ForeignSellTradingValuePercent/25.0, 1.0) * 10 * 0.20
	}

	// 4. 환율 변동 (Weight 0.15)
	krwChg := 0.0
	if macro != nil {
		krwChg = math.Abs(macro.Quotes["KRW=X"].ChangePercent)
		score += math.Min(krwChg/1.5, 1.0) * 10 * 0.15
	}

	// 5. 디커플링 강도 (Weight 0.15)
	score += (1 - math.Max(-1, math.Min(1, corrKOSPINASDAQ))) / 2 * 10 * 0.15

	// 지수 최종 보정
	result := math.Round(score*10) / 10

	// 급락/패닉 상황에서의 최소 지수 Floor 설정
	if kospiChg <= -8.0 {
		result = math.Max(result, 9.0)
	} else if kospiChg <= -7.0 {
		result = math.Max(result, 8.0)
	} else if kospiChg <= -5.0 {
		result = math.Max(result, 7.0)
	} else if kospiChg <= -3.0 {
		result = math.Max(result, 5.0)
	}

	if imp != nil && imp.SidecarStatus == "triggered" {
		result = math.Max(result, 6.0)
	}

	return result
}
