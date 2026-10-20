// Horizontal 0-100 bar for one characteristic, with an optional norm marker.
// The value is always printed next to the bar (not encoded by colour alone).
import { useTranslation } from '../../i18n'

export default function Scale({ label, current, norm, color = 'var(--primary)' }) {
  const { t } = useTranslation()
  const pct = Math.min(Math.max(current, 0), 100)
  return (
    <div className="min-w-0">
      <div className="mb-1 flex items-baseline justify-between gap-2 text-label">
        <span className="min-w-0 break-words text-ink">{label}</span>
        <span className="shrink-0 font-semibold tabular-nums text-ink">{current}%</span>
      </div>
      <div
        className="relative h-3 overflow-hidden rounded-ctl bg-surface-2"
        role="img"
        aria-label={t('charts.percent', { label, value: current })}
      >
        <div className="bar-grow h-full rounded-ctl" style={{ width: `${pct}%`, backgroundColor: color }} />
        {norm != null && (
          <div className="absolute inset-y-0 w-0.5 bg-ink" style={{ left: `${norm}%` }} title={t('charts.norm', { value: norm })} />
        )}
      </div>
    </div>
  )
}
