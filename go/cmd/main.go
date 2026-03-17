package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/kis-open-api/go/internal/auth"
	"github.com/kis-open-api/go/internal/commodityfuture"
	"github.com/kis-open-api/go/internal/companyanalysis"
	"github.com/kis-open-api/go/internal/dcf"
	"github.com/kis-open-api/go/internal/domesticfutureoption"
	"github.com/kis-open-api/go/internal/domesticstock"
	"github.com/kis-open-api/go/internal/overseasstock"
	"github.com/kis-open-api/go/internal/quadwitching"
	"github.com/kis-open-api/go/internal/shippingindex"
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
	kospiProxyTargetCoverage := getFloatOrDefault("KOSPI_PROXY_PBR_TARGET_COVERAGE", 0.80)
	ewySymbol := getOrDefault("EWY_SYMBOL", "EWY")
	exchangeRateMarketDivCode := getOrDefault("EXCHANGE_RATE_MARKET_DIV_CODE", "X")
	exchangeRateSymbol := getOrDefault("EXCHANGE_RATE_SYMBOL", "USDKRW")
	copperFutureTicker := getOrDefault("COPPER_FUTURE_TICKER", "HG=F")
	copperFutureProductCode := commodityfuture.NormalizeProductCode(getOrDefault("COPPER_FUTURE_PRODUCT_CODE", copperFutureTicker))
	copperFutureCMEProductID := strings.TrimSpace(os.Getenv("COPPER_FUTURE_CME_PRODUCT_ID"))
	goldFutureTicker := getOrDefault("GOLD_FUTURE_TICKER", "GC=F")
	goldFutureProductCode := commodityfuture.NormalizeProductCode(getOrDefault("GOLD_FUTURE_PRODUCT_CODE", goldFutureTicker))
	goldFutureCMEProductID := strings.TrimSpace(os.Getenv("GOLD_FUTURE_CME_PRODUCT_ID"))
	silverFutureTicker := getOrDefault("SILVER_FUTURE_TICKER", "SI=F")
	silverFutureProductCode := commodityfuture.NormalizeProductCode(getOrDefault("SILVER_FUTURE_PRODUCT_CODE", silverFutureTicker))
	silverFutureCMEProductID := strings.TrimSpace(os.Getenv("SILVER_FUTURE_CME_PRODUCT_ID"))
	commodityFutureProviderOrder := splitCSV(getOrDefault("COMMODITY_FUTURE_PROVIDER_ORDER", commodityfuture.DefaultProviderOrderCSV))
	commodityFutureUserAgent := strings.TrimSpace(getOrDefault("COMMODITY_FUTURE_USER_AGENT", commodityfuture.DefaultUserAgent))
	shippingIndexSymbols := splitCSV(getOrDefault("SHIPPING_INDEX_SYMBOLS", strings.Join(shippingindex.DefaultSymbols(), ",")))
	companyAnalysisSymbol := strings.ToUpper(strings.TrimSpace(getOrDefault("COMPANY_ANALYSIS_SYMBOL", "NVDA")))
	companyAnalysisBenchmarkSymbol := strings.ToUpper(strings.TrimSpace(getOrDefault("COMPANY_ANALYSIS_BENCHMARK_SYMBOL", "SPY")))
	companyAnalysisForecastYears := getIntOrDefault("COMPANY_ANALYSIS_FORECAST_YEARS", 5)
	companyAnalysisTerminalGrowth := getFloatOrDefault("COMPANY_ANALYSIS_TERMINAL_GROWTH", 0.025)
	companyAnalysisBetaLookbackDays := getIntOrDefault("COMPANY_ANALYSIS_BETA_LOOKBACK_DAYS", 252)
	companyAnalysisJSONFile := getOrDefault("COMPANY_ANALYSIS_JSON_FILE", ".cache/company_analysis.json")
	companyAnalysisSECTickersCacheFile := getOrDefault("COMPANY_ANALYSIS_SEC_TICKERS_CACHE_FILE", ".cache/sec_company_tickers.json")
	companyAnalysisSECCompanyFactsCacheFile := getOrDefault("COMPANY_ANALYSIS_SEC_COMPANYFACTS_CACHE_FILE", ".cache/sec_companyfacts.json")
	companyAnalysisSECUserAgent := strings.TrimSpace(os.Getenv("COMPANY_ANALYSIS_SEC_USER_AGENT"))
	if companyAnalysisSECUserAgent == "" {
		companyAnalysisSECContactEmail := strings.TrimSpace(os.Getenv("COMPANY_ANALYSIS_SEC_CONTACT_EMAIL"))
		if companyAnalysisSECContactEmail != "" {
			companyAnalysisSECUserAgent = "open-trading-api/1.0 " + companyAnalysisSECContactEmail
		}
	}
	if companyAnalysisSECUserAgent == "" {
		companyAnalysisSECUserAgent = "open-trading-api/1.0 contact@example.com"
	}
	quadWitchingLookaheadDays := getIntOrDefault("QUAD_WITCHING_LOOKAHEAD_DAYS", 7)
	quadWitchingGraceDays := getIntOrDefault("QUAD_WITCHING_GRACE_DAYS", 0)
	quadWitchingForce := getBoolOrDefault("QUAD_WITCHING_FORCE", false)

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
	futureSvc := domesticfutureoption.NewService(client)
	commodityFutureSvc := commodityfuture.NewService(client.Client, commodityfuture.Config{
		ProviderOrder: commodityFutureProviderOrder,
		UserAgent:     commodityFutureUserAgent,
	})
	overseasStockSvc := overseasstock.NewService(client)
	shippingIndexSvc := shippingindex.NewService(client)
	companyAnalysisSvc := companyanalysis.NewService(client.Client, companyanalysis.Config{
		SECUserAgent:             companyAnalysisSECUserAgent,
		SECTickersCachePath:      companyAnalysisSECTickersCacheFile,
		SECCompanyFactsCachePath: companyAnalysisSECCompanyFactsCacheFile,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pbrCtx, pbrCancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer pbrCancel()

	today := time.Now().Format("20060102")
	fromDate := time.Now().AddDate(0, -6, 0).Format("20060102")
	exchangeRateFromDate := time.Now().AddDate(0, 0, -14).Format("20060102")

	respMarketTime, err := svc.MarketTime(ctx)
	mustAPIResult("market-time", respMarketTime, err, "output1")
	printMarketTimeSummary(respMarketTime)
	businessDate := resolveBusinessDateFromMarketTime(respMarketTime, today)
	quadRunWindow, err := quadwitching.EvaluateRunWindow(businessDate, quadWitchingLookaheadDays, quadWitchingGraceDays)
	if err != nil {
		log.Printf("quad witching schedule error: %v", err)
		quadRunWindow = quadwitching.RunWindow{
			BusinessDate:  businessDate,
			LookaheadDays: quadWitchingLookaheadDays,
			GraceDays:     quadWitchingGraceDays,
		}
	}
	shouldRunQuadWitching := quadWitchingForce || quadRunWindow.ShouldRun
	printQuadWitchingGateSummary(quadRunWindow, quadWitchingForce, shouldRunQuadWitching)

	respKOSPI, err := svc.InquireIndexPrice(ctx, "0001")
	mustAPIResult("inquire-index-price (KOSPI 0001)", respKOSPI, err, "output")
	printIndexSummary("KOSPI", respKOSPI)

	respKOSDAQ, err := svc.InquireIndexPrice(ctx, "1001")
	mustAPIResult("inquire-index-price (KOSDAQ 1001)", respKOSDAQ, err, "output")
	printIndexSummary("KOSDAQ", respKOSDAQ)

	kospiActualPBR, err := svc.KOSPIActualPBR(pbrCtx, kospiProxyTargetCoverage, businessDate)
	if err != nil {
		log.Fatalf("KOSPI actual PBR error: %v", err)
	}
	printKOSPIActualPBRSummary(kospiActualPBR)

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

	respVI, err := svc.InquireVIStatus(ctx, businessDate)
	mustAPIResult("inquire-vi-status", respVI, err, "output")
	printVISummary(respVI)

	respFunds, err := svc.MarketFunds(ctx, "")
	mustAPIResult("mktfunds", respFunds, err, "output")
	printMarketFundsSummary(respFunds)

	if shouldRunQuadWitching {
		quadSnapshot := quadwitching.SnapshotExport{
			GeneratedAt:    time.Now().Format(time.RFC3339),
			BusinessDate:   businessDate,
			EndpointStates: map[string]quadwitching.EndpointSnapshot{},
			Notes: []string{
				"futures inquire-time-fuopccnl and inquire-member are experimental because no verified local sample existed in the repository",
			},
		}
		quadSnapshot.EndpointStates["market_time"] = quadwitching.NewEndpointSnapshot(respMarketTime, nil)
		quadSnapshot.EndpointStates["kospi_index_price"] = quadwitching.NewEndpointSnapshot(respKOSPI, nil)
		quadSnapshot.EndpointStates["program_trade_today"] = quadwitching.NewEndpointSnapshot(respProgramTrade, nil)

		quadWitchingFutures, err := futureSvc.ResolveNearMonthKOSPI200Futures(ctx, businessDate)
		if err != nil {
			log.Printf("quad witching futures code resolve error: %v", err)
			quadSnapshot.Notes = append(quadSnapshot.Notes, "futures resolver error: "+err.Error())
		} else {
			quadSnapshot.FuturesCode = quadWitchingFutures.Record.ShortCode
			quadSnapshot.FuturesName = quadWitchingFutures.Record.Name
			quadSnapshot.MasterCache = quadWitchingFutures.MasterCachePath
			quadSnapshot.MasterJSON = quadWitchingFutures.MasterJSONPath
			printQuadWitchingContractSummary(quadWitchingFutures)

			futuresCode := quadWitchingFutures.Record.ShortCode
			futuresMarketDivCode := getOrDefault("QUAD_WITCHING_FUTURES_MARKET_DIV_CODE", "F")

			respFuturePrice, err := futureSvc.InquirePrice(ctx, futuresMarketDivCode, futuresCode)
			quadSnapshot.EndpointStates["future_price"] = quadwitching.NewEndpointSnapshot(respFuturePrice, err)
			if optionalAPIResult("domestic-futureoption inquire-price ("+futuresCode+")", respFuturePrice, err, "output1") {
				printFuturePriceSummary(respFuturePrice)
			}

			respFutureBoardTop, err := futureSvc.DisplayBoardTop(ctx, futuresMarketDivCode, futuresCode, "", "", "", "")
			quadSnapshot.EndpointStates["future_board_top"] = quadwitching.NewEndpointSnapshot(respFutureBoardTop, err)
			if optionalAPIResult("domestic-futureoption display-board-top ("+futuresCode+")", respFutureBoardTop, err, "output1") {
				printFutureBoardTopSummary(respFutureBoardTop)
			}

			respFutureBoard, err := futureSvc.DisplayBoardFutures(ctx, futuresMarketDivCode, "", "")
			quadSnapshot.EndpointStates["future_board"] = quadwitching.NewEndpointSnapshot(respFutureBoard, err)
			if optionalAPIResult("domestic-futureoption display-board-futures", respFutureBoard, err, "output") {
				printFutureBoardSummary(respFutureBoard, futuresCode)
			}

			respFutureTimeChart, err := futureSvc.InquireTimeFuopChartPrice(
				ctx,
				futuresMarketDivCode,
				futuresCode,
				getOrDefault("QUAD_WITCHING_FUTURES_HOUR_CLS_CODE", "60"),
				getOrDefault("QUAD_WITCHING_FUTURES_INCLUDE_PAST_DATA", "Y"),
				getOrDefault("QUAD_WITCHING_FUTURES_INCLUDE_FAKE_TICK", "N"),
				businessDate,
				getOrDefault("QUAD_WITCHING_FUTURES_INPUT_HOUR", time.Now().Format("150405")),
			)
			quadSnapshot.EndpointStates["future_time_chart"] = quadwitching.NewEndpointSnapshot(respFutureTimeChart, err)
			if optionalAPIResult("domestic-futureoption inquire-time-fuopchartprice ("+futuresCode+")", respFutureTimeChart, err, "output2") {
				printFutureTimeChartSummary(respFutureTimeChart)
			}

			respFutureExpected, err := futureSvc.ExpPriceTrend(ctx, futuresCode, futuresMarketDivCode)
			quadSnapshot.EndpointStates["future_expected_price"] = quadwitching.NewEndpointSnapshot(respFutureExpected, err)
			if optionalAPIResult("domestic-futureoption exp-price-trend ("+futuresCode+")", respFutureExpected, err, "output2") {
				printFutureExpectedPriceSummary(respFutureExpected)
			}

			respFutureCCNL, err := futureSvc.InquireTimeFuopCCNL(ctx, futuresMarketDivCode, futuresCode)
			quadSnapshot.EndpointStates["future_time_conclusion_experimental"] = quadwitching.NewEndpointSnapshot(respFutureCCNL, err)
			if optionalAPIResult("domestic-futureoption inquire-time-fuopccnl ("+futuresCode+")", respFutureCCNL, err, "output") {
				printFutureExecutionSummary(respFutureCCNL)
			}

			respFutureMember, err := futureSvc.InquireMember(ctx, futuresMarketDivCode, futuresCode)
			quadSnapshot.EndpointStates["future_member_experimental"] = quadwitching.NewEndpointSnapshot(respFutureMember, err)
			if optionalAPIResult("domestic-futureoption inquire-member ("+futuresCode+")", respFutureMember, err, "output") {
				printFutureMemberSummary(respFutureMember)
			}
		}

		respQuadInvestor, err := svc.InquireInvestor(ctx, "J", getOrDefault("QUAD_WITCHING_INVESTOR_SYMBOL", "0001"))
		quadSnapshot.EndpointStates["kospi_investor"] = quadwitching.NewEndpointSnapshot(respQuadInvestor, err)
		if optionalAPIResult("domestic-stock inquire-investor", respQuadInvestor, err, "output") {
			printInvestorTrendSummary("Quad Witching KOSPI Investor Trend", respQuadInvestor)
		}

		respKOSPI200Investor, err := svc.InquireInvestor(ctx, "J", getOrDefault("QUAD_WITCHING_KOSPI200_INVESTOR_SYMBOL", "2001"))
		quadSnapshot.EndpointStates["kospi200_investor"] = quadwitching.NewEndpointSnapshot(respKOSPI200Investor, err)
		if optionalAPIResult("domestic-stock inquire-investor (KOSPI200)", respKOSPI200Investor, err, "output") {
			printInvestorTrendSummary("Quad Witching KOSPI200 Investor Trend", respKOSPI200Investor)
		}

		respForeignTotal, err := svc.ForeignInstitutionTotal(
			ctx,
			getOrDefault("QUAD_WITCHING_FOREIGN_MARKET_DIV_CODE", "V"),
			getOrDefault("QUAD_WITCHING_FOREIGN_SCREEN_DIV_CODE", "16449"),
			getOrDefault("QUAD_WITCHING_FOREIGN_INPUT_ISCD", "0000"),
			getOrDefault("QUAD_WITCHING_FOREIGN_DIV_CLS_CODE", "1"),
			getOrDefault("QUAD_WITCHING_FOREIGN_RANK_SORT_CODE", "0"),
			getOrDefault("QUAD_WITCHING_FOREIGN_ETC_CLS_CODE", "1"),
		)
		quadSnapshot.EndpointStates["foreign_institution_total"] = quadwitching.NewEndpointSnapshot(respForeignTotal, err)
		if optionalAPIResult("domestic-stock foreign-institution-total", respForeignTotal, err, "output") {
			printForeignInstitutionSummary(respForeignTotal)
		}

		respAskingPrice, err := svc.InquireAskingPriceExpCCN(ctx, "J", getOrDefault("QUAD_WITCHING_AUCTION_SYMBOL", targetSymbol))
		quadSnapshot.EndpointStates["asking_price_expected"] = quadwitching.NewEndpointSnapshot(respAskingPrice, err)
		if optionalAPIResult("domestic-stock inquire-asking-price-exp-ccn", respAskingPrice, err, "output1") {
			printAskingPriceExpSummary(getOrDefault("QUAD_WITCHING_AUCTION_SYMBOL", targetSymbol), respAskingPrice)
		}

		quadSnapshotPath := resolveQuadWitchingSnapshotPath(
			getOrDefault("QUAD_WITCHING_SNAPSHOT_JSON_FILE", ".cache/quad_witching_snapshot.json"),
			businessDate,
			quadSnapshot.FuturesCode,
		)
		if err := quadwitching.WriteSnapshot(quadSnapshotPath, quadSnapshot); err != nil {
			log.Printf("quad witching snapshot export error: %v", err)
		} else {
			printQuadWitchingSnapshotSummary(quadSnapshotPath, quadSnapshot)
		}
	} else {
		log.Printf("skipping quad witching stats: outside configured window for %s", quadRunWindow.QuadDate)
	}

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

	respExchangeRate, err := overseasStockSvc.InquireDailyChartPrice(
		ctx,
		exchangeRateMarketDivCode,
		exchangeRateSymbol,
		exchangeRateFromDate,
		today,
		"D",
	)
	mustAPIResult(
		"exchange-rate daily-chart ("+exchangeRateMarketDivCode+" "+exchangeRateSymbol+")",
		respExchangeRate,
		err,
		"output1",
	)
	printExchangeRateSummary(exchangeRateMarketDivCode, exchangeRateSymbol, respExchangeRate)

	printConfiguredCommodityFutureSummary(
		ctx,
		commodityFutureSvc,
		configuredCommodityInstrument("Copper", copperFutureTicker, copperFutureProductCode, copperFutureCMEProductID),
	)
	printConfiguredCommodityFutureSummary(
		ctx,
		commodityFutureSvc,
		configuredCommodityInstrument("Gold", goldFutureTicker, goldFutureProductCode, goldFutureCMEProductID),
	)
	printConfiguredCommodityFutureSummary(
		ctx,
		commodityFutureSvc,
		configuredCommodityInstrument("Silver", silverFutureTicker, silverFutureProductCode, silverFutureCMEProductID),
	)

	shippingQuotes, err := shippingIndexSvc.Quotes(ctx, shippingIndexSymbols)
	if err != nil {
		log.Printf("shipping index error: %v", err)
	}
	printShippingIndexSummary(shippingQuotes)

	companyAnalysisResult, err := companyAnalysisSvc.Analyze(ctx, companyAnalysisSymbol, companyanalysis.AnalysisOptions{
		BenchmarkSymbol:  companyAnalysisBenchmarkSymbol,
		ForecastYears:    companyAnalysisForecastYears,
		TerminalGrowth:   companyAnalysisTerminalGrowth,
		BetaLookbackDays: companyAnalysisBetaLookbackDays,
		RiskFreeRate:     getOptionalFloat("COMPANY_ANALYSIS_RISK_FREE_RATE"),
		Beta:             getOptionalFloat("COMPANY_ANALYSIS_BETA"),
		MarketPremium:    getOptionalFloat("COMPANY_ANALYSIS_MARKET_PREMIUM"),
		CostOfDebt:       getOptionalFloat("COMPANY_ANALYSIS_COST_OF_DEBT"),
		NetDebt:          getOptionalFloat("COMPANY_ANALYSIS_NET_DEBT"),
	})
	if err != nil {
		log.Printf("company analysis error: %v", err)
	} else {
		printCompanyAnalysisSummary(companyAnalysisResult)

		var companyReverseDCF *dcf.ReverseDCFResult
		if companyAnalysisResult.Quote.Price > 0 {
			companyReverseDCF, err = dcf.ReverseDCF(
				companyAnalysisResult.Financial,
				companyAnalysisResult.Market,
				companyAnalysisResult.Assumptions,
				companyAnalysisResult.Projection,
				companyAnalysisResult.Quote.Price,
				dcf.ReverseDCFConfig{},
			)
			if err != nil {
				log.Printf("company reverse DCF error: %v", err)
			} else {
				printCompanyReverseDCFSummary(companyAnalysisResult, companyReverseDCF)
			}
		}

		companyMonteCarloConfig := dcf.MonteCarloConfig{
			Iterations:           getIntOrDefault("DCF_MONTE_CARLO_ITERATIONS", 2000),
			Workers:              getIntOrDefault("DCF_MONTE_CARLO_WORKERS", 0),
			RevenueGrowthStdDev:  getFloatOrDefault("DCF_MONTE_CARLO_GROWTH_STDDEV", 0.02),
			WACCStdDev:           getFloatOrDefault("DCF_MONTE_CARLO_WACC_STDDEV", 0.01),
			TerminalGrowthStdDev: getFloatOrDefault("DCF_MONTE_CARLO_TERMINAL_STDDEV", 0.005),
		}
		companyMonteCarlo, companyMonteCarloErr := dcf.MonteCarlo(
			companyAnalysisResult.Financial,
			companyAnalysisResult.Market,
			companyAnalysisResult.Assumptions,
			companyAnalysisResult.Projection,
			companyMonteCarloConfig,
		)
		companyAnalysisJSONPath := resolveCompanyAnalysisJSONPath(companyAnalysisJSONFile, businessDate, companyAnalysisSymbol)
		if companyMonteCarloErr != nil {
			log.Printf("company monte carlo DCF error: %v", companyMonteCarloErr)
		} else {
			printCompanyMonteCarloSummary(companyMonteCarlo, companyAnalysisJSONPath)
		}
		if err := companyanalysis.WriteExport(companyAnalysisJSONPath, companyanalysis.Export{
			GeneratedAt:   time.Now().Format(time.RFC3339),
			BusinessDate:  businessDate,
			Symbol:        companyAnalysisSymbol,
			Result:        companyAnalysisResult,
			ReverseDCF:    companyReverseDCF,
			MonteCarloCfg: companyMonteCarloConfig,
			MonteCarlo:    companyMonteCarlo,
		}); err != nil {
			log.Printf("company analysis JSON export error: %v", err)
		}
	}

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

	dcfReadiness, err := svc.DCFReadiness(ctx, targetSymbol, "0")
	if err != nil {
		log.Printf("DCF readiness error: %v", err)
	} else {
		printDCFReadinessSummary(dcfReadiness)
	}

	dcfValuation, err := svc.DCFValuation(ctx, targetSymbol, domesticstock.DCFValuationOptions{})
	if err != nil {
		log.Printf("DCF valuation error: %v", err)
	} else {
		printDCFValuationSummary(dcfValuation)
	}

	var reverseDCFResult *dcf.ReverseDCFResult
	if dcfValuation != nil {
		if dcfValuation.CurrentPrice <= 0 {
			log.Printf("reverse DCF error: current price missing")
		} else {
			reverseDCFResult, err = dcf.ReverseDCF(
				dcfValuation.Financial,
				dcfValuation.Market,
				dcfValuation.Assumptions,
				dcfValuation.Projection,
				dcfValuation.CurrentPrice,
				dcf.ReverseDCFConfig{},
			)
			if err != nil {
				log.Printf("reverse DCF error: %v", err)
			} else {
				printReverseDCFSummary(reverseDCFResult)
			}
		}
	}

	monteCarloConfig := dcf.MonteCarloConfig{
		Iterations:           getIntOrDefault("DCF_MONTE_CARLO_ITERATIONS", 2000),
		Workers:              getIntOrDefault("DCF_MONTE_CARLO_WORKERS", 0),
		RevenueGrowthStdDev:  getFloatOrDefault("DCF_MONTE_CARLO_GROWTH_STDDEV", 0.02),
		WACCStdDev:           getFloatOrDefault("DCF_MONTE_CARLO_WACC_STDDEV", 0.01),
		TerminalGrowthStdDev: getFloatOrDefault("DCF_MONTE_CARLO_TERMINAL_STDDEV", 0.005),
	}
	if dcfValuation != nil {
		monteCarloResult, monteCarloErr := dcf.MonteCarlo(
			dcfValuation.Financial,
			dcfValuation.Market,
			dcfValuation.Assumptions,
			dcfValuation.Projection,
			monteCarloConfig,
		)
		if monteCarloErr != nil {
			log.Printf("monte carlo DCF error: %v", monteCarloErr)
		} else {
			monteCarloJSONPath := resolveMonteCarloJSONPath(getOrDefault("DCF_MONTE_CARLO_JSON_FILE", ".cache/dcf_monte_carlo.json"), businessDate, targetSymbol)
			if err := dcf.WriteMonteCarloExport(monteCarloJSONPath, dcf.MonteCarloExport{
				GeneratedAt:   time.Now().Format(time.RFC3339),
				BusinessDate:  businessDate,
				Symbol:        targetSymbol,
				CurrentPrice:  dcfValuation.CurrentPrice,
				Financial:     dcfValuation.Financial,
				Market:        dcfValuation.Market,
				Assumptions:   dcfValuation.Assumptions,
				Projection:    dcfValuation.Projection,
				Valuation:     dcfValuation.Valuation,
				ReverseDCF:    reverseDCFResult,
				MonteCarloCfg: monteCarloConfig,
				MonteCarlo:    monteCarloResult,
			}); err != nil {
				log.Printf("monte carlo JSON export error: %v", err)
			}
			printMonteCarloSummary(monteCarloResult, monteCarloJSONPath)
		}
	}

	printClientMetricsSummary(client.MetricsSnapshot())
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
		{"/uapi/domestic-stock/v1/quotations/inquire-investor", "종목/지수 투자자 수급"},
		{"/uapi/domestic-stock/v1/quotations/frgnmem-trade-estimate", "외국계 매매 가집계"},
		{"/uapi/domestic-stock/v1/quotations/investor-trend-estimate", "종목 외인/기관 추정 집계"},
		{"/uapi/domestic-stock/v1/quotations/inquire-vi-status", "VI 발동 현황"},
		{"/uapi/domestic-stock/v1/quotations/mktfunds", "증시자금 종합"},
		{"/uapi/domestic-stock/v1/quotations/inquire-asking-price-exp-ccn", "동시호가 예상체결/호가"},
		{"/uapi/domestic-futureoption/v1/quotations/inquire-price", "국내선물 현재가/베이시스"},
		{"/uapi/domestic-futureoption/v1/quotations/inquire-time-fuopchartprice", "국내선물 분봉/체결 추세"},
		{"/uapi/domestic-futureoption/v1/quotations/display-board-top", "국내선물 기초자산/잔존일"},
		{"/uapi/domestic-futureoption/v1/quotations/display-board-futures", "국내선물 전광판"},
		{"/uapi/domestic-futureoption/v1/quotations/exp-price-trend", "국내선물 예상체결 추이"},
		{"/uapi/domestic-futureoption/v1/quotations/inquire-time-fuopccnl", "국내선물 체결 추이(실험적 wrapper)"},
		{"/uapi/domestic-futureoption/v1/quotations/inquire-member", "국내선물 투자자별 매매동향(실험적 wrapper)"},
		{"/uapi/domestic-stock/v1/quotations/inquire-daily-itemchartprice", "RSI/기술지표 원천(OHLCV)"},
		{"/uapi/domestic-stock/v1/finance/balance-sheet", "DCF용 자산/부채/자본 구조"},
		{"/uapi/domestic-stock/v1/finance/income-statement", "DCF용 매출/영업이익/감가상각"},
		{"/uapi/domestic-stock/v1/finance/other-major-ratios", "DCF용 EBITDA/EV-EBITDA proxy"},
		{"/uapi/domestic-stock/v1/finance/stability-ratio", "DCF용 차입금의존도 proxy"},
		{"/uapi/domestic-stock/v1/quotations/comp-interest", "DCF용 무위험금리(국내채권 금리)"},
		{"/uapi/domestic-bond/v1/quotations/inquire-price", "DCF용 무위험금리(국내채권 코드 직접 조회)"},
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
	if resp.ParseError != "" {
		fmt.Printf("status_code=%d content_type=%s body_bytes=%d\n", resp.StatusCode, resp.Headers.Get("Content-Type"), len(resp.RawBody))
		if resp.RequestMethod != "" || resp.RequestURL != "" {
			fmt.Printf("request=%s %s\n", resp.RequestMethod, resp.RequestURL)
		}
		fmt.Printf("parse_error=%s\n", resp.ParseError)
		if len(resp.RawBody) > 0 {
			fmt.Printf("raw_body=%s\n", rawPreview(resp.RawBody))
		}
		if resp.ParseError == "empty response body" {
			fmt.Printf("note=server returned no JSON body; this endpoint may require additional parameters or may not be exposed in the current environment\n")
		}
		return
	}
	if outputKey == "" {
		if len(resp.RawBody) > 0 && len(resp.Body) == 0 {
			fmt.Printf("raw_body=%s\n", rawPreview(resp.RawBody))
		}
		return
	}
	if value, ok := resp.Body[outputKey]; ok {
		fmt.Printf("output(%s)=%s\n", outputKey, preview(value))
		return
	}
	if len(resp.RawBody) > 0 {
		fmt.Printf("raw_body=%s\n", rawPreview(resp.RawBody))
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

func optionalAPIResult(name string, resp *auth.RESTResponse, err error, outputKey string) bool {
	if err != nil {
		if isEmptyBodyResponse(resp, err) {
			log.Printf("%s returned an empty response body", name)
			if resp != nil {
				printAPIResult(name, resp, outputKey)
			}
			return false
		}
		log.Printf("%s error: %v", name, err)
		if resp != nil {
			printAPIResult(name, resp, outputKey)
		}
		return false
	}
	if resp == nil {
		log.Printf("%s error: response is nil", name)
		return false
	}
	if !resp.IsOK() {
		log.Printf("%s error: msg_cd=%s msg1=%s", name, resp.MessageCode(), resp.Message())
		return false
	}

	printAPIResult(name, resp, outputKey)
	return true
}

func isEmptyBodyResponse(resp *auth.RESTResponse, err error) bool {
	if resp == nil {
		return false
	}

	if resp.ParseError == "empty response body" {
		return true
	}

	if err == nil {
		return false
	}

	return strings.Contains(err.Error(), "empty response body")
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

func rawPreview(raw []byte) string {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return "<empty>"
	}
	const limit = 400
	if len(trimmed) <= limit {
		return trimmed
	}
	return trimmed[:limit] + "...(truncated)"
}

func getOrDefault(key string, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func getFloatOrDefault(key string, defaultValue float64) float64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return defaultValue
	}

	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		log.Fatalf("%s must be a float: %v", key, err)
	}
	return parsed
}

func getIntOrDefault(key string, defaultValue int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return defaultValue
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		log.Fatalf("%s must be an integer: %v", key, err)
	}
	return parsed
}

