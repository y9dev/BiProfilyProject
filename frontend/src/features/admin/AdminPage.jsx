// Admin: user directory with role change, block/unblock and password reset (admin-only route).
// Role change, block and password reset revoke the user's sessions on the server.
import { useCallback, useEffect, useRef, useState } from 'react'
import { fieldErrors } from '../../api/client'
import * as endpoints from '../../api/endpoints'
import { useAuth } from '../../auth/context'
import { useTranslation } from '../../i18n'
import { errorMessage } from '../../lib/errors'
import Alert from '../../ui/Alert'
import Badge from '../../ui/Badge'
import Button from '../../ui/Button'
import ConfirmDialog from '../../ui/ConfirmDialog'
import EmptyState from '../../ui/EmptyState'
import ErrorState from '../../ui/ErrorState'
import Input from '../../ui/Input'
import Modal from '../../ui/Modal'
import PageHeader from '../../ui/PageHeader'
import Select from '../../ui/Select'
import Skeleton from '../../ui/Skeleton'
import { useToast } from '../../ui/toastContext'
import DataTable from '../manager/DataTable'
import { formatDate } from '../manager/format'

const PAGE = 50
const ROLE_TONE = { admin: 'danger', manager: 'info', employee: 'neutral' }

export default function AdminPage() {
  const { t, lang } = useTranslation()
  const { me } = useAuth()
  const [search, setSearch] = useState('')
  const [query, setQuery] = useState('')
  const [role, setRole] = useState('')
  const [status, setStatus] = useState('')
  const [state, setState] = useState({ status: 'loading', items: [], total: 0 })
  const [loadingMore, setLoadingMore] = useState(false)
  const [dialog, setDialog] = useState(null) // { kind: 'role'|'block'|'unblock'|'password', user }
  const [notice, setNotice] = useState(null)
  const reqSeq = useRef(0)

  // Debounce the free-text search so typing does not fire a request per keystroke.
  useEffect(() => {
    const id = setTimeout(() => setQuery(search.trim()), 300)
    return () => clearTimeout(id)
  }, [search])

  const load = useCallback(
    async (offset, append) => {
      const seq = ++reqSeq.current
      if (!append) setState((s) => ({ ...s, status: 'loading' }))
      try {
        const res = await endpoints.admin.users({ search: query, role, status, limit: PAGE, offset })
        if (seq !== reqSeq.current) return
        setState((s) => ({
          status: 'ready',
          items: append ? [...s.items, ...res.items] : res.items,
          total: res.total,
        }))
      } catch {
        if (seq === reqSeq.current) setState((s) => ({ ...s, status: 'error' }))
      }
    },
    [query, role, status],
  )

  useEffect(() => {
    load(0, false)
  }, [load])

  const reload = () => load(0, false)
  const loadMore = async () => {
    setLoadingMore(true)
    await load(state.items.length, true)
    setLoadingMore(false)
  }

  const guardMessage = (err, user) => {
    if (['self_change', 'self_block', 'last_admin'].includes(err.code)) {
      return t(`admin.errors.${err.code}`, { name: user.fullName })
    }
    return errorMessage(t, err)
  }

  const close = () => setDialog(null)

  const columns = [
    {
      key: 'name',
      header: t('admin.col.user'),
      primary: true,
      className: 'w-[20%]',
      render: (u) => (
        <span className="block min-w-0">
          {u.fullName}
          {u.id === me?.id && <span className="font-normal text-muted"> ({t('admin.you')})</span>}
          <span className="block text-caption font-normal text-muted">@{u.login}</span>
        </span>
      ),
    },
    {
      key: 'job',
      header: t('admin.col.job'),
      className: 'w-[20%]',
      render: (u) => (
        <span className="block min-w-0">
          {u.jobTitle || '—'}
          <span className="block text-caption text-muted">{u.department?.name || '—'}</span>
        </span>
      ),
    },
    {
      key: 'role',
      header: t('admin.col.role'),
      className: 'w-[12%]',
      render: (u) => <Badge tone={ROLE_TONE[u.role]}>{t(`roles.${u.role}`)}</Badge>,
    },
    {
      key: 'status',
      header: t('admin.col.status'),
      className: 'w-[12%]',
      render: (u) => (
        <Badge tone={u.status === 'blocked' ? 'danger' : 'success'}>{t(`admin.status.${u.status}`)}</Badge>
      ),
    },
    { key: 'created', header: t('admin.col.created'), className: 'w-[12%]', render: (u) => formatDate(u.createdAt, lang) },
    {
      key: 'actions',
      header: <span className="sr-only">{t('manager.col.actions')}</span>,
      actions: true,
      className: 'w-[22%]',
      render: (u) => {
        return (
          <div className="flex flex-wrap gap-2">
            <Button variant="secondary" onClick={() => setDialog({ kind: 'role', user: u })}>
              {t('admin.change_role')}
            </Button>
            <Button variant="secondary" onClick={() => setDialog({ kind: 'password', user: u })}>
              {t('admin.reset_password')}
            </Button>
            {u.status === 'blocked' ? (
              <Button variant="secondary" onClick={() => setDialog({ kind: 'unblock', user: u })}>
                {t('admin.unblock')}
              </Button>
            ) : (
              <Button variant="danger-outline" onClick={() => setDialog({ kind: 'block', user: u })}>
                {t('admin.block')}
              </Button>
            )}
          </div>
        )
      },
    },
  ]

  return (
    <div className="min-w-0">
      <PageHeader title={t('admin.title')} description={t('admin.desc')} />

      <div className="mb-4 grid min-w-0 grid-cols-1 gap-3 sm:grid-cols-[minmax(0,2fr)_minmax(0,1fr)_minmax(0,1fr)]">
        <Input
          type="search"
          label={t('admin.search')}
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
        <Select label={t('admin.filter_role')} value={role} onChange={(e) => setRole(e.target.value)}>
          <option value="">{t('admin.all')}</option>
          {['admin', 'manager', 'employee'].map((r) => (
            <option key={r} value={r}>
              {t(`roles.${r}`)}
            </option>
          ))}
        </Select>
        <Select label={t('admin.filter_status')} value={status} onChange={(e) => setStatus(e.target.value)}>
          <option value="">{t('admin.all')}</option>
          {['active', 'blocked'].map((s) => (
            <option key={s} value={s}>
              {t(`admin.status.${s}`)}
            </option>
          ))}
        </Select>
      </div>

      {notice && (
        <Alert tone={notice.tone} className="mb-4">
          {notice.text}
        </Alert>
      )}

      {state.status === 'loading' && state.items.length === 0 && (
        <div className="space-y-3" aria-busy="true">
          <Skeleton className="h-16 w-full" />
          <Skeleton className="h-16 w-full" />
          <Skeleton className="h-16 w-full" />
        </div>
      )}
      {state.status === 'error' && <ErrorState onRetry={reload} />}
      {state.status === 'ready' && state.items.length === 0 && (
        <EmptyState title={t('admin.empty_title')} description={t('admin.empty_desc')} />
      )}
      {state.items.length > 0 && state.status !== 'error' && (
        <>
          <DataTable columns={columns} rows={state.items} rowKey={(u) => u.id} caption={t('admin.title')} />
          <div className="mt-4 flex flex-wrap items-center gap-3">
            <p className="text-caption text-muted">{t('admin.shown', { shown: state.items.length, total: state.total })}</p>
            {state.items.length < state.total && (
              <Button variant="secondary" onClick={loadMore} loading={loadingMore}>
                {t('admin.more')}
              </Button>
            )}
          </div>
        </>
      )}

      {dialog?.kind === 'role' && (
        <RoleDialog
          user={dialog.user}
          guardMessage={guardMessage}
          onClose={close}
          onSaved={() => {
            close()
            setNotice(null)
            reload()
          }}
        />
      )}
      {dialog?.kind === 'password' && <PasswordDialog user={dialog.user} onClose={close} />}
      {(dialog?.kind === 'block' || dialog?.kind === 'unblock') && (
        <BlockConfirm
          kind={dialog.kind}
          user={dialog.user}
          onClose={close}
          onDone={(err) => {
            close()
            if (err) setNotice({ tone: 'warning', text: guardMessage(err, dialog.user) })
            else {
              setNotice(null)
              reload()
            }
          }}
        />
      )}
    </div>
  )
}

