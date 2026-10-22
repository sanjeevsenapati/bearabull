package storage

import (
	"database/sql"
	"fmt"
	"time"
)

// UpsertStock inserts or updates a stock in the database.
func (db *DB) UpsertStock(s Stock) error {
	query := `
	INSERT INTO stocks (symbol, ticker, company_name, sector, exchange, is_active, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	ON CONFLICT(symbol) DO UPDATE SET
		ticker = excluded.ticker,
		company_name = excluded.company_name,
		sector = excluded.sector,
		exchange = excluded.exchange,
		is_active = excluded.is_active,
		updated_at = CURRENT_TIMESTAMP;
	`
	isActiveInt := 0
	if s.IsActive {
		isActiveInt = 1
	}
	_, err := db.Exec(query, s.Symbol, s.Ticker, s.CompanyName, s.Sector, s.Exchange, isActiveInt)
	return err
}

// GetStocks returns all stocks, optionally filtered by active status.
func (db *DB) GetStocks(activeOnly bool) ([]Stock, error) {
	query := "SELECT symbol, ticker, company_name, sector, exchange, is_active, updated_at FROM stocks"
	if activeOnly {
		query += " WHERE is_active = 1"
	}
	query += " ORDER BY ticker ASC"

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Stock
	for rows.Next() {
		var s Stock
		var isActiveInt int
		var updatedAt time.Time
		if err := rows.Scan(&s.Symbol, &s.Ticker, &s.CompanyName, &s.Sector, &s.Exchange, &isActiveInt, &updatedAt); err != nil {
			return nil, err
		}
		s.IsActive = isActiveInt == 1
		s.UpdatedAt = updatedAt
		result = append(result, s)
	}
	return result, rows.Err()
}

// GetStock returns a single stock by symbol or ticker.
func (db *DB) GetStock(identifier string) (*Stock, error) {
	query := `SELECT symbol, ticker, company_name, sector, exchange, is_active, updated_at 
	          FROM stocks WHERE symbol = ? OR ticker = ? LIMIT 1`
	var s Stock
	var isActiveInt int
	var updatedAt time.Time
	err := db.QueryRow(query, identifier, identifier).Scan(
		&s.Symbol, &s.Ticker, &s.CompanyName, &s.Sector, &s.Exchange, &isActiveInt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	s.IsActive = isActiveInt == 1
	s.UpdatedAt = updatedAt
	return &s, nil
}

// SetStockActive updates the active status of a stock.
func (db *DB) SetStockActive(symbol string, active bool) error {
	isActiveInt := 0
	if active {
		isActiveInt = 1
	}
	res, err := db.Exec("UPDATE stocks SET is_active = ?, updated_at = CURRENT_TIMESTAMP WHERE symbol = ?", isActiveInt, symbol)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("stock %s not found", symbol)
	}
	return nil
}

// GetSectors returns list of unique sectors and constituent count.
type SectorSummary struct {
	Sector     string `json:"sector"`
	StockCount int    `json:"stock_count"`
}

func (db *DB) GetSectors() ([]SectorSummary, error) {
	rows, err := db.Query("SELECT sector, COUNT(*) FROM stocks WHERE is_active = 1 GROUP BY sector ORDER BY sector ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []SectorSummary
	for rows.Next() {
		var s SectorSummary
		if err := rows.Scan(&s.Sector, &s.StockCount); err != nil {
			return nil, err
		}
		list = append(list, s)
	}
	return list, rows.Err()
}
