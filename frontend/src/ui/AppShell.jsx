// Authenticated layout route. Full-width: top bar + (lg) fixed sidebar + fluid content area.
// Below lg the sidebar becomes a drawer. `minimal` drops navigation (avatar onboarding).
import { Suspense, useEffect, useRef, useState } from 'react'
import { Outlet, useLocation } from 'react-router-dom'
import { useTranslation } from '../i18n'
import ErrorBoundary from './ErrorBoundary'
import { CloseIcon } from './icons'
import SidebarNav from './SidebarNav'
import Spinner from './Spinner'
import TopBar from './TopBar'

function Drawer({ open, onClose }) {
  const { t } = useTranslation()
  const closeRef = useRef(null)

  useEffect(() => {
    if (!open) return undefined
    closeRef.current?.focus()
    const onKey = (e) => e.key === 'Escape' && onClose()
    document.addEventListener('keydown', onKey)
    const prev = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => {
      document.removeEventListener('keydown', onKey)
      document.body.style.overflow = prev
    }
  }, [open, onClose])

  if (!open) return null
  return (
    <div className="fixed inset-0 z-50 lg:hidden" onMouseDown={(e) => e.target === e.currentTarget && onClose()}>
      <div className="absolute inset-0 bg-overlay" aria-hidden="true" />
      <div
        role="dialog"
        aria-modal="true"
        aria-label={t('nav.primary')}
        className="sheet-in relative flex h-full w-[min(20rem,88vw)] flex-col overflow-y-auto border-r border-line bg-surface"
      >
        <div className="flex justify-end p-2">
          <button
            ref={closeRef}
            type="button"
            onClick={onClose}
            aria-label={t('nav.close_menu')}
            className="flex min-h-touch min-w-touch items-center justify-center rounded-ctl text-muted hover:bg-surface-2 hover:text-ink"
          >
            <CloseIcon />
          </button>
        </div>
        <SidebarNav onNavigate={onClose} />
      </div>
    </div>
  )
}

export default function AppShell({ minimal = false }) {
  const { t } = useTranslation()
  const location = useLocation()
  // The drawer is open only for the path it was opened on, so navigating closes it.
  const [openAt, setOpenAt] = useState(null)
  const menuOpen = openAt === location.pathname
  const setMenuOpen = (open) => setOpenAt(open ? location.pathname : null)

  return (
    <div className="min-h-screen bg-bg">
      <a
        href="#main"
        className="sr-only focus:not-sr-only focus:fixed focus:left-2 focus:top-2 focus:z-[90] focus:rounded-ctl focus:bg-primary focus:px-4 focus:py-3 focus:text-primary-ink"
      >
        {t('nav.skip_to_content')}
      </a>
      <TopBar onMenu={minimal ? undefined : () => setMenuOpen(true)} menuOpen={menuOpen} showLogout={minimal} />
      {!minimal && <Drawer open={menuOpen} onClose={() => setMenuOpen(false)} />}
      <div className={minimal ? '' : 'lg:grid lg:grid-cols-[17rem_minmax(0,1fr)]'}>
        {!minimal && (
          <aside className="sticky top-topbar hidden h-[calc(100vh-3.5rem)] overflow-y-auto flex-col border-r border-line bg-surface lg:flex">
            <SidebarNav />
          </aside>
        )}
        <main id="main" tabIndex={-1} className="min-w-0 p-4 sm:p-6 lg:p-8">
          <ErrorBoundary resetKey={location.pathname}>
            <Suspense
              fallback={
                <div className="flex justify-center py-20">
                  <Spinner size="lg" label={t('ui.page_loading')} />
                </div>
              }
            >
              <div key={location.pathname} className="page-in min-w-0">
                <Outlet />
              </div>
            </Suspense>
          </ErrorBoundary>
        </main>
      </div>
    </div>
  )
}
