package risk

import (
	"bearabull/internal/storage"
	"fmt"
	"math"
)

// Config defines risk limits for the portfolio.
type Config struct {
	InitialCapital           float64 `json:"initial_capital"`
	MaxRiskPerTradePct       float64 `json:"max_risk_per_trade_pct"`       // e.g. 0.5%
	MaxAggregateRiskPct      float64 `json:"max_aggregate_risk_pct"`       // e.g. 2.0%
	MaxAllocationPerStockPct float64 `json:"max_allocation_per_stock_pct"` // e.g. 20.0%
	MaxConcurrentPositions   int     `json:"max_concurrent_positions"`     // e.g. 5
	MaxDailyLossPct          float64 `json:"max_daily_loss_pct"`           // e.g. 1.5%
	MaxWeeklyDrawdownPct     float64 `json:"max_weekly_drawdown_pct"`      // e.g. 3.0%
	SlippagePct              float64 `json:"slippage_pct"`                 // e.g. 0.05%
	BrokerageFeePct          float64 `json:"brokerage_fee_pct"`            // e.g. 0.05%
}

// DefaultConfig returns the default conservative ₹10 Lakh setup.
func DefaultConfig() Config {
	return Config{
		InitialCapital:           1000000.0,
		MaxRiskPerTradePct:       0.5,
		MaxAggregateRiskPct:      2.0,
		MaxAllocationPerStockPct: 20.0,
		MaxConcurrentPositions:   5,
		MaxDailyLossPct:          1.5,
		MaxWeeklyDrawdownPct:     3.0,
		SlippagePct:              0.05,
		BrokerageFeePct:          0.05,
	}
}

// TradeProposal represents a candidate order being evaluated for risk admission.
type TradeProposal struct {
	Symbol      string  `json:"symbol"`
	Sector      string  `json:"sector"`
	Direction   string  `json:"direction"` // "LONG"
	EntryPrice  float64 `json:"entry_price"`
	StopLoss    float64 `json:"stop_loss"`
	TargetPrice float64 `json:"target_price"`
}

// CheckResult details the outcome of risk checks for a proposal.
type CheckResult struct {
	Allowed                 bool               `json:"allowed"`
	RejectionReason         string             `json:"rejection_reason,omitempty"`
	AllocatedShares         int                `json:"allocated_shares"`
	MonetaryRisk            float64            `json:"monetary_risk"`
	StopLossPct             float64            `json:"stop_loss_pct"`
	CapitalRequired         float64            `json:"capital_required"`
	CapitalAllocationPct    float64            `json:"capital_allocation_pct"`
	EstimatedLoss           float64            `json:"estimated_loss"`
	EstimatedFees           float64            `json:"estimated_fees"`
	EstimatedSlippage       float64            `json:"estimated_slippage"`
	NewAggregateRiskPct     float64            `json:"new_aggregate_risk_pct"`
	CircuitBreakerTriggered string             `json:"circuit_breaker_triggered,omitempty"`
	RiskEvent               *storage.RiskEvent `json:"risk_event,omitempty"`
}

// Engine performs position sizing, portfolio constraints validation, and circuit breaker checks.
type Engine struct {
	cfg Config
}

// NewEngine creates a risk management engine.
func NewEngine(cfg Config) *Engine {
	if cfg.InitialCapital <= 0 {
		cfg = DefaultConfig()
	}
	return &Engine{cfg: cfg}
}

