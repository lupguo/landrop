import { rawUrl, type Item } from '../api'
import { t } from '../i18n'
import { formatSize } from '../lib/format'
import { extLabel, isImageMime } from '../lib/meta'

export function FileBody(props: { item: Item }) {
  const f = props.item.file
  if (!f) return null
  const img = isImageMime(f.mime)
  return (
    <div class="file">
      {img ? (
        <a class="thumb" href={rawUrl(props.item.id, true)} target="_blank" rel="noopener noreferrer">
          <img src={rawUrl(props.item.id, true)} loading="lazy" alt={f.name} />
        </a>
      ) : (
        <div class="ficon" aria-hidden="true">{extLabel(f.name) || t('item.file_icon')}</div>
      )}
      <div class="fmeta">
        <div class="fname" title={f.name}>{f.name}</div>
        <div class="fsub">{formatSize(f.size)}</div>
      </div>
    </div>
  )
}
