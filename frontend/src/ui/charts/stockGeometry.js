// Pure geometry for StockChart.jsx (unit-tested in stockGeometry.test.mjs). No React, no i18n.
// Values are 0-100 scores; `null` is a gap that is never bridged.

export const Y_TICKS = [0, 25, 50, 75, 100]

export const clamp01 = (v) => Math.min(Math.max(v, 0), 100)

// Index -> x across [x0, x1]; a single point sits in the middle.
export function xScale(n, x0, x1) {
  if (n <= 1) return () => (x0 + x1) / 2
  const step = (x1 - x0) / (n - 1)
  return (i) => x0 + i * step
}

// Value (clamped to 0-100) -> y, 100 at `top`, 0 at `bottom`.
export function yScale(top, bottom) {
  return (v) => bottom - (clamp01(v) / 100) * (bottom - top)
}

// Nearest point index for a pointer x (clamped to the plot).
export function nearestIndex(px, n, x0, x1) {
  if (n <= 1) return 0
  const i = Math.round(((px - x0) / (x1 - x0)) * (n - 1))
  return Math.min(Math.max(i, 0), n - 1)
}

// A value is drawable when it is a finite number and, for insufficientStyle 'hidden', not insufficient.
export function drawable(p, insufficientStyle = 'dim') {
  if (!p || p.value == null || !Number.isFinite(p.value)) return false
  return !(insufficientStyle === 'hidden' && p.insufficient)
}

// What a point shows in text (tooltip, live region, hidden table), consistent with what is drawn:
// 'value' when drawable, 'insufficient' when withheld or hidden with a sample (n > 0), else 'none'.
export function pointState(p, insufficientStyle = 'dim') {
  if (drawable(p, insufficientStyle)) return 'value'
  return p && p.employees > 0 ? 'insufficient' : 'none'
}

// Splits one series into what is drawn:
//   segments: [{ dim, points: [{ i, v }] }] with >= 2 points each. A "chain" is a maximal run of drawable
//             values; nulls end it (gaps are never bridged). Inside a chain, insufficient runs become their
//             own dim segments that share the boundary point with their sufficient neighbours, so the line
//             stays continuous while the dim part is styled separately.
//   dots:     [{ i, v, dim }] for chains of a single point (an isolated value is a dot, never a line).
export function splitSeries(values, insufficientStyle = 'dim') {
  const segments = []
  const dots = []
  const chains = []
  let cur = []
  values.forEach((p, i) => {
    if (drawable(p, insufficientStyle)) cur.push({ i, v: clamp01(p.value), dim: !!p.insufficient })
    else if (cur.length) {
      chains.push(cur)
      cur = []
    }
  })
  if (cur.length) chains.push(cur)

  for (const chain of chains) {
    if (chain.length === 1) {
      dots.push({ i: chain[0].i, v: chain[0].v, dim: chain[0].dim })
      continue
    }
    // Pairwise: an edge is dim when either end is insufficient. Group consecutive edges by dimness.
    let seg = null
    for (let k = 0; k < chain.length - 1; k++) {
      const a = chain[k]
      const b = chain[k + 1]
      const dim = a.dim || b.dim
      if (seg && seg.dim === dim) seg.points.push({ i: b.i, v: b.v })
      else {
        seg = { dim, points: [{ i: a.i, v: a.v }, { i: b.i, v: b.v }] }
        segments.push(seg)
      }
    }
  }
  return { segments, dots }
}

