// Accessible dialog: role=dialog + aria-modal, focus moves in and is restored, Tab is trapped,
// Escape / backdrop click / close button dismiss, page scroll is locked. Bottom sheet on phones.
import { useEffect, useId, useRef } from 'react'
import { createPortal } from 'react-dom'
import { useTranslation } from '../i18n'
import { cx } from '../lib/cx'
import { CloseIcon } from './icons'

const SIZES = { sm: 'sm:max-w-md', md: 'sm:max-w-xl', lg: 'sm:max-w-3xl', xl: 'sm:max-w-5xl' }
const FOCUSABLE =
  'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'

export default function Modal({ isOpen, onClose, title, size = 'md', footer, children }) {
  const { t } = useTranslation()
  const panelRef = useRef(null)
  const onCloseRef = useRef(onClose)
  const titleId = useId()

  useEffect(() => {
    onCloseRef.current = onClose
  })

  useEffect(() => {
    if (!isOpen) return undefined
    const previouslyFocused = document.activeElement
    const prevOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'

    const panel = panelRef.current
    const first = panel?.querySelector('[data-autofocus]') || panel?.querySelector(FOCUSABLE)
    ;(first || panel)?.focus()

    const onKey = (e) => {
      if (e.key === 'Escape') {
        e.stopPropagation()
        onCloseRef.current?.()
        return
      }
      if (e.key !== 'Tab' || !panel) return
      const nodes = [...panel.querySelectorAll(FOCUSABLE)]
      if (nodes.length === 0) {
        e.preventDefault()
        return
      }
      const firstNode = nodes[0]
      const lastNode = nodes[nodes.length - 1]
      if (e.shiftKey && document.activeElement === firstNode) {
        e.preventDefault()
        lastNode.focus()
      } else if (!e.shiftKey && document.activeElement === lastNode) {
        e.preventDefault()
        firstNode.focus()
      }
    }
    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('keydown', onKey)
      document.body.style.overflow = prevOverflow
      if (previouslyFocused instanceof HTMLElement) previouslyFocused.focus()
    }
  }, [isOpen])

  if (!isOpen) return null

  return createPortal(
    <div
      className="fixed inset-0 z-[60] flex items-end justify-center bg-overlay sm:items-center sm:p-4"
      onMouseDown={(e) => {
        if (e.target === e.currentTarget) onClose?.()
      }}
    >
      <div
        ref={panelRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby={title ? titleId : undefined}
        tabIndex={-1}
        className={cx(
          'sheet-in flex max-h-[92vh] w-full min-w-0 flex-col rounded-t-dialog border border-line bg-surface shadow-pop sm:rounded-dialog',
          SIZES[size],
        )}
      >
        <div className="flex items-start justify-between gap-3 border-b border-line px-4 py-3 sm:px-6">
          <h2 id={titleId} className="min-w-0 pt-2 text-title font-semibold text-ink">
            {title}
          </h2>
          <button
            type="button"
            onClick={onClose}
            aria-label={t('ui.dialog_close')}
            className="flex min-h-touch min-w-touch shrink-0 items-center justify-center rounded-ctl text-muted hover:bg-surface-2 hover:text-ink"
          >
            <CloseIcon />
          </button>
        </div>
        <div className="min-h-0 flex-1 overflow-y-auto px-4 py-4 sm:px-6">{children}</div>
        {footer && (
          <div className="flex flex-wrap justify-end gap-2 border-t border-line px-4 py-3 sm:px-6">{footer}</div>
        )}
      </div>
    </div>,
    document.body,
  )
}
