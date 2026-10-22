package orderblocks

import (
	"bearabull/internal/marketdata"
	"math"
)

// OrderBlockType represents bullish or bearish direction.
type OrderBlockType string

const (
	BullishOB OrderBlockType = "BULLISH"
	BearishOB OrderBlockType = "BEARISH"
)

// OrderBlock represents an institutional order block zone.
type OrderBlock struct {
	Type            OrderBlockType `json:"type"`
	CandleIndex     int            `json:"candle_index"`
	Date            string         `json:"date"`
	Top             float64        `json:"top"`
	Bottom          float64        `json:"bottom"`
	Volume          int64          `json:"volume"`
	IsMitigated     bool           `json:"is_mitigated"`
	MitigationCount int            `json:"mitigation_count"`
	IsInvalidated   bool           `json:"is_invalidated"`
	ImpulsePct      float64        `json:"impulse_pct"`
}

// FairValueGap represents a 3-candle price imbalance.
type FairValueGap struct {
	Type        OrderBlockType `json:"type"`
	CandleIndex int            `json:"candle_index"`
	Date        string         `json:"date"`
	Top         float64        `json:"top"`
	Bottom      float64        `json:"bottom"`
	IsFilled    bool           `json:"is_filled"`
}

// MarketStructureEvent models a Break of Structure or Change of Character.
type MarketStructureEvent struct {
	Type        string         `json:"type"` // "BOS" or "CHoCH"
	Direction   OrderBlockType `json:"direction"`
	CandleIndex int            `json:"candle_index"`
	Date        string         `json:"date"`
	Level       float64        `json:"level"`
}

// SMCAnalysis holds complete Smart Money Concepts analysis for a symbol.
type SMCAnalysis struct {
	Symbol           string                 `json:"symbol"`
	CurrentPrice     float64                `json:"current_price"`
	Trend            string                 `json:"trend"` // "BULLISH", "BEARISH", "NEUTRAL"
	ActiveBullishOB  *OrderBlock            `json:"active_bullish_ob,omitempty"`
	ActiveBearishOB  *OrderBlock            `json:"active_bearish_ob,omitempty"`
	OrderBlocks      []OrderBlock           `json:"order_blocks"`
	FairValueGaps    []FairValueGap         `json:"fair_value_gaps"`
	StructureEvents  []MarketStructureEvent `json:"structure_events"`
	LastBOS          *MarketStructureEvent  `json:"last_bos,omitempty"`
	LastCHoCH        *MarketStructureEvent  `json:"last_choch,omitempty"`
	RecommendedEntry float64                `json:"recommended_entry"`
	StopLoss         float64                `json:"stop_loss"`
	Target           float64                `json:"target"`
	RiskRewardRatio  float64                `json:"risk_reward_ratio"`
	FreshnessScore   float64                `json:"freshness_score"`
}

// Detector identifies Order Blocks, FVGs, BOS, and CHoCH from candles.
type Detector struct {
	swingLookback int
}

// NewDetector creates a Smart Money Concepts detector.
func NewDetector(swingLookback int) *Detector {
	if swingLookback <= 0 {
		swingLookback = 3
	}
	return &Detector{swingLookback: swingLookback}
}

