import { cx } from '../lib/cx'

// Loading placeholder block. Size it with className (h-*, w-*); decorative for assistive tech.
export default function Skeleton({ className }) {
  return <div aria-hidden="true" className={cx('pulse rounded-ctl bg-surface-2', className || 'h-4 w-full')} />
}
