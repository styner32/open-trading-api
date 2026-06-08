package snapshot

import (
	"context"
	"fmt"
	"strings"
)

// collectLateSession gathers late-session supply-demand statistics, basis, and checks for capitulation.
func collectLateSession(ctx context.Context, deps Deps, date string, priceSec *PriceSection) (*LateSessionSection, error) {
	section := &LateSessionSection{
		BusinessDate: date,
	}

	// 1. 선물-현물 베이시스 계산
	if err := fillBasis(ctx, deps, date, section); err != nil {
		fmt.Printf("[latesession] Warning: failed to fetch basis: %v\n", err)
	}

	// 2. 투자자별 프로그램매매 당일 동향 (코스피)
	if err := fillProgramTradeToday(ctx, deps, section); err != nil {
		fmt.Printf("[latesession] Warning: failed to fetch program trade today: %v\n", err)
	}

	// 3. 시간대별 프로그램 매매 추이 (15:00 vs 15:20 vs 15:30)
	if err := fillLateProgramFlow(ctx, deps, section); err != nil {
		fmt.Printf("[latesession] Warning: failed to fetch late program flow: %v\n", err)
	}

	// 4. 시간대별 투자자 매매 동향 (15:20 vs 15:30)
	if err := fillCloseSessionInvestorFlow(ctx, deps, section); err != nil {
		fmt.Printf("[latesession] Note: close session investor flow not fully available: %v\n", err)
	}

	// 5. Capitulation 이벤트 판정
	evaluateCapitulation(priceSec, section)

	return section, nil
}

func fillBasis(ctx context.Context, deps Deps, date string, sec *LateSessionSection) error {
	// 1) 현물 (KOSPI 200) 가격 조회 (코드: 2001)
	spotResp, err := deps.DomesticStock.InquireIndexPrice(ctx, "2001")
	if err != nil {
		return fmt.Errorf("fetch KOSPI200 spot price: %w", err)
	}
	spotRow := firstRow(spotResp, "output")
	if spotRow == nil {
		return fmt.Errorf("KOSPI200 spot price output is nil")
	}
	spotPrice, ok := num(spotRow, "bstp_nmix_prpr")
	if !ok {
		return fmt.Errorf("KOSPI200 spot price 'bstp_nmix_prpr' missing")
	}

	// 2) 최근월물 선물 코드 조회
	resolvedContract, err := deps.DomesticFuture.ResolveNearMonthKOSPI200Futures(ctx, date)
	if err != nil {
		return fmt.Errorf("resolve near-month futures: %w", err)
	}
	futuresCode := resolvedContract.Record.ShortCode

	// 3) 선물 가격 조회
	futuresResp, err := deps.DomesticFuture.InquirePrice(ctx, "F", futuresCode)
	if err != nil {
		return fmt.Errorf("fetch futures price: %w", err)
	}
	if futuresResp == nil || futuresResp.Body == nil {
		return fmt.Errorf("futures price response body is nil")
	}

	futuresRow := firstRow(futuresResp, "output1", "output")
	if futuresRow == nil {
		futuresRow = futuresResp.Body
	}

	futuresPrice, ok := num(futuresRow, "futs_prpr", "stck_prpr")
	if !ok {
		return fmt.Errorf("futures price 'futs_prpr'/'stck_prpr' missing")
	}

	sec.SpotPrice = spotPrice
	sec.FuturesPrice = futuresPrice
	sec.BasisPoint = futuresPrice - spotPrice
	if spotPrice > 0 {
		sec.BasisRate = (sec.BasisPoint / spotPrice) * 100
	}

	return nil
}

func fillProgramTradeToday(ctx context.Context, deps Deps, sec *LateSessionSection) error {
	resp, err := deps.DomesticStock.InvestorProgramTradeToday(ctx, "1") // 1: 코스피
	if err != nil {
		return err
	}

	rowsList := rows(resp, "output1")
	for _, row := range rowsList {
		nameVal, ok := row["invr_cls_name"].(string)
		if !ok {
			continue
		}
		netAmt, ok := num(row, "nabt_ntby_amt") // 비차익 순매수 금액 (백만 원 단위)
		if !ok {
			continue
		}
		netAmtEok := netAmt / 100.0

		name := strings.TrimSpace(nameVal)
		if strings.Contains(name, "외국인") {
			sec.KOSPINetNonArbitrageForeign = netAmtEok
		} else if strings.Contains(name, "기관") {
			sec.KOSPINetNonArbitrageOrgan = netAmtEok
		} else if strings.Contains(name, "합계") || strings.Contains(name, "전체") {
			sec.KOSPINetNonArbitrageTotal = netAmtEok
		}
	}
	return nil
}

