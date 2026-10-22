// app.js — Main application logic for Bearabull platform

const STATE = {
  currentTab: 'dashboard',
  stocks: [],
  signals: [],
  watchlist: new Set(),
  activeExplorerSymbol: 'RELIANCE.NS',
  activeTimeframeLimit: 365,
  isSyncing: false,
};

// DOM Content Loaded
document.addEventListener('DOMContentLoaded', () => {
  initNavigation();
  initGlobalSearch();
  initSyncControls();
  initScreenerEvents();
  initExplorerEvents();
  initWatchlistEvents();
  initBacktestEvents();
  initAutotraderEvents();

  // Initial load
  loadInitialData();
});

// INITIAL DATA BOOTSTRAP
async function loadInitialData() {
  try {
    await loadStocksUniverse();
    await Promise.all([
      loadDashboardOverview(),
      loadWeeklySignals(),
      loadWatchlist(),
    ]);
  } catch (err) {
    console.error('Initialization error:', err);
    showToast('Failed to initialize some market data. Check network status.', 'error');
  }
}

// 1. NAVIGATION
function initNavigation() {
  const navItems = document.querySelectorAll('.nav-item');
  navItems.forEach(item => {
    item.addEventListener('click', () => {
      const targetTab = item.getAttribute('data-tab');
      switchTab(targetTab);
    });
  });

  // Mobile drawer controls
  const mobileBtn = document.getElementById('mobileMenuBtn');
  const closeBtn = document.getElementById('mobileCloseBtn');
  const sidebar = document.getElementById('sidebar');

  if (mobileBtn && sidebar) {
    mobileBtn.addEventListener('click', () => sidebar.classList.add('mobile-open'));
  }
  if (closeBtn && sidebar) {
    closeBtn.addEventListener('click', () => sidebar.classList.remove('mobile-open'));
  }
}

function switchTab(tabId) {
  STATE.currentTab = tabId;

  // Update nav buttons
  document.querySelectorAll('.nav-item').forEach(b => {
    b.classList.toggle('active', b.getAttribute('data-tab') === tabId);
  });

  // Update panels
  document.querySelectorAll('.tab-panel').forEach(p => {
    p.classList.toggle('active', p.id === `tab-${tabId}`);
  });

  // Close mobile sidebar if open
  document.getElementById('sidebar')?.classList.remove('mobile-open');

  // Trigger tab-specific loads
  if (tabId === 'dashboard') {
    loadDashboardOverview();
  } else if (tabId === 'autotrader') {
    loadAutotraderDashboard();
  } else if (tabId === 'screener') {
    loadScreenerData();
  } else if (tabId === 'signals') {
    renderSignalsView();
  } else if (tabId === 'explorer') {
    loadStockExplorer(STATE.activeExplorerSymbol);
  } else if (tabId === 'watchlist') {
    loadWatchlist();
  } else if (tabId === 'settings') {
    renderSettingsUniverse();
  }
}

// 2. STOCKS UNIVERSE
async function loadStocksUniverse() {
  const res = await fetch('/api/v1/stocks');
  if (!res.ok) return;
  const data = await res.json();
  STATE.stocks = data.stocks || [];

  // Populate Explorer dropdown
  const select = document.getElementById('explorerStockSelect');
  if (select) {
    select.innerHTML = '';
    STATE.stocks.forEach(s => {
      const opt = document.createElement('option');
      opt.value = s.symbol;
      opt.textContent = `${s.ticker} — ${s.company_name}`;
      select.appendChild(opt);
    });
  }

  // Populate Screener sector filter
  const sectorFilter = document.getElementById('screenerSectorFilter');
  if (sectorFilter) {
    const sectors = [...new Set(STATE.stocks.map(s => s.sector).filter(Boolean))].sort();
    sectorFilter.innerHTML = '<option value="ALL">All Sectors</option>';
    sectors.forEach(sec => {
      const opt = document.createElement('option');
      opt.value = sec;
      opt.textContent = sec;
      sectorFilter.appendChild(opt);
    });
  }
}

// 3. DASHBOARD
async function loadDashboardOverview() {
  try {
    const res = await fetch('/api/v1/market/overview');
    if (!res.ok) return;
    const data = await res.json();

    // Top stats
    document.getElementById('statBullCount').textContent = data.bullish_count ?? 0;
    document.getElementById('statBearCount').textContent = data.bearish_count ?? 0;
    document.getElementById('statNeutralCount').textContent = data.neutral_count ?? 0;

    const breadth50 = data.market_breadth?.pct_above_sma50 ?? 0;
    const breadth200 = data.market_breadth?.pct_above_sma200 ?? 0;

    document.getElementById('statBreadth50').textContent = `${breadth50}%`;
    document.getElementById('lblBreadth50').textContent = `${breadth50}%`;
    document.getElementById('barBreadth50').style.width = `${breadth50}%`;

    document.getElementById('lblBreadth200').textContent = `${breadth200}%`;
    document.getElementById('barBreadth200').style.width = `${breadth200}%`;

    // Topbar pills
    const niftyChg = data.index_summary?.avg_weekly_change ?? 0;
    const niftyChgEl = document.getElementById('topbarNiftyChange');
    niftyChgEl.textContent = `${niftyChg >= 0 ? '+' : ''}${niftyChg.toFixed(2)}% (Avg)`;
    niftyChgEl.className = `pill-value ${niftyChg >= 0 ? 'bull-text' : 'bear-text'}`;

    document.getElementById('topbarBreadth').textContent = `${breadth50}% > SMA50`;

    // Demo Mode & Status Badge
    const isDemo = data.data_status?.is_demo_mode;
    document.getElementById('demoBadge')?.classList.toggle('hidden', !isDemo);

    if (data.data_status?.last_sync) {
      const d = new Date(data.data_status.last_sync);
      const str = d.toLocaleDateString() + ' ' + d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
      document.getElementById('lastSyncMeta').textContent = `Synced: ${str}`;
      document.getElementById('dashboardAsOf').textContent = `As of: ${str}`;
    }

    // Top Gainers & Losers
    renderMoversList('topGainersList', data.top_gainers || [], true);
    renderMoversList('topLosersList', data.top_losers || [], false);

    // Opportunity Table
    loadOpportunityTable();
  } catch (err) {
    console.error('Error loading dashboard overview:', err);
  }
}

function renderMoversList(containerId, items, isGainer) {
  const container = document.getElementById(containerId);
  if (!container) return;
  if (items.length === 0) {
    container.innerHTML = '<div class="empty-state">No data available</div>';
    return;
  }

  container.innerHTML = items.map(item => `
    <div class="mini-row" onclick="openExplorerForSymbol('${item.symbol}')">
      <div>
        <div class="mini-sym">${getTickerFromSymbol(item.symbol)}</div>
        <div class="mini-name">${escapeHTML(item.company_name)}</div>
      </div>
      <div class="mini-val ${item.weekly_change >= 0 ? 'bull-text' : 'bear-text'}">
        ${item.weekly_change >= 0 ? '+' : ''}${item.weekly_change.toFixed(2)}%
      </div>
    </div>
  `).join('');
}

