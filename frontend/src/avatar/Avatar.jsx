// Deterministic SVG avatar: a bust portrait generated from (universe, gender, style).
// No randomness and no ids, so the same triple always renders the same markup. Built from flat
// shapes with strokes >= 1 unit on a 64-unit canvas, so it stays readable from 32px to 160px+.
// The design spec (colours, shapes) comes from catalog.avatarSpec(); this file only draws it.
import { avatarSpec } from './catalog'

const W = 64

function Backdrop({ s }) {
  const p = s.palette
  switch (s.backdrop) {
    case 'burst':
      return (
        <>
          <rect width={W} height={W} fill={p.bg} />
          <path d="M0 44 L44 0 H64 V10 L10 64 H0Z" fill={p.bg2} opacity="0.55" />
          <circle cx="54" cy="12" r="2.2" fill={p.accent} />
          <circle cx="9" cy="30" r="1.8" fill={p.accent} />
          <circle cx="56" cy="40" r="1.6" fill={p.cloth} />
        </>
      )
    case 'panel':
      return (
        <>
          <rect width={W} height={W} fill={p.bg} />
          <circle cx="32" cy="34" r="25" fill={p.bg2} opacity="0.7" />
          <rect x="0" y="0" width="5" height={W} fill={p.bg2} opacity="0.6" />
        </>
      )
    case 'blobs':
      return (
        <>
          <rect width={W} height={W} fill={p.bg} />
          <circle cx="14" cy="16" r="15" fill={p.bg2} opacity="0.7" />
          <circle cx="54" cy="44" r="17" fill={p.bg2} opacity="0.55" />
          <circle cx="50" cy="10" r="5" fill={p.cloth2} opacity="0.8" />
        </>
      )
    default:
      return (
        <>
          <rect width={W} height={W} fill={p.bg} />
          <circle cx="32" cy="30" r="26" fill={p.bg2} opacity="0.8" />
          <g stroke={p.accent} strokeWidth="1" fill="none" strokeLinecap="round" strokeLinejoin="round" opacity="0.9">
            <path d="M0 14 H9 L13 10 H21" />
            <path d="M64 22 H55 L51 18 H44" />
            <path d="M0 50 H8 L12 54" />
            <path d="M64 8 H58" />
          </g>
          <g fill={p.accent}>
            <circle cx="21" cy="10" r="1.4" />
            <circle cx="44" cy="18" r="1.4" />
            <circle cx="12" cy="54" r="1.4" />
          </g>
        </>
      )
  }
}

function Star({ x, y, r, fill, opacity = 1 }) {
  const k = r * 0.28
  return (
    <path
      d={`M${x} ${y - r} L${x + k} ${y - k} L${x + r} ${y} L${x + k} ${y + k} L${x} ${y + r} L${x - k} ${y + k} L${x - r} ${y} L${x - k} ${y - k}Z`}
      fill={fill}
      opacity={opacity}
    />
  )
}

// Universe decor behind the figure.
function BackMotif({ s }) {
  const p = s.palette
  switch (s.motif) {
    case 'clapper':
      return (
        <g>
          <circle cx="13" cy="13" r="7" fill={p.ink} />
          <g fill={p.bg}>
            <circle cx="13" cy="9" r="1.6" />
            <circle cx="13" cy="17" r="1.6" />
            <circle cx="9" cy="13" r="1.6" />
            <circle cx="17" cy="13" r="1.6" />
          </g>
          <circle cx="13" cy="13" r="1.3" fill={p.bg} />
        </g>
      )
    case 'sparkles':
      return (
        <>
          <Star x={12} y={14} r={6} fill={p.accent} />
          <Star x={53} y={11} r={4} fill={p.cloth2} />
          <Star x={55} y={30} r={2.6} fill={p.accent} />
        </>
      )
    case 'paw':
      return (
        <g fill={p.accent} opacity="0.9">
          <ellipse cx="13" cy="15" rx="3.8" ry="3.2" />
          <circle cx="8" cy="10.6" r="1.7" />
          <circle cx="12" cy="8.4" r="1.7" />
          <circle cx="16.6" cy="9" r="1.7" />
        </g>
      )
    case 'ball':
      return (
        <g stroke={p.accent} strokeWidth="1.4" strokeLinecap="round" opacity="0.9">
          <path d="M5 12 H15" />
          <path d="M8 17 H18" />
          <path d="M4 22 H11" />
        </g>
      )
    default:
      return (
        <>
          <path d="M51 6 a7 7 0 1 0 6 10 a5.4 5.4 0 1 1 -6 -10Z" fill={p.metal} />
          <Star x={12} y={13} r={4.6} fill={p.metal} />
          <Star x={55} y={30} r={2.4} fill={p.cloth2} />
        </>
      )
  }
}

