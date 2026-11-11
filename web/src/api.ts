import { deviceName } from './lib/device'

export interface Attachment { id: string; name: string; size: number; mime: string }
export interface PeerRef { peer_id: string; name: string }
export interface Item {
  id: string
  seq: number
  type: 'text' | 'file' | 'message'
  content?: string
  file?: { name: string; size: number; mime: string }
  attachments?: Attachment[]
  from: PeerRef
  created_at: string
  expires_at: string
}
export interface Peer { peer_id: string; name: string; ua: string; ip: string; is_host: boolean }
export interface Me {
  peer_id: string
  role: 'host' | 'peer'
  no_auth: boolean
  public_url: string
  mdns_url: string
  /** Access code; "" with --no-auth. */
  code: string
  /** QR target: the token login link (plain address with --no-auth). */
  join_url: string
  has_dir: boolean
  max_size: number
}
export interface ShareResult { id: string; url: string; remaining: number; expires_at: string }
export interface ShareMeta {
  type: 'text' | 'file' | 'message'
  name: string
  size: number
  mime: string
  content?: string
  attachments?: Attachment[]
  remaining: number
  expires_at: string
}
export interface DirEntry { name: string; is_dir: boolean; size: number; mod_time: string }

export class ApiError extends Error {
  status: number
  code: string
  constructor(status: number, code: string) {
    super(code || `http ${status}`)
    this.status = status
    this.code = code
  }
}

/** Called on any 401 so the app can fall back to the login page. */
let onUnauthorized: () => void = () => {}
export function setUnauthorizedHandler(f: () => void): void { onUnauthorized = f }

function nameHeader(): string { return encodeURIComponent(deviceName.value) }

async function req<T>(method: string, url: string, body?: unknown, signal?: AbortSignal): Promise<T> {
  const headers: Record<string, string> = { 'X-LanDrop-Name': nameHeader() }
  let payload: string | undefined
  if (body !== undefined) {
    headers['Content-Type'] = 'application/json'
    payload = JSON.stringify(body)
  }
  const res = await fetch(url, { method, headers, body: payload, credentials: 'same-origin', ...(signal ? { signal } : {}) })
  if (!res.ok) {
    let code = ''
    try { code = (await res.json()).error ?? '' } catch { /* not json */ }
    if (res.status === 401 && url !== '/api/auth') onUnauthorized()
    throw new ApiError(res.status, code)
  }
  if (res.status === 204) return undefined as T
  const ct = res.headers.get('content-type') || ''
  return (ct.includes('json') ? await res.json() : undefined) as T
}

export const api = {
  me: (signal?: AbortSignal) => req<Me>('GET', '/api/me', undefined, signal),
  items: (since = 0) => req<{ items: Item[]; last_event_id: number }>('GET', `/api/items?since=${since}`),
  sendText: (content: string) => req<Item>('POST', '/api/text', { content }),
  sendMessage: (content: string, ids: string[]) => req<Item>('POST', '/api/messages', { content, attachments: ids }),
  del: (id: string) => req<void>('DELETE', `/api/items/${encodeURIComponent(id)}`),
  clear: () => req<void>('DELETE', '/api/items'),
  share: (id: string, maxDownloads: number, ttlSeconds: number) =>
    req<ShareResult>('POST', `/api/items/${encodeURIComponent(id)}/share`, { max_downloads: maxDownloads, ttl_seconds: ttlSeconds }),
  peers: () => req<{ peers: Peer[] }>('GET', '/api/peers'),
  kick: (peerId: string) => req<void>('DELETE', `/api/peers/${encodeURIComponent(peerId)}`),
  auth: (code: string) => req<void>('POST', '/api/auth', { code }),
  dir: (path: string) => req<{ path: string; entries: DirEntry[] }>('GET', `/api/dir?path=${encodeURIComponent(path)}`),
  shareMeta: (id: string) => req<ShareMeta>('GET', `/api/s/${encodeURIComponent(id)}`),
}

export function rawUrl(id: string, inline?: boolean): string {
  return `/api/items/${encodeURIComponent(id)}/raw${inline ? '?inline=1' : ''}`
}

export function attachmentUrl(itemId: string, attId: string, inline?: boolean): string {
  return `/api/items/${encodeURIComponent(itemId)}/attachments/${encodeURIComponent(attId)}/raw${inline ? '?inline=1' : ''}`
}

export function shareAttachmentUrl(shareId: string, attId: string, inline?: boolean): string {
  return `/s/${encodeURIComponent(shareId)}/attachments/${encodeURIComponent(attId)}/raw${inline ? '?inline=1' : ''}`
}

/**
 * POST a file as multipart part `file` with progress. `/api/upload` publishes a file
 * Item; `/api/attachments` stages it and resolves to an Attachment instead.
 */
export function upload<T = Item>(
  file: File,
  onProgress: (loaded: number, total: number) => void,
  url = '/api/upload',
): { promise: Promise<T>; abort(): void } {
  const xhr = new XMLHttpRequest()
  const promise = new Promise<T>((resolve, reject) => {
    xhr.open('POST', url)
    xhr.setRequestHeader('X-LanDrop-Name', nameHeader())
    xhr.upload.onprogress = (e) => { if (e.lengthComputable) onProgress(e.loaded, e.total) }
    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) {
        try { resolve(JSON.parse(xhr.responseText)) } catch { reject(new ApiError(xhr.status, 'bad_response')) }
        return
      }
      let code = ''
      try { code = JSON.parse(xhr.responseText).error ?? '' } catch { /* ignore */ }
      if (xhr.status === 401) onUnauthorized()
      reject(new ApiError(xhr.status, code))
    }
    xhr.onerror = () => reject(new ApiError(0, 'network'))
    xhr.onabort = () => reject(new ApiError(0, 'aborted'))
    const fd = new FormData()
    fd.append('file', file)
    xhr.send(fd)
  })
  return { promise, abort: () => xhr.abort() }
}
