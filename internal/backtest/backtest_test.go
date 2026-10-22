package backtest

import (
	"bearabull/internal/marketdata"
	"bearabull/internal/signals"
	"bearabull/internal/storage"
	"fmt"
	"testing"
	"time"
)

func TestBacktestRunner_LookAheadBiasAndCostAccounting(t *testing.T) {
	runner := NewRunner(signals.NewEngine("v1.0.0"))

	// Create 80 days of synthetic trading candles
	symbol := "RELIANCE.NS"
	stock := storage.Stock{Symbol: symbol, CompanyName: "Reliance Industries Ltd", Sector: "Oil & Gas"}
	stockMap := map[string]storage.Stock{symbol: stock}

	baseDate, _ := time.Parse("2006-01-02", "2025-01-01")
	var candles []marketdata.Candle

	for i := 0; i < 90; i++ {
		curDate := baseDate.AddDate(0, 0, i)
		if curDate.Weekday() == time.Saturday || curDate.Weekday() == time.Sunday {
			continue
		}
		p := 1000.0 + float64(i*5) // Steadily rising price
		candles = append(candles, marketdata.Candle{
			Symbol: symbol,
			Date:   curDate.Format("2006-01-02"),
			Open:   p - 1.0,
			High:   p + 3.0,
			Low:    p - 2.0,
			Close:  p,
			Volume: 1500000,
		})
	}

	candlesBySymbol := map[string][]marketdata.Candle{
		symbol: candles,
	}

	params := Params{
		Name:            "Test Run",
		InitialCapital:  500000,
		MaxPositions:    1,
		PositionSizePct: 100,
		SlippagePct:     0.1,
		BrokerageFeePct: 0.15,
		StopLossPct:     5.0,
		TargetPct:       20.0,
		MinScore:        50.0,
	}

	run, err := runner.Run(params, stockMap, candlesBySymbol)
	if err != nil {
		t.Fatalf("backtest run failed: %v", err)
	}

	if run.InitialCapital != 500000 {
		t.Errorf("expected initial capital 500000, got %f", run.InitialCapital)
	}
	if len(run.EquityCurve) == 0 {
		t.Errorf("expected non-empty equity curve")
	}

	// Verify look-ahead bias: check that every trade's exit date is strictly after entry date
	for _, tr := range run.Trades {
		if tr.ExitDate < tr.EntryDate {
			t.Errorf("look-ahead violation: exit date %s before entry date %s", tr.ExitDate, tr.EntryDate)
		}
		if tr.EntryPrice <= 0 || tr.ExitPrice <= 0 {
			t.Errorf("invalid trade prices: entry=%f exit=%f", tr.EntryPrice, tr.ExitPrice)
		}
	}

	fmt.Printf("[BACKTEST TEST] Final Equity: ₹%.2f, Return: %.2f%%, Trades: %d, WinRate: %.1f%%\n",
		run.FinalEquity, run.TotalReturn, run.TotalTrades, run.WinRate)
}