func getBoolOrDefault(key string, defaultValue bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return defaultValue
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		log.Fatalf("%s must be a boolean: %v", key, err)
	}
	return parsed
}

func getOptionalFloat(key string) *float64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return nil
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		log.Fatalf("%s must be a float: %v", key, err)
	}
	return &parsed
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

func printQuadWitchingGateSummary(window quadwitching.RunWindow, forceRun bool, shouldRun bool) {
	status := "skip"
	if shouldRun {
		status = "run"
	}

	reason := "outside run window"
	if forceRun {
		reason = "forced by QUAD_WITCHING_FORCE"
	} else if window.ShouldRun {
		reason = "inside quad witching window"
	}

	printSummaryBlock("Quad Witching Gate", []string{
		formatSummaryLine("Business Date", window.BusinessDate),
		formatSummaryLine("Target Date", window.QuadDate),
		formatSummaryLine("Window", joinNonEmpty(" ~ ", window.WindowStart, window.WindowEnd)),
		formatSummaryLine("Lookahead / Grace", fmt.Sprintf("%d / %d day(s)", window.LookaheadDays, window.GraceDays)),
		formatSummaryLine("Days Until Target", strconv.Itoa(window.DaysUntil)),
		formatSummaryLine("Status", status),
		formatSummaryLine("Reason", reason),
	})
}

