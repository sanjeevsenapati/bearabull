package signals

import (
	"bearabull/internal/indicators"
	"bearabull/internal/marketdata"
	"bearabull/internal/storage"
	"fmt"
	"math"
)

// Engine generates explainable weekly signals based on technical indicators.
type Engine struct {
	strategyVersion string
}

// NewEngine creates a new signal engine instance.
func NewEngine(version string) *Engine {
	if version == "" {
		version = "v1.0.0"
	}
	return &Engine{strategyVersion: version}
}

// Evaluate analyzes price candles and produces a weekly BUY / SELL / HOLD signal.
func (e *Engine) Evaluate(stock storage.Stock, candles []marketdata.Candle) *storage.WeeklySignal {
	n := len(candles)
	lastDate := ""
	lastClose := 0.0
	if n > 0 {
		lastDate = candles[n-1].Date
		lastClose = candles[n-1].Close
	}

	// Calculate weekly price change
	weeklyChangePct := 0.0
	if n >= 5 {
		prev5Close := candles[n-5].Close
		if prev5Close > 0 {
			weeklyChangePct = math.Round(((lastClose-prev5Close)/prev5Close)*10000) / 100
		}
	}

	// Validate data sufficiency: requires at least 50 historical daily candles
	if n < 50 {
		return &storage.WeeklySignal{
			Symbol:          stock.Symbol,
			CompanyName:     stock.CompanyName,
			Sector:          stock.Sector,
			SignalDate:      lastDate,
			Timeframe:       "Weekly",
			Signal:          "INSUFFICIENT DATA",
			Score:           0,
			BullishScore:    0,
			BearishScore:    0,
			TrendClass:      "Neutral",
			MomentumClass:   "Neutral",
			VolatilityClass: "Normal",
			Rationale:       fmt.Sprintf("Insufficient historical data (%d candles available, 50 minimum required for reliable moving averages and indicators).", n),
			StrategyVersion: e.strategyVersion,
			LastClose:       lastClose,
			WeeklyChangePct: weeklyChangePct,
		}
	}

	ind := indicators.ComputeAllIndicators(candles)
	if ind == nil {
		return &storage.WeeklySignal{
			Symbol:          stock.Symbol,
			CompanyName:     stock.CompanyName,
			Sector:          stock.Sector,
			SignalDate:      lastDate,
			Timeframe:       "Weekly",
			Signal:          "INSUFFICIENT DATA",
			Rationale:       "Technical indicators could not be fully calculated.",
			StrategyVersion: e.strategyVersion,
			LastClose:       lastClose,
			WeeklyChangePct: weeklyChangePct,
		}
	}

	var bullEvidence []storage.SignalEvidence
	var bearEvidence []storage.SignalEvidence

	var bullTrendPts, bearTrendPts float64
	var bullMomPts, bearMomPts float64
	var bullVolPts, bearVolPts float64
	var bullPAPts, bearPAPts float64
	var bullRiskPts, bearRiskPts float64

	// 1. TREND COMPONENT (35 Points Max)
	// Long-term trend: Price vs SMA 200
	if ind.SMA200 > 0 {
		if ind.Close > ind.SMA200 {
			bullTrendPts += 10
			bullEvidence = append(bullEvidence, storage.SignalEvidence{
				Type:        "BULLISH",
				Category:    "Trend",
				Description: fmt.Sprintf("Price (₹%.2f) trades above 200-day SMA (₹%.2f), confirming long-term bull market regime", ind.Close, ind.SMA200),
				Weight:      10,
			})
		} else {
			bearTrendPts += 10
			bearEvidence = append(bearEvidence, storage.SignalEvidence{
				Type:        "BEARISH",
				Category:    "Trend",
				Description: fmt.Sprintf("Price (₹%.2f) trades below 200-day SMA (₹%.2f), reflecting prolonged downward pressure", ind.Close, ind.SMA200),
				Weight:      10,
			})
		}
	} else {
		// When < 200 candles, distribute long-term trend weight proportionally to SMA 50 / SMA 20
		if ind.Close > ind.SMA50 {
			bullTrendPts += 10
		} else {
			bearTrendPts += 10
		}
	}

	// Medium-term trend: Price vs SMA 50 and SMA 20
	if ind.Close > ind.SMA50 && ind.SMA20 > ind.SMA50 {
		bullTrendPts += 10
		bullEvidence = append(bullEvidence, storage.SignalEvidence{
			Type:        "BULLISH",
			Category:    "Trend",
			Description: fmt.Sprintf("Golden alignment: Price > 50-day SMA (₹%.2f) and 20-day SMA (₹%.2f) > 50-day SMA", ind.SMA50, ind.SMA20),
			Weight:      10,
		})
	} else if ind.Close < ind.SMA50 && ind.SMA20 < ind.SMA50 {
		bearTrendPts += 10
		bearEvidence = append(bearEvidence, storage.SignalEvidence{
			Type:        "BEARISH",
			Category:    "Trend",
			Description: fmt.Sprintf("Bearish alignment: Price < 50-day SMA (₹%.2f) with 20-day SMA descending below 50-day SMA", ind.SMA50),
			Weight:      10,
		})
	}

	// Short-term trend: EMA 9 vs EMA 21
	if ind.EMA9 > ind.EMA21 {
		bullTrendPts += 5
		bullEvidence = append(bullEvidence, storage.SignalEvidence{
			Type:        "BULLISH",
			Category:    "Trend",
			Description: "Short-term momentum: 9-day EMA is trading above 21-day EMA",
			Weight:      5,
		})
	} else {
		bearTrendPts += 5
		bearEvidence = append(bearEvidence, storage.SignalEvidence{
			Type:        "BEARISH",
			Category:    "Trend",
			Description: "Short-term weakness: 9-day EMA has crossed below 21-day EMA",
			Weight:      5,
		})
	}

	// Supertrend
	if ind.SupertrendDir == 1 {
		bullTrendPts += 5
		bullEvidence = append(bullEvidence, storage.SignalEvidence{
			Type:        "BULLISH",
			Category:    "Trend",
			Description: fmt.Sprintf("Supertrend indicator is Bullish with trailing floor at ₹%.2f", ind.Supertrend),
			Weight:      5,
		})
	} else {
		bearTrendPts += 5
		bearEvidence = append(bearEvidence, storage.SignalEvidence{
			Type:        "BEARISH",
			Category:    "Trend",
			Description: fmt.Sprintf("Supertrend indicator is Bearish with overhead ceiling at ₹%.2f", ind.Supertrend),
			Weight:      5,
		})
	}

	// ADX Trend Strength
	if ind.ADX14 >= 20 {
		if ind.PlusDI14 > ind.MinusDI14 {
			bullTrendPts += 5
			bullEvidence = append(bullEvidence, storage.SignalEvidence{
				Type:        "BULLISH",
				Category:    "Trend",
				Description: fmt.Sprintf("Strong directional trend (ADX %.1f) led by buyers (+DI %.1f > -DI %.1f)", ind.ADX14, ind.PlusDI14, ind.MinusDI14),
				Weight:      5,
			})
		} else {
			bearTrendPts += 5
			bearEvidence = append(bearEvidence, storage.SignalEvidence{
				Type:        "BEARISH",
				Category:    "Trend",
				Description: fmt.Sprintf("Strong directional trend (ADX %.1f) led by sellers (-DI %.1f > +DI %.1f)", ind.ADX14, ind.MinusDI14, ind.PlusDI14),
				Weight:      5,
			})
		}
	} else {
		bullTrendPts += 3
	}

	// 2. MOMENTUM COMPONENT (25 Points Max)
	// MACD
	if ind.MACDLine > ind.MACDSignal && ind.MACDHistogram > 0 {
		bullMomPts += 10
		bullEvidence = append(bullEvidence, storage.SignalEvidence{
			Type:        "BULLISH",
			Category:    "Momentum",
			Description: fmt.Sprintf("MACD line (%.2f) > Signal line (%.2f) with expanding positive histogram", ind.MACDLine, ind.MACDSignal),
			Weight:      10,
		})
	} else if ind.MACDLine < ind.MACDSignal && ind.MACDHistogram < 0 {
		bearMomPts += 10
		bearEvidence = append(bearEvidence, storage.SignalEvidence{
			Type:        "BEARISH",
			Category:    "Momentum",
			Description: fmt.Sprintf("MACD line (%.2f) < Signal line (%.2f) with negative histogram", ind.MACDLine, ind.MACDSignal),
			Weight:      10,
		})
	}

	// RSI
	if ind.RSI14 >= 50 && ind.RSI14 <= 75 {
		bullMomPts += 8
		bullEvidence = append(bullEvidence, storage.SignalEvidence{
			Type:        "BULLISH",
			Category:    "Momentum",
			Description: fmt.Sprintf("RSI at %.1f demonstrates healthy bullish expansion without being overbought", ind.RSI14),
			Weight:      8,
		})
	} else if ind.RSI14 > 75 {
		bullMomPts += 5
		bearEvidence = append(bearEvidence, storage.SignalEvidence{
			Type:        "BEARISH",
			Category:    "Momentum",
			Description: fmt.Sprintf("RSI at %.1f is overbought (>75), cautioning potential upside consolidation", ind.RSI14),
			Weight:      4,
		})
	} else if ind.RSI14 < 35 {
		bullMomPts += 5
		bullEvidence = append(bullEvidence, storage.SignalEvidence{
			Type:        "BULLISH",
			Category:    "Momentum",
			Description: fmt.Sprintf("RSI at %.1f is oversold, setting up potential mean-reversion opportunity", ind.RSI14),
			Weight:      5,
		})
	} else {
		bearMomPts += 5
	}

	// Stochastic
	if ind.StochK > ind.StochD && ind.StochK < 80 {
		bullMomPts += 7
		bullEvidence = append(bullEvidence, storage.SignalEvidence{
			Type:        "BULLISH",
			Category:    "Momentum",
			Description: fmt.Sprintf("Stochastic oscillator shows positive crossover (%%K %.1f > %%D %.1f)", ind.StochK, ind.StochD),
			Weight:      7,
		})
	} else if ind.StochK < ind.StochD && ind.StochK > 20 {
		bearMomPts += 7
		bearEvidence = append(bearEvidence, storage.SignalEvidence{
			Type:        "BEARISH",
			Category:    "Momentum",
			Description: fmt.Sprintf("Stochastic oscillator exhibits negative cross (%%K %.1f < %%D %.1f)", ind.StochK, ind.StochD),
			Weight:      7,
		})
	}

	// 3. VOLUME CONFIRMATION (15 Points Max)
	if ind.RVOL >= 1.25 {
		if ind.Close >= candles[n-1].Open {
			bullVolPts += 10
			bullEvidence = append(bullEvidence, storage.SignalEvidence{
				Type:        "BULLISH",
				Category:    "Volume",
				Description: fmt.Sprintf("Elevated relative volume (RVOL %.2fx) validates aggressive institutional accumulation", ind.RVOL),
				Weight:      10,
			})
		} else {
			bearVolPts += 10
			bearEvidence = append(bearEvidence, storage.SignalEvidence{
				Type:        "BEARISH",
				Category:    "Volume",
				Description: fmt.Sprintf("High-volume distribution day (RVOL %.2fx) confirms institutional selling", ind.RVOL),
				Weight:      10,
			})
		}
	} else {
		bullVolPts += 3
		bearVolPts += 3
	}

	if ind.ROC14 > 0 {
		bullVolPts += 5
		bullEvidence = append(bullEvidence, storage.SignalEvidence{
			Type:        "BULLISH",
			Category:    "Volume",
			Description: fmt.Sprintf("14-day Rate of Change is positive (+%.1f%%)", ind.ROC14),
			Weight:      5,
		})
	} else {
		bearVolPts += 5
		bearEvidence = append(bearEvidence, storage.SignalEvidence{
			Type:        "BEARISH",
			Category:    "Volume",
			Description: fmt.Sprintf("14-day Rate of Change is negative (%.1f%%)", ind.ROC14),
			Weight:      5,
		})
	}

	// 4. PRICE ACTION & BREAKOUTS (15 Points Max)
	if ind.IsBreakoutUp {
		bullPAPts += 10
		bullEvidence = append(bullEvidence, storage.SignalEvidence{
			Type:        "BULLISH",
			Category:    "Price Action",
			Description: fmt.Sprintf("Price broke out above 20-day high (₹%.2f) on confirming volume", ind.SwingHigh),
			Weight:      10,
		})
	} else if ind.IsBreakoutDown {
		bearPAPts += 10
		bearEvidence = append(bearEvidence, storage.SignalEvidence{
			Type:        "BEARISH",
			Category:    "Price Action",
			Description: fmt.Sprintf("Price broke down below 20-day support floor (₹%.2f)", ind.SwingLow),
			Weight:      10,
		})
	} else {
		bullPAPts += 5
		bearPAPts += 5
	}

	if ind.IsGapUp {
		bullPAPts += 5
		bullEvidence = append(bullEvidence, storage.SignalEvidence{
			Type:        "BULLISH",
			Category:    "Price Action",
			Description: "Unfilled gap-up opening indicates strong opening bid",
			Weight:      5,
		})
	} else if ind.IsGapDown {
		bearPAPts += 5
		bearEvidence = append(bearEvidence, storage.SignalEvidence{
			Type:        "BEARISH",
			Category:    "Price Action",
			Description: "Gap-down opening indicates sudden supply overhang",
			Weight:      5,
		})
	}

	// 5. VOLATILITY & RISK FILTER (10 Points Max)
	volClass := "Normal"
	if ind.HistVolatility > 35 {
		volClass = "High"
		bearRiskPts += 5
		bearEvidence = append(bearEvidence, storage.SignalEvidence{
			Type:        "BEARISH",
			Category:    "Volatility",
			Description: fmt.Sprintf("Annualized historical volatility is elevated (%.1f%%), widening downside risk", ind.HistVolatility),
			Weight:      5,
		})
	} else if ind.HistVolatility < 18 {
		volClass = "Low"
		bullRiskPts += 8
		bullEvidence = append(bullEvidence, storage.SignalEvidence{
			Type:        "BULLISH",
			Category:    "Volatility",
			Description: fmt.Sprintf("Low historical volatility (%.1f%%) favors predictable trend continuation", ind.HistVolatility),
			Weight:      8,
		})
	} else {
		bullRiskPts += 5
		bearRiskPts += 5
	}

	// Aggregate Scores
	bullTotal := bullTrendPts + bullMomPts + bullVolPts + bullPAPts + bullRiskPts
	bearTotal := bearTrendPts + bearMomPts + bearVolPts + bearPAPts + bearRiskPts

	bullScore := math.Min(100, math.Max(0, math.Round(bullTotal)))
	bearScore := math.Min(100, math.Max(0, math.Round(bearTotal)))

	// Classifications
	trendClass := "Neutral"
	if bullTrendPts >= 25 {
		trendClass = "Bullish"
	} else if bearTrendPts >= 25 {
		trendClass = "Bearish"
	}

	momClass := "Neutral"
	if ind.RSI14 > 70 {
		momClass = "Overbought"
	} else if ind.RSI14 < 30 {
		momClass = "Oversold"
	} else if ind.MACDHistogram > 0 && ind.RSI14 > 50 {
		momClass = "Bullish Momentum"
	} else if ind.MACDHistogram < 0 && ind.RSI14 < 50 {
		momClass = "Bearish Momentum"
	}

	// Signal Determination
	signalLabel := "HOLD"
	primaryScore := bullScore

	if bullScore >= 70 && bullScore > bearScore+10 {
		signalLabel = "BUY"
		primaryScore = bullScore
	} else if bearScore >= 70 && bearScore > bullScore+10 {
		signalLabel = "SELL"
		primaryScore = bearScore
	} else {
		signalLabel = "HOLD"
		primaryScore = math.Max(bullScore, bearScore)
	}

	// Trade levels (Entry, Invalidation, Stop, Risk/Reward)
	atrVal := ind.ATR14
	if atrVal <= 0 {
		atrVal = ind.Close * 0.02
	}

	var entryLow, entryHigh, invalidation, stopLoss, riskReward float64
	if signalLabel == "BUY" {
		entryLow = math.Round((ind.Close-0.5*atrVal)*100) / 100
		entryHigh = math.Round((ind.Close+0.2*atrVal)*100) / 100
		stopLoss = math.Round((ind.Close-1.5*atrVal)*100) / 100
		invalidation = stopLoss
		target := math.Round((ind.Close+3.0*atrVal)*100) / 100
		risk := ind.Close - stopLoss
		reward := target - ind.Close
		if risk > 0 {
			riskReward = math.Round((reward/risk)*10) / 10
		}
	} else if signalLabel == "SELL" {
		entryHigh = math.Round((ind.Close+0.5*atrVal)*100) / 100
		entryLow = math.Round((ind.Close-0.2*atrVal)*100) / 100
		stopLoss = math.Round((ind.Close+1.5*atrVal)*100) / 100
		invalidation = stopLoss
		target := math.Round((ind.Close-3.0*atrVal)*100) / 100
		risk := stopLoss - ind.Close
		reward := ind.Close - target
		if risk > 0 {
			riskReward = math.Round((reward/risk)*10) / 10
		}
	} else {
		// Neutral hold levels
		entryLow = math.Round(ind.SupportLevel*100) / 100
		entryHigh = math.Round(ind.ResistLevel*100) / 100
		stopLoss = math.Round((ind.SupportLevel-atrVal)*100) / 100
		invalidation = stopLoss
		riskReward = 1.0
	}

	// Plain-Language Rationale
	rationale := ""
	switch signalLabel {
	case "BUY":
		rationale = fmt.Sprintf("BUY setup confirmed with strength score of %.0f/100. Trend alignment across 20/50/200 SMAs and bullish MACD momentum favor upside continuation with disciplined stop-loss at ₹%.2f.", bullScore, stopLoss)
	case "SELL":
		rationale = fmt.Sprintf("SELL bias established with bearish score of %.0f/100. Price action breaks below critical moving average floors with negative volume flow and MACD divergence. Invalidation level set at ₹%.2f.", bearScore, stopLoss)
	default:
		rationale = fmt.Sprintf("HOLD: Neutral consolidation (Score: Bullish %.0f / Bearish %.0f). Neither directional threshold (>=70) is satisfied. Wait for a decisive breakout above ₹%.2f or support confirmation near ₹%.2f.", bullScore, bearScore, ind.ResistLevel, ind.SupportLevel)
	}

	return &storage.WeeklySignal{
		Symbol:            stock.Symbol,
		CompanyName:       stock.CompanyName,
		Sector:            stock.Sector,
		SignalDate:        lastDate,
		Timeframe:         "Weekly",
		Signal:            signalLabel,
		Score:             primaryScore,
		BullishScore:      bullScore,
		BearishScore:      bearScore,
		TrendClass:        trendClass,
		MomentumClass:     momClass,
		VolatilityClass:   volClass,
		EntryZoneLow:      entryLow,
		EntryZoneHigh:     entryHigh,
		InvalidationLevel: invalidation,
		StopLoss:          stopLoss,
		RiskReward:        riskReward,
		Rationale:         rationale,
		StrategyVersion:   e.strategyVersion,
		BullishEvidence:   bullEvidence,
		BearishEvidence:   bearEvidence,
		LastClose:         lastClose,
		WeeklyChangePct:   weeklyChangePct,
	}
}
