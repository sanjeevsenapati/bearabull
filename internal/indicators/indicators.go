package indicators

import (
	"bearabull/internal/marketdata"
	"math"
)

// CalculateSMA computes Simple Moving Average for the given period.
func CalculateSMA(values []float64, period int) []float64 {
	n := len(values)
	res := make([]float64, n)
	if n < period || period <= 0 {
		return res
	}

	sum := 0.0
	for i := 0; i < period; i++ {
		sum += values[i]
	}
	res[period-1] = math.Round((sum/float64(period))*100) / 100

	for i := period; i < n; i++ {
		sum += values[i] - values[i-period]
		res[i] = math.Round((sum/float64(period))*100) / 100
	}
	return res
}

// CalculateEMA computes Exponential Moving Average for the given period.
func CalculateEMA(values []float64, period int) []float64 {
	n := len(values)
	res := make([]float64, n)
	if n < period || period <= 0 {
		return res
	}

	multiplier := 2.0 / float64(period+1)

	// First EMA is simple average of initial period
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += values[i]
	}
	prevEMA := sum / float64(period)
	res[period-1] = math.Round(prevEMA*100) / 100

	for i := period; i < n; i++ {
		currentEMA := (values[i]-prevEMA)*multiplier + prevEMA
		res[i] = math.Round(currentEMA*100) / 100
		prevEMA = currentEMA
	}
	return res
}

// CalculateRSI computes Relative Strength Index with Wilder's smoothing.
func CalculateRSI(closes []float64, period int) []float64 {
	n := len(closes)
	res := make([]float64, n)
	if n <= period || period <= 0 {
		return res
	}

	var gains, losses float64
	for i := 1; i <= period; i++ {
		diff := closes[i] - closes[i-1]
		if diff >= 0 {
			gains += diff
		} else {
			losses += -diff
		}
	}

	avgGain := gains / float64(period)
	avgLoss := losses / float64(period)

	if avgLoss == 0 {
		res[period] = 100
	} else {
		rs := avgGain / avgLoss
		res[period] = math.Round((100-(100/(1+rs)))*100) / 100
	}

	for i := period + 1; i < n; i++ {
		diff := closes[i] - closes[i-1]
		gain, loss := 0.0, 0.0
		if diff >= 0 {
			gain = diff
		} else {
			loss = -diff
		}

		avgGain = (avgGain*float64(period-1) + gain) / float64(period)
		avgLoss = (avgLoss*float64(period-1) + loss) / float64(period)

		if avgLoss == 0 {
			res[i] = 100
		} else {
			rs := avgGain / avgLoss
			res[i] = math.Round((100-(100/(1+rs)))*100) / 100
		}
	}
	return res
}

// MACDResult stores MACD Line, Signal Line, and Histogram series.
type MACDResult struct {
	MACD      []float64
	Signal    []float64
	Histogram []float64
}

// CalculateMACD computes MACD (fast=12, slow=26, signal=9).
func CalculateMACD(closes []float64, fastPeriod, slowPeriod, signalPeriod int) MACDResult {
	n := len(closes)
	empty := MACDResult{
		MACD:      make([]float64, n),
		Signal:    make([]float64, n),
		Histogram: make([]float64, n),
	}
	if n < slowPeriod+signalPeriod {
		return empty
	}

	fastEMA := CalculateEMA(closes, fastPeriod)
	slowEMA := CalculateEMA(closes, slowPeriod)

	macdLine := make([]float64, n)
	for i := slowPeriod - 1; i < n; i++ {
		macdLine[i] = math.Round((fastEMA[i]-slowEMA[i])*100) / 100
	}

	// Calculate Signal Line (EMA of MACD line starting from slowPeriod - 1)
	macdSub := macdLine[slowPeriod-1:]
	signalSub := CalculateEMA(macdSub, signalPeriod)

	signalLine := make([]float64, n)
	hist := make([]float64, n)
	for i := 0; i < len(signalSub); i++ {
		origIdx := slowPeriod - 1 + i
		signalLine[origIdx] = signalSub[i]
		if signalSub[i] != 0 || i >= signalPeriod-1 {
			hist[origIdx] = math.Round((macdLine[origIdx]-signalLine[origIdx])*100) / 100
		}
	}

	return MACDResult{
		MACD:      macdLine,
		Signal:    signalLine,
		Histogram: hist,
	}
}

