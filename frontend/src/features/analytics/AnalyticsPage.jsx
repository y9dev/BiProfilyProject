// Admin: /analytics. One shared filter {period, from, to, characteristics} lives in the URL and drives
// every tab; tab-local state (departments, department filter, employee, search, threshold) sits beside
// it in the URL and never affects other tabs. Server-side aggregates only (plan section 3/7).
import { useCallback, useEffect, useMemo, useRef } from 'react'
import { useSearchParams } from 'react-router-dom'
import * as endpoints from '../../api/endpoints'
import { fieldErrors } from '../../api/client'
import { useTranslation } from '../../i18n'
import { cx } from '../../lib/cx'
import { errorMessage } from '../../lib/errors'
import { useAsync } from '../../lib/useAsync'
import Alert from '../../ui/Alert'
import Button from '../../ui/Button'
import PageHeader from '../../ui/PageHeader'
import Card from '../../ui/Card'
import {
  PRESETS,
  TABS,
  effectiveSelection,
  knownDepartments,
  parseState,
  seriesStyles,
  serializeState,
  sharedKey,
  sharedParams,
  toggleCharacteristic,
  toggleDepartment,
} from './analyticsState'
import ChangesPanel from './ChangesPanel'
import CharacteristicPicker from './CharacteristicPicker'
import CompanyTrendCard from './CompanyTrendCard'
import DataCoveragePanel from './DataCoveragePanel'
import DepartmentFilter from './DepartmentFilter'
import DepartmentsView from './DepartmentsView'
import EmployeeView from './EmployeeView'
import ParticipationPanel from './ParticipationPanel'
import PeriodFilter from './PeriodFilter'
import RankingPanel from './RankingPanel'
import { formatDay } from './format'

function Tabs({ value, onChange }) {
  const { t } = useTranslation()
  const refs = useRef({})
  const onKey = (e) => {
    const i = TABS.indexOf(value)
    let next = null
    if (e.key === 'ArrowRight') next = TABS[(i + 1) % TABS.length]
    else if (e.key === 'ArrowLeft') next = TABS[(i - 1 + TABS.length) % TABS.length]
    else if (e.key === 'Home') next = TABS[0]
    else if (e.key === 'End') next = TABS[TABS.length - 1]
    if (!next) return
    e.preventDefault()
    onChange(next)
    refs.current[next]?.focus()
  }
  return (
    <div className="-mx-4 overflow-x-auto px-4 sm:mx-0 sm:px-0">
      <div role="tablist" aria-label={t('analytics.tabs.label')} className="flex min-w-max gap-1 border-b border-line">
        {TABS.map((tab) => {
          const on = tab === value
          return (
            <button
              key={tab}
              ref={(el) => {
                refs.current[tab] = el
              }}
              type="button"
              role="tab"
              id={`tab-${tab}`}
              aria-selected={on}
              aria-controls={`panel-${tab}`}
              tabIndex={on ? 0 : -1}
              onKeyDown={onKey}
              onClick={() => onChange(tab)}
              className={cx(
                '-mb-px min-h-touch whitespace-nowrap border-b-2 px-4 text-label font-semibold transition-colors',
                on ? 'border-primary text-primary' : 'border-transparent text-muted hover:border-line-strong hover:text-ink',
              )}
            >
              {t(`analytics.tabs.${tab}`)}
            </button>
          )
        })}
      </div>
    </div>
  )
}

