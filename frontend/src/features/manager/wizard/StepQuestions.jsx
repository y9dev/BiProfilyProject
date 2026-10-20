// Step 2: add / remove / reorder questions; per type editor (single, multiple, rating scale).
import { useEffect, useState } from 'react'
import { useTranslation } from '../../../i18n'
import { cx } from '../../../lib/cx'
import Badge from '../../../ui/Badge'
import Button from '../../../ui/Button'
import ConfirmDialog from '../../../ui/ConfirmDialog'
import Input from '../../../ui/Input'
import Textarea from '../../../ui/Textarea'
import { LIMITS, POINT_CHOICES, newOption, newQuestion, pointText } from './validate'
import { ArrowDownIcon, ArrowUpIcon, ChevronIcon, IconButton, PlusIcon, RadioCards, TrashIcon } from './parts'
import { errText } from './helpers'

const hasErr = (errors, prefix) => Object.keys(errors).some((p) => p === prefix || p.startsWith(`${prefix}.`) || p.startsWith(`${prefix}[`))

function QuestionCard({ q, qi, total, open, onToggle, errors, update, onRemove }) {
  const { t } = useTranslation()
  const base = `questions[${qi}]`
  const bad = hasErr(errors, base)
  const move = (dir) =>
    update((d) => {
      const [item] = d.questions.splice(qi, 1)
      d.questions.splice(qi + dir, 0, item)
    })
  const setQ = (fn) => update((d) => fn(d.questions[qi]))

  return (
    <li id={`wq-${q.key}`} className={cx('rounded-card border bg-surface', bad ? 'border-danger' : 'border-line')} data-error={bad || undefined}>
      <div className="flex items-start gap-1 p-2 sm:p-3">
        <button
          type="button"
          onClick={onToggle}
          aria-expanded={open}
          aria-controls={`wq-body-${q.key}`}
          className="flex min-h-touch min-w-0 flex-1 items-start gap-3 rounded-ctl p-1 text-left hover:bg-surface-2"
        >
          <span className="mt-0.5 flex size-8 shrink-0 items-center justify-center rounded-full bg-primary-soft text-label font-bold text-primary">{qi + 1}</span>
          <span className="min-w-0 flex-1">
            <span className={cx('block break-words text-label font-semibold', q.text.trim() ? 'text-ink' : 'text-muted')}>{q.text.trim() || t('wizard.q.untitled')}</span>
            <span className="mt-1 flex flex-wrap items-center gap-1.5">
              <Badge tone="info">{t(`wizard.type.${q.type}`)}</Badge>
              {bad && <Badge tone="danger">{t('wizard.q.has_errors')}</Badge>}
            </span>
          </span>
          <ChevronIcon open={open} className="mt-2 shrink-0 text-muted" />
        </button>
        <div className="flex shrink-0 flex-col sm:flex-row">
          <IconButton label={t('wizard.q.move_up')} disabled={qi === 0} onClick={() => move(-1)}>
            <ArrowUpIcon />
          </IconButton>
          <IconButton label={t('wizard.q.move_down')} disabled={qi === total - 1} onClick={() => move(1)}>
            <ArrowDownIcon />
          </IconButton>
          <IconButton label={t('wizard.q.remove')} tone="danger" onClick={onRemove}>
            <TrashIcon />
          </IconButton>
        </div>
      </div>

      {open && (
        <div id={`wq-body-${q.key}`} className="space-y-5 border-t border-line p-3 sm:p-4">
          <Textarea
            label={t('wizard.q.text_label')}
            required
            rows={2}
            value={q.text}
            placeholder={t('wizard.q.text_ph')}
            onChange={(e) => setQ((x) => void (x.text = e.target.value))}
            error={errText(t, errors[`${base}.text`])}
          />
          <RadioCards
            legend={t('wizard.q.type_legend')}
            value={q.type}
            columns="sm:grid-cols-3"
            onChange={(v) => setQ((x) => void (x.type = v))}
            options={['single', 'multiple', 'rating'].map((k) => ({ value: k, label: t(`wizard.type.${k}`), hint: t(`wizard.type.${k}_hint`) }))}
          />

          {q.type !== 'rating' ? (
            <fieldset className="min-w-0" aria-invalid={errors[`${base}.options`] ? true : undefined}>
              <legend className="mb-2 text-label font-medium text-ink">{t('wizard.q.options_legend')}</legend>
              <ol className="space-y-2">
                {q.options.map((o, oi) => (
                  <li key={o.key} className="flex items-start gap-1">
                    <span aria-hidden="true" className={cx('mt-3 size-4 shrink-0 border-2 border-line-strong', q.type === 'single' ? 'rounded-full' : 'rounded-sm')} />
                    <Input
                      className="flex-1"
                      aria-label={t('wizard.q.option_label', { n: oi + 1 })}
                      placeholder={t('wizard.q.option_ph', { n: oi + 1 })}
                      value={o.text}
                      onChange={(e) => setQ((x) => void (x.options[oi].text = e.target.value))}
                      error={errText(t, errors[`${base}.options[${oi}].text`])}
                      autoComplete="off"
                    />
                    <IconButton
                      label={t('wizard.q.remove_option', { n: oi + 1 })}
                      tone="danger"
                      disabled={q.options.length <= LIMITS.minOptions}
                      onClick={() => setQ((x) => void x.options.splice(oi, 1))}
                    >
                      <TrashIcon />
                    </IconButton>
                  </li>
                ))}
              </ol>
              {errors[`${base}.options`] && <p className="mt-2 text-caption font-medium text-danger">{errText(t, errors[`${base}.options`])}</p>}
              <div className="mt-3 flex flex-wrap items-center gap-3">
                <Button variant="secondary" disabled={q.options.length >= LIMITS.maxOptions} onClick={() => setQ((x) => void x.options.push(newOption()))}>
                  <PlusIcon /> {t('wizard.q.add_option')}
                </Button>
                <span className="text-caption text-muted">{t('wizard.q.options_hint', { min: LIMITS.minOptions, max: LIMITS.maxOptions })}</span>
              </div>
            </fieldset>
          ) : (
            <div className="space-y-4">
              <RadioCards
                legend={t('wizard.q.points_legend')}
                value={q.points}
                columns="grid-cols-2"
                onChange={(v) => setQ((x) => void (x.points = v))}
                options={POINT_CHOICES.map((n) => ({ value: n, label: t('wizard.q.points_n', { n }) }))}
              />
              <div className="grid gap-4 sm:grid-cols-2">
                <Input
                  label={t('wizard.q.low_label')}
                  optionalLabel={t('common.optional')}
                  placeholder={t('wizard.q.low_ph')}
                  maxLength={LIMITS.label}
                  value={q.low}
                  onChange={(e) => setQ((x) => void (x.low = e.target.value))}
                  error={errText(t, errors[`${base}.labels`])}
                />
                <Input
                  label={t('wizard.q.high_label')}
                  optionalLabel={t('common.optional')}
                  placeholder={t('wizard.q.high_ph')}
                  maxLength={LIMITS.label}
                  value={q.high}
                  onChange={(e) => setQ((x) => void (x.high = e.target.value))}
                />
              </div>
              <div>
                <p className="mb-1 text-caption font-medium text-muted">{t('wizard.q.scale_preview')}</p>
                <ol className="flex flex-wrap gap-1.5" aria-label={t('wizard.q.scale_preview')}>
                  {Array.from({ length: q.points }, (_, i) => (
                    <li key={i} className="flex size-9 items-center justify-center rounded-full border border-line-strong bg-surface-2 text-label font-semibold text-ink">
                      {i + 1}
                    </li>
                  ))}
                </ol>
                <p className="mt-2 break-words text-caption text-muted">
                  {pointText(q, 0)} &hellip; {pointText(q, q.points - 1)}
                </p>
              </div>
            </div>
          )}
        </div>
      )}
    </li>
  )
}

