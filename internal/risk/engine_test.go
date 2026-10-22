package risk

import (
	"bearabull/internal/storage"
	"testing"
)

func TestPositionSizingAndRiskLimits(t *testing.T) {
	cfg := DefaultConfig()
	engine := NewEngine(cfg)

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

	proposal := TradeProposal{
		Symbol:      "TCS.NS",
		Direction:   "LONG",
		EntryPrice:  4000.0,
		StopLoss:    3900.0, // ₹100 stop distance (2.5%)
		TargetPrice: 4250.0,
	}

	// 1. Initial Trade Check
	res := engine.EvaluateEntry(proposal, account, nil)
	if !res.Allowed {
		t.Fatalf("expected proposal to be allowed, got error: %s", res.RejectionReason)
	}

	// 0.5% of 1,000,000 = ₹5,000 max risk.
	// Stop distance = ₹100 + slippage + fees ~ ₹108.
	// Expected shares around ~46-49 shares.
	if res.AllocatedShares <= 0 || res.AllocatedShares > 50 {
		t.Errorf("unexpected allocated shares: %d", res.AllocatedShares)
	}

	// Capital required should not exceed 20% of equity (₹2,00,000)
	if res.CapitalRequired > 200000.0 {
		t.Errorf("capital required %v exceeds 20%% allocation limit", res.CapitalRequired)
	}

	// Monetary risk should be <= ₹5,000
	if res.MonetaryRisk > 5050.0 {
		t.Errorf("monetary risk %v exceeds 0.5%% allowed risk", res.MonetaryRisk)
	}

	// 2. Duplicate Position Check
	openPos := []storage.PaperPosition{
		{
			Symbol:     "TCS.NS",
			Shares:     res.AllocatedShares,
			EntryPrice: 4000.0,
			StopLoss:   3900.0,
			Status:     "OPEN",
		},
	}
	resDup := engine.EvaluateEntry(proposal, account, openPos)
	if resDup.Allowed {
		t.Errorf("expected duplicate position in TCS.NS to be rejected")
	}

	// 3. Max Concurrent Positions Limit (Max 5)
	fivePositions := []storage.PaperPosition{
		{Symbol: "S1", EntryPrice: 100, StopLoss: 95, Shares: 10, Status: "OPEN"},
		{Symbol: "S2", EntryPrice: 100, StopLoss: 95, Shares: 10, Status: "OPEN"},
		{Symbol: "S3", EntryPrice: 100, StopLoss: 95, Shares: 10, Status: "OPEN"},
		{Symbol: "S4", EntryPrice: 100, StopLoss: 95, Shares: 10, Status: "OPEN"},
		{Symbol: "S5", EntryPrice: 100, StopLoss: 95, Shares: 10, Status: "OPEN"},
	}
	resMax := engine.EvaluateEntry(proposal, account, fivePositions)
	if resMax.Allowed {
		t.Errorf("expected rejection when 5 positions are already open")
	}

	// 4. Daily Loss Circuit Breaker Test
	lossAccount := account
	lossAccount.DayStartEquity = 1000000.0
	lossAccount.CurrentEquity = 980000.0 // 2% daily loss (exceeds 1.5% limit)

	resBreaker := engine.EvaluateEntry(proposal, lossAccount, nil)
	if resBreaker.Allowed {
		t.Errorf("expected daily loss circuit breaker to block entry")
	}
	if resBreaker.CircuitBreakerTriggered != "DAILY_LOSS_LIMIT" {
		t.Errorf("expected DAILY_LOSS_LIMIT trigger, got %s", resBreaker.CircuitBreakerTriggered)
	}

	// 5. Weekly Drawdown Circuit Breaker Test
	ddAccount := account
	ddAccount.DayStartEquity = 966000.0 // small daily move, but peak weekly was 1000000
	ddAccount.PeakWeeklyEquity = 1000000.0
	ddAccount.CurrentEquity = 965000.0 // 3.5% weekly DD from peak (exceeds 3.0% limit)

	resWDD := engine.EvaluateEntry(proposal, ddAccount, nil)
	if resWDD.Allowed {
		t.Errorf("expected weekly drawdown circuit breaker to block entry")
	}
	if resWDD.CircuitBreakerTriggered != "WEEKLY_DRAWDOWN_LIMIT" {
		t.Errorf("expected WEEKLY_DRAWDOWN_LIMIT trigger, got %s", resWDD.CircuitBreakerTriggered)
	}
}
