package pulse

import (
	"fmt"
	"math"
	"strings"
	"time"
)

const kstLayout = "2006-01-02 15:04 KST"

// Render는 Pulse를 한국어 리포트 문자열로 렌더링합니다.
func Render(p *Pulse) string {
	var b strings.Builder
	nowKST := p.Now.In(kstLocation)

	// ── 헤더 ─────────────────────────────────────────────────────────────────
	storeInfo := ""
	storeDir := p.StoreDir
	if storeDir == "" {
		storeDir = ".cache/pulse"
	}
	if p.StoredCount > 0 && p.PrevTS != nil {
		prevKST := p.PrevTS.In(kstLocation)
		storeInfo = fmt.Sprintf("당일 적립 %d회 · 직전 %s", p.StoredCount, prevKST.Format("15:04"))
	} else if p.StoredCount > 0 {
		storeInfo = fmt.Sprintf("당일 적립 %d회", p.StoredCount)
	} else {
		storeInfo = "첫 실행 — 기준선 적립 중"
	}
	b.WriteString(fmt.Sprintf("🫀 시장 펄스  %s   (%s)\n\n",
		nowKST.Format(kstLayout), storeInfo))

	// ── 1. 지수 ───────────────────────────────────────────────────────────────
	b.WriteString("📈 지수 (전일대비 · 최근1h/2h)\n")
	renderMarketIndex(&b, p.KOSPI)
	renderMarketIndex(&b, p.KOSDAQ)
	b.WriteString("\n")

	// ── 2. 시장 안전장치 ───────────────────────────────────────────────────
	b.WriteString("🚨 시장 안전장치 근접도 (실제 발동 여부 미확인)\n")
	renderMarketSafety(&b, p.Safety)
	b.WriteString("\n")

	// ── 3. 수급 ───────────────────────────────────────────────────────────────
	flowWindow := "기준 구간"
	if p.KOSPI.FlowDelta1h != nil {
		flowWindow = "최근 " + elapsedLabel(p.KOSPI.FlowDelta1h.Elapsed)
	}
	b.WriteString(fmt.Sprintf("💰 수급 — %s 델타 (괄호=당일 누적)\n", flowWindow))
	renderMarketFlow(&b, p.KOSPI)
	renderMarketFlow(&b, p.KOSDAQ)
	// 기관 세부 (KOSPI 누적)
	if p.KOSPI.Flow.OK {
		renderFlowDetail(&b, p.KOSPI.Flow)
	}
	b.WriteString("\n")

	// ── 4. 프로그램매매 ─────────────────────────────────────────────────────
	b.WriteString("🧮 프로그램매매 (차익 / 비차익 / 합계)\n")
	renderProgramTrade(&b, "KOSPI", p.KOSPIProgram, p.KOSPIProgramDelta)
	renderProgramTrade(&b, "KOSDAQ", p.KOSDAQProgram, p.KOSDAQProgramDelta)
	b.WriteString("\n")

	// ── 5. KOSPI 기여도 ─────────────────────────────────────────────────────
	b.WriteString("🧱 KOSPI 시총 상위 10종목 포인트 기여도 (추정)\n")
	if len(p.Contributions) == 0 {
		b.WriteString("  데이터 없음\n")
	} else {
		for _, c := range p.Contributions {
			b.WriteString(fmt.Sprintf("  %-10s %-6s  등락 %+.2f%% · 비중 %.2f%% · %+.2fp\n",
				c.Name, c.Code, c.ChangePct, c.WeightPct, c.PointImpact))
		}
	}
	b.WriteString("\n")

	// ── 6. 국내 파생·변동성 ─────────────────────────────────────────────────
	b.WriteString("🌡️ 국내 파생·변동성\n")
	renderDomesticDerivatives(&b, p)
	b.WriteString("\n")

	// ── 7. 환율 ───────────────────────────────────────────────────────────────
	b.WriteString("💱 환율\n")
	renderWindowLine(&b, "  원/달러", p.USDKRW)
	b.WriteString("\n")

	// ── 8. 미국선물·매크로 ────────────────────────────────────────────────────
	b.WriteString("🌐 미국선물·매크로 (최근1h/2h)\n")
	for _, w := range p.Macro {
		if w.Symbol == "^TNX" {
			renderYieldLine(&b, "  "+w.Label, w)
		} else {
			renderWindowLine(&b, "  "+w.Label, w)
		}
	}
	b.WriteString("\n")

	// ── 9. 시장반영 분석 ──────────────────────────────────────────────────────
	b.WriteString("🧭 시장반영\n")
	if len(p.Analysis) == 0 {
		b.WriteString("  데이터 수집 중\n")
	}
	for _, bullet := range p.Analysis {
		b.WriteString("  • " + bullet + "\n")
	}
	b.WriteString("\n")

	// ── 10. 저장 정보 ─────────────────────────────────────────────────────────
	if len(p.Errors) > 0 {
		b.WriteString("⚠️  오류\n")
		for k, v := range p.Errors {
			b.WriteString(fmt.Sprintf("  [%s] %s\n", k, v))
		}
		b.WriteString("\n")
	}

	if p.Saved {
		b.WriteString(fmt.Sprintf("💾 %s/pulse_%s.jsonl (+1행) · pulse_%s.md 갱신\n", storeDir, p.Date, p.Date))
	} else {
		b.WriteString(fmt.Sprintf("💾 저장 안 함 · 대상 경로 %s/pulse_%s.jsonl\n", storeDir, p.Date))
	}

	return b.String()
}

