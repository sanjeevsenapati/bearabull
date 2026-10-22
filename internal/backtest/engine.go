package backtest

import (
	"bearabull/internal/marketdata"
	"bearabull/internal/signals"
	"bearabull/internal/storage"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"
)

// Params contains parameters for executing a backtest run.
type Params struct {
	Name            string   `json:"name"`
	Symbols         []string `json:"symbols"` // List of symbols or empty for all active
	StartDate       string   `json:"start_date"`
	EndDate         string   `json:"end_date"`
	InitialCapital  float64  `json:"initial_capital"`
	MaxPositions    int      `json:"max_positions"`
	PositionSizePct float64  `json:"position_size_pct"` // % of equity per position, e.g. 20.0
	SlippagePct     float64  `json:"slippage_pct"`      // e.g. 0.1%
	BrokerageFeePct float64  `json:"brokerage_fee_pct"` // e.g. 0.15% (STT, exchange, brokerage)
	StopLossPct     float64  `json:"stop_loss_pct"`     // e.g. 5.0%
	TargetPct       float64  `json:"target_pct"`        // e.g. 15.0%
	AllowShort      bool     `json:"allow_short"`
	MinScore        float64  `json:"min_score"` // default 70
}

// ActivePosition represents an open trade during the simulation.
type ActivePosition struct {
	Symbol       string
	EntryDate    string
	EntryPrice   float64
	PositionType string // "LONG"
	Shares       int
	StopLoss     float64
	Target       float64
}

// Runner executes backtest simulations.
type Runner struct {
	signalEngine *signals.Engine
}

// NewRunner creates a backtest runner.
func NewRunner(signalEngine *signals.Engine) *Runner {
	if signalEngine == nil {
		signalEngine = signals.NewEngine("v1.0.0")
	}
	return &Runner{signalEngine: signalEngine}
}

