package yahoo

import (
	"bearabull/internal/marketdata"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

// Client handles interaction with Yahoo Finance API.
type Client struct {
	httpClient *http.Client
	baseURL    string
	location   *time.Location
	userAgent  string
	rateMu     sync.Mutex
	lastCall   time.Time
	minDelay   time.Duration
}

// YahooChartResponse models Yahoo Finance Chart API response.
type YahooChartResponse struct {
	Chart struct {
		Result []struct {
			Meta struct {
				Currency           string  `json:"currency"`
				Symbol             string  `json:"symbol"`
				ExchangeName       string  `json:"exchangeName"`
				RegularMarketPrice float64 `json:"regularMarketPrice"`
				ChartPreviousClose float64 `json:"chartPreviousClose"`
				PreviousClose      float64 `json:"previousClose"`
				RegularMarketTime  int64   `json:"regularMarketTime"`
				Timezone           string  `json:"timezone"`
				ExchangeTimezone   string  `json:"exchangeTimezoneName"`
			} `json:"meta"`
			Timestamp  []int64 `json:"timestamp"`
			Indicators struct {
				Quote []struct {
					Open   []*float64 `json:"open"`
					High   []*float64 `json:"high"`
					Low    []*float64 `json:"low"`
					Close  []*float64 `json:"close"`
					Volume []*int64   `json:"volume"`
				} `json:"quote"`
				AdjClose []struct {
					AdjClose []*float64 `json:"adjclose"`
				} `json:"adjclose"`
			} `json:"indicators"`
		} `json:"result"`
		Error *struct {
			Code        string `json:"code"`
			Description string `json:"description"`
		} `json:"error"`
	} `json:"chart"`
}

// NewClient creates a new Yahoo Finance client with default settings.
func NewClient(timeout time.Duration, tzName string) *Client {
	loc, err := time.LoadLocation(tzName)
	if err != nil {
		loc = time.FixedZone("IST", 19800) // Asia/Kolkata fallback
	}

	return &Client{
		httpClient: &http.Client{
			Timeout: timeout,
		},
		baseURL:   "https://query1.finance.yahoo.com/v8/finance/chart",
		location:  loc,
		userAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
		minDelay:  300 * time.Millisecond,
	}
}

func (c *Client) Name() string {
	return "Yahoo Finance"
}

// throttle ensures requests are spaced apart to avoid hitting rate limits.
func (c *Client) throttle() {
	c.rateMu.Lock()
	defer c.rateMu.Unlock()

	elapsed := time.Since(c.lastCall)
	if elapsed < c.minDelay {
		time.Sleep(c.minDelay - elapsed)
	}
	c.lastCall = time.Now()
}

// GetHistoricalPrices fetches daily OHLCV candles from Yahoo Finance.
func (c *Client) GetHistoricalPrices(ctx context.Context, symbol string, rangeStr string, interval string) ([]marketdata.Candle, error) {
	if rangeStr == "" {
		rangeStr = "2y"
	}
	if interval == "" {
		interval = "1d"
	}

	url := fmt.Sprintf("%s/%s?interval=%s&range=%s&includeAdjustedClose=true", c.baseURL, symbol, interval, rangeStr)

	var body []byte
	var err error
	maxRetries := 3

	for attempt := 0; attempt < maxRetries; attempt++ {
		c.throttle()

		req, reqErr := http.NewRequestWithContext(ctx, "GET", url, nil)
		if reqErr != nil {
			return nil, reqErr
		}
		req.Header.Set("User-Agent", c.userAgent)
		req.Header.Set("Accept", "application/json")

		resp, doErr := c.httpClient.Do(req)
		if doErr != nil {
			err = doErr
			time.Sleep(time.Duration(1<<attempt) * 500 * time.Millisecond)
			continue
		}

		body, err = io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			break
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			time.Sleep(time.Duration(attempt+1) * 1500 * time.Millisecond)
			err = fmt.Errorf("rate limited by Yahoo Finance (HTTP 429)")
			continue
		}

		err = fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
		time.Sleep(500 * time.Millisecond)
	}

	if err != nil && len(body) == 0 {
		return nil, fmt.Errorf("failed fetching data for %s: %w", symbol, err)
	}

	var parsed YahooChartResponse
	if unmarshalErr := json.Unmarshal(body, &parsed); unmarshalErr != nil {
		return nil, fmt.Errorf("failed parsing Yahoo response for %s: %w", symbol, unmarshalErr)
	}

	if parsed.Chart.Error != nil {
		return nil, fmt.Errorf("yahoo API error for %s: %s", symbol, parsed.Chart.Error.Description)
	}

	if len(parsed.Chart.Result) == 0 {
		return nil, fmt.Errorf("empty chart result for %s", symbol)
	}

	res := parsed.Chart.Result[0]
	timestamps := res.Timestamp
	if len(timestamps) == 0 || len(res.Indicators.Quote) == 0 {
		return nil, fmt.Errorf("no candle data returned for %s", symbol)
	}

	quote := res.Indicators.Quote[0]
	var adjCloseList []*float64
	if len(res.Indicators.AdjClose) > 0 {
		adjCloseList = res.Indicators.AdjClose[0].AdjClose
	}

	var candles []marketdata.Candle
	for i, ts := range timestamps {
		if ts <= 0 {
			continue
		}

		// Check for nil prices
		if i >= len(quote.Open) || quote.Open[i] == nil ||
			i >= len(quote.High) || quote.High[i] == nil ||
			i >= len(quote.Low) || quote.Low[i] == nil ||
			i >= len(quote.Close) || quote.Close[i] == nil {
			continue
		}

		openVal := *quote.Open[i]
		highVal := *quote.High[i]
		lowVal := *quote.Low[i]
		closeVal := *quote.Close[i]

		if math.IsNaN(openVal) || math.IsNaN(highVal) || math.IsNaN(lowVal) || math.IsNaN(closeVal) ||
			openVal <= 0 || highVal <= 0 || lowVal <= 0 || closeVal <= 0 {
			continue
		}

		volVal := int64(0)
		if i < len(quote.Volume) && quote.Volume[i] != nil {
			volVal = *quote.Volume[i]
		}

		adjCloseVal := closeVal
		if i < len(adjCloseList) && adjCloseList[i] != nil && !math.IsNaN(*adjCloseList[i]) && *adjCloseList[i] > 0 {
			adjCloseVal = *adjCloseList[i]
		}

		candleTime := time.Unix(ts, 0).In(c.location)
		candles = append(candles, marketdata.Candle{
			Symbol:    symbol,
			Date:      candleTime.Format("2006-01-02"),
			DateTime:  candleTime.Format("2006-01-02 15:04:05"),
			Interval:  interval,
			Timestamp: ts,
			Open:      math.Round(openVal*100) / 100,
			High:      math.Round(highVal*100) / 100,
			Low:       math.Round(lowVal*100) / 100,
			Close:     math.Round(closeVal*100) / 100,
			AdjClose:  math.Round(adjCloseVal*100) / 100,
			Volume:    volVal,
		})
	}

	return candles, nil
}

