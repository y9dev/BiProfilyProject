// Failed load: message plus Retry. Use inside cards/pages; for field errors use Input/Alert.
import { useTranslation } from '../i18n'
import Button from './Button'
import { AlertIcon } from './icons'

export default function ErrorState({ title, description, onRetry, className = '' }) {
  const { t } = useTranslation()
  return (
    <div role="alert" className={`flex min-w-0 flex-col items-center justify-center px-4 py-12 text-center ${className}`}>
      <div className="mb-4 flex h-14 w-14 items-center justify-center rounded-full bg-danger-soft text-danger">
        <AlertIcon width={26} height={26} />
      </div>
      <h3 className="text-title font-semibold text-ink">{title || t('state.error_title')}</h3>
      <p className="mt-2 max-w-prose text-label text-muted">{description ?? t('state.error_desc')}</p>
      {onRetry && (
        <div className="mt-6">
          <Button variant="secondary" onClick={onRetry}>
            {t('common.retry')}
          </Button>
        </div>
      )}
    </div>
  )
}
