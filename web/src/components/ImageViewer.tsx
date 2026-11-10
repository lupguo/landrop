import { useEffect, useRef, useState } from 'preact/hooks'
import { t } from '../i18n'
import { useOverlay } from '../lib/overlay'

export type ViewerImage = { src: string; download: string; name: string }

/** Full-screen image viewer: arrows/keys/swipe navigate (clamped), Esc/backdrop close. */
export function ImageViewer(props: { images: ViewerImage[]; start: number; onClose: () => void }) {
  useOverlay()
  const n = props.images.length
  const [i, setI] = useState(Math.min(Math.max(props.start, 0), Math.max(n - 1, 0)))
  const box = useRef<HTMLDivElement>(null)
  const close = useRef(props.onClose)
  close.current = props.onClose
  const touchX = useRef<number | null>(null)
  const go = (d: number) => setI((c) => Math.min(Math.max(c + d, 0), n - 1))

  useEffect(() => {
    const prev = document.activeElement as HTMLElement | null
    box.current?.focus()
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape') { e.preventDefault(); close.current() }
      else if (e.key === 'ArrowRight') { e.preventDefault(); go(1) }
      else if (e.key === 'ArrowLeft') { e.preventDefault(); go(-1) }
    }
    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('keydown', onKey)
      try { prev?.focus({ preventScroll: true }) } catch { /* ignore */ }
    }
  }, [])

  const cur = props.images[i]
  if (!cur) return null
  return (
    <div class="overlay viewer" onMouseDown={(e) => { if (e.target === e.currentTarget) props.onClose() }}
      onTouchStart={(e) => { touchX.current = e.touches[0]?.clientX ?? null }}
      onTouchEnd={(e) => {
        const x0 = touchX.current; touchX.current = null
        const x1 = e.changedTouches[0]?.clientX
        if (x0 == null || x1 == null) return
        if (x1 - x0 > 50) go(-1)
        else if (x0 - x1 > 50) go(1)
      }}>
      <div class="viewer-box" role="dialog" aria-modal="true" aria-label={cur.name} tabIndex={-1} ref={box}
        onMouseDown={(e) => { if (e.target === e.currentTarget) props.onClose() }}>
        <img class="viewer-img" src={cur.src} alt={cur.name} draggable={false} />
        <div class="viewer-bar">
          {n > 1 && <span class="viewer-count">{t('viewer.count', { i: i + 1, n })}</span>}
          <a class="btn icon" href={cur.download} download aria-label={t('viewer.download')} title={t('viewer.download')}>
            <span aria-hidden="true">↓</span>
          </a>
          <button class="btn icon" aria-label={t('viewer.close')} title={t('viewer.close')} onClick={props.onClose}>
            <span aria-hidden="true">×</span>
          </button>
        </div>
        {n > 1 && <button class="btn icon viewer-nav prev" aria-label={t('viewer.prev')} disabled={i === 0} onClick={() => go(-1)}><span aria-hidden="true">‹</span></button>}
        {n > 1 && <button class="btn icon viewer-nav next" aria-label={t('viewer.next')} disabled={i === n - 1} onClick={() => go(1)}><span aria-hidden="true">›</span></button>}
      </div>
    </div>
  )
}
