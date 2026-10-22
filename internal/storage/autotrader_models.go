package storage

import "time"

// AutotraderConfig stores configuration and equity circuit-breaker state for the autonomous paper trader.
type AutotraderConfig struct {
	ID                       int       `json:"id"`
	IsEnabled                bool      `json:"is_enabled"`
	IsPaused                 bool      `json:"is_paused"`
	InitialCapital           float64   `json:"initial_capital"`
	CurrentCash              float64   `json:"current_cash"`
	CurrentEquity            float64   `json:"current_equity"`
	PeakWeeklyEquity         float64   `json:"peak_weekly_equity"`
	WeekStartEquity          float64   `json:"week_start_equity"`
	DayStartEquity           float64   `json:"day_start_equity"`
	LastEquityResetDate      string    `json:"last_equity_reset_date"`
	LastEquityResetWeek      string    `json:"last_equity_reset_week"`
	MaxPositions             int       `json:"max_positions"`
	MaxRiskPerTradePct       float64   `json:"max_risk_per_trade_pct"`
	MaxAggregateRiskPct      float64   `json:"max_aggregate_risk_pct"`
	MaxAllocationPerStockPct float64   `json:"max_allocation_per_stock_pct"`
	MaxDailyLossPct          float64   `json:"max_daily_loss_pct"`
	MaxWeeklyDrawdownPct     float64   `json:"max_weekly_drawdown_pct"`
	WeeklyTargetMinPct       float64   `json:"weekly_target_min_pct"`
	WeeklyTargetMaxPct       float64   `json:"weekly_target_max_pct"`
	SlippagePct              float64   `json:"slippage_pct"`
	BrokerageFeePct          float64   `json:"brokerage_fee_pct"`
	AllowShort               bool      `json:"allow_short"`
	FreshnessThresholdSec    int       `json:"freshness_threshold_sec"`
	ActiveStrategyVersion    string    `json:"active_strategy_version"`
	UpdatedAt                time.Time `json:"updated_at"`
}

// AutotraderCycle represents an individual 15-minute autonomous scan & execution run.
type AutotraderCycle struct {
	ID               int64      `json:"id"`
	CycleNumber      int        `json:"cycle_number"`
	SessionDate      string     `json:"session_date"` // YYYY-MM-DD
	StartedAt        time.Time  `json:"started_at"`
	CompletedAt      *time.Time `json:"completed_at,omitempty"`
	Status           string     `json:"status"` // 'RUNNING', 'COMPLETED', 'FAILED', 'SKIPPED_MARKET_CLOSED', 'SKIPPED_STALE_DATA'
	StocksScanned    int        `json:"stocks_scanned"`
	ShortlistedCount int        `json:"shortlisted_count"`
	OrdersSubmitted  int        `json:"orders_submitted"`
	OrdersFilled     int        `json:"orders_filled"`
	ErrorsCount      int        `json:"errors_count"`
	ProviderHealth   string     `json:"provider_health"`
	Summary          string     `json:"summary"`
	CreatedAt        time.Time  `json:"created_at"`
}

