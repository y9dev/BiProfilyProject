// Public layout (login, register): brand panel + form area filling the viewport.
import { Suspense } from 'react'
import { Outlet } from 'react-router-dom'
import { useTranslation } from '../i18n'
import Brand from './Brand'
import ErrorBoundary from './ErrorBoundary'
import LanguageSwitcher from './LanguageSwitcher'
import Spinner from './Spinner'
import ThemeToggle from './ThemeToggle'

export default function AuthLayout() {
  const { t } = useTranslation()
  return (
    <div className="grid min-h-screen bg-bg lg:grid-cols-[minmax(0,2fr)_minmax(0,3fr)]">
      <aside className="hidden flex-col justify-between bg-primary p-10 text-primary-ink lg:flex">
        <Brand inverted />
        <div className="min-w-0">
          <p className="text-display font-bold">{t('app.tagline')}</p>
          <p className="mt-4 max-w-md text-body opacity-90">{t('app.tagline_long')}</p>
        </div>
        <div aria-hidden="true" />
      </aside>

      <div className="flex min-w-0 flex-col">
        <div className="flex items-center gap-2 px-3 py-3 sm:px-6">
          <div className="min-w-0 flex-1 lg:invisible">
            <Brand />
          </div>
          <LanguageSwitcher />
          <ThemeToggle />
        </div>
        <main id="main" className="flex min-w-0 flex-1 items-start justify-center p-4 sm:p-8 lg:items-center">
          <div className="w-full min-w-0 max-w-xl">
            <ErrorBoundary>
              <Suspense
                fallback={
                  <div className="flex justify-center py-20">
                    <Spinner size="lg" label={t('ui.page_loading')} />
                  </div>
                }
              >
                <Outlet />
              </Suspense>
            </ErrorBoundary>
          </div>
        </main>
      </div>
    </div>
  )
}