async function loadOpportunityTable() {
  const tbody = document.getElementById('opportunityTableBody');
  if (!tbody) return;

  try {
    const res = await fetch('/api/v1/signals/weekly');
    if (!res.ok) return;
    const data = await res.json();
    STATE.signals = data.signals || [];

    renderOpportunityRows(STATE.signals);

    // Setup filter input
    const filterInput = document.getElementById('dashFilterInput');
    if (filterInput) {
      filterInput.oninput = () => {
        const query = filterInput.value.toLowerCase().trim();
        const filtered = STATE.signals.filter(s =>
          s.symbol.toLowerCase().includes(query) ||
          (s.company_name && s.company_name.toLowerCase().includes(query)) ||
          (s.sector && s.sector.toLowerCase().includes(query))
        );
        renderOpportunityRows(filtered);
      };
    }
  } catch (err) {
    tbody.innerHTML = '<tr><td colspan="11" class="empty-state">Failed to load weekly opportunities</td></tr>';
  }
}

function renderOpportunityRows(signalsList) {
  const tbody = document.getElementById('opportunityTableBody');
  if (!tbody) return;

  if (signalsList.length === 0) {
    tbody.innerHTML = '<tr><td colspan="11" class="empty-state">No matching stock signals found</td></tr>';
    return;
  }

  tbody.innerHTML = signalsList.map(s => {
    const ticker = getTickerFromSymbol(s.symbol);
    const sigClass = s.signal.toLowerCase();
    const isBull = s.weekly_change_pct >= 0;

    return `
      <tr>
        <td>
          <div class="sym-col">
            <span class="sym-ticker">${ticker}</span>
            <span class="sym-name">${escapeHTML(s.company_name || '')}</span>
          </div>
        </td>
        <td><span class="sector-badge">${escapeHTML(s.sector || '')}</span></td>
        <td class="num">₹${s.last_close > 0 ? s.last_close.toFixed(2) : '--'}</td>
        <td class="num ${isBull ? 'bull-text' : 'bear-text'}">
          ${isBull ? '+' : ''}${s.weekly_change_pct.toFixed(2)}%
        </td>
        <td><span class="signal-pill ${sigClass}">${s.signal}</span></td>
        <td class="num">
          <div class="score-bar-inline">
            <strong>${Math.round(s.score)}</strong>
            <div class="score-mini-track">
              <div class="score-mini-fill ${s.signal === 'BUY' ? 'bull' : (s.signal === 'SELL' ? 'bear' : 'neutral')}" 
                   style="width: ${s.score}%"></div>
            </div>
          </div>
        </td>
        <td class="num">${s.trend_class || '--'}</td>
        <td><span class="pill-value">${s.momentum_class || 'Neutral'}</span></td>
        <td class="num">${s.risk_reward > 0 ? s.risk_reward.toFixed(1) + 'x' : '--'}</td>
        <td><span class="badge ${s.volatility_class === 'High' ? 'bear' : 'bull'}">${s.volatility_class || 'Normal'}</span></td>
        <td>
          <button class="btn-table-action" onclick="openExplorerForSymbol('${s.symbol}')">Inspect</button>
        </td>
      </tr>
    `;
  }).join('');
}

// 4. SCREENER
function initScreenerEvents() {
  document.querySelectorAll('.preset-chips .chip').forEach(chip => {
    chip.addEventListener('click', () => {
      document.querySelectorAll('.preset-chips .chip').forEach(c => c.classList.remove('active'));
      chip.classList.add('active');
      loadScreenerData();
    });
  });

  const controls = ['screenerSignalFilter', 'screenerSectorFilter', 'screenerRiskFilter', 'screenerSortBy'];
  controls.forEach(id => {
    document.getElementById(id)?.addEventListener('change', loadScreenerData);
  });

  const slider = document.getElementById('screenerMinScore');
  slider?.addEventListener('input', () => {
    document.getElementById('valMinScore').textContent = slider.value;
    loadScreenerData();
  });
}

async function loadScreenerData() {
  const tbody = document.getElementById('screenerTableBody');
  if (!tbody) return;

  const activeChip = document.querySelector('.preset-chips .chip.active');
  const preset = activeChip ? activeChip.getAttribute('data-preset') : '';

  const signal = document.getElementById('screenerSignalFilter')?.value || 'ALL';
  const sector = document.getElementById('screenerSectorFilter')?.value || 'ALL';
  const minScore = document.getElementById('screenerMinScore')?.value || '0';
  const risk = document.getElementById('screenerRiskFilter')?.value || 'ALL';
  const sortBy = document.getElementById('screenerSortBy')?.value || 'score';

  const params = new URLSearchParams({
    preset,
    signal,
    sector,
    min_score: minScore,
    risk_level: risk,
    sort_by: sortBy,
  });

  try {
    const res = await fetch(`/api/v1/screener?${params.toString()}`);
    if (!res.ok) return;
    const data = await res.json();
    const items = data.items || [];

    document.getElementById('screenerCount').textContent = items.length;

    if (items.length === 0) {
      tbody.innerHTML = '<tr><td colspan="12" class="empty-state">No stocks matched the active screener filters</td></tr>';
      return;
    }

    tbody.innerHTML = items.map(item => `
      <tr>
        <td>
          <div class="sym-col">
            <span class="sym-ticker">${item.ticker}</span>
            <span class="sym-name">${escapeHTML(item.company_name)}</span>
          </div>
        </td>
        <td><span class="sector-badge">${escapeHTML(item.sector)}</span></td>
        <td class="num">₹${item.last_close > 0 ? item.last_close.toFixed(2) : '--'}</td>
        <td class="num ${item.weekly_change_pct >= 0 ? 'bull-text' : 'bear-text'}">
          ${item.weekly_change_pct >= 0 ? '+' : ''}${item.weekly_change_pct.toFixed(2)}%
        </td>
        <td><span class="signal-pill ${item.signal.toLowerCase()}">${item.signal}</span></td>
        <td class="num"><strong>${Math.round(item.score)}</strong>/100</td>
        <td class="num">${item.rsi > 0 ? item.rsi.toFixed(1) : '--'}</td>
        <td class="num">${item.rvol > 0 ? item.rvol.toFixed(2) + 'x' : '--'}</td>
        <td class="num">₹${item.support_level > 0 ? item.support_level.toFixed(2) : '--'}</td>
        <td class="num">₹${item.resist_level > 0 ? item.resist_level.toFixed(2) : '--'}</td>
        <td><span class="badge ${item.volatility_class === 'High' ? 'bear' : 'bull'}">${item.volatility_class || 'Normal'}</span></td>
        <td>
          <button class="btn-table-action" onclick="openExplorerForSymbol('${item.symbol}')">View Chart</button>
        </td>
      </tr>
    `).join('');
  } catch (err) {
    tbody.innerHTML = '<tr><td colspan="12" class="empty-state">Error loading screener data</td></tr>';
  }
}

// 5. WEEKLY SIGNALS DOSSIERS
async function loadWeeklySignals() {
  const res = await fetch('/api/v1/signals/weekly');
  if (!res.ok) return;
  const data = await res.json();
  STATE.signals = data.signals || [];
}

