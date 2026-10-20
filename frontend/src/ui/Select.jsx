import { useId } from 'react'
import { controlClass, fieldIds } from './controlClass'
import FieldShell from './FieldShell'
import { ChevronDownIcon } from './icons'

// Native <select> (best mobile UX and accessibility) with the shared control look.
// Options: pass <option> children, or `options=[{value,label}]`.
export default function Select({
  label,
  hint,
  error,
  required,
  optionalLabel,
  id: idProp,
  className,
  inputClassName,
  options,
  children,
  ...rest
}) {
  const auto = useId()
  const id = idProp || auto
  const { hintId, errorId } = fieldIds(id)
  return (
    <FieldShell id={id} label={label} required={required} optionalLabel={optionalLabel} hint={hint} error={error} className={className}>
      <div className="relative">
        <select
          id={id}
          required={required}
          aria-invalid={error ? true : undefined}
          aria-describedby={error ? errorId : hint ? hintId : undefined}
          className={controlClass(!!error, `appearance-none pr-10 ${inputClassName || ''}`)}
          {...rest}
        >
          {options
            ? options.map((o) => (
                <option key={o.value} value={o.value}>
                  {o.label}
                </option>
              ))
            : children}
        </select>
        <ChevronDownIcon className="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-muted" />
      </div>
    </FieldShell>
  )
}
