// Invitation details: code/URL/QR, usage counters, lifecycle actions and the paginated registrants list.
import { QRCodeSVG } from 'qrcode.react'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { fieldErrors } from '../../../api/client'
import * as endpoints from '../../../api/endpoints'
import { useTranslation } from '../../../i18n'
import { errorMessage } from '../../../lib/errors'
import Alert from '../../../ui/Alert'
import Badge from '../../../ui/Badge'
import Button from '../../../ui/Button'
import ConfirmDialog from '../../../ui/ConfirmDialog'
import Input from '../../../ui/Input'
import Modal from '../../../ui/Modal'
import Spinner from '../../../ui/Spinner'
import { useToast } from '../../../ui/toastContext'
import { copyText } from '../clipboard'
import ExpiryField from '../ExpiryField'
import { formatDateTime } from '../format'
import {
  defaultLocal,
  fromLocalInput,
  invitationActions,
  isPast,
  registrationUrl,
  remaining,
  statusTone,
  toLocalInput,
  validateMax,
} from './status'

const PAGE = 100

function Stat({ label, value }) {
  return (
    <div className="min-w-0 rounded-ctl bg-surface-2 p-3">
      <dt className="text-caption text-muted break-words">{label}</dt>
      <dd className="mt-1 text-title font-semibold text-ink break-words">{value}</dd>
    </div>
  )
}

