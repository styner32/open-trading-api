package pulse

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/kis-open-api/go/internal/auth"
	"github.com/kis-open-api/go/internal/domesticstock"
	"github.com/kis-open-api/go/internal/parse"
	"golang.org/x/sync/errgroup"
)

type marketTimeStock interface {
	MarketTime(context.Context) (*auth.RESTResponse, error)
}

type programTradeStock interface {
	CompProgramTradeToday(context.Context, string) (*auth.RESTResponse, error)
}

type vkospiStock interface {
	ResolveVKOSPICode(context.Context, []string) (string, error)
	InquireVKOSPIPrice(context.Context, string) (*auth.RESTResponse, error)
}

type contributionStock interface {
	KOSPIMarketCapSummary(context.Context, string) (*domesticstock.KOSPIMarketCapSummary, error)
	InquirePrice(context.Context, string) (*auth.RESTResponse, error)
}

func resolveBusinessDate(ctx context.Context, stock marketTimeStock, fallback string) (string, error) {
	resp, err := stock.MarketTime(ctx)
	if err != nil {
		return fallback, err
	}
	row := resp.FirstRow("output1")
	if row == nil {
		return fallback, fmt.Errorf("market-time output1 행 없음")
	}
	today := normalizeYMD(valueString(row["today"]))
	latest := ""
	for _, key := range []string{"date1", "date2", "date3", "date4", "date5"} {
		date := normalizeYMD(valueString(row[key]))
		if date == "" {
			continue
		}
		if today != "" && date == today {
			return today, nil
		}
		if (today == "" || date <= today) && date > latest {
			latest = date
		}
	}
	if latest != "" {
		return latest, nil
	}
	if today != "" {
		return today, nil
	}
	return fallback, fmt.Errorf("market-time 영업일 필드 없음")
}

func normalizeYMD(v string) string {
	v = strings.TrimSpace(v)
	if len(v) != 8 {
		return ""
	}
	for _, r := range v {
		if r < '0' || r > '9' {
			return ""
		}
	}
	return v
}

func valueString(v any) string {
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s)
	}
	return fmt.Sprint(v)
}

func collectProgramTrade(ctx context.Context, stock programTradeStock, marketClass string) (ProgramTradeSnapshot, error) {
	resp, err := stock.CompProgramTradeToday(ctx, marketClass)
	if err != nil {
		return ProgramTradeSnapshot{}, err
	}
	rows := resp.Rows("output")
	if len(rows) == 0 {
		return ProgramTradeSnapshot{}, fmt.Errorf("comp-program-trade-today (%s): output 행 없음", marketClass)
	}

	latest := rows[0]
	latestHour := valueString(latest["bsop_hour"])
	for _, row := range rows[1:] {
		hour := valueString(row["bsop_hour"])
		if hour > latestHour {
			latest, latestHour = row, hour
		}
	}
	get := func(key string) (float64, error) {
		v, ok := parse.Num(latest, key)
		if !ok {
			return 0, fmt.Errorf("%s 필드 없음", key)
		}
		return v / millionToEok, nil
	}
	arbitrage, err := get("arbt_smtn_ntby_tr_pbmn")
	if err != nil {
		return ProgramTradeSnapshot{}, err
	}
	nonArbitrage, err := get("nabt_smtn_ntby_tr_pbmn")
	if err != nil {
		return ProgramTradeSnapshot{}, err
	}
	total, err := get("whol_smtn_ntby_tr_pbmn")
	if err != nil {
		return ProgramTradeSnapshot{}, err
	}
	asOf := latestHour
	// KIS는 장 마감 후에도 15:30~17시대 행을 같은 종가 데이터로 반복한다.
	if asOf >= "153000" {
		asOf = "close"
	}
	return ProgramTradeSnapshot{
		Arbitrage: arbitrage, NonArbitrage: nonArbitrage, Total: total,
		AsOf: asOf, OK: true,
	}, nil
}

