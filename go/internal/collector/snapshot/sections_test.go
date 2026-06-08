package snapshot

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/kis-open-api/go/internal/auth"
	"github.com/kis-open-api/go/internal/domesticfutureoption"
	"github.com/kis-open-api/go/internal/domesticstock"
	"github.com/kis-open-api/go/internal/external/naver"
	"github.com/kis-open-api/go/internal/external/yahoo"
)

type fakeStock struct {
	dailyRows []map[string]any
	investor  *auth.RESTResponse
	prices    map[string]*auth.RESTResponse
	cap       *domesticstock.KOSPIMarketCapSummary
	program   *auth.RESTResponse
	timeFlow  *auth.RESTResponse
	compProg  *auth.RESTResponse
}

func (f fakeStock) InquireIndexDailyPrice(context.Context, string, string) ([]map[string]any, error) {
	return f.dailyRows, nil
}
func (f fakeStock) InquireIndexPrice(context.Context, string) (*auth.RESTResponse, error) {
	// KOSPI 200 지수 조회를 위한 mock 대응
	return &auth.RESTResponse{Body: map[string]any{"output": []any{map[string]any{"bstp_nmix_prpr": "350.0"}}}}, nil
}
func (f fakeStock) InquireInvestorDailyByMarket(context.Context, string) (*auth.RESTResponse, error) {
	return f.investor, nil
}
func (f fakeStock) InquirePrice(_ context.Context, symbol string) (*auth.RESTResponse, error) {
	if resp, ok := f.prices[symbol]; ok {
		return resp, nil
	}
	return nil, errors.New("price missing: " + symbol)
}
func (f fakeStock) KOSPIMarketCapSummary(context.Context, string) (*domesticstock.KOSPIMarketCapSummary, error) {
	if f.cap == nil {
		return nil, errors.New("cap summary missing")
	}
	return f.cap, nil
}
func (f fakeStock) ResolveVKOSPICode(context.Context, []string) (string, error) {
	return "2050", nil
}
func (f fakeStock) InquireVKOSPIPrice(context.Context, string) (*auth.RESTResponse, error) {
	return nil, nil
}
func (f fakeStock) InquireVKOSPIDailyPrice(context.Context, string, string) ([]map[string]any, error) {
	return nil, nil
}
func (f fakeStock) MarketFunds(context.Context, string) (*auth.RESTResponse, error) {
	return nil, nil
}
func (f fakeStock) InvestorProgramTradeToday(context.Context, string) (*auth.RESTResponse, error) {
	return f.program, nil
}
func (f fakeStock) InquireInvestorTimeByMarket(context.Context, string, string) (*auth.RESTResponse, error) {
	return f.timeFlow, nil
}
func (f fakeStock) CompProgramTradeToday(context.Context, string) (*auth.RESTResponse, error) {
	return f.compProg, nil
}

type fakeNaver struct {
	quote   *naver.IndexQuote
	history []naver.DailyClose
	err     error
}

func (f fakeNaver) GetIndexQuote(context.Context, string) (*naver.IndexQuote, error) {
	return f.quote, f.err
}
func (f fakeNaver) GetIndexDailyHistory(context.Context, string, int) ([]naver.DailyClose, error) {
	return f.history, f.err
}

type fakeFuture struct {
	resp *auth.RESTResponse
}

func (f fakeFuture) ResolveNearMonthKOSPI200Futures(context.Context, string) (*domesticfutureoption.ResolvedContract, error) {
	return &domesticfutureoption.ResolvedContract{Record: domesticfutureoption.MasterRecord{ShortCode: "101V03"}}, nil
}
func (f fakeFuture) InquirePrice(context.Context, string, string) (*auth.RESTResponse, error) {
	return f.resp, nil
}

type fakeYahoo struct {
	quotes map[string]yahoo.Quote
	err    error
}

func (f fakeYahoo) GetQuotes(context.Context, []string) (map[string]yahoo.Quote, error) {
	return f.quotes, f.err
}
func (f fakeYahoo) GetChartHistory(context.Context, string, string, string) ([]yahoo.DailyClose, error) {
	return nil, nil
}