export default function InvitationDetailsDialog({ invitation, onClose, onChanged }) {
  const { t, lang } = useTranslation()
  const toast = useToast()
  const [inv, setInv] = useState(invitation)
  const [expiry, setExpiry] = useState(() =>
    invitation.expiresAt && !isPast(invitation.expiresAt) ? toLocalInput(invitation.expiresAt) : defaultLocal(),
  )
  const [expiryErr, setExpiryErr] = useState('')
  const [max, setMax] = useState(String(invitation.maxRegistrations))
  const [maxErr, setMaxErr] = useState('')
  const [busy, setBusy] = useState('')
  const [confirm, setConfirm] = useState(null) // 'revoke' | 'delete'
  const [actionErr, setActionErr] = useState('')

  const origin = typeof window !== 'undefined' ? window.location.origin : ''
  const url = registrationUrl(origin, inv.code)
  const actions = invitationActions(inv)

  const apply = (next) => {
    setInv(next)
    setMax(String(next.maxRegistrations))
    onChanged()
  }

  const copy = async (text, key) => {
    const ok = await copyText(text)
    if (ok) toast.success(t(key))
    else toast.error(t('manager.invitations.toast.copy_failed'))
  }

  const saveExpiry = async () => {
    setExpiryErr('')
    setActionErr('')
    const iso = fromLocalInput(expiry)
    if (iso === undefined || (iso && isPast(iso))) {
      setExpiryErr(t('manager.invitations.errors.past_expiry'))
      return
    }
    setBusy('expiry')
    try {
      const wasExpired = inv.status === 'expired'
      apply(await endpoints.invitations.update(inv.id, { expiresAt: iso }))
      toast.success(t(wasExpired ? 'manager.invitations.toast.reactivated' : 'manager.invitations.toast.expiry_saved'))
    } catch (err) {
      const fe = fieldErrors(err)
      if (fe.expiresAt) setExpiryErr(t(`manager.invitations.errors.${fe.expiresAt}`))
      else setActionErr(errorMessage(t, err))
    } finally {
      setBusy('')
    }
  }

  const saveMax = async () => {
    setMaxErr('')
    setActionErr('')
    const bad = validateMax(max, inv.registrationsCount)
    if (bad) {
      setMaxErr(t(`manager.invitations.errors.${bad}`, { count: inv.registrationsCount }))
      return
    }
    setBusy('max')
    try {
      apply(await endpoints.invitations.update(inv.id, { maxRegistrations: Number(max) }))
      toast.success(t('manager.invitations.toast.max_saved'))
    } catch (err) {
      if (err.code === 'max_below_count') {
        // The count may have moved since the dialog opened: refresh it so the message is accurate.
        let count = inv.registrationsCount
        try {
          const fresh = (await endpoints.invitations.list()).find((x) => x.id === inv.id)
          if (fresh) {
            count = fresh.registrationsCount
            setInv(fresh)
            onChanged()
          }
        } catch {
          // keep the last known count
        }
        setMaxErr(t('manager.invitations.errors.below_count', { count }))
      } else if (fieldErrors(err).maxRegistrations) {
        setMaxErr(t('manager.invitations.errors.out_of_range'))
      } else {
        setActionErr(errorMessage(t, err))
      }
    } finally {
      setBusy('')
    }
  }

  const doConfirmed = async () => {
    const kind = confirm
    setBusy(kind)
    setActionErr('')
    try {
      if (kind === 'revoke') {
        apply(await endpoints.invitations.revoke(inv.id))
        toast.success(t('manager.invitations.toast.revoked'))
        setConfirm(null)
      } else {
        await endpoints.invitations.remove(inv.id)
        toast.success(t('manager.invitations.toast.deleted'))
        onChanged()
        onClose()
      }
    } catch (err) {
      setConfirm(null)
      setActionErr(
        err.code === 'has_registrations' ? t('manager.invitations.errors.has_registrations') : errorMessage(t, err),
      )
    } finally {
      setBusy('')
    }
  }

  return (
    <>
      <Modal isOpen onClose={confirm ? undefined : onClose} title={t('manager.invitations.details.title')} size="lg">
        <div className="space-y-6">
          {actionErr && <Alert tone="danger">{actionErr}</Alert>}

          <div className="grid min-w-0 gap-6 md:grid-cols-[minmax(0,1fr)_auto]">
            <div className="min-w-0 space-y-4">
              <div className="min-w-0">
                <p className="text-caption text-muted">{t('manager.invitations.col.code')}</p>
                <div className="mt-1 flex flex-wrap items-center gap-2">
                  <code className="min-w-0 break-all rounded-ctl bg-surface-2 px-3 py-2 font-mono text-title font-semibold tracking-wider text-ink">
                    {inv.code}
                  </code>
                  <Badge tone={statusTone(inv.status)}>{t(`manager.invitations.status.${inv.status}`)}</Badge>
                </div>
              </div>
              <div className="min-w-0">
                <p className="text-caption text-muted">{t('manager.invitations.details.url')}</p>
                <p
                  className="mt-1 min-w-0 break-all rounded-ctl bg-surface-2 px-3 py-2 text-label text-ink"
                  data-testid="reg-url"
                >
                  {url}
                </p>
              </div>
              <div className="flex flex-wrap gap-2">
                <Button variant="secondary" onClick={() => copy(inv.code, 'manager.invitations.toast.code_copied')}>
                  {t('manager.invitations.details.copy_code')}
                </Button>
                <Button variant="secondary" onClick={() => copy(url, 'manager.invitations.toast.url_copied')}>
                  {t('manager.invitations.details.copy_url')}
                </Button>
              </div>
            </div>
            <figure className="mx-auto flex flex-col items-center gap-2 md:mx-0">
              <div className="rounded-card border border-line bg-white p-3" data-testid="qr">
                <QRCodeSVG
                  value={url}
                  size={176}
                  level="M"
                  marginSize={1}
                  bgColor="#ffffff"
                  fgColor="#000000"
                  title={t('manager.invitations.details.qr_label')}
                />
              </div>
              <figcaption className="max-w-[200px] text-center text-caption text-muted">
                {t('manager.invitations.details.qr_label')}
              </figcaption>
            </figure>
          </div>

          <dl className="grid grid-cols-2 gap-3 md:grid-cols-4">
            <Stat
              label={t('manager.invitations.details.expires')}
              value={inv.expiresAt ? formatDateTime(inv.expiresAt, lang) : t('manager.invitations.never')}
            />
            <Stat label={t('manager.invitations.details.max')} value={inv.maxRegistrations} />
            <Stat label={t('manager.invitations.details.used')} value={inv.registrationsCount} />
            <Stat label={t('manager.invitations.details.remaining')} value={remaining(inv)} />
          </dl>

          {(actions.reactivate || actions.changeMax) && (
            <div className="grid min-w-0 gap-4 md:grid-cols-2">
              {actions.reactivate && (
                <fieldset className="min-w-0 space-y-3 rounded-card border border-line p-4">
                  <legend className="px-1 text-label font-semibold text-ink">
                    {t(
                      inv.status === 'expired'
                        ? 'manager.invitations.details.reactivate'
                        : 'manager.invitations.details.change_expiry',
                    )}
                  </legend>
                  <ExpiryField
                    label={t('manager.invitations.form.expires')}
                    value={expiry}
                    onChange={setExpiry}
                    error={expiryErr || undefined}
                  />
                  <Button onClick={saveExpiry} loading={busy === 'expiry'}>
                    {t('common.save')}
                  </Button>
                </fieldset>
              )}
              {actions.changeMax && (
                <fieldset className="min-w-0 space-y-3 rounded-card border border-line p-4">
                  <legend className="px-1 text-label font-semibold text-ink">
                    {t('manager.invitations.details.change_max')}
                  </legend>
                  <Input
                    type="number"
                    inputMode="numeric"
                    min={Math.max(1, inv.registrationsCount)}
                    max={10000}
                    label={t('manager.invitations.form.max')}
                    hint={t('manager.invitations.details.max_hint', { count: inv.registrationsCount })}
                    error={maxErr || undefined}
                    value={max}
                    onChange={(e) => setMax(e.target.value)}
                  />
                  <Button onClick={saveMax} loading={busy === 'max'}>
                    {t('common.save')}
                  </Button>
                </fieldset>
              )}
            </div>
          )}

          <div className="flex flex-wrap items-start gap-3 border-t border-line pt-4">
            {actions.revoke && (
              <Button variant="danger-outline" onClick={() => setConfirm('revoke')}>
                {t('manager.invitations.details.revoke')}
              </Button>
            )}
            {actions.remove ? (
              <Button variant="danger-outline" onClick={() => setConfirm('delete')}>
                {t('manager.invitations.details.delete')}
              </Button>
            ) : (
              <p className="min-w-0 flex-1 basis-60 text-caption text-muted">
                {t('manager.invitations.details.delete_blocked')}
              </p>
            )}
            {inv.status === 'revoked' && (
              <p className="min-w-0 flex-1 basis-60 text-caption text-muted">
                {t('manager.invitations.details.revoked_note')}
              </p>
            )}
          </div>

          <Registrants invitation={inv} />
        </div>
      </Modal>

      <ConfirmDialog
        isOpen={confirm === 'revoke'}
        onCancel={() => setConfirm(null)}
        onConfirm={doConfirmed}
        loading={busy === 'revoke'}
        title={t('manager.invitations.confirm.revoke_title')}
        message={t('manager.invitations.confirm.revoke_msg', { code: inv.code })}
        confirmLabel={t('manager.invitations.details.revoke')}
      />
      <ConfirmDialog
        isOpen={confirm === 'delete'}
        onCancel={() => setConfirm(null)}
        onConfirm={doConfirmed}
        loading={busy === 'delete'}
        title={t('manager.invitations.confirm.delete_title')}
        message={t('manager.invitations.confirm.delete_msg', { code: inv.code })}
      />
    </>
  )
}

