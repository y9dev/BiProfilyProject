// Locale-aware date formatting for the manager screens (see lib/dates.js for kk).
import { formatLocalDate } from '../../lib/dates'

export function formatDateTime(iso, lang) {
  return formatLocalDate(iso, lang, { style: 'medium', time: true })
}

export function formatDate(iso, lang) {
  return formatLocalDate(iso, lang, { style: 'medium' })
}
