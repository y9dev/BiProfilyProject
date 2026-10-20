// Manager: invitations list (table on desktop, cards on mobile) + create + details dialog.
import { useState } from 'react'
import * as endpoints from '../../../api/endpoints'
import { useTranslation } from '../../../i18n'
import { useAsync } from '../../../lib/useAsync'
import Async from '../../../ui/Async'
import Badge from '../../../ui/Badge'
import Button from '../../../ui/Button'
import PageHeader from '../../../ui/PageHeader'
import { useToast } from '../../../ui/toastContext'
import { copyText } from '../clipboard'
import DataTable from '../DataTable'
import { formatDateTime } from '../format'
import InvitationDetailsDialog from './InvitationDetailsDialog'
import InvitationFormDialog from './InvitationFormDialog'
import { registrationUrl, remaining, statusTone } from './status'

export default function InvitationsPage() {
  const { t, lang } = useTranslation()
  const toast = useToast()
  const list = useAsync((signal) => endpoints.invitations.list({ signal }))
  const [creating, setCreating] = useState(false)
  const [open, setOpen] = useState(null)

  const items = list.data ?? []

  const copyUrl = async (inv) => {
    const ok = await copyText(registrationUrl(window.location.origin, inv.code))
    if (ok) toast.success(t('manager.invitations.toast.url_copied'))
    else toast.error(t('manager.invitations.toast.copy_failed'))
  }

  const columns = [
    {
      key: 'code',
      header: t('manager.invitations.col.code'),
      primary: true,
      className: 'w-[18%]',
      render: (i) => <code className="font-mono font-semibold tracking-wide">{i.code}</code>,
    },
    {
      key: 'status',
      header: t('manager.invitations.col.status'),
      className: 'w-[12%]',
      render: (i) => <Badge tone={statusTone(i.status)}>{t(`manager.invitations.status.${i.status}`)}</Badge>,
    },
    {
      key: 'expires',
      header: t('manager.invitations.col.expires'),
      className: 'w-[17%]',
      render: (i) => (i.expiresAt ? formatDateTime(i.expiresAt, lang) : t('manager.invitations.never')),
    },
    {
      key: 'used',
      header: t('manager.invitations.col.used'),
      className: 'w-[15%]',
      render: (i) => t('manager.invitations.used_of', { used: i.registrationsCount, max: i.maxRegistrations, left: remaining(i) }),
    },
    {
      key: 'created',
      header: t('manager.invitations.col.created'),
      className: 'w-[14%]',
      render: (i) => (
        <span className="block min-w-0">
          {formatDateTime(i.createdAt, lang)}
          {i.createdBy?.fullName && <span className="block text-caption text-muted">{i.createdBy.fullName}</span>}
        </span>
      ),
    },
    {
      key: 'actions',
      header: <span className="sr-only">{t('manager.col.actions')}</span>,
      actions: true,
      className: 'w-[24%]',
      render: (i) => (
        <>
          <Button variant="secondary" onClick={() => setOpen(i)}>
            {t('manager.invitations.open')}
          </Button>
          <Button variant="ghost" onClick={() => copyUrl(i)}>
            {t('manager.invitations.copy_link')}
          </Button>
        </>
      ),
    },
  ]

  return (
    <div className="min-w-0">
      <PageHeader
        title={t('manager.invitations.title')}
        description={t('manager.invitations.desc')}
        actions={<Button onClick={() => setCreating(true)}>{t('manager.invitations.create')}</Button>}
      />

      <Async
        status={list.status}
        onRetry={list.reload}
        isEmpty={items.length === 0}
        empty={{
          title: t('manager.invitations.empty_title'),
          description: t('manager.invitations.empty_desc'),
          action: <Button onClick={() => setCreating(true)}>{t('manager.invitations.create')}</Button>,
        }}
      >
        <DataTable
          columns={columns}
          rows={items}
          rowKey={(i) => i.id}
          caption={t('manager.invitations.title')}
        />
      </Async>

      {creating && (
        <InvitationFormDialog
          isOpen
          onClose={() => setCreating(false)}
          onCreated={(inv) => {
            setCreating(false)
            list.reload()
            setOpen(inv)
          }}
        />
      )}
      {open && (
        <InvitationDetailsDialog key={open.id} invitation={open} onClose={() => setOpen(null)} onChanged={list.reload} />
      )}
    </div>
  )
}