const TORSO = {
  broad: 'M5 64 C5 51 17 46.5 32 46.5 C47 46.5 59 51 59 64Z',
  narrow: 'M9 64 C9 52 20 47.5 32 47.5 C44 47.5 55 52 55 64Z',
}

// Hair or hood that sits behind the head and torso.
function BackHair({ s }) {
  const c = s.hairColor
  const p = s.palette
  switch (s.hairShape) {
    case 'wave':
      return <path d="M17.5 30 C14 12 26 7.5 32 7.5 C38 7.5 50 12 46.5 30 C48.5 38 50 46 45.5 52 C42 49 41 46 41 44 L23 44 C23 46 22 49 18.5 52 C14 46 15.5 38 17.5 30Z" fill={c} />
    case 'anime_long':
      return <path d="M17 30 C14 12 26 8 32 8 C38 8 50 12 47 30 C48 40 49 50 46 56 L18 56 C15 50 16 40 17 30Z" fill={c} />
    case 'long_crown':
      return <path d="M17 30 C14 12 26 8 32 8 C38 8 50 12 47 30 C48.5 41 50 52 46 60 L18 60 C14 52 15.5 41 17 30Z" fill={c} />
    case 'ponytail':
      return <path d="M42 22 C52 22 56 32 52 42 C50 46 47 46 45.5 44 C49 38 47 32 42 30Z" fill={c} />
    case 'hood':
      return (
        <>
          <path d="M12 64 L12 38 C12 15 22 6 32 6 C42 6 52 15 52 38 L52 64Z" fill={p.cloth} />
          <path d="M12 64 L12 38 C12 15 22 6 32 6 C42 6 52 15 52 38 L52 64Z" fill={p.bg2} opacity="0.25" />
        </>
      )
    default:
      return null
  }
}

function Torso({ s }) {
  const p = s.palette
  const d = TORSO[s.torso]
  if (s.clothing === 'jersey') {
    return (
      <g>
        <path d={d} fill={p.jersey} />
        <path d="M5.5 56 C6 52 9 50 12 49 L14.5 64 H5Z" fill={p.jersey2} opacity="0.9" />
        <path d="M58.5 56 C58 52 55 50 52 49 L49.5 64 H59Z" fill={p.jersey2} opacity="0.9" />
        <path d="M28 54.5 H36 L31 63" stroke={p.jersey2} strokeWidth="2" fill="none" strokeLinecap="round" strokeLinejoin="round" />
      </g>
    )
  }
  if (s.clothing === 'robe') {
    return (
      <g>
        <path d={d} fill={p.cloth} />
        <path d="M32 47 V64" stroke={p.cloth2} strokeWidth="1.2" opacity="0.7" />
        <path d="M22 50 L32 58 L42 50" stroke={p.metal} strokeWidth="1.4" fill="none" strokeLinecap="round" strokeLinejoin="round" />
      </g>
    )
  }
  return <path d={d} fill={p.cloth} />
}

function Neck({ s }) {
  return <path d="M28 36 V46.5 Q32 52 36 46.5 V36Z" fill={s.body.shade} />
}