func fillLateProgramFlow(ctx context.Context, deps Deps, sec *LateSessionSection) error {
	resp, err := deps.DomesticStock.CompProgramTradeToday(ctx, "K") // K: 코스피
	if err != nil {
		return err
	}

	rowsList := rows(resp, "output")
	if len(rowsList) == 0 {
		return fmt.Errorf("comp program trade today empty output")
	}

	var p1500, p1520, p1530 float64
	var found1500, found1520, found1530 bool

	getTimeKey := func(row map[string]any) string {
		for _, k := range []string{"bsop_hour", "stck_cntg_hour", "cntg_hour", "aspr_hour"} {
			if val, ok := row[k].(string); ok && val != "" {
				return strings.TrimSpace(val)
			}
		}
		return ""
	}

	for _, row := range rowsList {
		hourVal := getTimeKey(row)
		if hourVal == "" {
			continue
		}
		val, ok := num(row, "whol_smtn_ntby_tr_pbmn")
		if !ok {
			continue
		}

		// 15:00 부근 매칭 (HHMMSS)
		if strings.HasPrefix(hourVal, "1500") || (strings.Compare(hourVal, "150000") >= 0 && strings.Compare(hourVal, "150500") <= 0) {
			p1500 = val
			found1500 = true
		}
		// 15:20 부근 매칭
		if strings.HasPrefix(hourVal, "1520") || (strings.Compare(hourVal, "152000") >= 0 && strings.Compare(hourVal, "152200") <= 0) {
			p1520 = val
			found1520 = true
		}
		// 15:30 마감 부근 매칭
		if strings.HasPrefix(hourVal, "1530") || strings.Compare(hourVal, "153000") >= 0 {
			p1530 = val
			found1530 = true
		}
	}

	// 15:00/15:20/15:30 명시적 데이터가 존재하지 않는 경우를 위한 폴백 (예: 장마감 한참 후 조회하여 데이터 컷 발생 시)
	if !found1530 && len(rowsList) > 0 {
		// 최신 시각 데이터(보통 인덱스 0)를 15:30으로 폴백
		val, ok := num(rowsList[0], "whol_smtn_ntby_tr_pbmn")
		if ok {
			p1530 = val
			found1530 = true
		}
	}

	if !found1500 && len(rowsList) > 0 {
		// 가장 오래된 시각 데이터(마지막 인덱스)를 15:00으로 폴백
		lastIdx := len(rowsList) - 1
		val, ok := num(rowsList[lastIdx], "whol_smtn_ntby_tr_pbmn")
		if ok {
			p1500 = val
			found1500 = true
		}
	}

	if !found1520 && len(rowsList) > 0 {
		// 15:30 마감 10분 전 혹은 중간 로우(마지막 1/3 지점)를 15:20으로 폴백
		targetIdx := len(rowsList) * 2 / 3
		if targetIdx >= len(rowsList) {
			targetIdx = len(rowsList) - 1
		}
		val, ok := num(rowsList[targetIdx], "whol_smtn_ntby_tr_pbmn")
		if ok {
			p1520 = val
			found1520 = true
		}
	}

	// 변화량 계산 (백만원 -> 억원)
	if found1500 && found1530 {
		sec.LateProgramNetEok = (p1530 - p1500) / 100.0
	}
	if found1520 && found1530 {
		sec.CloseSessionProgramNetEok = (p1530 - p1520) / 100.0
	}

	return nil
}

