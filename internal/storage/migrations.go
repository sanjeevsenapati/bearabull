package storage

import (
	"database/sql"
	"fmt"
	"log"
)

const schema = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY,
    applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS stocks (
    symbol TEXT PRIMARY KEY,
    ticker TEXT NOT NULL,
    company_name TEXT NOT NULL,
    sector TEXT NOT NULL,
    exchange TEXT NOT NULL DEFAULT 'NSE',
    is_active INTEGER NOT NULL DEFAULT 1,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS daily_prices (
    symbol TEXT NOT NULL,
    trade_date TEXT NOT NULL,
    open REAL NOT NULL,
    high REAL NOT NULL,
    low REAL NOT NULL,
    close REAL NOT NULL,
    adj_close REAL NOT NULL,
    volume INTEGER NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (symbol, trade_date),
    FOREIGN KEY (symbol) REFERENCES stocks(symbol) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_prices_date ON daily_prices(trade_date);
CREATE INDEX IF NOT EXISTS idx_prices_sym_date ON daily_prices(symbol, trade_date DESC);

CREATE TABLE IF NOT EXISTS technical_indicators (
    symbol TEXT NOT NULL,
    trade_date TEXT NOT NULL,
    sma_20 REAL,
    sma_50 REAL,
    sma_200 REAL,
    ema_9 REAL,
    ema_21 REAL,
    ema_50 REAL,
    rsi_14 REAL,
    stoch_k REAL,
    stoch_d REAL,
    macd_line REAL,
    macd_signal REAL,
    macd_hist REAL,
    bb_upper REAL,
    bb_middle REAL,
    bb_lower REAL,
    atr_14 REAL,
    supertrend REAL,
    supertrend_dir INTEGER,
    adx_14 REAL,
    rvol REAL,
    obv REAL,
    roc_14 REAL,
    hist_volatility REAL,
    swing_high REAL,
    swing_low REAL,
    support_level REAL,
    resist_level REAL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (symbol, trade_date),
    FOREIGN KEY (symbol) REFERENCES stocks(symbol) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_indicators_sym_date ON technical_indicators(symbol, trade_date DESC);

CREATE TABLE IF NOT EXISTS weekly_signals (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    symbol TEXT NOT NULL,
    signal_date TEXT NOT NULL,
    timeframe TEXT NOT NULL DEFAULT 'Weekly',
    signal TEXT NOT NULL,
    score REAL NOT NULL,
    bullish_score REAL NOT NULL,
    bearish_score REAL NOT NULL,
    trend_class TEXT NOT NULL,
    momentum_class TEXT NOT NULL,
    volatility_class TEXT NOT NULL,
    entry_zone_low REAL,
    entry_zone_high REAL,
    invalidation_level REAL,
    stop_loss REAL,
    risk_reward REAL,
    rationale TEXT,
    strategy_version TEXT NOT NULL,
    last_close REAL,
    weekly_change_pct REAL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (symbol, signal_date, strategy_version),
    FOREIGN KEY (symbol) REFERENCES stocks(symbol) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_signals_sym_date ON weekly_signals(symbol, signal_date DESC);
CREATE INDEX IF NOT EXISTS idx_signals_date ON weekly_signals(signal_date DESC);

CREATE TABLE IF NOT EXISTS signal_evidence (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    signal_id INTEGER NOT NULL,
    evidence_type TEXT NOT NULL,
    category TEXT NOT NULL,
    description TEXT NOT NULL,
    weight REAL NOT NULL,
    FOREIGN KEY (signal_id) REFERENCES weekly_signals(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_evidence_sig_id ON signal_evidence(signal_id);

CREATE TABLE IF NOT EXISTS watchlists (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS watchlist_stocks (
    watchlist_id INTEGER NOT NULL,
    symbol TEXT NOT NULL,
    added_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (watchlist_id, symbol),
    FOREIGN KEY (watchlist_id) REFERENCES watchlists(id) ON DELETE CASCADE,
    FOREIGN KEY (symbol) REFERENCES stocks(symbol) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS backtest_runs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    strategy_version TEXT NOT NULL,
    start_date TEXT NOT NULL,
    end_date TEXT NOT NULL,
    initial_capital REAL NOT NULL,
    final_equity REAL NOT NULL,
    total_return REAL NOT NULL,
    cagr REAL NOT NULL,
    max_drawdown REAL NOT NULL,
    win_rate REAL NOT NULL,
    profit_factor REAL NOT NULL,
    sharpe_ratio REAL NOT NULL,
    total_trades INTEGER NOT NULL,
    avg_holding_days REAL NOT NULL,
    benchmark_return REAL NOT NULL,
    parameters_json TEXT NOT NULL,
    equity_curve_json TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS backtest_trades (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id INTEGER NOT NULL,
    symbol TEXT NOT NULL,
    entry_date TEXT NOT NULL,
    exit_date TEXT NOT NULL,
    entry_price REAL NOT NULL,
    exit_price REAL NOT NULL,
    position_type TEXT NOT NULL,
    shares INTEGER NOT NULL,
    profit_loss REAL NOT NULL,
    return_pct REAL NOT NULL,
    exit_reason TEXT NOT NULL,
    FOREIGN KEY (run_id) REFERENCES backtest_runs(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_backtest_trades_run ON backtest_trades(run_id);

CREATE TABLE IF NOT EXISTS app_settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS data_sync_runs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    started_at TIMESTAMP NOT NULL,
    completed_at TIMESTAMP,
    status TEXT NOT NULL,
    stocks_synced INTEGER NOT NULL DEFAULT 0,
    total_stocks INTEGER NOT NULL DEFAULT 0,
    error_message TEXT
);
`

const schemaV2 = `
CREATE TABLE IF NOT EXISTS autotrader_config (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    is_enabled INTEGER NOT NULL DEFAULT 0,
    is_paused INTEGER NOT NULL DEFAULT 0,
    initial_capital REAL NOT NULL DEFAULT 1000000.0,
    current_cash REAL NOT NULL DEFAULT 1000000.0,
    current_equity REAL NOT NULL DEFAULT 1000000.0,
    peak_weekly_equity REAL NOT NULL DEFAULT 1000000.0,
    week_start_equity REAL NOT NULL DEFAULT 1000000.0,
    day_start_equity REAL NOT NULL DEFAULT 1000000.0,
    last_equity_reset_date TEXT NOT NULL DEFAULT '',
    last_equity_reset_week TEXT NOT NULL DEFAULT '',
    max_positions INTEGER NOT NULL DEFAULT 5,
    max_risk_per_trade_pct REAL NOT NULL DEFAULT 0.5,
    max_aggregate_risk_pct REAL NOT NULL DEFAULT 2.0,
    max_allocation_per_stock_pct REAL NOT NULL DEFAULT 20.0,
    max_daily_loss_pct REAL NOT NULL DEFAULT 1.5,
    max_weekly_drawdown_pct REAL NOT NULL DEFAULT 3.0,
    weekly_target_min_pct REAL NOT NULL DEFAULT 1.0,
    weekly_target_max_pct REAL NOT NULL DEFAULT 10.0,
    slippage_pct REAL NOT NULL DEFAULT 0.05,
    brokerage_fee_pct REAL NOT NULL DEFAULT 0.05,
    allow_short INTEGER NOT NULL DEFAULT 0,
    freshness_threshold_sec INTEGER NOT NULL DEFAULT 1800,
    active_strategy_version TEXT NOT NULL DEFAULT 'v1.0.0',
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS autotrader_cycles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    cycle_number INTEGER NOT NULL,
    session_date TEXT NOT NULL,
    started_at TIMESTAMP NOT NULL,
    completed_at TIMESTAMP,
    status TEXT NOT NULL,
    stocks_scanned INTEGER NOT NULL DEFAULT 0,
    shortlisted_count INTEGER NOT NULL DEFAULT 0,
    orders_submitted INTEGER NOT NULL DEFAULT 0,
    orders_filled INTEGER NOT NULL DEFAULT 0,
    errors_count INTEGER NOT NULL DEFAULT 0,
    provider_health TEXT NOT NULL DEFAULT 'HEALTHY',
    summary TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_cycles_date ON autotrader_cycles(session_date DESC);
CREATE INDEX IF NOT EXISTS idx_cycles_status ON autotrader_cycles(status);

CREATE TABLE IF NOT EXISTS autotrader_cycle_results (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    cycle_id INTEGER NOT NULL,
    symbol TEXT NOT NULL,
    score REAL NOT NULL,
    bullish_score REAL NOT NULL,
    bearish_score REAL NOT NULL,
    rank INTEGER NOT NULL,
    action TEXT NOT NULL,
    passed_filters INTEGER NOT NULL,
    rejection_reason TEXT,
    indicators_json TEXT,
    order_block_json TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (cycle_id) REFERENCES autotrader_cycles(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_cycle_results_cycle ON autotrader_cycle_results(cycle_id);
CREATE INDEX IF NOT EXISTS idx_cycle_results_sym ON autotrader_cycle_results(symbol);

CREATE TABLE IF NOT EXISTS autotrader_decisions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    cycle_id INTEGER NOT NULL,
    symbol TEXT NOT NULL,
    decision TEXT NOT NULL,
    current_price REAL NOT NULL,
    entry_target_price REAL,
    stop_loss REAL,
    take_profit REAL,
    shares INTEGER NOT NULL DEFAULT 0,
    planned_risk REAL NOT NULL DEFAULT 0.0,
    rationale TEXT,
    risk_checks_passed INTEGER NOT NULL DEFAULT 1,
    rejection_reason TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (cycle_id) REFERENCES autotrader_cycles(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_decisions_cycle ON autotrader_decisions(cycle_id);
CREATE INDEX IF NOT EXISTS idx_decisions_symbol ON autotrader_decisions(symbol);

CREATE TABLE IF NOT EXISTS market_observations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    symbol TEXT NOT NULL,
    observed_at TIMESTAMP NOT NULL,
    price REAL NOT NULL,
    candle_timestamp INTEGER,
    interval TEXT NOT NULL,
    source TEXT NOT NULL,
    is_stale INTEGER NOT NULL DEFAULT 0,
    volume INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_obs_sym_time ON market_observations(symbol, observed_at DESC);

CREATE TABLE IF NOT EXISTS paper_positions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    symbol TEXT NOT NULL,
    side TEXT NOT NULL,
    shares INTEGER NOT NULL,
    entry_price REAL NOT NULL,
    entry_time TIMESTAMP NOT NULL,
    current_price REAL NOT NULL,
    stop_loss REAL NOT NULL,
    target_price REAL NOT NULL,
    status TEXT NOT NULL DEFAULT 'OPEN',
    unrealized_pnl REAL NOT NULL DEFAULT 0.0,
    realized_pnl REAL NOT NULL DEFAULT 0.0,
    exit_price REAL,
    exit_time TIMESTAMP,
    exit_reason TEXT,
    max_price_seen REAL NOT NULL,
    min_price_seen REAL NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_paper_positions_status ON paper_positions(status);
CREATE INDEX IF NOT EXISTS idx_paper_positions_sym ON paper_positions(symbol);

CREATE TABLE IF NOT EXISTS paper_orders (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    position_id INTEGER,
    cycle_id INTEGER,
    symbol TEXT NOT NULL,
    order_type TEXT NOT NULL,
    side TEXT NOT NULL,
    shares INTEGER NOT NULL,
    limit_price REAL,
    stop_price REAL,
    fill_price REAL,
    status TEXT NOT NULL,
    submitted_at TIMESTAMP NOT NULL,
    filled_at TIMESTAMP,
    fees REAL NOT NULL DEFAULT 0.0,
    slippage REAL NOT NULL DEFAULT 0.0,
    notes TEXT
);

CREATE INDEX IF NOT EXISTS idx_paper_orders_pos ON paper_orders(position_id);
CREATE INDEX IF NOT EXISTS idx_paper_orders_status ON paper_orders(status);

CREATE TABLE IF NOT EXISTS paper_execution_comparisons (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    order_id INTEGER,
    position_id INTEGER,
    symbol TEXT NOT NULL,
    signal_time TIMESTAMP,
    decision_time TIMESTAMP NOT NULL,
    submission_time TIMESTAMP NOT NULL,
    expected_entry_price REAL NOT NULL,
    simulated_fill_price REAL NOT NULL,
    best_observed_price REAL NOT NULL,
    first_eligible_observed_price REAL NOT NULL,
    exit_trigger_price REAL,
    simulated_exit_price REAL,
    entry_deviation_bps REAL NOT NULL DEFAULT 0.0,
    exit_deviation_bps REAL NOT NULL DEFAULT 0.0,
    slippage_cost REAL NOT NULL DEFAULT 0.0,
    gross_pnl REAL NOT NULL DEFAULT 0.0,
    net_pnl REAL NOT NULL DEFAULT 0.0,
    max_favorable_excursion REAL NOT NULL DEFAULT 0.0,
    max_adverse_excursion REAL NOT NULL DEFAULT 0.0,
    planned_risk REAL NOT NULL DEFAULT 0.0,
    realized_risk REAL NOT NULL DEFAULT 0.0,
    holding_duration_minutes REAL NOT NULL DEFAULT 0.0,
    notes TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_exec_comp_sym ON paper_execution_comparisons(symbol);

CREATE TABLE IF NOT EXISTS risk_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    event_type TEXT NOT NULL,
    symbol TEXT,
    severity TEXT NOT NULL,
    message TEXT NOT NULL,
    equity_at_event REAL NOT NULL,
    details_json TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_risk_events_created ON risk_events(created_at DESC);

CREATE TABLE IF NOT EXISTS strategy_versions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    version_tag TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL,
    parameters_json TEXT NOT NULL,
    is_active INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS strategy_experiments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    base_version TEXT NOT NULL,
    candidate_version TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'PROPOSED',
    description TEXT NOT NULL,
    parameters_diff_json TEXT NOT NULL,
    training_start TEXT NOT NULL,
    training_end TEXT NOT NULL,
    validation_start TEXT NOT NULL,
    validation_end TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    approved_at TIMESTAMP
);

CREATE TABLE IF NOT EXISTS strategy_evaluations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    experiment_id INTEGER NOT NULL,
    eval_type TEXT NOT NULL,
    total_trades INTEGER NOT NULL,
    win_rate REAL NOT NULL,
    profit_factor REAL NOT NULL,
    net_return_pct REAL NOT NULL,
    max_drawdown_pct REAL NOT NULL,
    expectancy REAL NOT NULL,
    metrics_json TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (experiment_id) REFERENCES strategy_experiments(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_evals_exp ON strategy_evaluations(experiment_id);
`

// Migrate executes schema creation and checks migration state.
func (db *DB) Migrate() error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(schema); err != nil {
		return fmt.Errorf("migration failure v1: %w", err)
	}

	// Insert default watchlist if not exists
	var count int
	err = tx.QueryRow("SELECT COUNT(*) FROM watchlists WHERE name = 'Default'").Scan(&count)
	if err == nil && count == 0 {
		_, _ = tx.Exec("INSERT INTO watchlists (name) VALUES ('Default')")
	}

	// Mark migration version 1
	var vCount int
	_ = tx.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = 1").Scan(&vCount)
	if vCount == 0 {
		_, _ = tx.Exec("INSERT INTO schema_migrations (version) VALUES (1)")
	}

	// Apply migration version 2 (Autotrader, Execution, Risk & Optimization)
	if _, err := tx.Exec(schemaV2); err != nil {
		return fmt.Errorf("migration failure v2: %w", err)
	}

	// Seed default autotrader config if not present
	var cfgCount int
	_ = tx.QueryRow("SELECT COUNT(*) FROM autotrader_config WHERE id = 1").Scan(&cfgCount)
	if cfgCount == 0 {
		_, err = tx.Exec(`
			INSERT INTO autotrader_config (
				id, is_enabled, is_paused, initial_capital, current_cash, current_equity,
				peak_weekly_equity, week_start_equity, day_start_equity, max_positions,
				max_risk_per_trade_pct, max_aggregate_risk_pct, max_allocation_per_stock_pct,
				max_daily_loss_pct, max_weekly_drawdown_pct, weekly_target_min_pct,
				weekly_target_max_pct, slippage_pct, brokerage_fee_pct, allow_short,
				freshness_threshold_sec, active_strategy_version
			) VALUES (
				1, 0, 0, 1000000.0, 1000000.0, 1000000.0,
				1000000.0, 1000000.0, 1000000.0, 5,
				0.5, 2.0, 20.0,
				1.5, 3.0, 1.0,
				10.0, 0.05, 0.05, 0,
				1800, 'v1.0.0'
			)
		`)
		if err != nil {
			return fmt.Errorf("failed to seed autotrader_config: %w", err)
		}
	}

	// Seed default strategy baseline version
	var stratCount int
	_ = tx.QueryRow("SELECT COUNT(*) FROM strategy_versions WHERE version_tag = 'v1.0.0'").Scan(&stratCount)
	if stratCount == 0 {
		_, _ = tx.Exec(`
			INSERT INTO strategy_versions (
				version_tag, description, parameters_json, is_active
			) VALUES (
				'v1.0.0',
				'Baseline SMC + Trend Following (NIFTY 50 15m scan, 0.5% risk/trade)',
				'{"min_score":70.0,"max_risk_per_trade_pct":0.5,"stop_loss_mode":"order_block_invalidation","target_mode":"technical_rr"}',
				1
			)
		`)
	}

	var v2Count int
	_ = tx.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = 2").Scan(&v2Count)
	if v2Count == 0 {
		_, _ = tx.Exec("INSERT INTO schema_migrations (version) VALUES (2)")
	}

	log.Printf("[STORAGE] Schema migration verified successfully (v1 + v2)")
	return tx.Commit()
}

// ExecTx provides a transactional helper.
func (db *DB) ExecTx(fn func(*sql.Tx) error) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}
