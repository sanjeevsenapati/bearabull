package api

import (
	"bearabull/internal/autotrader"
	"bearabull/internal/backtest"
	"bearabull/internal/config"
	"bearabull/internal/indicators"
	"bearabull/internal/jobs"
	"bearabull/internal/marketdata"
	"bearabull/internal/screener"
	"bearabull/internal/signals"
	"bearabull/internal/storage"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Handler contains all dependencies for servicing HTTP API requests.
type Handler struct {
	cfg          *config.Config
	db           *storage.DB
	syncer       *jobs.Syncer
	signalEngine *signals.Engine
	backtestRun  *backtest.Runner
	scheduler    *autotrader.Scheduler
}

// NewHandler creates a new API handler.
func NewHandler(cfg *config.Config, db *storage.DB, syncer *jobs.Syncer) *Handler {
	sigEng := signals.NewEngine(cfg.StrategyVersion)
	return &Handler{
		cfg:          cfg,
		db:           db,
		syncer:       syncer,
		signalEngine: sigEng,
		backtestRun:  backtest.NewRunner(sigEng),
	}
}

// JSON write helper
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// Error write helper
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]interface{}{
		"error":  message,
		"status": status,
		"time":   time.Now().UTC().Format(time.RFC3339),
	})
}

// HealthHandler returns application health status.
func (h *Handler) HealthHandler(w http.ResponseWriter, r *http.Request) {
	syncStatus := h.syncer.GetStatus()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":           "healthy",
		"timestamp":        time.Now().UTC().Format(time.RFC3339),
		"version":          "1.0.0",
		"strategy_version": h.cfg.StrategyVersion,
		"database":         "connected",
		"is_syncing":       syncStatus.IsRunning,
		"last_sync":        syncStatus.LastSyncTime,
		"timezone":         h.cfg.AnalysisTimezone,
	})
}

// MarketOverviewHandler delivers consolidated market breadth and opportunities.
func (h *Handler) MarketOverviewHandler(w http.ResponseWriter, r *http.Request) {
	stocks, err := h.db.GetStocks(true)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed fetching stocks")
		return
	}

	signalsList, err := h.db.GetAllLatestWeeklySignals()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed fetching signals")
		return
	}

	pricesMap, _ := h.db.GetAllLatestPrices()

	bullCount := 0
	bearCount := 0
	neutralCount := 0
	insufficientCount := 0

	aboveSMA50Count := 0
	aboveSMA200Count := 0
	totalEvaluated := 0
	var sumChangePct float64

	type MoverItem struct {
		Symbol       string  `json:"symbol"`
		CompanyName  string  `json:"company_name"`
		LastClose    float64 `json:"last_close"`
		WeeklyChange float64 `json:"weekly_change"`
		Signal       string  `json:"signal"`
		Score        float64 `json:"score"`
	}

	var movers []MoverItem
	sectorCounts := make(map[string]map[string]int)

	for _, s := range signalsList {
		switch s.Signal {
		case "BUY":
			bullCount++
		case "SELL":
			bearCount++
		case "HOLD":
			neutralCount++
		default:
			insufficientCount++
		}

		if s.Sector != "" {
			if _, ok := sectorCounts[s.Sector]; !ok {
				sectorCounts[s.Sector] = map[string]int{"BUY": 0, "SELL": 0, "HOLD": 0}
			}
			sectorCounts[s.Sector][s.Signal]++
		}

		ind, _ := h.db.GetLatestIndicators(s.Symbol)
		if ind != nil && s.LastClose > 0 {
			totalEvaluated++
			if ind.SMA50 > 0 && s.LastClose > ind.SMA50 {
				aboveSMA50Count++
			}
			if ind.SMA200 > 0 && s.LastClose > ind.SMA200 {
				aboveSMA200Count++
			}
		}

		sumChangePct += s.WeeklyChangePct
		movers = append(movers, MoverItem{
			Symbol:       s.Symbol,
			CompanyName:  s.CompanyName,
			LastClose:    s.LastClose,
			WeeklyChange: s.WeeklyChangePct,
			Signal:       s.Signal,
			Score:        s.Score,
		})
	}

	// Sort gainers and losers
	sort.Slice(movers, func(i, j int) bool {
		return movers[i].WeeklyChange > movers[j].WeeklyChange
	})

	var topGainers []MoverItem
	var topLosers []MoverItem
	if len(movers) > 0 {
		endG := 5
		if len(movers) < endG {
			endG = len(movers)
		}
		topGainers = movers[:endG]

		startL := len(movers) - 5
		if startL < 0 {
			startL = 0
		}
		for i := len(movers) - 1; i >= startL; i-- {
			topLosers = append(topLosers, movers[i])
		}
	}

	pctAbove50 := 0.0
	pctAbove200 := 0.0
	if totalEvaluated > 0 {
		pctAbove50 = math.Round((float64(aboveSMA50Count)/float64(totalEvaluated))*1000) / 10
		pctAbove200 = math.Round((float64(aboveSMA200Count)/float64(totalEvaluated))*1000) / 10
	}

	avgWeeklyChange := 0.0
	if len(signalsList) > 0 {
		avgWeeklyChange = math.Round((sumChangePct/float64(len(signalsList)))*100) / 100
	}

	syncStatus := h.syncer.GetStatus()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"total_stocks":       len(stocks),
		"bullish_count":      bullCount,
		"bearish_count":      bearCount,
		"neutral_count":      neutralCount,
		"insufficient_count": insufficientCount,
		"market_breadth": map[string]interface{}{
			"pct_above_sma50":     pctAbove50,
			"pct_above_sma200":    pctAbove200,
			"stocks_above_sma50":  aboveSMA50Count,
			"stocks_above_sma200": aboveSMA200Count,
			"evaluated_stocks":    totalEvaluated,
		},
		"index_summary": map[string]interface{}{
			"name":              "NIFTY 50",
			"avg_weekly_change": avgWeeklyChange,
			"active_tracked":    len(pricesMap),
		},
		"top_gainers":      topGainers,
		"top_losers":       topLosers,
		"sector_breakdown": sectorCounts,
		"data_status": map[string]interface{}{
			"last_sync":       syncStatus.LastSyncTime,
			"is_syncing":      syncStatus.IsRunning,
			"is_demo_mode":    syncStatus.IsDemoData,
			"source":          "Yahoo Finance / NSE",
			"market_timezone": h.cfg.AnalysisTimezone,
		},
	})
}

