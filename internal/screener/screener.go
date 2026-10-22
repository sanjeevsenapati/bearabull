package screener

import (
	"sort"
	"strings"
)

// ScreenerFilter options for querying stocks.
type ScreenerFilter struct {
	Preset    string  `json:"preset"`     // "top_bullish", "top_bearish", "strong_momentum", "breakouts", "pullbacks", "high_volume", "low_volatility", "insufficient_data"
	Signal    string  `json:"signal"`     // "BUY", "SELL", "HOLD", "ALL"
	Sector    string  `json:"sector"`     // specific sector or empty
	MinScore  float64 `json:"min_score"`  // min score threshold
	RiskLevel string  `json:"risk_level"` // "Low", "Normal", "High"
	MinPrice  float64 `json:"min_price"`
	MaxPrice  float64 `json:"max_price"`
	SortBy    string  `json:"sort_by"`    // "score", "weekly_change", "rsi", "rvol", "price", "symbol"
	SortOrder string  `json:"sort_order"` // "asc", "desc"
}

// ScreenerItem combines Stock, Signal, Price, and Indicator snapshot.
type ScreenerItem struct {
	Symbol          string  `json:"symbol"`
	Ticker          string  `json:"ticker"`
	CompanyName     string  `json:"company_name"`
	Sector          string  `json:"sector"`
	LastClose       float64 `json:"last_close"`
	WeeklyChangePct float64 `json:"weekly_change_pct"`
	Signal          string  `json:"signal"`
	Score           float64 `json:"score"`
	BullishScore    float64 `json:"bullish_score"`
	BearishScore    float64 `json:"bearish_score"`
	TrendClass      string  `json:"trend_class"`
	MomentumClass   string  `json:"momentum_class"`
	VolatilityClass string  `json:"volatility_class"`
	RSI             float64 `json:"rsi"`
	RVOL            float64 `json:"rvol"`
	ATR             float64 `json:"atr"`
	SupportLevel    float64 `json:"support_level"`
	ResistLevel     float64 `json:"resist_level"`
	HistVolatility  float64 `json:"hist_volatility"`
	SignalDate      string  `json:"signal_date"`
	Rationale       string  `json:"rationale"`
	StopLoss        float64 `json:"stop_loss"`
	RiskReward      float64 `json:"risk_reward"`
	IsBreakout      bool    `json:"is_breakout"`
	IsPullback      bool    `json:"is_pullback"`
}

// FilterAndRank processes a list of screener items according to filter criteria.
func FilterAndRank(items []ScreenerItem, f ScreenerFilter) []ScreenerItem {
	var filtered []ScreenerItem

	for _, item := range items {
		// Preset filters
		switch f.Preset {
		case "top_bullish":
			if item.Signal != "BUY" && item.BullishScore < 65 {
				continue
			}
		case "top_bearish":
			if item.Signal != "SELL" && item.BearishScore < 65 {
				continue
			}
		case "strong_momentum":
			if item.RSI < 60 || item.WeeklyChangePct <= 0 {
				continue
			}
		case "breakouts":
			if !item.IsBreakout && item.RVOL < 1.3 {
				continue
			}
		case "pullbacks":
			if !item.IsPullback && (item.RSI < 40 || item.RSI > 55 || item.TrendClass != "Bullish") {
				continue
			}
		case "high_volume":
			if item.RVOL < 1.3 {
				continue
			}
		case "low_volatility":
			if item.VolatilityClass != "Low" && item.HistVolatility > 20 {
				continue
			}
		case "insufficient_data":
			if item.Signal != "INSUFFICIENT DATA" {
				continue
			}
		}

		// Signal filter
		if f.Signal != "" && strings.ToUpper(f.Signal) != "ALL" {
			if !strings.EqualFold(item.Signal, f.Signal) {
				continue
			}
		}

		// Sector filter
		if f.Sector != "" && strings.ToUpper(f.Sector) != "ALL" {
			if !strings.EqualFold(item.Sector, f.Sector) {
				continue
			}
		}

		// Min Score
		if f.MinScore > 0 && item.Score < f.MinScore {
			continue
		}

		// Risk Level
		if f.RiskLevel != "" && strings.ToUpper(f.RiskLevel) != "ALL" {
			if !strings.EqualFold(item.VolatilityClass, f.RiskLevel) {
				continue
			}
		}

		// Price Range
		if f.MinPrice > 0 && item.LastClose < f.MinPrice {
			continue
		}
		if f.MaxPrice > 0 && item.LastClose > f.MaxPrice {
			continue
		}

		filtered = append(filtered, item)
	}

	// Sorting
	isAsc := strings.ToLower(f.SortOrder) == "asc"
	sort.Slice(filtered, func(i, j int) bool {
		switch f.SortBy {
		case "weekly_change":
			if isAsc {
				return filtered[i].WeeklyChangePct < filtered[j].WeeklyChangePct
			}
			return filtered[i].WeeklyChangePct > filtered[j].WeeklyChangePct
		case "rsi":
			if isAsc {
				return filtered[i].RSI < filtered[j].RSI
			}
			return filtered[i].RSI > filtered[j].RSI
		case "rvol":
			if isAsc {
				return filtered[i].RVOL < filtered[j].RVOL
			}
			return filtered[i].RVOL > filtered[j].RVOL
		case "price":
			if isAsc {
				return filtered[i].LastClose < filtered[j].LastClose
			}
			return filtered[i].LastClose > filtered[j].LastClose
		case "symbol":
			if isAsc {
				return filtered[i].Symbol < filtered[j].Symbol
			}
			return filtered[i].Symbol > filtered[j].Symbol
		case "score":
			fallthrough
		default:
			if isAsc {
				return filtered[i].Score < filtered[j].Score
			}
			return filtered[i].Score > filtered[j].Score
		}
	})

	return filtered
}
