package pulse

import (
	"context"
	"fmt"
	"os"
	"time"

	"golang.org/x/sync/errgroup"
)

// Collect는 전체 펄스를 수집하고 적립합니다.
func Collect(ctx context.Context, deps Deps, opts Options) *Pulse {
	now := deps.Clock()
	nowKST := now.In(kstLocation)
	date := nowKST.Format("20060102")
	storeDir := resolveStoreDir(opts)

	pulse := &Pulse{
		Now:          now,
		Date:         date,
		BusinessDate: date,
		Errors:       map[string]string{},
		StoreDir:     storeDir,
	}
	if businessDate, err := resolveBusinessDate(ctx, deps.Stock, date); err != nil {
		pulse.Errors["market_time"] = err.Error()
	} else {
		pulse.BusinessDate = businessDate
	}

	// ── 1. 독립 시세를 제한 병렬 수집 ───────────────────────────────────────
	var kospiFlow, kosdaqFlow FlowSnapshot
	var kospiIdx, kosdaqIdx IndexLevel
	var kospiProgram, kosdaqProgram ProgramTradeSnapshot
	var kospi200Future, kosdaq150Future IndexFutureSnapshot
	var vkospi VolatilitySnapshot
	var wins map[string]Window
	marketErrors := map[string]string{}
	var kospiFlowErr, kosdaqFlowErr, kospiIdxErr, kosdaqIdxErr error
	var kospiProgramErr, kosdaqProgramErr, kospiFutureErr, kosdaqFutureErr, vkospiErr error

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(6)
	g.Go(func() error { kospiFlow, kospiFlowErr = collectFlow(gctx, deps.Stock, "KSP", "0001"); return nil })
	g.Go(func() error { kosdaqFlow, kosdaqFlowErr = collectFlow(gctx, deps.Stock, "KSQ", "1001"); return nil })
	g.Go(func() error { kospiIdx, kospiIdxErr = collectIndex(gctx, deps.Stock, "0001"); return nil })
	g.Go(func() error { kosdaqIdx, kosdaqIdxErr = collectIndex(gctx, deps.Stock, "1001"); return nil })
	g.Go(func() error { kospiProgram, kospiProgramErr = collectProgramTrade(gctx, deps.Stock, "K"); return nil })
	g.Go(func() error { kosdaqProgram, kosdaqProgramErr = collectProgramTrade(gctx, deps.Stock, "Q"); return nil })
	g.Go(func() error {
		kospi200Future, kospiFutureErr = collectIndexFuture(gctx, deps.Future, pulse.BusinessDate, "KOSPI200")
		kosdaq150Future, kosdaqFutureErr = collectIndexFuture(gctx, deps.Future, pulse.BusinessDate, "KOSDAQ150")
		return nil
	})
	g.Go(func() error { vkospi, vkospiErr = collectVKOSPI(gctx, deps.Stock, deps.Naver); return nil })
	g.Go(func() error { wins = collectMarket(gctx, deps.Yahoo, now, marketErrors); return nil })
	_ = g.Wait()

	for key, item := range map[string]error{
		"kospi_flow": kospiFlowErr, "kosdaq_flow": kosdaqFlowErr,
		"kospi_index": kospiIdxErr, "kosdaq_index": kosdaqIdxErr,
		"kospi_program": kospiProgramErr, "kosdaq_program": kosdaqProgramErr,
		"kospi200_future": kospiFutureErr, "kosdaq150_future": kosdaqFutureErr,
		"vkospi": vkospiErr,
	} {
		if item != nil {
			pulse.Errors[key] = item.Error()
		}
	}
	for key, value := range marketErrors {
		pulse.Errors[key] = value
	}
	if kospi200Future.OK && !kospi200Future.BasisMatch {
		pulse.Errors["kospi200_basis"] = fmt.Sprintf("계산 베이시스 %.2f와 KIS mrkt_basis %.2f 불일치", kospi200Future.Basis, kospi200Future.MarketBasis)
	}

	// ── 2. KOSPI 기여도 (지수 수집 후) ─────────────────────────────────────
	contributions, contributionErr := collectContributions(ctx, deps.Stock, pulse.BusinessDate, kospiIdx)
	if contributionErr != nil {
		pulse.Errors["kospi_contribution"] = contributionErr.Error()
	}
	if wins == nil {
		wins = map[string]Window{}
	}

	usdkrw := wins["KRW=X"]
	kospiWin := wins["^KS11"]
	kosdaqWin := wins["^KQ11"]
	macroWins := []Window{
		wins["NQ=F"],
		wins["ES=F"],
		wins["YM=F"],
		wins["^N225"],
		wins["CL=F"],
		wins["^TNX"],
	}

	// ── 5. 레코드 로드 → 델타 계산 ─────────────────────────────────────────
	records, loadErr := LoadRecords(storeDir, date)
	if loadErr != nil {
		pulse.Errors["store_load"] = loadErr.Error()
	}
	pulse.StoredCount = len(records)
	if len(records) > 0 {
		t := records[len(records)-1].TS
		pulse.PrevTS = &t
	}

	kospiFlowDelta1h, kospiFlowDelta2h := computeFlowDeltas(records, now, kospiFlow, kospiIdx.Price, "kospi")
	kosdaqFlowDelta1h, kosdaqFlowDelta2h := computeFlowDeltas(records, now, kosdaqFlow, kosdaqIdx.Price, "kosdaq")
	kospiProgramDelta := computeProgramDelta(records, now, kospiProgram, "kospi")
	kosdaqProgramDelta := computeProgramDelta(records, now, kosdaqProgram, "kosdaq")
	basisDelta1h := computeBasisDelta(records, now, kospi200Future, 1)
	basisDelta2h := computeBasisDelta(records, now, kospi200Future, 2)

	// ── 6. 시장 구조체 조립 ─────────────────────────────────────────────────
	pulse.KOSPI = Market{
		Name:        "KOSPI",
		Index:       kospiIdx,
		IntradayWin: kospiWin,
		Flow:        kospiFlow,
		FlowDelta1h: kospiFlowDelta1h,
		FlowDelta2h: kospiFlowDelta2h,
	}
	pulse.KOSDAQ = Market{
		Name:        "KOSDAQ",
		Index:       kosdaqIdx,
		IntradayWin: kosdaqWin,
		Flow:        kosdaqFlow,
		FlowDelta1h: kosdaqFlowDelta1h,
		FlowDelta2h: kosdaqFlowDelta2h,
	}
	pulse.KOSPIProgram = kospiProgram
	pulse.KOSDAQProgram = kosdaqProgram
	pulse.KOSPIProgramDelta = kospiProgramDelta
	pulse.KOSDAQProgramDelta = kosdaqProgramDelta
	pulse.KOSPI200Future = kospi200Future
	pulse.KOSDAQ150Future = kosdaq150Future
	pulse.BasisDelta1h = basisDelta1h
	pulse.BasisDelta2h = basisDelta2h
	pulse.VKOSPI = vkospi
	pulse.Safety = buildMarketSafety(kospiIdx, kosdaqIdx, kospi200Future, kosdaq150Future)
	pulse.Contributions = contributions
	pulse.USDKRW = usdkrw
	pulse.Macro = macroWins

	// ── 7. 분석 ────────────────────────────────────────────────────────────
	pulse.Analysis = Analyze(pulse)

	// ── 8. 적립 (--no-save가 아닌 경우) ────────────────────────────────────
	if !opts.NoSave {
		rec := PulseRecord{
			TS:             now,
			BusinessDate:   pulse.BusinessDate,
			KOSPIIdx:       kospiIdx.Price,
			KOSDAQIdx:      kosdaqIdx.Price,
			KOSPIFlow:      kospiFlow,
			KOSDAQFlow:     kosdaqFlow,
			KOSPIProgram:   kospiProgram,
			KOSDAQProgram:  kosdaqProgram,
			KOSPI200Future: kospi200Future,
			VKOSPI:         vkospi,
			USDKRW:         usdkrw.Current,
		}
		if appendErr := AppendRecord(storeDir, date, rec); appendErr != nil {
			pulse.Errors["store_append"] = appendErr.Error()
		} else {
			pulse.StoredCount++
			pulse.Saved = true
		}
	}

	return pulse
}

