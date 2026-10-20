// Four entry-point cards of the manager dashboard (Assessments, Invitations, Characteristics, Departments).
// Mounted by features/employee/Dashboard.jsx for manager and admin roles only.
import { Link } from 'react-router-dom'
import { MANAGER_ROLES, useAuth } from '../../auth/context'
import { useTranslation } from '../../i18n'

const ENTRIES = [
  { to: '/manage/assessments', key: 'assessments', path: 'M9 4h6l1 2h3v14H5V6h3l1-2zM9 12h6M9 16h4' },
  { to: '/manage/invitations', key: 'invitations', path: 'M3 7l9 6 9-6M3 7v10h18V7M3 7l9-4 9 4' },
  { to: '/manage/characteristics', key: 'characteristics', path: 'M4 20V10M10 20V4M16 20v-7M22 20H2' },
  { to: '/manage/departments', key: 'departments', path: 'M3 21h18M5 21V7l7-4 7 4v14M9 21v-6h6v6M9 10h.01M15 10h.01' },
]

export default function ManagerEntryPoints() {
  const { t } = useTranslation()
  const { me } = useAuth()
  if (!MANAGER_ROLES.includes(me?.role)) return null

  return (
    <section aria-labelledby="manager-entry-title" className="min-w-0">
      <h2 id="manager-entry-title" className="mb-3 text-title font-semibold text-ink">
        {t('manager.entry.title')}
      </h2>
      <ul className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-4">
        {ENTRIES.map((e) => (
          <li key={e.key} className="min-w-0">
            <Link
              to={e.to}
              className="group flex h-full min-w-0 flex-col gap-3 rounded-card border border-line bg-surface p-4 transition-colors hover:border-primary hover:bg-primary-soft sm:p-5"
            >
              <span className="flex h-11 w-11 items-center justify-center rounded-ctl bg-primary-soft text-primary group-hover:bg-surface">
                <svg
                  width="22"
                  height="22"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  aria-hidden="true"
                  focusable="false"
                >
                  <path d={e.path} />
                </svg>
              </span>
              <span className="min-w-0">
                <span className="block text-body font-semibold text-ink break-words">
                  {t(`manager.entry.${e.key}.title`)}
                </span>
                <span className="mt-1 block text-label text-muted break-words">{t(`manager.entry.${e.key}.desc`)}</span>
              </span>
            </Link>
          </li>
        ))}
      </ul>
    </section>
  )
}
