// Data coverage per characteristic in the period (from the trend catalog): employees measured,
// attempts, and whether the minimum sample is met. Explains why a characteristic is not a default.
import { useTranslation } from '../../i18n'
import Badge from '../../ui/Badge'
import Card from '../../ui/Card'
import EmptyState from '../../ui/EmptyState'
import DataTable from '../manager/DataTable'
import { charName, formatRate } from './analyticsState'
import { PanelHead, RateBar, Tile } from './parts'

export default function DataCoveragePanel({ trend, selection, defaultIds }) {
  const { t, lang } = useTranslation()
  if (!trend) return null
  const total = trend.summary.employees
  const rows = [...trend.characteristics].sort((a, b) => b.employees - a.employees || b.attempts - a.attempts || a.id - b.id)

  const columns = [
    {
      key: 'name',
      header: t('analytics.col.characteristic'),
      primary: true,
      className: 'w-[30%]',
      render: (c) => (
        <span className="inline-flex min-w-0 flex-wrap items-center gap-2">
          <span className="break-words">{charName(c, lang)}</span>
          {c.core && <Badge tone="info">{t('analytics.coverage.core')}</Badge>}
          {defaultIds.includes(c.id) && <Badge>{t('analytics.coverage.default')}</Badge>}
          {selection.includes(c.id) && <Badge tone="success">{t('analytics.coverage.selected')}</Badge>}
        </span>
      ),
    },
    {
      key: 'emp',
      header: t('analytics.col.measured'),
      className: 'w-[30%]',
      render: (c) => (
        <span className="flex min-w-0 items-center gap-3">
          <span className="w-16 shrink-0 tabular-nums">
            {c.employees}
            <span className="text-muted">{` / ${total}`}</span>
          </span>
          <RateBar rate={total ? c.employees / total : 0} />
        </span>
      ),
    },
    { key: 'att', header: t('analytics.col.attempts'), className: 'w-[15%] tabular-nums', render: (c) => c.attempts },
    {
      key: 'ok',
      header: t('analytics.col.sample_ok'),
      className: 'w-[25%]',
      render: (c) =>
        c.eligible ? (
          <Badge tone="success">{t('analytics.coverage.enough')}</Badge>
        ) : (
          <Badge tone="warning">{t('stats.insufficient', { n: c.employees })}</Badge>
        ),
    },
  ]

  return (
    <Card aria-labelledby="coverage-title">
      <PanelHead id="coverage-title" title={t('analytics.coverage.title')} description={t('analytics.coverage.desc')} />
      <ul className="mb-6 grid grid-cols-2 gap-3 lg:grid-cols-3">
        <Tile label={t('analytics.coverage.population')} value={total} />
        <Tile
          label={t('analytics.coverage.measured')}
          value={trend.summary.measuredEmployees}
          note={total ? formatRate(trend.summary.measuredEmployees / total) : undefined}
        />
        <Tile label={t('analytics.coverage.attempts')} value={trend.summary.attemptsInPeriod} />
      </ul>
      {rows.length ? (
        <DataTable columns={columns} rows={rows} rowKey={(c) => c.id} caption={t('analytics.coverage.title')} />
      ) : (
        <EmptyState title={t('stats.no_data')} description={t('stats.no_data_period')} />
      )}
    </Card>
  )
}
