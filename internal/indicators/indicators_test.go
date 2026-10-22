package indicators

import (
	"bearabull/internal/marketdata"
	"math"
	"testing"
)

func TestCalculateSMA(t *testing.T) {
	data := []float64{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}
	sma5 := CalculateSMA(data, 5)

	// SMA of first 5 numbers (10..50) = 150/5 = 30.0
	if math.Abs(sma5[4]-30.0) > 0.01 {
		t.Errorf("expected sma5[4] == 30.0, got %f", sma5[4])
	}
	// SMA of next 5 numbers (20..60) = 200/5 = 40.0
	if math.Abs(sma5[5]-40.0) > 0.01 {
		t.Errorf("expected sma5[5] == 40.0, got %f", sma5[5])
	}
	// SMA of last 5 numbers (60..100) = 400/5 = 80.0
	if math.Abs(sma5[9]-80.0) > 0.01 {
		t.Errorf("expected sma5[9] == 80.0, got %f", sma5[9])
	}
}

func TestCalculateEMA(t *testing.T) {
	data := []float64{10, 20, 30, 40, 50, 60}
	ema3 := CalculateEMA(data, 3)

	// First 3 average = (10+20+30)/3 = 20.0
	if math.Abs(ema3[2]-20.0) > 0.01 {
		t.Errorf("expected initial EMA = 20.0, got %f", ema3[2])
	}
	// Next multiplier = 2/(3+1) = 0.5. EMA = (40 - 20)*0.5 + 20 = 30.0
	if math.Abs(ema3[3]-30.0) > 0.01 {
		t.Errorf("expected EMA = 30.0, got %f", ema3[3])
	}
}

func TestCalculateRSI(t *testing.T) {
	// Monotonically increasing series -> RSI should be 100
	upSeries := make([]float64, 30)
	for i := range upSeries {
		upSeries[i] = 100.0 + float64(i*5)
	}
	rsiUp := CalculateRSI(upSeries, 14)
	if rsiUp[29] != 100.0 {
		t.Errorf("expected RSI 100 for strictly upward series, got %f", rsiUp[29])
	}

	// Monotonically decreasing series -> RSI should approach 0
	downSeries := make([]float64, 30)
	for i := range downSeries {
		downSeries[i] = 200.0 - float64(i*5)
	}
	rsiDown := CalculateRSI(downSeries, 14)
	if rsiDown[29] > 5.0 {
		t.Errorf("expected RSI < 5 for strictly downward series, got %f", rsiDown[29])
	}
}

func TestCalculateBollingerBands(t *testing.T) {
	data := make([]float64, 30)
	for i := range data {
		data[i] = 100.0 // flat line, std dev = 0
	}
	bb := CalculateBollingerBands(data, 20, 2.0)
	if bb.Middle[25] != 100.0 || bb.Upper[25] != 100.0 || bb.Lower[25] != 100.0 {
		t.Errorf("expected flat bands at 100.0, got M=%f U=%f L=%f", bb.Middle[25], bb.Upper[25], bb.Lower[25])
	}
}

func TestCalculateATR(t *testing.T) {
	candles := make([]marketdata.Candle, 25)
	for i := range candles {
		candles[i] = marketdata.Candle{
			High:  110.0,
			Low:   100.0,
			Close: 105.0,
			Open:  102.0,
		}
	}
	// Constant range of 10.0 -> ATR should equal 10.0
	atr := CalculateATR(candles, 14)
	if math.Abs(atr[20]-10.0) > 0.05 {
		t.Errorf("expected ATR ~10.0, got %f", atr[20])
	}
}
