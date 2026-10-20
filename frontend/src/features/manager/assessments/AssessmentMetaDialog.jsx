// Edit assessment title/description (structure and effects are immutable by design).
import { useState } from 'react'
import { fieldErrors } from '../../../api/client'
import * as endpoints from '../../../api/endpoints'
import { useTranslation } from '../../../i18n'
import { errorMessage } from '../../../lib/errors'
import Alert from '../../../ui/Alert'
import Button from '../../../ui/Button'
import Input from '../../../ui/Input'
import Modal from '../../../ui/Modal'
import Textarea from '../../../ui/Textarea'
import { useToast } from '../../../ui/toastContext'

export default function AssessmentMetaDialog({ assessment, onClose, onSaved }) {
  const { t } = useTranslation()
  const toast = useToast()
  const [title, setTitle] = useState(assessment.title)
  const [description, setDescription] = useState(assessment.description || '')
  const [errors, setErrors] = useState({})
  const [formError, setFormError] = useState('')
  const [busy, setBusy] = useState(false)

  const submit = async (e) => {
    e.preventDefault()
    setFormError('')
    const next = {}
    if (!title.trim()) next.title = 'required'
    setErrors(next)
    if (next.title) return
    setBusy(true)
    try {
      await endpoints.manage.updateAssessment(assessment.id, { title: title.trim(), description })
      toast.success(t('manager.assessments.toast.meta_saved'))
      onSaved()
    } catch (err) {
      const fe = fieldErrors(err)
      if (Object.keys(fe).length) setErrors(fe)
      else setFormError(errorMessage(t, err))
    } finally {
      setBusy(false)
    }
  }

  const msg = (path) => (errors[path] ? t(`manager.assessments.errors.${errors[path]}`) : undefined)

  return (
    <Modal
      isOpen
      onClose={busy ? undefined : onClose}
      title={t('manager.assessments.meta.title')}
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={busy}>
            {t('common.cancel')}
          </Button>
          <Button type="submit" form="assessment-meta-form" loading={busy}>
            {t('common.save')}
          </Button>
        </>
      }
    >
      <form id="assessment-meta-form" onSubmit={submit} noValidate className="space-y-4">
        {formError && <Alert tone="danger">{formError}</Alert>}
        <Alert tone="info">{t('manager.assessments.meta.note')}</Alert>
        <Input
          label={t('manager.assessments.meta.name')}
          required
          maxLength={200}
          value={title}
          error={msg('title')}
          onChange={(e) => setTitle(e.target.value)}
          data-autofocus
        />
        <Textarea
          label={t('manager.assessments.meta.description')}
          maxLength={4000}
          value={description}
          error={msg('description')}
          onChange={(e) => setDescription(e.target.value)}
        />
      </form>
    </Modal>
  )
}
