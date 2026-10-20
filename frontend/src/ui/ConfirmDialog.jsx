// Confirmation for destructive (default) or consequential actions.
// Cancel is auto-focused, so Enter never destroys by accident.
import { useTranslation } from '../i18n'
import Button from './Button'
import Modal from './Modal'

export default function ConfirmDialog({
  isOpen,
  onConfirm,
  onCancel,
  title,
  message,
  confirmLabel,
  cancelLabel,
  tone = 'danger',
  loading = false,
  children,
}) {
  const { t } = useTranslation()
  return (
    <Modal
      isOpen={isOpen}
      onClose={loading ? undefined : onCancel}
      title={title || t('ui.confirm_title')}
      size="sm"
      footer={
        <>
          <Button variant="secondary" onClick={onCancel} disabled={loading} data-autofocus>
            {cancelLabel || t('common.cancel')}
          </Button>
          <Button variant={tone === 'danger' ? 'danger' : 'primary'} onClick={onConfirm} loading={loading}>
            {confirmLabel || t('ui.confirm_delete')}
          </Button>
        </>
      }
    >
      {message && <p className="text-body text-ink">{message}</p>}
      {children}
    </Modal>
  )
}