// Collar / tie / piping depending on the style, drawn over torso and neck.
function ClothingDetail({ s }) {
  const p = s.palette
  if (s.clothing !== 'style') {
    if (s.clothing === 'robe') {
      return (
        <>
          <circle cx="32" cy="55" r="2.6" fill={p.metal} />
          <circle cx="31.2" cy="54.2" r="0.8" fill="#FFFFFF" opacity="0.9" />
        </>
      )
    }
    return <path d="M26.5 46.6 Q32 52.5 37.5 46.6" stroke={p.jersey2} strokeWidth="2" fill="none" strokeLinecap="round" />
  }
  switch (s.style) {
    case 'formal':
      return (
        <>
          <path d="M25.5 46.4 L32 59 L38.5 46.4 L36 46.2 L32 52 L28 46.2Z" fill={p.cloth2} />
          <path d="M28 46.4 Q32 52.2 36 46.4 L32 53Z" fill={s.body.shade} />
          {s.gender === 'male' ? (
            <>
              <path d="M30.4 52.6 H33.6 L34.2 55.2 H29.8Z" fill={p.accent} />
              <path d="M29.8 55.2 H34.2 L35.4 64 L32 66 L28.6 64Z" fill={p.accent} />
            </>
          ) : (
            <>
              <circle cx="32" cy="56.4" r="1.7" fill={p.accent} />
              <path d="M25.5 46.4 L22 52 L27.5 52Z M38.5 46.4 L42 52 L36.5 52Z" fill={p.cloth2} opacity="0.55" />
            </>
          )}
        </>
      )
    case 'calm':
      return <path d="M26 46.6 Q32 54 38 46.6" stroke={p.cloth2} strokeWidth="2.6" fill="none" strokeLinecap="round" />
    case 'tech':
      return (
        <>
          <path d="M26 46.4 L32 57.5 L38 46.4" stroke={p.accent} strokeWidth="1.4" fill="none" strokeLinecap="round" strokeLinejoin="round" />
          <circle cx="21" cy="58" r="1.3" fill={p.accent} />
          <path d="M43 56 H49 V60" stroke={p.accent} strokeWidth="1" fill="none" strokeLinecap="round" strokeLinejoin="round" />
        </>
      )
    default:
      return <path d="M12 58.5 H52" stroke={p.accent} strokeWidth="3" strokeLinecap="round" opacity="0.95" />
  }
}

function Ears({ s }) {
  const b = s.body
  const p = s.palette
  if (s.ears === 'animal') {
    if (s.gender === 'male') {
      return (
        <>
          <path d="M18.5 25 L19.5 6 L32 16Z" fill={b.skin} />
          <path d="M45.5 25 L44.5 6 L32 16Z" fill={b.skin} />
          <path d="M21.5 20 L21.8 11.5 L27.5 16Z" fill={b.inner} />
          <path d="M42.5 20 L42.2 11.5 L36.5 16Z" fill={b.inner} />
        </>
      )
    }
    return (
      <>
        <path d="M18 26 C16.5 16 18.5 9 21 8 C24 9 29 13 31 17Z" fill={b.skin} />
        <path d="M46 26 C47.5 16 45.5 9 43 8 C40 9 35 13 33 17Z" fill={b.skin} />
        <path d="M20 21 C19.5 15 20.5 12 21.4 11.4 C23 12.4 26 15 27 17.5Z" fill={b.inner} />
        <path d="M44 21 C44.5 15 43.5 12 42.6 11.4 C41 12.4 38 15 37 17.5Z" fill={b.inner} />
      </>
    )
  }
  if (s.ears === 'elf') {
    return (
      <>
        <path d="M20.6 32 L12.5 22.5 L21 27Z" fill={b.skin} />
        <path d="M43.4 32 L51.5 22.5 L43 27Z" fill={b.skin} />
      </>
    )
  }
  return (
    <>
      <circle cx="20.4" cy="30.5" r="2.7" fill={b.shade} />
      <circle cx="43.6" cy="30.5" r="2.7" fill={b.shade} />
      {s.style === 'tech' && <circle cx="20.4" cy="30.5" r="1.1" fill={p.accent} />}
    </>
  )
}

