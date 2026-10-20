// Radar chart of all characteristics (0-100) with a legend. No chart library.
// items: [{ id, label, value (mean 0..100), measured (bool), color, core (bool) }]
// Core items get their own colour on the vertex marker and the axis label; the rest are neutral.
// Hovering / focusing a legend entry (or an axis) highlights that axis; clicking pins it.
// The same data is available to assistive tech as a visually hidden table.
import { useState } from 'react'
import { useTranslation } from '../../i18n'
import { cx } from '../../lib/cx'
import { axisAngle, axisPoint, wrapLabel } from './radarGeometry'

const W = 460
const H = 400
const CX = W / 2
const CY = H / 2
const R = 108
const RINGS = [20, 40, 60, 80, 100]
const LINE_H = 14

const fmt = (v) => `${Math.round(v * 10) / 10}`

function AxisLabel({ item, i, n, active, dimmed }) {
  const a = axisAngle(i, n)
  const cos = Math.cos(a)
  const sin = Math.sin(a)
  const lines = wrapLabel(item.label)
  const x = CX + (R + 14) * cos
  const baseY = CY + (R + 14) * sin
  const anchor = cos > 0.25 ? 'start' : cos < -0.25 ? 'end' : 'middle'
  // Stack lines away from the chart: upwards at the top, downwards at the bottom, centred at the sides.
  let y0
  if (sin < -0.5) y0 = baseY - (lines.length - 1) * LINE_H
  else if (sin > 0.5) y0 = baseY + 10
  else y0 = baseY - ((lines.length - 1) * LINE_H) / 2 + 4
  return (
    <text
      x={x}
      y={y0}
      textAnchor={anchor}
      fontSize="13"
      fontWeight={item.core || active ? 700 : 500}
      fill={item.core ? item.color : 'var(--muted)'}
      opacity={dimmed ? 0.5 : 1}
      style={{ transition: 'opacity 0.16s ease-out' }}
    >
      <title>{item.label}</title>
      {lines.map((ln, k) => (
        <tspan key={k} x={x} dy={k === 0 ? 0 : LINE_H}>
          {ln}
        </tspan>
      ))}
    </text>
  )
}

function Legend({ items, activeId, setHover, setFocus, pinned, setPinned, t }) {
  const Row = ({ item, compact }) => {
    const on = activeId === item.id
    return (
      <li key={item.id} className="min-w-0">
        <button
          type="button"
          aria-pressed={pinned === item.id}
          onMouseEnter={() => setHover(item.id)}
          onMouseLeave={() => setHover(null)}
          onFocus={() => setFocus(item.id)}
          onBlur={() => setFocus(null)}
          onClick={() => setPinned(pinned === item.id ? null : item.id)}
          className={cx(
            'flex w-full min-w-0 items-center gap-3 rounded-ctl border text-left',
            compact ? 'min-h-9 px-2 py-1 text-caption' : 'min-h-touch px-3 py-2 text-label',
            on ? 'border-line-strong bg-surface-2' : 'border-transparent hover:bg-surface-2',
          )}
        >
          <span
            aria-hidden="true"
            className={cx('shrink-0 rounded-full', compact ? 'h-2.5 w-2.5' : 'h-3.5 w-3.5')}
            style={{ backgroundColor: item.color }}
          />
          <span className="min-w-0 flex-1 break-words text-ink">{item.label}</span>
          <b className="shrink-0 tabular-nums text-ink">{item.measured ? `${fmt(item.value)}%` : t('radar.none')}</b>
        </button>
      </li>
    )
  }
  const core = items.filter((x) => x.core)
  const extras = items.filter((x) => !x.core)
  return (
    <div className="min-w-0 space-y-4">
      {core.length > 0 && (
        <div>
          <h3 className="mb-1 px-3 text-caption font-semibold uppercase tracking-wide text-muted">{t('radar.core')}</h3>
          <ul>{core.map((it) => Row({ item: it }))}</ul>
        </div>
      )}
      {extras.length > 0 && (
        <div>
          <h3 className="mb-1 px-3 text-caption font-semibold uppercase tracking-wide text-muted">{t('radar.extra')}</h3>
          <ul className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-1 2xl:grid-cols-2">
            {extras.map((it) => Row({ item: it, compact: true }))}
          </ul>
        </div>
      )}
    </div>
  )
}

