// Read-only preview of an assessment's questions, options and scoring effects (GET /api/manage/assessments/{id}).
import { useMemo } from 'react'
import * as endpoints from '../../../api/endpoints'
import { useTranslation } from '../../../i18n'
import { useAsync } from '../../../lib/useAsync'
import Async from '../../../ui/Async'
import Badge from '../../../ui/Badge'
import Modal from '../../../ui/Modal'

const LEVEL_WEIGHT = { low: 1, moderate: 2, high: 3 }

export default function AssessmentPreview({ assessment, onClose }) {
  const { t, lang } = useTranslation()
  const detail = useAsync((signal) => endpoints.manage.assessment(assessment.id, { signal }), [assessment.id])
  const chars = useAsync((signal) => endpoints.characteristics.list({ signal }))

  const names = useMemo(() => {
    const m = new Map()
    for (const c of chars.data ?? []) m.set(c.id, c.nameI18n?.[lang] || c.name)
    return m
  }, [chars.data, lang])

  const status = detail.status === 'error' ? 'error' : detail.status === 'loading' ? 'loading' : 'ready'
  const questions = detail.data?.questions ?? []

  return (
    <Modal isOpen onClose={onClose} title={assessment.title} size="xl">
      <p className="mb-4 text-label text-muted">{t('manager.assessments.preview.note')}</p>
      <Async status={status} onRetry={detail.reload} isEmpty={status === 'ready' && questions.length === 0}>
        <ol className="space-y-4">
          {questions.map((q, qi) => (
            <li key={q.id} className="min-w-0 rounded-card border border-line p-4">
              <div className="flex flex-wrap items-start gap-2">
                <span className="text-caption font-semibold text-muted">{qi + 1}.</span>
                <p className="min-w-0 flex-1 basis-48 font-semibold text-ink break-words">{q.text}</p>
                <Badge tone="info">{t(`manager.assessments.preview.type.${q.type}`)}</Badge>
              </div>
              <ul className="mt-3 space-y-2">
                {q.options.map((o) => (
                  <li key={o.id} className="min-w-0 rounded-ctl bg-surface-2 p-3">
                    <p className="text-label text-ink break-words">{o.text}</p>
                    {o.effects.length === 0 ? (
                      <p className="mt-1 text-caption text-muted">{t('manager.assessments.preview.no_effects')}</p>
                    ) : (
                      <ul className="mt-2 flex flex-wrap gap-1.5">
                        {o.effects.map((ef) => (
                          <li key={ef.characteristicId}>
                            <Badge tone={ef.direction > 0 ? 'success' : 'danger'}>
                              {ef.direction > 0 ? '+' : '−'}
                              {LEVEL_WEIGHT[ef.level] ?? ''} {names.get(ef.characteristicId) ?? `#${ef.characteristicId}`} (
                              {t(`manager.assessments.preview.level.${ef.level}`)})
                            </Badge>
                          </li>
                        ))}
                      </ul>
                    )}
                  </li>
                ))}
              </ul>
            </li>
          ))}
        </ol>
      </Async>
    </Modal>
  )
}