// StocksListHandler returns all configured constituents.
func (h *Handler) StocksListHandler(w http.ResponseWriter, r *http.Request) {
	stocks, err := h.db.GetStocks(false)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed querying stocks")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"stocks": stocks,
		"count":  len(stocks),
	})
}

// StockDetailHandler returns individual stock information with current technical profile.
func (h *Handler) StockDetailHandler(w http.ResponseWriter, r *http.Request) {
	symbol := r.PathValue("symbol")
	if symbol == "" {
		writeError(w, http.StatusBadRequest, "symbol is required")
		return
	}

	stock, err := h.db.GetStock(symbol)
	if err != nil || stock == nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("stock %s not found", symbol))
		return
	}

	sig, _ := h.db.GetLatestWeeklySignal(stock.Symbol)
	ind, _ := h.db.GetLatestIndicators(stock.Symbol)
	latestPrice, _ := h.db.GetLatestPrice(stock.Symbol)
	inWatchlist, _ := h.db.IsInWatchlist("Default", stock.Symbol)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"stock":        stock,
		"latest_price": latestPrice,
		"signal":       sig,
		"indicators":   ind,
		"in_watchlist": inWatchlist,
	})
}

// StockHistoryHandler returns OHLCV candles formatted for TradingView charts.
func (h *Handler) StockHistoryHandler(w http.ResponseWriter, r *http.Request) {
	symbol := r.PathValue("symbol")
	if symbol == "" {
		writeError(w, http.StatusBadRequest, "symbol is required")
		return
	}

	limit := 500
	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 {
			limit = l
		}
	}

	prices, err := h.db.GetDailyPrices(symbol, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed fetching price history")
		return
	}

	// Format for Lightweight Charts: {time: "YYYY-MM-DD", open, high, low, close, volume}
	type ChartBar struct {
		Time   string  `json:"time"`
		Open   float64 `json:"open"`
		High   float64 `json:"high"`
		Low    float64 `json:"low"`
		Close  float64 `json:"close"`
		Volume int64   `json:"volume"`
	}

	bars := make([]ChartBar, len(prices))
	for i, p := range prices {
		bars[i] = ChartBar{
			Time:   p.Date,
			Open:   p.Open,
			High:   p.High,
			Low:    p.Low,
			Close:  p.Close,
			Volume: p.Volume,
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"symbol": symbol,
		"count":  len(bars),
		"bars":   bars,
	})
}

// StockIndicatorsHandler returns calculated indicators for a stock.
func (h *Handler) StockIndicatorsHandler(w http.ResponseWriter, r *http.Request) {
	symbol := r.PathValue("symbol")
	if symbol == "" {
		writeError(w, http.StatusBadRequest, "symbol is required")
		return
	}

	ind, err := h.db.GetLatestIndicators(symbol)
	if err != nil || ind == nil {
		writeError(w, http.StatusNotFound, "indicators not found")
		return
	}

	writeJSON(w, http.StatusOK, ind)
}

