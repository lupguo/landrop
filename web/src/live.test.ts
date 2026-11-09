import { afterEach, beforeEach, expect, test, vi } from 'vitest'

class FakeES {
  static all: FakeES[] = []
  onopen: (() => void) | null = null
  onerror: (() => void) | null = null
  closed = false
  listeners: Record<string, ((e: MessageEvent) => void)[]> = {}
  constructor(public url: string) { FakeES.all.push(this) }
  addEventListener(type: string, fn: (e: MessageEvent) => void) { (this.listeners[type] ??= []).push(fn) }
  emit(type: string, data: unknown) { for (const fn of this.listeners[type] ?? []) fn({ data: JSON.stringify(data) } as MessageEvent) }
  close() { this.closed = true }
}

beforeEach(() => {
  vi.useFakeTimers()
  vi.resetModules()
  FakeES.all = []
  vi.stubGlobal('EventSource', FakeES)
})
afterEach(() => { vi.useRealTimers(); vi.unstubAllGlobals() })

test('401 after SSE error: me=null, no reconnect, no pending timer', async () => {
  vi.stubGlobal('fetch', vi.fn(async () => new Response('{"error":"unauthorized"}', { status: 401, headers: { 'content-type': 'application/json' } })))
  const live = await import('./live')
  live.me.value = { peer_id: 'p' } as never
  live.startLive()
  expect(FakeES.all.length).toBe(1)
  FakeES.all[0].onerror!()
  await vi.advanceTimersByTimeAsync(0)
  expect(live.me.value).toBeNull()
  expect(vi.getTimerCount()).toBe(0)
  await vi.advanceTimersByTimeAsync(10000)
  expect(FakeES.all.length).toBe(1)
})

test('non-401 error keeps reconnecting with backoff', async () => {
  vi.stubGlobal('fetch', vi.fn(async () => new Response('{}', { status: 500 })))
  const live = await import('./live')
  live.startLive()
  FakeES.all[0].onerror!()
  await vi.advanceTimersByTimeAsync(0)
  expect(vi.getTimerCount()).toBe(1)
  await vi.advanceTimersByTimeAsync(500)
  expect(FakeES.all.length).toBe(2)
})

const json = (v: unknown) => new Response(JSON.stringify(v), { status: 200, headers: { 'content-type': 'application/json' } })

test('reload merges SSE events that arrive while it is in flight', async () => {
  let resolveItems!: (r: Response) => void
  vi.stubGlobal('fetch', vi.fn(async (url: string) => {
    if (url.startsWith('/api/items')) return new Promise<Response>((r) => { resolveItems = r })
    return json({ peers: [] })
  }))
  const live = await import('./live')
  live.startLive()
  const es = FakeES.all[0]
  es.onopen!()
  await vi.advanceTimersByTimeAsync(0)
  es.emit('item.created', { id: 'new', type: 'text', content: 'n' })
  es.emit('item.created', { id: 'gone', type: 'text', content: 'g' })
  es.emit('item.deleted', { id: 'gone' })
  es.emit('item.deleted', { id: 'old2' })
  // The (stale) response still has old2 and lacks new.
  resolveItems(json({ items: [{ id: 'old1' }, { id: 'old2' }], last_event_id: 1 }))
  await vi.advanceTimersByTimeAsync(0)
  expect(live.items.value.map((x) => x.id)).toEqual(['new', 'old1'])
  live.stopLive()
})

test('reload after items.cleared keeps only items created since', async () => {
  let resolveItems!: (r: Response) => void
  vi.stubGlobal('fetch', vi.fn(async (url: string) => {
    if (url.startsWith('/api/items')) return new Promise<Response>((r) => { resolveItems = r })
    return json({ peers: [] })
  }))
  const live = await import('./live')
  live.startLive()
  const es = FakeES.all[0]
  es.onopen!()
  await vi.advanceTimersByTimeAsync(0)
  es.emit('items.cleared', {})
  es.emit('item.created', { id: 'after', type: 'text', content: 'a' })
  resolveItems(json({ items: [{ id: 'old1' }], last_event_id: 1 }))
  await vi.advanceTimersByTimeAsync(0)
  expect(live.items.value.map((x) => x.id)).toEqual(['after'])
  live.stopLive()
})

// A /api/me fetch that hangs until aborted.
function hangingMe() {
  return vi.fn((_url: string, init?: RequestInit) => new Promise<Response>((_, reject) => {
    init?.signal?.addEventListener('abort', () => reject(new DOMException('aborted', 'AbortError')))
  }))
}

test('reconnect probe uses AbortSignal.timeout(2000)', async () => {
  const fetchMock = hangingMe()
  vi.stubGlobal('fetch', fetchMock)
  const ctl = new AbortController()
  const spy = vi.spyOn(AbortSignal, 'timeout').mockReturnValue(ctl.signal)
  const live = await import('./live')
  live.startLive()
  FakeES.all[0].onerror!()
  expect(spy).toHaveBeenCalledWith(2000)
  expect(fetchMock.mock.calls[0][1]?.signal).toBe(ctl.signal)
  ctl.abort()
  await vi.advanceTimersByTimeAsync(500)
  expect(FakeES.all.length).toBe(2)
  spy.mockRestore()
  live.stopLive()
})

test('reconnect probe times out via fallback when AbortSignal.timeout is missing', async () => {
  vi.stubGlobal('fetch', hangingMe())
  const orig = AbortSignal.timeout
  ;(AbortSignal as unknown as { timeout?: unknown }).timeout = undefined
  try {
    const live = await import('./live')
    live.startLive()
    FakeES.all[0].onerror!()
    await vi.advanceTimersByTimeAsync(1999)
    expect(FakeES.all.length).toBe(1)
    await vi.advanceTimersByTimeAsync(1 + 500)
    expect(FakeES.all.length).toBe(2) // reconnected within ~2.5s despite a hung probe
    live.stopLive()
  } finally {
    AbortSignal.timeout = orig
  }
})
