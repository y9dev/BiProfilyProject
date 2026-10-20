// Manager: departments. Used departments are deactivated (never lost: employees keep them); unused ones are deleted.
import { useMemo, useState } from 'react'
import { fieldErrors } from '../../../api/client'
import * as endpoints from '../../../api/endpoints'
import { useTranslation } from '../../../i18n'
import { errorMessage } from '../../../lib/errors'
import { useAsync } from '../../../lib/useAsync'
import Alert from '../../../ui/Alert'
import Async from '../../../ui/Async'
import Badge from '../../../ui/Badge'
import Button from '../../../ui/Button'
import Card from '../../../ui/Card'
import ConfirmDialog from '../../../ui/ConfirmDialog'
import Input from '../../../ui/Input'
import Modal from '../../../ui/Modal'
import PageHeader from '../../../ui/PageHeader'
import { useToast } from '../../../ui/toastContext'
import DataTable from '../DataTable'

export default function DepartmentsPage() {
  const { t } = useTranslation()
  const toast = useToast()
  const list = useAsync((signal) => endpoints.departments.list(true, { signal }))
  const [name, setName] = useState('')
  const [nameErr, setNameErr] = useState('')
  const [creating, setCreating] = useState(false)
  const [renaming, setRenaming] = useState(null)
  const [removing, setRemoving] = useState(null)
  const [busy, setBusy] = useState(false)
  const [busyId, setBusyId] = useState(null)
  const [notice, setNotice] = useState(null)

  const items = useMemo(() => list.data ?? [], [list.data])

  const create = async (e) => {
    e.preventDefault()
    setNameErr('')
    setNotice(null)
    if (!name.trim()) {
      setNameErr(t('manager.departments.errors.required'))
      return
    }
    setCreating(true)
    try {
      await endpoints.departments.create(name.trim())
      toast.success(t('manager.departments.toast.created'))
      setName('')
      list.reload()
    } catch (err) {
      const fe = fieldErrors(err)
      if (err.code === 'duplicate_name') setNameErr(t('manager.departments.errors.duplicate_name'))
      else if (fe.name) setNameErr(t(`manager.departments.errors.${fe.name}`))
      else setNotice({ tone: 'danger', text: errorMessage(t, err) })
    } finally {
      setCreating(false)
    }
  }

  const activate = async (d) => {
    setBusyId(d.id)
    setNotice(null)
    try {
      await endpoints.departments.activate(d.id)
      toast.success(t('manager.departments.toast.activated'))
      list.reload()
    } catch (err) {
      if (err.code === 'duplicate_name') {
        setNotice({ tone: 'warning', text: t('manager.departments.errors.activate_clash', { name: d.name }) })
      } else {
        toast.error(errorMessage(t, err))
      }
    } finally {
      setBusyId(null)
    }
  }

  const confirmRemove = async () => {
    const d = removing
    setBusy(true)
    try {
      const res = await endpoints.departments.remove(d.id)
      toast.success(
        t(res?.deactivated ? 'manager.departments.toast.deactivated' : 'manager.departments.toast.deleted', { name: d.name }),
      )
      setRemoving(null)
      list.reload()
    } catch (err) {
      setRemoving(null)
      toast.error(errorMessage(t, err))
    } finally {
      setBusy(false)
    }
  }

  const columns = [
    { key: 'name', header: t('manager.departments.col.name'), primary: true, className: 'w-[36%]', render: (d) => d.name },
    {
      key: 'status',
      header: t('manager.departments.col.status'),
      className: 'w-[18%]',
      render: (d) => (
        <Badge tone={d.active ? 'success' : 'neutral'}>
          {t(d.active ? 'manager.departments.active' : 'manager.departments.inactive')}
        </Badge>
      ),
    },
    { key: 'users', header: t('manager.departments.col.users'), className: 'w-[14%]', render: (d) => d.userCount },
    {
      key: 'actions',
      header: <span className="sr-only">{t('manager.col.actions')}</span>,
      actions: true,
      className: 'w-[34%]',
      render: (d) => (
        <>
          <Button variant="secondary" onClick={() => setRenaming(d)}>
            {t('manager.departments.rename')}
          </Button>
          {!d.active && (
            <Button variant="secondary" loading={busyId === d.id} onClick={() => activate(d)}>
              {t('manager.departments.activate')}
            </Button>
          )}
          {d.active && (
            <Button variant="danger-outline" onClick={() => setRemoving(d)}>
              {t(d.userCount > 0 ? 'manager.departments.deactivate' : 'common.delete')}
            </Button>
          )}
          {!d.active && d.userCount === 0 && (
            <Button variant="danger-outline" onClick={() => setRemoving(d)}>
              {t('common.delete')}
            </Button>
          )}
        </>
      ),
    },
  ]

  return (
    <div className="min-w-0">
      <PageHeader title={t('manager.departments.title')} description={t('manager.departments.desc')} />

      <Card className="mb-6">
        <form onSubmit={create} noValidate className="flex flex-wrap items-start gap-3">
          <Input
            className="min-w-0 flex-1 basis-64"
            label={t('manager.departments.new_label')}
            maxLength={120}
            value={name}
            error={nameErr || undefined}
            onChange={(e) => setName(e.target.value)}
          />
          <Button type="submit" loading={creating} className="sm:mt-6">
            {t('manager.departments.add')}
          </Button>
        </form>
      </Card>

      {notice && (
        <Alert tone={notice.tone} className="mb-4">
          {notice.text}
        </Alert>
      )}
      <Alert tone="info" className="mb-4">
        {t('manager.departments.keep_note')}
      </Alert>

      <Async
        status={list.status}
        onRetry={list.reload}
        isEmpty={items.length === 0}
        empty={{ title: t('manager.departments.empty_title'), description: t('manager.departments.empty_desc') }}
      >
        <DataTable
          columns={columns}
          rows={items}
          rowKey={(d) => d.id}
          caption={t('manager.departments.title')}
          rowClassName={(d) => (d.active ? '' : 'bg-surface-2/50')}
        />
      </Async>

      {renaming && (
        <RenameDialog
          item={renaming}
          onClose={() => setRenaming(null)}
          onSaved={() => {
            setRenaming(null)
            list.reload()
          }}
        />
      )}
      <ConfirmDialog
        isOpen={!!removing}
        onCancel={() => setRemoving(null)}
        onConfirm={confirmRemove}
        loading={busy}
        tone={removing?.userCount > 0 ? 'primary' : 'danger'}
        title={t(removing?.userCount > 0 ? 'manager.departments.confirm_deactivate_title' : 'manager.departments.confirm_delete_title')}
        message={
          removing
            ? t(removing.userCount > 0 ? 'manager.departments.confirm_deactivate_msg' : 'manager.departments.confirm_delete_msg', {
                name: removing.name,
                count: removing.userCount,
              })
            : ''
        }
        confirmLabel={t(removing?.userCount > 0 ? 'manager.departments.deactivate' : 'common.delete')}
      />
    </div>
  )
}

