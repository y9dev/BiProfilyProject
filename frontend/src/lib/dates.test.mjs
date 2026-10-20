import { test } from 'node:test'
import assert from 'node:assert/strict'
import { formatLocalDate } from './dates.js'

const d = new Date(2026, 9, 19, 9, 5) // local 19 Oct 2026 09:05

test('kk is formatted without Intl month data (no "M10")', () => {
  assert.equal(formatLocalDate(d, 'kk'), '2026 ж. 19 қазан')
  assert.equal(formatLocalDate(d, 'kk', { time: true }), '2026 ж. 19 қазан, 09:05')
})

test('ru uses Intl', () => {
  assert.match(formatLocalDate(d, 'ru'), /19 октября 2026/)
  assert.match(formatLocalDate(d, 'ru', { style: 'medium', time: true }), /19 окт\. 2026.*09:05/)
})

test('empty and invalid input', () => {
  assert.equal(formatLocalDate(null, 'ru'), '')
  assert.equal(formatLocalDate('nope', 'kk'), '')
})
