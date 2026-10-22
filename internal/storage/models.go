package storage

import "time"

// Stock represents a company in the stock universe.
type Stock struct {
	Symbol      string    `json:"symbol"`       // e.g. "RELIANCE.NS"
	Ticker      string    `json:"ticker"`       // e.g. "RELIANCE"
	CompanyName string    `json:"company_name"` // e.g. "Reliance Industries Ltd"
	Sector      string    `json:"sector"`       // e.g. "Oil & Gas"
	Exchange    string    `json:"exchange"`     // e.g. "NSE"
	IsActive    bool      `json:"is_active"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// DailyPrice represents a single daily OHLCV bar.
type DailyPrice struct {
	Symbol    string    `json:"symbol"`
	Date      string    `json:"date"` // YYYY-MM-DD
	Open      float64   `json:"open"`
	High      float64   `json:"high"`
	Low       float64   `json:"low"`
	Close     float64   `json:"close"`
	AdjClose  float64   `json:"adj_close"`
	Volume    int64     `json:"volume"`
	CreatedAt time.Time `json:"created_at"`
}

// TechnicalIndicators represents calculated indicators for a symbol on a specific date.
type TechnicalIndicators struct {
	Symbol         string    `json:"symbol"`
	Date           string    `json:"date"` // YYYY-MM-DD
	SMA20          float64   `json:"sma_20"`
	SMA50          float64   `json:"sma_50"`
	SMA200         float64   `json:"sma_200"`
	EMA9           float64   `json:"ema_9"`
	EMA21          float64   `json:"ema_21"`
	EMA50          float64   `json:"ema_50"`
	RSI14          float64   `json:"rsi_14"`
	StochK         float64   `json:"stoch_k"`
	StochD         float64   `json:"stoch_d"`
	MACDLine       float64   `json:"macd_line"`
	MACDSignal     float64   `json:"macd_signal"`
	MACDHistogram  float64   `json:"macd_hist"`
	BBUpper        float64   `json:"bb_upper"`
	BBMiddle       float64   `json:"bb_middle"`
	BBLower        float64   `json:"bb_lower"`
	ATR14          float64   `json:"atr_14"`
	Supertrend     float64   `json:"supertrend"`
	SupertrendDir  int       `json:"supertrend_dir"` // 1 for bull, -1 for bear
	ADX14          float64   `json:"adx_14"`
	RVOL           float64   `json:"rvol"`
	OBV            float64   `json:"obv"`
	ROC14          float64   `json:"roc_14"`
	HistVolatility float64   `json:"hist_volatility"`
	SwingHigh      float64   `json:"swing_high"`
	SwingLow       float64   `json:"swing_low"`
	SupportLevel   float64   `json:"support_level"`
	ResistLevel    float64   `json:"resist_level"`
	CreatedAt      time.Time `json:"created_at"`
}

// WeeklySignal represents a calculated signal for a stock.
type WeeklySignal struct {
	ID                int64            `json:"id"`
	Symbol            string           `json:"symbol"`
	CompanyName       string           `json:"company_name,omitempty"`
	Sector            string           `json:"sector,omitempty"`
	SignalDate        string           `json:"signal_date"` // YYYY-MM-DD
	Timeframe         string           `json:"timeframe"`   // "Weekly"
	Signal            string           `json:"signal"`      // "BUY", "SELL", "HOLD", "INSUFFICIENT DATA"
	Score             float64          `json:"score"`       // 0 - 100
	BullishScore      float64          `json:"bullish_score"`
	BearishScore      float64          `json:"bearish_score"`
	TrendClass        string           `json:"trend_class"`      // "Bullish", "Bearish", "Neutral"
	MomentumClass     string           `json:"momentum_class"`   // "Oversold", "Neutral", "Overbought", "Bullish Momentum", "Bearish Momentum"
	VolatilityClass   string           `json:"volatility_class"` // "Low", "Normal", "High"
	EntryZoneLow      float64          `json:"entry_zone_low"`
	EntryZoneHigh     float64          `json:"entry_zone_high"`
	InvalidationLevel float64          `json:"invalidation_level"`
	StopLoss          float64          `json:"stop_loss"`
	RiskReward        float64          `json:"risk_reward"`
	Rationale         string           `json:"rationale"`
	StrategyVersion   string           `json:"strategy_version"`
	BullishEvidence   []SignalEvidence `json:"bullish_evidence,omitempty"`
	BearishEvidence   []SignalEvidence `json:"bearish_evidence,omitempty"`
	LastClose         float64          `json:"last_close,omitempty"`
	WeeklyChangePct   float64          `json:"weekly_change_pct,omitempty"`
	CreatedAt         time.Time        `json:"created_at"`
}

// SignalEvidence holds individual evidence bullets supporting a signal.
type SignalEvidence struct {
	Type        string  `json:"type"` // "BULLISH" or "BEARISH"
	Category    string  `json:"category"`
	Description string  `json:"description"`
	Weight      float64 `json:"weight"`
}

// Watchlist represents a collection of tracked symbols.
type Watchlist struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	Symbols   []string  `json:"symbols,omitempty"`
}

// BacktestRun represents a complete backtest simulation record.
type BacktestRun struct {
	ID              int64           `json:"id"`
	Name            string          `json:"name"`
	StrategyVersion string          `json:"strategy_version"`
	StartDate       string          `json:"start_date"`
	EndDate         string          `json:"end_date"`
	InitialCapital  float64         `json:"initial_capital"`
	FinalEquity     float64         `json:"final_equity"`
	TotalReturn     float64         `json:"total_return"`
	CAGR            float64         `json:"cagr"`
	MaxDrawdown     float64         `json:"max_drawdown"`
	WinRate         float64         `json:"win_rate"`
	ProfitFactor    float64         `json:"profit_factor"`
	SharpeRatio     float64         `json:"sharpe_ratio"`
	TotalTrades     int             `json:"total_trades"`
	AvgHoldingDays  float64         `json:"avg_holding_days"`
	BenchmarkReturn float64         `json:"benchmark_return"` // NIFTY 50 buy & hold
	ParametersJSON  string          `json:"parameters_json"`
	CreatedAt       time.Time       `json:"created_at"`
	Trades          []BacktestTrade `json:"trades,omitempty"`
	EquityCurve     []EquityPoint   `json:"equity_curve,omitempty"`
}

// EquityPoint represents equity on a specific date in a backtest.
type EquityPoint struct {
	Date      string  `json:"date"`
	Equity    float64 `json:"equity"`
	Benchmark float64 `json:"benchmark"`
}

// BacktestTrade represents an executed trade in a backtest.
type BacktestTrade struct {
	ID           int64   `json:"id"`
	RunID        int64   `json:"run_id"`
	Symbol       string  `json:"symbol"`
	CompanyName  string  `json:"company_name,omitempty"`
	EntryDate    string  `json:"entry_date"`
	ExitDate     string  `json:"exit_date"`
	EntryPrice   float64 `json:"entry_price"`
	ExitPrice    float64 `json:"exit_price"`
	PositionType string  `json:"position_type"` // "LONG"
	Shares       int     `json:"shares"`
	ProfitLoss   float64 `json:"profit_loss"`
	ReturnPct    float64 `json:"return_pct"`
	ExitReason   string  `json:"exit_reason"` // "Signal Exit", "Stop Loss", "Target Reached"
}

// DataSyncRun tracks sync executions.
type DataSyncRun struct {
	ID           int64      `json:"id"`
	StartedAt    time.Time  `json:"started_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	Status       string     `json:"status"` // "running", "completed", "failed", "partial"
	StocksSynced int        `json:"stocks_synced"`
	TotalStocks  int        `json:"total_stocks"`
	ErrorMessage string     `json:"error_message,omitempty"`
}