function RenameDialog({ item, onClose, onSaved }) {
  const { t } = useTranslation()
  const toast = useToast()
  const [name, setName] = useState(item.name)
  const [err, setErr] = useState('')
  const [busy, setBusy] = useState(false)

  const submit = async (e) => {
    e.preventDefault()
    if (!name.trim()) {
      setErr(t('manager.departments.errors.required'))
      return
    }
    setBusy(true)
    try {
      await endpoints.departments.rename(item.id, name.trim())
      toast.success(t('manager.departments.toast.renamed'))
      onSaved()
    } catch (e2) {
      setErr(e2.code === 'duplicate_name' ? t('manager.departments.errors.duplicate_name') : errorMessage(t, e2))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal
      isOpen
      onClose={busy ? undefined : onClose}
      title={t('manager.departments.rename')}
      size="sm"
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={busy}>
            {t('common.cancel')}
          </Button>
          <Button type="submit" form="dep-rename-form" loading={busy}>
            {t('common.save')}
          </Button>
        </>
      }
    >
      <form id="dep-rename-form" onSubmit={submit} noValidate>
        <Input
          label={t('manager.departments.col.name')}
          maxLength={120}
          value={name}
          error={err || undefined}
          onChange={(e) => setName(e.target.value)}
          data-autofocus
        />
      </form>
    </Modal>
  )
}
