# Bearabull: Autonomous Paper Trading, 15-Minute Market Scanner and Risk Optimization Engine

## Objective

Extend the existing Bearabull Go application into an autonomous paper-trading research platform for NSE NIFTY 50 stocks.

The system must independently:

1. Monitor the NIFTY 50 universe during Indian equity market hours.
2. Refresh available market metrics every 15 minutes.
3. Identify and rank the five strongest qualified trading opportunities.
4. Analyze technical indicators, order blocks, market structure, and risk.
5. Automatically create, manage, and close simulated trades using virtual money.
6. Compare simulated execution prices and portfolio valuations against actual observed market prices.
7. Analyze profitable trades, losing trades, missed opportunities, slippage, and risk.
8. Evaluate strategy improvements through controlled historical and out-of-sample testing.

Use Go for the backend and reuse the existing SQLite database, Yahoo Finance adapter, signal engine, order-block detector, paper-trading module, and dashboard.

This system must remain paper-trading-only. Never connect its execution engine to a real broker or exchange.

## 1. Autonomous Trading Scheduler

Create a Go package named `internal/autotrader/` with a scheduler that runs the complete analysis cycle every 15 minutes during the NSE regular equity session.

Use Asia/Kolkata for market schedules.

Regular session: 09:15–15:30 IST on eligible trading days. Account for exchange holidays and special sessions using a configurable, verifiable trading calendar.

Each cycle should:

- Check whether the market session is active.
- Verify that the market-data provider is healthy.
- Refresh available market data for the configured NIFTY 50 constituents.
- Validate quote timestamps and candle completeness.
- Update technical indicators and order-block states.
- Recalculate eligible trade setups.
- Rank candidates.
- Evaluate existing positions and pending orders.
- Check stop-loss, target, risk, and account constraints.
- Create new paper orders when the strategy permits.
- Update portfolio valuations and performance metrics.
- Record the complete cycle results and execution decisions.

Run only one active scan/execution cycle at a time. Use database-backed job records or equivalent coordination to prevent duplicate cycles after restarts or overlapping schedules.

Record cycle start/end times, number of stocks processed, data errors, shortlisted stocks, rejected setups, orders submitted, fills, and errors.

Run a reconciliation cycle after market close and a recovery check after application restarts.

Outside market hours, do not generate fictitious new intraday prices. Continue reporting portfolio performance using the latest available validated prices.

## 2. Market Data and 15-Minute Polling

Use the existing Yahoo Finance market-data adapter through a provider interface.

Important: a 15-minute polling schedule does not guarantee fresh, real-time market data. Verify the timestamps and interval of the returned data. Yahoo Finance may provide delayed, cached, incomplete, or unavailable quotes.

Requirements:

- Poll available data every 15 minutes.
- Request historical intraday candles where the provider supports them.
- Use daily candles for longer-term trend confirmation.
- Keep the last successful refresh timestamp for each symbol.
- Record provider timestamps separately from polling timestamps.
- Detect stale data, missing candles, gaps, and duplicate observations.
- Never treat an unchanged cached price as proof that a new market update occurred.
- Do not manufacture intraday candles from daily OHLC data.
- Handle provider throttling using bounded concurrency, caching, backoff, and rate limits.
- Do not issue requests more frequently than permitted by the data provider.
- Provide a configurable licensed-provider alternative if Yahoo Finance cannot supply sufficiently fresh data.
- Do not label delayed data as real-time.

When the latest price is too stale to support a new trade, skip the entry and display a data-quality warning. Use a configurable freshness threshold appropriate to the selected provider and candle interval.

## 3. NIFTY 50 Stock Selection

Evaluate the configured NIFTY 50 constituents and rank eligible opportunities at every successful scan.

Use the existing analysis engines:

- Weekly trend.
- Daily trend.
- Available intraday momentum.
- RSI, MACD, moving averages, ADX, and ATR.
- Relative volume.
- Support and resistance.
- Bullish and bearish order blocks.
- Break of Structure (BOS).
- Change of Character (CHoCH).
- Fair Value Gaps, when supported by the detection rules.
- Multi-timeframe alignment.
- Order-block freshness and mitigation.
- Proposed entry distance.
- Stop-loss distance.
- Target feasibility.
- Estimated transaction costs.
- Risk/reward ratio.

