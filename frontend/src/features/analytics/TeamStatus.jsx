// Team status (manager/admin dashboard, in place of TrendsPanel): KPI tiles, per-test completion and the
// needs-attention list (name, department, reasons, last test date, drop delta). Fetches on its own
// (/api/dashboard/team-status) so the personal profile load is never blocked. Characteristic names for
// drop reasons come from /api/characteristics; if that fails the reason falls back to "#id".
import { useMemo } from 'react'
import * as endpoints from '../../api/endpoints'
import { useTranslation } from '../../i18n'
import { cx } from '../../lib/cx'
import { useAsync } from '../../lib/useAsync'
import Async from '../../ui/Async'
import Badge from '../../ui/Badge'
import Card, { CardTitle } from '../../ui/Card'
import Skeleton from '../../ui/Skeleton'
import { charName, formatDate, initials } from '../employee/format'

const fmt1 = (v) => `${Math.round(v * 10) / 10}`
const pct = (rate) => Math.round((rate || 0) * 100)
const signed = (v) => (v > 0 ? `+${fmt1(v)}` : v < 0 ? `−${fmt1(Math.abs(v))}` : '0')

// Copy of the Stat tile pattern from features/employee/TrendsPanel.jsx (file-local there; that file stays unchanged).
function Stat({ path, label, value, sub, tone, className }) {
  return (
    <li className={cx('flex min-w-0 items-center gap-3 rounded-card bg-surface-2 px-4 py-3', className)}>
      <svg
        width="18"
        height="18"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="2.5"
        strokeLinecap="round"
        strokeLinejoin="round"
        aria-hidden="true"
        className={cx('shrink-0', tone)}
      >
        <path d={path} />
      </svg>
      <div className="min-w-0">
        <b className="block text-title tabular-nums text-ink">{value}</b>
        <span className="block min-w-0 text-label text-muted">{label}</span>
        {sub && <span className="block text-caption tabular-nums text-faint">{sub}</span>}
      </div>
    </li>
  )
}

const ICON = {
  people: 'M16 19v-1.5a3.5 3.5 0 0 0-3.5-3.5h-5A3.5 3.5 0 0 0 4 17.5V19M10 10.5a3 3 0 1 0 0-6 3 3 0 0 0 0 6M20 19v-1.5a3.5 3.5 0 0 0-2.5-3.35M15.5 4.6a3 3 0 0 1 0 5.8',
  check: 'M20 6 9 17l-5-5',
  none: 'M5 12h14',
  pulse: 'M3 12h4l3-8 4 16 3-8h4',
  clock: 'M12 7v5l3 2M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0',
}

function StatusSkeleton() {
  return (
    <div className="space-y-5" aria-busy="true">
      <div className="grid grid-cols-2 gap-3 lg:grid-cols-5">
        {[0, 1, 2, 3, 4].map((k) => (
          <Skeleton key={k} className="h-20 w-full" />
        ))}
      </div>
      <div className="grid gap-5 lg:grid-cols-2">
        <Skeleton className="h-48 w-full" />
        <Skeleton className="h-48 w-full" />
      </div>
    </div>
  )
}

function SectionTitle({ id, children }) {
  return (
    <h3 id={id} className="text-caption font-semibold uppercase tracking-wide text-muted">
      {children}
    </h3>
  )
}

function TestsCompletion({ tests }) {
  const { t } = useTranslation()
  return (
    <section aria-labelledby="team-tests-title" className="min-w-0 space-y-3">
      <SectionTitle id="team-tests-title">{t('dashboard.team.tests.title')}</SectionTitle>
      {tests.length === 0 ? (
        <p className="text-label text-muted">{t('dashboard.team.tests.empty')}</p>
      ) : (
        <ul className="space-y-3">
          {tests.map((x) => {
            const p = pct(x.rate)
            return (
              <li key={x.id} className="min-w-0">
                <div className="flex items-baseline justify-between gap-3 text-label">
                  <span className="min-w-0 break-words text-ink">{x.title}</span>
                  <span className="shrink-0 tabular-nums text-muted">
                    {t('dashboard.team.tests.count', { completed: x.completed, employees: x.employees })}
                    <b className="ml-2 inline-block w-12 text-right text-ink">{p} %</b>
                  </span>
                </div>
                <div
                  role="img"
                  aria-label={t('dashboard.team.tests.bar_aria', { title: x.title, completed: x.completed, employees: x.employees, pct: p })}
                  className="mt-1.5 h-2 overflow-hidden rounded-full bg-surface-2"
                >
                  <div className="bar-grow h-full rounded-full bg-primary" style={{ width: `${Math.min(100, Math.max(0, p))}%` }} />
                </div>
              </li>
            )
          })}
        </ul>
      )}
    </section>
  )
}

function ReasonBadge({ reason, names }) {
  const { t, lang } = useTranslation()
  if (reason.code === 'drop') {
    const name = names.get(reason.characteristicId) || t('dashboard.team.char_fallback', { id: reason.characteristicId })
    const delta = Number(reason.delta) || 0
    return (
      <span title={t('dashboard.team.reason.drop_title', { abs: fmt1(Math.abs(delta)), date: formatDate(reason.at, lang) })}>
        <Badge tone="danger">
          <span className="tabular-nums">{t('dashboard.team.reason.drop', { name, delta: signed(delta) })}</span>
        </Badge>
      </span>
    )
  }
  if (reason.code === 'inactive') return <Badge tone="warning">{t('dashboard.team.reason.inactive', { days: reason.days ?? '' })}</Badge>
  if (reason.code === 'no_first_test') return <Badge tone="neutral">{t('dashboard.team.reason.no_first_test')}</Badge>
  return <Badge tone="neutral">{reason.code}</Badge>
}

