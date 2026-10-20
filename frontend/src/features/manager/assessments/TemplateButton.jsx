// "Create from template": opens the existing wizard prefilled with a template (nothing is created yet).
// An unsent wizard draft would be replaced, so that case asks first.
import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '../../../auth/context'
import { useTranslation } from '../../../i18n'
import { getJSON, keys } from '../../../lib/storage'
import Button from '../../../ui/Button'
import ConfirmDialog from '../../../ui/ConfirmDialog'
import { isPristine, sanitizeDraft } from '../wizard/validate'

export const DEFAULT_TEMPLATE = 'construction-staff'

export default function TemplateButton({ template = DEFAULT_TEMPLATE }) {
  const { t } = useTranslation()
  const { me } = useAuth()
  const navigate = useNavigate()
  const [confirming, setConfirming] = useState(false)
  const go = () => navigate(`/manage/assessments/new?template=${encodeURIComponent(template)}`)
  const click = () => {
    const draft = sanitizeDraft(getJSON(keys.wizard(me?.id ?? 'anon')))
    if (draft && !isPristine(draft)) setConfirming(true)
    else go()
  }
  return (
    <>
      <Button variant="secondary" onClick={click}>
        {t('manager.assessments.from_template')}
      </Button>
      <ConfirmDialog
        isOpen={confirming}
        tone="primary"
        title={t('manager.assessments.template_replace_title')}
        message={t('manager.assessments.template_replace_msg')}
        confirmLabel={t('manager.assessments.from_template')}
        onCancel={() => setConfirming(false)}
        onConfirm={go}
      />
    </>
  )
}
