import { cx } from '../lib/cx'
import { AlertIcon, CheckIcon, InfoIcon } from './icons'

const TONES = {
  info: { box: 'bg-primary-soft border-primary text-ink', icon: 'text-primary', Icon: InfoIcon },
  success: { box: 'bg-success-soft border-success text-ink', icon: 'text-success', Icon: CheckIcon },
  warning: { box: 'bg-warning-soft border-warning text-ink', icon: 'text-warning', Icon: AlertIcon },
  danger: { box: 'bg-danger-soft border-danger text-ink', icon: 'text-danger', Icon: AlertIcon },
}

// Inline message. danger/warning use role=alert, others role=status.
export default function Alert({ tone = 'info', title, className, children }) {
  const { box, icon, Icon } = TONES[tone]
  return (
    <div
      role={tone === 'danger' || tone === 'warning' ? 'alert' : 'status'}
      className={cx('flex min-w-0 items-start gap-3 rounded-card border-l-4 p-3 sm:p-4', box, className)}
    >
      <Icon className={cx('mt-0.5 shrink-0', icon)} />
      <div className="min-w-0 text-label">
        {title && <p className="font-semibold">{title}</p>}
        {children && <div className={title ? 'mt-1' : ''}>{children}</div>}
      </div>
    </div>
  )
}
