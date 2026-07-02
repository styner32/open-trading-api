package pulse

import (
	"context"
	"fmt"
	"math"

	"github.com/kis-open-api/go/internal/auth"
)

type indexStock interface {
	InquireIndexPrice(context.Context, string) (*auth.RESTResponse, error)
}

// collectIndex는 KIS inquire-index-price (TRID FHPUP02100000)로 지수 현황을 가져옵니다.
// indexCode: "0001" (KOSPI), "1001" (KOSDAQ)
func collectIndex(ctx context.Context, stock indexStock, indexCode string) (IndexLevel, error) {
	resp, err := stock.InquireIndexPrice(ctx, indexCode)
	if err != nil {
		return IndexLevel{}, fmt.Errorf("inquire-index-price (%s): %w", indexCode, err)
	}

	row := firstRowOf(resp, "output")
	if row == nil {
		return IndexLevel{}, fmt.Errorf("inquire-index-price (%s): output 행 없음", indexCode)
	}

	get := func(key string) float64 {
		v, _ := numOf(row, key)
		return v
	}
	getInt := func(key string) int {
		v, _ := intOf(row, key)
		return v
	}

	price := get("bstp_nmix_prpr")
	prdyVrss := get("bstp_nmix_prdy_vrss")
	prevClose := price - prdyVrss
	changePct := 0.0
	if prevClose != 0 {
		changePct = prdyVrss / prevClose * 100
	}

	// 전일 대비 % 필드가 있으면 우선 사용
	if v, ok := numOf(row, "bstp_nmix_prdy_ctrt"); ok && math.Abs(v) > 0.0001 {
		changePct = v
	}

	tradingValue := get("acml_tr_pbmn") / millionToEok // 백만원 → 억원

	return IndexLevel{
		Price:        price,
		PrevClose:    prevClose,
		ChangePct:    changePct,
		Open:         get("bstp_nmix_oprc"),
		High:         get("bstp_nmix_hgpr"),
		Low:          get("bstp_nmix_lwpr"),
		TradingValue: tradingValue,
		Advancers:    getInt("ascn_issu_cnt"),
		Decliners:    getInt("down_issu_cnt"),
		Unchanged:    getInt("stnr_issu_cnt"),
		OK:           true,
	}, nil
}