function renderSignalsView() {
  const grid = document.getElementById('signalsCardsGrid');
  if (!grid) return;

  if (STATE.signals.length === 0) {
    grid.innerHTML = '<div class="empty-state">No weekly signals generated yet. Please run market sync.</div>';
    return;
  }

  grid.innerHTML = STATE.signals.map(s => {
    const ticker = getTickerFromSymbol(s.symbol);
    const sigClass = s.signal.toLowerCase();

    return `
      <div class="signal-card">
        <div class="sig-card-header">
          <div>
            <div class="sig-stock-title">${ticker} — ${escapeHTML(s.company_name || '')}</div>
            <div class="sig-stock-sector">${escapeHTML(s.sector || '')} • As of ${s.signal_date}</div>
          </div>
          <div class="sig-score-box">
            <span class="signal-pill ${sigClass}">${s.signal}</span>
            <div class="sig-score-num ${s.signal === 'BUY' ? 'bull-text' : (s.signal === 'SELL' ? 'bear-text' : 'neutral-text')}">
              ${Math.round(s.score)}/100
            </div>
          </div>
        </div>

        <div class="sig-levels-grid">
          <div>
            <span>Entry Zone</span>
            <strong>₹${s.entry_zone_low ? s.entry_zone_low.toFixed(1) : '--'} - ${s.entry_zone_high ? s.entry_zone_high.toFixed(1) : '--'}</strong>
          </div>
          <div>
            <span>Invalidation</span>
            <strong>₹${s.stop_loss ? s.stop_loss.toFixed(1) : '--'}</strong>
          </div>
          <div>
            <span>Risk / Reward</span>
            <strong>${s.risk_reward > 0 ? s.risk_reward.toFixed(1) + 'x' : '--'}</strong>
          </div>
        </div>

        <div class="sig-evidence-box">
          <strong class="bull-text" style="font-size: 11px;">BULLISH EVIDENCE:</strong>
          ${(s.bullish_evidence || []).slice(0, 2).map(ev => `
            <div class="sig-ev-item"><span>•</span><span>${escapeHTML(ev.description)}</span></div>
          `).join('') || '<div class="sig-ev-item text-dim">None recorded</div>'}
        </div>

        <div class="sig-evidence-box">
          <strong class="bear-text" style="font-size: 11px;">BEARISH RISKS:</strong>
          ${(s.bearish_evidence || []).slice(0, 2).map(ev => `
            <div class="sig-ev-item"><span>•</span><span>${escapeHTML(ev.description)}</span></div>
          `).join('') || '<div class="sig-ev-item text-dim">None recorded</div>'}
        </div>

        <div class="sig-rationale">
          ${escapeHTML(s.rationale || '')}
        </div>

        <div style="margin-top: auto; display: flex; justify-content: flex-end;">
          <button class="btn-primary" style="padding: 6px 14px; font-size: 12px;" onclick="openExplorerForSymbol('${s.symbol}')">
            Explore Full Chart & Indicators
          </button>
        </div>
      </div>
    `;
  }).join('');
}

// 6. STOCK EXPLORER
function initExplorerEvents() {
  const select = document.getElementById('explorerStockSelect');
  select?.addEventListener('change', () => {
    loadStockExplorer(select.value);
  });

  document.querySelectorAll('.timeframe-selector .tf-btn').forEach(btn => {
    btn.addEventListener('click', () => {
      document.querySelectorAll('.timeframe-selector .tf-btn').forEach(b => b.classList.remove('active'));
      btn.classList.add('active');
      STATE.activeTimeframeLimit = parseInt(btn.getAttribute('data-limit'), 10);
      loadStockChart(STATE.activeExplorerSymbol, STATE.activeTimeframeLimit);
    });
  });

  const watchlistToggle = document.getElementById('btnToggleWatchlist');
  watchlistToggle?.addEventListener('click', async () => {
    const sym = STATE.activeExplorerSymbol;
    if (STATE.watchlist.has(sym)) {
      await removeFromWatchlist(sym);
    } else {
      await addToWatchlist(sym);
    }
    updateWatchlistButtonState(sym);
  });
}

function openExplorerForSymbol(symbol) {
  STATE.activeExplorerSymbol = symbol;
  switchTab('explorer');
}

async function loadStockExplorer(symbol) {
  STATE.activeExplorerSymbol = symbol;

  const select = document.getElementById('explorerStockSelect');
  if (select && select.value !== symbol) {
    select.value = symbol;
  }

  updateWatchlistButtonState(symbol);

  try {
    const res = await fetch(`/api/v1/stocks/${symbol}`);
    if (!res.ok) return;
    const data = await res.json();

    const stock = data.stock || {};
    const sig = data.signal || {};
    const ind = data.indicators || {};
    const price = data.latest_price || {};

    // Populate Hero
    document.getElementById('heroCompanyName').textContent = stock.company_name || symbol;
    document.getElementById('heroTicker').textContent = stock.ticker || symbol;
    document.getElementById('heroSector').textContent = stock.sector || 'NIFTY 50';

    const lastClose = price.close || sig.last_close || 0;
    document.getElementById('heroPrice').textContent = `₹${lastClose.toFixed(2)}`;

    const chg = sig.weekly_change_pct || 0;
    const heroChg = document.getElementById('heroWeeklyChange');
    heroChg.textContent = `${chg >= 0 ? '+' : ''}${chg.toFixed(2)}% (Weekly)`;
    heroChg.className = `hero-change ${chg >= 0 ? 'bull-text' : 'bear-text'}`;

    const badge = document.getElementById('heroSignalBadge');
    badge.textContent = sig.signal || 'HOLD';
    badge.className = `signal-tag-large signal-pill ${(sig.signal || 'hold').toLowerCase()}`;

    document.getElementById('heroScore').textContent = Math.round(sig.score || 0);
    document.getElementById('heroEntryZone').textContent = `₹${sig.entry_zone_low ? sig.entry_zone_low.toFixed(1) : '--'} - ₹${sig.entry_zone_high ? sig.entry_zone_high.toFixed(1) : '--'}`;
    document.getElementById('heroStopLoss').textContent = `₹${sig.stop_loss ? sig.stop_loss.toFixed(1) : '--'}`;
    document.getElementById('heroRR').textContent = `${sig.risk_reward > 0 ? sig.risk_reward.toFixed(1) + 'x' : '--'}`;

    // Populate Indicators Grid
    renderIndicatorMatrix(ind, lastClose);

    // Evidence
    const bullUl = document.getElementById('heroBullEvidence');
    bullUl.innerHTML = (sig.bullish_evidence || []).map(ev => `
      <li><strong>${escapeHTML(ev.category)}:</strong> ${escapeHTML(ev.description)}</li>
    `).join('') || '<li class="text-dim">No bullish triggers recorded</li>';

    const bearUl = document.getElementById('heroBearEvidence');
    bearUl.innerHTML = (sig.bearish_evidence || []).map(ev => `
      <li><strong>${escapeHTML(ev.category)}:</strong> ${escapeHTML(ev.description)}</li>
    `).join('') || '<li class="text-dim">No bearish risk conditions met</li>';

    document.getElementById('heroRationale').textContent = sig.rationale || 'Consolidation phase without decisive directional triggers.';

    // Load Chart
    loadStockChart(symbol, STATE.activeTimeframeLimit);
  } catch (err) {
    console.error('Error loading stock details:', err);
  }
}

async function loadStockChart(symbol, limit) {
  try {
    const res = await fetch(`/api/v1/stocks/${symbol}/history?limit=${limit}`);
    if (!res.ok) return;
    const data = await res.json();
    renderCandlestickChart('tradingviewChart', data.bars || []);
  } catch (err) {
    console.error('Error loading chart:', err);
  }
}

