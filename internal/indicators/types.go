package indicators

import (
	"bearabull/internal/marketdata"
	"bearabull/internal/storage"
)

// IndicatorSet holds all computed technical indicators for a given candle series.
type IndicatorSet struct {
	Symbol         string
	Date           string
	Close          float64
	SMA20          float64
	SMA50          float64
	SMA200         float64
	EMA9           float64
	EMA21          float64
	EMA50          float64
	RSI14          float64
	StochK         float64
	StochD         float64
	MACDLine       float64
	MACDSignal     float64
	MACDHistogram  float64
	BBUpper        float64
	BBMiddle       float64
	BBLower        float64
	ATR14          float64
	Supertrend     float64
	SupertrendDir  int // 1 = Bullish, -1 = Bearish
	ADX14          float64
	PlusDI14       float64
	MinusDI14      float64
	RVOL           float64
	OBV            float64
	ROC14          float64
	HistVolatility float64
	SwingHigh      float64
	SwingLow       float64
	SupportLevel   float64
	ResistLevel    float64
	IsBreakoutUp   bool
	IsBreakoutDown bool
	IsGapUp        bool
	IsGapDown      bool
}

// ToStorage converts IndicatorSet to storage.TechnicalIndicators.
func (set *IndicatorSet) ToStorage() storage.TechnicalIndicators {
	return storage.TechnicalIndicators{
		Symbol:         set.Symbol,
		Date:           set.Date,
		SMA20:          set.SMA20,
		SMA50:          set.SMA50,
		SMA200:         set.SMA200,
		EMA9:           set.EMA9,
		EMA21:          set.EMA21,
		EMA50:          set.EMA50,
		RSI14:          set.RSI14,
		StochK:         set.StochK,
		StochD:         set.StochD,
		MACDLine:       set.MACDLine,
		MACDSignal:     set.MACDSignal,
		MACDHistogram:  set.MACDHistogram,
		BBUpper:        set.BBUpper,
		BBMiddle:       set.BBMiddle,
		BBLower:        set.BBLower,
		ATR14:          set.ATR14,
		Supertrend:     set.Supertrend,
		SupertrendDir:  set.SupertrendDir,
		ADX14:          set.ADX14,
		RVOL:           set.RVOL,
		OBV:            set.OBV,
		ROC14:          set.ROC14,
		HistVolatility: set.HistVolatility,
		SwingHigh:      set.SwingHigh,
		SwingLow:       set.SwingLow,
		SupportLevel:   set.SupportLevel,
		ResistLevel:    set.ResistLevel,
	}
}

// CandlesFromPrices converts storage.DailyPrice slice to marketdata.Candle slice.
func CandlesFromPrices(prices []storage.DailyPrice) []marketdata.Candle {
	res := make([]marketdata.Candle, len(prices))
	for i, p := range prices {
		res[i] = marketdata.Candle{
			Symbol:   p.Symbol,
			Date:     p.Date,
			Open:     p.Open,
			High:     p.High,
			Low:      p.Low,
			Close:    p.Close,
			AdjClose: p.AdjClose,
			Volume:   p.Volume,
		}
	}
	return res
}
