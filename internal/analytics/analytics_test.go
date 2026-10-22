package analytics

import (
	"bearabull/internal/storage"
	"testing"
	"time"
)

func TestWeeklyMetricsComputation(t *testing.T) {
	account := storage.AutotraderConfig{
		InitialCapital:     1000000.0,
		WeekStartEquity:    1000000.0,
		CurrentEquity:      1035000.0, // +3.5%
		PeakWeeklyEquity:   1040000.0,
		WeeklyTargetMinPct: 1.0,
		WeeklyTargetMaxPct: 10.0,
	}

	openPositions := []storage.PaperPosition{
		{
			Symbol:        "TCS.NS",
			Shares:        50,
			EntryPrice:    4000.0,
			CurrentPrice:  4100.0,
			StopLoss:      3900.0,
			UnrealizedPnL: 5000.0,
		},
	}

	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC) // Wednesday
	metrics := ComputeWeeklyMetrics(account, openPositions, nil, nil, 2, now)

	if metrics.WeeklyReturnPct != 3.5 {
		t.Errorf("expected weekly return 3.5%%, got %v", metrics.WeeklyReturnPct)
	}

	if metrics.TargetStatus != "IN_TARGET_RANGE" {
		t.Errorf("expected IN_TARGET_RANGE, got %s", metrics.TargetStatus)
	}

	if metrics.DistanceFromLowerPct != -2.5 { // 1.0 - 3.5 = -2.5 (2.5% above min target)
		t.Errorf("expected dist lower -2.5, got %v", metrics.DistanceFromLowerPct)
	}

	if metrics.DistanceFromUpperPct != 6.5 { // 10.0 - 3.5 = 6.5
		t.Errorf("expected dist upper 6.5, got %v", metrics.DistanceFromUpperPct)
	}

	if metrics.RiskCurrentlyExposed <= 0 {
		t.Errorf("expected positive risk exposed, got %v", metrics.RiskCurrentlyExposed)
	}
}

func TestStrategyAnalysisAndWalkForward(t *testing.T) {
	now := time.Now()
	exitTime := now.Add(2 * time.Hour)

	closedTrades := []storage.PaperPosition{
		{Symbol: "RELIANCE.NS", RealizedPnL: 6000.0, EntryTime: now, ExitTime: &exitTime},
		{Symbol: "INFY.NS", RealizedPnL: 4000.0, EntryTime: now, ExitTime: &exitTime},
		{Symbol: "TCS.NS", RealizedPnL: -2500.0, EntryTime: now, ExitTime: &exitTime},
		{Symbol: "HDFCBANK.NS", RealizedPnL: 5000.0, EntryTime: now, ExitTime: &exitTime},
	}

	summary := AnalyzeTrades(closedTrades, nil, 1000000.0)

	if summary.TotalTrades != 4 {
		t.Errorf("expected 4 trades, got %d", summary.TotalTrades)
	}

	if summary.WinRate != 75.0 {
		t.Errorf("expected win rate 75.0%%, got %v", summary.WinRate)
	}

	if summary.ProfitFactor <= 1.0 {
		t.Errorf("expected profit factor > 1.0, got %v", summary.ProfitFactor)
	}

	if summary.Expectancy <= 0 {
		t.Errorf("expected positive expectancy, got %v", summary.Expectancy)
	}

	// Walk-forward validation test
	exp := &storage.StrategyExperiment{
		ID:               1,
		CandidateVersion: "v1.1.0",
	}

	inSample := closedTrades
	outOfSample := []storage.PaperPosition{
		{Symbol: "SBIN.NS", RealizedPnL: 3000.0, EntryTime: now, ExitTime: &exitTime},
		{Symbol: "LT.NS", RealizedPnL: -1000.0, EntryTime: now, ExitTime: &exitTime},
	}

	inEval, outEval, approved := EvaluateExperimentCandidate(exp, inSample, outOfSample, 1000000.0)
	if inEval.TotalTrades != 4 || outEval.TotalTrades != 2 {
		t.Errorf("unexpected evaluation trade counts: in=%d, out=%d", inEval.TotalTrades, outEval.TotalTrades)
	}

	if !approved {
		t.Errorf("expected walk-forward approved for profitable out-of-sample data")
	}
}
