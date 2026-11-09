import { signal, type Signal } from '@preact/signals'
import { api, setUnauthorizedHandler, type Item, type Me, type Peer } from './api'
import { deviceName } from './lib/device'

export const items: Signal<Item[]> = signal([])
export const peers: Signal<Peer[]> = signal([])
export const connected: Signal<boolean> = signal(false)
export const me: Signal<Me | null> = signal(null)

let es: EventSource | null = null
let timer: ReturnType<typeof setTimeout> | undefined
let backoff = 500
let running = false

function stop(): void {
  running = false
  clearTimeout(timer)
  if (es) { es.close(); es = null }
  connected.value = false
}

function revoked(): void {
  me.value = null
  stop()
}

setUnauthorizedHandler(revoked)

/** SSE item changes seen while a reload is in flight, applied on top of its response. */
type Tracker = { created: Map<string, Item>; deleted: Set<string>; cleared: boolean }
const trackers = new Set<Tracker>()

function track(f: (t: Tracker) => void): void { for (const t of trackers) f(t) }

async function reload(): Promise<void> {
  const tr: Tracker = { created: new Map(), deleted: new Set(), cleared: false }
  trackers.add(tr)
  try {
    const r = await api.items(0)
    const base = tr.cleared ? [] : (r.items ?? []).filter((x) => !tr.deleted.has(x.id) && !tr.created.has(x.id))
    items.value = [...[...tr.created.values()].reverse(), ...base]
  } catch { /* 401 handled by api; other errors retried on next reconnect */ } finally {
    trackers.delete(tr)
  }
}

/** AbortSignal that fires after ms (AbortSignal.timeout or a setTimeout fallback). */
function timeoutSignal(ms: number): AbortSignal {
  if (typeof AbortSignal.timeout === 'function') return AbortSignal.timeout(ms)
  const c = new AbortController()
  setTimeout(() => c.abort(), ms)
  return c.signal
}
const PROBE_TIMEOUT_MS = 2000

async function reloadPeers(): Promise<void> {
  try { peers.value = (await api.peers()).peers ?? [] } catch { /* ignore */ }
}

function parse<T>(e: MessageEvent): T | null {
  try { return JSON.parse(e.data) as T } catch { return null }
}

function connect(): void {
  if (!running) return
  const src = new EventSource(`/events?name=${encodeURIComponent(deviceName.value)}`)
  es = src

  src.onopen = () => {
    backoff = 500
    connected.value = true
    void reload()
    void reloadPeers()
  }
  src.onerror = () => {
    connected.value = false
    src.close()
    if (es === src) es = null
    if (!running) return
    // Probe auth: a 401 triggers the unauthorized handler, which stops us.
    // The probe is capped at 2s so a hung request cannot stall reconnects.
    void api.me(timeoutSignal(PROBE_TIMEOUT_MS)).catch(() => {}).then(() => {
      if (!running || es) return
      clearTimeout(timer)
      timer = setTimeout(connect, backoff)
      backoff = Math.min(backoff * 2, 3000)
    })
  }

  src.addEventListener('item.created', (e) => {
    const it = parse<Item>(e as MessageEvent)
    if (!it) return
    track((t) => { t.created.delete(it.id); t.created.set(it.id, it); t.deleted.delete(it.id) })
    items.value = [it, ...items.value.filter((x) => x.id !== it.id)]
  })
  src.addEventListener('item.deleted', (e) => {
    const d = parse<{ id: string }>(e as MessageEvent)
    if (!d) return
    track((t) => { t.created.delete(d.id); t.deleted.add(d.id) })
    items.value = items.value.filter((x) => x.id !== d.id)
  })
  src.addEventListener('items.cleared', () => {
    track((t) => { t.cleared = true; t.created.clear() })
    items.value = []
  })
  src.addEventListener('peer.joined', (e) => {
    const p = parse<Peer>(e as MessageEvent)
    if (p) peers.value = [...peers.value.filter((x) => x.peer_id !== p.peer_id), p]
  })
  src.addEventListener('peer.left', (e) => {
    const d = parse<{ peer_id: string }>(e as MessageEvent)
    if (d) peers.value = peers.value.filter((x) => x.peer_id !== d.peer_id)
  })
  src.addEventListener('session.revoked', revoked)
  src.addEventListener('resync', () => { void reload(); void reloadPeers() })
}

/** Start (or restart) the live connection. Safe to call repeatedly. */
export function startLive(): void {
  stop()
  running = true
  backoff = 500
  connect()
}

export function stopLive(): void { stop() }
