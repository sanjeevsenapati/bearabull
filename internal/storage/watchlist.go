package storage

import (
	"database/sql"
	"fmt"
	"time"
)

// GetWatchlist returns a watchlist by name with its symbols.
func (db *DB) GetWatchlist(name string) (*Watchlist, error) {
	var wl Watchlist
	err := db.QueryRow("SELECT id, name, created_at FROM watchlists WHERE name = ?", name).Scan(
		&wl.ID, &wl.Name, &wl.CreatedAt,
	)
	if err == sql.ErrNoRows {
		// Auto create default
		if name == "Default" {
			res, err := db.Exec("INSERT INTO watchlists (name) VALUES ('Default')")
			if err != nil {
				return nil, err
			}
			wl.ID, _ = res.LastInsertId()
			wl.Name = "Default"
			wl.CreatedAt = time.Now()
		} else {
			return nil, fmt.Errorf("watchlist %s not found", name)
		}
	} else if err != nil {
		return nil, err
	}

	rows, err := db.Query("SELECT symbol FROM watchlist_stocks WHERE watchlist_id = ? ORDER BY added_at DESC", wl.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var sym string
		if err := rows.Scan(&sym); err == nil {
			wl.Symbols = append(wl.Symbols, sym)
		}
	}
	return &wl, nil
}

// AddToWatchlist adds a symbol to a watchlist.
func (db *DB) AddToWatchlist(watchlistName, symbol string) error {
	wl, err := db.GetWatchlist(watchlistName)
	if err != nil {
		return err
	}

	query := `INSERT OR IGNORE INTO watchlist_stocks (watchlist_id, symbol, added_at) VALUES (?, ?, CURRENT_TIMESTAMP)`
	_, err = db.Exec(query, wl.ID, symbol)
	return err
}

// RemoveFromWatchlist removes a symbol from a watchlist.
func (db *DB) RemoveFromWatchlist(watchlistName, symbol string) error {
	wl, err := db.GetWatchlist(watchlistName)
	if err != nil {
		return err
	}

	_, err = db.Exec("DELETE FROM watchlist_stocks WHERE watchlist_id = ? AND symbol = ?", wl.ID, symbol)
	return err
}

// IsInWatchlist checks if a symbol is in the specified watchlist.
func (db *DB) IsInWatchlist(watchlistName, symbol string) (bool, error) {
	wl, err := db.GetWatchlist(watchlistName)
	if err != nil {
		return false, err
	}

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM watchlist_stocks WHERE watchlist_id = ? AND symbol = ?", wl.ID, symbol).Scan(&count)
	return count > 0, err
}
