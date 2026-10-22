package main

import (
	"bearabull/internal/api"
	"bearabull/internal/autotrader"
	"bearabull/internal/config"
	"bearabull/internal/jobs"
	"bearabull/internal/marketdata"
	"bearabull/internal/storage"
	"bearabull/internal/yahoo"
	"bearabull/web"
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	portFlag := flag.String("port", "", "Server listening port (e.g. 8080)")
	configFlag := flag.String("config", "config.json", "Path to optional JSON configuration file")
	syncFlag := flag.Bool("sync", false, "Force synchronization on startup")
	demoFlag := flag.Bool("demo", false, "Populate with deterministic demo baseline data")
	flag.Parse()

	log.Println("================================================================")
	log.Println(" Bearabull — NIFTY 50 Weekly Prediction & Technical Terminal")
	log.Println(" Tagline: Read the Trend. Understand the Risk.")
	log.Println("================================================================")

	cfg, err := config.Load(*configFlag)
	if err != nil {
		log.Fatalf("[FATAL] Could not load config: %v", err)
	}

	if *portFlag != "" {
		cfg.AppPort = *portFlag
	}

	// 1. Initialize SQLite Database
	db, err := storage.Open(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("[FATAL] Failed to connect to SQLite: %v", err)
	}
	defer db.Close()

	// 2. Seed Universe
	if err := marketdata.SeedUniverse(db); err != nil {
		log.Fatalf("[FATAL] Failed to seed NIFTY 50 universe: %v", err)
	}

	// 3. Initialize Market Data Provider
	var provider marketdata.Provider
	if cfg.YahooFinanceEnabled {
		provider = yahoo.NewClient(cfg.RequestTimeout, cfg.AnalysisTimezone)
		log.Printf("[DATA] Yahoo Finance client configured (Timeout: %v, Timezone: %s)", cfg.RequestTimeout, cfg.AnalysisTimezone)
	} else {
		provider = marketdata.NewLicensedProvider("Licensed Direct Feed")
		log.Println("[DATA] Licensed Direct Feed configured")
	}

	// 4. Initialize Background Syncer & Autonomous Scheduler
	syncer := jobs.NewSyncer(db, provider, cfg.MaxConcurrentRequests, cfg.StrategyVersion)
	scheduler := autotrader.NewScheduler(db, provider, cfg.MaxConcurrentRequests)
	scheduler.Start(context.Background())

	// Check if initial sync is needed (e.g. fresh database)
	pricesMap, _ := db.GetAllLatestPrices()
	needsInitialSync := len(pricesMap) == 0 || *syncFlag

	if needsInitialSync {
		log.Printf("[STARTUP] Database has %d price records. Initiating automatic background sync...", len(pricesMap))
		go func() {
			syncCtx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
			defer cancel()
			_ = syncer.Run(syncCtx, *demoFlag)
		}()
	}

	// Periodic Refresh Routine
	if cfg.MarketDataRefreshInterval > 0 {
		ticker := time.NewTicker(cfg.MarketDataRefreshInterval)
		go func() {
			for range ticker.C {
				log.Println("[SCHEDULE] Triggering scheduled market data refresh...")
				refreshCtx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
				_ = syncer.Run(refreshCtx, false)
				cancel()
			}
		}()
	}

	// 5. Setup Router with embedded web static assets
	handler := api.NewHandler(cfg, db, syncer)
	handler.SetAutotraderScheduler(scheduler)
	router := api.NewRouter(cfg, handler, web.Files)

	serverAddr := fmt.Sprintf(":%s", cfg.AppPort)
	srv := &http.Server{
		Addr:         serverAddr,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Graceful Shutdown Channel
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("[SERVER] Bearabull web server running at http://localhost:%s", cfg.AppPort)
		log.Printf("[SERVER] REST API endpoints available at http://localhost:%s/api/v1/health", cfg.AppPort)
		log.Printf("[SERVER] Auto-Trader API endpoints available at http://localhost:%s/api/v1/autotrader/status", cfg.AppPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[FATAL] HTTP server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	sig := <-stopChan
	log.Printf("[SERVER] Received signal %v. Initiating graceful shutdown...", sig)
	scheduler.Stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("[SERVER] Server shutdown error: %v", err)
	} else {
		log.Println("[SERVER] Server exited cleanly")
	}
}
