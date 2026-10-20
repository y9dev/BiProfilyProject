// Avatar catalog + deterministic design spec.
//
// An avatar is the triple (universe, gender, style), stored by id on the server
// (backend/internal/avatar). The picture is generated from the triple only: no randomness,
// no external assets. avatarSpec() is a pure function that returns plain data (palette, hair
// shape, motif, ...) which Avatar.jsx turns into SVG, so the same triple always gives the
// same output and the 40 combinations are covered by catalog.test.mjs.

export const UNIVERSES = ['movies', 'anime', 'animals', 'sports', 'fantasy']
export const GENDERS = ['male', 'female']
export const STYLES = ['vibrant', 'formal', 'calm', 'tech']

export const DEFAULT_AVATAR = Object.freeze({ universe: 'movies', gender: 'male', style: 'vibrant' })

const pick = (list, v, fallback) => (list.includes(v) ? v : fallback)

export function isValidAvatar(a) {
  return !!a && UNIVERSES.includes(a.universe) && GENDERS.includes(a.gender) && STYLES.includes(a.style)
}

// Always returns a valid triple (unknown / missing parts fall back to the defaults).
export function normalizeAvatar(a) {
  return {
    universe: pick(UNIVERSES, a?.universe, DEFAULT_AVATAR.universe),
    gender: pick(GENDERS, a?.gender, DEFAULT_AVATAR.gender),
    style: pick(STYLES, a?.style, DEFAULT_AVATAR.style),
  }
}

export const avatarKey = (a) => {
  const n = normalizeAvatar(a)
  return `${n.universe}.${n.gender}.${n.style}`
}

export function allAvatars() {
  const out = []
  for (const universe of UNIVERSES)
    for (const gender of GENDERS) for (const style of STYLES) out.push({ universe, gender, style })
  return out
}

// Style = colour mood. bg/bg2 backdrop, cloth/cloth2 clothing, accent = signature colour,
// metal = crown/clasp, hair + hairAlt (anime / fantasy hair), eye = anime iris, ink = line colour.
const PALETTES = {
  vibrant: {
    bg: '#FFC21A', bg2: '#FF7A1A', cloth: '#2F6BFF', cloth2: '#FFFFFF', accent: '#FF3D71',
    metal: '#FFD23F', hair: '#2A1A14', hairAlt: '#FF3D71', eye: '#1E88FF', ink: '#1B1230',
    jersey: '#E8332D', jersey2: '#FFFFFF', blush: '#FF8FA8',
  },
  formal: {
    bg: '#CBD3DD', bg2: '#B3BECB', cloth: '#28323F', cloth2: '#FFFFFF', accent: '#8E2433',
    metal: '#C9A23E', hair: '#2B2623', hairAlt: '#6B7685', eye: '#3B4A5C', ink: '#1B1F26',
    jersey: '#1F3A8A', jersey2: '#FFFFFF', blush: '#E7A9A0',
  },
  calm: {
    bg: '#D8EBE2', bg2: '#BFD9E6', cloth: '#9DBBD0', cloth2: '#F4EFE6', accent: '#E8A5A0',
    metal: '#E9C27A', hair: '#8B6F5C', hairAlt: '#C9B8E8', eye: '#6C9BCB', ink: '#4A4350',
    jersey: '#F2A7A0', jersey2: '#FFFFFF', blush: '#F2B3AE',
  },
  tech: {
    bg: '#0C1626', bg2: '#17304D', cloth: '#1B2F4A', cloth2: '#2D4B73', accent: '#35F0DC',
    metal: '#35F0DC', hair: '#5A7092', hairAlt: '#35F0DC', eye: '#35F0DC', ink: '#050B14',
    jersey: '#1B2F4A', jersey2: '#35F0DC', blush: '#3A6F8F',
  },
}

const SKIN = {
  male: { skin: '#EDB98F', shade: '#D99C74' },
  female: { skin: '#F4CBA8', shade: '#E1AB86' },
}
const ELF_SKIN = {
  male: { skin: '#F3D5BC', shade: '#DDB698' },
  female: { skin: '#F8E0CC', shade: '#E4C0A4' },
}
// Animals: fox (male) and cat (female).
const FUR = {
  male: { skin: '#E07A2D', shade: '#B85C1B', light: '#FFF4E6', inner: '#3A2418' },
  female: { skin: '#EFD2AC', shade: '#D2AE82', light: '#FFFDF8', inner: '#F3A6B0' },
}

const HAIR_SHAPE = {
  movies: { male: 'swept', female: 'wave' },
  anime: { male: 'spiky', female: 'anime_long' },
  animals: { male: 'none', female: 'none' },
  sports: { male: 'short', female: 'ponytail' },
  fantasy: { male: 'hood', female: 'long_crown' },
}

const CLOTHING = {
  movies: 'style',
  anime: 'style',
  animals: 'style',
  sports: 'jersey',
  fantasy: 'robe',
}

const MOTIF = {
  movies: 'clapper',
  anime: 'sparkles',
  animals: 'paw',
  sports: 'ball',
  fantasy: 'stars',
}

const EARS = { movies: 'human', anime: 'human', animals: 'animal', sports: 'human', fantasy: 'elf' }
const EYES = { movies: 'dots', anime: 'anime', animals: 'dots', sports: 'dots', fantasy: 'dots' }
const MOUTH = { vibrant: 'grin', formal: 'slight', calm: 'soft', tech: 'flat' }
const BACKDROP = { vibrant: 'burst', formal: 'panel', calm: 'blobs', tech: 'circuit' }

export function avatarSpec(input) {
  const { universe, gender, style } = normalizeAvatar(input)
  const palette = PALETTES[style]
  const animal = universe === 'animals'
  const fantasy = universe === 'fantasy'
  const body = animal ? FUR[gender] : fantasy ? ELF_SKIN[gender] : SKIN[gender]
  const altHair = universe === 'anime' || (fantasy && gender === 'female')
  return {
    key: `${universe}.${gender}.${style}`,
    universe,
    gender,
    style,
    palette,
    body,
    hairShape: HAIR_SHAPE[universe][gender],
    hairColor: altHair ? palette.hairAlt : palette.hair,
    clothing: CLOTHING[universe],
    motif: MOTIF[universe],
    ears: EARS[universe],
    eyes: EYES[universe],
    mouth: MOUTH[style],
    backdrop: BACKDROP[style],
    visor: style === 'tech',
    blush: style === 'calm' || universe === 'anime',
    // Torso silhouette differs by gender (broader / narrower shoulders).
    torso: gender === 'male' ? 'broad' : 'narrow',
  }
}
