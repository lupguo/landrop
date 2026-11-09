import type { ComponentChildren } from 'preact'
import { useEffect, useRef } from 'preact/hooks'
import { t } from '../i18n'
import { useOverlay } from '../lib/overlay'

const FOCUSABLE = 'a[href],button:not([disabled]),input:not([disabled]),select:not([disabled]),textarea:not([disabled]),[tabindex]:not([tabindex="-1"])'

/** Modal overlay: Esc / backdrop close, focus moves in on open and is restored on close. */
export function Dialog(props: { title: string; onClose: () => void; children: ComponentChildren }) {
  useOverlay()
  const box = useRef<HTMLDivElement>(null)
  const close = useRef(props.onClose)
  close.current = props.onClose

  useEffect(() => {
    const prev = document.activeElement as HTMLElement | null
    const first = box.current?.querySelector<HTMLElement>('input:not([readonly]),select,textarea')
    ;(first ?? box.current)?.focus()
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape') { e.preventDefault(); close.current(); return }
      if (e.key !== 'Tab' || !box.current) return
      const f = Array.from(box.current.querySelectorAll<HTMLElement>(FOCUSABLE))
      if (!f.length) return
      const a = document.activeElement
      if (e.shiftKey && (a === f[0] || a === box.current)) { e.preventDefault(); f[f.length - 1].focus() }
      else if (!e.shiftKey && a === f[f.length - 1]) { e.preventDefault(); f[0].focus() }
    }
    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('keydown', onKey)
      try { prev?.focus({ preventScroll: true }) } catch { /* ignore */ }
    }
  }, [])

  return (
    <div class="overlay" onMouseDown={(e) => { if (e.target === e.currentTarget) props.onClose() }}>
      <div class="dialog card" role="dialog" aria-modal="true" aria-label={props.title} tabIndex={-1} ref={box}>
        <div class="dlg-head">
          <h2>{props.title}</h2>
          <button class="btn icon" data-close aria-label={t('dialog.close')} title={t('dialog.close')} onClick={props.onClose}>
            <span aria-hidden="true">×</span>
          </button>
        </div>
        {props.children}
      </div>
    </div>
  )
}
