// Pure state helpers for /analytics. No React, no i18n: unit-tested by analyticsState.test.mjs.
//
// URL model (plan section 3, "Shared query model"):
//   shared     ?period=&from=&to=&c=1&c=7      -> sent unchanged by every tab
//   tab-local  tab, dep (repeated), pdept / edept / ddept (department filter of the Overview,
//              Employees and Data tabs, independent of each other), emp, q, data, thr
// Invalid values never throw: each one falls back to its default, so a hand-edited or stale link
// always opens a working page. An empty `c` list means "the server default selection"
// (core four + top-3 by coverage, returned as `defaultIds` by /api/analytics/trend).

export const PRESETS = ['7d', '30d', '90d', '12m', 'all']
export const PERIODS = [...PRESETS, 'custom']
export const DEFAULT_PERIOD = '90d'
export const TABS = ['overview', 'departments', 'employees', 'data']
export const DATA_FILTERS = ['all', 'no_data', 'stale']
export const MAX_CHARACTERISTICS = 10
export const MAX_DEPARTMENTS = 8
export const MIN_SAMPLE = 5
export const DEFAULT_THRESHOLD = 15
export const NO_DEPARTMENT = 'none'
const MAX_SPAN_DAYS = 3660

const DATE_RE = /^(\d{4})-(\d{2})-(\d{2})$/

// Real calendar date in YYYY-MM-DD form (rejects 2026-02-30 and similar).
export function isIsoDate(s) {
  if (typeof s !== 'string') return false
  const m = DATE_RE.exec(s)
  if (!m) return false
  const [y, mo, d] = [Number(m[1]), Number(m[2]), Number(m[3])]
  const dt = new Date(Date.UTC(y, mo - 1, d))
  return dt.getUTCFullYear() === y && dt.getUTCMonth() === mo - 1 && dt.getUTCDate() === d
}

const dayNumber = (s) => {
  const [y, m, d] = s.split('-').map(Number)
  return Date.UTC(y, m - 1, d) / 86400000
}

// Today's date in Asia/Almaty (UTC+5, no DST since 2024-03-01), the backend's calendar.
export function almatyToday(now = new Date()) {
  const t = new Date(now.getTime() + 5 * 3600 * 1000)
  return t.toISOString().slice(0, 10)
}

// Client-side check of a custom range; returns a stats.validation.* code or null.
// `in_future` and `span_too_large` mirror the server; the server stays the authority.
export function customRangeError(from, to, today) {
  if (!from) return { path: 'from', code: 'required' }
  if (!to) return { path: 'to', code: 'required' }
  if (!isIsoDate(from)) return { path: 'from', code: 'invalid_date' }
  if (!isIsoDate(to)) return { path: 'to', code: 'invalid_date' }
  if (from > to) return { path: 'from', code: 'after_to' }
  if (today && to > today) return { path: 'to', code: 'in_future' }
  if (dayNumber(to) - dayNumber(from) + 1 > MAX_SPAN_DAYS) return { path: 'to', code: 'span_too_large' }
  return null
}

const posInt = (v) => {
  if (typeof v !== 'string' || !/^\d{1,9}$/.test(v)) return null
  const n = Number(v)
  return n > 0 ? n : null
}

// Deduplicate preserving order, drop invalid entries, cap at `max`.
function cleanList(raw, parse, max) {
  const out = []
  for (const v of raw) {
    const x = parse(v)
    if (x === null || out.includes(x)) continue
    out.push(x)
    if (out.length === max) break
  }
  return out
}

// Tab-local department filters: state field -> URL key.
export const DEPT_FILTERS = [
  ['participationDept', 'pdept'],
  ['employeesDept', 'edept'],
  ['dataDept', 'ddept'],
]

const parseDepartment = (v) => (v === NO_DEPARTMENT ? NO_DEPARTMENT : posInt(v))

/** @param {URLSearchParams} sp */
export function parseState(sp) {
  let period = sp.get('period') || DEFAULT_PERIOD
  if (!PERIODS.includes(period)) period = DEFAULT_PERIOD
  let from = ''
  let to = ''
  if (period === 'custom') {
    from = sp.get('from') || ''
    to = sp.get('to') || ''
    // Custom needs both valid dates in order; otherwise the whole period falls back.
    if (customRangeError(from, to)) {
      period = DEFAULT_PERIOD
      from = ''
      to = ''
    }
  }
  const tab = TABS.includes(sp.get('tab')) ? sp.get('tab') : TABS[0]
  const data = DATA_FILTERS.includes(sp.get('data')) ? sp.get('data') : 'all'
  const thrRaw = posInt(sp.get('thr') ?? '')
  const threshold = thrRaw !== null && thrRaw <= 100 ? thrRaw : DEFAULT_THRESHOLD
  return {
    period,
    from,
    to,
    characteristics: cleanList(sp.getAll('c'), posInt, MAX_CHARACTERISTICS),
    tab,
    departments: cleanList(sp.getAll('dep'), parseDepartment, MAX_DEPARTMENTS),
    ...Object.fromEntries(DEPT_FILTERS.map(([field, key]) => [field, sp.get(key) === null ? null : parseDepartment(sp.get(key))])),
    employee: posInt(sp.get('emp') ?? ''),
    search: (sp.get('q') || '').slice(0, 100),
    data,
    threshold,
  }
}

