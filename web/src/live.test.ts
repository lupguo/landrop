import { afterEach, beforeEach, expect, test, vi } from 'vitest'

class FakeES {
  static all: FakeES[] = []
  onopen: (() => void) | null = null
  onerror: (() => void) | null = null
  closed = false
  constructor(public url: string) { FakeES.all.push(this) }
  addEventListener() {}
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
