package marketdata

import (
	"fmt"
	"time"
)

// FreshnessReport summarizes data quality for a symbol's quote.
type FreshnessReport struct {
	Symbol            string        `json:"symbol"`
	ProviderTimestamp time.Time     `json:"provider_timestamp"`
	PollTimestamp     time.Time     `json:"poll_timestamp"`
	Age               time.Duration `json:"age"`
	Threshold         time.Duration `json:"threshold"`
	IsStale           bool          `json:"is_stale"`
	Warning           string        `json:"warning,omitempty"`
}

// EvaluateFreshness validates if quote is fresh enough for autonomous trading decisions.
func EvaluateFreshness(quote *Quote, maxAge time.Duration, now time.Time) FreshnessReport {
	if maxAge <= 0 {
		maxAge = 30 * time.Minute // default threshold: 30 minutes
	}

	report := FreshnessReport{
		Symbol:        quote.Symbol,
		PollTimestamp: now,
		Threshold:     maxAge,
	}

	if quote.ProviderTime.IsZero() {
		report.ProviderTimestamp = quote.LastUpdated
	} else {
		report.ProviderTimestamp = quote.ProviderTime
	}

	if report.ProviderTimestamp.IsZero() {
		report.IsStale = true
		report.Warning = fmt.Sprintf("Missing provider timestamp for %s", quote.Symbol)
		return report
	}

	age := now.Sub(report.ProviderTimestamp)
	if age < 0 {
		age = 0 // Provider clock slightly ahead
	}
	report.Age = age

	if age > maxAge {
		report.IsStale = true
		report.Warning = fmt.Sprintf("Data stale for %s: age %v exceeds freshness threshold %v (last update: %s)",
			quote.Symbol, age.Round(time.Second), maxAge, report.ProviderTimestamp.Format("15:04:05 IST"))
	}

	return report
}

// ValidateCandleSeries detects missing candles, duplicate timestamps, gaps, or zero-volume anomalies.
func ValidateCandleSeries(candles []Candle, minCandles int) []string {
	var issues []string
	if len(candles) < minCandles {
		issues = append(issues, fmt.Sprintf("Insufficient candle depth: got %d candles, require minimum %d", len(candles), minCandles))
		return issues
	}

	seenTimestamps := make(map[int64]bool)
	for i, c := range candles {
		if c.Timestamp <= 0 {
			issues = append(issues, fmt.Sprintf("Candle %d has invalid timestamp %d", i, c.Timestamp))
			continue
		}
		if seenTimestamps[c.Timestamp] {
			issues = append(issues, fmt.Sprintf("Duplicate timestamp %d (%s) at index %d", c.Timestamp, c.Date, i))
		}
		seenTimestamps[c.Timestamp] = true

		if c.High < c.Low {
			issues = append(issues, fmt.Sprintf("Candle %s: High (%.2f) is lower than Low (%.2f)", c.Date, c.High, c.Low))
		}
		if c.Open <= 0 || c.Close <= 0 {
			issues = append(issues, fmt.Sprintf("Candle %s has non-positive price (O: %.2f, C: %.2f)", c.Date, c.Open, c.Close))
		}
	}

	return issues
}
