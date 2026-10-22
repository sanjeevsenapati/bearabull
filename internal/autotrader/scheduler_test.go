package autotrader

import (
	"bearabull/internal/marketdata"
	"bearabull/internal/storage"
	"context"
	"os"
	"testing"
	"time"
)

func TestCalendarAndMarketHours(t *testing.T) {
	cal := NewNSECalendar()

	// 1. Saturday test
	sat := time.Date(2026, 10, 24, 11, 0, 0, 0, cal.Location()) // Saturday
	isDay, reason := cal.IsMarketDay(sat)
	if isDay {
		t.Errorf("expected Saturday to not be market day, got true (%s)", reason)
	}

	// 2. Republic Day holiday test (2026-01-26)
	repDay := time.Date(2026, 1, 26, 11, 0, 0, 0, cal.Location())
	isHolidayDay, hReason := cal.IsMarketDay(repDay)
	if isHolidayDay {
		t.Errorf("expected Republic Day to be closed, got true (%s)", hReason)
	}

	// 3. Regular trading hours test (Wednesday 11:30 IST)
	tradingTime := time.Date(2026, 10, 21, 11, 30, 0, 0, cal.Location())
	active, aReason := cal.IsMarketSessionActive(tradingTime)
	if !active {
		t.Errorf("expected Wednesday 11:30 IST to be active, got false (%s)", aReason)
	}

	// 4. Pre-market test (Wednesday 08:45 IST)
	preMarket := time.Date(2026, 10, 21, 8, 45, 0, 0, cal.Location())
	pActive, _ := cal.IsMarketSessionActive(preMarket)
	if pActive {
		t.Errorf("expected 08:45 IST to be pre-market inactive")
	}

	// 5. Post-market test (Wednesday 16:00 IST)
	postMarket := time.Date(2026, 10, 21, 16, 0, 0, 0, cal.Location())
	postActive, _ := cal.IsMarketSessionActive(postMarket)
	if postActive {
		t.Errorf("expected 16:00 IST to be post-market inactive")
	}

	// 6. Next 15-min boundary test: 10:07 -> 10:15
	t1 := time.Date(2026, 10, 21, 10, 7, 0, 0, cal.Location())
	next := cal.Next15MinBoundary(t1)
	if next.Minute() != 15 || next.Hour() != 10 {
		t.Errorf("expected 10:15, got %s", next.Format("15:04"))
	}
}

func TestSchedulerCycleExecution(t *testing.T) {
	testDBPath := "./test_sched.db"
	_ = os.Remove(testDBPath)
	defer os.Remove(testDBPath)

	db, err := storage.Open(testDBPath)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	defer db.Close()

	// Seed universe with a test stock
	err = db.UpsertStock(storage.Stock{
		Symbol:      "TCS.NS",
		Ticker:      "TCS",
		CompanyName: "Tata Consultancy Services",
		Sector:      "IT",
		IsActive:    true,
	})
	if err != nil {
		t.Fatalf("failed to seed test stock: %v", err)
	}

	// Use LicensedProvider with deterministic candles
	mockProv := marketdata.NewLicensedProvider("Test Direct Feed")
	mockProv.SetPrice("TCS.NS", 4000.0)

	sched := NewScheduler(db, mockProv, 2)

	// Enable autotrader for testing orders
	if err := db.SetAutotraderEnabled(true); err != nil {
		t.Fatalf("failed to enable autotrader: %v", err)
	}

	// Run Cycle with forceManual = true
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cycle, err := sched.RunCycle(ctx, true)
	if err != nil {
		t.Fatalf("scheduler RunCycle error: %v", err)
	}

	if cycle == nil {
		t.Fatalf("expected non-nil cycle record")
	}

	if cycle.Status != "COMPLETED" {
		t.Errorf("expected cycle status COMPLETED, got %s", cycle.Status)
	}

	if cycle.StocksScanned != 1 {
		t.Errorf("expected 1 stock scanned, got %d", cycle.StocksScanned)
	}

	// Verify cycle was persisted in DB
	cycles, err := db.GetAutotraderCycles(10)
	if err != nil || len(cycles) == 0 {
		t.Fatalf("expected saved cycle in DB, got %d (err: %v)", len(cycles), err)
	}

	// Verify dashboard status endpoint helper
	status, err := sched.GetStatus()
	if err != nil {
		t.Fatalf("failed to get scheduler status: %v", err)
	}
	if status == nil || status.Config.InitialCapital != 1000000.0 {
		t.Errorf("unexpected status response: %+v", status)
	}
}
