import { createContext, useContext } from 'react'

export const ToastContext = createContext(null)

// useToast() -> { push(message, { tone, duration }), info(msg), success(msg), error(msg), dismiss(id) }
// `message` must already be localized.
export function useToast() {
  const ctx = useContext(ToastContext)
  if (!ctx) throw new Error('useToast must be used within ToastProvider')
  return ctx
}