// StockSignalsHandler returns historical signals for a symbol.
func (h *Handler) StockSignalsHandler(w http.ResponseWriter, r *http.Request) {
	symbol := r.PathValue("symbol")
	if symbol == "" {
		writeError(w, http.StatusBadRequest, "symbol is required")
		return
	}

	history, err := h.db.GetSignalHistory(symbol, 20)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed querying signal history")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"symbol":  symbol,
		"signals": history,
	})
}

// WeeklySignalsHandler returns the latest weekly signal table for all stocks.
func (h *Handler) WeeklySignalsHandler(w http.ResponseWriter, r *http.Request) {
	signalsList, err := h.db.GetAllLatestWeeklySignals()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed querying weekly signals")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"signals":   signalsList,
		"count":     len(signalsList),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// ScreenerHandler executes multi-factor filtering and ranking.
func (h *Handler) ScreenerHandler(w http.ResponseWriter, r *http.Request) {
	signalsList, err := h.db.GetAllLatestWeeklySignals()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed loading signals")
		return
	}

	var items []screener.ScreenerItem
	for _, sig := range signalsList {
		ind, _ := h.db.GetLatestIndicators(sig.Symbol)
		stock, _ := h.db.GetStock(sig.Symbol)

		ticker := sig.Symbol
		if stock != nil {
			ticker = stock.Ticker
		}

		item := screener.ScreenerItem{
			Symbol:          sig.Symbol,
			Ticker:          ticker,
			CompanyName:     sig.CompanyName,
			Sector:          sig.Sector,
			LastClose:       sig.LastClose,
			WeeklyChangePct: sig.WeeklyChangePct,
			Signal:          sig.Signal,
			Score:           sig.Score,
			BullishScore:    sig.BullishScore,
			BearishScore:    sig.BearishScore,
			TrendClass:      sig.TrendClass,
			MomentumClass:   sig.MomentumClass,
			VolatilityClass: sig.VolatilityClass,
			SignalDate:      sig.SignalDate,
			Rationale:       sig.Rationale,
			StopLoss:        sig.StopLoss,
			RiskReward:      sig.RiskReward,
		}

		if ind != nil {
			item.RSI = ind.RSI14
			item.RVOL = ind.RVOL
			item.ATR = ind.ATR14
			item.SupportLevel = ind.SupportLevel
			item.ResistLevel = ind.ResistLevel
			item.HistVolatility = ind.HistVolatility
			item.IsBreakout = ind.RVOL >= 1.25 && item.LastClose >= ind.SwingHigh
			item.IsPullback = sig.TrendClass == "Bullish" && ind.RSI14 >= 40 && ind.RSI14 <= 55
		}

		items = append(items, item)
	}

	q := r.URL.Query()
	minScore, _ := strconv.ParseFloat(q.Get("min_score"), 64)
	minPrice, _ := strconv.ParseFloat(q.Get("min_price"), 64)
	maxPrice, _ := strconv.ParseFloat(q.Get("max_price"), 64)

	filter := screener.ScreenerFilter{
		Preset:    q.Get("preset"),
		Signal:    q.Get("signal"),
		Sector:    q.Get("sector"),
		MinScore:  minScore,
		RiskLevel: q.Get("risk_level"),
		MinPrice:  minPrice,
		MaxPrice:  maxPrice,
		SortBy:    q.Get("sort_by"),
		SortOrder: q.Get("sort_order"),
	}

	ranked := screener.FilterAndRank(items, filter)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items":  ranked,
		"count":  len(ranked),
		"filter": filter,
	})
}

// SectorsHandler returns list of unique sectors.
func (h *Handler) SectorsHandler(w http.ResponseWriter, r *http.Request) {
	sectors, err := h.db.GetSectors()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed querying sectors")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"sectors": sectors,
	})
}

// WatchlistGetHandler returns the user's watchlist items with latest technical overview.
func (h *Handler) WatchlistGetHandler(w http.ResponseWriter, r *http.Request) {
	wl, err := h.db.GetWatchlist("Default")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed getting watchlist")
		return
	}

	type WatchlistItem struct {
		Stock      storage.Stock                `json:"stock"`
		Signal     *storage.WeeklySignal        `json:"signal"`
		Indicators *storage.TechnicalIndicators `json:"indicators"`
		LastPrice  *storage.DailyPrice          `json:"last_price"`
	}

	var items []WatchlistItem
	for _, sym := range wl.Symbols {
		stock, _ := h.db.GetStock(sym)
		if stock == nil {
			continue
		}
		sig, _ := h.db.GetLatestWeeklySignal(sym)
		ind, _ := h.db.GetLatestIndicators(sym)
		price, _ := h.db.GetLatestPrice(sym)

		items = append(items, WatchlistItem{
			Stock:      *stock,
			Signal:     sig,
			Indicators: ind,
			LastPrice:  price,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"watchlist": wl.Name,
		"count":     len(items),
		"items":     items,
	})
}