function renderIndicatorMatrix(ind, close) {
  const container = document.getElementById('indicatorMatrix');
  if (!container) return;

  const cells = [
    { label: 'SMA 20', val: ind.sma_20 ? `₹${ind.sma_20.toFixed(2)}` : '--' },
    { label: 'SMA 50', val: ind.sma_50 ? `₹${ind.sma_50.toFixed(2)}` : '--' },
    { label: 'SMA 200', val: ind.sma_200 ? `₹${ind.sma_200.toFixed(2)}` : '--' },
    { label: 'EMA 9 / 21', val: ind.ema_9 ? `₹${ind.ema_9.toFixed(1)} / ₹${ind.ema_21.toFixed(1)}` : '--' },
    { label: 'RSI (14)', val: ind.rsi_14 ? ind.rsi_14.toFixed(1) : '--', color: ind.rsi_14 > 70 ? 'bear' : (ind.rsi_14 < 35 ? 'bull' : '') },
    { label: 'MACD Hist', val: ind.macd_hist ? ind.macd_hist.toFixed(2) : '--', color: ind.macd_hist >= 0 ? 'bull' : 'bear' },
    { label: 'Supertrend', val: ind.supertrend ? `₹${ind.supertrend.toFixed(1)}` : '--', color: ind.supertrend_dir === 1 ? 'bull' : 'bear' },
    { label: 'ADX (14)', val: ind.adx_14 ? ind.adx_14.toFixed(1) : '--' },
    { label: 'Bollinger Upper', val: ind.bb_upper ? `₹${ind.bb_upper.toFixed(2)}` : '--' },
    { label: 'Bollinger Lower', val: ind.bb_lower ? `₹${ind.bb_lower.toFixed(2)}` : '--' },
    { label: 'Relative Vol (RVOL)', val: ind.rvol ? `${ind.rvol.toFixed(2)}x` : '--', color: ind.rvol >= 1.25 ? 'bull' : '' },
    { label: 'ATR (14)', val: ind.atr_14 ? `₹${ind.atr_14.toFixed(2)}` : '--' },
    { label: 'Support Level', val: ind.support_level ? `₹${ind.support_level.toFixed(2)}` : '--' },
    { label: 'Resistance Level', val: ind.resist_level ? `₹${ind.resist_level.toFixed(2)}` : '--' },
    { label: 'Hist Volatility', val: ind.hist_volatility ? `${ind.hist_volatility.toFixed(1)}%` : '--' },
    { label: '14-Day ROC', val: ind.roc_14 ? `${ind.roc_14 >= 0 ? '+' : ''}${ind.roc_14.toFixed(1)}%` : '--' },
  ];

  container.innerHTML = cells.map(c => `
    <div class="ind-cell">
      <div class="ind-lbl">${c.label}</div>
      <div class="ind-val ${c.color === 'bull' ? 'bull-text' : (c.color === 'bear' ? 'bear-text' : '')}">${c.val}</div>
    </div>
  `).join('');
}

// 7. WATCHLIST
function initWatchlistEvents() {
  document.getElementById('btnAddToWatchlist')?.addEventListener('click', async () => {
    const input = document.getElementById('watchlistAddInput');
    const sym = input?.value.trim().toUpperCase();
    if (!sym) return;
    await addToWatchlist(sym);
    input.value = '';
    loadWatchlist();
  });
}

async function loadWatchlist() {
  try {
    const res = await fetch('/api/v1/watchlist');
    if (!res.ok) return;
    const data = await res.json();
    const items = data.items || [];

    STATE.watchlist.clear();
    items.forEach(i => STATE.watchlist.add(i.stock.symbol));

    const badge = document.getElementById('watchlistBadge');
    if (badge) badge.textContent = items.length;

    const grid = document.getElementById('watchlistGrid');
    if (!grid) return;

    if (items.length === 0) {
      grid.innerHTML = '<div class="empty-state">No stocks added to your watchlist yet. Add any NIFTY 50 constituent above.</div>';
      return;
    }

    grid.innerHTML = items.map(item => {
      const s = item.stock;
      const sig = item.signal || {};
      const p = item.last_price || {};
      const sigClass = (sig.signal || 'hold').toLowerCase();
      const lastClose = p.close || sig.last_close || 0;

      return `
        <div class="watchlist-card">
          <div class="watchlist-card-top">
            <div>
              <div style="font-weight: 700; font-size: 16px;">${s.ticker}</div>
              <div style="font-size: 11.5px; color: var(--text-dim);">${escapeHTML(s.company_name)}</div>
            </div>
            <span class="signal-pill ${sigClass}">${sig.signal || 'HOLD'}</span>
          </div>

          <div style="display: flex; justify-content: space-between; align-items: baseline; margin: 6px 0;">
            <div style="font-size: 22px; font-weight: 800; font-family: var(--font-mono);">
              ₹${lastClose > 0 ? lastClose.toFixed(2) : '--'}
            </div>
            <div style="font-size: 13px; font-weight: 600;" class="${(sig.weekly_change_pct || 0) >= 0 ? 'bull-text' : 'bear-text'}">
              ${(sig.weekly_change_pct || 0) >= 0 ? '+' : ''}${(sig.weekly_change_pct || 0).toFixed(2)}%
            </div>
          </div>

          <div style="font-size: 12px; color: var(--text-muted); display: flex; gap: 14px;">
            <span>Score: <strong>${Math.round(sig.score || 0)}/100</strong></span>
            <span>RSI: <strong>${item.indicators?.rsi_14 ? item.indicators.rsi_14.toFixed(1) : '--'}</strong></span>
          </div>

          <div style="display: flex; gap: 8px; margin-top: 8px;">
            <button class="btn-primary" style="flex: 1; padding: 6px 10px; font-size: 12px;" onclick="openExplorerForSymbol('${s.symbol}')">Inspect</button>
            <button class="btn-secondary" style="padding: 6px 12px; font-size: 12px;" onclick="removeFromWatchlist('${s.symbol}')">Remove</button>
          </div>
        </div>
      `;
    }).join('');
  } catch (err) {
    console.error('Error loading watchlist:', err);
  }
}

async function addToWatchlist(symbol) {
  try {
    const res = await fetch('/api/v1/watchlist', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ symbol }),
    });
    if (res.ok) {
      STATE.watchlist.add(symbol);
      showToast(`Added ${getTickerFromSymbol(symbol)} to watchlist`, 'success');
      loadWatchlist();
    }
  } catch (err) {
    showToast('Failed adding to watchlist', 'error');
  }
}

async function removeFromWatchlist(symbol) {
  try {
    const res = await fetch(`/api/v1/watchlist/${symbol}`, { method: 'DELETE' });
    if (res.ok) {
      STATE.watchlist.delete(symbol);
      showToast(`Removed from watchlist`, 'info');
      loadWatchlist();
    }
  } catch (err) {
    showToast('Failed removing from watchlist', 'error');
  }
}

function updateWatchlistButtonState(symbol) {
  const btn = document.getElementById('btnToggleWatchlist');
  const icon = document.getElementById('watchlistBtnIcon');
  const text = document.getElementById('watchlistBtnText');
  if (!btn || !icon || !text) return;

  if (STATE.watchlist.has(symbol)) {
    icon.textContent = '★';
    text.textContent = 'In Watchlist';
    btn.style.color = '#f59e0b';
  } else {
    icon.textContent = '⭐';
    text.textContent = 'Add to Watchlist';
    btn.style.color = '';
  }
}

// 8. BACKTESTING
function initBacktestEvents() {
  document.getElementById('btnRunBacktest')?.addEventListener('click', runBacktest);
}

