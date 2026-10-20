import { test } from 'node:test'
import assert from 'node:assert/strict'
import { parseRatingScale } from './ratingScale.js'
import { pointText } from '../manager/wizard/validate.js'

const opts = (texts) => texts.map((text, i) => ({ id: i + 10, text }))

test('wizard payload with end labels: numbers on points, labels once', () => {
  const s = parseRatingScale(opts(['1 · Not me', '2', '3', '4', '5 · Exactly me']))
  assert.deepEqual(
    s.points.map((p) => p.value),
    ['1', '2', '3', '4', '5'],
  )
  assert.equal(s.low, 'Not me')
  assert.equal(s.high, 'Exactly me')
  assert.equal(s.points[0].id, 10)
})

test('plain numeric scale has empty end labels (caller falls back to generic ones)', () => {
  const s = parseRatingScale(opts(['1', '2', '3']))
  assert.equal(s.low, '')
  assert.equal(s.high, '')
})

test('non-numeric options are not a scale', () => {
  assert.equal(parseRatingScale(opts(['Low', 'High'])), null)
  assert.equal(parseRatingScale(opts(['1'])), null)
  assert.equal(parseRatingScale(null), null)
})

test('round-trips the wizard pointText format', () => {
  const q = { points: 4, low: ' нет ', high: 'да' }
  const texts = [0, 1, 2, 3].map((i) => pointText(q, i))
  const s = parseRatingScale(opts(texts))
  assert.equal(s.low, 'нет')
  assert.equal(s.high, 'да')
  assert.equal(s.points.length, 4)
})
