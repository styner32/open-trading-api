package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/kis-open-api/go/internal/auth"
	"github.com/kis-open-api/go/internal/domesticstock"
	"github.com/kis-open-api/go/internal/overseasstock"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	appKey := os.Getenv("APP_KEY")
	secretKey := os.Getenv("APP_SECRET")
	userAgent := os.Getenv("USER_AGENT")
	baseURL := "https://openapi.koreainvestment.com:9443"
	tokenCachePath := getOrDefault("AUTH_TOKEN_FILE", ".auth_token.json")
	targetSymbol := getOrDefault("TARGET_SYMBOL", "005930")
	//	crudeOilProductCode := getOrDefault("CRUDE_OIL_PRODUCT_CODE", "CL")
	ewySymbol := getOrDefault("EWY_SYMBOL", "EWY")

	client := auth.NewKIClient(appKey, secretKey, baseURL, userAgent)
	client.SetTokenCachePath(tokenCachePath)

	token, err := client.EnsureAuthToken(context.Background())
	if err != nil {
		log.Fatalf("Error ensuring auth token: %v", err)
	}

	fmt.Printf("\n--- Auth Token Issued ---\n")
	fmt.Printf("Token Cache File: %s\n", tokenCachePath)
	fmt.Printf("Access Token (prefix): %.16s...\n", client.AuthToken)
	fmt.Printf("Valid Until: %s\n", token.TokenExpired)
	fmt.Printf("-------------------------\n")

	printUsefulEndpoints()

	svc := domesticstock.NewService(client)
	//	overseasFutureSvc := overseasfuture.NewService(client)
	overseasStockSvc := overseasstock.NewService(client)

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	today := time.Now().Format("20060102")
	fromDate := time.Now().AddDate(0, -6, 0).Format("20060102")

	respMarketTime, err := svc.MarketTime(ctx)
	mustAPIResult("market-time", respMarketTime, err, "output1")
	printMarketTimeSummary(respMarketTime)

	respKOSPI, err := svc.InquireIndexPrice(ctx, "0001")
	mustAPIResult("inquire-index-price (KOSPI 0001)", respKOSPI, err, "output")
	printIndexSummary("KOSPI", respKOSPI)

	respKOSDAQ, err := svc.InquireIndexPrice(ctx, "1001")
	mustAPIResult("inquire-index-price (KOSDAQ 1001)", respKOSDAQ, err, "output")
	printIndexSummary("KOSDAQ", respKOSDAQ)

	vkospiCode, err := svc.ResolveVKOSPICode(ctx, nil)
	if err != nil {
		log.Fatalf("VKOSPI code resolve error: %v", err)
	}
	respVKOSPI, err := svc.InquireIndexPrice(ctx, vkospiCode)
	mustAPIResult("inquire-index-price (VKOSPI "+vkospiCode+")", respVKOSPI, err, "output")
	printIndexSummary("VKOSPI", respVKOSPI)

	respProgramTrade, err := svc.CompProgramTradeToday(ctx, "K")
	mustAPIResult("comp-program-trade-today (KOSPI)", respProgramTrade, err, "output")
	printProgramTradeSummary(respProgramTrade)

	respVI, err := svc.InquireVIStatus(ctx, today)
	mustAPIResult("inquire-vi-status", respVI, err, "output")
	printVISummary(respVI)

	respFunds, err := svc.MarketFunds(ctx, "")
	mustAPIResult("mktfunds", respFunds, err, "output")
	printMarketFundsSummary(respFunds)

	// crudeOilSeriesCode, err := overseasFutureSvc.ResolveCrudeOilSeriesCode(ctx)
	// if err != nil {
	// 	log.Fatalf("crude oil series code resolve error: %v", err)
	// }
	// respCrudeOil, err := overseasFutureSvc.InquirePrice(ctx, crudeOilSeriesCode)
	// mustAPIResult(
	// 	"overseas-future inquire-price ("+crudeOilProductCode+" "+crudeOilSeriesCode+")",
	// 	respCrudeOil,
	// 	err,
	// 	"output1",
	// )
	// printOverseasFutureSummary("Crude Oil Future", respCrudeOil)

	ewyExchangeCode, err := overseasStockSvc.ResolveEWYExchangeCode(ctx)
	if err != nil {
		log.Fatalf("EWY exchange code resolve error: %v", err)
	}
	respEWY, err := overseasStockSvc.Price(ctx, ewyExchangeCode, ewySymbol)
	mustAPIResult(
		"overseas-stock price ("+ewyExchangeCode+" "+ewySymbol+")",
		respEWY,
		err,
		"output",
	)
	printOverseasStockSummary("EWY", respEWY)

	rsiResult, err := svc.RSIFromDailyChart(ctx, targetSymbol, 14, fromDate, today)
	if err != nil {
		log.Fatalf("RSI calculation error: %v", err)
	}
	fmt.Printf("\n--- RSI Result ---\n")
	fmt.Printf("Symbol: %s\n", rsiResult.Symbol)
	fmt.Printf("Period: %d\n", rsiResult.Period)
	fmt.Printf("Sample Size: %d\n", rsiResult.SampleSize)
	fmt.Printf("RSI: %.2f\n", rsiResult.Last)
	fmt.Printf("Signal: %s\n", rsiResult.Signal)
	fmt.Printf("------------------\n")
}

