import { useEffect, useRef } from 'preact/hooks'
import { api } from '../api'
import { t } from '../i18n'
import { guessDeviceName } from '../lib/device'
import { useOverlay } from '../lib/overlay'
import { me, peers } from '../live'
import { toast } from './Toast'

async function kick(id: string, name: string) {
  if (!confirm(t('peers.kick_confirm', { name }))) return
  try { await api.kick(id) } catch { toast(t('peers.kick_failed'), 'err', 4000) }
}

/** Dropdown under the online pill. */
export function PeersPanel(props: { onClose: () => void }) {
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
      // The pill itself toggles the panel; don't close-then-reopen.
      if (el && (box.current?.contains(el) || el.closest('.pill'))) return
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
  const list = peers.value
  return (
    <div class="peers card" role="dialog" aria-label={t('peers.title')} tabIndex={-1} ref={box}>
      <div class="peers-head">{t('peers.title')}</div>
      {list.length === 0 && <p class="mut hint">{t('peers.empty')}</p>}
      {list.map((p) => {
        const self = !!m && p.peer_id === m.peer_id
        const name = p.name || t('feed.unknown')
        return (
          <div class="peer" key={p.peer_id}>
            <div class="peer-main">
              <div class="peer-name">
                <span class="fname">{name}</span>
                {self && <span class="badge">{t('peers.this_device')}</span>}
                {p.is_host && <span class="badge host">{t('peers.host')}</span>}
              </div>
              <div class="fsub"><span class="mono">{p.ip}</span> · <span>{guessDeviceName(p.ua)}</span></div>
            </div>
            {/* No kick with --no-auth: anyone can rejoin without a code. */}
            {m?.role === 'host' && !m.no_auth && !self && (
              <button class="btn" onClick={() => void kick(p.peer_id, name)}>{t('peers.kick')}</button>
            )}
          </div>
        )
      })}
    </div>
  )
}
