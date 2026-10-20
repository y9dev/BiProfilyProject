package analytics

import (
	"reflect"
	"testing"
	"time"

	"bigroup-backend/internal/httpapi/dto"
)

func d(s string) Date {
	v, ok := ParseDate(s)
	if !ok {
		panic("bad date " + s)
	}
	return v
}

func TestAlmatyOffset(t *testing.T) {
	_, off := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).In(Location()).Zone()
	if off != 5*3600 {
		t.Fatalf("Asia/Almaty offset = %ds, want +5h (stale tzdata?)", off)
	}
}

func TestTodayAlmatyBoundary(t *testing.T) {
	if got := Today(time.Date(2026, 10, 20, 19, 30, 0, 0, time.UTC)); got != d("2026-10-21") {
		t.Fatalf("19:30Z = %s, want next Almaty day 2026-10-21", got)
	}
	if got := Today(time.Date(2026, 10, 20, 18, 59, 59, 0, time.UTC)); got != d("2026-10-20") {
		t.Fatalf("18:59:59Z = %s, want 2026-10-20", got)
	}
}

func TestParseDate(t *testing.T) {
	for _, s := range []string{"2026-02-30", "2026-1-01", "20261001", "", "2026-10-20T00:00:00Z"} {
		if _, ok := ParseDate(s); ok {
			t.Errorf("ParseDate(%q) accepted", s)
		}
	}
	if _, ok := ParseDate("2028-02-29"); !ok {
		t.Error("leap day rejected")
	}
}

func TestResolveWindow(t *testing.T) {
	today := d("2026-10-20")
	early := d("2024-03-05")
	cases := []struct {
		name     string
		q        PeriodQuery
		earliest *Date
		from, to string
	}{
		{"7d", PeriodQuery{Preset: Period7d}, nil, "2026-10-14", "2026-10-20"},
		{"30d", PeriodQuery{Preset: Period30d}, nil, "2026-09-21", "2026-10-20"},
		{"90d", PeriodQuery{Preset: Period90d}, nil, "2026-07-23", "2026-10-20"},
		{"default", PeriodQuery{}, nil, "2026-07-23", "2026-10-20"},
		{"12m", PeriodQuery{Preset: Period12m}, nil, "2025-10-21", "2026-10-20"},
		{"all with data", PeriodQuery{Preset: PeriodAll}, &early, "2024-03-05", "2026-10-20"},
		{"all no data", PeriodQuery{Preset: PeriodAll}, nil, "2026-10-20", "2026-10-20"},
		{"custom one day", PeriodQuery{Preset: PeriodCustom, From: d("2026-05-01"), To: d("2026-05-01")}, nil, "2026-05-01", "2026-05-01"},
	}
	for _, c := range cases {
		f, to := ResolveWindow(today, c.q, c.earliest)
		if f.String() != c.from || to.String() != c.to {
			t.Errorf("%s: got %s..%s, want %s..%s", c.name, f, to, c.from, c.to)
		}
	}
	// 12m on Feb 29: the year-earlier day clamps to Feb 28, plus one day.
	f, _ := ResolveWindow(d("2028-02-29"), PeriodQuery{Preset: Period12m}, nil)
	if f.String() != "2027-03-01" {
		t.Errorf("12m from Feb 29 = %s, want 2027-03-01", f)
	}
}

