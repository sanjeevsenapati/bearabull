// charts.js — Lightweight Charts integration with offline fallback

let mainChart = null;
let candleSeries = null;
let volumeSeries = null;
let sma20Series = null;
let sma50Series = null;
let ema21Series = null;
let bbUpperSeries = null;
let bbLowerSeries = null;

let rsiChart = null;
let rsiSeries = null;

let macdChart = null;
let macdLineSeries = null;
let macdSignalSeries = null;
let macdHistSeries = null;

let equityChart = null;
let strategyEquitySeries = null;
let benchmarkEquitySeries = null;

// Initialize or update the main TradingView candlestick chart
function renderCandlestickChart(containerId, bars) {
  const container = document.getElementById(containerId);
  if (!container) return;

  if (mainChart) {
    mainChart.remove();
    mainChart = null;
  }
  container.innerHTML = '';

  if (!bars || bars.length === 0) {
    container.innerHTML = '<div class="empty-state">No price candles available for this period.</div>';
    return;
  }

  // Check if LightweightCharts library is loaded
  if (typeof LightweightCharts === 'undefined') {
    renderCanvasCandlestickFallback(container, bars);
    return;
  }

  const chartOptions = {
    layout: {
      background: { color: '#151b26' },
      textColor: '#94a3b8',
      fontSize: 11,
      fontFamily: 'Inter, sans-serif',
    },
    grid: {
      vertLines: { color: '#1a2230' },
      horzLines: { color: '#1a2230' },
    },
    crosshair: {
      mode: LightweightCharts.CrosshairMode.Normal,
    },
    rightPriceScale: {
      borderColor: '#232d3f',
      scaleMargins: { top: 0.1, bottom: 0.2 },
    },
    timeScale: {
      borderColor: '#232d3f',
      timeVisible: true,
      secondsVisible: false,
    },
  };

  mainChart = LightweightCharts.createChart(container, chartOptions);

  candleSeries = mainChart.addCandlestickSeries({
    upColor: '#10b981',
    downColor: '#ef4444',
    borderVisible: false,
    wickUpColor: '#10b981',
    wickDownColor: '#ef4444',
  });

  volumeSeries = mainChart.addHistogramSeries({
    priceFormat: { type: 'volume' },
    priceScaleId: '',
    scaleMargins: { top: 0.8, bottom: 0 },
  });

  // Overlays
  sma20Series = mainChart.addLineSeries({ color: '#3b82f6', lineWidth: 1.5, title: 'SMA 20' });
  sma50Series = mainChart.addLineSeries({ color: '#f59e0b', lineWidth: 1.5, title: 'SMA 50' });
  ema21Series = mainChart.addLineSeries({ color: '#8b5cf6', lineWidth: 1.5, title: 'EMA 21' });
  bbUpperSeries = mainChart.addLineSeries({ color: 'rgba(6, 182, 212, 0.6)', lineWidth: 1, lineStyle: 2, title: 'BB Upper' });
  bbLowerSeries = mainChart.addLineSeries({ color: 'rgba(6, 182, 212, 0.6)', lineWidth: 1, lineStyle: 2, title: 'BB Lower' });

  // Format and set candle data
  const candleData = [];
  const volumeData = [];
  const closes = [];

  for (const b of bars) {
    candleData.push({
      time: b.time,
      open: b.open,
      high: b.high,
      low: b.low,
      close: b.close,
    });
    volumeData.push({
      time: b.time,
      value: b.volume,
      color: b.close >= b.open ? 'rgba(16, 185, 129, 0.35)' : 'rgba(239, 68, 68, 0.35)',
    });
    closes.push(b.close);
  }

  candleSeries.setData(candleData);
  volumeSeries.setData(volumeData);

  // Calculate and set moving averages and Bollinger Bands
  const sma20 = calcClientSMA(closes, 20);
  const sma50 = calcClientSMA(closes, 50);
  const ema21 = calcClientEMA(closes, 21);
  const bb = calcClientBollinger(closes, 20, 2.0);

  const sma20Data = [];
  const sma50Data = [];
  const ema21Data = [];
  const bbUpData = [];
  const bbLowData = [];

  for (let i = 0; i < bars.length; i++) {
    if (sma20[i] > 0) sma20Data.push({ time: bars[i].time, value: sma20[i] });
    if (sma50[i] > 0) sma50Data.push({ time: bars[i].time, value: sma50[i] });
    if (ema21[i] > 0) ema21Data.push({ time: bars[i].time, value: ema21[i] });
    if (bb.upper[i] > 0) {
      bbUpData.push({ time: bars[i].time, value: bb.upper[i] });
      bbLowData.push({ time: bars[i].time, value: bb.lower[i] });
    }
  }

  sma20Series.setData(sma20Data);
  sma50Series.setData(sma50Data);
  ema21Series.setData(ema21Data);
  bbUpperSeries.setData(bbUpData);
  bbLowerSeries.setData(bbLowData);

  mainChart.timeScale().fitContent();

  // Resize handler
  window.addEventListener('resize', () => {
    if (mainChart && container) {
      mainChart.applyOptions({ width: container.clientWidth });
    }
  });

  // Render Subcharts (RSI & MACD)
  renderRSISubchart('rsiChart', bars, closes);
  renderMACDSubchart('macdChart', bars, closes);
}

