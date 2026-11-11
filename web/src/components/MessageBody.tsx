import type { ComponentChildren } from 'preact'
import { useState } from 'preact/hooks'
import type { Attachment } from '../api'
import { t } from '../i18n'
import { formatSize } from '../lib/format'
import { extLabel, isImageMime } from '../lib/meta'
import { previewKind } from '../lib/preview'
import { ImageViewer, type ViewerItem } from './ImageViewer'
import { TextBody } from './TextBody'

/**
 * A message: text (when non-blank), then images in send order, then other attachments.
 * `src(a, inline)` builds the attachment URL (feed or public share route).
 * `preview` lets non-image attachments open in the viewer (feed only; share
 * links stay download-only).
 */
export function MessageBody(props: {
  content: string
  attachments: Attachment[]
  src: (a: Attachment, inline: boolean) => string
  actions?: ComponentChildren
  preview?: boolean
}) {
  const [view, setView] = useState<number | null>(null)
  const [file, setFile] = useState<ViewerItem | null>(null)
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
      {files.map((a) => {
        const kind = props.preview ? previewKind(a.name, a.mime, a.size) : null
        const open = kind && (() => setFile({ name: a.name, kind, mime: a.mime, size: a.size, src: props.src(a, false), download: props.src(a, false) }))
        return (
        <div class="file msg-file" key={a.id}>
          {open ? (
            <button type="button" class="ficon pv-open" title={t('viewer.preview', { name: a.name })}
              aria-label={t('viewer.preview', { name: a.name })} onClick={open}>
              {extLabel(a.name) || t('item.file_icon')}
            </button>
          ) : <div class="ficon" aria-hidden="true">{extLabel(a.name) || t('item.file_icon')}</div>}
          <div class="fmeta">
            {open
              ? <button type="button" class="fname pv-link" title={a.name} onClick={open}>{a.name}</button>
              : <div class="fname" title={a.name}>{a.name}</div>}
            <div class="fsub">{formatSize(a.size)}</div>
          </div>
          <span class="sp" />
          <a class="btn icon" href={props.src(a, false)} download={a.name}
            aria-label={t('item.download_name', { name: a.name })} title={t('item.download_name', { name: a.name })}>
            <span aria-hidden="true">⬇</span>
          </a>
        </div>
        )
      })}
      {view !== null && (
        <ImageViewer
          images={imgs.map((a) => ({ src: props.src(a, true), download: props.src(a, false), name: a.name }))}
          start={view} onClose={() => setView(null)} />
      )}
      {file && <ImageViewer images={[file]} start={0} onClose={() => setFile(null)} />}
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
