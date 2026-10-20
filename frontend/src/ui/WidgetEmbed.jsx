// Mounts the server-rendered profile widget (GET /api/widget/{id}) into a Shadow DOM root.
// The fragment is standalone HTML with its own <style>, so the shadow boundary keeps its styles
// out of the app and the app's styles out of it. Content comes from our own API (all dynamic
// text is escaped server-side); scripts inserted via innerHTML never execute.
import { useEffect, useRef, useState } from 'react'
import * as endpoints from '../api/endpoints'
import { useTranslation } from '../i18n'
import { cx } from '../lib/cx'
import Button from './Button'
import Skeleton from './Skeleton'

export default function WidgetEmbed({ userId, version = 0, className }) {
  const { t, lang } = useTranslation()
  const hostRef = useRef(null)
  const [status, setStatus] = useState('loading') // loading | ready | error
  const [loaded, setLoaded] = useState(false) // the shadow root holds content (kept while refetching)
  const [tick, setTick] = useState(0)

  useEffect(() => {
    if (userId == null) return undefined
    const ctrl = new AbortController()
    setStatus('loading')
    fetch(endpoints.widget.url(userId, lang), {
      headers: { Accept: 'text/html' },
      credentials: 'omit',
      signal: ctrl.signal,
    })
      .then((res) => {
        if (!res.ok) throw new Error(`widget ${res.status}`)
        return res.text()
      })
      .then((html) => {
        const host = hostRef.current
        if (ctrl.signal.aborted || !host) return
        const root = host.shadowRoot || host.attachShadow({ mode: 'open' })
        root.innerHTML = html
        setLoaded(true)
        setStatus('ready')
      })
      .catch((e) => {
        if (e?.name === 'AbortError' || ctrl.signal.aborted) return
        hostRef.current?.shadowRoot?.replaceChildren()
        setLoaded(false)
        setStatus('error')
      })
    return () => ctrl.abort()
  }, [userId, lang, tick, version])

  return (
    <div className={cx('mx-auto flex w-full max-w-sm flex-col xl:min-h-0 xl:max-w-none xl:flex-1', className)} aria-busy={status === 'loading'}>
      {status === 'loading' && !loaded && <Skeleton className="h-[30rem] w-full" />}
      {status === 'error' && (
        <div
          role="alert"
          className="flex min-h-48 flex-col items-center justify-center gap-3 rounded-card border border-line bg-surface p-6 text-center"
        >
          <p className="text-label text-muted">{t('widget.error')}</p>
          <Button variant="secondary" onClick={() => setTick((n) => n + 1)}>
            {t('common.retry')}
          </Button>
        </div>
      )}
      <div ref={hostRef} role="group" aria-label={t('employee.profile.aria')} hidden={!loaded} className="xl:min-h-0 xl:flex-1" />
    </div>
  )
}
