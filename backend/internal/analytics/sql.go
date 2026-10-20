package analytics

import (
	"cmp"
	"context"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"bigroup-backend/internal/httpapi/dto"
)

// SQL building blocks. Every business date bound is an Asia/Almaty day passed as a
// parameter; nothing here uses now()/current_date (the only exception is the existing
// assessments.VisibleSQL expiry predicate, reused as-is for per-test completion lists).

// popSQL is the population predicate for users alias u (plan sec. 4).
const popSQL = `u.role = 'employee' AND u.status = 'active'`

// dayOf is the Asia/Almaty calendar day of attempts alias t.
const dayOf = `(t.completed_at AT TIME ZONE 'Asia/Almaty')::date`

// startOf / endOf turn a ::date parameter into the Almaty-midnight instant bounds:
// t.completed_at >= startOf($n) AND t.completed_at < endOf($n).
func startOf(p string) string { return `((` + p + `::date)::timestamp AT TIME ZONE 'Asia/Almaty')` }
func endOf(p string) string   { return `((` + p + `::date + 1)::timestamp AT TIME ZONE 'Asia/Almaty')` }

// args accumulates positional parameters.
type args []any

func (a *args) add(v any) string {
	*a = append(*a, v)
	return fmt.Sprintf("$%d", len(*a))
}

// popFilter restricts the population by current department: one selection (Dept)
// or a set (Depts, the departments view). Both nil = whole company.
type popFilter struct {
	Dept  *DepartmentSel
	Depts []DepartmentSel
}

// cond returns " AND ..." for users alias u (empty when unfiltered).
func (f popFilter) cond(a *args) string {
	sels := f.Depts
	if f.Dept != nil {
		sels = []DepartmentSel{*f.Dept}
	}
	if len(sels) == 0 {
		return ""
	}
	var ids []int64
	none := false
	for _, s := range sels {
		if s.None {
			none = true
		} else {
			ids = append(ids, s.ID)
		}
	}
	var parts []string
	if len(ids) > 0 {
		parts = append(parts, `u.department_id = ANY(`+a.add(ids)+`::bigint[])`)
	}
	if none {
		parts = append(parts, `u.department_id IS NULL`)
	}
	return ` AND (` + strings.Join(parts, " OR ") + `)`
}

// noDept is the department key of "No department" in grouped results.
const noDept int64 = 0

// companyKey is the department key of the ungrouped (company or filtered) sweep.
const companyKey int64 = -1

type groupKey struct{ dk, cid int64 }

// runRow is one step of the as-of step function: from day on, until the next row,
// sum/n are the running score sum and open measurement count, attempts the running
// count of raw measurements.
type runRow struct {
	day      Date
	sum      float64
	n        int
	attempts int
}

// measSQL streams the population's raw measurements up to the last included day $2
// for characteristics $1 (no ORDER BY: one hash-join scan). {{DK}} is the department
// key expression, {{POP}} the population filter. Day is days since 1970-01-01 in
// Asia/Almaty; the score comes as integer hundredths (scores are NUMERIC(5,2)), so the
// sweep's sums are exact.
//
// Bench (Step 2b, .omc/evidence/manager-admin-analytics/bench.txt): the all-SQL sweep
// (DISTINCT ON + lead() + UNION ALL events + window sums) spent its time sorting and
// re-scanning the materialized CTE (external-merge sort, numeric sums over ~500k event
// rows); no index helps a full-history scan. The same sweep in Go over one streamed
// scan is several times faster, so SQL fetches and Go sweeps.
const measSQL = `
SELECT t.user_id, {{DK}}, s.characteristic_id,
       (` + dayOf + ` - DATE '1970-01-01')::int4, t.completed_at, t.id, (s.score * 100)::int4
  FROM users u
  JOIN attempts t ON t.user_id = u.id
  JOIN attempt_scores s ON s.attempt_id = t.id
 WHERE ` + popSQL + `{{POP}}
   AND s.characteristic_id = ANY($1::bigint[])
   AND t.completed_at < ((($2::date) + 1)::timestamp AT TIME ZONE 'Asia/Almaty')`

// measurement is one raw (user, characteristic, attempt) score.
type measurement struct {
	user, dk, cid int64
	day           int32 // Almaty days since 1970-01-01
	at            int64 // completed_at, unix microseconds
	aid           int64
	cents         int64 // score * 100
}

var epochDay = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)

func dateOfDay(n int32) Date { return dateFromUTC(epochDay.AddDate(0, 0, int(n))) }

// sweep runs the event sweep (plan sec. 4 "As-of value via event sweep") up to `to`
// and returns the step function per (department key, characteristic), rows ascending
// by day. byDept=false puts everything under companyKey.
//
// Per (user, characteristic) the measurements are deduplicated per Almaty day (latest
// completed_at, then attempt id, wins). Each kept measurement contributes +score/+1 on
// its day and -score/-1 on the user's next measurement day; every raw measurement adds
// +1 attempt on its day. Running sums of the per-day deltas give the as-of mean and n.
func (r *repo) sweep(ctx context.Context, cids []int64, to Date, f popFilter, byDept bool) (map[groupKey][]runRow, error) {
	out := map[groupKey][]runRow{}
	if len(cids) == 0 {
		return out, nil
	}
	ms, err := r.measurements(ctx, cids, to, f, byDept)
	if err != nil {
		return nil, err
	}
	return sweepRows(ms), nil
}

