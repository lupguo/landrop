// @vitest-environment jsdom
import { cleanup, render, waitFor } from '@testing-library/preact'
import { afterEach, expect, test, vi } from 'vitest'
import { setLang } from '../i18n'
import { me, peers } from '../live'
import { openOverlay } from '../lib/overlay'
import { MainPage } from './MainPage'
import { Settings } from './Settings'

afterEach(() => { cleanup(); vi.unstubAllGlobals() })

function paste(text: string) {
  const ev = new Event('paste', { bubbles: true, cancelable: true }) as unknown as ClipboardEvent
  Object.defineProperty(ev, 'clipboardData', { value: { files: [], getData: () => text } })
  document.dispatchEvent(ev)
  return ev
}

test('global paste is ignored while an overlay is open, and works again after', async () => {
  setLang('en')
  me.value = { peer_id: 'p1', role: 'host', no_auth: false, public_url: '', mdns_url: '', code: '', join_url: '', has_dir: false, max_size: 1 }
  peers.value = []
  const f = vi.fn(async () => new Response('{}', { status: 201, headers: { 'content-type': 'application/json' } }))
  vi.stubGlobal('fetch', f)
  const { getByText, unmount } = render(<MainPage />)
  getByText('Settings').click()
  await waitFor(() => expect(openOverlay.value).toBeGreaterThan(0))
  const ev = paste('hello')
  expect(ev.defaultPrevented).toBe(false)
  expect(f).not.toHaveBeenCalled()
  document.querySelector<HTMLButtonElement>('[data-close]')!.click()
  await waitFor(() => expect(openOverlay.value).toBe(0))
  expect(paste('hello').defaultPrevented).toBe(true)
  unmount()
})

test('Settings mounted increments overlay, unmount decrements', async () => {
  const before = openOverlay.value
  const { unmount } = render(<Settings onClose={() => {}} />)
  await waitFor(() => expect(openOverlay.value).toBe(before + 1))
  unmount()
  expect(openOverlay.value).toBe(before)
})
