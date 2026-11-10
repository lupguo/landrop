// @vitest-environment jsdom
import { act, cleanup, render, screen, waitFor } from '@testing-library/preact'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'
import { setLang } from '../i18n'
import { me, peers } from '../live'
import { draft, pending, sending } from '../lib/draft'
import { MainPage } from './MainPage'

const png = (n: string) => new File(['png'], n, { type: 'image/png' })
let fetchMock: ReturnType<typeof vi.fn>

let n = 0
beforeEach(() => {
  setLang('en')
  me.value = { peer_id: 'p1', role: 'host', no_auth: false, public_url: '', mdns_url: '', has_dir: false, max_size: 1 << 20 }
  peers.value = []
  URL.createObjectURL = vi.fn(() => `blob:${++n}`)
  URL.revokeObjectURL = vi.fn()
  fetchMock = vi.fn(async () => new Response('{}', { status: 201, headers: { 'content-type': 'application/json' } }))
  vi.stubGlobal('fetch', fetchMock)
})
afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
  pending.value = []
  draft.value = ''
  sending.value = false
})

function paste(files: File[], text: string) {
  const ev = new Event('paste', { bubbles: true, cancelable: true })
  Object.defineProperty(ev, 'clipboardData', { value: { files, getData: () => text } })
  act(() => { document.dispatchEvent(ev) })
  return ev
}

const textarea = () => screen.getByLabelText('Message') as HTMLTextAreaElement

test('outside paste of files goes to tray and focuses textarea', () => {
  render(<MainPage />)
  const ev = paste([png('a.png')], '')
  expect(ev.defaultPrevented).toBe(true)
  expect(pending.value.length).toBe(1)
  expect(document.activeElement).toBe(textarea())
  expect(fetchMock).not.toHaveBeenCalled()
})

test('outside paste of text sends when draft empty, appends when draft has attachments', async () => {
  render(<MainPage />)
  expect(paste([], 'hello').defaultPrevented).toBe(true)
  await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1))
  expect(String(fetchMock.mock.calls[0][0])).toContain('/api/text')
  expect(draft.value).toBe('')

  textarea().blur()
  paste([png('a.png')], '')
  textarea().blur()
  expect(paste([], 'world').defaultPrevented).toBe(true)
  expect(draft.value).toBe('world')
  expect(document.activeElement).toBe(textarea())
  expect(fetchMock).toHaveBeenCalledTimes(1)
})

test('drop adds to tray instead of uploading', () => {
  render(<MainPage />)
  const ev = new Event('drop', { bubbles: true, cancelable: true })
  Object.defineProperty(ev, 'dataTransfer', { value: { types: ['Files'], files: [png('d.png')] } })
  act(() => { document.dispatchEvent(ev) })
  expect(ev.defaultPrevented).toBe(true)
  expect(pending.value.length).toBe(1)
  expect(pending.value[0].file.name).toBe('d.png')
  expect(fetchMock).not.toHaveBeenCalled()
})

test('outside text paste while sending does not change the draft', () => {
  render(<MainPage />)
  act(() => { draft.value = 'abc'; sending.value = true })
  textarea().blur()
  paste([], 'def')
  expect(draft.value).toBe('abc')
  expect(fetchMock).not.toHaveBeenCalled()
})
