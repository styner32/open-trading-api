package pulse

import (
	"context"
	"time"

	"github.com/kis-open-api/go/internal/auth"
	"github.com/kis-open-api/go/internal/domesticfutureoption"
	"github.com/kis-open-api/go/internal/domesticstock"
	"github.com/kis-open-api/go/internal/external/naver"
	"github.com/kis-open-api/go/internal/external/yahoo"
)

// DomesticStock는 펄스에 필요한 KIS 국내 주식 API 인터페이스.
type DomesticStock interface {
	MarketTime(context.Context) (*auth.RESTResponse, error)
	InquireInvestorTimeByMarket(context.Context, string, string) (*auth.RESTResponse, error)
	InquireIndexPrice(context.Context, string) (*auth.RESTResponse, error)
	InquireVKOSPIPrice(context.Context, string) (*auth.RESTResponse, error)
	InquirePrice(context.Context, string) (*auth.RESTResponse, error)
	CompProgramTradeToday(context.Context, string) (*auth.RESTResponse, error)
	KOSPIMarketCapSummary(context.Context, string) (*domesticstock.KOSPIMarketCapSummary, error)
	ResolveVKOSPICode(context.Context, []string) (string, error)
}

// DomesticFuture는 펄스의 지수선물/베이시스 조회에 필요한 최소 인터페이스입니다.
type DomesticFuture interface {
	ResolveNearMonthKOSPI200Futures(context.Context, string) (*domesticfutureoption.ResolvedContract, error)
	ResolveNearMonthKOSDAQ150Futures(context.Context, string) (*domesticfutureoption.ResolvedContract, error)
	InquirePrice(context.Context, string, string) (*auth.RESTResponse, error)
}

// YahooQuotes는 Yahoo Finance API 인터페이스.
type YahooQuotes interface {
	GetQuotes(context.Context, []string) (map[string]yahoo.Quote, error)
	GetChartHistory(context.Context, string, string, string) ([]yahoo.DailyClose, error)
}

// NaverFinance는 VKOSPI의 비공식 fallback 소스입니다.
type NaverFinance interface {
	GetIndexQuote(context.Context, string) (*naver.IndexQuote, error)
}

// Deps는 Collect에 주입되는 의존성 묶음.
type Deps struct {
	Stock    DomesticStock
	Future   DomesticFuture
	Yahoo    YahooQuotes
	Naver    NaverFinance
	Clock    func() time.Time // 테스트에서 고정 시각 주입 가능
	StoreDir string
}

// Options는 runIntradayPulse에서 파싱된 실행 옵션.
type Options struct {
	StoreDir string
	Lookback time.Duration // 기본 2h
	NoSave   bool
	JSON     bool
}

// FlowSnapshot은 KIS inquire-investor-time-by-market 1행 파싱 결과 (단위: 억원).
type FlowSnapshot struct {
	Foreign     float64
	Institution float64
	Individual  float64
	FinInvest   float64 // 금융투자 (scrt)
	InvTrust    float64 // 투신 (ivtr)
	Pension     float64 // 연기금 (fund)
	PrivEquity  float64 // 사모 (pe_fund)
	Insurance   float64 // 보험 (insu)
	Bank        float64 // 은행 (bank)
	EtcCorp     float64 // 기타법인 (etc_corp)
	OK          bool
}

// IndexLevel은 KIS inquire-index-price 파싱 결과.
type IndexLevel struct {
	Price        float64
	PrevClose    float64
	ChangePct    float64
	Open         float64
	High         float64
	Low          float64
	TradingValue float64 // 억원 단위
	Advancers    int
	Decliners    int
	Unchanged    int
	OK           bool
}

// Window는 Yahoo 분봉 기반 구간 변동 (1h / 2h).
type Window struct {
	Symbol    string
	Label     string
	Current   float64
	ChangePct float64 // 전일 종가 대비 (quote.ChangePercent)
	LastTS    time.Time
	Move1hPct *float64
	Move2hPct *float64
	OK        bool
	Reason    string
}

// FlowDelta는 1h 또는 2h 기준 수급 변화량 (억원).
type FlowDelta struct {
	RefTS       time.Time
	Elapsed     float64 // 분
	Foreign     float64
	Institution float64
	Individual  float64
	IndexDelta  float64
}

// ProgramTradeSnapshot은 차익/비차익 프로그램 순매수 누적값입니다 (억원).
type ProgramTradeSnapshot struct {
	Arbitrage    float64 `json:"arbitrage"`
	NonArbitrage float64 `json:"non_arbitrage"`
	Total        float64 `json:"total"`
	AsOf         string  `json:"as_of,omitempty"`
	OK           bool    `json:"ok"`
}

// ProgramTradeDelta는 저장된 누적 프로그램매매의 구간 변화량입니다 (억원).
type ProgramTradeDelta struct {
	RefTS        time.Time `json:"ref_ts"`
	Elapsed      float64   `json:"elapsed_minutes"`
	Arbitrage    float64   `json:"arbitrage"`
	NonArbitrage float64   `json:"non_arbitrage"`
	Total        float64   `json:"total"`
}