// GetQuote returns the latest quote summary for a symbol with explicit provider timestamp.
func (c *Client) GetQuote(ctx context.Context, symbol string) (*marketdata.Quote, error) {
	// Try fetching 15m intraday chart to get fresh metadata and regularMarketTime
	url := fmt.Sprintf("%s/%s?interval=15m&range=1d&includeAdjustedClose=true", c.baseURL, symbol)
	var body []byte

	for attempt := 0; attempt < 2; attempt++ {
		c.throttle()
		req, reqErr := http.NewRequestWithContext(ctx, "GET", url, nil)
		if reqErr != nil {
			break
		}
		req.Header.Set("User-Agent", c.userAgent)
		req.Header.Set("Accept", "application/json")
		resp, doErr := c.httpClient.Do(req)
		if doErr == nil {
			body, _ = io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				break
			}
		}
	}

	var parsed YahooChartResponse
	if len(body) > 0 && json.Unmarshal(body, &parsed) == nil && len(parsed.Chart.Result) > 0 {
		res := parsed.Chart.Result[0]
		meta := res.Meta
		providerTime := time.Now().In(c.location)
		if meta.RegularMarketTime > 0 {
			providerTime = time.Unix(meta.RegularMarketTime, 0).In(c.location)
		}
		price := meta.RegularMarketPrice
		prevClose := meta.PreviousClose
		if prevClose == 0 {
			prevClose = meta.ChartPreviousClose
		}

		if price <= 0 && len(res.Indicators.Quote) > 0 && len(res.Indicators.Quote[0].Close) > 0 {
			closes := res.Indicators.Quote[0].Close
			for idx := len(closes) - 1; idx >= 0; idx-- {
				if closes[idx] != nil && *closes[idx] > 0 {
					price = *closes[idx]
					break
				}
			}
		}

		if price > 0 {
			change := price - prevClose
			changePct := 0.0
			if prevClose > 0 {
				changePct = (change / prevClose) * 100
			}

			tradeDate := providerTime.Format("2006-01-02")
			currency := meta.Currency
			if currency == "" {
				currency = "INR"
			}
			exchange := meta.ExchangeName
			if exchange == "" {
				exchange = "NSE"
			}

			return &marketdata.Quote{
				Symbol:        symbol,
				Price:         math.Round(price*100) / 100,
				PreviousClose: math.Round(prevClose*100) / 100,
				Change:        math.Round(change*100) / 100,
				ChangePercent: math.Round(changePct*100) / 100,
				TradeDate:     tradeDate,
				LastUpdated:   time.Now().In(c.location),
				ProviderTime:  providerTime,
				Currency:      currency,
				Exchange:      exchange,
			}, nil
		}
	}

	// Fallback to daily candle series
	candles, err := c.GetHistoricalPrices(ctx, symbol, "5d", "1d")
	if err != nil {
		return nil, err
	}
	if len(candles) == 0 {
		return nil, fmt.Errorf("no quote data found for %s", symbol)
	}

	last := candles[len(candles)-1]
	prevClose := last.Open
	if len(candles) >= 2 {
		prevClose = candles[len(candles)-2].Close
	}

	change := last.Close - prevClose
	changePct := 0.0
	if prevClose > 0 {
		changePct = (change / prevClose) * 100
	}

	providerTime := time.Unix(last.Timestamp, 0).In(c.location)
	return &marketdata.Quote{
		Symbol:        symbol,
		Price:         last.Close,
		PreviousClose: prevClose,
		Change:        math.Round(change*100) / 100,
		ChangePercent: math.Round(changePct*100) / 100,
		Volume:        last.Volume,
		High:          last.High,
		Low:           last.Low,
		Open:          last.Open,
		TradeDate:     last.Date,
		LastUpdated:   time.Now().In(c.location),
		ProviderTime:  providerTime,
		Currency:      "INR",
		Exchange:      "NSE",
	}, nil
}

