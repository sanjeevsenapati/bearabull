package execution

import (
	"bearabull/internal/storage"
	"fmt"
	"math"
	"time"
)

// ExecutionResult captures orders, fills, and reconciliation records for a cycle.
type ExecutionResult struct {
	OrdersSubmitted int
	OrdersFilled    int
	Errors          []string
}

// PaperExecutor manages virtual order execution, fills, and reconciliation with actual observed prices.
type PaperExecutor struct {
	db *storage.DB
}

// NewPaperExecutor creates an execution engine instance.
func NewPaperExecutor(db *storage.DB) *PaperExecutor {
	return &PaperExecutor{db: db}
}

// ExecuteDecisions processes decisions, submits paper orders, updates holdings, and logs execution comparisons.
func (pe *PaperExecutor) ExecuteDecisions(
	cycleID int64,
	decisions []PlannedDecision,
	account *storage.AutotraderConfig,
	observedQuotes map[string]float64,
	now time.Time,
) ExecutionResult {
	res := ExecutionResult{}

	for _, d := range decisions {
		switch d.Type {
		case DecisionOpenLong:
			if err := pe.executeOpenLong(cycleID, d, account, observedQuotes[d.Symbol], now); err != nil {
				res.Errors = append(res.Errors, fmt.Sprintf("failed executing OPEN_LONG for %s: %v", d.Symbol, err))
			} else {
				res.OrdersSubmitted++
				res.OrdersFilled++
			}

		case DecisionCloseLong:
			if err := pe.executeCloseLong(cycleID, d, account, observedQuotes[d.Symbol], now); err != nil {
				res.Errors = append(res.Errors, fmt.Sprintf("failed executing CLOSE_LONG for %s: %v", d.Symbol, err))
			} else {
				res.OrdersSubmitted++
				res.OrdersFilled++
			}
		}
	}

	// Reconcile equity and cash across all open positions
	if err := pe.ReconcilePortfolio(account, observedQuotes); err != nil {
		res.Errors = append(res.Errors, fmt.Sprintf("reconciliation error: %v", err))
	}

	return res
}

