// Localized message for a server/client error code.
export function errText(t, code) {
  if (!code) return undefined
  const v = t(`wizard.err.${code}`)
  return v === `wizard.err.${code}` ? t('wizard.err.invalid') : v
}

export function charName(c, lang) {
  return (c?.nameI18n && c.nameI18n[lang]) || c?.name || ''
}