func renderMarketIndex(b *strings.Builder, m Market) {
	idx := m.Index
	if !idx.OK {
		b.WriteString(fmt.Sprintf("  %-7s  데이터 없음\n", m.Name))
		return
	}

	win1h := fmtPctPtr(m.IntradayWin.Move1hPct)
	win2h := fmtPctPtr(m.IntradayWin.Move2hPct)

	tradingStr := ""
	if idx.TradingValue > 0 {
		tradingStr = fmt.Sprintf("   거래대금 %s", fmtAmountEok(idx.TradingValue))
	}

	b.WriteString(fmt.Sprintf("  %-7s %9.2f  %s%s%%   1h %s  2h %s%s\n",
		m.Name, idx.Price,
		arrowNeutral(idx.ChangePct), fmt.Sprintf("%.2f", idx.ChangePct),
		win1h, win2h,
		tradingStr,
	))

	// OHLC + 종목 수
	if idx.Open > 0 {
		b.WriteString(fmt.Sprintf("          시 %.2f / 고 %.2f / 저 %.2f",
			idx.Open, idx.High, idx.Low))
		if idx.Advancers+idx.Decliners > 0 {
			b.WriteString(fmt.Sprintf("   상승 %d · 하락 %d", idx.Advancers, idx.Decliners))
		}
		b.WriteString("\n")
	}
}

func renderMarketFlow(b *strings.Builder, m Market) {
	flow := m.Flow
	d1h := m.FlowDelta1h

	if !flow.OK {
		b.WriteString(fmt.Sprintf("  %-7s  수급 데이터 없음\n", m.Name))
		return
	}

	if d1h == nil {
		// 첫 실행: 누적만 표시
		b.WriteString(fmt.Sprintf("  %-7s  기준선 적립 — 잠시 후 재실행 시 구간 델타 표시\n", m.Name))
		b.WriteString(fmt.Sprintf("           누적: 외국인 %s · 기관 %s · 개인 %s\n",
			fmtEok(flow.Foreign), fmtEok(flow.Institution), fmtEok(flow.Individual)))
		return
	}

	acc := FlowAcceleration(m.FlowDelta1h, m.FlowDelta2h, func(d *FlowDelta) float64 { return d.Foreign })
	accStr := ""
	if acc != "" {
		accStr = " " + acc
	}

	b.WriteString(fmt.Sprintf("  %-7s  외국인 %s%s (누적 %s) · 기관 %s (누적 %s) · 개인 %s (누적 %s)\n",
		m.Name,
		fmtEok(d1h.Foreign), accStr,
		fmtEok(flow.Foreign),
		fmtEok(d1h.Institution),
		fmtEok(flow.Institution),
		fmtEok(d1h.Individual),
		fmtEok(flow.Individual),
	))
}

