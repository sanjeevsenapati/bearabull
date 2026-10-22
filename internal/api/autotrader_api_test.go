package api

import (
	"bearabull/internal/autotrader"
	"bearabull/internal/config"
	"bearabull/internal/marketdata"
	"bearabull/internal/storage"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func setupTestRouter(t *testing.T) (http.Handler, *storage.DB, func()) {
	testDBPath := "./test_api.db"
	_ = os.Remove(testDBPath)

	db, err := storage.Open(testDBPath)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	cfg := config.DefaultConfig()
	mockProv := marketdata.NewLicensedProvider("Direct Feed")
	sched := autotrader.NewScheduler(db, mockProv, 2)

	handler := NewHandler(cfg, db, nil)
	handler.SetAutotraderScheduler(sched)
	router := NewRouter(cfg, handler, nil)

	cleanup := func() {
		db.Close()
		_ = os.Remove(testDBPath)
	}
	return router, db, cleanup
}

func TestAutotraderAPIEndpoints(t *testing.T) {
	router, db, cleanup := setupTestRouter(t)
	defer cleanup()

	// 1. GET /api/v1/autotrader/status
	req := httptest.NewRequest("GET", "/api/v1/autotrader/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /status returned status %d: %s", w.Code, w.Body.String())
	}

	// 2. POST /api/v1/autotrader/enable
	req = httptest.NewRequest("POST", "/api/v1/autotrader/enable", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("POST /enable returned status %d", w.Code)
	}

	// 3. POST /api/v1/autotrader/pause
	req = httptest.NewRequest("POST", "/api/v1/autotrader/pause", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("POST /pause returned status %d", w.Code)
	}

	// 4. POST /api/v1/autotrader/resume
	req = httptest.NewRequest("POST", "/api/v1/autotrader/resume", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("POST /resume returned status %d", w.Code)
	}

	// 5. POST /api/v1/autotrader/disable
	req = httptest.NewRequest("POST", "/api/v1/autotrader/disable", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("POST /disable returned status %d", w.Code)
	}

	// 6. POST /api/v1/autotrader/scan (seed a stock first)
	_ = db.UpsertStock(storage.Stock{
		Symbol:      "TCS.NS",
		Ticker:      "TCS",
		CompanyName: "Tata Consultancy Services",
		Sector:      "IT",
		IsActive:    true,
	})
	req = httptest.NewRequest("POST", "/api/v1/autotrader/scan", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("POST /scan returned status %d: %s", w.Code, w.Body.String())
	}

	// 7. GET /api/v1/autotrader/cycles
	req = httptest.NewRequest("GET", "/api/v1/autotrader/cycles", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /cycles returned status %d", w.Code)
	}

	// 8. GET /api/v1/autotrader/opportunities
	req = httptest.NewRequest("GET", "/api/v1/autotrader/opportunities", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /opportunities returned status %d", w.Code)
	}

	// 9. GET /api/v1/autotrader/performance
	req = httptest.NewRequest("GET", "/api/v1/autotrader/performance", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /performance returned status %d", w.Code)
	}

	// 10. GET /api/v1/autotrader/reconciliation
	req = httptest.NewRequest("GET", "/api/v1/autotrader/reconciliation", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /reconciliation returned status %d", w.Code)
	}

	// 11. GET /api/v1/autotrader/risk
	req = httptest.NewRequest("GET", "/api/v1/autotrader/risk", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /risk returned status %d", w.Code)
	}

	// 12. GET /api/v1/autotrader/insights
	req = httptest.NewRequest("GET", "/api/v1/autotrader/insights", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /insights returned status %d", w.Code)
	}

	// 13. GET /api/v1/autotrader/experiments
	expID, _ := db.CreateStrategyExperiment(&storage.StrategyExperiment{
		Name:               "Test Optimization",
		BaseVersion:        "v1.0.0",
		CandidateVersion:   "v1.0.1",
		Status:             "PROPOSED",
		Description:        "Tighter stop loss and 2.5 R:R target",
		ParametersDiffJSON: `{"min_score":72.0}`,
		TrainingStart:      "2026-01-01",
		TrainingEnd:        "2026-06-30",
		ValidationStart:    "2026-07-01",
		ValidationEnd:      "2026-10-01",
	})

	req = httptest.NewRequest("GET", "/api/v1/autotrader/experiments", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /experiments returned status %d", w.Code)
	}

	// 14. POST /api/v1/autotrader/experiments/{id}/approve
	approveURL := "/api/v1/autotrader/experiments/" + string(rune('0'+expID)) + "/approve"
	req = httptest.NewRequest("POST", approveURL, nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("POST /experiments/{id}/approve returned status %d: %s", w.Code, w.Body.String())
	}
}
