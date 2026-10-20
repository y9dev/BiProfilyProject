// Pure helpers for invitation lifecycle rules (mirror backend invitations/status.go).
// Status precedence is decided by the server: revoked > expired > full > active.

export const INVITATION_STATUSES = ['active', 'expired', 'full', 'revoked']

const TONES = { active: 'success', expired: 'warning', full: 'info', revoked: 'danger' }

export function statusTone(status) {
  return TONES[status] ?? 'neutral'
}

// Which lifecycle actions the UI offers. The server stays the authority (409 on violations).
export function invitationActions(inv) {
  const revoked = inv.status === 'revoked'
  return {
    reactivate: !revoked && (inv.status === 'expired' || inv.status === 'full' || inv.status === 'active'),
    changeMax: !revoked,
    revoke: !revoked,
    remove: inv.registrationsCount === 0,
  }
}

export function remaining(inv) {
  return Math.max(0, inv.maxRegistrations - inv.registrationsCount)
}

// Returns an error code or null. 'below_count' = cannot go under current registrations.
export function validateMax(raw, count) {
  const s = String(raw ?? '').trim()
  if (!/^\d+$/.test(s)) return 'out_of_range'
  const n = Number(s)
  if (n < 1 || n > 10000) return 'out_of_range'
  if (n < count) return 'below_count'
  return null
}

export function registrationUrl(origin, code) {
  return `${origin}/register?code=${encodeURIComponent(code)}`
}

const pad = (n) => String(n).padStart(2, '0')

// ISO instant -> value for <input type="datetime-local"> in the viewer's zone ('' when null).
export function toLocalInput(iso) {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

// <input type="datetime-local"> value -> ISO instant (null when empty, undefined when unparsable).
export function fromLocalInput(value) {
  if (!value) return null
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return undefined
  return d.toISOString()
}

export function isPast(iso, now = Date.now()) {
  return !!iso && new Date(iso).getTime() <= now
}

// Tomorrow 18:00 local: a sensible future default when "no expiry" is unticked.
export function defaultLocal(now = Date.now()) {
  const d = new Date(now + 24 * 3600 * 1000)
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T18:00`
}
