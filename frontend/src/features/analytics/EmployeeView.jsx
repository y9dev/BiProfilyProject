// Employees tab: a searchable picker (also the "no data / outdated data" list) and the selected
// employee's current scores against company and department averages, measurements on the company
// chart and attempt history for the period.
import { useEffect, useMemo, useState } from 'react'
import * as endpoints from '../../api/endpoints'
import { useTranslation } from '../../i18n'
import { cx } from '../../lib/cx'
import { useAsync } from '../../lib/useAsync'
import Async from '../../ui/Async'
import Badge from '../../ui/Badge'
import Button from '../../ui/Button'
import Card from '../../ui/Card'
import EmptyState from '../../ui/EmptyState'
import Input from '../../ui/Input'
import Select from '../../ui/Select'
import StockChart from '../../ui/charts/StockChart'
import DataTable from '../manager/DataTable'
import { almatyToday, charName, formatScore, periodParams } from './analyticsState'
import DepartmentFilter from './DepartmentFilter'
import { Aggregate, Delta, DeptName, Pager, PanelHead } from './parts'
import { formatDay, formatInstant } from './format'

const LIMIT = 50
const DATA_TONE = { ok: 'success', no_data: 'warning', stale: 'neutral' }

function EmployeeList({ state, departments, onPatch }) {
  const { t, lang } = useTranslation()
  const [query, setQuery] = useState(state.search)
  const [offset, setOffset] = useState(0)
  const filterKey = `${state.search}|${state.data}|${state.employeesDept}`

  // Debounce typing into the URL; filters reset paging.
  useEffect(() => {
    if (query === state.search) return undefined
    const id = setTimeout(() => onPatch({ search: query.trim() }, true), 300)
    return () => clearTimeout(id)
  }, [query, state.search, onPatch])
  const [prevKey, setPrevKey] = useState(filterKey)
  if (prevKey !== filterKey) {
    setPrevKey(filterKey)
    setOffset(0)
  }

  const params = {
    search: state.search || undefined,
    data: state.data,
    departmentId: state.employeesDept ?? undefined,
    limit: LIMIT,
    offset,
  }
  const res = useAsync((signal) => endpoints.analytics.employees(params, { signal }), [JSON.stringify(params)])
  const items = res.data?.items ?? []

  const columns = [
    {
      key: 'name',
      header: t('analytics.col.employee'),
      primary: true,
      className: 'w-[32%]',
      render: (u) => (
        <button
          type="button"
          onClick={() => onPatch({ employee: u.id })}
          aria-current={state.employee === u.id ? 'true' : undefined}
          className="min-h-touch text-left font-semibold text-primary underline-offset-2 hover:underline"
        >
          {u.fullName}
          {u.jobTitle && <span className="block text-caption font-normal text-muted">{u.jobTitle}</span>}
        </button>
      ),
    },
    { key: 'dept', header: t('analytics.col.department'), className: 'w-[24%]', render: (u) => <DeptName department={u.department} /> },
    { key: 'att', header: t('analytics.col.attempts'), className: 'w-[12%] tabular-nums', render: (u) => u.attempts },
    {
      key: 'last',
      header: t('analytics.col.last_attempt'),
      className: 'w-[16%]',
      render: (u) => (u.lastAttemptAt ? formatInstant(u.lastAttemptAt, lang) : <span className="text-muted">{'—'}</span>),
    },
    {
      key: 'state',
      header: t('analytics.col.data_state'),
      className: 'w-[16%]',
      render: (u) => <Badge tone={DATA_TONE[u.dataState]}>{t(`analytics.employees.state.${u.dataState}`)}</Badge>,
    },
  ]

  return (
    <Card aria-labelledby="employees-title">
      <PanelHead id="employees-title" title={t('analytics.employees.title')} description={t('analytics.employees.desc')} />
      <div className="mb-4 grid gap-3 md:grid-cols-3">
        <Input
          type="search"
          label={t('analytics.employees.search')}
          value={query}
          maxLength={100}
          onChange={(e) => setQuery(e.target.value)}
          placeholder={t('analytics.employees.search_placeholder')}
        />
        <Select
          label={t('analytics.employees.data_filter')}
          value={state.data}
          onChange={(e) => onPatch({ data: e.target.value }, true)}
          options={['all', 'no_data', 'stale'].map((v) => ({ value: v, label: t(`analytics.employees.filter.${v}`) }))}
        />
        <DepartmentFilter
          departments={departments}
          value={state.employeesDept}
          onChange={(v) => onPatch({ employeesDept: v }, true)}
          className="w-full"
        />
      </div>
      <Async
        status={res.status}
        onRetry={res.reload}
        isEmpty={!items.length}
        empty={{ title: t('analytics.employees.empty_title'), description: t('analytics.employees.empty_desc') }}
      >
        <p className="mb-3 text-label text-muted" role="status">
          {t('analytics.employees.total', { n: res.data?.total ?? 0 })}
        </p>
        <DataTable
          columns={columns}
          rows={items}
          rowKey={(u) => u.id}
          caption={t('analytics.employees.title')}
          rowClassName={(u) => (u.id === state.employee ? 'bg-primary-soft' : undefined)}
        />
        <Pager total={res.data?.total ?? 0} offset={offset} limit={LIMIT} onChange={setOffset} />
      </Async>
    </Card>
  )
}