// Registrants are fetched in pages of 100 (server max 500) so hundreds of rows stay fast;
// the list scrolls inside the dialog and can be filtered client-side.
function Registrants({ invitation }) {
  const { t } = useTranslation()
  const [rows, setRows] = useState([])
  const [status, setStatus] = useState('loading')
  const [more, setMore] = useState(false)
  const [loadingMore, setLoadingMore] = useState(false)
  const [filter, setFilter] = useState('')
  const total = invitation.registrationsCount

  const load = useCallback(
    async (offset, signal) => {
      if (offset === 0) setStatus('loading')
      try {
        const page = await endpoints.invitations.registrations(invitation.id, {
          signal,
          query: { limit: PAGE, offset },
        })
        if (signal?.aborted) return
        setRows((prev) => (offset === 0 ? page : [...prev, ...page]))
        setMore(page.length === PAGE)
        setStatus('ready')
      } catch {
        if (!signal?.aborted) setStatus('error')
      }
    },
    [invitation.id],
  )

  useEffect(() => {
    const ctrl = new AbortController()
    load(0, ctrl.signal)
    return () => ctrl.abort()
  }, [load, total])

  const shown = useMemo(() => {
    const q = filter.trim().toLowerCase()
    if (!q) return rows
    return rows.filter((r) => `${r.fullName} ${r.jobTitle} ${r.department?.name ?? ''}`.toLowerCase().includes(q))
  }, [rows, filter])

  const loadMore = async () => {
    setLoadingMore(true)
    await load(rows.length)
    setLoadingMore(false)
  }

  return (
    <section aria-labelledby="registrants-title" className="min-w-0 space-y-3">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <h3 id="registrants-title" className="text-body font-semibold text-ink">
          {t('manager.invitations.registrants.title', { count: total })}
        </h3>
        {rows.length > 8 && (
          <Input
            className="w-full sm:w-64"
            type="search"
            label={t('manager.invitations.registrants.filter')}
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
          />
        )}
      </div>

      {status === 'loading' && (
        <div className="flex justify-center py-6">
          <Spinner />
        </div>
      )}
      {status === 'error' && (
        <Alert tone="danger">
          <span>{t('state.error_title')}</span>{' '}
          <button type="button" className="font-semibold underline" onClick={() => load(0)}>
            {t('common.retry')}
          </button>
        </Alert>
      )}
      {status === 'ready' && rows.length === 0 && (
        <p className="text-label text-muted">{t('manager.invitations.registrants.empty')}</p>
      )}
      {status === 'ready' && rows.length > 0 && (
        <>
          <ul
            className="max-h-80 divide-y divide-line overflow-y-auto rounded-card border border-line"
            data-testid="registrants"
          >
            {shown.map((r, i) => (
              <li key={i} className="grid min-w-0 gap-1 px-3 py-2 text-label sm:grid-cols-3 sm:gap-3">
                <span className="min-w-0 font-medium text-ink break-words">{r.fullName}</span>
                <span className="min-w-0 text-muted break-words">{r.jobTitle || '—'}</span>
                <span className="min-w-0 text-muted break-words">{r.department?.name || '—'}</span>
              </li>
            ))}
            {shown.length === 0 && (
              <li className="px-3 py-3 text-label text-muted">{t('manager.invitations.registrants.no_match')}</li>
            )}
          </ul>
          <div className="flex flex-wrap items-center gap-3">
            <p className="text-caption text-muted">
              {t('manager.invitations.registrants.shown', { shown: rows.length, total })}
            </p>
            {more && (
              <Button variant="secondary" onClick={loadMore} loading={loadingMore}>
                {t('manager.invitations.registrants.more')}
              </Button>
            )}
          </div>
        </>
      )}
    </section>
  )
}
