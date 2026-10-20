import { test } from 'node:test'
import assert from 'node:assert/strict'
import { computeTrends, orderCharacteristics } from './trends.js'

const chars = [{ id: 1 }, { id: 2 }, { id: 9 }]
const at = (attemptId, completedAt, scores) => ({
  attemptId,
  completedAt,
  scores: Object.entries(scores).map(([id, score]) => ({ characteristicId: Number(id), score })),
})

test('no attempts: nothing to compare', () => {
  const r = computeTrends([], chars)
  assert.equal(r.attemptCount, 0)
  assert.deepEqual(r.summary, { improved: 0, declined: 0, unchanged: 0, noComparison: 3 })
  assert.equal(r.items[0].latest, null)
})

test('one attempt: latest value, no delta', () => {
  const r = computeTrends([at(1, '2026-10-01T00:00:00Z', { 1: 40 })], chars)
  assert.equal(r.items[0].latest, 40)
  assert.equal(r.items[0].direction, 'none')
  assert.equal(r.summary.noComparison, 3)
})

test('delta against previous measurement, input order independent', () => {
  const r = computeTrends(
    [
      at(3, '2026-10-20T00:00:00Z', { 1: 80, 2: 50, 9: 10 }),
      at(1, '2026-10-01T00:00:00Z', { 1: 60, 2: 50, 9: 30.04 }),
    ],
    chars,
  )
  const by = Object.fromEntries(r.items.map((x) => [x.id, x]))
  assert.equal(by[1].delta, 20)
  assert.equal(by[1].direction, 'up')
  assert.equal(by[2].direction, 'same')
  assert.equal(by[9].direction, 'down')
  assert.deepEqual(r.summary, { improved: 1, declined: 1, unchanged: 1, noComparison: 0 })
})

test('characteristic missing from the latest attempt compares its own last two measurements', () => {
  const r = computeTrends(
    [
      at(1, '2026-10-01T00:00:00Z', { 1: 10, 2: 20 }),
      at(2, '2026-10-05T00:00:00Z', { 1: 30 }),
      at(3, '2026-10-09T00:00:00Z', { 2: 20.02 }),
    ],
    chars,
  )
  const by = Object.fromEntries(r.items.map((x) => [x.id, x]))
  assert.equal(by[1].delta, 20)
  assert.equal(by[2].direction, 'same')
  assert.equal(by[9].direction, 'none')
})

test('orderCharacteristics: core by id with palette slots, extras gray after', () => {
  const o = orderCharacteristics([
    { id: 7, nameI18n: null },
    { id: 3, nameI18n: { ru: 'b' } },
    { id: 1, nameI18n: { ru: 'a' } },
  ])
  assert.deepEqual(o.map((x) => x.c.id), [1, 3, 7])
  assert.deepEqual(o.map((x) => x.color), ['var(--core-1)', 'var(--core-2)', 'var(--extra)'])
  assert.deepEqual(o.map((x) => x.core), [true, true, false])
})
