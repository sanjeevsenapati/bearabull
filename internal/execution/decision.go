package execution

import (
	"bearabull/internal/marketdata"
	"bearabull/internal/risk"
	"bearabull/internal/screener"
	"bearabull/internal/storage"
	"fmt"
)

// DecisionType represents an autonomous action.
type DecisionType string

const (
	DecisionOpenLong            DecisionType = "OPEN_LONG"
	DecisionCloseLong           DecisionType = "CLOSE_LONG"
	DecisionOpenShort           DecisionType = "OPEN_SHORT"
	DecisionCloseShort          DecisionType = "CLOSE_SHORT"
	DecisionHold                DecisionType = "HOLD"
	DecisionWaitForRetest       DecisionType = "WAIT_FOR_RETEST"
	DecisionWaitForConfirmation DecisionType = "WAIT_FOR_CONFIRMATION"
	DecisionSkip                DecisionType = "SKIP"
)

// PlannedDecision wraps a proposed execution action.
type PlannedDecision struct {
	Symbol           string
	Type             DecisionType
	CurrentPrice     float64
	EntryPrice       float64
	StopLoss         float64
	TargetPrice      float64
	Shares           int
	PlannedRisk      float64
	Rationale        string
	RiskChecksPassed bool
	RejectionReason  string
	PositionID       *int64
	ExitReason       string
}

// DecisionEngine determines autonomous paper trading actions.
type DecisionEngine struct {
	riskEngine *risk.Engine
}

// NewDecisionEngine creates a decision engine.
func NewDecisionEngine(riskEngine *risk.Engine) *DecisionEngine {
	return &DecisionEngine{riskEngine: riskEngine}
}

// EvaluatePositions checks open positions against current market prices for stop, target, or invalidation.
func (de *DecisionEngine) EvaluatePositions(
	openPositions []storage.PaperPosition,
	currentPrices map[string]float64,
) []PlannedDecision {
	var decisions []PlannedDecision

	for _, pos := range openPositions {
		curPrice, exists := currentPrices[pos.Symbol]
		if !exists || curPrice <= 0 {
			curPrice = pos.CurrentPrice
		}

		// Check Stop Loss hit
		if pos.Side == "LONG" && curPrice <= pos.StopLoss {
			posID := pos.ID
			decisions = append(decisions, PlannedDecision{
				Symbol:           pos.Symbol,
				Type:             DecisionCloseLong,
				CurrentPrice:     curPrice,
				StopLoss:         pos.StopLoss,
				TargetPrice:      pos.TargetPrice,
				Shares:           pos.Shares,
				Rationale:        fmt.Sprintf("Stop-loss triggered: Observed price ₹%.2f breached protective stop ₹%.2f", curPrice, pos.StopLoss),
				RiskChecksPassed: true,
				PositionID:       &posID,
				ExitReason:       "STOP_LOSS_HIT",
			})
			continue
		}

		// Check Target reached
		if pos.Side == "LONG" && curPrice >= pos.TargetPrice {
			posID := pos.ID
			decisions = append(decisions, PlannedDecision{
				Symbol:           pos.Symbol,
				Type:             DecisionCloseLong,
				CurrentPrice:     curPrice,
				StopLoss:         pos.StopLoss,
				TargetPrice:      pos.TargetPrice,
				Shares:           pos.Shares,
				Rationale:        fmt.Sprintf("Technical target achieved: Observed price ₹%.2f reached target ₹%.2f", curPrice, pos.TargetPrice),
				RiskChecksPassed: true,
				PositionID:       &posID,
				ExitReason:       "TARGET_REACHED",
			})
			continue
		}

		// Otherwise HOLD existing position
		posID := pos.ID
		unrealizedPnL := (curPrice - pos.EntryPrice) * float64(pos.Shares)
		decisions = append(decisions, PlannedDecision{
			Symbol:           pos.Symbol,
			Type:             DecisionHold,
			CurrentPrice:     curPrice,
			StopLoss:         pos.StopLoss,
			TargetPrice:      pos.TargetPrice,
			Shares:           pos.Shares,
			Rationale:        fmt.Sprintf("Holding active position: Current price ₹%.2f (Unrealized P&L: ₹%.2f)", curPrice, unrealizedPnL),
			RiskChecksPassed: true,
			PositionID:       &posID,
		})
	}

	return decisions
}

