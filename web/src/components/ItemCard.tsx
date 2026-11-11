import { api, attachmentUrl, rawUrl, type Item } from '../api'
import { t } from '../i18n'
import { copyText } from '../lib/clipboard'
import { errText } from '../lib/errors'
import { avatarColor, avatarLetter, hhmm } from '../lib/meta'
import { FileBody } from './FileBody'
import { MessageBody } from './MessageBody'
import { TextBody } from './TextBody'
import { toast } from './Toast'

async function copy(text: string) {
  const ok = await copyText(text)
  toast(ok ? t('toast.copied') : t('toast.copy_failed'), ok ? 'ok' : 'err')
}

async function del(id: string) {
  // No optimistic removal: the item.deleted SSE event updates the list.
  try { await api.del(id) } catch (e) { toast(errText(e), 'err', 4000) }
}

export function ItemCard(props: { item: Item; onShare: (item: Item) => void }) {
  const it = props.item
  const name = it.from?.name || t('feed.unknown')
  const common = (
    <>
      <button class="btn" onClick={() => props.onShare(it)}>{t('item.share')}</button>
      <button class="btn icon" aria-label={t('item.delete')} title={t('item.delete')} onClick={() => void del(it.id)}>
        <span aria-hidden="true">🗑</span>
      </button>
    </>
  )
  return (
    <div class="item" data-testid="item">
      <div class="meta">
        <span class="av" style={{ background: avatarColor(it.from?.peer_id || '') }} aria-hidden="true">{avatarLetter(it.from?.name || '')}</span>
        <span>{t('feed.from', { name, time: hhmm(it.created_at) })}</span>
      </div>
      {it.type === 'text' ? (
        <TextBody content={it.content ?? ''} actions={<>
          <button class="btn pri" onClick={() => void copy(it.content ?? '')}>{t('item.copy')}</button>
          {common}
        </>} />
      ) : it.type === 'message' ? (
        <MessageBody content={it.content ?? ''} attachments={it.attachments ?? []} preview
          src={(a, inline) => attachmentUrl(it.id, a.id, inline)} actions={<>
            {(it.content ?? '').trim() !== '' && (
              <button class="btn pri" onClick={() => void copy(it.content ?? '')}>{t('item.copy')}</button>
            )}
            {common}
          </>} />
      ) : (
        <>
          <FileBody item={it} />
          <div class="acts">
            <span class="sp" />
            <a class="btn pri" href={rawUrl(it.id)} download={it.file?.name ?? ''}>{t('item.download')}</a>
            {common}
          </div>
        </>
      )}
    </div>
  )
}