// IndexFutureSnapshot은 근월 지수선물과 동일 응답의 기초지수 값입니다.
type IndexFutureSnapshot struct {
	Code          string  `json:"code"`
	Name          string  `json:"name"`
	Price         float64 `json:"price"`
	PrevClose     float64 `json:"prev_close"`
	ChangePct     float64 `json:"change_pct"`
	SpotPrice     float64 `json:"spot_price"`
	SpotChangePct float64 `json:"spot_change_pct"`
	Basis         float64 `json:"basis"`
	MarketBasis   float64 `json:"market_basis"`
	BasisMatch    bool    `json:"basis_match"`
	OK            bool    `json:"ok"`
}

type BasisDelta struct {
	RefTS   time.Time `json:"ref_ts"`
	Elapsed float64   `json:"elapsed_minutes"`
	Value   float64   `json:"value"`
}

type ThresholdStatus struct {
	ThresholdPct   float64 `json:"threshold_pct"`
	CurrentGapPct  float64 `json:"current_gap_pct"`
	LowGapPct      float64 `json:"low_gap_pct"`
	CurrentReached bool    `json:"current_reached"`
	LowReached     bool    `json:"low_reached"`
}

type CircuitBreakerStatus struct {
	Market           string            `json:"market"`
	CurrentChangePct float64           `json:"current_change_pct"`
	LowChangePct     float64           `json:"low_change_pct"`
	Levels           []ThresholdStatus `json:"levels"`
	OK               bool              `json:"ok"`
}

type SidecarStatus struct {
	Market              string  `json:"market"`
	FuturesCode         string  `json:"futures_code"`
	Direction           string  `json:"direction"`
	FuturesChangePct    float64 `json:"futures_change_pct"`
	SpotChangePct       float64 `json:"spot_change_pct"`
	FuturesThresholdPct float64 `json:"futures_threshold_pct"`
	SpotThresholdPct    float64 `json:"spot_threshold_pct,omitempty"`
	FuturesGapPct       float64 `json:"futures_gap_pct"`
	SpotGapPct          float64 `json:"spot_gap_pct,omitempty"`
	ThresholdReached    bool    `json:"threshold_reached"`
	ActivationConfirmed bool    `json:"activation_confirmed"`
	OK                  bool    `json:"ok"`
}

type MarketSafety struct {
	CircuitBreakers []CircuitBreakerStatus `json:"circuit_breakers"`
	Sidecars        []SidecarStatus        `json:"sidecars"`
}

type IndexContribution struct {
	Code        string  `json:"code"`
	Name        string  `json:"name"`
	MarketCap   float64 `json:"market_cap"`
	WeightPct   float64 `json:"weight_pct"`
	ChangePct   float64 `json:"change_pct"`
	PointImpact float64 `json:"point_impact"`
}

type VolatilitySnapshot struct {
	Code      string  `json:"code"`
	Value     float64 `json:"value"`
	ChangePct float64 `json:"change_pct"`
	Source    string  `json:"source"`
	Reason    string  `json:"reason,omitempty"`
	OK        bool    `json:"ok"`
}

// Market은 단일 시장(KOSPI/KOSDAQ)의 통합 데이터.
type Market struct {
	Name        string
	Index       IndexLevel
	IntradayWin Window
	Flow        FlowSnapshot
	FlowDelta1h *FlowDelta
	FlowDelta2h *FlowDelta
}

// Pulse는 전체 펄스 수집 결과.
type Pulse struct {
	Now                time.Time
	Date               string // KST YYYYMMDD
	BusinessDate       string
	KOSPI              Market
	KOSDAQ             Market
	KOSPIProgram       ProgramTradeSnapshot
	KOSDAQProgram      ProgramTradeSnapshot
	KOSPIProgramDelta  *ProgramTradeDelta
	KOSDAQProgramDelta *ProgramTradeDelta
	KOSPI200Future     IndexFutureSnapshot
	KOSDAQ150Future    IndexFutureSnapshot
	BasisDelta1h       *BasisDelta
	BasisDelta2h       *BasisDelta
	VKOSPI             VolatilitySnapshot
	Safety             MarketSafety
	Contributions      []IndexContribution
	USDKRW             Window
	Macro              []Window // NQ=F, ES=F, YM=F, ^N225, CL=F, ^TNX
	StoredCount        int      // 당일 누적 레코드 수
	PrevTS             *time.Time
	Analysis           []string
	Errors             map[string]string
	StoreDir           string
	Saved              bool
}

// PulseRecord는 JSONL에 적립되는 한 줄 레코드.
type PulseRecord struct {
	TS             time.Time            `json:"ts"`
	BusinessDate   string               `json:"business_date,omitempty"`
	KOSPIIdx       float64              `json:"kospi_idx"`
	KOSDAQIdx      float64              `json:"kosdaq_idx"`
	KOSPIFlow      FlowSnapshot         `json:"kospi_flow"`
	KOSDAQFlow     FlowSnapshot         `json:"kosdaq_flow"`
	KOSPIProgram   ProgramTradeSnapshot `json:"kospi_program,omitempty"`
	KOSDAQProgram  ProgramTradeSnapshot `json:"kosdaq_program,omitempty"`
	KOSPI200Future IndexFutureSnapshot  `json:"kospi200_future,omitempty"`
	VKOSPI         VolatilitySnapshot   `json:"vkospi,omitempty"`
	USDKRW         float64              `json:"usdkrw"`
}
