import { useSignal } from '@preact/signals'
import { useEffect } from 'preact/hooks'
import { api, ApiError, shareAttachmentUrl, type ShareMeta } from '../api'
import { t } from '../i18n'
import { copyText } from '../lib/clipboard'
import { formatSize } from '../lib/format'
import { LangSwitch } from './LangSwitch'
import { MessageBody } from './MessageBody'
import { Toasts, toast } from './Toast'

type View = 'loading' | 'ready' | 'gone' | 'error'

export function SharePage(props: { id: string }) {
  const view = useSignal<View>('loading')
  const meta = useSignal<ShareMeta | null>(null)
  const text = useSignal<string | null>(null)
  const busy = useSignal(false)
  const rawHref = `/s/${encodeURIComponent(props.id)}/raw`

  useEffect(() => {
    let alive = true
    api.shareMeta(props.id).then(
      (m) => { if (alive) { meta.value = m; view.value = 'ready' } },
      (e) => { if (alive) view.value = e instanceof ApiError && e.status === 410 ? 'gone' : 'error' },
    )
    return () => { alive = false }
  }, [props.id])

  // Only on explicit click: link-preview bots that run JS must not burn the single use.
  async function showText() {
    busy.value = true
    try {
      const res = await fetch(rawHref, { credentials: 'same-origin' })
      if (res.status === 410) { view.value = 'gone'; return }
      if (!res.ok) { view.value = 'error'; return }
      text.value = await res.text()
    } catch {
      view.value = 'error'
    } finally {
      busy.value = false
    }
  }

  async function copy(s: string) {
    const ok = await copyText(s)
    toast(ok ? t('toast.copied') : t('toast.copy_failed'), ok ? 'ok' : 'err')
  }

  const m = meta.value
  return (
    <div class="login">
      <div class="login-top"><LangSwitch /></div>
      <div class="card share-card">
        <div class="logo">Lan<span>Drop</span></div>
        {view.value === 'loading' && <p class="mut">{t('share.loading')}</p>}
        {view.value === 'gone' && <p class="gone" role="alert">{t('share.gone')}</p>}
        {view.value === 'error' && <p class="error" role="alert">{t('share.error')}</p>}
        {view.value === 'ready' && m && m.type === 'message' && (
          <>
            <div class="fname share-name">{t('share.message_title')}</div>
            <p class="hint">{t('share.expires_on', { time: new Date(m.expires_at).toLocaleString() })}</p>
            <div class="share-msg">
              <MessageBody content={m.content ?? ''} attachments={m.attachments ?? []}
                src={(a, inline) => shareAttachmentUrl(props.id, a.id, inline)}
                actions={(m.content ?? '').trim() !== '' ? (
                  <button class="btn pri" onClick={() => void copy(m.content ?? '')}>{t('item.copy')}</button>
                ) : undefined} />
            </div>
          </>
        )}
        {view.value === 'ready' && m && m.type !== 'message' && (
          <>
            {m.type === 'file' ? (
              <>
                <div class="fname share-name">{m.name}</div>
                <div class="fsub">{formatSize(m.size)}</div>
              </>
            ) : (
              <div class="fsub">{formatSize(m.size)}</div>
            )}
            <p class="hint">
              {t('share.remaining', { n: m.remaining })} · {t('share.expires_on', { time: new Date(m.expires_at).toLocaleString() })}
            </p>
            {m.type === 'file' ? (
              <a class="btn pri big" href={rawHref} download={m.name}>{t('share.download')}</a>
            ) : text.value === null ? (
              <>
                <button class="btn pri big" disabled={busy.value} onClick={() => void showText()}>{t('share.show_text')}</button>
                <p class="hint">{t('share.text_note')}</p>
              </>
            ) : (
              <>
                <pre class="body share-text">{text.value}</pre>
                <button class="btn pri" onClick={() => void copy(text.value ?? '')}>{t('item.copy')}</button>
              </>
            )}
          </>
        )}
      </div>
      <Toasts />
    </div>
  )
}
