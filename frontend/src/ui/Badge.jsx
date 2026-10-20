import { cx } from '../lib/cx'

const TONES = {
  neutral: 'bg-surface-2 text-ink border-line',
  info: 'bg-primary-soft text-primary border-transparent',
  success: 'bg-success-soft text-success border-transparent',
  warning: 'bg-warning-soft text-warning border-transparent',
  danger: 'bg-danger-soft text-danger border-transparent',
}

// Status chip: the label always carries the meaning (colour is never the only signal).
export default function Badge({ tone = 'neutral', className, children }) {
  return (
    <span
      className={cx(
        'inline-flex max-w-full items-center rounded-ctl border px-2 py-0.5 text-caption font-medium break-words',
        TONES[tone],
        className,
      )}
    >
      {children}
    </span>
  )
}
