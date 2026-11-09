import type { ComponentChildren } from 'preact'
import { useLayoutEffect, useMemo, useRef, useState } from 'preact/hooks'
import { t } from '../i18n'
import { looksLikeCode, looksLikeLog } from '../lib/detect'
import { highlight } from '../lib/highlight'
import { linkify } from '../lib/linkify'
import { lineCount } from '../lib/meta'

const MAX_LINES = 4

/** Renders the text body plus the actions row, whose left side holds the expand toggle. */
export function TextBody(props: { content: string; actions: ComponentChildren }) {
  const text = props.content
  // Logs: monospace, no hljs, keep links clickable. Code: monospace + hljs. Otherwise prose.
  const kind = useMemo(() => (looksLikeLog(text) ? 'log' : looksLikeCode(text) ? 'code' : 'prose'), [text])
  const html = useMemo(() => (kind === 'code' ? highlight(text) : ''), [text, kind])
  const segs = useMemo(() => (kind === 'code' ? [] : linkify(text)), [text, kind])
  const lines = useMemo(() => lineCount(text), [text])
  const [open, setOpen] = useState(false)
  const [over, setOver] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  // Show the toggle only when the collapsed body really overflows (long lines wrap too).
  useLayoutEffect(() => {
    const el = ref.current
    if (!el || open) return
    const measure = () => setOver(el.scrollHeight > el.clientHeight + 1)
    measure()
    if (typeof ResizeObserver === 'undefined') return
    const ro = new ResizeObserver(measure)
    ro.observe(el)
    return () => ro.disconnect()
  }, [text, kind, open])

  const cls = `body${kind === 'prose' ? '' : ' mono'}${kind === 'code' ? ' hljs' : ''}${open ? '' : ' clamp'}${over && !open ? ' over' : ''}`
  return (
    <>
      {kind === 'code' ? (
        // highlight.js escapes its input; this is the only allowed innerHTML (see lib/highlight.ts).
        <div ref={ref} class={cls} dangerouslySetInnerHTML={{ __html: html }} />
      ) : (
        <div ref={ref} class={cls}>
          {segs.map((s, i) => (s.href ? <a key={i} href={s.href} target="_blank" rel="noopener noreferrer">{s.text}</a> : s.text))}
        </div>
      )}
      <div class="acts">
        {over && (
          <button class="toggle" aria-expanded={open} onClick={() => setOpen(!open)}>
            {open ? t('feed.collapse') : lines > MAX_LINES ? t('feed.expand', { n: lines }) : t('feed.expand_plain')}
          </button>
        )}
        <span class="sp" />
        {props.actions}
      </div>
    </>
  )
}