func renderProgramTrade(b *strings.Builder, market string, cur ProgramTradeSnapshot, delta *ProgramTradeDelta) {
	if !cur.OK {
		b.WriteString(fmt.Sprintf("  %-7s 데이터 없음\n", market))
		return
	}
	b.WriteString(fmt.Sprintf("  %-7s 누적 %s / %s / %s", market,
		fmtEok(cur.Arbitrage), fmtEok(cur.NonArbitrage), fmtEok(cur.Total)))
	if delta != nil {
		b.WriteString(fmt.Sprintf("   · 최근 %s %s / %s / %s", elapsedLabel(delta.Elapsed),
			fmtEok(delta.Arbitrage), fmtEok(delta.NonArbitrage), fmtEok(delta.Total)))
	}
	if cur.AsOf == "close" {
		b.WriteString("  @장마감")
	} else if len(cur.AsOf) >= 4 {
		b.WriteString(fmt.Sprintf("  @%s:%s", cur.AsOf[:2], cur.AsOf[2:4]))
	}
	b.WriteString("\n")
}

func renderMarketSafety(b *strings.Builder, safety MarketSafety) {
	if len(safety.CircuitBreakers) == 0 && len(safety.Sidecars) == 0 {
		b.WriteString("  데이터 없음\n")
		return
	}
	for _, cb := range safety.CircuitBreakers {
		if !cb.OK || len(cb.Levels) == 0 {
			continue
		}
		phase1 := cb.Levels[0]
		state := fmt.Sprintf("현재까지 %.2f%%p · 장중저점 기준 %.2f%%p", phase1.CurrentGapPct, phase1.LowGapPct)
		if phase1.CurrentReached || phase1.LowReached {
			state = "임계 도달 · 실제 발동 미확인"
		}
		b.WriteString(fmt.Sprintf("  %s CB 1단계(-8%%): 현재 %+.2f%% / 저점 %+.2f%% → %s\n",
			cb.Market, cb.CurrentChangePct, cb.LowChangePct, state))
	}
	for _, sc := range safety.Sidecars {
		if !sc.OK {
			continue
		}
		state := fmt.Sprintf("선물 임계까지 %.2f%%p", sc.FuturesGapPct)
		if sc.SpotThresholdPct > 0 {
			state += fmt.Sprintf(" · 현물 조건까지 %.2f%%p", sc.SpotGapPct)
		}
		if sc.ThresholdReached {
			state = "임계 도달 · 실제 발동 미확인"
		}
		b.WriteString(fmt.Sprintf("  %s 사이드카: 선물 %+.2f%%", sc.Market, sc.FuturesChangePct))
		if sc.SpotThresholdPct > 0 {
			b.WriteString(fmt.Sprintf(" / 기초지수 %+.2f%%", sc.SpotChangePct))
		}
		b.WriteString(" → " + state + "\n")
	}
}

func renderDomesticDerivatives(b *strings.Builder, p *Pulse) {
	if p.KOSPI200Future.OK {
		regime := "콘탱고"
		if p.KOSPI200Future.Basis < 0 {
			regime = "백워데이션"
		}
		b.WriteString(fmt.Sprintf("  KOSPI200 선물 %s  %.2f (%+.2f%%) · 현물 %.2f · 베이시스 %+.2fp (%s)",
			p.KOSPI200Future.Code, p.KOSPI200Future.Price, p.KOSPI200Future.ChangePct,
			p.KOSPI200Future.SpotPrice, p.KOSPI200Future.Basis, regime))
		if p.BasisDelta1h != nil {
			b.WriteString(fmt.Sprintf(" · 최근 %s %+.2fp", elapsedLabel(p.BasisDelta1h.Elapsed), p.BasisDelta1h.Value))
		}
		b.WriteString("\n")
	} else {
		b.WriteString("  KOSPI200 베이시스 데이터 없음\n")
	}
	if p.VKOSPI.OK {
		b.WriteString(fmt.Sprintf("  VKOSPI %.2f  전일 %+.2f%% · %s\n", p.VKOSPI.Value, p.VKOSPI.ChangePct, p.VKOSPI.Source))
	} else {
		b.WriteString("  VKOSPI 데이터 없음\n")
	}
}

