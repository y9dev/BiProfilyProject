// Guided assessment creation (Step 6F). Four steps: Info -> Questions -> Effects -> Review.
// Client validation is a UX mirror only; the server's 422 {errors:[{path,code}]} is authoritative and is
// mapped back onto the exact field, jumping to the first step that has an error. Draft: bip:v1:wizard:{userId}.
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { ApiError } from '../../../api/client'
import * as endpoints from '../../../api/endpoints'
import { useAuth } from '../../../auth/context'
import { useTranslation } from '../../../i18n'
import { cx } from '../../../lib/cx'
import { errorMessage } from '../../../lib/errors'
import { getJSON, keys, removeItem, setJSON } from '../../../lib/storage'
import { useAsync } from '../../../lib/useAsync'
import Alert from '../../../ui/Alert'
import Badge from '../../../ui/Badge'
import Button from '../../../ui/Button'
import Card from '../../../ui/Card'
import ConfirmDialog from '../../../ui/ConfirmDialog'
import PageHeader from '../../../ui/PageHeader'
import { useToast } from '../../../ui/toastContext'
import StepEffects from './StepEffects'
import StepMeta from './StepMeta'
import StepQuestions from './StepQuestions'
import StepReview from './StepReview'
import { CheckMark } from './parts'
import { TEMPLATES, buildTemplateState } from './templates'
import { charName } from './helpers'
import {
  buildPayload,
  effectiveOptions,
  errorsForStep,
  firstErrorStep,
  initialState,
  isPristine,
  normalizeServerErrors,
  sanitizeDraft,
  validateAll,
  validateEffects,
  validateMeta,
  validateQuestions,
} from './validate'

const STEPS = ['meta', 'questions', 'effects', 'review']

function Stepper({ step, maxReached, onGo, t }) {
  return (
    <nav aria-label={t('wizard.progress_label')} className="mb-5">
      <p className="mb-2 text-label font-semibold text-ink sm:hidden">
        {t('wizard.step_of', { n: step + 1, total: STEPS.length })}
        {' · '}
        {t(`wizard.steps.${STEPS[step]}`)}
      </p>
      <ol className="grid grid-cols-4 gap-1.5 sm:gap-3">
        {STEPS.map((k, i) => {
          const done = i < step
          const current = i === step
          const reachable = i <= maxReached
          return (
            <li key={k} className="min-w-0">
              <div className={cx('h-1.5 rounded-full', done || current ? 'bg-primary' : 'bg-line-strong')} aria-hidden="true" />
              <button
                type="button"
                disabled={!reachable}
                onClick={() => onGo(i)}
                aria-current={current ? 'step' : undefined}
                className={cx(
                  'mt-2 hidden min-h-touch w-full items-center gap-2 rounded-ctl px-1 text-left text-label sm:flex',
                  current ? 'font-bold text-ink' : 'font-medium text-muted',
                  reachable && !current && 'hover:bg-surface-2',
                  !reachable && 'cursor-not-allowed opacity-60',
                )}
              >
                <span
                  className={cx(
                    'flex size-7 shrink-0 items-center justify-center rounded-full border text-caption font-bold',
                    done ? 'border-primary bg-primary text-primary-ink' : current ? 'border-primary text-primary' : 'border-line-strong',
                  )}
                >
                  {done ? <CheckMark width={16} height={16} /> : i + 1}
                </span>
                <span className="min-w-0 break-words">{t(`wizard.steps.${k}`)}</span>
              </button>
              <span className="sr-only sm:hidden">{t(`wizard.steps.${k}`)}</span>
            </li>
          )
        })}
      </ol>
    </nav>
  )
}

