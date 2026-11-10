import { computed, signal, type ReadonlySignal, type Signal } from '@preact/signals'
import { api, ApiError, upload, type Attachment } from '../api'
import { toast } from '../components/Toast'
import { t } from '../i18n'
import { me } from '../live'
import { errText } from './errors'
import { isImageMime } from './meta'

/** A file waiting in the composer to be sent with the draft. */
export type PendingFile = {
  key: string
  file: File
  /** Object URL for a thumbnail; images only. */
  url?: string
  isImage: boolean
  state: 'idle' | 'uploading' | 'done' | 'error'
  loaded: number
  error?: string
  /** Set once the file is staged on the server (POST /api/attachments). */
  stagedId?: string
}

export const MAX_ATTACHMENTS = 20
const CONCURRENCY = 3

/** The composer's text; MainPage's global paste handler may fill it. */
export const draft: Signal<string> = signal('')
export const pending: Signal<PendingFile[]> = signal([])
export const sending: Signal<boolean> = signal(false)
export const canSend: ReadonlySignal<boolean> = computed(
  () =>
    !sending.value &&
    (draft.value.trim() !== '' || pending.value.length > 0) &&
    !pending.value.some((p) => p.error === 'too_large'),
)

let seq = 0
let cancelled = false
const inflight = new Map<string, { abort(): void }>()

const MIME_EXT: Record<string, string> = {
  'image/png': 'png',
  'image/jpeg': 'jpg',
  'image/gif': 'gif',
  'image/webp': 'webp',
  'image/bmp': 'bmp',
  'image/svg+xml': 'svg',
  'image/tiff': 'tiff',
  'image/avif': 'avif',
  'image/heic': 'heic',
}
const GENERIC = /^image(\.\w+)?$/i
const pad = (v: number) => String(v).padStart(2, '0')

/** `<prefix>-YYYYMMDD-HHMMSS[-n].<ext>` in local time; n <= 0 adds no suffix. */
export function pasteName(file: File, now: Date, n: number, prefix: string): string {
  const ext = MIME_EXT[file.type] || /\.(\w+)$/.exec(file.name)?.[1]?.toLowerCase() || 'png'
  const stamp =
    `${now.getFullYear()}${pad(now.getMonth() + 1)}${pad(now.getDate())}-` +
    `${pad(now.getHours())}${pad(now.getMinutes())}${pad(now.getSeconds())}`
  return `${prefix}-${stamp}${n > 0 ? `-${n}` : ''}.${ext}`
}

function oversize(size: number): boolean {
  const max = me.value?.max_size
  return typeof max === 'number' && max > 0 && size > max
}

function patch(key: string, p: Partial<PendingFile>): void {
  pending.value = pending.value.map((x) => (x.key === key ? { ...x, ...p } : x))
}

function codeOf(e: unknown): string {
  return e instanceof ApiError ? e.code || `http_${e.status}` : 'network'
}

/** Add files to the draft. Pasted files with a generic name ("image.png") get a timestamped one. */
export function addFiles(files: File[], opts?: { pasted?: boolean }): void {
  if (!files.length) return
  if (sending.value) {
    toast(t('composer.busy'), 'err')
    return
  }
  const room = MAX_ATTACHMENTS - pending.value.length
  if (files.length > room) toast(t('composer.too_many'), 'err')
  const take = files.slice(0, Math.max(0, room))
  if (!take.length) return
  const used = new Set(pending.value.map((p) => p.file.name))
  const now = new Date()
  const added = take.map((f): PendingFile => {
    let file = f
    if (opts?.pasted && (f.name === '' || GENERIC.test(f.name))) {
      const prefix = t('composer.shot_prefix')
      let n = 0
      let name = pasteName(f, now, n, prefix)
      while (used.has(name)) name = pasteName(f, now, (n = n ? n + 1 : 2), prefix)
      file = new File([f], name, { type: f.type, lastModified: f.lastModified })
    }
    used.add(file.name)
    const isImage = isImageMime(file.type)
    const p: PendingFile = { key: `d${++seq}`, file, isImage, state: 'idle', loaded: 0 }
    if (isImage) p.url = URL.createObjectURL(file)
    // Oversize files fail at once instead of after sending gigabytes.
    if (oversize(file.size)) Object.assign(p, { state: 'error', error: 'too_large' })
    return p
  })
  pending.value = [...pending.value, ...added]
}

