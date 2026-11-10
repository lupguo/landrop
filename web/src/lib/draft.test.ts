import { afterEach, beforeEach, expect, test, vi } from 'vitest'

type Up = {
  name: string
  url: string
  resolve: (v: unknown) => void
  reject: (e: unknown) => void
  progress: (l: number, t: number) => void
  aborted: boolean
}
const ups: Up[] = []
const toasts: string[] = []
const calls = vi.hoisted(() => ({
  sendText: [] as string[],
  sendMessage: [] as [string, string[]][],
  sendTextImpl: null as null | (() => Promise<unknown>),
  sendMessageImpl: null as null | (() => Promise<unknown>),
}))

vi.mock('../api', () => {
  class ApiError extends Error {
    status: number; code: string
    constructor(status: number, code: string) { super(code); this.status = status; this.code = code }
  }
  return {
    ApiError,
    setUnauthorizedHandler: () => {},
    api: {
      sendText: (content: string) => {
        calls.sendText.push(content)
        return calls.sendTextImpl ? calls.sendTextImpl() : Promise.resolve({ id: 'i1' })
      },
      sendMessage: (content: string, ids: string[]) => {
        calls.sendMessage.push([content, ids])
        return calls.sendMessageImpl ? calls.sendMessageImpl() : Promise.resolve({ id: 'm1' })
      },
    },
    upload: (file: File, onProgress: (l: number, t: number) => void, url = '/api/upload') => {
      let u!: Up
      const promise = new Promise((resolve, reject) => {
        u = { name: file.name, url, resolve, reject, progress: onProgress, aborted: false }
      })
      ups.push(u)
      return {
        promise,
        abort() { u.aborted = true; u.reject(new ApiError(0, 'aborted')) },
      }
    },
  }
})

vi.mock('../components/Toast', () => ({ toast: (msg: string) => { toasts.push(msg) } }))

let created = 0
const revoked: string[] = []

beforeEach(() => {
  ups.length = 0
  toasts.length = 0
  calls.sendText.length = 0
  calls.sendMessage.length = 0
  calls.sendTextImpl = null
  calls.sendMessageImpl = null
  revoked.length = 0
  created = 0
  URL.createObjectURL = vi.fn(() => `blob:${++created}`)
  URL.revokeObjectURL = vi.fn((u: string) => { revoked.push(u) })
  vi.resetModules()
})

afterEach(async () => {
  const { me } = await import('../live')
  me.value = null
})

const png = (n: string, size = 3) => new File(['x'.repeat(size)], n, { type: 'image/png' })
const txt = (n: string, size = 3) => new File(['x'.repeat(size)], n, { type: 'text/plain' })
const flush = () => new Promise((r) => setTimeout(r, 0))

async function load() {
  const d = await import('./draft')
  const { t } = await import('../i18n')
  const { ApiError } = await import('../api')
  return { d, t, ApiError }
}

test('pasteName formats and dedups', async () => {
  const { d } = await load()
  const now = new Date(2026, 10, 10, 9, 5, 7)
  expect(d.pasteName(new File([], 'image.png', { type: 'image/png' }), now, 0, 'screenshot')).toBe('screenshot-20261110-090507.png')
  expect(d.pasteName(new File([], 'image.png', { type: 'image/png' }), now, 2, 'screenshot')).toBe('screenshot-20261110-090507-2.png')
  expect(d.pasteName(new File([], 'image', { type: 'image/jpeg' }), now, 0, 'shot')).toBe('shot-20261110-090507.jpg')
})

test('addFiles renames only pasted generic names', async () => {
  const { d, t } = await load()
  const prefix = t('composer.shot_prefix')
  d.addFiles([png('image.png'), png('diagram.png')], { pasted: true })
  d.addFiles([png('image.png')])
  const names = d.pending.value.map((p) => p.file.name)
  expect(names[0]).toMatch(new RegExp(`^${prefix}-\\d{8}-\\d{6}\\.png$`))
  expect(names[1]).toBe('diagram.png')
  expect(names[2]).toBe('image.png')
  d.addFiles([png('image.png'), png('')], { pasted: true })
  const all = d.pending.value.map((p) => p.file.name)
  expect(new Set(all).size).toBe(all.length)
  expect(all[4]).toMatch(new RegExp(`^${prefix}-\\d{8}-\\d{6}(-\\d+)?\\.png$`))
  expect(d.pending.value[0].isImage).toBe(true)
  expect(d.pending.value[0].url).toMatch(/^blob:/)
  d.addFiles([txt('a.txt')])
  expect(d.pending.value[5].isImage).toBe(false)
  expect(d.pending.value[5].url).toBeUndefined()
})

