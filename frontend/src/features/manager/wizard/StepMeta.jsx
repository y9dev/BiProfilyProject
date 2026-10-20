// Step 1: title, short description, visibility, expiry.
import { useState } from 'react'
import { useTranslation } from '../../../i18n'
import Input from '../../../ui/Input'
import Textarea from '../../../ui/Textarea'
import { LIMITS } from './validate'
import { RadioCards } from './parts'
import { errText } from './helpers'

function todayISO(plusDays = 0) {
  const d = new Date()
  d.setDate(d.getDate() + plusDays)
  const p = (n) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}`
}

export default function StepMeta({ state, update, errors }) {
  const { t } = useTranslation()
  const [dated, setDated] = useState(!!state.expires)

  return (
    <div className="space-y-6">
      <Input
        label={t('wizard.meta.title_label')}
        required
        value={state.title}
        maxLength={LIMITS.title + 50}
        placeholder={t('wizard.meta.title_ph')}
        onChange={(e) => update((d) => void (d.title = e.target.value))}
        error={errText(t, errors.title)}
        autoComplete="off"
      />
      <Textarea
        label={t('wizard.meta.description_label')}
        optionalLabel={t('common.optional')}
        hint={t('wizard.meta.description_hint')}
        rows={3}
        value={state.description}
        onChange={(e) => update((d) => void (d.description = e.target.value))}
        error={errText(t, errors.description)}
      />
      <RadioCards
        legend={t('wizard.meta.visibility_legend')}
        value={state.hidden}
        onChange={(v) => update((d) => void (d.hidden = v))}
        options={[
          { value: false, label: t('wizard.meta.visible'), hint: t('wizard.meta.visible_hint') },
          { value: true, label: t('wizard.meta.hidden'), hint: t('wizard.meta.hidden_hint') },
        ]}
      />
      <div className="space-y-3">
        <RadioCards
          legend={t('wizard.meta.expiry_legend')}
          value={dated}
          onChange={(v) => {
            setDated(v)
            if (!v) update((d) => void (d.expires = ''))
            else if (!state.expires) update((d) => void (d.expires = todayISO(30)))
          }}
          options={[
            { value: false, label: t('wizard.meta.expiry_none'), hint: t('wizard.meta.expiry_none_hint') },
            { value: true, label: t('wizard.meta.expiry_date'), hint: t('wizard.meta.expiry_date_hint') },
          ]}
        />
        {dated && (
          <Input
            type="date"
            label={t('wizard.meta.expiry_date_label')}
            required
            min={todayISO()}
            value={state.expires}
            onChange={(e) => update((d) => void (d.expires = e.target.value))}
            error={errText(t, errors.expiresAt)}
            className="max-w-xs"
          />
        )}
      </div>
    </div>
  )
}