// Analyze processes candles and extracts SMC structures.
func (d *Detector) Analyze(symbol string, candles []marketdata.Candle) *SMCAnalysis {
	n := len(candles)
	analysis := &SMCAnalysis{
		Symbol:       symbol,
		Trend:        "NEUTRAL",
		CurrentPrice: 0,
	}

	if n < 10 {
		return analysis
	}

	analysis.CurrentPrice = candles[n-1].Close

	// 1. Detect Swing Highs and Swing Lows
	swingHighs := make([]int, 0)
	swingLows := make([]int, 0)

	for i := d.swingLookback; i < n-d.swingLookback; i++ {
		isHigh := true
		isLow := true
		for j := i - d.swingLookback; j <= i+d.swingLookback; j++ {
			if j == i {
				continue
			}
			if candles[j].High >= candles[i].High {
				isHigh = false
			}
			if candles[j].Low <= candles[i].Low {
				isLow = false
			}
		}
		if isHigh {
			swingHighs = append(swingHighs, i)
		}
		if isLow {
			swingLows = append(swingLows, i)
		}
	}

	// 2. Detect Break of Structure (BOS) and Change of Character (CHoCH)
	currentTrend := "NEUTRAL"
	for i := 1; i < n; i++ {
		c := candles[i]
		// Check break above recent swing high
		for k := len(swingHighs) - 1; k >= 0; k-- {
			shIdx := swingHighs[k]
			if shIdx >= i {
				continue
			}
			sh := candles[shIdx]
			if c.Close > sh.High && candles[i-1].Close <= sh.High {
				evtType := "BOS"
				if currentTrend == "BEARISH" {
					evtType = "CHoCH"
				}
				currentTrend = "BULLISH"
				evt := MarketStructureEvent{
					Type:        evtType,
					Direction:   BullishOB,
					CandleIndex: i,
					Date:        c.Date,
					Level:       sh.High,
				}
				analysis.StructureEvents = append(analysis.StructureEvents, evt)
				if evtType == "BOS" {
					analysis.LastBOS = &evt
				} else {
					analysis.LastCHoCH = &evt
				}
				break
			}
		}

		// Check break below recent swing low
		for k := len(swingLows) - 1; k >= 0; k-- {
			slIdx := swingLows[k]
			if slIdx >= i {
				continue
			}
			sl := candles[slIdx]
			if c.Close < sl.Low && candles[i-1].Close >= sl.Low {
				evtType := "BOS"
				if currentTrend == "BULLISH" {
					evtType = "CHoCH"
				}
				currentTrend = "BEARISH"
				evt := MarketStructureEvent{
					Type:        evtType,
					Direction:   BearishOB,
					CandleIndex: i,
					Date:        c.Date,
					Level:       sl.Low,
				}
				analysis.StructureEvents = append(analysis.StructureEvents, evt)
				if evtType == "BOS" {
					analysis.LastBOS = &evt
				} else {
					analysis.LastCHoCH = &evt
				}
				break
			}
		}
	}
	analysis.Trend = currentTrend

	// 3. Detect Fair Value Gaps (FVG)
	for i := 2; i < n; i++ {
		c1 := candles[i-2]
		c3 := candles[i]

		// Bullish FVG: Low of candle 3 > High of candle 1
		if c3.Low > c1.High {
			fvg := FairValueGap{
				Type:        BullishOB,
				CandleIndex: i - 1,
				Date:        candles[i-1].Date,
				Top:         c3.Low,
				Bottom:      c1.High,
			}
			// Check if filled by subsequent candles
			for k := i + 1; k < n; k++ {
				if candles[k].Low <= fvg.Bottom {
					fvg.IsFilled = true
					break
				}
			}
			analysis.FairValueGaps = append(analysis.FairValueGaps, fvg)
		}

		// Bearish FVG: High of candle 3 < Low of candle 1
		if c3.High < c1.Low {
			fvg := FairValueGap{
				Type:        BearishOB,
				CandleIndex: i - 1,
				Date:        candles[i-1].Date,
				Top:         c1.Low,
				Bottom:      c3.High,
			}
			for k := i + 1; k < n; k++ {
				if candles[k].High >= fvg.Top {
					fvg.IsFilled = true
					break
				}
			}
			analysis.FairValueGaps = append(analysis.FairValueGaps, fvg)
		}
	}

	// 4. Detect Order Blocks (OB)
	// Bullish OB: last down candle before a strong upward displacement
	// Bearish OB: last up candle before a strong downward displacement
	var allOBs []OrderBlock
	for i := 1; i < n-2; i++ {
		c := candles[i]
		next1 := candles[i+1]
		next2 := candles[i+2]

		// Bullish OB candidate: bearish candle followed by strong 2-bar rally
		if c.Close < c.Open {
			displacement := (next2.Close - c.Close) / c.Close
			if displacement > 0.015 && next1.Close > c.High {
				top := math.Max(c.Open, c.High)
				bottom := c.Low
				ob := OrderBlock{
					Type:        BullishOB,
					CandleIndex: i,
					Date:        c.Date,
					Top:         math.Round(top*100) / 100,
					Bottom:      math.Round(bottom*100) / 100,
					Volume:      c.Volume,
					ImpulsePct:  math.Round(displacement*10000) / 100,
				}

				// Check mitigation & invalidation
				for k := i + 3; k < n; k++ {
					curr := candles[k]
					if curr.Close < ob.Bottom {
						ob.IsInvalidated = true
						break
					}
					if curr.Low <= ob.Top && curr.Low >= ob.Bottom {
						ob.IsMitigated = true
						ob.MitigationCount++
					}
				}
				if !ob.IsInvalidated {
					allOBs = append(allOBs, ob)
				}
			}
		}

		// Bearish OB candidate: bullish candle followed by strong 2-bar drop
		if c.Close > c.Open {
			displacement := (c.Close - next2.Close) / c.Close
			if displacement > 0.015 && next1.Close < c.Low {
				top := c.High
				bottom := math.Min(c.Open, c.Low)
				ob := OrderBlock{
					Type:        BearishOB,
					CandleIndex: i,
					Date:        c.Date,
					Top:         math.Round(top*100) / 100,
					Bottom:      math.Round(bottom*100) / 100,
					Volume:      c.Volume,
					ImpulsePct:  math.Round(displacement*10000) / 100,
				}

				for k := i + 3; k < n; k++ {
					curr := candles[k]
					if curr.Close > ob.Top {
						ob.IsInvalidated = true
						break
					}
					if curr.High >= ob.Bottom && curr.High <= ob.Top {
						ob.IsMitigated = true
						ob.MitigationCount++
					}
				}
				if !ob.IsInvalidated {
					allOBs = append(allOBs, ob)
				}
			}
		}
	}
	analysis.OrderBlocks = allOBs

	// 5. Select active unmitigated Order Blocks closest to current price
	var bestBullOB *OrderBlock
	var bestBearOB *OrderBlock

	for i := len(allOBs) - 1; i >= 0; i-- {
		ob := allOBs[i]
		if ob.Type == BullishOB && !ob.IsInvalidated {
			if bestBullOB == nil || (!ob.IsMitigated && bestBullOB.IsMitigated) {
				cp := ob
				bestBullOB = &cp
			}
		}
		if ob.Type == BearishOB && !ob.IsInvalidated {
			if bestBearOB == nil || (!ob.IsMitigated && bestBearOB.IsMitigated) {
				cp := ob
				bestBearOB = &cp
			}
		}
	}
	analysis.ActiveBullishOB = bestBullOB
	analysis.ActiveBearishOB = bestBearOB

	// 6. Propose Entry, Stop Loss, Target, and Risk/Reward for active setups
	currPrice := analysis.CurrentPrice
	if bestBullOB != nil && currPrice >= bestBullOB.Bottom {
		analysis.RecommendedEntry = math.Round(math.Max(bestBullOB.Bottom, math.Min(currPrice, bestBullOB.Top))*100) / 100
		buffer := bestBullOB.Bottom * 0.005 // 0.5% invalidation buffer
		analysis.StopLoss = math.Round((bestBullOB.Bottom-buffer)*100) / 100

		// Target: 2.5x risk or recent swing high
		riskPerShare := analysis.RecommendedEntry - analysis.StopLoss
		if riskPerShare > 0 {
			target := analysis.RecommendedEntry + (riskPerShare * 2.5)
			// If swing high is higher, use swing high
			for k := len(swingHighs) - 1; k >= 0; k-- {
				sh := candles[swingHighs[k]].High
				if sh > analysis.RecommendedEntry && sh > target {
					target = sh
					break
				}
			}
			analysis.Target = math.Round(target*100) / 100
			analysis.RiskRewardRatio = math.Round((target-analysis.RecommendedEntry)/riskPerShare*100) / 100
		}

		freshScore := 100.0
		if bestBullOB.IsMitigated {
			freshScore = math.Max(20.0, 100.0-float64(bestBullOB.MitigationCount)*30.0)
		}
		analysis.FreshnessScore = freshScore
	}

	return analysis
}
