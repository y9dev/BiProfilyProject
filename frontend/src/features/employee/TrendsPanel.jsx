// Dynamics: small multiples per characteristic plus a summary of changes since the previous test.
import { useMemo } from 'react'
import { useTranslation } from '../../i18n'
import Card, { CardTitle } from '../../ui/Card'
import EmptyState from '../../ui/EmptyState'
import TrendMultiples from '../../ui/charts/TrendMultiples'
import { charName } from './format'
import { computeTrends, orderCharacteristics } from './trends'

function Stat({ path, label, value, tone }) {
  return (
    <li className="flex min-w-0 items-center gap-2 rounded-card bg-surface-2 px-4 py-3">
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
        className={tone}
      >
        <path d={path} />
      </svg>
      <b className="text-title tabular-nums text-ink">{value}</b>
      <span className="min-w-0 text-label text-muted">{label}</span>
    </li>
  )
}

export default function TrendsPanel({ profile }) {
  const { t, lang } = useTranslation()
  const { attemptCount, items, summary } = useMemo(
    () => computeTrends(profile.timeline, profile.characteristics),
    [profile],
  )
  const tiles = useMemo(() => {
    const byId = new Map(items.map((x) => [x.id, x]))
    return orderCharacteristics(profile.characteristics).map(({ c, core, color }) => ({
      ...byId.get(c.id),
      label: charName(c, lang),
      core,
      color,
    }))
  }, [items, profile.characteristics, lang])

  return (
    <Card aria-labelledby="trends-title" className="space-y-5">
      <div>
        <CardTitle id="trends-title">{t('trends.title')}</CardTitle>
        <p className="mt-1 text-label text-muted">{t('trends.desc')}</p>
      </div>
      {attemptCount === 0 || tiles.length === 0 ? (
        <EmptyState title={t('trends.empty_title')} description={t('trends.empty_desc')} />
      ) : (
        <>
          {attemptCount === 1 ? (
            <p role="status" className="rounded-card bg-primary-soft px-4 py-3 text-label text-ink">
              {t('trends.need_two')}
            </p>
          ) : (
            <section aria-label={t('trends.summary')}>
              <h3 className="mb-2 text-caption font-semibold uppercase tracking-wide text-muted">{t('trends.summary')}</h3>
              <ul className="grid grid-cols-1 gap-3 sm:grid-cols-3">
                <Stat path="M12 19V5M5 12l7-7 7 7" tone="text-success" value={summary.improved} label={t('trends.improved')} />
                <Stat path="M12 5v14M19 12l-7 7-7-7" tone="text-danger" value={summary.declined} label={t('trends.declined')} />
                <Stat path="M5 12h14" tone="text-muted" value={summary.unchanged} label={t('trends.unchanged')} />
              </ul>
            </section>
          )}
          <TrendMultiples items={tiles} />
        </>
      )}
    </Card>
  )
}