// GetLastUpdated returns latest timestamp.
func (c *Client) GetLastUpdated(ctx context.Context, symbol string) (time.Time, error) {
	q, err := c.GetQuote(ctx, symbol)
	if err != nil {
		return time.Time{}, err
	}
	return q.LastUpdated, nil
}

// GenerateDeterministicHistoricalData creates realistic synthetic daily candles
// for offline testing, demos, or when Yahoo Finance access is restricted.
func GenerateDeterministicHistoricalData(symbol string, basePrice float64, days int, loc *time.Location) []marketdata.Candle {
	if basePrice <= 0 {
		basePrice = 1500.0
	}
	if loc == nil {
		loc = time.FixedZone("IST", 19800)
	}

	// Use deterministic seed based on symbol name
	var seed int64
	for _, b := range symbol {
		seed = seed*31 + int64(b)
	}
	rng := rand.New(rand.NewSource(seed))

	now := time.Now().In(loc)
	currentPrice := basePrice

	var allCandles []marketdata.Candle
	for i := days; i >= 0; i-- {
		t := now.AddDate(0, 0, -i)
		// Skip weekends (Saturday=6, Sunday=0)
		if t.Weekday() == time.Saturday || t.Weekday() == time.Sunday {
			continue
		}

		// Geometric Brownian motion drift + shock
		drift := 0.0004
		volatility := 0.015
		shock := rng.NormFloat64() * volatility
		dailyReturn := drift + shock
		openPrice := currentPrice * (1.0 + (rng.NormFloat64() * 0.004))
		closePrice := currentPrice * (1.0 + dailyReturn)
		highPrice := math.Max(openPrice, closePrice) * (1.0 + math.Abs(rng.NormFloat64()*0.008))
		lowPrice := math.Min(openPrice, closePrice) * (1.0 - math.Abs(rng.NormFloat64()*0.008))
		currentPrice = closePrice

		volBase := 1000000.0 + rng.Float64()*1500000.0
		if math.Abs(dailyReturn) > 0.02 {
			volBase *= 2.0 // Surge volume on big moves
		}

		allCandles = append(allCandles, marketdata.Candle{
			Symbol:    symbol,
			Date:      t.Format("2006-01-02"),
			Timestamp: t.Unix(),
			Open:      math.Round(openPrice*100) / 100,
			High:      math.Round(highPrice*100) / 100,
			Low:       math.Round(lowPrice*100) / 100,
			Close:     math.Round(closePrice*100) / 100,
			AdjClose:  math.Round(closePrice*100) / 100,
			Volume:    int64(volBase),
		})
	}

	log.Printf("[DEMO/FALLBACK] Generated %d deterministic candles for %s", len(allCandles), symbol)
	return allCandles
}
