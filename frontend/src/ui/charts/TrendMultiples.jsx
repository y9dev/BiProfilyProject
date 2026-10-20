// Small multiples: one compact line chart per characteristic, all on the same 0-100 scale,
// each with its latest value and the change vs the previous measurement.
// The change is printed as arrow + words + number, never by colour alone.
// items: [{ id, label, color, core, series: [{ v }], latest, delta, direction: 'up'|'down'|'same'|'none' }]
import { useTranslation } from '../../i18n'
import { cx } from '../../lib/cx'

const fmt = (v) => `${Math.round(v * 10) / 10}`

const TONE = { up: 'text-success', down: 'text-danger', same: 'text-muted', none: 'text-muted' }
const ARROW = {
  up: 'M12 19V5M5 12l7-7 7 7',
  down: 'M12 5v14M19 12l-7 7-7-7',
  same: 'M5 12h14',
  none: 'M8 12h8',
}

export function DeltaBadge({ direction, delta, compact }) {
  const { t } = useTranslation()
  const text =
    direction === 'none'
      ? t('trends.single')
      : direction === 'same'
        ? t('trends.same')
        : t(direction === 'up' ? 'trends.up' : 'trends.down', { n: fmt(Math.abs(delta)) })
  return (
    <span className={cx('inline-flex items-center gap-1 font-semibold', compact ? 'text-caption' : 'text-label', TONE[direction])}>
      <svg
        width={compact ? 12 : 14}
        height={compact ? 12 : 14}
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="2.5"
        strokeLinecap="round"
        strokeLinejoin="round"
        aria-hidden="true"
      >
        <path d={ARROW[direction]} />
      </svg>
      {text}
    </span>
  )
}

function Line({ series, color, h }) {
  const W = 200
  const pad = 6
  const n = series.length
  const x = (i) => (n === 1 ? W / 2 : pad + (i * (W - pad * 2)) / (n - 1))
  const y = (v) => h - pad - (Math.min(Math.max(v, 0), 100) / 100) * (h - pad * 2)
  const d = series.map((p, i) => `${i === 0 ? 'M' : 'L'}${x(i).toFixed(1)},${y(p.v).toFixed(1)}`).join(' ')
  return (
    <svg viewBox={`0 0 ${W} ${h}`} width="100%" aria-hidden="true" className="block overflow-visible">
      <line x1={pad} y1={y(0)} x2={W - pad} y2={y(0)} stroke="var(--line)" strokeWidth="1" />
      <line x1={pad} y1={y(50)} x2={W - pad} y2={y(50)} stroke="var(--line)" strokeWidth="1" strokeDasharray="3 4" />
      <line x1={pad} y1={y(100)} x2={W - pad} y2={y(100)} stroke="var(--line)" strokeWidth="1" />
      {n > 1 && <path d={d} fill="none" stroke={color} strokeWidth="2.25" strokeLinejoin="round" strokeLinecap="round" />}
      {series.map((p, i) => (
        <circle
          key={i}
          cx={x(i)}
          cy={y(p.v)}
          r={i === n - 1 ? 4 : 2.5}
          fill={i === n - 1 ? color : 'var(--surface)'}
          stroke={color}
          strokeWidth="1.75"
        >
          <title>{`${fmt(p.v)}%`}</title>
        </circle>
      ))}
    </svg>
  )
}

function Tile({ item, compact }) {
  const { t } = useTranslation()
  const values = item.series.map((p) => fmt(p.v)).join(' → ')
  return (
    <li
      className={cx('min-w-0 rounded-card border border-line', compact ? 'p-3' : 'p-4')}
      style={item.core ? { borderTopColor: item.color, borderTopWidth: 3 } : undefined}
    >
      <div className="flex items-baseline justify-between gap-2">
        <h4 className={cx('min-w-0 break-words font-semibold', compact ? 'text-caption text-muted' : 'text-label text-ink')}>{item.label}</h4>
        <b className={cx('shrink-0 tabular-nums text-ink', compact ? 'text-label' : 'text-title')}>
          {item.latest == null ? '–' : `${fmt(item.latest)}%`}
        </b>
      </div>
      <div className={compact ? 'mt-0.5' : 'mt-1'}>
        <DeltaBadge direction={item.direction} delta={item.delta} compact={compact} />
      </div>
      <div
        className={compact ? 'mt-2' : 'mt-3'}
        role="img"
        aria-label={item.series.length ? t('trends.series_aria', { label: item.label, values }) : t('trends.no_data_aria', { label: item.label })}
      >
        {item.series.length > 0 ? (
          <Line series={item.series} color={item.color} h={compact ? 34 : 64} />
        ) : (
          <p className="py-2 text-caption text-muted">{t('trends.not_measured')}</p>
        )}
      </div>
    </li>
  )
}

export default function TrendMultiples({ items }) {
  const core = items.filter((x) => x.core)
  const extras = items.filter((x) => !x.core)
  return (
    <div className="space-y-6">
      {core.length > 0 && (
        <ul className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
          {core.map((it) => (
            <Tile key={it.id} item={it} />
          ))}
        </ul>
      )}
      {extras.length > 0 && (
        <ul className="grid grid-cols-1 gap-3 xs:grid-cols-2 md:grid-cols-3 xl:grid-cols-4 2xl:grid-cols-6">
          {extras.map((it) => (
            <Tile key={it.id} item={it} compact />
          ))}
        </ul>
      )}
    </div>
  )
}
