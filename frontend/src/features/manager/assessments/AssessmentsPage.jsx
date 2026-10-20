// Manager: assessments catalog with management controls (preview, edit info, hide/show, expiry, delete).
// Card look matches the employee catalog (features/employee/AssessmentCatalog.jsx); the primary action is replaced by controls.
import { useMemo, useState } from 'react'
import * as endpoints from '../../../api/endpoints'
import { useTranslation } from '../../../i18n'
import { errorMessage } from '../../../lib/errors'
import { useAsync } from '../../../lib/useAsync'
import Async from '../../../ui/Async'
import Badge from '../../../ui/Badge'
import Button from '../../../ui/Button'
import ConfirmDialog from '../../../ui/ConfirmDialog'
import PageHeader from '../../../ui/PageHeader'
import { useToast } from '../../../ui/toastContext'
import { formatDate } from '../format'
import AssessmentMetaDialog from './AssessmentMetaDialog'
import AssessmentPreview from './AssessmentPreview'
import ExpiryDialog from './ExpiryDialog'
import TemplateButton from './TemplateButton'

const TONES = { active: 'success', hidden: 'neutral', expired: 'warning' }
const FILTERS = ['all', 'active', 'hidden', 'expired']

export default function AssessmentsPage() {
  const { t, lang } = useTranslation()
  const toast = useToast()
  const list = useAsync((signal) => endpoints.manage.assessments({ signal }))
  const [filter, setFilter] = useState('all')
  const [dialog, setDialog] = useState(null) // { kind: 'preview'|'meta'|'expiry'|'delete', item }
  const [busyId, setBusyId] = useState(null)
  const [deleting, setDeleting] = useState(false)

  const items = useMemo(() => list.data ?? [], [list.data])
  const counts = useMemo(() => {
    const c = { all: items.length, active: 0, hidden: 0, expired: 0 }
    for (const a of items) if (c[a.status] !== undefined) c[a.status] += 1
    return c
  }, [items])
  const shown = filter === 'all' ? items : items.filter((a) => a.status === filter)

  const close = () => setDialog(null)
  const saved = () => {
    close()
    list.reload()
  }

  const toggleVisibility = async (a) => {
    setBusyId(a.id)
    try {
      await endpoints.manage.setVisibility(a.id, !a.hidden)
      toast.success(t(a.hidden ? 'manager.assessments.toast.shown' : 'manager.assessments.toast.hidden'))
      list.reload()
    } catch (err) {
      toast.error(errorMessage(t, err))
    } finally {
      setBusyId(null)
    }
  }

  const confirmDelete = async () => {
    const a = dialog.item
    setDeleting(true)
    try {
      await endpoints.manage.removeAssessment(a.id)
      toast.success(t(a.attemptCount > 0 ? 'manager.assessments.toast.archived' : 'manager.assessments.toast.deleted'))
      saved()
    } catch (err) {
      toast.error(errorMessage(t, err))
      close()
    } finally {
      setDeleting(false)
    }
  }

  return (
    <div className="min-w-0">
      <PageHeader
        title={t('manager.assessments.title')}
        description={t('manager.assessments.desc')}
        actions={
          <>
            <TemplateButton />
            <Button to="/manage/assessments/new">{t('manager.assessments.create')}</Button>
          </>
        }
      />

      <Async
        status={list.status}
        onRetry={list.reload}
        isEmpty={items.length === 0}
        empty={{
          title: t('manager.assessments.empty_title'),
          description: t('manager.assessments.empty_desc'),
          action: (
            <div className="flex flex-wrap justify-center gap-2">
              <TemplateButton />
              <Button to="/manage/assessments/new">{t('manager.assessments.create')}</Button>
            </div>
          ),
        }}
      >
        <div role="group" aria-label={t('manager.assessments.filter')} className="mb-4 flex flex-wrap gap-2">
          {FILTERS.map((f) => (
            <button
              key={f}
              type="button"
              aria-pressed={filter === f}
              onClick={() => setFilter(f)}
              className={
                'min-h-touch rounded-ctl border px-3 text-label font-medium ' +
                (filter === f
                  ? 'border-primary bg-primary-soft text-primary'
                  : 'border-line-strong bg-surface text-ink hover:bg-surface-2')
              }
            >
              {t(`manager.assessments.status.${f}`)} ({counts[f]})
            </button>
          ))}
        </div>

        {shown.length === 0 ? (
          <p className="py-8 text-center text-label text-muted">{t('manager.assessments.filter_empty')}</p>
        ) : (
          <ul className="grid grid-cols-1 gap-4 lg:grid-cols-2 2xl:grid-cols-3">
            {shown.map((a) => (
              <li key={a.id} className="min-w-0">
                <article
                  className="flex h-full min-w-0 flex-col gap-3 rounded-card border border-line bg-surface p-4"
                  aria-label={a.title}
                >
                  <div className="flex flex-wrap items-start justify-between gap-2">
                    <h3 className="min-w-0 flex-1 basis-40 text-body font-semibold text-ink break-words">{a.title}</h3>
                    <Badge tone={TONES[a.status] || 'neutral'}>{t(`manager.assessments.status.${a.status}`)}</Badge>
                  </div>
                  <p className="min-w-0 flex-1 text-label text-muted break-words">
                    {a.description || t('manager.assessments.no_description')}
                  </p>
                  <dl className="grid grid-cols-2 gap-x-3 gap-y-1 text-caption text-muted">
                    <div className="min-w-0">
                      <dt className="inline">{t('manager.assessments.questions')}: </dt>
                      <dd className="inline font-medium text-ink">{a.questionCount}</dd>
                    </div>
                    <div className="min-w-0">
                      <dt className="inline">{t('manager.assessments.attempts')}: </dt>
                      <dd className="inline font-medium text-ink">{a.attemptCount}</dd>
                    </div>
                    <div className="col-span-2 min-w-0">
                      <dt className="inline">{t('manager.assessments.expires')}: </dt>
                      <dd className="inline font-medium text-ink">
                        {a.expiresAt ? formatDate(a.expiresAt, lang) : t('manager.assessments.no_expiry')}
                      </dd>
                    </div>
                  </dl>
                  <div className="flex flex-wrap gap-2 border-t border-line pt-3">
                    <Button variant="secondary" onClick={() => setDialog({ kind: 'preview', item: a })}>
                      {t('manager.assessments.preview.open')}
                    </Button>
                    <Button variant="secondary" onClick={() => setDialog({ kind: 'meta', item: a })}>
                      {t('manager.assessments.edit')}
                    </Button>
                    <Button variant="secondary" loading={busyId === a.id} onClick={() => toggleVisibility(a)}>
                      {t(a.hidden ? 'manager.assessments.show' : 'manager.assessments.hide')}
                    </Button>
                    <Button variant="secondary" onClick={() => setDialog({ kind: 'expiry', item: a })}>
                      {t('manager.assessments.change_expiry')}
                    </Button>
                    <Button variant="danger-outline" onClick={() => setDialog({ kind: 'delete', item: a })}>
                      {t('common.delete')}
                    </Button>
                  </div>
                </article>
              </li>
            ))}
          </ul>
        )}
      </Async>

      {dialog?.kind === 'preview' && <AssessmentPreview assessment={dialog.item} onClose={close} />}
      {dialog?.kind === 'meta' && <AssessmentMetaDialog assessment={dialog.item} onClose={close} onSaved={saved} />}
      {dialog?.kind === 'expiry' && <ExpiryDialog assessment={dialog.item} onClose={close} onSaved={saved} />}
      <ConfirmDialog
        isOpen={dialog?.kind === 'delete'}
        onCancel={close}
        onConfirm={confirmDelete}
        loading={deleting}
        title={t('manager.assessments.delete.title', { name: dialog?.item?.title ?? '' })}
        confirmLabel={t(dialog?.item?.attemptCount > 0 ? 'manager.assessments.delete.archive' : 'common.delete')}
      >
        <div className="space-y-3 text-label text-ink">
          <ul className="space-y-2">
            <li>
              <strong>{t('manager.assessments.hide')}.</strong> {t('manager.assessments.delete.hide_explain')}
            </li>
            <li>
              <strong>{t('manager.assessments.change_expiry')}.</strong> {t('manager.assessments.delete.expiry_explain')}
            </li>
            <li>
              <strong>{t('common.delete')}.</strong> {t('manager.assessments.delete.delete_explain')}
            </li>
          </ul>
          <p className="rounded-ctl bg-warning-soft p-3 font-medium">
            {dialog?.item?.attemptCount > 0
              ? t('manager.assessments.delete.has_attempts', { count: dialog.item.attemptCount })
              : t('manager.assessments.delete.no_attempts')}
          </p>
        </div>
      </ConfirmDialog>
    </div>
  )
}
