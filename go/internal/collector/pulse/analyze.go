package pulse

import (
	"fmt"
	"math"
)

// Analyze는 Pulse에서 결정적(rule-based) 한국어 분석 불릿 목록을 생성합니다.
// §6.3 규칙 참조.
func Analyze(p *Pulse) []string {
	var bullets []string

	// 1. 지수 모멘텀
	bullets = append(bullets, analyzeIndexMomentum(p)...)

	// 2. 수급 주도 주체
	bullets = append(bullets, analyzeFlowLeader(p)...)

	// 3. 외국인 × 환율 연계
	bullets = append(bullets, analyzeForexLink(p)...)

	// 4. 미국선물 동조/디커플링
	bullets = append(bullets, analyzeFuturesSync(p)...)

	// 5. 금리/유가 보조
	bullets = append(bullets, analyzeMacroSignals(p)...)

	// 6. 종합 리스크 라벨
	bullets = append(bullets, analyzeRiskLabel(p))

	return bullets
}

func analyzeIndexMomentum(p *Pulse) []string {
	var out []string
	m1h := p.KOSPI.IntradayWin.Move1hPct
	if m1h == nil {
		return out
	}
	v := *m1h
	switch {
	case v <= -0.5:
		out = append(out, fmt.Sprintf("코스피 최근 1h %s 하방 모멘텀 우위", fmtPct(v)))
	case v >= 0.5:
		out = append(out, fmt.Sprintf("코스피 최근 1h %s 반등 시도", fmtPct(v)))
	default:
		out = append(out, fmt.Sprintf("코스피 최근 1h %s 횡보", fmtPct(v)))
	}
	return out
}

func analyzeFlowLeader(p *Pulse) []string {
	var out []string
	d1h := p.KOSPI.FlowDelta1h
	d2h := p.KOSPI.FlowDelta2h
	if d1h == nil {
		return out
	}

	check := func(name string, v1h float64, field func(*FlowDelta) float64) {
		if math.Abs(hourlyRate(v1h, d1h.Elapsed)) < 1000 {
			return
		}
		acc := FlowAcceleration(d1h, d2h, field)
		accStr := ""
		if acc != "" {
			accStr = " (" + acc + ")"
		}
		cumStr := ""
		switch name {
		case "외국인":
			if p.KOSPI.Flow.OK {
				cumStr = fmt.Sprintf(" (누적 %s)", fmtEok(p.KOSPI.Flow.Foreign))
			}
		case "기관":
			if p.KOSPI.Flow.OK {
				cumStr = fmt.Sprintf(" (누적 %s)", fmtEok(p.KOSPI.Flow.Institution))
			}
		case "개인":
			if p.KOSPI.Flow.OK {
				cumStr = fmt.Sprintf(" (누적 %s)", fmtEok(p.KOSPI.Flow.Individual))
			}
		}
		out = append(out, fmt.Sprintf("KOSPI %s 최근 %s %s %s%s%s", name, elapsedLabel(d1h.Elapsed), fmtEok(v1h), flowDirection(v1h), accStr, cumStr))
	}

	check("외국인", d1h.Foreign, func(d *FlowDelta) float64 { return d.Foreign })
	check("기관", d1h.Institution, func(d *FlowDelta) float64 { return d.Institution })
	check("개인", d1h.Individual, func(d *FlowDelta) float64 { return d.Individual })

	return out
}

func analyzeForexLink(p *Pulse) []string {
	var out []string
	d1h := p.KOSPI.FlowDelta1h
	if d1h == nil || !p.KOSPI.Flow.OK {
		return out
	}

	foreignSelling := hourlyRate(d1h.Foreign, d1h.Elapsed) < -500
	usdkrwRise := p.USDKRW.Move1hPct != nil && *p.USDKRW.Move1hPct > 0

	if foreignSelling && usdkrwRise {
		out = append(out, fmt.Sprintf("원화 약세(원/달러 1h %s) 동반 외국인 이탈 → 환차손 회피성 매도 가능성", fmtPct(*p.USDKRW.Move1hPct)))
	} else if !usdkrwRise && p.USDKRW.Move1hPct != nil {
		out = append(out, fmt.Sprintf("원/달러 1h %s (원화 강보합) → 환율發 압력 제한적", fmtPct(*p.USDKRW.Move1hPct)))
	}
	return out
}