async function runBacktest() {
  const btn = document.getElementById('btnRunBacktest');
  btn.disabled = true;
  btn.innerHTML = '<span>Simulating Trades...</span>';

  const payload = {
    initial_capital: parseFloat(document.getElementById('btCapital').value),
    max_positions: parseInt(document.getElementById('btMaxPositions').value, 10),
    position_size_pct: parseFloat(document.getElementById('btPosSize').value),
    min_score: parseFloat(document.getElementById('btMinScore').value),
    stop_loss_pct: parseFloat(document.getElementById('btStopLoss').value),
    target_pct: parseFloat(document.getElementById('btTarget').value),
    slippage_pct: parseFloat(document.getElementById('btSlippage').value),
    brokerage_fee_pct: parseFloat(document.getElementById('btFees').value),
  };

  try {
    const res = await fetch('/api/v1/backtests', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });

    if (!res.ok) {
      const errData = await res.json();
      throw new Error(errData.error || 'Backtest execution failed');
    }

    const data = await res.json();
    displayBacktestResults(data);
    showToast('Simulation complete!', 'success');
  } catch (err) {
    showToast(err.message, 'error');
  } finally {
    btn.disabled = false;
    btn.innerHTML = '<span>▶ Run Simulation</span>';
  }
}

function displayBacktestResults(data) {
  const resultsArea = document.getElementById('backtestResultsArea');
  resultsArea.classList.remove('hidden');

  document.getElementById('btTotalReturn').textContent = `${data.total_return >= 0 ? '+' : ''}${data.total_return.toFixed(2)}%`;
  document.getElementById('btTotalReturn').className = `stat-value ${data.total_return >= 0 ? 'bull-text' : 'bear-text'}`;

  document.getElementById('btVsBenchmark').textContent = `Benchmark: ${data.benchmark_return >= 0 ? '+' : ''}${data.benchmark_return.toFixed(2)}%`;
  document.getElementById('btCAGR').textContent = `${data.cagr.toFixed(2)}%`;
  document.getElementById('btMaxDD').textContent = `-${data.max_drawdown.toFixed(2)}%`;
  document.getElementById('btWinRate').textContent = `${data.win_rate.toFixed(1)}%`;
  document.getElementById('btTradesCount').textContent = `${data.total_trades} Executed Trades`;
  document.getElementById('btProfitFactor').textContent = data.profit_factor.toFixed(2);
  document.getElementById('btSharpe').textContent = data.sharpe_ratio.toFixed(2);

  // Render Equity Chart
  renderBacktestEquityChart('equityCurveBox', data.equity_curve || []);

  // Render Trades Table
  const tbody = document.getElementById('btTradesTableBody');
  if (tbody) {
    const trades = data.trades || [];
    if (trades.length === 0) {
      tbody.innerHTML = '<tr><td colspan="9" class="empty-state">No trades executed with selected criteria.</td></tr>';
      return;
    }
    tbody.innerHTML = trades.map(t => `
      <tr>
        <td><strong>${getTickerFromSymbol(t.symbol)}</strong></td>
        <td>${t.entry_date}</td>
        <td>${t.exit_date}</td>
        <td class="num">₹${t.entry_price.toFixed(2)}</td>
        <td class="num">₹${t.exit_price.toFixed(2)}</td>
        <td class="num">${t.shares}</td>
        <td class="num ${t.profit_loss >= 0 ? 'bull-text' : 'bear-text'}">₹${t.profit_loss.toFixed(2)}</td>
        <td class="num ${t.return_pct >= 0 ? 'bull-text' : 'bear-text'}">${t.return_pct >= 0 ? '+' : ''}${t.return_pct.toFixed(2)}%</td>
        <td><span class="badge ${t.profit_loss >= 0 ? 'bull' : 'bear'}">${escapeHTML(t.exit_reason)}</span></td>
      </tr>
    `).join('');
  }
}

// 9. SETTINGS & SYNC
function initSyncControls() {
  document.getElementById('btnSync')?.addEventListener('click', () => triggerSync(false));
  document.getElementById('btnTriggerLiveSync')?.addEventListener('click', () => triggerSync(false));
  document.getElementById('btnTriggerDemoSync')?.addEventListener('click', () => triggerSync(true));
}

async function triggerSync(forceDemo) {
  if (STATE.isSyncing) return;
  STATE.isSyncing = true;

  showToast(`Market sync initiated${forceDemo ? ' (Demo Baseline)' : ''}...`, 'info');

  try {
    const res = await fetch(`/api/v1/sync/trigger?demo=${forceDemo ? 'true' : 'false'}`, { method: 'POST' });
    if (!res.ok) {
      showToast('Sync trigger rejected', 'error');
      STATE.isSyncing = false;
      return;
    }

    // Poll status until completion
    const pollInterval = setInterval(async () => {
      const sRes = await fetch('/api/v1/sync/status');
      if (sRes.ok) {
        const sData = await sRes.json();
        if (!sData.is_running) {
          clearInterval(pollInterval);
          STATE.isSyncing = false;
          showToast('Market synchronization finished successfully!', 'success');
          loadInitialData();
        }
      }
    }, 2000);
  } catch (err) {
    STATE.isSyncing = false;
    showToast('Failed to trigger sync', 'error');
  }
}

function renderSettingsUniverse() {
  const tbody = document.getElementById('settingsUniverseTableBody');
  if (!tbody || STATE.stocks.length === 0) return;

  tbody.innerHTML = STATE.stocks.map(s => `
    <tr>
      <td><strong>${s.ticker}</strong></td>
      <td><code>${s.symbol}</code></td>
      <td>${escapeHTML(s.company_name)}</td>
      <td><span class="sector-badge">${escapeHTML(s.sector)}</span></td>
      <td>${s.exchange}</td>
      <td><span class="badge ${s.is_active ? 'bull' : 'bear'}">${s.is_active ? 'Active' : 'Inactive'}</span></td>
    </tr>
  `).join('');
}

// 10. GLOBAL SEARCH
function initGlobalSearch() {
  const input = document.getElementById('globalSearchInput');
  const dropdown = document.getElementById('searchResultsDropdown');
  if (!input || !dropdown) return;

  input.addEventListener('input', () => {
    const q = input.value.trim().toLowerCase();
    if (!q) {
      dropdown.style.display = 'none';
      return;
    }

    const matches = STATE.stocks.filter(s =>
      s.ticker.toLowerCase().includes(q) ||
      s.company_name.toLowerCase().includes(q) ||
      s.symbol.toLowerCase().includes(q)
    ).slice(0, 8);

    if (matches.length === 0) {
      dropdown.innerHTML = '<div style="padding: 10px; color: var(--text-dim); text-align: center;">No constituent matches</div>';
    } else {
      dropdown.innerHTML = matches.map(s => `
        <div class="search-result-item" onclick="selectSearchResult('${s.symbol}')">
          <div>
            <strong>${s.ticker}</strong>
            <span style="font-size: 11px; color: var(--text-dim); margin-left: 6px;">${escapeHTML(s.company_name)}</span>
          </div>
          <span class="sector-badge">${escapeHTML(s.sector)}</span>
        </div>
      `).join('');
    }
    dropdown.style.display = 'block';
  });

  document.addEventListener('click', (e) => {
    if (!e.target.closest('.search-box')) {
      dropdown.style.display = 'none';
    }
  });
}

function selectSearchResult(symbol) {
  document.getElementById('searchResultsDropdown').style.display = 'none';
  document.getElementById('globalSearchInput').value = '';
  openExplorerForSymbol(symbol);
}

// UTILITIES
function getTickerFromSymbol(symbol) {
  if (!symbol) return '';
  return symbol.replace('.NS', '');
}

