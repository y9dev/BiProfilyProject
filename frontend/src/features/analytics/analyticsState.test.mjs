import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  DEFAULT_PERIOD,
  MAX_CHARACTERISTICS,
  MAX_DEPARTMENTS,
  almatyToday,
  customRangeError,
  effectiveSelection,
  formatDelta,
  isDefaultSelection,
  isIsoDate,
  parseState,
  seriesStyles,
  serializeState,
  sharedKey,
  sharedParams,
  toggleCharacteristic,
  toggleDepartment,
  knownDepartments,
} from './analyticsState.js'

const parse = (qs) => parseState(new URLSearchParams(qs))

const catalog = [
  { id: 1, core: true },
  { id: 2, core: true },
  { id: 3, core: true },
  { id: 4, core: true },
  { id: 7, core: false },
  { id: 9, core: false },
  { id: 12, core: false },
  { id: 15, core: false },
]
const defaultIds = [1, 2, 3, 4, 9, 12, 7]

test('empty URL gives defaults', () => {
  const s = parse('')
  assert.equal(s.period, DEFAULT_PERIOD)
  assert.deepEqual(s.characteristics, [])
  assert.equal(s.tab, 'overview')
  assert.deepEqual(s.departments, [])
  assert.equal(s.participationDept, null)
  assert.equal(s.employeesDept, null)
  assert.equal(s.dataDept, null)
  assert.equal(s.threshold, 15)
  assert.equal(s.data, 'all')
})

test('URL round-trip keeps repeated c= in order', () => {
  const qs = 'period=custom&from=2026-01-01&to=2026-03-31&c=7&c=1&c=12&tab=departments&dep=3&dep=none&pdept=none&edept=4&ddept=6&emp=8&q=ab&data=stale&thr=20'
  const s = parse(qs)
  assert.deepEqual(s.characteristics, [7, 1, 12])
  assert.deepEqual(s.departments, [3, 'none'])
  assert.equal(s.participationDept, 'none')
  assert.equal(s.employeesDept, 4)
  assert.equal(s.dataDept, 6)
  assert.equal(s.employee, 8)
  assert.equal(s.threshold, 20)
  const again = parseState(serializeState(s))
  assert.deepEqual(again, s)
  assert.deepEqual(serializeState(s).getAll('c'), ['7', '1', '12'])
})

test('defaults are omitted when serializing', () => {
  assert.equal(serializeState(parse('period=90d&tab=overview&thr=15&data=all')).toString(), '')
})

test('invalid URL values fall back to defaults', () => {
  const s = parse('period=5y&c=abc&c=-1&c=0&c=3.5&c=4&tab=nope&dep=x&pdept=x&edept=0&ddept=-1&emp=-2&data=bad&thr=500')
  assert.equal(s.period, DEFAULT_PERIOD)
  assert.deepEqual(s.characteristics, [4])
  assert.equal(s.tab, 'overview')
  assert.deepEqual(s.departments, [])
  assert.equal(s.participationDept, null)
  assert.equal(s.employeesDept, null)
  assert.equal(s.dataDept, null)
  assert.equal(s.employee, null)
  assert.equal(s.data, 'all')
  assert.equal(s.threshold, 15)
})

test('duplicates are removed and caps applied (10 characteristics, 8 departments)', () => {
  const c = Array.from({ length: 14 }, (_, i) => `c=${i + 1}`).join('&')
  const dep = Array.from({ length: 11 }, (_, i) => `dep=${i + 1}`).join('&')
  const s = parse(`c=1&c=1&${c}&${dep}&dep=1`)
  assert.equal(s.characteristics.length, MAX_CHARACTERISTICS)
  assert.deepEqual(s.characteristics.slice(0, 3), [1, 2, 3])
  assert.equal(s.departments.length, MAX_DEPARTMENTS)
})

test('custom period requires both valid dates in order', () => {
  assert.equal(parse('period=custom&from=2026-01-01').period, DEFAULT_PERIOD)
  assert.equal(parse('period=custom&to=2026-01-01').period, DEFAULT_PERIOD)
  assert.equal(parse('period=custom&from=2026-02-30&to=2026-03-01').period, DEFAULT_PERIOD)
  assert.equal(parse('period=custom&from=2026-03-02&to=2026-03-01').period, DEFAULT_PERIOD)
  const one = parse('period=custom&from=2026-03-01&to=2026-03-01')
  assert.equal(one.period, 'custom')
  assert.equal(one.from, '2026-03-01')
  // from/to without custom are dropped.
  const p = parse('period=30d&from=2026-01-01&to=2026-02-01')
  assert.equal(p.from, '')
  assert.equal(serializeState(p).has('from'), false)
})

