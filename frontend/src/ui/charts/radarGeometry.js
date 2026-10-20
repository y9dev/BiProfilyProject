// Pure geometry/text helpers for Radar.jsx (unit-tested in radarGeometry.test.mjs).

// Angle of axis i of n, starting at the top and going clockwise.
export const axisAngle = (i, n) => -Math.PI / 2 + (2 * Math.PI * i) / Math.max(n, 1)

export function axisPoint(cx, cy, r, i, n, value = 100) {
  const a = axisAngle(i, n)
  const k = (r * Math.min(Math.max(value, 0), 100)) / 100
  return [cx + k * Math.cos(a), cy + k * Math.sin(a)]
}

// Greedy word wrap into at most `maxLines` lines of `max` characters; overflow ends with an ellipsis.
export function wrapLabel(text, max = 14, maxLines = 2) {
  const words = String(text ?? '').trim().split(/\s+/).filter(Boolean)
  const cut = (w) => (w.length > max ? `${w.slice(0, max - 1)}…` : w)
  const lines = []
  let line = ''
  for (const w of words) {
    if (!line) line = w
    else if (`${line} ${w}`.length <= max) line = `${line} ${w}`
    else {
      lines.push(line)
      line = w
    }
  }
  if (line) lines.push(line)
  if (lines.length <= maxLines) return lines.map(cut)
  const kept = lines.slice(0, maxLines)
  const rest = `${kept[maxLines - 1]} ${lines.slice(maxLines).join(' ')}`
  kept[maxLines - 1] = `${rest.slice(0, max - 1).trimEnd()}…`
  return kept.map(cut)
}
