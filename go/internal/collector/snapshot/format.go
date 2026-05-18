package snapshot

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

func number(v float64, decimals int) string {
	sign := ""
	if v < 0 {
		sign = "-"
		v = math.Abs(v)
	}
	raw := strconv.FormatFloat(v, 'f', decimals, 64)
	parts := strings.Split(raw, ".")
	intPart := parts[0]
	var grouped []byte
	for i, r := range reverse(intPart) {
		if i > 0 && i%3 == 0 {
			grouped = append(grouped, ',')
		}
		grouped = append(grouped, byte(r))
	}
	out := sign + reverse(string(grouped))
	if len(parts) == 2 {
		out += "." + parts[1]
	}
	return out
}

func signedNumber(v float64, decimals int) string {
	if v > 0 {
		return "+" + number(v, decimals)
	}
	return number(v, decimals)
}

func percent(v float64) string { return signedNumber(v, 2) + "%" }

func percentPlain(v float64) string { return number(v, 2) + "%" }

func eok(v float64) string { return signedNumber(math.Round(v), 0) }

func trillionFromEok(v float64) string {
	return signedNumber(v/10000, 1) + "조원"
}

func pp(v float64) string { return signedNumber(v, 1) + "%p" }

func na(reason string) string {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "manual input not provided"
	}
	return "n/a (" + reason + ")"
}

func valuePercent(value *float64, reason string) string {
	if value == nil {
		return na(reason)
	}
	return percentPlain(*value)
}

func quotePercent(value *float64, reason string) string {
	if value == nil {
		return na(reason)
	}
	return percent(*value)
}

func reverse(value string) string {
	runes := []rune(value)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

func appendReason(value string, reason string) string {
	if strings.TrimSpace(reason) == "" {
		return value
	}
	return fmt.Sprintf("%s (%s)", value, reason)
}
