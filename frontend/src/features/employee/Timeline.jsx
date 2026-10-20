// History of completed assessments (oldest to newest from the API, shown newest first)
// with the detailed per-characteristic result of each attempt.
import { Link } from 'react-router-dom'
import { useTranslation } from '../../i18n'
import Card, { CardTitle } from '../../ui/Card'
import EmptyState from '../../ui/EmptyState'
import Scale from '../../ui/charts/Scale'
import { charName, formatDate, roundScore } from './format'

export default function Timeline({ timeline }) {
  const { t, lang } = useTranslation()
  const items = [...timeline].sort((a, b) => new Date(b.completedAt) - new Date(a.completedAt))

  return (
    <Card aria-labelledby="timeline-title">
      <CardTitle id="timeline-title" className="mb-4">
        {t('employee.timeline.title')}
      </CardTitle>
      {items.length === 0 ? (
        <EmptyState title={t('employee.timeline.empty_title')} description={t('employee.timeline.empty_desc')} />
      ) : (
        <ol className="grid grid-cols-1 gap-4 lg:grid-cols-2 2xl:grid-cols-3">
          {items.map((a) => (
            <li key={a.attemptId} className="min-w-0 rounded-card border border-line p-4">
              <div className="mb-3 flex flex-wrap items-start justify-between gap-x-3 gap-y-1">
                <div className="min-w-0 flex-1 basis-40">
                  <h3 className="text-body font-semibold text-ink break-words">{a.assessmentTitle}</h3>
                  <time dateTime={a.completedAt} className="text-caption text-muted">
                    {formatDate(a.completedAt, lang, true)}
                  </time>
                </div>
                <Link
                  to={`/results/${a.attemptId}`}
                  className="inline-flex min-h-touch items-center text-label font-semibold text-primary hover:underline"
                >
                  {t('employee.timeline.open')}
                </Link>
              </div>
              {a.scores.length === 0 ? (
                <p className="text-label text-muted">{t('employee.timeline.no_scores')}</p>
              ) : (
                <ul className="space-y-3">
                  {a.scores.map((s) => (
                    <li key={s.characteristicId}>
                      <Scale label={charName(s, lang)} current={roundScore(s.score)} />
                    </li>
                  ))}
                </ul>
              )}
            </li>
          ))}
        </ol>
      )}
    </Card>
  )
}