// Render RSI Subchart
function renderRSISubchart(containerId, bars, closes) {
  const container = document.getElementById(containerId);
  if (!container || typeof LightweightCharts === 'undefined') return;

  if (rsiChart) {
    rsiChart.remove();
    rsiChart = null;
  }
  container.innerHTML = '';

  rsiChart = LightweightCharts.createChart(container, {
    layout: { background: { color: '#151b26' }, textColor: '#94a3b8', fontSize: 10 },
    grid: { vertLines: { color: '#1a2230' }, horzLines: { color: '#1a2230' } },
    rightPriceScale: { borderColor: '#232d3f' },
    timeScale: { visible: false },
  });

  rsiSeries = rsiChart.addLineSeries({ color: '#8b5cf6', lineWidth: 1.5 });
  const rsiValues = calcClientRSI(closes, 14);
  const rsiData = [];

  for (let i = 0; i < bars.length; i++) {
    if (rsiValues[i] > 0) {
      rsiData.push({ time: bars[i].time, value: rsiValues[i] });
    }
  }
  rsiSeries.setData(rsiData);

  // Add 70 and 30 reference levels
  const overboughtLine = rsiChart.addLineSeries({ color: 'rgba(239, 68, 68, 0.4)', lineWidth: 1, lineStyle: 2 });
  const oversoldLine = rsiChart.addLineSeries({ color: 'rgba(16, 185, 129, 0.4)', lineWidth: 1, lineStyle: 2 });
  const obData = bars.map(b => ({ time: b.time, value: 70 }));
  const osData = bars.map(b => ({ time: b.time, value: 30 }));
  overboughtLine.setData(obData);
  oversoldLine.setData(osData);

  if (rsiValues.length > 0) {
    const lastRSI = rsiValues[rsiValues.length - 1];
    const el = document.getElementById('chartRSIVal');
    if (el) el.textContent = lastRSI.toFixed(1);
  }

  rsiChart.timeScale().fitContent();
}

