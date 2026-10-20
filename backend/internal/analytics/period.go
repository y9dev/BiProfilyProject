package analytics

import (
	"time"

	"bigroup-backend/internal/httpapi/dto"
)

// Granularities of PeriodInfo.Granularity.
const (
	GranDay   = "day"
	GranWeek  = "week"
	GranMonth = "month"
)

// Span limits for granularity selection (inclusive day counts, plan sec. 4 "Buckets").
const (
	maxDailySpan  = 92
	maxWeeklySpan = 400
)

// ParseDate parses a strict YYYY-MM-DD calendar date.
func ParseDate(s string) (Date, bool) {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil || t.Format(time.DateOnly) != s {
		return Date{}, false
	}
	return dateFromUTC(t), true
}

// ValidatePeriod applies the section 3 period rules to raw query values and returns
// the syntactically valid PeriodQuery or the 422 field errors. preset "" means 90d.
// allowCustom=false (dashboard) rejects period=custom.
func ValidatePeriod(today Date, preset, from, to string, allowCustom bool) (PeriodQuery, []dto.FieldError) {
	if preset == "" {
		preset = Period90d
	}
	switch preset {
	case Period7d, Period30d, Period90d, Period12m, PeriodAll:
		if from != "" || to != "" {
			return PeriodQuery{}, []dto.FieldError{{Path: "from", Code: "unexpected"}}
		}
		return PeriodQuery{Preset: preset}, nil
	case PeriodCustom:
		if !allowCustom {
			return PeriodQuery{}, []dto.FieldError{{Path: "period", Code: "invalid"}}
		}
	default:
		return PeriodQuery{}, []dto.FieldError{{Path: "period", Code: "invalid"}}
	}
	var errs []dto.FieldError
	var f, t Date
	for _, p := range []struct {
		path, raw string
		dst       *Date
	}{{"from", from, &f}, {"to", to, &t}} {
		if p.raw == "" {
			errs = append(errs, dto.FieldError{Path: p.path, Code: "required"})
			continue
		}
		d, ok := ParseDate(p.raw)
		if !ok {
			errs = append(errs, dto.FieldError{Path: p.path, Code: "invalid_date"})
			continue
		}
		*p.dst = d
	}
	if len(errs) > 0 {
		return PeriodQuery{}, errs
	}
	q := PeriodQuery{Preset: PeriodCustom, From: f, To: t}
	if e := checkCustom(today, q); e != nil {
		return PeriodQuery{}, []dto.FieldError{*e}
	}
	return q, nil
}

// checkCustom enforces from <= to <= today and the MaxSpanDays cap.
func checkCustom(today Date, q PeriodQuery) *dto.FieldError {
	switch {
	case q.From.After(q.To):
		return &dto.FieldError{Path: "from", Code: "after_to"}
	case q.To.After(today):
		return &dto.FieldError{Path: "to", Code: "in_future"}
	case q.From.DaysUntil(q.To)+1 > MaxSpanDays:
		return &dto.FieldError{Path: "to", Code: "span_too_large"}
	}
	return nil
}

// checkPeriod re-validates a PeriodQuery handed to the repo (defence in depth).
func checkPeriod(today Date, q PeriodQuery, allowCustom bool) error {
	switch q.Preset {
	case "", Period7d, Period30d, Period90d, Period12m, PeriodAll:
		return nil
	case PeriodCustom:
		if !allowCustom {
			break
		}
		if e := checkCustom(today, q); e != nil {
			return &ValidationError{Errors: []dto.FieldError{*e}}
		}
		return nil
	}
	return &ValidationError{Errors: []dto.FieldError{{Path: "period", Code: "invalid"}}}
}

// ResolveWindow turns a validated PeriodQuery into an inclusive [from, to] window.
// earliest is the earliest population measurement day (PeriodAll only); nil = no data.
func ResolveWindow(today Date, q PeriodQuery, earliest *Date) (Date, Date) {
	switch q.Preset {
	case Period7d:
		return today.AddDays(-6), today
	case Period30d:
		return today.AddDays(-29), today
	case "", Period90d:
		return today.AddDays(-89), today
	case Period12m:
		return yearBefore(today).AddDays(1), today
	case PeriodAll:
		if earliest == nil || earliest.After(today) {
			return today, today
		}
		from := *earliest
		// Keep the window inside the MaxSpanDays cap.
		if from.DaysUntil(today)+1 > MaxSpanDays {
			from = today.AddDays(-(MaxSpanDays - 1))
		}
		return from, today
	}
	return q.From, q.To
}

// yearBefore is the same calendar day one year earlier; Feb 29 clamps to Feb 28.
func yearBefore(d Date) Date {
	y := Date{Year: d.Year - 1, Month: d.Month, Day: d.Day}
	if y.Month == time.February && y.Day == 29 && dateFromUTC(y.utc()) != y {
		y.Day = 28
	}
	return y
}

// Granularity picks the bucket size for an inclusive window.
func Granularity(from, to Date) string {
	span := from.DaysUntil(to) + 1
	switch {
	case span <= maxDailySpan:
		return GranDay
	case span <= maxWeeklySpan:
		return GranWeek
	}
	return GranMonth
}

// BucketEnds lists the bucket end dates of [from, to] in ascending order: every day;
// every ISO week's Sunday; or every month's last day. The last bucket always ends at to.
func BucketEnds(from, to Date, gran string) []Date {
	var out []Date
	for d := from; !d.After(to); d = d.AddDays(1) {
		var end bool
		switch gran {
		case GranDay:
			end = true
		case GranWeek:
			end = d.utc().Weekday() == time.Sunday
		default:
			end = d.AddDays(1).Day == 1
		}
		if end || d == to {
			out = append(out, d)
		}
	}
	return out
}

// PeriodInfo is the wire form of a resolved window.
func PeriodInfo(from, to Date) dto.PeriodInfo {
	return dto.PeriodInfo{From: from.String(), To: to.String(), Granularity: Granularity(from, to), Timezone: TZ}
}

func dateStrings(ds []Date) []string {
	out := make([]string, len(ds))
	for i, d := range ds {
		out[i] = d.String()
	}
	return out
}
