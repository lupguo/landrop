// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/preact'
import { afterEach, expect, test, vi } from 'vitest'
import { setLang, t } from '../i18n'
import { SharePage } from './SharePage'

afterEach(() => { cleanup(); vi.unstubAllGlobals() })

function json(status: number, body: unknown, headers: Record<string, string> = {}) {
  return new Response(JSON.stringify(body), { status, headers: { 'content-type': 'application/json', ...headers } })
}

test('SharePage shows gone state on 410', async () => {
  setLang('en')
  vi.stubGlobal('fetch', vi.fn(async () => json(410, { error: 'gone' })))
  render(<SharePage id="abc" />)
  expect(await screen.findByText(t('share.gone'))).toBeTruthy()
})

test('SharePage text: does not fetch raw until "Show text" is clicked', async () => {
  setLang('en')
  const f = vi.fn(async (url: string) => {
    if (url === '/api/s/abc') return json(200, { type: 'text', name: '', size: 5, mime: 'text/plain', remaining: 1, expires_at: new Date(Date.now() + 600000).toISOString() })
    if (url === '/s/abc/raw') return new Response('hello', { status: 200, headers: { 'content-type': 'text/plain' } })
    throw new Error('unexpected ' + url)
  })
  vi.stubGlobal('fetch', f)
  render(<SharePage id="abc" />)
  const btn = await screen.findByText(t('share.show_text'))
  expect(f.mock.calls.map((c) => c[0])).toEqual(['/api/s/abc'])
  fireEvent.click(btn)
  expect(await screen.findByText('hello')).toBeTruthy()
  expect(f.mock.calls.map((c) => c[0])).toEqual(['/api/s/abc', '/s/abc/raw'])
})

test('SharePage file: download link to raw with download attr', async () => {
  setLang('en')
  vi.stubGlobal('fetch', vi.fn(async () => json(200, { type: 'file', name: 'a.zip', size: 2048, mime: 'application/zip', remaining: 2, expires_at: new Date(Date.now() + 600000).toISOString() })))
  render(<SharePage id="abc" />)
  const a = (await screen.findByText(t('share.download'))) as HTMLAnchorElement
  await waitFor(() => expect(a.getAttribute('href')).toBe('/s/abc/raw'))
  expect(a.hasAttribute('download')).toBe(true)
})

test('message share shows content and attachments without consuming', async () => {
  setLang('en')
  const f = vi.fn(async (url: string) => {
    if (url === '/api/s/abc') return json(200, {
      type: 'message', name: '', size: 0, mime: '', remaining: -1, content: 'hi there',
      attachments: [{ id: 'p1', name: 'pic.png', size: 5, mime: 'image/png' }, { id: 'd1', name: 'doc.pdf', size: 9, mime: 'application/pdf' }],
      expires_at: new Date(Date.now() + 600000).toISOString(),
    })
    throw new Error('unexpected ' + url)
  })
  vi.stubGlobal('fetch', f)
  const { container } = render(<SharePage id="abc" />)
  expect(await screen.findByText('hi there')).toBeTruthy()
  expect(screen.queryByText(t('share.show_text'))).toBeNull()
  expect(container.textContent).not.toContain(t('share.remaining', { n: -1 }))
  expect(container.querySelector('img')!.getAttribute('src')).toBe('/s/abc/attachments/p1/raw?inline=1')
  const hrefs = Array.from(container.querySelectorAll('a[download]')).map((a) => a.getAttribute('href'))
  expect(hrefs).toEqual(['/s/abc/attachments/p1/raw', '/s/abc/attachments/d1/raw'])
  expect(f.mock.calls.map((c) => c[0])).toEqual(['/api/s/abc'])
})
