// Client-side draft of an in-progress assessment (plan section 8).
// Key: bip:v1:draft:{userId}:{assessmentId}; value: {v:1, fp, idx, answers:{[questionId]:optionId[]}, savedAt}.
// Drafts are only ever sent as the single final submit payload.
import { getJSON, keys, keysWithPrefix, removeItem, setJSON } from '../../lib/storage.js'

const MAX_AGE_MS = 30 * 24 * 60 * 60 * 1000
const SAVE_DELAY_MS = 300

// FNV-1a over question ids and option ids in order.
export function fingerprint(takeView) {
  const parts = []
  for (const q of takeView?.questions ?? []) {
    parts.push(`q${q.id}`)
    for (const o of q.options ?? []) parts.push(`o${o.id}`)
  }
  let h = 0x811c9dc5
  const s = parts.join(',')
  for (let i = 0; i < s.length; i += 1) {
    h ^= s.charCodeAt(i)
    h = Math.imul(h, 0x01000193) >>> 0
  }
  return h.toString(16)
}

// Drops unknown question/option ids, enforces single-choice cardinality, clamps idx.
export function sanitize(draft, takeView) {
  const questions = takeView?.questions ?? []
  const answers = {}
  const src = draft && typeof draft.answers === 'object' && draft.answers ? draft.answers : {}
  for (const q of questions) {
    const raw = src[q.id]
    if (!Array.isArray(raw)) continue
    const valid = new Set(q.options.map((o) => o.id))
    let ids = [...new Set(raw.filter((id) => valid.has(id)))]
    if (q.type !== 'multiple') ids = ids.slice(0, 1)
    if (ids.length) answers[q.id] = ids
  }
  const max = Math.max(questions.length - 1, 0)
  const n = Number.isInteger(draft?.idx) ? draft.idx : 0
  return { idx: Math.min(Math.max(n, 0), max), answers }
}

export function loadDraft(userId, assessmentId, takeView) {
  const d = getJSON(keys.draft(userId, assessmentId))
  if (!d || d.v !== 1 || d.fp !== fingerprint(takeView)) {
    if (d) removeItem(keys.draft(userId, assessmentId))
    return null
  }
  return sanitize(d, takeView)
}

export function saveDraft(userId, assessmentId, takeView, { idx, answers }) {
  return setJSON(keys.draft(userId, assessmentId), {
    v: 1,
    fp: fingerprint(takeView),
    idx,
    answers,
    savedAt: Date.now(),
  })
}

export function clearDraft(userId, assessmentId) {
  removeItem(keys.draft(userId, assessmentId))
}

// Debounced writer. flush() writes immediately; cancel() drops a pending write (use before clear).
export function createDraftSaver(userId, assessmentId, takeView, delay = SAVE_DELAY_MS) {
  let timer = null
  let pending = null
  const write = () => {
    timer = null
    if (pending) saveDraft(userId, assessmentId, takeView, pending)
    pending = null
  }
  return {
    save(state) {
      pending = state
      if (timer) clearTimeout(timer)
      timer = setTimeout(write, delay)
    },
    flush() {
      if (timer) clearTimeout(timer)
      write()
    },
    cancel() {
      if (timer) clearTimeout(timer)
      timer = null
      pending = null
    },
  }
}

export function purgeOldDrafts(now = Date.now()) {
  for (const k of keysWithPrefix(keys.draftPrefix())) {
    const d = getJSON(k)
    if (!d || typeof d.savedAt !== 'number' || now - d.savedAt > MAX_AGE_MS) removeItem(k)
  }
}

// Answers payload for the submit request, in question order.
export function buildPayload(takeView, answers) {
  return takeView.questions.map((q) => ({ questionId: q.id, optionIds: answers[q.id] ?? [] }))
}

export function isAnswered(answers, q) {
  return (answers[q.id]?.length ?? 0) > 0
}
