package storage

import (
	"database/sql"
	"fmt"
	"time"
)

// GetAutotraderConfig loads the current single autotrader configuration record.
func (db *DB) GetAutotraderConfig() (*AutotraderConfig, error) {
	query := `
		SELECT 
			id, is_enabled, is_paused, initial_capital, current_cash, current_equity,
			peak_weekly_equity, week_start_equity, day_start_equity,
			last_equity_reset_date, last_equity_reset_week, max_positions,
			max_risk_per_trade_pct, max_aggregate_risk_pct, max_allocation_per_stock_pct,
			max_daily_loss_pct, max_weekly_drawdown_pct, weekly_target_min_pct,
			weekly_target_max_pct, slippage_pct, brokerage_fee_pct, allow_short,
			freshness_threshold_sec, active_strategy_version, updated_at
		FROM autotrader_config WHERE id = 1
	`
	row := db.QueryRow(query)
	var cfg AutotraderConfig
	var enabled, paused, allowShort int
	var updatedAt string

	err := row.Scan(
		&cfg.ID, &enabled, &paused, &cfg.InitialCapital, &cfg.CurrentCash, &cfg.CurrentEquity,
		&cfg.PeakWeeklyEquity, &cfg.WeekStartEquity, &cfg.DayStartEquity,
		&cfg.LastEquityResetDate, &cfg.LastEquityResetWeek, &cfg.MaxPositions,
		&cfg.MaxRiskPerTradePct, &cfg.MaxAggregateRiskPct, &cfg.MaxAllocationPerStockPct,
		&cfg.MaxDailyLossPct, &cfg.MaxWeeklyDrawdownPct, &cfg.WeeklyTargetMinPct,
		&cfg.WeeklyTargetMaxPct, &cfg.SlippagePct, &cfg.BrokerageFeePct, &allowShort,
		&cfg.FreshnessThresholdSec, &cfg.ActiveStrategyVersion, &updatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch autotrader_config: %w", err)
	}

	cfg.IsEnabled = enabled == 1
	cfg.IsPaused = paused == 1
	cfg.AllowShort = allowShort == 1
	cfg.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &cfg, nil
}

// UpdateAutotraderConfig updates the configuration record.
func (db *DB) UpdateAutotraderConfig(cfg *AutotraderConfig) error {
	enabled := 0
	if cfg.IsEnabled {
		enabled = 1
	}
	paused := 0
	if cfg.IsPaused {
		paused = 1
	}
	allowShort := 0
	if cfg.AllowShort {
		allowShort = 1
	}

	query := `
		UPDATE autotrader_config SET
			is_enabled = ?, is_paused = ?, initial_capital = ?, current_cash = ?, current_equity = ?,
			peak_weekly_equity = ?, week_start_equity = ?, day_start_equity = ?,
			last_equity_reset_date = ?, last_equity_reset_week = ?, max_positions = ?,
			max_risk_per_trade_pct = ?, max_aggregate_risk_pct = ?, max_allocation_per_stock_pct = ?,
			max_daily_loss_pct = ?, max_weekly_drawdown_pct = ?, weekly_target_min_pct = ?,
			weekly_target_max_pct = ?, slippage_pct = ?, brokerage_fee_pct = ?, allow_short = ?,
			freshness_threshold_sec = ?, active_strategy_version = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = 1
	`
	_, err := db.Exec(query,
		enabled, paused, cfg.InitialCapital, cfg.CurrentCash, cfg.CurrentEquity,
		cfg.PeakWeeklyEquity, cfg.WeekStartEquity, cfg.DayStartEquity,
		cfg.LastEquityResetDate, cfg.LastEquityResetWeek, cfg.MaxPositions,
		cfg.MaxRiskPerTradePct, cfg.MaxAggregateRiskPct, cfg.MaxAllocationPerStockPct,
		cfg.MaxDailyLossPct, cfg.MaxWeeklyDrawdownPct, cfg.WeeklyTargetMinPct,
		cfg.WeeklyTargetMaxPct, cfg.SlippagePct, cfg.BrokerageFeePct, allowShort,
		cfg.FreshnessThresholdSec, cfg.ActiveStrategyVersion,
	)
	return err
}

// SetAutotraderEnabled enables or disables autonomous trading.
func (db *DB) SetAutotraderEnabled(enabled bool) error {
	val := 0
	if enabled {
		val = 1
	}
	_, err := db.Exec("UPDATE autotrader_config SET is_enabled = ?, updated_at = CURRENT_TIMESTAMP WHERE id = 1", val)
	return err
}