// Monotone cubic interpolation (Fritsch-Carlson, as d3.curveMonotoneX): never overshoots between points,
// so a curve through 0-100 values stays inside 0-100. pts: [{ x, y }] with increasing x.
export function monotonePath(pts) {
  const n = pts.length
  if (n === 0) return ''
  const f = (v) => Math.round(v * 100) / 100
  if (n === 1) return `M${f(pts[0].x)},${f(pts[0].y)}`
  if (n === 2) return `M${f(pts[0].x)},${f(pts[0].y)}L${f(pts[1].x)},${f(pts[1].y)}`
  const h = []
  const s = []
  for (let i = 0; i < n - 1; i++) {
    h.push(pts[i + 1].x - pts[i].x)
    s.push(h[i] ? (pts[i + 1].y - pts[i].y) / h[i] : 0)
  }
  const sign = (v) => (v < 0 ? -1 : 1)
  const t = new Array(n)
  for (let i = 1; i < n - 1; i++) {
    const s0 = s[i - 1]
    const s1 = s[i]
    const p = (s0 * h[i] + s1 * h[i - 1]) / (h[i - 1] + h[i])
    t[i] = (sign(s0) + sign(s1)) * Math.min(Math.abs(s0), Math.abs(s1), 0.5 * Math.abs(p)) || 0
  }
  t[0] = (3 * s[0] - t[1]) / 2
  t[n - 1] = (3 * s[n - 2] - t[n - 2]) / 2
  // Endpoint tangents must keep the end interval monotone too.
  if (sign(t[0]) !== sign(s[0]) || s[0] === 0) t[0] = 0
  if (sign(t[n - 1]) !== sign(s[n - 2]) || s[n - 2] === 0) t[n - 1] = 0
  t[0] = sign(t[0]) * Math.min(Math.abs(t[0]), 3 * Math.abs(s[0]))
  t[n - 1] = sign(t[n - 1]) * Math.min(Math.abs(t[n - 1]), 3 * Math.abs(s[n - 2]))

  let d = `M${f(pts[0].x)},${f(pts[0].y)}`
  for (let i = 0; i < n - 1; i++) {
    const dx = h[i] / 3
    d += `C${f(pts[i].x + dx)},${f(pts[i].y + dx * t[i])},${f(pts[i + 1].x - dx)},${f(pts[i + 1].y - dx * t[i + 1])},${f(pts[i + 1].x)},${f(pts[i + 1].y)}`
  }
  return d
}

// X-axis tick indices for `n` points on a plot `width` px wide, with labels `labelWidth` px wide.
// Anchored on the latest point (the right edge always has a label); stride keeps labels >= gap apart.
export function pickTicks(n, width, labelWidth, gap = 12) {
  if (n <= 0 || width <= 0) return []
  if (n === 1) return [0]
  const step = width / (n - 1)
  const stride = Math.max(1, Math.ceil((labelWidth + gap) / step))
  const out = []
  for (let i = n - 1; i >= 0; i -= stride) out.push(i)
  return out.reverse()
}

// Spreads right-edge value tags vertically so they do not overlap (min distance `gap`), within [top, bottom].
// tags: [{ y, ... }] -> same objects with `ty` (the label y), order preserved.
export function spreadTags(tags, gap, top, bottom) {
  const sorted = tags.map((t, k) => ({ t, k })).sort((a, b) => a.t.y - b.t.y)
  const ys = sorted.map(({ t }) => Math.min(Math.max(t.y, top), bottom))
  for (let i = 1; i < ys.length; i++) if (ys[i] - ys[i - 1] < gap) ys[i] = ys[i - 1] + gap
  // Overflowed the bottom: push back up.
  if (ys.length && ys[ys.length - 1] > bottom) {
    ys[ys.length - 1] = bottom
    for (let i = ys.length - 2; i >= 0; i--) if (ys[i + 1] - ys[i] < gap) ys[i] = ys[i + 1] - gap
  }
  const out = new Array(tags.length)
  sorted.forEach(({ t, k }, i) => {
    out[k] = { ...t, ty: ys[i] }
  })
  return out
}

// 'YYYY-MM-DD' -> { y, m (0-11), d } without going through Date (no time-zone shifts).
export function parseDay(s) {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(String(s || ''))
  if (!m) return null
  return { y: Number(m[1]), m: Number(m[2]) - 1, d: Number(m[3]) }
}

// Series colours for characteristics: core ones (by id) get --core-1..4 solid, the rest --extra-1..6
// dashed in the order given. Returns the seriesStyle function CompanyTrendCard expects.
export function paletteFor(characteristics) {
  const list = characteristics || []
  const core = list.filter((c) => c.core).map((c) => c.id).sort((a, b) => a - b)
  const extras = list.filter((c) => !c.core).map((c) => c.id)
  return (c) => {
    if (c.core) {
      const k = core.indexOf(c.id)
      return { color: `var(--core-${((k < 0 ? 0 : k) % 4) + 1})`, dashed: false }
    }
    const k = extras.indexOf(c.id)
    return { color: `var(--extra-${((k < 0 ? 0 : k) % 6) + 1})`, dashed: true }
  }
}
