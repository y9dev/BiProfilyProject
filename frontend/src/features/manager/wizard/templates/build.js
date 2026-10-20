// Ready-made assessment templates. A template is plain data (see construction-staff.json); opening one
// only prefills the existing wizard state, so editing, validation and publishing are the normal flow.
// Effects are keyed by characteristic NAME in the JSON and resolved to ids against the live list here,
// so nothing about scoring is decided on the client: the server stores and applies the weights.
import { initialState, newOption, newQuestion } from '../validate.js'

const levelCode = (n) => (n > 0 ? `up${n}` : `down${-n}`)

const nameOf = (c) => [c.name, c.nameI18n?.ru]

// Returns { state } (a brand-new draft; every call builds fresh objects and keys) or
// { missing } with the trait names that have no matching characteristic.
export function buildTemplateState(template, characteristics) {
  const idByTrait = new Map()
  for (const trait of template.traits) {
    const c = characteristics.find((x) => nameOf(x).includes(trait))
    if (c) idByTrait.set(trait, c.id)
  }
  const missing = template.traits.filter((t) => !idByTrait.has(t))
  if (missing.length) return { missing }

  const questions = template.questions.map((q) => {
    const nq = newQuestion(q.type)
    nq.text = `${q.title}\n${q.text}`
    nq.options = q.options.map((o) => {
      const opt = newOption(o.text)
      for (const [trait, n] of Object.entries(o.effects)) opt.fx[String(idByTrait.get(trait))] = levelCode(n)
      return opt
    })
    return nq
  })
  const description = [template.description, template.instructions].filter(Boolean).join('\n\n')
  return { state: { ...initialState(), title: template.title, description, questions } }
}
