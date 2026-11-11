import { useEffect, useRef } from 'preact/hooks'
import { t } from '../i18n'
import { copyText } from '../lib/clipboard'
import { useOverlay } from '../lib/overlay'
import { me } from '../live'
import { toast } from './Toast'

/**
 * Dropdown under the Invite button (a bottom sheet on phones). Mirrors the
 * terminal banner: the QR code is the token link, so scanning skips the access
 * code; the copyable address is the plain one and asks for the code.
 */
export function InvitePanel(props: { onClose: () => void }) {
  useOverlay()
  const box = useRef<HTMLDivElement>(null)
  const close = useRef(props.onClose)
  close.current = props.onClose

  useEffect(() => {
    const prev = document.activeElement as HTMLElement | null
    box.current?.focus()
    function onKey(e: KeyboardEvent) { if (e.key === 'Escape') { e.preventDefault(); close.current() } }
    function onDown(e: MouseEvent) {
      const el = e.target as HTMLElement | null
      // The Invite button itself toggles the panel; don't close-then-reopen.
      if (el && (box.current?.contains(el) || el.closest('.invite-btn'))) return
      close.current()
    }
    document.addEventListener('keydown', onKey)
    document.addEventListener('mousedown', onDown)
    return () => {
      document.removeEventListener('keydown', onKey)
      document.removeEventListener('mousedown', onDown)
      try { prev?.focus({ preventScroll: true }) } catch { /* ignore */ }
    }
  }, [])

  const m = me.value
  if (!m) return null
  const addr = m.public_url

  async function copy() {
    const ok = await copyText(addr)
    toast(ok ? t('toast.copied') : t('toast.copy_failed'), ok ? 'ok' : 'err')
  }

  return (
    <>
      <div class="sheet-scrim" aria-hidden="true" />
      <div class="invite card" role="dialog" aria-label={t('invite.title')} tabIndex={-1} ref={box}>
        <div class="peers-head">{t('invite.title')}</div>
        <div class="qr"><img src={`/api/qr?data=${encodeURIComponent(m.join_url)}`} alt={t('invite.qr_alt')} width="180" height="180" /></div>
        {m.code && <p class="hint center">{t('invite.scan')}</p>}
        <div class="field">
          <label for="inv-url">{t('invite.address')}</label>
          <div class="row-copy">
            <input id="inv-url" class="inp mono" type="text" readOnly value={addr} onFocus={(e) => (e.target as HTMLInputElement).select()} />
            <button class="btn pri" onClick={() => void copy()}>{t('item.copy')}</button>
          </div>
        </div>
        {m.code && (
          <div class="invite-code">
            <span class="lbl">{t('invite.code')}</span>
            <span class="code-val mono">{m.code}</span>
          </div>
        )}
        {m.code && <p class="hint">{t('invite.code_hint')}</p>}
      </div>
    </>
  )
}
