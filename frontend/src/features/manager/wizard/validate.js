// Wizard model + UX-mirror validation + payload builder. Pure (no React, no DOM), unit-tested.
// The server (backend/internal/assessments/validate.go) is authoritative: this file only gives
// early inline feedback and maps the contract's 422 `{path, code}` list back onto wizard fields.
// There is deliberately NO scoring here: effects stay qualitative (direction + level).

export const LIMITS = { title: 200, description: 4000, question: 1000, option: 500, label: 120, minOptions: 2, maxOptions: 10 }
export const POINT_CHOICES = [5, 7]
export const MAX_POINTS = 7

// Select order: strongest lowering ... none ... strongest raising.
export const EFFECT_CODES = ['down3', 'down2', 'down1', 'none', 'up1', 'up2', 'up3']
const TO_API = {
  down3: { direction: -1, level: 'high' },
  down2: { direction: -1, level: 'moderate' },
  down1: { direction: -1, level: 'low' },
  up1: { direction: 1, level: 'low' },
  up2: { direction: 1, level: 'moderate' },
  up3: { direction: 1, level: 'high' },
}

export function effectToApi(code) {
  return TO_API[code] || null
}

export function effectFromApi(direction, level) {
  for (const [code, v] of Object.entries(TO_API)) {
    if (v.direction === direction && v.level === level) return code
  }
  return 'none'
}

export const isRaising = (code) => code === 'up1' || code === 'up2' || code === 'up3'
export const isLowering = (code) => code === 'down1' || code === 'down2' || code === 'down3'

let seq = 0
export const uid = () => {
  seq += 1
  return `k${Date.now().toString(36)}${seq}`
}

export const newOption = (text = '') => ({ key: uid(), text, fx: {} })

export function newQuestion(type = 'single') {
  return {
    key: uid(),
    text: '',
    type,
    options: [newOption(), newOption()],
    points: 5,
    low: '',
    high: '',
    scale: Array.from({ length: MAX_POINTS }, () => ({ key: uid(), fx: {} })),
  }
}

export const initialState = () => ({ v: 1, step: 0, title: '', description: '', hidden: false, expires: '', questions: [newQuestion()] })

// The options that are actually sent/edited for a question (choice options or the scale points).
export const effectiveOptions = (q) => (q.type === 'rating' ? q.scale.slice(0, q.points) : q.options)

export function pointText(q, i) {
  const n = q.points
  const num = String(i + 1)
  if (i === 0 && q.low.trim()) return `${num} · ${q.low.trim()}`
  if (i === n - 1 && q.high.trim()) return `${num} · ${q.high.trim()}`
  return num
}

// Local end of the chosen day, so "until 31 Dec" includes the whole day. '' -> null.
export function expiryToISO(expires) {
  if (!expires) return null
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(expires)
  if (!m) return null
  const d = new Date(Number(m[1]), Number(m[2]) - 1, Number(m[3]), 23, 59, 59)
  return Number.isNaN(d.getTime()) ? null : d.toISOString()
}

const len = (s) => Array.from(String(s ?? '').trim()).length
const longer = (s, max) => Array.from(String(s ?? '')).length > max

// ---- step validators: return { [path]: code } with the server's path/code vocabulary ----

export function validateMeta(state, now = new Date()) {
  const e = {}
  if (len(state.title) === 0) e.title = 'required'
  else if (len(state.title) > LIMITS.title) e.title = 'too_long'
  if (longer(state.description, LIMITS.description)) e.description = 'too_long'
  if (state.expires) {
    const iso = expiryToISO(state.expires)
    if (!iso) e.expiresAt = 'invalid_date'
    else if (new Date(iso) <= now) e.expiresAt = 'past_expiry'
  }
  return e
}

export function validateQuestions(state) {
  const e = {}
  if (state.questions.length === 0) e.questions = 'required'
  state.questions.forEach((q, qi) => {
    const qp = `questions[${qi}]`
    if (len(q.text) === 0) e[`${qp}.text`] = 'required'
    else if (len(q.text) > LIMITS.question) e[`${qp}.text`] = 'too_long'
    if (q.type === 'rating') {
      if (len(pointText(q, 0)) > LIMITS.option || len(pointText(q, q.points - 1)) > LIMITS.option) e[`${qp}.labels`] = 'too_long'
      return
    }
    if (q.options.length < LIMITS.minOptions) e[`${qp}.options`] = 'too_few_options'
    q.options.forEach((o, oi) => {
      if (len(o.text) === 0) e[`${qp}.options[${oi}].text`] = 'required'
      else if (len(o.text) > LIMITS.option) e[`${qp}.options[${oi}].text`] = 'too_long'
    })
  })
  return e
}

// At least one option must be able to RAISE a characteristic (otherwise nothing is measurable).
export function validateEffects(state, knownIds) {
  const e = {}
  let raising = false
  for (const q of state.questions) {
    for (const o of effectiveOptions(q)) {
      for (const [id, code] of Object.entries(o.fx)) {
        if (knownIds && !knownIds.has(Number(id))) continue
        if (isRaising(code)) raising = true
      }
    }
  }
  if (!raising) e.questions = 'no_measurable_characteristic'
  return e
}

export function validateAll(state, knownIds, now) {
  return { ...validateMeta(state, now), ...validateQuestions(state), ...validateEffects(state, knownIds) }
}

// ---- path routing ----

// Which wizard step shows an error at this (normalized) path.
export function stepOfPath(path, code) {
  if (/^(title|description|expiresAt|hidden)/.test(path)) return 0
  if (code === 'no_measurable_characteristic' || /\.fx\./.test(path)) return 2
  return 1
}

