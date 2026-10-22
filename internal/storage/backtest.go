package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// SaveBacktestRun persists a backtest execution with trades and equity curve.
func (db *DB) SaveBacktestRun(run *BacktestRun) (int64, error) {
	var runID int64
	curveJSON, _ := json.Marshal(run.EquityCurve)

	err := db.ExecTx(func(tx *sql.Tx) error {
		query := `
		INSERT INTO backtest_runs (
			name, strategy_version, start_date, end_date, initial_capital, final_equity,
			total_return, cagr, max_drawdown, win_rate, profit_factor, sharpe_ratio,
			total_trades, avg_holding_days, benchmark_return, parameters_json, equity_curve_json,
			created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		RETURNING id;
		`
		err := tx.QueryRow(query,
			run.Name, run.StrategyVersion, run.StartDate, run.EndDate, run.InitialCapital, run.FinalEquity,
			run.TotalReturn, run.CAGR, run.MaxDrawdown, run.WinRate, run.ProfitFactor, run.SharpeRatio,
			run.TotalTrades, run.AvgHoldingDays, run.BenchmarkReturn, run.ParametersJSON, string(curveJSON),
		).Scan(&runID)
		if err != nil {
			return fmt.Errorf("failed saving backtest run: %w", err)
		}

		tradeStmt, err := tx.Prepare(`
			INSERT INTO backtest_trades (
				run_id, symbol, entry_date, exit_date, entry_price, exit_price,
				position_type, shares, profit_loss, return_pct, exit_reason
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`)
		if err != nil {
			return err
		}
		defer tradeStmt.Close()

		for _, t := range run.Trades {
			if _, err := tradeStmt.Exec(
				runID, t.Symbol, t.EntryDate, t.ExitDate, t.EntryPrice, t.ExitPrice,
				t.PositionType, t.Shares, t.ProfitLoss, t.ReturnPct, t.ExitReason,
			); err != nil {
				return fmt.Errorf("failed saving trade: %w", err)
			}
		}

		return nil
	})

	run.ID = runID
	return runID, err
}

// GetBacktestRuns returns list of completed backtests.
func (db *DB) GetBacktestRuns(limit int) ([]BacktestRun, error) {
	query := `
		SELECT id, name, strategy_version, start_date, end_date, initial_capital, final_equity,
		       total_return, cagr, max_drawdown, win_rate, profit_factor, sharpe_ratio,
		       total_trades, avg_holding_days, benchmark_return, parameters_json, created_at
		FROM backtest_runs
		ORDER BY id DESC
	`
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []BacktestRun
	for rows.Next() {
		var r BacktestRun
		var createdAt time.Time
		if err := rows.Scan(
			&r.ID, &r.Name, &r.StrategyVersion, &r.StartDate, &r.EndDate, &r.InitialCapital, &r.FinalEquity,
			&r.TotalReturn, &r.CAGR, &r.MaxDrawdown, &r.WinRate, &r.ProfitFactor, &r.SharpeRatio,
			&r.TotalTrades, &r.AvgHoldingDays, &r.BenchmarkReturn, &r.ParametersJSON, &createdAt,
		); err != nil {
			return nil, err
		}
		r.CreatedAt = createdAt
		runs = append(runs, r)
	}

	return runs, rows.Err()
}

// GetBacktestRunDetails retrieves complete backtest details including trades and equity curve.
func (db *DB) GetBacktestRunDetails(id int64) (*BacktestRun, error) {
	query := `
		SELECT id, name, strategy_version, start_date, end_date, initial_capital, final_equity,
		       total_return, cagr, max_drawdown, win_rate, profit_factor, sharpe_ratio,
		       total_trades, avg_holding_days, benchmark_return, parameters_json, equity_curve_json, created_at
		FROM backtest_runs
		WHERE id = ?
	`
	var r BacktestRun
	var curveJSON sql.NullString
	var createdAt time.Time

	err := db.QueryRow(query, id).Scan(
		&r.ID, &r.Name, &r.StrategyVersion, &r.StartDate, &r.EndDate, &r.InitialCapital, &r.FinalEquity,
		&r.TotalReturn, &r.CAGR, &r.MaxDrawdown, &r.WinRate, &r.ProfitFactor, &r.SharpeRatio,
		&r.TotalTrades, &r.AvgHoldingDays, &r.BenchmarkReturn, &r.ParametersJSON, &curveJSON, &createdAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r.CreatedAt = createdAt

	if curveJSON.Valid && curveJSON.String != "" {
		_ = json.Unmarshal([]byte(curveJSON.String), &r.EquityCurve)
	}

	// Fetch trades
	tradeRows, err := db.Query(`
		SELECT t.id, t.run_id, t.symbol, s.company_name, t.entry_date, t.exit_date,
		       t.entry_price, t.exit_price, t.position_type, t.shares, t.profit_loss,
		       t.return_pct, t.exit_reason
		FROM backtest_trades t
		LEFT JOIN stocks s ON t.symbol = s.symbol
		WHERE t.run_id = ?
		ORDER BY t.entry_date ASC
	`, id)
	if err == nil {
		defer tradeRows.Close()
		for tradeRows.Next() {
			var t BacktestTrade
			var comp sql.NullString
			if err := tradeRows.Scan(
				&t.ID, &t.RunID, &t.Symbol, &comp, &t.EntryDate, &t.ExitDate,
				&t.EntryPrice, &t.ExitPrice, &t.PositionType, &t.Shares, &t.ProfitLoss,
				&t.ReturnPct, &t.ExitReason,
			); err == nil {
				if comp.Valid {
					t.CompanyName = comp.String
				}
				r.Trades = append(r.Trades, t)
			}
		}
	}

	return &r, nil
}
