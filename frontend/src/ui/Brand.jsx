import { Link } from 'react-router-dom'
import { BRAND_NAME, BRAND_WORD } from '../lib/brand'

// Wordmark: a solid "BI" badge + "Profily", read together as "BI Profily". No gradients or imagery.
export default function Brand({ to = '/', inverted = false }) {
  const content = (
    <>
      <span
        aria-hidden="true"
        className={`flex h-8 w-8 shrink-0 items-center justify-center rounded-ctl text-label font-bold ${
          inverted ? 'bg-primary-ink text-primary' : 'bg-primary text-primary-ink'
        }`}
      >
        BI
      </span>
      <span aria-hidden="true" className="min-w-0 break-words text-body font-bold leading-tight">
        {BRAND_WORD}
      </span>
      <span className="sr-only">{BRAND_NAME}</span>
    </>
  )
  const cls = `inline-flex min-h-touch min-w-0 items-center gap-2 rounded-ctl ${inverted ? 'text-primary-ink' : 'text-ink'}`
  return to ? (
    <Link to={to} className={cls}>
      {content}
    </Link>
  ) : (
    <div className={cls}>{content}</div>
  )
}