function escapeHTML(str) {
  if (!str) return '';
  return str.replace(/[&<>'"]/g, tag => ({
    '&': '&amp;',
    '<': '&lt;',
    '>': '&gt;',
    "'": '&#39;',
    '"': '&quot;',
  }[tag] || tag));
}

function showToast(message, type = 'info') {
  const container = document.getElementById('toastContainer');
  if (!container) return;

  const toast = document.createElement('div');
  toast.className = `toast ${type}`;
  toast.textContent = message;
  container.appendChild(toast);

  setTimeout(() => {
    toast.remove();
  }, 4000);
}

/* ==============================================================
   AUTO-TRADER CONTROLLER & RENDERING LOGIC
   ============================================================== */

let autotraderTimer = null;
let atCurrentConfig = null;

function initAutotraderEvents() {
  const btnToggle = document.getElementById('btnAtToggle');
  const btnPause = document.getElementById('btnAtPause');
  const btnScan = document.getElementById('btnAtScan');

  if (btnToggle) {
    btnToggle.addEventListener('click', async () => {
      if (!atCurrentConfig) return;
      const willEnable = !atCurrentConfig.is_enabled;
      const endpoint = willEnable ? '/api/v1/autotrader/enable' : '/api/v1/autotrader/disable';
      try {
        btnToggle.disabled = true;
        const res = await fetch(endpoint, { method: 'POST' });
        const data = await res.json();
        if (data.success) {
          showToast(data.message, 'success');
          await loadAutotraderDashboard();
        } else {
          showToast(data.error || 'Failed to update auto-trader state', 'error');
        }
      } catch (err) {
        showToast('Network error updating auto-trader state', 'error');
      } finally {
        btnToggle.disabled = false;
      }
    });
  }

  if (btnPause) {
    btnPause.addEventListener('click', async () => {
      if (!atCurrentConfig) return;
      const willPause = !atCurrentConfig.is_paused;
      const endpoint = willPause ? '/api/v1/autotrader/pause' : '/api/v1/autotrader/resume';
      try {
        btnPause.disabled = true;
        const res = await fetch(endpoint, { method: 'POST' });
        const data = await res.json();
        if (data.success) {
          showToast(data.message, 'info');
          await loadAutotraderDashboard();
        } else {
          showToast(data.error || 'Failed to toggle pause state', 'error');
        }
      } catch (err) {
        showToast('Network error toggling pause state', 'error');
      } finally {
        btnPause.disabled = false;
      }
    });
  }

  if (btnScan) {
    btnScan.addEventListener('click', async () => {
      try {
        btnScan.disabled = true;
        btnScan.innerHTML = '<span class="pulse-dot"></span> Scanning...';
        const res = await fetch('/api/v1/autotrader/scan', { method: 'POST' });
        const data = await res.json();
        if (data.success) {
          showToast('15-Minute scan completed successfully!', 'success');
          await loadAutotraderDashboard();
        } else {
          showToast(data.error || 'Scan cycle error', 'error');
        }
      } catch (err) {
        showToast('Network error triggering manual scan', 'error');
      } finally {
        btnScan.disabled = false;
        btnScan.innerHTML = `<svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" style="margin-right:4px;"><path d="M21.5 2v6h-6M21.34 15.57a10 10 0 1 1-.57-8.38l5.67-5.67"/></svg><span>Scan (15m)</span>`;
      }
    });
  }
}

async function loadAutotraderDashboard() {
  try {
    const [statusRes, oppsRes, perfRes, reconRes, insightsRes, expRes, cyclesRes] = await Promise.all([
      fetch('/api/v1/autotrader/status').then(r => r.json()),
      fetch('/api/v1/autotrader/opportunities').then(r => r.json()),
      fetch('/api/v1/autotrader/performance').then(r => r.json()),
      fetch('/api/v1/autotrader/reconciliation').then(r => r.json()),
      fetch('/api/v1/autotrader/insights').then(r => r.json()),
      fetch('/api/v1/autotrader/experiments').then(r => r.json()),
      fetch('/api/v1/autotrader/cycles').then(r => r.json()),
    ]);

    if (statusRes && statusRes.config) {
      renderAutotraderStatus(statusRes);
    }
    if (oppsRes && oppsRes.opportunities) {
      renderAutotraderOpportunities(oppsRes.opportunities);
    }
    if (perfRes) {
      renderAutotraderPositions(perfRes.open_positions || []);
    }
    if (reconRes && reconRes.reconciliations) {
      renderAutotraderReconciliation(reconRes.reconciliations);
    }
    if (insightsRes) {
      renderAutotraderInsights(insightsRes);
    }
    if (expRes) {
      renderAutotraderExperiments(expRes.experiments || [], expRes.strategy_versions || []);
    }
    if (cyclesRes && cyclesRes.cycles) {
      renderAutotraderCycles(cyclesRes.cycles);
    }
  } catch (err) {
    console.error('Failed to load autotrader dashboard data:', err);
  }
}

