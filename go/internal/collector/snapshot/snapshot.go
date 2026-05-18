package snapshot

import (
	"context"
)

func Collect(ctx context.Context, deps Deps, opts Options) *Snapshot {
	date, ts := normalizeDate(opts.Date)
	s := &Snapshot{Timestamp: ts, Errors: map[string]error{}}
	if section, err := collectPrice(ctx, deps.DomesticStock, date); err != nil {
		s.Errors["price"] = err
	} else {
		s.Price = section
	}
	if section, err := collectFlow(ctx, deps.DomesticStock, date); err != nil {
		s.Errors["flow"] = err
	} else {
		s.Flow = section
	}
	if section, err := collectGlobal(ctx, deps.Yahoo); err != nil {
		s.Errors["global"] = err
	} else {
		s.Global = section
	}
	if section, err := collectMacro(ctx, deps.Yahoo, opts); err != nil {
		s.Errors["macro"] = err
	} else {
		s.Macro = section
	}
	s.Impact = collectImpact(ctx, deps, date, s.Flow, s.Price, opts)
	s.Cumulative = collectCumulative(ctx, deps.DomesticStock, date, opts)
	return s
}
