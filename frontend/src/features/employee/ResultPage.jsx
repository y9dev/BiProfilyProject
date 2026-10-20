// Result of one completed attempt, exactly as returned by GET /api/me/attempts/{id}.
import { useParams } from 'react-router-dom'
import * as endpoints from '../../api/endpoints'
import { useTranslation } from '../../i18n'
import { useAsync } from '../../lib/useAsync'
import Async from '../../ui/Async'
import Button from '../../ui/Button'
import Card, { CardTitle } from '../../ui/Card'
import EmptyState from '../../ui/EmptyState'
import PageHeader from '../../ui/PageHeader'
import Scale from '../../ui/charts/Scale'
import { AlertIcon } from '../../ui/icons'
import { charName, formatDate, roundScore } from './format'

export default function ResultPage() {
  const { attemptId } = useParams()
  const { t, lang } = useTranslation()
  const { status, data, error, reload } = useAsync(
    (signal) => endpoints.employee.attempt(attemptId, { signal }),
    [attemptId],
  )

  if (status === 'error' && (error?.status === 404 || error?.status === 403)) {
    return (
      <Card>
        <EmptyState
          icon={<AlertIcon width={26} height={26} />}
          title={t('employee.result.not_found')}
          description=""
          action={<Button to="/">{t('employee.result.back')}</Button>}
        />
      </Card>
    )
  }

  return (
    <Async status={status} onRetry={reload}>
      {data && (
        <div className="min-w-0">
          <PageHeader
            title={data.assessmentTitle}
            description={t('employee.result.completed_at', { date: formatDate(data.completedAt, lang, true) })}
            actions={
              <Button variant="secondary" to="/">
                {t('employee.result.back')}
              </Button>
            }
          />
          <Card aria-labelledby="result-title">
            <CardTitle id="result-title">{t('employee.result.scores')}</CardTitle>
            <p className="mt-1 mb-5 text-label text-muted">{t('employee.result.scores_desc')}</p>
            {data.scores.length === 0 ? (
              <p className="text-label text-muted">{t('employee.result.no_scores')}</p>
            ) : (
              <ul className="grid grid-cols-1 gap-x-8 gap-y-5 md:grid-cols-2 2xl:grid-cols-3">
                {data.scores.map((s) => (
                  <li key={s.characteristicId} className="min-w-0">
                    <Scale label={charName(s, lang)} current={roundScore(s.score)} />
                  </li>
                ))}
              </ul>
            )}
          </Card>
        </div>
      )}
    </Async>
  )
}
