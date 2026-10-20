// Three radio groups (universe, gender, style) whose tiles preview the avatar with the other
// two current choices applied. Controlled: `value` is the {universe, gender, style} triple.
import { useTranslation } from '../i18n'
import Avatar from './Avatar'
import { GENDERS, STYLES, UNIVERSES } from './catalog'

const GROUPS = [
  { field: 'universe', all: UNIVERSES, cols: 'grid-cols-3 sm:grid-cols-5' },
  { field: 'gender', all: GENDERS, cols: 'grid-cols-2' },
  { field: 'style', all: STYLES, cols: 'grid-cols-2 sm:grid-cols-4' },
]

const FIELD_KEYS = { universe: 'universes', gender: 'genders', style: 'styles' }

// Keeps the server's order but only ids the generator can draw; the full list when no catalog yet.
function options(catalog, group) {
  const fromServer = catalog?.[FIELD_KEYS[group.field]]
  if (!Array.isArray(fromServer)) return group.all
  const known = fromServer.filter((id) => group.all.includes(id))
  return known.length ? known : group.all
}

function Tile({ group, id, checked, name, preview, onChange }) {
  const { t } = useTranslation()
  const label = t(`avatar.${group.field}.${id}`)
  return (
    <label className="relative block min-w-0 cursor-pointer">
      <input
        type="radio"
        name={name}
        value={id}
        checked={checked}
        onChange={() => onChange(id)}
        className="peer sr-only"
      />
      <span
        className={[
          'flex h-full min-h-touch flex-col items-center gap-1.5 rounded-card border-2 p-2 text-center transition-colors',
          'border-line bg-surface hover:bg-surface-2',
          'peer-checked:border-primary peer-checked:bg-primary-soft',
          'peer-focus-visible:outline peer-focus-visible:outline-2 peer-focus-visible:outline-offset-2 peer-focus-visible:outline-ring',
        ].join(' ')}
      >
        <Avatar avatar={preview} size={56} />
        <span className="min-w-0 text-caption font-medium text-ink [overflow-wrap:anywhere]">{label}</span>
      </span>
    </label>
  )
}

export default function AvatarPicker({ value, onChange, catalog, idPrefix = 'avatar' }) {
  const { t } = useTranslation()
  return (
    <div className="space-y-6">
      {GROUPS.map((group) => (
        <fieldset key={group.field} className="min-w-0">
          <legend className="mb-2 text-label font-semibold text-ink">{t(`avatar.group.${group.field}`)}</legend>
          <div className={`grid gap-2 ${group.cols}`}>
            {options(catalog, group).map((id) => (
              <Tile
                key={id}
                group={group}
                id={id}
                name={`${idPrefix}-${group.field}`}
                checked={value[group.field] === id}
                preview={{ ...value, [group.field]: id }}
                onChange={(next) => onChange({ ...value, [group.field]: next })}
              />
            ))}
          </div>
        </fieldset>
      ))}
    </div>
  )
}