func printUsefulEndpoints() {
	fmt.Printf("\n--- Useful Endpoints For Market Status ---\n")
	endpoints := []struct {
		Path    string
		Purpose string
	}{
		{"/uapi/domestic-stock/v1/quotations/market-time", "장 운영 시간/영업일"},
		{"/uapi/domestic-stock/v1/quotations/inquire-index-price", "KOSPI/KOSDAQ/KOSPI200/VKOSPI 현재지수"},
		{"/uapi/domestic-stock/v1/quotations/inquire-index-timeprice", "업종 분봉 지수"},
		{"/uapi/domestic-stock/v1/quotations/inquire-index-daily-price", "업종 일/주/월 지수"},
		{"/uapi/domestic-stock/v1/quotations/inquire-index-tickprice", "업종 틱 지수"},
		{"/uapi/domestic-stock/v1/quotations/comp-program-trade-today", "프로그램매매 시간대 동향"},
		{"/uapi/domestic-stock/v1/quotations/comp-program-trade-daily", "프로그램매매 일별 동향"},
		{"/uapi/domestic-stock/v1/quotations/investor-program-trade-today", "투자자 프로그램매매 당일 동향"},
		{"/uapi/domestic-stock/v1/quotations/inquire-investor-daily-by-market", "시장별 투자자매매 동향"},
		{"/uapi/domestic-stock/v1/quotations/foreign-institution-total", "외인/기관 매매집계"},
		{"/uapi/domestic-stock/v1/quotations/frgnmem-trade-estimate", "외국계 매매 가집계"},
		{"/uapi/domestic-stock/v1/quotations/investor-trend-estimate", "종목 외인/기관 추정 집계"},
		{"/uapi/domestic-stock/v1/quotations/inquire-vi-status", "VI 발동 현황"},
		{"/uapi/domestic-stock/v1/quotations/mktfunds", "증시자금 종합"},
		{"/uapi/domestic-stock/v1/quotations/inquire-daily-itemchartprice", "RSI/기술지표 원천(OHLCV)"},
		{"/uapi/domestic-stock/v1/quotations/inquire-time-itemconclusion", "종목 체결 시계열(초단위)"},
		{"/uapi/domestic-stock/v1/quotations/pbar-tratio", "매물대/거래비중"},
		{"/uapi/domestic-stock/v1/quotations/tradprt-byamt", "체결금액대별 매매비중"},
		{"/uapi/domestic-stock/v1/ranking/volume-power", "체결강도 상위"},
		{"/uapi/domestic-stock/v1/ranking/volume-rank", "거래량 순위"},
		{"/uapi/domestic-stock/v1/ranking/fluctuation", "상승/하락률 랭킹"},
		{"/uapi/domestic-stock/v1/ranking/near-new-highlow", "신고/신저 근접"},
		{"/uapi/domestic-stock/v1/ranking/market-cap", "시가총액 상위"},
		{"/uapi/domestic-stock/v1/quotations/capture-uplowprice", "상/하한가 포착"},
	}

	for _, item := range endpoints {
		fmt.Printf("- %s : %s\n", item.Path, item.Purpose)
	}
	fmt.Printf("------------------------------------------\n")
}

func printAPIResult(name string, resp *auth.RESTResponse, outputKey string) {
	if resp == nil {
		fmt.Printf("%s: response is nil\n", name)
		return
	}

	fmt.Printf("\n[%s]\n", name)
	fmt.Printf("rt_cd=%v msg_cd=%v msg1=%v\n", resp.Body["rt_cd"], resp.Body["msg_cd"], resp.Body["msg1"])
	if outputKey == "" {
		return
	}
	if value, ok := resp.Body[outputKey]; ok {
		fmt.Printf("output(%s)=%s\n", outputKey, preview(value))
	}
}

