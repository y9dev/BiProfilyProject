// UX-mirror tests. Server validation stays authoritative (backend/internal/assessments/validate_test.go).
// Run: node --test frontend/src
import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  buildPayload,
  effectFromApi,
  effectToApi,
  errorsForStep,
  expiryToISO,
  firstErrorStep,
  initialState,
  isPristine,
  mayMissZero,
  newOption,
  newQuestion,
  normalizeServerErrors,
  pointText,
  sanitizeDraft,
  stepOfPath,
  validateAll,
  validateEffects,
  validateMeta,
  validateQuestions,
} from './validate.js'

const known = new Set([1, 2, 3])

function filled() {
  const s = initialState()
  s.title = 'Teamwork'
  const q = s.questions[0]
  q.text = 'What do you do?'
  q.options[0].text = 'Call'
  q.options[1].text = 'Escalate'
  q.options[0].fx = { 1: 'up3', 3: 'down1' }
  return s
}

test('effect codes map to the contract direction + level', () => {
  assert.deepEqual(effectToApi('down3'), { direction: -1, level: 'high' })
  assert.deepEqual(effectToApi('down2'), { direction: -1, level: 'moderate' })
  assert.deepEqual(effectToApi('down1'), { direction: -1, level: 'low' })
  assert.deepEqual(effectToApi('up1'), { direction: 1, level: 'low' })
  assert.deepEqual(effectToApi('up2'), { direction: 1, level: 'moderate' })
  assert.deepEqual(effectToApi('up3'), { direction: 1, level: 'high' })
  assert.equal(effectToApi('none'), null)
  assert.equal(effectFromApi(-1, 'moderate'), 'down2')
  assert.equal(effectFromApi(1, 'high'), 'up3')
})

test('meta: title required, expiry in the past', () => {
  const s = initialState()
  assert.equal(validateMeta(s).title, 'required')
  s.title = 'x'
  s.expires = '2000-01-01'
  assert.equal(validateMeta(s).expiresAt, 'past_expiry')
  s.expires = '2999-01-01'
  assert.deepEqual(validateMeta(s), {})
  assert.equal(expiryToISO(''), null)
  assert.match(expiryToISO('2999-01-01'), /^2999-01-0[12]T/)
})

test('questions: texts required, at least 2 options for choice', () => {
  const s = initialState()
  s.questions[0].options.pop()
  const e = validateQuestions(s)
  assert.equal(e['questions[0].text'], 'required')
  assert.equal(e['questions[0].options'], 'too_few_options')
  assert.equal(e['questions[0].options[0].text'], 'required')
  s.questions = []
  assert.equal(validateQuestions(s).questions, 'required')
})

test('rating needs no option texts; labels are optional', () => {
  const s = initialState()
  s.questions[0] = Object.assign(newQuestion('rating'), { text: 'How sure?' })
  assert.deepEqual(validateQuestions(s), {})
  s.questions[0].low = 'Not like me at all'
  s.questions[0].high = 'Exactly like me'
  assert.equal(pointText(s.questions[0], 0), '1 · Not like me at all')
  assert.equal(pointText(s.questions[0], 2), '3')
  assert.equal(pointText(s.questions[0], 4), '5 · Exactly like me')
})

test('effects: needs at least one raising effect among known characteristics', () => {
  const s = filled()
  assert.deepEqual(validateEffects(s, known), {})
  s.questions[0].options[0].fx = { 3: 'down1' }
  assert.equal(validateEffects(s, known).questions, 'no_measurable_characteristic')
  s.questions[0].options[0].fx = { 99: 'up1' }
  assert.equal(validateEffects(s, known).questions, 'no_measurable_characteristic')
  assert.deepEqual(validateAll(filled(), known), {})
})

