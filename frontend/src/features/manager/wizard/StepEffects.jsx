// Step 3: qualitative effects. Per option: pick characteristics and a word-scale effect (no numbers).
// Desktop: always-open compact grid per option. Mobile: accordion per option.
import { useState } from 'react'
import { Link } from 'react-router-dom'
import { useTranslation } from '../../../i18n'
import { cx } from '../../../lib/cx'
import Alert from '../../../ui/Alert'
import Badge from '../../../ui/Badge'
import Button from '../../../ui/Button'
import EmptyState from '../../../ui/EmptyState'
import ErrorState from '../../../ui/ErrorState'
import Skeleton from '../../../ui/Skeleton'
import { EFFECT_CODES, effectiveOptions, isLowering, isRaising, pointText } from './validate'
import { ChevronIcon } from './parts'
import { charName, errText } from './helpers'

const tone = (code) =>
  isRaising(code) ? 'border-success bg-success-soft' : isLowering(code) ? 'border-danger bg-danger-soft' : 'border-line-strong bg-surface'

function OptionFx({ q, qi, o, oi, chars, errors, update, defaultOpen }) {
  const { t, lang } = useTranslation()
  const [open, setOpen] = useState(defaultOpen)
  const base = `questions[${qi}].options[${oi}]`
  const count = Object.keys(o.fx).filter((id) => chars.some((c) => String(c.id) === id)).length
  const optErr = errors[base]
  const cellErrs = chars.filter((c) => errors[`${base}.fx.${c.id}`])
  const label = q.type === 'rating' ? pointText(q, oi) : o.text.trim() || t('wizard.q.option_label', { n: oi + 1 })
  const bad = !!optErr || cellErrs.length > 0

  const setFx = (id, code) =>
    update((d) => {
      const opt = effectiveOptions(d.questions[qi])[oi]
      if (code === 'none') delete opt.fx[id]
      else opt.fx[id] = code
    })

  return (
    <li className={cx('rounded-card border', bad ? 'border-danger' : 'border-line')} data-error={bad || undefined}>
      <button
        type="button"
        className="flex min-h-touch w-full items-start gap-2 rounded-card px-3 py-2 text-left md:pointer-events-none"
        aria-expanded={open}
        onClick={() => setOpen((v) => !v)}
      >
        <span className="min-w-0 flex-1 break-words text-label font-semibold text-ink">{label}</span>
        <Badge tone={count ? 'info' : 'neutral'}>{count ? t('wizard.fx.affects_n', { n: count }) : t('wizard.fx.unaffected')}</Badge>
        <ChevronIcon open={open} className="mt-0.5 shrink-0 text-muted md:hidden" />
      </button>
      <div className={cx('border-t border-line p-3', open ? 'block' : 'hidden md:block')}>
        {optErr && <p className="mb-2 text-caption font-medium text-danger">{errText(t, optErr)}</p>}
        <div className="grid grid-cols-[repeat(auto-fill,minmax(10.5rem,1fr))] gap-x-3 gap-y-2">
          {chars.map((c) => {
            const code = o.fx[c.id] || 'none'
            const err = errors[`${base}.fx.${c.id}`]
            const id = `fx-${q.key}-${oi}-${c.id}`
            return (
              <div key={c.id} className="min-w-0">
                <label htmlFor={id} className="mb-0.5 block break-words text-caption font-medium text-muted">
                  {charName(c, lang)}
                </label>
                <select
                  id={id}
                  value={code}
                  aria-invalid={err ? true : undefined}
                  onChange={(e) => setFx(c.id, e.target.value)}
                  className={cx('min-h-touch w-full min-w-0 rounded-ctl border px-2 py-1.5 text-label text-ink', tone(code), err && 'border-danger')}
                >
                  {EFFECT_CODES.map((k) => (
                    <option key={k} value={k}>
                      {t(`wizard.fx.level.${k}`)}
                    </option>
                  ))}
                </select>
                {err && <p className="mt-0.5 text-caption font-medium text-danger">{errText(t, err)}</p>}
              </div>
            )
          })}
        </div>
        {count > 0 && (
          <div className="mt-2">
            <Button variant="ghost" onClick={() => update((d) => void (effectiveOptions(d.questions[qi])[oi].fx = {}))}>
              {t('wizard.fx.clear')}
            </Button>
          </div>
        )}
      </div>
    </li>
  )
}

export default function StepEffects({ state, update, errors, chars }) {
  const { t } = useTranslation()

  if (chars.status === 'loading') {
    return (
      <div className="space-y-3" aria-busy="true">
        <Skeleton className="h-6 w-1/3" />
        <Skeleton className="h-28 w-full" />
        <Skeleton className="h-28 w-full" />
      </div>
    )
  }
  if (chars.status === 'error') return <ErrorState onRetry={chars.reload} />
  const list = chars.data || []
  if (list.length === 0) {
    return (
      <EmptyState
        title={t('wizard.fx.no_chars_title')}
        description={t('wizard.fx.no_chars_desc')}
        action={
          <Button to="/manage/characteristics" variant="secondary">
            {t('wizard.fx.no_chars_cta')}
          </Button>
        }
      />
    )
  }

  return (
    <div className="space-y-5">
      <p className="text-label text-muted">{t('wizard.fx.intro')}</p>
      {errors.questions && (
        <Alert tone="danger" title={errText(t, errors.questions)}>
          {t('wizard.fx.measurable_hint')}
        </Alert>
      )}
      <ol className="space-y-5">
        {state.questions.map((q, qi) => (
          <li key={q.key} className="min-w-0">
            <div className="mb-2 flex items-start gap-2">
              <span className="flex size-7 shrink-0 items-center justify-center rounded-full bg-primary-soft text-caption font-bold text-primary">{qi + 1}</span>
              <h3 className="min-w-0 flex-1 text-label font-semibold text-ink">{q.text.trim() || t('wizard.q.untitled')}</h3>
              <Badge tone="info">{t(`wizard.type.${q.type}`)}</Badge>
            </div>
            {q.type === 'rating' && <p className="mb-2 text-caption text-muted">{t('wizard.fx.rating_hint')}</p>}
            <ul className="space-y-2">
              {effectiveOptions(q).map((o, oi) => (
                <OptionFx key={o.key} q={q} qi={qi} o={o} oi={oi} chars={list} errors={errors} update={update} defaultOpen={qi === 0 && oi === 0} />
              ))}
            </ul>
          </li>
        ))}
      </ol>
      <p className="text-caption text-muted">
        {t('wizard.fx.missing_char')}{' '}
        <Link to="/manage/characteristics" className="underline">
          {t('wizard.fx.missing_char_link')}
        </Link>
      </p>
    </div>
  )
}