function Outline({ state, chars, goTo, focusQuestion, t, lang }) {
  const qErrors = validateQuestions(state)
  const list = chars.data || []
  const used = new Set()
  for (const q of state.questions) for (const o of effectiveOptions(q)) for (const id of Object.keys(o.fx)) used.add(id)
  const usedChars = list.filter((c) => used.has(String(c.id)))
  const metaOk = Object.keys(validateMeta(state)).length === 0

  return (
    <aside aria-label={t('wizard.outline.title')} className="hidden min-w-0 xl:block">
      <div className="sticky top-4 max-h-[calc(100vh-2rem)] space-y-4 overflow-y-auto rounded-card border border-line bg-surface p-4">
        <h2 className="text-label font-bold uppercase tracking-wide text-muted">{t('wizard.outline.title')}</h2>
        <button type="button" onClick={() => goTo(0)} className="block w-full min-w-0 rounded-ctl p-1 text-left hover:bg-surface-2">
          <span className="block break-words text-label font-semibold text-ink">{state.title.trim() || t('wizard.outline.no_title')}</span>
          <span className="mt-1 flex flex-wrap gap-1.5">
            <Badge tone={state.hidden ? 'warning' : 'success'}>{state.hidden ? t('wizard.meta.hidden') : t('wizard.meta.visible')}</Badge>
            {!metaOk && <Badge tone="warning">{t('wizard.outline.incomplete')}</Badge>}
          </span>
        </button>
        <ol className="space-y-1">
          {state.questions.map((q, qi) => {
            const incomplete = Object.keys(qErrors).some((p) => p.startsWith(`questions[${qi}]`))
            const fxCount = effectiveOptions(q).filter((o) => Object.keys(o.fx).length).length
            return (
              <li key={q.key}>
                <button type="button" onClick={() => focusQuestion(q.key)} className="flex w-full min-w-0 items-start gap-2 rounded-ctl p-1 text-left hover:bg-surface-2">
                  <span
                    className={cx(
                      'mt-0.5 flex size-6 shrink-0 items-center justify-center rounded-full text-caption font-bold',
                      incomplete ? 'bg-warning-soft text-warning' : 'bg-success-soft text-success',
                    )}
                  >
                    {qi + 1}
                  </span>
                  <span className="min-w-0 flex-1">
                    <span className="block break-words text-caption text-ink">{q.text.trim() || t('wizard.q.untitled')}</span>
                    <span className="block text-caption text-muted">
                      {t(`wizard.type.${q.type}`)}
                      {' · '}
                      {t('wizard.outline.fx', { n: fxCount, total: effectiveOptions(q).length })}
                    </span>
                  </span>
                </button>
              </li>
            )
          })}
        </ol>
        <div>
          <h3 className="mb-1 text-caption font-bold text-muted">{t('wizard.outline.measures')}</h3>
          {usedChars.length ? (
            <ul className="flex flex-wrap gap-1.5">
              {usedChars.map((c) => (
                <li key={c.id}>
                  <Badge>{charName(c, lang)}</Badge>
                </li>
              ))}
            </ul>
          ) : (
            <p className="text-caption text-muted">{t('wizard.outline.none')}</p>
          )}
        </div>
      </div>
    </aside>
  )
}

