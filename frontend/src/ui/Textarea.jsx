import { useId } from 'react'
import { controlClass, fieldIds } from './controlClass'
import FieldShell from './FieldShell'

export default function Textarea({
  label,
  hint,
  error,
  required,
  optionalLabel,
  id: idProp,
  className,
  inputClassName,
  rows = 4,
  ...rest
}) {
  const auto = useId()
  const id = idProp || auto
  const { hintId, errorId } = fieldIds(id)
  return (
    <FieldShell id={id} label={label} required={required} optionalLabel={optionalLabel} hint={hint} error={error} className={className}>
      <textarea
        id={id}
        rows={rows}
        required={required}
        aria-invalid={error ? true : undefined}
        aria-describedby={error ? errorId : hint ? hintId : undefined}
        className={controlClass(!!error, `resize-y ${inputClassName || ''}`)}
        {...rest}
      />
    </FieldShell>
  )
}
