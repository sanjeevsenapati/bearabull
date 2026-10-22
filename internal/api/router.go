package api

import (
	"bearabull/internal/config"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

var requestSeq uint64

// NewRouter constructs the HTTP router with middlewares and route mappings.
func NewRouter(cfg *config.Config, handler *Handler, staticFS fs.FS) http.Handler {
	mux := http.NewServeMux()

	// 1. REST API v1 Endpoints
	mux.HandleFunc("GET /api/v1/health", handler.HealthHandler)
	mux.HandleFunc("GET /api/v1/ready", handler.HealthHandler)
	mux.HandleFunc("GET /api/v1/market/overview", handler.MarketOverviewHandler)
	mux.HandleFunc("GET /api/v1/stocks", handler.StocksListHandler)
	mux.HandleFunc("GET /api/v1/stocks/{symbol}", handler.StockDetailHandler)
	mux.HandleFunc("GET /api/v1/stocks/{symbol}/history", handler.StockHistoryHandler)
	mux.HandleFunc("GET /api/v1/stocks/{symbol}/indicators", handler.StockIndicatorsHandler)
	mux.HandleFunc("GET /api/v1/stocks/{symbol}/signals", handler.StockSignalsHandler)
	mux.HandleFunc("GET /api/v1/signals/weekly", handler.WeeklySignalsHandler)
	mux.HandleFunc("GET /api/v1/screener", handler.ScreenerHandler)
	mux.HandleFunc("GET /api/v1/sectors", handler.SectorsHandler)
	mux.HandleFunc("GET /api/v1/watchlist", handler.WatchlistGetHandler)
	mux.HandleFunc("POST /api/v1/watchlist", handler.WatchlistAddHandler)
	mux.HandleFunc("DELETE /api/v1/watchlist/{symbol}", handler.WatchlistDeleteHandler)
	mux.HandleFunc("GET /api/v1/backtests", handler.BacktestsListHandler)
	mux.HandleFunc("GET /api/v1/backtests/{id}", handler.BacktestDetailHandler)
	mux.HandleFunc("POST /api/v1/backtests", handler.BacktestRunHandler)
	mux.HandleFunc("GET /api/v1/sync/status", handler.SyncStatusHandler)
	mux.HandleFunc("POST /api/v1/sync/trigger", handler.SyncTriggerHandler)

	// Autotrader Endpoints (Section 11)
	mux.HandleFunc("GET /api/v1/autotrader/status", handler.AutotraderStatusHandler)
	mux.HandleFunc("POST /api/v1/autotrader/enable", handler.AutotraderEnableHandler)
	mux.HandleFunc("POST /api/v1/autotrader/disable", handler.AutotraderDisableHandler)
	mux.HandleFunc("POST /api/v1/autotrader/pause", handler.AutotraderPauseHandler)
	mux.HandleFunc("POST /api/v1/autotrader/resume", handler.AutotraderResumeHandler)
	mux.HandleFunc("POST /api/v1/autotrader/scan", handler.AutotraderScanHandler)
	mux.HandleFunc("GET /api/v1/autotrader/cycles", handler.AutotraderCyclesHandler)
	mux.HandleFunc("GET /api/v1/autotrader/opportunities", handler.AutotraderOpportunitiesHandler)
	mux.HandleFunc("GET /api/v1/autotrader/performance", handler.AutotraderPerformanceHandler)
	mux.HandleFunc("GET /api/v1/autotrader/reconciliation", handler.AutotraderReconciliationHandler)
	mux.HandleFunc("GET /api/v1/autotrader/risk", handler.AutotraderRiskHandler)
	mux.HandleFunc("GET /api/v1/autotrader/insights", handler.AutotraderInsightsHandler)
	mux.HandleFunc("GET /api/v1/autotrader/experiments", handler.AutotraderExperimentsHandler)
	mux.HandleFunc("POST /api/v1/autotrader/experiments/{id}/approve", handler.AutotraderExperimentApproveHandler)

	// Admin API check for operational trigger
	mux.HandleFunc("POST /api/v1/analysis/run", func(w http.ResponseWriter, r *http.Request) {
		if cfg.AdminAuthEnabled {
			key := r.Header.Get("X-Admin-Key")
			if key != cfg.AdminAPIKey {
				writeError(w, http.StatusUnauthorized, "unauthorized: invalid admin key")
				return
			}
		}
		handler.SyncTriggerHandler(w, r)
	})

	// 2. Static Web UI Files
	if staticFS != nil {
		fileServer := http.FileServer(http.FS(staticFS))
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path
			// If request is for an API endpoint that didn't match, return 404 JSON
			if strings.HasPrefix(path, "/api/") {
				writeError(w, http.StatusNotFound, "API endpoint not found")
				return
			}
			fileServer.ServeHTTP(w, r)
		})
	}

	// 3. Middlewares: Recovery, Security, CORS, Logging
	return applyMiddlewares(mux)
}

func applyMiddlewares(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := atomic.AddUint64(&requestSeq, 1)
		start := time.Now()

		// Set Security & CORS Headers
		w.Header().Set("X-Request-ID", fmt.Sprintf("req-%d", reqID))
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Admin-Key, X-Request-ID")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// Panic recovery
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("[PANIC] [req-%d] %v", reqID, rec)
				writeError(w, http.StatusInternalServerError, "internal server error")
			}
		}()

		recorder := &responseRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(recorder, r)

		duration := time.Since(start)
		if strings.HasPrefix(r.URL.Path, "/api/") {
			log.Printf("[HTTP] [req-%d] %s %s -> %d (%s)", reqID, r.Method, r.URL.Path, recorder.statusCode, duration)
		}
	})
}

type responseRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *responseRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}
