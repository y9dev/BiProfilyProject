import { useTranslation } from '../i18n'

const SIZES = { sm: 'h-4 w-4', md: 'h-6 w-6', lg: 'h-10 w-10' }

// Announces itself to assistive tech (role=status) with a visually hidden label.
export default function Spinner({ size = 'md', label, className = '' }) {
  const { t } = useTranslation()
  return (
    <span role="status" className={`inline-flex items-center gap-2 text-muted ${className}`}>
      <svg className={`spin ${SIZES[size]}`} viewBox="0 0 24 24" fill="none" aria-hidden="true">
        <circle cx="12" cy="12" r="9" stroke="currentColor" strokeWidth="3" opacity="0.25" />
        <path d="M21 12a9 9 0 0 0-9-9" stroke="currentColor" strokeWidth="3" strokeLinecap="round" />
      </svg>
      <span className="sr-only">{label || t('state.loading')}</span>
    </span>
  )
}
