// Date/time input with an explicit "no expiry" switch. `value` is a datetime-local string or ''.
import { useId } from 'react'
import { useTranslation } from '../../i18n'
import Input from '../../ui/Input'
import { defaultLocal } from './invitations/status'

export default function ExpiryField({ value, onChange, error, label, hint, disabled }) {
  const { t } = useTranslation()
  const cbId = useId()
  const never = value === ''
  return (
    <div className="space-y-2">
      <Input
        type="datetime-local"
        label={label}
        hint={hint}
        error={error}
        value={value}
        disabled={disabled || never}
        onChange={(e) => onChange(e.target.value)}
      />
      <div className="flex min-h-touch items-center gap-2">
        <input
          id={cbId}
          type="checkbox"
          className="h-5 w-5 shrink-0 accent-primary"
          checked={never}
          disabled={disabled}
          onChange={(e) => onChange(e.target.checked ? '' : defaultLocal())}
        />
        <label htmlFor={cbId} className="min-w-0 text-label text-ink">
          {t('manager.expiry.never')}
        </label>
      </div>
    </div>
  )
}