export function removeFile(key: string): void {
  if (sending.value) {
    toast(t('composer.busy'), 'err')
    return
  }
  const p = pending.value.find((x) => x.key === key)
  if (!p) return
  if (p.url) URL.revokeObjectURL(p.url)
  pending.value = pending.value.filter((x) => x.key !== key)
}

/** Abort in-flight uploads of the current send; the draft stays as it is. */
export function cancelSend(): void {
  if (!sending.value) return
  cancelled = true
  for (const h of inflight.values()) h.abort()
}

/** Upload one pending file to `url`; resolves to the server's JSON, or null on failure/cancel. */
async function uploadOne<T>(key: string, url: string): Promise<T | null> {
  const p = pending.value.find((x) => x.key === key)
  if (!p) return null
  patch(key, { state: 'uploading', loaded: 0, error: undefined })
  const h = upload<T>(p.file, (loaded) => patch(key, { loaded }), url)
  inflight.set(key, h)
  try {
    const res = await h.promise
    patch(key, { state: 'done', loaded: p.file.size })
    return res
  } catch (e) {
    patch(key, cancelled ? { state: 'idle', loaded: 0 } : { state: 'error', loaded: 0, error: codeOf(e) })
    return null
  } finally {
    inflight.delete(key)
  }
}

/** Stage every pending file that has no staged id yet, CONCURRENCY at a time. */
async function stageAll(): Promise<boolean> {
  const queue = pending.value.filter((p) => !p.stagedId).map((p) => p.key)
  let i = 0
  let ok = true
  const worker = async () => {
    while (!cancelled && i < queue.length) {
      const key = queue[i++]
      const a = await uploadOne<Attachment>(key, '/api/attachments')
      if (a) patch(key, { stagedId: a.id })
      else ok = false
    }
  }
  await Promise.all(Array.from({ length: Math.min(CONCURRENCY, queue.length) }, worker))
  return ok && !cancelled
}

function clearDraft(text: string): void {
  for (const p of pending.value) if (p.url) URL.revokeObjectURL(p.url)
  pending.value = []
  if (draft.value === text) draft.value = ''
}

/**
 * Send the draft: text only → text item; one file and blank text → file item;
 * anything else → message with staged attachments. On failure the draft is kept.
 */
export async function sendDraft(): Promise<boolean> {
  if (!canSend.value) return false
  const text = draft.value
  const files = pending.value
  sending.value = true
  cancelled = false
  try {
    if (!files.length) {
      await api.sendText(text)
    } else if (files.length === 1 && !text.trim()) {
      if (!(await uploadOne(files[0].key, '/api/upload'))) {
        if (!cancelled) toast(errText(pending.value[0]?.error ?? ''), 'err', 4000)
        return false
      }
    } else {
      if (!(await stageAll())) {
        const failed = pending.value.find((p) => p.state === 'error')
        if (!cancelled && failed) toast(errText(failed.error ?? ''), 'err', 4000)
        return false
      }
      await api.sendMessage(text, pending.value.map((p) => p.stagedId!))
    }
    clearDraft(text)
    return true
  } catch (e) {
    if (e instanceof ApiError && e.code === 'bad_attachment') {
      // Staged files expired or were lost; re-upload them all on the next send.
      pending.value = pending.value.map((p) => ({ ...p, stagedId: undefined, state: 'idle', loaded: 0 }))
    }
    // A request that fails because the user cancelled needs no error toast.
    if (!cancelled) toast(errText(e), 'err', 4000)
    return false
  } finally {
    sending.value = false
    cancelled = false
  }
}