func fillCloseSessionInvestorFlow(ctx context.Context, deps Deps, sec *LateSessionSection) error {
	resp, err := deps.DomesticStock.InquireInvestorTimeByMarket(ctx, "KSP", "0001")
	if err != nil {
		return err
	}

	rowsList := rows(resp, "output")
	if len(rowsList) < 2 {
		return fmt.Errorf("too few rows for time-based flow (got %d), likely end-of-day summary", len(rowsList))
	}

	var f1520, f1530, o1520, o1530 float64
	var found1520, found1530 bool

	getTimeKey := func(row map[string]any) string {
		for _, k := range []string{"aspr_hour", "bsop_hour", "stck_cntg_hour", "cntg_hour"} {
			if val, ok := row[k].(string); ok && val != "" {
				return strings.TrimSpace(val)
			}
		}
		return ""
	}

	for _, row := range rowsList {
		hourVal := getTimeKey(row)
		if hourVal == "" {
			continue
		}

		frgnAmt, okF := num(row, "frgn_ntby_tr_pbmn")
		orgnAmt, okO := num(row, "orgn_ntby_tr_pbmn")
		if !okF || !okO {
			continue
		}

		if strings.HasPrefix(hourVal, "1520") || (strings.Compare(hourVal, "152000") >= 0 && strings.Compare(hourVal, "152100") <= 0) {
			f1520 = frgnAmt
			o1520 = orgnAmt
			found1520 = true
		}
		if strings.HasPrefix(hourVal, "1530") || strings.Compare(hourVal, "153000") >= 0 {
			f1530 = frgnAmt
			o1530 = orgnAmt
			found1530 = true
		}
	}

	if !found1520 {
		for _, row := range rowsList {
			hourVal := getTimeKey(row)
			if len(hourVal) >= 4 && strings.HasPrefix(hourVal, "152") {
				frgnAmt, okF := num(row, "frgn_ntby_tr_pbmn")
				orgnAmt, okO := num(row, "orgn_ntby_tr_pbmn")
				if okF && okO {
					f1520 = frgnAmt
					o1520 = orgnAmt
					found1520 = true
					break
				}
			}
		}
	}

	if !found1530 && len(rowsList) > 0 {
		frgnAmt, okF := num(rowsList[0], "frgn_ntby_tr_pbmn")
		orgnAmt, okO := num(rowsList[0], "orgn_ntby_tr_pbmn")
		if okF && okO {
			f1530 = frgnAmt
			o1530 = orgnAmt
			found1530 = true
		}
	}

	if found1520 && found1530 {
		sec.CloseSessionForeignNetEok = (f1530 - f1520) / 100.0
		sec.CloseSessionOrganNetEok = (o1530 - o1520) / 100.0
	}

	return nil
}

func evaluateCapitulation(priceSec *PriceSection, sec *LateSessionSection) {
	if priceSec == nil || priceSec.Low <= 0 {
		return
	}

	volatilityRange := (priceSec.High - priceSec.Low) / priceSec.Low
	closeToLow := (priceSec.Close - priceSec.Low) / priceSec.Low

	score := 0.0

	// 1) 고가 대비 반등 시도
	if volatilityRange >= 0.01 {
		score += 0.5
	}
	if volatilityRange >= 0.02 {
		score += 0.5
	}

	// 2) 저점 부근 종가 마감
	if closeToLow <= 0.003 {
		score += 1.0
	} else if closeToLow <= 0.006 {
		score += 0.5
	}

	// 3) 15시 이후 장 막판 프로그램 순매도
	if sec.LateProgramNetEok <= -300.0 {
		score += 0.5
	}
	if sec.LateProgramNetEok <= -700.0 {
		score += 0.5
	}

	// 4) 종가 동시호가(15:20~15:30) 외인 순매도 또는 프로그램 순매도
	if sec.CloseSessionForeignNetEok <= -100.0 {
		score += 0.5
	}
	if sec.CloseSessionForeignNetEok <= -300.0 {
		score += 0.5
	}
	if sec.CloseSessionProgramNetEok <= -200.0 {
		score += 0.5
	}
	if sec.CloseSessionProgramNetEok <= -500.0 {
		score += 0.5
	}

	// 5) 베이시스 백워데이션 상태
	if sec.BasisPoint < -0.5 {
		score += 0.5
	}

	sec.CapitulationScore = score

	if score >= 2.0 {
		sec.CapitulationEvent = true
	}
}
