// Manager: characteristics. Defaults are protected (no rename/delete); custom ones can be renamed and,
// while unused by any assessment, deleted. 409 default_protected / in_use / duplicate_name are explained inline.
import { useCallback, useMemo, useState } from 'react'
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

export default function CharacteristicsPage() {
  const { t, lang } = useTranslation()
  const toast = useToast()
  const list = useAsync((signal) => endpoints.characteristics.list({ signal }))
  const [name, setName] = useState('')
  const [nameErr, setNameErr] = useState('')
  const [creating, setCreating] = useState(false)
  const [renaming, setRenaming] = useState(null)
  const [deleting, setDeleting] = useState(null)
  const [busy, setBusy] = useState(false)
  const [notice, setNotice] = useState(null) // { tone, text, titles? }
  const [usage, setUsage] = useState({ status: 'idle', map: new Map() })

  const items = useMemo(() => list.data ?? [], [list.data])
  const label = (c) => c.nameI18n?.[lang] || c.name

  const create = async (e) => {
    e.preventDefault()
    setNameErr('')
    setNotice(null)
    if (!name.trim()) {
      setNameErr(t('manager.characteristics.errors.required'))
      return
    }
    setCreating(true)
    try {
      await endpoints.characteristics.create(name.trim())
      toast.success(t('manager.characteristics.toast.created'))
      setName('')
      setUsage({ status: 'idle', map: new Map() })
      list.reload()
    } catch (err) {
      const fe = fieldErrors(err)
      if (err.code === 'duplicate_name') setNameErr(t('manager.characteristics.errors.duplicate_name'))
      else if (fe.name) setNameErr(t(`manager.characteristics.errors.${fe.name}`))
      else setNotice({ tone: 'danger', text: errorMessage(t, err) })
    } finally {
      setCreating(false)
    }
  }

  // Which assessments use which characteristic: loaded on demand (reads every assessment definition once).
  const loadUsage = useCallback(async () => {
    setUsage((u) => ({ ...u, status: 'loading' }))
    try {
      const all = await endpoints.manage.assessments()
      const defs = await Promise.all(all.map((a) => endpoints.manage.assessment(a.id)))
      const map = new Map()
      for (const d of defs) {
        const seen = new Set()
        for (const q of d.questions) for (const o of q.options) for (const ef of o.effects) seen.add(ef.characteristicId)
        for (const id of seen) map.set(id, [...(map.get(id) ?? []), d.title])
      }
      setUsage({ status: 'ready', map })
    } catch {
      setUsage({ status: 'error', map: new Map() })
    }
  }, [])

  const confirmDelete = async () => {
    const c = deleting
    setBusy(true)
    try {
      await endpoints.characteristics.remove(c.id)
      toast.success(t('manager.characteristics.toast.deleted'))
      setDeleting(null)
      list.reload()
    } catch (err) {
      setDeleting(null)
      if (err.code === 'in_use' || err.code === 'default_protected') {
        let titles = []
        if (err.code === 'in_use' && Array.isArray(err.errors)) {
          try {
            const all = await endpoints.manage.assessments()
            const byId = new Map(all.map((a) => [String(a.id), a.title]))
            // The 409 message lists the titles in errors[] order ("; "-joined). Archived
            // assessments are absent from the manage list, so their title comes from there.
            const serverTitles = err.message && err.message !== err.code ? err.message.split('; ') : []
            const aligned = serverTitles.length === err.errors.length
            titles = err.errors
              .map((x, i) => [/assessments\[(\d+)\]/.exec(x.path ?? '')?.[1], i])
              .filter(([id]) => id)
              .map(([id, i]) =>
                byId.get(id) ??
                (aligned && serverTitles[i]
                  ? t('manager.characteristics.archived_title', { title: serverTitles[i] })
                  : t('manager.characteristics.archived_unknown')),
              )
          } catch {
            titles = []
          }
        }
        setNotice({ tone: 'warning', text: t(`manager.characteristics.errors.${err.code}`, { name: label(c) }), titles })
        list.reload()
      } else {
        toast.error(errorMessage(t, err))
      }
    } finally {
      setBusy(false)
    }
  }

  const columns = [
    {
      key: 'name',
      header: t('manager.characteristics.col.name'),
      primary: true,
      className: 'w-[28%]',
      render: (c) => label(c),
    },
    {
      key: 'kind',
      header: t('manager.characteristics.col.kind'),
      className: 'w-[16%]',
      render: (c) => (
        <Badge tone={c.isDefault ? 'info' : 'neutral'}>
          {t(c.isDefault ? 'manager.characteristics.default' : 'manager.characteristics.custom')}
        </Badge>
      ),
    },
    {
      key: 'usage',
      header: t('manager.characteristics.col.usage'),
      className: 'w-[26%]',
      render: (c) => {
        const titles = usage.map.get(c.id)
        return (
          <span className="block min-w-0 space-y-1">
            <Badge tone={c.inUse ? 'success' : 'neutral'}>
              {t(c.inUse ? 'manager.characteristics.in_use' : 'manager.characteristics.unused')}
            </Badge>
            {titles && <span className="block text-caption text-muted">{titles.join(', ')}</span>}
            {!titles && c.inUse && usage.status === 'ready' && (
              <span className="block text-caption text-muted">{t('manager.characteristics.archived_only')}</span>
            )}
          </span>
        )
      },
    },
    {
      key: 'actions',
      header: <span className="sr-only">{t('manager.col.actions')}</span>,
      actions: true,
      className: 'w-[28%]',
      render: (c) =>
        c.isDefault ? (
          <span className="text-caption text-muted">{t('manager.characteristics.protected')}</span>
        ) : (
          <>
            <Button variant="secondary" onClick={() => setRenaming(c)}>
              {t('manager.characteristics.rename')}
            </Button>
            {c.inUse ? (
              <span className="min-w-0 self-center text-caption text-muted">{t('manager.characteristics.delete_blocked')}</span>
            ) : (
              <Button variant="danger-outline" onClick={() => setDeleting(c)}>
                {t('common.delete')}
              </Button>
            )}
          </>
        ),
    },
  ]

  return (
    <div className="min-w-0">
      <PageHeader title={t('manager.characteristics.title')} description={t('manager.characteristics.desc')} />

      <Card className="mb-6">
        <form onSubmit={create} noValidate className="flex flex-wrap items-start gap-3">
          <Input
            className="min-w-0 flex-1 basis-64"
            label={t('manager.characteristics.new_label')}
            maxLength={120}
            value={name}
            error={nameErr || undefined}
            onChange={(e) => setName(e.target.value)}
          />
          <Button type="submit" loading={creating} className="sm:mt-6">
            {t('manager.characteristics.add')}
          </Button>
        </form>
      </Card>

      {notice && (
        <Alert tone={notice.tone} className="mb-4">
          <p>{notice.text}</p>
          {notice.titles?.length > 0 && (
            <ul className="mt-1 list-disc pl-5">
              {notice.titles.map((x) => (
                <li key={x} className="break-words">
                  {x}
                </li>
              ))}
            </ul>
          )}
        </Alert>
      )}

      <Async
        status={list.status}
        onRetry={list.reload}
        isEmpty={items.length === 0}
        empty={{ title: t('manager.characteristics.empty_title'), description: t('manager.characteristics.empty_desc') }}
      >
        {items.some((c) => c.inUse) && usage.status !== 'ready' && (
          <div className="mb-3 flex flex-wrap items-center gap-3">
            <Button variant="secondary" onClick={loadUsage} loading={usage.status === 'loading'}>
              {t('manager.characteristics.show_usage')}
            </Button>
            {usage.status === 'error' && <span className="text-label text-danger">{t('errors.generic')}</span>}
          </div>
        )}
        <DataTable columns={columns} rows={items} rowKey={(c) => c.id} caption={t('manager.characteristics.title')} />
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
        isOpen={!!deleting}
        onCancel={() => setDeleting(null)}
        onConfirm={confirmDelete}
        loading={busy}
        title={t('manager.characteristics.confirm_title')}
        message={t('manager.characteristics.confirm_msg', { name: deleting ? label(deleting) : '' })}
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
      setErr(t('manager.characteristics.errors.required'))
      return
    }
    setBusy(true)
    try {
      await endpoints.characteristics.rename(item.id, name.trim())
      toast.success(t('manager.characteristics.toast.renamed'))
      onSaved()
    } catch (e2) {
      if (e2.code === 'duplicate_name') setErr(t('manager.characteristics.errors.duplicate_name'))
      else if (e2.code === 'default_protected') setErr(t('manager.characteristics.errors.default_protected', { name: item.name }))
      else setErr(errorMessage(t, e2))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal
      isOpen
      onClose={busy ? undefined : onClose}
      title={t('manager.characteristics.rename')}
      size="sm"
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={busy}>
            {t('common.cancel')}
          </Button>
          <Button type="submit" form="char-rename-form" loading={busy}>
            {t('common.save')}
          </Button>
        </>
      }
    >
      <form id="char-rename-form" onSubmit={submit} noValidate>
        <Input
          label={t('manager.characteristics.col.name')}
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