// BollingerResult stores Bollinger Bands series.
type BollingerResult struct {
	Upper  []float64
	Middle []float64
	Lower  []float64
}

// CalculateBollingerBands computes Bollinger Bands (period=20, stdDev=2).
func CalculateBollingerBands(closes []float64, period int, numStdDev float64) BollingerResult {
	n := len(closes)
	middle := CalculateSMA(closes, period)
	upper := make([]float64, n)
	lower := make([]float64, n)

	for i := period - 1; i < n; i++ {
		m := middle[i]
		sumSq := 0.0
		for j := i - period + 1; j <= i; j++ {
			diff := closes[j] - m
			sumSq += diff * diff
		}
		stdDev := math.Sqrt(sumSq / float64(period))
		upper[i] = math.Round((m+numStdDev*stdDev)*100) / 100
		lower[i] = math.Round((m-numStdDev*stdDev)*100) / 100
	}

	return BollingerResult{
		Upper:  upper,
		Middle: middle,
		Lower:  lower,
	}
}

// CalculateATR computes Average True Range with Wilder's smoothing.
func CalculateATR(candles []marketdata.Candle, period int) []float64 {
	n := len(candles)
	res := make([]float64, n)
	if n <= period || period <= 0 {
		return res
	}

	tr := make([]float64, n)
	tr[0] = candles[0].High - candles[0].Low
	for i := 1; i < n; i++ {
		hl := candles[i].High - candles[i].Low
		hc := math.Abs(candles[i].High - candles[i-1].Close)
		lc := math.Abs(candles[i].Low - candles[i-1].Close)
		tr[i] = math.Max(hl, math.Max(hc, lc))
	}

	// Initial SMA of TR
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += tr[i]
	}
	prevATR := sum / float64(period)
	res[period-1] = math.Round(prevATR*100) / 100

	for i := period; i < n; i++ {
		currentATR := (prevATR*float64(period-1) + tr[i]) / float64(period)
		res[i] = math.Round(currentATR*100) / 100
		prevATR = currentATR
	}

	return res
}

// SupertrendResult stores Supertrend values and direction.
type SupertrendResult struct {
	Supertrend []float64
	Direction  []int // 1 for bull, -1 for bear
}

// CalculateSupertrend calculates Supertrend indicator (default period=10, multiplier=3.0).
func CalculateSupertrend(candles []marketdata.Candle, period int, multiplier float64) SupertrendResult {
	n := len(candles)
	st := make([]float64, n)
	dir := make([]int, n)
	if n < period {
		return SupertrendResult{Supertrend: st, Direction: dir}
	}

	atr := CalculateATR(candles, period)
	upperBand := make([]float64, n)
	lowerBand := make([]float64, n)

	for i := period - 1; i < n; i++ {
		hl2 := (candles[i].High + candles[i].Low) / 2.0
		basicUpper := hl2 + (multiplier * atr[i])
		basicLower := hl2 - (multiplier * atr[i])

		if i == period-1 {
			upperBand[i] = basicUpper
			lowerBand[i] = basicLower
			dir[i] = 1
			st[i] = basicLower
			continue
		}

		// Adjust upper band
		if basicUpper < upperBand[i-1] || candles[i-1].Close > upperBand[i-1] {
			upperBand[i] = basicUpper
		} else {
			upperBand[i] = upperBand[i-1]
		}

		// Adjust lower band
		if basicLower > lowerBand[i-1] || candles[i-1].Close < lowerBand[i-1] {
			lowerBand[i] = basicLower
		} else {
			lowerBand[i] = lowerBand[i-1]
		}

		// Determine direction
		if dir[i-1] == 1 {
			if candles[i].Close < lowerBand[i] {
				dir[i] = -1
				st[i] = math.Round(upperBand[i]*100) / 100
			} else {
				dir[i] = 1
				st[i] = math.Round(lowerBand[i]*100) / 100
			}
		} else {
			if candles[i].Close > upperBand[i] {
				dir[i] = 1
				st[i] = math.Round(lowerBand[i]*100) / 100
			} else {
				dir[i] = -1
				st[i] = math.Round(upperBand[i]*100) / 100
			}
		}
	}

	return SupertrendResult{Supertrend: st, Direction: dir}
}

