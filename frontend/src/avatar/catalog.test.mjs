import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'
import {
  allAvatars,
  avatarKey,
  avatarSpec,
  DEFAULT_AVATAR,
  GENDERS,
  isValidAvatar,
  normalizeAvatar,
  STYLES,
  UNIVERSES,
} from './catalog.js'

const golden = JSON.parse(readFileSync(new URL('../../../contracts/golden/avatar_catalog.json', import.meta.url), 'utf8'))

test('catalog ids match the frozen server catalog', () => {
  assert.deepEqual(UNIVERSES, golden.universes)
  assert.deepEqual(GENDERS, golden.genders)
  assert.deepEqual(STYLES, golden.styles)
})

test('allAvatars enumerates 5 x 2 x 4 valid distinct triples', () => {
  const all = allAvatars()
  assert.equal(all.length, 40)
  assert.equal(new Set(all.map(avatarKey)).size, 40)
  assert.ok(all.every(isValidAvatar))
})

test('same triple gives an identical spec (deterministic)', () => {
  for (const a of allAvatars()) {
    assert.equal(JSON.stringify(avatarSpec(a)), JSON.stringify(avatarSpec({ ...a })))
  }
})

test('every combination has a distinct visual spec', () => {
  const seen = new Set(allAvatars().map((a) => JSON.stringify({ ...avatarSpec(a), key: undefined })))
  assert.equal(seen.size, 40)
})

test('spec reflects the triple: universe motif, gender silhouette, style palette', () => {
  const motifs = new Set(UNIVERSES.map((u) => avatarSpec({ ...DEFAULT_AVATAR, universe: u }).motif))
  assert.equal(motifs.size, UNIVERSES.length)
  assert.notEqual(avatarSpec({ ...DEFAULT_AVATAR, gender: 'male' }).torso, avatarSpec({ ...DEFAULT_AVATAR, gender: 'female' }).torso)
  const bgs = new Set(STYLES.map((s) => avatarSpec({ ...DEFAULT_AVATAR, style: s }).palette.bg))
  assert.equal(bgs.size, STYLES.length)
  assert.equal(avatarSpec({ ...DEFAULT_AVATAR, style: 'tech' }).visor, true)
  assert.equal(avatarSpec({ ...DEFAULT_AVATAR, style: 'vibrant' }).visor, false)
})

test('all palette colours are valid hex', () => {
  for (const a of allAvatars()) {
    const spec = avatarSpec(a)
    for (const v of [...Object.values(spec.palette), ...Object.values(spec.body), spec.hairColor]) {
      assert.match(v, /^#[0-9A-Fa-f]{6}$/)
    }
  }
})

test('normalizeAvatar falls back to defaults for unknown parts', () => {
  assert.deepEqual(normalizeAvatar(null), DEFAULT_AVATAR)
  assert.deepEqual(normalizeAvatar({ universe: 'nope', gender: 'female', style: 'calm' }), {
    universe: DEFAULT_AVATAR.universe,
    gender: 'female',
    style: 'calm',
  })
  assert.equal(isValidAvatar({ universe: 'movies', gender: 'male' }), false)
})