// sweepRows computes the step functions from raw measurements (sorts ms in place).
func sweepRows(ms []measurement) map[groupKey][]runRow {
	out := map[groupKey][]runRow{}
	if len(ms) == 0 {
		return out
	}
	slices.SortFunc(ms, func(a, b measurement) int {
		switch {
		case a.user != b.user:
			return cmp.Compare(a.user, b.user)
		case a.cid != b.cid:
			return cmp.Compare(a.cid, b.cid)
		case a.at != b.at: // day is a function of at
			return cmp.Compare(a.at, b.at)
		}
		return cmp.Compare(a.aid, b.aid)
	})
	// Per-day deltas live in one dense slice per group, indexed by day - minDay.
	minDay, maxDay := ms[0].day, ms[0].day
	for i := range ms {
		minDay, maxDay = min(minDay, ms[i].day), max(maxDay, ms[i].day)
	}
	span := int(maxDay-minDay) + 1
	type delta struct {
		cents       int64
		n, attempts int32
		touched     bool
	}
	days := map[groupKey][]delta{}
	for i := 0; i < len(ms); {
		j := i // [i, j) is one (user, characteristic)
		for j < len(ms) && ms[j].user == ms[i].user && ms[j].cid == ms[i].cid {
			j++
		}
		k := groupKey{ms[i].dk, ms[i].cid}
		ds := days[k]
		if ds == nil {
			ds = make([]delta, span)
			days[k] = ds
		}
		var prev *measurement
		for x := i; x < j; x++ {
			m := &ms[x]
			d := &ds[m.day-minDay]
			d.attempts++
			d.touched = true
			if x+1 < j && ms[x+1].day == m.day {
				continue // a later measurement on the same day wins
			}
			d.cents += m.cents
			d.n++
			if prev != nil {
				d.cents -= prev.cents
				d.n--
			}
			prev = m
		}
		i = j
	}
	for k, ds := range days {
		var rows []runRow
		var cents int64
		var n, attempts int
		for off := range ds {
			d := &ds[off]
			if !d.touched {
				continue
			}
			cents, n, attempts = cents+d.cents, n+int(d.n), attempts+int(d.attempts)
			rows = append(rows, runRow{day: dateOfDay(minDay + int32(off)), sum: float64(cents) / 100, n: n, attempts: attempts})
		}
		out[k] = rows
	}
	return out
}

// scanParts splits the measurement scan into user_id % scanParts partitions fetched
// on separate connections (bench: row transfer dominates, ~2x faster with 4).
const scanParts = 4

// measurements fetches the raw measurements of the sweep (see measSQL).
func (r *repo) measurements(ctx context.Context, cids []int64, to Date, f popFilter, byDept bool) ([]measurement, error) {
	dk := fmt.Sprintf("%d::bigint", companyKey)
	if byDept {
		dk = fmt.Sprintf("COALESCE(u.department_id, %d)", noDept)
	}
	parts := make([][]measurement, scanParts)
	fns := make([]func() error, scanParts)
	for i := range fns {
		fns[i] = func() error {
			a := args{cids, to.String()}
			pop := f.cond(&a) + fmt.Sprintf(" AND u.id %% %d = %d", scanParts, i)
			q := strings.NewReplacer("{{DK}}", dk, "{{POP}}", pop).Replace(measSQL)
			rows, err := r.pool.Query(ctx, q, a...)
			if err != nil {
				return err
			}
			defer rows.Close()
			var m measurement
			var completed time.Time
			var cents int32
			for rows.Next() {
				if err := rows.Scan(&m.user, &m.dk, &m.cid, &m.day, &completed, &m.aid, &cents); err != nil {
					return err
				}
				m.at, m.cents = completed.UnixMicro(), int64(cents)
				parts[i] = append(parts[i], m)
			}
			return rows.Err()
		}
	}
	if err := parallel(fns...); err != nil {
		return nil, err
	}
	return slices.Concat(parts...), nil
}

// parallel runs fns concurrently (each on its own pool connection) and returns the
// first error in argument order.
func parallel(fns ...func() error) error {
	errs := make([]error, len(fns))
	var wg sync.WaitGroup
	for i, fn := range fns {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = fn()
		}()
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// agg is one sampled aggregate.
type agg struct {
	sum      float64
	n        int
	attempts int
}

// sample reads the step function at each ascending bucket end in one linear pass.
func sample(rows []runRow, ends []Date) []agg {
	out := make([]agg, len(ends))
	i := -1
	for j, e := range ends {
		for i+1 < len(rows) && !rows[i+1].day.After(e) {
			i++
		}
		if i >= 0 {
			out[j] = agg{sum: rows[i].sum, n: rows[i].n, attempts: rows[i].attempts}
		}
	}
	return out
}

// point converts an aggregate to a ValuePoint. n = 0 is a gap. Below MinSample the
// point is insufficient; group=true (department and other group views) also withholds
// the value, otherwise the unredacted (admin) value is kept.
func (g agg) point(group bool) dto.ValuePoint {
	p := dto.ValuePoint{Employees: g.n, Attempts: g.attempts, Insufficient: g.n < MinSample}
	if g.n > 0 && !(group && p.Insufficient) {
		v := round2(g.sum / float64(g.n))
		p.Value = &v
	}
	return p
}

func round2(x float64) float64 { return math.Round(x*100) / 100 }

func itoa(n int) string { return strconv.Itoa(n) }

func ratio(num, den int) *float64 {
	if den == 0 {
		return nil
	}
	v := math.Round(float64(num)/float64(den)*10000) / 10000
	return &v
}