// ADXResult stores ADX, +DI, and -DI series.
type ADXResult struct {
	ADX     []float64
	PlusDI  []float64
	MinusDI []float64
}

// CalculateADX calculates Average Directional Index (period=14).
func CalculateADX(candles []marketdata.Candle, period int) ADXResult {
	n := len(candles)
	empty := ADXResult{
		ADX:     make([]float64, n),
		PlusDI:  make([]float64, n),
		MinusDI: make([]float64, n),
	}
	if n < period*2 {
		return empty
	}

	plusDM := make([]float64, n)
	minusDM := make([]float64, n)
	tr := make([]float64, n)

	for i := 1; i < n; i++ {
		hDiff := candles[i].High - candles[i-1].High
		lDiff := candles[i-1].Low - candles[i].Low

		if hDiff > lDiff && hDiff > 0 {
			plusDM[i] = hDiff
		}
		if lDiff > hDiff && lDiff > 0 {
			minusDM[i] = lDiff
		}

		hl := candles[i].High - candles[i].Low
		hc := math.Abs(candles[i].High - candles[i-1].Close)
		lc := math.Abs(candles[i].Low - candles[i-1].Close)
		tr[i] = math.Max(hl, math.Max(hc, lc))
	}

	smoothedTR := 0.0
	smoothedPlusDM := 0.0
	smoothedMinusDM := 0.0
	for i := 1; i <= period; i++ {
		smoothedTR += tr[i]
		smoothedPlusDM += plusDM[i]
		smoothedMinusDM += minusDM[i]
	}

	plusDI := make([]float64, n)
	minusDI := make([]float64, n)
	dx := make([]float64, n)

	if smoothedTR > 0 {
		plusDI[period] = (smoothedPlusDM / smoothedTR) * 100
		minusDI[period] = (smoothedMinusDM / smoothedTR) * 100
		diff := math.Abs(plusDI[period] - minusDI[period])
		sum := plusDI[period] + minusDI[period]
		if sum > 0 {
			dx[period] = (diff / sum) * 100
		}
	}

	for i := period + 1; i < n; i++ {
		smoothedTR = smoothedTR - (smoothedTR / float64(period)) + tr[i]
		smoothedPlusDM = smoothedPlusDM - (smoothedPlusDM / float64(period)) + plusDM[i]
		smoothedMinusDM = smoothedMinusDM - (smoothedMinusDM / float64(period)) + minusDM[i]

		if smoothedTR > 0 {
			plusDI[i] = (smoothedPlusDM / smoothedTR) * 100
			minusDI[i] = (smoothedMinusDM / smoothedTR) * 100
			diff := math.Abs(plusDI[i] - minusDI[i])
			sum := plusDI[i] + minusDI[i]
			if sum > 0 {
				dx[i] = (diff / sum) * 100
			}
		}
	}

	adx := make([]float64, n)
	sumDX := 0.0
	for i := period; i < period*2; i++ {
		sumDX += dx[i]
	}
	prevADX := sumDX / float64(period)
	adx[period*2-1] = math.Round(prevADX*100) / 100

	for i := period * 2; i < n; i++ {
		curADX := (prevADX*float64(period-1) + dx[i]) / float64(period)
		adx[i] = math.Round(curADX*100) / 100
		prevADX = curADX
	}

	return ADXResult{
		ADX:     adx,
		PlusDI:  plusDI,
		MinusDI: minusDI,
	}
}

