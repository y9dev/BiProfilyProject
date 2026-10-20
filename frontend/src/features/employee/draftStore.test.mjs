import { test, beforeEach } from 'node:test'
import assert from 'node:assert/strict'

const store = new Map()
globalThis.localStorage = {
  getItem: (k) => (store.has(k) ? store.get(k) : null),
  setItem: (k, v) => void store.set(k, String(v)),
  removeItem: (k) => void store.delete(k),
  key: (i) => [...store.keys()][i] ?? null,
  get length() {
    return store.size
  },
}

const d = await import('./draftStore.js')

const view = {
  questions: [
    { id: 1, type: 'single', options: [{ id: 10 }, { id: 11 }] },
    { id: 2, type: 'multiple', options: [{ id: 20 }, { id: 21 }, { id: 22 }] },
    { id: 3, type: 'rating', options: [{ id: 30 }, { id: 31 }] },
  ],
}

beforeEach(() => store.clear())

test('save then load restores answers and index under the namespaced key', () => {
  d.saveDraft(7, 5, view, { idx: 1, answers: { 1: [10], 2: [20, 22] } })
  assert.ok(store.has('bip:v1:draft:7:5'))
  assert.deepEqual(d.loadDraft(7, 5, view), { idx: 1, answers: { 1: [10], 2: [20, 22] } })
})

test('fingerprint mismatch discards the draft', () => {
  d.saveDraft(7, 5, view, { idx: 1, answers: { 1: [10] } })
  const changed = { questions: [{ id: 1, type: 'single', options: [{ id: 10 }, { id: 99 }] }] }
  assert.equal(d.loadDraft(7, 5, changed), null)
  assert.equal(store.has('bip:v1:draft:7:5'), false)
})

test('sanitize drops unknown ids, clamps idx, enforces single cardinality', () => {
  const out = d.sanitize({ idx: 99, answers: { 1: [10, 11], 2: [20, 777], 42: [1], 3: 'x' } }, view)
  assert.deepEqual(out, { idx: 2, answers: { 1: [10], 2: [20] } })
  assert.equal(d.sanitize({ idx: -4, answers: {} }, view).idx, 0)
})

test('drafts are keyed per user', () => {
  d.saveDraft(1, 5, view, { idx: 0, answers: { 1: [10] } })
  assert.equal(d.loadDraft(2, 5, view), null)
})

test('clearDraft removes the key; corrupt JSON is ignored', () => {
  d.saveDraft(7, 5, view, { idx: 0, answers: {} })
  d.clearDraft(7, 5)
  assert.equal(store.has('bip:v1:draft:7:5'), false)
  store.set('bip:v1:draft:7:6', '{oops')
  assert.equal(d.loadDraft(7, 6, view), null)
})

test('purgeOldDrafts removes drafts older than 30 days only', () => {
  const now = Date.now()
  store.set('bip:v1:draft:1:1', JSON.stringify({ v: 1, savedAt: now - 31 * 86400000 }))
  store.set('bip:v1:draft:1:2', JSON.stringify({ v: 1, savedAt: now - 1000 }))
  d.purgeOldDrafts(now)
  assert.equal(store.has('bip:v1:draft:1:1'), false)
  assert.equal(store.has('bip:v1:draft:1:2'), true)
})

test('buildPayload keeps question order and empty arrays for unanswered', () => {
  assert.deepEqual(d.buildPayload(view, { 2: [21], 1: [11] }), [
    { questionId: 1, optionIds: [11] },
    { questionId: 2, optionIds: [21] },
    { questionId: 3, optionIds: [] },
  ])
})

test('debounced saver writes once after the delay; cancel drops pending', async () => {
  const s = d.createDraftSaver(7, 5, view, 20)
  s.save({ idx: 0, answers: { 1: [10] } })
  s.save({ idx: 1, answers: { 1: [11] } })
  assert.equal(store.has('bip:v1:draft:7:5'), false)
  await new Promise((r) => setTimeout(r, 50))
  assert.equal(d.loadDraft(7, 5, view).idx, 1)
  d.clearDraft(7, 5)
  s.save({ idx: 0, answers: {} })
  s.cancel()
  await new Promise((r) => setTimeout(r, 50))
  assert.equal(store.has('bip:v1:draft:7:5'), false)
})
