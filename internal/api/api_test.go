package api

import (
	"bearabull/internal/config"
	"bearabull/internal/jobs"
	"bearabull/internal/marketdata"
	"bearabull/internal/storage"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func setupTestServer(t *testing.T) (*storage.DB, http.Handler, func()) {
	tmpFile, err := os.CreateTemp("", "bearabull_test_*.db")
	if err != nil {
		t.Fatalf("failed creating temp db: %v", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()

	db, err := storage.Open(tmpPath)
	if err != nil {
		t.Fatalf("failed opening test db: %v", err)
	}

	// Seed universe
	if err := marketdata.SeedUniverse(db); err != nil {
		t.Fatalf("failed seeding universe: %v", err)
	}

	cfg := config.DefaultConfig()
	syncer := jobs.NewSyncer(db, nil, 2, "v1.0.0")
	handler := NewHandler(cfg, db, syncer)
	router := NewRouter(cfg, handler, nil)

	cleanup := func() {
		db.Close()
		os.Remove(tmpPath)
	}

	return db, router, cleanup
}

func TestHealthEndpoint(t *testing.T) {
	_, router, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/v1/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var res map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed parsing json: %v", err)
	}

	if res["status"] != "healthy" {
		t.Errorf("expected status == healthy, got %v", res["status"])
	}
}

func TestStocksListEndpoint(t *testing.T) {
	_, router, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/v1/stocks", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var res struct {
		Stocks []storage.Stock `json:"stocks"`
		Count  int             `json:"count"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed parsing json: %v", err)
	}

	if res.Count < 50 {
		t.Errorf("expected at least 50 stocks seeded, got %d", res.Count)
	}
}

func TestWatchlistEndpoints(t *testing.T) {
	_, router, cleanup := setupTestServer(t)
	defer cleanup()

	// 1. Add to watchlist
	body := bytes.NewBufferString(`{"symbol":"RELIANCE.NS"}`)
	reqAdd := httptest.NewRequest("POST", "/api/v1/watchlist", body)
	wAdd := httptest.NewRecorder()
	router.ServeHTTP(wAdd, reqAdd)

	if wAdd.Code != http.StatusOK {
		t.Fatalf("expected status 200 on add, got %d", wAdd.Code)
	}

	// 2. Fetch watchlist
	reqGet := httptest.NewRequest("GET", "/api/v1/watchlist", nil)
	wGet := httptest.NewRecorder()
	router.ServeHTTP(wGet, reqGet)

	var resGet struct {
		Count int `json:"count"`
	}
	_ = json.Unmarshal(wGet.Body.Bytes(), &resGet)
	if resGet.Count != 1 {
		t.Errorf("expected 1 watchlist item, got %d", resGet.Count)
	}

	// 3. Delete from watchlist
	reqDel := httptest.NewRequest("DELETE", "/api/v1/watchlist/RELIANCE.NS", nil)
	wDel := httptest.NewRecorder()
	router.ServeHTTP(wDel, reqDel)

	if wDel.Code != http.StatusOK {
		t.Fatalf("expected status 200 on delete, got %d", wDel.Code)
	}
}
