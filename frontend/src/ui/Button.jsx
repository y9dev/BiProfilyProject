import { Link } from 'react-router-dom'
import { cx } from '../lib/cx'
import Spinner from './Spinner'

const VARIANTS = {
  primary: 'bg-primary text-primary-ink hover:bg-primary-hover border border-transparent',
  secondary: 'bg-surface text-ink border border-line-strong hover:bg-surface-2',
  ghost: 'bg-transparent text-primary border border-transparent hover:bg-primary-soft',
  danger: 'bg-danger text-danger-ink hover:bg-danger-hover border border-transparent',
  'danger-outline': 'bg-surface text-danger border border-danger hover:bg-danger-soft',
}

// Min 44x44 target, label wraps (long ru/kk text), never a fixed width.
// States: default, hover, focus-visible (global ring), disabled, loading (aria-busy, disabled).
// Pass `to` to render a router link styled as a button.
export default function Button({
  variant = 'primary',
  loading = false,
  disabled = false,
  fullWidth = false,
  to,
  type = 'button',
  className,
  children,
  ...rest
}) {
  const classes = cx(
    'inline-flex min-h-touch min-w-touch max-w-full items-center justify-center gap-2 rounded-ctl px-4 py-2',
    'text-center text-label font-semibold break-words transition-colors',
    'aria-disabled:cursor-not-allowed aria-disabled:opacity-50 disabled:cursor-not-allowed disabled:opacity-50',
    VARIANTS[variant],
    fullWidth && 'w-full',
    className,
  )
  const inactive = disabled || loading

  if (to && !inactive) {
    return (
      <Link to={to} className={classes} {...rest}>
        {children}
      </Link>
    )
  }

  return (
    <button
      type={type}
      className={classes}
      disabled={inactive}
      aria-busy={loading || undefined}
      {...rest}
    >
      {loading && <Spinner size="sm" className="text-current" label="" />}
      <span className="min-w-0">{children}</span>
    </button>
  )
}
