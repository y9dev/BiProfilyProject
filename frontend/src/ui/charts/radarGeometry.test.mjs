import { test } from 'node:test'
import assert from 'node:assert/strict'
import { axisPoint, wrapLabel } from './radarGeometry.js'

test('axis 0 points straight up, axes are evenly spaced', () => {
  const [x, y] = axisPoint(100, 100, 50, 0, 4)
  assert.ok(Math.abs(x - 100) < 1e-9 && Math.abs(y - 50) < 1e-9)
  const [x1, y1] = axisPoint(100, 100, 50, 1, 4)
  assert.ok(Math.abs(x1 - 150) < 1e-9 && Math.abs(y1 - 100) < 1e-9)
})

test('value is clamped to 0..100 and scales the radius', () => {
  const [, y] = axisPoint(0, 0, 100, 0, 3, 50)
  assert.ok(Math.abs(y + 50) < 1e-9)
  const [, y2] = axisPoint(0, 0, 100, 0, 3, 500)
  assert.ok(Math.abs(y2 + 100) < 1e-9)
})

test('wrapLabel keeps short labels, wraps long ones, truncates overflow', () => {
  assert.deepEqual(wrapLabel('Short'), ['Short'])
  assert.deepEqual(wrapLabel('Critical thinking'), ['Critical', 'thinking'])
  const w = wrapLabel('A very long characteristic name of an employee', 14, 2)
  assert.equal(w.length, 2)
  assert.ok(w[1].endsWith('…'))
  assert.ok(w.every((l) => l.length <= 14))
  assert.ok(wrapLabel('Supercalifragilisticexpialidocious')[0].length <= 14)
  assert.deepEqual(wrapLabel(''), [])
})
