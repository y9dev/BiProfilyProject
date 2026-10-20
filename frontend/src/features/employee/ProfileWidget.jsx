// Personal widget: rendered by the backend (single source of truth), embedded here, with a button
// to choose the avatar again (the avatar is part of the widget, so it is re-fetched after saving).
import { useState } from 'react'
import { useTranslation } from '../../i18n'
import Button from '../../ui/Button'
import WidgetEmbed from '../../ui/WidgetEmbed'
import EditAvatarDialog from './EditAvatarDialog'

export default function ProfileWidget({ me }) {
  const { t } = useTranslation()
  const [editing, setEditing] = useState(false)
  const [version, setVersion] = useState(0)
  return (
    <div className="mx-auto flex w-full max-w-sm min-w-0 flex-col gap-3 xl:h-full xl:max-w-none">
      <WidgetEmbed userId={me.id} version={version} />
      <Button variant="secondary" fullWidth onClick={() => setEditing(true)}>
        {t('employee.profile.edit_avatar')}
      </Button>
      {editing && (
        <EditAvatarDialog
          onClose={() => setEditing(false)}
          onSaved={() => {
            setEditing(false)
            setVersion((n) => n + 1)
          }}
        />
      )}
    </div>
  )
}
