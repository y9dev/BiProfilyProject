// Step 4: qualitative summary of everything that will be created. No numbers, no scoring.
import { useTranslation } from '../../../i18n'
import Alert from '../../../ui/Alert'
import Badge from '../../../ui/Badge'
import Button from '../../../ui/Button'
import { effectiveOptions, isLowering, isRaising, mayMissZero, pointText } from './validate'
import { charName } from './helpers'
import { formatLocalDate } from '../../../lib/dates'

function Section({ title, onEdit, editLabel, children }) {
  return (
    <section className="min-w-0 rounded-card border border-line p-3 sm:p-4">
      <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
        <h3 className="text-label font-bold uppercase tracking-wide text-muted">{title}</h3>
        {onEdit && (
          <Button variant="ghost" onClick={onEdit}>
            {editLabel}
          </Button>
        )}
      </div>
      {children}
    </section>
  )
}

export default function StepReview({ state, chars, goTo, submitError }) {
  const { t, lang } = useTranslation()
  const list = chars.data || []
  const byId = new Map(list.map((c) => [String(c.id), c]))
  const known = new Set(list.map((c) => c.id))

  // Characteristics that something can raise (measured) vs. only lowered.
  const raised = new Set()
  const lowered = new Set()
  for (const q of state.questions) {
    for (const o of effectiveOptions(q)) {
      for (const [id, code] of Object.entries(o.fx)) {
        if (!byId.has(id)) continue
        if (isRaising(code)) raised.add(id)
        else if (isLowering(code)) lowered.add(id)
      }
    }
  }
  const onlyLowered = [...lowered].filter((id) => !raised.has(id))
  const names = (ids) => ids.map((id) => charName(byId.get(String(id)), lang)).join(', ')
  const zero = mayMissZero(state, known)

  return (
    <div className="space-y-4">
      <Alert tone="warning" title={t('wizard.review.immutable_title')}>
        {t('wizard.review.immutable_msg')}
      </Alert>
      {submitError && <Alert tone="danger">{submitError}</Alert>}

      <Section title={t('wizard.review.info')} onEdit={() => goTo(0)} editLabel={t('wizard.review.edit')}>
        <dl className="grid gap-x-6 gap-y-2 text-label sm:grid-cols-[auto_1fr]">
          <dt className="text-muted">{t('wizard.meta.title_label')}</dt>
          <dd className="min-w-0 break-words font-semibold text-ink">{state.title.trim()}</dd>
          {state.description.trim() && (
            <>
              <dt className="text-muted">{t('wizard.meta.description_label')}</dt>
              <dd className="min-w-0 whitespace-pre-line break-words text-ink">{state.description.trim()}</dd>
            </>
          )}
          <dt className="text-muted">{t('wizard.review.visibility')}</dt>
          <dd>
            <Badge tone={state.hidden ? 'warning' : 'success'}>{state.hidden ? t('wizard.meta.hidden') : t('wizard.meta.visible')}</Badge>
          </dd>
          <dt className="text-muted">{t('wizard.review.expiry')}</dt>
          <dd className="text-ink">
            {state.expires ? t('wizard.review.expiry_until', { date: formatLocalDate(`${state.expires}T00:00:00`, lang, { style: 'medium' }) }) : t('wizard.review.expiry_none')}
          </dd>
        </dl>
      </Section>

      <Section title={t('wizard.review.measures')} onEdit={() => goTo(2)} editLabel={t('wizard.review.edit')}>
        {raised.size ? (
          <ul className="flex flex-wrap gap-2">
            {[...raised].map((id) => (
              <li key={id}>
                <Badge tone="success">{charName(byId.get(id), lang)}</Badge>
              </li>
            ))}
          </ul>
        ) : (
          <p className="text-label text-danger">{t('wizard.review.measures_empty')}</p>
        )}
        {onlyLowered.length > 0 && (
          <Alert tone="info" className="mt-3">
            {t('wizard.review.only_lowered', { names: names(onlyLowered) })}
          </Alert>
        )}
        {zero.length > 0 && (
          <Alert tone="info" className="mt-3" title={t('wizard.review.zero_title')}>
            {t('wizard.review.zero_msg', { names: names(zero) })}
          </Alert>
        )}
      </Section>

      <Section title={t('wizard.review.questions', { n: state.questions.length })} onEdit={() => goTo(1)} editLabel={t('wizard.review.edit')}>
        <ol className="space-y-4">
          {state.questions.map((q, qi) => (
            <li key={q.key} className="min-w-0">
              <div className="flex items-start gap-2">
                <span className="flex size-7 shrink-0 items-center justify-center rounded-full bg-primary-soft text-caption font-bold text-primary">{qi + 1}</span>
                <p className="min-w-0 flex-1 break-words text-label font-semibold text-ink">{q.text.trim()}</p>
                <Badge tone="info">{t(`wizard.type.${q.type}`)}</Badge>
              </div>
              <ul className="mt-2 space-y-1.5 pl-0 sm:pl-9">
                {effectiveOptions(q).map((o, oi) => {
                  const fx = Object.entries(o.fx).filter(([id]) => byId.has(id))
                  return (
                    <li key={o.key} className="min-w-0 rounded-ctl bg-surface-2 px-3 py-2">
                      <p className="break-words text-label text-ink">{q.type === 'rating' ? pointText(q, oi) : o.text.trim()}</p>
                      {fx.length ? (
                        <ul className="mt-1 flex flex-wrap gap-1.5">
                          {fx.map(([id, code]) => (
                            <li key={id}>
                              <Badge tone={isRaising(code) ? 'success' : 'danger'}>
                                {charName(byId.get(id), lang)}: {t(`wizard.fx.level.${code}`)}
                              </Badge>
                            </li>
                          ))}
                        </ul>
                      ) : (
                        <p className="mt-0.5 text-caption text-muted">{t('wizard.review.no_effects')}</p>
                      )}
                    </li>
                  )
                })}
              </ul>
            </li>
          ))}
        </ol>
      </Section>
    </div>
  )
}
