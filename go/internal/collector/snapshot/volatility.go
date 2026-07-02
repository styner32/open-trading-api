package snapshot

import (
	"context"
	"fmt"
	"time"
)

// VolatilitySection은 VKOSPI/VIX 변동성 지표를 담습니다.
type VolatilitySection struct {
	VKOSPI         float64 // 현재 VKOSPI 수치
	VKOSPIChange   float64 // 전일 대비 변동률 (%)
	VKOSPI5DayAvg  float64 // 5거래일 단순 평균
	VIX            float64 // CBOE VIX
	VIXChange      float64 // VIX 전일 대비 변동률 (%)
	DecouplingFlag bool    // 지수-VKOSPI 방향 디커플링
	Level          string  // "정상"/"주의"/"위험"
	Reason         string  // 부분 실패 메시지
	Source         string  // KIS 또는 Naver
}

// collectVolatility는 VKOSPI(Naver Finance)와 VIX(Yahoo Finance)를 조회합니다.
//
// VKOSPI 소스: Naver Finance polling API (SERVICE_INDEX:VKOSPI)
//   - 장중: 실시간값
//   - 장 마감 후: 해당 영업일 종가
//   - TODO: unofficial API — 장기적으로 KRX 공식 데이터 또는 EODHD 등 대체 검토.
//
// VIX 소스: Yahoo Finance v8 chart API (^VIX)
func collectVolatility(ctx context.Context, stock DomesticStock, naverClient NaverFinance, yahoo YahooQuotes, indexChange float64) *VolatilitySection {
	s := &VolatilitySection{}

	// VIX는 VKOSPI 성공 여부와 무관하게 항상 조회
	if yahoo != nil {
		if quotes, yErr := yahoo.GetQuotes(ctx, []string{"^VIX"}); yErr == nil {
			if q, ok := quotes["^VIX"]; ok {
				s.VIX = q.Price
				s.VIXChange = q.ChangePercent
			}
		} else {
			s.Reason = appendReason(s.Reason, "VIX: "+yErr.Error())
		}
	}

	// KIS 지수 API를 우선 사용한다. Naver는 KIS가 값을 주지 못할 때만 fallback이다.
	if stock != nil {
		if code, resolveErr := stock.ResolveVKOSPICode(ctx, []string{"0503", "2050"}); resolveErr == nil {
			if resp, priceErr := stock.InquireVKOSPIPrice(ctx, code); priceErr == nil {
				if row := firstRow(resp, "output"); row != nil {
					vk, vkOK := num(row, "bstp_nmix_prpr")
					change, _ := num(row, "bstp_nmix_prdy_ctrt")
					if vkOK && vk >= 5 && vk <= 100 {
						s.VKOSPI = vk
						s.VKOSPIChange = change
						s.Level = vkospiLevel(vk)
						s.DecouplingFlag = isDecoupling(indexChange, change)
						s.Source = "KIS"
						fromDate := time.Now().AddDate(0, 0, -14).Format("20060102")
						if history, histErr := stock.InquireVKOSPIDailyPrice(ctx, code, fromDate); histErr == nil {
							sum, count := 0.0, 0
							for _, historyRow := range history {
								if closeValue, ok := num(historyRow, "bstp_nmix_prpr", "stck_clpr"); ok && closeValue >= 5 && closeValue <= 100 {
									sum += closeValue
									count++
									if count == 5 {
										break
									}
								}
							}
							if count > 0 {
								s.VKOSPI5DayAvg = sum / float64(count)
							}
						} else {
							s.Reason = appendReason(s.Reason, "KIS VKOSPI 5d avg: "+histErr.Error())
						}
						return s
					}
				}
			}
		}
	}

	if naverClient == nil {
		s.Reason = appendReason(s.Reason, "naver client is nil — VKOSPI unavailable")
		return s
	}

	// ── VKOSPI 현재가 ────────────────────────────────────────────────────────
	// 1차: polling API (장중 실시간)
	// 2차: sise_index_day 최근 종가 (장 마감 후 fallback)
	quote, err := naverClient.GetIndexQuote(ctx, "VKOSPI")
	if err != nil {
		// polling 실패 → sise_index_day에서 최근 종가로 fallback
		history, histErr := naverClient.GetIndexDailyHistory(ctx, "VKOSPI", 10)
		if histErr != nil {
			s.Reason = appendReason(s.Reason, fmt.Sprintf("VKOSPI unavailable (polling: %v, history: %v)", err, histErr))
			return s
		}
		if len(history) == 0 {
			s.Reason = appendReason(s.Reason, fmt.Sprintf("VKOSPI: no history data (polling: %v)", err))
			return s
		}
		// 가장 최근 종가 사용
		last := history[len(history)-1]
		vk := last.Close
		if vk < 5 || vk > 100 {
			s.Reason = appendReason(s.Reason, fmt.Sprintf("VKOSPI 종가 %.2f 정상범위 초과", vk))
			return s
		}
		s.VKOSPI = vk
		s.Level = vkospiLevel(vk)
		s.Source = "Naver"
		// 5일 평균
		if len(history) >= 5 {
			sum, count := 0.0, 0
			for i := len(history) - 1; i >= 0 && count < 5; i-- {
				sum += history[i].Close
				count++
			}
			s.VKOSPI5DayAvg = sum / float64(count)
		}
		return s
	}
	vk := quote.Price
	if vk < 5 || vk > 100 {
		s.Reason = appendReason(s.Reason,
			fmt.Sprintf("VKOSPI %.2f 정상범위(5~100) 초과 — 데이터 이상", vk))
		return s
	}
	s.VKOSPI = vk
	s.VKOSPIChange = quote.ChangePercent
	s.Level = vkospiLevel(vk)
	s.DecouplingFlag = isDecoupling(indexChange, quote.ChangePercent)
	s.Source = "Naver"

	// ── VKOSPI 5거래일 평균 ───────────────────────────────────────────────────
	history, histErr := naverClient.GetIndexDailyHistory(ctx, "VKOSPI", 10)
	if histErr == nil && len(history) >= 5 {
		sum, count := 0.0, 0
		for i := len(history) - 1; i >= 0 && count < 5; i-- {
			sum += history[i].Close
			count++
		}
		s.VKOSPI5DayAvg = sum / float64(count)
	} else if histErr != nil {
		s.Reason = appendReason(s.Reason, "VKOSPI 5d avg: "+histErr.Error())
	}

	return s
}

func vkospiLevel(v float64) string {
	switch {
	case v < 20:
		return "정상"
	case v < 25:
		return "평상시"
	case v < 30:
		return "주의"
	default:
		return "위험"
	}
}

// isDecoupling: 지수와 VKOSPI 방향이 다르고 VKOSPI 변동폭 > 5%
func isDecoupling(indexChg, vkospiChg float64) bool {
	if vkospiChg < -5 || vkospiChg > 5 {
		return (indexChg > 0 && vkospiChg < -5) || (indexChg < 0 && vkospiChg > 5)
	}
	return false
}
