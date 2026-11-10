// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen } from '@testing-library/preact'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'
import type { Item } from '../api'
import { setLang } from '../i18n'
import { ShareDialog } from './ShareDialog'

const res = (remaining: number) => ({
  id: 's1', url: 'http://h/s/s1', remaining, expires_at: new Date(Date.now() + 600_000).toISOString(),
})
const item = (type: string) => ({ id: 'i1', type, seq: 1, created_at: '', expires_at: '', from: {} } as unknown as Item)

beforeEach(() => setLang('en'))
afterEach(() => { cleanup(); vi.unstubAllGlobals() })

function stub(remaining: number) {
  vi.stubGlobal('fetch', vi.fn(async () => new Response(JSON.stringify(res(remaining)), { status: 201, headers: { 'content-type': 'application/json' } })))
}

test('message link result says it works until it expires', async () => {
  stub(-1)
  render(<ShareDialog item={item('message')} onClose={() => {}} />)
  fireEvent.click(screen.getByText('Create link'))
  expect(await screen.findByText('No download limit: the link works until it expires.')).toBeTruthy()
  expect(screen.queryByText(/download\(s\) left/)).toBeNull()
})

test('file link result shows remaining downloads and no expiry-only hint', async () => {
  stub(1)
  render(<ShareDialog item={item('file')} onClose={() => {}} />)
  fireEvent.click(screen.getByText('Create link'))
  expect(await screen.findByText(/1 download\(s\) left/)).toBeTruthy()
  expect(screen.queryByText(/works until it expires/)).toBeNull()
})