test('customRangeError codes', () => {
  assert.deepEqual(customRangeError('', '2026-01-01'), { path: 'from', code: 'required' })
  assert.deepEqual(customRangeError('2026-01-01', ''), { path: 'to', code: 'required' })
  assert.deepEqual(customRangeError('2026-13-01', '2026-01-01'), { path: 'from', code: 'invalid_date' })
  assert.deepEqual(customRangeError('2026-02-01', '2026-01-01'), { path: 'from', code: 'after_to' })
  assert.deepEqual(customRangeError('2026-01-01', '2026-10-21', '2026-10-20'), { path: 'to', code: 'in_future' })
  assert.deepEqual(customRangeError('2010-01-01', '2026-01-01'), { path: 'to', code: 'span_too_large' })
  assert.equal(customRangeError('2026-01-01', '2026-10-20', '2026-10-20'), null)
  assert.equal(isIsoDate('2024-02-29'), true)
  assert.equal(isIsoDate('2025-02-29'), false)
})

test('almatyToday uses UTC+5', () => {
  assert.equal(almatyToday(new Date('2026-10-20T18:59:00Z')), '2026-10-20')
  assert.equal(almatyToday(new Date('2026-10-20T19:00:00Z')), '2026-10-21')
})

test('default selection comes from defaultIds and restore default works', () => {
  assert.deepEqual(effectiveSelection([], catalog, defaultIds), defaultIds)
  // Unknown ids from a stale URL are ignored; if nothing is left, default applies.
  assert.deepEqual(effectiveSelection([99], catalog, defaultIds), defaultIds)
  assert.deepEqual(effectiveSelection([15, 99, 1], catalog, defaultIds), [15, 1])
  assert.equal(isDefaultSelection([], defaultIds), true)
  // Adding to the default makes an explicit list; removing it again collapses back to default ([]).
  const added = toggleCharacteristic(defaultIds, 15, defaultIds)
  assert.deepEqual(added, [...defaultIds, 15])
  assert.equal(isDefaultSelection(added, defaultIds), false)
  assert.deepEqual(toggleCharacteristic(added, 15, defaultIds), [])
})

test('characteristic toggle respects the cap and keeps at least one', () => {
  const ten = [1, 2, 3, 4, 7, 9, 12, 15, 20, 21]
  assert.equal(toggleCharacteristic(ten, 30, defaultIds), null)
  assert.equal(toggleCharacteristic([1], 1, defaultIds), null)
  assert.deepEqual(toggleCharacteristic(ten, 21, defaultIds), ten.slice(0, 9))
})

test('department toggle respects the cap of 8', () => {
  const eight = [1, 2, 3, 4, 5, 6, 7, 'none']
  assert.equal(toggleDepartment(eight, 9), null)
  assert.deepEqual(toggleDepartment(eight, 'none'), [1, 2, 3, 4, 5, 6, 7])
  assert.deepEqual(toggleDepartment([], 'none'), ['none'])
})

test('shared params and key ignore tab-local state', () => {
  const a = parse('period=30d&c=1&c=7&tab=data&thr=30')
  const b = parse('period=30d&c=1&c=7&tab=employees&emp=4')
  assert.equal(sharedKey(a), sharedKey(b))
  assert.deepEqual(sharedParams(a), { period: '30d', characteristics: [1, 7] })
  assert.deepEqual(sharedParams(parse('')), { period: '90d' })
  assert.deepEqual(sharedParams(parse('period=custom&from=2026-01-01&to=2026-01-31')), {
    period: 'custom',
    from: '2026-01-01',
    to: '2026-01-31',
  })
})

test('series styles: core solid by id, extras dashed in selection order', () => {
  const st = seriesStyles([12, 3, 1, 7], catalog)
  assert.deepEqual(st.get(1), { color: 'var(--core-1)', dashed: false })
  assert.deepEqual(st.get(3), { color: 'var(--core-3)', dashed: false })
  assert.deepEqual(st.get(12), { color: 'var(--extra-1)', dashed: true })
  assert.deepEqual(st.get(7), { color: 'var(--extra-2)', dashed: true })
})

test('formatDelta', () => {
  assert.equal(formatDelta(1.64), '+1.6')
  assert.equal(formatDelta(-22.5), '-22.5')
  assert.equal(formatDelta(0.01), '0')
  assert.equal(formatDelta(null), '')
})

test('department filters are tab-local', () => {
  const s = parse('pdept=3')
  assert.equal(s.participationDept, 3)
  assert.equal(s.employeesDept, null)
  assert.equal(s.dataDept, null)
  assert.equal(serializeState({ ...s, dataDept: 'none' }).toString(), 'pdept=3&ddept=none')
})

test('compared departments drop ids that no longer exist', () => {
  const known = [{ id: 3 }, { id: 6 }]
  assert.deepEqual(knownDepartments([3, 99, 'none', 6], known), [3, 'none', 6])
  assert.deepEqual(knownDepartments([99], null), [99])
  assert.deepEqual(knownDepartments([99], []), [])
})