// computeFlowDeltas는 1h / 2h 수급 델타를 계산합니다.
func computeFlowDeltas(records []PulseRecord, now time.Time, cur FlowSnapshot, curIdx float64, market string) (*FlowDelta, *FlowDelta) {
	if !cur.OK {
		return nil, nil
	}

	var delta1h, delta2h *FlowDelta
	for _, h := range []int{1, 2} {
		target := now.Add(-time.Duration(h) * time.Hour)
		prev := LoadNearest(records, target)
		if prev == nil {
			continue
		}
		elapsed := now.Sub(prev.TS).Minutes()

		var prevFlow FlowSnapshot
		var prevIdx float64
		if market == "kospi" {
			prevFlow = prev.KOSPIFlow
			prevIdx = prev.KOSPIIdx
		} else {
			prevFlow = prev.KOSDAQFlow
			prevIdx = prev.KOSDAQIdx
		}

		d := &FlowDelta{
			RefTS:       prev.TS,
			Elapsed:     elapsed,
			Foreign:     cur.Foreign - prevFlow.Foreign,
			Institution: cur.Institution - prevFlow.Institution,
			Individual:  cur.Individual - prevFlow.Individual,
			IndexDelta:  curIdx - prevIdx,
		}
		if h == 1 {
			delta1h = d
		} else {
			delta2h = d
		}
	}
	return delta1h, delta2h
}

