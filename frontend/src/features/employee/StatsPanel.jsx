// Skills hero: radar of ALL characteristics (means from /api/me/profile) with a legend.
// Core characteristics (they carry nameI18n) get their own colour, custom ones are gray.
import { useTranslation } from '../../i18n'
import Card, { CardTitle } from '../../ui/Card'
import EmptyState from '../../ui/EmptyState'
import Radar from '../../ui/charts/Radar'
import { charName } from './format'
import { orderCharacteristics } from './trends'

export default function StatsPanel({ profile, className }) {
  const { t, lang } = useTranslation()
  const items = orderCharacteristics(profile.characteristics).map(({ c, core, color }) => ({
    id: c.id,
    label: charName(c, lang),
    value: c.mean,
    measured: c.count > 0,
    color,
    core,
  }))
  const measured = items.some((x) => x.measured)

  return (
    <Card aria-labelledby="stats-title" className={className}>
      <CardTitle id="stats-title">{t('employee.stats.title')}</CardTitle>
      <p className="mb-4 mt-1 text-label text-muted">
        {measured ? t('employee.stats.desc') : t('employee.stats.desc_empty')}
      </p>
      {items.length === 0 ? (
        <EmptyState title={t('employee.stats.empty_title')} description={t('employee.stats.empty_desc')} />
      ) : (
        <Radar items={items} />
      )}
    </Card>
  )
}
