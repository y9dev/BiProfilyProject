import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import { buildPayload, validateAll } from '../validate.js'
import { buildTemplateState } from './build.js'

const template = JSON.parse(readFileSync(new URL('./construction-staff.json', import.meta.url), 'utf8'))

// Characteristics as the API returns them (the four defaults carry nameI18n) plus a custom one.
const CHARS = [
  { id: 11, name: 'Критическое мышление', nameI18n: { ru: 'Критическое мышление', kk: 'Сыни ойлау' } },
  { id: 12, name: 'Уверенность в себе', nameI18n: { ru: 'Уверенность в себе', kk: 'Өзіне сену' } },
  { id: 13, name: 'Дисциплина', nameI18n: { ru: 'Дисциплина', kk: 'Тәртіп' } },
  { id: 14, name: 'Вовлечённость', nameI18n: { ru: 'Вовлечённость', kk: 'Қатысу' } },
  { id: 99, name: 'Интеллект', nameI18n: null },
]
const ID = { КМ: '11', УВ: '12', ДЦ: '13', ВЛ: '14' }

// The supplied scoring key, typed independently of the JSON, in the original notation.
const KEY = `
1 A: КМ +2, УВ +1, ВЛ +1
1 B: КМ +1, ДЦ +1, ВЛ −1
1 C: КМ +1, УВ −1, ДЦ +2
1 D: КМ +1, УВ +1, ВЛ +1
2 A: КМ +1, ДЦ +2, ВЛ +1
2 B: УВ −1, ДЦ +1, ВЛ −2
2 C: КМ +1, УВ +1, ДЦ +2, ВЛ +1
2 D: КМ +1, УВ +1, ДЦ −1
3 A: КМ +1, УВ +2, ДЦ +1, ВЛ +1
3 B: УВ −1, ДЦ +2, ВЛ −1
3 C: КМ +2, УВ −1, ВЛ +2
3 D: КМ +2, УВ +1, ВЛ +1
4 A: КМ +1, УВ +1, ДЦ +2, ВЛ +1
4 B: УВ +2, ДЦ +1
4 C: КМ +2, ДЦ +1, ВЛ −1
4 D: КМ +1, УВ −1, ВЛ +1
5 A: КМ +2, УВ +1, ДЦ +1, ВЛ +1
5 B: КМ +1, УВ −1, ДЦ +2, ВЛ −1
5 C: КМ +2, УВ +1, ВЛ +1
5 D: КМ +1, ВЛ +2
6 A: КМ +1, УВ +2, ДЦ +2, ВЛ +1
6 B: КМ +1, УВ −1, ДЦ +2
6 C: КМ +1, УВ +2, ВЛ +2
6 D: КМ +1, ДЦ +2, ВЛ −1
7 A: КМ +2, УВ +1, ДЦ +2
7 B: КМ +1, УВ −1, ДЦ +2, ВЛ −1
7 C: КМ +1, ВЛ +2
7 D: КМ +2, УВ +1, ДЦ +1
8 A: КМ +1, УВ +2, ВЛ +2
8 B: КМ +2, ДЦ +1, ВЛ +1
8 C: КМ +1, ДЦ +2, ВЛ +1
8 D: УВ +1, ДЦ +1, ВЛ +2
9 A: КМ +2, УВ +1, ДЦ +2, ВЛ +1
9 B: КМ +1, УВ −1, ДЦ +2, ВЛ +1
9 C: КМ +2, УВ +1, ДЦ +1
9 D: КМ +1, ДЦ +2, ВЛ −1
10 A: КМ +1, УВ +1, ДЦ +2, ВЛ +1
10 B: КМ +2, УВ +2, ВЛ −1
10 C: КМ +1, УВ +1, ДЦ +2
10 D: КМ +1, УВ −1, ВЛ +2
11 A: КМ +2, УВ +1, ДЦ +2, ВЛ +1
11 B: КМ +1, УВ +2, ДЦ +2
11 C: КМ +1, ДЦ +1, ВЛ +2
11 D: КМ +1, УВ −1, ДЦ +2
12 A: КМ +2, УВ +1, ДЦ +2, ВЛ +1
12 B: УВ +1, ДЦ +2, ВЛ −1
12 C: КМ +1, УВ +2, ВЛ +2
12 D: КМ +1, ДЦ +2
`

const TITLES = [
  'Несоответствие на объекте', 'Сжатые сроки', 'Несогласие с руководителем', 'Ошибка в собственной работе',
  'Изменение условий', 'Зависимость от другой бригады', 'Неоднозначная инструкция', 'Инициатива без поручения',
  'Противоречивые данные', 'Реакция на обратную связь', 'Повторяющаяся проблема', 'Выбор при ограниченных ресурсах',
]
const FIRST_WORDS = [
  ['Проверю размеры', 'Продолжу подготовку', 'Приостановлю связанные', 'Сопоставлю ситуацию'],
  ['Оставлю проверки', 'Сохраню привычный', 'Определю, какие', 'Предложу выборочно'],
  ['Кратко изложу', 'Поддержу решение', 'Попрошу коллег', 'Предложу альтернативу'],
  ['Сообщу ответственному', 'Исправлю ошибку', 'Сначала выясню, является', 'Попрошу опытного'],
  ['Выделю операции', 'Дождусь подтверждённой', 'Подготовлю предварительный', 'Обсужу с бригадой'],
  ['Согласую промежуточный', 'Передам руководителю участка', 'Попробую договориться', 'Буду исходить'],
  ['Изучу связанные', 'Не начну операцию', 'Уточню понимание', 'Подготовлю оба'],
  ['Предложу небольшой', 'Сначала выясню, замечают', 'Составлю краткое', 'Попробую улучшить'],
  ['Сверю документы', 'Попрошу ответственного', 'Проверю наиболее', 'Зафиксирую расхождение'],
  ['Исправлю подтверждённые', 'Самостоятельно проанализирую', 'Согласую с руководителем', 'Обсужу замечания'],
  ['Соберу данные', 'Организую более раннюю', 'Обсужу проблему', 'Передам руководителю конкретные'],
  ['Оценю последствия', 'Завершу плановую', 'Помогу коллеге', 'Подготовлю сведения'],
]