func resolveBusinessDateFromMarketTime(resp *auth.RESTResponse, fallback string) string {
	if strings.TrimSpace(fallback) == "" {
		fallback = time.Now().Format("20060102")
	}

	row := firstRow(resp, "output1")
	if row == nil {
		return fallback
	}

	today := normalizeYMD(fieldString(row, "today"))
	dates := []string{
		normalizeYMD(fieldString(row, "date1")),
		normalizeYMD(fieldString(row, "date2")),
		normalizeYMD(fieldString(row, "date3")),
		normalizeYMD(fieldString(row, "date4")),
		normalizeYMD(fieldString(row, "date5")),
	}

	if today != "" {
		for _, date := range dates {
			if date == today {
				return today
			}
		}

		latest := ""
		for _, date := range dates {
			if date == "" {
				continue
			}
			if date <= today && date > latest {
				latest = date
			}
		}
		if latest != "" {
			return latest
		}

		return today
	}

	latest := ""
	for _, date := range dates {
		if date > latest {
			latest = date
		}
	}
	if latest != "" {
		return latest
	}

	return fallback
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

func printKOSPIActualPBRSummary(result *domesticstock.ActualPBRResult) {
	if result == nil {
		return
	}

	lines := []string{
		formatSummaryLine("Method", result.Method),
		formatSummaryLine("Weighted PBR", formatFloat(result.WeightedPBR)),
		formatSummaryLine("Target Coverage", formatPercent(result.TargetCoverage)),
		formatSummaryLine("Used Coverage", formatPercent(result.UsedCoverage)),
		formatSummaryLine("Raw Coverage", formatPercent(result.RawCoverage)),
		formatSummaryLine("Selected / Used / Skipped", fmt.Sprintf("%d / %d / %d", result.SelectedCount, result.UsedCount, result.SkippedCount)),
		formatSummaryLine("Cache Hit / API Call", fmt.Sprintf("%d / %d", result.CacheHitCount, result.APICallCount)),
		formatSummaryLine("Market Cap Used", formatFloat(result.UsedMarketCap)),
		formatSummaryLine("Book Value Used", formatFloat(result.AggregateBookValue)),
		formatSummaryLine("Business Date", result.BusinessDate),
		formatSummaryLine("Actual PBR Cache", result.ActualPBRCachePath),
		formatSummaryLine("Master Cache", result.MasterCachePath),
		formatSummaryLine("Master JSON", result.MasterJSONPath),
		formatSummaryLine("Master Load", result.MasterLoadTime.String()),
		formatSummaryLine("Cache Load", result.CacheLoadTime.String()),
		formatSummaryLine("Rate Limit Wait", result.RateLimitWaitTime.String()),
		formatSummaryLine("Price Fetch", result.PriceFetchTime.String()),
		formatSummaryLine("Cache Save", result.CacheSaveTime.String()),
		formatSummaryLine("Total Time", result.TotalDuration.String()),
	}

	for i, constituent := range result.Constituents {
		if i >= 5 {
			break
		}
		lines = append(lines, formatSummaryLine(
			fmt.Sprintf("Top %d", i+1),
			fmt.Sprintf("%s %s | cap=%s | pbr=%s | weight=%s | %s",
				constituent.Code,
				constituent.Name,
				formatFloat(constituent.MarketCap),
				formatFloat(constituent.PBR),
				formatPercent(constituent.Coverage),
				cacheLabel(constituent.CacheHit),
			),
		))
	}

	printSummaryBlock("KOSPI Actual PBR Summary", lines)
}

func printDCFReadinessSummary(result *domesticstock.DCFReadinessResult) {
	if result == nil {
		return
	}

	lines := []string{
		formatSummaryLine("Symbol", result.Symbol),
		formatSummaryLine("Division", result.Division),
		formatSummaryLine("Balance / Income Periods", fmt.Sprintf("%d / %d", result.BalancePeriods, result.IncomePeriods)),
		formatSummaryLine("Can Project FCF", formatBool(result.CanProjectFCF)),
		formatSummaryLine("Can Compute WACC", formatBool(result.CanComputeWACC)),
		formatSummaryLine("Can Compute EV", formatBool(result.CanComputeEnterpriseValue)),
		formatSummaryLine("Can Compute Target Price", formatBool(result.CanComputeTargetPrice)),
		formatSummaryLine("Missing For FCF", strings.Join(result.MissingForFCF, ", ")),
		formatSummaryLine("Missing For WACC", strings.Join(result.MissingForWACC, ", ")),
		formatSummaryLine("Missing For Target Price", strings.Join(result.MissingForTargetPrice, ", ")),
	}

	for _, input := range result.Inputs {
		value := string(input.Status)
		if input.HasValue {
			value = DCFInputStatusText(input)
		}
		if input.Note != "" {
			value += " | " + input.Note
		}
		lines = append(lines, formatSummaryLine(input.Name, value))
	}

	printSummaryBlock("DCF Readiness Summary", lines)
}

func printDCFValuationSummary(result *domesticstock.DCFValuationResult) {
	if result == nil || result.Valuation == nil {
		return
	}

	lines := []string{
		formatSummaryLine("Symbol", result.Symbol),
		formatSummaryLine("Revenue / EBIT", fmt.Sprintf("%s / %s", formatFloat(result.Financial.Revenue), formatFloat(result.Financial.EBIT))),
		formatSummaryLine("Current Price", formatFloat(result.CurrentPrice)),
		formatSummaryLine("Base FCF", formatFloat(result.Valuation.BaseFCF)),
		formatSummaryLine("Risk Free / Beta / MRP", fmt.Sprintf("%s / %s / %s",
			formatPercent(result.Market.RiskFreeRate),
			formatFloat(result.Market.Beta),
			formatPercent(result.Market.MarketPremium),
		)),
		formatSummaryLine("Cost Of Debt", formatPercent(result.Market.CostOfDebt)),
		formatSummaryLine("Equity / Debt Weight", fmt.Sprintf("%s / %s",
			formatPercent(result.Market.EquityWeight),
			formatPercent(result.Market.DebtWeight),
		)),
		formatSummaryLine("Cost Of Equity", formatPercent(result.Valuation.CostOfEquity)),
		formatSummaryLine("WACC", formatPercent(result.Valuation.WACC)),
		formatSummaryLine("Terminal Growth", formatPercent(result.Assumptions.TerminalGrowth)),
		formatSummaryLine("Enterprise Value", formatFloat(result.Valuation.EnterpriseValue)),
		formatSummaryLine("Equity Value", formatFloat(result.Valuation.EquityValue)),
		formatSummaryLine("Net Debt", formatFloat(result.Financial.NetDebt)),
		formatSummaryLine("Shares Out", formatFloat(result.Financial.SharesOut)),
		formatSummaryLine("Target Price Raw", formatFloat(result.Valuation.TargetPriceRaw)),
		formatSummaryLine("Target Price Scale", formatFloat(result.Valuation.TargetPriceScale)),
		formatSummaryLine("Target Price Unit", result.Valuation.TargetPriceUnit),
		formatSummaryLine("Target Price", formatFloat(result.Valuation.TargetPrice)),
		formatSummaryLine("Projection", fmt.Sprintf("growth=%s ebit=%s dna=%s capex=%s nwc=%s",
			formatPercent(result.Projection.RevenueGrowth),
			formatPercent(result.Projection.EBITMargin),
			formatPercent(result.Projection.DNAMargin),
			formatPercent(result.Projection.CapExMargin),
			formatPercent(result.Projection.NWCMargin),
		)),
	}

	for i, year := range result.Valuation.Forecast {
		if i >= 3 {
			break
		}
		lines = append(lines, formatSummaryLine(
			fmt.Sprintf("Year %d", year.Year),
			fmt.Sprintf("rev=%s fcf=%s pv=%s",
				formatFloat(year.Revenue),
				formatFloat(year.FCF),
				formatFloat(year.PresentValue),
			),
		))
	}

	for i, note := range result.Notes {
		if i >= 3 {
			break
		}
		lines = append(lines, formatSummaryLine(fmt.Sprintf("Note %d", i+1), note))
	}

	printSummaryBlock("DCF Valuation Summary", lines)
}

func printReverseDCFSummary(result *dcf.ReverseDCFResult) {
	if result == nil || result.Valuation == nil {
		return
	}

	printSummaryBlock("Reverse DCF Summary", []string{
		formatSummaryLine("Target Price", formatFloat(result.TargetPrice)),
		formatSummaryLine("Implied Revenue Growth", formatPercent(result.ImpliedRevenueGrowth)),
		formatSummaryLine("Iterations", fmt.Sprintf("%d", result.Iterations)),
		formatSummaryLine("Price Error", formatFloat(result.PriceError)),
		formatSummaryLine("Solved WACC", formatPercent(result.Valuation.WACC)),
		formatSummaryLine("Solved EV / Equity", fmt.Sprintf("%s / %s",
			formatFloat(result.Valuation.EnterpriseValue),
			formatFloat(result.Valuation.EquityValue),
		)),
	})
}

func printMonteCarloSummary(result *dcf.MonteCarloResult, jsonPath string) {
	if result == nil {
		return
	}

	printSummaryBlock("Monte Carlo DCF Summary", []string{
		formatSummaryLine("Requested / Valid / Invalid", fmt.Sprintf("%d / %d / %d", result.RequestedIterations, result.ValidIterations, result.InvalidIterations)),
		formatSummaryLine("Mean", formatFloat(result.Mean)),
		formatSummaryLine("P10 / P50 / P90", fmt.Sprintf("%s / %s / %s",
			formatFloat(result.P10),
			formatFloat(result.P50),
			formatFloat(result.P90),
		)),
		formatSummaryLine("Min / Max", fmt.Sprintf("%s / %s",
			formatFloat(result.Min),
			formatFloat(result.Max),
		)),
		formatSummaryLine("JSON Export", jsonPath),
	})
}

func printCompanyAnalysisSummary(result *companyanalysis.Result) {
	if result == nil || result.Valuation == nil {
		log.Printf("company analysis summary: result or valuation is nil: %v", result)
		return
	}

	upside := ""
	if result.Quote.Price > 0 {
		upside = formatPercent((result.Valuation.TargetPrice / result.Quote.Price) - 1)
	}

	lines := []string{
		formatSummaryLine("Company / Symbol", joinNonEmpty(" / ", result.CompanyName, result.Symbol)),
		formatSummaryLine("CIK / Benchmark", joinNonEmpty(" / ", result.CIK, result.BenchmarkSymbol)),
		formatSummaryLine("Quote", joinNonEmpty(" ", formatMoney(result.Quote.Currency, result.Quote.Price), "on", result.Quote.PriceDate)),
		formatSummaryLine("Previous / Change", fmt.Sprintf("%s / %s (%s)",
			formatMoney(result.Quote.Currency, result.Quote.PreviousClose),
			formatSignedMoney(result.Quote.Currency, result.Quote.Change),
			formatPercentPoints(result.Quote.ChangePercent),
		)),
		formatSummaryLine("Revenue / EBIT / Net Income", fmt.Sprintf("%s / %s / %s",
			formatMoney(result.Quote.Currency, result.Financials[0].Revenue),
			formatMoney(result.Quote.Currency, result.Financials[0].EBIT),
			formatMoney(result.Quote.Currency, result.Financials[0].NetIncome),
		)),
		formatSummaryLine("Cash / Debt / Net Debt", fmt.Sprintf("%s / %s / %s",
			formatMoney(result.Quote.Currency, result.Financials[0].Cash),
			formatMoney(result.Quote.Currency, result.Financials[0].TotalDebt),
			formatMoney(result.Quote.Currency, result.KeyMetrics.NetDebt),
		)),
		formatSummaryLine("Market Cap / EV", fmt.Sprintf("%s / %s",
			formatMoney(result.Quote.Currency, result.KeyMetrics.MarketCap),
			formatMoney(result.Quote.Currency, result.KeyMetrics.EnterpriseValue),
		)),
		formatSummaryLine("Growth / Op Margin / ROE", fmt.Sprintf("%s / %s / %s",
			formatPercent(result.KeyMetrics.RevenueGrowth),
			formatPercent(result.KeyMetrics.OperatingMargin),
			formatPercent(result.KeyMetrics.ROE),
		)),
		formatSummaryLine("Current Ratio / Debt To Equity", fmt.Sprintf("%s / %s",
			formatFloat(result.KeyMetrics.CurrentRatio),
			formatFloat(result.KeyMetrics.DebtToEquity),
		)),
		formatSummaryLine("Risk Free / Beta / MRP", fmt.Sprintf("%s / %s / %s",
			formatPercent(result.Market.RiskFreeRate),
			formatFloat(result.Market.Beta),
			formatPercent(result.Market.MarketPremium),
		)),
		formatSummaryLine("Cost Of Debt / WACC", fmt.Sprintf("%s / %s",
			formatPercent(result.Market.CostOfDebt),
			formatPercent(result.Valuation.WACC),
		)),
		formatSummaryLine("Target Price", joinNonEmpty(" ", formatMoney(result.Quote.Currency, result.Valuation.TargetPrice), "("+result.Valuation.TargetPriceUnit+")")),
		formatSummaryLine("Upside / Downside", upside),
		formatSummaryLine("Projection", fmt.Sprintf("growth=%s ebit=%s dna=%s capex=%s nwc=%s",
			formatPercent(result.Projection.RevenueGrowth),
			formatPercent(result.Projection.EBITMargin),
			formatPercent(result.Projection.DNAMargin),
			formatPercent(result.Projection.CapExMargin),
			formatPercent(result.Projection.NWCMargin),
		)),
	}

	for i, record := range result.Financials {
		if i >= 3 {
			break
		}
		lines = append(lines, formatSummaryLine(
			fmt.Sprintf("FY %d", record.FiscalYear),
			fmt.Sprintf("rev=%s ebit=%s cash=%s debt=%s",
				formatMoney(result.Quote.Currency, record.Revenue),
				formatMoney(result.Quote.Currency, record.EBIT),
				formatMoney(result.Quote.Currency, record.Cash),
				formatMoney(result.Quote.Currency, record.TotalDebt),
			),
		))
	}

	for i, note := range result.Notes {
		if i >= 3 {
			break
		}
		lines = append(lines, formatSummaryLine(fmt.Sprintf("Note %d", i+1), note))
	}

	printSummaryBlock("Company Analysis Summary", lines)
}

func printCompanyReverseDCFSummary(result *companyanalysis.Result, reverse *dcf.ReverseDCFResult) {
	if result == nil || reverse == nil || reverse.Valuation == nil {
		return
	}

	printSummaryBlock("Company Reverse DCF Summary", []string{
		formatSummaryLine("Market Price", formatMoney(result.Quote.Currency, result.Quote.Price)),
		formatSummaryLine("Implied Revenue Growth", formatPercent(reverse.ImpliedRevenueGrowth)),
		formatSummaryLine("Iterations", fmt.Sprintf("%d", reverse.Iterations)),
		formatSummaryLine("Price Error", formatMoney(result.Quote.Currency, reverse.PriceError)),
		formatSummaryLine("Solved WACC", formatPercent(reverse.Valuation.WACC)),
	})
}

func printCompanyMonteCarloSummary(result *dcf.MonteCarloResult, jsonPath string) {
	if result == nil {
		return
	}

	printSummaryBlock("Company Monte Carlo DCF Summary", []string{
		formatSummaryLine("Requested / Valid / Invalid", fmt.Sprintf("%d / %d / %d", result.RequestedIterations, result.ValidIterations, result.InvalidIterations)),
		formatSummaryLine("Mean", formatFloat(result.Mean)),
		formatSummaryLine("P10 / P50 / P90", fmt.Sprintf("%s / %s / %s",
			formatFloat(result.P10),
			formatFloat(result.P50),
			formatFloat(result.P90),
		)),
		formatSummaryLine("Min / Max", fmt.Sprintf("%s / %s", formatFloat(result.Min), formatFloat(result.Max))),
		formatSummaryLine("JSON Export", jsonPath),
	})
}

func printProgramTradeSummary(resp *auth.RESTResponse) {
	row := firstRow(resp, "output")
	if row == nil {
		return
	}

	printSummaryBlock("Program Trade Summary", []string{
		formatSummaryLine("Date", firstNonEmpty(row, "stck_bsop_date")),
		formatSummaryLine("Market Close", humanNumber(firstNonEmpty(row, "stck_clpr"))),
		formatSummaryLine("Program Net Position", netFlowText(firstNonEmpty(row, "whol_smtn_ntby_qty"), "")),
		formatSummaryLine("Program Net Amount", netFlowText(firstNonEmpty(row, "whol_smtn_ntby_tr_pbmn"), "")),
		formatSummaryLine("Arbitrage Net Amount", netFlowText(firstNonEmpty(row, "prsm_nslg_pbmn"), "")),
		formatSummaryLine("Non-Arbitrage Net Amount", netFlowText(firstNonEmpty(row, "nprsm_nslg_pbmn"), "")),
		formatSummaryLine("Buy / Sell Volume", joinNonEmpty(" / ",
			humanNumber(firstNonEmpty(row, "whol_smtn_shnu_vol")),
			humanNumber(firstNonEmpty(row, "whol_smtn_seln_vol")),
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

func printQuadWitchingContractSummary(resolved *domesticfutureoption.ResolvedContract) {
	if resolved == nil {
		return
	}

	printSummaryBlock("Quad Witching Contract", []string{
		formatSummaryLine("Business Date", resolved.BusinessDate),
		formatSummaryLine("Source", resolved.Source),
		formatSummaryLine("Contract", joinNonEmpty(" ", resolved.Record.ShortCode, resolved.Record.Name)),
		formatSummaryLine("Month Class", resolved.Record.MonthClassCode),
		formatSummaryLine("Underlying", joinNonEmpty(" / ", resolved.Record.UnderlyingShortCode, resolved.Record.UnderlyingName)),
		formatSummaryLine("Master Cache", resolved.MasterCachePath),
		formatSummaryLine("Master JSON", resolved.MasterJSONPath),
	})
}

func printFuturePriceSummary(resp *auth.RESTResponse) {
	row := firstRowAny(resp, "output1", "output", "output2", "output3")
	if row == nil {
		return
	}

	printSummaryBlock("Quad Witching Futures Price", []string{
		formatSummaryLine("Contract", firstNonEmpty(row, "hts_kor_isnm")),
		formatSummaryLine("Futures Price", humanNumber(firstNonEmpty(row, "futs_prpr"))),
		formatSummaryLine("Spot Index", humanNumber(firstNonEmpty(row, "bstp_nmix_prpr"))),
		formatSummaryLine("Basis (Futures - Spot)", formatOptionalHumanNumber(computeBasis(row))),
		formatSummaryLine("Market Basis", humanNumber(firstNonEmpty(row, "mrkt_basis"))),
		formatSummaryLine("Open Interest", joinNonEmpty(" / ",
			humanNumber(firstNonEmpty(row, "hts_otst_stpl_qty")),
			signedChangeText(firstNonEmpty(row, "otst_stpl_qty_icdc"), "increase", "decrease", ""),
		)),
		formatSummaryLine("Days To Expiry", daysText(firstNonEmpty(row, "hts_rmnn_dynu"))),
		formatSummaryLine("Volume / Turnover", joinNonEmpty(" / ",
			humanNumber(firstNonEmpty(row, "acml_vol")),
			humanNumber(firstNonEmpty(row, "acml_tr_pbmn")),
		)),
	})
}

func printFutureBoardTopSummary(resp *auth.RESTResponse) {
	topRow := firstRow(resp, "output1")
	if topRow == nil {
		return
	}

	printSummaryBlock("Quad Witching Futures Board Top", []string{
		formatSummaryLine("Underlying Price", humanNumber(firstNonEmpty(topRow, "unas_prpr"))),
		formatSummaryLine("Underlying Change / Rate", joinNonEmpty(" / ",
			signedChangeText(firstNonEmpty(topRow, "unas_prdy_vrss"), "up", "down", ""),
			firstNonEmpty(topRow, "unas_prdy_ctrt"),
		)),
		formatSummaryLine("Futures Price", humanNumber(firstNonEmpty(topRow, "futs_prpr"))),
		formatSummaryLine("Futures Change / Rate", joinNonEmpty(" / ",
			signedChangeText(firstNonEmpty(topRow, "futs_prdy_vrss"), "up", "down", ""),
			firstNonEmpty(topRow, "futs_prdy_ctrt"),
		)),
		formatSummaryLine("Days To Expiry", daysText(firstNonEmpty(topRow, "hts_rmnn_dynu"))),
	})
}

func printFutureBoardSummary(resp *auth.RESTResponse, targetCode string) {
	rows := rowsFromResponse(resp, "output")
	if len(rows) == 0 {
		return
	}

	lines := []string{
		formatSummaryLine("Contracts", fmt.Sprintf("%d", len(rows))),
	}

	if target := findRowByValue(rows, "futs_shrn_iscd", targetCode); target != nil {
		lines = append(lines, formatSummaryLine(
			"Selected Contract",
			fmt.Sprintf("%s %s, current %s, open interest %s, %s to expiry, expected match %s",
				firstNonEmpty(target, "futs_shrn_iscd"),
				firstNonEmpty(target, "hts_kor_isnm"),
				humanNumber(firstNonEmpty(target, "futs_prpr")),
				humanNumber(firstNonEmpty(target, "hts_otst_stpl_qty")),
				daysText(firstNonEmpty(target, "hts_rmnn_dynu")),
				humanNumber(firstNonEmpty(target, "futs_antc_cnpr")),
			),
		))
	}

	limit := 3
	if len(rows) < limit {
		limit = len(rows)
	}
	for i := 0; i < limit; i++ {
		row := rows[i]
		lines = append(lines, formatSummaryLine(
			fmt.Sprintf("Board Row %d", i+1),
			fmt.Sprintf("%s %s, current %s, open interest %s, expected match %s",
				firstNonEmpty(row, "futs_shrn_iscd"),
				firstNonEmpty(row, "hts_kor_isnm"),
				humanNumber(firstNonEmpty(row, "futs_prpr")),
				humanNumber(firstNonEmpty(row, "hts_otst_stpl_qty")),
				humanNumber(firstNonEmpty(row, "futs_antc_cnpr")),
			),
		))
	}

	printSummaryBlock("Quad Witching Futures Board", lines)
}

func printFutureTimeChartSummary(resp *auth.RESTResponse) {
	header := firstRow(resp, "output1")
	rows := rowsFromResponse(resp, "output2")

	lines := []string{
		formatSummaryLine("Contract", firstNonEmpty(header, "hts_kor_isnm")),
		formatSummaryLine("Samples", fmt.Sprintf("%d", len(rows))),
	}

	if len(rows) > 0 {
		latest := rows[0]
		lines = append(lines,
			formatSummaryLine("Latest Time", firstNonEmpty(latest, "stck_cntg_hour")),
			formatSummaryLine("Latest Futures Price", humanNumber(firstNonEmpty(latest, "futs_prpr"))),
			formatSummaryLine("Latest Basis / KOSPI200", joinNonEmpty(" / ",
				humanNumber(firstNonEmpty(latest, "basis")),
				humanNumber(firstNonEmpty(latest, "kospi200_nmix")),
			)),
			formatSummaryLine("Latest Trade Volume", humanNumber(firstNonEmpty(latest, "cntg_vol"))),
			formatSummaryLine("Open Interest / Change", joinNonEmpty(" / ",
				humanNumber(firstNonEmpty(latest, "hts_otst_stpl_qty")),
				signedChangeText(firstNonEmpty(latest, "otst_stpl_qty_icdc"), "increase", "decrease", ""),
			)),
		)
	}

	printSummaryBlock("Quad Witching Futures Time Chart", lines)
}

func printFutureExpectedPriceSummary(resp *auth.RESTResponse) {
	header := firstRow(resp, "output1")
	rows := rowsFromResponse(resp, "output2")
	if header == nil && len(rows) == 0 {
		return
	}

	lines := []string{
		formatSummaryLine("Contract", firstNonEmpty(header, "hts_kor_isnm")),
		formatSummaryLine("Samples", fmt.Sprintf("%d", len(rows))),
	}

	if len(rows) > 0 {
		latest := rows[0]
		lines = append(lines,
			formatSummaryLine("Latest Time", firstNonEmpty(latest, "stck_cntg_hour")),
			formatSummaryLine("Expected Match Price", humanNumber(firstNonEmpty(latest, "futs_antc_cnpr"))),
			formatSummaryLine("Expected Change", signedChangeText(firstNonEmpty(latest, "futs_antc_cntg_vrss"), "up", "down", "")),
			formatSummaryLine("Expected Rate", firstNonEmpty(latest, "antc_cntg_prdy_ctrt")),
		)
	}

	printSummaryBlock("Quad Witching Expected Price", lines)
}

func printFutureExecutionSummary(resp *auth.RESTResponse) {
	row := firstRowAny(resp, "output", "output1", "output2")
	rows := rowsFromResponse(resp, "output")
	if row == nil && len(rows) == 0 {
		return
	}

	lines := []string{
		formatSummaryLine("Rows", fmt.Sprintf("%d", len(rows))),
	}
	if row != nil {
		lines = append(lines,
			formatSummaryLine("Latest Time", firstNonEmpty(row, "stck_cntg_hour", "aspr_acpt_hour")),
			formatSummaryLine("Latest Price / Change", joinNonEmpty(" / ",
				humanNumber(firstNonEmpty(row, "futs_prpr")),
				signedChangeText(firstNonEmpty(row, "futs_prdy_vrss", "futs_antc_cntg_vrss"), "up", "down", ""),
			)),
			formatSummaryLine("Trade Volume", humanNumber(firstNonEmpty(row, "cntg_vol", "acml_vol"))),
			formatSummaryLine("First Row", preview(row)),
		)
	}

	printSummaryBlock("Quad Witching Futures Execution", lines)
}

func printFutureMemberSummary(resp *auth.RESTResponse) {
	row := firstRowAny(resp, "output", "output1", "output2")
	rows := rowsFromResponse(resp, "output")
	if row == nil && len(rows) == 0 {
		return
	}

	lines := []string{
		formatSummaryLine("Rows", fmt.Sprintf("%d", len(rows))),
	}
	if row != nil {
		lines = append(lines,
			formatSummaryLine("Foreign Position", netFlowText(firstNonEmpty(row, "frgn_ntby_qty", "frgn_shnu_vol", "frgn_seln_vol"), "계약")),
			formatSummaryLine("Institution Position", netFlowText(firstNonEmpty(row, "orgn_ntby_qty", "orgn_shnu_vol", "orgn_seln_vol"), "계약")),
			formatSummaryLine("Personal Position", netFlowText(firstNonEmpty(row, "prsn_ntby_qty", "prsn_shnu_vol", "prsn_seln_vol"), "계약")),
			formatSummaryLine("Raw First Row", preview(row)),
		)
	}

	printSummaryBlock("Quad Witching Futures Member", lines)
}

func printInvestorTrendSummary(title string, resp *auth.RESTResponse) {
	row := firstRow(resp, "output")
	if row == nil {
		return
	}

	if strings.TrimSpace(title) == "" {
		title = "Quad Witching Investor Trend"
	}

	printSummaryBlock(title, []string{
		formatSummaryLine("Business Date", firstNonEmpty(row, "stck_bsop_date")),
		formatSummaryLine("Personal", netFlowText(firstNonEmpty(row, "prsn_ntby_qty"), "주")),
		formatSummaryLine("Foreign", netFlowText(firstNonEmpty(row, "frgn_ntby_qty"), "주")),
		formatSummaryLine("Institution", netFlowText(firstNonEmpty(row, "orgn_ntby_qty"), "주")),
		formatSummaryLine("Net Amounts", joinNonEmpty(" / ",
			"개인 "+netFlowText(firstNonEmpty(row, "prsn_ntby_tr_pbmn"), ""),
			"외국인 "+netFlowText(firstNonEmpty(row, "frgn_ntby_tr_pbmn"), ""),
			"기관 "+netFlowText(firstNonEmpty(row, "orgn_ntby_tr_pbmn"), ""),
		)),
	})
}

func printForeignInstitutionSummary(resp *auth.RESTResponse) {
	rows := rowsFromResponse(resp, "output")
	if len(rows) == 0 {
		return
	}

	lines := []string{
		formatSummaryLine("Rows", fmt.Sprintf("%d", len(rows))),
	}

	limit := 30
	if len(rows) < limit {
		limit = len(rows)
	}
	for i := 0; i < limit; i++ {
		row := rows[i]
		lines = append(lines, formatSummaryLine(
			fmt.Sprintf("Rank %d", i+1),
			fmt.Sprintf("%s %s, 현재가 %s, 순수량은 %s, 순대금은 %s",
				firstNonEmpty(row, "mksc_shrn_iscd"),
				firstNonEmpty(row, "hts_kor_isnm"),
				stockPriceText(firstNonEmpty(row, "stck_prpr")),
				netFlowText(firstNonEmpty(row, "ntby_qty", "frgn_ntby_qty", "orgn_ntby_qty"), "주"),
				netFlowText(firstNonEmpty(row, "frgn_ntby_tr_pbmn", "orgn_ntby_tr_pbmn"), ""),
			),
		))
	}

	printSummaryBlock("Quad Witching Foreign/Institution Total", lines)
}

func printAskingPriceExpSummary(symbol string, resp *auth.RESTResponse) {
	orderBook := firstRow(resp, "output1")
	expected := firstRowAny(resp, "output2", "output")
	if orderBook == nil && expected == nil {
		return
	}

	printSummaryBlock("Quad Witching Closing Auction", []string{
		formatSummaryLine("Symbol", symbol),
		formatSummaryLine("Current Price", stockPriceText(firstNonEmpty(orderBook, "stck_prpr"))),
		formatSummaryLine("Expected Match Price", stockPriceText(firstNonEmpty(expected, "antc_cnpr"))),
		formatSummaryLine("Expected Change / Rate", joinNonEmpty(" / ",
			signedChangeText(firstNonEmpty(expected, "antc_cntg_vrss"), "up", "down", "flat"),
			firstNonEmpty(expected, "antc_cntg_prdy_ctrt"),
		)),
		formatSummaryLine("Expected Match Volume", humanNumber(firstNonEmpty(expected, "antc_vol"))),
		formatSummaryLine("Total Ask / Bid Balance", joinNonEmpty(" / ",
			humanNumber(firstNonEmpty(orderBook, "total_askp_rsqn")),
			humanNumber(firstNonEmpty(orderBook, "total_bidp_rsqn")),
		)),
		formatSummaryLine("VI Code", firstNonEmpty(orderBook, "vi_cls_code")),
	})
}

func printQuadWitchingSnapshotSummary(path string, snapshot quadwitching.SnapshotExport) {
	printSummaryBlock("Quad Witching Snapshot", []string{
		formatSummaryLine("Business Date", snapshot.BusinessDate),
		formatSummaryLine("Futures", joinNonEmpty(" ", snapshot.FuturesCode, snapshot.FuturesName)),
		formatSummaryLine("Endpoint Count", fmt.Sprintf("%d", len(snapshot.EndpointStates))),
		formatSummaryLine("JSON Export", path),
	})
}

func printConfiguredCommodityFutureSummary(
	ctx context.Context,
	svc *commodityfuture.Service,
	instrument commodityfuture.Instrument,
) {
	quote, err := svc.Quote(ctx, instrument)
	if err != nil {
		log.Printf("%s future quote error: %v", instrument.Name, err)
		return
	}

	title := joinNonEmpty(" ", instrument.Name, instrument.Symbol)
	printCommodityFutureSummary(title, quote)
}

func printCommodityFutureSummary(name string, quote *commodityfuture.Quote) {
	if quote == nil {
		return
	}

	printSummaryBlock(name+" Summary", []string{
		formatSummaryLine("Provider", commodityProviderPathText(quote.ProviderPath)),
		formatSummaryLine("Symbol / Product", joinNonEmpty(" / ", quote.Symbol, quote.ProductCode)),
		formatSummaryLine("Contract", joinNonEmpty(" / ", quote.Contract, quote.QuoteCode)),
		formatSummaryLine("Current", quoteNumberText(quote.Price)),
		formatSummaryLine("Change / Rate", joinNonEmpty(" / ",
			quoteSignedNumberText(quote.Change),
			quotePercentText(quote.ChangePercent),
		)),
		formatSummaryLine("Previous Close", quoteNumberText(quote.PreviousClose)),
		formatSummaryLine("Open / High / Low", joinNonEmpty(" / ",
			quoteNumberText(quote.Open),
			quoteNumberText(quote.High),
			quoteNumberText(quote.Low),
		)),
		formatSummaryLine("Volume", quoteNumberText(quote.Volume)),
		formatSummaryLine("Exchange / Currency", joinNonEmpty(" / ", quote.Exchange, quote.Currency)),
		formatSummaryLine("As Of", quote.AsOf),
		formatSummaryLine("Delay", quote.Delay),
		formatSummaryLine("Source", quote.SourceURL),
		formatSummaryLine("CME Page", quote.ReferenceURL),
		formatSummaryLine("Note", quote.Note),
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

func printExchangeRateSummary(marketDivCode string, symbol string, resp *auth.RESTResponse) {
	row := firstRow(resp, "output1")
	if row == nil {
		return
	}

	printSummaryBlock("Exchange Rate Summary", []string{
		formatSummaryLine("Market / Symbol", joinNonEmpty(" / ", marketDivCode, symbol)),
		formatSummaryLine("Name", firstNonEmpty(row, "hts_kor_isnm")),
		formatSummaryLine("Current", firstNonEmpty(row, "ovrs_nmix_prpr")),
		formatSummaryLine("Change", firstNonEmpty(row, "ovrs_nmix_prdy_vrss")),
		formatSummaryLine("Rate", firstNonEmpty(row, "prdy_ctrt")),
		formatSummaryLine("Previous Close", firstNonEmpty(row, "ovrs_nmix_prdy_clpr")),
		formatSummaryLine("Open / High / Low", joinNonEmpty(" / ",
			firstNonEmpty(row, "ovrs_nmix_oprc", "ovrs_prod_oprc"),
			firstNonEmpty(row, "ovrs_nmix_hgpr", "ovrs_prod_hgpr"),
			firstNonEmpty(row, "ovrs_nmix_lwpr", "ovrs_prod_lwpr"),
		)),
		formatSummaryLine("Volume", firstNonEmpty(row, "acml_vol")),
	})
}

func printShippingIndexSummary(quotes []shippingindex.Quote) {
	if len(quotes) == 0 {
		return
	}

	lines := []string{
		formatSummaryLine("Source", "StockQ public market summary"),
	}

	for _, quote := range quotes {
		lines = append(lines, formatSummaryLine(
			quote.Symbol,
			fmt.Sprintf("%s (%s) | %s / %s / %s | local %s",
				quote.Name,
				quote.Symbol,
				humanNumber(strconv.FormatFloat(quote.Price, 'f', -1, 64)),
				formatSignedFloat(quote.Change),
				formatPercentPoints(quote.ChangePercent),
				quote.Local,
			),
		))
	}

	printSummaryBlock("Shipping Index Summary", lines)
}

func printClientMetricsSummary(metrics auth.HTTPMetricsSnapshot) {
	lines := []string{
		formatSummaryLine("Call Count", strconv.Itoa(metrics.CallCount)),
		formatSummaryLine("Success / Error", fmt.Sprintf("%d / %d", metrics.SuccessCount, metrics.ErrorCount)),
		formatSummaryLine("Total Time", metrics.TotalDuration.String()),
		formatSummaryLine("Average Time", metrics.AverageTime.String()),
		formatSummaryLine("Elapsed", metrics.Elapsed.String()),
		formatSummaryLine("RPM", formatFloat(metrics.RPM)),
	}

	if !metrics.StartedAt.IsZero() {
		lines = append(lines, formatSummaryLine("Started At", metrics.StartedAt.Format(time.RFC3339)))
	}
	if !metrics.LastCallAt.IsZero() {
		lines = append(lines, formatSummaryLine("Last Call At", metrics.LastCallAt.Format(time.RFC3339)))
	}

	printSummaryBlock("KIClient Metrics", lines)
}

func configuredCommodityInstrument(name string, ticker string, productCode string, cmeProductID string) commodityfuture.Instrument {
	instrument := commodityfuture.ResolveInstrument(name, ticker, productCode)
	if strings.TrimSpace(cmeProductID) != "" {
		instrument.CMEProductID = strings.TrimSpace(cmeProductID)
	}
	return instrument
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

func humanNumber(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, ",", ""))
	if value == "" {
		return ""
	}

	parsedInt, err := strconv.ParseInt(value, 10, 64)
	if err == nil {
		return formatIntWithCommas(parsedInt)
	}

	parsedFloat, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return value
	}

	if parsedFloat == float64(int64(parsedFloat)) {
		return formatIntWithCommas(int64(parsedFloat))
	}

	negative := parsedFloat < 0
	if negative {
		parsedFloat = -parsedFloat
	}
	raw := strconv.FormatFloat(parsedFloat, 'f', 2, 64)
	parts := strings.SplitN(raw, ".", 2)
	intPart, _ := strconv.ParseInt(parts[0], 10, 64)
	formatted := formatIntWithCommas(intPart)
	if len(parts) == 2 {
		decimal := strings.TrimRight(parts[1], "0")
		if decimal != "" {
			formatted += "." + decimal
		}
	}
	if negative {
		return "-" + formatted
	}
	return formatted
}

func formatIntWithCommas(value int64) string {
	negative := value < 0
	if negative {
		value = -value
	}

	raw := strconv.FormatInt(value, 10)
	if len(raw) <= 3 {
		if negative {
			return "-" + raw
		}
		return raw
	}

	var builder strings.Builder
	if negative {
		builder.WriteByte('-')
	}
	prefixLen := len(raw) % 3
	if prefixLen == 0 {
		prefixLen = 3
	}
	builder.WriteString(raw[:prefixLen])
	for i := prefixLen; i < len(raw); i += 3 {
		builder.WriteByte(',')
		builder.WriteString(raw[i : i+3])
	}
	return builder.String()
}

func netFlowText(value string, unit string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	parsed, err := strconv.ParseFloat(strings.ReplaceAll(value, ",", ""), 64)
	if err != nil {
		return joinNonEmpty(" ", value, unit)
	}

	action := "순매수"
	if parsed < 0 {
		action = "순매도"
		parsed = -parsed
	}
	if parsed == 0 {
		action = "중립"
	}

	number := humanNumber(strconv.FormatFloat(parsed, 'f', -1, 64))
	if action == "중립" {
		if unit != "" {
			return action + " " + number + unit
		}
		return action + " " + number
	}
	return joinNonEmpty(" ", action, number+unit)
}

func signedChangeText(value string, positiveWord string, negativeWord string, zeroWord string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	parsed, err := strconv.ParseFloat(strings.ReplaceAll(value, ",", ""), 64)
	if err != nil {
		return value
	}

	switch {
	case parsed > 0:
		if positiveWord == "" {
			return "+" + humanNumber(strconv.FormatFloat(parsed, 'f', -1, 64))
		}
		return positiveWord + " " + humanNumber(strconv.FormatFloat(parsed, 'f', -1, 64))
	case parsed < 0:
		if negativeWord == "" {
			return humanNumber(strconv.FormatFloat(parsed, 'f', -1, 64))
		}
		return negativeWord + " " + humanNumber(strconv.FormatFloat(-parsed, 'f', -1, 64))
	default:
		if zeroWord != "" {
			return zeroWord
		}
		return "0"
	}
}

func stockPriceText(value string) string {
	value = humanNumber(value)
	if value == "" {
		return ""
	}
	return value + "원"
}

func daysText(value string) string {
	value = humanNumber(value)
	if value == "" {
		return ""
	}
	return value + "일"
}

func DCFInputStatusText(input domesticstock.DCFInputValue) string {
	if !input.HasValue {
		return string(input.Status)
	}

	switch input.Name {
	case "EffectiveTax", "EquityWeight", "DebtWeight", "RiskFreeRate", "MarketPremium", "CostOfDebt", "RevenueGrowth", "EBITMargin", "DNAMargin", "CapExMargin", "NWCMargin":
		return fmt.Sprintf("%s (%s)", formatPercent(input.Value), input.Status)
	default:
		return fmt.Sprintf("%s (%s)", formatFloat(input.Value), input.Status)
	}
}

func formatBool(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

func firstRow(resp *auth.RESTResponse, outputKey string) map[string]any {
	rows := rowsFromResponse(resp, outputKey)
	if len(rows) == 0 {
		return nil
	}
	return rows[0]
}

func firstRowAny(resp *auth.RESTResponse, outputKeys ...string) map[string]any {
	for _, outputKey := range outputKeys {
		if row := firstRow(resp, outputKey); row != nil {
			return row
		}
	}
	return nil
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

func findRowByValue(rows []map[string]any, key string, target string) map[string]any {
	target = strings.TrimSpace(target)
	if target == "" {
		return nil
	}
	for _, row := range rows {
		if fieldString(row, key) == target {
			return row
		}
	}
	return nil
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

func normalizeYMD(value string) string {
	value = strings.TrimSpace(value)
	if len(value) != 8 {
		return ""
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return ""
		}
	}
	return value
}

func fieldFloat(row map[string]any, key string) (float64, bool) {
	value := fieldString(row, key)
	if value == "" {
		return 0, false
	}

	parsed, err := strconv.ParseFloat(strings.ReplaceAll(value, ",", ""), 64)
	if err != nil {
		return 0, false
	}
	return parsed, true
}

func computeBasis(row map[string]any) (float64, bool) {
	if basis, ok := fieldFloat(row, "basis"); ok {
		return basis, true
	}

	futuresPrice, ok := fieldFloat(row, "futs_prpr")
	if !ok {
		return 0, false
	}
	spotIndex, ok := fieldFloat(row, "bstp_nmix_prpr")
	if !ok {
		return 0, false
	}

	return futuresPrice - spotIndex, true
}

func formatOptionalFloat(value float64, ok bool) string {
	if !ok {
		return ""
	}
	return formatFloat(value)
}

func formatOptionalHumanNumber(value float64, ok bool) string {
	if !ok {
		return ""
	}
	return humanNumber(strconv.FormatFloat(value, 'f', -1, 64))
}

func resolveMonteCarloJSONPath(basePath string, businessDate string, symbol string) string {
	basePath = strings.TrimSpace(basePath)
	if basePath == "" {
		basePath = ".cache/dcf_monte_carlo.json"
	}

	dir := filepath.Dir(basePath)
	filename := filepath.Base(basePath)
	ext := filepath.Ext(filename)
	stem := filename
	if ext != "" {
		stem = strings.TrimSuffix(filename, ext)
	}

	parts := []string{stem}
	if normalizedDate := normalizeYMD(businessDate); normalizedDate != "" {
		parts = append(parts, normalizedDate)
	}
	symbol = strings.TrimSpace(symbol)
	if symbol != "" {
		parts = append(parts, symbol)
	}

	resolved := strings.Join(parts, ".")
	if ext == "" {
		ext = ".json"
	}
	return filepath.Join(dir, resolved+ext)
}

func resolveCompanyAnalysisJSONPath(basePath string, businessDate string, symbol string) string {
	basePath = strings.TrimSpace(basePath)
	if basePath == "" {
		basePath = ".cache/company_analysis.json"
	}

	dir := filepath.Dir(basePath)
	filename := filepath.Base(basePath)
	ext := filepath.Ext(filename)
	stem := filename
	if ext != "" {
		stem = strings.TrimSuffix(filename, ext)
	}

	parts := []string{stem}
	if normalizedDate := normalizeYMD(businessDate); normalizedDate != "" {
		parts = append(parts, normalizedDate)
	}
	symbol = strings.TrimSpace(symbol)
	if symbol != "" {
		parts = append(parts, symbol)
	}

	resolved := strings.Join(parts, ".")
	if ext == "" {
		ext = ".json"
	}
	return filepath.Join(dir, resolved+ext)
}

func resolveQuadWitchingSnapshotPath(basePath string, businessDate string, futuresCode string) string {
	basePath = strings.TrimSpace(basePath)
	if basePath == "" {
		basePath = ".cache/quad_witching_snapshot.json"
	}

	dir := filepath.Dir(basePath)
	filename := filepath.Base(basePath)
	ext := filepath.Ext(filename)
	stem := filename
	if ext != "" {
		stem = strings.TrimSuffix(filename, ext)
	}

	parts := []string{stem}
	if normalizedDate := normalizeYMD(businessDate); normalizedDate != "" {
		parts = append(parts, normalizedDate)
	}
	futuresCode = strings.TrimSpace(futuresCode)
	if futuresCode != "" {
		parts = append(parts, futuresCode)
	}

	resolved := strings.Join(parts, ".")
	if ext == "" {
		ext = ".json"
	}
	return filepath.Join(dir, resolved+ext)
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

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		items = append(items, part)
	}
	return items
}

func formatFloat(value float64) string {
	return fmt.Sprintf("%.2f", value)
}

func formatSignedFloat(value float64) string {
	if value > 0 {
		return "+" + formatFloat(value)
	}
	return formatFloat(value)
}

func formatPercentPoints(value float64) string {
	return fmt.Sprintf("%.2f%%", value)
}

func formatPercent(value float64) string {
	return fmt.Sprintf("%.2f%%", value*100)
}

func formatMoney(currency string, value float64) string {
	label := strings.TrimSpace(currency)
	number := humanNumber(strconv.FormatFloat(value, 'f', 2, 64))
	if label == "" {
		return number
	}
	return label + " " + number
}

func formatSignedMoney(currency string, value float64) string {
	if value > 0 {
		return "+" + formatMoney(currency, value)
	}
	return formatMoney(currency, value)
}

func commodityProviderPathText(path string) string {
	parts := splitCSV(strings.ReplaceAll(path, "->", ","))
	if len(parts) == 0 {
		return ""
	}

	labels := make([]string, 0, len(parts))
	for _, part := range parts {
		switch strings.ToLower(strings.TrimSpace(part)) {
		case commodityfuture.ProviderCME:
			labels = append(labels, "CME delayed")
		case commodityfuture.ProviderYahoo:
			labels = append(labels, "Yahoo Finance")
		default:
			labels = append(labels, part)
		}
	}
	return strings.Join(labels, " -> ")
}

func quoteNumberText(value *float64) string {
	if value == nil {
		return ""
	}
	return humanNumber(strconv.FormatFloat(*value, 'f', -1, 64))
}

func quoteSignedNumberText(value *float64) string {
	if value == nil {
		return ""
	}
	number := quoteNumberText(value)
	if number == "" {
		return ""
	}
	if *value > 0 {
		return "+" + number
	}
	return number
}

func quotePercentText(value *float64) string {
	if value == nil {
		return ""
	}
	return formatPercentPoints(*value)
}

func cacheLabel(cacheHit bool) string {
	if cacheHit {
		return "cache"
	}
	return "api"
}