// SetAutotraderPaused pauses or resumes new entries.
func (db *DB) SetAutotraderPaused(paused bool) error {
	val := 0
	if paused {
		val = 1
	}
	_, err := db.Exec("UPDATE autotrader_config SET is_paused = ?, updated_at = CURRENT_TIMESTAMP WHERE id = 1", val)
	return err
}

// ResetDailyWeeklyEquityIfNeeded resets daily and weekly start equity baselines when date/week rolls over.
func (db *DB) ResetDailyWeeklyEquityIfNeeded(todayDate string, currentWeek string) (*AutotraderConfig, error) {
	cfg, err := db.GetAutotraderConfig()
	if err != nil {
		return nil, err
	}

	changed := false
	if cfg.LastEquityResetDate != todayDate {
		cfg.DayStartEquity = cfg.CurrentEquity
		cfg.LastEquityResetDate = todayDate
		changed = true
	}

	if cfg.LastEquityResetWeek != currentWeek {
		cfg.WeekStartEquity = cfg.CurrentEquity
		cfg.PeakWeeklyEquity = cfg.CurrentEquity
		cfg.LastEquityResetWeek = currentWeek
		changed = true
	} else if cfg.CurrentEquity > cfg.PeakWeeklyEquity {
		cfg.PeakWeeklyEquity = cfg.CurrentEquity
		changed = true
	}

	if changed {
		if err := db.UpdateAutotraderConfig(cfg); err != nil {
			return nil, err
		}
	}
	return cfg, nil
}

