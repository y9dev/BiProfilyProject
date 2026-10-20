// Participation and completion for the selected period: company KPIs, per-department rows
// (groups below the minimum sample show "not enough data") and per-test completion.
import * as endpoints from '../../api/endpoints'
import { useTranslation } from '../../i18n'
import { useAsync } from '../../lib/useAsync'
import Async from '../../ui/Async'
import Card from '../../ui/Card'
import DataTable from '../manager/DataTable'
import { formatRate, periodParams } from './analyticsState'
import DepartmentFilter from './DepartmentFilter'
import { DeptName, PanelHead, RateBar, Tile } from './parts'
import { deptKey } from './format'

export default function ParticipationPanel({ state, departments, onDepartment }) {
  const { t } = useTranslation()
  const params = { ...periodParams(state), departmentId: state.participationDept ?? undefined }
  const key = JSON.stringify(params)
  const res = useAsync((signal) => endpoints.analytics.participation(params, { signal }), [key])
  const d = res.data

  const rate = (row) =>
    row.rate === null || row.rate === undefined ? (
      <span className="text-muted">{t('stats.insufficient', { n: row.employees })}</span>
    ) : (
      <span className="flex min-w-0 items-center gap-3">
        <span className="w-12 shrink-0 font-semibold tabular-nums">{formatRate(row.rate)}</span>
        <RateBar rate={row.rate} />
      </span>
    )

  const columns = [
    { key: 'dept', header: t('analytics.col.department'), primary: true, className: 'w-[34%]', render: (r) => <DeptName department={r.department} /> },
    { key: 'emp', header: t('analytics.col.employees'), className: 'w-[14%] tabular-nums', render: (r) => r.employees },
    { key: 'part', header: t('analytics.col.participated'), className: 'w-[14%] tabular-nums', render: (r) => r.participated },
    { key: 'att', header: t('analytics.col.attempts'), className: 'w-[14%] tabular-nums', render: (r) => r.attempts },
    { key: 'rate', header: t('analytics.col.rate'), className: 'w-[24%]', render: rate },
  ]

  return (
    <Card aria-labelledby="participation-title">
      <PanelHead
        id="participation-title"
        title={t('analytics.participation.title')}
        description={t('analytics.participation.desc')}
        actions={<DepartmentFilter departments={departments} value={state.participationDept} onChange={onDepartment} />}
      />
      <Async status={res.status} onRetry={res.reload}>
        {d && (
          <div className="space-y-6">
            <ul className="grid grid-cols-2 gap-3 lg:grid-cols-4">
              <Tile label={t('analytics.participation.employees')} value={d.company.employees} />
              <Tile label={t('analytics.participation.participated')} value={d.company.participated} />
              <Tile
                label={t('analytics.participation.rate')}
                value={d.company.rate === null ? '—' : formatRate(d.company.rate)}
                note={d.company.rate === null ? t('stats.insufficient', { n: d.company.employees }) : undefined}
              />
              <Tile label={t('analytics.participation.attempts')} value={d.company.attempts} />
            </ul>

            {state.participationDept === null && (
              <section aria-labelledby="participation-depts">
                <h3 id="participation-depts" className="mb-3 text-body font-semibold text-ink">
                  {t('analytics.participation.by_department')}
                </h3>
                {d.departments.length ? (
                  <DataTable
                    columns={columns}
                    rows={d.departments}
                    rowKey={(r) => deptKey(r.department)}
                    caption={t('analytics.participation.by_department')}
                  />
                ) : (
                  <p className="text-label text-muted">{t('stats.no_data_period')}</p>
                )}
              </section>
            )}

            <section aria-labelledby="participation-tests">
              <h3 id="participation-tests" className="mb-3 text-body font-semibold text-ink">
                {t('analytics.participation.by_test')}
              </h3>
              {d.tests.length ? (
                <ul className="space-y-3">
                  {d.tests.map((x) => (
                    <li key={x.id} className="min-w-0">
                      <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1 text-label">
                        <span className="min-w-0 break-words text-ink">{x.title}</span>
                        <span className="tabular-nums text-muted">
                          {t('analytics.participation.completed', { completed: x.completed, employees: x.employees })}
                          {x.rate !== null && x.rate !== undefined && (
                            <b className="ml-2 font-semibold text-ink">{formatRate(x.rate)}</b>
                          )}
                        </span>
                      </div>
                      <div className="mt-1">
                        <RateBar rate={x.rate} />
                      </div>
                    </li>
                  ))}
                </ul>
              ) : (
                <p className="text-label text-muted">{t('analytics.participation.no_tests')}</p>
              )}
            </section>
          </div>
        )}
      </Async>
    </Card>
  )
}
