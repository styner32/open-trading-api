package snapshot

import (
	"fmt"
	"strings"
)

func Render(s *Snapshot) string {
	if s == nil {
		_, ts := normalizeDate("")
		s = &Snapshot{Timestamp: ts, Errors: map[string]error{}}
	}
	var b strings.Builder
	b.WriteString("# Market Snapshot — " + s.Timestamp.Format(kstLayout) + "\n\n")
	renderPrice(&b, s)
	renderFlow(&b, s)
	renderImpact(&b, s)
	renderGlobal(&b, s)
	renderCumulative(&b, s)
	renderMacro(&b, s)
	return b.String()
}

func renderPrice(b *strings.Builder, s *Snapshot) {
	b.WriteString("## 1. 일중 가격 흐름\n")
	if s.Price == nil {
		b.WriteString("- " + na(sectionErr(s, "price")) + "\n\n")
		return
	}
	p := s.Price
	highNote := ""
	if p.YearHigh {
		highNote = " (연중 최고)"
	}
	b.WriteString(fmt.Sprintf("- 시가: %s (%s)\n", number(p.Open, 2), percent((p.Open/p.PreviousClose-1)*100)))
	b.WriteString(fmt.Sprintf("- 고가: %s%s\n", number(p.High, 2), highNote))
	b.WriteString(fmt.Sprintf("- 저가: %s\n", number(p.Low, 2)))
	b.WriteString(fmt.Sprintf("- 종가: %s\n", number(p.Close, 2)))
	b.WriteString(fmt.Sprintf("- 일중 변동폭: %sp (%s)\n\n", number(p.RangePoints, 2), percentPlain(p.RangePercent)))
}

func renderFlow(b *strings.Builder, s *Snapshot) {
	b.WriteString("## 2. 수급 (코스피)\n| 주체 | 순매수 (억원) |\n|---|---|\n")
	if s.Flow == nil {
		reason := na(sectionErr(s, "flow"))
		b.WriteString("| 외국인 | " + reason + " |\n| 기관 | " + reason + " |\n| 개인 | " + reason + " |\n\n")
		return
	}
	b.WriteString(fmt.Sprintf("| 외국인 | %s |\n", eok(s.Flow.ForeignEok)))
	b.WriteString(fmt.Sprintf("| 기관 | %s |\n", eok(s.Flow.InstitutionEok)))
	b.WriteString(fmt.Sprintf("| 개인 | %s |\n\n", eok(s.Flow.IndividualEok)))
}

func renderImpact(b *strings.Builder, s *Snapshot) {
	b.WriteString("## 3. 시장 충격 지표\n")
	i := s.Impact
	if i == nil {
		b.WriteString("- " + na("impact section unavailable") + "\n\n")
		return
	}
	b.WriteString("- 시총 대비 외인 매도 비중: " + valuePercent(i.ForeignSellMarketCapPercent, i.ForeignSellReason) + "\n")
	semiconductor := valuePercent(i.SemiconductorSellConcentrationPct, i.SemiconductorReason)
	if i.SemiconductorSellConcentrationPct != nil {
		semiconductor += " (외인 매도의)"
	}
	b.WriteString("- 반도체 매도 집중도: " + semiconductor + "\n")
	b.WriteString("- 코스피200 선물 변동률: " + quotePercent(i.FuturesChangePercent, i.FuturesReason) + "\n")
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
