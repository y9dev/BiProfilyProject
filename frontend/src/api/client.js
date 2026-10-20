// API client: JSON, cookie session, CSRF double-submit, typed errors.
//
// Errors are normalized to ApiError { status, code, message, errors } where `code` is the
// backend error code ("avatar_required", "already_completed", "validation", ...) or a
// client code ("network", "http_<status>").
//
// Side effects are limited to two events on `apiEvents`, consumed by AuthContext:
//   'avatar-required'  403 { error: "avatar_required" }  -> go to /onboarding/avatar
//   'unauthorized'     401 on any authenticated endpoint -> the session is gone
// A generic 403 never logs the user out: the caller decides how to render it.

export const BASE_URL = import.meta.env.VITE_API_URL ?? 'http://localhost:8080'
const CSRF_COOKIE = 'bi_csrf'
const AUTH_PROBE_PATHS = ['/api/auth/login', '/api/auth/register', '/api/auth/me']

export const apiEvents = new EventTarget()

export class ApiError extends Error {
  constructor({ status, code, message, errors }) {
    super(message || code)
    this.name = 'ApiError'
    this.status = status
    this.code = code
    this.errors = errors || null
  }
}

function readCookie(name) {
  if (typeof document === 'undefined') return ''
  const m = document.cookie.match(new RegExp(`(?:^|; )${name}=([^;]*)`))
  return m ? decodeURIComponent(m[1]) : ''
}

function withQuery(path, query) {
  if (!query) return path
  const qs = new URLSearchParams()
  for (const [k, v] of Object.entries(query)) {
    if (v !== undefined && v !== null && v !== '') qs.set(k, String(v))
  }
  const s = qs.toString()
  return s ? `${path}${path.includes('?') ? '&' : '?'}${s}` : path
}

export async function request(path, { method = 'GET', body, query, signal, headers: extra } = {}) {
  const headers = { Accept: 'application/json', ...extra }
  if (body !== undefined) headers['Content-Type'] = 'application/json'
  if (method !== 'GET' && method !== 'HEAD') headers['X-CSRF-Token'] = readCookie(CSRF_COOKIE)

  let res
  try {
    res = await fetch(`${BASE_URL}${withQuery(path, query)}`, {
      method,
      credentials: 'include',
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
      signal,
    })
  } catch (e) {
    if (e?.name === 'AbortError') throw e
    throw new ApiError({ status: 0, code: 'network', message: 'network' })
  }

  let data = null
  if (res.status !== 204) {
    const text = await res.text()
    if (text) {
      try {
        data = JSON.parse(text)
      } catch {
        data = null
      }
    }
  }

  if (res.ok) return data

  const err = new ApiError({
    status: res.status,
    code: data?.error || `http_${res.status}`,
    message: data?.message,
    errors: data?.errors,
  })

  if (import.meta.env.DEV) console.warn('[api]', method, path, res.status, err.code)

  if (res.status === 403 && err.code === 'avatar_required') {
    apiEvents.dispatchEvent(new Event('avatar-required'))
  } else if (res.status === 401 && !AUTH_PROBE_PATHS.includes(path.split('?')[0])) {
    apiEvents.dispatchEvent(new Event('unauthorized'))
  }
  throw err
}

export const api = {
  get: (path, opts) => request(path, { ...opts, method: 'GET' }),
  post: (path, body, opts) => request(path, { ...opts, method: 'POST', body }),
  put: (path, body, opts) => request(path, { ...opts, method: 'PUT', body }),
  patch: (path, body, opts) => request(path, { ...opts, method: 'PATCH', body }),
  del: (path, opts) => request(path, { ...opts, method: 'DELETE' }),
}

// Field errors from a 422 `validation` response as { [path]: code }.
export function fieldErrors(err) {
  const out = {}
  if (err instanceof ApiError && Array.isArray(err.errors)) {
    for (const e of err.errors) if (e?.path && !(e.path in out)) out[e.path] = e.code
  }
  return out
}
