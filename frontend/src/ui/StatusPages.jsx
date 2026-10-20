// 403 / 404 views rendered inside the shell.
import { useTranslation } from '../i18n'
import Button from './Button'
import Card from './Card'
import EmptyState from './EmptyState'
import { AlertIcon } from './icons'

function StatusPage({ titleKey, descKey }) {
  const { t } = useTranslation()
  return (
    <Card>
      <EmptyState
        icon={<AlertIcon width={26} height={26} />}
        title={t(titleKey)}
        description={t(descKey)}
        action={<Button to="/">{t('common.home')}</Button>}
      />
    </Card>
  )
}

export function ForbiddenPage() {
  return <StatusPage titleKey="pages.forbidden.title" descKey="pages.forbidden.desc" />
}

export function NotFoundPage() {
  return <StatusPage titleKey="pages.not_found.title" descKey="pages.not_found.desc" />
}
