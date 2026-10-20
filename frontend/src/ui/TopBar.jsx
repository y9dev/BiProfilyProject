// Sticky application bar: menu button (below lg), brand, language, theme and, in the
// minimal shell (no sidebar), sign-out.
import { useAuth } from '../auth/context'
import { useTranslation } from '../i18n'
import Brand from './Brand'
import { LogoutIcon, MenuIcon } from './icons'
import LanguageSwitcher from './LanguageSwitcher'
import ThemeToggle from './ThemeToggle'

export default function TopBar({ onMenu, menuOpen = false, showLogout = false }) {
  const { t } = useTranslation()
  const { me, logout } = useAuth()
  return (
    <header className="sticky top-0 z-40 flex min-h-topbar items-center gap-2 border-b border-line bg-surface px-3 sm:px-4">
      {onMenu && (
        <button
          type="button"
          onClick={onMenu}
          aria-label={t('nav.open_menu')}
          aria-expanded={menuOpen}
          className="flex min-h-touch min-w-touch items-center justify-center rounded-ctl border border-line-strong text-ink hover:bg-surface-2 lg:hidden"
        >
          <MenuIcon />
        </button>
      )}
      <div className="min-w-0 flex-1">
        <Brand />
      </div>
      <LanguageSwitcher />
      <ThemeToggle />
      {showLogout && me && (
        <button
          type="button"
          onClick={logout}
          aria-label={t('common.sign_out')}
          title={t('common.sign_out')}
          className="flex min-h-touch min-w-touch items-center justify-center rounded-ctl border border-line-strong text-ink hover:bg-surface-2"
        >
          <LogoutIcon />
        </button>
      )}
    </header>
  )
}