function Head({ s }) {
  const male = s.gender === 'male'
  return (
    <path
      d={
        male
          ? 'M20 28 C20 17 26 14.5 32 14.5 C38 14.5 44 17 44 28 C44 37 39 43.5 32 43.5 C25 43.5 20 37 20 28Z'
          : 'M20.6 28 C20.6 17 26 14.5 32 14.5 C38 14.5 43.4 17 43.4 28 C43.4 37 38.6 43.5 32 43.5 C25.4 43.5 20.6 37 20.6 28Z'
      }
      fill={s.body.skin}
    />
  )
}

function Mouth({ s }) {
  const ink = s.palette.ink
  if (s.universe === 'anime') {
    return <path d="M29.6 37 Q32 39.4 34.4 37" stroke={ink} strokeWidth="1.1" fill="none" strokeLinecap="round" />
  }
  switch (s.mouth) {
    case 'grin':
      return <path d="M27.8 36.2 Q32 41.8 36.2 36.2 Q32 37.8 27.8 36.2Z" fill="#8E1F2E" stroke={ink} strokeWidth="0.9" strokeLinejoin="round" />
    case 'slight':
      return <path d="M29.2 37.4 Q32 38.9 34.8 37.4" stroke={ink} strokeWidth="1.2" fill="none" strokeLinecap="round" />
    case 'soft':
      return <path d="M29 36.8 Q32 39.8 35 36.8" stroke={ink} strokeWidth="1.2" fill="none" strokeLinecap="round" />
    default:
      return <path d="M29.4 37.6 H34.6" stroke={ink} strokeWidth="1.3" fill="none" strokeLinecap="round" />
  }
}

function Face({ s }) {
  const p = s.palette
  const ink = p.ink
  const animal = s.universe === 'animals'
  const eyeY = animal ? 29.5 : 30
  return (
    <g>
      {animal && (
        <>
          <path d="M20.2 31 Q27 31.5 32 40.5 Q37 31.5 43.8 31 Q44 41 32 44 Q20 41 20.2 31Z" fill={s.body.light} />
          <ellipse cx="32" cy="36" rx="2" ry="1.4" fill={ink} />
          <path d="M32 37.2 V38.8 M29.6 39.3 Q32 41.4 34.4 39.3" stroke={ink} strokeWidth="1" fill="none" strokeLinecap="round" />
          <g stroke={ink} strokeWidth="0.7" strokeLinecap="round" opacity="0.7">
            <path d="M21 36 L14 34.6 M21 38.4 L14.5 39.4" />
            <path d="M43 36 L50 34.6 M43 38.4 L49.5 39.4" />
          </g>
        </>
      )}
      {s.eyes === 'anime' ? (
        <>
          <ellipse cx="27" cy="30.4" rx="2.9" ry="3.7" fill={ink} />
          <ellipse cx="37" cy="30.4" rx="2.9" ry="3.7" fill={ink} />
          <ellipse cx="27" cy="31" rx="2.2" ry="3" fill={p.eye} />
          <ellipse cx="37" cy="31" rx="2.2" ry="3" fill={p.eye} />
          <circle cx="26.1" cy="29.2" r="1.15" fill="#FFFFFF" />
          <circle cx="36.1" cy="29.2" r="1.15" fill="#FFFFFF" />
          <circle cx="28" cy="32.4" r="0.5" fill="#FFFFFF" />
          <circle cx="38" cy="32.4" r="0.5" fill="#FFFFFF" />
        </>
      ) : (
        <>
          <ellipse cx="27.4" cy={eyeY} rx="1.5" ry="1.8" fill={ink} />
          <ellipse cx="36.6" cy={eyeY} rx="1.5" ry="1.8" fill={ink} />
          <circle cx="27.9" cy={eyeY - 0.6} r="0.5" fill="#FFFFFF" />
          <circle cx="37.1" cy={eyeY - 0.6} r="0.5" fill="#FFFFFF" />
        </>
      )}
      {!animal && s.universe !== 'anime' && s.hairShape !== 'hood' && (
        <g stroke={s.hairColor} strokeWidth={s.style === 'formal' ? 1.5 : 1.1} strokeLinecap="round" fill="none">
          <path d="M24.6 26 Q27.2 24.6 29.8 25.8" />
          <path d="M34.2 25.8 Q36.8 24.6 39.4 26" />
        </g>
      )}
      {s.blush && (
        <g fill={p.blush} opacity="0.7">
          <ellipse cx="24.2" cy="35.2" rx="2.4" ry="1.4" />
          <ellipse cx="39.8" cy="35.2" rx="2.4" ry="1.4" />
        </g>
      )}
      {!animal && <Mouth s={s} />}
      {!animal && s.universe !== 'anime' && (
        <path d="M32 31.5 Q31 34.4 32.6 34.9" stroke={s.body.shade} strokeWidth="1" fill="none" strokeLinecap="round" />
      )}
    </g>
  )
}

