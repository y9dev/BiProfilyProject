// Small wizard-local building blocks: icons, radio cards, icon buttons, error helpers.
import { useId } from 'react'
import { cx } from '../../../lib/cx'

const svg = {
  width: 20,
  height: 20,
  viewBox: '0 0 24 24',
  fill: 'none',
  stroke: 'currentColor',
  strokeWidth: 2,
  strokeLinecap: 'round',
  strokeLinejoin: 'round',
  'aria-hidden': true,
  focusable: 'false',
}

export const PlusIcon = (p) => (
  <svg {...svg} {...p}>
    <path d="M12 5v14M5 12h14" />
  </svg>
)
export const TrashIcon = (p) => (
  <svg {...svg} {...p}>
    <path d="M4 7h16M10 11v6M14 11v6M6 7l1 12a2 2 0 0 0 2 2h6a2 2 0 0 0 2-2l1-12M9 7V4h6v3" />
  </svg>
)
export const ArrowUpIcon = (p) => (
  <svg {...svg} {...p}>
    <path d="M12 19V5M6 11l6-6 6 6" />
  </svg>
)
export const ArrowDownIcon = (p) => (
  <svg {...svg} {...p}>
    <path d="M12 5v14M6 13l6 6 6-6" />
  </svg>
)
export const ChevronIcon = ({ open, ...p }) => (
  <svg {...svg} {...p} className={cx('transition-transform', open && 'rotate-180', p.className)}>
    <path d="M6 9l6 6 6-6" />
  </svg>
)
export const CheckMark = (p) => (
  <svg {...svg} {...p}>
    <path d="M5 12.5l4.5 4.5L19 7.5" />
  </svg>
)

// 44px icon-only button with an accessible name.
export function IconButton({ label, tone = 'neutral', className, children, ...rest }) {
  return (
    <button
      type="button"
      aria-label={label}
      title={label}
      className={cx(
        'inline-flex min-h-touch min-w-touch shrink-0 items-center justify-center rounded-ctl border border-transparent transition-colors',
        'disabled:cursor-not-allowed disabled:opacity-40',
        tone === 'danger' ? 'text-danger hover:bg-danger-soft' : 'text-muted hover:bg-surface-2 hover:text-ink',
        className,
      )}
      {...rest}
    >
      {children}
    </button>
  )
}

// Group of native radios rendered as selectable cards. options: [{value,label,hint}]
export function RadioCards({ legend, value, onChange, options, columns = 'sm:grid-cols-2', error }) {
  const name = useId()
  return (
    <fieldset className="min-w-0" aria-invalid={error ? true : undefined}>
      <legend className="mb-1 block text-label font-medium text-ink">{legend}</legend>
      <div className={cx('grid gap-2', columns)}>
        {options.map((o) => {
          const checked = o.value === value
          return (
            <label
              key={String(o.value)}
              className={cx(
                'flex min-h-touch min-w-0 cursor-pointer items-start gap-3 rounded-card border p-3 transition-colors',
                checked ? 'border-primary bg-primary-soft' : 'border-line bg-surface hover:bg-surface-2',
                'has-[:focus-visible]:outline has-[:focus-visible]:outline-2 has-[:focus-visible]:outline-offset-2 has-[:focus-visible]:outline-ring',
              )}
            >
              <input
                type="radio"
                name={name}
                checked={checked}
                onChange={() => onChange(o.value)}
                className="mt-0.5 size-5 shrink-0 accent-[var(--primary)]"
              />
              <span className="min-w-0">
                <span className="block text-label font-semibold text-ink">{o.label}</span>
                {o.hint && <span className="mt-0.5 block text-caption text-muted">{o.hint}</span>}
              </span>
            </label>
          )
        })}
      </div>
      {error && <p className="mt-1 text-caption font-medium text-danger">{error}</p>}
    </fieldset>
  )
}