function AttentionList({ attention, threshold, names }) {
  const { t, lang } = useTranslation()
  const items = attention?.items || []
  const counts = attention?.counts || {}
  return (
    <section aria-labelledby="team-attention-title" className="min-w-0 space-y-3">
      <div>
        <SectionTitle id="team-attention-title">{t('dashboard.team.attention.title')}</SectionTitle>
        <p className="mt-1 text-caption text-faint">{t('dashboard.team.attention.desc', { threshold })}</p>
      </div>
      {items.length === 0 ? (
        <p role="status" className="rounded-card bg-success-soft px-4 py-3 text-label text-success">
          <b className="font-semibold">{t('dashboard.team.attention.empty_title')}</b> {t('dashboard.team.attention.empty_desc')}
        </p>
      ) : (
        <>
          <ul className="flex flex-wrap gap-2" aria-label={t('dashboard.team.attention.title')}>
            {[
              ['drop', counts.drop, 'danger'],
              ['inactive', counts.inactive, 'warning'],
              ['no_first_test', counts.noFirstTest, 'neutral'],
            ].map(([k, v, tone]) => (
              <li key={k}>
                <Badge tone={tone}>
                  {t(`dashboard.team.counts.${k}`)}: <b className="ml-1 tabular-nums">{v ?? 0}</b>
                </Badge>
              </li>
            ))}
          </ul>
          <ul className="divide-y divide-line rounded-card border border-line">
            {items.map((it) => (
              <li key={it.user.id} className="flex min-w-0 gap-3 px-3 py-3 sm:px-4">
                <span
                  aria-hidden="true"
                  className="mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-surface-2 text-caption font-semibold text-muted"
                >
                  {initials(it.user.fullName)}
                </span>
                <div className="min-w-0 flex-1 space-y-1.5">
                  <div className="flex min-w-0 flex-wrap items-baseline gap-x-2 gap-y-0.5">
                    <b className="min-w-0 break-words font-semibold text-ink">{it.user.fullName}</b>
                    <span className="text-caption text-muted">
                      {it.department ? it.department.name : t('stats.no_department')}
                      {it.department && it.department.active === false && ` · ${t('stats.department_inactive')}`}
                    </span>
                  </div>
                  <div className="flex flex-wrap gap-1.5">
                    {(it.reasons || []).map((r, k) => (
                      <ReasonBadge key={`${r.code}-${k}`} reason={r} names={names} />
                    ))}
                  </div>
                  <p className="text-caption text-faint">
                    {it.lastAttemptAt ? t('dashboard.team.last_test', { date: formatDate(it.lastAttemptAt, lang) }) : t('dashboard.team.never')}
                  </p>
                </div>
              </li>
            ))}
          </ul>
          {attention.total > items.length && (
            <p className="text-caption text-faint">{t('dashboard.team.attention.shown', { shown: items.length, total: attention.total })}</p>
          )}
        </>
      )}
    </section>
  )
}

export default function TeamStatus() {
  const { t, lang } = useTranslation()
  const { status, data, reload } = useAsync(async (signal) => {
    const [team, chars] = await Promise.all([
      endpoints.dashboard.teamStatus({ signal }),
      endpoints.characteristics.list({ signal }).catch(() => []),
    ])
    return { team, chars: Array.isArray(chars) ? chars : [] }
  }, [])

  const names = useMemo(() => new Map((data?.chars || []).map((c) => [c.id, charName(c, lang)])), [data, lang])
  const team = data?.team
  const k = team?.kpis

  return (
    <Card aria-labelledby="team-status-title" className="space-y-5">
      <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
        <CardTitle id="team-status-title">{t('dashboard.team.title')}</CardTitle>
        {team && <p className="text-caption text-muted">{t('stats.as_of', { date: formatDate(team.asOf, lang) })}</p>}
      </div>
      <Async
        status={status}
        onRetry={reload}
        skeleton={<StatusSkeleton />}
        isEmpty={status === 'ready' && !(k?.employees > 0)}
        empty={{ title: t('dashboard.team.empty_title'), description: t('dashboard.team.empty_desc') }}
      >
        {team && k && (
          <>
            <ul className="grid grid-cols-1 gap-3 xs:grid-cols-2 lg:grid-cols-5">
              <Stat className="xs:col-span-2 lg:col-span-1" path={ICON.people} tone="text-primary" value={k.employees} label={t('dashboard.team.kpi.employees')} />
              <Stat
                path={ICON.check}
                tone="text-success"
                value={k.participated}
                label={t('dashboard.team.kpi.participated')}
                sub={t('dashboard.team.kpi.rate', { pct: pct(k.participationRate) })}
              />
              <Stat path={ICON.none} tone="text-muted" value={k.noAttempts} label={t('dashboard.team.kpi.no_attempts')} />
              <Stat path={ICON.pulse} tone="text-primary" value={k.activeRecently} label={t('dashboard.team.kpi.active_recently', { days: team.inactiveDays })} />
              <Stat path={ICON.clock} tone="text-warning" value={k.inactive} label={t('dashboard.team.kpi.inactive', { days: team.inactiveDays })} />
            </ul>
            <div className="grid min-w-0 gap-6 lg:grid-cols-[minmax(0,2fr)_minmax(0,3fr)]">
              <TestsCompletion tests={team.tests || []} />
              <AttentionList attention={team.attention} threshold={team.dropThreshold} names={names} />
            </div>
          </>
        )}
      </Async>
    </Card>
  )
}