Create a configurable scoring model that produces a score from 0 to 100.

Rank bullish and bearish opportunities separately before applying the portfolio selection rules.

The scanner may display the top five ranked candidates, but it must only trade candidates that pass every mandatory risk and data-quality filter.

Do not force the system to open five positions. It may open fewer positions or remain entirely in cash.

Avoid duplicate entries into the same stock and direction when an existing position or pending order already represents that setup.

## 4. Autonomous Paper-Trade Decision Engine

For every scan, choose among:

- OPEN LONG: a qualifying bullish setup with a valid entry and acceptable risk.
- CLOSE LONG: an exit rule, stop-loss, target, or invalidation condition has been met.
- OPEN SHORT: available only when a separate short-simulation setting is explicitly enabled.
- CLOSE SHORT: cover an existing simulated short when its exit rules trigger.
- HOLD: maintain an existing position.
- WAIT FOR RETEST: the preferred order-block entry has not been reached.
- WAIT FOR CONFIRMATION: price has reached a zone but confirmation is missing.
- SKIP: insufficient data, unsuitable risk/reward, excessive exposure, or invalid setup.

The default strategy must be long-only. Short trading must be a separate, clearly labeled simulation mode and must not imply real-market permissions.

Use the existing order-block engine to define entry zones, invalidation conditions, protective stops, and technically justified targets.

Before every new entry, revalidate the signal using the latest eligible data. A historical BUY signal must not automatically trigger a new trade if the price has moved away from its entry zone.

Do not open trades merely to reach a weekly return target.

## 5. Automatic Paper Execution

Connect the autonomous decision engine to the existing virtual account and order-management engine.

Execution must use a deterministic, documented simulation policy.

For an intraday strategy:

- Prefer the next eligible observed market price or candle after the decision timestamp.
- Use the configured price source and interval.
- Include configurable slippage and transaction costs.
- Respect order types and price limits.
- Model adverse price gaps.
- Do not assume that a stop-loss guarantees the exit price.
- Do not fill an order using data that was unavailable at the time of submission.
- Do not simulate intraday fills using daily candle values alone.

If reliable intraday prices are unavailable, use a clearly documented next-eligible-session policy instead of pretending the order was filled during the current session.

The execution engine must:

- Reserve cash before accepting eligible buy orders.
- Reject orders that violate account limits.
- Prevent duplicate fills.
- Process eligible orders transactionally.
- Update holdings and the immutable ledger consistently.
- Maintain order and fill audit trails.
- Close positions when the configured exit rules are triggered.
- Reconcile account cash and positions after every execution cycle.

Automatic trading must be independently configurable. Users must be able to enable or disable it without disabling market scanning.

## 6. Weekly Return Objective: 1–10%

Add a configurable weekly performance objective, initially set to 1–10%.

This is an aspirational research target, not a guaranteed return, a probability estimate, or a mandatory trading instruction.

Display:

- Current weekly return.
- Weekly target range.
- Distance from the lower target.
- Distance from the upper target.
- Remaining trading sessions.
- Realized weekly P&L.
- Unrealized weekly P&L.
- Weekly fees and estimated slippage.
- Peak-to-trough weekly drawdown.
- Risk currently exposed.
- Number of completed trades.
- Number of qualifying opportunities skipped.

Calculate weekly return using a clearly defined equity-based method that accounts for realized and unrealized P&L, cash, fees, and external deposits or withdrawals.

Default objective evaluation should use net returns after configured costs.

If the target is reached:

- Display the result.
- Do not automatically increase position sizes.
- Do not force additional trades.
- Continue managing existing positions under the configured exit and risk rules.

If the target appears unreachable:

- Do not loosen filters or increase risk to chase the target.
- Report the actual performance and remaining risk.
- Preserve the same risk controls.

Allow historical analysis of how frequently the strategy achieved, missed, or exceeded its objective. Do not present a target as evidence that the strategy can achieve it consistently.

## 7. Risk Management Engine

Implement a dedicated `internal/risk/` package.

Configurable initial settings for a ₹10,00,000 virtual account:

- Maximum risk per trade: 0.5% of current account equity.
- Maximum aggregate planned risk across open positions: 2% of equity.
- Maximum allocation per stock: 20% of equity.
- Maximum concurrent long positions: 5.
- Maximum daily loss: 1.5% of start-of-day equity.
- Maximum weekly drawdown: 3% from the week's equity peak.

