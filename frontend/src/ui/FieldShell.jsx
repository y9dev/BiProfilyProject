import { fieldIds } from './controlClass'
import { AlertIcon } from './icons'

// Label + control slot + hint + error. Aria wiring comes from the parent control through fieldIds().

export default function FieldShell({ id, label, required, optionalLabel, hint, error, className = '', children }) {
  const { hintId, errorId } = fieldIds(id)
  return (
    <div className={`min-w-0 ${className}`}>
      {label && (
        <label htmlFor={id} className="mb-1 block text-label font-medium text-ink">
          {label}
          {required && (
            <span className="text-danger" aria-hidden="true">
              {' *'}
            </span>
          )}
          {!required && optionalLabel && <span className="font-normal text-muted">{` (${optionalLabel})`}</span>}
        </label>
      )}
      {children}
      {hint && !error && (
        <p id={hintId} className="mt-1 text-caption text-muted">
          {hint}
        </p>
      )}
      {error && (
        <p id={errorId} className="mt-1 flex items-start gap-1 text-caption font-medium text-danger">
          <AlertIcon width={16} height={16} className="mt-0.5 shrink-0" />
          <span className="min-w-0">{error}</span>
        </p>
      )}
    </div>
  )
}
