package autotrader

import (
	"bearabull/internal/analytics"
	"bearabull/internal/execution"
	"bearabull/internal/marketdata"
	"bearabull/internal/risk"
	"bearabull/internal/screener"
	"bearabull/internal/storage"
	"context"
	"fmt"
	"log"
	"math"
	"sync"
	"time"
)

// Scheduler coordinates autonomous 15-minute scans, paper trade executions, and market reconciliation.
type Scheduler struct {
	db             *storage.DB
	provider       marketdata.Provider
	calendar       *NSEHolidayCalendar
	ranker         *screener.OpportunityRanker
	riskEngine     *risk.Engine
	decisionEngine *execution.DecisionEngine
	executor       *execution.PaperExecutor
	cycleMu        sync.Mutex
	isRunning      bool
	stopChan       chan struct{}
	lastScanAt     *time.Time
	nextScanAt     *time.Time
	maxConcurrency int
}

// NewScheduler creates an autonomous trading scheduler.
func NewScheduler(
	db *storage.DB,
	provider marketdata.Provider,
	maxConcurrency int,
) *Scheduler {
	if maxConcurrency <= 0 {
		maxConcurrency = 5
	}
	calendar := NewNSECalendar()
	riskCfg := risk.DefaultConfig()
	riskEng := risk.NewEngine(riskCfg)
	decEng := execution.NewDecisionEngine(riskEng)
	exec := execution.NewPaperExecutor(db)
	ranker := screener.NewOpportunityRanker(70.0)

	s := &Scheduler{
		db:             db,
		provider:       provider,
		calendar:       calendar,
		ranker:         ranker,
		riskEngine:     riskEng,
		decisionEngine: decEng,
		executor:       exec,
		stopChan:       make(chan struct{}),
		maxConcurrency: maxConcurrency,
	}

	// Perform recovery check on initialization
	s.recoverUnfinishedCycles()
	return s
}

// recoverUnfinishedCycles marks any crashed cycles from prior process runs as failed.
func (s *Scheduler) recoverUnfinishedCycles() {
	query := "UPDATE autotrader_cycles SET status = 'FAILED_RECOVERED', completed_at = CURRENT_TIMESTAMP, summary = 'Recovered after application restart' WHERE status = 'RUNNING'"
	_, _ = s.db.Exec(query)
	log.Println("[AUTOTRADER] Startup recovery check completed: cleared any dangling running cycles")
}

// Start begins the 15-minute background polling and execution routine.
func (s *Scheduler) Start(ctx context.Context) {
	s.cycleMu.Lock()
	if s.isRunning {
		s.cycleMu.Unlock()
		return
	}
	s.isRunning = true
	s.cycleMu.Unlock()

	log.Println("[AUTOTRADER] Autonomous 15-minute scheduler started (NSE India regular session)")

	go func() {
		ticker := time.NewTicker(15 * time.Minute)
		defer ticker.Stop()

		// Run an initial evaluation cycle on startup
		now := time.Now().In(s.calendar.Location())
		if active, _ := s.calendar.IsMarketSessionActive(now); active {
			runCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
			_, _ = s.RunCycle(runCtx, false)
			cancel()
		}

		for {
			select {
			case <-s.stopChan:
				log.Println("[AUTOTRADER] Autonomous scheduler stopped")
				return
			case <-ctx.Done():
				return
			case t := <-ticker.C:
				runCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
				_, _ = s.RunCycle(runCtx, false)
				cancel()
				_ = t
			}
		}
	}()
}

// Stop halts the background routine gracefully.
func (s *Scheduler) Stop() {
	s.cycleMu.Lock()
	defer s.cycleMu.Unlock()
	if s.isRunning {
		s.isRunning = false
		close(s.stopChan)
	}
}

