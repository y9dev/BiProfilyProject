// Registration, step 2 of 2: the mandatory avatar. Two panes: picker (left) and a live preview (right on
// desktop, a sticky strip on phones). The catalog comes from GET /api/avatar/catalog, the choice is saved with
// PUT /api/me/avatar; afterwards /api/auth/me is re-read so the route guards see profileComplete = true.
import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import * as endpoints from '../../api/endpoints'
import { useAuth } from '../../auth/context'
import Avatar from '../../avatar/Avatar'
import AvatarPicker from '../../avatar/AvatarPicker'
import { normalizeAvatar } from '../../avatar/catalog'
import { useTranslation } from '../../i18n'
import { errorMessage } from '../../lib/errors'
import { useAsync } from '../../lib/useAsync'
import Alert from '../../ui/Alert'
import Async from '../../ui/Async'
import Button from '../../ui/Button'
import Card from '../../ui/Card'
import Skeleton from '../../ui/Skeleton'

function useSummary(value) {
  const { t } = useTranslation()
  return {
    universe: t(`avatar.universe.${value.universe}`),
    gender: t(`avatar.gender.${value.gender}`),
    style: t(`avatar.style.${value.style}`),
  }
}

export default function AvatarOnboardingPage() {
  const { t } = useTranslation()
  const { me, refresh } = useAuth()
  const navigate = useNavigate()
  const catalog = useAsync((signal) => endpoints.avatar.catalog({ signal }), [])
  const [value, setValue] = useState(() => normalizeAvatar(me?.avatar))
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const summary = useSummary(value)
  const alt = t('avatar.preview_alt', summary)

  async function save() {
    if (busy) return
    setBusy(true)
    setError('')
    try {
      await endpoints.avatar.save(value.universe, value.gender, value.style)
      await refresh()
      navigate('/', { replace: true })
    } catch (e) {
      setError(errorMessage(t, e))
      setBusy(false)
    }
  }

  const ready = catalog.status === 'ready'

  return (
    <div className="mx-auto w-full min-w-0 max-w-6xl">
      <p className="text-label font-semibold uppercase tracking-wide text-primary">{t('auth.register.step', { n: 2 })}</p>
      <h1 className="mt-1 text-display font-bold text-ink">{t('avatar.page_title')}</h1>
      <p className="mt-2 max-w-2xl text-body text-muted">{t('avatar.page_lead')}</p>

      <div className="mt-6 grid min-w-0 gap-4 lg:grid-cols-[minmax(0,1fr)_22rem] lg:items-start lg:gap-8">
        <Card
          as="div"
          className="sticky top-topbar z-10 order-first !p-3 lg:top-[4.5rem] lg:order-last lg:!p-6"
          aria-label={t('avatar.preview')}
        >
          <div className="flex min-w-0 items-center gap-4 lg:flex-col lg:text-center">
            <div className="w-20 flex-none lg:w-44">
              <Avatar avatar={value} size="100%" label={alt} />
            </div>
            <div className="min-w-0 flex-1">
              <p className="text-caption font-medium uppercase tracking-wide text-muted">{t('avatar.preview')}</p>
              <p className="mt-0.5 text-label font-semibold text-ink [overflow-wrap:anywhere]">
                {summary.universe} · {summary.gender} · {summary.style}
              </p>
            </div>
          </div>

          <div className="mt-5 hidden border-t border-line pt-4 lg:block">
            <p className="text-caption text-muted">{t('avatar.sizes')}</p>
            <div className="mt-2 flex items-end justify-center gap-3">
              <Avatar avatar={value} size={32} />
              <Avatar avatar={value} size={48} />
              <Avatar avatar={value} size={72} />
            </div>
          </div>

          <div className="mt-5 hidden lg:block">
            <Button fullWidth loading={busy} disabled={!ready} onClick={save}>
              {t('avatar.save')}
            </Button>
          </div>
        </Card>

        <Card className="min-w-0">
          <Async
            status={catalog.status}
            onRetry={catalog.reload}
            skeleton={
              <div className="space-y-4" aria-busy="true">
                <Skeleton className="h-6 w-1/3" />
                <Skeleton className="h-24 w-full" />
                <Skeleton className="h-24 w-full" />
              </div>
            }
          >
            <AvatarPicker value={value} onChange={setValue} catalog={catalog.data} />
            {error && (
              <Alert tone="danger" className="mt-6">
                {error}
              </Alert>
            )}
            <div className="mt-6 lg:hidden">
              <Button fullWidth loading={busy} onClick={save}>
                {t('avatar.save')}
              </Button>
            </div>
          </Async>
        </Card>
      </div>
    </div>
  )
}
