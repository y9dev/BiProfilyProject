// Display formatting for /analytics (no React).
import { formatLocalDate } from '../../lib/dates'
import { NO_DEPARTMENT } from './analyticsState'

// 'YYYY-MM-DD' is a calendar date (Almaty), not an instant: build it in local time so it never shifts.
export function formatDay(day, lang) {
  if (!day) return ''
  const [y, m, d] = day.split('-').map(Number)
  return formatLocalDate(new Date(y, m - 1, d), lang, { style: 'medium' })
}

export function formatInstant(iso, lang) {
  return iso ? formatLocalDate(iso, lang, { style: 'medium' }) : ''
}

export const deptKey = (department) => (department ? String(department.id) : NO_DEPARTMENT)
