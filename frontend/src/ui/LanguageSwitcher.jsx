// ru/kk segmented switch. Persisted by TranslationProvider (bip:v1:lang). Latin codes are
// language-neutral labels; the full names are in the tooltip / accessible name.
import { LANGS, useTranslation } from '../i18n'
import { cx } from '../lib/cx'

export default function LanguageSwitcher() {
  const { lang, setLang, t } = useTranslation()
  return (
    <div
      role="group"
      aria-label={t('lang.label')}
      className="inline-flex overflow-hidden rounded-ctl border border-line-strong bg-surface"
    >
      {LANGS.map((code) => {
        const active = lang === code
        return (
          <button
            key={code}
            type="button"
            lang={code}
            aria-pressed={active}
            aria-label={t(`lang.${code}`)}
            title={t(`lang.${code}`)}
            onClick={() => setLang(code)}
            className={cx(
              'min-h-touch min-w-touch px-2 text-label font-semibold transition-colors',
              active ? 'bg-primary text-primary-ink' : 'text-ink hover:bg-surface-2',
            )}
          >
            {code.toUpperCase()}
          </button>
        )
      })}
    </div>
  )
}
