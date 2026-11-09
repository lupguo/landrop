import { useSignal } from '@preact/signals'
import { useEffect } from 'preact/hooks'
import { api, ApiError, type DirEntry } from '../api'
import { t } from '../i18n'
import { breadcrumbs, joinPath, sortEntries } from '../lib/dir'
import { formatSize } from '../lib/format'

export function DirBrowser() {
  const path = useSignal('')
  const entries = useSignal<DirEntry[] | null>(null)
  const err = useSignal<'' | 'dir.forbidden' | 'dir.not_found' | 'dir.error'>('')

  useEffect(() => {
    let alive = true
    entries.value = null
    err.value = ''
    api.dir(path.value).then(
      (r) => { if (alive) entries.value = sortEntries(r.entries ?? []) },
      (e) => {
        if (!alive) return
        const s = e instanceof ApiError ? e.status : 0
        err.value = s === 403 ? 'dir.forbidden' : s === 404 ? 'dir.not_found' : 'dir.error'
      },
    )
    return () => { alive = false }
  }, [path.value])

  const crumbs = breadcrumbs(path.value)
  return (
    <section class="dir">
      <nav class="crumbs" aria-label={t('dir.nav')}>
        {crumbs.length === 0 ? <span class="cur">{t('dir.root')}</span> : <button class="toggle" onClick={() => { path.value = '' }}>{t('dir.root')}</button>}
        {crumbs.map((c, i) => (
          <span key={c.path}>
            <span class="sep" aria-hidden="true"> / </span>
            {i === crumbs.length - 1 ? <span class="cur">{c.name}</span> : <button class="toggle" onClick={() => { path.value = c.path }}>{c.name}</button>}
          </span>
        ))}
      </nav>
      <div class="feed dir-list">
        {err.value && <p class="error" role="alert">{t(err.value)}</p>}
        {!err.value && entries.value === null && <p class="mut empty">{t('dir.loading')}</p>}
        {!err.value && entries.value?.length === 0 && <p class="mut empty">{t('dir.empty')}</p>}
        {!err.value && entries.value?.map((e) => {
          const p = joinPath(path.value, e.name)
          const when = new Date(e.mod_time)
          const sub = e.is_dir ? '' : `${formatSize(e.size)} · `
          const mod = isNaN(when.getTime()) ? '' : when.toLocaleString()
          return e.is_dir ? (
            <button class="item dir-row" key={e.name} onClick={() => { path.value = p }}>
              <span class="ficon" aria-hidden="true">DIR</span>
              <span class="fmeta"><span class="fname">{e.name}</span><span class="fsub">{mod}</span></span>
            </button>
          ) : (
            <a class="item dir-row" key={e.name} href={`/api/dir/raw?path=${encodeURIComponent(p)}`} download={e.name}>
              <span class="ficon" aria-hidden="true">{t('item.file_icon')}</span>
              <span class="fmeta"><span class="fname">{e.name}</span><span class="fsub">{sub}{mod}</span></span>
            </a>
          )
        })}
      </div>
    </section>
  )
}