test('payload: omits "none", maps qualitative levels, rating options are scale points', () => {
  const s = filled()
  s.questions[0].options[1].fx = { 2: 'none' }
  const r = Object.assign(newQuestion('rating'), { text: 'Sure?', low: 'Low', high: 'High', points: 7 })
  r.scale[1].fx = { 2: 'up2' }
  s.questions.push(r)
  const { payload, refs } = buildPayload(s, known)
  assert.equal(payload.hidden, false)
  assert.equal(payload.expiresAt, null)
  assert.deepEqual(payload.questions[0].options[0].effects, [
    { characteristicId: 1, level: 'high', direction: 1 },
    { characteristicId: 3, level: 'low', direction: -1 },
  ])
  assert.deepEqual(payload.questions[0].options[1].effects, [])
  assert.equal(payload.questions[1].options.length, 7)
  assert.equal(payload.questions[1].options[0].text, '1 · Low')
  assert.equal(payload.questions[1].options[6].text, '7 · High')
  assert.deepEqual(payload.questions[1].options[1].effects, [{ characteristicId: 2, level: 'moderate', direction: 1 }])
  assert.deepEqual(refs['questions[0].options[0]'], [1, 3])
})

test('server errors are mapped back to fields and steps', () => {
  const s = filled()
  const r = Object.assign(newQuestion('rating'), { text: 'Sure?' })
  s.questions.push(r)
  const { refs } = buildPayload(s, known)
  const out = normalizeServerErrors(
    [
      { path: 'title', code: 'required' },
      { path: 'questions[0].options[0].effects[1].characteristicId', code: 'unknown_characteristic' },
      { path: 'questions[1].options[0].text', code: 'too_long' },
      { path: 'questions[0].options', code: 'too_few_options' },
      { path: 'questions', code: 'no_measurable_characteristic' },
      { path: 'title', code: 'too_long' },
    ],
    refs,
    s,
  )
  assert.equal(out.title, 'required')
  assert.equal(out['questions[0].options[0].fx.3'], 'unknown_characteristic')
  assert.equal(out['questions[1].labels'], 'too_long')
  assert.equal(out['questions[0].options'], 'too_few_options')
  assert.equal(stepOfPath('title', 'required'), 0)
  assert.equal(stepOfPath('expiresAt', 'past_expiry'), 0)
  assert.equal(stepOfPath('questions[0].options', 'too_few_options'), 1)
  assert.equal(stepOfPath('questions', 'no_measurable_characteristic'), 2)
  assert.equal(stepOfPath('questions[0].options[0].fx.3', 'unknown_characteristic'), 2)
  assert.equal(firstErrorStep({ 'questions[0].text': 'required', 'questions[0].options[0].fx.1': 'x' }), 1)
  assert.equal(firstErrorStep({}), null)
  assert.deepEqual(Object.keys(errorsForStep(out, 2)).sort(), ['questions', 'questions[0].options[0].fx.3'])
})

test('mayMissZero hints only when nothing lowers the characteristic and a question forces a raise', () => {
  const s = initialState()
  s.questions[0].options[0].fx = { 1: 'up1' }
  s.questions[0].options[1].fx = { 1: 'up2' }
  assert.deepEqual(mayMissZero(s, known), [1])
  s.questions[0].options[1].fx = { 1: 'down1' }
  assert.deepEqual(mayMissZero(s, known), [])
  s.questions[0].options[1].fx = {}
  assert.deepEqual(mayMissZero(s, known), [])
})

test('draft sanitize drops junk, keeps valid data; pristine detection', () => {
  assert.equal(sanitizeDraft(null), null)
  assert.equal(sanitizeDraft({ v: 2, questions: [] }), null)
  const raw = JSON.parse(JSON.stringify(filled()))
  raw.step = 99
  raw.expires = 'garbage'
  raw.questions[0].options[0].fx = { 1: 'up3', x: 'up1', 2: 'evil' }
  const d = sanitizeDraft(raw)
  assert.equal(d.step, 3)
  assert.equal(d.expires, '')
  assert.deepEqual(d.questions[0].options[0].fx, { 1: 'up3' })
  assert.equal(d.questions[0].scale.length, 7)
  assert.equal(isPristine(initialState()), true)
  assert.equal(isPristine(filled()), false)
  const q = newQuestion()
  q.options.push(newOption('a'))
  assert.equal(q.options.length, 3)
})
