// One question: single choice (radio), multiple choice (checkbox) or rating scale (radio scale).
// Native inputs give keyboard and screen-reader behaviour; the label card is the large touch target.
import { useTranslation } from '../../i18n'
import { cx } from '../../lib/cx'
import { parseRatingScale } from './ratingScale'

const CARD =
  'flex min-h-touch cursor-pointer items-center gap-3 rounded-card border-2 border-line bg-surface p-3 sm:p-4 ' +
  'transition-colors hover:border-line-strong has-[:checked]:border-primary has-[:checked]:bg-primary-soft ' +
  'has-[:focus-visible]:outline-2 has-[:focus-visible]:outline-offset-2 has-[:focus-visible]:outline-ring'

function Mark({ checked, round }) {
  return (
    <span
      aria-hidden="true"
      className={cx(
        'flex h-6 w-6 shrink-0 items-center justify-center border-2',
        round ? 'rounded-full' : 'rounded-ctl',
        checked ? 'border-primary bg-primary text-primary-ink' : 'border-line-strong bg-surface',
      )}
    >
      {checked && (
        <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="2.4">
          <path d="M3 8.5l3.2 3L13 4.5" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
      )}
    </span>
  )
}

function ChoiceList({ question, selected, onChange, multiple }) {
  const name = `q-${question.id}`
  return (
    <div className="space-y-3">
      {question.options.map((o) => {
        const checked = selected.includes(o.id)
        return (
          <label key={o.id} className={CARD}>
            <input
              className="sr-only"
              type={multiple ? 'checkbox' : 'radio'}
              name={name}
              value={o.id}
              checked={checked}
              onChange={() => {
                if (!multiple) return onChange([o.id])
                onChange(checked ? selected.filter((x) => x !== o.id) : [...selected, o.id])
              }}
            />
            <Mark checked={checked} round={!multiple} />
            <span className="min-w-0 text-body text-ink break-words">{o.text}</span>
          </label>
        )
      })}
    </div>
  )
}

function RatingScale({ question, selected, onChange, t }) {
  const name = `q-${question.id}`
  const scale = parseRatingScale(question.options)
  if (!scale) {
    return <ChoiceList question={question} selected={selected} onChange={onChange} multiple={false} />
  }
  return (
    <div>
      <div className="grid grid-cols-5 gap-2 sm:flex sm:flex-wrap">
        {scale.points.map((o) => {
          const checked = selected.includes(o.id)
          return (
            <label
              key={o.id}
              className={cx(
                'flex min-h-touch min-w-touch cursor-pointer items-center justify-center rounded-card border-2 px-3 py-2 sm:min-w-14',
                'text-title font-semibold tabular-nums transition-colors',
                'has-[:focus-visible]:outline-2 has-[:focus-visible]:outline-offset-2 has-[:focus-visible]:outline-ring',
                checked
                  ? 'border-primary bg-primary text-primary-ink'
                  : 'border-line bg-surface text-ink hover:border-line-strong',
              )}
            >
              <input
                className="sr-only"
                type="radio"
                name={name}
                value={o.id}
                checked={checked}
                onChange={() => onChange([o.id])}
              />
              {o.value}
            </label>
          )
        })}
      </div>
      <div className="mt-2 flex justify-between gap-4 text-caption text-muted">
        <span className="min-w-0 break-words">{scale.low || t('employee.take.scale_low')}</span>
        <span className="min-w-0 break-words text-right">{scale.high || t('employee.take.scale_high')}</span>
      </div>
    </div>
  )
}

export default function QuestionView({ question, selected, onChange, invalid, errorId }) {
  const { t } = useTranslation()
  const hint = t(`employee.take.type_${question.type}`)
  return (
    <fieldset aria-invalid={invalid || undefined} aria-describedby={invalid ? errorId : undefined} className="min-w-0">
      <legend className="mb-1 w-full whitespace-pre-line text-title font-semibold text-ink break-words sm:text-display">
        {question.text}
      </legend>
      <p className="mb-4 text-label text-muted">{hint}</p>
      {question.type === 'rating' ? (
        <RatingScale question={question} selected={selected} onChange={onChange} t={t} />
      ) : (
        <ChoiceList question={question} selected={selected} onChange={onChange} multiple={question.type === 'multiple'} />
      )}
    </fieldset>
  )
}
