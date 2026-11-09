import { beforeEach, expect, test, vi } from 'vitest'

type Pending = { resolve: (v: unknown) => void; reject: (e: unknown) => void; progress: (l: number, t: number) => void; name: string }
const pending: Pending[] = []

vi.mock('../api', () => {
  class ApiError extends Error {
    status: number; code: string
    constructor(status: number, code: string) { super(code); this.status = status; this.code = code }
  }
  return {
    ApiError,
    upload: (file: File, onProgress: (l: number, t: number) => void) => {
      let p!: Pending
      const promise = new Promise((resolve, reject) => { p = { resolve, reject, progress: onProgress, name: file.name } })
      pending.push(p)
      return { promise, abort() {} }
    },
  }
})

beforeEach(() => { pending.length = 0; vi.resetModules() })

const file = (n: string, size = 10) => new File(['x'.repeat(size)], n)
const flush = () => new Promise((r) => setTimeout(r, 0))

test('upload queue concurrency 3', async () => {
  const u = await import('./uploads')
  u.enqueue([file('a'), file('b'), file('c'), file('d'), file('e')])
  expect(u.uploads.value.length).toBe(5)
  expect(u.uploads.value.filter((x) => x.state === 'uploading').length).toBe(3)
  expect(u.uploads.value.filter((x) => x.state === 'queued').length).toBe(2)
  expect(pending.map((p) => p.name)).toEqual(['a', 'b', 'c'])
  pending[0].resolve({})
  await flush()
  expect(pending.map((p) => p.name)).toEqual(['a', 'b', 'c', 'd'])
  expect(u.uploads.value.filter((x) => x.state === 'uploading').length).toBe(3)
  expect(u.uploads.value.find((x) => x.name === 'a')!.state).toBe('done')
})

test('progress, error and retry', async () => {
  const u = await import('./uploads')
  const { ApiError } = await import('../api')
  u.enqueue([file('a', 100)])
  pending[0].progress(40, 100)
  expect(u.uploads.value[0].loaded).toBe(40)
  pending[0].reject(new ApiError(413, 'too_large'))
  await flush()
  expect(u.uploads.value[0].state).toBe('error')
  expect(u.uploads.value[0].error).toBe('too_large')
  u.retry(u.uploads.value[0].id)
  expect(u.uploads.value[0].state).toBe('uploading')
  expect(u.uploads.value[0].loaded).toBe(0)
  expect(pending.length).toBe(2)
})

test('done tasks are removed after a delay', async () => {
  vi.useFakeTimers()
  try {
    const u = await import('./uploads')
    u.enqueue([file('a')])
    pending[0].resolve({})
    await vi.advanceTimersByTimeAsync(0)
    expect(u.uploads.value[0].state).toBe('done')
    await vi.advanceTimersByTimeAsync(2000)
    expect(u.uploads.value.length).toBe(0)
  } finally { vi.useRealTimers() }
})

test('speedOver averages bytes over the last ~1s window', async () => {
  const u = await import('./uploads')
  expect(u.speedOver([], 0)).toBe(0)
  // samples: [time ms, loaded bytes]
  expect(u.speedOver([[0, 0], [500, 500], [1000, 1000]], 1000)).toBe(1000)
  expect(u.speedOver([[0, 0], [2000, 1000], [2500, 3000]], 2500)).toBe(4000)
})