// Hair in front of the face (fringe, cap), hood rim, crown.
function FrontHair({ s }) {
  const c = s.hairColor
  const p = s.palette
  switch (s.hairShape) {
    case 'swept':
      return <path d="M19.8 29 C18.6 15 26 11 32 11 C39 11 45.6 15.5 44.2 29 C43 23 39.5 19.6 33 19.4 C27 19.2 22 22 19.8 29Z" fill={c} />
    case 'short':
      return <path d="M19.8 28 C19 16 25.5 12 32 12 C38.5 12 45 16 44.2 28 C42.6 22.4 38.6 20.4 32 20.4 C25.4 20.4 21.4 22.4 19.8 28Z" fill={c} />
    case 'wave':
      return <path d="M20 29 C18 15 25 11 32 11 C39 11 46 15 44 29 C43 24 40 19 34 17.6 C30 21 25 22.6 20 29Z" fill={c} />
    case 'ponytail':
      return <path d="M20.2 27.5 C19.4 16 25.5 12.5 32 12.5 C38.5 12.5 44.6 16 43.8 27.5 C42 22.2 38 20.2 32 20.2 C26 20.2 22 22.2 20.2 27.5Z" fill={c} />
    case 'spiky':
      return (
        <path
          d="M19.4 29 C17.6 21 19.4 15 23.4 12.6 L22.8 6.8 L28.4 11.2 L32 4.6 L35.6 11.2 L41.2 6.8 L40.6 12.6 C44.6 15 46.4 21 44.6 29 C43 24 40.4 20.4 38 19 L34 22.6 L30 19 C26.4 20.4 22 24 19.4 29Z"
          fill={c}
        />
      )
    case 'anime_long':
      return (
        <>
          <path d="M19.6 30 C17.6 16 25.4 10.6 32 10.6 C38.6 10.6 46.4 16 44.4 30 C43.6 24 41 20.4 37 19 C35 22 30 22 27.4 19 C23.4 20.4 20.6 24 19.6 30Z" fill={c} />
          <path d="M19.6 30 C17.4 38 17.6 48 20.4 54 L23.8 54 C22 47 22.6 38 23.6 31Z" fill={c} />
          <path d="M44.4 30 C46.6 38 46.4 48 43.6 54 L40.2 54 C42 47 41.4 38 40.4 31Z" fill={c} />
          <circle cx="41.4" cy="17" r="2.1" fill={s.style === 'tech' ? p.cloth2 : p.accent} />
          <circle cx="41.4" cy="17" r="0.8" fill="#FFFFFF" />
        </>
      )
    case 'long_crown':
      return (
        <>
          <path d="M19.8 29.5 C18.6 17 25.6 12.6 32 12.6 C38.4 12.6 45.4 17 44.2 29.5 C42.6 24 38.6 19.6 32.6 19.4 C26 19.2 21.8 23.6 19.8 29.5Z" fill={c} />
          <path d="M22.2 21 L23.8 14.4 L27.6 18.6 L32 12 L36.4 18.6 L40.2 14.4 L41.8 21Z" fill={p.metal} stroke={p.ink} strokeWidth="0.5" strokeLinejoin="round" />
          <circle cx="32" cy="17.6" r="1.1" fill="#FFFFFF" opacity="0.9" />
        </>
      )
    case 'hood':
      return (
        <>
          <path d="M17.6 42 C13.6 24 21.6 8.6 32 8.6 C42.4 8.6 50.4 24 46.4 42 C47.6 26 41.4 15.4 32 15.4 C22.6 15.4 16.4 26 17.6 42Z" fill={p.cloth} />
          <path d="M17.6 42 C13.6 24 21.6 8.6 32 8.6 C42.4 8.6 50.4 24 46.4 42 C47.6 26 41.4 15.4 32 15.4 C22.6 15.4 16.4 26 17.6 42Z" fill={p.bg2} opacity="0.25" />
        </>
      )
    default:
      return null
  }
}

