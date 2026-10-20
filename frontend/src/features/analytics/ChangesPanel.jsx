// Significant score changes in the period: consecutive measurements of one characteristic whose
// difference reaches the threshold (default 15 points), largest first.
import { useState } from 'react'
import * as endpoints from '../../api/endpoints'
import { useTranslation } from '../../i18n'
import { useAsync } from '../../lib/useAsync'
import Async from '../../ui/Async'
import Button from '../../ui/Button'
import Card from '../../ui/Card'
import Input from '../../ui/Input'
import DataTable from '../manager/DataTable'
import { DEFAULT_THRESHOLD, charName, formatScore, sharedKey, sharedParams } from './analyticsState'
import { Delta, DeptName, Pager, PanelHead } from './parts'
import { formatInstant } from './format'

const LIMIT = 50

export default function ChangesPanel({ state, catalog, onThreshold }) {
  const { t, lang } = useTranslation()
  const [draft, setDraft] = useState(String(state.threshold))
  const [draftErr, setDraftErr] = useState('')
  const [offset, setOffset] = useState(0)
  const resetKey = `${sharedKey(state)}|${state.dataDept}|${state.threshold}`
  const [prevKey, setPrevKey] = useState(resetKey)
  if (prevKey !== resetKey) {
    setPrevKey(resetKey)
    setOffset(0)
    setDraft(String(state.threshold))
  }

  const params = {
    ...sharedParams(state),
    departmentId: state.dataDept ?? undefined,
    threshold: state.threshold,
    limit: LIMIT,
    offset,
  }
  const res = useAsync((signal) => endpoints.analytics.changes(params, { signal }), [resetKey, offset])
  const names = new Map((catalog || []).map((c) => [c.id, charName(c, lang)]))

  const apply = (e) => {
    e.preventDefault()
    const n = Number(draft)
    if (!/^\d{1,3}$/.test(draft) || n < 1 || n > 100) {
      setDraftErr(t('analytics.changes.threshold_error'))
      return
    }
    setDraftErr('')
    onThreshold(n)
  }

  const point = (p) => (
    <span className="flex flex-col">
      <b className="tabular-nums text-ink">{formatScore(p.score)}</b>
      <span className="text-caption text-muted">{formatInstant(p.at, lang)}</span>
    </span>
  )
  const columns = [
    {
      key: 'user',
      header: t('analytics.col.employee'),
      primary: true,
      className: 'w-[24%]',
      render: (r) => r.user.fullName,
    },
    { key: 'dept', header: t('analytics.col.department'), className: 'w-[18%]', render: (r) => <DeptName department={r.department} /> },
    {
      key: 'char',
      header: t('analytics.col.characteristic'),
      className: 'w-[20%]',
      render: (r) => names.get(r.characteristicId) || `#${r.characteristicId}`,
    },
    { key: 'from', header: t('analytics.col.before'), className: 'w-[13%]', render: (r) => point(r.from) },
    { key: 'to', header: t('analytics.col.after'), className: 'w-[13%]', render: (r) => point(r.to) },
    { key: 'delta', header: t('analytics.col.delta'), className: 'w-[12%]', render: (r) => <Delta value={r.delta} className="text-body" /> },
  ]

  return (
    <Card aria-labelledby="changes-title">
      <PanelHead
        id="changes-title"
        title={t('analytics.changes.title')}
        description={t('analytics.changes.desc', { n: state.threshold })}
        actions={
          <form onSubmit={apply} noValidate className="flex flex-wrap items-start gap-2">
            <Input
              type="number"
              inputMode="numeric"
              min={1}
              max={100}
              label={t('analytics.changes.threshold')}
              hint={t('analytics.changes.threshold_hint', { n: DEFAULT_THRESHOLD })}
              value={draft}
              error={draftErr}
              onChange={(e) => setDraft(e.target.value)}
              className="w-40"
            />
            <Button type="submit" variant="secondary" className="mt-6">
              {t('analytics.period.apply')}
            </Button>
          </form>
        }
      />
      <Async
        status={res.status}
        onRetry={res.reload}
        isEmpty={!res.data?.items?.length}
        empty={{ title: t('analytics.changes.empty_title'), description: t('analytics.changes.empty_desc', { n: state.threshold }) }}
      >
        <p className="mb-3 text-label text-muted" role="status">
          {t('analytics.changes.total', { n: res.data?.total ?? 0 })}
        </p>
        <DataTable
          columns={columns}
          rows={res.data?.items ?? []}
          rowKey={(r) => `${r.user.id}-${r.characteristicId}-${r.to.at}`}
          caption={t('analytics.changes.title')}
        />
        <Pager total={res.data?.total ?? 0} offset={offset} limit={LIMIT} onChange={setOffset} />
      </Async>
    </Card>
  )
}
