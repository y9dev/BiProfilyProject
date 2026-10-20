import { cx } from '../lib/cx'

// Flat surface with a hairline border. `padding="none"` for tables/lists that bleed to the edge.
export default function Card({ as: Tag = 'section', padding = 'md', className, children, ...rest }) {
  return (
    <Tag
      className={cx(
        'card-in min-w-0 rounded-card border border-line bg-surface transition-colors',
        padding === 'md' && 'p-4 sm:p-6',
        className,
      )}
      {...rest}
    >
      {children}
    </Tag>
  )
}

export function CardTitle({ as: Tag = 'h2', className, children, ...rest }) {
  return (
    <Tag className={cx('text-title font-semibold text-ink', className)} {...rest}>
      {children}
    </Tag>
  )
}