// RunCycle executes a full 15-minute scan, scoring, risk check, and execution round.
func (s *Scheduler) RunCycle(ctx context.Context, forceManual bool) (*storage.AutotraderCycle, error) {
	// Prevent concurrent overlapping cycles
	if !s.cycleMu.TryLock() {
		return nil, fmt.Errorf("a scan cycle is currently already running")
	}
	defer s.cycleMu.Unlock()

	now := time.Now().In(s.calendar.Location())
	todayDate := now.Format("2006-01-02")
	currentWeek := now.Format("2006-W02")

	// 1. Reset daily/weekly start equity baselines if date/week rolled over
	account, err := s.db.ResetDailyWeeklyEquityIfNeeded(todayDate, currentWeek)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare account config: %w", err)
	}

	// 2. Check market session activity (unless forceManual is requested)
	isActive, sessionReason := s.calendar.IsMarketSessionActive(now)
	if !isActive && !forceManual {
		log.Printf("[AUTOTRADER] Market session inactive (%s). Skipping autonomous cycle.", sessionReason)
		// Still run portfolio reconciliation using latest observed prices
		_ = s.executor.ReconcilePortfolio(account, nil)
		return nil, nil
	}

	s.lastScanAt = &now
	nextScan := s.calendar.Next15MinBoundary(now)
	s.nextScanAt = &nextScan

	// 3. Create cycle record in database (status: RUNNING)
	cycleRecord := &storage.AutotraderCycle{
		CycleNumber:    int(time.Now().Unix()),
		SessionDate:    todayDate,
		StartedAt:      now,
		Status:         "RUNNING",
		ProviderHealth: "HEALTHY",
	}

	cycleID, err := s.db.CreateAutotraderCycle(cycleRecord)
	if err != nil {
		return nil, fmt.Errorf("failed creating cycle record: %w", err)
	}
	cycleRecord.ID = cycleID

	log.Printf("[AUTOTRADER] Starting Cycle #%d (Date: %s, Time: %s)", cycleID, todayDate, now.Format("15:04:05 IST"))

	// 4. Retrieve Active Stocks Universe
	stocks, err := s.db.GetStocks(true)
	if err != nil || len(stocks) == 0 {
		cycleRecord.Status = "FAILED"
		cycleRecord.Summary = fmt.Sprintf("No active stocks found in universe: %v", err)
		completedAt := time.Now()
		cycleRecord.CompletedAt = &completedAt
		_ = s.db.UpdateAutotraderCycle(cycleRecord)
		return cycleRecord, err
	}

	// 5. Fetch Latest Prices and Candles using bounded concurrency
	candlesBySymbol := make(map[string][]marketdata.Candle)
	observedQuotes := make(map[string]float64)
	freshnessReports := make(map[string]marketdata.FreshnessReport)

	type fetchResult struct {
		symbol    string
		quote     *marketdata.Quote
		candles   []marketdata.Candle
		freshness marketdata.FreshnessReport
		err       error
	}

	resultChan := make(chan fetchResult, len(stocks))
	semaphore := make(chan struct{}, s.maxConcurrency)
	var wg sync.WaitGroup

	for _, stock := range stocks {
		wg.Add(1)
		go func(sym string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			res := fetchResult{symbol: sym}
			q, qErr := s.provider.GetQuote(ctx, sym)
			if qErr != nil {
				res.err = qErr
				resultChan <- res
				return
			}
			res.quote = q

			freshnessDuration := time.Duration(account.FreshnessThresholdSec) * time.Second
			res.freshness = marketdata.EvaluateFreshness(q, freshnessDuration, now)

			// Fetch historical candles for indicators & SMC
			cList, cErr := s.provider.GetHistoricalPrices(ctx, sym, "60d", "1d")
			if cErr == nil {
				res.candles = cList
			}
			resultChan <- res
		}(stock.Symbol)
	}

	wg.Wait()
	close(resultChan)

	errorsCount := 0
	for res := range resultChan {
		if res.err != nil {
			errorsCount++
			continue
		}
		if res.quote != nil {
			observedQuotes[res.symbol] = res.quote.Price
			freshnessReports[res.symbol] = res.freshness

			// Save market observation for price reconciliation history (Section 8)
			_ = s.db.SaveMarketObservation(&storage.MarketObservation{
				Symbol:          res.symbol,
				ObservedAt:      now,
				Price:           res.quote.Price,
				CandleTimestamp: res.quote.ProviderTime.Unix(),
				Interval:        "15m",
				Source:          s.provider.Name(),
				IsStale:         res.freshness.IsStale,
				Volume:          res.quote.Volume,
			})
		}
		if len(res.candles) > 0 {
			candlesBySymbol[res.symbol] = res.candles
		}
	}

	// 6. Check Provider Health
	providerHealth := "HEALTHY"
	if errorsCount > len(stocks)/3 {
		providerHealth = "DEGRADED"
	}
	if errorsCount >= len(stocks) {
		providerHealth = "UNAVAILABLE"
		cycleRecord.Status = "FAILED"
		cycleRecord.Summary = "Data provider unavailable: 100% request errors"
		completedAt := time.Now()
		cycleRecord.CompletedAt = &completedAt
		_ = s.db.UpdateAutotraderCycle(cycleRecord)
		return cycleRecord, fmt.Errorf("provider unavailable")
	}

	// 7. Get Current Open Positions
	openPositions, err := s.db.GetOpenPaperPositions()
	if err != nil {
		log.Printf("[AUTOTRADER] Error fetching open positions: %v", err)
	}
	openSymbols := make(map[string]bool)
	for _, p := range openPositions {
		openSymbols[p.Symbol] = true
	}

	// 8. Rank and Score Opportunities (Section 3)
	bullishOpps, _, topFive := s.ranker.RankConstituents(stocks, candlesBySymbol, openSymbols, account.FreshnessThresholdSec)

	// Save cycle screening results
	var cycleResults []storage.AutotraderCycleResult
	for _, opp := range bullishOpps {
		cycleResults = append(cycleResults, opp.ConvertToStorageResult(cycleID))
	}
	_ = s.db.SaveCycleResults(cycleResults)

	// 9. Generate Decisions (Section 4)
	// Evaluate existing open positions for exits/holds
	positionDecisions := s.decisionEngine.EvaluatePositions(openPositions, observedQuotes)

	// Evaluate top candidates for new entries
	candidateDecisions := s.decisionEngine.EvaluateCandidates(topFive, *account, openPositions, freshnessReports)

	var allDecisions []execution.PlannedDecision
	allDecisions = append(allDecisions, positionDecisions...)
	allDecisions = append(allDecisions, candidateDecisions...)

	// Save decisions to DB
	var storageDecisions []storage.AutotraderDecision
	for _, d := range allDecisions {
		storageDecisions = append(storageDecisions, storage.AutotraderDecision{
			CycleID:          cycleID,
			Symbol:           d.Symbol,
			Decision:         string(d.Type),
			CurrentPrice:     d.CurrentPrice,
			EntryTargetPrice: d.EntryPrice,
			StopLoss:         d.StopLoss,
			TakeProfit:       d.TargetPrice,
			Shares:           d.Shares,
			PlannedRisk:      d.PlannedRisk,
			Rationale:        d.Rationale,
			RiskChecksPassed: d.RiskChecksPassed,
			RejectionReason:  d.RejectionReason,
		})
	}
	_ = s.db.SaveDecisions(storageDecisions)

	// 10. Execute Decisions (Section 5)
	// Paper execution occurs if autonomous trading is enabled
	ordersSubmitted := 0
	ordersFilled := 0
	if account.IsEnabled {
		execResult := s.executor.ExecuteDecisions(cycleID, allDecisions, account, observedQuotes, now)
		ordersSubmitted = execResult.OrdersSubmitted
		ordersFilled = execResult.OrdersFilled
		for _, execErr := range execResult.Errors {
			log.Printf("[AUTOTRADER] Execution warning: %s", execErr)
		}
	} else {
		log.Println("[AUTOTRADER] Execution engine is disabled. Decisions recorded in audit log without order placement.")
		// Still reconcile existing positions
		_ = s.executor.ReconcilePortfolio(account, observedQuotes)
	}

	// 11. Finalize Cycle Record
	completedTime := time.Now()
	cycleRecord.CompletedAt = &completedTime
	cycleRecord.Status = "COMPLETED"
	cycleRecord.StocksScanned = len(stocks)
	cycleRecord.ShortlistedCount = len(topFive)
	cycleRecord.OrdersSubmitted = ordersSubmitted
	cycleRecord.OrdersFilled = ordersFilled
	cycleRecord.ErrorsCount = errorsCount
	cycleRecord.ProviderHealth = providerHealth
	cycleRecord.Summary = fmt.Sprintf("Cycle completed. Scanned %d stocks, found %d top candidates, submitted %d orders (%d filled). Equity: ₹%.2f",
		len(stocks), len(topFive), ordersSubmitted, ordersFilled, account.CurrentEquity)

	_ = s.db.UpdateAutotraderCycle(cycleRecord)
	log.Printf("[AUTOTRADER] %s", cycleRecord.Summary)

	return cycleRecord, nil
}

