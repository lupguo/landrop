import type { ComponentChildren } from 'preact'
import { useState } from 'preact/hooks'
import type { Attachment } from '../api'
import { t } from '../i18n'
import { formatSize } from '../lib/format'
import { extLabel, isImageMime } from '../lib/meta'
import { ImageViewer } from './ImageViewer'
import { TextBody } from './TextBody'

/**
 * A message: text (when non-blank), then images in send order, then other attachments.
 * `src(a, inline)` builds the attachment URL (feed or public share route).
 */
export function MessageBody(props: {
  content: string
  attachments: Attachment[]
  src: (a: Attachment, inline: boolean) => string
  actions?: ComponentChildren
}) {
  const [view, setView] = useState<number | null>(null)
  const imgs = props.attachments.filter((a) => isImageMime(a.mime))
  const files = props.attachments.filter((a) => !isImageMime(a.mime))
  const cls = imgs.length === 1 ? 'msg-imgs one' : `msg-imgs grid${imgs.length === 2 ? ' two' : ''}`

  const after = (
    <>
      {imgs.length > 0 && (
        <div class={cls}>
          {imgs.map((a, i) => (
            <div class="msg-cell" key={a.id}>
              <button type="button" class="msg-img" title={a.name} onClick={() => setView(i)}>
                <img src={props.src(a, true)} loading="lazy" alt={a.name} />
              </button>
              <a class="msg-dl" href={props.src(a, false)} download={a.name}
                aria-label={t('item.download_name', { name: a.name })} title={t('item.download_name', { name: a.name })}>
                <span aria-hidden="true">⬇</span>
              </a>
            </div>
          ))}
        </div>
      )}
      {files.map((a) => (
        <div class="file msg-file" key={a.id}>
          <div class="ficon" aria-hidden="true">{extLabel(a.name) || t('item.file_icon')}</div>
          <div class="fmeta">
            <div class="fname" title={a.name}>{a.name}</div>
            <div class="fsub">{formatSize(a.size)}</div>
          </div>
          <span class="sp" />
          <a class="btn icon" href={props.src(a, false)} download={a.name}
            aria-label={t('item.download_name', { name: a.name })} title={t('item.download_name', { name: a.name })}>
            <span aria-hidden="true">⬇</span>
          </a>
        </div>
      ))}
      {view !== null && (
        <ImageViewer
          images={imgs.map((a) => ({ src: props.src(a, true), download: props.src(a, false), name: a.name }))}
          start={view} onClose={() => setView(null)} />
      )}
    </>
  )

  if (props.content.trim()) return <TextBody content={props.content} actions={props.actions} after={after} />
  return (
    <>
      {after}
      {props.actions && <div class="acts"><span class="sp" />{props.actions}</div>}
    </>
  )
}