func collectIndexFuture(ctx context.Context, future DomesticFuture, businessDate, market string) (IndexFutureSnapshot, error) {
	if future == nil {
		return IndexFutureSnapshot{}, fmt.Errorf("domestic future service is nil")
	}
	var code, name string
	if market == "KOSPI200" {
		resolved, err := future.ResolveNearMonthKOSPI200Futures(ctx, businessDate)
		if err != nil {
			return IndexFutureSnapshot{}, err
		}
		code, name = resolved.Record.ShortCode, resolved.Record.Name
	} else {
		resolved, err := future.ResolveNearMonthKOSDAQ150Futures(ctx, businessDate)
		if err != nil {
			return IndexFutureSnapshot{}, err
		}
		code, name = resolved.Record.ShortCode, resolved.Record.Name
	}
	resp, err := future.InquirePrice(ctx, "F", code)
	if err != nil {
		return IndexFutureSnapshot{}, err
	}
	futureRow := resp.FirstRow("output1", "output")
	spotRow := resp.FirstRow("output3")
	if futureRow == nil || spotRow == nil {
		return IndexFutureSnapshot{}, fmt.Errorf("index future %s: output1/output3 행 없음", code)
	}
	price, ok := parse.Num(futureRow, "futs_prpr")
	if !ok {
		return IndexFutureSnapshot{}, fmt.Errorf("index future %s: futs_prpr 없음", code)
	}
	prevClose, _ := parse.Num(futureRow, "futs_prdy_clpr", "futs_sdpr")
	changePct, _ := parse.Num(futureRow, "futs_prdy_ctrt")
	spotPrice, ok := parse.Num(spotRow, "bstp_nmix_prpr")
	if !ok {
		return IndexFutureSnapshot{}, fmt.Errorf("index future %s: output3 spot 없음", code)
	}
	spotChangePct, _ := parse.Num(spotRow, "bstp_nmix_prdy_ctrt")
	computedBasis := price - spotPrice
	marketBasis, marketBasisOK := parse.Num(futureRow, "mrkt_basis")
	basisMatch := !marketBasisOK || math.Abs(computedBasis-marketBasis) <= 0.05
	return IndexFutureSnapshot{
		Code: code, Name: name, Price: price, PrevClose: prevClose, ChangePct: changePct,
		SpotPrice: spotPrice, SpotChangePct: spotChangePct, Basis: computedBasis,
		MarketBasis: marketBasis, BasisMatch: basisMatch, OK: true,
	}, nil
}

func collectVKOSPI(ctx context.Context, stock vkospiStock, naverClient NaverFinance) (VolatilitySnapshot, error) {
	var lastErr error
	for _, code := range []string{"0503", "2050"} {
		resp, err := stock.InquireVKOSPIPrice(ctx, code)
		if err == nil {
			if row := resp.FirstRow("output"); row != nil {
				value, valueOK := parse.Num(row, "bstp_nmix_prpr")
				change, _ := parse.Num(row, "bstp_nmix_prdy_ctrt")
				if valueOK && value >= 5 && value <= 100 {
					return VolatilitySnapshot{Code: code, Value: value, ChangePct: change, Source: "KIS", OK: true}, nil
				}
			}
		} else {
			lastErr = err
		}
	}
	code, resolveErr := stock.ResolveVKOSPICode(ctx, nil)
	if resolveErr == nil && code != "0503" && code != "2050" {
		resp, err := stock.InquireVKOSPIPrice(ctx, code)
		if err == nil {
			if row := resp.FirstRow("output"); row != nil {
				value, valueOK := parse.Num(row, "bstp_nmix_prpr")
				change, _ := parse.Num(row, "bstp_nmix_prdy_ctrt")
				if valueOK && value >= 5 && value <= 100 {
					return VolatilitySnapshot{Code: code, Value: value, ChangePct: change, Source: "KIS", OK: true}, nil
				}
			}
		} else if err != nil {
			lastErr = err
		}
	}
	if naverClient != nil {
		quote, err := naverClient.GetIndexQuote(ctx, "VKOSPI")
		if err == nil && quote != nil && quote.Price >= 5 && quote.Price <= 100 {
			return VolatilitySnapshot{Code: "VKOSPI", Value: quote.Price, ChangePct: quote.ChangePercent, Source: "Naver", OK: true}, nil
		}
	}
	if lastErr != nil {
		return VolatilitySnapshot{}, lastErr
	}
	if resolveErr != nil {
		return VolatilitySnapshot{}, resolveErr
	}
	return VolatilitySnapshot{}, fmt.Errorf("VKOSPI KIS/Naver 조회 실패")
}

func collectContributions(ctx context.Context, stock contributionStock, businessDate string, idx IndexLevel) ([]IndexContribution, error) {
	if !idx.OK || idx.PrevClose <= 0 {
		return nil, fmt.Errorf("KOSPI 전일 종가 없음")
	}
	summary, err := stock.KOSPIMarketCapSummary(ctx, businessDate)
	if err != nil {
		return nil, err
	}
	if summary.TotalMarketCap <= 0 {
		return nil, fmt.Errorf("KOSPI 총 시가총액 없음")
	}
	limit := 10
	if len(summary.Constituents) < limit {
		limit = len(summary.Constituents)
	}
	out := make([]IndexContribution, 0, limit)
	var failures []string
	var mu sync.Mutex
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(4)
	for _, item := range summary.Constituents[:limit] {
		item := item
		g.Go(func() error {
			resp, reqErr := stock.InquirePrice(gctx, item.Code)
			if reqErr != nil {
				mu.Lock()
				failures = append(failures, item.Code)
				mu.Unlock()
				return nil
			}
			row := resp.FirstRow("output")
			changePct, ok := parse.Num(row, "prdy_ctrt")
			if row == nil || !ok {
				mu.Lock()
				failures = append(failures, item.Code)
				mu.Unlock()
				return nil
			}
			weight := item.MarketCap / summary.TotalMarketCap
			contribution := IndexContribution{
				Code: item.Code, Name: item.Name, MarketCap: item.MarketCap,
				WeightPct: weight * 100, ChangePct: changePct,
				PointImpact: idx.PrevClose * weight * changePct / 100,
			}
			mu.Lock()
			out = append(out, contribution)
			mu.Unlock()
			return nil
		})
	}
	_ = g.Wait()
	sort.SliceStable(out, func(i, j int) bool {
		return math.Abs(out[i].PointImpact) > math.Abs(out[j].PointImpact)
	})
	if len(out) == 0 {
		return nil, fmt.Errorf("KOSPI 상위종목 시세 조회 실패: %s", strings.Join(failures, ","))
	}
	if len(failures) > 0 {
		return out, fmt.Errorf("일부 종목 시세 조회 실패: %s", strings.Join(failures, ","))
	}
	return out, nil
}

