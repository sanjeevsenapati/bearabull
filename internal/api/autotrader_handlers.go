package api

import (
	"bearabull/internal/analytics"
	"bearabull/internal/autotrader"
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// SetAutotraderScheduler binds the scheduler to the API handler.
func (h *Handler) SetAutotraderScheduler(sched *autotrader.Scheduler) {
	h.scheduler = sched
}

func (h *Handler) checkAdminAuth(w http.ResponseWriter, r *http.Request) bool {
	if h.cfg.AdminAuthEnabled {
		key := r.Header.Get("X-Admin-Key")
		if key != h.cfg.AdminAPIKey {
			writeError(w, http.StatusUnauthorized, "unauthorized: invalid admin key")
			return false
		}
	}
	return true
}

// AutotraderStatusHandler returns complete autotrader state.
// GET /api/v1/autotrader/status
func (h *Handler) AutotraderStatusHandler(w http.ResponseWriter, r *http.Request) {
	if h.scheduler == nil {
		writeError(w, http.StatusServiceUnavailable, "autotrader scheduler not initialized")
		return
	}
	status, err := h.scheduler.GetStatus()
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get autotrader status: %v", err))
		return
	}
	writeJSON(w, http.StatusOK, status)
}

// AutotraderEnableHandler enables autonomous trading.
// POST /api/v1/autotrader/enable
func (h *Handler) AutotraderEnableHandler(w http.ResponseWriter, r *http.Request) {
	if !h.checkAdminAuth(w, r) {
		return
	}
	if err := h.db.SetAutotraderEnabled(true); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to enable autotrader")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Autonomous paper trading enabled",
		"enabled": true,
	})
}

// AutotraderDisableHandler disables autonomous trading.
// POST /api/v1/autotrader/disable
func (h *Handler) AutotraderDisableHandler(w http.ResponseWriter, r *http.Request) {
	if !h.checkAdminAuth(w, r) {
		return
	}
	if err := h.db.SetAutotraderEnabled(false); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to disable autotrader")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Autonomous paper trading disabled (market scanning continues)",
		"enabled": false,
	})
}

// AutotraderPauseHandler pauses new trade entries while continuing protective exits.
// POST /api/v1/autotrader/pause
func (h *Handler) AutotraderPauseHandler(w http.ResponseWriter, r *http.Request) {
	if !h.checkAdminAuth(w, r) {
		return
	}
	if err := h.db.SetAutotraderPaused(true); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to pause autotrader")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "New entries paused. Protective stops and targets remain active.",
		"paused":  true,
	})
}

// AutotraderResumeHandler resumes normal entry processing.
// POST /api/v1/autotrader/resume
func (h *Handler) AutotraderResumeHandler(w http.ResponseWriter, r *http.Request) {
	if !h.checkAdminAuth(w, r) {
		return
	}
	if err := h.db.SetAutotraderPaused(false); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to resume autotrader")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Autonomous trade entries resumed",
		"paused":  false,
	})
}

// AutotraderScanHandler triggers an immediate manual scan cycle.
// POST /api/v1/autotrader/scan
func (h *Handler) AutotraderScanHandler(w http.ResponseWriter, r *http.Request) {
	if !h.checkAdminAuth(w, r) {
		return
	}
	if h.scheduler == nil {
		writeError(w, http.StatusServiceUnavailable, "autotrader scheduler not initialized")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()

	cycle, err := h.scheduler.RunCycle(ctx, true)
	if err != nil {
		writeError(w, http.StatusConflict, fmt.Sprintf("scan cycle error: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Manual scan completed successfully",
		"cycle":   cycle,
	})
}

// AutotraderCyclesHandler returns history of scan cycles.
// GET /api/v1/autotrader/cycles
func (h *Handler) AutotraderCyclesHandler(w http.ResponseWriter, r *http.Request) {
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}

	cycles, err := h.db.GetAutotraderCycles(limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch cycles")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"cycles": cycles,
		"count":  len(cycles),
	})
}

// AutotraderOpportunitiesHandler returns the top 5 ranked qualified opportunities.
// GET /api/v1/autotrader/opportunities
func (h *Handler) AutotraderOpportunitiesHandler(w http.ResponseWriter, r *http.Request) {
	opps, err := h.db.GetTopOpportunities(5)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch opportunities")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"opportunities": opps,
		"count":         len(opps),
	})
}

