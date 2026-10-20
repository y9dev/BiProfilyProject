// Shown instead of the radar, trends and history until the first test is completed.
import { useTranslation } from '../../i18n'
import Card, { CardTitle } from '../../ui/Card'

export default function StatsLocked() {
  const { t } = useTranslation()
  return (
    <Card aria-labelledby="locked-title" className="text-center">
      <CardTitle id="locked-title">{t('employee.first.locked_title')}</CardTitle>
      <p className="mx-auto mt-2 max-w-xl text-label text-muted">{t('employee.first.locked_desc')}</p>
    </Card>
  )
}
