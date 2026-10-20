import { useId } from 'react'
import { controlClass, fieldIds } from './controlClass'
import FieldShell from './FieldShell'

// Text input with label, hint and error. `error` is an already-localized string.
export default function Input({
  label,
  hint,
  error,
  required,
  optionalLabel,
  id: idProp,
  className,
  inputClassName,
  type = 'text',
  ...rest
}) {
  const auto = useId()
  const id = idProp || auto
  const { hintId, errorId } = fieldIds(id)
  return (
    <FieldShell id={id} label={label} required={required} optionalLabel={optionalLabel} hint={hint} error={error} className={className}>
      <input
        id={id}
        type={type}
        required={required}
        aria-invalid={error ? true : undefined}
        aria-describedby={error ? errorId : hint ? hintId : undefined}
        className={controlClass(!!error, inputClassName)}
        {...rest}
      />
    </FieldShell>
  )
}
