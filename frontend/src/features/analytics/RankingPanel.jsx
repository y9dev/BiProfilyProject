// Ranking by characteristic as of the period end: employees (ties share a rank) and departments
// with at least the minimum sample. One characteristic is shown at a time to keep phones readable.
import { useState } from 'react'
import * as endpoints from '../../api/endpoints'
import { useTranslation } from '../../i18n'
import { useAsync } from '../../lib/useAsync'
import Alert from '../../ui/Alert'
import Async from '../../ui/Async'
import Card from '../../ui/Card'
import Select from '../../ui/Select'
import DataTable from '../manager/DataTable'
import { charName, formatScore, sharedKey, sharedParams } from './analyticsState'
import { DeptName, Pager, PanelHead, Sample } from './parts'
import { deptKey, formatDay, formatInstant } from './format'

const LIMIT = 20

export default function RankingPanel({ state }) {
  const { t, lang } = useTranslation()
  const [picked, setPicked] = useState(null)
  const [offset, setOffset] = useState(0)
  // limit/offset page the employees of every block; a new filter starts again at the first page.
  const resetKey = `${sharedKey(state)}|${state.dataDept}`
  const [prevKey, setPrevKey] = useState(resetKey)
  if (prevKey !== resetKey) {
    setPrevKey(resetKey)
    setOffset(0)
  }
  const params = { ...sharedParams(state), departmentId: state.dataDept ?? undefined, limit: LIMIT, offset }
  const res = useAsync((signal) => endpoints.analytics.ranking(params, { signal }), [resetKey, offset])
  const blocks = res.data?.blocks ?? []
  const block = blocks.find((b) => b.characteristic.id === picked) || blocks[0]

  const rank = (r) => <b className="tabular-nums text-ink">{r.rank}</b>
  const empColumns = [
    { key: 'rank', header: t('analytics.col.rank'), className: 'w-[10%]', render: rank },
    { key: 'name', header: t('analytics.col.employee'), primary: true, className: 'w-[34%]', render: (r) => r.user.fullName },
    { key: 'dept', header: t('analytics.col.department'), className: 'w-[26%]', render: (r) => <DeptName department={r.department} /> },
    { key: 'score', header: t('analytics.col.score'), className: 'w-[12%]', render: (r) => <b className="tabular-nums">{formatScore(r.score)}</b> },
    { key: 'at', header: t('analytics.col.measured_at'), className: 'w-[18%]', render: (r) => formatInstant(r.measuredAt, lang) },
  ]
  const depColumns = [
    { key: 'rank', header: t('analytics.col.rank'), className: 'w-[10%]', render: rank },
    { key: 'dept', header: t('analytics.col.department'), primary: true, className: 'w-[40%]', render: (r) => <DeptName department={r.department} /> },
    { key: 'value', header: t('analytics.col.average'), className: 'w-[20%]', render: (r) => <b className="tabular-nums">{formatScore(r.value)}</b> },
    { key: 'n', header: t('analytics.col.sample'), className: 'w-[30%]', render: (r) => <Sample employees={r.employees} attempts={r.attempts} /> },
  ]

  return (
    <Card aria-labelledby="ranking-title">
      <PanelHead
        id="ranking-title"
        title={t('analytics.ranking.title')}
        description={res.data ? t('analytics.ranking.desc', { date: formatDay(res.data.asOf, lang) }) : undefined}
        actions={
          blocks.length > 1 && (
            <Select
              label={t('analytics.ranking.characteristic')}
              value={String(block?.characteristic.id ?? '')}
              onChange={(e) => setPicked(Number(e.target.value))}
              className="w-full sm:w-72"
              options={blocks.map((b) => ({ value: String(b.characteristic.id), label: charName(b.characteristic, lang) }))}
            />
          )
        }
      />
      <Async
        status={res.status}
        onRetry={res.reload}
        isEmpty={!block}
        empty={{ title: t('stats.no_data'), description: t('stats.no_data_period') }}
      >
        {block && (
          <div className="space-y-6">
            {block.insufficient ? (
              <Alert tone="info" title={t('stats.insufficient', { n: block.employees.total })}>
                {t('analytics.ranking.insufficient')}
              </Alert>
            ) : (
              <section aria-labelledby="ranking-emp">
                <div className="mb-3 flex flex-wrap items-baseline justify-between gap-2">
                  <h3 id="ranking-emp" className="text-body font-semibold text-ink">
                    {t('analytics.ranking.employees')}
                  </h3>
                  <span className="text-caption text-muted">
                    {t('analytics.ranking.shown', {
                      shown: block.employees.items.length ? `${offset + 1}–${offset + block.employees.items.length}` : 0,
                      total: block.employees.total,
                    })}
                  </span>
                </div>
                <DataTable
                  columns={empColumns}
                  rows={block.employees.items}
                  rowKey={(r) => r.user.id}
                  caption={t('analytics.ranking.employees')}
                />
                <Pager total={block.employees.total} offset={offset} limit={LIMIT} onChange={setOffset} />
              </section>
            )}
            <section aria-labelledby="ranking-dep">
              <h3 id="ranking-dep" className="mb-3 text-body font-semibold text-ink">
                {t('analytics.ranking.departments')}
              </h3>
              {block.departments.length ? (
                <DataTable
                  columns={depColumns}
                  rows={block.departments}
                  rowKey={(r) => deptKey(r.department)}
                  caption={t('analytics.ranking.departments')}
                />
              ) : (
                <p className="text-label text-muted">{t('analytics.ranking.no_departments')}</p>
              )}
            </section>
          </div>
        )}
      </Async>
    </Card>
  )
}