// Render MACD Subchart
function renderMACDSubchart(containerId, bars, closes) {
  const container = document.getElementById(containerId);
  if (!container || typeof LightweightCharts === 'undefined') return;

  if (macdChart) {
    macdChart.remove();
    macdChart = null;
  }
  container.innerHTML = '';

  macdChart = LightweightCharts.createChart(container, {
    layout: { background: { color: '#151b26' }, textColor: '#94a3b8', fontSize: 10 },
    grid: { vertLines: { color: '#1a2230' }, horzLines: { color: '#1a2230' } },
    rightPriceScale: { borderColor: '#232d3f' },
    timeScale: { visible: false },
  });

  const macd = calcClientMACD(closes);
  macdHistSeries = macdChart.addHistogramSeries({});
  macdLineSeries = macdChart.addLineSeries({ color: '#38bdf8', lineWidth: 1.5 });
  macdSignalSeries = macdChart.addLineSeries({ color: '#f59e0b', lineWidth: 1.5 });

  const lineData = [];
  const sigData = [];
  const histData = [];

  for (let i = 0; i < bars.length; i++) {
    if (macd.macd[i] !== 0) {
      lineData.push({ time: bars[i].time, value: macd.macd[i] });
      sigData.push({ time: bars[i].time, value: macd.signal[i] });
      histData.push({
        time: bars[i].time,
        value: macd.hist[i],
        color: macd.hist[i] >= 0 ? '#10b981' : '#ef4444',
      });
    }
  }

  macdLineSeries.setData(lineData);
  macdSignalSeries.setData(sigData);
  macdHistSeries.setData(histData);

  if (macd.macd.length > 0) {
    const lastM = macd.macd[macd.macd.length - 1];
    const el = document.getElementById('chartMACDVal');
    if (el) el.textContent = lastM.toFixed(2);
  }

  macdChart.timeScale().fitContent();
}

// Render Equity Curve Chart for Backtest
function renderBacktestEquityChart(containerId, equityPoints) {
  const container = document.getElementById(containerId);
  if (!container) return;

  if (equityChart) {
    equityChart.remove();
    equityChart = null;
  }
  container.innerHTML = '';

  if (!equityPoints || equityPoints.length === 0) {
    container.innerHTML = '<div class="empty-state">No equity points to plot.</div>';
    return;
  }

  if (typeof LightweightCharts === 'undefined') {
    container.innerHTML = '<div class="empty-state">Lightweight Charts offline. Performance metrics are listed above.</div>';
    return;
  }

  equityChart = LightweightCharts.createChart(container, {
    layout: { background: { color: '#151b26' }, textColor: '#94a3b8', fontSize: 11 },
    grid: { vertLines: { color: '#1a2230' }, horzLines: { color: '#1a2230' } },
    rightPriceScale: { borderColor: '#232d3f' },
    timeScale: { borderColor: '#232d3f', timeVisible: true },
  });

  strategyEquitySeries = equityChart.addLineSeries({
    color: '#10b981',
    lineWidth: 2.5,
    title: 'Strategy Equity (₹)',
  });

  benchmarkEquitySeries = equityChart.addLineSeries({
    color: '#64748b',
    lineWidth: 1.5,
    lineStyle: 1,
    title: 'NIFTY 50 Benchmark (₹)',
  });

  const stratData = equityPoints.map(p => ({ time: p.date, value: p.equity }));
  const benchData = equityPoints.map(p => ({ time: p.date, value: p.benchmark }));

  strategyEquitySeries.setData(stratData);
  benchmarkEquitySeries.setData(benchData);

  equityChart.timeScale().fitContent();
}

// Offline canvas fallback renderer if CDN is blocked
function renderCanvasCandlestickFallback(container, bars) {
  const canvas = document.createElement('canvas');
  canvas.width = container.clientWidth || 800;
  canvas.height = 400;
  canvas.style.width = '100%';
  canvas.style.height = '100%';
  container.appendChild(canvas);

  const ctx = canvas.getContext('2d');
  ctx.fillStyle = '#151b26';
  ctx.fillRect(0, 0, canvas.width, canvas.height);

  let minP = Infinity, maxP = -Infinity;
  bars.forEach(b => {
    if (b.low < minP) minP = b.low;
    if (b.high > maxP) maxP = b.high;
  });

  const pad = 30;
  const h = canvas.height - pad * 2;
  const w = canvas.width - pad * 2;
  const step = w / bars.length;

  const yCoord = (p) => canvas.height - pad - ((p - minP) / (maxP - minP)) * h;

  bars.forEach((b, i) => {
    const x = pad + i * step + step / 2;
    const isUp = b.close >= b.open;
    ctx.strokeStyle = isUp ? '#10b981' : '#ef4444';
    ctx.fillStyle = isUp ? '#10b981' : '#ef4444';

    // Wick
    ctx.beginPath();
    ctx.moveTo(x, yCoord(b.high));
    ctx.lineTo(x, yCoord(b.low));
    ctx.stroke();

    // Body
    const top = yCoord(Math.max(b.open, b.close));
    const bot = yCoord(Math.min(b.open, b.close));
    const bodyH = Math.max(2, bot - top);
    ctx.fillRect(x - step * 0.35, top, Math.max(2, step * 0.7), bodyH);
  });
}