export default function Radar({ items, className }) {
  const { t } = useTranslation()
  const [hover, setHover] = useState(null)
  const [focus, setFocus] = useState(null)
  const [pinned, setPinned] = useState(null)
  const n = items.length
  const activeId = hover ?? focus ?? pinned

  const pts = items.map((it, i) => axisPoint(CX, CY, R, i, n, it.value))
  // 3+ axes form a polygon; with fewer there is no area, so draw the open outline instead.
  const shape = pts.map(([x, y], i) => `${i === 0 ? 'M' : 'L'}${x.toFixed(1)},${y.toFixed(1)}`).join(' ') + (n >= 3 ? ' Z' : '')

  return (
    <figure className={cx('grid min-w-0 items-center gap-6 lg:grid-cols-[minmax(0,1fr)_minmax(0,18rem)]', className)}>
      <div className="min-w-0">
        <svg viewBox={`0 0 ${W} ${H}`} width="100%" role="img" aria-label={t('radar.aria', { n })} className="mx-auto block max-w-xl overflow-visible">
          {RINGS.map((r) =>
            n >= 3 ? (
              <polygon
                key={r}
                points={items.map((_, i) => axisPoint(CX, CY, R, i, n, r).join(',')).join(' ')}
                fill="none"
                stroke={r === 100 ? 'var(--line-strong)' : 'var(--line)'}
                strokeWidth="1"
              />
            ) : (
              <circle key={r} cx={CX} cy={CY} r={(R * r) / 100} fill="none" stroke={r === 100 ? 'var(--line-strong)' : 'var(--line)'} />
            ),
          )}
          {[50, 100].map((r) => (
            <text key={r} x={CX + 4} y={CY - (R * r) / 100 + 11} fontSize="10" fill="var(--faint)" aria-hidden="true">
              {r}
            </text>
          ))}
          {items.map((it, i) => {
            const [x, y] = axisPoint(CX, CY, R, i, n)
            const on = activeId === it.id
            return (
              <line
                key={it.id}
                x1={CX}
                y1={CY}
                x2={x}
                y2={y}
                stroke={on ? it.color : 'var(--line-strong)'}
                strokeWidth={on ? 2.5 : 1}
                opacity={on || activeId == null ? 1 : 0.6}
              />
            )
          })}
          {n > 0 && (
            <g className="radar-grow" style={{ transformOrigin: `${CX}px ${CY}px` }}>
              <path
                d={shape}
                fill={n >= 3 ? 'var(--primary)' : 'none'}
                fillOpacity="0.16"
                stroke="var(--primary)"
                strokeWidth="2"
                strokeLinejoin="round"
              />
              {items.map((it, i) => {
                const on = activeId === it.id
                return (
                  <circle
                    key={it.id}
                    cx={pts[i][0]}
                    cy={pts[i][1]}
                    r={(it.core ? 5.5 : 3.5) + (on ? 2.5 : 0)}
                    fill={it.color}
                    stroke="var(--surface)"
                    strokeWidth="2"
                    onMouseEnter={() => setHover(it.id)}
                    onMouseLeave={() => setHover(null)}
                  >
                    <title>{`${it.label}: ${it.measured ? `${fmt(it.value)}%` : t('radar.none')}`}</title>
                  </circle>
                )
              })}
            </g>
          )}
          {items.map((it, i) => (
            <AxisLabel key={it.id} item={it} i={i} n={n} active={activeId === it.id} dimmed={activeId != null && activeId !== it.id} />
          ))}
        </svg>
        <table className="sr-only">
          <caption>{t('radar.table_caption')}</caption>
          <thead>
            <tr>
              <th scope="col">{t('radar.col_name')}</th>
              <th scope="col">{t('radar.col_value')}</th>
            </tr>
          </thead>
          <tbody>
            {items.map((it) => (
              <tr key={it.id}>
                <th scope="row">{it.label}</th>
                <td>{it.measured ? `${fmt(it.value)}%` : t('radar.none')}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <figcaption className="min-w-0">
        <Legend items={items} activeId={activeId} setHover={setHover} setFocus={setFocus} pinned={pinned} setPinned={setPinned} t={t} />
      </figcaption>
    </figure>
  )
}
