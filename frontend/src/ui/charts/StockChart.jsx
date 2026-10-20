// Stock-style multi-series line chart for company aggregates (dashboard and /analytics).
// FROZEN PROP CONTRACT (plan section 7, Step 1): stream C implements the chart behind
// these props; any change goes back through stream 0. Geometry lives in stockGeometry.js.
// Gaps (value null) are never bridged; insufficient runs are drawn dimmed ('dim') or not at all ('hidden').
// Hand-written SVG sized by ResizeObserver (viewBox = measured px, so text never scales). Hover, tap (pins)
// or Left/Right keys move a crosshair; the tooltip lists every series with its sample size. The same data
// is in a visually hidden table for assistive tech.
import { useEffect, useId, useMemo, useRef, useState } from 'react'
import { useTranslation } from '../../i18n'
import { cx } from '../../lib/cx'
import {
  Y_TICKS,
  drawable,
  monotonePath,
  nearestIndex,
  parseDay,
  pickTicks,
  pointState,
  splitSeries,
  spreadTags,
  xScale,
  yScale,
} from './stockGeometry'

/**
 * @typedef {{ value: number|null, employees: number, attempts: number, insufficient: boolean }} ValuePoint
 *
 * @typedef {Object} StockSeries
 * @property {number|string} id
 * @property {string} label
 * @property {string} color                CSS colour, e.g. 'var(--core-1)'
 * @property {boolean} dashed              extras are dashed
 * @property {boolean} core
 * @property {ValuePoint[]} values         aligned with `points`
 *
 * @typedef {Object} StockChartProps
 * @property {string[]} points             bucket end dates (YYYY-MM-DD)
 * @property {'day'|'week'|'month'} granularity
 * @property {StockSeries[]} series        picker-driven, 1..10
 * @property {'dim'|'hidden'} insufficientStyle   dim when a value is present, hidden otherwise
 * @property {{ label: string, values: ValuePoint[] }} [reference]   company line in the department view
 * @property {{ date: string, seriesId: number|string, value: number }[]} [markers]   employee measurement dots
 * @property {'ru'|'kk'} lang
 */

const PAD_T = 14
const PAD_B = 30
const PAD_L = 34
const TAG_W = 40
const TAG_H = 17
const REF_ID = '__reference'
const DASH = '7 5'
const DOTTED = '1.5 4.5'

const fmt = (v) => `${Math.round(v * 10) / 10}`

// Date labels from i18n month lists ("янв.|февр.|..."), never Intl: Chromium lacks Kazakh month names.
function useDateLabels(t) {
  return useMemo(() => {
    const gen = t('dashboard.chart.months_gen').split('|')
    const nom = t('dashboard.chart.months_nom').split('|')
    const tick = (s, g) => {
      const p = parseDay(s)
      if (!p) return String(s ?? '')
      return g === 'month' ? t('dashboard.chart.month', { m: nom[p.m], y: p.y }) : t('dashboard.chart.day', { d: p.d, m: gen[p.m] })
    }
    const full = (s, g) => {
      const p = parseDay(s)
      if (!p) return String(s ?? '')
      if (g === 'month') return t('dashboard.chart.month', { m: nom[p.m], y: p.y })
      const day = t('dashboard.chart.day_full', { d: p.d, m: gen[p.m], y: p.y })
      return g === 'week' ? t('dashboard.chart.week', { date: day }) : day
    }
    return { tick, full }
  }, [t])
}

function useSize(ref) {
  const [size, setSize] = useState({ w: 640, h: 288 })
  useEffect(() => {
    const el = ref.current
    if (!el || typeof ResizeObserver === 'undefined') return undefined
    const ro = new ResizeObserver(([entry]) => {
      const { width, height } = entry.contentRect
      if (width > 0 && height > 0) setSize((s) => (s.w === Math.round(width) && s.h === Math.round(height) ? s : { w: Math.round(width), h: Math.round(height) }))
    })
    ro.observe(el)
    return () => ro.disconnect()
  }, [ref])
  return size
}

