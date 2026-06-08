package snapshot

import (
	"fmt"
	"strings"
)

// render_extra.go: Section 7 (변동성), 9 (Regime), 10 (집중도) 렌더링

func renderVolatility(b *strings.Builder, s *Snapshot, p *SnapshotJSON) {
	b.WriteString("## 7. 변동성\n")
	v := s.Volatility
	if v == nil {
		b.WriteString("- " + na("volatility section unavailable") + "\n\n")
		return
	}
	if v.VKOSPI <= 0 {
		b.WriteString("- " + na(v.Reason) + "\n\n")
		return
	}
	vkLine := fmt.Sprintf("%.2f [%s]", v.VKOSPI, v.Level)
	if v.VKOSPIChange != 0 {
		vkLine += fmt.Sprintf(" (전일 %s)", percent(v.VKOSPIChange))
	}
	if p != nil && p.Volatility != nil && p.Volatility.VKOSPI > 0 {
		diff := v.VKOSPI - p.Volatility.VKOSPI
		vkLine += fmt.Sprintf("  [%s: %.2f, △%.2f]", p.Date, p.Volatility.VKOSPI, diff)
	}
	b.WriteString("- VKOSPI: " + vkLine + "\n")
	b.WriteString("  - 임계값: <20 정상, 20~25 평상시, 25~30 주의, >30 위험\n")
	if v.VKOSPI5DayAvg > 0 {
		b.WriteString(fmt.Sprintf("- VKOSPI 5일 평균: %.2f\n", v.VKOSPI5DayAvg))
	}
	if v.VIX > 0 {
		vixLine := fmt.Sprintf("%.2f", v.VIX)
		if v.VIXChange != 0 {
			vixLine += fmt.Sprintf(" (전일 %s)", percent(v.VIXChange))
		}
		if p != nil && p.Volatility != nil && p.Volatility.VIX > 0 {
			diff := v.VIX - p.Volatility.VIX
			vixLine += fmt.Sprintf("  [%s: %.2f, △%.2f]", p.Date, p.Volatility.VIX, diff)
		}
		b.WriteString("- VIX: " + vixLine + "\n")
	}
	if v.DecouplingFlag {
		b.WriteString("- 지수-VKOSPI 디커플링: ⚠ 감지됨\n")
	}
	if v.Reason != "" {
		b.WriteString("- 부분 오류: " + v.Reason + "\n")
	}
	b.WriteString("\n")
}

func renderCredit(b *strings.Builder, s *Snapshot, p *SnapshotJSON) {
	b.WriteString("## 8. 신용잔고 / 반대매매\n")
	c := s.Credit
	if c == nil {
		b.WriteString("- " + na(sectionErr(s, "credit")) + "\n\n")
		return
	}
	creditLine := trillionFromEok(c.CreditLoanBalanceEok)
	if p != nil && p.Credit != nil && p.Credit.CreditLoanBalanceEok != 0 {
		diff := c.CreditLoanBalanceEok - p.Credit.CreditLoanBalanceEok
		creditLine += fmt.Sprintf("  [전일 %s, %s]", trillionFromEok(p.Credit.CreditLoanBalanceEok), eok(diff)+"억")
	}
	b.WriteString("- 신용융자 잔고: " + creditLine + "\n")

	depositLine := trillionFromEok(c.CustomerDepositEok)
	if c.DepositChangeEok != 0 {
		depositLine += fmt.Sprintf(" (전일 대비 %s억)", eok(c.DepositChangeEok))
	}
	if p != nil && p.Credit != nil && p.Credit.CustomerDepositEok != 0 {
		diff := c.CustomerDepositEok - p.Credit.CustomerDepositEok
		depositLine += fmt.Sprintf("  [전일 %s, %s]", trillionFromEok(p.Credit.CustomerDepositEok), eok(diff)+"억")
	}
	b.WriteString("- 고객예탁금: " + depositLine + "\n")

	if c.FuturesDepositEok != 0 {
		futuresLine := trillionFromEok(c.FuturesDepositEok)
		if p != nil && p.Credit != nil && p.Credit.FuturesDepositEok != 0 {
			diff := c.FuturesDepositEok - p.Credit.FuturesDepositEok
			futuresLine += fmt.Sprintf("  [전일 %s, %s]", trillionFromEok(p.Credit.FuturesDepositEok), eok(diff)+"억")
		}
		b.WriteString("- 선물예수금: " + futuresLine + "\n")
	}

	// 반대매매 (KOFIA FreeSIS)
	if c.ForcedSellAmountEok > 0 || c.MarginReceivableEok > 0 {
		if c.MarginReceivableEok > 0 {
			marginLine := eok(c.MarginReceivableEok) + "억"
			if p != nil && p.Credit != nil && p.Credit.MarginReceivableEok > 0 {
				diff := c.MarginReceivableEok - p.Credit.MarginReceivableEok
				marginLine += fmt.Sprintf("  [전일 %s억, %s억]", eok(p.Credit.MarginReceivableEok), eok(diff))
			}
			b.WriteString("- 위탁매매 미수금: " + marginLine + "\n")
		}
		forcedLine := eok(c.ForcedSellAmountEok) + "억"
		if p != nil && p.Credit != nil && p.Credit.ForcedSellAmountEok > 0 {
			diff := c.ForcedSellAmountEok - p.Credit.ForcedSellAmountEok
			forcedLine += fmt.Sprintf("  [전일 %s억, %s억]", eok(p.Credit.ForcedSellAmountEok), eok(diff))
		}
		b.WriteString("- 실제 반대매매: " + forcedLine)
		if c.ForcedSellRatioPct > 0 {
			level := forcedSellLevel(c.ForcedSellRatioPct)
			b.WriteString(fmt.Sprintf(" (미수금 대비 %.1f%% %s)", c.ForcedSellRatioPct, level))
		}
		b.WriteString("\n")
	} else if c.Reason != "" {
		b.WriteString("- 반대매매: " + na(c.Reason) + "\n")
	}
	b.WriteString("\n")
}

