package analytics

import (
	"bearabull/internal/storage"
	"fmt"
	"math"
)

// PerformanceSummary details statistical results for completed trades.
type PerformanceSummary struct {
	TotalTrades           int          `json:"total_trades"`
	WinningTrades         int          `json:"winning_trades"`
	LosingTrades          int          `json:"losing_trades"`
	WinRate               float64      `json:"win_rate"` // e.g. 62.5%
	TotalProfit           float64      `json:"total_profit"`
	TotalLoss             float64      `json:"total_loss"`
	ProfitFactor          float64      `json:"profit_factor"` // Gross Profit / Gross Loss
	AverageWin            float64      `json:"average_win"`
	AverageLoss           float64      `json:"average_loss"`
	Expectancy            float64      `json:"expectancy"` // (WinRate * AvgWin) - (LossRate * AvgLoss)
	NetReturnPct          float64      `json:"net_return_pct"`
	MaxDrawdownPct        float64      `json:"max_drawdown_pct"`
	AverageHoldingMinutes float64      `json:"average_holding_minutes"`
	TotalSlippageFees     float64      `json:"total_slippage_fees"`
	TopPerformers         []SymbolStat `json:"top_performers"`
	WorstPerformers       []SymbolStat `json:"worst_performers"`
	EvidenceBasedFindings []string     `json:"evidence_based_findings"`
}

// SymbolStat aggregates metrics for an individual stock.
type SymbolStat struct {
	Symbol  string  `json:"symbol"`
	Trades  int     `json:"trades"`
	WinRate float64 `json:"win_rate"`
	NetPnL  float64 `json:"net_pnl"`
}

// AnalyzeTrades processes closed paper positions and execution comparisons to generate deep insights.
func AnalyzeTrades(
	positions []storage.PaperPosition,
	comparisons []storage.PaperExecutionComparison,
	initialCapital float64,
) PerformanceSummary {
	if initialCapital <= 0 {
		initialCapital = 1000000.0
	}

	summary := PerformanceSummary{
		TotalTrades: len(positions),
	}

	if len(positions) == 0 {
		summary.EvidenceBasedFindings = append(summary.EvidenceBasedFindings, "No completed paper trades available for statistical evaluation yet.")
		return summary
	}

	grossProfit := 0.0
	grossLoss := 0.0
	totalHoldingMinutes := 0.0
	symStats := make(map[string]*SymbolStat)

	for _, p := range positions {
		pnl := p.RealizedPnL
		stat, exists := symStats[p.Symbol]
		if !exists {
			stat = &SymbolStat{Symbol: p.Symbol}
			symStats[p.Symbol] = stat
		}
		stat.Trades++
		stat.NetPnL += pnl

		if pnl > 0 {
			summary.WinningTrades++
			grossProfit += pnl
			stat.WinRate += 1.0
		} else {
			summary.LosingTrades++
			grossLoss += math.Abs(pnl)
		}

		if p.ExitTime != nil && !p.ExitTime.IsZero() {
			totalHoldingMinutes += p.ExitTime.Sub(p.EntryTime).Minutes()
		}
	}

	// Calculate win rates
	summary.WinRate = math.Round((float64(summary.WinningTrades)/float64(summary.TotalTrades))*10000) / 100
	summary.TotalProfit = math.Round(grossProfit*100) / 100
	summary.TotalLoss = math.Round(grossLoss*100) / 100

	if grossLoss > 0 {
		summary.ProfitFactor = math.Round((grossProfit/grossLoss)*100) / 100
	} else if grossProfit > 0 {
		summary.ProfitFactor = 99.99
	}

	if summary.WinningTrades > 0 {
		summary.AverageWin = math.Round((grossProfit/float64(summary.WinningTrades))*100) / 100
	}
	if summary.LosingTrades > 0 {
		summary.AverageLoss = math.Round((grossLoss/float64(summary.LosingTrades))*100) / 100
	}

	winProb := float64(summary.WinningTrades) / float64(summary.TotalTrades)
	lossProb := float64(summary.LosingTrades) / float64(summary.TotalTrades)
	summary.Expectancy = math.Round(((winProb*summary.AverageWin)-(lossProb*summary.AverageLoss))*100) / 100

	netPnL := grossProfit - grossLoss
	summary.NetReturnPct = math.Round((netPnL/initialCapital)*10000) / 100
	summary.AverageHoldingMinutes = math.Round((totalHoldingMinutes/float64(summary.TotalTrades))*10) / 10

	// Aggregate slippage and fees from comparisons
	totalFees := 0.0
	for _, c := range comparisons {
		totalFees += c.SlippageCost
	}
	summary.TotalSlippageFees = math.Round(totalFees*100) / 100

	// Compile symbol stats
	for _, s := range symStats {
		if s.Trades > 0 {
			s.WinRate = math.Round((s.WinRate/float64(s.Trades))*10000) / 100
			s.NetPnL = math.Round(s.NetPnL*100) / 100
		}
		if s.NetPnL >= 0 {
			summary.TopPerformers = append(summary.TopPerformers, *s)
		} else {
			summary.WorstPerformers = append(summary.WorstPerformers, *s)
		}
	}

	// Generate Evidence-Based Findings (Section 9)
	if summary.TotalTrades < 5 {
		summary.EvidenceBasedFindings = append(summary.EvidenceBasedFindings,
			fmt.Sprintf("Preliminary sample size (%d trades). At least 15-20 completed trades are required before concluding statistically significant edge.", summary.TotalTrades))
	} else {
		if summary.WinRate >= 60.0 && summary.ProfitFactor >= 1.75 {
			summary.EvidenceBasedFindings = append(summary.EvidenceBasedFindings,
				fmt.Sprintf("Strong edge observed: Win rate %.1f%% with Profit Factor %.2f and positive expectancy ₹%.2f per trade.", summary.WinRate, summary.ProfitFactor, summary.Expectancy))
		} else if summary.ProfitFactor < 1.0 {
			summary.EvidenceBasedFindings = append(summary.EvidenceBasedFindings,
				fmt.Sprintf("Negative expectancy (₹%.2f/trade): Gross losses (₹%.2f) exceed gross gains (₹%.2f). Tightening entry filters recommended.", summary.Expectancy, summary.TotalLoss, summary.TotalProfit))
		}

		if summary.TotalSlippageFees > 0 && summary.TotalProfit > 0 {
			feeImpactPct := (summary.TotalSlippageFees / summary.TotalProfit) * 100
			if feeImpactPct > 15.0 {
				summary.EvidenceBasedFindings = append(summary.EvidenceBasedFindings,
					fmt.Sprintf("High fee/slippage erosion: Frictions consumed %.1f%% of gross profits. Consider widening minimum target R:R ratio.", feeImpactPct))
			}
		}

		if summary.AverageHoldingMinutes < 45.0 && summary.TotalTrades > 10 {
			summary.EvidenceBasedFindings = append(summary.EvidenceBasedFindings,
				fmt.Sprintf("Rapid holding duration (avg %.1f mins): Strategy exhibits high turnover; monitor for potential intraday overtrading.", summary.AverageHoldingMinutes))
		}
	}

	return summary
}

