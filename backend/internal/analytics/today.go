package analytics

import (
	"time"

	// Embedded tz database: Asia/Almaty must not depend on the host OS (plan sec. 4).
	_ "time/tzdata"
)

var almaty = mustLoadTZ()

func mustLoadTZ() *time.Location {
	loc, err := time.LoadLocation(TZ)
	if err != nil {
		panic("analytics: load " + TZ + ": " + err.Error())
	}
	return loc
}

// Location returns the business timezone (Asia/Almaty).
func Location() *time.Location { return almaty }

// Today is the Asia/Almaty calendar day of now. Handlers call it once per request.
func Today(now time.Time) Date { return DateOf(now) }

// DateOf is the Asia/Almaty calendar day of the instant t.
func DateOf(t time.Time) Date {
	l := t.In(almaty)
	return Date{Year: l.Year(), Month: l.Month(), Day: l.Day()}
}

// utc is the date as midnight UTC; used only for calendar arithmetic.
func (d Date) utc() time.Time { return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, time.UTC) }

func dateFromUTC(t time.Time) Date { return Date{Year: t.Year(), Month: t.Month(), Day: t.Day()} }

// AddDays returns d shifted by n calendar days.
func (d Date) AddDays(n int) Date { return dateFromUTC(d.utc().AddDate(0, 0, n)) }

// Before reports whether d is strictly earlier than o.
func (d Date) Before(o Date) bool { return d.utc().Before(o.utc()) }

// After reports whether d is strictly later than o.
func (d Date) After(o Date) bool { return d.utc().After(o.utc()) }

// DaysUntil is the number of calendar days from d to o (o - d).
func (d Date) DaysUntil(o Date) int { return int(o.utc().Sub(d.utc()).Hours() / 24) }

// IsZero reports whether d is the zero Date.
func (d Date) IsZero() bool { return d == Date{} }
