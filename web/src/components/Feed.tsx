import { api, type Item } from '../api'
import { t } from '../i18n'
import { errText } from '../lib/errors'
import { items } from '../live'
import { ItemCard } from './ItemCard'
import { toast } from './Toast'

async function clearAll(n: number) {
  if (!confirm(t('feed.clear_confirm', { n }))) return
  try { await api.clear() } catch (e) { toast(errText(e), 'err', 4000) }
}

export function Feed(props: { onShare: (item: Item) => void }) {
  const list = items.value
  return (
    <section class="feed-wrap">
      <div class="feedhead">
        <span>{t('app.items', { n: list.length })}</span>
        {list.length > 0 && <button class="toggle" onClick={() => void clearAll(list.length)}>{t('feed.clear')}</button>}
      </div>
      <div class="feed" data-testid="feed">
        {list.length === 0 && <p class="empty mut">{t('feed.empty')}</p>}
        {list.map((it) => <ItemCard key={it.id} item={it} onShare={props.onShare} />)}
      </div>
    </section>
  )
}
