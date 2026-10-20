// Responsive list: a real <table> from md up, stacked cards below (no horizontal scroll at 375px).
// columns: [{ key, header, render(row), primary?, actions?, className? }]
//   primary -> card title on mobile; actions -> footer row on mobile / last cell on desktop.
import { cx } from '../../lib/cx'

export default function DataTable({ columns, rows, rowKey, caption, rowClassName }) {
  const dataCols = columns.filter((c) => !c.primary && !c.actions)
  const primary = columns.find((c) => c.primary)
  const actions = columns.find((c) => c.actions)

  return (
    <>
      <div className="hidden min-w-0 overflow-hidden rounded-card border border-line bg-surface xl:block">
        <table className="w-full table-fixed border-collapse text-left text-label">
          {caption && <caption className="sr-only">{caption}</caption>}
          <thead className="bg-surface-2 text-caption font-semibold uppercase tracking-wide text-muted">
            <tr>
              {columns.map((c) => (
                <th key={c.key} scope="col" className={cx('px-4 py-3 align-bottom', c.className)}>
                  {c.header}
                </th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-line">
            {rows.map((row) => (
              <tr key={rowKey(row)} className={cx('align-top', rowClassName?.(row))}>
                {columns.map((c) => (
                  <td key={c.key} className={cx('min-w-0 px-4 py-3 break-words', c.className)}>
                    {c.render(row)}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <ul className="space-y-3 xl:hidden">
        {rows.map((row) => (
          <li
            key={rowKey(row)}
            className={cx('min-w-0 rounded-card border border-line bg-surface p-4', rowClassName?.(row))}
          >
            {primary && <div className="min-w-0 text-body font-semibold text-ink break-words">{primary.render(row)}</div>}
            <dl className={cx('grid grid-cols-1 gap-2', primary && 'mt-3')}>
              {dataCols.map((c) => (
                <div key={c.key} className="grid min-w-0 grid-cols-[minmax(0,2fr)_minmax(0,3fr)] gap-2 text-label">
                  <dt className="text-muted break-words">{c.header}</dt>
                  <dd className="min-w-0 text-ink break-words">{c.render(row)}</dd>
                </div>
              ))}
            </dl>
            {actions && <div className="mt-3 flex flex-wrap gap-2">{actions.render(row)}</div>}
          </li>
        ))}
      </ul>
    </>
  )
}