// "1 A: КМ +2, УВ −1" -> { q: 0, o: 0, fx: { '11': 'up2', '12': 'down1' } }
function parseKey() {
  return KEY.trim()
    .split('\n')
    .map((line) => {
      const [, q, letter, rest] = /^(\d+) ([A-D]): (.+)$/.exec(line.trim())
      const fx = {}
      for (const part of rest.split(', ')) {
        const [, abbr, sign, n] = /^(\S+) ([+−-])(\d)$/.exec(part)
        fx[ID[abbr]] = `${sign === '+' ? 'up' : 'down'}${n}`
      }
      return { q: Number(q) - 1, o: 'ABCD'.indexOf(letter), fx }
    })
}

const build = () => {
  const r = buildTemplateState(template, CHARS)
  assert.ok(r.state, 'template resolves against the characteristics')
  return r.state
}

test('prefill: metadata, 12 questions, 48 options in A-D order', () => {
  const s = build()
  assert.equal(s.title, 'Оценка профессиональных характеристик строительного персонала')
  assert.match(s.description, /^Ситуационный тест для оценки четырёх профессиональных характеристик/)
  assert.match(s.description, /Результат формируется как четыре независимых показателя, а не единая оценка личности\./)
  assert.match(s.description, /Выберите вариант, который наиболее точно описывает ваши действия/)
  assert.equal(s.questions.length, 12)
  assert.equal(s.questions.flatMap((q) => q.options).length, 48)
  s.questions.forEach((q, i) => {
    assert.equal(q.type, 'single')
    assert.equal(q.options.length, 4)
    assert.ok(q.text.startsWith(`${TITLES[i]}\n`), `title of question ${i + 1}`)
    q.options.forEach((o, j) => assert.ok(o.text.startsWith(FIRST_WORDS[i][j]), `question ${i + 1} option ${'ABCD'[j]}`))
  })
  assert.equal(s.step, 0)
  assert.equal(s.hidden, false)
})

test('scoring entries map to the right question, option and trait (all 48, zeros stay empty)', () => {
  const s = build()
  const entries = parseKey()
  assert.equal(entries.length, 48)
  for (const { q, o, fx } of entries) {
    assert.deepEqual(s.questions[q].options[o].fx, fx, `question ${q + 1} option ${'ABCD'[o]}`)
  }
})

test('payload sends direction and level; nothing for unlisted traits; template passes client validation', () => {
  const s = build()
  const known = new Set(CHARS.map((c) => c.id))
  assert.deepEqual(validateAll(s, known), {})
  const { payload } = buildPayload(s, known)
  assert.equal(payload.questions.length, 12)
  // Question 2 option B: УВ -1, ДЦ +1, ВЛ -2
  const eff = payload.questions[1].options[1].effects
  assert.deepEqual(
    eff.map((e) => [e.characteristicId, e.direction, e.level]).sort((a, b) => a[0] - b[0]),
    [[12, -1, 'low'], [13, 1, 'low'], [14, -1, 'moderate']],
  )
  // Question 4 option B has only УВ and ДЦ: КМ and ВЛ are absent (zero)
  assert.deepEqual(payload.questions[3].options[1].effects.map((e) => e.characteristicId).sort(), [12, 13])
  assert.equal(payload.hidden, false)
})

test('every call is an independent draft; the template definition is never mutated', () => {
  const before = JSON.stringify(template)
  const a = build()
  const b = build()
  a.title = 'changed'
  a.questions[0].options[0].fx['11'] = 'down3'
  a.questions[0].options[0].text = 'edited'
  a.questions.pop()
  assert.equal(b.title, template.title)
  assert.equal(b.questions.length, 12)
  assert.equal(b.questions[0].options[0].fx['11'], 'up2')
  assert.notEqual(b.questions[0].key, a.questions[0].key)
  assert.equal(JSON.stringify(template), before)
  const keys = new Set(build().questions.flatMap((q) => [q.key, ...q.options.map((o) => o.key)]))
  assert.equal(keys.size, 12 + 48)
})

test('missing characteristics are reported instead of silently dropping scoring', () => {
  const r = buildTemplateState(template, CHARS.filter((c) => c.id !== 13))
  assert.equal(r.state, undefined)
  assert.deepEqual(r.missing, ['Дисциплина'])
})

test('traits are matched by name or Russian i18n name, ignoring other characteristics', () => {
  const renamed = CHARS.map((c) => (c.id === 11 ? { ...c, name: 'Critical thinking' } : c))
  assert.ok(buildTemplateState(template, renamed).state)
})