export default function AnalyticsPage() {
  const { t, lang } = useTranslation()
  const [sp, setSp] = useSearchParams()
  const state = useMemo(() => parseState(sp), [sp])

  const patch = useCallback(
    (p, replace = false) => setSp((prev) => serializeState({ ...parseState(prev), ...p }), { replace }),
    [setSp],
  )

  const key = sharedKey(state)
  const trend = useAsync((signal) => endpoints.analytics.trend(sharedParams(state), { signal }), [key])
  const deps = useAsync((signal) => endpoints.departments.list(true, { signal }))
  const departments = deps.data ?? []
  // ready: the list; loading: undefined (wait); error: null (send the URL selection as is).
  const knownDeps = deps.status === 'ready' ? departments : deps.status === 'error' ? null : undefined

  const data = trend.data
  const catalog = useMemo(() => data?.characteristics ?? [], [data])
  const defaultIds = useMemo(() => data?.defaultIds ?? [], [data])
  const selection = useMemo(
    () => (data ? effectiveSelection(state.characteristics, catalog, defaultIds) : state.characteristics),
    [data, state.characteristics, catalog, defaultIds],
  )
  const styles = useMemo(() => seriesStyles(selection, catalog), [selection, catalog])
  const seriesStyle = useCallback(
    (c) => styles.get(c.id) ?? { color: c.core ? 'var(--core-1)' : 'var(--extra-1)', dashed: !c.core },
    [styles],
  )

  // A 422 from a stale or hand-edited link: unknown characteristic ids fall back to the default
  // selection; period problems are shown on the period control with a reset.
  const fe = trend.status === 'error' ? fieldErrors(trend.error) : {}
  const periodError = ['period', 'from', 'to'].map((p) => fe[p] && { path: p === 'period' ? 'from' : p, code: fe[p] }).find(Boolean)
  const charsInvalid = trend.status === 'error' && !!fe.characteristics && state.characteristics.length > 0
  useEffect(() => {
    if (charsInvalid) patch({ characteristics: [] }, true)
  }, [charsInvalid, patch])

  const onToggleChar = (id) => {
    const next = toggleCharacteristic(selection, id, defaultIds)
    if (next) patch({ characteristics: next })
  }
  const onToggleDept = (id) => {
    const next = toggleDepartment(knownDepartments(state.departments, knownDeps), id)
    if (next) patch({ departments: next })
  }

  const trendErrorText =
    trend.status === 'error' && !charsInvalid
      ? trend.error?.code === 'analytics_timeout'
        ? t('stats.errors.timeout')
        : periodError
          ? t(`stats.validation.${periodError.code}`)
          : errorMessage(t, trend.error)
      : null

  return (
    <div className="min-w-0 space-y-6">
      <PageHeader title={t('analytics.title')} description={t('analytics.desc')} />

      <Card aria-label={t('analytics.filters')} className="grid gap-6 lg:grid-cols-[minmax(0,auto)_minmax(0,1fr)]">
        <PeriodFilter
          key={`${state.period}|${state.from}|${state.to}`}
          period={state.period}
          from={state.from}
          to={state.to}
          serverError={periodError}
          onChange={(p) => patch(p)}
        />
        <div className="min-w-0 lg:border-l lg:border-line lg:pl-6">
          <CharacteristicPicker
            catalog={catalog}
            selection={selection}
            urlSelection={state.characteristics}
            defaultIds={defaultIds}
            styles={styles}
            disabled={!data}
            onToggle={onToggleChar}
            onRestore={() => patch({ characteristics: [] })}
          />
        </div>
      </Card>

      {trendErrorText && (
        <Alert tone="danger" title={t('analytics.load_failed')}>
          <p>{trendErrorText}</p>
          <div className="mt-3 flex flex-wrap gap-2">
            <Button variant="secondary" onClick={trend.reload}>
              {t('common.retry')}
            </Button>
            {(periodError || state.period !== '90d') && (
              <Button variant="ghost" onClick={() => patch({ period: '90d', from: '', to: '' }, true)}>
                {t('analytics.reset_period')}
              </Button>
            )}
          </div>
        </Alert>
      )}

      <Tabs value={state.tab} onChange={(tab) => patch({ tab })} />

      {state.tab !== 'overview' && (
      <div className="space-y-1">
        <Alert tone="info">{t('stats.disclaimer')}</Alert>
        {data && (
          <p className="flex flex-wrap gap-x-4 gap-y-1 px-1 pt-2 text-caption text-muted">
            <span>
              {t('stats.period.range', { from: formatDay(data.period.from, lang), to: formatDay(data.period.to, lang) })}
              {' · '}
              {t(`stats.granularity.${data.period.granularity}`)}
            </span>
            <span>{t('stats.timezone_note')}</span>
            <span>{t('stats.insufficient_hint', { min: data.minSample })}</span>
          </p>
        )}
      </div>
      )}

      <div role="tabpanel" id={`panel-${state.tab}`} aria-labelledby={`tab-${state.tab}`} className="min-w-0 space-y-6">
        {state.tab === 'overview' && (
          <>
            <CompanyTrendCard
              data={data}
              loading={trend.status === 'loading'}
              error={trend.status === 'error' ? trend.error : null}
              mode="analytics"
              period={state.period}
              presets={PRESETS}
              allowCustom
              from={state.from}
              to={state.to}
              seriesStyle={seriesStyle}
              title={t('analytics.overview.chart_title')}
            />
            <ParticipationPanel state={state} departments={departments} onDepartment={(v) => patch({ participationDept: v })} />
          </>
        )}

        {state.tab === 'departments' && (
          <DepartmentsView
            state={state}
            departments={departments}
            known={knownDeps}
            onToggleDepartment={onToggleDept}
            onReplace={(next) => patch({ departments: next }, true)}
          />
        )}

        {state.tab === 'employees' && (
          <EmployeeView
            state={state}
            departments={departments}
            selection={selection}
            styles={styles}
            trend={data}
            onPatch={patch}
          />
        )}

        {state.tab === 'data' && (
          <>
            <Card className="flex flex-wrap items-end justify-between gap-4">
              <p className="min-w-0 flex-1 basis-64 text-label text-muted">{t('analytics.data.desc')}</p>
              <DepartmentFilter departments={departments} value={state.dataDept} onChange={(v) => patch({ dataDept: v })} />
            </Card>
            <RankingPanel key={key} state={state} />
            <ChangesPanel state={state} catalog={catalog} onThreshold={(n) => patch({ threshold: n })} />
            <DataCoveragePanel trend={data} selection={selection} defaultIds={defaultIds} />
          </>
        )}
      </div>
    </div>
  )
}