func TestValidatePeriod(t *testing.T) {
	today := d("2026-10-20")
	fe := func(p, c string) []dto.FieldError { return []dto.FieldError{{Path: p, Code: c}} }
	cases := []struct {
		name             string
		preset, from, to string
		allowCustom      bool
		want             []dto.FieldError
	}{
		{"unknown period", "5y", "", "", true, fe("period", "invalid")},
		{"custom on dashboard", "custom", "2026-01-01", "2026-01-02", false, fe("period", "invalid")},
		{"custom without from", "custom", "", "2026-01-02", true, fe("from", "required")},
		{"custom without to", "custom", "2026-01-01", "", true, fe("to", "required")},
		{"from without custom", "30d", "2026-01-01", "", true, fe("from", "unexpected")},
		{"to without custom", "", "", "2026-01-01", true, fe("from", "unexpected")},
		{"malformed from", "custom", "2026-13-01", "2026-01-02", true, fe("from", "invalid_date")},
		{"malformed to", "custom", "2026-01-01", "x", true, fe("to", "invalid_date")},
		{"from after to", "custom", "2026-02-01", "2026-01-01", true, fe("from", "after_to")},
		{"to in future", "custom", "2026-10-01", "2026-10-21", true, fe("to", "in_future")},
		{"span too large", "custom", "2016-10-12", "2026-10-20", true, fe("to", "span_too_large")},
		{"span at cap", "custom", "2016-10-13", "2026-10-20", true, nil},
		{"from == to", "custom", "2026-10-20", "2026-10-20", true, nil},
		{"preset ok", "12m", "", "", false, nil},
	}
	for _, c := range cases {
		_, got := ValidatePeriod(today, c.preset, c.from, c.to, c.allowCustom)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
	// Span at the cap is exactly MaxSpanDays inclusive.
	if n := d("2016-10-13").DaysUntil(today) + 1; n != MaxSpanDays {
		t.Fatalf("cap fixture span = %d", n)
	}
}

func TestGranularity(t *testing.T) {
	from := d("2026-01-01")
	cases := []struct {
		span int
		want string
	}{{1, GranDay}, {92, GranDay}, {93, GranWeek}, {400, GranWeek}, {401, GranMonth}, {MaxSpanDays, GranMonth}}
	for _, c := range cases {
		if got := Granularity(from, from.AddDays(c.span-1)); got != c.want {
			t.Errorf("span %d: %s, want %s", c.span, got, c.want)
		}
	}
}

func TestBucketEnds(t *testing.T) {
	// ISO weeks end on Sunday; the last bucket ends at `to` (a Wednesday here).
	got := dateStrings(BucketEnds(d("2026-09-30"), d("2026-10-21"), GranWeek))
	want := []string{"2026-10-04", "2026-10-11", "2026-10-18", "2026-10-21"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("weeks: %v, want %v", got, want)
	}
	// Month ends incl. a leap February; last bucket at `to`.
	got = dateStrings(BucketEnds(d("2028-01-15"), d("2028-04-10"), GranMonth))
	want = []string{"2028-01-31", "2028-02-29", "2028-03-31", "2028-04-10"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("months: %v, want %v", got, want)
	}
	// `to` already a month end is not duplicated.
	got = dateStrings(BucketEnds(d("2026-01-01"), d("2026-03-31"), GranMonth))
	if !reflect.DeepEqual(got, []string{"2026-01-31", "2026-02-28", "2026-03-31"}) {
		t.Errorf("months to month end: %v", got)
	}
	if n := len(BucketEnds(d("2026-01-01"), d("2026-01-03"), GranDay)); n != 3 {
		t.Errorf("days: %d points", n)
	}
	// Point count caps: <= 92 daily, <= 59 weekly, <= 121 monthly.
	today := d("2026-10-20")
	if n := len(BucketEnds(today.AddDays(-399), today, GranWeek)); n > 59 {
		t.Errorf("weekly points %d", n)
	}
	if n := len(BucketEnds(today.AddDays(-(MaxSpanDays - 1)), today, GranMonth)); n > 121 {
		t.Errorf("monthly points %d", n)
	}
}

func TestSample(t *testing.T) {
	rows := []runRow{{day: d("2026-01-02"), sum: 10, n: 1, attempts: 1}, {day: d("2026-01-05"), sum: 30, n: 2, attempts: 3}}
	got := sample(rows, []Date{d("2026-01-01"), d("2026-01-02"), d("2026-01-04"), d("2026-01-05"), d("2026-01-09")})
	want := []agg{{}, {10, 1, 1}, {10, 1, 1}, {30, 2, 3}, {30, 2, 3}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sample: %v, want %v", got, want)
	}
	if p := (agg{}).point(false); p.Value != nil || !p.Insufficient {
		t.Errorf("gap point: %+v", p)
	}
	if p := (agg{sum: 30, n: 3}).point(false); p.Value == nil || *p.Value != 10 || !p.Insufficient {
		t.Errorf("admin insufficient point: %+v", p)
	}
	if p := (agg{sum: 30, n: 3}).point(true); p.Value != nil {
		t.Errorf("group insufficient point keeps value: %+v", p)
	}
}
