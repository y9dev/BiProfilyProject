// Department comparison: pick 1-8 departments (or "No department"); one chart per selected
// characteristic with the company as a reference line, then a table of current averages with n and
// the difference from the company. Groups below the minimum sample are never drawn or ranked.
import { useEffect } from 'react'
import { fieldErrors } from '../../api/client'
import * as endpoints from '../../api/endpoints'
import { useTranslation } from '../../i18n'
import { cx } from '../../lib/cx'
import { useAsync } from '../../lib/useAsync'
import Async from '../../ui/Async'
import Card from '../../ui/Card'
import EmptyState from '../../ui/EmptyState'
import StockChart from '../../ui/charts/StockChart'
import DataTable from '../manager/DataTable'
import { MAX_DEPARTMENTS, MIN_SAMPLE, NO_DEPARTMENT, knownDepartments, charName, sharedParams, sharedKey } from './analyticsState'
import { LineSwatch } from './CharacteristicPicker'
import { Aggregate, Delta, DeptName, PanelHead } from './parts'
import { deptKey } from './format'

// Ten distinct line colours; department lines are solid, the company reference is drawn by the chart.
const GROUP_COLORS = ['--core-1', '--core-2', '--core-3', '--core-4', '--extra-1', '--extra-2', '--extra-3', '--extra-4'].map(
  (v) => `var(${v})`,
)

function DepartmentToggles({ departments, selected, onToggle }) {
  const { t } = useTranslation()
  const atCap = selected.length >= MAX_DEPARTMENTS
  const options = [{ id: NO_DEPARTMENT, name: t('stats.no_department'), active: true }, ...(departments || [])]
  return (
    <fieldset className="min-w-0">
      <legend className="mb-2 flex w-full flex-wrap items-baseline justify-between gap-2 text-label font-medium text-ink">
        <span>{t('analytics.departments.pick')}</span>
        <span className="text-caption font-normal tabular-nums text-muted">
          {t('analytics.departments.count', { n: selected.length, max: MAX_DEPARTMENTS })}
        </span>
      </legend>
      <div className="flex flex-wrap gap-2">
        {options.map((d) => {
          const on = selected.includes(d.id)
          const idx = selected.indexOf(d.id)
          return (
            <button
              key={d.id}
              type="button"
              aria-pressed={on}
              disabled={!on && atCap}
              onClick={() => onToggle(d.id)}
              className={cx(
                'inline-flex min-h-touch max-w-full items-center gap-2 rounded-ctl border px-3 py-1 text-label transition-colors',
                'disabled:cursor-not-allowed disabled:opacity-50',
                on ? 'border-primary bg-primary-soft font-semibold text-primary' : 'border-line bg-surface text-ink hover:bg-surface-2',
              )}
            >
              {on && <LineSwatch color={GROUP_COLORS[idx % GROUP_COLORS.length]} />}
              <span className="min-w-0 break-words">{d.name}</span>
              {!d.active && <span className="text-caption font-normal text-muted">({t('stats.department_inactive')})</span>}
            </button>
          )
        })}
      </div>
      {atCap && <p className="mt-2 text-caption text-muted">{t('stats.validation.count')}</p>}
    </fieldset>
  )
}