function renderAutotraderStatus(st) {
  atCurrentConfig = st.config;
  const cfg = st.config;

  // Toggle & pause button states
  const btnToggle = document.getElementById('btnAtToggle');
  const btnPause = document.getElementById('btnAtPause');
  const statusBadge = document.getElementById('atStatusBadge');
  const sessionBadge = document.getElementById('atSessionBadge');

  if (btnToggle) {
    if (cfg.is_enabled) {
      btnToggle.textContent = 'Disable Auto-Trading';
      btnToggle.className = 'btn-secondary';
    } else {
      btnToggle.textContent = 'Enable Auto-Trading';
      btnToggle.className = 'btn-primary';
    }
  }

  if (btnPause) {
    if (cfg.is_paused) {
      btnPause.textContent = 'Resume Entries';
      btnPause.style.borderColor = 'var(--bull-primary)';
      btnPause.style.color = 'var(--bull-primary)';
    } else {
      btnPause.textContent = 'Pause Entries';
      btnPause.style.borderColor = '';
      btnPause.style.color = '';
    }
  }

  if (statusBadge) {
    if (!cfg.is_enabled) {
      statusBadge.className = 'badge badge-disabled';
      statusBadge.innerHTML = '<span class="pulse-dot"></span>DISABLED';
    } else if (cfg.is_paused) {
      statusBadge.className = 'badge badge-paused';
      statusBadge.innerHTML = '<span class="pulse-dot"></span>ENTRIES PAUSED';
    } else {
      statusBadge.className = 'badge badge-enabled';
      statusBadge.innerHTML = '<span class="pulse-dot"></span>AUTO ACTIVE';
    }
  }

  if (sessionBadge) {
    sessionBadge.textContent = st.market_session_status || 'Session: Active';
    sessionBadge.style.color = st.is_market_open ? 'var(--bull-primary)' : 'var(--text-muted)';
  }

  // Account card
  const totalEqEl = document.getElementById('atTotalEquity');
  const cashEl = document.getElementById('atCurrentCash');
  const dayPnLEl = document.getElementById('atDayPnL');

  if (totalEqEl) totalEqEl.textContent = `₹${cfg.current_equity.toLocaleString('en-IN', { maximumFractionDigits: 2 })}`;
  if (cashEl) cashEl.textContent = `₹${cfg.current_cash.toLocaleString('en-IN', { maximumFractionDigits: 2 })}`;
  if (dayPnLEl) {
    const daySign = st.daily_pnl_pct >= 0 ? '+' : '';
    dayPnLEl.textContent = `Day Move: ${daySign}${st.daily_pnl_pct.toFixed(2)}%`;
  }

  // Weekly target card
  const returnEl = document.getElementById('atWeeklyReturn');
  const targetBadge = document.getElementById('atTargetStatusBadge');
  const progressFill = document.getElementById('atTargetProgressFill');
  const remSessionsEl = document.getElementById('atRemainingSessions');
  const realizedEl = document.getElementById('atWeeklyRealized');

  if (returnEl) {
    const sign = st.weekly_return_pct >= 0 ? '+' : '';
    returnEl.textContent = `${sign}${st.weekly_return_pct.toFixed(2)}%`;
    returnEl.style.color = st.weekly_return_pct >= 1.0 ? 'var(--bull-primary)' : (st.weekly_return_pct < 0 ? 'var(--bear-primary)' : 'var(--text-main)');
  }

  if (targetBadge) {
    if (st.weekly_return_pct >= cfg.weekly_target_min_pct) {
      targetBadge.className = 'stat-badge bull';
      targetBadge.textContent = 'IN RANGE';
    } else {
      targetBadge.className = 'stat-badge neutral';
      targetBadge.textContent = 'BELOW 1%';
    }
  }

  if (progressFill) {
    const pct = Math.max(0, Math.min(100, (st.weekly_return_pct / cfg.weekly_target_max_pct) * 100));
    progressFill.style.width = `${pct}%`;
  }

  if (realizedEl) realizedEl.textContent = `Realized: ₹${st.weekly_realized_pnl.toLocaleString('en-IN', { maximumFractionDigits: 2 })}`;

  // Risk card
  const riskExposedEl = document.getElementById('atRiskExposed');
  const aggRiskEl = document.getElementById('atAggregateRiskPct');
  const drawdownEl = document.getElementById('atDrawdownPct');
  const remDailyRiskEl = document.getElementById('atRemainingDailyRisk');

  if (riskExposedEl) riskExposedEl.textContent = `₹${st.total_planned_risk.toLocaleString('en-IN', { maximumFractionDigits: 2 })}`;
  if (aggRiskEl) {
    const aggPct = ((st.total_planned_risk / cfg.current_equity) * 100).toFixed(2);
    aggRiskEl.textContent = `${aggPct}% / ${cfg.max_aggregate_risk_pct}%`;
  }
  if (drawdownEl) drawdownEl.textContent = `${st.weekly_drawdown_pct.toFixed(2)}% / ${cfg.max_weekly_drawdown_pct}%`;
  if (remDailyRiskEl) remDailyRiskEl.textContent = `${st.remaining_daily_risk_pct.toFixed(2)}% remaining`;

  // Scheduler & Polling card
  const lastScanEl = document.getElementById('atLastScanTime');
  const nextScanEl = document.getElementById('atNextScanTime');
  const openPosCountEl = document.getElementById('atOpenPositionsCount');

  if (lastScanEl) {
    if (st.last_scan_at) {
      const d = new Date(st.last_scan_at);
      lastScanEl.textContent = d.toLocaleTimeString('en-IN', { hour: '2-digit', minute: '2-digit', second: '2-digit' });
    } else {
      lastScanEl.textContent = 'Initialized';
    }
  }

  if (nextScanEl && st.next_scan_at) {
    const d = new Date(st.next_scan_at);
    nextScanEl.textContent = d.toLocaleTimeString('en-IN', { hour: '2-digit', minute: '2-digit' });
  }

  if (openPosCountEl) openPosCountEl.textContent = `${st.open_positions_count} / ${cfg.max_positions}`;
}

function renderAutotraderOpportunities(opps) {
  const container = document.getElementById('atOpportunitiesGrid');
  if (!container) return;

  if (!opps || opps.length === 0) {
    container.innerHTML = '<div class="empty-state">No qualified opportunities identified in the latest scan. All filters passed: Cash preserved.</div>';
    return;
  }

  container.innerHTML = opps.map((opp, idx) => {
    let ind = {};
    let smc = {};
    try { if (opp.indicators_json) ind = JSON.parse(opp.indicators_json); } catch(e){}
    try { if (opp.order_block_json) smc = JSON.parse(opp.order_block_json); } catch(e){}

    const ticker = getTickerFromSymbol(opp.symbol);
    const scoreColor = opp.score >= 80 ? 'var(--bull-primary)' : 'var(--text-main)';
    const freshTag = smc.active_bullish_ob && !smc.active_bullish_ob.is_mitigated ? '<span class="opp-tag fresh">Fresh OB</span>' : '<span class="opp-tag">Mitigated OB</span>';
    const bosTag = smc.last_bos ? `<span class="opp-tag" style="color:#60a5fa; border-color:rgba(96,165,250,0.3);">BOS ₹${smc.last_bos.level}</span>` : '';

    return `
      <div class="opp-card">
        <span class="opp-rank-badge">#${idx + 1} • Score ${opp.score.toFixed(1)}</span>
        <div class="opp-header">
          <div>
            <div class="opp-symbol">${escapeHTML(ticker)}</div>
            <div class="opp-company">${escapeHTML(opp.symbol)}</div>
          </div>
        </div>

        <div class="opp-metrics-row">
          <div>
            <div class="opp-metric-label">Entry</div>
            <div class="opp-metric-val">₹${(smc.recommended_entry || smc.current_price || 0).toFixed(2)}</div>
          </div>
          <div>
            <div class="opp-metric-label">Stop Loss</div>
            <div class="opp-metric-val" style="color:var(--bear-primary);">₹${(smc.stop_loss || 0).toFixed(2)}</div>
          </div>
          <div>
            <div class="opp-metric-label">Target (R:R)</div>
            <div class="opp-metric-val" style="color:var(--bull-primary);">1:${(smc.risk_reward_ratio || 2.5).toFixed(1)}</div>
          </div>
        </div>

        <div class="opp-tags">
          <span class="opp-tag" style="background:rgba(16,185,129,0.15); color:var(--bull-primary); font-weight:700;">BUY LONG</span>
          ${freshTag}
          ${bosTag}
          <span class="opp-tag">RSI ${(ind.rsi_14 || 52).toFixed(1)}</span>
        </div>

        <div class="opp-rationale">
          ${escapeHTML(opp.rejection_reason || 'Qualified SMC bullish order-block confirmation with technical target feasibility.')}
        </div>
      </div>
    `;
  }).join('');
}

function renderAutotraderPositions(positions) {
  const tbody = document.getElementById('atPositionsTableBody');
  const badge = document.getElementById('atOpenPositionsBadge');
  if (badge) badge.textContent = `${positions.length} Active`;
  if (!tbody) return;

  if (!positions || positions.length === 0) {
    tbody.innerHTML = '<tr><td colspan="10" class="empty-state">No open paper positions. Full virtual cash preserved.</td></tr>';
    return;
  }

  tbody.innerHTML = positions.map(pos => {
    const pnl = pos.unrealized_pnl || 0;
    const pnlClass = pnl >= 0 ? 'bull-text' : 'bear-text';
    const sign = pnl >= 0 ? '+' : '';

    return `
      <tr>
        <td><strong>${escapeHTML(getTickerFromSymbol(pos.symbol))}</strong></td>
        <td><span class="badge" style="background:rgba(16,185,129,0.15); color:var(--bull-primary);">${escapeHTML(pos.side)}</span></td>
        <td style="font-family:var(--font-mono);">${pos.shares}</td>
        <td style="font-family:var(--font-mono);">₹${pos.entry_price.toFixed(2)}</td>
        <td style="font-family:var(--font-mono);">₹${pos.current_price.toFixed(2)}</td>
        <td style="font-family:var(--font-mono);" class="${pnlClass}">${sign}₹${pnl.toFixed(2)}</td>
        <td style="font-family:var(--font-mono); color:var(--bear-primary);">₹${pos.stop_loss.toFixed(2)}</td>
        <td style="font-family:var(--font-mono); color:var(--bull-primary);">₹${pos.target_price.toFixed(2)}</td>
        <td style="font-family:var(--font-mono);">₹${pos.max_price_seen.toFixed(2)}</td>
        <td style="font-family:var(--font-mono);">₹${pos.min_price_seen.toFixed(2)}</td>
      </tr>
    `;
  }).join('');
}

