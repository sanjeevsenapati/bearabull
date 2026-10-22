package analytics

import (
	"bearabull/internal/storage"
	"math"
	"time"
)

// WeeklyMetrics contains comprehensive weekly target tracking data.
type WeeklyMetrics struct {
	WeekLabel                 string  `json:"week_label"`
	StartEquity               float64 `json:"start_equity"`
	CurrentEquity             float64 `json:"current_equity"`
	PeakEquity                float64 `json:"peak_equity"`
	WeeklyReturnPct           float64 `json:"weekly_return_pct"`
	TargetMinPct              float64 `json:"weekly_target_min_pct"` // 1.0%
	TargetMaxPct              float64 `json:"weekly_target_max_pct"` // 10.0%
	DistanceFromLowerPct      float64 `json:"distance_from_lower_pct"`
	DistanceFromUpperPct      float64 `json:"distance_from_upper_pct"`
	TargetStatus              string  `json:"target_status"` // "BELOW_TARGET", "IN_TARGET_RANGE", "EXCEEDED_TARGET"
	RemainingSessions         int     `json:"remaining_sessions"`
	RealizedWeeklyPnL         float64 `json:"realized_weekly_pnl"`
	UnrealizedWeeklyPnL       float64 `json:"unrealized_weekly_pnl"`
	TotalFeesAndSlippage      float64 `json:"total_fees_and_slippage"`
	WeeklyDrawdownPct         float64 `json:"weekly_drawdown_pct"`
	RiskCurrentlyExposed      float64 `json:"risk_currently_exposed"`
	CompletedTradesCount      int     `json:"completed_trades_count"`
	SkippedOpportunitiesCount int     `json:"skipped_opportunities_count"`
	TargetGuidanceNote        string  `json:"target_guidance_note"`
}

// ComputeWeeklyMetrics calculates equity-based weekly target progress.
func ComputeWeeklyMetrics(
	account storage.AutotraderConfig,
	openPositions []storage.PaperPosition,
	closedPositions []storage.PaperPosition,
	comparisons []storage.PaperExecutionComparison,
	skippedCount int,
	now time.Time,
) WeeklyMetrics {
	startEq := account.WeekStartEquity
	if startEq <= 0 {
		startEq = account.InitialCapital
	}
	currEq := account.CurrentEquity
	if currEq <= 0 {
		currEq = startEq
	}
	peakEq := account.PeakWeeklyEquity
	if peakEq < currEq {
		peakEq = currEq
	}

	weeklyReturnPct := 0.0
	if startEq > 0 {
		weeklyReturnPct = math.Round(((currEq-startEq)/startEq)*10000) / 100
	}

	minTarget := account.WeeklyTargetMinPct
	maxTarget := account.WeeklyTargetMaxPct
	if minTarget <= 0 {
		minTarget = 1.0
	}
	if maxTarget <= 0 {
		maxTarget = 10.0
	}

	distLower := math.Round((minTarget-weeklyReturnPct)*100) / 100
	distUpper := math.Round((maxTarget-weeklyReturnPct)*100) / 100

	targetStatus := "BELOW_TARGET"
	guidance := "Maintain standard risk controls. Do not loosen trade filters or chase trades to meet weekly benchmark."
	if weeklyReturnPct >= minTarget && weeklyReturnPct <= maxTarget {
		targetStatus = "IN_TARGET_RANGE"
		guidance = "Weekly target range achieved. Continue managing open positions according to predefined rules without increasing position sizing."
	} else if weeklyReturnPct > maxTarget {
		targetStatus = "EXCEEDED_TARGET"
		guidance = "Upper target exceeded. Preserve capital and adhere strictly to maximum risk constraints."
	}

	// Calculate weekly drawdown from peak
	weeklyDDPct := 0.0
	if peakEq > 0 && currEq < peakEq {
		weeklyDDPct = math.Round(((peakEq-currEq)/peakEq)*10000) / 100
	}

	// Calculate realized & unrealized P&L
	realizedPnL := 0.0
	weekStart := now.AddDate(0, 0, -int(now.Weekday()))
	for _, cp := range closedPositions {
		if cp.ExitTime != nil && cp.ExitTime.After(weekStart) {
			realizedPnL += cp.RealizedPnL
		}
	}

	unrealizedPnL := 0.0
	riskExposed := 0.0
	for _, op := range openPositions {
		unrealizedPnL += op.UnrealizedPnL
		riskPerShare := op.EntryPrice - op.StopLoss
		if riskPerShare > 0 {
			riskExposed += riskPerShare * float64(op.Shares)
		}
	}

	// Fees and slippage from execution comparisons
	totalFeesSlippage := 0.0
	for _, c := range comparisons {
		totalFeesSlippage += c.SlippageCost
	}

	// Remaining sessions in current week (Monday to Friday)
	weekday := int(now.Weekday())
	remainingSessions := 0
	if weekday >= 1 && weekday <= 5 {
		remainingSessions = 5 - weekday
	}

	return WeeklyMetrics{
		WeekLabel:                 now.Format("2006-W02"),
		StartEquity:               math.Round(startEq*100) / 100,
		CurrentEquity:             math.Round(currEq*100) / 100,
		PeakEquity:                math.Round(peakEq*100) / 100,
		WeeklyReturnPct:           weeklyReturnPct,
		TargetMinPct:              minTarget,
		TargetMaxPct:              maxTarget,
		DistanceFromLowerPct:      distLower,
		DistanceFromUpperPct:      distUpper,
		TargetStatus:              targetStatus,
		RemainingSessions:         remainingSessions,
		RealizedWeeklyPnL:         math.Round(realizedPnL*100) / 100,
		UnrealizedWeeklyPnL:       math.Round(unrealizedPnL*100) / 100,
		TotalFeesAndSlippage:      math.Round(totalFeesSlippage*100) / 100,
		WeeklyDrawdownPct:         weeklyDDPct,
		RiskCurrentlyExposed:      math.Round(riskExposed*100) / 100,
		CompletedTradesCount:      len(closedPositions),
		SkippedOpportunitiesCount: skippedCount,
		TargetGuidanceNote:        guidance,
	}
}