export function firstErrorStep(errors) {
  let min = null
  for (const [path, code] of Object.entries(errors)) {
    const s = stepOfPath(path, code)
    if (min === null || s < min) min = s
  }
  return min
}

export function errorsForStep(errors, step) {
  const out = {}
  for (const [path, code] of Object.entries(errors)) if (stepOfPath(path, code) === step) out[path] = code
  return out
}

// ---- payload ----

// Builds the contract body. `refs` records which characteristic id sits at each effects[k]
// so a 422 path like questions[0].options[1].effects[2].characteristicId can be mapped back to a UI field.
export function buildPayload(state, knownIds) {
  const refs = {}
  const questions = state.questions.map((q, qi) => ({
    text: q.text.trim(),
    type: q.type,
    options: effectiveOptions(q).map((o, oi) => {
      const effects = []
      const ids = []
      for (const [id, code] of Object.entries(o.fx)) {
        const api = effectToApi(code)
        if (!api) continue
        if (knownIds && !knownIds.has(Number(id))) continue
        effects.push({ characteristicId: Number(id), level: api.level, direction: api.direction })
        ids.push(Number(id))
      }
      refs[`questions[${qi}].options[${oi}]`] = ids
      return { text: q.type === 'rating' ? pointText(q, oi) : o.text.trim(), effects }
    }),
  }))
  return {
    payload: {
      title: state.title.trim(),
      description: state.description.trim(),
      expiresAt: expiryToISO(state.expires),
      hidden: !!state.hidden,
      questions,
    },
    refs,
  }
}

// Server 422 list -> { [uiPath]: code } (effects become questions[i].options[j].fx.<charId>).
export function normalizeServerErrors(list, refs, state) {
  const out = {}
  for (const item of Array.isArray(list) ? list : []) {
    if (!item || typeof item.path !== 'string') continue
    let path = item.path
    const fx = /^(questions\[\d+\]\.options\[\d+\])\.effects\[(\d+)\]\.\w+$/.exec(path)
    if (fx) {
      const id = refs?.[fx[1]]?.[Number(fx[2])]
      path = id === undefined ? fx[1] : `${fx[1]}.fx.${id}`
    } else {
      const opt = /^questions\[(\d+)\]\.options\[\d+\]\.text$/.exec(path)
      if (opt && state?.questions?.[Number(opt[1])]?.type === 'rating') path = `questions[${opt[1]}].labels`
    }
    if (!(path in out)) out[path] = item.code || 'invalid'
  }
  return out
}

// Advisory only: with NO lowering effect on a characteristic anywhere, if some question forces a
// raising option (every option raises it), the 0% minimum cannot be reached. The server's
// min_unreachable warning is authoritative and covers the mixed cases.
export function mayMissZero(state, knownIds) {
  const ids = new Set()
  for (const q of state.questions) for (const o of effectiveOptions(q)) for (const id of Object.keys(o.fx)) ids.add(Number(id))
  const out = []
  for (const id of ids) {
    if (knownIds && !knownIds.has(id)) continue
    const all = state.questions.flatMap((q) => effectiveOptions(q))
    if (all.some((o) => isLowering(o.fx[id]))) continue
    const forced = state.questions.some((q) => {
      const opts = effectiveOptions(q)
      return opts.length > 0 && opts.every((o) => isRaising(o.fx[id]))
    })
    if (forced) out.push(id)
  }
  return out
}

// ---- draft (localStorage) ----

function cleanFx(fx) {
  const out = {}
  if (fx && typeof fx === 'object') {
    for (const [id, code] of Object.entries(fx)) {
      if (/^\d+$/.test(id) && TO_API[code]) out[id] = code
    }
  }
  return out
}

export function sanitizeDraft(raw) {
  if (!raw || raw.v !== 1 || !Array.isArray(raw.questions)) return null
  const base = initialState()
  const questions = raw.questions.slice(0, 200).map((q) => {
    const nq = newQuestion(['single', 'multiple', 'rating'].includes(q?.type) ? q.type : 'single')
    nq.text = String(q?.text ?? '')
    if (Array.isArray(q?.options) && q.options.length) {
      nq.options = q.options.slice(0, LIMITS.maxOptions).map((o) => ({ key: uid(), text: String(o?.text ?? ''), fx: cleanFx(o?.fx) }))
    }
    nq.points = POINT_CHOICES.includes(q?.points) ? q.points : 5
    nq.low = String(q?.low ?? '')
    nq.high = String(q?.high ?? '')
    if (Array.isArray(q?.scale)) {
      q.scale.slice(0, MAX_POINTS).forEach((p, i) => {
        nq.scale[i].fx = cleanFx(p?.fx)
      })
    }
    return nq
  })
  return {
    ...base,
    step: Number.isInteger(raw.step) ? Math.min(Math.max(raw.step, 0), 3) : 0,
    title: String(raw.title ?? ''),
    description: String(raw.description ?? ''),
    hidden: !!raw.hidden,
    expires: /^\d{4}-\d{2}-\d{2}$/.test(raw.expires || '') ? raw.expires : '',
    questions: questions.length ? questions : base.questions,
  }
}

// A draft with nothing typed is not worth restoring.
export function isPristine(state) {
  if (state.title.trim() || state.description.trim() || state.expires || state.hidden) return false
  return state.questions.every(
    (q) =>
      !q.text.trim() &&
      !q.low.trim() &&
      !q.high.trim() &&
      q.options.every((o) => !o.text.trim() && !Object.keys(o.fx).length) &&
      q.scale.every((p) => !Object.keys(p.fx).length),
  )
}
