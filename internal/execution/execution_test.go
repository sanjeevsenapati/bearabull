package execution

import (
	"bearabull/internal/risk"
	"bearabull/internal/screener"
	"bearabull/internal/storage"
	"os"
	"testing"
	"time"
)

func TestDecisionEngine(t *testing.T) {
	riskEng := risk.NewEngine(risk.DefaultConfig())
	decEng := NewDecisionEngine(riskEng)

	// 1. Position evaluation tests
	openPositions := []storage.PaperPosition{
		{
			ID:           1,
			Symbol:       "INFY.NS",
			Side:         "LONG",
			Shares:       50,
			EntryPrice:   1500.0,
			CurrentPrice: 1510.0,
			StopLoss:     1450.0,
			TargetPrice:  1600.0,
			Status:       "OPEN",
		},
		{
			ID:           2,
			Symbol:       "HDFCBANK.NS",
			Side:         "LONG",
			Shares:       40,
			EntryPrice:   1600.0,
			CurrentPrice: 1590.0,
			StopLoss:     1550.0,
			TargetPrice:  1700.0,
			Status:       "OPEN",
		},
	}

	// Case A: INFY hits target (1610 >= 1600), HDFC hits stop loss (1540 <= 1550)
	prices := map[string]float64{
		"INFY.NS":     1610.0,
		"HDFCBANK.NS": 1540.0,
	}

	decisions := decEng.EvaluatePositions(openPositions, prices)
	if len(decisions) != 2 {
		t.Fatalf("expected 2 decisions, got %d", len(decisions))
	}

	if decisions[0].Type != DecisionCloseLong || decisions[0].ExitReason != "TARGET_REACHED" {
		t.Errorf("expected INFY.NS TARGET_REACHED, got %v (%s)", decisions[0].Type, decisions[0].ExitReason)
	}

	if decisions[1].Type != DecisionCloseLong || decisions[1].ExitReason != "STOP_LOSS_HIT" {
		t.Errorf("expected HDFCBANK.NS STOP_LOSS_HIT, got %v (%s)", decisions[1].Type, decisions[1].ExitReason)
	}

	// 2. Candidate evaluation tests
	account := storage.AutotraderConfig{
		InitialCapital:           1000000.0,
		CurrentCash:              1000000.0,
		CurrentEquity:            1000000.0,
		PeakWeeklyEquity:         1000000.0,
		DayStartEquity:           1000000.0,
		MaxPositions:             5,
		MaxRiskPerTradePct:       0.5,
		MaxAggregateRiskPct:      2.0,
		MaxAllocationPerStockPct: 20.0,
		MaxDailyLossPct:          1.5,
		MaxWeeklyDrawdownPct:     3.0,
	}

	candidates := []screener.CandidateOpportunity{
		{
			Symbol:          "TCS.NS",
			CurrentPrice:    3800.0,
			EntryPrice:      3800.0,
			StopLoss:        3700.0,
			TargetPrice:     4050.0,
			Score:           85.0,
			PassedFilters:   true,
			RiskRewardRatio: 2.5,
		},
		{
			Symbol:          "RELIANCE.NS",
			CurrentPrice:    2950.0,
			EntryPrice:      2850.0, // extended by > 1.5%
			StopLoss:        2750.0,
			TargetPrice:     3100.0,
			Score:           78.0,
			PassedFilters:   true,
			RiskRewardRatio: 2.5,
		},
	}

	candDecisions := decEng.EvaluateCandidates(candidates, account, nil, nil)
	if len(candDecisions) != 2 {
		t.Fatalf("expected 2 candidate decisions, got %d", len(candDecisions))
	}

	if candDecisions[0].Type != DecisionOpenLong {
		t.Errorf("expected TCS.NS to be OPEN_LONG, got %v", candDecisions[0].Type)
	}

	if candDecisions[1].Type != DecisionWaitForRetest {
		t.Errorf("expected RELIANCE.NS to be WAIT_FOR_RETEST, got %v", candDecisions[1].Type)
	}
}

func TestPaperExecutor(t *testing.T) {
	testDBPath := "./test_exec.db"
	_ = os.Remove(testDBPath)
	defer os.Remove(testDBPath)

	db, err := storage.Open(testDBPath)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	defer db.Close()

	executor := NewPaperExecutor(db)
	account, err := db.GetAutotraderConfig()
	if err != nil {
		t.Fatalf("failed to get account config: %v", err)
	}

	now := time.Now()
	decisions := []PlannedDecision{
		{
			Symbol:       "WIPRO.NS",
			Type:         DecisionOpenLong,
			CurrentPrice: 500.0,
			EntryPrice:   500.0,
			StopLoss:     480.0,
			TargetPrice:  550.0,
			Shares:       100,
			PlannedRisk:  2000.0,
		},
	}

	observedPrices := map[string]float64{
		"WIPRO.NS": 500.0,
	}

	// 1. Test Execute OPEN_LONG
	res := executor.ExecuteDecisions(1, decisions, account, observedPrices, now)
	if len(res.Errors) > 0 {
		t.Fatalf("unexpected execution errors: %v", res.Errors)
	}
	if res.OrdersFilled != 1 {
		t.Errorf("expected 1 order filled, got %d", res.OrdersFilled)
	}

	openPositions, err := db.GetOpenPaperPositions()
	if err != nil || len(openPositions) != 1 {
		t.Fatalf("expected 1 open position, got %d (err: %v)", len(openPositions), err)
	}
	pos := openPositions[0]
	if pos.Symbol != "WIPRO.NS" || pos.Shares != 100 {
		t.Errorf("unexpected position data: %+v", pos)
	}

	// 2. Test Execute CLOSE_LONG
	posID := pos.ID
	closeDecisions := []PlannedDecision{
		{
			Symbol:       "WIPRO.NS",
			Type:         DecisionCloseLong,
			CurrentPrice: 550.0,
			StopLoss:     480.0,
			TargetPrice:  550.0,
			Shares:       100,
			PositionID:   &posID,
			ExitReason:   "TARGET_REACHED",
		},
	}
	observedPrices["WIPRO.NS"] = 550.0

	resClose := executor.ExecuteDecisions(2, closeDecisions, account, observedPrices, now.Add(2*time.Hour))
	if len(resClose.Errors) > 0 {
		t.Fatalf("unexpected close execution errors: %v", resClose.Errors)
	}

	closedPositions, err := db.GetPaperPositions(10, "CLOSED")
	if err != nil || len(closedPositions) != 1 {
		t.Fatalf("expected 1 closed position, got %d", len(closedPositions))
	}
	if closedPositions[0].RealizedPnL <= 0 {
		t.Errorf("expected positive realized PnL on target reached, got %v", closedPositions[0].RealizedPnL)
	}

	// 3. Verify execution comparison record was saved
	comparisons, err := db.GetExecutionComparisons(10)
	if err != nil || len(comparisons) < 2 {
		t.Errorf("expected at least 2 comparison records, got %d (err: %v)", len(comparisons), err)
	}
}
