// Localized messages for field-level validation codes returned by the backend (422 `errors[{path,code}]`)
// and for the client-side checks that mirror them.
// Lookup order: auth.field_errors.<field>.<code>, auth.field_errors.<code>, errors.validation.
export function fieldMessage(t, field, code) {
  for (const key of [`auth.field_errors.${field}.${code}`, `auth.field_errors.${code}`]) {
    const v = t(key)
    if (v !== key) return v
  }
  return t('errors.validation')
}

export const LOGIN_RE = /^[A-Za-z0-9._-]{3,64}$/

// Same rules as backend/internal/httpapi/register.go validateRegister. Returns { field: code }.
export function validateRegisterForm(f) {
  const errs = {}
  if (!f.code.trim()) errs.code = 'required'
  if (!f.fullName.trim()) errs.fullName = 'required'
  else if ([...f.fullName.trim()].length > 160) errs.fullName = 'too_long'
  if (!f.jobTitle.trim()) errs.jobTitle = 'required'
  else if ([...f.jobTitle.trim()].length > 160) errs.jobTitle = 'too_long'
  if (!f.login.trim()) errs.login = 'required'
  else if (!LOGIN_RE.test(f.login.trim())) errs.login = 'invalid'
  const n = [...f.password].length
  if (n === 0) errs.password = 'required'
  else if (n < 8) errs.password = 'too_short'
  else if (n > 128) errs.password = 'too_long'
  return errs
}