// AutotraderPerformanceHandler returns weekly target tracking and trade analytics.
// GET /api/v1/autotrader/performance
func (h *Handler) AutotraderPerformanceHandler(w http.ResponseWriter, r *http.Request) {
	account, err := h.db.GetAutotraderConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load account config")
		return
	}

	openPositions, _ := h.db.GetOpenPaperPositions()
	closedPositions, _ := h.db.GetPaperPositions(200, "CLOSED")
	comparisons, _ := h.db.GetExecutionComparisons(200)

	now := time.Now()
	weekly := analytics.ComputeWeeklyMetrics(*account, openPositions, closedPositions, comparisons, 0, now)
	stats := analytics.AnalyzeTrades(closedPositions, comparisons, account.InitialCapital)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"weekly":         weekly,
		"overall_stats":  stats,
		"open_positions": openPositions,
		"recent_closed":  closedPositions,
	})
}

// AutotraderReconciliationHandler returns trade execution comparisons vs observed market prices.
// GET /api/v1/autotrader/reconciliation
func (h *Handler) AutotraderReconciliationHandler(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}

	comps, err := h.db.GetExecutionComparisons(limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch reconciliation records")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"reconciliations": comps,
		"count":           len(comps),
	})
}

// AutotraderRiskHandler returns risk settings, portfolio exposure, and risk audit events.
// GET /api/v1/autotrader/risk
func (h *Handler) AutotraderRiskHandler(w http.ResponseWriter, r *http.Request) {
	account, err := h.db.GetAutotraderConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load account config")
		return
	}

	events, _ := h.db.GetRecentRiskEvents(50)
	openPositions, _ := h.db.GetOpenPaperPositions()

	currentAggregateRisk := 0.0
	for _, p := range openPositions {
		riskPerShare := p.EntryPrice - p.StopLoss
		if riskPerShare > 0 {
			currentAggregateRisk += riskPerShare * float64(p.Shares)
		}
	}

	aggregateRiskPct := 0.0
	if account.CurrentEquity > 0 {
		aggregateRiskPct = (currentAggregateRisk / account.CurrentEquity) * 100
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"config":                 account,
		"current_aggregate_risk": currentAggregateRisk,
		"aggregate_risk_pct":     aggregateRiskPct,
		"open_positions_count":   len(openPositions),
		"max_concurrent_allowed": account.MaxPositions,
		"daily_loss_limit_pct":   account.MaxDailyLossPct,
		"weekly_drawdown_limit":  account.MaxWeeklyDrawdownPct,
		"events":                 events,
	})
}

// AutotraderInsightsHandler returns evidence-based findings and strategy analytics.
// GET /api/v1/autotrader/insights
func (h *Handler) AutotraderInsightsHandler(w http.ResponseWriter, r *http.Request) {
	account, err := h.db.GetAutotraderConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load account config")
		return
	}

	closedPositions, _ := h.db.GetPaperPositions(200, "CLOSED")
	comparisons, _ := h.db.GetExecutionComparisons(200)

	stats := analytics.AnalyzeTrades(closedPositions, comparisons, account.InitialCapital)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"summary":             stats,
		"findings":            stats.EvidenceBasedFindings,
		"top_performers":      stats.TopPerformers,
		"worst_performers":    stats.WorstPerformers,
		"total_slippage_fees": stats.TotalSlippageFees,
	})
}

// AutotraderExperimentsHandler lists proposed and approved strategy optimizations.
// GET /api/v1/autotrader/experiments
func (h *Handler) AutotraderExperimentsHandler(w http.ResponseWriter, r *http.Request) {
	experiments, err := h.db.GetStrategyExperiments()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load experiments")
		return
	}

	versions, _ := h.db.GetStrategyVersions()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"experiments":       experiments,
		"strategy_versions": versions,
	})
}

// AutotraderExperimentApproveHandler activates a proposed strategy version after operator approval.
// POST /api/v1/autotrader/experiments/{id}/approve
func (h *Handler) AutotraderExperimentApproveHandler(w http.ResponseWriter, r *http.Request) {
	if !h.checkAdminAuth(w, r) {
		return
	}

	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid experiment id")
		return
	}

	exp, err := h.db.GetStrategyExperimentByID(id)
	if err != nil || exp == nil {
		writeError(w, http.StatusNotFound, "strategy experiment not found")
		return
	}

	if exp.Status == "APPROVED" {
		writeError(w, http.StatusBadRequest, "strategy experiment already approved")
		return
	}

	if err := h.db.ApproveStrategyExperiment(id); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed approving experiment: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":           true,
		"message":           fmt.Sprintf("Strategy version %s approved and activated", exp.CandidateVersion),
		"activated_version": exp.CandidateVersion,
	})
}