function Block({ block, points, granularity, colorFor }) {
  const { t, lang } = useTranslation()
  const name = charName(block.characteristic, lang)
  const groupLabel = (dep) => (dep ? dep.name : t('stats.no_department'))
  const series = block.departments.map((row) => ({
    id: deptKey(row.department),
    label: groupLabel(row.department),
    color: colorFor(row.department),
    dashed: false,
    core: true,
    values: row.values,
  }))
  const company = block.company.values[block.company.values.length - 1]

  const columns = [
    {
      key: 'dept',
      header: t('analytics.col.department'),
      primary: true,
      className: 'w-[30%]',
      render: (r) => (
        <span className="inline-flex min-w-0 items-center gap-2">
          <LineSwatch color={colorFor(r.department)} />
          <DeptName department={r.department} />
        </span>
      ),
    },
    { key: 'cur', header: t('analytics.col.current'), className: 'w-[26%]', render: (r) => <Aggregate point={r.current} /> },
    {
      key: 'vs',
      header: t('analytics.col.vs_company'),
      className: 'w-[16%]',
      render: (r) => <Delta value={r.current.vsCompany} />,
    },
    { key: 'emp', header: t('analytics.col.employees'), className: 'w-[14%] tabular-nums', render: (r) => r.employees },
    { key: 'att', header: t('analytics.col.attempts_period'), className: 'w-[14%] tabular-nums', render: (r) => r.attemptsInPeriod },
  ]

  return (
    <Card aria-labelledby={`dep-block-${block.characteristic.id}`} className="space-y-4">
      <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
        <h3 id={`dep-block-${block.characteristic.id}`} className="min-w-0 text-title font-semibold text-ink">
          {name}
        </h3>
        <span className="flex flex-wrap items-baseline gap-2 text-label text-muted">
          <span>{t('analytics.departments.company_now')}</span>
          <Aggregate point={company} inline />
        </span>
      </div>
      <StockChart
        points={points}
        granularity={granularity}
        series={series}
        insufficientStyle="hidden"
        reference={{ label: t('stats.company'), values: block.company.values }}
        lang={lang}
      />
      <DataTable columns={columns} rows={block.departments} rowKey={(r) => deptKey(r.department)} caption={name} />
    </Card>
  )
}

export default function DepartmentsView({ state, departments, known, onToggleDepartment, onReplace }) {
  const { t } = useTranslation()
  // A deleted department id in a saved link must not dead-end the tab: compare only known ids.
  const selected = knownDepartments(state.departments, known)
  const stale = selected.length !== state.departments.length
  const waiting = known === undefined
  const key = `${sharedKey(state)}|${selected.join(',')}|${waiting}`
  const res = useAsync(
    (signal) =>
      selected.length && !waiting
        ? endpoints.analytics.departments({ ...sharedParams(state), departments: selected }, { signal })
        : Promise.resolve(null),
    [key],
  )
  const colorFor = (dep) => {
    const i = selected.indexOf(dep ? dep.id : NO_DEPARTMENT)
    return GROUP_COLORS[(i < 0 ? 0 : i) % GROUP_COLORS.length]
  }
  const d = res.data

  // 422 on `departments` (deleted meanwhile): keep the known subset, or start over if none is left.
  const rejected = res.status === 'error' && !!fieldErrors(res.error).departments
  useEffect(() => {
    if (stale) onReplace(selected)
    else if (rejected) onReplace([])
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [stale, rejected])

  return (
    <div className="space-y-6">
      <Card aria-labelledby="departments-title">
        <PanelHead id="departments-title" title={t('analytics.departments.title')} description={t('analytics.departments.desc')} />
        <DepartmentToggles departments={departments} selected={selected} onToggle={onToggleDepartment} />
        <p className="mt-3 text-caption text-muted">{t('stats.insufficient_hint', { min: MIN_SAMPLE })}</p>
      </Card>

      {!selected.length ? (
        <Card>
          <EmptyState title={t('analytics.departments.empty_title')} description={t('analytics.departments.empty_desc')} />
        </Card>
      ) : (
        <Async status={waiting ? 'loading' : res.status} onRetry={res.reload}>
          {d && (
            <div className="space-y-6">
              {d.blocks.length ? (
                d.blocks.map((b) => (
                  <Block key={b.characteristic.id} block={b} points={d.points} granularity={d.period.granularity} colorFor={colorFor} />
                ))
              ) : (
                <Card>
                  <EmptyState title={t('stats.no_data')} description={t('stats.no_data_period')} />
                </Card>
              )}
            </div>
          )}
        </Async>
      )}
    </div>
  )
}
