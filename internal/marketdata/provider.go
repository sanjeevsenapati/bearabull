package marketdata

import (
	"context"
	"time"
)

// Candle represents an OHLCV candle.
type Candle struct {
	Symbol    string  `json:"symbol"`
	Date      string  `json:"date"`               // YYYY-MM-DD
	DateTime  string  `json:"datetime,omitempty"` // YYYY-MM-DD HH:MM:SS
	Interval  string  `json:"interval,omitempty"` // e.g. "15m", "1d"
	Timestamp int64   `json:"timestamp"`
	Open      float64 `json:"open"`
	High      float64 `json:"high"`
	Low       float64 `json:"low"`
	Close     float64 `json:"close"`
	AdjClose  float64 `json:"adj_close"`
	Volume    int64   `json:"volume"`
}

// Quote represents the current or latest market quote for a symbol.
type Quote struct {
	Symbol        string    `json:"symbol"`
	Price         float64   `json:"price"`
	PreviousClose float64   `json:"previous_close"`
	Change        float64   `json:"change"`
	ChangePercent float64   `json:"change_percent"`
	Volume        int64     `json:"volume"`
	High          float64   `json:"high"`
	Low           float64   `json:"low"`
	Open          float64   `json:"open"`
	TradeDate     string    `json:"trade_date"`
	LastUpdated   time.Time `json:"last_updated"`
	ProviderTime  time.Time `json:"provider_time"`
	IsStale       bool      `json:"is_stale"`
	Currency      string    `json:"currency"`
	Exchange      string    `json:"exchange"`
}

// Provider represents a financial market data provider (Yahoo Finance or licensed alternate).
type Provider interface {
	GetQuote(ctx context.Context, symbol string) (*Quote, error)
	GetHistoricalPrices(ctx context.Context, symbol string, rangeStr string, interval string) ([]Candle, error)
	GetLastUpdated(ctx context.Context, symbol string) (time.Time, error)
	Name() string
}