func analyzeFuturesSync(p *Pulse) []string {
	var out []string
	kospi1h := p.KOSPI.IntradayWin.Move1hPct
	var nqWin *Window
	for i := range p.Macro {
		if p.Macro[i].Symbol == "NQ=F" {
			nqWin = &p.Macro[i]
			break
		}
	}
	if kospi1h == nil || nqWin == nil || nqWin.Move1hPct == nil {
		return out
	}

	nq1h := *nqWin.Move1hPct
	ksp1h := *kospi1h

	if sign(nq1h) == sign(ksp1h) {
		out = append(out, fmt.Sprintf("나스닥선물 1h %s — 코스피와 동조", fmtPct(nq1h)))
	} else {
		out = append(out, fmt.Sprintf(
			"디커플링: 나스닥선물 1h %s vs 코스피 1h %s — 갭 메우기 여지",
			fmtPct(nq1h), fmtPct(ksp1h),
		))
	}
	return out
}

func analyzeMacroSignals(p *Pulse) []string {
	var out []string
	for i := range p.Macro {
		w := &p.Macro[i]
		if w.Move1hPct == nil {
			continue
		}
		v := *w.Move1hPct
		switch w.Symbol {
		case "^TNX":
			if v >= 1.5 {
				out = append(out, fmt.Sprintf("미국채10Y 1h %s 급등 → 금리 상승이 위험자산 부담", fmtPct(v)))
			} else if v <= -1.5 {
				out = append(out, fmt.Sprintf("미국채10Y 1h %s 하락 → 금리 하락, 위험자산 긍정적", fmtPct(v)))
			}
		case "CL=F":
			if math.Abs(v) >= 1.5 {
				dir := "급등"
				if v < 0 {
					dir = "급락"
				}
				out = append(out, fmt.Sprintf("WTI원유 1h %s %s → 에너지·인플레이션 주의", fmtPct(v), dir))
			}
		}
	}
	return out
}

// analyzeRiskLabel은 신호들을 합산해 리스크 라벨을 반환합니다.
func analyzeRiskLabel(p *Pulse) string {
	score := 0 // 양수 = Risk-on, 음수 = Risk-off

	if p.KOSPI.IntradayWin.Move1hPct != nil {
		if *p.KOSPI.IntradayWin.Move1hPct >= 0.5 {
			score++
		} else if *p.KOSPI.IntradayWin.Move1hPct <= -0.5 {
			score--
		}
	}

	if p.KOSPI.FlowDelta1h != nil {
		foreignHourly := hourlyRate(p.KOSPI.FlowDelta1h.Foreign, p.KOSPI.FlowDelta1h.Elapsed)
		if foreignHourly < -500 {
			score -= 2
		} else if foreignHourly > 500 {
			score++
		}
	}

	if p.USDKRW.Move1hPct != nil {
		if *p.USDKRW.Move1hPct > 0.1 {
			score-- // 원화 약세
		}
	}

	for i := range p.Macro {
		w := &p.Macro[i]
		if w.Symbol == "NQ=F" && w.Move1hPct != nil {
			if *w.Move1hPct >= 0.5 {
				score++
			} else if *w.Move1hPct <= -0.5 {
				score--
			}
		}
		if w.Symbol == "^TNX" && w.Move1hPct != nil {
			if *w.Move1hPct >= 1.5 {
				score--
			}
		}
	}

	label := "중립 (Neutral)"
	if score >= 2 {
		label = "위험선호 (Risk-on)"
	} else if score <= -2 {
		label = "위험회피 (Risk-off)"
	}
	return fmt.Sprintf("종합: %s (신호점수 %+d)", label, score)
}

func sign(v float64) int {
	if v > 0 {
		return 1
	}
	if v < 0 {
		return -1
	}
	return 0
}