// FlowAcceleration은 1h 델타가 직전 1h 대비 가속/둔화를 판단합니다.
func FlowAcceleration(delta1h, delta2h *FlowDelta, field func(*FlowDelta) float64) string {
	if delta1h == nil || delta2h == nil {
		return ""
	}
	currentMinutes := delta1h.Elapsed
	previousMinutes := delta2h.Elapsed - delta1h.Elapsed
	if currentMinutes <= 0 || previousMinutes <= 0 {
		return ""
	}
	currentRate := hourlyRate(field(delta1h), currentMinutes)
	previousRate := hourlyRate(field(delta2h)-field(delta1h), previousMinutes)
	if sign(currentRate) != 0 && sign(previousRate) != 0 && sign(currentRate) != sign(previousRate) {
		if currentRate > 0 {
			return "매수전환"
		}
		return "매도전환"
	}
	if absDelta(currentRate) > absDelta(previousRate) {
		return "가속"
	}
	if absDelta(currentRate) < absDelta(previousRate) {
		return "둔화"
	}
	return "유지"
}

func absDelta(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// resolveStoreDir는 옵션/환경변수/기본값 순으로 적립 디렉터리를 반환합니다.
func resolveStoreDir(opts Options) string {
	if opts.StoreDir != "" {
		return opts.StoreDir
	}
	if v := os.Getenv("PULSE_OUTPUT_DIR"); v != "" {
		return v
	}
	return ".cache/pulse"
}

// StorePath는 적립 JSONL 경로를 반환합니다.
func StorePath(opts Options, date string) string {
	return pulseFilePath(resolveStoreDir(opts), date)
}

// MDPath는 MD 저장 경로를 반환합니다.
func MDPath(opts Options, date string) string {
	return pulseMDPath(resolveStoreDir(opts), date)
}

// SaveMD는 렌더 결과를 MD 파일로 저장합니다.
func SaveMD(opts Options, date, content string) error {
	dir := resolveStoreDir(opts)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(pulseMDPath(dir, date), []byte(content), 0o644)
}
