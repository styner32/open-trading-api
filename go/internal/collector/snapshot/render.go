package snapshot

import (
	"fmt"
	"strings"
)

// Render는 스냅샷을 마크다운으로 렌더링합니다.
// prev가 제공되면 주요 지표에 전일 대비 변화를 함께 표시합니다.
func Render(s *Snapshot, prev ...*SnapshotJSON) string {
	if s == nil {
		_, ts := normalizeDate("")
		s = &Snapshot{Timestamp: ts, Errors: map[string]error{}}
	}
	var p *SnapshotJSON
	if len(prev) > 0 {
		p = prev[0]
	}
	var b strings.Builder
	header := "# Market Snapshot — " + s.Timestamp.Format(kstLayout)
	if p != nil {
		header += fmt.Sprintf("  (전일: %s)", p.Date)
	}
	b.WriteString(header + "\n\n")
	renderPrice(&b, s, p)
	renderFlow(&b, s, p)
	renderImpact(&b, s)
	renderGlobal(&b, s)
	renderCumulative(&b, s)
	renderMacro(&b, s)
	renderVolatility(&b, s, p)
	renderCredit(&b, s, p)
	renderRegime(&b, s, p)
	renderConcentration(&b, s, p)
	renderLateSession(&b, s, p)
	return b.String()
}

func renderPrice(b *strings.Builder, s *Snapshot, p *SnapshotJSON) {
	b.WriteString("## 1. 일중 가격 흐름\n")
	if s.Price == nil {
		b.WriteString("- " + na(sectionErr(s, "price")) + "\n\n")
		return
	}
	pr := s.Price
	highNote := ""
	if pr.YearHigh {
		highNote = " (연중 최고)"
	}
	b.WriteString(fmt.Sprintf("- 시가: %s (%s)\n", number(pr.Open, 2), percent((pr.Open/pr.PreviousClose-1)*100)))
	b.WriteString(fmt.Sprintf("- 고가: %s%s\n", number(pr.High, 2), highNote))
	b.WriteString(fmt.Sprintf("- 저가: %s\n", number(pr.Low, 2)))

	closeLine := number(pr.Close, 2)
	if p != nil && p.Price != nil && p.Price.Close > 0 {
		diff := pr.Close - p.Price.Close
		closeLine += fmt.Sprintf("  [전일 종가 %s, %s]", number(p.Price.Close, 2), signedNumber(diff, 2))
	}
	b.WriteString("- 종가: " + closeLine + "\n")
	b.WriteString(fmt.Sprintf("- 일중 변동폭: %sp (%s)\n\n", number(pr.RangePoints, 2), percentPlain(pr.RangePercent)))
}

func renderFlow(b *strings.Builder, s *Snapshot, p *SnapshotJSON) {
	b.WriteString("## 2. 수급 (코스피)\n| 주체 | 순매수 (억원) | 전일 |\n|---|---|---|\n")
	if s.Flow == nil {
		reason := na(sectionErr(s, "flow"))
		b.WriteString("| 외국인 | " + reason + " | - |\n| 기관 | " + reason + " | - |\n| 개인 | " + reason + " | - |\n\n")
		return
	}
	fmtFlowRow := func(name string, current float64, prevFunc func() float64) {
		prevStr := "-"
		if p != nil && p.Flow != nil {
			pv := prevFunc()
			prevStr = eok(pv)
		}
		b.WriteString(fmt.Sprintf("| %s | %s | %s |\n", name, eok(current), prevStr))
	}
	fmtFlowRow("외국인", s.Flow.ForeignEok, func() float64 { return p.Flow.ForeignEok })
	fmtFlowRow("기관", s.Flow.InstitutionEok, func() float64 { return p.Flow.InstitutionEok })
	fmtFlowRow("개인", s.Flow.IndividualEok, func() float64 { return p.Flow.IndividualEok })
	b.WriteString("\n")
}

func renderImpact(b *strings.Builder, s *Snapshot) {
	b.WriteString("## 3. 시장 충격 지표\n")
	i := s.Impact
	if i == nil {
		b.WriteString("- " + na("impact section unavailable") + "\n\n")
		return
	}
	tvLine := na(i.ForeignSellTradingValueReason)
	if i.ForeignSellTradingValuePercent != nil {
		tvLine = percentPlain(*i.ForeignSellTradingValuePercent) + " [" + i.ForeignSellTradingValueLabel + "]"
	}
	b.WriteString("- 외인 거래대금 비중: " + tvLine + "\n")
	b.WriteString("  - 임계값: <10% 정상, 10~20% 주의, >20% 위험\n")
	b.WriteString("- 시총 대비 외인 매도: " + valuePercent(i.ForeignSellMarketCapPercent, i.ForeignSellReason) + " (참고)\n")
	semiconductor := valuePercent(i.SemiconductorSellConcentrationPct, i.SemiconductorReason)
	if i.SemiconductorSellConcentrationPct != nil {
		semiconductor += " (외인 매도의)"
	}
	b.WriteString("- 반도체 매도 집중도: " + semiconductor + "\n")
	b.WriteString("- 코스피200 선물 변동률: " + quotePercent(i.FuturesChangePercent, i.FuturesReason) + "\n")
	if i.BasisPoints != nil && i.BasisPercent != nil {
		b.WriteString(fmt.Sprintf("- 선물-현물 베이시스: %sp (%s)\n", number(*i.BasisPoints, 1), percent(*i.BasisPercent)))
	} else {
		b.WriteString("- 선물-현물 베이시스: " + na(i.BasisReason) + "\n")
	}
	sidecar := na("manual input not provided")
	if i.SidecarStatus == "triggered" {
		sidecar = appendReason("✓", i.SidecarTime)
	} else if i.SidecarStatus == "not-triggered" {
		sidecar = "-"
	}
	b.WriteString("- 매도 사이드카 발동: " + sidecar + "\n\n")
}


func renderGlobal(b *strings.Builder, s *Snapshot) {
	b.WriteString("## 4. 글로벌 동조성 (전일 대비)\n| 자산 | 변동률 |\n|---|---|\n")
	for _, item := range []struct{ label, symbol string }{{"닛케이225", "^N225"}, {"나스닥 선물", "NQ=F"}, {"WTI 유가", "CL=F"}, {"BTC", "BTC-USD"}, {"USD/KRW", "KRW=X"}} {
		value := na(sectionErr(s, "global"))
		if s.Global != nil {
			if q, ok := s.Global.Quotes[item.symbol]; ok {
				value = percent(q.ChangePercent)
				if item.symbol == "KRW=X" {
					value += " (" + number(q.Price, 2) + ")"
				}
			} else {
				value = na(s.Global.Reason)
			}
		}
		b.WriteString(fmt.Sprintf("| %s | %s |\n", item.label, value))
	}
	b.WriteString("\n")
}