func (pe *PaperExecutor) executeOpenLong(
	cycleID int64,
	d PlannedDecision,
	account *storage.AutotraderConfig,
	observedPrice float64,
	now time.Time,
) error {
	if observedPrice <= 0 {
		observedPrice = d.CurrentPrice
	}

	// 1. Model adverse slippage: Buying pays slightly above observed price
	slippagePct := account.SlippagePct
	fillPrice := math.Round(observedPrice*(1.0+(slippagePct/100.0))*100) / 100
	slippagePerShare := fillPrice - observedPrice

	// 2. Transaction costs: Brokerage + STT + GST + Stamp duty allowance
	totalTradeValue := fillPrice * float64(d.Shares)
	fees := math.Round(totalTradeValue*(account.BrokerageFeePct/100.0)*100) / 100
	totalCost := totalTradeValue + fees

	// 3. Ensure cash is reserved
	if totalCost > account.CurrentCash {
		return fmt.Errorf("insufficient virtual cash (need ₹%.2f, have ₹%.2f)", totalCost, account.CurrentCash)
	}

	// 4. Create Paper Position
	pos := &storage.PaperPosition{
		Symbol:        d.Symbol,
		Side:          "LONG",
		Shares:        d.Shares,
		EntryPrice:    fillPrice,
		EntryTime:     now,
		CurrentPrice:  observedPrice,
		StopLoss:      d.StopLoss,
		TargetPrice:   d.TargetPrice,
		Status:        "OPEN",
		UnrealizedPnL: (observedPrice - fillPrice) * float64(d.Shares),
		RealizedPnL:   0.0,
		MaxPriceSeen:  observedPrice,
		MinPriceSeen:  observedPrice,
	}

	posID, err := pe.db.CreatePaperPosition(pos)
	if err != nil {
		return err
	}

	// 5. Create Paper Order Audit Trail
	order := &storage.PaperOrder{
		PositionID:  &posID,
		CycleID:     &cycleID,
		Symbol:      d.Symbol,
		OrderType:   "MARKET",
		Side:        "BUY",
		Shares:      d.Shares,
		FillPrice:   &fillPrice,
		Status:      "FILLED",
		SubmittedAt: now,
		FilledAt:    &now,
		Fees:        fees,
		Slippage:    math.Round(slippagePerShare*float64(d.Shares)*100) / 100,
		Notes:       fmt.Sprintf("Autonomous paper entry. Slippage: %.2f%%, Fees: ₹%.2f", slippagePct, fees),
	}

	orderID, err := pe.db.CreatePaperOrder(order)
	if err != nil {
		return err
	}

	// 6. Record Execution Comparison vs Observed Market Price (Section 8)
	entryDeviation := fillPrice - observedPrice
	entryDeviationBps := 0.0
	if observedPrice > 0 {
		entryDeviationBps = math.Round((entryDeviation/observedPrice)*10000*10) / 10
	}

	comparison := &storage.PaperExecutionComparison{
		OrderID:                    &orderID,
		PositionID:                 &posID,
		Symbol:                     d.Symbol,
		DecisionTime:               now,
		SubmissionTime:             now,
		ExpectedEntryPrice:         d.EntryPrice,
		SimulatedFillPrice:         fillPrice,
		BestObservedPrice:          observedPrice,
		FirstEligibleObservedPrice: observedPrice,
		EntryDeviationBps:          entryDeviationBps,
		SlippageCost:               math.Round(slippagePerShare*float64(d.Shares)*100) / 100,
		PlannedRisk:                d.PlannedRisk,
		HoldingDurationMinutes:     0,
		Notes:                      fmt.Sprintf("Simulated BUY fill ₹%.2f vs observed market price ₹%.2f (Dev: %.1f bps)", fillPrice, observedPrice, entryDeviationBps),
	}
	_ = pe.db.SaveExecutionComparison(comparison)

	// 7. Deduct Cash
	account.CurrentCash -= totalCost
	return nil
}

