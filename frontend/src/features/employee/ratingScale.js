// Parses a rating question's options into a numeric scale.
// The wizard stores points as "1 · low label", "2", ..., "5 · high label" (end labels optional),
// so the take view shows only the numbers on the buttons and the end labels once, under the scale.
// Returns null when any option is not "<number>" or "<number> · <label>" (rendered as a plain list).
const POINT = /^(\d+)(?:\s*·\s*(.*\S))?\s*$/

export function parseRatingScale(options) {
  if (!Array.isArray(options) || options.length < 2) return null
  const points = []
  for (const o of options) {
    const m = POINT.exec(String(o?.text ?? '').trim())
    if (!m) return null
    points.push({ id: o.id, value: m[1], label: m[2] || '' })
  }
  return { points, low: points[0].label, high: points[points.length - 1].label }
}
