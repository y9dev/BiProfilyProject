// Assessment runner: one question at a time, answers in client state + localStorage draft,
// exactly one submit request at the end. The client never computes scores.
import { useEffect, useRef, useState } from 'react'
import { useToast } from '../../ui/toastContext'
import { useNavigate, useParams } from 'react-router-dom'
import { useAuth } from '../../auth/context'
import * as endpoints from '../../api/endpoints'
import { useTranslation } from '../../i18n'
import { errorMessage } from '../../lib/errors'
import { useAsync } from '../../lib/useAsync'
import Alert from '../../ui/Alert'
import Async from '../../ui/Async'
import Button from '../../ui/Button'
import Card from '../../ui/Card'
import EmptyState from '../../ui/EmptyState'
import ErrorState from '../../ui/ErrorState'
import { AlertIcon, CheckIcon } from '../../ui/icons'
import {
  buildPayload,
  clearDraft,
  createDraftSaver,
  isAnswered,
  loadDraft,
} from './draftStore'
import QuestionView from './QuestionView'

function Blocked({ kind, attemptId }) {
  const { t } = useTranslation()
  const done = kind === 'already_completed'
  return (
    <Card>
      <EmptyState
        icon={done ? <CheckIcon width={26} height={26} /> : <AlertIcon width={26} height={26} />}
        title={done ? t('errors.already_completed') : t('employee.take.not_available')}
        description={done ? t('employee.take.already_completed') : ''}
        action={
          <>
            {done && attemptId && <Button to={`/results/${attemptId}`}>{t('employee.take.already_view')}</Button>}
            <Button variant="secondary" to="/">
              {t('employee.take.to_dashboard')}
            </Button>
          </>
        }
      />
    </Card>
  )
}