func forcedSellLevel(pct float64) string {
	switch {
	case pct >= 5.0:
		return "⚠ [높음]"
	case pct >= 3.0:
		return "[주의]"
	default:
		return "[정상]"
	}
}

func renderRegime(b *strings.Builder, s *Snapshot, p *SnapshotJSON) {
	b.WriteString("## 9. 매크로 채널\n")
	r := s.Regime
	if r == nil {
		b.WriteString("- " + na("regime section unavailable") + "\n\n")
		return
	}
	if r.Reason != "" && r.Phase == "" {
		b.WriteString("- " + na(r.Reason) + "\n\n")
		return
	}
	phaseLine := r.Phase
	if p != nil && p.Regime != nil && p.Regime.Phase != "" && p.Regime.Phase != r.Phase {
		phaseLine += fmt.Sprintf("  [전일: %s → %s]", p.Regime.Phase, r.Phase)
	}
	b.WriteString("- 시장 국면: " + phaseLine + "\n")
	b.WriteString("- 디커플링 강도\n")
	if r.KOSPINASDAQCorr != 0 {
		b.WriteString(fmt.Sprintf("  - KOSPI vs NASDAQ 30일 상관계수: %.2f%s\n", r.KOSPINASDAQCorr, corrLevel(r.KOSPINASDAQCorr)))
	}
	if r.KOSPINIKKEICorr != 0 {
		b.WriteString(fmt.Sprintf("  - KOSPI vs NIKKEI 30일 상관계수: %.2f%s\n", r.KOSPINIKKEICorr, corrLevel(r.KOSPINIKKEICorr)))
	}
	riskLine := fmt.Sprintf("%.1f / 10", r.RiskAversionIdx)
	if p != nil && p.Regime != nil {
		diff := r.RiskAversionIdx - p.Regime.RiskAversionIdx
		if diff != 0 {
			riskLine += fmt.Sprintf("  [전일 %.1f, %s]", p.Regime.RiskAversionIdx, signedNumber(diff, 1))
		}
	}
	b.WriteString("- 위험회피 지수: " + riskLine + "\n")
	if r.Reason != "" {
		b.WriteString("- 부분 오류: " + r.Reason + "\n")
	}
	b.WriteString("\n")
}

func corrLevel(corr float64) string {
	switch {
	case corr >= 0.7:
		return " (안정)"
	case corr >= 0.4:
		return " (디커플링 진행)"
	default:
		return " ⚠ (디커플링 심화)"
	}
}

