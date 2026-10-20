// Sign-in. On success AuthContext sets `me` and the PublicOnly guard moves the user on
// (to the avatar step when the profile is incomplete, else where they wanted to go).
import { useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { useAuth } from '../../auth/context'
import { useTranslation } from '../../i18n'
import { errorMessage } from '../../lib/errors'
import Alert from '../../ui/Alert'
import Button from '../../ui/Button'
import Input from '../../ui/Input'
import { fieldMessage } from './formErrors'

// 401 is deliberately one generic message; blocked accounts (403) and rate limiting (429) get their own.
function loginErrorMessage(t, err) {
  if (err?.status === 401) return t('errors.invalid_credentials')
  if (err?.status === 403) return t('errors.account_blocked')
  return errorMessage(t, err)
}

export default function LoginPage() {
  const { t } = useTranslation()
  const { login } = useAuth()
  const [form, setForm] = useState({ login: '', password: '' })
  const [fieldErrs, setFieldErrs] = useState({})
  const [formError, setFormError] = useState('')
  const [busy, setBusy] = useState(false)
  const loginRef = useRef(null)
  const passwordRef = useRef(null)

  const set = (k) => (e) => setForm((f) => ({ ...f, [k]: e.target.value }))

  async function onSubmit(e) {
    e.preventDefault()
    if (busy) return
    const errs = {}
    if (!form.login.trim()) errs.login = 'required'
    if (!form.password) errs.password = 'required'
    setFieldErrs(errs)
    setFormError('')
    if (errs.login) return loginRef.current?.focus()
    if (errs.password) return passwordRef.current?.focus()
    setBusy(true)
    try {
      await login(form.login.trim(), form.password)
    } catch (err) {
      setFormError(loginErrorMessage(t, err))
      setForm((f) => ({ ...f, password: '' }))
      passwordRef.current?.focus()
      setBusy(false)
    }
  }

  return (
    <div className="min-w-0">
      <h1 className="text-display font-bold text-ink">{t('auth.login.title')}</h1>
      <p className="mt-2 text-body text-muted">{t('auth.login.subtitle')}</p>

      <form onSubmit={onSubmit} noValidate className="mt-6 space-y-4" aria-busy={busy || undefined}>
        {formError && <Alert tone="danger">{formError}</Alert>}
        <Input
          label={t('auth.login.login')}
          name="login"
          autoComplete="username"
          autoCapitalize="none"
          spellCheck={false}
          required
          value={form.login}
          onChange={set('login')}
          error={fieldErrs.login ? fieldMessage(t, 'login', fieldErrs.login) : undefined}
          ref={loginRef}
          autoFocus
        />
        <Input
          label={t('auth.login.password')}
          name="password"
          type="password"
          autoComplete="current-password"
          required
          value={form.password}
          onChange={set('password')}
          error={fieldErrs.password ? fieldMessage(t, 'password', fieldErrs.password) : undefined}
          ref={passwordRef}
        />
        <Button type="submit" fullWidth loading={busy}>
          {t('auth.login.submit')}
        </Button>
      </form>

      <p className="mt-6 text-label text-muted">
        {t('auth.login.no_account')}{' '}
        <Link to="/register" className="font-semibold underline underline-offset-2">
          {t('auth.login.go_register')}
        </Link>
      </p>
    </div>
  )
}
