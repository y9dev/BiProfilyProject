// Page title row. Wraps on narrow screens; actions never overflow.
export default function PageHeader({ title, description, actions }) {
  return (
    <header className="mb-6 flex flex-wrap items-start justify-between gap-x-6 gap-y-3">
      <div className="min-w-0 flex-1 basis-64">
        <h1 className="text-title font-bold text-ink sm:text-display">{title}</h1>
        {description && <p className="mt-1 text-body text-muted">{description}</p>}
      </div>
      {actions && <div className="flex flex-wrap items-center gap-2">{actions}</div>}
    </header>
  )
}
