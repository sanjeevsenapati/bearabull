package storage

import (
	"database/sql"
	"fmt"
	"time"
)

// SaveWeeklySignal persists a weekly signal along with its supporting evidence items.
func (db *DB) SaveWeeklySignal(sig WeeklySignal) (int64, error) {
	var signalID int64
	err := db.ExecTx(func(tx *sql.Tx) error {
		query := `
		INSERT INTO weekly_signals (
			symbol, signal_date, timeframe, signal, score, bullish_score, bearish_score,
			trend_class, momentum_class, volatility_class,
			entry_zone_low, entry_zone_high, invalidation_level, stop_loss, risk_reward,
			rationale, strategy_version, last_close, weekly_change_pct, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(symbol, signal_date, strategy_version) DO UPDATE SET
			signal = excluded.signal,
			score = excluded.score,
			bullish_score = excluded.bullish_score,
			bearish_score = excluded.bearish_score,
			trend_class = excluded.trend_class,
			momentum_class = excluded.momentum_class,
			volatility_class = excluded.volatility_class,
			entry_zone_low = excluded.entry_zone_low,
			entry_zone_high = excluded.entry_zone_high,
			invalidation_level = excluded.invalidation_level,
			stop_loss = excluded.stop_loss,
			risk_reward = excluded.risk_reward,
			rationale = excluded.rationale,
			last_close = excluded.last_close,
			weekly_change_pct = excluded.weekly_change_pct,
			created_at = CURRENT_TIMESTAMP
		RETURNING id;
		`
		err := tx.QueryRow(query,
			sig.Symbol, sig.SignalDate, sig.Timeframe, sig.Signal, sig.Score, sig.BullishScore, sig.BearishScore,
			sig.TrendClass, sig.MomentumClass, sig.VolatilityClass,
			sig.EntryZoneLow, sig.EntryZoneHigh, sig.InvalidationLevel, sig.StopLoss, sig.RiskReward,
			sig.Rationale, sig.StrategyVersion, sig.LastClose, sig.WeeklyChangePct,
		).Scan(&signalID)
		if err != nil {
			return fmt.Errorf("failed saving weekly signal: %w", err)
		}

		// Delete existing evidence for this signal
		_, _ = tx.Exec("DELETE FROM signal_evidence WHERE signal_id = ?", signalID)

		// Insert evidence
		evStmt, err := tx.Prepare("INSERT INTO signal_evidence (signal_id, evidence_type, category, description, weight) VALUES (?, ?, ?, ?, ?)")
		if err != nil {
			return err
		}
		defer evStmt.Close()

		for _, ev := range sig.BullishEvidence {
			if _, err := evStmt.Exec(signalID, "BULLISH", ev.Category, ev.Description, ev.Weight); err != nil {
				return err
			}
		}
		for _, ev := range sig.BearishEvidence {
			if _, err := evStmt.Exec(signalID, "BEARISH", ev.Category, ev.Description, ev.Weight); err != nil {
				return err
			}
		}

		return nil
	})

	return signalID, err
}