These are initial simulation defaults, not universally optimal risk limits. Make them configurable and test their behavior.

Use the following risk-based position-sizing principle:

Position quantity must be determined by the permitted monetary risk divided by the estimated loss per share, including the distance between entry and stop and an allowance for applicable costs or slippage.

Then apply cash, liquidity, position-size, and concentration limits.

Use conservative rounding for whole-share quantities.

Before every entry:

- Calculate planned monetary risk.
- Calculate stop distance as a percentage of entry price.
- Estimate fees and slippage.
- Check aggregate portfolio risk.
- Check sector and stock concentration.
- Check the daily loss limit.
- Check the weekly drawdown limit.
- Reject trades that fail mandatory controls.

If the daily loss limit is reached, block new entries for the remainder of the session while continuing to manage and close existing positions under their risk rules.

If the weekly drawdown limit is reached, suspend new entries until the configured review or reset period.

Do not disable protective exits because a trading limit has been reached.

Display planned risk and maximum estimated loss for each trade. Explain that actual losses may exceed estimates because of gaps and execution uncertainty.

## 8. Compare Paper Trades Against Actual Market Prices

Build a Trade Execution Quality and Market Price Reconciliation engine.

For every paper trade, record:

- Signal-generation timestamp.
- Decision timestamp.
- Order-submission timestamp.
- Expected entry price.
- Simulated fill price.
- Best available observed market price at decision time.
- First eligible observed market price after submission.
- Current observed price.
- Exit trigger price.
- Simulated exit fill price.
- Market-data source and timestamps.
- Applicable slippage assumptions.
- Fees and transaction costs.
- Original order-block zone.
- Planned stop-loss and targets.
- Realized and unrealized P&L.

Calculate:

- Entry execution deviation in rupees and basis points.
- Exit execution deviation in rupees and basis points.
- Simulated slippage cost.
- Gross and net P&L.
- Return on allocated capital.
- Return on total account equity.
- Maximum favorable excursion.
- Maximum adverse excursion.
- Planned versus realized risk.
- Target and stop outcomes.
- Time spent in the trade.

Clearly distinguish:

1. The simulated execution price.
2. The actual observed market price.
3. A hypothetical alternative execution price.

Never claim that a hypothetical fill occurred in the real market.

Use time-aligned observations. If the exact timestamp is unavailable, mark the comparison as approximate and record the nearest observation used.

Maintain a separate reconciliation history so users can inspect how the simulated portfolio compares with the market.

## 9. Strategy Analysis and Optimization

Create a strategy research engine that analyzes completed paper trades and identifies potential improvements.

Analyze:

- Win rate.
- Average winning and losing trade.
- Profit factor.
- Expectancy per trade.
- Net return after costs.
- Maximum drawdown.
- Average holding duration.
- Risk/reward at entry.
- Slippage and fee impact.
- Performance by stock and sector.
- Performance by weekday and market regime.
- Performance by signal score.
- Performance by order-block freshness.
- Performance by timeframe alignment.
- Performance by entry confirmation type.
- Performance by stop-loss and target method.
- Results when the market is trending versus ranging.
- Missed opportunities and rejected setups.
- Trades where entry occurred too late.
- Stop-outs followed by subsequent price recovery.

Generate evidence-based findings, for example:

- Which setup types historically performed better.
- Which filters reduced losing trades.
- Whether tighter entry conditions improved net expectancy.
- Whether transaction costs erased a small gross edge.
- Whether the current strategy overtrades.
- Whether a parameter change increased returns while also increasing drawdown.

Do not conclude that a rule improves performance based on one or two trades.

### Safe optimization workflow

1. Record a versioned baseline strategy.
2. Analyze completed trades and historical candles.
3. Form candidate rule or parameter changes.
4. Test them on an earlier training period.
5. Validate them on a separate chronological period.
6. Perform walk-forward evaluation.
7. Compare net returns, drawdown, trade count, and stability.
8. Reject changes that only improve training-period results or exceed risk limits.
9. Publish a proposed strategy version with evidence.
10. Activate the new version only through explicit user approval.

Do not let a live optimization loop silently change the active strategy or increase risk.

