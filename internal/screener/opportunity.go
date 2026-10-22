package screener

import (
	"bearabull/internal/indicators"
	"bearabull/internal/marketdata"
	"bearabull/internal/orderblocks"
	"bearabull/internal/storage"
	"encoding/json"
	"fmt"
	"math"
	"sort"
)

// CandidateOpportunity represents a fully scored and filtered trading candidate.
type CandidateOpportunity struct {
	Symbol          string                   `json:"symbol"`
	CompanyName     string                   `json:"company_name"`
	Sector          string                   `json:"sector"`
	CurrentPrice    float64                  `json:"current_price"`
	Direction       string                   `json:"direction"` // "LONG" or "SHORT"
	Score           float64                  `json:"score"`     // 0 - 100
	BullishScore    float64                  `json:"bullish_score"`
	BearishScore    float64                  `json:"bearish_score"`
	Rank            int                      `json:"rank"`
	Action          string                   `json:"action"` // "QUALIFIED", "REJECTED", "WATCH"
	PassedFilters   bool                     `json:"passed_filters"`
	RejectionReason string                   `json:"rejection_reason,omitempty"`
	EntryPrice      float64                  `json:"entry_price"`
	StopLoss        float64                  `json:"stop_loss"`
	TargetPrice     float64                  `json:"target_price"`
	RiskRewardRatio float64                  `json:"risk_reward_ratio"`
	StopLossPct     float64                  `json:"stop_loss_pct"`
	FreshnessScore  float64                  `json:"freshness_score"`
	HasFreshOB      bool                     `json:"has_fresh_ob"`
	HasBOS          bool                     `json:"has_bos"`
	HasCHoCH        bool                     `json:"has_choch"`
	RVOL            float64                  `json:"rvol"`
	RSI             float64                  `json:"rsi"`
	Rationale       string                   `json:"rationale"`
	SMCAnalysis     *orderblocks.SMCAnalysis `json:"smc_analysis,omitempty"`
	Indicators      *indicators.IndicatorSet `json:"indicators,omitempty"`
}

// OpportunityRanker evaluates universe stocks and produces ranked opportunities.
type OpportunityRanker struct {
	smcDetector *orderblocks.Detector
	minScore    float64
}

// NewOpportunityRanker creates a candidate ranking instance.
func NewOpportunityRanker(minScore float64) *OpportunityRanker {
	if minScore <= 0 {
		minScore = 70.0
	}
	return &OpportunityRanker{
		smcDetector: orderblocks.NewDetector(3),
		minScore:    minScore,
	}
}

// RankConstituents scores all stocks and separates bullish and bearish setups.
func (r *OpportunityRanker) RankConstituents(
	stocks []storage.Stock,
	candlesBySymbol map[string][]marketdata.Candle,
	openSymbols map[string]bool,
	minFreshnessThresholdSec int,
) ([]CandidateOpportunity, []CandidateOpportunity, []CandidateOpportunity) {
	var bullishList []CandidateOpportunity
	var bearishList []CandidateOpportunity

	for _, stock := range stocks {
		candles := candlesBySymbol[stock.Symbol]
		if len(candles) < 30 {
			continue
		}

		opp := r.scoreStock(stock, candles)

		// Check if position already open for this stock
		if openSymbols[stock.Symbol] {
			opp.PassedFilters = false
			opp.Action = "WATCH"
			opp.RejectionReason = "Active position or pending order already exists for this symbol"
		}

		if opp.Direction == "LONG" {
			bullishList = append(bullishList, opp)
		} else if opp.Direction == "SHORT" {
			bearishList = append(bearishList, opp)
		}
	}

	// Sort bullish opportunities by Score descending
	sort.Slice(bullishList, func(i, j int) bool {
		return bullishList[i].Score > bullishList[j].Score
	})
	for i := range bullishList {
		bullishList[i].Rank = i + 1
	}

	// Sort bearish opportunities by Score descending
	sort.Slice(bearishList, func(i, j int) bool {
		return bearishList[i].Score > bearishList[j].Score
	})
	for i := range bearishList {
		bearishList[i].Rank = i + 1
	}

	// Top candidates (up to 5 strongest)
	var topCandidates []CandidateOpportunity
	for _, opp := range bullishList {
		if len(topCandidates) >= 5 {
			break
		}
		topCandidates = append(topCandidates, opp)
	}

	return bullishList, bearishList, topCandidates
}

