// Create invitation: optional expiry + max registrations. Server is authoritative (422 field errors are shown inline).
import { useState } from 'react'
import { fieldErrors } from '../../../api/client'
import * as endpoints from '../../../api/endpoints'
import { useTranslation } from '../../../i18n'
import { errorMessage } from '../../../lib/errors'
import Alert from '../../../ui/Alert'
import Button from '../../../ui/Button'
import Input from '../../../ui/Input'
import Modal from '../../../ui/Modal'
import { useToast } from '../../../ui/toastContext'
import ExpiryField from '../ExpiryField'
import { defaultLocal, fromLocalInput, isPast, validateMax } from './status'

export default function InvitationFormDialog({ isOpen, onClose, onCreated }) {
  const { t } = useTranslation()
  const toast = useToast()
  const [expiry, setExpiry] = useState(() => defaultLocalWeek())
  const [max, setMax] = useState('10')
  const [errors, setErrors] = useState({})
  const [formError, setFormError] = useState('')
  const [busy, setBusy] = useState(false)

  const submit = async (e) => {
    e.preventDefault()
    const next = {}
    const iso = fromLocalInput(expiry)
    if (iso === undefined || (iso && isPast(iso))) next.expiresAt = 'past_expiry'
    if (validateMax(max, 0)) next.maxRegistrations = 'out_of_range'
    setErrors(next)
    setFormError('')
    if (Object.keys(next).length) return
    setBusy(true)
    try {
      const inv = await endpoints.invitations.create(iso, Number(max))
      toast.success(t('manager.invitations.toast.created'))
      onCreated(inv)
    } catch (err) {
      const fe = fieldErrors(err)
      if (Object.keys(fe).length) setErrors(fe)
      else setFormError(errorMessage(t, err))
    } finally {
      setBusy(false)
    }
  }

  const fieldMsg = (path) => (errors[path] ? t(`manager.invitations.errors.${errors[path]}`) : undefined)

  return (
    <Modal
      isOpen={isOpen}
      onClose={busy ? undefined : onClose}
      title={t('manager.invitations.form.title')}
      size="sm"
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={busy}>
            {t('common.cancel')}
          </Button>
          <Button type="submit" form="invitation-form" loading={busy}>
            {t('common.create')}
          </Button>
        </>
      }
    >
      <form id="invitation-form" onSubmit={submit} noValidate className="space-y-4">
        {formError && <Alert tone="danger">{formError}</Alert>}
        <Input
          type="number"
          inputMode="numeric"
          min={1}
          max={10000}
          label={t('manager.invitations.form.max')}
          hint={t('manager.invitations.form.max_hint')}
          error={fieldMsg('maxRegistrations')}
          value={max}
          onChange={(e) => setMax(e.target.value)}
          data-autofocus
        />
        <ExpiryField
          label={t('manager.invitations.form.expires')}
          hint={t('manager.invitations.form.expires_hint')}
          error={fieldMsg('expiresAt')}
          value={expiry}
          onChange={setExpiry}
        />
      </form>
    </Modal>
  )
}

function defaultLocalWeek() {
  const v = defaultLocal()
  const d = new Date(v)
  d.setDate(d.getDate() + 6)
  const p = (n) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}T18:00`
}
