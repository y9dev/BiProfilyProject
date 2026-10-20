import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  monotonePath,
  nearestIndex,
  paletteFor,
  parseDay,
  pickTicks,
  pointState,
  splitSeries,
  spreadTags,
  xScale,
  yScale,
} from './stockGeometry.js'

const vp = (value, employees = 10, insufficient = false) => ({ value, employees, attempts: employees, insufficient })
const gap = (employees = 0) => ({ value: null, employees, attempts: employees, insufficient: true })

test('null splits segments: no segment spans a gap', () => {
  const values = [vp(10), vp(20), gap(), vp(30), vp(40), gap(3), vp(50), vp(60)]
  const { segments, dots } = splitSeries(values)
  assert.equal(dots.length, 0)
  assert.deepEqual(
    segments.map((s) => s.points.map((p) => p.i)),
    [[0, 1], [3, 4], [6, 7]],
  )
  for (const s of segments) {
    for (let k = 1; k < s.points.length; k++) assert.equal(s.points[k].i, s.points[k - 1].i + 1)
    for (const p of s.points) assert.notEqual(values[p.i].value, null)
  }
})

test('employees = 0 points are never crossed by a path command', () => {
  const values = [vp(40), gap(0), vp(60)]
  const { segments, dots } = splitSeries(values)
  assert.equal(segments.length, 0)
  assert.deepEqual(dots.map((d) => d.i), [0, 2])
})

test('an isolated point is a dot', () => {
  const { segments, dots } = splitSeries([gap(), vp(55), gap(), gap()])
  assert.equal(segments.length, 0)
  assert.deepEqual(dots, [{ i: 1, v: 55, dim: false }])
  assert.equal(splitSeries([vp(55, 3, true)]).dots[0].dim, true)
})

test('insufficient runs are separate dim segments sharing boundary points', () => {
  const values = [vp(10), vp(20), vp(25, 3, true), vp(30, 4, true), vp(40), vp(50)]
  const { segments } = splitSeries(values, 'dim')
  assert.deepEqual(
    segments.map((s) => [s.dim, s.points.map((p) => p.i)]),
    [
      [false, [0, 1]],
      [true, [1, 2, 3, 4]],
      [false, [4, 5]],
    ],
  )
})

test("insufficientStyle 'hidden' drops insufficient values entirely", () => {
  const values = [vp(10), vp(20), vp(25, 3, true), vp(40), vp(50)]
  const { segments, dots } = splitSeries(values, 'hidden')
  assert.deepEqual(segments.map((s) => s.points.map((p) => p.i)), [[0, 1], [3, 4]])
  assert.ok(segments.every((s) => !s.dim))
  assert.equal(dots.length, 0)
})

test('y clamps to 0-100', () => {
  const y = yScale(10, 210)
  assert.equal(y(0), 210)
  assert.equal(y(100), 10)
  assert.equal(y(50), 110)
  assert.equal(y(-20), 210)
  assert.equal(y(140), 10)
  const { dots } = splitSeries([vp(130)])
  assert.equal(dots[0].v, 100)
})

test('x scale spans the plot; a single point is centred', () => {
  const x = xScale(5, 0, 400)
  assert.equal(x(0), 0)
  assert.equal(x(4), 400)
  assert.equal(xScale(1, 0, 400)(0), 200)
})

test('nearestIndex snaps and clamps', () => {
  assert.equal(nearestIndex(-50, 10, 0, 900), 0)
  assert.equal(nearestIndex(140, 10, 0, 900), 1)
  assert.equal(nearestIndex(160, 10, 0, 900), 2)
  assert.equal(nearestIndex(5000, 10, 0, 900), 9)
  assert.equal(nearestIndex(10, 1, 0, 900), 0)
})

