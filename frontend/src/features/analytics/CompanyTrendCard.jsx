// Company trend card: period control + StockChart over a TrendResponse
// (/api/dashboard/trend or /api/analytics/trend).
// FROZEN PROP CONTRACT (plan section 7, Step 1): stream C implements the card behind
// these props; stream D uses it on /analytics. Any change goes back through stream 0.
// The period control renders only when onPeriodChange is given (the analytics page has its own filter).
// DashboardTrendCard (below) is the manager/admin dashboard container: own period state and fetch.

/**
 * @typedef {Object} CompanyTrendCardProps
 * @property {Object|null} data            TrendResponse (contracts/golden/dashboard_trend.json) or null while loading
 * @property {boolean} loading
 * @property {unknown} error
 * @property {'dashboard'|'analytics'} mode
 * @property {string} period               '7d'|'30d'|'90d'|'12m'|'all'|'custom'
 * @property {(period: string) => void} onPeriodChange
 * @property {string[]} presets            dashboard: 7d/30d/90d/12m/all
 * @property {boolean} allowCustom         analytics only
 * @property {string} [from]               YYYY-MM-DD, with allowCustom
 * @property {string} [to]                 YYYY-MM-DD, with allowCustom
 * @property {(range: { from: string, to: string }) => void} [onRangeChange]   analytics only
 * @property {(characteristic: { id: number, name: string, nameI18n: Object|null, core: boolean }) => { color: string, dashed: boolean }} seriesStyle
 *           core solid --core-1..4, extras dashed --extra-1..6
 * @property {string} title
 */

import { useMemo, useState } from 'react'
import * as endpoints from '../../api/endpoints'
import { useTranslation } from '../../i18n'
import { cx } from '../../lib/cx'
import { useAsync } from '../../lib/useAsync'
import Card, { CardTitle } from '../../ui/Card'
import ErrorState from '../../ui/ErrorState'
import Skeleton from '../../ui/Skeleton'
import StockChart from '../../ui/charts/StockChart'
import { paletteFor, parseDay } from '../../ui/charts/stockGeometry'
import { charName } from '../employee/format'

const DASHBOARD_PRESETS = ['7d', '30d', '90d', '12m', 'all']
const DEFAULT_PERIOD = '90d'

function PeriodControl({ period, presets, allowCustom, onPeriodChange }) {
  const { t } = useTranslation()
  const options = allowCustom ? [...presets, 'custom'] : presets
  return (
    <div className="-mx-1 max-w-full overflow-x-auto px-1 pb-1 sm:pb-0">
      <div role="group" aria-label={t('stats.period.label')} className="inline-flex shrink-0 gap-0.5 rounded-ctl border border-line bg-surface-2 p-0.5">
        {options.map((p) => {
          const on = p === period
          return (
            <button
              key={p}
              type="button"
              aria-pressed={on}
              onClick={() => !on && onPeriodChange(p)}
              className={cx(
                'min-h-touch whitespace-nowrap rounded-[calc(var(--radius-ctl)-2px)] px-3 text-label sm:min-h-9',
                on ? 'bg-surface font-semibold text-ink shadow-[0_1px_2px_rgb(0_0_0/0.08)]' : 'text-muted hover:text-ink',
              )}
            >
              {t(`stats.period.${p}`)}
            </button>
          )
        })}
      </div>
    </div>
  )
}

function RangeInputs({ from, to, onRangeChange }) {
  const { t } = useTranslation()
  const [draft, setDraft] = useState({ from: from || '', to: to || '' })
  const set = (k) => (e) => {
    const next = { ...draft, [k]: e.target.value }
    setDraft(next)
    if (next.from && next.to) onRangeChange(next)
  }
  const field = 'min-h-touch rounded-ctl border border-line-strong bg-surface px-2 text-label text-ink sm:min-h-9'
  return (
    <div className="flex flex-wrap items-center gap-2">
      <label className="flex items-center gap-2 text-caption text-muted">
        {t('stats.period.from')}
        <input type="date" value={draft.from} max={draft.to || undefined} onChange={set('from')} className={field} />
      </label>
      <label className="flex items-center gap-2 text-caption text-muted">
        {t('stats.period.to')}
        <input type="date" value={draft.to} min={draft.from || undefined} onChange={set('to')} className={field} />
      </label>
    </div>
  )
}

function useLongDate() {
  const { t } = useTranslation()
  return (s) => {
    const p = parseDay(s)
    if (!p) return s || ''
    const months = t('dashboard.chart.months_gen').split('|')
    return t('dashboard.chart.day_full', { d: p.d, m: months[p.m], y: p.y })
  }
}

function Meta({ children }) {
  return <span className="inline-flex items-center gap-1.5 whitespace-nowrap">{children}</span>
}