function SportsHeadband({ s }) {
  const p = s.palette
  return (
    <path
      d="M20 24.4 Q32 20.2 44 24.4 L44 27.2 Q32 23 20 27.2Z"
      fill={s.style === 'tech' ? p.accent : p.jersey}
      stroke={p.jersey2}
      strokeWidth="0.6"
    />
  )
}

function Visor({ s }) {
  const p = s.palette
  return (
    <g>
      <rect x="21" y="26.2" width="22" height="8" rx="3.4" fill={p.accent} fillOpacity="0.22" stroke={p.accent} strokeWidth="1" />
      <path d="M23 28.4 H27" stroke="#FFFFFF" strokeWidth="0.8" strokeLinecap="round" opacity="0.7" />
    </g>
  )
}

// Foreground universe motif (held item).
function FrontMotif({ s }) {
  const p = s.palette
  if (s.motif === 'clapper') {
    return (
      <g>
        <rect x="37.5" y="45.5" width="16" height="9.2" rx="1" fill={p.ink} />
        <path d="M40 48.4 H51 M40 51.6 H48" stroke="#FFFFFF" strokeWidth="1" strokeLinecap="round" opacity="0.85" />
        <g transform="rotate(-14 37.5 45.5)">
          <rect x="37.5" y="41.6" width="16" height="3.8" rx="0.8" fill={p.ink} />
          <path d="M41 41.6 L43.4 45.4 H45.4 L43 41.6Z M46.4 41.6 L48.8 45.4 H50.8 L48.4 41.6Z" fill="#FFFFFF" />
        </g>
      </g>
    )
  }
  if (s.motif === 'ball') {
    return (
      <g>
        <circle cx="17.5" cy="49.5" r="6" fill="#FFFFFF" stroke={p.ink} strokeWidth="0.9" />
        <path d="M17.5 46.6 L20.2 48.6 L19.2 51.8 H15.8 L14.8 48.6Z" fill={p.ink} />
        <path d="M17.5 46.6 V44 M20.2 48.6 L22.8 47.6 M19.2 51.8 L21 54 M15.8 51.8 L14 54 M14.8 48.6 L12.2 47.6" stroke={p.ink} strokeWidth="0.8" strokeLinecap="round" />
      </g>
    )
  }
  return null
}

// A circular 64-unit portrait. `size` is px (or any CSS length). Decorative unless `label` is set.
// Pass either `avatar={{universe, gender, style}}` or the three props.
export default function Avatar({ universe, gender, style, avatar, size = 48, label, className, rounded = true }) {
  const s = avatarSpec(avatar || { universe, gender, style })
  const dim = typeof size === 'number' ? `${size}px` : size
  return (
    <svg
      viewBox={`0 0 ${W} ${W}`}
      width={dim}
      height={dim}
      className={className}
      style={{ display: 'block', flex: 'none', maxWidth: '100%', borderRadius: rounded ? '50%' : undefined }}
      role={label ? 'img' : undefined}
      aria-label={label || undefined}
      aria-hidden={label ? undefined : true}
      focusable="false"
      data-avatar={s.key}
    >
      <Backdrop s={s} />
      <BackMotif s={s} />
      <BackHair s={s} />
      <Torso s={s} />
      <Neck s={s} />
      <ClothingDetail s={s} />
      <Ears s={s} />
      <Head s={s} />
      <Face s={s} />
      <FrontHair s={s} />
      {s.universe === 'sports' && <SportsHeadband s={s} />}
      {s.visor && <Visor s={s} />}
      <FrontMotif s={s} />
    </svg>
  )
}