// AutotraderCycleResult represents the screening and ranking result for a stock in a cycle.
type AutotraderCycleResult struct {
	ID              int64     `json:"id"`
	CycleID         int64     `json:"cycle_id"`
	Symbol          string    `json:"symbol"`
	Score           float64   `json:"score"`
	BullishScore    float64   `json:"bullish_score"`
	BearishScore    float64   `json:"bearish_score"`
	Rank            int       `json:"rank"`
	Action          string    `json:"action"` // 'QUALIFIED', 'REJECTED', 'WATCH'
	PassedFilters   bool      `json:"passed_filters"`
	RejectionReason string    `json:"rejection_reason,omitempty"`
	IndicatorsJSON  string    `json:"indicators_json,omitempty"`
	OrderBlockJSON  string    `json:"order_block_json,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

// AutotraderDecision represents the discrete action taken for a stock in a cycle.
type AutotraderDecision struct {
	ID               int64     `json:"id"`
	CycleID          int64     `json:"cycle_id"`
	Symbol           string    `json:"symbol"`
	Decision         string    `json:"decision"` // OPEN_LONG, CLOSE_LONG, OPEN_SHORT, CLOSE_SHORT, HOLD, WAIT_FOR_RETEST, WAIT_FOR_CONFIRMATION, SKIP
	CurrentPrice     float64   `json:"current_price"`
	EntryTargetPrice float64   `json:"entry_target_price,omitempty"`
	StopLoss         float64   `json:"stop_loss,omitempty"`
	TakeProfit       float64   `json:"take_profit,omitempty"`
	Shares           int       `json:"shares"`
	PlannedRisk      float64   `json:"planned_risk"`
	Rationale        string    `json:"rationale"`
	RiskChecksPassed bool      `json:"risk_checks_passed"`
	RejectionReason  string    `json:"rejection_reason,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

// MarketObservation stores snapshot observations used for price reconciliation.
type MarketObservation struct {
	ID              int64     `json:"id"`
	Symbol          string    `json:"symbol"`
	ObservedAt      time.Time `json:"observed_at"`
	Price           float64   `json:"price"`
	CandleTimestamp int64     `json:"candle_timestamp"`
	Interval        string    `json:"interval"` // '15m', '1d', etc.
	Source          string    `json:"source"`
	IsStale         bool      `json:"is_stale"`
	Volume          int64     `json:"volume"`
}

// PaperPosition represents an active or closed paper trading position.
type PaperPosition struct {
	ID            int64      `json:"id"`
	Symbol        string     `json:"symbol"`
	Side          string     `json:"side"` // 'LONG' or 'SHORT'
	Shares        int        `json:"shares"`
	EntryPrice    float64    `json:"entry_price"`
	EntryTime     time.Time  `json:"entry_time"`
	CurrentPrice  float64    `json:"current_price"`
	StopLoss      float64    `json:"stop_loss"`
	TargetPrice   float64    `json:"target_price"`
	Status        string     `json:"status"` // 'OPEN' or 'CLOSED'
	UnrealizedPnL float64    `json:"unrealized_pnl"`
	RealizedPnL   float64    `json:"realized_pnl"`
	ExitPrice     *float64   `json:"exit_price,omitempty"`
	ExitTime      *time.Time `json:"exit_time,omitempty"`
	ExitReason    string     `json:"exit_reason,omitempty"`
	MaxPriceSeen  float64    `json:"max_price_seen"`
	MinPriceSeen  float64    `json:"min_price_seen"`
	CreatedAt     time.Time  `json:"created_at"`
}

// PaperOrder represents an order submitted to the paper execution engine.
type PaperOrder struct {
	ID          int64      `json:"id"`
	PositionID  *int64     `json:"position_id,omitempty"`
	CycleID     *int64     `json:"cycle_id,omitempty"`
	Symbol      string     `json:"symbol"`
	OrderType   string     `json:"order_type"` // 'MARKET', 'LIMIT', 'STOP'
	Side        string     `json:"side"`       // 'BUY', 'SELL'
	Shares      int        `json:"shares"`
	LimitPrice  *float64   `json:"limit_price,omitempty"`
	StopPrice   *float64   `json:"stop_price,omitempty"`
	FillPrice   *float64   `json:"fill_price,omitempty"`
	Status      string     `json:"status"` // 'PENDING', 'FILLED', 'CANCELLED', 'REJECTED'
	SubmittedAt time.Time  `json:"submitted_at"`
	FilledAt    *time.Time `json:"filled_at,omitempty"`
	Fees        float64    `json:"fees"`
	Slippage    float64    `json:"slippage"`
	Notes       string     `json:"notes,omitempty"`
}

// PaperExecutionComparison records quality metrics reconciling paper fills against observed market prices.
type PaperExecutionComparison struct {
	ID                         int64      `json:"id"`
	OrderID                    *int64     `json:"order_id,omitempty"`
	PositionID                 *int64     `json:"position_id,omitempty"`
	Symbol                     string     `json:"symbol"`
	SignalTime                 *time.Time `json:"signal_time,omitempty"`
	DecisionTime               time.Time  `json:"decision_time"`
	SubmissionTime             time.Time  `json:"submission_time"`
	ExpectedEntryPrice         float64    `json:"expected_entry_price"`
	SimulatedFillPrice         float64    `json:"simulated_fill_price"`
	BestObservedPrice          float64    `json:"best_observed_price"`
	FirstEligibleObservedPrice float64    `json:"first_eligible_observed_price"`
	ExitTriggerPrice           *float64   `json:"exit_trigger_price,omitempty"`
	SimulatedExitPrice         *float64   `json:"simulated_exit_price,omitempty"`
	EntryDeviationBps          float64    `json:"entry_deviation_bps"`
	ExitDeviationBps           float64    `json:"exit_deviation_bps"`
	SlippageCost               float64    `json:"slippage_cost"`
	GrossPnL                   float64    `json:"gross_pnl"`
	NetPnL                     float64    `json:"net_pnl"`
	MaxFavorableExcursion      float64    `json:"max_favorable_excursion"`
	MaxAdverseExcursion        float64    `json:"max_adverse_excursion"`
	PlannedRisk                float64    `json:"planned_risk"`
	RealizedRisk               float64    `json:"realized_risk"`
	HoldingDurationMinutes     float64    `json:"holding_duration_minutes"`
	Notes                      string     `json:"notes,omitempty"`
	CreatedAt                  time.Time  `json:"created_at"`
}

// RiskEvent logs circuit breakers, margin violations, and risk interventions.
type RiskEvent struct {
	ID            int64     `json:"id"`
	EventType     string    `json:"event_type"` // 'DAILY_LOSS_BREACH', 'WEEKLY_DRAWDOWN_BREACH', 'CONCENTRATION_BREACH', etc.
	Symbol        string    `json:"symbol,omitempty"`
	Severity      string    `json:"severity"` // 'INFO', 'WARNING', 'CRITICAL'
	Message       string    `json:"message"`
	EquityAtEvent float64   `json:"equity_at_event"`
	DetailsJSON   string    `json:"details_json,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// StrategyVersion models a snapshot of versioned strategy parameters.
type StrategyVersion struct {
	ID             int64     `json:"id"`
	VersionTag     string    `json:"version_tag"`
	Description    string    `json:"description"`
	ParametersJSON string    `json:"parameters_json"`
	IsActive       bool      `json:"is_active"`
	CreatedAt      time.Time `json:"created_at"`
}

// StrategyExperiment tracks proposed rule/parameter optimizations.
type StrategyExperiment struct {
	ID                 int64      `json:"id"`
	Name               string     `json:"name"`
	BaseVersion        string     `json:"base_version"`
	CandidateVersion   string     `json:"candidate_version"`
	Status             string     `json:"status"` // 'PROPOSED', 'APPROVED', 'REJECTED'
	Description        string     `json:"description"`
	ParametersDiffJSON string     `json:"parameters_diff_json"`
	TrainingStart      string     `json:"training_start"`
	TrainingEnd        string     `json:"training_end"`
	ValidationStart    string     `json:"validation_start"`
	ValidationEnd      string     `json:"validation_end"`
	CreatedAt          time.Time  `json:"created_at"`
	ApprovedAt         *time.Time `json:"approved_at,omitempty"`
}

// StrategyEvaluation tracks in-sample, out-of-sample, and walk-forward verification of an experiment.
type StrategyEvaluation struct {
	ID             int64     `json:"id"`
	ExperimentID   int64     `json:"experiment_id"`
	EvalType       string    `json:"eval_type"` // 'IN_SAMPLE', 'OUT_OF_SAMPLE', 'WALK_FORWARD'
	TotalTrades    int       `json:"total_trades"`
	WinRate        float64   `json:"win_rate"`
	ProfitFactor   float64   `json:"profit_factor"`
	NetReturnPct   float64   `json:"net_return_pct"`
	MaxDrawdownPct float64   `json:"max_drawdown_pct"`
	Expectancy     float64   `json:"expectancy"`
	MetricsJSON    string    `json:"metrics_json,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

// AutotraderStatusResponse combines configuration, equity, and session status for the UI.
type AutotraderStatusResponse struct {
	Config                AutotraderConfig `json:"config"`
	IsMarketOpen          bool             `json:"is_market_open"`
	MarketSessionStatus   string           `json:"market_session_status"`
	CurrentServerTime     string           `json:"current_server_time"`
	LastScanAt            *time.Time       `json:"last_scan_at,omitempty"`
	NextScanAt            *time.Time       `json:"next_scan_at,omitempty"`
	OpenPositionsCount    int              `json:"open_positions_count"`
	PendingOrdersCount    int              `json:"pending_orders_count"`
	WeeklyReturnPct       float64          `json:"weekly_return_pct"`
	WeeklyTargetMinPct    float64          `json:"weekly_target_min_pct"`
	WeeklyTargetMaxPct    float64          `json:"weekly_target_max_pct"`
	WeeklyRealizedPnL     float64          `json:"weekly_realized_pnl"`
	WeeklyUnrealizedPnL   float64          `json:"weekly_unrealized_pnl"`
	WeeklyDrawdownPct     float64          `json:"weekly_drawdown_pct"`
	DailyPnLPct           float64          `json:"daily_pnl_pct"`
	TotalPlannedRisk      float64          `json:"total_planned_risk"`
	RemainingDailyRiskPct float64          `json:"remaining_daily_risk_pct"`
	ProviderHealth        string           `json:"provider_health"`
	DataFreshnessOK       bool             `json:"data_freshness_ok"`
}
