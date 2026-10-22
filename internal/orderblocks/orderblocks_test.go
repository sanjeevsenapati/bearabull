package orderblocks

import (
	"bearabull/internal/marketdata"
	"testing"
)

func TestOrderBlockDetection(t *testing.T) {
	// Generate a synthetic sequence that forms a clean bullish order block
	// Down candle followed by huge 2-bar rally breaking swing high
	candles := []marketdata.Candle{
		{Date: "2026-01-01", Open: 100, High: 102, Low: 98, Close: 101, Volume: 1000, Timestamp: 1},
		{Date: "2026-01-02", Open: 101, High: 105, Low: 100, Close: 104, Volume: 1200, Timestamp: 2}, // Swing High
		{Date: "2026-01-03", Open: 104, High: 104, Low: 99, Close: 100, Volume: 900, Timestamp: 3},
		{Date: "2026-01-04", Open: 100, High: 101, Low: 95, Close: 96, Volume: 800, Timestamp: 4},    // Swing Low
		{Date: "2026-01-05", Open: 96, High: 97, Low: 93, Close: 94, Volume: 1500, Timestamp: 5},     // OB down-candle
		{Date: "2026-01-06", Open: 94, High: 102, Low: 94, Close: 101, Volume: 2500, Timestamp: 6},   // Huge impulse up
		{Date: "2026-01-07", Open: 101, High: 108, Low: 101, Close: 107, Volume: 3000, Timestamp: 7}, // Breaks swing high 105 (BOS)
		{Date: "2026-01-08", Open: 107, High: 109, Low: 105, Close: 108, Volume: 1800, Timestamp: 8},
		{Date: "2026-01-09", Open: 108, High: 110, Low: 106, Close: 109, Volume: 1600, Timestamp: 9},
		{Date: "2026-01-10", Open: 109, High: 111, Low: 107, Close: 110, Volume: 1400, Timestamp: 10},
	}

	detector := NewDetector(2)
	analysis := detector.Analyze("RELIANCE.NS", candles)

	if analysis == nil {
		t.Fatalf("expected non-nil analysis")
	}

	if len(analysis.OrderBlocks) == 0 {
		t.Errorf("expected detected order blocks, got 0")
	}

	if analysis.ActiveBullishOB == nil {
		t.Errorf("expected active bullish order block")
	} else {
		if analysis.ActiveBullishOB.Bottom > 95 {
			t.Errorf("expected OB bottom <= 95, got %v", analysis.ActiveBullishOB.Bottom)
		}
	}

	// Verify Stop Loss is set below OB bottom
	if analysis.StopLoss <= 0 || analysis.StopLoss >= analysis.RecommendedEntry {
		t.Errorf("invalid stop loss %v for entry %v", analysis.StopLoss, analysis.RecommendedEntry)
	}

	// Verify Target provides positive Risk/Reward
	if analysis.RiskRewardRatio < 1.0 {
		t.Errorf("expected risk/reward >= 1.0, got %v", analysis.RiskRewardRatio)
	}
}
