// Holds the signed-in user (`me`) for the whole app.
// status: 'loading' (first /api/auth/me in flight) | 'ready' | 'error' (server unreachable).
// Reacts to client events: 'avatar-required' -> profile marked incomplete (guards redirect to
// /onboarding/avatar); 'unauthorized' -> session dropped. A generic 403 never touches auth state.
import { useCallback, useEffect, useMemo, useState } from 'react'
import { apiEvents } from '../api/client'
import * as endpoints from '../api/endpoints'
import { AuthContext } from './context'

export default function AuthProvider({ children }) {
  const [me, setMe] = useState(null)
  const [status, setStatus] = useState('loading')
  // true after an explicit logout: the next sign-in (maybe another user) must not return to
  // the previous user's page, so RequireAuth does not remember it.
  const [signedOut, setSignedOut] = useState(false)

  const load = useCallback(async (signal) => {
    try {
      const user = await endpoints.auth.me({ signal })
      if (signal?.aborted) return
      setMe(user)
      setStatus('ready')
    } catch (e) {
      if (signal?.aborted) return
      if (e?.status === 401) {
        setMe(null)
        setStatus('ready')
      } else {
        setStatus('error')
      }
    }
  }, [])

  useEffect(() => {
    const ctrl = new AbortController()
    load(ctrl.signal)
    return () => ctrl.abort()
  }, [load])

  useEffect(() => {
    const onAvatar = () => setMe((m) => (m ? { ...m, profileComplete: false } : m))
    const onUnauthorized = () => setMe(null)
    apiEvents.addEventListener('avatar-required', onAvatar)
    apiEvents.addEventListener('unauthorized', onUnauthorized)
    return () => {
      apiEvents.removeEventListener('avatar-required', onAvatar)
      apiEvents.removeEventListener('unauthorized', onUnauthorized)
    }
  }, [])

  const refresh = useCallback(() => {
    setStatus((s) => (s === 'error' ? 'loading' : s))
    return load()
  }, [load])

  const login = useCallback(async (loginName, password) => {
    const user = await endpoints.auth.login(loginName, password)
    setMe(user)
    setSignedOut(false)
    return user
  }, [])

  const register = useCallback(async (input) => {
    const user = await endpoints.auth.register(input)
    setMe(user)
    setSignedOut(false)
    return user
  }, [])

  const logout = useCallback(async () => {
    try {
      await endpoints.auth.logout()
    } catch {
      /* the session is dropped locally either way */
    }
    setSignedOut(true)
    setMe(null)
  }, [])

  const value = useMemo(
    () => ({ me, status, signedOut, refresh, login, register, logout }),
    [me, status, signedOut, refresh, login, register, logout],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}
