package autotrader

import (
	"time"
)

// NSEHolidayCalendar provides NSE India equity market trading schedule and holiday checks.
type NSEHolidayCalendar struct {
	loc      *time.Location
	holidays map[string]string // YYYY-MM-DD -> Holiday Name
}

// NewNSECalendar creates a new NSE trading calendar.
func NewNSECalendar() *NSEHolidayCalendar {
	loc, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		loc = time.FixedZone("IST", 19800)
	}

	// Verifiable NSE equity holidays (major standard market holidays)
	holidays := map[string]string{
		"2026-01-26": "Republic Day",
		"2026-03-03": "Holi",
		"2026-03-20": "Id-Ul-Fitr",
		"2026-04-03": "Good Friday",
		"2026-04-14": "Dr. Baba Saheb Ambedkar Jayanti",
		"2026-05-01": "Maharashtra Day",
		"2026-05-27": "Bakri Id",
		"2026-08-15": "Independence Day",
		"2026-10-02": "Mahatma Gandhi Jayanti",
		"2026-10-20": "Dussehra",
		"2026-11-09": "Diwali Laxmi Pujan",
		"2026-11-10": "Diwali Balipratipada",
		"2026-11-24": "Gurunanak Jayanti",
		"2026-12-25": "Christmas",
	}

	return &NSEHolidayCalendar{
		loc:      loc,
		holidays: holidays,
	}
}

// Location returns the market location (Asia/Kolkata).
func (c *NSEHolidayCalendar) Location() *time.Location {
	return c.loc
}

// IsMarketDay checks if the date is an eligible Monday-Friday trading day (not a weekend or holiday).
func (c *NSEHolidayCalendar) IsMarketDay(t time.Time) (bool, string) {
	ist := t.In(c.loc)
	weekday := ist.Weekday()

	if weekday == time.Saturday || weekday == time.Sunday {
		return false, "Weekend (Market Closed)"
	}

	dateStr := ist.Format("2006-01-02")
	if holidayName, ok := c.holidays[dateStr]; ok {
		return false, "NSE Holiday: " + holidayName
	}

	return true, "Trading Day"
}

// IsMarketSessionActive checks if the current time is within the NSE regular trading session (09:15–15:30 IST).
func (c *NSEHolidayCalendar) IsMarketSessionActive(t time.Time) (bool, string) {
	isDay, reason := c.IsMarketDay(t)
	if !isDay {
		return false, reason
	}

	ist := t.In(c.loc)
	hour, min, _ := ist.Clock()
	minuteOfDay := hour*60 + min

	// 09:15 IST = 9*60 + 15 = 555
	// 15:30 IST = 15*60 + 30 = 930
	sessionStart := 9*60 + 15
	sessionEnd := 15*60 + 30

	if minuteOfDay < sessionStart {
		return false, "Pre-Market / Before Session (Opens 09:15 IST)"
	}
	if minuteOfDay > sessionEnd {
		return false, "Post-Market / Session Closed (Closed 15:30 IST)"
	}

	return true, "Regular Trading Session Active"
}

// Next15MinBoundary calculates the next scheduled 15-minute polling interval boundary.
func (c *NSEHolidayCalendar) Next15MinBoundary(t time.Time) time.Time {
	ist := t.In(c.loc)
	minute := ist.Minute()
	nextMinute := ((minute / 15) + 1) * 15

	next := time.Date(ist.Year(), ist.Month(), ist.Day(), ist.Hour(), 0, 0, 0, c.loc)
	if nextMinute == 60 {
		next = next.Add(time.Hour)
	} else {
		next = next.Add(time.Duration(nextMinute) * time.Minute)
	}
	return next
}