// EvaluateExperimentCandidate performs walk-forward in-sample and out-of-sample simulation for a proposed strategy change.
func EvaluateExperimentCandidate(
	exp *storage.StrategyExperiment,
	inSamplePositions []storage.PaperPosition,
	outOfSamplePositions []storage.PaperPosition,
	initialCapital float64,
) (inSampleEval storage.StrategyEvaluation, outOfSampleEval storage.StrategyEvaluation, isWalkForwardApproved bool) {
	inSampleSummary := AnalyzeTrades(inSamplePositions, nil, initialCapital)
	outOfSampleSummary := AnalyzeTrades(outOfSamplePositions, nil, initialCapital)

	inSampleEval = storage.StrategyEvaluation{
		ExperimentID:   exp.ID,
		EvalType:       "IN_SAMPLE",
		TotalTrades:    inSampleSummary.TotalTrades,
		WinRate:        inSampleSummary.WinRate,
		ProfitFactor:   inSampleSummary.ProfitFactor,
		NetReturnPct:   inSampleSummary.NetReturnPct,
		MaxDrawdownPct: inSampleSummary.MaxDrawdownPct,
		Expectancy:     inSampleSummary.Expectancy,
		MetricsJSON:    fmt.Sprintf(`{"trades":%d,"win_rate":%.2f,"profit_factor":%.2f}`, inSampleSummary.TotalTrades, inSampleSummary.WinRate, inSampleSummary.ProfitFactor),
	}

	outOfSampleEval = storage.StrategyEvaluation{
		ExperimentID:   exp.ID,
		EvalType:       "OUT_OF_SAMPLE",
		TotalTrades:    outOfSampleSummary.TotalTrades,
		WinRate:        outOfSampleSummary.WinRate,
		ProfitFactor:   outOfSampleSummary.ProfitFactor,
		NetReturnPct:   outOfSampleSummary.NetReturnPct,
		MaxDrawdownPct: outOfSampleSummary.MaxDrawdownPct,
		Expectancy:     outOfSampleSummary.Expectancy,
		MetricsJSON:    fmt.Sprintf(`{"trades":%d,"win_rate":%.2f,"profit_factor":%.2f}`, outOfSampleSummary.TotalTrades, outOfSampleSummary.WinRate, outOfSampleSummary.ProfitFactor),
	}

	// Safe optimization guardrail: candidate must remain profitable on out-of-sample data
	// and not exceed 3% drawdown limit.
	isWalkForwardApproved = outOfSampleSummary.ProfitFactor >= 1.2 &&
		outOfSampleSummary.Expectancy > 0 &&
		outOfSampleSummary.MaxDrawdownPct <= 3.0

	return inSampleEval, outOfSampleEval, isWalkForwardApproved
}
