package storage

import (
	"database/sql"
	"time"
)

// StartSyncRun logs the initiation of a sync run.
func (db *DB) StartSyncRun(totalStocks int) (int64, error) {
	query := `INSERT INTO data_sync_runs (started_at, status, stocks_synced, total_stocks) VALUES (?, 'running', 0, ?) RETURNING id`
	var id int64
	err := db.QueryRow(query, time.Now().UTC(), totalStocks).Scan(&id)
	return id, err
}

// UpdateSyncRun updates progress or completion of a sync run.
func (db *DB) UpdateSyncRun(id int64, synced int, status string, errMsg string) error {
	var completedAt *time.Time
	if status == "completed" || status == "failed" || status == "partial" {
		now := time.Now().UTC()
		completedAt = &now
	}

	query := `
		UPDATE data_sync_runs
		SET stocks_synced = ?, status = ?, completed_at = ?, error_message = ?
		WHERE id = ?
	`
	_, err := db.Exec(query, synced, status, completedAt, errMsg, id)
	return err
}

// GetLatestSyncRun returns the most recent sync execution.
func (db *DB) GetLatestSyncRun() (*DataSyncRun, error) {
	query := `SELECT id, started_at, completed_at, status, stocks_synced, total_stocks, error_message 
	          FROM data_sync_runs ORDER BY id DESC LIMIT 1`
	var r DataSyncRun
	var completedAt sql.NullTime
	var errMsg sql.NullString

	err := db.QueryRow(query).Scan(
		&r.ID, &r.StartedAt, &completedAt, &r.Status, &r.StocksSynced, &r.TotalStocks, &errMsg,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if completedAt.Valid {
		r.CompletedAt = &completedAt.Time
	}
	if errMsg.Valid {
		r.ErrorMessage = errMsg.String
	}
	return &r, nil
}

// SetSetting sets an arbitrary key-value setting.
func (db *DB) SetSetting(key, value string) error {
	query := `
		INSERT INTO app_settings (key, value, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP
	`
	_, err := db.Exec(query, key, value)
	return err
}

// GetSetting reads a setting by key.
func (db *DB) GetSetting(key string) (string, error) {
	var val string
	err := db.QueryRow("SELECT value FROM app_settings WHERE key = ?", key).Scan(&val)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return val, err
}