// CalculateStochastic calculates %K and %D (14, 3, 3).
func CalculateStochastic(candles []marketdata.Candle, kPeriod, dPeriod int) ([]float64, []float64) {
	n := len(candles)
	k := make([]float64, n)
	d := make([]float64, n)
	if n < kPeriod+dPeriod {
		return k, d
	}

	rawK := make([]float64, n)
	for i := kPeriod - 1; i < n; i++ {
		low := candles[i].Low
		high := candles[i].High
		for j := i - kPeriod + 1; j <= i; j++ {
			if candles[j].Low < low {
				low = candles[j].Low
			}
			if candles[j].High > high {
				high = candles[j].High
			}
		}

		if high-low > 0 {
			rawK[i] = ((candles[i].Close - low) / (high - low)) * 100.0
		} else {
			rawK[i] = 50.0
		}
	}

	k = CalculateSMA(rawK[kPeriod-1:], 3)
	fullK := make([]float64, n)
	copy(fullK[kPeriod+1:], k[2:])

	slowD := CalculateSMA(fullK[kPeriod+1:], dPeriod)
	copy(d[kPeriod+1+dPeriod-1:], slowD[dPeriod-1:])

	return fullK, d
}

// CalculateOBV computes On-Balance Volume.
func CalculateOBV(candles []marketdata.Candle) []float64 {
	n := len(candles)
	obv := make([]float64, n)
	if n == 0 {
		return obv
	}

	obv[0] = float64(candles[0].Volume)
	for i := 1; i < n; i++ {
		if candles[i].Close > candles[i-1].Close {
			obv[i] = obv[i-1] + float64(candles[i].Volume)
		} else if candles[i].Close < candles[i-1].Close {
			obv[i] = obv[i-1] - float64(candles[i].Volume)
		} else {
			obv[i] = obv[i-1]
		}
	}
	return obv
}

// CalculateROC computes Rate of Change for given period.
func CalculateROC(closes []float64, period int) []float64 {
	n := len(closes)
	roc := make([]float64, n)
	for i := period; i < n; i++ {
		if closes[i-period] > 0 {
			roc[i] = math.Round(((closes[i]-closes[i-period])/closes[i-period])*10000) / 100
		}
	}
	return roc
}

// CalculateHistoricalVolatility computes 20-day annualized volatility.
func CalculateHistoricalVolatility(closes []float64, period int) []float64 {
	n := len(closes)
	vol := make([]float64, n)
	if n <= period {
		return vol
	}

	returns := make([]float64, n)
	for i := 1; i < n; i++ {
		if closes[i-1] > 0 {
			returns[i] = math.Log(closes[i] / closes[i-1])
		}
	}

	for i := period; i < n; i++ {
		sum := 0.0
		for j := i - period + 1; j <= i; j++ {
			sum += returns[j]
		}
		mean := sum / float64(period)

		sumSq := 0.0
		for j := i - period + 1; j <= i; j++ {
			diff := returns[j] - mean
			sumSq += diff * diff
		}
		dailyVol := math.Sqrt(sumSq / float64(period-1))
		annualized := dailyVol * math.Sqrt(252) * 100
		vol[i] = math.Round(annualized*100) / 100
	}
	return vol
}