// EvaluateEntry assesses whether a new trade can be accepted and computes whole-share position size.
func (e *Engine) EvaluateEntry(
	proposal TradeProposal,
	account storage.AutotraderConfig,
	openPositions []storage.PaperPosition,
) CheckResult {
	res := CheckResult{
		Allowed: false,
	}

	equity := account.CurrentEquity
	if equity <= 0 {
		equity = account.InitialCapital
	}
	cash := account.CurrentCash

	// 1. Check Circuit Breaker: Daily Loss Limit (1.5% of start-of-day equity)
	if account.DayStartEquity > 0 {
		dailyLoss := account.DayStartEquity - equity
		dailyLossPct := (dailyLoss / account.DayStartEquity) * 100
		if dailyLossPct >= e.cfg.MaxDailyLossPct {
			res.CircuitBreakerTriggered = "DAILY_LOSS_LIMIT"
			res.RejectionReason = fmt.Sprintf("Circuit breaker active: Daily loss %.2f%% reached or exceeded limit %.2f%%. New entries blocked for session.", dailyLossPct, e.cfg.MaxDailyLossPct)
			res.RiskEvent = &storage.RiskEvent{
				EventType:     "DAILY_LOSS_BREACH",
				Symbol:        proposal.Symbol,
				Severity:      "CRITICAL",
				Message:       res.RejectionReason,
				EquityAtEvent: equity,
			}
			return res
		}
	}

	// 2. Check Circuit Breaker: Weekly Drawdown Limit (3% from peak weekly equity)
	peakWeekly := account.PeakWeeklyEquity
	if peakWeekly < equity {
		peakWeekly = equity
	}
	if peakWeekly > 0 {
		weeklyDD := peakWeekly - equity
		weeklyDDPct := (weeklyDD / peakWeekly) * 100
		if weeklyDDPct >= e.cfg.MaxWeeklyDrawdownPct {
			res.CircuitBreakerTriggered = "WEEKLY_DRAWDOWN_LIMIT"
			res.RejectionReason = fmt.Sprintf("Circuit breaker active: Weekly drawdown %.2f%% reached or exceeded limit %.2f%% from peak ₹%.2f. New entries suspended.", weeklyDDPct, e.cfg.MaxWeeklyDrawdownPct, peakWeekly)
			res.RiskEvent = &storage.RiskEvent{
				EventType:     "WEEKLY_DRAWDOWN_BREACH",
				Symbol:        proposal.Symbol,
				Severity:      "CRITICAL",
				Message:       res.RejectionReason,
				EquityAtEvent: equity,
			}
			return res
		}
	}

	// 3. Check Concurrent Open Positions Limit (Max 5)
	if len(openPositions) >= e.cfg.MaxConcurrentPositions {
		res.RejectionReason = fmt.Sprintf("Maximum concurrent positions (%d) already open. Position rejected.", e.cfg.MaxConcurrentPositions)
		return res
	}

	// 4. Duplicate Position Check: Avoid duplicate entries into the same stock
	for _, pos := range openPositions {
		if pos.Symbol == proposal.Symbol && pos.Status == "OPEN" {
			res.RejectionReason = fmt.Sprintf("Position in %s is already open. Duplicate entries blocked.", proposal.Symbol)
			return res
		}
	}

	// 5. Calculate Aggregate Risk across existing open positions
	currentAggregateRisk := 0.0
	for _, pos := range openPositions {
		riskPerShare := pos.EntryPrice - pos.StopLoss
		if riskPerShare > 0 {
			currentAggregateRisk += riskPerShare * float64(pos.Shares)
		}
	}

	// 6. Stop Loss & Price Validity Checks
	if proposal.EntryPrice <= 0 || proposal.StopLoss <= 0 {
		res.RejectionReason = "Invalid non-positive entry price or stop loss"
		return res
	}

	if proposal.StopLoss >= proposal.EntryPrice {
		res.RejectionReason = fmt.Sprintf("Stop loss (₹%.2f) must be strictly below entry price (₹%.2f) for long trade", proposal.StopLoss, proposal.EntryPrice)
		return res
	}

	stopDist := proposal.EntryPrice - proposal.StopLoss
	stopDistPct := (stopDist / proposal.EntryPrice) * 100
	res.StopLossPct = math.Round(stopDistPct*100) / 100

	if stopDistPct < 0.5 {
		res.RejectionReason = fmt.Sprintf("Stop distance %.2f%% is too narrow (minimum 0.5%%)", stopDistPct)
		return res
	}
	if stopDistPct > 6.0 {
		res.RejectionReason = fmt.Sprintf("Stop distance %.2f%% exceeds maximum allowed 6.0%%", stopDistPct)
		return res
	}

	// 7. Position Sizing based on Permitted Monetary Risk
	// Max monetary risk per trade = 0.5% of current equity
	maxAllowedRisk := equity * (e.cfg.MaxRiskPerTradePct / 100.0)

	// Loss per share includes stop distance + slippage allowance + roundtrip fees
	slippageAllowance := proposal.EntryPrice * (e.cfg.SlippagePct / 100.0) * 2.0
	feeAllowance := proposal.EntryPrice * (e.cfg.BrokerageFeePct / 100.0) * 2.0
	totalLossPerShare := stopDist + slippageAllowance + feeAllowance

	rawShares := maxAllowedRisk / totalLossPerShare
	shares := int(math.Floor(rawShares))
	if shares < 1 {
		res.RejectionReason = fmt.Sprintf("Calculated position size is 0 shares (loss per share ₹%.2f > max risk ₹%.2f)", totalLossPerShare, maxAllowedRisk)
		return res
	}

	// 8. Capital & Allocation Constraints (Max 20% equity per stock & available cash)
	maxCapitalAllowed := equity * (e.cfg.MaxAllocationPerStockPct / 100.0)
	capitalRequired := float64(shares) * proposal.EntryPrice

	if capitalRequired > maxCapitalAllowed {
		// Conservative whole-share reduction to respect 20% allocation ceiling
		shares = int(math.Floor(maxCapitalAllowed / proposal.EntryPrice))
		capitalRequired = float64(shares) * proposal.EntryPrice
	}

	if capitalRequired > cash {
		// Reduce to fit available virtual cash
		shares = int(math.Floor(cash / proposal.EntryPrice))
		capitalRequired = float64(shares) * proposal.EntryPrice
	}

	if shares < 1 {
		res.RejectionReason = fmt.Sprintf("Insufficient available virtual cash (Need ₹%.2f, Available ₹%.2f)", proposal.EntryPrice, cash)
		return res
	}

	// 9. Aggregate Risk Check across Open Positions + New Position (Max 2% of equity)
	newTradeRisk := float64(shares) * stopDist
	totalProjectedRisk := currentAggregateRisk + newTradeRisk
	projectedRiskPct := (totalProjectedRisk / equity) * 100

	if projectedRiskPct > e.cfg.MaxAggregateRiskPct {
		// Try scaling down shares to fit remaining risk allowance
		remainingRiskBudget := (equity * (e.cfg.MaxAggregateRiskPct / 100.0)) - currentAggregateRisk
		if remainingRiskBudget <= 0 {
			res.RejectionReason = fmt.Sprintf("Aggregate portfolio risk %.2f%% has reached the 2.0%% equity ceiling", (currentAggregateRisk/equity)*100)
			return res
		}
		shares = int(math.Floor(remainingRiskBudget / stopDist))
		if shares < 1 {
			res.RejectionReason = "Remaining aggregate risk budget insufficient for minimum 1 share"
			return res
		}
		newTradeRisk = float64(shares) * stopDist
		totalProjectedRisk = currentAggregateRisk + newTradeRisk
		capitalRequired = float64(shares) * proposal.EntryPrice
	}

	// Final check metrics
	res.Allowed = true
	res.AllocatedShares = shares
	res.MonetaryRisk = math.Round(newTradeRisk*100) / 100
	res.CapitalRequired = math.Round(capitalRequired*100) / 100
	res.CapitalAllocationPct = math.Round((capitalRequired/equity)*10000) / 100
	res.EstimatedLoss = math.Round(float64(shares)*totalLossPerShare*100) / 100
	res.EstimatedFees = math.Round(float64(shares)*feeAllowance*100) / 100
	res.EstimatedSlippage = math.Round(float64(shares)*slippageAllowance*100) / 100
	res.NewAggregateRiskPct = math.Round((totalProjectedRisk/equity)*10000) / 100

	return res
}

// CheckSessionLimits assesses current circuit breaker status for reporting.
func (e *Engine) CheckSessionLimits(account storage.AutotraderConfig) (dailyLossPct float64, weeklyDDPct float64, dailyBlocked bool, weeklyBlocked bool) {
	equity := account.CurrentEquity
	if account.DayStartEquity > 0 {
		dLoss := account.DayStartEquity - equity
		dailyLossPct = (dLoss / account.DayStartEquity) * 100
		if dailyLossPct >= e.cfg.MaxDailyLossPct {
			dailyBlocked = true
		}
	}

	peakWeekly := account.PeakWeeklyEquity
	if peakWeekly < equity {
		peakWeekly = equity
	}
	if peakWeekly > 0 {
		wDD := peakWeekly - equity
		weeklyDDPct = (wDD / peakWeekly) * 100
		if weeklyDDPct >= e.cfg.MaxWeeklyDrawdownPct {
			weeklyBlocked = true
		}
	}
	return
}