func renderConcentration(b *strings.Builder, s *Snapshot, p *SnapshotJSON) {
	b.WriteString("## 10. 시장 집중도\n")
	c := s.Concentration
	if c == nil {
		b.WriteString("- " + na("concentration section unavailable") + "\n\n")
		return
	}
	if c.Reason != "" && c.HHI == 0 {
		b.WriteString("- " + na(c.Reason) + "\n\n")
		return
	}
	top5Line := fmt.Sprintf("%.1f%%", c.Top5Percent)
	top10Line := fmt.Sprintf("%.1f%%", c.Top10Percent)
	hhiLine := fmt.Sprintf("%.0f [%s]", c.HHI, c.HHILevel)
	if p != nil && p.Concentration != nil {
		if p.Concentration.Top5Percent > 0 {
			diff := c.Top5Percent - p.Concentration.Top5Percent
			top5Line += fmt.Sprintf("  [전일 %.1f%%, %s%%p]", p.Concentration.Top5Percent, signedNumber(diff, 1))
		}
		if p.Concentration.Top10Percent > 0 {
			diff := c.Top10Percent - p.Concentration.Top10Percent
			top10Line += fmt.Sprintf("  [전일 %.1f%%, %s%%p]", p.Concentration.Top10Percent, signedNumber(diff, 1))
		}
		if p.Concentration.HHI > 0 {
			diff := c.HHI - p.Concentration.HHI
			hhiLine += fmt.Sprintf("  [전일 %.0f, %s]", p.Concentration.HHI, signedNumber(diff, 0))
		}
	}
	b.WriteString("- 상위 5종목 시총 비중: " + top5Line + "\n")
	b.WriteString("- 상위 10종목 시총 비중: " + top10Line + "\n")
	b.WriteString("- HHI (시총 기준): " + hhiLine + "\n")
	b.WriteString("  - 임계값: <1,500 비집중, 1,500~2,500 중간, >2,500 고집중\n")
	b.WriteString("- 자기파괴 위험도: " + c.RiskLevel + "\n\n")
}

func renderLateSession(b *strings.Builder, s *Snapshot, p *SnapshotJSON) {
	b.WriteString("## 11. 막판 수급 및 Capitulation 분석\n")
	ls := s.LateSession
	if ls == nil {
		b.WriteString("- " + na(sectionErr(s, "late_session")) + "\n\n")
		return
	}

	// 1) 선물-현물 베이시스
	if ls.SpotPrice > 0 {
		basisStr := fmt.Sprintf("%.2fpt (%.2f%%)", ls.BasisPoint, ls.BasisRate)
		if p != nil && p.LateSession != nil && p.LateSession.SpotPrice > 0 {
			diffPoint := ls.BasisPoint - p.LateSession.BasisPoint
			basisStr += fmt.Sprintf(" [전일 %.2fpt, %spt]", p.LateSession.BasisPoint, signedNumber(diffPoint, 2))
		}
		b.WriteString(fmt.Sprintf("- 선물-현물 베이시스: %s\n", basisStr))
		b.WriteString(fmt.Sprintf("  - 현물(KOSPI 200): %.2f | 선물(최근월): %.2f\n", ls.SpotPrice, ls.FuturesPrice))
	} else {
		b.WriteString("- 선물-현물 베이시스: N/A\n")
	}

	// 2) 프로그램 비차익 당일 누적 수급 (억 원)
	b.WriteString("- 코스피 프로그램 비차익 누적 (억 원):\n")
	b.WriteString(fmt.Sprintf("  - 외국인: %s | 기관: %s | 전체: %s\n", eok(ls.KOSPINetNonArbitrageForeign), eok(ls.KOSPINetNonArbitrageOrgan), eok(ls.KOSPINetNonArbitrageTotal)))

	// 3) 15시 이후 장 막판 흐름
	b.WriteString("- 장 막판 수급 변화 (15:00 ~ 15:30, 억 원):\n")
	b.WriteString(fmt.Sprintf("  - 프로그램 전체 순매수 변화: %s\n", eok(ls.LateProgramNetEok)))

	// 4) 종가 동시호가 중 수급 변화 (15:20 ~ 15:30)
	b.WriteString("- 종가 동시호가 수급 변화 (15:20 ~ 15:30, 억 원):\n")
	b.WriteString(fmt.Sprintf("  - 프로그램 전체 순매수 변화: %s\n", eok(ls.CloseSessionProgramNetEok)))
	if ls.CloseSessionForeignNetEok != 0 || ls.CloseSessionOrganNetEok != 0 {
		b.WriteString(fmt.Sprintf("  - 외국인(시장): %s | 기관(시장): %s\n", eok(ls.CloseSessionForeignNetEok), eok(ls.CloseSessionOrganNetEok)))
	}

	// 5) Capitulation 이벤트 감지 표시
	b.WriteString("- Capitulation 감지 점수: " + fmt.Sprintf("%.1f / 10\n", ls.CapitulationScore))
	if ls.CapitulationEvent {
		b.WriteString("  - **상태**: 🚨 **LATE-SESSION CAPITULATION EVENT DETECTED**\n")
		b.WriteString("  - **설명**: 장중 고점 대비 반등을 시도했으나 장 막판 동시호가에 외국인 및 비차익 프로그램 매도 폭탄이 쏟아져 당일 저가 부근에서 마감하였습니다. 익일 시초가 반대매매 갭다운에 유의하십시오.\n")
	} else {
		b.WriteString("  - **상태**: 정상 (감지되지 않음)\n")
	}
	b.WriteString("\n")
}