func TestPriceCollectsDailyRange(t *testing.T) {
	tests := []struct {
		name             string
		rows             []map[string]any
		wantRangePoints  float64
		wantRangePercent float64
		wantYearHigh     bool
	}{
		{name: "daily row", rows: []map[string]any{{
			"stck_bsop_date": "20260515", "bstp_nmix_prpr": "7950",
			"bstp_nmix_oprc": "7900", "bstp_nmix_hgpr": "8050", "bstp_nmix_lwpr": "7600",
			"stck_prdy_clpr": "8000", "dryy_bstp_nmix_hgpr": "8050",
		}}, wantRangePoints: 450, wantRangePercent: 5.625, wantYearHigh: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := collectPrice(context.Background(), fakeStock{dailyRows: tt.rows}, "20260515")
			if err != nil {
				t.Fatal(err)
			}
			if got.RangePoints != tt.wantRangePoints || got.RangePercent != tt.wantRangePercent || got.YearHigh != tt.wantYearHigh {
				t.Fatalf("price section = %+v", got)
			}
		})
	}
}

func TestFlowConvertsTradeAmountsToEok(t *testing.T) {
	tests := []struct {
		name                                     string
		row                                      map[string]any
		wantForeign, wantInstitution, wantRetail float64
	}{
		{name: "million KRW to eok", row: map[string]any{
			"frgn_ntby_tr_pbmn": "-4834200", "orgn_ntby_tr_pbmn": "-734000", "prsn_ntby_tr_pbmn": "5419800",
		}, wantForeign: -48342, wantInstitution: -7340, wantRetail: 54198},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &auth.RESTResponse{Body: map[string]any{"output": []any{tt.row}}}
			got, err := collectFlow(context.Background(), fakeStock{investor: resp}, "20260515")
			if err != nil {
				t.Fatal(err)
			}
			if got.ForeignEok != tt.wantForeign || got.InstitutionEok != tt.wantInstitution || got.IndividualEok != tt.wantRetail {
				t.Fatalf("flow section = %+v", got)
			}
		})
	}
}

func TestImpactComputesRatiosAndSidecar(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "ratios and manual sidecar"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			semiSell := -42000.0
			stock := fakeStock{cap: &domesticstock.KOSPIMarketCapSummary{TotalMarketCap: 7_100_000}}
			futures := fakeFuture{resp: &auth.RESTResponse{Body: map[string]any{"output1": map[string]any{"futs_prdy_ctrt": "-5.09"}}}}
			got := collectImpact(context.Background(), Deps{DomesticStock: stock, DomesticFuture: futures}, "20260515", &FlowSection{ForeignEok: -48342}, &PriceSection{TradingValueEok: 200_000}, Options{
				SidecarStatus: "triggered", SidecarTime: "13:28:49", SemiconductorForeignNetSellEok: &semiSell,
			})
			if got.ForeignSellMarketCapPercent == nil || *got.ForeignSellMarketCapPercent < 0.68 {
				t.Fatalf("foreign sell ratio = %+v", got.ForeignSellMarketCapPercent)
			}
			if got.SemiconductorSellConcentrationPct == nil || *got.SemiconductorSellConcentrationPct < 86 {
				t.Fatalf("semiconductor ratio = %+v", got.SemiconductorSellConcentrationPct)
			}
			if got.FuturesChangePercent == nil || *got.FuturesChangePercent != -5.09 || got.SidecarStatus != "triggered" {
				t.Fatalf("impact section = %+v", got)
			}
		})
	}
}

func TestGlobalKeepsPartialQuotes(t *testing.T) {
	tests := []struct {
		name       string
		quotes     map[string]yahoo.Quote
		err        error
		wantReason string
	}{
		{name: "partial quotes", quotes: map[string]yahoo.Quote{
			"KRW=X": {Symbol: "KRW=X", Price: 1494.2, ChangePercent: 0.51},
		}, err: errors.New("yahoo quote missing: BTC-USD"), wantReason: "BTC-USD"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := collectGlobal(context.Background(), fakeYahoo{quotes: tt.quotes, err: tt.err})
			if err != nil || !strings.Contains(got.Reason, tt.wantReason) {
				t.Fatalf("global = %+v err=%v", got, err)
			}
		})
	}
}

