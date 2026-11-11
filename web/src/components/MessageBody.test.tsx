// @vitest-environment jsdom
import { cleanup, fireEvent, render } from '@testing-library/preact'
import { afterEach, expect, test, vi } from 'vitest'
import type { Attachment } from '../api'
import { setLang, t } from '../i18n'
import { MessageBody } from './MessageBody'

afterEach(cleanup)

const atts: Attachment[] = [
  { id: 'a1', name: 'one.png', size: 10, mime: 'image/png' },
  { id: 'a2', name: 'two.png', size: 20, mime: 'image/png' },
  { id: 'a3', name: 'a.pdf', size: 30, mime: 'application/pdf' },
]
const src = (a: Attachment, inline: boolean) => `/x/${a.id}${inline ? '?inline=1' : ''}`

test('renders text, image grid in order, and file rows', () => {
  setLang('en')
  const { container, getByText } = render(<MessageBody content="hello world" attachments={atts} src={src} />)
  getByText('hello world')
  const imgs = Array.from(container.querySelectorAll('img'))
  expect(imgs.map((i) => i.getAttribute('src'))).toEqual(['/x/a1?inline=1', '/x/a2?inline=1'])
  const rows = container.querySelectorAll('.msg-file')
  expect(rows.length).toBe(1)
  expect(rows[0].textContent).toContain('a.pdf')
  expect((rows[0].querySelector('a') as HTMLAnchorElement).getAttribute('href')).toBe('/x/a3')
})

test('each image has its own download link', () => {
  setLang('en')
  const { getByLabelText } = render(<MessageBody content="" attachments={atts} src={src} />)
  for (const a of atts.slice(0, 2)) {
    const link = getByLabelText(t('item.download_name', { name: a.name })) as HTMLAnchorElement
    expect(link.getAttribute('href')).toBe(src(a, false))
    expect(link.hasAttribute('download')).toBe(true)
  }
})

test('clicking an image opens the viewer at that index', () => {
  setLang('en')
  const { container, getByText } = render(<MessageBody content="" attachments={atts} src={src} />)
  fireEvent.click(container.querySelectorAll('.msg-img')[1])
  getByText('2 / 2')
})

test('no text block when content blank', () => {
  setLang('en')
  const { container } = render(<MessageBody content={'  \n '} attachments={atts.slice(0, 1)} src={src} />)
  expect(container.querySelector('.body')).toBeNull()
  expect(container.querySelectorAll('img').length).toBe(1)
})

test('non-image files open a preview only when enabled (feed, not share)', async () => {
  setLang('en')
  const txt: Attachment[] = [{ id: 't1', name: 'notes.txt', size: 5, mime: 'text/plain' }]
  const { queryByLabelText, unmount } = render(<MessageBody content="" attachments={txt} src={src} />)
  expect(queryByLabelText('Preview notes.txt')).toBeNull()
  unmount()

  const fetch = vi.fn(async () => new Response('hello', { status: 206 }))
  vi.stubGlobal('fetch', fetch)
  const r = render(<MessageBody content="" attachments={txt} src={src} preview />)
  fireEvent.click(r.getByLabelText('Preview notes.txt'))
  await r.findByText('hello')
  expect(fetch).toHaveBeenCalledWith('/x/t1', expect.objectContaining({ headers: { Range: 'bytes=0-1048575' } }))
  r.getByLabelText('Download')
  vi.unstubAllGlobals()
})
