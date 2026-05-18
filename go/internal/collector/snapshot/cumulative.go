package snapshot

import (
	"context"
	"fmt"
)

type CumulativeSection struct {
	MonthlyForeignNetSellEok *float64
	MonthlyForeignNote       string
	MonthlyReason            string
	ForeignHoldingChangePP   *float64
	ForeignHoldingReason     string
	SamsungSKHynixCapRatio   *float64
	CapRatioReason           string
}

// collectCumulative computes Samsung Electronics + SK Hynix KOSPI cap ratio
// as (005930 market cap + 000660 market cap)/sum(KOSPI market cap)*100.
func collectCumulative(ctx context.Context, stock DomesticStock, date string, opts Options) *CumulativeSection {
	section := &CumulativeSection{
		MonthlyForeignNetSellEok: opts.MonthlyForeignNetSellEok,
		MonthlyForeignNote:       opts.MonthlyForeignNote,
		ForeignHoldingChangePP:   opts.ForeignHoldingChangePP,
		SamsungSKHynixCapRatio:   opts.SamsungSKHynixCapRatio,
	}
	if section.MonthlyForeignNetSellEok == nil {
		section.MonthlyReason = "manual input not provided"
	}
	if section.ForeignHoldingChangePP == nil {
		section.ForeignHoldingReason = "manual input not provided"
	}
	if section.SamsungSKHynixCapRatio == nil {
		section.SamsungSKHynixCapRatio, section.CapRatioReason = autoSemiconductorCapRatio(ctx, stock, date)
	}
	return section
}

func autoSemiconductorCapRatio(ctx context.Context, stock DomesticStock, date string) (*float64, string) {
	if stock == nil {
		return nil, "domestic stock dependency is nil"
	}
	samsung, err := stock.InquirePrice(ctx, "005930")
	if err != nil {
		return nil, "005930 market cap: " + err.Error()
	}
	hynix, err := stock.InquirePrice(ctx, "000660")
	if err != nil {
		return nil, "000660 market cap: " + err.Error()
	}
	summary, err := stock.KOSPIMarketCapSummary(ctx, date)
	if err != nil {
		return nil, err.Error()
	}
	samsungCap, ok := num(firstRow(samsung, "output"), "hts_avls")
	if !ok {
		return nil, "005930 hts_avls missing"
	}
	hynixCap, ok := num(firstRow(hynix, "output"), "hts_avls")
	if !ok {
		return nil, "000660 hts_avls missing"
	}
	if summary.TotalMarketCap <= 0 {
		return nil, fmt.Sprintf("invalid KOSPI total market cap %.2f", summary.TotalMarketCap)
	}
	return ptr((samsungCap + hynixCap) / summary.TotalMarketCap * 100), ""
}