test('addFiles caps at 20 and toasts', async () => {
  const { d, t } = await load()
  d.addFiles(Array.from({ length: 25 }, (_, i) => txt(`f${i}.txt`)))
  expect(d.pending.value.length).toBe(20)
  expect(d.MAX_ATTACHMENTS).toBe(20)
  expect(toasts).toContain(t('composer.too_many'))
})

test('oversize file marked error and blocks send', async () => {
  const { d } = await load()
  const { me } = await import('../live')
  me.value = { max_size: 10 } as never
  d.addFiles([txt('big.txt', 11)])
  const p = d.pending.value[0]
  expect(p.state).toBe('error')
  expect(p.error).toBe('too_large')
  d.draft.value = 'hi'
  expect(d.canSend.value).toBe(false)
  expect(await d.sendDraft()).toBe(false)
  expect(ups.length).toBe(0)
  d.removeFile(p.key)
  expect(d.canSend.value).toBe(true)
})

test('addFiles while sending is rejected', async () => {
  const { d, t } = await load()
  d.sending.value = true
  d.addFiles([txt('a.txt')])
  expect(d.pending.value.length).toBe(0)
  expect(toasts).toContain(t('composer.busy'))
})

test('removeFile revokes object url', async () => {
  const { d } = await load()
  d.addFiles([png('a.png')])
  const p = d.pending.value[0]
  d.removeFile(p.key)
  expect(d.pending.value.length).toBe(0)
  expect(revoked).toEqual([p.url])
})

test('canSend', async () => {
  const { d } = await load()
  d.draft.value = '   '
  expect(d.canSend.value).toBe(false)
  d.draft.value = 'hello'
  expect(d.canSend.value).toBe(true)
  d.draft.value = ''
  d.addFiles([txt('a.txt')])
  expect(d.canSend.value).toBe(true)
})

test('send picks type', async () => {
  // text only
  let { d } = await load()
  d.draft.value = 'hello'
  expect(await d.sendDraft()).toBe(true)
  expect(calls.sendText).toEqual(['hello'])
  expect(ups.length).toBe(0)

  // 1 file + blank text → plain file upload
  vi.resetModules()
  ;({ d } = await load())
  d.addFiles([txt('a.txt')])
  let p = d.sendDraft()
  expect(ups.map((u) => [u.name, u.url])).toEqual([['a.txt', '/api/upload']])
  ups[0].resolve({ id: 'f1' })
  expect(await p).toBe(true)
  expect(calls.sendMessage.length).toBe(0)

  // 1 file + text → stage then message
  vi.resetModules(); ups.length = 0
  ;({ d } = await load())
  d.draft.value = 't'
  d.addFiles([txt('a.txt')])
  p = d.sendDraft()
  expect(ups.map((u) => u.url)).toEqual(['/api/attachments'])
  ups[0].resolve({ id: 's1', name: 'a.txt', size: 3, mime: 'text/plain' })
  expect(await p).toBe(true)
  expect(calls.sendMessage).toEqual([['t', ['s1']]])

  // 2 files, blank text → 2 stage uploads then message
  vi.resetModules(); ups.length = 0; calls.sendMessage.length = 0
  ;({ d } = await load())
  d.addFiles([txt('a.txt'), txt('b.txt')])
  p = d.sendDraft()
  expect(ups.map((u) => u.url)).toEqual(['/api/attachments', '/api/attachments'])
  ups[0].resolve({ id: 's1' })
  ups[1].resolve({ id: 's2' })
  expect(await p).toBe(true)
  expect(calls.sendMessage).toEqual([['', ['s1', 's2']]])
})