// Client-side helper math functions for overlays
function calcClientSMA(values, period) {
  const res = new Array(values.length).fill(0);
  let sum = 0;
  for (let i = 0; i < values.length; i++) {
    sum += values[i];
    if (i >= period) sum -= values[i - period];
    if (i >= period - 1) res[i] = Math.round((sum / period) * 100) / 100;
  }
  return res;
}

function calcClientEMA(values, period) {
  const res = new Array(values.length).fill(0);
  if (values.length < period) return res;
  const k = 2 / (period + 1);
  let sum = 0;
  for (let i = 0; i < period; i++) sum += values[i];
  let prevEMA = sum / period;
  res[period - 1] = prevEMA;
  for (let i = period; i < values.length; i++) {
    prevEMA = (values[i] - prevEMA) * k + prevEMA;
    res[i] = Math.round(prevEMA * 100) / 100;
  }
  return res;
}

function calcClientBollinger(values, period, numStdDev) {
  const upper = new Array(values.length).fill(0);
  const lower = new Array(values.length).fill(0);
  const sma = calcClientSMA(values, period);

  for (let i = period - 1; i < values.length; i++) {
    const m = sma[i];
    let sumSq = 0;
    for (let j = i - period + 1; j <= i; j++) {
      sumSq += (values[j] - m) * (values[j] - m);
    }
    const std = Math.sqrt(sumSq / period);
    upper[i] = Math.round((m + numStdDev * std) * 100) / 100;
    lower[i] = Math.round((m - numStdDev * std) * 100) / 100;
  }
  return { upper, lower };
}

function calcClientRSI(closes, period = 14) {
  const res = new Array(closes.length).fill(0);
  if (closes.length <= period) return res;

  let gains = 0, losses = 0;
  for (let i = 1; i <= period; i++) {
    const d = closes[i] - closes[i - 1];
    if (d >= 0) gains += d; else losses += -d;
  }
  let avgGain = gains / period;
  let avgLoss = losses / period;
  res[period] = avgLoss === 0 ? 100 : 100 - (100 / (1 + avgGain / avgLoss));

  for (let i = period + 1; i < closes.length; i++) {
    const d = closes[i] - closes[i - 1];
    const g = d >= 0 ? d : 0;
    const l = d < 0 ? -d : 0;
    avgGain = (avgGain * (period - 1) + g) / period;
    avgLoss = (avgLoss * (period - 1) + l) / period;
    res[i] = avgLoss === 0 ? 100 : Math.round((100 - (100 / (1 + avgGain / avgLoss))) * 10) / 10;
  }
  return res;
}

function calcClientMACD(closes) {
  const fast = calcClientEMA(closes, 12);
  const slow = calcClientEMA(closes, 26);
  const macd = new Array(closes.length).fill(0);
  for (let i = 25; i < closes.length; i++) {
    macd[i] = Math.round((fast[i] - slow[i]) * 100) / 100;
  }
  const signal = calcClientEMA(macd.slice(25), 9);
  const fullSignal = new Array(closes.length).fill(0);
  const hist = new Array(closes.length).fill(0);
  for (let i = 0; i < signal.length; i++) {
    const idx = 25 + i;
    fullSignal[idx] = signal[i];
    hist[idx] = Math.round((macd[idx] - signal[i]) * 100) / 100;
  }
  return { macd, signal: fullSignal, hist };
}
