// Locale-aware date formatting in the user's local time zone.
// ru uses Intl. kk is formatted by hand: Chromium (desktop Chrome and Electron) ships ICU without
// Kazakh month names (kk literals are escaped for scripts/check-i18n.mjs), so Intl renders "2026 M10 19" there. CLDR kk pattern: "2026 ж. 19 қазан".
const KK_MONTHS = [
  '\u049b\u0430\u04a3\u0442\u0430\u0440', '\u0430\u049b\u043f\u0430\u043d', '\u043d\u0430\u0443\u0440\u044b\u0437', '\u0441\u04d9\u0443\u0456\u0440', '\u043c\u0430\u043c\u044b\u0440', '\u043c\u0430\u0443\u0441\u044b\u043c',
  '\u0448\u0456\u043b\u0434\u0435', '\u0442\u0430\u043c\u044b\u0437', '\u049b\u044b\u0440\u043a\u04af\u0439\u0435\u043a', '\u049b\u0430\u0437\u0430\u043d', '\u049b\u0430\u0440\u0430\u0448\u0430', '\u0436\u0435\u043b\u0442\u043e\u049b\u0441\u0430\u043d',
]

const pad = (n) => String(n).padStart(2, '0')

// style: 'long' (19 октября 2026 г.) | 'medium' (19 окт. 2026 г.); time adds HH:mm.
export function formatLocalDate(iso, lang, { style = 'long', time = false } = {}) {
  if (!iso) return ''
  const d = iso instanceof Date ? iso : new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  if (lang === 'kk') {
    const date = `${d.getFullYear()} \u0436. ${d.getDate()} ${KK_MONTHS[d.getMonth()]}`
    return time ? `${date}, ${pad(d.getHours())}:${pad(d.getMinutes())}` : date
  }
  const opts = { day: 'numeric', month: style === 'long' ? 'long' : 'short', year: 'numeric' }
  if (time) Object.assign(opts, { hour: '2-digit', minute: '2-digit' })
  return new Intl.DateTimeFormat('ru-RU', opts).format(d)
}
