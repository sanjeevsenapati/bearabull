# Bearabull — NIFTY 50 Weekly Stock Prediction & Analysis Platform

> **"Read the Trend. Understand the Risk."**

Bearabull is a high-performance, data-driven quantitative technical analysis and weekly decision-support platform engineered in Go for the complete **NIFTY 50** Indian stock universe. It combines robust market data ingestion from Yahoo Finance, a deterministic 5-pillar technical indicator scoring engine, an interactive fintech terminal UI, and a walk-forward backtesting simulator with zero look-ahead bias and realistic trading friction.

---

## 🌟 Key Highlights & Architecture

- **Single Self-Contained Binary**: All web frontend assets (HTML, CSS, JS, TradingView Lightweight Charts) are embedded directly into the Go executable via Go `embed.FS`. The entire application runs standalone with `./bearabull`.
- **Pure-Go Embedded SQLite Database**: Utilizes `modernc.org/sqlite` without CGO dependencies. Supports ACID transactions, WAL mode, foreign key integrity, and automated schema migrations.
- **Configurable NIFTY 50 Universe**: Full registry of all 50 constituent stocks with verified NSE tickers (`.NS`), sectors, and exchange classifications.
- **Yahoo Finance Integration with Resilience**: Fetches real daily OHLCV candles with exponential backoff retries, user-agent rotation, bounded concurrency throttling, Asia/Kolkata market timezone alignment, and deterministic fallback generators.
- **Explainable 5-Pillar Scoring Model**:
  1. **Trend (35%)**: SMA 20, 50, 200; EMA 9, 21, 50; Supertrend (10, 3.0); ADX 14 with +DI/-DI.
  2. **Momentum (25%)**: MACD (12, 26, 9) histogram; Wilder's RSI 14; Stochastic %K/%D.
  3. **Volume Confirmation (15%)**: Relative Volume (RVOL >= 1.25x), Rate of Change (ROC 14), On-Balance Volume (OBV).
  4. **Price Action & Breakouts (15%)**: 20-day swing highs/lows, breakout detection, opening gap analysis.
  5. **Volatility & Risk Filter (10%)**: Average True Range (ATR 14), Bollinger Bands (20, 2), 20-day annualized historical volatility.
- **Interactive Fintech Terminal**:
  - **Dashboard**: Market Breadth (% above SMA 50/200), bull/bear/neutral ratio, top gainers/losers, opportunity table.
  - **Stock Screener**: Multi-factor scanner with preset filters (*Top Bullish*, *Top Bearish*, *Breakouts*, *Pullbacks*, *High Volume*, *Low Volatility*).
  - **Weekly Signals**: Transparent signal dossiers with itemized bullish/bearish evidence weights, entry zones, and stop-loss levels.
  - **Stock Explorer**: Interactive TradingView candlestick charts, SMA/EMA overlays, Bollinger Bands, RSI & MACD subcharts, 16+ technical indicators matrix.
  - **Watchlist**: Personal portfolio monitoring with live alerts and quick inspection.
  - **Backtesting Module**: Chronological walk-forward simulation with custom slippage, brokerage/STT taxes, stop-loss, target exits, equity curve vs NIFTY 50 benchmark.
  - **Methodology & Settings**: Rulebook transparency, data sync controls, and symbol registry inspection.

---

## 🛠️ Quick Start

### 1. Prerequisites
- **Go 1.22+** (tested and verified on Go 1.26 darwin/arm64 and Linux).
- Alternatively, **Docker** and **Docker Compose**.

### 2. Local Installation & Run
```bash
# Clone or navigate to the repository
cd /Users/sanjeev/workspace/bearabull

# Build the standalone binary
make build

# Run the platform (starts at http://localhost:8080)
make run
```

Or run directly with Go:
```bash
go run ./cmd/bearabull
```

### 3. Run with Demo Data
To test or demo the platform with deterministic baseline data without hitting external network endpoints:
```bash
./bearabull -demo -sync
```

### 4. Running via Podman
```bash
# Build the container image with Podman
make podman-build

# Run the container (starts in background with persistent volume at http://localhost:8080)
make podman-run

# View live container logs
make podman-logs

# Stop and remove container
make podman-stop
```

Or run directly using the Podman CLI:
```bash
# Build
podman build -t bearabull:latest .

# Run with persistent SQLite data volume
podman run -d --name bearabull-terminal -p 8080:8080 -v bearabull_data:/app/data:Z bearabull:latest
```

---

## 📊 Application Tour

### 1. Dashboard & Market Breadth
- Real-time aggregation of bullish vs bearish market breadth across all 50 NIFTY constituents.
- Live progress tracks indicating the percentage of stocks trading above their **50-day SMA** and **200-day SMA**.
- Sortable opportunity table displaying last price, weekly change %, signal classification, and conviction score.