function EmployeeDetail({ state, employeeId, selection, styles, trend, onClose }) {
  const { t, lang } = useTranslation()
  const params = periodParams(state)
  const res = useAsync(
    (signal) => endpoints.analytics.employee(employeeId, params, { signal }),
    [employeeId, JSON.stringify(params)],
  )
  const d = res.data
  const [showAll, setShowAll] = useState(false)

  const allCurrent = d?.current ?? []
  const current = showAll ? allCurrent : allCurrent.filter((r) => selection.includes(r.characteristic.id))

  // Company lines for the selected characteristics with this employee's measurements as markers.
  const chart = useMemo(() => {
    if (!d || !trend) return null
    const series = trend.series.map((s) => ({
      id: s.characteristic.id,
      label: charName(s.characteristic, lang),
      color: styles.get(s.characteristic.id)?.color ?? 'var(--extra)',
      dashed: styles.get(s.characteristic.id)?.dashed ?? !s.characteristic.core,
      core: s.characteristic.core,
      values: s.values,
    }))
    const ids = new Set(series.map((s) => s.id))
    const markers = d.history.flatMap((h) =>
      h.scores
        .filter((sc) => ids.has(sc.characteristicId))
        .map((sc) => ({ date: almatyToday(new Date(h.completedAt)), seriesId: sc.characteristicId, value: sc.score })),
    )
    return { series, markers }
  }, [d, trend, styles, lang])

  const columns = [
    {
      key: 'char',
      header: t('analytics.col.characteristic'),
      primary: true,
      className: 'w-[24%]',
      render: (r) => charName(r.characteristic, lang),
    },
    {
      key: 'score',
      header: t('analytics.col.score'),
      className: 'w-[16%]',
      render: (r) => (
        <span className="flex flex-col">
          <b className="tabular-nums text-ink">{formatScore(r.score)}</b>
          <span className="text-caption text-muted">{formatInstant(r.measuredAt, lang)}</span>
        </span>
      ),
    },
    { key: 'company', header: t('analytics.col.company_avg'), className: 'w-[22%]', render: (r) => <Aggregate point={r.company} /> },
    { key: 'dept', header: t('analytics.col.department_avg'), className: 'w-[22%]', render: (r) => <Aggregate point={r.department} /> },
    {
      key: 'vs',
      header: t('analytics.col.vs_company'),
      className: 'w-[16%]',
      render: (r) => <Delta value={r.company.value === null ? null : r.score - r.company.value} />,
    },
  ]

  const historyColumns = [
    {
      key: 'title',
      header: t('analytics.col.assessment'),
      primary: true,
      className: 'w-[30%]',
      render: (h) => (
        <span className="flex flex-col">
          <span>{h.assessmentTitle}</span>
          <span className="text-caption font-normal text-muted">{formatInstant(h.completedAt, lang)}</span>
        </span>
      ),
    },
    {
      key: 'scores',
      header: t('analytics.col.scores'),
      className: 'w-[70%]',
      render: (h) => (
        <ul className="flex flex-wrap gap-2">
          {h.scores.map((sc) => (
            <li key={sc.characteristicId} className="inline-flex items-baseline gap-2 rounded-ctl bg-surface-2 px-2 py-1 text-caption">
              <span className="text-muted">{charName(sc, lang)}</span>
              <b className="tabular-nums text-ink">{formatScore(sc.score)}</b>
            </li>
          ))}
        </ul>
      ),
    },
  ]

  return (
    <Card aria-labelledby="employee-title" className="space-y-6">
      {res.status === 'error' && res.error?.status === 404 ? (
        <EmptyState
          title={t('analytics.employee.not_found')}
          action={
            <Button variant="secondary" onClick={onClose}>
              {t('analytics.employee.close')}
            </Button>
          }
        />
      ) : (
      <Async status={res.status} onRetry={res.reload}>
        {d && (
          <>
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div className="min-w-0 flex-1 basis-56">
                <h2 id="employee-title" className="text-title font-semibold text-ink">
                  {d.user.fullName}
                </h2>
                <p className="mt-1 flex flex-wrap items-center gap-x-3 gap-y-1 text-label text-muted">
                  {d.user.jobTitle && <span>{d.user.jobTitle}</span>}
                  <DeptName department={d.user.department} />
                  {d.user.status === 'blocked' && <Badge tone="danger">{t('analytics.employee.blocked')}</Badge>}
                </p>
                <p className="mt-1 text-caption text-muted">
                  {d.user.lastAttemptAt
                    ? t('analytics.employee.last_attempt', { date: formatInstant(d.user.lastAttemptAt, lang) })
                    : t('analytics.employee.no_attempts')}
                </p>
              </div>
              <Button variant="secondary" onClick={onClose}>
                {t('analytics.employee.close')}
              </Button>
            </div>

            <section aria-labelledby="employee-current">
              <div className="mb-3 flex flex-wrap items-baseline justify-between gap-2">
                <h3 id="employee-current" className="text-body font-semibold text-ink">
                  {t('analytics.employee.current', { date: formatDay(d.asOf, lang) })}
                </h3>
                {d.current.length > 0 && (
                  <Button variant="ghost" aria-pressed={showAll} onClick={() => setShowAll((v) => !v)}>
                    {t(showAll ? 'analytics.employee.only_selected' : 'analytics.employee.show_all', { n: d.current.length })}
                  </Button>
                )}
              </div>
              {current.length ? (
                <DataTable columns={columns} rows={current} rowKey={(r) => r.characteristic.id} caption={t('analytics.employee.current_caption')} />
              ) : (
                <p className="text-label text-muted">
                  {d.current.length ? t('analytics.employee.none_selected') : t('analytics.employee.no_scores')}
                </p>
              )}
            </section>

            {chart && chart.markers.length > 0 && trend.points.length > 0 && (
              <section aria-labelledby="employee-chart">
                <h3 id="employee-chart" className="mb-1 text-body font-semibold text-ink">
                  {t('analytics.employee.chart')}
                </h3>
                <p className="mb-3 text-caption text-muted">{t('analytics.employee.chart_hint')}</p>
                <StockChart
                  points={trend.points}
                  granularity={trend.period.granularity}
                  series={chart.series}
                  insufficientStyle="dim"
                  markers={chart.markers}
                  lang={lang}
                />
              </section>
            )}

            <section aria-labelledby="employee-history">
              <h3 id="employee-history" className="mb-3 text-body font-semibold text-ink">
                {t('analytics.employee.history')}
              </h3>
              {d.history.length ? (
                <DataTable columns={historyColumns} rows={d.history} rowKey={(h) => h.attemptId} caption={t('analytics.employee.history')} />
              ) : (
                <p className="text-label text-muted">{t('stats.no_data_period')}</p>
              )}
            </section>
          </>
        )}
      </Async>
      )}
    </Card>
  )
}

export default function EmployeeView({ state, departments, selection, styles, trend, onPatch }) {
  const { t } = useTranslation()
  return (
    <div className={cx('grid min-w-0 gap-6', state.employee && '2xl:grid-cols-[minmax(0,5fr)_minmax(0,7fr)]')}>
      {state.employee ? (
        <div className="min-w-0 2xl:order-2">
          <EmployeeDetail
            key={state.employee}
            state={state}
            employeeId={state.employee}
            selection={selection}
            styles={styles}
            trend={trend}
            onClose={() => onPatch({ employee: null })}
          />
        </div>
      ) : (
        <Card>
          <EmptyState title={t('analytics.employee.pick_title')} description={t('analytics.employee.pick_desc')} />
        </Card>
      )}
      <div className="min-w-0 2xl:order-1">
        <EmployeeList state={state} departments={departments} onPatch={onPatch} />
      </div>
    </div>
  )
}