// EvaluateCandidates evaluates qualified opportunities for new entries.
func (de *DecisionEngine) EvaluateCandidates(
	candidates []screener.CandidateOpportunity,
	account storage.AutotraderConfig,
	openPositions []storage.PaperPosition,
	freshnessReports map[string]marketdata.FreshnessReport,
) []PlannedDecision {
	var decisions []PlannedDecision

	openSymbols := make(map[string]bool)
	for _, p := range openPositions {
		if p.Status == "OPEN" {
			openSymbols[p.Symbol] = true
		}
	}

	for _, cand := range candidates {
		// 1. Data Freshness Check
		if rep, ok := freshnessReports[cand.Symbol]; ok && rep.IsStale {
			decisions = append(decisions, PlannedDecision{
				Symbol:           cand.Symbol,
				Type:             DecisionSkip,
				CurrentPrice:     cand.CurrentPrice,
				Rationale:        fmt.Sprintf("Data freshness warning: %s. Entry skipped to avoid execution on stale quotes.", rep.Warning),
				RiskChecksPassed: false,
				RejectionReason:  "DATA_STALE",
			})
			continue
		}

		// 2. Already Open Check
		if openSymbols[cand.Symbol] {
			decisions = append(decisions, PlannedDecision{
				Symbol:           cand.Symbol,
				Type:             DecisionSkip,
				CurrentPrice:     cand.CurrentPrice,
				Rationale:        "Existing position is already active in this symbol",
				RiskChecksPassed: false,
				RejectionReason:  "DUPLICATE_POSITION",
			})
			continue
		}

		// 3. User Paused Check
		if account.IsPaused {
			decisions = append(decisions, PlannedDecision{
				Symbol:           cand.Symbol,
				Type:             DecisionSkip,
				CurrentPrice:     cand.CurrentPrice,
				Rationale:        "New entries are temporarily paused by user operator",
				RiskChecksPassed: false,
				RejectionReason:  "AUTOTRADER_PAUSED",
			})
			continue
		}

		// 4. Filter Qualification Check
		if !cand.PassedFilters {
			decisions = append(decisions, PlannedDecision{
				Symbol:           cand.Symbol,
				Type:             DecisionSkip,
				CurrentPrice:     cand.CurrentPrice,
				Rationale:        fmt.Sprintf("Candidate rejected: %s", cand.RejectionReason),
				RiskChecksPassed: false,
				RejectionReason:  cand.RejectionReason,
			})
			continue
		}

		// 5. Entry Zone Proximity Check
		// If current price is more than 1.5% above recommended entry zone, wait for pullback/retest
		entryDistPct := 0.0
		if cand.EntryPrice > 0 {
			entryDistPct = ((cand.CurrentPrice - cand.EntryPrice) / cand.EntryPrice) * 100
		}

		if entryDistPct > 1.5 {
			decisions = append(decisions, PlannedDecision{
				Symbol:           cand.Symbol,
				Type:             DecisionWaitForRetest,
				CurrentPrice:     cand.CurrentPrice,
				EntryPrice:       cand.EntryPrice,
				StopLoss:         cand.StopLoss,
				TargetPrice:      cand.TargetPrice,
				Rationale:        fmt.Sprintf("Price (₹%.2f) has extended %.2f%% above the order block entry zone (₹%.2f). Waiting for retest.", cand.CurrentPrice, entryDistPct, cand.EntryPrice),
				RiskChecksPassed: true,
			})
			continue
		}

		// 6. Risk Engine Position Sizing & Portfolio Circuit Breakers
		proposal := risk.TradeProposal{
			Symbol:      cand.Symbol,
			Sector:      cand.Sector,
			Direction:   "LONG",
			EntryPrice:  cand.CurrentPrice,
			StopLoss:    cand.StopLoss,
			TargetPrice: cand.TargetPrice,
		}

		riskResult := de.riskEngine.EvaluateEntry(proposal, account, openPositions)
		if !riskResult.Allowed {
			decisions = append(decisions, PlannedDecision{
				Symbol:           cand.Symbol,
				Type:             DecisionSkip,
				CurrentPrice:     cand.CurrentPrice,
				EntryPrice:       cand.CurrentPrice,
				StopLoss:         cand.StopLoss,
				TargetPrice:      cand.TargetPrice,
				Rationale:        fmt.Sprintf("Risk check rejected: %s", riskResult.RejectionReason),
				RiskChecksPassed: false,
				RejectionReason:  riskResult.RejectionReason,
			})
			continue
		}

		// 7. Qualifying OPEN LONG Decision
		decisions = append(decisions, PlannedDecision{
			Symbol:           cand.Symbol,
			Type:             DecisionOpenLong,
			CurrentPrice:     cand.CurrentPrice,
			EntryPrice:       cand.CurrentPrice,
			StopLoss:         cand.StopLoss,
			TargetPrice:      cand.TargetPrice,
			Shares:           riskResult.AllocatedShares,
			PlannedRisk:      riskResult.MonetaryRisk,
			Rationale:        fmt.Sprintf("Qualified setup (Score %.1f): %s Position size: %d shares (₹%.2f risk, %.2f%% allocation).", cand.Score, cand.Rationale, riskResult.AllocatedShares, riskResult.MonetaryRisk, riskResult.CapitalAllocationPct),
			RiskChecksPassed: true,
		})
	}

	return decisions
}
