// Registration, step 1 of 2 (step 2 is the mandatory avatar, AvatarOnboardingPage).
// The invitation code comes from `?code=` (disabled, muted input) or is typed by hand. It is checked
// through GET /api/register/options, which also supplies the department list (never free text).
// An invalid code blocks the form; the server re-validates atomically on submit.
import { useCallback, useEffect, useRef, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { fieldErrors } from '../../api/client'
import * as endpoints from '../../api/endpoints'
import { useAuth } from '../../auth/context'
import { useTranslation } from '../../i18n'
import { errorMessage } from '../../lib/errors'
import Alert from '../../ui/Alert'
import Button from '../../ui/Button'
import Input from '../../ui/Input'
import Select from '../../ui/Select'
import { fieldMessage, validateRegisterForm } from './formErrors'

const EMPTY = { code: '', fullName: '', jobTitle: '', departmentId: '', login: '', password: '' }
const FIELD_ORDER = ['code', 'fullName', 'jobTitle', 'departmentId', 'login', 'password']

export default function RegisterPage() {
  const { t } = useTranslation()
  const { register } = useAuth()
  const [params] = useSearchParams()
  const codeParam = (params.get('code') || '').trim()
  const prefilled = codeParam !== ''

  const [form, setForm] = useState({ ...EMPTY, code: codeParam })
  const [errs, setErrs] = useState({})
  const [formError, setFormError] = useState('')
  const [busy, setBusy] = useState(false)
  // 'idle' (not checked yet) | 'checking' | 'valid' | 'invalid' | 'error' (the check itself failed)
  const [codeStatus, setCodeStatus] = useState(prefilled ? 'checking' : 'idle')
  const [departments, setDepartments] = useState([])
  const seq = useRef(0)
  const checked = useRef('')
  const formRef = useRef(null)

  const verify = useCallback(
    async (code) => {
      const mine = ++seq.current
      checked.current = code
      setCodeStatus('checking')
      try {
        const res = await endpoints.auth.registerOptions(code)
        if (mine !== seq.current) return null
        if (res?.valid) {
          setDepartments(Array.isArray(res.departments) ? res.departments : [])
          setCodeStatus('valid')
          return 'valid'
        }
        setDepartments([])
        setCodeStatus('invalid')
        return 'invalid'
      } catch (e) {
        if (mine !== seq.current) return null
        setDepartments([])
        setCodeStatus('error')
        checked.current = ''
        setFormError(errorMessage(t, e))
        return 'error'
      }
    },
    [t],
  )

  useEffect(() => {
    if (codeParam) verify(codeParam)
    // The code in the URL is checked once per URL value.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [codeParam])

  const set = (k) => (e) => {
    const value = e.target.value
    setForm((f) => ({ ...f, [k]: value }))
    setErrs((x) => (x[k] ? { ...x, [k]: undefined } : x))
  }

  const onCodeChange = (e) => {
    const value = e.target.value
    seq.current += 1 // drop any check in flight
    checked.current = ''
    setForm((f) => ({ ...f, code: value, departmentId: '' }))
    setDepartments([])
    setCodeStatus('idle')
    setFormError('')
    setErrs((x) => (x.code ? { ...x, code: undefined } : x))
  }

  const onCodeBlur = () => {
    const code = form.code.trim()
    if (!prefilled && code && code !== checked.current) verify(code)
  }

  const focusFirst = (errors) => {
    const first = FIELD_ORDER.find((f) => errors[f])
    if (first) formRef.current?.elements.namedItem(first)?.focus()
  }

  async function onSubmit(e) {
    e.preventDefault()
    if (busy || codeStatus === 'invalid') return
    setFormError('')
    const codes = validateRegisterForm(form)
    const local = {}
    for (const [field, code] of Object.entries(codes)) local[field] = fieldMessage(t, field, code)
    setErrs(local)
    if (Object.keys(local).length) return focusFirst(local)

    setBusy(true)
    const code = form.code.trim()
    if (codeStatus !== 'valid' || checked.current !== code) {
      const res = await verify(code)
      if (res !== 'valid') return setBusy(false)
    }
    try {
      await register({
        code,
        login: form.login.trim(),
        password: form.password,
        fullName: form.fullName.trim(),
        jobTitle: form.jobTitle.trim(),
        ...(form.departmentId ? { departmentId: Number(form.departmentId) } : {}),
      })
      // AuthContext now holds the new session; PublicOnly sends the user to the avatar step.
    } catch (err) {
      setBusy(false)
      if (err?.status === 410) {
        setCodeStatus('invalid')
      } else if (err?.status === 409 && err.code === 'login_taken') {
        const next = { login: t('errors.login_taken') }
        setErrs(next)
        focusFirst(next)
      } else if (err?.status === 422) {
        const next = {}
        for (const [path, c] of Object.entries(fieldErrors(err))) next[path] = fieldMessage(t, path, c)
        setErrs(next)
        if (Object.keys(next).length) focusFirst(next)
        else setFormError(errorMessage(t, err))
      } else {
        setFormError(errorMessage(t, err))
      }
    }
  }

  const locked = codeStatus !== 'valid'
  const errorCount = Object.values(errs).filter(Boolean).length

  const codeHint =
    codeStatus === 'checking'
      ? t('auth.register.code_checking')
      : codeStatus === 'valid'
        ? t('auth.register.code_ok')
        : prefilled
          ? t('auth.register.code_hint_prefilled')
          : t('auth.register.code_hint_manual')

  return (
    <div className="min-w-0">
      <p className="text-label font-semibold uppercase tracking-wide text-primary">{t('auth.register.step', { n: 1 })}</p>
      <h1 className="mt-1 text-display font-bold text-ink">{t('auth.register.title')}</h1>
      <p className="mt-2 text-body text-muted">{t('auth.register.subtitle')}</p>

      <form ref={formRef} onSubmit={onSubmit} noValidate className="mt-6 space-y-4" aria-busy={busy || undefined}>
        {codeStatus === 'invalid' && (
          <Alert tone="danger">
            <p>{t('auth.invite_invalid')}</p>
            {prefilled && (
              <p className="mt-2">
                <Link to="/register" className="font-semibold underline underline-offset-2">
                  {t('auth.register.enter_manually')}
                </Link>
              </p>
            )}
          </Alert>
        )}
        {formError && <Alert tone="danger">{formError}</Alert>}
        {errorCount > 1 && <Alert tone="danger">{t('auth.summary_errors', { count: errorCount })}</Alert>}

        <Input
          label={t('auth.register.code')}
          name="code"
          required
          value={form.code}
          onChange={onCodeChange}
          onBlur={onCodeBlur}
          disabled={prefilled}
          readOnly={prefilled}
          autoComplete="off"
          autoCapitalize="characters"
          spellCheck={false}
          hint={codeHint}
          error={errs.code}
          inputClassName="font-mono tracking-wide"
        />

        <div className="grid gap-4 sm:grid-cols-2">
          <Input
            className="sm:col-span-2"
            label={t('auth.register.full_name')}
            name="fullName"
            required
            autoComplete="name"
            value={form.fullName}
            onChange={set('fullName')}
            error={errs.fullName}
          />
          <Input
            label={t('auth.register.job_title')}
            name="jobTitle"
            required
            autoComplete="organization-title"
            value={form.jobTitle}
            onChange={set('jobTitle')}
            error={errs.jobTitle}
          />
          <Select
            label={t('auth.register.department')}
            optionalLabel={t('auth.register.department_optional')}
            name="departmentId"
            value={form.departmentId}
            onChange={set('departmentId')}
            disabled={locked || departments.length === 0}
            hint={locked ? t('auth.register.department_locked') : undefined}
            error={errs.departmentId}
          >
            <option value="">{t('auth.register.department_none')}</option>
            {departments.map((d) => (
              <option key={d.id} value={d.id}>
                {d.name}
              </option>
            ))}
          </Select>
          <Input
            label={t('auth.register.login')}
            name="login"
            required
            autoComplete="username"
            autoCapitalize="none"
            spellCheck={false}
            value={form.login}
            onChange={set('login')}
            hint={t('auth.register.login_hint')}
            error={errs.login}
          />
          <Input
            label={t('auth.register.password')}
            name="password"
            type="password"
            required
            autoComplete="new-password"
            value={form.password}
            onChange={set('password')}
            hint={t('auth.register.password_hint')}
            error={errs.password}
          />
        </div>

        <div className="pt-2">
          <Button type="submit" fullWidth loading={busy} disabled={codeStatus === 'invalid' || codeStatus === 'checking'}>
            {t('auth.register.submit')}
          </Button>
          <p className="mt-2 text-center text-caption text-muted">{t('auth.register.next_hint')}</p>
        </div>
      </form>

      <p className="mt-6 text-label text-muted">
        {t('auth.register.has_account')}{' '}
        <Link to="/login" className="font-semibold underline underline-offset-2">
          {t('auth.register.go_login')}
        </Link>
      </p>
    </div>
  )
}
