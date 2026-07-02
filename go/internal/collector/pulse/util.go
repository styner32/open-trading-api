package pulse

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/kis-open-api/go/internal/auth"
)

const millionToEok = 100.0 // 백만원 → 억원

// rowsOf는 RESTResponse에서 key의 []map[string]any 를 꺼냅니다.
func rowsOf(resp *auth.RESTResponse, key string) []map[string]any {
	if resp == nil || resp.Body == nil {
		return nil
	}
	switch typed := resp.Body[key].(type) {
	case []any:
		out := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			if row, ok := item.(map[string]any); ok {
				out = append(out, row)
			}
		}
		return out
	case map[string]any:
		return []map[string]any{typed}
	default:
		return nil
	}
}

// firstRowOf는 여러 key 중 첫 번째로 행이 있는 것의 첫 행을 반환합니다.
func firstRowOf(resp *auth.RESTResponse, keys ...string) map[string]any {
	for _, key := range keys {
		if rs := rowsOf(resp, key); len(rs) > 0 {
			return rs[0]
		}
	}
	return nil
}

// numOf는 row에서 key의 숫자 값을 파싱합니다.
func numOf(row map[string]any, keys ...string) (float64, bool) {
	for _, key := range keys {
		if v, ok := parseNum(row[key]); ok {
			return v, true
		}
	}
	return 0, false
}

func parseNum(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case string:
		clean := strings.ReplaceAll(strings.TrimSpace(v), ",", "")
		if clean == "" {
			return 0, false
		}
		parsed, err := strconv.ParseFloat(clean, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func intOf(row map[string]any, keys ...string) (int, bool) {
	for _, key := range keys {
		if v, ok := parseNum(row[key]); ok {
			return int(math.Round(v)), true
		}
	}
	return 0, false
}

// fmtEok은 억원 금액을 "▲+1.23조" 형식으로 포맷합니다.
func fmtEok(v float64) string {
	ar := arrow(v)
	abs := math.Abs(v)
	if abs >= 10000 {
		return fmt.Sprintf("%s%.2f조", ar, v/10000)
	}
	return fmt.Sprintf("%s%.0f억", ar, v)
}

// fmtAmountEok은 방향성이 없는 금액을 부호/화살표 없이 표시합니다.
func fmtAmountEok(v float64) string {
	abs := math.Abs(v)
	if abs >= 10000 {
		return fmt.Sprintf("%.2f조", abs/10000)
	}
	return fmt.Sprintf("%.0f억", abs)
}

func flowDirection(v float64) string {
	if v > 0 {
		return "순매수"
	}
	if v < 0 {
		return "순매도"
	}
	return "중립"
}

func elapsedLabel(minutes float64) string {
	if minutes <= 0 {
		return "구간"
	}
	return fmt.Sprintf("%.0fm", minutes)
}

func hourlyRate(value, elapsedMinutes float64) float64 {
	if elapsedMinutes <= 0 {
		return 0
	}
	return value * 60 / elapsedMinutes
}

// fmtEokWithUnit은 조/억 단위를 명시합니다.
func fmtEokWithUnit(v float64) string {
	abs := math.Abs(v)
	ar := arrow(v)
	if abs >= 10000 {
		return fmt.Sprintf("%s%.2f조", ar, v/10000)
	}
	return fmt.Sprintf("%s%.0f억", ar, v)
}

// fmtPct는 퍼센트 포맷.
func fmtPct(v float64) string {
	return fmt.Sprintf("%+.2f%%", v)
}

// fmtPctPtr는 nil-safe 퍼센트.
func fmtPctPtr(v *float64) string {
	if v == nil {
		return "N/A"
	}
	return fmtPct(*v)
}

// arrow는 부호 화살표.
func arrow(v float64) string {
	if v > 0 {
		return "▲+"
	}
	if v < 0 {
		return "▼"
	}
	return " "
}

// arrowNeutral은 부호 화살표 (0은 blank).
func arrowNeutral(v float64) string {
	if v > 0 {
		return "▲"
	}
	if v < 0 {
		return "▼"
	}
	return "─"
}

func ptr(v float64) *float64 { return &v }