// GetStatus returns the current live status for the Auto-Trader dashboard.
func (s *Scheduler) GetStatus() (*storage.AutotraderStatusResponse, error) {
	account, err := s.db.GetAutotraderConfig()
	if err != nil {
		return nil, err
	}

	now := time.Now().In(s.calendar.Location())
	isActive, sessionReason := s.calendar.IsMarketSessionActive(now)

	openPositions, _ := s.db.GetOpenPaperPositions()
	closedPositions, _ := s.db.GetPaperPositions(100, "CLOSED")
	comparisons, _ := s.db.GetExecutionComparisons(100)
	pendingOrders, _ := s.db.GetPendingOrders()

	weeklyMetrics := analytics.ComputeWeeklyMetrics(*account, openPositions, closedPositions, comparisons, 0, now)

	// Check circuit breaker status
	dailyLossPct, weeklyDDPct, _, _ := s.riskEngine.CheckSessionLimits(*account)

	remDailyRisk := math.Max(0.0, account.MaxDailyLossPct-dailyLossPct)

	return &storage.AutotraderStatusResponse{
		Config:                *account,
		IsMarketOpen:          isActive,
		MarketSessionStatus:   sessionReason,
		CurrentServerTime:     now.Format("2006-01-02 15:04:05 IST"),
		LastScanAt:            s.lastScanAt,
		NextScanAt:            s.nextScanAt,
		OpenPositionsCount:    len(openPositions),
		PendingOrdersCount:    len(pendingOrders),
		WeeklyReturnPct:       weeklyMetrics.WeeklyReturnPct,
		WeeklyTargetMinPct:    account.WeeklyTargetMinPct,
		WeeklyTargetMaxPct:    account.WeeklyTargetMaxPct,
		WeeklyRealizedPnL:     weeklyMetrics.RealizedWeeklyPnL,
		WeeklyUnrealizedPnL:   weeklyMetrics.UnrealizedWeeklyPnL,
		WeeklyDrawdownPct:     weeklyDDPct,
		DailyPnLPct:           math.Round(dailyLossPct*100) / 100,
		TotalPlannedRisk:      weeklyMetrics.RiskCurrentlyExposed,
		RemainingDailyRiskPct: math.Round(remDailyRisk*100) / 100,
		ProviderHealth:        "HEALTHY",
		DataFreshnessOK:       true,
	}, nil
}