/** @param {CompanyTrendCardProps} props */
export default function CompanyTrendCard({
  data,
  loading,
  error,
  period,
  onPeriodChange,
  presets = DASHBOARD_PRESETS,
  allowCustom = false,
  from,
  to,
  onRangeChange,
  seriesStyle,
  title,
}) {
  const { t, lang } = useTranslation()
  const longDate = useLongDate()

  const series = useMemo(() => {
    if (!data?.series) return []
    const style = seriesStyle || paletteFor(data.series.map((s) => s.characteristic))
    return [...data.series]
      .sort((a, b) => Number(b.characteristic.core) - Number(a.characteristic.core) || (a.characteristic.core ? a.characteristic.id - b.characteristic.id : 0))
      .map((s) => ({
        id: s.characteristic.id,
        label: charName(s.characteristic, lang),
        core: !!s.characteristic.core,
        values: s.values,
        ...style(s.characteristic),
      }))
  }, [data, seriesStyle, lang])

  const minSample = data?.minSample ?? 5
  const hasDim = series.some((s) => s.values.some((v) => v.insufficient && v.value != null))
  const hasWithheld = series.some((s) => s.values.some((v) => v.insufficient && v.value == null && v.employees > 0))
  const timeout = error && (error.code === 'analytics_timeout' || error.status === 503)

  return (
    <Card aria-labelledby="company-trend-title" aria-busy={loading || undefined} className="flex min-w-0 flex-col gap-4">
      <div className="flex min-w-0 flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
        <div className="min-w-0">
          <CardTitle id="company-trend-title">{title}</CardTitle>
          <p className="mt-1 text-label text-muted">{t('dashboard.trend.desc')}</p>
        </div>
        {onPeriodChange && (
          <div className="flex min-w-0 flex-col gap-2 lg:items-end">
            <PeriodControl period={period} presets={presets} allowCustom={allowCustom} onPeriodChange={onPeriodChange} />
            {allowCustom && period === 'custom' && onRangeChange && <RangeInputs from={from} to={to} onRangeChange={onRangeChange} />}
          </div>
        )}
      </div>

      {data && (
        <div className="flex min-w-0 flex-wrap gap-x-4 gap-y-1 border-y border-line py-2 text-caption text-muted">
          <Meta>
            <b className="font-semibold text-ink">{t('stats.period.range', { from: longDate(data.period.from), to: longDate(data.period.to) })}</b>
          </Meta>
          <Meta>{t(`stats.granularity.${data.period.granularity}`)}</Meta>
          {data.summary && (
            <>
              <Meta>
                <span className="tabular-nums">{t('dashboard.trend.measured', { measured: data.summary.measuredEmployees, total: data.summary.employees })}</span>
              </Meta>
              <Meta>
                <span className="tabular-nums">{t('dashboard.trend.attempts', { n: data.summary.attemptsInPeriod })}</span>
              </Meta>
            </>
          )}
        </div>
      )}

      {error ? (
        <ErrorState description={timeout ? t('stats.errors.timeout') : undefined} />
      ) : !data ? (
        <div aria-busy="true">
          <Skeleton className="h-72 w-full md:h-96" />
        </div>
      ) : (
        <div className={cx('min-w-0 transition-opacity', loading && 'opacity-50')}>
          <StockChart
            points={data.points}
            granularity={data.period.granularity}
            series={series}
            insufficientStyle="dim"
            lang={lang}
          />
        </div>
      )}

      {data && (
        <div className="space-y-1 text-caption text-faint">
          {hasDim && <p>{t('dashboard.trend.dim_note', { min: minSample })}</p>}
          {hasWithheld && <p>{t('stats.insufficient_hint', { min: minSample })}</p>}
          <p>
            {t('stats.disclaimer')} {t('stats.timezone_note')}
          </p>
        </div>
      )}
    </Card>
  )
}

// Manager/admin dashboard: presets 7d/30d/90d/12m/all (default 90d, no custom range), own fetch so the
// personal profile load is never blocked by the company query.
export function DashboardTrendCard() {
  const { t } = useTranslation()
  const [period, setPeriod] = useState(DEFAULT_PERIOD)
  const { status, data, error } = useAsync((signal) => endpoints.dashboard.trend({ period }, { signal }), [period])
  const seriesStyle = useMemo(() => paletteFor((data?.series || []).map((s) => s.characteristic)), [data])
  return (
    <CompanyTrendCard
      data={data}
      loading={status === 'loading'}
      error={status === 'error' ? error : null}
      mode="dashboard"
      period={period}
      onPeriodChange={setPeriod}
      presets={DASHBOARD_PRESETS}
      allowCustom={false}
      seriesStyle={seriesStyle}
      title={t('dashboard.trend.title')}
    />
  )
}
