// Route guards, used as layout routes (they render <Outlet/>).
//   RequireAuth    signed in (else /login, remembering where the user wanted to go)
//   RequireAvatar  profile complete (else /onboarding/avatar); mounted inside RequireAuth
//   RequireRole    one of `roles` (else the forbidden view; never logs the user out)
//   PublicOnly     login/register: signed-in users are sent on to their landing page
import { Navigate, Outlet, useLocation } from 'react-router-dom'
import { useTranslation } from '../i18n'
import ErrorState from '../ui/ErrorState'
import Spinner from '../ui/Spinner'
import { ForbiddenPage } from '../ui/StatusPages'
import { useAuth } from './context'

function FullPageSpinner() {
  const { t } = useTranslation()
  return (
    <div className="flex min-h-screen items-center justify-center bg-bg">
      <Spinner size="lg" label={t('ui.page_loading')} />
    </div>
  )
}

function useBootGate() {
  const { status, refresh } = useAuth()
  const { t } = useTranslation()
  if (status === 'loading') return <FullPageSpinner />
  if (status === 'error') {
    return (
      <div className="flex min-h-screen items-center justify-center bg-bg p-6">
        <ErrorState title={t('errors.network')} onRetry={refresh} />
      </div>
    )
  }
  return null
}

export function RequireAuth() {
  const gate = useBootGate()
  const { me, signedOut } = useAuth()
  const location = useLocation()
  if (gate) return gate
  if (!me) {
    return <Navigate to="/login" replace state={signedOut ? undefined : { from: location.pathname + location.search }} />
  }
  return <Outlet />
}

export function RequireAvatar() {
  const { me } = useAuth()
  if (me && me.profileComplete === false) return <Navigate to="/onboarding/avatar" replace />
  return <Outlet />
}

export function RequireRole({ roles }) {
  const { me } = useAuth()
  if (!me || !roles.includes(me.role)) return <ForbiddenPage />
  return <Outlet />
}

export function PublicOnly() {
  const gate = useBootGate()
  const { me } = useAuth()
  const location = useLocation()
  if (gate) return gate
  if (me) {
    if (me.profileComplete === false) return <Navigate to="/onboarding/avatar" replace />
    const from = location.state?.from
    return <Navigate to={typeof from === 'string' && from.startsWith('/') ? from : '/'} replace />
  }
  return <Outlet />
}
