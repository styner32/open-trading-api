package snapshot

import (
	"context"
	"time"

	"github.com/kis-open-api/go/internal/auth"
	"github.com/kis-open-api/go/internal/domesticfutureoption"
	"github.com/kis-open-api/go/internal/domesticstock"
	"github.com/kis-open-api/go/internal/external/yahoo"
)

type DomesticStock interface {
	InquireIndexDailyPrice(context.Context, string, string) ([]map[string]any, error)
	InquireIndexPrice(context.Context, string) (*auth.RESTResponse, error)
	InquireInvestorDailyByMarket(context.Context, string) (*auth.RESTResponse, error)
	InquirePrice(context.Context, string) (*auth.RESTResponse, error)
	KOSPIMarketCapSummary(context.Context, string) (*domesticstock.KOSPIMarketCapSummary, error)
}

type DomesticFuture interface {
	ResolveNearMonthKOSPI200Futures(context.Context, string) (*domesticfutureoption.ResolvedContract, error)
	InquirePrice(context.Context, string, string) (*auth.RESTResponse, error)
}

type YahooQuotes interface {
	GetQuotes(context.Context, []string) (map[string]yahoo.Quote, error)
}

type Deps struct {
	DomesticStock  DomesticStock
	DomesticFuture DomesticFuture
	Yahoo          YahooQuotes
}

type Options struct {
	Date                           string
	SidecarStatus                  string
	SidecarTime                    string
	SemiconductorForeignNetSellEok *float64
	MonthlyForeignNetSellEok       *float64
	MonthlyForeignNote             string
	ForeignHoldingChangePP         *float64
	SamsungSKHynixCapRatio         *float64
	USDKRWMonthStart               *float64
}

type Snapshot struct {
	Timestamp  time.Time
	Price      *PriceSection
	Flow       *FlowSection
	Impact     *ImpactSection
	Global     *GlobalSection
	Cumulative *CumulativeSection
	Macro      *MacroSection
	Errors     map[string]error
}
