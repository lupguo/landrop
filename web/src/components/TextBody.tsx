import type { ComponentChildren } from 'preact'
import { useMemo, useState } from 'preact/hooks'
import { t } from '../i18n'
import { looksLikeCode } from '../lib/detect'
import { highlight } from '../lib/highlight'
import { linkify } from '../lib/linkify'
import { lineCount } from '../lib/meta'

const MAX_LINES = 4
// A very long single line (minified JSON, base64) would also flood the feed.
const MAX_CHARS = 600

/** Renders the text body plus the actions row, whose left side holds the expand toggle. */
export function TextBody(props: { content: string; actions: ComponentChildren }) {
  const text = props.content
  const code = useMemo(() => looksLikeCode(text), [text])
  const html = useMemo(() => (code ? highlight(text) : ''), [text, code])
  const segs = useMemo(() => (code ? [] : linkify(text)), [text, code])
  const lines = useMemo(() => lineCount(text), [text])
  const long = lines > MAX_LINES || text.length > MAX_CHARS
  const [open, setOpen] = useState(false)
  const clamp = long && !open

  return (
    <>
      {code ? (
        // highlight.js escapes its input; this is the only allowed innerHTML (see lib/highlight.ts).
        <div class={`body mono hljs${clamp ? ' clamp' : ''}`} dangerouslySetInnerHTML={{ __html: html }} />
      ) : (
        <div class={`body${clamp ? ' clamp' : ''}`}>
          {segs.map((s, i) => (s.href ? <a key={i} href={s.href} target="_blank" rel="noopener noreferrer">{s.text}</a> : s.text))}
        </div>
      )}
      <div class="acts">
        {long && (
          <button class="toggle" aria-expanded={open} onClick={() => setOpen(!open)}>
            {open ? t('feed.collapse') : t('feed.expand', { n: lines })}
          </button>
        )}
        <span class="sp" />
        {props.actions}
      </div>
    </>
  )
}