// GetLatestWeeklySignal retrieves the most recent weekly signal and evidence for a stock.
func (db *DB) GetLatestWeeklySignal(symbol string) (*WeeklySignal, error) {
	query := `
		SELECT s.id, s.symbol, st.company_name, st.sector, s.signal_date, s.timeframe, s.signal, s.score,
		       s.bullish_score, s.bearish_score, s.trend_class, s.momentum_class, s.volatility_class,
		       s.entry_zone_low, s.entry_zone_high, s.invalidation_level, s.stop_loss, s.risk_reward,
		       s.rationale, s.strategy_version, s.last_close, s.weekly_change_pct, s.created_at
		FROM weekly_signals s
		LEFT JOIN stocks st ON s.symbol = st.symbol
		WHERE s.symbol = ?
		ORDER BY s.signal_date DESC
		LIMIT 1
	`
	var sig WeeklySignal
	var lastClose, weeklyChange sql.NullFloat64
	var createdAt time.Time

	err := db.QueryRow(query, symbol).Scan(
		&sig.ID, &sig.Symbol, &sig.CompanyName, &sig.Sector, &sig.SignalDate, &sig.Timeframe, &sig.Signal, &sig.Score,
		&sig.BullishScore, &sig.BearishScore, &sig.TrendClass, &sig.MomentumClass, &sig.VolatilityClass,
		&sig.EntryZoneLow, &sig.EntryZoneHigh, &sig.InvalidationLevel, &sig.StopLoss, &sig.RiskReward,
		&sig.Rationale, &sig.StrategyVersion, &lastClose, &weeklyChange, &createdAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if lastClose.Valid {
		sig.LastClose = lastClose.Float64
	}
	if weeklyChange.Valid {
		sig.WeeklyChangePct = weeklyChange.Float64
	}
	sig.CreatedAt = createdAt

	// Load evidence
	evidenceRows, err := db.Query("SELECT evidence_type, category, description, weight FROM signal_evidence WHERE signal_id = ? ORDER BY id ASC", sig.ID)
	if err == nil {
		defer evidenceRows.Close()
		for evidenceRows.Next() {
			var ev SignalEvidence
			if err := evidenceRows.Scan(&ev.Type, &ev.Category, &ev.Description, &ev.Weight); err == nil {
				if ev.Type == "BULLISH" {
					sig.BullishEvidence = append(sig.BullishEvidence, ev)
				} else {
					sig.BearishEvidence = append(sig.BearishEvidence, ev)
				}
			}
		}
	}

	return &sig, nil
}

// GetAllLatestWeeklySignals returns the latest signal for every active stock.
func (db *DB) GetAllLatestWeeklySignals() ([]WeeklySignal, error) {
	query := `
		SELECT s.id, s.symbol, st.company_name, st.sector, s.signal_date, s.timeframe, s.signal, s.score,
		       s.bullish_score, s.bearish_score, s.trend_class, s.momentum_class, s.volatility_class,
		       s.entry_zone_low, s.entry_zone_high, s.invalidation_level, s.stop_loss, s.risk_reward,
		       s.rationale, s.strategy_version, s.last_close, s.weekly_change_pct, s.created_at
		FROM weekly_signals s
		INNER JOIN (
			SELECT symbol, MAX(signal_date) AS max_date
			FROM weekly_signals
			GROUP BY symbol
		) latest ON s.symbol = latest.symbol AND s.signal_date = latest.max_date
		LEFT JOIN stocks st ON s.symbol = st.symbol
		WHERE st.is_active = 1
		ORDER BY s.score DESC, s.symbol ASC
	`
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []WeeklySignal
	for rows.Next() {
		var sig WeeklySignal
		var lastClose, weeklyChange sql.NullFloat64
		var createdAt time.Time

		if err := rows.Scan(
			&sig.ID, &sig.Symbol, &sig.CompanyName, &sig.Sector, &sig.SignalDate, &sig.Timeframe, &sig.Signal, &sig.Score,
			&sig.BullishScore, &sig.BearishScore, &sig.TrendClass, &sig.MomentumClass, &sig.VolatilityClass,
			&sig.EntryZoneLow, &sig.EntryZoneHigh, &sig.InvalidationLevel, &sig.StopLoss, &sig.RiskReward,
			&sig.Rationale, &sig.StrategyVersion, &lastClose, &weeklyChange, &createdAt,
		); err != nil {
			return nil, err
		}
		if lastClose.Valid {
			sig.LastClose = lastClose.Float64
		}
		if weeklyChange.Valid {
			sig.WeeklyChangePct = weeklyChange.Float64
		}
		sig.CreatedAt = createdAt
		list = append(list, sig)
	}

	return list, rows.Err()
}

// GetSignalHistory returns historical weekly signals for a symbol.
func (db *DB) GetSignalHistory(symbol string, limit int) ([]WeeklySignal, error) {
	query := `
		SELECT s.id, s.symbol, st.company_name, st.sector, s.signal_date, s.timeframe, s.signal, s.score,
		       s.bullish_score, s.bearish_score, s.trend_class, s.momentum_class, s.volatility_class,
		       s.entry_zone_low, s.entry_zone_high, s.invalidation_level, s.stop_loss, s.risk_reward,
		       s.rationale, s.strategy_version, s.last_close, s.weekly_change_pct, s.created_at
		FROM weekly_signals s
		LEFT JOIN stocks st ON s.symbol = st.symbol
		WHERE s.symbol = ?
		ORDER BY s.signal_date DESC
	`
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	rows, err := db.Query(query, symbol)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []WeeklySignal
	for rows.Next() {
		var sig WeeklySignal
		var lastClose, weeklyChange sql.NullFloat64
		var createdAt time.Time

		if err := rows.Scan(
			&sig.ID, &sig.Symbol, &sig.CompanyName, &sig.Sector, &sig.SignalDate, &sig.Timeframe, &sig.Signal, &sig.Score,
			&sig.BullishScore, &sig.BearishScore, &sig.TrendClass, &sig.MomentumClass, &sig.VolatilityClass,
			&sig.EntryZoneLow, &sig.EntryZoneHigh, &sig.InvalidationLevel, &sig.StopLoss, &sig.RiskReward,
			&sig.Rationale, &sig.StrategyVersion, &lastClose, &weeklyChange, &createdAt,
		); err != nil {
			return nil, err
		}
		if lastClose.Valid {
			sig.LastClose = lastClose.Float64
		}
		if weeklyChange.Valid {
			sig.WeeklyChangePct = weeklyChange.Float64
		}
		sig.CreatedAt = createdAt
		list = append(list, sig)
	}

	return list, rows.Err()
}