function RoleDialog({ user, guardMessage, onClose, onSaved }) {
  const { t } = useTranslation()
  const toast = useToast()
  const [role, setRole] = useState(user.role === 'manager' ? 'employee' : 'manager')
  const [err, setErr] = useState('')
  const [busy, setBusy] = useState(false)

  const save = async () => {
    setBusy(true)
    setErr('')
    try {
      await endpoints.admin.setRole(user.id, role)
      toast.success(t('admin.toast.role_changed', { name: user.fullName, role: t(`roles.${role}`) }))
      onSaved()
    } catch (e) {
      setErr(guardMessage(e, user))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal
      isOpen
      onClose={busy ? undefined : onClose}
      title={t('admin.role_title', { name: user.fullName })}
      size="sm"
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={busy}>
            {t('common.cancel')}
          </Button>
          <Button onClick={save} loading={busy} disabled={role === user.role}>
            {t('common.save')}
          </Button>
        </>
      }
    >
      <div className="space-y-4">
        {err && <Alert tone="danger">{err}</Alert>}
        <p className="text-label text-muted">
          {t('admin.role_current', { role: t(`roles.${user.role}`) })}
        </p>
        <Select label={t('admin.new_role')} value={role} onChange={(e) => setRole(e.target.value)} data-autofocus>
          <option value="manager">{t('roles.manager')}</option>
          <option value="employee">{t('roles.employee')}</option>
        </Select>
        <Alert tone="info">{t('admin.sessions_note')}</Alert>
      </div>
    </Modal>
  )
}

