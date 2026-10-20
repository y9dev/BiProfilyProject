// Empty list/section: icon, title, short explanation and an optional action.
import { useTranslation } from '../i18n'
import { InboxIcon } from './icons'

export default function EmptyState({ title, description, action, icon, className = '' }) {
  const { t } = useTranslation()
  return (
    <div className={`flex min-w-0 flex-col items-center justify-center px-4 py-12 text-center ${className}`}>
      <div className="mb-4 flex h-14 w-14 items-center justify-center rounded-full bg-surface-2 text-muted">
        {icon || <InboxIcon width={26} height={26} />}
      </div>
      <h3 className="text-title font-semibold text-ink">{title || t('state.empty_title')}</h3>
      <p className="mt-2 max-w-prose text-label text-muted">{description ?? t('state.empty_desc')}</p>
      {action && <div className="mt-6 flex flex-wrap justify-center gap-2">{action}</div>}
    </div>
  )
}