func TestMacroComputesKRWMonthStartAndTNXRender(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "month start and tnx scale"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := 1430.0
			s := &Snapshot{Macro: &MacroSection{USDKRWMonthStart: &start, USDKRWMonthStartPct: ptr(4.49), Quotes: map[string]yahoo.Quote{
				"KRW=X": {Price: 1494.2}, "CL=F": {Price: 102.34}, "^TNX": {Price: 4.52}, // raw = percent
			}}}
			out := Render(s)
			if !strings.Contains(out, "USD/KRW: 1,494.20") || !strings.Contains(out, "미국 10년물: 4.52%") {
				t.Fatalf("rendered macro missing values:\n%s", out)
			}
		})
	}
}

func TestCumulativeManualAndMissingValues(t *testing.T) {
	tests := []struct {
		name    string
		monthly float64
	}{
		{name: "manual monthly only", monthly: -202000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := collectCumulative(context.Background(), nil, "20260515", Options{MonthlyForeignNetSellEok: &tt.monthly})
			if got.MonthlyForeignNetSellEok == nil || got.ForeignHoldingReason == "" || got.CapRatioReason == "" {
				t.Fatalf("cumulative section = %+v", got)
			}
		})
	}
}

func TestLateSession(t *testing.T) {
	t.Run("capitulation event detected", func(t *testing.T) {
		// Mock KIS REST responses
		progResp := &auth.RESTResponse{
			Body: map[string]any{
				"output1": []any{
					map[string]any{"invr_cls_name": "외국인투자자", "nabt_ntby_amt": "-40000"},
					map[string]any{"invr_cls_name": "기관합계", "nabt_ntby_amt": "-20000"},
					map[string]any{"invr_cls_name": "합계", "nabt_ntby_amt": "-60000"},
				},
			},
		}

		compProgResp := &auth.RESTResponse{
			Body: map[string]any{
				"output": []any{
					map[string]any{"bsop_hour": "153000", "whol_smtn_ntby_tr_pbmn": "-180000"},
					map[string]any{"bsop_hour": "152000", "whol_smtn_ntby_tr_pbmn": "-140000"},
					map[string]any{"bsop_hour": "150000", "whol_smtn_ntby_tr_pbmn": "-100000"},
				},
			},
		}

		timeFlowResp := &auth.RESTResponse{
			Body: map[string]any{
				"output": []any{
					map[string]any{"aspr_hour": "153000", "frgn_ntby_tr_pbmn": "-50000", "orgn_ntby_tr_pbmn": "-40000"},
					map[string]any{"aspr_hour": "152000", "frgn_ntby_tr_pbmn": "-20000", "orgn_ntby_tr_pbmn": "-20000"},
				},
			},
		}

		stock := fakeStock{
			program:  progResp,
			compProg: compProgResp,
			timeFlow: timeFlowResp,
		}

		futures := fakeFuture{
			resp: &auth.RESTResponse{
				Body: map[string]any{
					"output": []any{
						map[string]any{"futs_prpr": "349.5"},
					},
				},
			},
		}

		deps := Deps{
			DomesticStock:  stock,
			DomesticFuture: futures,
		}

		priceSec := &PriceSection{
			High: 100,
			Low:  90,
			Close: 90.1,
		}

		got, err := collectLateSession(context.Background(), deps, "20260515", priceSec)
		if err != nil {
			t.Fatal(err)
		}

		if got.SpotPrice != 350.0 {
			t.Errorf("expected SpotPrice 350.0, got %.2f", got.SpotPrice)
		}
		if got.FuturesPrice != 349.5 {
			t.Errorf("expected FuturesPrice 349.5, got %.2f", got.FuturesPrice)
		}
		if got.BasisPoint != -0.5 {
			t.Errorf("expected BasisPoint -0.5, got %.2f", got.BasisPoint)
		}
		if got.KOSPINetNonArbitrageForeign != -400.0 {
			t.Errorf("expected non-arbitrage foreign -400.0, got %.2f", got.KOSPINetNonArbitrageForeign)
		}
		if got.LateProgramNetEok != -800.0 {
			t.Errorf("expected late program flow -800.0, got %.2f", got.LateProgramNetEok)
		}
		if got.CloseSessionProgramNetEok != -400.0 {
			t.Errorf("expected close session program -400.0, got %.2f", got.CloseSessionProgramNetEok)
		}
		if got.CloseSessionForeignNetEok != -300.0 {
			t.Errorf("expected close session foreign -300.0, got %.2f", got.CloseSessionForeignNetEok)
		}
		if !got.CapitulationEvent {
			t.Errorf("expected CapitulationEvent to be true, got false (score: %.2f)", got.CapitulationScore)
		}
	})
}