function PasswordDialog({ user, onClose }) {
  const { t } = useTranslation()
  const toast = useToast()
  const [password, setPassword] = useState('')
  const [err, setErr] = useState('')
  const [busy, setBusy] = useState(false)

  const submit = async (e) => {
    e.preventDefault()
    if (password.length < 8) {
      setErr(t('admin.errors.invalid_password'))
      return
    }
    setBusy(true)
    setErr('')
    try {
      await endpoints.admin.resetPassword(user.id, password)
      toast.success(t('admin.toast.password_reset', { name: user.fullName }))
      onClose()
    } catch (e2) {
      const fe = fieldErrors(e2)
      setErr(fe.password ? t('admin.errors.invalid_password') : errorMessage(t, e2))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal
      isOpen
      onClose={busy ? undefined : onClose}
      title={t('admin.password_title', { name: user.fullName })}
      size="sm"
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={busy}>
            {t('common.cancel')}
          </Button>
          <Button type="submit" form="reset-password-form" loading={busy}>
            {t('admin.reset_password')}
          </Button>
        </>
      }
    >
      <form id="reset-password-form" onSubmit={submit} noValidate className="space-y-4">
        <Input
          type="password"
          autoComplete="new-password"
          label={t('admin.new_password')}
          hint={t('admin.password_hint')}
          error={err || undefined}
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          data-autofocus
        />
        <Alert tone="info">{t('admin.sessions_note')}</Alert>
      </form>
    </Modal>
  )
}

function BlockConfirm({ kind, user, onClose, onDone }) {
  const { t } = useTranslation()
  const toast = useToast()
  const [busy, setBusy] = useState(false)

  const run = async () => {
    setBusy(true)
    try {
      if (kind === 'block') await endpoints.admin.block(user.id)
      else await endpoints.admin.unblock(user.id)
      toast.success(t(kind === 'block' ? 'admin.toast.blocked' : 'admin.toast.unblocked', { name: user.fullName }))
      onDone(null)
    } catch (e) {
      onDone(e)
    } finally {
      setBusy(false)
    }
  }

  return (
    <ConfirmDialog
      isOpen
      onCancel={onClose}
      onConfirm={run}
      loading={busy}
      tone={kind === 'block' ? 'danger' : 'primary'}
      title={t(kind === 'block' ? 'admin.block_title' : 'admin.unblock_title', { name: user.fullName })}
      message={t(kind === 'block' ? 'admin.block_msg' : 'admin.unblock_msg', { name: user.fullName })}
      confirmLabel={t(kind === 'block' ? 'admin.block' : 'admin.unblock')}
    />
  )
}
