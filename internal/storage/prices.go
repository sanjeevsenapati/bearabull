package storage

import (
	"database/sql"
	"fmt"
	"time"
)

// UpsertDailyPrices inserts or updates daily price records in batches.
func (db *DB) UpsertDailyPrices(prices []DailyPrice) error {
	if len(prices) == 0 {
		return nil
	}

	return db.ExecTx(func(tx *sql.Tx) error {
		stmt, err := tx.Prepare(`
			INSERT INTO daily_prices (symbol, trade_date, open, high, low, close, adj_close, volume, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
			ON CONFLICT(symbol, trade_date) DO UPDATE SET
				open = excluded.open,
				high = excluded.high,
				low = excluded.low,
				close = excluded.close,
				adj_close = excluded.adj_close,
				volume = excluded.volume;
		`)
		if err != nil {
			return err
		}
		defer stmt.Close()

		for _, p := range prices {
			if p.Symbol == "" || p.Date == "" {
				continue
			}
			if _, err := stmt.Exec(p.Symbol, p.Date, p.Open, p.High, p.Low, p.Close, p.AdjClose, p.Volume); err != nil {
				return fmt.Errorf("failed inserting price for %s on %s: %w", p.Symbol, p.Date, err)
			}
		}
		return nil
	})
}

// GetDailyPrices retrieves price history for a symbol sorted chronologically ascending.
func (db *DB) GetDailyPrices(symbol string, limit int) ([]DailyPrice, error) {
	query := `SELECT symbol, trade_date, open, high, low, close, adj_close, volume, created_at 
	          FROM daily_prices WHERE symbol = ? ORDER BY trade_date DESC`
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	rows, err := db.Query(query, symbol)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var descList []DailyPrice
	for rows.Next() {
		var p DailyPrice
		var createdAt time.Time
		if err := rows.Scan(&p.Symbol, &p.Date, &p.Open, &p.High, &p.Low, &p.Close, &p.AdjClose, &p.Volume, &createdAt); err != nil {
			return nil, err
		}
		p.CreatedAt = createdAt
		descList = append(descList, p)
	}

	// Reverse to chronological order (oldest to newest)
	n := len(descList)
	result := make([]DailyPrice, n)
	for i, p := range descList {
		result[n-1-i] = p
	}
	return result, rows.Err()
}

