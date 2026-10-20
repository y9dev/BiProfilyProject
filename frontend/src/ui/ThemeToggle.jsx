import { useTranslation } from '../i18n'
import { useTheme } from '../lib/theme'
import { MoonIcon, SunIcon } from './icons'

export default function ThemeToggle() {
  const { t } = useTranslation()
  const { theme, toggle } = useTheme()
  const dark = theme === 'dark'
  return (
    <button
      type="button"
      onClick={toggle}
      aria-label={dark ? t('theme.to_light') : t('theme.to_dark')}
      title={t('theme.toggle')}
      className="flex min-h-touch min-w-touch items-center justify-center rounded-ctl border border-line-strong bg-surface text-ink hover:bg-surface-2"
    >
      {dark ? <SunIcon /> : <MoonIcon />}
    </button>
  )
}
