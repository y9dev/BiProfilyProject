// Choose the avatar again (same picker and API as onboarding); the widget is refreshed by the caller.
import { useState } from 'react'
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
import Modal from '../../ui/Modal'
import Skeleton from '../../ui/Skeleton'
import { useToast } from '../../ui/toastContext'

export default function EditAvatarDialog({ onClose, onSaved }) {
  const { t } = useTranslation()
  const { me, refresh } = useAuth()
  const toast = useToast()
  const catalog = useAsync((signal) => endpoints.avatar.catalog({ signal }), [])
  const [value, setValue] = useState(() => normalizeAvatar(me?.avatar))
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  async function save() {
    if (busy) return
    setBusy(true)
    setError('')
    try {
      await endpoints.avatar.save(value.universe, value.gender, value.style)
      await refresh()
      toast.success(t('employee.profile.avatar_saved'))
      onSaved()
    } catch (e) {
      setError(errorMessage(t, e))
      setBusy(false)
    }
  }

  return (
    <Modal
      isOpen
      onClose={busy ? undefined : onClose}
      size="xl"
      title={t('employee.profile.edit_avatar')}
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={busy}>
            {t('common.cancel')}
          </Button>
          <Button loading={busy} disabled={catalog.status !== 'ready'} onClick={save}>
            {t('common.save')}
          </Button>
        </>
      }
    >
      <div className="grid min-w-0 gap-4 md:grid-cols-[minmax(0,1fr)_9rem]">
        <div className="min-w-0 md:order-first">
          <Async
            status={catalog.status}
            onRetry={catalog.reload}
            skeleton={
              <div className="space-y-4" aria-busy="true">
                <Skeleton className="h-6 w-1/3" />
                <Skeleton className="h-24 w-full" />
              </div>
            }
          >
            <AvatarPicker value={value} onChange={setValue} catalog={catalog.data} />
          </Async>
          {error && (
            <Alert tone="danger" className="mt-4">
              {error}
            </Alert>
          )}
        </div>
        <div className="order-first mx-auto w-24 md:order-last md:w-full">
          <Avatar avatar={value} size="100%" label={t('avatar.preview')} />
        </div>
      </div>
    </Modal>
  )
}