func buildMarketSafety(kospi, kosdaq IndexLevel, k200, kq150 IndexFutureSnapshot) MarketSafety {
	s := MarketSafety{}
	for _, item := range []struct {
		name string
		idx  IndexLevel
	}{{"KOSPI", kospi}, {"KOSDAQ", kosdaq}} {
		if !item.idx.OK || item.idx.PrevClose <= 0 {
			continue
		}
		lowPct := (item.idx.Low - item.idx.PrevClose) / item.idx.PrevClose * 100
		cb := CircuitBreakerStatus{Market: item.name, CurrentChangePct: item.idx.ChangePct, LowChangePct: lowPct, OK: true}
		for _, threshold := range []float64{8, 15, 20} {
			currentGap, currentReached := downsideGap(item.idx.ChangePct, threshold)
			lowGap, lowReached := downsideGap(lowPct, threshold)
			cb.Levels = append(cb.Levels, ThresholdStatus{
				ThresholdPct: threshold, CurrentGapPct: currentGap, LowGapPct: lowGap,
				CurrentReached: currentReached, LowReached: lowReached,
			})
		}
		s.CircuitBreakers = append(s.CircuitBreakers, cb)
	}
	if k200.OK {
		s.Sidecars = append(s.Sidecars, buildSidecar("KOSPI", k200, 5, 0))
	}
	if kq150.OK {
		s.Sidecars = append(s.Sidecars, buildSidecar("KOSDAQ", kq150, 6, 3))
	}
	return s
}

func downsideGap(changePct, threshold float64) (float64, bool) {
	gap := changePct + threshold
	if gap <= 0 {
		return 0, true
	}
	return gap, false
}


func buildSidecar(market string, f IndexFutureSnapshot, futuresThreshold, spotThreshold float64) SidecarStatus {
	direction := "보합"
	if f.ChangePct > 0 {
		direction = "상승"
	} else if f.ChangePct < 0 {
		direction = "하락"
	}
	futuresGap := math.Max(0, futuresThreshold-math.Abs(f.ChangePct))
	spotGap := 0.0
	spotReached := true
	if spotThreshold > 0 {
		spotGap = math.Max(0, spotThreshold-math.Abs(f.SpotChangePct))
		spotReached = spotGap == 0 && sign(f.ChangePct) == sign(f.SpotChangePct)
	}
	return SidecarStatus{
		Market: market, FuturesCode: f.Code, Direction: direction,
		FuturesChangePct: f.ChangePct, SpotChangePct: f.SpotChangePct,
		FuturesThresholdPct: futuresThreshold, SpotThresholdPct: spotThreshold,
		FuturesGapPct: futuresGap, SpotGapPct: spotGap,
		ThresholdReached:    futuresGap == 0 && spotReached,
		ActivationConfirmed: false, OK: true,
	}
}

func computeProgramDelta(records []PulseRecord, now time.Time, cur ProgramTradeSnapshot, market string) *ProgramTradeDelta {
	if !cur.OK {
		return nil
	}
	prev := LoadNearest(records, now.Add(-time.Hour))
	if prev == nil {
		return nil
	}
	previous := prev.KOSPIProgram
	if market == "kosdaq" {
		previous = prev.KOSDAQProgram
	}
	if !previous.OK {
		return nil
	}
	return &ProgramTradeDelta{
		RefTS: prev.TS, Elapsed: now.Sub(prev.TS).Minutes(),
		Arbitrage:    cur.Arbitrage - previous.Arbitrage,
		NonArbitrage: cur.NonArbitrage - previous.NonArbitrage,
		Total:        cur.Total - previous.Total,
	}
}

func computeBasisDelta(records []PulseRecord, now time.Time, cur IndexFutureSnapshot, hours int) *BasisDelta {
	if !cur.OK {
		return nil
	}
	prev := LoadNearest(records, now.Add(-time.Duration(hours)*time.Hour))
	if prev == nil || !prev.KOSPI200Future.OK {
		return nil
	}
	return &BasisDelta{RefTS: prev.TS, Elapsed: now.Sub(prev.TS).Minutes(), Value: cur.Basis - prev.KOSPI200Future.Basis}
}
