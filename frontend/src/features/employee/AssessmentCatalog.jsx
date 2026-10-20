// Tests the employee can still take (completed ones live in the history). The API already hides hidden/expired ones.
import { useTranslation } from '../../i18n'
import Badge from '../../ui/Badge'
import Button from '../../ui/Button'
import Card, { CardTitle } from '../../ui/Card'
import EmptyState from '../../ui/EmptyState'
import { formatDate } from './format'

function AssessmentCard({ a }) {
  const { t, lang } = useTranslation()
  return (
    <li className="min-w-0">
      <article className="flex h-full min-w-0 flex-col gap-3 rounded-card border border-line bg-surface-2/40 p-4">
        <div className="flex flex-wrap items-start justify-between gap-2">
          <h3 className="min-w-0 flex-1 basis-40 text-body font-semibold text-ink break-words">{a.title}</h3>
          <Badge tone="info">{t('employee.catalog.status_available')}</Badge>
        </div>
        <p className="min-w-0 flex-1 whitespace-pre-line text-label text-muted break-words line-clamp-6">
          {a.description || t('employee.catalog.no_description')}
        </p>
        <p className="text-caption text-muted">
          {t('employee.catalog.questions', { n: a.questionCount })}
          {' · '}
          {a.expiresAt
            ? t('employee.catalog.until', { date: formatDate(a.expiresAt, lang) })
            : t('employee.catalog.no_limit')}
        </p>
        <Button to={`/assessments/${a.id}/take`} fullWidth>
          {t('employee.catalog.start')}
        </Button>
      </article>
    </li>
  )
}

export default function AssessmentCatalog({ items, allDone = false, title, intro }) {
  const { t } = useTranslation()
  return (
    <Card aria-labelledby="catalog-title">
      <CardTitle id="catalog-title" className="mb-4">
        {title || t('employee.catalog.title')}
      </CardTitle>
      {intro && <p className="-mt-2 mb-4 text-label text-muted">{intro}</p>}
      {items.length === 0 ? (
        <EmptyState
          title={allDone ? t('employee.catalog.all_done_title') : t('employee.catalog.empty_title')}
          description={allDone ? t('employee.catalog.all_done_desc') : t('employee.catalog.empty_desc')}
        />
      ) : (
        <ul className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
          {items.map((a) => (
            <AssessmentCard key={a.id} a={a} />
          ))}
        </ul>
      )}
    </Card>
  )
}