// Inverse of parseState; defaults are omitted so the canonical URL stays short.
export function serializeState(s) {
  const sp = new URLSearchParams()
  if (s.period && s.period !== DEFAULT_PERIOD) sp.set('period', s.period)
  if (s.period === 'custom') {
    if (s.from) sp.set('from', s.from)
    if (s.to) sp.set('to', s.to)
  }
  for (const c of s.characteristics || []) sp.append('c', String(c))
  if (s.tab && s.tab !== TABS[0]) sp.set('tab', s.tab)
  for (const d of s.departments || []) sp.append('dep', String(d))
  for (const [field, key] of DEPT_FILTERS) if (s[field] !== null && s[field] !== undefined) sp.set(key, String(s[field]))
  if (s.employee) sp.set('emp', String(s.employee))
  if (s.search) sp.set('q', s.search)
  if (s.data && s.data !== 'all') sp.set('data', s.data)
  if (s.threshold && s.threshold !== DEFAULT_THRESHOLD) sp.set('thr', String(s.threshold))
  return sp
}

// The shared filter sent unchanged by every tab. Empty characteristics = server default.
export function sharedParams(s) {
  const p = { period: s.period }
  if (s.period === 'custom') Object.assign(p, { from: s.from, to: s.to })
  if (s.characteristics.length) p.characteristics = s.characteristics
  return p
}

export function periodParams(s) {
  return s.period === 'custom' ? { period: s.period, from: s.from, to: s.to } : { period: s.period }
}

// Stable key of the shared filter: tabs refetch exactly when it changes.
export const sharedKey = (s) => serializeState({ period: s.period, from: s.from, to: s.to, characteristics: s.characteristics }).toString()

const sameSet = (a, b) => a.length === b.length && a.every((x) => b.includes(x))

// Selection actually in effect: the URL list restricted to known ids, else the server default.
export function effectiveSelection(selected, catalog, defaultIds) {
  const known = new Set((catalog || []).map((c) => c.id))
  const kept = (selected || []).filter((id) => known.has(id))
  return kept.length ? kept : [...(defaultIds || [])]
}

export function isDefaultSelection(selected, defaultIds) {
  return !selected.length || sameSet(selected, defaultIds || [])
}

// Next URL list after toggling `id`. Returns null when the change is not allowed
// (cap reached, or removing the last one). A list equal to the default collapses to [].
export function toggleCharacteristic(current, id, defaultIds) {
  const next = current.includes(id) ? current.filter((x) => x !== id) : [...current, id]
  if (next.length > MAX_CHARACTERISTICS || next.length === 0) return null
  return sameSet(next, defaultIds || []) ? [] : next
}

// Compared departments restricted to ids that still exist ('none' always does). Before the
// department list has loaded (`known` null) the selection is kept as is.
export function knownDepartments(selected, known) {
  if (!known) return selected
  const ids = new Set(known.map((d) => d.id))
  return selected.filter((id) => id === NO_DEPARTMENT || ids.has(id))
}

export function toggleDepartment(current, id) {
  if (current.includes(id)) return current.filter((x) => x !== id)
  if (current.length >= MAX_DEPARTMENTS) return null
  return [...current, id]
}

// Series colours: core characteristics take --core-1..4 by id (stable across selections, same as
// the dashboard and trends.js); additional ones take --extra-1..6 in selection order, always dashed.
export function seriesStyles(selection, catalog) {
  const byId = new Map((catalog || []).map((c) => [c.id, c]))
  const coreIds = (catalog || []).filter((c) => c.core).map((c) => c.id).sort((a, b) => a - b)
  const styles = new Map()
  let extra = 0
  for (const id of selection) {
    const c = byId.get(id)
    if (!c) continue
    if (c.core) styles.set(id, { color: `var(--core-${(coreIds.indexOf(id) % 4) + 1})`, dashed: false })
    else styles.set(id, { color: `var(--extra-${(extra++ % 6) + 1})`, dashed: true })
  }
  return styles
}

// Localized characteristic name: core ones carry nameI18n, custom ones are authored text.
export function charName(c, lang) {
  return c?.nameI18n?.[lang] || c?.name || ''
}

// Signed one-decimal delta for display (+1.6 / -22.5 / 0).
export function formatDelta(v) {
  if (v === null || v === undefined || !Number.isFinite(v)) return ''
  const r = Math.round(v * 10) / 10
  if (r === 0) return '0'
  return r > 0 ? `+${r}` : `${r}`
}

export function formatScore(v) {
  if (v === null || v === undefined || !Number.isFinite(v)) return ''
  return String(Math.round(v * 10) / 10)
}

export function formatRate(r) {
  if (r === null || r === undefined || !Number.isFinite(r)) return ''
  return `${Math.round(r * 100)}%`
}

// Paging helper for limit/offset lists.
export function pageInfo(total, offset, limit) {
  const page = Math.floor(offset / limit) + 1
  const pages = Math.max(1, Math.ceil(total / limit))
  return { page, pages, hasPrev: offset > 0, hasNext: offset + limit < total }
}
