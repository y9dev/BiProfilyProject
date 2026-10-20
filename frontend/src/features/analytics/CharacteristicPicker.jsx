// Characteristic selection shared by every tab: selected chips (line sample shows solid core vs dashed
// additional), an expandable checklist with coverage n, a 10-item cap and "restore default".
import { useId, useState } from 'react'
import { useTranslation } from '../../i18n'
import { cx } from '../../lib/cx'
import Button from '../../ui/Button'
import { ChevronDownIcon, CloseIcon } from '../../ui/icons'
import { MAX_CHARACTERISTICS, charName, isDefaultSelection } from './analyticsState'

export function LineSwatch({ color, dashed }) {
  return (
    <svg width="22" height="8" viewBox="0 0 22 8" aria-hidden="true" className="shrink-0">
      <line
        x1="1"
        y1="4"
        x2="21"
        y2="4"
        stroke={color}
        strokeWidth="3"
        strokeLinecap="round"
        strokeDasharray={dashed ? '4 4' : undefined}
      />
    </svg>
  )
}

export default function CharacteristicPicker({ catalog, selection, urlSelection, defaultIds, styles, onToggle, onRestore, disabled }) {
  const { t, lang } = useTranslation()
  const [open, setOpen] = useState(false)
  const listId = useId()
  const byId = new Map((catalog || []).map((c) => [c.id, c]))
  const atCap = selection.length >= MAX_CHARACTERISTICS
  const isDefault = isDefaultSelection(urlSelection, defaultIds)
  const groups = [
    { key: 'core', title: t('stats.core'), items: (catalog || []).filter((c) => c.core) },
    { key: 'extra', title: t('stats.extra'), items: (catalog || []).filter((c) => !c.core) },
  ].filter((g) => g.items.length)

  return (
    <div className="min-w-0">
      <div className="mb-1 flex flex-wrap items-baseline justify-between gap-2">
        <p className="text-label font-medium text-ink">{t('analytics.picker.label')}</p>
        <span className="text-caption tabular-nums text-muted">
          {t('analytics.picker.count', { n: selection.length, max: MAX_CHARACTERISTICS })}
        </span>
      </div>

      {catalog && catalog.length === 0 && !disabled && <p className="text-label text-muted">{t('stats.no_data_period')}</p>}
      <ul className="flex flex-wrap gap-2" aria-label={t('analytics.picker.selected')}>
        {selection.map((id) => {
          const c = byId.get(id)
          if (!c) return null
          const st = styles.get(id) || {}
          return (
            <li
              key={id}
              className="inline-flex min-h-touch max-w-full items-center gap-2 rounded-ctl border border-line bg-surface py-1 pl-3 pr-1 text-label text-ink"
            >
              <LineSwatch color={st.color} dashed={st.dashed} />
              <span className="min-w-0 break-words">{charName(c, lang)}</span>
              <button
                type="button"
                disabled={disabled || selection.length <= 1}
                onClick={() => onToggle(id)}
                aria-label={t('analytics.picker.remove', { name: charName(c, lang) })}
                className="flex h-9 w-9 shrink-0 items-center justify-center rounded-ctl text-muted hover:bg-surface-2 hover:text-ink disabled:cursor-not-allowed disabled:opacity-40"
              >
                <CloseIcon width={16} height={16} />
              </button>
            </li>
          )
        })}
      </ul>

      <div className="mt-3 flex flex-wrap gap-2">
        <Button variant="secondary" aria-expanded={open} aria-controls={listId} disabled={disabled} onClick={() => setOpen((v) => !v)}>
          {t(open ? 'analytics.picker.close' : 'analytics.picker.edit')}
          <ChevronDownIcon className={cx('transition-transform', open && 'rotate-180')} />
        </Button>
        <Button variant="ghost" disabled={disabled || isDefault} onClick={onRestore}>
          {t('analytics.picker.restore')}
        </Button>
      </div>

      {open && (
        <div id={listId} className="mt-3 rounded-card border border-line bg-surface-2 p-3 sm:p-4">
          {atCap && (
            <p role="status" className="mb-3 text-caption font-medium text-warning">
              {t('stats.validation.too_many')}
            </p>
          )}
          <div className="grid gap-4 md:grid-cols-2">
            {groups.map((g) => (
              <fieldset key={g.key} className="min-w-0">
                <legend className="mb-2 text-caption font-semibold uppercase tracking-wide text-muted">{g.title}</legend>
                <ul className="space-y-1">
                  {g.items.map((c) => {
                    const checked = selection.includes(c.id)
                    const blocked = (!checked && atCap) || (checked && selection.length <= 1)
                    return (
                      <li key={c.id}>
                        <label
                          className={cx(
                            'flex min-h-touch cursor-pointer items-center gap-3 rounded-ctl px-2 py-1 hover:bg-surface',
                            blocked && 'cursor-not-allowed opacity-60',
                          )}
                        >
                          <input
                            type="checkbox"
                            className="h-5 w-5 shrink-0 accent-[var(--primary)]"
                            checked={checked}
                            disabled={disabled || blocked}
                            onChange={() => onToggle(c.id)}
                          />
                          <span className="min-w-0 flex-1">
                            <span className="block break-words text-label text-ink">{charName(c, lang)}</span>
                            <span className="block text-caption text-muted">
                              {c.eligible
                                ? t('analytics.picker.coverage', { employees: c.employees, attempts: c.attempts })
                                : t('stats.insufficient', { n: c.employees })}
                            </span>
                          </span>
                        </label>
                      </li>
                    )
                  })}
                </ul>
              </fieldset>
            ))}
          </div>
          <p className="mt-3 text-caption text-muted">{t('analytics.picker.default_hint')}</p>
        </div>
      )}
    </div>
  )
}