// Sample a cubic bezier path and return all sampled y values.
function sampleYs(d) {
  const nums = d.match(/-?\d+(\.\d+)?/g).map(Number)
  const ys = []
  let y0 = nums[1]
  for (let k = 2; k + 6 <= nums.length; k += 6) {
    const [, c1y, , c2y, , y3] = nums.slice(k, k + 6)
    for (let s = 0; s <= 20; s++) {
      const t = s / 20
      const u = 1 - t
      const y = u * u * u * y0 + 3 * u * u * t * c1y + 3 * u * t * t * c2y + t * t * t * y3
      ys.push({ y, lo: Math.min(y0, y3), hi: Math.max(y0, y3) })
    }
    y0 = y3
  }
  return ys
}

test('monotone cubic never overshoots between points', () => {
  const pts = [
    { x: 0, y: 200 },
    { x: 10, y: 0 },
    { x: 20, y: 0 },
    { x: 30, y: 180 },
    { x: 40, y: 190 },
    { x: 50, y: 10 },
  ]
  const d = monotonePath(pts)
  assert.ok(d.startsWith('M0,200C'))
  for (const { y, lo, hi } of sampleYs(d)) {
    assert.ok(y >= lo - 1e-6 && y <= hi + 1e-6, `y=${y} outside [${lo}, ${hi}]`)
  }
  assert.equal(monotonePath([{ x: 0, y: 1 }, { x: 5, y: 2 }]), 'M0,1L5,2')
  assert.equal(monotonePath([]), '')
})

test('ticks do not overlap at 320 / 768 / 1280 px and always label the latest point', () => {
  const label = 64
  for (const width of [320, 768, 1280]) {
    for (const n of [1, 3, 7, 30, 58, 90, 92, 121]) {
      const plot = width - 80
      const ticks = pickTicks(n, plot, label)
      assert.ok(ticks.length >= 1)
      assert.equal(ticks[ticks.length - 1], n - 1)
      const x = xScale(n, 0, plot)
      for (let k = 1; k < ticks.length; k++) {
        assert.ok(x(ticks[k]) - x(ticks[k - 1]) >= label + 12 - 1e-9, `overlap at width ${width}, n ${n}`)
      }
    }
  }
})

test('spreadTags keeps tags apart and inside bounds', () => {
  const out = spreadTags([{ y: 50, id: 'a' }, { y: 52, id: 'b' }, { y: 51, id: 'c' }], 16, 0, 200)
  const ys = out.map((t) => t.ty).sort((a, b) => a - b)
  assert.ok(ys[1] - ys[0] >= 16 && ys[2] - ys[1] >= 16)
  assert.equal(out[0].id, 'a')
  const low = spreadTags([{ y: 199 }, { y: 200 }], 16, 0, 200)
  assert.ok(low.every((t) => t.ty <= 200))
})

test('parseDay avoids time zones', () => {
  assert.deepEqual(parseDay('2026-10-20'), { y: 2026, m: 9, d: 20 })
  assert.equal(parseDay('bad'), null)
})

test('paletteFor: core by id solid, extras dashed in order', () => {
  const chars = [
    { id: 9, core: false },
    { id: 3, core: true },
    { id: 1, core: true },
    { id: 12, core: false },
  ]
  const style = paletteFor(chars)
  assert.deepEqual(style({ id: 1, core: true }), { color: 'var(--core-1)', dashed: false })
  assert.deepEqual(style({ id: 3, core: true }), { color: 'var(--core-2)', dashed: false })
  assert.deepEqual(style({ id: 9, core: false }), { color: 'var(--extra-1)', dashed: true })
  assert.deepEqual(style({ id: 12, core: false }), { color: 'var(--extra-2)', dashed: true })
})

test('pointState matches drawing: hidden insufficient values never show their number', () => {
  assert.equal(pointState(vp(61.4), 'hidden'), 'value')
  assert.equal(pointState(vp(58.3, 3, true), 'dim'), 'value')
  assert.equal(pointState(vp(58.3, 3, true), 'hidden'), 'insufficient')
  assert.equal(pointState(gap(3), 'dim'), 'insufficient')
  assert.equal(pointState(gap(0), 'hidden'), 'none')
  assert.equal(pointState(undefined), 'none')
})
