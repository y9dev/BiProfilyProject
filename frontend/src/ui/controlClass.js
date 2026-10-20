// Shared look of text-like form controls (Input, Select, Textarea).
export function controlClass(hasError, extra) {
  return [
    'block w-full min-h-touch min-w-0 rounded-ctl border bg-surface px-3 py-2 text-body text-ink placeholder:text-faint',
    'disabled:cursor-not-allowed disabled:bg-surface-2 disabled:text-muted',
    hasError ? 'border-danger' : 'border-line-strong',
    extra,
  ]
    .filter(Boolean)
    .join(' ')
}

export function fieldIds(id) {
  return { hintId: `${id}-hint`, errorId: `${id}-error` }
}