test('staging runs at most 3 uploads at once', async () => {
  const { d } = await load()
  d.addFiles([txt('a'), txt('b'), txt('c'), txt('d'), txt('e')])
  const p = d.sendDraft()
  expect(ups.length).toBe(3)
  ups[0].resolve({ id: 's0' })
  await flush()
  expect(ups.length).toBe(4)
  for (let i = 1; i < 4; i++) ups[i].resolve({ id: `s${i}` })
  await flush()
  expect(ups.length).toBe(5)
  ups[4].resolve({ id: 's4' })
  expect(await p).toBe(true)
  expect(calls.sendMessage[0][1]).toEqual(['s0', 's1', 's2', 's3', 's4'])
})

test('success clears draft and revokes urls', async () => {
  const { d } = await load()
  d.draft.value = 'msg'
  d.addFiles([png('a.png'), png('b.png')])
  const urls = d.pending.value.map((p) => p.url)
  const p = d.sendDraft()
  expect(d.sending.value).toBe(true)
  ups[0].resolve({ id: 's1' })
  ups[1].resolve({ id: 's2' })
  expect(await p).toBe(true)
  expect(d.draft.value).toBe('')
  expect(d.pending.value).toEqual([])
  expect(d.sending.value).toBe(false)
  expect(revoked.sort()).toEqual(urls.sort())
})

test('stage failure keeps draft, no sendMessage', async () => {
  const { d, ApiError } = await load()
  d.draft.value = 'keep me'
  d.addFiles([txt('a.txt'), txt('b.txt')])
  const p = d.sendDraft()
  ups[0].resolve({ id: 's1' })
  ups[1].reject(new ApiError(507, 'disk_full'))
  expect(await p).toBe(false)
  expect(calls.sendMessage.length).toBe(0)
  expect(d.draft.value).toBe('keep me')
  const [a, b] = d.pending.value
  expect(a.stagedId).toBe('s1')
  expect(b.state).toBe('error')
  expect(b.error).toBe('disk_full')
  expect(toasts.length).toBe(1)
  expect(d.sending.value).toBe(false)

  const p2 = d.sendDraft()
  expect(ups.slice(2).map((u) => u.name)).toEqual(['b.txt'])
  ups[2].resolve({ id: 's2' })
  expect(await p2).toBe(true)
  expect(calls.sendMessage).toEqual([['keep me', ['s1', 's2']]])
})

test('bad_attachment drops staged ids', async () => {
  const { d, ApiError } = await load()
  d.draft.value = 'x'
  d.addFiles([txt('a.txt'), txt('b.txt')])
  calls.sendMessageImpl = () => Promise.reject(new ApiError(400, 'bad_attachment'))
  const p = d.sendDraft()
  ups[0].resolve({ id: 's1' })
  ups[1].resolve({ id: 's2' })
  expect(await p).toBe(false)
  expect(d.pending.value.every((x) => x.stagedId === undefined)).toBe(true)
  expect(d.draft.value).toBe('x')

  calls.sendMessageImpl = null
  const p2 = d.sendDraft()
  expect(ups.slice(2).map((u) => u.name)).toEqual(['a.txt', 'b.txt'])
  ups[2].resolve({ id: 's3' })
  ups[3].resolve({ id: 's4' })
  expect(await p2).toBe(true)
  expect(calls.sendMessage[1]).toEqual(['x', ['s3', 's4']])
})

test('single-file upload failure keeps draft', async () => {
  const { d, ApiError } = await load()
  d.addFiles([txt('a.txt')])
  const p = d.sendDraft()
  ups[0].reject(new ApiError(413, 'too_large'))
  expect(await p).toBe(false)
  expect(d.pending.value.length).toBe(1)
  expect(d.pending.value[0].state).toBe('error')
})

test('cancelSend aborts uploads and keeps draft', async () => {
  const { d } = await load()
  d.draft.value = 'stay'
  d.addFiles([txt('a.txt'), txt('b.txt')])
  const p = d.sendDraft()
  ups[0].progress(2, 3)
  expect(d.pending.value[0].loaded).toBe(2)
  d.cancelSend()
  expect(await p).toBe(false)
  expect(ups.every((u) => u.aborted)).toBe(true)
  expect(calls.sendMessage.length).toBe(0)
  expect(d.draft.value).toBe('stay')
  expect(d.pending.value.length).toBe(2)
  expect(d.pending.value.every((x) => x.state === 'idle' && x.loaded === 0)).toBe(true)
  expect(d.sending.value).toBe(false)
  expect(toasts.length).toBe(0)
})
