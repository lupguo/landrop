import { useState } from 'preact/hooks'
import { rawUrl, type Item } from '../api'
import { t } from '../i18n'
import { formatSize } from '../lib/format'
import { extLabel } from '../lib/meta'
import { previewKind } from '../lib/preview'
import { ImageViewer } from './ImageViewer'

export function FileBody(props: { item: Item }) {
  const f = props.item.file
  const [view, setView] = useState(false)
  if (!f) return null
  const kind = previewKind(f.name, f.mime, f.size)
  const img = kind === 'image'
  const id = props.item.id
  return (
    <div class="file">
      {img ? (
        <button type="button" class="thumb" title={f.name} onClick={() => setView(true)}>
          <img src={rawUrl(id, true)} loading="lazy" alt={f.name} />
        </button>
      ) : kind ? (
        <button type="button" class="ficon pv-open" title={t('viewer.preview', { name: f.name })}
          aria-label={t('viewer.preview', { name: f.name })} onClick={() => setView(true)}>
          {extLabel(f.name) || t('item.file_icon')}
        </button>
      ) : (
        <div class="ficon" aria-hidden="true">{extLabel(f.name) || t('item.file_icon')}</div>
      )}
      <div class="fmeta">
        {kind && !img
          ? <button type="button" class="fname pv-link" title={f.name} onClick={() => setView(true)}>{f.name}</button>
          : <div class="fname" title={f.name}>{f.name}</div>}
        <div class="fsub">{formatSize(f.size)}</div>
      </div>
      {view && kind && (
        <ImageViewer
          images={[{ name: f.name, kind, mime: f.mime, size: f.size, src: rawUrl(id, img), download: rawUrl(id) }]}
          start={0} onClose={() => setView(false)} />
      )}
    </div>
  )
}
