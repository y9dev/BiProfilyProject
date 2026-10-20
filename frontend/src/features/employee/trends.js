// Pure helpers for the skills views. No React, no i18n: unit-tested by trends.test.mjs.

const EPS = 0.05
const round1 = (v) => Math.round(v * 10) / 10

// Core characteristics (the four defaults carry nameI18n) first, ordered by id, each with its palette
// slot (--core-1..4); custom ones follow in API order with the neutral --extra colour.
export function orderCharacteristics(chars) {
  const core = chars.filter((c) => c.nameI18n).sort((a, b) => a.id - b.id)
  const extras = chars.filter((c) => !c.nameI18n)
  return [
    ...core.map((c, i) => ({ c, core: true, color: `var(--core-${(i % 4) + 1})` })),
    ...extras.map((c) => ({ c, core: false, color: 'var(--extra)' })),
  ]
}

export function sortAttempts(timeline) {
  return [...timeline].sort(
    (a, b) => new Date(a.completedAt) - new Date(b.completedAt) || a.attemptId - b.attemptId,
  )
}

// Per characteristic: its series over time (oldest first) and the change between its two latest
// measurements. "Previous" is the previous attempt that measured THAT characteristic, because an
// assessment only covers some characteristics. direction: 'up' | 'down' | 'same' | 'none' (< 2 points).
export function computeTrends(timeline, characteristics) {
  const attempts = sortAttempts(timeline)
  const items = characteristics.map((c) => {
    const series = []
    for (const a of attempts) {
      const s = a.scores.find((x) => x.characteristicId === c.id)
      if (s && Number.isFinite(s.score)) series.push({ v: round1(s.score), at: a.completedAt, attemptId: a.attemptId })
    }
    const latest = series.length ? series[series.length - 1].v : null
    const previous = series.length > 1 ? series[series.length - 2].v : null
    const delta = previous == null ? null : round1(latest - previous)
    let direction = 'none'
    if (delta != null) direction = Math.abs(delta) < EPS ? 'same' : delta > 0 ? 'up' : 'down'
    return { id: c.id, series, latest, previous, delta, direction }
  })
  const count = (d) => items.filter((x) => x.direction === d).length
  return {
    attemptCount: attempts.length,
    items,
    summary: { improved: count('up'), declined: count('down'), unchanged: count('same'), noComparison: count('none') },
  }
}