### 2. Stock Screener & Presets
Filter and rank the NIFTY 50 by:
- **Preset Scanners**: Top Bullish Setups, Top Bearish Setups, Strong Momentum, Breakout Candidates, Pullback Candidates, High-Volume Movers, Lowest Volatility.
- **Custom Filters**: Signal (BUY/SELL/HOLD), Sector, Minimum Score (0-95), Risk Level (Low/Normal/High), Sort By (Score, Weekly %, RSI, RVOL, Price).

### 3. Stock Explorer & Candlestick Charts
- High-fidelity **TradingView Lightweight Charts** with configurable candlestick timeframes (1M, 3M, 6M, 1Y, 2Y).
- Overlays: **SMA 20**, **SMA 50**, **EMA 21**, **Bollinger Bands (20, 2)**.
- Subcharts: **RSI (14)** with 70/30 overbought/oversold boundaries, **MACD (12, 26, 9)** line, signal line, and color-coded histogram bars.
- 16-field quantitative indicator matrix and plain-language strategy rationale.

### 4. Backtesting Engine (Zero Look-Ahead Bias)
- **Execution Mechanism**: Decisions evaluated strictly at Friday's candle close; orders executed on Monday's market open price with slippage (0.10%) and brokerage/taxes (0.15%).
- **Risk Controls**: Dynamic stop-loss (% or ATR-based) and profit target exits.
- **Performance Reporting**:
  - Cumulative Strategy Return vs NIFTY 50 Benchmark Buy & Hold
  - Compound Annual Growth Rate (CAGR)
  - Maximum Drawdown (%)
  - Win Rate (%) & Total Trades
  - Profit Factor (Gross Profits / Gross Losses)
  - Annualized Sharpe Ratio (relative to 6% risk-free rate)
  - Full Audited Trade Journal

---

## 🔌 RESTful API Reference

Bearabull provides a complete RESTful API documenting all endpoints according to OpenAPI 3.0 specification (`docs/openapi.yaml`):

| Method | Endpoint | Description |
|---|---|---|
| `GET` | `/api/v1/health` | Service health status and database connectivity |
| `GET` | `/api/v1/market/overview` | Market breadth, NIFTY stats, top movers |
| `GET` | `/api/v1/stocks` | List all 50 constituent stocks |
| `GET` | `/api/v1/stocks/{symbol}` | Stock profile, latest quote, and signal |
| `GET` | `/api/v1/stocks/{symbol}/history` | Historical daily OHLCV candlestick series |
| `GET` | `/api/v1/stocks/{symbol}/indicators` | Technical indicator snapshot |
| `GET` | `/api/v1/stocks/{symbol}/signals` | Historical weekly signals |
| `GET` | `/api/v1/signals/weekly` | Current weekly BUY / SELL / HOLD universe signals |
| `GET` | `/api/v1/screener` | Multi-factor screener and ranking query |
| `GET` | `/api/v1/sectors` | Sector breakdown and constituent counts |
| `GET` | `/api/v1/watchlist` | Get user watchlist items |
| `POST`| `/api/v1/watchlist` | Add stock to user watchlist |
| `DELETE`| `/api/v1/watchlist/{symbol}` | Remove stock from watchlist |
| `GET` | `/api/v1/backtests` | List saved backtest simulation runs |
| `POST`| `/api/v1/backtests` | Execute custom backtest simulation |
| `GET` | `/api/v1/backtests/{id}` | Retrieve specific backtest result & trades |
| `GET` | `/api/v1/sync/status` | Current data synchronization progress |
| `POST`| `/api/v1/sync/trigger` | Trigger background data synchronization |

---

## 🧪 Automated Testing

All technical indicators, signal thresholds, backtesting accounting, and REST endpoints are verified via automated unit and integration tests:

```bash
# Run all unit tests
go test -v ./...
```

Included test suites:
- `internal/indicators/indicators_test.go`: Verified calculations against known reference series for SMA, EMA, RSI, Bollinger Bands, ATR.
- `internal/signals/signals_test.go`: Data sufficiency checks, bullish/bearish scoring rules, risk level determination.
- `internal/backtest/backtest_test.go`: Verification of look-ahead bias prevention, fee deductions, and trade lifecycle accounting.
- `internal/api/api_test.go`: Health endpoint, constituent listing, and watchlist CRUD operations.

---

## ⚖️ Regulatory Notice & Disclaimer

Bearabull is a technical analysis decision-support tool built for informational and educational purposes. It is **not** an automated execution system, and no model output or signal should be construed as financial advice or a guarantee of profit. All market trading carries risk of financial loss. Always exercise independent judgement and verify technical patterns against broader market context.