export default function AssessmentWizard() {
  const { t, lang } = useTranslation()
  const { me } = useAuth()
  const navigate = useNavigate()
  const toast = useToast()
  const [searchParams, setSearchParams] = useSearchParams()
  const templateId = searchParams.get('template')
  const draftKey = keys.wizard(me?.id ?? 'anon')

  const chars = useAsync((signal) => endpoints.characteristics.list({ signal }), [])
  const knownIds = useMemo(() => (chars.status === 'ready' ? new Set((chars.data || []).map((c) => c.id)) : null), [chars.status, chars.data])

  const [restored, setRestored] = useState(false)
  const [state, setState] = useState(() => {
    if (templateId) return initialState() // replaced by the template once the characteristics are loaded
    const saved = sanitizeDraft(getJSON(draftKey))
    return saved && !isPristine(saved) ? saved : initialState()
  })
  const [maxReached, setMaxReached] = useState(state.step)
  const [attempted, setAttempted] = useState({})
  const [serverErrors, setServerErrors] = useState({})
  const [submitError, setSubmitError] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [discarding, setDiscarding] = useState(false)
  const [focusKey, setFocusKey] = useState(null)
  const [focusTick, setFocusTick] = useState(0)
  const refsRef = useRef({})
  const inFlightRef = useRef(false) // synchronous guard: state updates are too late for a fast double click
  const idemKeyRef = useRef(null) // one key per distinct definition, reused by retries
  const topRef = useRef(null)
  const step = state.step

  useEffect(() => {
    if (templateId) return
    const saved = sanitizeDraft(getJSON(draftKey))
    if (saved && !isPristine(saved)) setRestored(true)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  // "Create from template": prefill this same wizard with a brand-new draft (nothing is created yet).
  const templateApplied = useRef(false)
  useEffect(() => {
    if (!templateId || templateApplied.current || chars.status !== 'ready') return
    templateApplied.current = true
    const template = TEMPLATES[templateId]
    const built = template ? buildTemplateState(template, chars.data || []) : { missing: [] }
    if (built.state) {
      setState(built.state)
      setMaxReached(0)
      toast.success(t('wizard.template.applied'))
    } else if (built.missing?.length) {
      toast.error(t('wizard.template.missing', { names: built.missing.join(', ') }))
    }
    setSearchParams({}, { replace: true })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [templateId, chars.status])

  // Debounced draft save; a pristine wizard leaves nothing behind.
  useEffect(() => {
    const id = setTimeout(() => {
      if (isPristine(state)) removeItem(draftKey)
      else setJSON(draftKey, state)
    }, 400)
    return () => clearTimeout(id)
  }, [state, draftKey])

  // Drop effects that point at characteristics deleted since the draft was saved.
  useEffect(() => {
    if (!knownIds) return
    const stale = state.questions.some((q) => [...q.options, ...q.scale].some((o) => Object.keys(o.fx).some((id) => !knownIds.has(Number(id)))))
    if (!stale) return
    setState((s) => {
      const d = structuredClone(s)
      for (const q of d.questions)
        for (const o of [...q.options, ...q.scale])
          for (const id of Object.keys(o.fx)) if (!knownIds.has(Number(id))) delete o.fx[id]
      return d
    })
  }, [knownIds, state.questions])

  const update = useCallback((fn) => {
    setServerErrors({})
    setSubmitError('')
    idemKeyRef.current = null
    setState((s) => {
      const d = structuredClone(s)
      fn(d)
      return d
    })
  }, [])

  const clientErrors = useMemo(() => validateAll(state, knownIds), [state, knownIds])
  const errors = useMemo(
    () => ({ ...(attempted[step] ? errorsForStep(clientErrors, step) : {}), ...errorsForStep(serverErrors, step) }),
    [attempted, step, clientErrors, serverErrors],
  )
  const errorCount = Object.keys(errors).length

  // After a jump: focus the first invalid field of the (new) step.
  useEffect(() => {
    if (!focusTick) return
    const id = setTimeout(() => {
      const el = topRef.current?.querySelector('[aria-invalid="true"], [data-error]')
      const target = el || topRef.current
      target?.scrollIntoView({ block: 'center', behavior: 'smooth' })
      const focusable = el?.matches('input,select,textarea,button') ? el : el?.querySelector('input,select,textarea,button')
      focusable?.focus({ preventScroll: true })
    }, 120)
    return () => clearTimeout(id)
  }, [focusTick])

  const goTo = useCallback((n) => {
    setState((s) => ({ ...s, step: n }))
    setMaxReached((m) => Math.max(m, n))
    window.scrollTo({ top: 0, behavior: 'smooth' })
  }, [])

  const stepErrors = (n) => {
    if (n === 0) return validateMeta(state)
    if (n === 1) return validateQuestions(state)
    if (n === 2) return validateEffects(state, knownIds)
    return {}
  }

  const next = () => {
    setAttempted((a) => ({ ...a, [step]: true }))
    if (step === 2 && chars.status !== 'ready') return
    if (Object.keys(stepErrors(step)).length) {
      setFocusTick((n) => n + 1)
      return
    }
    goTo(step + 1)
  }

  const openStepFromStepper = (i) => {
    if (i <= step) goTo(i)
    else if (i <= maxReached) goTo(i)
  }

  const submit = async () => {
    if (inFlightRef.current) return
    const all = validateAll(state, knownIds)
    if (Object.keys(all).length) {
      setAttempted({ 0: true, 1: true, 2: true })
      goTo(firstErrorStep(all))
      setFocusTick((n) => n + 1)
      toast.error(t('wizard.err.fix_first'))
      return
    }
    const { payload, refs } = buildPayload(state, knownIds)
    refsRef.current = refs
    inFlightRef.current = true
    setSubmitting(true)
    setSubmitError('')
    if (!idemKeyRef.current) idemKeyRef.current = crypto.randomUUID()
    try {
      const res = await endpoints.manage.createAssessment(payload, idemKeyRef.current)
      removeItem(draftKey)
      idemKeyRef.current = null
      const warned = (res?.warnings || []).filter((w) => w.code === 'min_unreachable')
      toast.success(t('wizard.done'))
      if (warned.length) {
        const byId = new Map((chars.data || []).map((c) => [c.id, c]))
        const names = [...new Set(warned.map((w) => charName(byId.get(w.characteristicId), lang)).filter(Boolean))].join(', ')
        toast.push(t('wizard.done_min_unreachable', { names: names || '—' }), { tone: 'warning', duration: 15000 })
      }
      navigate('/manage/assessments')
    } catch (err) {
      inFlightRef.current = false // success keeps the guard up: the page is navigating away
      if (err instanceof ApiError && err.status === 422 && Array.isArray(err.errors) && err.errors.length) {
        const mapped = normalizeServerErrors(err.errors, refsRef.current, state)
        setServerErrors(mapped)
        const target = firstErrorStep(mapped)
        goTo(target ?? 0)
        setFocusTick((n) => n + 1)
        toast.error(t('wizard.err.server_rejected', { n: Object.keys(mapped).length }))
      } else {
        setSubmitError(errorMessage(t, err))
      }
    } finally {
      setSubmitting(false)
    }
  }

  const discard = () => {
    removeItem(draftKey)
    setState(initialState())
    setAttempted({})
    setServerErrors({})
    setMaxReached(0)
    setRestored(false)
    setDiscarding(false)
    window.scrollTo({ top: 0 })
  }

  const focusQuestion = (key) => {
    goTo(1)
    setFocusKey(key)
  }

  const last = step === STEPS.length - 1

  return (
    <>
      <PageHeader
        title={t('pages.wizard.title')}
        description={t('wizard.subtitle')}
        actions={
          <>
            <Button variant="secondary" to="/manage/assessments">
              {t('wizard.cancel')}
            </Button>
            {!isPristine(state) && (
              <Button variant="danger-outline" onClick={() => setDiscarding(true)}>
                {t('wizard.draft.discard')}
              </Button>
            )}
          </>
        }
      />

      {restored && (
        <Alert tone="info" className="mb-4">
          <span className="flex flex-wrap items-center gap-x-3 gap-y-1">
            <span className="min-w-0">{t('wizard.draft.restored')}</span>
            <button type="button" className="text-label font-semibold underline" onClick={() => setRestored(false)}>
              {t('common.dismiss')}
            </button>
          </span>
        </Alert>
      )}

      <Stepper step={step} maxReached={maxReached} onGo={openStepFromStepper} t={t} />

      <div className="grid items-start gap-6 xl:grid-cols-[minmax(0,1fr)_20rem] 2xl:grid-cols-[minmax(0,1fr)_24rem]">
        <Card className="min-w-0" aria-labelledby="wizard-step-title">
          <div ref={topRef}>
            <h2 id="wizard-step-title" className="mb-1 text-title font-semibold text-ink" tabIndex={-1}>
              {t(`wizard.steps.${STEPS[step]}`)}
            </h2>
            <p className="mb-5 text-label text-muted">{t(`wizard.step_hint.${STEPS[step]}`)}</p>

            {errorCount > 0 && (
              <Alert tone="danger" className="mb-5" title={t('wizard.err.summary', { n: errorCount })}>
                {t('wizard.err.summary_hint')}
              </Alert>
            )}

            {step === 0 && <StepMeta state={state} update={update} errors={errors} />}
            {step === 1 && <StepQuestions state={state} update={update} errors={errors} focusKey={focusKey} clearFocus={() => setFocusKey(null)} />}
            {step === 2 && <StepEffects state={state} update={update} errors={errors} chars={chars} />}
            {step === 3 && <StepReview state={state} chars={chars} goTo={goTo} submitError={submitError} />}
          </div>

          <div className="sticky bottom-0 z-10 -mx-4 -mb-4 mt-6 flex flex-wrap items-center justify-between gap-2 rounded-b-card border-t border-line bg-surface px-4 py-3 sm:-mx-6 sm:-mb-6 sm:px-6">
            <Button variant="secondary" onClick={() => goTo(step - 1)} disabled={step === 0 || submitting}>
              {t('common.back')}
            </Button>
            {last ? (
              <Button onClick={submit} loading={submitting} disabled={chars.status !== 'ready'}>
                {submitting ? t('wizard.creating') : t('wizard.create')}
              </Button>
            ) : (
              <Button onClick={next}>{t('common.next')}</Button>
            )}
          </div>
        </Card>

        <Outline state={state} chars={chars} goTo={goTo} focusQuestion={focusQuestion} t={t} lang={lang} />
      </div>

      <ConfirmDialog
        isOpen={discarding}
        title={t('wizard.draft.discard_title')}
        message={t('wizard.draft.discard_msg')}
        confirmLabel={t('wizard.draft.discard_confirm')}
        onCancel={() => setDiscarding(false)}
        onConfirm={discard}
      />
    </>
  )
}
