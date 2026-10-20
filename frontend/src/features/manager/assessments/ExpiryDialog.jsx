// Change (or clear) the assessment availability deadline.
import { useState } from 'react'
import { fieldErrors } from '../../../api/client'
import * as endpoints from '../../../api/endpoints'
import { useTranslation } from '../../../i18n'
import { errorMessage } from '../../../lib/errors'
import Alert from '../../../ui/Alert'
import Button from '../../../ui/Button'
import Modal from '../../../ui/Modal'
import { useToast } from '../../../ui/toastContext'
import ExpiryField from '../ExpiryField'
import { fromLocalInput, toLocalInput } from '../invitations/status'

export default function ExpiryDialog({ assessment, onClose, onSaved }) {
  const { t } = useTranslation()
  const toast = useToast()
  const [value, setValue] = useState(() => (assessment.expiresAt ? toLocalInput(assessment.expiresAt) : ''))
  const [error, setError] = useState('')
  const [formError, setFormError] = useState('')
  const [busy, setBusy] = useState(false)

  const submit = async (e) => {
    e.preventDefault()
    setError('')
    setFormError('')
    const iso = fromLocalInput(value)
    if (iso === undefined) {
      setError(t('manager.assessments.errors.past_expiry'))
      return
    }
    setBusy(true)
    try {
      await endpoints.manage.setExpiry(assessment.id, iso)
      toast.success(t(iso ? 'manager.assessments.toast.expiry_saved' : 'manager.assessments.toast.expiry_cleared'))
      onSaved()
    } catch (err) {
      const fe = fieldErrors(err)
      if (fe.expiresAt) setError(t(`manager.assessments.errors.${fe.expiresAt}`))
      else setFormError(errorMessage(t, err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal
      isOpen
      onClose={busy ? undefined : onClose}
      title={t('manager.assessments.expiry.title')}
      size="sm"
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={busy}>
            {t('common.cancel')}
          </Button>
          <Button type="submit" form="assessment-expiry-form" loading={busy}>
            {t('common.save')}
          </Button>
        </>
      }
    >
      <form id="assessment-expiry-form" onSubmit={submit} noValidate className="space-y-4">
        {formError && <Alert tone="danger">{formError}</Alert>}
        <p className="text-label text-muted">{t('manager.assessments.expiry.note')}</p>
        <ExpiryField
          label={t('manager.assessments.expiry.label')}
          value={value}
          onChange={setValue}
          error={error || undefined}
        />
      </form>
    </Modal>
  )
}