func renderFlowDetail(b *strings.Builder, flow FlowSnapshot) {
	b.WriteString(fmt.Sprintf("  └ 기관 세부(누적): 금융투자 %s · 투신 %s · 연기금 %s · 사모 %s\n",
		fmtEok(flow.FinInvest),
		fmtEok(flow.InvTrust),
		fmtEok(flow.Pension),
		fmtEok(flow.PrivEquity),
	))
}

func renderWindowLine(b *strings.Builder, label string, w Window) {
	if !w.OK {
		b.WriteString(fmt.Sprintf("%-20s  데이터 없음\n", label))
		return
	}

	move1h := "N/A"
	if w.Move1hPct != nil {
		move1h = fmtPct(*w.Move1hPct)
	}
	move2h := "N/A"
	if w.Move2hPct != nil {
		move2h = fmtPct(*w.Move2hPct)
	}

	lastStr := ""
	if !w.LastTS.IsZero() {
		lastStr = fmt.Sprintf("  @%s", w.LastTS.In(kstLocation).Format("15:04"))
	}

	b.WriteString(fmt.Sprintf("%-20s  %10.4f  전일 %s%s%%   1h %s  2h %s%s\n",
		label, w.Current,
		arrowNeutral(w.ChangePct), fmt.Sprintf("%.2f", w.ChangePct),
		move1h, move2h,
		lastStr,
	))
}

func renderYieldLine(b *strings.Builder, label string, w Window) {
	if !w.OK {
		b.WriteString(fmt.Sprintf("%-20s  데이터 없음\n", label))
		return
	}
	toBP := func(changePct *float64) string {
		if changePct == nil {
			return "N/A"
		}
		denom := 1 + *changePct/100
		if math.Abs(denom) < 1e-9 {
			return "N/A"
		}
		ref := w.Current / denom
		return fmt.Sprintf("%+.1fbp", (w.Current-ref)*100)
	}
	change := w.ChangePct
	lastStr := ""
	if !w.LastTS.IsZero() {
		lastStr = fmt.Sprintf("  @%s", w.LastTS.In(kstLocation).Format("15:04"))
	}
	b.WriteString(fmt.Sprintf("%-20s  %7.3f%%  전일 %s   1h %s  2h %s%s\n",
		label, w.Current, toBP(&change), toBP(w.Move1hPct), toBP(w.Move2hPct), lastStr))
}

// RenderJSON은 Pulse를 간단한 JSON 요약으로 변환합니다 (--json 플래그용).
// 실제 JSON 직렬화는 encoding/json을 통해 호출자가 처리합니다.
func PulseToMap(p *Pulse) map[string]any {
	nowKST := p.Now.In(kstLocation)
	return map[string]any{
		"ts":            nowKST.Format(time.RFC3339),
		"date":          p.Date,
		"business_date": p.BusinessDate,
		"stored_count":  p.StoredCount,
		"kospi": map[string]any{
			"price":      p.KOSPI.Index.Price,
			"change_pct": p.KOSPI.Index.ChangePct,
			"move_1h":    p.KOSPI.IntradayWin.Move1hPct,
			"move_2h":    p.KOSPI.IntradayWin.Move2hPct,
			"flow": map[string]any{
				"foreign":     p.KOSPI.Flow.Foreign,
				"institution": p.KOSPI.Flow.Institution,
				"individual":  p.KOSPI.Flow.Individual,
			},
		},
		"kosdaq": map[string]any{
			"price":      p.KOSDAQ.Index.Price,
			"change_pct": p.KOSDAQ.Index.ChangePct,
			"move_1h":    p.KOSDAQ.IntradayWin.Move1hPct,
		},
		"usdkrw": map[string]any{
			"price":   p.USDKRW.Current,
			"move_1h": p.USDKRW.Move1hPct,
		},
		"market_safety": p.Safety,
		"program_trade": map[string]any{
			"kospi": p.KOSPIProgram, "kosdaq": p.KOSDAQProgram,
			"kospi_delta": p.KOSPIProgramDelta, "kosdaq_delta": p.KOSDAQProgramDelta,
		},
		"kospi200_future":     p.KOSPI200Future,
		"basis_delta_1h":      p.BasisDelta1h,
		"basis_delta_2h":      p.BasisDelta2h,
		"vkospi":              p.VKOSPI,
		"kospi_contributions": p.Contributions,
		"analysis":            p.Analysis,
		"errors":              p.Errors,
	}
}
