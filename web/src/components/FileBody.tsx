import { useState } from 'preact/hooks'
import { rawUrl, type Item } from '../api'
import { t } from '../i18n'
import { formatSize } from '../lib/format'
import { extLabel, isImageMime } from '../lib/meta'
import { ImageViewer } from './ImageViewer'

export function FileBody(props: { item: Item }) {
  const f = props.item.file
  const [view, setView] = useState(false)
  if (!f) return null
  const img = isImageMime(f.mime)
  return (
    <div class="file">
      {img ? (
        <button type="button" class="thumb" title={f.name} onClick={() => setView(true)}>
          <img src={rawUrl(props.item.id, true)} loading="lazy" alt={f.name} />
        </button>
      ) : (
        <div class="ficon" aria-hidden="true">{extLabel(f.name) || t('item.file_icon')}</div>
      )}
      <div class="fmeta">
        <div class="fname" title={f.name}>{f.name}</div>
        <div class="fsub">{formatSize(f.size)}</div>
      </div>
      {view && (
        <ImageViewer images={[{ src: rawUrl(props.item.id, true), download: rawUrl(props.item.id), name: f.name }]}
          start={0} onClose={() => setView(false)} />
      )}
    </div>
  )
}
