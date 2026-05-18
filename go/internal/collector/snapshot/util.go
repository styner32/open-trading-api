package snapshot

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/kis-open-api/go/internal/auth"
)

func rows(resp *auth.RESTResponse, key string) []map[string]any {
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

func firstRow(resp *auth.RESTResponse, keys ...string) map[string]any {
	for _, key := range keys {
		if rows := rows(resp, key); len(rows) > 0 {
			return rows[0]
		}
	}
	return nil
}

func num(row map[string]any, keys ...string) (float64, bool) {
	for _, key := range keys {
		value, ok := parseNumber(row[key])
		if ok {
			return value, true
		}
	}
	return 0, false
}

func parseNumber(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case int:
		return float64(typed), true
	case string:
		clean := strings.ReplaceAll(strings.TrimSpace(typed), ",", "")
		if clean == "" {
			return 0, false
		}
		parsed, err := strconv.ParseFloat(clean, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func ptr(v float64) *float64 { return &v }

func absPercent(numerator float64, denominator float64) *float64 {
	if denominator == 0 || math.IsNaN(denominator) || math.IsInf(denominator, 0) {
		return nil
	}
	return ptr(math.Abs(numerator) / math.Abs(denominator) * 100)
}

func errText(err error) string {
	if err == nil {
		return "manual input not provided"
	}
	return fmt.Sprintf("%v", err)
}
