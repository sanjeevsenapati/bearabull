package signals

import (
	"bearabull/internal/marketdata"
	"bearabull/internal/storage"
	"testing"
)

func TestSignalEngine_InsufficientData(t *testing.T) {
	engine := NewEngine("v1.0.0")
	stock := storage.Stock{Symbol: "RELIANCE.NS", CompanyName: "Reliance Industries Ltd", Sector: "Oil & Gas"}

	// Only 20 candles -> must return INSUFFICIENT DATA
	candles := make([]marketdata.Candle, 20)
	for i := range candles {
		candles[i] = marketdata.Candle{
			Symbol: stock.Symbol,
			Date:   "2026-01-01",
			Close:  100.0,
			High:   105.0,
			Low:    95.0,
			Open:   100.0,
			Volume: 100000,
		}
	}

	sig := engine.Evaluate(stock, candles)
	if sig.Signal != "INSUFFICIENT DATA" {
		t.Errorf("expected INSUFFICIENT DATA for 20 candles, got %s", sig.Signal)
	}
}

func TestSignalEngine_BullishEvaluation(t *testing.T) {
	engine := NewEngine("v1.0.0")
	stock := storage.Stock{Symbol: "TCS.NS", CompanyName: "Tata Consultancy Services Ltd", Sector: "IT"}

	// 100 candles of steady uptrend
	candles := make([]marketdata.Candle, 100)
	for i := range candles {
		p := 1000.0 + float64(i*10)
		candles[i] = marketdata.Candle{
			Symbol: stock.Symbol,
			Date:   "2026-01-10",
			Open:   p - 2.0,
			High:   p + 5.0,
			Low:    p - 3.0,
			Close:  p,
			Volume: 2000000,
		}
	}

	sig := engine.Evaluate(stock, candles)
	if sig.Signal == "INSUFFICIENT DATA" {
		t.Fatalf("expected valid signal, got INSUFFICIENT DATA")
	}
	if sig.BullishScore < 60 {
		t.Errorf("expected high bullish score for strong uptrend, got %f", sig.BullishScore)
	}
	if len(sig.BullishEvidence) == 0 {
		t.Errorf("expected bullish evidence points to be populated")
	}
	if sig.StopLoss <= 0 || sig.StopLoss >= sig.LastClose {
		t.Errorf("expected stop loss to be below last close for bullish setup, got stop=%f, close=%f", sig.StopLoss, sig.LastClose)
	}
}
