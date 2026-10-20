// Shared period control for /analytics: preset segments plus a custom range that needs both dates.
// The segment row scrolls inside itself on phones (never the page).
import { useState } from 'react'
import { useTranslation } from '../../i18n'
import { cx } from '../../lib/cx'
import Button from '../../ui/Button'
import Input from '../../ui/Input'
import { PRESETS, almatyToday, customRangeError } from './analyticsState'

export default function PeriodFilter({ period, from, to, onChange, serverError }) {
  const { t } = useTranslation()
  const today = almatyToday()
  const [open, setOpen] = useState(period === 'custom')
  const [draft, setDraft] = useState({ from: from || '', to: to || '' })
  const [error, setError] = useState(null)

  const pickPreset = (p) => {
    setOpen(false)
    setError(null)
    onChange({ period: p, from: '', to: '' })
  }

  const apply = (e) => {
    e.preventDefault()
    const err = customRangeError(draft.from, draft.to, today)
    setError(err)
    if (!err) onChange({ period: 'custom', from: draft.from, to: draft.to })
  }

  const shownError = error || serverError
  const msg = (path) => (shownError?.path === path ? t(`stats.validation.${shownError.code}`) : undefined)
  const customActive = period === 'custom' || open

  return (
    <div className="min-w-0">
      <p id="period-label" className="mb-1 text-label font-medium text-ink">
        {t('stats.period.label')}
      </p>
      <div className="-mx-1 overflow-x-auto px-1 pb-1">
        <div role="group" aria-labelledby="period-label" className="inline-flex gap-1 rounded-ctl border border-line bg-surface-2 p-1">
          {PRESETS.map((p) => (
            <button
              key={p}
              type="button"
              aria-pressed={period === p && !open}
              onClick={() => pickPreset(p)}
              className={cx(
                'min-h-touch shrink-0 whitespace-nowrap rounded-ctl px-3 text-label font-semibold transition-colors',
                period === p && !open ? 'bg-surface text-primary shadow-sm ring-1 ring-line' : 'text-muted hover:text-ink',
              )}
            >
              {t(`stats.period.${p}`)}
            </button>
          ))}
          <button
            type="button"
            aria-pressed={customActive}
            aria-expanded={open}
            aria-controls="period-custom"
            onClick={() => {
              setDraft({ from: from || '', to: to || '' })
              setError(null)
              setOpen((v) => !v)
            }}
            className={cx(
              'min-h-touch shrink-0 whitespace-nowrap rounded-ctl px-3 text-label font-semibold transition-colors',
              customActive ? 'bg-surface text-primary shadow-sm ring-1 ring-line' : 'text-muted hover:text-ink',
            )}
          >
            {t('stats.period.custom')}
          </button>
        </div>
      </div>

      {open && (
        <form id="period-custom" onSubmit={apply} noValidate className="mt-3 flex flex-wrap items-start gap-3">
          <Input
            type="date"
            label={t('stats.period.from')}
            value={draft.from}
            max={today}
            onChange={(e) => setDraft((d) => ({ ...d, from: e.target.value }))}
            error={msg('from')}
            required
            className="w-full xs:w-44"
          />
          <Input
            type="date"
            label={t('stats.period.to')}
            value={draft.to}
            max={today}
            onChange={(e) => setDraft((d) => ({ ...d, to: e.target.value }))}
            error={msg('to')}
            required
            className="w-full xs:w-44"
          />
          <Button type="submit" variant="secondary" className="xs:mt-6">
            {t('analytics.period.apply')}
          </Button>
        </form>
      )}
    </div>
  )
}