// Run executes the simulation across provided symbols and historical candles.
func (r *Runner) Run(params Params, stockMap map[string]storage.Stock, candlesBySymbol map[string][]marketdata.Candle) (*storage.BacktestRun, error) {
	if params.InitialCapital <= 0 {
		params.InitialCapital = 1000000.0 // Default 10 Lakh INR
	}
	if params.MaxPositions <= 0 {
		params.MaxPositions = 5
	}
	if params.PositionSizePct <= 0 {
		params.PositionSizePct = 100.0 / float64(params.MaxPositions)
	}
	if params.MinScore <= 0 {
		params.MinScore = 70.0
	}
	if params.StopLossPct <= 0 {
		params.StopLossPct = 5.0
	}
	if params.TargetPct <= 0 {
		params.TargetPct = 15.0
	}

	// 1. Collect and unify all trading dates chronologically
	dateSet := make(map[string]bool)
	for _, candles := range candlesBySymbol {
		for _, c := range candles {
			if (params.StartDate == "" || c.Date >= params.StartDate) &&
				(params.EndDate == "" || c.Date <= params.EndDate) {
				dateSet[c.Date] = true
			}
		}
	}

	var allDates []string
	for d := range dateSet {
		allDates = append(allDates, d)
	}
	sort.Strings(allDates)

	if len(allDates) < 20 {
		return nil, fmt.Errorf("insufficient trading dates (%d) for backtest", len(allDates))
	}

	// Index candles by symbol and date for O(1) lookup
	candleLookup := make(map[string]map[string]marketdata.Candle)
	for sym, cList := range candlesBySymbol {
		candleLookup[sym] = make(map[string]marketdata.Candle)
		for _, c := range cList {
			candleLookup[sym][c.Date] = c
		}
	}

	cash := params.InitialCapital
	positions := make(map[string]*ActivePosition)
	var closedTrades []storage.BacktestTrade
	var equityCurve []storage.EquityPoint

	// Benchmark calculation (equal weight buy & hold of NIFTY universe across date range)
	var benchmarkStartPrice, benchmarkLastPrice float64
	firstDate := allDates[0]
	lastDate := allDates[len(allDates)-1]

	for _, candles := range candlesBySymbol {
		if c, ok := candleLookup[candles[0].Symbol][firstDate]; ok && c.Close > 0 {
			benchmarkStartPrice += c.Close
		}
	}
	if benchmarkStartPrice <= 0 {
		benchmarkStartPrice = 1.0
	}

	peakEquity := cash
	maxDrawdown := 0.0

	// Pending orders queued at end of weekly candle for next market open execution (prevents look-ahead bias)
	type PendingOrder struct {
		Symbol    string
		Action    string // "BUY", "SELL"
		Score     float64
		StopLoss  float64
		Target    float64
		OrderDate string
	}
	var pendingOrders []PendingOrder

	// Simulation Day-by-Day Loop
	for dayIdx, date := range allDates {
		// 1. EXECUTE PENDING ORDERS ON TODAY'S OPEN (Next-session execution)
		for _, order := range pendingOrders {
			c, hasCandle := candleLookup[order.Symbol][date]
			if !hasCandle || c.Open <= 0 {
				continue
			}

			// Apply slippage and transaction costs to entry
			effectiveOpen := c.Open * (1.0 + (params.SlippagePct / 100.0))
			feePct := params.BrokerageFeePct / 100.0

			if order.Action == "BUY" && positions[order.Symbol] == nil && len(positions) < params.MaxPositions {
				currentEquity := cash
				for _, p := range positions {
					if curC, ok := candleLookup[p.Symbol][date]; ok {
						currentEquity += float64(p.Shares) * curC.Close
					}
				}

				allocation := currentEquity * (params.PositionSizePct / 100.0)
				if allocation > cash {
					allocation = cash
				}

				shares := int(allocation / (effectiveOpen * (1.0 + feePct)))
				if shares > 0 {
					cost := float64(shares) * effectiveOpen * (1.0 + feePct)
					if cash >= cost {
						cash -= cost
						positions[order.Symbol] = &ActivePosition{
							Symbol:       order.Symbol,
							EntryDate:    date,
							EntryPrice:   math.Round(effectiveOpen*100) / 100,
							PositionType: "LONG",
							Shares:       shares,
							StopLoss:     math.Round(effectiveOpen*(1.0-(params.StopLossPct/100.0))*100) / 100,
							Target:       math.Round(effectiveOpen*(1.0+(params.TargetPct/100.0))*100) / 100,
						}
					}
				}
			}
		}
		pendingOrders = nil // Clear executed orders

		// 2. CHECK STOP-LOSS AND PROFIT TARGET EXITS DURING INTRADAY
		for sym, pos := range positions {
			c, hasCandle := candleLookup[sym][date]
			if !hasCandle {
				continue
			}

			exitTriggered := false
			exitPrice := 0.0
			exitReason := ""

			// Stop-loss hit
			if c.Low <= pos.StopLoss {
				exitTriggered = true
				exitPrice = math.Min(c.Open, pos.StopLoss) * (1.0 - (params.SlippagePct / 100.0))
				exitReason = "Stop Loss"
			} else if c.High >= pos.Target {
				// Target hit
				exitTriggered = true
				exitPrice = pos.Target * (1.0 - (params.SlippagePct / 100.0))
				exitReason = "Target Reached"
			}

			if exitTriggered {
				feePct := params.BrokerageFeePct / 100.0
				proceeds := float64(pos.Shares) * exitPrice * (1.0 - feePct)
				cash += proceeds

				pnl := proceeds - (float64(pos.Shares) * pos.EntryPrice)
				returnPct := 0.0
				if pos.EntryPrice > 0 {
					returnPct = ((exitPrice - pos.EntryPrice) / pos.EntryPrice) * 100.0
				}

				closedTrades = append(closedTrades, storage.BacktestTrade{
					Symbol:       pos.Symbol,
					CompanyName:  stockMap[pos.Symbol].CompanyName,
					EntryDate:    pos.EntryDate,
					ExitDate:     date,
					EntryPrice:   pos.EntryPrice,
					ExitPrice:    math.Round(exitPrice*100) / 100,
					PositionType: pos.PositionType,
					Shares:       pos.Shares,
					ProfitLoss:   math.Round(pnl*100) / 100,
					ReturnPct:    math.Round(returnPct*100) / 100,
					ExitReason:   exitReason,
				})
				delete(positions, sym)
			}
		}

		// 3. WEEKLY SIGNAL EVALUATION AT MARKET CLOSE (FRIDAY / WEEKLY INTERVAL)
		// Check if today is end of week (Friday or last day before gap)
		parsedDate, _ := time.Parse("2006-01-02", date)
		isWeeklyClose := parsedDate.Weekday() == time.Friday || dayIdx == len(allDates)-1

		if isWeeklyClose {
			for sym, candles := range candlesBySymbol {
				// Find slice up to today's date
				cutoffIdx := -1
				for i, c := range candles {
					if c.Date == date {
						cutoffIdx = i
						break
					}
				}

				if cutoffIdx < 50 {
					continue // Need at least 50 candles
				}

				// Evaluate using historical slice ONLY up to today (Strict No Look-Ahead)
				histSlice := candles[:cutoffIdx+1]
				sig := r.signalEngine.Evaluate(stockMap[sym], histSlice)
				if sig == nil {
					continue
				}

				// If we have an existing long position and signal flipped to SELL -> queue exit
				if pos, open := positions[sym]; open {
					if sig.Signal == "SELL" {
						c := candleLookup[sym][date]
						exitPrice := c.Close * (1.0 - (params.SlippagePct / 100.0))
						feePct := params.BrokerageFeePct / 100.0
						proceeds := float64(pos.Shares) * exitPrice * (1.0 - feePct)
						cash += proceeds

						pnl := proceeds - (float64(pos.Shares) * pos.EntryPrice)
						returnPct := ((exitPrice - pos.EntryPrice) / pos.EntryPrice) * 100.0

						closedTrades = append(closedTrades, storage.BacktestTrade{
							Symbol:       pos.Symbol,
							CompanyName:  stockMap[pos.Symbol].CompanyName,
							EntryDate:    pos.EntryDate,
							ExitDate:     date,
							EntryPrice:   pos.EntryPrice,
							ExitPrice:    math.Round(exitPrice*100) / 100,
							PositionType: pos.PositionType,
							Shares:       pos.Shares,
							ProfitLoss:   math.Round(pnl*100) / 100,
							ReturnPct:    math.Round(returnPct*100) / 100,
							ExitReason:   "Signal Exit (SELL)",
						})
						delete(positions, sym)
					}
				} else if sig.Signal == "BUY" && sig.Score >= params.MinScore {
					// Queue pending BUY order for next market open
					pendingOrders = append(pendingOrders, PendingOrder{
						Symbol:    sym,
						Action:    "BUY",
						Score:     sig.Score,
						OrderDate: date,
					})
				}
			}
		}

		// 4. CALCULATE EQUITY FOR TODAY
		dayEquity := cash
		for sym, pos := range positions {
			if c, ok := candleLookup[sym][date]; ok {
				dayEquity += float64(pos.Shares) * c.Close
			}
		}

		if dayEquity > peakEquity {
			peakEquity = dayEquity
		}
		dd := (peakEquity - dayEquity) / peakEquity * 100.0
		if dd > maxDrawdown {
			maxDrawdown = dd
		}

		// Benchmark day close
		currentBenchmarkClose := 0.0
		for _, candles := range candlesBySymbol {
			if c, ok := candleLookup[candles[0].Symbol][date]; ok && c.Close > 0 {
				currentBenchmarkClose += c.Close
			}
		}
		benchmarkEquity := params.InitialCapital * (currentBenchmarkClose / benchmarkStartPrice)

		equityCurve = append(equityCurve, storage.EquityPoint{
			Date:      date,
			Equity:    math.Round(dayEquity*100) / 100,
			Benchmark: math.Round(benchmarkEquity*100) / 100,
		})
	}

	// Close remaining positions at last date's close
	lastDateStr := allDates[len(allDates)-1]
	for sym, pos := range positions {
		c, ok := candleLookup[sym][lastDateStr]
		exitPrice := pos.EntryPrice
		if ok {
			exitPrice = c.Close
		}
		feePct := params.BrokerageFeePct / 100.0
		proceeds := float64(pos.Shares) * exitPrice * (1.0 - feePct)
		cash += proceeds
		pnl := proceeds - (float64(pos.Shares) * pos.EntryPrice)
		retPct := 0.0
		if pos.EntryPrice > 0 {
			retPct = ((exitPrice - pos.EntryPrice) / pos.EntryPrice) * 100.0
		}
		closedTrades = append(closedTrades, storage.BacktestTrade{
			Symbol:       pos.Symbol,
			CompanyName:  stockMap[pos.Symbol].CompanyName,
			EntryDate:    pos.EntryDate,
			ExitDate:     lastDateStr,
			EntryPrice:   pos.EntryPrice,
			ExitPrice:    math.Round(exitPrice*100) / 100,
			PositionType: pos.PositionType,
			Shares:       pos.Shares,
			ProfitLoss:   math.Round(pnl*100) / 100,
			ReturnPct:    math.Round(retPct*100) / 100,
			ExitReason:   "End of Backtest",
		})
	}

	finalEquity := cash

	// PERFORMANCE METRICS CALCULATION
	totalReturn := ((finalEquity - params.InitialCapital) / params.InitialCapital) * 100.0

	// CAGR
	days := float64(len(allDates)) * (365.0 / 252.0)
	years := days / 365.25
	cagr := 0.0
	if years > 0 && finalEquity > 0 {
		cagr = (math.Pow(finalEquity/params.InitialCapital, 1.0/years) - 1.0) * 100.0
	}

	// Win Rate & Profit Factor & Average Holding Days
	totalTrades := len(closedTrades)
	wins := 0
	grossProfits := 0.0
	grossLosses := 0.0
	totalHoldingDays := 0.0

	for _, t := range closedTrades {
		if t.ProfitLoss > 0 {
			wins++
			grossProfits += t.ProfitLoss
		} else {
			grossLosses += math.Abs(t.ProfitLoss)
		}

		d1, _ := time.Parse("2006-01-02", t.EntryDate)
		d2, _ := time.Parse("2006-01-02", t.ExitDate)
		holdDays := d2.Sub(d1).Hours() / 24.0
		if holdDays < 1 {
			holdDays = 1
		}
		totalHoldingDays += holdDays
	}

	winRate := 0.0
	if totalTrades > 0 {
		winRate = (float64(wins) / float64(totalTrades)) * 100.0
	}

	profitFactor := 0.0
	if grossLosses > 0 {
		profitFactor = grossProfits / grossLosses
	} else if grossProfits > 0 {
		profitFactor = 99.0
	}

	avgHoldingDays := 0.0
	if totalTrades > 0 {
		avgHoldingDays = totalHoldingDays / float64(totalTrades)
	}

	// Sharpe Ratio calculation from daily equity returns (assuming 6% annual risk-free rate)
	sharpeRatio := 0.0
	if len(equityCurve) > 1 {
		dailyReturns := make([]float64, len(equityCurve)-1)
		sum := 0.0
		for i := 1; i < len(equityCurve); i++ {
			prevEq := equityCurve[i-1].Equity
			if prevEq > 0 {
				r := (equityCurve[i].Equity - prevEq) / prevEq
				dailyReturns[i-1] = r
				sum += r
			}
		}
		meanDailyReturn := sum / float64(len(dailyReturns))
		var sumSqDiff float64
		for _, r := range dailyReturns {
			diff := r - meanDailyReturn
			sumSqDiff += diff * diff
		}
		stdDevDaily := math.Sqrt(sumSqDiff / float64(len(dailyReturns)-1))
		dailyRf := 0.06 / 252.0
		if stdDevDaily > 0 {
			sharpeRatio = ((meanDailyReturn - dailyRf) / stdDevDaily) * math.Sqrt(252)
		}
	}

	// Benchmark return
	for _, candles := range candlesBySymbol {
		if c, ok := candleLookup[candles[0].Symbol][lastDate]; ok && c.Close > 0 {
			benchmarkLastPrice += c.Close
		}
	}
	benchmarkReturn := 0.0
	if benchmarkStartPrice > 0 {
		benchmarkReturn = ((benchmarkLastPrice - benchmarkStartPrice) / benchmarkStartPrice) * 100.0
	}

	paramsJSON, _ := json.Marshal(params)

	runName := params.Name
	if runName == "" {
		runName = fmt.Sprintf("NIFTY 50 Strategy Backtest (%s - %s)", firstDate, lastDate)
	}

	return &storage.BacktestRun{
		Name:            runName,
		StrategyVersion: "v1.0.0",
		StartDate:       firstDate,
		EndDate:         lastDate,
		InitialCapital:  params.InitialCapital,
		FinalEquity:     math.Round(finalEquity*100) / 100,
		TotalReturn:     math.Round(totalReturn*100) / 100,
		CAGR:            math.Round(cagr*100) / 100,
		MaxDrawdown:     math.Round(maxDrawdown*100) / 100,
		WinRate:         math.Round(winRate*100) / 100,
		ProfitFactor:    math.Round(profitFactor*100) / 100,
		SharpeRatio:     math.Round(sharpeRatio*100) / 100,
		TotalTrades:     totalTrades,
		AvgHoldingDays:  math.Round(avgHoldingDays*10) / 10,
		BenchmarkReturn: math.Round(benchmarkReturn*100) / 100,
		ParametersJSON:  string(paramsJSON),
		Trades:          closedTrades,
		EquityCurve:     equityCurve,
	}, nil
}