Keep an untouched evaluation period where practical. Prevent look-ahead bias and survivorship bias wherever the necessary data is available.

Treat optimization findings as hypotheses, not proof of future profitability.

## 10. Dashboard: Autonomous Trader

Add a primary navigation section named "Auto-Trader".

Display:

- Auto-trader enabled or disabled.
- Paper-trading-only status.
- Current market session status.
- Last completed scan.
- Next scheduled scan.
- Data-provider health.
- Number of eligible stocks analyzed.
- Top five qualified opportunities.
- Open paper positions.
- Pending orders.
- Available virtual cash.
- Total account equity.
- Realized and unrealized P&L.
- Weekly net return.
- Weekly target progress.
- Current drawdown.
- Planned aggregate risk.
- Remaining daily risk allowance.
- Reconciliation status.
- Most recent strategy-analysis findings.

For every scan, allow users to inspect:

- Why each stock was selected.
- Why other stocks were rejected.
- Indicator values and order-block zones.
- Entry, stop-loss, target, and position sizing.
- Risk checks that passed or failed.
- Every simulated order and fill.
- The market-data timestamp used.

Add controls:

- Enable or disable autonomous paper trading.
- Pause new entries.
- Resume new entries.
- Close a position.
- Cancel a pending order.
- Run a manual scan.
- View scan logs.
- View risk settings.
- Review proposed strategy improvements.

Require confirmation for destructive actions. Pausing new entries must not silently cancel protective exits.

## 11. Database and API Extensions

Add versioned SQLite migrations for:

- `autotrader_config`
- `autotrader_cycles`
- `autotrader_cycle_results`
- `autotrader_decisions`
- `market_observations`
- `paper_execution_comparisons`
- `risk_events`
- `strategy_versions`
- `strategy_experiments`
- `strategy_evaluations`

Add endpoints:

- GET `/api/v1/autotrader/status`
- POST `/api/v1/autotrader/enable`
- POST `/api/v1/autotrader/disable`
- POST `/api/v1/autotrader/pause`
- POST `/api/v1/autotrader/resume`
- POST `/api/v1/autotrader/scan`
- GET `/api/v1/autotrader/cycles`
- GET `/api/v1/autotrader/opportunities`
- GET `/api/v1/autotrader/performance`
- GET `/api/v1/autotrader/reconciliation`
- GET `/api/v1/autotrader/risk`
- GET `/api/v1/autotrader/insights`
- GET `/api/v1/autotrader/experiments`
- POST `/api/v1/autotrader/experiments/{id}/approve`

Use authentication and account authorization for all state-changing operations. Protect against concurrent jobs and duplicate order execution.

## 12. Reliability and Testing

Add deterministic tests for:

- 15-minute scheduling and market-session boundaries.
- Exchange holidays and early closures.
- Stale and missing data.
- Partial provider failures.
- Duplicate scan prevention.
- Duplicate order prevention.
- Position sizing and risk limits.
- Daily and weekly loss circuit breakers.
- Entry and exit execution.
- Stop-loss gaps and slippage.
- Price reconciliation.
- Weekly equity calculations.
- Fee calculations.
- Trade outcome analytics.
- Strategy versioning and approval.
- Out-of-sample optimization.
- Look-ahead bias prevention.
- Recovery after application restart.

Run the full Go test suite and build the application after integrating the new feature.

## 13. Acceptance Criteria

The feature is complete when:

- Bearabull can scan the configured NIFTY 50 universe on a 15-minute schedule during eligible market hours.
- Data freshness is validated before trading.
- The five strongest qualified candidates are identified without forcing five positions.
- Simulated trades are placed and managed automatically using virtual funds.
- Every decision and fill is auditable.
- Paper fills can be reconciled with actual observed prices.
- P&L, costs, portfolio risk, and drawdown are calculated correctly.
- The weekly 1–10% objective is tracked without encouraging reckless trading.
- The system produces measurable strategy-analysis reports.
- Proposed strategy improvements require out-of-sample validation and explicit approval.
- No order is ever sent to a real broker.
- The dashboard and documentation clearly explain the limitations of simulated trading.

Inspect the existing Bearabull repository first. Reuse existing modules, avoid duplicate implementations, implement the feature in phases, run automated tests, and provide a final summary of the files changed, endpoints added, configuration settings, and verification results.