func (r *OpportunityRanker) scoreStock(stock storage.Stock, candles []marketdata.Candle) CandidateOpportunity {
	smc := r.smcDetector.Analyze(stock.Symbol, candles)
	ind := indicators.ComputeAllIndicators(candles)

	opp := CandidateOpportunity{
		Symbol:       stock.Symbol,
		CompanyName:  stock.CompanyName,
		Sector:       stock.Sector,
		CurrentPrice: smc.CurrentPrice,
		Direction:    "LONG", // Default long-only per spec
		SMCAnalysis:  smc,
		Indicators:   ind,
	}

	if ind == nil || smc == nil {
		opp.Score = 0
		opp.PassedFilters = false
		opp.Action = "REJECTED"
		opp.RejectionReason = "Indicators or structure could not be calculated"
		return opp
	}

	var score float64
	var rationaleParts []string

	// 1. Multi-Timeframe Trend & Alignment (30 pts max)
	if ind.Close > ind.SMA200 && ind.SMA200 > 0 {
		score += 10
		rationaleParts = append(rationaleParts, "Price above 200 SMA (Bullish Regime)")
	}
	if ind.SMA50 > ind.SMA200 && ind.SMA200 > 0 {
		score += 5
		rationaleParts = append(rationaleParts, "Golden Cross / 50 SMA > 200 SMA")
	}
	if ind.SupertrendDir == 1 {
		score += 10
		rationaleParts = append(rationaleParts, "Supertrend Bullish")
	}
	if ind.ADX14 > 22 && ind.PlusDI14 > ind.MinusDI14 {
		score += 5
		rationaleParts = append(rationaleParts, fmt.Sprintf("Strong Bullish Trend (ADX %.1f)", ind.ADX14))
	}

	// 2. Smart Money Concepts: Order Blocks & Market Structure (35 pts max)
	if smc.ActiveBullishOB != nil {
		score += 15
		opp.HasFreshOB = !smc.ActiveBullishOB.IsMitigated
		if opp.HasFreshOB {
			score += 5
			rationaleParts = append(rationaleParts, fmt.Sprintf("Fresh Bullish OB at ₹%.2f-₹%.2f", smc.ActiveBullishOB.Bottom, smc.ActiveBullishOB.Top))
		} else {
			rationaleParts = append(rationaleParts, fmt.Sprintf("Mitigated Bullish OB at ₹%.2f-₹%.2f", smc.ActiveBullishOB.Bottom, smc.ActiveBullishOB.Top))
		}
	}

	if smc.LastBOS != nil && smc.LastBOS.Direction == orderblocks.BullishOB {
		score += 10
		opp.HasBOS = true
		rationaleParts = append(rationaleParts, fmt.Sprintf("Bullish BOS broken at ₹%.2f", smc.LastBOS.Level))
	} else if smc.LastCHoCH != nil && smc.LastCHoCH.Direction == orderblocks.BullishOB {
		score += 10
		opp.HasCHoCH = true
		rationaleParts = append(rationaleParts, fmt.Sprintf("Bullish CHoCH reversal at ₹%.2f", smc.LastCHoCH.Level))
	}

	if len(smc.FairValueGaps) > 0 {
		for _, fvg := range smc.FairValueGaps {
			if fvg.Type == orderblocks.BullishOB && !fvg.IsFilled {
				score += 5
				rationaleParts = append(rationaleParts, "Active Bullish FVG imbalance zone")
				break
			}
		}
	}

	// 3. Momentum & Volume (20 pts max)
	opp.RSI = ind.RSI14
	opp.RVOL = ind.RVOL
	if ind.RSI14 >= 45 && ind.RSI14 <= 68 {
		score += 10
		rationaleParts = append(rationaleParts, fmt.Sprintf("RSI in optimal expansion zone (%.1f)", ind.RSI14))
	}
	if ind.MACDHistogram > 0 {
		score += 5
		rationaleParts = append(rationaleParts, "MACD Histogram positive")
	}
	if ind.RVOL >= 1.2 {
		score += 5
		rationaleParts = append(rationaleParts, fmt.Sprintf("High Relative Volume (%.2fx)", ind.RVOL))
	}

	// 4. Risk / Reward & Entry Feasibility (15 pts max)
	entryPrice := smc.RecommendedEntry
	if entryPrice <= 0 {
		entryPrice = opp.CurrentPrice
	}
	stopLoss := smc.StopLoss
	if stopLoss <= 0 {
		stopLoss = math.Round(entryPrice*0.96*100) / 100 // fallback 4% stop
	}
	targetPrice := smc.Target
	if targetPrice <= 0 {
		targetPrice = math.Round(entryPrice*1.08*100) / 100 // fallback 8% target
	}

	stopDist := entryPrice - stopLoss
	stopDistPct := 0.0
	if entryPrice > 0 {
		stopDistPct = math.Round((stopDist/entryPrice)*10000) / 100
	}

	rrRatio := 0.0
	if stopDist > 0 {
		rrRatio = math.Round(((targetPrice-entryPrice)/stopDist)*100) / 100
	}

	opp.EntryPrice = entryPrice
	opp.StopLoss = stopLoss
	opp.TargetPrice = targetPrice
	opp.StopLossPct = stopDistPct
	opp.RiskRewardRatio = rrRatio
	opp.FreshnessScore = smc.FreshnessScore

	if rrRatio >= 2.0 {
		score += 10
		rationaleParts = append(rationaleParts, fmt.Sprintf("Favorable Risk/Reward (1:%.2f)", rrRatio))
	} else if rrRatio >= 1.5 {
		score += 5
		rationaleParts = append(rationaleParts, fmt.Sprintf("Acceptable Risk/Reward (1:%.2f)", rrRatio))
	}

	if stopDistPct >= 0.8 && stopDistPct <= 4.5 {
		score += 5
	}

	opp.BullishScore = math.Min(100.0, math.Round(score*10)/10)
	opp.Score = opp.BullishScore

	// Check mandatory qualification filters
	passed := true
	rejection := ""

	if opp.Score < r.minScore {
		passed = false
		rejection = fmt.Sprintf("Score %.1f below minimum threshold %.1f", opp.Score, r.minScore)
	} else if opp.RiskRewardRatio < 1.5 {
		passed = false
		rejection = fmt.Sprintf("Risk/Reward 1:%.2f below 1:1.5 minimum filter", opp.RiskRewardRatio)
	} else if opp.StopLossPct > 5.0 {
		passed = false
		rejection = fmt.Sprintf("Stop loss distance %.2f%% too wide (max 5%%)", opp.StopLossPct)
	} else if opp.StopLossPct < 0.5 {
		passed = false
		rejection = fmt.Sprintf("Stop loss distance %.2f%% too tight (min 0.5%%)", opp.StopLossPct)
	}

	opp.PassedFilters = passed
	opp.RejectionReason = rejection
	if passed {
		opp.Action = "QUALIFIED"
	} else {
		opp.Action = "REJECTED"
	}

	if len(rationaleParts) > 0 {
		opp.Rationale = fmt.Sprintf("%s. Stop: ₹%.2f (%.1f%%), Target: ₹%.2f (R:R 1:%.2f).",
			rationaleParts[0], opp.StopLoss, opp.StopLossPct, opp.TargetPrice, opp.RiskRewardRatio)
		for _, part := range rationaleParts[1:] {
			opp.Rationale += " " + part + "."
		}
	} else {
		opp.Rationale = "Insufficient technical confluence."
	}

	return opp
}

// ConvertToStorageResult converts CandidateOpportunity to storage.AutotraderCycleResult.
func (o *CandidateOpportunity) ConvertToStorageResult(cycleID int64) storage.AutotraderCycleResult {
	indJSON, _ := json.Marshal(o.Indicators)
	smcJSON, _ := json.Marshal(o.SMCAnalysis)

	return storage.AutotraderCycleResult{
		CycleID:         cycleID,
		Symbol:          o.Symbol,
		Score:           o.Score,
		BullishScore:    o.BullishScore,
		BearishScore:    o.BearishScore,
		Rank:            o.Rank,
		Action:          o.Action,
		PassedFilters:   o.PassedFilters,
		RejectionReason: o.RejectionReason,
		IndicatorsJSON:  string(indJSON),
		OrderBlockJSON:  string(smcJSON),
	}
}
