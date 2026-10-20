// Small presentation helpers shared by the employee screens. No scoring is computed here.
import { formatLocalDate } from '../../lib/dates'

export function charName(c, lang) {
  return c?.nameI18n?.[lang] || c?.name || ''
}

export function formatDate(iso, lang, withTime = false) {
  return formatLocalDate(iso, lang, { style: 'long', time: withTime })
}

export const roundScore = (v) => Math.round(v * 10) / 10

export function initials(name) {
  return (name || '?')
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((p) => p[0].toUpperCase())
    .join('')
}