// ComputeAllIndicators calculates all technical indicators for the entire candle series
// and returns the IndicatorSet for the latest candle.
func ComputeAllIndicators(candles []marketdata.Candle) *IndicatorSet {
	n := len(candles)
	if n < 50 {
		return nil
	}

	closes := make([]float64, n)
	volumes := make([]float64, n)
	for i, c := range candles {
		closes[i] = c.Close
		volumes[i] = float64(c.Volume)
	}

	lastIdx := n - 1
	lastCandle := candles[lastIdx]

	sma20 := CalculateSMA(closes, 20)
	sma50 := CalculateSMA(closes, 50)
	sma200 := CalculateSMA(closes, 200)

	ema9 := CalculateEMA(closes, 9)
	ema21 := CalculateEMA(closes, 21)
	ema50 := CalculateEMA(closes, 50)

	rsi := CalculateRSI(closes, 14)
	macd := CalculateMACD(closes, 12, 26, 9)
	bb := CalculateBollingerBands(closes, 20, 2.0)
	atr := CalculateATR(candles, 14)
	st := CalculateSupertrend(candles, 10, 3.0)
	adx := CalculateADX(candles, 14)
	stochK, stochD := CalculateStochastic(candles, 14, 3)
	volSMA20 := CalculateSMA(volumes, 20)
	obv := CalculateOBV(candles)
	roc := CalculateROC(closes, 14)
	histVol := CalculateHistoricalVolatility(closes, 20)

	// RVOL
	rvol := 1.0
	if volSMA20[lastIdx] > 0 {
		rvol = math.Round((float64(lastCandle.Volume)/volSMA20[lastIdx])*100) / 100
	}

	// Price action: 20-period swing high / low
	swingLookback := 20
	if swingLookback > n {
		swingLookback = n
	}
	swingHigh := candles[lastIdx].High
	swingLow := candles[lastIdx].Low
	for i := lastIdx - swingLookback + 1; i <= lastIdx; i++ {
		if candles[i].High > swingHigh {
			swingHigh = candles[i].High
		}
		if candles[i].Low < swingLow {
			swingLow = candles[i].Low
		}
	}

	// Breakouts & Gaps
	isBreakoutUp := false
	isBreakoutDown := false
	if lastIdx >= 20 {
		prev20High := candles[lastIdx-1].High
		prev20Low := candles[lastIdx-1].Low
		for j := lastIdx - 20; j < lastIdx; j++ {
			if candles[j].High > prev20High {
				prev20High = candles[j].High
			}
			if candles[j].Low < prev20Low {
				prev20Low = candles[j].Low
			}
		}
		if lastCandle.Close > prev20High && rvol >= 1.2 {
			isBreakoutUp = true
		}
		if lastCandle.Close < prev20Low && rvol >= 1.2 {
			isBreakoutDown = true
		}
	}

	isGapUp := false
	isGapDown := false
	if lastIdx >= 1 {
		prev := candles[lastIdx-1]
		if lastCandle.Low > prev.High {
			isGapUp = true
		} else if lastCandle.High < prev.Low {
			isGapDown = true
		}
	}

	// Support and Resistance levels based on swing levels and moving averages
	supportLevel := math.Min(swingLow, math.Min(sma20[lastIdx], ema50[lastIdx]))
	if supportLevel <= 0 {
		supportLevel = lastCandle.Low * 0.95
	}
	resistLevel := math.Max(swingHigh, bb.Upper[lastIdx])
	if resistLevel <= 0 {
		resistLevel = lastCandle.High * 1.05
	}

	return &IndicatorSet{
		Symbol:         lastCandle.Symbol,
		Date:           lastCandle.Date,
		Close:          lastCandle.Close,
		SMA20:          sma20[lastIdx],
		SMA50:          sma50[lastIdx],
		SMA200:         sma200[lastIdx],
		EMA9:           ema9[lastIdx],
		EMA21:          ema21[lastIdx],
		EMA50:          ema50[lastIdx],
		RSI14:          rsi[lastIdx],
		StochK:         stochK[lastIdx],
		StochD:         stochD[lastIdx],
		MACDLine:       macd.MACD[lastIdx],
		MACDSignal:     macd.Signal[lastIdx],
		MACDHistogram:  macd.Histogram[lastIdx],
		BBUpper:        bb.Upper[lastIdx],
		BBMiddle:       bb.Middle[lastIdx],
		BBLower:        bb.Lower[lastIdx],
		ATR14:          atr[lastIdx],
		Supertrend:     st.Supertrend[lastIdx],
		SupertrendDir:  st.Direction[lastIdx],
		ADX14:          adx.ADX[lastIdx],
		PlusDI14:       adx.PlusDI[lastIdx],
		MinusDI14:      adx.MinusDI[lastIdx],
		RVOL:           rvol,
		OBV:            obv[lastIdx],
		ROC14:          roc[lastIdx],
		HistVolatility: histVol[lastIdx],
		SwingHigh:      math.Round(swingHigh*100) / 100,
		SwingLow:       math.Round(swingLow*100) / 100,
		SupportLevel:   math.Round(supportLevel*100) / 100,
		ResistLevel:    math.Round(resistLevel*100) / 100,
		IsBreakoutUp:   isBreakoutUp,
		IsBreakoutDown: isBreakoutDown,
		IsGapUp:        isGapUp,
		IsGapDown:      isGapDown,
	}
}
