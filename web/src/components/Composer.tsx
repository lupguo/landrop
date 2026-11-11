import { useLayoutEffect, useRef, useState } from 'preact/hooks'
import { api } from '../api'
import { t } from '../i18n'
import { errText } from '../lib/errors'
import { addFiles, cancelSend, canSend, draft, pending, removeFile, sendDraft, sending, type PendingFile } from '../lib/draft'
import { formatSize } from '../lib/format'
import { utf8Len } from '../lib/meta'
import { previewKind } from '../lib/preview'
import { ImageViewer, type ViewerItem } from './ImageViewer'
import { toast } from './Toast'

export const TEXT_LIMIT = 1 << 20
const MAX_H = 180

const isMac = typeof navigator !== 'undefined' && /Mac|iPhone|iPad/.test(navigator.platform || navigator.userAgent)

export async function sendText(text: string): Promise<boolean> {
  if (!text.trim()) return false
  try {
    await api.sendText(text)
    return true
  } catch (e) {
    toast(errText(e), 'err', 4000)
    return false
  }
}

/** Viewer item for a pending (local) file; null when it can't be previewed. */
function trayView(p: PendingFile): ViewerItem | null {
  if (p.isImage && p.url) return { name: p.file.name, kind: 'image', src: p.url }
  const kind = previewKind(p.file.name, p.file.type, p.file.size)
  return kind && kind !== 'image' ? { name: p.file.name, kind, file: p.file, mime: p.file.type, size: p.file.size } : null
}

function TrayItem(props: { p: PendingFile; busy: boolean; onOpen?: () => void }) {
  const { p, onOpen } = props
  const name = p.file.name
  const pct = p.file.size > 0 ? Math.min(100, Math.round((p.loaded / p.file.size) * 100)) : 0
  const err = p.state === 'error' ? (p.error === 'too_large' ? t('composer.file_too_large') : errText(p.error ?? '')) : ''
  return (
    <li class={`att ${p.isImage ? 'img' : 'chip'} ${p.state}`} title={err ? `${name} — ${err}` : name}>
      {p.isImage && p.url
        ? (onOpen
          ? <button type="button" class="att-open" aria-label={t('viewer.preview', { name })} onClick={onOpen}><img src={p.url} alt={name} /></button>
          : <img src={p.url} alt={name} />)
        : onOpen ? (
          <button type="button" class="att-meta att-open" aria-label={t('viewer.preview', { name })} onClick={onOpen}>
            <span class="att-name">{name}</span>
            <span class="fsub">{formatSize(p.file.size)}</span>
          </button>
        ) : (
          <span class="att-meta">
            <span class="att-name">{name}</span>
            <span class="fsub">{formatSize(p.file.size)}</span>
          </span>
        )}
      {p.state === 'uploading' && <span class="att-pct" aria-hidden="true">{pct}%</span>}
      {err && <span class="att-err" role="alert">{err}</span>}
      <button
        class="att-x"
        aria-label={t('composer.remove', { name })}
        disabled={props.busy}
        onClick={() => removeFile(p.key)}
      >✕</button>
    </li>
  )
}

export function Composer(props: { dragging: boolean; textareaRef?: { current: HTMLTextAreaElement | null } }) {
  const local = useRef<HTMLTextAreaElement>(null)
  const ta = props.textareaRef ?? local
  const fileInput = useRef<HTMLInputElement>(null)
  const text = draft.value
  const files = pending.value
  const busy = sending.value
  const tooLarge = text.length > TEXT_LIMIT / 4 && utf8Len(text) > TEXT_LIMIT
  const ready = canSend.value && !tooLarge
  const [viewKey, setViewKey] = useState<string | null>(null)
  const views = new Map<string, ViewerItem>()
  for (const p of files) { const v = trayView(p); if (v) views.set(p.key, v) }

  // Images open as a gallery of the pending images; other files open alone.
  let viewer: { items: ViewerItem[]; start: number } | null = null
  const opened = viewKey ? views.get(viewKey) : undefined
  if (opened?.kind === 'image') {
    const keys = files.filter((p) => views.get(p.key)?.kind === 'image').map((p) => p.key)
    viewer = { items: keys.map((k) => views.get(k)!), start: keys.indexOf(viewKey!) }
  } else if (opened) viewer = { items: [opened], start: 0 }

  useLayoutEffect(() => {
    const el = ta.current
    if (!el) return
    el.style.height = 'auto'
    const h = el.scrollHeight
    el.style.height = `${Math.min(h, MAX_H)}px`
    el.style.overflowY = h > MAX_H ? 'auto' : 'hidden'
  }, [text])

  function send() {
    if (!ready) return
    void sendDraft()
  }

  function sendAsFile() {
    // Clear the text only once it is safely in the tray (the tray may be full).
    if (addFiles([new File([draft.value], 'text.txt', { type: 'text/plain' })])) draft.value = ''
  }

  function onKeyDown(e: KeyboardEvent) {
    if (e.key === 'Enter' && (e.metaKey || e.ctrlKey) && !e.isComposing) {
      e.preventDefault()
      send()
    }
  }

  function onPaste(e: ClipboardEvent) {
    const list = e.clipboardData?.files
    if (!list || !list.length) return // plain text: browser default
    e.preventDefault()
    addFiles(Array.from(list), { pasted: true })
  }

  function onPick(e: Event) {
    const input = e.target as HTMLInputElement
    if (input.files?.length) addFiles(Array.from(input.files))
    input.value = ''
  }

  return (
    <div class={`composer${props.dragging ? ' drag' : ''}`}>
      {files.length > 0 && (
        <ul class="tray">
          {files.map((p) => <TrayItem key={p.key} p={p} busy={busy} onOpen={views.has(p.key) ? () => setViewKey(p.key) : undefined} />)}
        </ul>
      )}
      {viewer && <ImageViewer images={viewer.items} start={viewer.start} onClose={() => setViewKey(null)} />}
      <textarea
        ref={ta}
        rows={2}
        value={text}
        readOnly={busy}
        aria-label={t('composer.label')}
        placeholder={props.dragging ? t('composer.drop') : t('composer.placeholder')}
        onInput={(e) => { draft.value = (e.target as HTMLTextAreaElement).value }}
        onKeyDown={onKeyDown}
        onPaste={onPaste}
      />
      {tooLarge && (
        <div class="too-large" role="alert">
          <span>{t('composer.too_large')}</span>
          <button class="btn" disabled={busy} onClick={sendAsFile}>{t('composer.send_as_file')}</button>
        </div>
      )}
      <div class="row">
        <button class="btn" disabled={busy} onClick={() => fileInput.current?.click()}>
          <span class="ico" aria-hidden="true">📎</span>{t('composer.pick')}
        </button>
        <input ref={fileInput} type="file" multiple hidden onChange={onPick} />
        <span class="row-end">
          {text && <span class="hint">{t('composer.chars', { n: text.length })}</span>}
          {/* One button that turns into Cancel while sending, so keyboard focus stays on it. */}
          <button class={busy ? 'btn' : 'btn pri'} disabled={!busy && !ready} onClick={busy ? cancelSend : send}>
            {busy
              ? t('composer.cancel')
              : <>{t('composer.send')} <span class="kbd" aria-hidden="true">{isMac ? '⌘↵' : 'Ctrl+↵'}</span></>}
          </button>
        </span>
      </div>
    </div>
  )
}