// Text for one value point: "61.4 %", "not enough data (n = 3)" or "no data". Uses the same rule as drawing,
// so a point hidden by insufficientStyle 'hidden' never shows its number.
function pointText(t, p, style) {
  const state = pointState(p, style)
  if (state === 'value') return t('dashboard.chart.value', { value: fmt(p.value) })
  if (state === 'insufficient') return t('stats.insufficient', { n: p.employees })
  return t('stats.no_data')
}

function sampleText(t, p) {
  if (!p || !(p.employees > 0)) return ''
  return `${t('stats.sample.employees', { n: p.employees })} · ${t('stats.sample.attempts', { n: p.attempts })}`
}

function Swatch({ color, dashed, dotted }) {
  return (
    <svg width="22" height="10" viewBox="0 0 22 10" aria-hidden="true" className="shrink-0 overflow-visible">
      <line
        x1="1"
        y1="5"
        x2="21"
        y2="5"
        stroke={color}
        strokeWidth={dotted ? 2 : 2.5}
        strokeLinecap="round"
        strokeDasharray={dotted ? '1 3.5' : dashed ? '5 3' : undefined}
      />
    </svg>
  )
}

/** @param {StockChartProps} props */
export default function StockChart({ points = [], granularity = 'day', series = [], insufficientStyle = 'dim', reference, markers, lang }) {
  const { t } = useTranslation()
  const labels = useDateLabels(t)
  const boxRef = useRef(null)
  const { w, h } = useSize(boxRef)
  const uid = useId().replace(/[^a-zA-Z0-9_-]/g, '')
  const hintId = `stock-hint-${uid}`

  const [active, setActive] = useState(null)
  const [pinned, setPinned] = useState(false)
  const [focusSeries, setFocusSeries] = useState(null)
  const down = useRef(null)

  const n = points.length
  const narrow = w < 420
  const x0 = PAD_L
  const x1 = Math.max(x0 + 1, w - TAG_W - (narrow ? 6 : 10))
  const yTop = PAD_T
  const yBot = Math.max(yTop + 1, h - PAD_B)
  const x = useMemo(() => xScale(n, x0, x1), [n, x0, x1])
  const y = useMemo(() => yScale(yTop, yBot), [yTop, yBot])

  // Everything drawn, reference first (underneath).
  const lines = useMemo(() => {
    const all = series.map((s) => ({ ...s, ref: false }))
    if (reference?.values?.length) {
      all.unshift({ id: REF_ID, label: reference.label, color: 'var(--muted)', dashed: false, core: false, values: reference.values, ref: true })
    }
    return all.map((s) => ({ ...s, ...splitSeries(s.values || [], insufficientStyle) }))
  }, [series, reference, insufficientStyle])

  const empty = lines.every((s) => s.segments.length === 0 && s.dots.length === 0)

  const ticks = useMemo(() => {
    if (!n) return []
    const sample = points.map((p) => labels.tick(p, granularity))
    const longest = sample.reduce((m, s) => Math.max(m, s.length), 0)
    return pickTicks(n, x1 - x0, longest * 6.4 + 6).map((i) => ({ i, text: sample[i] }))
  }, [points, n, granularity, labels, x0, x1])

  // Latest value per series, as stock-style tags in the right gutter.
  const tags = useMemo(() => {
    const raw = []
    for (const s of lines) {
      const vals = s.values || []
      for (let i = vals.length - 1; i >= 0; i--) {
        if (drawable(vals[i], insufficientStyle)) {
          raw.push({ id: s.id, color: s.color, ref: s.ref, dim: !!vals[i].insufficient, v: vals[i].value, y: y(vals[i].value) })
          break
        }
      }
    }
    return spreadTags(raw, TAG_H + 2, yTop + TAG_H / 2, yBot - TAG_H / 2)
  }, [lines, insufficientStyle, y, yTop, yBot])

  const markerDots = useMemo(() => {
    if (!markers?.length || !n) return []
    const colorOf = new Map(series.map((s) => [s.id, s.color]))
    return markers
      .map((m) => {
        let i = points.indexOf(m.date)
        if (i < 0) i = points.findIndex((p) => p >= m.date)
        return i < 0 ? null : { ...m, i, color: colorOf.get(m.seriesId) || 'var(--ink)' }
      })
      .filter(Boolean)
  }, [markers, points, n, series])

  const idx = active == null || active >= n ? null : active

  const toLocalX = (e) => {
    const r = e.currentTarget.getBoundingClientRect()
    return ((e.clientX - r.left) / Math.max(r.width, 1)) * w
  }
  const onPointerMove = (e) => {
    if (pinned || e.pointerType !== 'mouse' || !n) return
    setActive(nearestIndex(toLocalX(e), n, x0, x1))
  }
  const onPointerLeave = () => {
    if (!pinned) setActive(null)
  }
  const onPointerDown = (e) => {
    down.current = { x: e.clientX, y: e.clientY }
  }
  // Tap / click pins the crosshair to the nearest point; tapping the same point again releases it.
  const onPointerUp = (e) => {
    const d = down.current
    down.current = null
    if (!d || !n || Math.hypot(e.clientX - d.x, e.clientY - d.y) > 8) return
    const i = nearestIndex(toLocalX(e), n, x0, x1)
    if (pinned && i === idx) {
      setPinned(false)
      setActive(e.pointerType === 'mouse' ? i : null)
    } else {
      setPinned(true)
      setActive(i)
    }
  }
  const onKeyDown = (e) => {
    if (!n) return
    const last = n - 1
    const cur = (v) => (v == null || v >= n ? null : v)
    let step
    if (e.key === 'ArrowLeft') step = (v) => (cur(v) == null ? last : Math.max(0, v - 1))
    else if (e.key === 'ArrowRight') step = (v) => (cur(v) == null ? last : Math.min(last, v + 1))
    else if (e.key === 'Home') step = () => 0
    else if (e.key === 'End') step = () => last
    else if (e.key === 'Escape') {
      setPinned(false)
      setActive(null)
      return
    } else return
    e.preventDefault()
    setPinned(true)
    setActive(step)
  }

  const live = idx == null ? '' : [labels.full(points[idx], granularity), ...lines.map((s) => `${s.label}: ${pointText(t, s.values?.[idx], insufficientStyle)}`)].join('; ')

  const tipLeft = idx == null ? 0 : x(idx)
  const tipOnLeft = tipLeft > w / 2
  const fadeOf = (id) => (focusSeries == null || focusSeries === id ? 1 : 0.18)
  const revealKey = `${points[0]}-${n}-${series.map((s) => s.id).join('.')}`

  return (
    <div className="min-w-0" lang={lang}>
      <div
        ref={boxRef}
        role="figure"
        aria-label={t('stats.chart.caption')}
        aria-describedby={hintId}
        tabIndex={0}
        onKeyDown={onKeyDown}
        onBlur={() => {
          if (!pinned) setActive(null)
        }}
        className="relative h-72 w-full select-none rounded-ctl outline-offset-4 md:h-96"
      >
        <svg
          width={w}
          height={h}
          viewBox={`0 0 ${w} ${h}`}
          aria-hidden="true"
          className="absolute inset-0 block h-full w-full touch-pan-y"
          onPointerMove={onPointerMove}
          onPointerLeave={onPointerLeave}
          onPointerDown={onPointerDown}
          onPointerUp={onPointerUp}
          style={{ cursor: n ? 'crosshair' : 'default' }}
        >
          <defs>
            <clipPath id={`clip-${uid}`}>
              <rect key={revealKey} className="chart-reveal" x={x0 - 8} y={0} width={Math.max(0, w - x0 + 8)} height={h} />
            </clipPath>
          </defs>

          {/* Grid: hairlines at 0/25/50/75/100, the baseline a touch stronger. */}
          {Y_TICKS.map((v) => (
            <g key={v}>
              <line x1={x0} x2={x1} y1={y(v)} y2={y(v)} stroke={v === 0 ? 'var(--line-strong)' : 'var(--line)'} strokeWidth="1" shapeRendering="crispEdges" />
              <text x={x0 - 8} y={y(v)} dy="0.35em" textAnchor="end" fontSize="11" fill="var(--faint)" className="tabular-nums">
                {v}
              </text>
            </g>
          ))}

          {ticks.map(({ i, text }) => {
            const tx = x(i)
            const half = (text.length * 6.4) / 2
            const anchor = n > 1 && tx - half < 2 ? 'start' : tx + half > w - 2 ? 'end' : 'middle'
            return (
              <g key={i}>
                <line x1={tx} x2={tx} y1={yBot} y2={yBot + 4} stroke="var(--line-strong)" shapeRendering="crispEdges" />
                <text x={tx} y={h - 9} textAnchor={anchor} fontSize="11" fill="var(--faint)">
                  {text}
                </text>
              </g>
            )
          })}

          <g clipPath={`url(#clip-${uid})`}>
            {lines.map((s) => (
              <g key={s.id} opacity={fadeOf(s.id)} style={{ transition: 'opacity 0.16s ease-out' }}>
                {s.segments.map((seg, k) => (
                  <path
                    key={k}
                    d={monotonePath(seg.points.map((p) => ({ x: x(p.i), y: y(p.v) })))}
                    fill="none"
                    stroke={s.color}
                    strokeWidth={s.ref ? 1.75 : seg.dim ? 1.75 : 2.25}
                    strokeOpacity={seg.dim ? 0.6 : 1}
                    strokeDasharray={seg.dim || s.ref ? DOTTED : s.dashed ? DASH : undefined}
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  />
                ))}
                {s.dots.map((d) => (
                  <circle key={d.i} cx={x(d.i)} cy={y(d.v)} r={3.5} fill={s.color} fillOpacity={d.dim ? 0.45 : 1} stroke="var(--surface)" strokeWidth="1.5" />
                ))}
              </g>
            ))}
            {markerDots.map((m, k) => (
              <circle key={k} cx={x(m.i)} cy={y(m.value)} r="4" fill="var(--surface)" stroke={m.color} strokeWidth="2" opacity={fadeOf(m.seriesId)} />
            ))}
          </g>

          {/* Latest-value tags in the right gutter, joined to the line end by a hairline. */}
          {!empty &&
            tags.map((tg) => (
              <g key={tg.id} opacity={fadeOf(tg.id) * (tg.dim ? 0.6 : 1)} style={{ transition: 'opacity 0.16s ease-out' }}>
                <line x1={x1} x2={x1 + 5} y1={tg.y} y2={tg.ty} stroke={tg.color} strokeWidth="1" />
                <rect x={x1 + 5} y={tg.ty - TAG_H / 2} width={TAG_W - 6} height={TAG_H} rx="3" fill={tg.ref ? 'var(--surface-2)' : tg.color} stroke={tg.ref ? 'var(--line-strong)' : 'none'} />
                <text
                  x={x1 + 5 + (TAG_W - 6) / 2}
                  y={tg.ty}
                  dy="0.35em"
                  textAnchor="middle"
                  fontSize="10.5"
                  fontWeight="700"
                  fill={tg.ref ? 'var(--ink)' : 'var(--surface)'}
                  className="tabular-nums"
                >
                  {fmt(tg.v)}
                </text>
              </g>
            ))}

          {idx != null && (
            <g pointerEvents="none">
              <line x1={x(idx)} x2={x(idx)} y1={yTop} y2={yBot} stroke="var(--ink)" strokeOpacity="0.45" strokeWidth="1" strokeDasharray="3 3" shapeRendering="crispEdges" />
              {lines.map((s) => {
                const p = s.values?.[idx]
                if (!drawable(p, insufficientStyle)) return null
                return <circle key={s.id} cx={x(idx)} cy={y(p.value)} r="4.5" fill={s.color} fillOpacity={p.insufficient ? 0.5 : 1} stroke="var(--surface)" strokeWidth="2" />
              })}
            </g>
          )}

          {empty && (
            <text x={(x0 + x1) / 2} y={(yTop + yBot) / 2} textAnchor="middle" fontSize="13" fill="var(--muted)">
              {t('stats.no_data_period')}
            </text>
          )}
        </svg>

        {idx != null && (
          <div
            className="pointer-events-none absolute top-2 z-10 w-max max-w-[min(22rem,calc(100%-1rem))] rounded-card border border-line bg-surface px-3 py-2 text-caption shadow-pop"
            style={tipOnLeft ? { right: Math.max(8, w - tipLeft + 12) } : { left: Math.min(tipLeft + 12, Math.max(8, w - 280)) }}
          >
            <p className="mb-1.5 font-semibold text-ink">{labels.full(points[idx], granularity)}</p>
            <ul className="space-y-1.5">
              {lines.map((s) => {
                const p = s.values?.[idx]
                const has = pointState(p, insufficientStyle) === 'value'
                return (
                  <li key={s.id} className="min-w-0">
                    <div className="flex items-center gap-2">
                      <Swatch color={s.color} dashed={s.dashed} dotted={s.ref} />
                      <span className="min-w-0 flex-1 truncate text-muted">{s.label}</span>
                      <b className={cx('shrink-0 tabular-nums', has ? 'text-ink' : 'font-medium text-faint', has && p.insufficient && 'opacity-70')}>{pointText(t, p, insufficientStyle)}</b>
                    </div>
                    {p?.employees > 0 && <p className="pl-[30px] tabular-nums text-faint sm:whitespace-nowrap">{sampleText(t, p)}</p>}
                  </li>
                )
              })}
            </ul>
          </div>
        )}

        <p id={hintId} className="sr-only">
          {t('dashboard.chart.hint')}
        </p>
        <p className="sr-only" aria-live="polite">
          {live}
        </p>
      </div>

      {/* Legend: hovering an entry highlights its series. */}
      <ul aria-label={t('dashboard.chart.legend')} className="mt-3 flex flex-wrap gap-x-4 gap-y-1.5">
        {lines.map((s) => (
          <li
            key={s.id}
            onMouseEnter={() => setFocusSeries(s.id)}
            onMouseLeave={() => setFocusSeries(null)}
            className={cx(
              'flex min-w-0 max-w-full items-center gap-2 text-caption transition-opacity',
              focusSeries != null && focusSeries !== s.id ? 'opacity-50' : 'opacity-100',
            )}
          >
            <Swatch color={s.color} dashed={s.dashed} dotted={s.ref} />
            <span className={cx('min-w-0 break-words', s.core ? 'font-semibold text-ink' : 'text-muted')}>{s.label}</span>
          </li>
        ))}
      </ul>

      {/* sr-only on a wrapper: a <table> ignores the 1px width and would widen the page. */}
      <div className="sr-only">
      <table>
        <caption>{t('stats.chart.table_caption')}</caption>
        <thead>
          <tr>
            <th scope="col">{t('stats.period.label')}</th>
            {lines.map((s) => (
              <th key={s.id} scope="col">
                {s.label}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {points.map((d, i) => (
            <tr key={d}>
              <th scope="row">{labels.full(d, granularity)}</th>
              {lines.map((s) => {
                const p = s.values?.[i]
                const sample = sampleText(t, p)
                return <td key={s.id}>{sample ? `${pointText(t, p, insufficientStyle)} (${sample})` : pointText(t, p, insufficientStyle)}</td>
              })}
            </tr>
          ))}
        </tbody>
      </table>
      </div>
    </div>
  )
}
