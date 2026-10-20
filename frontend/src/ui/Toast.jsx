import { useCallback, useMemo, useRef, useState } from 'react'
import { useTranslation } from '../i18n'
import { cx } from '../lib/cx'
import { CloseIcon } from './icons'
import { ToastContext } from './toastContext'

const TONES = {
  info: 'border-l-primary',
  success: 'border-l-success',
  warning: 'border-l-warning',
  danger: 'border-l-danger',
}

// Polite live region in the corner (bottom on phones). Auto-dismisses; each toast can be dismissed.
export default function ToastProvider({ children }) {
  const { t } = useTranslation()
  const [items, setItems] = useState([])
  const seq = useRef(0)

  const dismiss = useCallback((id) => setItems((list) => list.filter((x) => x.id !== id)), [])

  const push = useCallback(
    (message, { tone = 'info', duration = 5000 } = {}) => {
      seq.current += 1
      const id = seq.current
      setItems((list) => [...list.slice(-3), { id, message, tone }])
      if (duration > 0) setTimeout(() => dismiss(id), duration)
      return id
    },
    [dismiss],
  )

  const api = useMemo(
    () => ({
      push,
      dismiss,
      info: (m, o) => push(m, { ...o, tone: 'info' }),
      success: (m, o) => push(m, { ...o, tone: 'success' }),
      error: (m, o) => push(m, { ...o, tone: 'danger', duration: 8000 }),
    }),
    [push, dismiss],
  )

  return (
    <ToastContext.Provider value={api}>
      {children}
      <div
        aria-live="polite"
        aria-label={t('ui.notifications')}
        className="pointer-events-none fixed inset-x-4 bottom-4 z-[80] flex flex-col gap-2 sm:left-auto sm:right-4 sm:w-96"
      >
        {items.map((item) => (
          <div
            key={item.id}
            role={item.tone === 'danger' ? 'alert' : 'status'}
            className={cx(
              'toast-in pointer-events-auto flex min-w-0 items-start gap-2 rounded-card border border-l-4 border-line bg-surface p-3 text-label text-ink shadow-pop',
              TONES[item.tone],
            )}
          >
            <p className="min-w-0 flex-1 py-2.5">{item.message}</p>
            <button
              type="button"
              onClick={() => dismiss(item.id)}
              aria-label={t('common.dismiss')}
              className="flex min-h-touch min-w-touch items-center justify-center rounded-ctl text-muted hover:bg-surface-2"
            >
              <CloseIcon />
            </button>
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  )
}