func (pe *PaperExecutor) executeCloseLong(
	cycleID int64,
	d PlannedDecision,
	account *storage.AutotraderConfig,
	observedPrice float64,
	now time.Time,
) error {
	if d.PositionID == nil {
		return fmt.Errorf("missing position id for exit")
	}

	pos, err := pe.db.GetPaperPositionBySymbol(d.Symbol)
	if err != nil || pos == nil {
		return fmt.Errorf("position for %s not found: %w", d.Symbol, err)
	}

	if observedPrice <= 0 {
		observedPrice = d.CurrentPrice
	}

	// Model exit slippage & gap adverse execution
	slippagePct := account.SlippagePct
	fillPrice := math.Round(observedPrice*(1.0-(slippagePct/100.0))*100) / 100
	if d.ExitReason == "STOP_LOSS_HIT" && observedPrice < pos.StopLoss {
		// Adverse gap below stop loss: exited at market open/low, not at ideal stop
		fillPrice = math.Round(observedPrice*(1.0-(slippagePct/100.0))*100) / 100
	}

	// Transaction fees on exit
	tradeValue := fillPrice * float64(pos.Shares)
	exitFees := math.Round(tradeValue*(account.BrokerageFeePct/100.0)*100) / 100

	grossPnL := math.Round((fillPrice-pos.EntryPrice)*float64(pos.Shares)*100) / 100
	netPnL := math.Round((grossPnL-exitFees)*100) / 100

	// Update Position
	pos.Status = "CLOSED"
	pos.ExitPrice = &fillPrice
	pos.ExitTime = &now
	pos.ExitReason = d.ExitReason
	pos.RealizedPnL = netPnL
	pos.UnrealizedPnL = 0.0

	if err := pe.db.UpdatePaperPosition(pos); err != nil {
		return err
	}

	// Create Paper Order for Exit
	posID := pos.ID
	order := &storage.PaperOrder{
		PositionID:  &posID,
		CycleID:     &cycleID,
		Symbol:      d.Symbol,
		OrderType:   "MARKET",
		Side:        "SELL",
		Shares:      pos.Shares,
		FillPrice:   &fillPrice,
		Status:      "FILLED",
		SubmittedAt: now,
		FilledAt:    &now,
		Fees:        exitFees,
		Slippage:    math.Round((observedPrice-fillPrice)*float64(pos.Shares)*100) / 100,
		Notes:       fmt.Sprintf("Autonomous paper exit: %s. Net P&L: ₹%.2f", d.ExitReason, netPnL),
	}
	orderID, _ := pe.db.CreatePaperOrder(order)

	// Record Execution Comparison for Exit
	duration := now.Sub(pos.EntryTime).Minutes()
	exitDev := observedPrice - fillPrice
	exitDevBps := 0.0
	if observedPrice > 0 {
		exitDevBps = math.Round((exitDev/observedPrice)*10000*10) / 10
	}

	mfe := math.Max(0.0, (pos.MaxPriceSeen-pos.EntryPrice)*float64(pos.Shares))
	mae := math.Max(0.0, (pos.EntryPrice-pos.MinPriceSeen)*float64(pos.Shares))

	comparison := &storage.PaperExecutionComparison{
		OrderID:                &orderID,
		PositionID:             &posID,
		Symbol:                 pos.Symbol,
		DecisionTime:           now,
		SubmissionTime:         now,
		ExpectedEntryPrice:     pos.EntryPrice,
		SimulatedFillPrice:     pos.EntryPrice,
		BestObservedPrice:      observedPrice,
		SimulatedExitPrice:     &fillPrice,
		ExitTriggerPrice:       &observedPrice,
		ExitDeviationBps:       exitDevBps,
		SlippageCost:           math.Round((observedPrice-fillPrice)*float64(pos.Shares)*100) / 100,
		GrossPnL:               grossPnL,
		NetPnL:                 netPnL,
		MaxFavorableExcursion:  math.Round(mfe*100) / 100,
		MaxAdverseExcursion:    math.Round(mae*100) / 100,
		HoldingDurationMinutes: math.Round(duration*10) / 10,
		Notes:                  fmt.Sprintf("Closed position %s via %s: Net P&L ₹%.2f", pos.Symbol, d.ExitReason, netPnL),
	}
	_ = pe.db.SaveExecutionComparison(comparison)

	// Credit proceeds back to cash
	account.CurrentCash += math.Round((tradeValue-exitFees)*100) / 100
	return nil
}

// ReconcilePortfolio synchronizes open positions, MFE/MAE, unrealized P&L, cash, and account equity.
func (pe *PaperExecutor) ReconcilePortfolio(
	account *storage.AutotraderConfig,
	latestPrices map[string]float64,
) error {
	openPositions, err := pe.db.GetOpenPaperPositions()
	if err != nil {
		return err
	}

	totalPositionValue := 0.0
	for _, pos := range openPositions {
		price, ok := latestPrices[pos.Symbol]
		if ok && price > 0 {
			pos.CurrentPrice = price
			pos.UnrealizedPnL = math.Round((price-pos.EntryPrice)*float64(pos.Shares)*100) / 100
			if price > pos.MaxPriceSeen {
				pos.MaxPriceSeen = price
			}
			if price < pos.MinPriceSeen || pos.MinPriceSeen <= 0 {
				pos.MinPriceSeen = price
			}
			_ = pe.db.UpdatePaperPosition(&pos)
		}
		totalPositionValue += pos.CurrentPrice * float64(pos.Shares)
	}

	// Update account total equity
	account.CurrentEquity = math.Round((account.CurrentCash+totalPositionValue)*100) / 100
	if account.CurrentEquity > account.PeakWeeklyEquity {
		account.PeakWeeklyEquity = account.CurrentEquity
	}

	return pe.db.UpdateAutotraderConfig(account)
}
