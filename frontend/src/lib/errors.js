// Maps an ApiError to a localized message. Looks up `errors.<code>`, then `errors.http_<status>`,
// then the generic message. Never shows raw backend text to the user.
export function errorMessage(t, err) {
  const code = err?.code
  const probe = (key) => {
    const v = t(key)
    return v === key ? null : v
  }
  if (code) {
    const byCode = probe(`errors.${code}`)
    if (byCode) return byCode
  }
  if (err?.status) {
    const byStatus = probe(`errors.http_${err.status}`)
    if (byStatus) return byStatus
    if (err.status === 403) return t('errors.forbidden')
    if (err.status === 404) return t('errors.not_found')
  }
  return t('errors.generic')
}