// GetLatestPrice returns the most recent daily price candle for a symbol.
func (db *DB) GetLatestPrice(symbol string) (*DailyPrice, error) {
	query := `SELECT symbol, trade_date, open, high, low, close, adj_close, volume, created_at 
	          FROM daily_prices WHERE symbol = ? ORDER BY trade_date DESC LIMIT 1`
	var p DailyPrice
	var createdAt time.Time
	err := db.QueryRow(query, symbol).Scan(
		&p.Symbol, &p.Date, &p.Open, &p.High, &p.Low, &p.Close, &p.AdjClose, &p.Volume, &createdAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p.CreatedAt = createdAt
	return &p, nil
}

// GetAllLatestPrices returns the latest price record for each active stock.
func (db *DB) GetAllLatestPrices() (map[string]DailyPrice, error) {
	query := `
		SELECT p.symbol, p.trade_date, p.open, p.high, p.low, p.close, p.adj_close, p.volume, p.created_at
		FROM daily_prices p
		INNER JOIN (
			SELECT symbol, MAX(trade_date) AS max_date
			FROM daily_prices
			GROUP BY symbol
		) latest ON p.symbol = latest.symbol AND p.trade_date = latest.max_date
	`
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := make(map[string]DailyPrice)
	for rows.Next() {
		var p DailyPrice
		var createdAt time.Time
		if err := rows.Scan(&p.Symbol, &p.Date, &p.Open, &p.High, &p.Low, &p.Close, &p.AdjClose, &p.Volume, &createdAt); err != nil {
			return nil, err
		}
		p.CreatedAt = createdAt
		res[p.Symbol] = p
	}
	return res, rows.Err()
}

// UpsertTechnicalIndicators persists technical indicator results for a symbol and date.
func (db *DB) UpsertTechnicalIndicators(ind TechnicalIndicators) error {
	query := `
	INSERT INTO technical_indicators (
		symbol, trade_date, sma_20, sma_50, sma_200, ema_9, ema_21, ema_50,
		rsi_14, stoch_k, stoch_d, macd_line, macd_signal, macd_hist,
		bb_upper, bb_middle, bb_lower, atr_14, supertrend, supertrend_dir,
		adx_14, rvol, obv, roc_14, hist_volatility, swing_high, swing_low,
		support_level, resist_level, created_at
	) VALUES (
		?, ?, ?, ?, ?, ?, ?, ?,
		?, ?, ?, ?, ?, ?,
		?, ?, ?, ?, ?, ?,
		?, ?, ?, ?, ?, ?, ?,
		?, ?, CURRENT_TIMESTAMP
	) ON CONFLICT(symbol, trade_date) DO UPDATE SET
		sma_20 = excluded.sma_20,
		sma_50 = excluded.sma_50,
		sma_200 = excluded.sma_200,
		ema_9 = excluded.ema_9,
		ema_21 = excluded.ema_21,
		ema_50 = excluded.ema_50,
		rsi_14 = excluded.rsi_14,
		stoch_k = excluded.stoch_k,
		stoch_d = excluded.stoch_d,
		macd_line = excluded.macd_line,
		macd_signal = excluded.macd_signal,
		macd_hist = excluded.macd_hist,
		bb_upper = excluded.bb_upper,
		bb_middle = excluded.bb_middle,
		bb_lower = excluded.bb_lower,
		atr_14 = excluded.atr_14,
		supertrend = excluded.supertrend,
		supertrend_dir = excluded.supertrend_dir,
		adx_14 = excluded.adx_14,
		rvol = excluded.rvol,
		obv = excluded.obv,
		roc_14 = excluded.roc_14,
		hist_volatility = excluded.hist_volatility,
		swing_high = excluded.swing_high,
		swing_low = excluded.swing_low,
		support_level = excluded.support_level,
		resist_level = excluded.resist_level;
	`
	_, err := db.Exec(query,
		ind.Symbol, ind.Date, ind.SMA20, ind.SMA50, ind.SMA200, ind.EMA9, ind.EMA21, ind.EMA50,
		ind.RSI14, ind.StochK, ind.StochD, ind.MACDLine, ind.MACDSignal, ind.MACDHistogram,
		ind.BBUpper, ind.BBMiddle, ind.BBLower, ind.ATR14, ind.Supertrend, ind.SupertrendDir,
		ind.ADX14, ind.RVOL, ind.OBV, ind.ROC14, ind.HistVolatility, ind.SwingHigh, ind.SwingLow,
		ind.SupportLevel, ind.ResistLevel,
	)
	return err
}

// GetLatestIndicators returns the most recent indicator snapshot for a symbol.
func (db *DB) GetLatestIndicators(symbol string) (*TechnicalIndicators, error) {
	query := `SELECT 
		symbol, trade_date, sma_20, sma_50, sma_200, ema_9, ema_21, ema_50,
		rsi_14, stoch_k, stoch_d, macd_line, macd_signal, macd_hist,
		bb_upper, bb_middle, bb_lower, atr_14, supertrend, supertrend_dir,
		adx_14, rvol, obv, roc_14, hist_volatility, swing_high, swing_low,
		support_level, resist_level, created_at
	FROM technical_indicators WHERE symbol = ? ORDER BY trade_date DESC LIMIT 1`

	var ind TechnicalIndicators
	var createdAt time.Time
	err := db.QueryRow(query, symbol).Scan(
		&ind.Symbol, &ind.Date, &ind.SMA20, &ind.SMA50, &ind.SMA200, &ind.EMA9, &ind.EMA21, &ind.EMA50,
		&ind.RSI14, &ind.StochK, &ind.StochD, &ind.MACDLine, &ind.MACDSignal, &ind.MACDHistogram,
		&ind.BBUpper, &ind.BBMiddle, &ind.BBLower, &ind.ATR14, &ind.Supertrend, &ind.SupertrendDir,
		&ind.ADX14, &ind.RVOL, &ind.OBV, &ind.ROC14, &ind.HistVolatility, &ind.SwingHigh, &ind.SwingLow,
		&ind.SupportLevel, &ind.ResistLevel, &createdAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	ind.CreatedAt = createdAt
	return &ind, nil
}
