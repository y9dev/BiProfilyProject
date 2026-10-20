// Role-aware navigation + account block. Used by the desktop sidebar and the mobile drawer.
import { NavLink, useLocation } from 'react-router-dom'
import { MANAGER_ROLES, useAuth } from '../auth/context'
import { useTranslation } from '../i18n'
import { cx } from '../lib/cx'
import Badge from './Badge'
import Button from './Button'
import { LogoutIcon } from './icons'

function buildSections(role) {
  // Take and result pages belong to the dashboard section (they have no nav item of their own).
  const home = { to: '/', labelKey: 'nav.dashboard', end: true, also: ['/assessments/', '/results/'] }
  const sections = [{ id: 'main', titleKey: 'nav.main', items: [home] }]
  if (MANAGER_ROLES.includes(role)) {
    sections.push({
      id: 'manage',
      titleKey: 'nav.manage',
      items: [
        { to: '/manage/invitations', labelKey: 'nav.invitations' },
        { to: '/manage/assessments', labelKey: 'nav.assessments' },
        { to: '/manage/characteristics', labelKey: 'nav.characteristics' },
        { to: '/manage/departments', labelKey: 'nav.departments' },
      ],
    })
  }
  if (role === 'admin') {
    sections.push({ id: 'admin', titleKey: 'nav.admin', items: [{ to: '/admin', labelKey: 'nav.admin' }] })
  }
  return sections
}

export default function SidebarNav({ onNavigate }) {
  const { t } = useTranslation()
  const { me, logout } = useAuth()
  const sections = buildSections(me?.role)
  const { pathname } = useLocation()

  return (
    <div className="flex flex-1 flex-col gap-6 p-3">
      <nav aria-label={t('nav.primary')} className="flex flex-col gap-5">
        {sections.map((section) => (
          <div key={section.id}>
            {sections.length > 1 && section.id !== 'admin' && (
              <p className="mb-1 px-3 text-caption font-semibold uppercase tracking-wide text-faint">
                {t(section.titleKey)}
              </p>
            )}
            <ul className="flex flex-col gap-1">
              {section.items.map((item) => (
                <li key={item.to}>
                  <NavLink
                    to={item.to}
                    end={item.end}
                    onClick={onNavigate}
                    className={({ isActive }) =>
                      cx(
                        'flex min-h-touch min-w-0 items-center rounded-ctl border-l-4 px-3 py-2 text-label break-words',
                        isActive || item.also?.some((p) => pathname.startsWith(p))
                          ? 'border-primary bg-primary-soft font-semibold text-primary'
                          : 'border-transparent text-ink hover:bg-surface-2',
                      )
                    }
                  >
                    {t(item.labelKey)}
                  </NavLink>
                </li>
              ))}
            </ul>
          </div>
        ))}
      </nav>

      {me && (
        <div className="mt-auto flex min-w-0 flex-col gap-3 border-t border-line pt-4">
          <div className="min-w-0 px-1">
            <p className="break-words text-label font-semibold text-ink">{me.fullName}</p>
            {me.jobTitle && <p className="break-words text-caption text-muted">{me.jobTitle}</p>}
            <div className="mt-2">
              <Badge tone="info">{t(`roles.${me.role}`)}</Badge>
            </div>
          </div>
          <Button variant="secondary" fullWidth onClick={logout}>
            <LogoutIcon />
            {t('common.sign_out')}
          </Button>
        </div>
      )}
    </div>
  )
}