func mustAPIResult(name string, resp *auth.RESTResponse, err error, outputKey string) {
	if err != nil {
		log.Fatalf("%s error: %v", name, err)
	}
	if resp == nil {
		log.Fatalf("%s error: response is nil", name)
	}
	if !resp.IsOK() {
		log.Fatalf("%s error: msg_cd=%s msg1=%s", name, resp.MessageCode(), resp.Message())
	}

	printAPIResult(name, resp, outputKey)
}

func preview(value any) string {
	switch typed := value.(type) {
	case map[string]any:
		return fmt.Sprintf("%v", typed)
	case []any:
		if len(typed) == 0 {
			return "[]"
		}
		return fmt.Sprintf("len=%d first=%v", len(typed), typed[0])
	default:
		return fmt.Sprintf("%v", typed)
	}
}

func getOrDefault(key string, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func printMarketTimeSummary(resp *auth.RESTResponse) {
	row := firstRow(resp, "output1")
	if row == nil {
		return
	}

	dates := joinNonEmpty(", ",
		fieldString(row, "date1"),
		fieldString(row, "date2"),
		fieldString(row, "date3"),
		fieldString(row, "date4"),
		fieldString(row, "date5"),
	)

	printSummaryBlock("Market Time Summary", []string{
		formatSummaryLine("Today", fieldString(row, "today")),
		formatSummaryLine("Now", fieldString(row, "time")),
		formatSummaryLine("Open / Close", joinNonEmpty(" / ", fieldString(row, "s_time"), fieldString(row, "e_time"))),
		formatSummaryLine("Business Days", dates),
	})
}

func printIndexSummary(name string, resp *auth.RESTResponse) {
	row := firstRow(resp, "output")
	if row == nil {
		return
	}

	printSummaryBlock(name+" Summary", []string{
		formatSummaryLine("Current", firstNonEmpty(row, "bstp_nmix_prpr", "stck_prpr")),
		formatSummaryLine("Change", firstNonEmpty(row, "bstp_nmix_prdy_vrss", "prdy_vrss")),
		formatSummaryLine("Rate", firstNonEmpty(row, "bstp_nmix_prdy_ctrt", "prdy_ctrt")),
		formatSummaryLine("Open / High / Low", joinNonEmpty(" / ",
			firstNonEmpty(row, "bstp_nmix_oprc", "stck_oprc"),
			firstNonEmpty(row, "bstp_nmix_hgpr", "stck_hgpr"),
			firstNonEmpty(row, "bstp_nmix_lwpr", "stck_lwpr"),
		)),
		formatSummaryLine("Volume", firstNonEmpty(row, "acml_vol")),
		formatSummaryLine("Turnover", firstNonEmpty(row, "acml_tr_pbmn")),
	})
}

func printProgramTradeSummary(resp *auth.RESTResponse) {
	row := firstRow(resp, "output")
	if row == nil {
		return
	}

	printSummaryBlock("Program Trade Summary", []string{
		formatSummaryLine("Date", firstNonEmpty(row, "stck_bsop_date")),
		formatSummaryLine("Market Close", firstNonEmpty(row, "stck_clpr")),
		formatSummaryLine("Net Buy Qty", firstNonEmpty(row, "whol_smtn_ntby_qty")),
		formatSummaryLine("Net Buy Amount", firstNonEmpty(row, "whol_smtn_ntby_tr_pbmn")),
		formatSummaryLine("Buy / Sell Volume", joinNonEmpty(" / ",
			firstNonEmpty(row, "whol_smtn_shnu_vol"),
			firstNonEmpty(row, "whol_smtn_seln_vol"),
		)),
	})
}

func printVISummary(resp *auth.RESTResponse) {
	rows := rowsFromResponse(resp, "output")
	if len(rows) == 0 {
		return
	}

	lines := []string{
		formatSummaryLine("Triggered Items", fmt.Sprintf("%d", len(rows))),
	}

	limit := 3
	if len(rows) < limit {
		limit = len(rows)
	}
	for i := 0; i < limit; i++ {
		row := rows[i]
		item := fmt.Sprintf(
			"%s (%s) | VI=%s | %s~%s",
			firstNonEmpty(row, "hts_kor_isnm"),
			firstNonEmpty(row, "mksc_shrn_iscd"),
			firstNonEmpty(row, "vi_cls_code"),
			firstNonEmpty(row, "cntg_vi_hour"),
			firstNonEmpty(row, "vi_cncl_hour"),
		)
		lines = append(lines, formatSummaryLine(fmt.Sprintf("Item %d", i+1), item))
	}

	printSummaryBlock("VI Summary", lines)
}

func printMarketFundsSummary(resp *auth.RESTResponse) {
	row := firstRow(resp, "output")
	if row == nil {
		return
	}

	printSummaryBlock("Market Funds Summary", []string{
		formatSummaryLine("Business Date", firstNonEmpty(row, "bsop_date")),
		formatSummaryLine("Customer Deposit", firstNonEmpty(row, "cust_dpmn_amt")),
		formatSummaryLine("Deposit Change", firstNonEmpty(row, "cust_dpmn_amt_prdy_vrss")),
		formatSummaryLine("Credit Loan Balance", firstNonEmpty(row, "crdt_loan_rmnd")),
		formatSummaryLine("Futures Deposit", firstNonEmpty(row, "futs_tfam_amt")),
		formatSummaryLine("Amount Turnover", firstNonEmpty(row, "amt_tnrt")),
	})
}

func printOverseasFutureSummary(name string, resp *auth.RESTResponse) {
	row := firstRow(resp, "output1")
	if row == nil {
		return
	}

	printSummaryBlock(name+" Summary", []string{
		formatSummaryLine("Current", firstNonEmpty(row, "last_price")),
		formatSummaryLine("Change", firstNonEmpty(row, "prev_diff_price")),
		formatSummaryLine("Rate", firstNonEmpty(row, "prev_diff_rate")),
		formatSummaryLine("Open / High / Low", joinNonEmpty(" / ",
			firstNonEmpty(row, "open_price"),
			firstNonEmpty(row, "high_price"),
			firstNonEmpty(row, "low_price"),
		)),
		formatSummaryLine("Volume", firstNonEmpty(row, "vol")),
		formatSummaryLine("Exchange / Currency", joinNonEmpty(" / ",
			firstNonEmpty(row, "exch_cd"),
			firstNonEmpty(row, "crc_cd"),
		)),
		formatSummaryLine("Expiry", firstNonEmpty(row, "expr_date")),
	})
}

func printOverseasStockSummary(name string, resp *auth.RESTResponse) {
	row := firstRow(resp, "output")
	if row == nil {
		return
	}

	printSummaryBlock(name+" Summary", []string{
		formatSummaryLine("Symbol", firstNonEmpty(row, "symb", "rsym")),
		formatSummaryLine("Current", firstNonEmpty(row, "last")),
		formatSummaryLine("Change", firstNonEmpty(row, "diff")),
		formatSummaryLine("Rate", firstNonEmpty(row, "rate")),
		formatSummaryLine("Previous Close", firstNonEmpty(row, "base")),
		formatSummaryLine("Volume", firstNonEmpty(row, "tvol")),
		formatSummaryLine("Turnover", firstNonEmpty(row, "tamt")),
	})
}

func printSummaryBlock(title string, lines []string) {
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		filtered = append(filtered, line)
	}
	if len(filtered) == 0 {
		return
	}

	fmt.Printf("\n--- %s ---\n", title)
	for _, line := range filtered {
		fmt.Println(line)
	}
	fmt.Printf("%s\n", strings.Repeat("-", len(title)+8))
}

