import { useSignal } from '@preact/signals'
import { useEffect } from 'preact/hooks'
import { api, type Item, type ShareResult } from '../api'
import { t } from '../i18n'
import { copyText } from '../lib/clipboard'
import { Dialog } from './Dialog'
import { toast } from './Toast'

const DOWNLOADS = [1, 3, 10]
const TTLS: { sec: number; key: 'share.ttl_10m' | 'share.ttl_1h' | 'share.ttl_24h' }[] = [
  { sec: 600, key: 'share.ttl_10m' },
  { sec: 3600, key: 'share.ttl_1h' },
  { sec: 86400, key: 'share.ttl_24h' },
]

export function ShareDialog(props: { item: Item; onClose: () => void }) {
  const downloads = useSignal(1)
  const ttl = useSignal(600)
  const busy = useSignal(false)
  const failed = useSignal(false)
  const result = useSignal<ShareResult | null>(null)

  async function create() {
    busy.value = true
    failed.value = false
    try {
      result.value = await api.share(props.item.id, downloads.value, ttl.value)
    } catch {
      failed.value = true
    } finally {
      busy.value = false
    }
  }

  async function copy(text: string) {
    const ok = await copyText(text)
    toast(ok ? t('toast.copied') : t('toast.copy_failed'), ok ? 'ok' : 'err')
  }

  // The select that had focus is gone once the link exists; keep focus inside the dialog.
  useEffect(() => { if (result.value) document.getElementById('sh-url')?.focus() }, [result.value])

  const r = result.value
  return (
    <Dialog title={t('share.title')} onClose={props.onClose}>
      {!r ? (
        <>
          <div class="field">
            <label for="sh-dl">{t('share.downloads')}</label>
            <select id="sh-dl" class="inp" value={downloads.value} onChange={(e) => { downloads.value = Number((e.target as HTMLSelectElement).value) }}>
              {DOWNLOADS.map((n) => <option key={n} value={n}>{t('share.n_downloads', { n })}</option>)}
            </select>
          </div>
          <div class="field">
            <label for="sh-ttl">{t('share.expiry')}</label>
            <select id="sh-ttl" class="inp" value={ttl.value} onChange={(e) => { ttl.value = Number((e.target as HTMLSelectElement).value) }}>
              {TTLS.map((o) => <option key={o.sec} value={o.sec}>{t(o.key)}</option>)}
            </select>
          </div>
          <p class="error" role="alert">{failed.value ? t('share.failed') : ''}</p>
          <div class="dlg-acts">
            <button class="btn pri" disabled={busy.value} onClick={() => void create()}>{busy.value ? t('share.creating') : t('share.create')}</button>
          </div>
        </>
      ) : (
        <>
          <div class="field">
            <label for="sh-url">{t('share.url')}</label>
            <div class="row-copy">
              <input id="sh-url" class="inp mono" type="text" readOnly value={r.url} onFocus={(e) => (e.target as HTMLInputElement).select()} />
              <button class="btn pri" onClick={() => void copy(r.url)}>{t('item.copy')}</button>
            </div>
          </div>
          <div class="qr"><img src={`/api/qr?data=${encodeURIComponent(r.url)}`} alt={t('share.qr_alt')} width="180" height="180" /></div>
          <p class="hint center">{t('share.expires_at', { time: new Date(r.expires_at).toLocaleString() })} · {t('share.remaining', { n: r.remaining })}</p>
        </>
      )}
    </Dialog>
  )
}