export default function StepQuestions({ state, update, errors, focusKey, clearFocus }) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(() => new Set(state.questions.length === 1 ? [state.questions[0].key] : []))
  const [removing, setRemoving] = useState(null)

  // Expand cards that have errors; expand + scroll to a card requested by the outline.
  const errKey = Object.keys(errors).join('|')
  useEffect(() => {
    const bad = state.questions.filter((_, qi) => hasErr(errors, `questions[${qi}]`)).map((q) => q.key)
    if (bad.length) setOpen((s) => new Set([...s, ...bad]))
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [errKey])

  useEffect(() => {
    if (!focusKey) return
    setOpen((s) => new Set([...s, focusKey]))
    const id = requestAnimationFrame(() => {
      document.getElementById(`wq-${focusKey}`)?.scrollIntoView({ block: 'start', behavior: 'smooth' })
      clearFocus?.()
    })
    return () => cancelAnimationFrame(id)
  }, [focusKey, clearFocus])

  const toggle = (key) =>
    setOpen((s) => {
      const n = new Set(s)
      if (n.has(key)) n.delete(key)
      else n.add(key)
      return n
    })

  const add = (type) => {
    const q = newQuestion(type)
    update((d) => void d.questions.push(q))
    setOpen((s) => new Set([...s, q.key]))
    requestAnimationFrame(() => document.getElementById(`wq-${q.key}`)?.scrollIntoView({ block: 'nearest', behavior: 'smooth' }))
  }

  const allOpen = state.questions.every((q) => open.has(q.key))

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <p className="min-w-0 text-label text-muted">{t('wizard.q.intro')}</p>
        {state.questions.length > 1 && (
          <Button variant="ghost" onClick={() => setOpen(allOpen ? new Set() : new Set(state.questions.map((q) => q.key)))}>
            {allOpen ? t('wizard.q.collapse_all') : t('wizard.q.expand_all')}
          </Button>
        )}
      </div>

      {errors.questions && <p className="text-label font-medium text-danger">{errText(t, errors.questions)}</p>}

      <ol className="space-y-3">
        {state.questions.map((q, qi) => (
          <QuestionCard
            key={q.key}
            q={q}
            qi={qi}
            total={state.questions.length}
            open={open.has(q.key)}
            onToggle={() => toggle(q.key)}
            errors={errors}
            update={update}
            onRemove={() => {
              const empty = !q.text.trim() && q.options.every((o) => !o.text.trim())
              if (empty) update((d) => void d.questions.splice(qi, 1))
              else setRemoving(qi)
            }}
          />
        ))}
      </ol>

      <div className="rounded-card border border-dashed border-line-strong p-3">
        <p className="mb-2 text-label font-medium text-ink">{t('wizard.q.add_title')}</p>
        <div className="flex flex-wrap gap-2">
          {['single', 'multiple', 'rating'].map((k) => (
            <Button key={k} variant="secondary" onClick={() => add(k)}>
              <PlusIcon /> {t(`wizard.type.${k}`)}
            </Button>
          ))}
        </div>
      </div>

      <ConfirmDialog
        isOpen={removing !== null}
        title={t('wizard.q.remove_title')}
        message={t('wizard.q.remove_msg', { n: (removing ?? 0) + 1 })}
        confirmLabel={t('wizard.q.remove')}
        onCancel={() => setRemoving(null)}
        onConfirm={() => {
          update((d) => void d.questions.splice(removing, 1))
          setRemoving(null)
        }}
      />
    </div>
  )
}