// CreateAutotraderCycle logs a new cycle start.
func (db *DB) CreateAutotraderCycle(cycle *AutotraderCycle) (int64, error) {
	query := `
		INSERT INTO autotrader_cycles (
			cycle_number, session_date, started_at, status, stocks_scanned,
			shortlisted_count, orders_submitted, orders_filled, errors_count,
			provider_health, summary
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	res, err := db.Exec(query,
		cycle.CycleNumber, cycle.SessionDate, cycle.StartedAt, cycle.Status,
		cycle.StocksScanned, cycle.ShortlistedCount, cycle.OrdersSubmitted,
		cycle.OrdersFilled, cycle.ErrorsCount, cycle.ProviderHealth, cycle.Summary,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateAutotraderCycle updates an existing cycle record upon completion.
func (db *DB) UpdateAutotraderCycle(cycle *AutotraderCycle) error {
	query := `
		UPDATE autotrader_cycles SET
			completed_at = ?, status = ?, stocks_scanned = ?, shortlisted_count = ?,
			orders_submitted = ?, orders_filled = ?, errors_count = ?,
			provider_health = ?, summary = ?
		WHERE id = ?
	`
	_, err := db.Exec(query,
		cycle.CompletedAt, cycle.Status, cycle.StocksScanned, cycle.ShortlistedCount,
		cycle.OrdersSubmitted, cycle.OrdersFilled, cycle.ErrorsCount,
		cycle.ProviderHealth, cycle.Summary, cycle.ID,
	)
	return err
}

// GetLatestAutotraderCycle fetches the most recent cycle run.
func (db *DB) GetLatestAutotraderCycle() (*AutotraderCycle, error) {
	query := `
		SELECT 
			id, cycle_number, session_date, started_at, completed_at, status,
			stocks_scanned, shortlisted_count, orders_submitted, orders_filled,
			errors_count, provider_health, summary, created_at
		FROM autotrader_cycles
		ORDER BY id DESC LIMIT 1
	`
	row := db.QueryRow(query)
	var c AutotraderCycle
	var startedAt, createdAt string
	var completedAt sql.NullString
	var summary sql.NullString

	err := row.Scan(
		&c.ID, &c.CycleNumber, &c.SessionDate, &startedAt, &completedAt, &c.Status,
		&c.StocksScanned, &c.ShortlistedCount, &c.OrdersSubmitted, &c.OrdersFilled,
		&c.ErrorsCount, &c.ProviderHealth, &summary, &createdAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	c.StartedAt, _ = time.Parse(time.RFC3339, startedAt)
	if completedAt.Valid {
		t, _ := time.Parse(time.RFC3339, completedAt.String)
		c.CompletedAt = &t
	}
	c.Summary = summary.String
	c.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	return &c, nil
}

// GetAutotraderCycles lists recent cycles.
func (db *DB) GetAutotraderCycles(limit int) ([]AutotraderCycle, error) {
	if limit <= 0 {
		limit = 20
	}
	query := `
		SELECT 
			id, cycle_number, session_date, started_at, completed_at, status,
			stocks_scanned, shortlisted_count, orders_submitted, orders_filled,
			errors_count, provider_health, summary, created_at
		FROM autotrader_cycles
		ORDER BY id DESC LIMIT ?
	`
	rows, err := db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []AutotraderCycle
	for rows.Next() {
		var c AutotraderCycle
		var startedAt, createdAt string
		var completedAt, summary sql.NullString

		if err := rows.Scan(
			&c.ID, &c.CycleNumber, &c.SessionDate, &startedAt, &completedAt, &c.Status,
			&c.StocksScanned, &c.ShortlistedCount, &c.OrdersSubmitted, &c.OrdersFilled,
			&c.ErrorsCount, &c.ProviderHealth, &summary, &createdAt,
		); err != nil {
			return nil, err
		}
		c.StartedAt, _ = time.Parse(time.RFC3339, startedAt)
		if completedAt.Valid {
			t, _ := time.Parse(time.RFC3339, completedAt.String)
			c.CompletedAt = &t
		}
		c.Summary = summary.String
		c.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		list = append(list, c)
	}
	return list, nil
}

// SaveCycleResults inserts screening results for a cycle.
func (db *DB) SaveCycleResults(results []AutotraderCycleResult) error {
	if len(results) == 0 {
		return nil
	}
	return db.ExecTx(func(tx *sql.Tx) error {
		stmt, err := tx.Prepare(`
			INSERT INTO autotrader_cycle_results (
				cycle_id, symbol, score, bullish_score, bearish_score,
				rank, action, passed_filters, rejection_reason,
				indicators_json, order_block_json
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`)
		if err != nil {
			return err
		}
		defer stmt.Close()

		for _, r := range results {
			passed := 0
			if r.PassedFilters {
				passed = 1
			}
			_, err := stmt.Exec(
				r.CycleID, r.Symbol, r.Score, r.BullishScore, r.BearishScore,
				r.Rank, r.Action, passed, r.RejectionReason,
				r.IndicatorsJSON, r.OrderBlockJSON,
			)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// GetTopOpportunities fetches the latest ranked qualified opportunities.
func (db *DB) GetTopOpportunities(limit int) ([]AutotraderCycleResult, error) {
	if limit <= 0 {
		limit = 5
	}
	query := `
		SELECT 
			r.id, r.cycle_id, r.symbol, r.score, r.bullish_score, r.bearish_score,
			r.rank, r.action, r.passed_filters, r.rejection_reason,
			r.indicators_json, r.order_block_json, r.created_at
		FROM autotrader_cycle_results r
		WHERE r.cycle_id = (SELECT MAX(id) FROM autotrader_cycles WHERE status = 'COMPLETED')
		ORDER BY r.rank ASC
		LIMIT ?
	`
	rows, err := db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []AutotraderCycleResult
	for rows.Next() {
		var r AutotraderCycleResult
		var passed int
		var rej, ind, ob sql.NullString
		var createdAt string

		if err := rows.Scan(
			&r.ID, &r.CycleID, &r.Symbol, &r.Score, &r.BullishScore, &r.BearishScore,
			&r.Rank, &r.Action, &passed, &rej, &ind, &ob, &createdAt,
		); err != nil {
			return nil, err
		}
		r.PassedFilters = passed == 1
		r.RejectionReason = rej.String
		r.IndicatorsJSON = ind.String
		r.OrderBlockJSON = ob.String
		r.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		list = append(list, r)
	}
	return list, nil
}

// SaveDecisions persists decisions made in a cycle.
func (db *DB) SaveDecisions(decisions []AutotraderDecision) error {
	if len(decisions) == 0 {
		return nil
	}
	return db.ExecTx(func(tx *sql.Tx) error {
		stmt, err := tx.Prepare(`
			INSERT INTO autotrader_decisions (
				cycle_id, symbol, decision, current_price, entry_target_price,
				stop_loss, take_profit, shares, planned_risk, rationale,
				risk_checks_passed, rejection_reason
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`)
		if err != nil {
			return err
		}
		defer stmt.Close()

		for _, d := range decisions {
			passed := 0
			if d.RiskChecksPassed {
				passed = 1
			}
			_, err := stmt.Exec(
				d.CycleID, d.Symbol, d.Decision, d.CurrentPrice, d.EntryTargetPrice,
				d.StopLoss, d.TakeProfit, d.Shares, d.PlannedRisk, d.Rationale,
				passed, d.RejectionReason,
			)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// SaveMarketObservation records an observed price for reconciliation.
func (db *DB) SaveMarketObservation(obs *MarketObservation) error {
	stale := 0
	if obs.IsStale {
		stale = 1
	}
	query := `
		INSERT INTO market_observations (
			symbol, observed_at, price, candle_timestamp, interval, source, is_stale, volume
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := db.Exec(query,
		obs.Symbol, obs.ObservedAt, obs.Price, obs.CandleTimestamp,
		obs.Interval, obs.Source, stale, obs.Volume,
	)
	return err
}

// GetLatestObservation returns the newest observation for a symbol.
func (db *DB) GetLatestObservation(symbol string) (*MarketObservation, error) {
	query := `
		SELECT id, symbol, observed_at, price, candle_timestamp, interval, source, is_stale, volume
		FROM market_observations
		WHERE symbol = ?
		ORDER BY observed_at DESC LIMIT 1
	`
	row := db.QueryRow(query, symbol)
	var obs MarketObservation
	var observedAt string
	var stale int

	err := row.Scan(&obs.ID, &obs.Symbol, &observedAt, &obs.Price, &obs.CandleTimestamp, &obs.Interval, &obs.Source, &stale, &obs.Volume)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	obs.ObservedAt, _ = time.Parse(time.RFC3339, observedAt)
	obs.IsStale = stale == 1
	return &obs, nil
}

// CreatePaperPosition creates a new open position.
func (db *DB) CreatePaperPosition(pos *PaperPosition) (int64, error) {
	query := `
		INSERT INTO paper_positions (
			symbol, side, shares, entry_price, entry_time, current_price,
			stop_loss, target_price, status, unrealized_pnl, realized_pnl,
			max_price_seen, min_price_seen
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	res, err := db.Exec(query,
		pos.Symbol, pos.Side, pos.Shares, pos.EntryPrice, pos.EntryTime, pos.CurrentPrice,
		pos.StopLoss, pos.TargetPrice, pos.Status, pos.UnrealizedPnL, pos.RealizedPnL,
		pos.MaxPriceSeen, pos.MinPriceSeen,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdatePaperPosition updates an open or closing position.
func (db *DB) UpdatePaperPosition(pos *PaperPosition) error {
	query := `
		UPDATE paper_positions SET
			current_price = ?, stop_loss = ?, target_price = ?, status = ?,
			unrealized_pnl = ?, realized_pnl = ?, exit_price = ?, exit_time = ?,
			exit_reason = ?, max_price_seen = ?, min_price_seen = ?
		WHERE id = ?
	`
	_, err := db.Exec(query,
		pos.CurrentPrice, pos.StopLoss, pos.TargetPrice, pos.Status,
		pos.UnrealizedPnL, pos.RealizedPnL, pos.ExitPrice, pos.ExitTime,
		pos.ExitReason, pos.MaxPriceSeen, pos.MinPriceSeen, pos.ID,
	)
	return err
}

// GetOpenPaperPositions fetches all currently open positions.
func (db *DB) GetOpenPaperPositions() ([]PaperPosition, error) {
	query := `
		SELECT 
			id, symbol, side, shares, entry_price, entry_time, current_price,
			stop_loss, target_price, status, unrealized_pnl, realized_pnl,
			max_price_seen, min_price_seen, created_at
		FROM paper_positions
		WHERE status = 'OPEN'
		ORDER BY id ASC
	`
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []PaperPosition
	for rows.Next() {
		var p PaperPosition
		var entryTime, createdAt string
		if err := rows.Scan(
			&p.ID, &p.Symbol, &p.Side, &p.Shares, &p.EntryPrice, &entryTime, &p.CurrentPrice,
			&p.StopLoss, &p.TargetPrice, &p.Status, &p.UnrealizedPnL, &p.RealizedPnL,
			&p.MaxPriceSeen, &p.MinPriceSeen, &createdAt,
		); err != nil {
			return nil, err
		}
		p.EntryTime, _ = time.Parse(time.RFC3339, entryTime)
		p.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		list = append(list, p)
	}
	return list, nil
}

// GetPaperPositionBySymbol fetches an open position for a given symbol.
func (db *DB) GetPaperPositionBySymbol(symbol string) (*PaperPosition, error) {
	query := `
		SELECT 
			id, symbol, side, shares, entry_price, entry_time, current_price,
			stop_loss, target_price, status, unrealized_pnl, realized_pnl,
			max_price_seen, min_price_seen, created_at
		FROM paper_positions
		WHERE symbol = ? AND status = 'OPEN'
		LIMIT 1
	`
	row := db.QueryRow(query, symbol)
	var p PaperPosition
	var entryTime, createdAt string

	err := row.Scan(
		&p.ID, &p.Symbol, &p.Side, &p.Shares, &p.EntryPrice, &entryTime, &p.CurrentPrice,
		&p.StopLoss, &p.TargetPrice, &p.Status, &p.UnrealizedPnL, &p.RealizedPnL,
		&p.MaxPriceSeen, &p.MinPriceSeen, &createdAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	p.EntryTime, _ = time.Parse(time.RFC3339, entryTime)
	p.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	return &p, nil
}

// GetPaperPositions lists paper positions with status filter.
func (db *DB) GetPaperPositions(limit int, status string) ([]PaperPosition, error) {
	if limit <= 0 {
		limit = 50
	}
	whereClause := ""
	var args []interface{}
	if status != "" {
		whereClause = "WHERE status = ?"
		args = append(args, status, limit)
	} else {
		args = append(args, limit)
	}

	query := fmt.Sprintf(`
		SELECT 
			id, symbol, side, shares, entry_price, entry_time, current_price,
			stop_loss, target_price, status, unrealized_pnl, realized_pnl,
			exit_price, exit_time, exit_reason, max_price_seen, min_price_seen, created_at
		FROM paper_positions
		%s
		ORDER BY id DESC LIMIT ?
	`, whereClause)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []PaperPosition
	for rows.Next() {
		var p PaperPosition
		var entryTime, createdAt string
		var exitPrice sql.NullFloat64
		var exitTime, exitReason sql.NullString

		if err := rows.Scan(
			&p.ID, &p.Symbol, &p.Side, &p.Shares, &p.EntryPrice, &entryTime, &p.CurrentPrice,
			&p.StopLoss, &p.TargetPrice, &p.Status, &p.UnrealizedPnL, &p.RealizedPnL,
			&exitPrice, &exitTime, &exitReason, &p.MaxPriceSeen, &p.MinPriceSeen, &createdAt,
		); err != nil {
			return nil, err
		}
		p.EntryTime, _ = time.Parse(time.RFC3339, entryTime)
		p.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		if exitPrice.Valid {
			p.ExitPrice = &exitPrice.Float64
		}
		if exitTime.Valid {
			t, _ := time.Parse(time.RFC3339, exitTime.String)
			p.ExitTime = &t
		}
		p.ExitReason = exitReason.String
		list = append(list, p)
	}
	return list, nil
}

// CreatePaperOrder records a new paper order.
func (db *DB) CreatePaperOrder(order *PaperOrder) (int64, error) {
	query := `
		INSERT INTO paper_orders (
			position_id, cycle_id, symbol, order_type, side, shares,
			limit_price, stop_price, fill_price, status, submitted_at,
			filled_at, fees, slippage, notes
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	res, err := db.Exec(query,
		order.PositionID, order.CycleID, order.Symbol, order.OrderType, order.Side, order.Shares,
		order.LimitPrice, order.StopPrice, order.FillPrice, order.Status, order.SubmittedAt,
		order.FilledAt, order.Fees, order.Slippage, order.Notes,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdatePaperOrder updates order status and fills.
func (db *DB) UpdatePaperOrder(order *PaperOrder) error {
	query := `
		UPDATE paper_orders SET
			fill_price = ?, status = ?, filled_at = ?, fees = ?, slippage = ?, notes = ?
		WHERE id = ?
	`
	_, err := db.Exec(query,
		order.FillPrice, order.Status, order.FilledAt, order.Fees, order.Slippage, order.Notes, order.ID,
	)
	return err
}

// GetPendingOrders returns unfilled paper orders.
func (db *DB) GetPendingOrders() ([]PaperOrder, error) {
	query := `
		SELECT 
			id, position_id, cycle_id, symbol, order_type, side, shares,
			limit_price, stop_price, fill_price, status, submitted_at, fees, slippage, notes
		FROM paper_orders
		WHERE status = 'PENDING'
		ORDER BY id ASC
	`
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []PaperOrder
	for rows.Next() {
		var o PaperOrder
		var posID, cycID sql.NullInt64
		var limitP, stopP, fillP sql.NullFloat64
		var subAt string
		var notes sql.NullString

		if err := rows.Scan(
			&o.ID, &posID, &cycID, &o.Symbol, &o.OrderType, &o.Side, &o.Shares,
			&limitP, &stopP, &fillP, &o.Status, &subAt, &o.Fees, &o.Slippage, &notes,
		); err != nil {
			return nil, err
		}
		if posID.Valid {
			o.PositionID = &posID.Int64
		}
		if cycID.Valid {
			o.CycleID = &cycID.Int64
		}
		if limitP.Valid {
			o.LimitPrice = &limitP.Float64
		}
		if stopP.Valid {
			o.StopPrice = &stopP.Float64
		}
		if fillP.Valid {
			o.FillPrice = &fillP.Float64
		}
		o.SubmittedAt, _ = time.Parse(time.RFC3339, subAt)
		o.Notes = notes.String
		list = append(list, o)
	}
	return list, nil
}

// SaveExecutionComparison records trade execution quality metrics.
func (db *DB) SaveExecutionComparison(c *PaperExecutionComparison) error {
	query := `
		INSERT INTO paper_execution_comparisons (
			order_id, position_id, symbol, signal_time, decision_time,
			submission_time, expected_entry_price, simulated_fill_price,
			best_observed_price, first_eligible_observed_price,
			exit_trigger_price, simulated_exit_price,
			entry_deviation_bps, exit_deviation_bps, slippage_cost,
			gross_pnl, net_pnl, max_favorable_excursion, max_adverse_excursion,
			planned_risk, realized_risk, holding_duration_minutes, notes
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := db.Exec(query,
		c.OrderID, c.PositionID, c.Symbol, c.SignalTime, c.DecisionTime,
		c.SubmissionTime, c.ExpectedEntryPrice, c.SimulatedFillPrice,
		c.BestObservedPrice, c.FirstEligibleObservedPrice,
		c.ExitTriggerPrice, c.SimulatedExitPrice,
		c.EntryDeviationBps, c.ExitDeviationBps, c.SlippageCost,
		c.GrossPnL, c.NetPnL, c.MaxFavorableExcursion, c.MaxAdverseExcursion,
		c.PlannedRisk, c.RealizedRisk, c.HoldingDurationMinutes, c.Notes,
	)
	return err
}

// GetExecutionComparisons fetches the history of paper trade execution reconciliations.
func (db *DB) GetExecutionComparisons(limit int) ([]PaperExecutionComparison, error) {
	if limit <= 0 {
		limit = 50
	}
	query := `
		SELECT 
			id, order_id, position_id, symbol, signal_time, decision_time,
			submission_time, expected_entry_price, simulated_fill_price,
			best_observed_price, first_eligible_observed_price,
			exit_trigger_price, simulated_exit_price,
			entry_deviation_bps, exit_deviation_bps, slippage_cost,
			gross_pnl, net_pnl, max_favorable_excursion, max_adverse_excursion,
			planned_risk, realized_risk, holding_duration_minutes, notes, created_at
		FROM paper_execution_comparisons
		ORDER BY id DESC LIMIT ?
	`
	rows, err := db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []PaperExecutionComparison
	for rows.Next() {
		var c PaperExecutionComparison
		var ordID, posID sql.NullInt64
		var sigTime sql.NullString
		var decTime, subTime, createdAt string
		var exitTrigger, simExit sql.NullFloat64
		var notes sql.NullString

		if err := rows.Scan(
			&c.ID, &ordID, &posID, &c.Symbol, &sigTime, &decTime,
			&subTime, &c.ExpectedEntryPrice, &c.SimulatedFillPrice,
			&c.BestObservedPrice, &c.FirstEligibleObservedPrice,
			&exitTrigger, &simExit,
			&c.EntryDeviationBps, &c.ExitDeviationBps, &c.SlippageCost,
			&c.GrossPnL, &c.NetPnL, &c.MaxFavorableExcursion, &c.MaxAdverseExcursion,
			&c.PlannedRisk, &c.RealizedRisk, &c.HoldingDurationMinutes, &notes, &createdAt,
		); err != nil {
			return nil, err
		}
		if ordID.Valid {
			c.OrderID = &ordID.Int64
		}
		if posID.Valid {
			c.PositionID = &posID.Int64
		}
		if sigTime.Valid && sigTime.String != "" {
			t, _ := time.Parse(time.RFC3339, sigTime.String)
			c.SignalTime = &t
		}
		c.DecisionTime, _ = time.Parse(time.RFC3339, decTime)
		c.SubmissionTime, _ = time.Parse(time.RFC3339, subTime)
		if exitTrigger.Valid {
			c.ExitTriggerPrice = &exitTrigger.Float64
		}
		if simExit.Valid {
			c.SimulatedExitPrice = &simExit.Float64
		}
		c.Notes = notes.String
		c.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		list = append(list, c)
	}
	return list, nil
}

// LogRiskEvent records an action or circuit breaker event.
func (db *DB) LogRiskEvent(e *RiskEvent) error {
	query := `
		INSERT INTO risk_events (
			event_type, symbol, severity, message, equity_at_event, details_json
		) VALUES (?, ?, ?, ?, ?, ?)
	`
	_, err := db.Exec(query, e.EventType, e.Symbol, e.Severity, e.Message, e.EquityAtEvent, e.DetailsJSON)
	return err
}

// GetRecentRiskEvents retrieves recent risk warnings and circuit breaker activations.
func (db *DB) GetRecentRiskEvents(limit int) ([]RiskEvent, error) {
	if limit <= 0 {
		limit = 50
	}
	query := `
		SELECT id, event_type, symbol, severity, message, equity_at_event, details_json, created_at
		FROM risk_events
		ORDER BY id DESC LIMIT ?
	`
	rows, err := db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []RiskEvent
	for rows.Next() {
		var e RiskEvent
		var sym, details sql.NullString
		var createdAt string
		if err := rows.Scan(&e.ID, &e.EventType, &sym, &e.Severity, &e.Message, &e.EquityAtEvent, &details, &createdAt); err != nil {
			return nil, err
		}
		e.Symbol = sym.String
		e.DetailsJSON = details.String
		e.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		list = append(list, e)
	}
	return list, nil
}

// Strategy Versions and Experiments

// GetStrategyVersions lists all strategy versions.
func (db *DB) GetStrategyVersions() ([]StrategyVersion, error) {
	query := `SELECT id, version_tag, description, parameters_json, is_active, created_at FROM strategy_versions ORDER BY id DESC`
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []StrategyVersion
	for rows.Next() {
		var v StrategyVersion
		var active int
		var createdAt string
		if err := rows.Scan(&v.ID, &v.VersionTag, &v.Description, &v.ParametersJSON, &active, &createdAt); err != nil {
			return nil, err
		}
		v.IsActive = active == 1
		v.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		list = append(list, v)
	}
	return list, nil
}

// GetStrategyExperiments lists all proposed or approved experiments.
func (db *DB) GetStrategyExperiments() ([]StrategyExperiment, error) {
	query := `
		SELECT id, name, base_version, candidate_version, status, description,
		       parameters_diff_json, training_start, training_end, validation_start, validation_end,
		       created_at, approved_at
		FROM strategy_experiments
		ORDER BY id DESC
	`
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []StrategyExperiment
	for rows.Next() {
		var e StrategyExperiment
		var createdAt string
		var approvedAt sql.NullString
		if err := rows.Scan(
			&e.ID, &e.Name, &e.BaseVersion, &e.CandidateVersion, &e.Status, &e.Description,
			&e.ParametersDiffJSON, &e.TrainingStart, &e.TrainingEnd, &e.ValidationStart, &e.ValidationEnd,
			&createdAt, &approvedAt,
		); err != nil {
			return nil, err
		}
		e.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		if approvedAt.Valid {
			t, _ := time.Parse(time.RFC3339, approvedAt.String)
			e.ApprovedAt = &t
		}
		list = append(list, e)
	}
	return list, nil
}

// GetStrategyExperimentByID retrieves an experiment with its ID.
func (db *DB) GetStrategyExperimentByID(id int64) (*StrategyExperiment, error) {
	query := `
		SELECT id, name, base_version, candidate_version, status, description,
		       parameters_diff_json, training_start, training_end, validation_start, validation_end,
		       created_at, approved_at
		FROM strategy_experiments
		WHERE id = ?
	`
	row := db.QueryRow(query, id)
	var e StrategyExperiment
	var createdAt string
	var approvedAt sql.NullString
	if err := row.Scan(
		&e.ID, &e.Name, &e.BaseVersion, &e.CandidateVersion, &e.Status, &e.Description,
		&e.ParametersDiffJSON, &e.TrainingStart, &e.TrainingEnd, &e.ValidationStart, &e.ValidationEnd,
		&createdAt, &approvedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	e.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	if approvedAt.Valid {
		t, _ := time.Parse(time.RFC3339, approvedAt.String)
		e.ApprovedAt = &t
	}
	return &e, nil
}

// CreateStrategyExperiment stores a proposed optimization experiment.
func (db *DB) CreateStrategyExperiment(e *StrategyExperiment) (int64, error) {
	query := `
		INSERT INTO strategy_experiments (
			name, base_version, candidate_version, status, description,
			parameters_diff_json, training_start, training_end, validation_start, validation_end
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	res, err := db.Exec(query,
		e.Name, e.BaseVersion, e.CandidateVersion, e.Status, e.Description,
		e.ParametersDiffJSON, e.TrainingStart, e.TrainingEnd, e.ValidationStart, e.ValidationEnd,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ApproveStrategyExperiment approves a candidate strategy and activates it in config.
func (db *DB) ApproveStrategyExperiment(id int64) error {
	return db.ExecTx(func(tx *sql.Tx) error {
		var candidateVersion string
		err := tx.QueryRow("SELECT candidate_version FROM strategy_experiments WHERE id = ?", id).Scan(&candidateVersion)
		if err != nil {
			return err
		}

		// Update experiment status
		_, err = tx.Exec("UPDATE strategy_experiments SET status = 'APPROVED', approved_at = CURRENT_TIMESTAMP WHERE id = ?", id)
		if err != nil {
			return err
		}

		// Set other versions inactive and activate candidate
		_, _ = tx.Exec("UPDATE strategy_versions SET is_active = 0")
		_, _ = tx.Exec("UPDATE strategy_versions SET is_active = 1 WHERE version_tag = ?", candidateVersion)

		// Update autotrader config active version
		_, err = tx.Exec("UPDATE autotrader_config SET active_strategy_version = ?, updated_at = CURRENT_TIMESTAMP WHERE id = 1", candidateVersion)
		return err
	})
}

// SaveStrategyEvaluation stores in-sample or out-of-sample evaluation stats.
func (db *DB) SaveStrategyEvaluation(ev *StrategyEvaluation) error {
	query := `
		INSERT INTO strategy_evaluations (
			experiment_id, eval_type, total_trades, win_rate, profit_factor,
			net_return_pct, max_drawdown_pct, expectancy, metrics_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := db.Exec(query,
		ev.ExperimentID, ev.EvalType, ev.TotalTrades, ev.WinRate, ev.ProfitFactor,
		ev.NetReturnPct, ev.MaxDrawdownPct, ev.Expectancy, ev.MetricsJSON,
	)
	return err
}

// GetStrategyEvaluations retrieves all evaluations for an experiment.
func (db *DB) GetStrategyEvaluations(experimentID int64) ([]StrategyEvaluation, error) {
	query := `
		SELECT id, experiment_id, eval_type, total_trades, win_rate, profit_factor,
		       net_return_pct, max_drawdown_pct, expectancy, metrics_json, created_at
		FROM strategy_evaluations
		WHERE experiment_id = ?
		ORDER BY id ASC
	`
	rows, err := db.Query(query, experimentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []StrategyEvaluation
	for rows.Next() {
		var ev StrategyEvaluation
		var createdAt string
		var metrics sql.NullString
		if err := rows.Scan(
			&ev.ID, &ev.ExperimentID, &ev.EvalType, &ev.TotalTrades, &ev.WinRate,
			&ev.ProfitFactor, &ev.NetReturnPct, &ev.MaxDrawdownPct, &ev.Expectancy,
			&metrics, &createdAt,
		); err != nil {
			return nil, err
		}
		ev.MetricsJSON = metrics.String
		ev.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		list = append(list, ev)
	}
	return list, nil
}