function renderAutotraderReconciliation(reconciliations) {
  const tbody = document.getElementById('atReconciliationTableBody');
  if (!tbody) return;

  if (!reconciliations || reconciliations.length === 0) {
    tbody.innerHTML = '<tr><td colspan="10" class="empty-state">No execution comparisons recorded yet. Fills are logged with millisecond deviations against market quotes.</td></tr>';
    return;
  }

  tbody.innerHTML = reconciliations.map(r => {
    const dateStr = r.decision_time ? new Date(r.decision_time).toLocaleTimeString('en-IN', { hour: '2-digit', minute: '2-digit' }) : '--';
    const exitP = r.simulated_exit_price ? `₹${r.simulated_exit_price.toFixed(2)}` : '--';
    const pnlClass = r.net_pnl >= 0 ? 'bull-text' : 'bear-text';
    const sign = r.net_pnl >= 0 ? '+' : '';

    return `
      <tr>
        <td><strong>${escapeHTML(getTickerFromSymbol(r.symbol))}</strong></td>
        <td>${escapeHTML(dateStr)}</td>
        <td style="font-family:var(--font-mono);">₹${r.simulated_fill_price.toFixed(2)}</td>
        <td style="font-family:var(--font-mono);">₹${r.best_observed_price.toFixed(2)}</td>
        <td style="font-family:var(--font-mono); color:${r.entry_deviation_bps > 10 ? 'var(--bear-primary)' : 'var(--text-muted)'};">${r.entry_deviation_bps.toFixed(1)} bps</td>
        <td style="font-family:var(--font-mono);">${exitP}</td>
        <td style="font-family:var(--font-mono);">${r.exit_deviation_bps.toFixed(1)} bps</td>
        <td style="font-family:var(--font-mono); color:var(--bear-primary);">₹${r.slippage_cost.toFixed(2)}</td>
        <td style="font-family:var(--font-mono);" class="${pnlClass}">${sign}₹${r.net_pnl.toFixed(2)}</td>
        <td style="font-family:var(--font-mono);">${r.holding_duration_minutes.toFixed(0)}m</td>
      </tr>
    `;
  }).join('');
}

function renderAutotraderInsights(insights) {
  const container = document.getElementById('atFindingsList');
  if (!container) return;

  const findings = insights.findings || [];
  if (findings.length === 0) {
    container.innerHTML = '<div class="finding-item">Sufficient trade history being collected. Statistical findings will appear automatically.</div>';
    return;
  }

  container.innerHTML = findings.map(f => `
    <div class="finding-item">
      <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" style="flex-shrink:0; margin-top:2px; color:var(--info-primary);"><circle cx="12" cy="12" r="10"/><line x1="12" y1="16" x2="12" y2="12"/><line x1="12" y1="8" x2="12.01" y2="8"/></svg>
      <div>${escapeHTML(f)}</div>
    </div>
  `).join('');
}

function renderAutotraderExperiments(experiments, versions) {
  const container = document.getElementById('atExperimentsContainer');
  const activeVerTag = document.getElementById('atActiveVersionTag');

  const active = versions.find(v => v.is_active);
  if (activeVerTag && active) {
    activeVerTag.textContent = active.version_tag;
  }

  if (!container) return;

  if (!experiments || experiments.length === 0) {
    container.innerHTML = `<div class="empty-state">No proposed strategy experiments pending. Baseline active: <strong>${active ? active.version_tag : 'v1.0.0'}</strong>.</div>`;
    return;
  }

  container.innerHTML = experiments.map(exp => {
    const isApproved = exp.status === 'APPROVED';
    const approveBtn = isApproved ? 
      '<span class="badge" style="background:rgba(16,185,129,0.15); color:var(--bull-primary);">ACTIVE & APPROVED</span>' :
      `<button class="btn-primary" style="padding:4px 10px; font-size:12px;" onclick="approveExperiment(${exp.id})">Approve & Activate</button>`;

    return `
      <div style="background:var(--bg-app); border:1px solid var(--border-subtle); border-radius:var(--radius-sm); padding:12px; margin-bottom:10px;">
        <div style="display:flex; justify-content:space-between; align-items:center;">
          <div>
            <strong>${escapeHTML(exp.name)}</strong>
            <span style="font-size:12px; color:var(--text-dim); margin-left:6px;">(${escapeHTML(exp.base_version)} &rarr; ${escapeHTML(exp.candidate_version)})</span>
          </div>
          <div>${approveBtn}</div>
        </div>
        <p style="font-size:12px; color:var(--text-muted); margin-top:6px;">${escapeHTML(exp.description)}</p>
      </div>
    `;
  }).join('');
}

async function approveExperiment(id) {
  if (!confirm(`Are you sure you want to approve and switch the active strategy model to experiment #${id}? This will update the operational parameter set.`)) {
    return;
  }
  try {
    const res = await fetch(`/api/v1/autotrader/experiments/${id}/approve`, { method: 'POST' });
    const data = await res.json();
    if (data.success) {
      showToast(data.message, 'success');
      await loadAutotraderDashboard();
    } else {
      showToast(data.error || 'Failed to approve experiment', 'error');
    }
  } catch (err) {
    showToast('Network error approving experiment', 'error');
  }
}

function renderAutotraderCycles(cycles) {
  const tbody = document.getElementById('atCyclesTableBody');
  if (!tbody) return;

  if (!cycles || cycles.length === 0) {
    tbody.innerHTML = '<tr><td colspan="9" class="empty-state">No cycles recorded yet.</td></tr>';
    return;
  }

  tbody.innerHTML = cycles.slice(0, 10).map(c => {
    const startTime = c.started_at ? new Date(c.started_at).toLocaleTimeString('en-IN', { hour: '2-digit', minute: '2-digit', second: '2-digit' }) : '--';
    const statusClass = c.status === 'COMPLETED' ? 'bull-text' : (c.status === 'RUNNING' ? 'neutral-text' : 'bear-text');

    return `
      <tr>
        <td style="font-family:var(--font-mono);">#${c.id}</td>
        <td>${escapeHTML(c.session_date)}</td>
        <td>${escapeHTML(startTime)}</td>
        <td class="${statusClass}"><strong>${escapeHTML(c.status)}</strong></td>
        <td style="font-family:var(--font-mono);">${c.stocks_scanned}</td>
        <td style="font-family:var(--font-mono);">${c.shortlisted_count}</td>
        <td style="font-family:var(--font-mono);">${c.orders_submitted} / ${c.orders_filled}</td>
        <td><span class="badge" style="background:rgba(16,185,129,0.15); color:var(--bull-primary);">${escapeHTML(c.provider_health)}</span></td>
        <td style="max-width:250px; overflow:hidden; text-overflow:ellipsis; white-space:nowrap;" title="${escapeHTML(c.summary)}">${escapeHTML(c.summary || '--')}</td>
      </tr>
    `;
  }).join('');
}

