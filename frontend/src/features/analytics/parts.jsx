// Small presentational pieces shared by the /analytics panels.
import { useTranslation } from '../../i18n'
import { cx } from '../../lib/cx'
import Badge from '../../ui/Badge'
import Button from '../../ui/Button'
import { formatDelta, formatScore, pageInfo } from './analyticsState'

// "n = 12 · 30" style sample size: employees, then attempts. Always shown beside an aggregate.
export function Sample({ employees, attempts, className }) {
  const { t } = useTranslation()
  const parts = [t('analytics.sample.employees', { n: employees ?? 0 })]
  if (attempts !== undefined && attempts !== null) parts.push(t('analytics.sample.attempts', { n: attempts }))
  return <span className={cx('text-caption tabular-nums text-muted', className)}>{parts.join(' · ')}</span>
}

// A ValuePoint as "63 %" plus n, or the "not enough data (n = 3)" note when below the minimum sample.
export function Aggregate({ point, inline = false }) {
  const { t } = useTranslation()
  if (!point) return <span className="text-muted">{t('stats.no_data')}</span>
  const missing = point.value === null || point.value === undefined
  return (
    <span className={cx('flex min-w-0 gap-x-2', inline ? 'flex-wrap items-baseline' : 'flex-col')}>
      {missing ? (
        <span className="text-label text-muted">
          {point.employees ? t('stats.insufficient', { n: point.employees }) : t('stats.no_data')}
        </span>
      ) : (
        <span className={cx('font-semibold tabular-nums text-ink', point.insufficient && 'opacity-60')}>
          {formatScore(point.value)}
          <span className="font-normal text-muted"> / 100</span>
        </span>
      )}
      {!missing || point.employees ? <Sample employees={point.employees} attempts={point.attempts} /> : null}
    </span>
  )
}

export function Delta({ value, className }) {
  if (value === null || value === undefined) return <span className={cx('text-muted', className)}>{'—'}</span>
  const tone = value > 0 ? 'text-success' : value < 0 ? 'text-danger' : 'text-muted'
  return <span className={cx('font-semibold tabular-nums', tone, className)}>{formatDelta(value)}</span>
}

export function DeptName({ department }) {
  const { t } = useTranslation()
  if (!department) return <span className="italic text-muted">{t('stats.no_department')}</span>
  return (
    <span className="inline-flex min-w-0 flex-wrap items-center gap-2">
      <span className="break-words">{department.name}</span>
      {department.active === false && <Badge>{t('stats.department_inactive')}</Badge>}
    </span>
  )
}

// KPI tile: big tabular number, label, optional footnote.
export function Tile({ label, value, note, tone }) {
  return (
    <li className="min-w-0 rounded-card bg-surface-2 px-4 py-3">
      <p className="text-caption font-medium uppercase tracking-wide text-muted">{label}</p>
      <p className={cx('mt-1 text-display font-bold leading-none tabular-nums', tone || 'text-ink')}>{value}</p>
      {note && <p className="mt-1 text-caption text-muted">{note}</p>}
    </li>
  )
}

// Horizontal completion bar (decorative; the number beside it carries the meaning).
export function RateBar({ rate }) {
  const pct = rate === null || rate === undefined ? 0 : Math.max(0, Math.min(1, rate)) * 100
  return (
    <span aria-hidden="true" className="block h-2 w-full overflow-hidden rounded-full bg-surface-2">
      <span className="block h-full rounded-full bg-primary transition-[width] duration-500" style={{ width: `${pct}%` }} />
    </span>
  )
}

export function Pager({ total, offset, limit, onChange }) {
  const { t } = useTranslation()
  const { page, pages, hasPrev, hasNext } = pageInfo(total, offset, limit)
  if (pages <= 1) return null
  return (
    <nav aria-label={t('analytics.pager.label')} className="mt-4 flex flex-wrap items-center justify-between gap-3">
      <span className="text-label text-muted">{t('analytics.pager.page', { page, pages })}</span>
      <span className="flex gap-2">
        <Button variant="secondary" disabled={!hasPrev} onClick={() => onChange(Math.max(0, offset - limit))}>
          {t('analytics.pager.prev')}
        </Button>
        <Button variant="secondary" disabled={!hasNext} onClick={() => onChange(offset + limit)}>
          {t('analytics.pager.next')}
        </Button>
      </span>
    </nav>
  )
}

// Section heading inside a card: title + optional description + right-aligned actions.
export function PanelHead({ id, title, description, actions }) {
  return (
    <div className="mb-4 flex flex-wrap items-start justify-between gap-x-4 gap-y-3">
      <div className="min-w-0 flex-1 basis-56">
        <h2 id={id} className="text-title font-semibold text-ink">
          {title}
        </h2>
        {description && <p className="mt-1 text-label text-muted">{description}</p>}
      </div>
      {actions && <div className="flex min-w-0 flex-wrap items-end gap-3">{actions}</div>}
    </div>
  )
}