// WatchlistAddHandler adds a stock symbol to the user's watchlist.
func (h *Handler) WatchlistAddHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Symbol string `json:"symbol"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Symbol == "" {
		writeError(w, http.StatusBadRequest, "valid symbol is required")
		return
	}

	// Normalize symbol if user typed just ticker (e.g. "RELIANCE")
	sym := body.Symbol
	if !strings.HasSuffix(sym, ".NS") {
		sym = sym + ".NS"
	}

	if err := h.db.AddToWatchlist("Default", sym); err != nil {
		writeError(w, http.StatusInternalServerError, "failed adding to watchlist: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "added to watchlist",
		"symbol":  sym,
	})
}

// WatchlistDeleteHandler removes a stock symbol from watchlist.
func (h *Handler) WatchlistDeleteHandler(w http.ResponseWriter, r *http.Request) {
	symbol := r.PathValue("symbol")
	if symbol == "" {
		writeError(w, http.StatusBadRequest, "symbol is required")
		return
	}

	if !strings.HasSuffix(symbol, ".NS") {
		symbol = symbol + ".NS"
	}

	if err := h.db.RemoveFromWatchlist("Default", symbol); err != nil {
		writeError(w, http.StatusInternalServerError, "failed removing from watchlist: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "removed from watchlist",
		"symbol":  symbol,
	})
}

// BacktestsListHandler lists historical backtest simulation runs.
func (h *Handler) BacktestsListHandler(w http.ResponseWriter, r *http.Request) {
	runs, err := h.db.GetBacktestRuns(50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed querying backtest runs")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"runs":  runs,
		"count": len(runs),
	})
}

// BacktestDetailHandler retrieves specific backtest run with trades and equity curve.
func (h *Handler) BacktestDetailHandler(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid backtest ID")
		return
	}

	run, err := h.db.GetBacktestRunDetails(id)
	if err != nil || run == nil {
		writeError(w, http.StatusNotFound, "backtest run not found")
		return
	}

	writeJSON(w, http.StatusOK, run)
}

// BacktestRunHandler executes a new backtest simulation with user-defined parameters.
func (h *Handler) BacktestRunHandler(w http.ResponseWriter, r *http.Request) {
	var params backtest.Params
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		// Use defaults if empty body
		params = backtest.Params{}
	}

	stocks, err := h.db.GetStocks(true)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed fetching stocks")
		return
	}

	stockMap := make(map[string]storage.Stock)
	for _, s := range stocks {
		stockMap[s.Symbol] = s
	}

	// Load historical daily prices for active stocks
	candlesBySymbol := make(map[string][]marketdata.Candle)
	for _, s := range stocks {
		prices, _ := h.db.GetDailyPrices(s.Symbol, 500)
		if len(prices) >= 50 {
			candlesBySymbol[s.Symbol] = indicators.CandlesFromPrices(prices)
		}
	}

	if len(candlesBySymbol) == 0 {
		writeError(w, http.StatusBadRequest, "no historical price data available. Please trigger market sync first.")
		return
	}

	runResult, err := h.backtestRun.Run(params, stockMap, candlesBySymbol)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "backtest execution error: "+err.Error())
		return
	}

	// Persist run
	runID, err := h.db.SaveBacktestRun(runResult)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed saving backtest result: "+err.Error())
		return
	}

	runResult.ID = runID
	writeJSON(w, http.StatusCreated, runResult)
}

// SyncStatusHandler returns current sync status.
func (h *Handler) SyncStatusHandler(w http.ResponseWriter, r *http.Request) {
	status := h.syncer.GetStatus()
	writeJSON(w, http.StatusOK, status)
}

// SyncTriggerHandler kicks off background data synchronization.
func (h *Handler) SyncTriggerHandler(w http.ResponseWriter, r *http.Request) {
	if h.syncer.IsRunning() {
		writeJSON(w, http.StatusConflict, map[string]string{
			"message": "sync is already in progress",
		})
		return
	}

	forceDemo := r.URL.Query().Get("demo") == "true"

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		_ = h.syncer.Run(ctx, forceDemo)
	}()

	writeJSON(w, http.StatusAccepted, map[string]string{
		"message": "synchronization started in background",
		"status":  "running",
	})
}