func formatSummaryLine(label string, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return fmt.Sprintf("%s: %s", label, value)
}

func firstRow(resp *auth.RESTResponse, outputKey string) map[string]any {
	rows := rowsFromResponse(resp, outputKey)
	if len(rows) == 0 {
		return nil
	}
	return rows[0]
}

func rowsFromResponse(resp *auth.RESTResponse, outputKey string) []map[string]any {
	if resp == nil || resp.Body == nil {
		return nil
	}

	raw, ok := resp.Body[outputKey]
	if !ok || raw == nil {
		return nil
	}

	switch typed := raw.(type) {
	case map[string]any:
		return []map[string]any{typed}
	case []any:
		rows := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			row, ok := item.(map[string]any)
			if !ok {
				continue
			}
			rows = append(rows, row)
		}
		return rows
	default:
		return nil
	}
}

func firstNonEmpty(row map[string]any, keys ...string) string {
	for _, key := range keys {
		value := fieldString(row, key)
		if value != "" {
			return value
		}
	}
	return ""
}

func fieldString(row map[string]any, key string) string {
	if row == nil {
		return ""
	}
	value, ok := row[key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprintf("%v", value))
}

func joinNonEmpty(sep string, parts ...string) string {
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		filtered = append(filtered, part)
	}
	return strings.Join(filtered, sep)
}