function Runner({ view, userId, assessmentId, onBlocked }) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const toast = useToast()
  const [restored] = useState(() => loadDraft(userId, assessmentId, view))
  const [idx, setIdx] = useState(restored?.idx ?? 0)
  const [answers, setAnswers] = useState(restored?.answers ?? {})
  const [showRestored, setShowRestored] = useState(() => !!restored && Object.keys(restored.answers).length > 0)
  const [error, setError] = useState(null)
  const [submitting, setSubmitting] = useState(false)
  const submittedRef = useRef(false)
  const stateRef = useRef(null)
  const [saver] = useState(() => createDraftSaver(userId, assessmentId, view))
  const headingRef = useRef(null)
  const firstRender = useRef(true)

  useEffect(() => {
    const s = saver
    const flush = () => {
      if (!submittedRef.current) s.flush()
    }
    window.addEventListener('pagehide', flush)
    return () => {
      window.removeEventListener('pagehide', flush)
      flush()
    }
  }, [saver])

  useEffect(() => {
    if (firstRender.current) {
      firstRender.current = false
      return
    }
    headingRef.current?.focus()
  }, [idx])

  const questions = view.questions
  const q = questions[idx]
  const last = idx === questions.length - 1
  const answeredCount = questions.filter((x) => isAnswered(answers, x)).length
  const errorId = 'take-error'

  const update = (next) => {
    stateRef.current = next
    setIdx(next.idx)
    setAnswers(next.answers)
    saver.save(next)
  }

  const onAnswer = (ids) => {
    setError(null)
    setShowRestored(false)
    const next = { ...answers }
    if (ids.length) next[q.id] = ids
    else delete next[q.id]
    update({ idx, answers: next })
  }

  const go = (delta) => {
    setError(null)
    setShowRestored(false)
    update({ idx: idx + delta, answers })
  }

  const onNext = () => {
    if (!isAnswered(answers, q)) return setError({ type: 'required' })
    go(1)
  }

  const onSubmit = async () => {
    if (submittedRef.current) return
    const firstMissing = questions.findIndex((x) => !isAnswered(answers, x))
    if (firstMissing !== -1) {
      setError({ type: 'required_submit', n: firstMissing + 1 })
      if (firstMissing !== idx) update({ idx: firstMissing, answers })
      return
    }
    submittedRef.current = true
    saver.cancel()
    setSubmitting(true)
    setError(null)
    try {
      await endpoints.employee.submit(assessmentId, buildPayload(view, answers))
      clearDraft(userId, assessmentId)
      toast.success(t('employee.take.completed'))
      navigate('/', { replace: true })
    } catch (e) {
      if (e?.status === 409 || e?.status === 404) {
        clearDraft(userId, assessmentId)
        let attemptId = null
        if (e.status === 409) {
          const list = await endpoints.employee.assessments().catch(() => [])
          attemptId = list.find((a) => String(a.id) === String(assessmentId))?.attemptId ?? null
        }
        onBlocked({ kind: e.status === 409 ? 'already_completed' : 'not_available', attemptId })
        return
      }
      submittedRef.current = false
      setSubmitting(false)
      saver.save(stateRef.current)
      setError({ type: 'failed', message: errorMessage(t, e) })
    }
  }

  const errorText =
    error &&
    (error.type === 'required'
      ? t('employee.take.required')
      : error.type === 'required_submit'
        ? t('employee.take.required_submit', { n: error.n })
        : `${t('employee.take.submit_failed')} ${error.message}`)

  return (
    <div className="mx-auto w-full max-w-3xl min-w-0">
      <div className="mb-1 flex flex-wrap items-baseline justify-between gap-x-4 text-label text-muted">
        <h1 className="min-w-0 flex-1 basis-48 text-label font-semibold text-ink break-words">{view.title}</h1>
        <span>{t('employee.take.answered', { n: answeredCount, total: questions.length })}</span>
      </div>
      <div
        role="progressbar"
        aria-label={t('employee.take.progress_aria')}
        aria-valuemin={1}
        aria-valuemax={questions.length}
        aria-valuenow={idx + 1}
        aria-valuetext={t('employee.take.progress', { current: idx + 1, total: questions.length })}
        className="h-2 overflow-hidden rounded-ctl bg-surface-2"
      >
        <div
          className="h-full bg-primary transition-[width] duration-300"
          style={{ width: `${((idx + 1) / questions.length) * 100}%` }}
        />
      </div>
      <p className="mt-2 mb-4 text-caption text-muted" aria-live="polite">
        {t('employee.take.progress', { current: idx + 1, total: questions.length })}
      </p>

      {showRestored && (
        <Alert tone="info" className="mb-4">
          {t('employee.take.restored')}
        </Alert>
      )}

      <Card>
        <div ref={headingRef} tabIndex={-1} className="outline-none">
          <QuestionView
            key={q.id}
            question={q}
            selected={answers[q.id] ?? []}
            onChange={onAnswer}
            invalid={error?.type === 'required' || error?.type === 'required_submit'}
            errorId={errorId}
          />
        </div>
        {error && (
          <Alert tone="danger" className="mt-4">
            <span id={errorId}>{errorText}</span>
          </Alert>
        )}
        <div className="mt-6 flex flex-col-reverse gap-3 sm:flex-row sm:justify-between">
          <Button variant="secondary" onClick={() => go(-1)} disabled={idx === 0 || submitting}>
            {t('employee.take.prev')}
          </Button>
          {last ? (
            <Button onClick={onSubmit} loading={submitting}>
              {submitting ? t('employee.take.submitting') : t('employee.take.submit')}
            </Button>
          ) : (
            <Button onClick={onNext}>{t('employee.take.next')}</Button>
          )}
        </div>
      </Card>
    </div>
  )
}

export default function TakeAssessmentPage() {
  const { id } = useParams()
  const { me } = useAuth()
  const [blocked, setBlocked] = useState(null)
  const { status, data, error, reload } = useAsync(
    async (signal) => {
      try {
        return { view: await endpoints.employee.takeView(id, { signal }) }
      } catch (e) {
        if (e?.name === 'AbortError') throw e
        if (e?.status === 409 || e?.status === 404) {
          clearDraft(me.id, id)
          let attemptId = null
          if (e.status === 409) {
            const list = await endpoints.employee.assessments({ signal }).catch(() => [])
            attemptId = list.find((a) => String(a.id) === String(id))?.attemptId ?? null
          }
          return { blocked: e.status === 409 ? 'already_completed' : 'not_available', attemptId }
        }
        throw e
      }
    },
    [id],
  )

  if (blocked) return <Blocked kind={blocked.kind} attemptId={blocked.attemptId} />
  if (status === 'error' && error) return <ErrorState onRetry={reload} />

  return (
    <Async status={status} onRetry={reload}>
      {data?.blocked && <Blocked kind={data.blocked} attemptId={data.attemptId} />}
      {data?.view && (
        <Runner key={id} view={data.view} userId={me.id} assessmentId={id} onBlocked={setBlocked} />
      )}
    </Async>
  )
}
